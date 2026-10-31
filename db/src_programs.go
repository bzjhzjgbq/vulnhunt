package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// SRC bounty program catalog (see schema.sql §SRC). The catalog is reference data
// synced from a public index; a task opts in by setting tasks.src_program_id.

// SRCProgramInput is one program as delivered by the upstream catalog.
type SRCProgramInput struct {
	SourceID              int64    `json:"id"`
	Name                  string   `json:"name"`
	TargetURL             string   `json:"target_url"`
	Host                  string   `json:"host"`
	IconURL               string   `json:"icon_url"`
	Status                string   `json:"status"`
	StatusLabel           string   `json:"status_label"`
	Heat                  int      `json:"heat"`
	Tags                  []string `json:"tags"`
	TagIDs                []int64  `json:"tag_ids"`
	AnalysisHasData       bool     `json:"analysis_has_data"`
	AnalysisLatestMonth   string   `json:"analysis_latest_month"`
	AnalysisLatestFetched string   `json:"analysis_latest_fetched_at"`
}

// SRCRegionInput groups programs by the upstream region bucket.
type SRCRegionInput struct {
	Name     string            `json:"name"`
	Programs []SRCProgramInput `json:"programs"`
}

// SRCSyncResult reports what one sync changed.
type SRCSyncResult struct {
	Fetched  int `json:"fetched"`
	Inserted int `json:"inserted"`
	Updated  int `json:"updated"`
}

// SRCProgram is one catalog row plus the newest task linked to it.
type SRCProgram struct {
	ID                    int64      `json:"id"`
	SourceID              int64      `json:"source_id"`
	Name                  string     `json:"name"`
	TargetURL             string     `json:"target_url"`
	Host                  string     `json:"host"`
	IconURL               string     `json:"icon_url"`
	Region                string     `json:"region"`
	Status                string     `json:"status"`
	StatusLabel           string     `json:"status_label"`
	Heat                  int        `json:"heat"`
	Tags                  []string   `json:"tags"`
	TagIDs                []int64    `json:"tag_ids"`
	AnalysisHasData       bool       `json:"analysis_has_data"`
	AnalysisLatestMonth   string     `json:"analysis_latest_month"`
	AnalysisLatestFetched *time.Time `json:"analysis_latest_fetched_at,omitempty"`
	UpdatedAt             time.Time  `json:"updated_at"`
	LinkedTaskID          *int64     `json:"linked_task_id,omitempty"`
	LinkedTaskStatus      string     `json:"linked_task_status,omitempty"`
}

// SRCProgramFilter narrows the catalog list.
type SRCProgramFilter struct {
	Query  string
	Tag    string
	Status string
	Region string
}

// SyncSRCPrograms upserts a full catalog snapshot. Rows missing from the snapshot
// are kept (the upstream may hide programs temporarily) but marked stale by
// synced_at, so nothing the operator linked disappears.
func (d *DB) SyncSRCPrograms(ctx context.Context, regions []SRCRegionInput) (SRCSyncResult, error) {
	var res SRCSyncResult
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return res, err
	}
	defer tx.Rollback() //nolint:errcheck
	for _, region := range regions {
		for _, p := range region.Programs {
			if p.SourceID <= 0 || strings.TrimSpace(p.Name) == "" {
				continue
			}
			res.Fetched++
			tags, _ := json.Marshal(p.Tags)
			tagIDs, _ := json.Marshal(p.TagIDs)
			var fetchedAt any
			if t, err := time.Parse(time.RFC3339, p.AnalysisLatestFetched); err == nil {
				fetchedAt = t
			}
			var inserted bool
			err := tx.QueryRowContext(ctx, `
INSERT INTO src_programs (source_id, name, target_url, host, icon_url, region,
  status, status_label, heat, tags, tag_ids, analysis_has_data,
  analysis_latest_month, analysis_latest_fetched_at, updated_at, synced_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,now(),now())
ON CONFLICT (source_id) DO UPDATE SET
  name=EXCLUDED.name, target_url=EXCLUDED.target_url, host=EXCLUDED.host,
  icon_url=EXCLUDED.icon_url, region=EXCLUDED.region, status=EXCLUDED.status,
  status_label=EXCLUDED.status_label, heat=EXCLUDED.heat, tags=EXCLUDED.tags,
  tag_ids=EXCLUDED.tag_ids, analysis_has_data=EXCLUDED.analysis_has_data,
  analysis_latest_month=EXCLUDED.analysis_latest_month,
  analysis_latest_fetched_at=EXCLUDED.analysis_latest_fetched_at,
  updated_at=now(), synced_at=now()
RETURNING (xmax = 0)`, p.SourceID, p.Name, p.TargetURL, p.Host, p.IconURL, region.Name,
				p.Status, p.StatusLabel, p.Heat, tags, tagIDs, p.AnalysisHasData,
				p.AnalysisLatestMonth, fetchedAt).Scan(&inserted)
			if err != nil {
				return res, fmt.Errorf("upsert src program #%d: %w", p.SourceID, err)
			}
			if inserted {
				res.Inserted++
			} else {
				res.Updated++
			}
		}
	}
	return res, tx.Commit()
}

