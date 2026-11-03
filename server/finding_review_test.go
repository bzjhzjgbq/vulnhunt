package server

import (
	"context"
	"iter"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Autumn-27/norma/llm"
	"vulnhunt/agent"
	"vulnhunt/db"
)

type findingReviewProvider struct {
	text string
	req  llm.CompletionRequest
}

func (p *findingReviewProvider) Complete(_ context.Context, req llm.CompletionRequest) (llm.Message, string, llm.Usage, error) {
	p.req = req
	return llm.Message{Role: llm.RoleAssistant, Content: []llm.ContentBlock{llm.TextBlock(p.text)}}, "end_turn", llm.Usage{InputTokens: 10, OutputTokens: 5}, nil
}

func (*findingReviewProvider) Stream(context.Context, llm.CompletionRequest) iter.Seq2[llm.StreamEvent, error] {
	return func(yield func(llm.StreamEvent, error) bool) { yield(llm.StreamEvent{}, nil) }
}

func waitFindingReview(t *testing.T, s *Server) *findingReviewJob {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		s.reviewMu.Lock()
		job := s.reviewJob
		running := job != nil && job.Running
		s.reviewMu.Unlock()
		if job != nil && !running {
			return job.snapshot()
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("finding review did not finish")
	return nil
}

func TestFindingReviewBatchAndTriageReason(t *testing.T) {
	m, err := NewManager(t.TempDir(), "")
	if err != nil {
		t.Skipf("postgres unavailable (%v)", err)
	}
	defer m.Close()

	id, err := m.pg.AddFinding(0, 0, "信息泄露", "review test", "high", "summary", "HTTP/1.1 200 OK secret=abc", "worker", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = m.pg.DeleteFinding(id) }()

	s := &Server{m: m, ctx: context.Background()}
	prov := &findingReviewProvider{text: "```json\n{\"verdict\":\"verified\",\"reason\":\"请求与响应一致\",\"missing\":[]}\n```"}
	s.llmOn = true
	s.llmProv = prov
	s.llmCfg = agent.Config{Model: "review-test"}

	body := `{"ids":[` + i64s(id) + `]}`
	req := httptest.NewRequest(http.MethodPost, "/api/exploration/finding-review", strings.NewReader(body))
	rec := httptest.NewRecorder()
	s.startFindingReview(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("start status=%d body=%s", rec.Code, rec.Body.String())
	}
	job := waitFindingReview(t, s)
	if job.Total != 1 || job.Processed != 1 || job.Verified != 1 || job.Failed != 0 {
		t.Fatalf("job=%+v", job)
	}
	if len(prov.req.System) != 1 || !strings.Contains(prov.req.System[0], "复核员") {
		t.Fatalf("review prompt not sent: %+v", prov.req.System)
	}
	if len(prov.req.Messages) != 1 || !strings.Contains(prov.req.Messages[0].Text(), "secret=abc") {
		t.Fatalf("finding evidence not sent: %+v", prov.req.Messages)
	}

	f, err := m.pg.GetFinding(id)
	if err != nil {
		t.Fatal(err)
	}
	if f.ReviewStatus != db.FindingReviewVerified || !strings.Contains(f.ReviewReason, "请求与响应一致") || f.ReviewModel != "review-test" || f.ReviewedAt == nil {
		t.Fatalf("finding review not stored: %+v", f)
	}
	if f.Status != db.FindingPending {
		t.Fatalf("review must not change human status: %s", f.Status)
	}

	// Triage reason is stored independently of status and validates length.
	patch := httptest.NewRequest(http.MethodPatch, "/", strings.NewReader(`{"triage_reason":"仅暴露公开信息"}`))
	patch.SetPathValue("id", i64s(id))
	rec = httptest.NewRecorder()
	s.patchFinding(rec, patch)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "仅暴露公开信息") {
		t.Fatalf("triage reason patch status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestParseFindingReviewVerdict(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		ok   bool
	}{
		{"fenced", "```json\n{\"verdict\":\"likely_fp\",\"reason\":\"仅是版本匹配\",\"missing\":[]}\n```", true},
		{"insufficient", `{"verdict":"evidence_insufficient","reason":"缺响应","missing":["原始响应"]}`, true},
		{"bad verdict", `{"verdict":"maybe","reason":"x"}`, false},
		{"no reason", `{"verdict":"verified"}`, false},
		{"not json", `同意`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v, err := parseFindingReviewVerdict(tc.in)
			if tc.ok && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tc.ok && err == nil {
				t.Fatalf("expected error, got %+v", v)
			}
		})
	}
}
