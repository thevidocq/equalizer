package db_test

import (
	"context"
	"os"
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
