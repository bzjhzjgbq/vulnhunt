package db

import "testing"

func TestListRunningAndActiveIntents(t *testing.T) {
	d, err := Open(testDSN(t))
	if err != nil {
		t.Skipf("postgres unavailable (%v) - skipping", err)
	}
	defer d.Close()

	expID, err := d.CreateExploration("test", "intent list helpers")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Exec(`DELETE FROM explorations WHERE id=$1`, expID) //nolint:errcheck
	es := d.Exploration(expID)

	openID, err := es.AddIntent(map[string]any{"summary": "open direction"}, 5, nil, "planner")
	if err != nil {
		t.Fatal(err)
	}
	runningID, err := es.AddIntent(map[string]any{"summary": "running direction"}, 5, nil, "planner")
	if err != nil {
		t.Fatal(err)
	}
	doneID, err := es.AddIntent(map[string]any{"summary": "finished direction"}, 5, nil, "planner")
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := es.ClaimIntent(runningID, "work#1"); err != nil || !ok {
		t.Fatalf("claim: ok=%v err=%v", ok, err)
	}
	if err := es.SetIntentState(doneID, "done"); err != nil {
		t.Fatal(err)
	}

	running, err := es.ListRunningIntents()
	if err != nil {
		t.Fatal(err)
	}
	if len(running) != 1 || running[0].ID != runningID {
		t.Fatalf("running = %+v, want only #%d", running, runningID)
	}

	active, err := es.ListActiveIntents(10)
	if err != nil {
		t.Fatal(err)
	}
	got := map[int64]string{}
	for _, n := range active {
		got[n.ID] = n.State
	}
	if len(got) != 2 || got[openID] != "open" || got[runningID] != "running" {
		t.Fatalf("active = %v, want open #%d + running #%d (done excluded)", got, openID, runningID)
	}
	if _, ok := got[doneID]; ok {
		t.Fatalf("done intent #%d leaked into active list", doneID)
	}
}
