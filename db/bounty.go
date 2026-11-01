package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Bounty ledger (schema.sql §Bounty): platform submissions and their money,
// tracked separately from the finding's internal triage status.

// BountySubmission is one row plus the joined finding/program labels.
type BountySubmission struct {
	ID             int64      `json:"id"`
	FindingID      int64      `json:"finding_id"`
	FindingName    string     `json:"finding_name"`
	FindingSeverity string    `json:"finding_severity"`
	TaskID         *int64     `json:"task_id,omitempty"`
	ProgramID      *int64     `json:"program_id,omitempty"`
	ProgramName    string     `json:"program_name,omitempty"`
	Platform       string     `json:"platform"`
	ExternalID     string     `json:"external_id"`
	Status         string     `json:"status"`
	Severity       string     `json:"severity"`
	AmountCents    int64      `json:"amount_cents"`
	Currency       string     `json:"currency"`
	SubmittedAt    *time.Time `json:"submitted_at,omitempty"`
	DecidedAt      *time.Time `json:"decided_at,omitempty"`
	PaidAt         *time.Time `json:"paid_at,omitempty"`
	Note           string     `json:"note"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// BountySubmissionInput is the writable shape (create/update).
type BountySubmissionInput struct {
	FindingID  int64      `json:"finding_id"`
	ProgramID  *int64     `json:"program_id"`
	Platform   string     `json:"platform"`
	ExternalID string     `json:"external_id"`
	Status     string     `json:"status"`
	Severity   string     `json:"severity"`
	AmountCents int64     `json:"amount_cents"`
	Currency   string     `json:"currency"`
	SubmittedAt *time.Time `json:"submitted_at"`
	DecidedAt   *time.Time `json:"decided_at"`
	PaidAt      *time.Time `json:"paid_at"`
	Note       string     `json:"note"`
}

// BountySummary aggregates the ledger for the dashboard.
type BountySummary struct {
	Total          int            `json:"total"`
	ByStatus       map[string]int `json:"by_status"`
	PendingAmountCents int64      `json:"pending_amount_cents"`
	AcceptedAmountCents int64     `json:"accepted_amount_cents"`
	PaidAmountCents int64         `json:"paid_amount_cents"`
	ByProgram      []BountyProgramStat `json:"by_program"`
}

// BountyProgramStat is one program's contribution.
type BountyProgramStat struct {
	ProgramID   *int64 `json:"program_id,omitempty"`
	ProgramName string `json:"program_name"`
	Total       int    `json:"total"`
	PaidCents   int64  `json:"paid_cents"`
	AcceptedCents int64 `json:"accepted_cents"`
}

var bountyStatuses = map[string]bool{
	"submitted": true, "accepted": true, "rejected": true,
	"duplicate": true, "paid": true, "withdrawn": true,
}

// ValidBountyStatus reports whether s is a known ledger state.
func ValidBountyStatus(s string) bool { return bountyStatuses[s] }

func normalizeBountyInput(in *BountySubmissionInput) error {
	in.Status = strings.TrimSpace(in.Status)
	if in.Status == "" {
		in.Status = "submitted"
	}
	if !ValidBountyStatus(in.Status) {
		return fmt.Errorf("未知的提交状态：%s", in.Status)
	}
	if in.AmountCents < 0 {
		return fmt.Errorf("赏金金额不能为负")
	}
	if strings.TrimSpace(in.Currency) == "" {
		in.Currency = "CNY"
	}
	in.Platform = strings.TrimSpace(in.Platform)
	in.ExternalID = strings.TrimSpace(in.ExternalID)
	in.Severity = strings.TrimSpace(in.Severity)
	in.Note = strings.TrimSpace(in.Note)
	if in.FindingID <= 0 {
		return fmt.Errorf("finding_id 必填")
	}
	return nil
}

const bountyCols = `b.id, b.finding_id, COALESCE(f.name,''), COALESCE(f.severity,''), f.task_id,
 b.program_id, COALESCE(p.name,''), b.platform, b.external_id, b.status, b.severity,
 b.amount_cents, b.currency, b.submitted_at, b.decided_at, b.paid_at, b.note,
 b.created_at, b.updated_at`

func scanBountyRows(rows *sql.Rows) ([]BountySubmission, error) {
	var out []BountySubmission
	for rows.Next() {
		var b BountySubmission
		var taskID, programID sql.NullInt64
		var submitted, decided, paid sql.NullTime
		if err := rows.Scan(&b.ID, &b.FindingID, &b.FindingName, &b.FindingSeverity, &taskID,
			&programID, &b.ProgramName, &b.Platform, &b.ExternalID, &b.Status, &b.Severity,
			&b.AmountCents, &b.Currency, &submitted, &decided, &paid, &b.Note,
			&b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		if taskID.Valid {
			v := taskID.Int64
			b.TaskID = &v
		}
		if programID.Valid {
			v := programID.Int64
			b.ProgramID = &v
		}
		if submitted.Valid {
			v := submitted.Time
			b.SubmittedAt = &v
		}
		if decided.Valid {
			v := decided.Time
			b.DecidedAt = &v
		}
		if paid.Valid {
			v := paid.Time
			b.PaidAt = &v
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// CreateBountySubmission validates and inserts one ledger row.
func (d *DB) CreateBountySubmission(ctx context.Context, in BountySubmissionInput) (*BountySubmission, error) {
	if err := normalizeBountyInput(&in); err != nil {
		return nil, err
	}
	var id int64
	err := d.QueryRowContext(ctx, `INSERT INTO bounty_submissions
 (finding_id, program_id, platform, external_id, status, severity, amount_cents, currency,
  submitted_at, decided_at, paid_at, note)
 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
 RETURNING id`, in.FindingID, in.ProgramID, in.Platform, in.ExternalID, in.Status, in.Severity,
		in.AmountCents, in.Currency, in.SubmittedAt, in.DecidedAt, in.PaidAt, in.Note).Scan(&id)
	if err != nil {
		if strings.Contains(err.Error(), "foreign key") {
			return nil, fmt.Errorf("finding #%d 不存在", in.FindingID)
		}
		return nil, err
	}
	return d.GetBountySubmission(ctx, id)
}

// UpdateBountySubmission replaces the writable fields of one row.
func (d *DB) UpdateBountySubmission(ctx context.Context, id int64, in BountySubmissionInput) (*BountySubmission, error) {
	if err := normalizeBountyInput(&in); err != nil {
		return nil, err
	}
	res, err := d.ExecContext(ctx, `UPDATE bounty_submissions SET
 finding_id=$2, program_id=$3, platform=$4, external_id=$5, status=$6, severity=$7,
 amount_cents=$8, currency=$9, submitted_at=$10, decided_at=$11, paid_at=$12, note=$13,
 updated_at=now()
 WHERE id=$1`, id, in.FindingID, in.ProgramID, in.Platform, in.ExternalID, in.Status,
		in.Severity, in.AmountCents, in.Currency, in.SubmittedAt, in.DecidedAt, in.PaidAt, in.Note)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, nil
	}
	return d.GetBountySubmission(ctx, id)
}

// DeleteBountySubmission removes one ledger row.
func (d *DB) DeleteBountySubmission(ctx context.Context, id int64) (bool, error) {
	res, err := d.ExecContext(ctx, `DELETE FROM bounty_submissions WHERE id=$1`, id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// GetBountySubmission returns one row with labels.
func (d *DB) GetBountySubmission(ctx context.Context, id int64) (*BountySubmission, error) {
	rows, err := d.QueryContext(ctx, `SELECT `+bountyCols+` FROM bounty_submissions b
LEFT JOIN findings f ON f.id=b.finding_id
LEFT JOIN src_programs p ON p.id=b.program_id
WHERE b.id=$1`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items, err := scanBountyRows(rows)
	if err != nil || len(items) == 0 {
		return nil, err
	}
	return &items[0], nil
}

// BountyFilter narrows the ledger list.
type BountyFilter struct {
	FindingID int64
	ProgramID int64
	Status    string
}

// ListBountySubmissions returns ledger rows, newest first.
func (d *DB) ListBountySubmissions(ctx context.Context, f BountyFilter) ([]BountySubmission, error) {
	var args []any
	where := []string{"1=1"}
	if f.FindingID > 0 {
		args = append(args, f.FindingID)
		where = append(where, fmt.Sprintf("b.finding_id=$%d", len(args)))
	}
	if f.ProgramID > 0 {
		args = append(args, f.ProgramID)
		where = append(where, fmt.Sprintf("b.program_id=$%d", len(args)))
	}
	if s := strings.TrimSpace(f.Status); s != "" {
		args = append(args, s)
		where = append(where, fmt.Sprintf("b.status=$%d", len(args)))
	}
	rows, err := d.QueryContext(ctx, `SELECT `+bountyCols+` FROM bounty_submissions b
LEFT JOIN findings f ON f.id=b.finding_id
LEFT JOIN src_programs p ON p.id=b.program_id
WHERE `+strings.Join(where, " AND ")+`
ORDER BY b.id DESC LIMIT 500`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanBountyRows(rows)
}

// BountySpend returns the aggregate view for the dashboard.
func (d *DB) BountySpend(ctx context.Context) (*BountySummary, error) {
	out := &BountySummary{ByStatus: map[string]int{}}
	rows, err := d.QueryContext(ctx, `SELECT status, count(*), COALESCE(sum(amount_cents),0)
FROM bounty_submissions GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var count int
		var amount int64
		if err := rows.Scan(&status, &count, &amount); err != nil {
			return nil, err
		}
		out.ByStatus[status] = count
		out.Total += count
		switch status {
		case "submitted", "accepted":
			out.PendingAmountCents += amount
			if status == "accepted" {
				out.AcceptedAmountCents += amount
			}
		case "paid":
			out.PaidAmountCents += amount
			out.AcceptedAmountCents += amount
		}
	}
	progRows, err := d.QueryContext(ctx, `SELECT b.program_id, COALESCE(p.name,'未关联平台'),
 count(*), COALESCE(sum(CASE WHEN b.status='paid' THEN b.amount_cents ELSE 0 END),0),
 COALESCE(sum(CASE WHEN b.status IN ('accepted','paid') THEN b.amount_cents ELSE 0 END),0)
FROM bounty_submissions b LEFT JOIN src_programs p ON p.id=b.program_id
GROUP BY b.program_id, p.name ORDER BY 4 DESC, 3 DESC LIMIT 50`)
	if err != nil {
		return nil, err
	}
	defer progRows.Close()
	for progRows.Next() {
		var stat BountyProgramStat
		var pid sql.NullInt64
		if err := progRows.Scan(&pid, &stat.ProgramName, &stat.Total, &stat.PaidCents, &stat.AcceptedCents); err != nil {
			return nil, err
		}
		if pid.Valid {
			v := pid.Int64
			stat.ProgramID = &v
		}
		out.ByProgram = append(out.ByProgram, stat)
	}
	return out, progRows.Err()
}
