package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestTaskDigestReportsGoalsIntentsAndLiveness(t *testing.T) {
	m, err := NewManager(t.TempDir(), "")
	if err != nil {
		t.Skipf("postgres unavailable (%v) - skipping", err)
	}
	defer m.Close()

	task, err := m.CreateTask("digest task", "goal", nil, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	taskID := mustTaskID(t, task.ID)
	defer m.pg.DeleteTask(taskID) //nolint:errcheck

	if _, err := task.Store.AddGoal(map[string]any{"text": "find one real bug"}, "human"); err != nil {
		t.Fatal(err)
	}
	openID, err := task.Store.AddIntent(map[string]any{"summary": "queued direction"}, 5, nil, "planner")
	if err != nil {
		t.Fatal(err)
	}
	runningID, err := task.Store.AddIntent(map[string]any{"summary": "running direction"}, 7, nil, "planner")
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := task.Store.ClaimIntent(runningID, "work#1"); err != nil || !ok {
		t.Fatalf("claim: ok=%v err=%v", ok, err)
	}

	// Call the handler directly: building a full Server would resume every active
	// task from the shared database, which a read-only digest test must not do.
	s := &Server{m: m, engine: NewEngine(m)}
	req := httptest.NewRequest(http.MethodGet, "/api/tasks/"+task.ID+"/digest", nil)
	req.SetPathValue("id", task.ID)
	rec := httptest.NewRecorder()
	s.taskDigest(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var dto taskDigestDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if dto.TaskID != task.ID || dto.GoalOpen != 1 || dto.GoalMet != 0 {
		t.Fatalf("task/goals wrong: id=%q open=%d met=%d", dto.TaskID, dto.GoalOpen, dto.GoalMet)
	}
	if dto.IntentCounts["running"] != 1 || dto.IntentCounts["open"] != 1 || dto.Queued != 1 {
		t.Fatalf("intent counts wrong: %+v queued=%d", dto.IntentCounts, dto.Queued)
	}
	if len(dto.Running) != 1 || dto.Running[0].ID != strconv.FormatInt(runningID, 10) {
		t.Fatalf("running = %+v, want only #%d", dto.Running, runningID)
	}
	// No engine work was registered for this claim: the digest must say so, which is
	// exactly what the reaper uses to decide a running row is orphaned.
	if dto.Running[0].HasLiveWork {
		t.Fatalf("has_live_work = true without registerWork")
	}
	if len(dto.Blocked) != 0 || len(dto.Findings) != 0 {
		t.Fatalf("unexpected blocked/findings: %+v %+v", dto.Blocked, dto.Findings)
	}
	_ = openID
}

func TestTaskDigestUnknownTaskIs404(t *testing.T) {
	m, err := NewManager(t.TempDir(), "")
	if err != nil {
		t.Skipf("postgres unavailable (%v) - skipping", err)
	}
	defer m.Close()

	s := &Server{m: m, engine: NewEngine(m)}
	req := httptest.NewRequest(http.MethodGet, "/api/tasks/999999/digest", nil)
	req.SetPathValue("id", "999999")
	rec := httptest.NewRecorder()
	s.taskDigest(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}
