package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
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

	for _, format := range []string{"md-single", "md-zip", "json"} {
		exportURL := "/api/exploration/findings/export?scope=selected&format=" + format + "&ids=" + idStr
		req = httptest.NewRequest(http.MethodGet, exportURL, nil)
		rec = httptest.NewRecorder()
		s.findingsExport(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s export status=%d body=%s", format, rec.Code, rec.Body.String())
		}
		if format == "md-single" {
			if !strings.Contains(rec.Body.String(), "结构化证据") || !strings.Contains(rec.Body.String(), "HTTP/1.1 200 OK") {
				t.Fatalf("md-single export missing structured evidence:\n%s", rec.Body.String())
			}
			continue
		}
		if format == "json" {
			if !strings.Contains(rec.Body.String(), `"structured_evidence"`) || !strings.Contains(rec.Body.String(), "HTTP/1.1 200 OK") {
				t.Fatalf("json export missing structured evidence:\n%s", rec.Body.String())
			}
			continue
		}
		archive, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
		if err != nil {
			t.Fatal(err)
		}
		var bodyFile, sidecar, linked bool
		for _, entry := range archive.File {
			r, err := entry.Open()
			if err != nil {
				t.Fatal(err)
			}
			raw, err := io.ReadAll(r)
			r.Close()
			if err != nil {
				t.Fatal(err)
			}
			switch {
			case strings.HasSuffix(entry.Name, ".md"):
				if bytes.Contains(raw, []byte("/structured/")) {
					linked = true
				}
			case strings.Contains(entry.Name, "/structured/") && strings.HasSuffix(entry.Name, ".txt"):
				bodyFile = bytes.Contains(raw, []byte("HTTP/1.1 200 OK"))
			case strings.Contains(entry.Name, "/structured/") && strings.HasSuffix(entry.Name, ".json"):
				sidecar = bytes.Contains(raw, []byte(added.Evidence.ContentHash))
			}
		}
		if !bodyFile || !sidecar || !linked {
			t.Fatalf("md-zip missing structured evidence (body=%v sidecar=%v linked=%v): %v", bodyFile, sidecar, linked, archive.File)
		}
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
