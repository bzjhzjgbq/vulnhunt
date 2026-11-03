package db

import (
	"context"
	"strings"
	"testing"
)

func TestFalsePositivePatterns(t *testing.T) {
	d, err := Open(testDSN(t))
	if err != nil {
		t.Skipf("postgres unavailable (%v)", err)
	}
	defer d.Close()

	task, err := d.CreateTask("pattern test", "goal", nil, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Exec(`DELETE FROM tasks WHERE id=$1`, task.ID)

	fp1, err := d.AddFinding(task.ID, 0, "信息泄露", "公开信息", "medium", "s", "e", "w", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer d.DeleteFinding(fp1)
	fp2, err := d.AddFinding(task.ID, 0, "信息泄露", "目录列表", "low", "s", "e", "w", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer d.DeleteFinding(fp2)
	other, err := d.AddFinding(task.ID, 0, "SSRF", "内网探测", "high", "s", "e", "w", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer d.DeleteFinding(other)

	for _, id := range []int64{fp1, fp2} {
		if _, err := d.SetFindingStatus(id, FindingFalsePositive); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := d.SetFindingTriageReason(fp1, "仅暴露公开信息"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetFindingStatus(other, FindingIgnored); err != nil {
		t.Fatal(err)
	}

	pats, err := d.Exploration(task.ExplorationID).FalsePositivePatterns(context.Background(), "信息泄露", nil, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(pats) != 1 || pats[0].VulnClass != "信息泄露" || pats[0].Count != 2 || !strings.Contains(pats[0].Reasons, "仅暴露公开信息") {
		t.Fatalf("patterns=%+v", pats)
	}

	all, err := d.Exploration(task.ExplorationID).FalsePositivePatterns(context.Background(), "", nil, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("unfiltered patterns=%+v", all)
	}
}
