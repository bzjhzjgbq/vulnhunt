package db

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"
)

var ErrFindingEvidenceNotFound = errors.New("finding evidence not found")

// FindingEvidence is one structured evidence item attached to a finding. It is
// independent of captured HTTP traffic so non-HTTP findings can carry command
// output, file hashes or screenshot notes.
type FindingEvidence struct {
	ID          int64     `json:"id"`
	FindingID   int64     `json:"finding_id"`
	Kind        string    `json:"kind"`
	Label       string    `json:"label"`
	Content     string    `json:"content"`
	ContentHash string    `json:"content_hash"`
	Note        string    `json:"note"`
	Author      string    `json:"author"`
	CreatedAt   time.Time `json:"created_at"`
}

func evidenceHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

// ListFindingEvidence returns newest-first.
func (d *DB) ListFindingEvidence(ctx context.Context, findingID int64) ([]FindingEvidence, error) {
	rows, err := d.QueryContext(ctx,
		`SELECT id,finding_id,kind,label,content,content_hash,note,author,created_at
		 FROM finding_evidence WHERE finding_id=$1 ORDER BY created_at DESC, id DESC`, findingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FindingEvidence{}
	for rows.Next() {
		var e FindingEvidence
		if err := rows.Scan(&e.ID, &e.FindingID, &e.Kind, &e.Label, &e.Content, &e.ContentHash, &e.Note, &e.Author, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// AddFindingEvidence appends one evidence item. content_hash is always computed
// server-side so callers cannot claim a different hash.
func (d *DB) AddFindingEvidence(ctx context.Context, findingID int64, kind, label, content, note, author string) (*FindingEvidence, error) {
	var exists bool
	if err := d.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM findings WHERE id=$1)`, findingID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrFindingNotFound
	}
	e := &FindingEvidence{
		FindingID: findingID, Kind: kind, Label: label, Content: content,
		ContentHash: evidenceHash(content), Note: note, Author: author,
	}
	if err := d.QueryRowContext(ctx,
		`INSERT INTO finding_evidence(finding_id,kind,label,content,content_hash,note,author)
		 VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id, created_at`,
		findingID, kind, label, content, e.ContentHash, note, author,
	).Scan(&e.ID, &e.CreatedAt); err != nil {
		return nil, err
	}
	return e, nil
}

// DeleteFindingEvidence removes one item scoped to its finding.
func (d *DB) DeleteFindingEvidence(ctx context.Context, findingID, evidenceID int64) error {
	res, err := d.ExecContext(ctx,
		`DELETE FROM finding_evidence WHERE id=$1 AND finding_id=$2`, evidenceID, findingID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrFindingEvidenceNotFound
	}
	return nil
}
