package server

import (
	"net/http"
	"time"
)

// GET /api/llm/usage?days=30
//
// Global token/call/error/latency aggregates from the always-on llm_usage
// ledger. Cost is intentionally not guessed here: providers price models
// differently, so the dashboard shows measured tokens and latency only.
func (s *Server) llmUsageReport(w http.ResponseWriter, r *http.Request) {
	days := atoiDefault(r.URL.Query().Get("days"), 30)
	if days < 1 {
		days = 1
	}
	if days > 365 {
		days = 365
	}
	to := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, 1)
	from := to.AddDate(0, 0, -days)
	rep, err := s.m.pg.LLMUsageReport(r.Context(), from, to)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{
		"from":       from,
		"to":         to,
		"summary":    rep.Summary,
		"daily":      rep.Daily,
		"by_model":   rep.ByModel,
		"by_profile": rep.ByProfile,
		"by_task":    rep.ByTask,
	})
}
