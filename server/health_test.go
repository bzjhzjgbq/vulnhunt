package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthReportsDatabaseState(t *testing.T) {
	m, err := NewManager(t.TempDir(), "")
	if err != nil {
		t.Skipf("postgres unavailable (%v) - skipping", err)
	}
	defer m.Close()

	s := &Server{m: m}
	rec := httptest.NewRecorder()
	s.health(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"ok":true`) || !strings.Contains(body, `"database":"ok"`) {
		t.Fatalf("health body missing ok/database: %s", body)
	}
}
