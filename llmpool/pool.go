package llmpool

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"log"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/Autumn-27/norma/llm"
)

// Member is one LLM profile in the chain, already built into a provider (wrapped
// with the recorder by the caller, so a failed attempt is still recorded under
// its own profile name).
type Member struct {
	ID       int64  // llm_profiles.id
	Name     string // profile name, for logs / UI
	Model    string
	Format   string // "anthropic" | "openai"
	Priority int    // the profile's configured priority (display only)
	Active   bool   // is_default (display only)
	// Rank is the ordering key the caller assigned: higher goes first, and members
	// sharing a Rank take turns leading (load-spreading across duplicate keys).
	// The caller encodes "active profile heads the chain" as a Rank above every
	// user-settable priority, so this type needs no policy of its own.
	Rank int
	// WindowTokens is the profile's context window in tokens. A member whose
	// window can't hold the request is skipped rather than made to fail on it.
	WindowTokens int
	Prov         llm.Provider
}

// RankActive is the Rank the caller gives the chain head (the active profile, or
// an explicitly bound one) so it always outranks any configured priority.
const RankActive = int(^uint(0)>>1) - 1

// ErrExhausted is returned when every member of the chain failed.
var ErrExhausted = errors.New("LLM 轮询：所有配置均不可用")

// errEmptyCompletion marks a member whose stream closed without producing any
// model output.
//
// Some gateways answer a throttled/broken upstream with HTTP 200 plus an SSE
// error frame (`data: {"error":{"message":"... exceeded the quota"}}`). norma
// parses such a frame into zero events, so its own empty-response retry loop
// (norma/llm/openai.go: Stream) exhausts and then closes the stream with just
// the terminal events — a success by shape and nothing at all in fact. Before
// this, the pool recorded that as a healthy member (Registry.Pass), so the
// caller silently received an empty turn: the planner logged 规划完成 with zero
// new intents and every task stalled, while the breaker never tripped because
// nothing looked like an error. Treating it as a failure routes around the
// member and lets the breaker do its job.
var errEmptyCompletion = errors.New("LLM 轮询：空完成（流未产出任何内容）")

// isTerminalEvent reports whether ev carries no model output of its own — it
// only closes the message (stop reason / usage / stop marker). Every other event
// type is model output (text, thinking, tool call), so its arrival proves the
// member actually produced something.
func isTerminalEvent(t llm.StreamEventType) bool {
	return t == llm.SEMessageDelta || t == llm.SEMessageStop
}

// Pool is an llm.Provider that fails over across an ordered chain of members.
// It is safe for concurrent use: members are immutable after construction and
// all mutable state lives in the shared Registry.
type Pool struct {
	members []*Member // in chain order (active first, then priority DESC)
	health  *Registry
	rr      atomic.Uint64 // rotates the starting point within an equal-priority group
}

// New builds a Pool over members (already in chain order). Returns nil when the
// chain is empty. A single-member chain is still a valid Pool — it just behaves
// exactly like the bare provider.
func New(members []*Member, health *Registry) *Pool {
	if len(members) == 0 {
		return nil
	}
	if health == nil {
		health = NewRegistry(nil, nil)
	}
	return &Pool{members: members, health: health}
}

// Members returns the chain in order (read-only).
func (p *Pool) Members() []*Member { return p.members }

// Head returns the first member of the chain.
func (p *Pool) Head() *Member { return p.members[0] }

