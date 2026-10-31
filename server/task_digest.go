package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/Autumn-27/artex/db"
)

// Task digest: a one-call answer to "what is this task doing and why is it stuck?".
// The web UI needs ~6 endpoints to assemble this today (goals, frontier, intents,
// findings, stats, activity), which is why diagnosing a slow task means paging
// through hundreds of intents. This endpoint collapses the state into a small,
// stable shape that both the UI and external operators can poll.

type digestIntent struct {
	ID          string `json:"id"`
	State       string `json:"state"`
	Priority    int    `json:"priority"`
	Summary     string `json:"summary"`
	TS          string `json:"ts,omitempty"`
	HasLiveWork bool   `json:"has_live_work"`
	SilentSecs  int64  `json:"silent_secs,omitempty"`
}

type digestFinding struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Severity string `json:"severity"`
	Status   string `json:"status"`
	TS       string `json:"ts,omitempty"`
}

type digestGoal struct {
	ID    string `json:"id"`
	Text  string `json:"text"`
	State string `json:"state"`
}

type taskDigestDTO struct {
	TaskID       string          `json:"task_id"`
	Name         string          `json:"name"`
	Status       string          `json:"status"`
	Paused       bool            `json:"paused"`
	InFlight     int64           `json:"in_flight"`
	EngineMode   string          `json:"engine_mode"`
	LastActivity int64           `json:"last_activity"`
	Goals        []digestGoal    `json:"goals"`
	GoalOpen     int             `json:"goal_open"`
	GoalMet      int             `json:"goal_met"`
	IntentCounts map[string]int  `json:"intent_counts"`
	Running      []digestIntent  `json:"running"`
	Blocked      []digestIntent  `json:"blocked_recent"`
	Findings     []digestFinding `json:"findings_recent"`
	Queued       int             `json:"queued"`
	GeneratedAt  string          `json:"generated_at"`
}

func intentSummary(n *db.Node) string {
	var p struct {
		Summary string `json:"summary"`
	}
	_ = json.Unmarshal(n.Payload, &p)
	return p.Summary
}

// taskDigest GET /api/tasks/{id}/digest
func (s *Server) taskDigest(w http.ResponseWriter, r *http.Request) {
	t := s.m.ResolveTask(r.PathValue("id"))
	if t == nil {
		writeErr(w, 404, "task not found")
		return
	}
	dto := taskDigestDTO{
		TaskID:       t.ID,
		Name:         t.Name,
		IntentCounts: map[string]int{},
		GeneratedAt:  time.Now().Format(time.RFC3339),
	}
	if s.engine != nil {
		dto.Paused = s.engine.IsPaused(t.ID)
		dto.InFlight = s.engine.inflightCount(t.ID)
		dto.LastActivity = s.engine.LastActivity(t.ID)
	}
	// lifecycleSnapshot can still say "created" while the engine is working (the
	// persisted column is only written on lifecycle transitions); derive the view
	// the operator expects.
	st := t.lifecycleSnapshot().Status
	switch {
	case dto.Paused:
		st = "paused"
	case isTerminalStatus(st):
		// done / failed / timeout — keep the persisted terminal state.
	case t.Queued:
		st = "queued"
	case s.engine != nil && s.engine.Started(t.ID):
		st = "running"
	}
	dto.Status = st

	// Goals: open first, then met, capped so a task with a long history stays small.
	if goals, err := t.Store.ListByKind(db.KindGoal, 100); err == nil {
		for _, g := range goals {
			var p struct {
				Text string `json:"text"`
			}
			_ = json.Unmarshal(g.Payload, &p)
			dto.Goals = append(dto.Goals, digestGoal{ID: strconv.FormatInt(g.ID, 10), Text: p.Text, State: g.State})
			switch g.State {
			case "open":
				dto.GoalOpen++
			case "met":
				dto.GoalMet++
			}
		}
	}

	if counts, err := t.Store.IntentStateCounts(); err == nil {
		dto.IntentCounts = counts
	}
	dto.Queued = dto.IntentCounts["open"]

	// Running intents with liveness: the watchdog's view, exposed.
	if running, err := t.Store.ListRunningIntents(); err == nil {
		now := time.Now()
		for _, n := range running {
			item := digestIntent{
				ID: strconv.FormatInt(n.ID, 10), State: n.State, Priority: n.Priority,
				Summary: intentSummary(n), TS: n.CreatedAt.Format(time.RFC3339),
			}
			if s.engine != nil {
				item.HasLiveWork = s.engine.hasLiveWork(n.ID)
				if last, ok := s.engine.intentLastActivity(n.ID); ok {
					item.SilentSecs = int64(now.Sub(last).Seconds())
				}
			}
			dto.Running = append(dto.Running, item)
		}
	}

	// Latest blocked intents: these are the "why we stopped" trail a human reads.
	if intents, err := t.Store.ListByKind(db.KindIntent, 200); err == nil {
		for _, n := range intents {
			if n.State != "blocked" || len(dto.Blocked) >= 10 {
				continue
			}
			dto.Blocked = append(dto.Blocked, digestIntent{
				ID: strconv.FormatInt(n.ID, 10), State: n.State, Priority: n.Priority,
				Summary: intentSummary(n), TS: n.CreatedAt.Format(time.RFC3339),
			})
		}
	}

	// Latest findings, from the task's own graph (the findings table is global).
	if findings, err := t.Store.ListByKind(db.KindFinding, 10); err == nil {
		for _, n := range findings {
			var p struct {
				Name     string `json:"name"`
				Severity string `json:"severity"`
				Status   string `json:"status"`
			}
			_ = json.Unmarshal(n.Payload, &p)
			dto.Findings = append(dto.Findings, digestFinding{
				ID: strconv.FormatInt(n.ID, 10), Name: p.Name, Severity: p.Severity, Status: p.Status,
				TS: n.CreatedAt.Format(time.RFC3339),
			})
		}
	}

	// engine_mode mirrors /api/stats so both views agree.
	switch {
	case dto.Paused:
		dto.EngineMode = "paused"
	case dto.InFlight > 0 && s.engine != nil && s.engine.ActiveLLMCalls(t.ID) == 0 &&
		dto.LastActivity > 0 && time.Now().Unix()-dto.LastActivity > 120:
		dto.EngineMode = "stalled"
	case s.engine != nil && s.engine.ActiveLLMCalls(t.ID) > 0:
		dto.EngineMode = "exploring"
	default:
		dto.EngineMode = "idle"
	}

	writeJSON(w, 200, dto)
}
