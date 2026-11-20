package db_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/thevidocq/equalizer/panel-api/internal/db"
	"github.com/thevidocq/equalizer/panel-api/internal/dbtest"
)

var pg *dbtest.Server

func TestMain(m *testing.M) {
	var err error
	pg, err = dbtest.Start()
	if err != nil {
		panic(err)
	}
	code := m.Run()
	pg.Stop()
	os.Exit(code)
}

func TestMigrateIsIdempotent(t *testing.T) {
	if pg == nil {
		t.Skip("PostgreSQL not installed")
	}
	conn := pg.NewDatabase(t)
	applied, err := db.Migrate(context.Background(), conn)
	if err != nil || len(applied) != 0 {
		t.Fatalf("second run: %v %v", applied, err)
	}
	var n int
	if err := conn.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&n); err != nil || n == 0 {
		t.Fatalf("migrations table: %d %v", n, err)
	}
}

// The migration that removed the maps' "risk" field: the label goes, a form
// field named risk stays.
func TestDropRiskMigration(t *testing.T) {
	if pg == nil {
		t.Skip("PostgreSQL not installed")
	}
	conn := pg.NewDatabase(t)
	sql, err := os.ReadFile("migrations/0017_drop_map_risk.sql")
	if err != nil {
		t.Fatal(err)
	}
	body := string(sql)
	fn := body[strings.Index(body, "CREATE FUNCTION"):strings.Index(body, "UPDATE map_versions")]
	if _, err := conn.Exec(fn); err != nil {
		t.Fatal(err)
	}
	defer conn.Exec(`DROP FUNCTION eq_drop_risk(jsonb)`) //nolint:errcheck
	var out string
	err = conn.QueryRow(`SELECT eq_drop_risk($1::jsonb)::text`,
		`{"routes":[{"id":"a","risk":"low","x":[1,{"risk":"high"}]}],"forms":[{"fields":{"risk":{"type":"int"}}}],"e":[]}`).Scan(&out)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"e": [], "forms": [{"fields": {"risk": {"type": "int"}}}], "routes": [{"x": [1, {}], "id": "a"}]}`
	if out != want {
		t.Errorf("got %s\nwant %s", out, want)
	}
}
