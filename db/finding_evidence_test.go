package db

import (
	"context"
	"errors"
	"testing"
)

func TestFindingEvidenceCRUD(t *testing.T) {
	d, err := Open(testDSN(t))
	if err != nil {
		t.Skipf("postgres unavailable (%v)", err)
	}
	defer d.Close()

	id, err := d.AddFinding(0, 0, "未授权文件上传", "evidence test", SeverityMedium, "summary", "evidence", "tester", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = d.DeleteFinding(id) }()

	ctx := context.Background()
	ev, err := d.AddFindingEvidence(ctx, id, "command", "upload probe", "HTTP/1.1 200 OK", "匿名上传回读", "tester")
	if err != nil {
		t.Fatal(err)
	}
	if ev.ID <= 0 || ev.ContentHash == "" || len(ev.ContentHash) != 64 {
		t.Fatalf("evidence=%+v", ev)
	}
	items, err := d.ListFindingEvidence(ctx, id)
	if err != nil || len(items) != 1 || items[0].ContentHash != ev.ContentHash {
		t.Fatalf("items=%+v err=%v", items, err)
	}
	if err := d.DeleteFindingEvidence(ctx, id, ev.ID); err != nil {
		t.Fatal(err)
	}
	if err := d.DeleteFindingEvidence(ctx, id, ev.ID); !errors.Is(err, ErrFindingEvidenceNotFound) {
		t.Fatalf("second delete err=%v, want not found", err)
	}
	if items, err = d.ListFindingEvidence(ctx, id); err != nil || len(items) != 0 {
		t.Fatalf("after delete items=%+v err=%v", items, err)
	}
}
