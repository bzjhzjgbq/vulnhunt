package llmpool

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Autumn-27/norma/llm"
)

// A gateway can answer a throttled upstream with HTTP 200 plus an SSE error frame
// — exactly what this deployment's gateway did for a drained channel:
//
//	data: {"error":{"code":"upstream_parse","message":"solo error code=4008
//	       msg=Your requests have exceeded the quota.","type":"api_error"}}
//
// norma parses such a frame into zero events, retries a few times, then closes the
// stream with the terminal events only: an empty completion shaped like a success.
// The pool used to record that as a healthy member, so the planner received an
// empty turn, logged 规划完成 with no new intents, and every task sat idle while
// the breaker — seeing nothing that looked like an error — never tripped.
//
// This drives that wire behaviour through a real provider, so the fix is verified
// end to end rather than only against a scripted stub.
func TestPoolFailsOverOnGatewayErrorFrameStream(t *testing.T) {
	var drainedCalls int
	drained := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		drainedCalls++
		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `data: {"error":{"code":"upstream_parse","message":"solo error code=4008 msg=Your requests have exceeded the quota.","type":"api_error"}}`+"\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer drained.Close()

	healthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `data: {"choices":[{"index":0,"delta":{"content":"recovered"},"finish_reason":null}]}`+"\n\n")
		_, _ = io.WriteString(w, `data: {"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`+"\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer healthy.Close()

	providerAt := func(baseURL string) llm.Provider {
		p, err := llm.NewProvider(llm.Config{
			Format:     llm.FormatOpenAI,
			BaseURL:    baseURL,
			APIKey:     "test-key",
			Model:      "test-model",
			HTTPClient: &http.Client{},
		})
		if err != nil {
			t.Fatalf("build provider: %v", err)
		}
		return p
	}

	reg := NewRegistry(nil, nil)
	pool := New([]*Member{
		member(1, "drained", 10, providerAt(drained.URL)),
		member(2, "healthy", 5, providerAt(healthy.URL)),
	}, reg)

	var text strings.Builder
	var streamErr error
	for ev, err := range pool.Stream(context.Background(), llm.CompletionRequest{
		Messages: []llm.Message{llm.UserText("ping")},
	}) {
		if err != nil {
			streamErr = err
			break
		}
		if ev.Type == llm.SETextDelta {
			text.WriteString(ev.Text)
		}
	}

	if streamErr != nil {
		t.Fatalf("pool returned %v, want a failover to the healthy member", streamErr)
	}
	if got := text.String(); got != "recovered" {
		t.Fatalf("text = %q, want the healthy member's output", got)
	}
	// norma's own empty-response retry hits the drained member more than once
	// before the pool ever sees the empty completion; what matters is that it was
	// abandoned rather than counted as a success.
	if drainedCalls == 0 {
		t.Fatal("drained member was never called")
	}
	if st := reg.Get(1); st.Fails == 0 && st.Trips == 0 {
		t.Fatal("drained member was not counted as a failure")
	}
	if st := reg.Get(2); st.Fails != 0 || st.Trips != 0 {
		t.Fatalf("healthy member was penalised: %+v", st)
	}
}
