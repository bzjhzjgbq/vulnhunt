package db

import "context"

// FalsePositivePatterns aggregates historical false-positive/ignored findings of
// this task (plus automated likely_fp verdicts) by vuln class. When vulnClass or
// assetIDs are supplied it narrows to that class or findings sharing an asset, so
// the warning is about "the same kind of non-issue", not unrelated noise.
//
// This is advisory: a DB error must never block reporting, so callers treat an
// error as "no patterns".
func (s *ExplorationStore) FalsePositivePatterns(ctx context.Context, vulnClass string, assetIDs []int64, limit int) ([]FalsePositivePattern, error) {
	if limit <= 0 {
		limit = 5
	}
	if assetIDs == nil {
		assetIDs = []int64{}
	}
	rows, err := s.db.QueryContext(ctx, `
		WITH mine AS (
			SELECT DISTINCT ts.company_id
			FROM task_scope ts
			JOIN tasks t ON t.id = ts.task_id
			WHERE t.exploration_id = $1 AND ts.kind = 'company' AND ts.company_id IS NOT NULL
		)
		SELECT COALESCE(NULLIF(f.vulnclass, ''), '未分类') AS vc,
		       count(*)::int,
		       COALESCE(string_agg(DISTINCT NULLIF(COALESCE(NULLIF(f.triage_reason, ''), NULLIF(f.review_reason, '')), ''), '；'), '')
		FROM findings f
		JOIN tasks t ON t.id = f.task_id AND t.deleted_at IS NULL
		WHERE (t.exploration_id = $1
		       OR EXISTS (SELECT 1 FROM task_scope x JOIN mine m ON m.company_id = x.company_id
		                  WHERE x.task_id = f.task_id AND x.kind = 'company'))
		  AND (f.status IN ('false_positive','ignored') OR f.review_status = 'likely_fp')
		  AND ($2 = '' OR lower(f.vulnclass) = lower($2)
		       OR EXISTS (SELECT 1 FROM jsonb_array_elements(f.asset_ids) e WHERE (e)::bigint = ANY($3::bigint[])))
		GROUP BY 1
		ORDER BY 2 DESC, 1
		LIMIT $4`, s.expID, vulnClass, assetIDs, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FalsePositivePattern{}
	for rows.Next() {
		var p FalsePositivePattern
		if err := rows.Scan(&p.VulnClass, &p.Count, &p.Reasons); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
