package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"vulnhunt/db"
)

// Bounty ledger API: record per-platform submissions and read the aggregate.

func (s *Server) bountySubmissions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := db.BountyFilter{
		FindingID: int64(atoiDefault(q.Get("finding_id"), 0)),
		ProgramID: int64(atoiDefault(q.Get("program_id"), 0)),
		Status:    q.Get("status"),
	}
	items, err := s.m.pg.ListBountySubmissions(r.Context(), filter)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"items": items, "total": len(items)})
}

func (s *Server) bountySummary(w http.ResponseWriter, r *http.Request) {
	sum, err := s.m.pg.BountySpend(r.Context())
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, sum)
}

type bountyBody struct {
	FindingID   int64      `json:"finding_id"`
	ProgramID   *int64     `json:"program_id"`
	Platform    string     `json:"platform"`
	ExternalID  string     `json:"external_id"`
	Status      string     `json:"status"`
	Severity    string     `json:"severity"`
	AmountCents int64      `json:"amount_cents"`
	Currency    string     `json:"currency"`
	SubmittedAt *time.Time `json:"submitted_at"`
	DecidedAt   *time.Time `json:"decided_at"`
	PaidAt      *time.Time `json:"paid_at"`
	Note        string     `json:"note"`
}

func (b bountyBody) input() db.BountySubmissionInput {
	return db.BountySubmissionInput{
		FindingID: b.FindingID, ProgramID: b.ProgramID, Platform: b.Platform,
		ExternalID: b.ExternalID, Status: b.Status, Severity: b.Severity,
		AmountCents: b.AmountCents, Currency: b.Currency,
		SubmittedAt: b.SubmittedAt, DecidedAt: b.DecidedAt, PaidAt: b.PaidAt, Note: b.Note,
	}
}

func (s *Server) bountyCreate(w http.ResponseWriter, r *http.Request) {
	var body bountyBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, 400, "bad json: "+err.Error())
		return
	}
	row, err := s.m.pg.CreateBountySubmission(r.Context(), body.input())
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	writeJSON(w, 201, row)
}

func (s *Server) bountyUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, 400, "bad submission id")
		return
	}
	var body bountyBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, 400, "bad json: "+err.Error())
		return
	}
	row, err := s.m.pg.UpdateBountySubmission(r.Context(), id, body.input())
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if row == nil {
		writeErr(w, 404, "submission not found")
		return
	}
	writeJSON(w, 200, row)
}

func (s *Server) bountyDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, 400, "bad submission id")
		return
	}
	ok, err := s.m.pg.DeleteBountySubmission(r.Context(), id)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if !ok {
		writeErr(w, 404, "submission not found")
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
