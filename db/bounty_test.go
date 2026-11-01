package db

import (
	"context"
	"testing"
)

func TestBountyLedgerCRUD(t *testing.T) {
	d, err := Open(testDSN(t))
	if err != nil {
		t.Skipf("postgres unavailable (%v) - skipping", err)
	}
	defer d.Close()
	ctx := context.Background()

	var findingID int64
	if err := d.QueryRow(`INSERT INTO findings (vulnclass, name, severity, summary)
VALUES ('test', 'ledger fixture', 'high', 'fixture') RETURNING id`).Scan(&findingID); err != nil {
		t.Fatalf("insert finding: %v", err)
	}
	defer d.Exec(`DELETE FROM findings WHERE id=$1`, findingID) //nolint:errcheck

	amount := int64(150000) // ¥1500
	row, err := d.CreateBountySubmission(ctx, BountySubmissionInput{
		FindingID: findingID, Platform: "漏洞盒子", Status: "submitted",
		Severity: "high", AmountCents: amount, Currency: "CNY", Note: "first submit",
	})
	if err != nil {
		t.Fatal(err)
	}
	if row.ID == 0 || row.FindingName != "ledger fixture" || row.AmountCents != amount {
		t.Fatalf("created row = %+v", row)
	}
	if row.Status != "submitted" || row.Currency != "CNY" {
		t.Fatalf("defaults wrong: %+v", row)
	}

	row, err = d.UpdateBountySubmission(ctx, row.ID, BountySubmissionInput{
		FindingID: findingID, Platform: "漏洞盒子", Status: "paid",
		Severity: "high", AmountCents: 200000, Currency: "CNY", Note: "paid",
	})
	if err != nil {
		t.Fatal(err)
	}
	if row == nil || row.Status != "paid" || row.AmountCents != 200000 {
		t.Fatalf("updated row = %+v", row)
	}

	list, err := d.ListBountySubmissions(ctx, BountyFilter{FindingID: findingID})
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %d rows, err=%v", len(list), err)
	}
	if list[0].Status != "paid" {
		t.Fatalf("list row status = %q", list[0].Status)
	}

	sum, err := d.BountySpend(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Total < 1 || sum.PaidAmountCents < 200000 {
		t.Fatalf("summary = %+v, want at least our paid row", sum)
	}

	if _, err := d.CreateBountySubmission(ctx, BountySubmissionInput{
		FindingID: findingID, Status: "bogus",
	}); err == nil {
		t.Fatal("invalid status should be rejected")
	}

	ok, err := d.DeleteBountySubmission(ctx, row.ID)
	if err != nil || !ok {
		t.Fatalf("delete: ok=%v err=%v", ok, err)
	}
	if got, _ := d.GetBountySubmission(ctx, row.ID); got != nil {
		t.Fatalf("row still present after delete: %+v", got)
	}
}
