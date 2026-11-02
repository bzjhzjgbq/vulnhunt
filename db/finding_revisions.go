package db

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// ErrFindingContentConflict means the caller edited a stale copy of the
// finding's report/evidence/summary. Re-read the current content_version and
// retry instead of overwriting a newer edit.
var ErrFindingContentConflict = errors.New("finding content version conflict")

// FindingRevision is one saved snapshot of a finding's human-facing content.
type FindingRevision struct {
	ID        int64     `json:"id"`
	FindingID int64     `json:"finding_id"`
	Version   int64     `json:"version"`
	Report    string    `json:"report"`
	Evidence  string    `json:"evidence"`
	Summary   string    `json:"summary"`
	Reason    string    `json:"reason"`
	Author    string    `json:"author"`
	CreatedAt time.Time `json:"created_at"`
}

// SaveFindingContent writes report/evidence/summary and appends a revision.
// expectedVersion is the content_version the caller read; a mismatch returns
// ErrFindingContentConflict. Pass a negative value only for internal callers
// that intentionally want last-write-wins.
func (d *DB) SaveFindingContent(ctx context.Context, findingID, expectedVersion int64, report, evidence, summary, reason, author string) (*FindingRevision, error) {
	var rev FindingRevision
	err := d.WithEvidenceTx(ctx, func(tx *sql.Tx) error {
		var current int64
		if err := tx.QueryRowContext(ctx,
			`SELECT content_version FROM findings WHERE id=$1 FOR UPDATE`, findingID,
		).Scan(&current); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrFindingNotFound
			}
			return err
		}
		if expectedVersion >= 0 && current != expectedVersion {
			return ErrFindingContentConflict
		}
		next := current + 1
		if _, err := tx.ExecContext(ctx,
			`UPDATE findings SET report=$2, evidence=$3, summary=$4, content_version=$5 WHERE id=$1`,
			findingID, report, evidence, summary, next,
		); err != nil {
			return err
		}
		rev = FindingRevision{
			FindingID: findingID, Version: next, Report: report, Evidence: evidence,
			Summary: summary, Reason: reason, Author: author,
		}
		if err := tx.QueryRowContext(ctx,
			`INSERT INTO finding_report_versions(finding_id,version,report,evidence,summary,reason,author)
			 VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id, created_at`,
			findingID, next, report, evidence, summary, reason, author,
		).Scan(&rev.ID, &rev.CreatedAt); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &rev, nil
}

// insertFindingRevisionTx appends a version row inside an existing transaction.
// The caller is responsible for updating findings.content_version first.
func insertFindingRevisionTx(ctx context.Context, tx *sql.Tx, findingID, version int64, report, evidence, summary, reason, author string) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO finding_report_versions(finding_id,version,report,evidence,summary,reason,author)
		 VALUES($1,$2,$3,$4,$5,$6,$7)`,
		findingID, version, report, evidence, summary, reason, author,
	)
	return err
}

// ListFindingRevisions returns snapshots newest-first.
func (d *DB) ListFindingRevisions(ctx context.Context, findingID int64) ([]FindingRevision, error) {
	rows, err := d.QueryContext(ctx,
		`SELECT id,finding_id,version,report,evidence,summary,reason,author,created_at
		 FROM finding_report_versions WHERE finding_id=$1 ORDER BY version DESC`, findingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FindingRevision{}
	for rows.Next() {
		var r FindingRevision
		if err := rows.Scan(&r.ID, &r.FindingID, &r.Version, &r.Report, &r.Evidence, &r.Summary, &r.Reason, &r.Author, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetFindingRevision returns one snapshot, or ErrFindingNotFound.
func (d *DB) GetFindingRevision(ctx context.Context, findingID, version int64) (*FindingRevision, error) {
	var r FindingRevision
	err := d.QueryRowContext(ctx,
		`SELECT id,finding_id,version,report,evidence,summary,reason,author,created_at
		 FROM finding_report_versions WHERE finding_id=$1 AND version=$2`, findingID, version,
	).Scan(&r.ID, &r.FindingID, &r.Version, &r.Report, &r.Evidence, &r.Summary, &r.Reason, &r.Author, &r.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrFindingNotFound
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}
