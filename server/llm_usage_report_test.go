package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLLMUsageReportAPI(t *testing.T) {
	m, err := NewManager(t.TempDir(), "")
	if err != nil {
		t.Skipf("postgres unavailable (%v)", err)
	}
	defer m.Close()

	model := "usage-api-" + time.Now().UTC().Format("20060102150405.000000000")
	t.Cleanup(func() { _, _ = m.pg.Exec(`DELETE FROM llm_usage WHERE model=$1`, model) })
	if _, err := m.pg.Exec(`
INSERT INTO llm_usage(ts,task_id,worker,model,profile_name,latency_ms,input_tokens,output_tokens,cache_read,cache_write,status)
VALUES (now(),'usage-task','worker',$1,'usage-profile',120,40,8,3,2,'ok')`, model); err != nil {
		t.Fatal(err)
	}

	s := &Server{m: m}
	req := httptest.NewRequest(http.MethodGet, "/api/llm/usage?days=2", nil)
	rec := httptest.NewRecorder()
	s.llmUsageReport(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var out struct {
		Summary struct {
			Calls int64 `json:"calls"`
		} `json:"summary"`
		ByModel []struct {
			Key          string `json:"key"`
			Calls        int64  `json:"calls"`
			InputTokens  int64  `json:"input_tokens"`
			OutputTokens int64  `json:"output_tokens"`
		} `json:"by_model"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Summary.Calls < 1 {
		t.Fatalf("summary=%+v", out.Summary)
	}
	found := false
	for _, b := range out.ByModel {
		if b.Key == model {
			found = b.Calls == 1 && b.InputTokens == 40 && b.OutputTokens == 8
		}
	}
	if !found {
		t.Fatalf("by_model missing %s: %+v", model, out.ByModel)
	}
}
