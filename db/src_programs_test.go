package db

import (
	"context"
	"testing"
)

func TestSyncAndListSRCPrograms(t *testing.T) {
	d, err := Open(testDSN(t))
	if err != nil {
		t.Skipf("postgres unavailable (%v) - skipping", err)
	}
	defer d.Close()
	ctx := context.Background()
	const sourceID = 990001
	defer d.Exec(`DELETE FROM src_programs WHERE source_id=$1`, sourceID) //nolint:errcheck

	regions := []SRCRegionInput{{
		Name: "国内",
		Programs: []SRCProgramInput{{
			SourceID: sourceID, Name: "测试赏金平台", TargetURL: "https://src.example.test/",
			Host: "src.example.test", Status: "normal", StatusLabel: "正常运营",
			Heat: 42, Tags: []string{"互联网"}, TagIDs: []int64{1},
			AnalysisHasData: true, AnalysisLatestMonth: "2026-10",
			AnalysisLatestFetched: "2026-10-30T19:24:19Z",
		}},
	}}
	res, err := d.SyncSRCPrograms(ctx, regions)
	if err != nil {
		t.Fatal(err)
	}
	if res.Fetched != 1 || res.Inserted != 1 || res.Updated != 0 {
		t.Fatalf("first sync = %+v, want 1 fetched / 1 inserted", res)
	}
	// A second identical sync must update, not duplicate.
	res, err = d.SyncSRCPrograms(ctx, regions)
	if err != nil {
		t.Fatal(err)
	}
	if res.Inserted != 0 || res.Updated != 1 {
		t.Fatalf("second sync = %+v, want 1 updated", res)
	}

	list, err := d.ListSRCPrograms(ctx, SRCProgramFilter{Query: "测试赏金"})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("list = %d rows, want 1", len(list))
	}
	got := list[0]
	if got.Name != "测试赏金平台" || got.Region != "国内" || got.Heat != 42 {
		t.Fatalf("row = %+v", got)
	}
	if len(got.Tags) != 1 || got.Tags[0] != "互联网" {
		t.Fatalf("tags = %v", got.Tags)
	}
	if got.AnalysisLatestFetched == nil || got.AnalysisHasData == false {
		t.Fatalf("analysis fields lost: %+v", got)
	}
	if got.LinkedTaskID != nil {
		t.Fatalf("unexpected linked task: %v", *got.LinkedTaskID)
	}

	byID, err := d.GetSRCProgram(ctx, got.ID)
	if err != nil || byID == nil || byID.SourceID != sourceID {
		t.Fatalf("GetSRCProgram = %+v, %v", byID, err)
	}

	if err := d.SetTaskSRCProgram(ctx, 0, got.ID); err != nil {
		t.Fatalf("SetTaskSRCProgram on missing task should be a no-op update, got %v", err)
	}
}
