package server

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/Autumn-27/artex/db"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// liveArtex answers whether a real ARTEX instance is serving the same machine.
// Server tests load every task from PostgreSQL and start the engine for them, so
// running them against a live deployment's database steals its intents and leaves
// workers pointed at a closed pool (observed 2026-10-31: tests claimed production
// intents and every finding/traffic test failed on "database is closed").
func liveArtex() bool {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get("http://127.0.0.1:8787/api/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// TestMain acquires a PostgreSQL advisory lock (7337741002) for the entire
// server test suite so cross-package DELETE cleanup races with db/agent
// packages are avoided when running `go test ./...`.
//
// It also refuses to run against a live instance's database unless explicitly
// overridden: set ARTEX_TESTS_ALLOW_LIVE=1 when that interference is understood.
func TestMain(m *testing.M) {
	if os.Getenv("ARTEX_TESTS_ALLOW_LIVE") != "1" && liveArtex() {
		fmt.Fprintln(os.Stderr, "ARTEX is serving on 127.0.0.1:8787; skipping server tests to protect the live database "+
			"(set ARTEX_TESTS_ALLOW_LIVE=1 to run anyway)")
		os.Exit(0)
	}
	dsn, _, err := db.DSN()
	if err != nil {
		os.Exit(m.Run())
	}
	conn, err := sql.Open("pgx", dsn)
	if err != nil || conn.Ping() != nil {
		os.Exit(m.Run())
	}
	defer conn.Close()
	if _, err := conn.Exec(`SELECT pg_advisory_lock(7337741002)`); err != nil {
		os.Exit(m.Run())
	}
	defer conn.Exec(`SELECT pg_advisory_unlock(7337741002)`) //nolint:errcheck
	os.Exit(m.Run())
}
