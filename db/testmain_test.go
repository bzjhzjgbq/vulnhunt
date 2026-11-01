package db

import (
	"database/sql"
	"net/http"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// liveVulnhunt reports whether a real instance is serving on this machine. The
// db tests create findings/tasks directly in the configured database and clean
// them up imperfectly; running them against a live deployment pollutes it (on
// 2026-10-31 the findings table accumulated 107 fixture rows this way).
func liveVulnhunt() bool {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get("http://127.0.0.1:8787/api/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// TestMain acquires a PostgreSQL advisory lock (7337741002) for the entire
// db test suite. Packages agent and server hold the same lock, so parallel
// `go test ./...` runs serialize across packages on the shared dev DB and
// avoid cross-package cleanup races (e.g. DELETE FROM assets WHERE id > X
// from one package deleting assets created by another).
func TestMain(m *testing.M) {
	if os.Getenv("VULNHUNT_TESTS_ALLOW_LIVE") != "1" && liveVulnhunt() {
		os.Stderr.WriteString("VULNHUNT is serving on 127.0.0.1:8787; skipping db tests to protect the live database " +
			"(set VULNHUNT_TESTS_ALLOW_LIVE=1 to run anyway)\n")
		os.Exit(0)
	}
	dsn, _, err := DSN()
	if err != nil {
		// No DB configured — tests that need PG will skip themselves.
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