const srcProgramCols = `p.id, p.source_id, p.name, p.target_url, p.host, p.icon_url,
 p.region, p.status, p.status_label, p.heat, p.tags, p.tag_ids,
 p.analysis_has_data, p.analysis_latest_month, p.analysis_latest_fetched_at,
 p.updated_at, t.id, COALESCE(t.status, '')`

func scanSRCPrograms(rows *sql.Rows) ([]SRCProgram, error) {
	var out []SRCProgram
	for rows.Next() {
		var p SRCProgram
		var tags, tagIDs []byte
		var fetched sql.NullTime
		var taskID sql.NullInt64
		if err := rows.Scan(&p.ID, &p.SourceID, &p.Name, &p.TargetURL, &p.Host, &p.IconURL,
			&p.Region, &p.Status, &p.StatusLabel, &p.Heat, &tags, &tagIDs,
			&p.AnalysisHasData, &p.AnalysisLatestMonth, &fetched,
			&p.UpdatedAt, &taskID, &p.LinkedTaskStatus); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(tags, &p.Tags)
		_ = json.Unmarshal(tagIDs, &p.TagIDs)
		if fetched.Valid {
			t := fetched.Time
			p.AnalysisLatestFetched = &t
		}
		if taskID.Valid {
			id := taskID.Int64
			p.LinkedTaskID = &id
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ListSRCPrograms returns the catalog, hottest first, with the newest linked task.
func (d *DB) ListSRCPrograms(ctx context.Context, f SRCProgramFilter) ([]SRCProgram, error) {
	var args []any
	where := []string{"1=1"}
	if q := strings.TrimSpace(f.Query); q != "" {
		args = append(args, "%"+q+"%")
		where = append(where, fmt.Sprintf("(p.name ILIKE $%d OR p.host ILIKE $%d)", len(args), len(args)))
	}
	if tag := strings.TrimSpace(f.Tag); tag != "" {
		args = append(args, tag)
		where = append(where, fmt.Sprintf("p.tags @> jsonb_build_array($%d::text)", len(args)))
	}
	if status := strings.TrimSpace(f.Status); status != "" {
		args = append(args, status)
		where = append(where, fmt.Sprintf("p.status = $%d", len(args)))
	}
	if region := strings.TrimSpace(f.Region); region != "" {
		args = append(args, region)
		where = append(where, fmt.Sprintf("p.region = $%d", len(args)))
	}
	query := `SELECT ` + srcProgramCols + ` FROM src_programs p
LEFT JOIN LATERAL (
  SELECT id, status FROM tasks
  WHERE src_program_id = p.id AND deleted_at IS NULL
  ORDER BY id DESC LIMIT 1
) t ON true
WHERE ` + strings.Join(where, " AND ") + `
ORDER BY p.heat DESC, p.id ASC`
	rows, err := d.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSRCPrograms(rows)
}

// GetSRCProgram returns one catalog row (no task join).
func (d *DB) GetSRCProgram(ctx context.Context, id int64) (*SRCProgram, error) {
	rows, err := d.QueryContext(ctx, `SELECT `+srcProgramCols+` FROM src_programs p
LEFT JOIN LATERAL (
  SELECT id, status FROM tasks
  WHERE src_program_id = p.id AND deleted_at IS NULL
  ORDER BY id DESC LIMIT 1
) t ON true WHERE p.id = $1`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items, err := scanSRCPrograms(rows)
	if err != nil || len(items) == 0 {
		return nil, err
	}
	return &items[0], nil
}

// SetTaskSRCProgram links a task to its originating program.
func (d *DB) SetTaskSRCProgram(ctx context.Context, taskID, programID int64) error {
	_, err := d.ExecContext(ctx, `UPDATE tasks SET src_program_id=$2, updated_at=now() WHERE id=$1`, taskID, programID)
	return err
}
