package agent

import (
	"database/sql"
	"net/http"
	"os"
	"testing"
	"time"

	"vulnhunt/db"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// liveVulnhunt reports whether a real instance is serving on this machine. The
// agent tests write findings/intents into the configured database; running them
// against a live deployment pollutes production data.
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
// agent test suite so cross-package DELETE cleanup races with db/server
// packages are avoided when running `go test ./...`.
func TestMain(m *testing.M) {
	if os.Getenv("VULNHUNT_TESTS_ALLOW_LIVE") != "1" && liveVulnhunt() {
		os.Stderr.WriteString("VULNHUNT is serving on 127.0.0.1:8787; skipping agent tests to protect the live database " +
			"(set VULNHUNT_TESTS_ALLOW_LIVE=1 to run anyway)\n")
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
