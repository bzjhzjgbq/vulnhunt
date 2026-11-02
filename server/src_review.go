package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"vulnhunt/db"
)

// 标准体检：按任务所属 SRC 平台的收录标准，把「平台不收 / 等级不够」的待处理发现
// 批量标出来，供人工确认后一键忽略。它只做预览与忽略，不删除任何数据。

type standardReviewItem struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	VulnClass string `json:"vulnclass"`
	Severity  string `json:"severity"`
	Reason    string `json:"reason"`
	Keyword   string `json:"keyword,omitempty"`
}

type standardReviewResponse struct {
	TaskID       string               `json:"task_id"`
	TaskName     string               `json:"task_name"`
	StandardID   string               `json:"standard_id"`
	StandardName string               `json:"standard_name"`
	MinSeverity  string               `json:"min_severity"`
	Total        int                  `json:"total"`
	Keep         int                  `json:"keep"`
	Flagged      []standardReviewItem `json:"flagged"`
}

// standardForTask resolves the applicable standard from the task name/description.
// Launched tasks use the program name, so this covers both linked and legacy tasks.
func standardForTask(task *Task) *SRCStandard {
	needle := strings.TrimSpace(task.Name + " " + task.Description)
	std := resolveSRCStandard(&db.SRCProgram{Name: needle})
	// Task names look like "<vendor>｜<platform>SRC". resolveSRCStandard has
	// already merged the vendor's program rules into the platform base; here we
	// only make the explicit platform win when naming the standard, so a task
	// opened on 漏洞盒子 is not reported as the vendor's program standard.
	if platform := namedPlatform(needle); platform != nil {
		std.ID = platform.ID
		std.Name = platform.Name
	}
	return std
}

func (s *Server) taskStandardReview(w http.ResponseWriter, r *http.Request) {
	task := s.m.ResolveTask(r.PathValue("id"))
	if task == nil {
		writeErr(w, 404, "task not found")
		return
	}
	tid, err := strconv.ParseInt(task.ID, 10, 64)
	if err != nil || tid <= 0 {
		writeErr(w, 400, "task id is not numeric")
		return
	}
	std := standardForTask(task)
	rows, err := s.m.pg.QueryContext(r.Context(), `SELECT id, vulnclass, COALESCE(name, ''), severity, COALESCE(summary, ''), status
FROM findings WHERE task_id = $1 AND status = 'pending' ORDER BY id`, tid)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer rows.Close()

	resp := standardReviewResponse{
		TaskID:       task.ID,
		TaskName:     task.Name,
		StandardID:   std.ID,
		StandardName: std.Name,
		MinSeverity:  std.MinSeverity,
		Flagged:      []standardReviewItem{},
	}
	for rows.Next() {
		var id int64
		var vulnclass, name, severity, summary, status string
		if err := rows.Scan(&id, &vulnclass, &name, &severity, &summary, &status); err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		resp.Total++
		reason, keyword := standardReviewReason(std, vulnclass, name, summary, severity)
		if reason == "" {
			resp.Keep++
			continue
		}
		resp.Flagged = append(resp.Flagged, standardReviewItem{
			ID:        id,
			Name:      firstNonEmpty(name, vulnclass),
			VulnClass: vulnclass,
			Severity:  severity,
			Reason:    reason,
			Keyword:   keyword,
		})
	}
	if err := rows.Err(); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, resp)
}

func (s *Server) taskStandardReviewApply(w http.ResponseWriter, r *http.Request) {
	task := s.m.ResolveTask(r.PathValue("id"))
	if task == nil {
		writeErr(w, 404, "task not found")
		return
	}
	tid, err := strconv.ParseInt(task.ID, 10, 64)
	if err != nil || tid <= 0 {
		writeErr(w, 400, "task id is not numeric")
		return
	}
	var body struct {
		IDs []int64 `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, 400, "bad json: "+err.Error())
		return
	}
	if len(body.IDs) == 0 {
		writeErr(w, 400, "ids is empty")
		return
	}
	if len(body.IDs) > 500 {
		writeErr(w, 400, "too many ids (max 500)")
		return
	}
	res, err := s.m.pg.ExecContext(r.Context(), `UPDATE findings SET status = 'ignored'
WHERE task_id = $1 AND status = 'pending' AND id = ANY($2)`, tid, body.IDs)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	n, _ := res.RowsAffected()
	writeJSON(w, 200, map[string]any{"ok": true, "ignored": n})
}

// standardReviewReason decides whether one finding violates the standard.
// It returns a human-readable reason plus the matched reject keyword.
func standardReviewReason(std *SRCStandard, vulnclass, name, summary, severity string) (string, string) {
	var reasons []string
	keyword := ""
	if rank := severityRank(std.MinSeverity); rank > 0 && severityRank(severity) < rank {
		reasons = append(reasons, "等级低于标准（"+severityOrUnknown(severity)+" < "+std.MinSeverity+"）")
	}
	text := strings.ToLower(vulnclass + " " + name + " " + summary)
	for _, kw := range std.RejectKeywords {
		kw = strings.ToLower(strings.TrimSpace(kw))
		if kw != "" && strings.Contains(text, kw) {
			keyword = kw
			break
		}
	}
	if keyword != "" {
		reasons = append(reasons, "命中平台不收类型："+keyword)
	}
	return strings.Join(reasons, "；"), keyword
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func severityOrUnknown(sev string) string {
	if strings.TrimSpace(sev) == "" {
		return "unknown"
	}
	return sev
}
