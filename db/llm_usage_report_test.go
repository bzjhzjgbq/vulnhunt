package db

import (
	"context"
	"testing"
	"time"
)

func TestLLMUsageReportAggregates(t *testing.T) {
	d, err := Open(testDSN(t))
	if err != nil {
		t.Skipf("postgres unavailable (%v)", err)
	}
	defer d.Close()
	if err := d.EnsureLLMUsageTable(); err != nil {
		t.Fatal(err)
	}

	model := "report-test-" + time.Now().UTC().Format("20060102150405.000000000")
	t.Cleanup(func() { _, _ = d.Exec(`DELETE FROM llm_usage WHERE model=$1`, model) })
	if _, err := d.Exec(`
INSERT INTO llm_usage(ts,task_id,worker,model,profile_name,latency_ms,input_tokens,output_tokens,cache_read,cache_write,status)
VALUES (now(),'report-task','worker',$1,'report-profile',100,10,5,2,1,'ok'),
       (now()-interval '1 day','report-task','worker',$1,'report-profile',300,20,7,0,0,'error')`, model); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	rep, err := d.LLMUsageReport(context.Background(), now.Add(-48*time.Hour), now.Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if rep.Summary.Calls < 2 || rep.Summary.InputTokens < 30 || rep.Summary.Errors < 1 {
		t.Fatalf("summary=%+v", rep.Summary)
	}
	var found *LLMUsageBucket
	for i := range rep.ByModel {
		if rep.ByModel[i].Key == model {
			found = &rep.ByModel[i]
		}
	}
	if found == nil || found.Calls != 2 || found.InputTokens != 30 || found.OutputTokens != 12 || found.Errors != 1 {
		t.Fatalf("by_model=%+v (looked for %s)", rep.ByModel, model)
	}
	if len(rep.Daily) < 2 {
		t.Fatalf("daily=%+v", rep.Daily)
	}
}
