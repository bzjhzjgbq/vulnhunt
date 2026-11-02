package db

import (
	"context"
	"errors"
	"strings"
	"testing"
)

const testRevisionReport = `# 测试漏洞

- 漏洞等级：**高危**

## 一句话结论

未授权接口可读取数据。

## 成立前提

匿名可访问。

## 关键动作

curl 请求返回 200。

## 影响范围

测试范围。
`

func TestFindingContentRevisions(t *testing.T) {
	d, err := Open(testDSN(t))
	if err != nil {
		t.Skipf("postgres unavailable (%v)", err)
	}
	defer d.Close()

	id, err := d.AddFinding(0, 0, "XSS", "revision test", SeverityHigh, "summary", "evidence", "tester", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = d.DeleteFinding(id) }()

	ctx := context.Background()
	rev1, err := d.SaveFindingContent(ctx, id, 0, testRevisionReport, "ev1", "sum1", "initial", "tester")
	if err != nil {
		t.Fatal(err)
	}
	if rev1.Version != 1 || rev1.Report != testRevisionReport {
		t.Fatalf("rev1=%+v", rev1)
	}
	rev2, err := d.SaveFindingContent(ctx, id, 1, testRevisionReport+"\n后续修改", "ev2", "sum2", "edit", "tester")
	if err != nil {
		t.Fatal(err)
	}
	if rev2.Version != 2 {
		t.Fatalf("rev2 version=%d", rev2.Version)
	}
	if _, err := d.SaveFindingContent(ctx, id, 1, testRevisionReport, "stale", "stale", "stale", "tester"); !errors.Is(err, ErrFindingContentConflict) {
		t.Fatalf("stale save err=%v, want conflict", err)
	}

	revs, err := d.ListFindingRevisions(ctx, id)
	if err != nil || len(revs) != 2 || revs[0].Version != 2 || revs[1].Version != 1 {
		t.Fatalf("revisions=%+v err=%v", revs, err)
	}
	got, err := d.GetFindingRevision(ctx, id, 1)
	if err != nil || got == nil || !strings.Contains(got.Report, "测试漏洞") || got.Evidence != "ev1" {
		t.Fatalf("revision 1=%+v err=%v", got, err)
	}
	f, err := d.GetFinding(id)
	if err != nil || f == nil || f.ContentVersion != 2 || f.Evidence != "ev2" || f.Summary != "sum2" || !strings.Contains(f.Report, "后续修改") {
		t.Fatalf("finding=%+v err=%v", f, err)
	}
}
