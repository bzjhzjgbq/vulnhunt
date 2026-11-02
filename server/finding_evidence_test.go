package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestFindingEvidenceAPIAndSubmission(t *testing.T) {
	m, err := NewManager(t.TempDir(), "")
	if err != nil {
		t.Skipf("postgres unavailable (%v)", err)
	}
	defer m.Close()

	id, err := m.pg.AddFinding(0, 0, "未授权文件上传", "evidence api test", "medium", "summary", "evidence", "tester", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = m.pg.DeleteFinding(id) }()
	idStr := strconv.FormatInt(id, 10)
	s := &Server{m: m}

	body := `{"kind":"command","label":"upload probe","content":"HTTP/1.1 200 OK","note":"anonymous upload"}`
	req := httptest.NewRequest(http.MethodPost, "/api/exploration/findings/"+idStr+"/evidence", strings.NewReader(body))
	req.SetPathValue("id", idStr)
	rec := httptest.NewRecorder()
	s.addFindingEvidence(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("add status=%d body=%s", rec.Code, rec.Body.String())
	}
	var added struct {
		Evidence struct {
			ID          int64  `json:"id"`
			ContentHash string `json:"content_hash"`
		} `json:"evidence"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &added); err != nil || added.Evidence.ID <= 0 || added.Evidence.ContentHash == "" {
		t.Fatalf("added=%+v err=%v", added, err)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/exploration/findings/"+idStr+"/evidence", nil)
	req.SetPathValue("id", idStr)
	rec = httptest.NewRecorder()
	s.listFindingEvidence(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "upload probe") {
		t.Fatalf("list status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/exploration/findings/"+idStr+"/submission?template=generic", nil)
	req.SetPathValue("id", idStr)
	rec = httptest.NewRecorder()
	s.findingSubmission(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "结构化证据") || !strings.Contains(rec.Body.String(), "HTTP/1.1 200 OK") {
		t.Fatalf("submission status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/exploration/findings/"+idStr+"/evidence/"+strconv.FormatInt(added.Evidence.ID, 10), nil)
	req.SetPathValue("id", idStr)
	req.SetPathValue("evidenceID", strconv.FormatInt(added.Evidence.ID, 10))
	rec = httptest.NewRecorder()
	s.deleteFindingEvidence(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status=%d body=%s", rec.Code, rec.Body.String())
	}
}
