package db

import (
	"context"
	"fmt"
	"time"
)

// LLMUsageSummary is the window total for the always-on llm_usage ledger.
type LLMUsageSummary struct {
	Calls        int64 `json:"calls"`
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
	CacheRead    int64 `json:"cache_read"`
	CacheWrite   int64 `json:"cache_write"`
	Errors       int64 `json:"errors"`
	AvgLatencyMs int64 `json:"avg_latency_ms"`
}

// LLMUsageBucket is one grouped row (day/model/profile/task).
type LLMUsageBucket struct {
	Key          string `json:"key"`
	Calls        int64  `json:"calls"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	CacheRead    int64  `json:"cache_read"`
	CacheWrite   int64  `json:"cache_write"`
	Errors       int64  `json:"errors"`
	AvgLatencyMs int64  `json:"avg_latency_ms"`
}

// LLMUsageReport is the dashboard payload. Daily is chronological; the other
// slices are ordered by total tokens (desc) and capped.
type LLMUsageReport struct {
	Summary   LLMUsageSummary  `json:"summary"`
	Daily     []LLMUsageBucket `json:"daily"`
	ByModel   []LLMUsageBucket `json:"by_model"`
	ByProfile []LLMUsageBucket `json:"by_profile"`
	ByTask    []LLMUsageBucket `json:"by_task"`
}

const llmUsageAggSelect = `count(*),
	COALESCE(SUM(input_tokens),0), COALESCE(SUM(output_tokens),0),
	COALESCE(SUM(cache_read),0), COALESCE(SUM(cache_write),0),
	COALESCE(SUM(CASE WHEN status='error' THEN 1 ELSE 0 END),0),
	COALESCE(AVG(NULLIF(latency_ms,0))::bigint,0)`

// LLMUsageReport aggregates the ledger for [from, to). All timestamps are UTC.
func (d *DB) LLMUsageReport(ctx context.Context, from, to time.Time) (*LLMUsageReport, error) {
	rep := &LLMUsageReport{
		Daily: []LLMUsageBucket{}, ByModel: []LLMUsageBucket{},
		ByProfile: []LLMUsageBucket{}, ByTask: []LLMUsageBucket{},
	}
	if err := d.QueryRowContext(ctx,
		`SELECT `+llmUsageAggSelect+` FROM llm_usage WHERE ts >= $1 AND ts < $2`, from, to,
	).Scan(&rep.Summary.Calls, &rep.Summary.InputTokens, &rep.Summary.OutputTokens,
		&rep.Summary.CacheRead, &rep.Summary.CacheWrite, &rep.Summary.Errors, &rep.Summary.AvgLatencyMs); err != nil {
		return nil, err
	}

	var err error
	if rep.Daily, err = d.llmUsageBuckets(ctx,
		`TO_CHAR(DATE_TRUNC('day', ts AT TIME ZONE 'UTC'), 'YYYY-MM-DD')`,
		`key ASC`, 1000, from, to); err != nil {
		return nil, err
	}
	if rep.ByModel, err = d.llmUsageBuckets(ctx,
		`COALESCE(NULLIF(model,''),'(unknown)')`,
		`SUM(input_tokens)+SUM(output_tokens) DESC, key`, 20, from, to); err != nil {
		return nil, err
	}
	if rep.ByProfile, err = d.llmUsageBuckets(ctx,
		`COALESCE(NULLIF(profile_name,''),'(unknown)')`,
		`SUM(input_tokens)+SUM(output_tokens) DESC, key`, 20, from, to); err != nil {
		return nil, err
	}
	if rep.ByTask, err = d.llmUsageBuckets(ctx,
		`COALESCE(NULLIF(task_id,''),'(non-task)')`,
		`SUM(input_tokens)+SUM(output_tokens) DESC, key`, 20, from, to); err != nil {
		return nil, err
	}
	return rep, nil
}

// llmUsageBuckets groups by an internal constant expression. keyExpr and order
// are never user input; keeping them as parameters avoids four near-identical
// query bodies.
func (d *DB) llmUsageBuckets(ctx context.Context, keyExpr, order string, limit int, from, to time.Time) ([]LLMUsageBucket, error) {
	q := fmt.Sprintf(`SELECT %s AS key, %s
		FROM llm_usage WHERE ts >= $1 AND ts < $2
		GROUP BY 1 ORDER BY %s LIMIT $3`, keyExpr, llmUsageAggSelect, order)
	rows, err := d.QueryContext(ctx, q, from, to, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LLMUsageBucket{}
	for rows.Next() {
		var b LLMUsageBucket
		if err := rows.Scan(&b.Key, &b.Calls, &b.InputTokens, &b.OutputTokens,
			&b.CacheRead, &b.CacheWrite, &b.Errors, &b.AvgLatencyMs); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}