// Stream implements llm.Provider with failover.
//
// The one hard rule: a member may only be abandoned BEFORE it has yielded any
// event. Once text or a tool_use has reached the caller, re-sending the same
// request to another model would duplicate output and corrupt the conversation
// history — so a mid-stream failure is surfaced as-is and left to the agent
// harness's resume logic. Fortunately the failures this exists for (402 no
// credit, 401 bad key, 429, 5xx) all surface during request establishment,
// before the body is read, so they always land in the safe window.
//
// Terminal events (message_delta / message_stop) are held back until the member
// is known to have produced output. A stream carrying nothing but those is an
// empty completion (see errEmptyCompletion), and forwarding its stop marker would
// end the caller's turn as though the model had answered.
func (p *Pool) Stream(ctx context.Context, req llm.CompletionRequest) iter.Seq2[llm.StreamEvent, error] {
	order := p.order(req)
	return func(yield func(llm.StreamEvent, error) bool) {
		var lastErr error
		for i, m := range order {
			emitted := false  // anything reached the caller
			produced := false // model output (text / thinking / tool) reached the caller
			var pending []llm.StreamEvent
			var failed error
			for ev, err := range m.Prov.Stream(ctx, req) {
				if err != nil {
					if !emitted && shouldFailover(ctx, err) {
						failed = err
						break // safe window: nothing reached the caller yet
					}
					if !yield(ev, err) {
						return // caller stopped consuming (cancel / early exit)
					}
					return // terminal error already handed to the caller
				}
				if isTerminalEvent(ev.Type) {
					pending = append(pending, ev) // hold back until output is proven
					continue
				}
				produced = true
				emitted = true
				if !yield(ev, nil) {
					return // caller stopped consuming (cancel / early exit)
				}
			}
			if failed == nil && !produced {
				failed = errEmptyCompletion
			}
			if failed == nil {
				for _, ev := range pending {
					if !yield(ev, nil) {
						return
					}
				}
				p.health.Pass(m.ID) // completed and produced output
				return
			}
			lastErr = failed
			hard := isHardFailure(failed)
			if p.health.Trip(m.ID, trimErr(failed), hard) {
				log.Printf("[llmpool] 配置 %q(%s) 已熔断：%s", m.Name, m.Model, trimErr(failed))
			}
			if i+1 < len(order) {
				n := order[i+1]
				log.Printf("[llmpool] LLM 故障转移：%q(%s) → %q(%s)，原因：%s",
					m.Name, m.Model, n.Name, n.Model, trimErr(failed))
			}
		}
		if lastErr == nil {
			lastErr = ErrExhausted
		}
		log.Printf("[llmpool] 轮询链已耗尽(%d 个配置全部失败)，最后错误：%s", len(order), trimErr(lastErr))
		yield(llm.StreamEvent{}, fmt.Errorf("%w：%v", ErrExhausted, lastErr))
	}
}

// Complete implements llm.Provider with failover for non-streaming calls. A
// non-streaming request is atomic — it never delivers partial output — so every
// failover-eligible failure lands in the safe window and the next member can be
// tried without risk of duplicated output. Mirrors Stream's health-tripping and
// chain-exhaustion behavior.
func (p *Pool) Complete(ctx context.Context, req llm.CompletionRequest) (llm.Message, string, llm.Usage, error) {
	order := p.order(req)
	var lastErr error
	for i, m := range order {
		msg, sr, usage, err := m.Prov.Complete(ctx, req)
		if err == nil {
			p.health.Pass(m.ID)
			return msg, sr, usage, nil
		}
		if !shouldFailover(ctx, err) {
			// Non-failover error (e.g. ctx cancel, deterministic 4xx): surface as-is
			// without tripping health, matching Stream's non-failover path.
			p.health.Pass(m.ID)
			return llm.Message{}, "", llm.Usage{}, err
		}
		lastErr = err
		hard := isHardFailure(err)
		if p.health.Trip(m.ID, trimErr(err), hard) {
			log.Printf("[llmpool] 配置 %q(%s) 已熔断：%s", m.Name, m.Model, trimErr(err))
		}
		if i+1 < len(order) {
			n := order[i+1]
			log.Printf("[llmpool] LLM 故障转移：%q(%s) → %q(%s)，原因：%s",
				m.Name, m.Model, n.Name, n.Model, trimErr(err))
		}
	}
	if lastErr == nil {
		lastErr = ErrExhausted
	}
	log.Printf("[llmpool] 轮询链已耗尽(%d 个配置全部失败)，最后错误：%s", len(order), trimErr(lastErr))
	return llm.Message{}, "", llm.Usage{}, fmt.Errorf("%w：%v", ErrExhausted, lastErr)
}

// order picks the members to try, in order: skip those in a cooling-off window
// and those whose context window can't hold this request, then rotate within each
// equal-priority group so same-priority profiles share the load. Never returns an
// empty slice — if everything is filtered out, the head of the chain is tried
// anyway, since stalling the engine is worse than one more failed request.
func (p *Pool) order(req llm.CompletionRequest) []*Member {
	est := estimateTokens(req)
	var open []*Member
	for _, m := range p.members {
		if p.health.IsOpen(m.ID) {
			continue
		}
		if m.WindowTokens > 0 && est > m.WindowTokens {
			continue // would 400 on length — not a useful failover target
		}
		open = append(open, m)
	}
	if len(open) == 0 {
		return p.members[:1] // last resort: probe the head rather than stall
	}
	return rotateGroups(open, p.rr.Add(1)-1)
}

