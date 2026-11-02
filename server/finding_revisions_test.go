package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

const revisionTestReport = `# 测试漏洞

- 漏洞等级：**高危**

## 一句话结论

未授权接口可读取数据。

## 成立前提

匿名可访问。

## 关键动作

curl 请求返回 200。

## 影响范围

测试范围。
`

func TestFindingReportEditAndRevisions(t *testing.T) {
	m, err := NewManager(t.TempDir(), "")
	if err != nil {
		t.Skipf("postgres unavailable (%v)", err)
	}
	defer m.Close()

	id, err := m.pg.AddFinding(0, 0, "XSS", "revision api test", "high", "summary", "evidence", "tester", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = m.pg.DeleteFinding(id) }()
	idStr := strconv.FormatInt(id, 10)
	s := &Server{m: m}

	put := func(version int64, report string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]any{
			"report": report, "evidence": "new evidence", "summary": "new summary",
			"content_version": version, "reason": "test edit",
		})
		req := httptest.NewRequest(http.MethodPut, "/api/exploration/findings/"+idStr+"/report", strings.NewReader(string(body)))
		req.SetPathValue("id", idStr)
		rec := httptest.NewRecorder()
		s.putFindingReport(rec, req)
		return rec
	}

	if rec := put(0, revisionTestReport); rec.Code != http.StatusOK {
		t.Fatalf("save status=%d body=%s", rec.Code, rec.Body.String())
	}
	var saved struct {
		Finding struct {
			ContentVersion int64  `json:"content_version"`
			Report         string `json:"report"`
			Evidence       string `json:"evidence"`
		} `json:"finding"`
		Revision struct {
			Version int64 `json:"version"`
		} `json:"revision"`
	}
	rec := put(1, revisionTestReport+"\n第二次")
	if rec.Code != http.StatusOK {
		t.Fatalf("second save status=%d body=%s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Finding.ContentVersion != 2 || saved.Revision.Version != 2 || saved.Finding.Evidence != "new evidence" {
		t.Fatalf("saved=%+v", saved)
	}

	if rec := put(1, revisionTestReport); rec.Code != http.StatusConflict {
		t.Fatalf("stale save status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := put(2, "# 缺少章节"); rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid report status=%d body=%s", rec.Code, rec.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/api/exploration/findings/"+idStr+"/revisions", nil)
	req.SetPathValue("id", idStr)
	rec = httptest.NewRecorder()
	s.listFindingRevisions(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list revisions status=%d body=%s", rec.Code, rec.Body.String())
	}
	var list struct {
		Revisions []struct {
			Version int64 `json:"version"`
		} `json:"revisions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list.Revisions) != 2 || list.Revisions[0].Version != 2 {
		t.Fatalf("revisions=%+v err=%v", list, err)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/exploration/findings/"+idStr+"/submission?template=butian", nil)
	req.SetPathValue("id", idStr)
	rec = httptest.NewRecorder()
	s.findingSubmission(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("submission status=%d body=%s", rec.Code, rec.Body.String())
	}
	var sub struct {
		Template string `json:"template"`
		Markdown string `json:"markdown"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &sub); err != nil || sub.Template != "butian" || !strings.Contains(sub.Markdown, "测试漏洞") {
		t.Fatalf("submission=%+v err=%v", sub, err)
	}
}
