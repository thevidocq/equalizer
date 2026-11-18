// Package dbtest starts a throwaway PostgreSQL cluster for tests. Tests
// that need it are skipped when PostgreSQL is not installed.
package dbtest

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/thevidocq/equalizer/panel-api/internal/db"
)

// Server is a running test cluster.
type Server struct {
	dir  string
	port int
	cmd  *exec.Cmd
	asPG bool
}

func binDir() string {
	if d := os.Getenv("EQ_PG_BIN"); d != "" {
		return d
	}
	matches, _ := filepath.Glob("/usr/lib/postgresql/*/bin/initdb")
	if len(matches) == 0 {
		if p, err := exec.LookPath("initdb"); err == nil {
			return filepath.Dir(p)
		}
		return ""
	}
	return filepath.Dir(matches[len(matches)-1])
}

// Start initialises and starts a cluster, or returns nil when PostgreSQL is
// not installed.
func Start() (*Server, error) {
	bin := binDir()
	if bin == "" {
		return nil, nil
	}
	dir, err := os.MkdirTemp("", "eq-pg-")
	if err != nil {
		return nil, err
	}
	s := &Server{dir: dir, asPG: os.Geteuid() == 0}
	if s.asPG {
		// initdb refuses to run as root: use the postgres account.
		if err := exec.Command("chown", "postgres", dir).Run(); err != nil {
			return nil, err
		}
		_ = os.Chmod(dir, 0o700)
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	s.port = l.Addr().(*net.TCPAddr).Port
	l.Close()
	data := filepath.Join(dir, "data")
	if out, err := s.run(filepath.Join(bin, "initdb"), "-D", data, "-U", "eq", "-A", "trust", "--no-sync").CombinedOutput(); err != nil {
		return nil, fmt.Errorf("initdb: %v: %s", err, out)
	}
	s.cmd = s.run(filepath.Join(bin, "postgres"), "-D", data, "-p", strconv.Itoa(s.port), "-k", dir,
		"-c", "listen_addresses=127.0.0.1", "-c", "fsync=off", "-c", "full_page_writes=off")
	if err := s.cmd.Start(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Server) run(name string, args ...string) *exec.Cmd {
	var cmd *exec.Cmd
	if s.asPG {
		cmd = exec.Command("runuser", append([]string{"-u", "postgres", "--", name}, args...)...)
	} else {
		cmd = exec.Command(name, args...)
	}
	// The postgres account must be able to read its working directory.
	cmd.Dir = s.dir
	// Own process group: Stop signals postgres even behind runuser.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd
}

// DSN returns the connection string of database name.
func (s *Server) DSN(name string) string {
	return fmt.Sprintf("postgres://eq@127.0.0.1:%d/%s?sslmode=disable", s.port, name)
}

// NewDatabase creates an empty migrated database for one test.
func (s *Server) NewDatabase(t testing.TB) *sql.DB {
	t.Helper()
	ctx := context.Background()
	admin, err := db.Open(ctx, s.DSN("postgres"), 20*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	name := fmt.Sprintf("t%d", time.Now().UnixNano())
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	conn, err := db.Open(ctx, s.DSN(name), 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if _, err := db.Migrate(ctx, conn); err != nil {
		t.Fatal(err)
	}
	return conn
}

// Stop stops the cluster and removes its files.
func (s *Server) Stop() {
	if s == nil {
		return
	}
	if s.cmd != nil && s.cmd.Process != nil {
		// SIGINT is PostgreSQL's fast shutdown.
		_ = syscall.Kill(-s.cmd.Process.Pid, syscall.SIGINT)
		done := make(chan struct{})
		go func() { _, _ = s.cmd.Process.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			_ = syscall.Kill(-s.cmd.Process.Pid, syscall.SIGKILL)
		}
	}
	_ = os.RemoveAll(s.dir)
}
