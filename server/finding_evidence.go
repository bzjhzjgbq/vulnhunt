package server

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"vulnhunt/db"
)

var findingEvidenceKinds = map[string]bool{
	"http":       true,
	"command":    true,
	"file":       true,
	"screenshot": true,
	"text":       true,
}

// GET /api/exploration/findings/{id}/evidence
func (s *Server) listFindingEvidence(w http.ResponseWriter, r *http.Request) {
	id := int64(atoiDefault(r.PathValue("id"), 0))
	if id <= 0 {
		writeErr(w, 400, "bad finding id")
		return
	}
	f, err := s.m.pg.GetFinding(id)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if f == nil {
		writeErr(w, 404, "finding not found")
		return
	}
	items, err := s.m.pg.ListFindingEvidence(r.Context(), id)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"evidence": items})
}

// POST /api/exploration/findings/{id}/evidence
func (s *Server) addFindingEvidence(w http.ResponseWriter, r *http.Request) {
	id := int64(atoiDefault(r.PathValue("id"), 0))
	if id <= 0 {
		writeErr(w, 400, "bad finding id")
		return
	}
	var body struct {
		Kind    string `json:"kind"`
		Label   string `json:"label"`
		Content string `json:"content"`
		Note    string `json:"note"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, 400, "bad json: "+err.Error())
		return
	}
	kind := strings.TrimSpace(strings.ToLower(body.Kind))
	if kind == "" {
		kind = "text"
	}
	if !findingEvidenceKinds[kind] {
		writeErr(w, 400, "kind 只支持 http / command / file / screenshot / text")
		return
	}
	if len(body.Content) > 4<<20 {
		writeErr(w, http.StatusRequestEntityTooLarge, "证据内容不能超过 4MB")
		return
	}
	if strings.TrimSpace(body.Content) == "" {
		writeErr(w, 400, "证据内容不能为空")
		return
	}
	ev, err := s.m.pg.AddFindingEvidence(r.Context(), id, kind,
		strings.TrimSpace(body.Label), body.Content, strings.TrimSpace(body.Note), "human")
	if errors.Is(err, db.ErrFindingNotFound) {
		writeErr(w, 404, "finding not found")
		return
	}
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 201, map[string]any{"evidence": ev})
}

// DELETE /api/exploration/findings/{id}/evidence/{evidenceID}
func (s *Server) deleteFindingEvidence(w http.ResponseWriter, r *http.Request) {
	id := int64(atoiDefault(r.PathValue("id"), 0))
	evID, err := strconv.ParseInt(r.PathValue("evidenceID"), 10, 64)
	if id <= 0 || err != nil || evID <= 0 {
		writeErr(w, 400, "bad finding evidence id")
		return
	}
	err = s.m.pg.DeleteFindingEvidence(r.Context(), id, evID)
	if errors.Is(err, db.ErrFindingEvidenceNotFound) {
		writeErr(w, 404, "evidence not found")
		return
	}
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"deleted": true, "id": evID})
}
