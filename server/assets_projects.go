package server

import (
	"context"
	"net/http"

	"github.com/Autumn-27/artex/db"
)

// assetProject is one SRC project's asset footprint. A task == one project run;
// the label prefers the linked SRC program so the fleet can be told apart by
// platform (Airbnb / BILISRC / OSRC …) instead of opaque task ids.
type assetProject struct {
	TaskID      string         `json:"task_id"`
	TaskName    string         `json:"task_name"`
	ProgramID   *int64         `json:"program_id,omitempty"`
	ProgramName string         `json:"program_name,omitempty"`
	Label       string         `json:"label"`
	Total       int            `json:"total"`
	Counts      map[string]int `json:"counts"`
}

// assetProjects GET /api/assets/projects — per-task asset counts grouped by type.
func (s *Server) assetProjects(w http.ResponseWriter, r *http.Request) {
	pg := s.m.pg
	if pg == nil {
		writeErr(w, 503, "database unavailable")
		return
	}
	ctx := r.Context()
	projects, err := listAssetProjects(ctx, pg)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"projects": projects})
}

func listAssetProjects(ctx context.Context, pg *db.DB) ([]assetProject, error) {
	rows, err := pg.QueryContext(ctx, `
SELECT t.id, COALESCE(NULLIF(t.name,''), t.description), t.src_program_id, COALESCE(p.name,'')
FROM tasks t
LEFT JOIN src_programs p ON p.id = t.src_program_id
WHERE t.deleted_at IS NULL
  AND EXISTS (SELECT 1 FROM assets a WHERE t.id = ANY(a.task_ids))
ORDER BY t.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	projects := []*assetProject{}
	index := map[int64]*assetProject{}
	for rows.Next() {
		var id int64
		var name string
		var programID *int64
		var programName string
		if err := rows.Scan(&id, &name, &programID, &programName); err != nil {
			return nil, err
		}
		p := &assetProject{
			TaskID:      i64s(id),
			TaskName:    name,
			ProgramID:   programID,
			ProgramName: programName,
			Counts:      map[string]int{},
		}
		if programName != "" {
			p.Label = programName
		} else {
			p.Label = name
		}
		projects = append(projects, p)
		index[id] = p
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	countRows, err := pg.QueryContext(ctx, `
SELECT t.id, a.type, count(*)
FROM tasks t
JOIN assets a ON t.id = ANY(a.task_ids)
WHERE t.deleted_at IS NULL
GROUP BY t.id, a.type`)
	if err != nil {
		return nil, err
	}
	defer countRows.Close()
	for countRows.Next() {
		var taskID int64
		var typ string
		var n int
		if err := countRows.Scan(&taskID, &typ, &n); err != nil {
			return nil, err
		}
		if p := index[taskID]; p != nil {
			p.Counts[typ] = n
			p.Total += n
		}
	}
	if err := countRows.Err(); err != nil {
		return nil, err
	}

	out := make([]assetProject, 0, len(projects))
	for _, p := range projects {
		out = append(out, *p)
	}
	// Largest footprint first.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Total > out[j-1].Total; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out, nil
}
