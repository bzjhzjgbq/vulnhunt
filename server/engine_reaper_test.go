package server

import (
	"testing"
	"time"
)

func TestOrphanTrackerRequeuesThenStops(t *testing.T) {
	tr := newOrphanTracker(3 * time.Minute)
	now := time.Unix(1_800_000_000, 0)
	running := []int64{11, 12}
	hasWork := func(id int64) bool { return id == 12 }

	// First sight only starts the clock, even for the orphan.
	if reopen, stop := tr.decide(running, hasWork, now); len(reopen)+len(stop) != 0 {
		t.Fatalf("first pass acted: reopen=%v stop=%v", reopen, stop)
	}
	// Before grace: still nothing.
	if reopen, stop := tr.decide(running, hasWork, now.Add(time.Minute)); len(reopen)+len(stop) != 0 {
		t.Fatalf("inside grace acted: reopen=%v stop=%v", reopen, stop)
	}
	// After grace: the orphan is requeued, the live one untouched.
	reopen, stop := tr.decide(running, hasWork, now.Add(4*time.Minute))
	if len(reopen) != 1 || reopen[0] != 11 || len(stop) != 0 {
		t.Fatalf("want reopen [11], got reopen=%v stop=%v", reopen, stop)
	}
	// It stalls again: second reap stops it instead of looping.
	tr.decide(running, hasWork, now.Add(5*time.Minute))
	reopen, stop = tr.decide(running, hasWork, now.Add(9*time.Minute))
	if len(reopen) != 0 || len(stop) != 1 || stop[0] != 11 {
		t.Fatalf("want stop [11], got reopen=%v stop=%v", reopen, stop)
	}
}

func TestOrphanTrackerForgetsLiveWork(t *testing.T) {
	tr := newOrphanTracker(time.Minute)
	now := time.Unix(1_800_000_000, 0)
	running := []int64{7}

	tr.decide(running, func(int64) bool { return false }, now)
	// A worker shows up late (registerWork landed after the first scan).
	tr.decide(running, func(int64) bool { return true }, now.Add(2*time.Minute))
	// Then it disappears again: it must get a fresh grace window, not be stopped
	// on the strength of the stale first sighting.
	reopen, stop := tr.decide(running, func(int64) bool { return false }, now.Add(2*time.Minute+30*time.Second))
	if len(reopen)+len(stop) != 0 {
		t.Fatalf("stale sighting acted: reopen=%v stop=%v", reopen, stop)
	}
	reopen, _ = tr.decide(running, func(int64) bool { return false }, now.Add(5*time.Minute))
	if len(reopen) != 1 || reopen[0] != 7 {
		t.Fatalf("want reopen [7] after fresh grace, got %v", reopen)
	}
}
