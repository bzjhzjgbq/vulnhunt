package server

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"vulnhunt/agent"
	"vulnhunt/db"
)

// Intent watchdog: reclaim work the engine can no longer account for.
//
// Two failure shapes leave an intent stuck in 'running' and keep the task looking
// active forever:
//
//   - orphaned: the DB says running but this process holds no live work handle
//     for it (the settling goroutine returned without a state write, a CAS lost a
//     race, or a run was abandoned between claim and registerWork);
//   - hung: the work handle still exists but the intent has produced no activity
//     for a long time (an upstream that accepts the request and never streams,
//     i.e. exactly the stall the 60s stream idle-timeout cannot reach because the
//     read itself never returns).
//
// Startup already resets every running intent (ResetRunningIntents); this is the
// runtime equivalent, so a long-lived process does not accumulate zombies.

const (
	// orphanGrace is how long an intent may sit "running with no live work" before
	// the watchdog acts. ClaimIntent (DB) and registerWork (memory) are adjacent,
	// so the only reason to wait is to absorb scheduling jitter, not correctness.
	orphanGrace = 3 * time.Minute
	// hungWorkTimeout bounds total silence from a live work run. Normal steps emit
	// activity (tool calls, results, usage); a worker that has written nothing for
	// this long is not going to finish on its own.
	hungWorkTimeout = 20 * time.Minute
	// reaperInterval is the watchdog scan period. Cheap: one indexed SELECT plus a
	// few map lookups per task.
	reaperInterval = 60 * time.Second
)

// orphanTracker is the pure decision state for the "running without live work"
// half of the watchdog. Kept separate from the engine so it can be tested without
// a database or goroutines: feed it the running ids and a hasWork predicate.
type orphanTracker struct {
	mu    sync.Mutex
	grace time.Duration
	seen  map[int64]time.Time // intent -> first time observed orphaned
	hits  map[int64]int       // intent -> reap count (2nd reap = stop, not requeue)
}

func newOrphanTracker(grace time.Duration) *orphanTracker {
	return &orphanTracker{grace: grace, seen: map[int64]time.Time{}, hits: map[int64]int{}}
}

// decide classifies the currently running intents. An intent whose worker is gone
// must stay orphaned for grace to be actioned: the first reap requeues it (open),
// the second stops it (stopped) so a poison intent cannot loop forever.
func (o *orphanTracker) decide(running []int64, hasWork func(int64) bool, now time.Time) (reopen, stop []int64) {
	o.mu.Lock()
	defer o.mu.Unlock()
	grace := o.grace
	if grace <= 0 {
		grace = orphanGrace
	}
	for _, id := range running {
		if hasWork(id) {
			delete(o.seen, id)
			continue
		}
		first, ok := o.seen[id]
		if !ok {
			o.seen[id] = now
			continue
		}
		if now.Sub(first) < grace {
			continue
		}
		delete(o.seen, id)
		o.hits[id]++
		if o.hits[id] >= 2 {
			stop = append(stop, id)
			continue
		}
		reopen = append(reopen, id)
	}
	return reopen, stop
}

func (e *Engine) hasLiveWork(intentID int64) bool {
	e.workMu.Lock()
	defer e.workMu.Unlock()
	_, ok := e.work[intentID]
	return ok
}

// intentLastActivity returns the newest activity timestamp recorded for an intent.
func (e *Engine) intentLastActivity(intentID int64) (time.Time, bool) {
	v, ok := e.intentAct.Load(intentID)
	if !ok {
		return time.Time{}, false
	}
	unix, _ := v.(int64)
	if unix <= 0 {
		return time.Time{}, false
	}
	return time.Unix(unix, 0), true
}

// cancelAbandonedWork cancels a live but silent work context. The run settles as
// stopped through the normal killed branch, so partial graph writes stay.
func (e *Engine) cancelAbandonedWork(intentID int64) bool {
	e.workMu.Lock()
	defer e.workMu.Unlock()
	run := e.work[intentID]
	if run == nil {
		return false
	}
	run.cancel(agent.AbortWorkStalled)
	return true
}

// intentReaperLoop scans one task's running intents periodically.
func (e *Engine) intentReaperLoop(ctx context.Context, t *Task) {
	tick := time.NewTicker(reaperInterval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			if e.IsPaused(t.ID) || e.IsDeleting(t.ID) || e.isSettling(t.ID) ||
				isTerminalStatus(e.m.TaskStatus(t.ID)) {
				continue
			}
			e.reapIntentWatchdog(t)
		}
	}
}

// reapIntentWatchdog performs one scan: first cancel hung live work, then reclaim
// orphans the engine can no longer account for.
func (e *Engine) reapIntentWatchdog(t *Task) {
	rows, err := t.Store.ListRunningIntents()
	if err != nil {
		log.Printf("[reaper] task %s 查询 running 意图失败: %v", t.ID, err)
		return
	}
	ids := make([]int64, 0, len(rows))
	for _, n := range rows {
		ids = append(ids, n.ID)
	}
	now := time.Now()
	for _, id := range ids {
		if !e.hasLiveWork(id) {
			continue
		}
		last, ok := e.intentLastActivity(id)
		if !ok {
			e.intentAct.Store(id, now.Unix()) // first sight: start the clock
			continue
		}
		if now.Sub(last) < hungWorkTimeout {
			continue
		}
		if !e.cancelAbandonedWork(id) {
			continue
		}
		log.Printf("[reaper] task %s 意图 #%d 超过 %s 无活动，取消本次运行", t.ID, id, hungWorkTimeout)
		e.trackIntentActivity(id, now)
		e.emitActivity(t, db.Activity{Worker: "system", Kind: "text",
			Summary: fmt.Sprintf("意图 #%d 超过 %s 没有任何活动，已取消本次运行并回收（已完成的事实与资产保留）", id, hungWorkTimeout)})
		e.touch(t.ID)
	}
	reopen, stop := e.orphans.decide(ids, e.hasLiveWork, now)
	for _, id := range reopen {
		changed, err := t.Store.CompareAndSetIntentState(id, "running", "open")
		if err != nil {
			log.Printf("[reaper] task %s 意图 #%d 重新入队失败: %v", t.ID, id, err)
			continue
		}
		if !changed {
			continue
		}
		log.Printf("[reaper] task %s 意图 #%d 已无活动 worker，重新入队", t.ID, id)
		e.emitActivity(t, db.Activity{Worker: "system", Kind: "text",
			Summary: fmt.Sprintf("意图 #%d 的 worker 已不存在（无活动句柄），已重新入队待领取", id)})
		e.touch(t.ID)
		t.Notify()
	}
	for _, id := range stop {
		changed, err := t.Store.CompareAndSetIntentState(id, "running", "stopped")
		if err != nil {
			log.Printf("[reaper] task %s 意图 #%d 停止失败: %v", t.ID, id, err)
			continue
		}
		if !changed {
			continue
		}
		log.Printf("[reaper] task %s 意图 #%d 重跑后再次失去 worker，标记 stopped", t.ID, id)
		e.emitActivity(t, db.Activity{Worker: "system", Kind: "text",
			Summary: fmt.Sprintf("意图 #%d 重新入队后再次卡住，已标记 stopped（可在会话列表手动「重跑」）", id)})
		e.touch(t.ID)
		t.Notify()
	}
}

// trackIntentActivity records per-intent liveness for the reaper. Called from the
// activity path for every record carrying an intent id, and when a run starts.
func (e *Engine) trackIntentActivity(intentID int64, at time.Time) {
	e.intentAct.Store(intentID, at.Unix())
}
