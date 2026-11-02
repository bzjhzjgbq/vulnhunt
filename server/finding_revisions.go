package server

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"vulnhunt/db"
	reportpkg "vulnhunt/report"
)

// GET /api/exploration/findings/{id}/revisions
func (s *Server) listFindingRevisions(w http.ResponseWriter, r *http.Request) {
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
	revs, err := s.m.pg.ListFindingRevisions(r.Context(), id)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"revisions": revs})
}

// GET /api/exploration/findings/{id}/revisions/{version}
func (s *Server) getFindingRevision(w http.ResponseWriter, r *http.Request) {
	id := int64(atoiDefault(r.PathValue("id"), 0))
	version, err := strconv.ParseInt(r.PathValue("version"), 10, 64)
	if id <= 0 || err != nil || version <= 0 {
		writeErr(w, 400, "bad finding revision")
		return
	}
	rev, err := s.m.pg.GetFindingRevision(r.Context(), id, version)
	if errors.Is(err, db.ErrFindingNotFound) {
		writeErr(w, 404, "revision not found")
		return
	}
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, rev)
}

// PUT /api/exploration/findings/{id}/report
//
// Human edit path for the report/evidence/summary. The agent path is
// update_finding_report; both append a finding_report_versions row.
func (s *Server) putFindingReport(w http.ResponseWriter, r *http.Request) {
	id := int64(atoiDefault(r.PathValue("id"), 0))
	if id <= 0 {
		writeErr(w, 400, "bad finding id")
		return
	}
	var body struct {
		Report         string `json:"report"`
		Evidence       string `json:"evidence"`
		Summary        string `json:"summary"`
		Reason         string `json:"reason"`
		ContentVersion *int64 `json:"content_version"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, 400, "bad json: "+err.Error())
		return
	}
	if body.ContentVersion == nil {
		writeErr(w, 400, "content_version 必填（先读取当前版本再保存）")
		return
	}
	if len(body.Report) > 4<<20 || len(body.Evidence) > 4<<20 || len(body.Summary) > 4<<20 {
		writeErr(w, http.StatusRequestEntityTooLarge, "报告/证据/摘要不能超过 4MB")
		return
	}
	if missing := reportpkg.ValidateFindingReport(body.Report); len(missing) > 0 {
		writeErr(w, 400, "报告不符合固定六段结构，缺少："+strings.Join(missing, "、"))
		return
	}
	reason := strings.TrimSpace(body.Reason)
	if reason == "" {
		reason = "人工编辑"
	}
	rev, err := s.m.pg.SaveFindingContent(r.Context(), id, *body.ContentVersion,
		body.Report, body.Evidence, body.Summary, reason, "human")
	if errors.Is(err, db.ErrFindingContentConflict) {
		writeErr(w, http.StatusConflict, "内容已被其他编辑更新，请刷新后重试")
		return
	}
	if errors.Is(err, db.ErrFindingNotFound) {
		writeErr(w, 404, "finding not found")
		return
	}
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	f, err := s.m.pg.GetFinding(id)
	if err != nil || f == nil {
		writeErr(w, 500, "saved but reload failed")
		return
	}
	writeJSON(w, 200, map[string]any{
		"finding":  findingFromDB(f, s.resolveAssetIDs(f.AssetIDs)),
		"revision": rev,
	})
}