// rotateGroups rotates each run of equal-ranked members by n, so profiles sharing
// a priority take turns going first (free load-spreading across duplicate keys).
// The chain head has its own Rank and always stays at the front.
func rotateGroups(in []*Member, n uint64) []*Member {
	out := make([]*Member, 0, len(in))
	for i := 0; i < len(in); {
		j := i + 1
		for j < len(in) && in[j].Rank == in[i].Rank {
			j++
		}
		g := in[i:j]
		if len(g) > 1 {
			off := int(n % uint64(len(g)))
			for k := range g {
				out = append(out, g[(k+off)%len(g)])
			}
		} else {
			out = append(out, g...)
		}
		i = j
	}
	return out
}

// statusRe pulls the HTTP status out of the SDK's error text, which is formatted
// as "<prefix>: status <code>: <body>" (norma/llm/retry.go). The SDK exposes no
// typed error, so the string is what we have.
var statusRe = regexp.MustCompile(`status (\d{3})`)

// statusOf returns the HTTP status carried by err, or 0 if it isn't one.
func statusOf(err error) int {
	m := statusRe.FindStringSubmatch(err.Error())
	if m == nil {
		return 0
	}
	code, _ := strconv.Atoi(m[1])
	return code
}

// lengthLimitRe matches the "this request is too long for me" family of 400s.
//
// These are NOT identical everywhere: context windows differ per channel, so a
// prompt that overflows one member's window is routinely accepted by the next.
// Without this such a request died on the chain head — Pool.order only skips a
// member when it KNOWS the window (WindowTokens > 0), and these profiles carry
// context_window_k = 0 (= unlimited, see order), so the pool had no way to route
// around an over-long prompt. Matching the message recovers that routing without
// hard-coding per-channel window sizes we cannot verify.
//
// Kept deliberately narrow so a genuinely malformed request (bad JSON, bad role,
// unknown parameter) still fails fast on the first member instead of burning the
// whole chain.
var lengthLimitRe = regexp.MustCompile(`(?i)(` +
	`context[ _-]?length|context[ _-]?window|maximum context|` +
	`max[ _-]?(output[ _-]?)?tokens|maximum number of tokens|too many tokens|token limit|length limit|` +
	`prompt is too long|input is too long|request is too long|reduce the length|` +
	`exceeds? the (maximum )?(context|token|length)|` +
	`上下文|长度超|超出.{0,8}(长度|上限|限制)|过长` +
	`)`)

// shouldFailover reports whether err justifies trying the next profile.
//
// Never fails over on context cancellation — that's the user stopping a task or a
// task-level timeout, and burning a backup key on it would both waste credit and
// pollute the run's termination diagnosis.
//
// A plain 400 does not fail over: a malformed request fails identically
// everywhere. The one exception is a length-related 400 — per-channel context
// windows differ, so the next member may well accept the prompt (lengthLimitRe).
func shouldFailover(ctx context.Context, err error) bool {
	if err == nil {
		return false
	}
	if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	switch code := statusOf(err); {
	case code == 0:
		return true // no status → transport-level failure (reset / DNS / timeout)
	case code == 400:
		return lengthLimitRe.MatchString(err.Error()) // over-long prompt: next member may fit it
	case code == 401, code == 402, code == 403, code == 404, code == 408, code == 429:
		return true
	case code >= 500:
		return true
	}
	return false
}

// isHardFailure reports whether the failure is deterministic (the profile will
// keep failing until a human fixes it) rather than transient. Hard failures open
// the breaker on the first occurrence.
func isHardFailure(err error) bool {
	switch statusOf(err) {
	case 401, 402, 403, 404:
		return true
	}
	return false
}

// trimErr shortens an error for logs/UI — provider bodies can be long.
func trimErr(err error) string {
	s := strings.TrimSpace(strings.ReplaceAll(err.Error(), "\n", " "))
	if len(s) > 300 {
		s = s[:300] + "…"
	}
	return s
}

// estimateTokens roughly sizes a request so members whose context window clearly
// can't hold it are skipped. Deliberately crude (~3.5 chars/token) and biased to
// over-estimate slightly; it only needs to tell "fits" from "nowhere near".
func estimateTokens(req llm.CompletionRequest) int {
	n := 0
	for _, s := range req.System {
		n += len(s)
	}
	for _, m := range req.Messages {
		n += blocksLen(m.Content)
	}
	for _, t := range req.Tools {
		// InputSchema is a decoded map, so its serialized size isn't available
		// cheaply — charge a flat ~120 chars per top-level property instead.
		n += len(t.Name) + len(t.Description) + len(t.InputSchema)*120
	}
	return n * 2 / 7 // ≈ len/3.5
}

func blocksLen(bs []llm.ContentBlock) int {
	n := 0
	for _, b := range bs {
		n += len(b.Text) + len(b.Thinking) + len(b.Input) + len(b.Name)
		if len(b.Content) > 0 {
			n += blocksLen(b.Content)
		}
	}
	return n
}
