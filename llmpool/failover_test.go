package llmpool

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/Autumn-27/norma/llm"
)

// TestShouldFailoverMatrix pins the whole decision table, with emphasis on the
// 400 split: a length-related 400 must fail over (per-channel context windows
// differ), while any other 400 must not (a malformed request fails everywhere, so
// replaying it only burns the rest of the chain).
func TestShouldFailoverMatrix(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"transport-no-status", errors.New("Post \"http://127.0.0.1:7863/v1/chat/completions\": connection refused"), true},
		{"stream-stalled", errors.New("llm: stream stalled (no data for 60s)"), true},
		{"context-canceled", context.Canceled, false},
		{"deadline-exceeded", context.DeadlineExceeded, false},

		// --- the family this change exists for ---
		{"400-context-length-openai",
			errors.New(`openai: status 400: {"error":{"message":"This model's maximum context length is 32768 tokens. However, you requested 41000 tokens"}}`), true},
		{"400-too-many-tokens",
			errors.New(`openai: status 400: {"error":{"code":"invalid_request","message":"too many tokens in request"}}`), true},
		{"400-prompt-too-long",
			errors.New(`openai: status 400: {"error":{"message":"prompt is too long: 210000 tokens > 131072 maximum"}}`), true},
		{"400-context-window-cn",
			errors.New(`openai: status 400: 请求失败：上下文长度超出模型上限`), true},
		{"400-max-tokens-param",
			errors.New(`openai: status 400: {"error":{"message":"max_tokens must be <= 8192 for this model"}}`), true},

		// --- genuine bad requests must still fail fast ---
		{"400-malformed-json",
			errors.New(`openai: status 400: {"error":{"code":"invalid_request","message":"invalid JSON body: unexpected end of JSON input"}}`), false},
		{"400-bad-role",
			errors.New(`openai: status 400: {"error":{"message":"messages[1].role must be one of system,user,assistant"}}`), false},
		{"400-model-not-found",
			errors.New(`openai: status 400: {"error":{"code":"model_not_found","message":"no such model"}}`), false},

		// --- unchanged neighbours ---
		{"401", errors.New("openai: status 401: unauthorized"), true},
		{"402-quota", errors.New("openai: status 402: 积分已耗尽，调用失败"), true},
		{"403", errors.New("openai: status 403: forbidden"), true},
		{"404", errors.New("openai: status 404: not found"), true},
		{"408", errors.New("openai: status 408: request timeout"), true},
		{"429", errors.New("openai: status 429: rate limited"), true},
		{"500", errors.New("openai: status 500: internal"), true},
		{"502", errors.New("openai: status 502: bad gateway"), true},
		{"503-no-healthy-account", errors.New(`openai: status 503: {"error":{"code":"no_healthy_account"}}`), true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldFailover(ctx, tc.err); got != tc.want {
				t.Fatalf("shouldFailover(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

// TestShouldFailoverCancelledCtxWins proves a cancelled context suppresses
// failover even for an otherwise-retryable error: the caller gave up, and a
// backup key must not be spent on it.
func TestShouldFailoverCancelledCtxWins(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, err := range []error{
		errors.New("openai: status 503: no_healthy_account"),
		errors.New(`openai: status 400: {"error":{"message":"maximum context length is 8192 tokens"}}`),
	} {
		if shouldFailover(ctx, err) {
			t.Fatalf("shouldFailover with cancelled ctx = true for %v, want false", err)
		}
	}
}

// TestPoolFailsOverOnLengthLimit400 is the end-to-end proof of the fix: the head
// rejects the prompt as too long and the next member answers. Before this change
// the pool stopped at the head and the whole request failed.
func TestPoolFailsOverOnLengthLimit400(t *testing.T) {
	head := &fakeProv{errs: []error{
		fmt.Errorf(`openai: status 400: {"error":{"message":"This model's maximum context length is 8192 tokens"}}`),
	}}
	next := okProv("big-window", "ok")

	p := New([]*Member{member(1, "small-window", 10, head), member(2, "big-window", 5, next)}, NewRegistry(nil, nil))

	got, err := drain(p.Stream(context.Background(), llm.CompletionRequest{}))
	if err != nil {
		t.Fatalf("expected failover past the length-limit 400, got %v", err)
	}
	if got != "ok" {
		t.Fatalf("text = %q, want %q", got, "ok")
	}
	if head.calls != 1 {
		t.Fatalf("head provider called %d times, want 1", head.calls)
	}
}

// TestPoolDoesNotFailoverOnMalformedRequest is the guard rail: a bad request must
// not be replayed down the chain.
func TestPoolDoesNotFailoverOnMalformedRequest(t *testing.T) {
	head := &fakeProv{errs: []error{
		fmt.Errorf(`openai: status 400: {"error":{"code":"invalid_request","message":"invalid JSON body"}}`),
	}}
	next := okProv("next", "should-not-run")

	p := New([]*Member{member(1, "head", 10, head), member(2, "next", 5, next)}, NewRegistry(nil, nil))

	if _, err := drain(p.Stream(context.Background(), llm.CompletionRequest{})); err == nil {
		t.Fatal("expected error for malformed request, got nil")
	}
	if next.calls != 0 {
		t.Fatalf("second provider called %d times, want 0 (bad request must not fail over)", next.calls)
	}
}
