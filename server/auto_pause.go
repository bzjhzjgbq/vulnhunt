package server

import (
	"context"
	"database/sql"
	"log"
	"strconv"
	"strings"
	"time"

	"vulnhunt/db"
)

const settingAutoPauseHours = "auto_pause_stale_hours"
const defaultAutoPauseHours = 24
const autoPauseInterval = 30 * time.Minute

// autoPauseStaleHours returns the configured idle window in hours (0 = disabled).
func autoPauseStaleHours(pg *db.DB) int {
	v, ok, err := pg.GetSetting(settingAutoPauseHours)
	if err != nil || !ok {
		return defaultAutoPauseHours
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n < 0 || n > 720 {
		return defaultAutoPauseHours
	}
	return n
}

// autoPauseLoop stops tasks that keep burning tokens without producing findings.
// A task is only considered when it already has findings, so fresh reconnaissance
// runs are never killed before their first result.
func (s *Server) autoPauseLoop(ctx context.Context) {
	ticker := time.NewTicker(autoPauseInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			hours := autoPauseStaleHours(s.m.pg)
			if hours <= 0 {
				continue
			}
			s.pauseStaleTasks(ctx, hours)
		}
	}
}

func (s *Server) pauseStaleTasks(ctx context.Context, hours int) {
	cutoff := time.Now().Add(-time.Duration(hours) * time.Hour)
	for _, task := range s.m.List() {
		if task == nil || task.Paused || task.Queued || task.CompletedAt != 0 {
			continue
		}
		tid, err := strconv.ParseInt(task.ID, 10, 64)
		if err != nil || tid <= 0 {
			continue
		}
		var last sql.NullTime
		if err := s.m.pg.QueryRowContext(ctx,
			`SELECT max(created_at) FROM findings WHERE task_id = $1`, tid).Scan(&last); err != nil {
			continue
		}
		reason := ""
		var lastAt time.Time
		if last.Valid && !last.Time.IsZero() {
			if last.Time.After(cutoff) {
				continue
			}
			reason = "no new finding"
			lastAt = last.Time
		} else {
			// A task that never produced a finding is the most expensive kind of
			// idle run: fall back to its creation time so it cannot burn forever.
			if task.CreatedAt <= 0 {
				continue
			}
			created := time.Unix(task.CreatedAt, 0)
			if created.After(cutoff) {
				continue
			}
			reason = "no finding since creation"
			lastAt = created
		}
		if err := s.m.SetTaskPaused(task.ID, true); err != nil {
			log.Printf("[auto-pause] task %s pause failed: %v", task.ID, err)
			continue
		}
		log.Printf("[auto-pause] task %s (%s) paused: %s for %dh (since %s)",
			task.ID, task.Name, reason, hours, lastAt.Format("2006-01-02 15:04"))
	}
}
