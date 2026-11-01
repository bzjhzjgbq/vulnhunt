package server

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

// 提交自检：在把 finding 推进到「已确认 → 提交台账」之前，检查证据是否足以
// 支撑一条可提交的漏洞报告。它只做启发式检查（长度、请求/响应、报告结构、
// 等级），目的是挡住空壳/占位/低等级产出，不替代人工判断。

var (
	httpRequestRe  = regexp.MustCompile(`(?i)(^|\n)\s*(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)\s+/\S*|curl\s+-`)
	httpResponseRe = regexp.MustCompile(`(?i)HTTP/[12](\.\d)?\s+\d{3}|响应|response|状态码`)
	reportPartRe   = regexp.MustCompile(`(?i)##|复现|影响|修复建议|漏洞详情`)
	// 客户端/二进制类漏洞没有 HTTP 请求响应，用调用链与结果证明代替。
	clientSideRe = regexp.MustCompile(`(?i)android|ios|客户端|apk|ipa|二进制|逆向|静态分析`)
	reproStepRe  = regexp.MustCompile(`(?i)步骤|调用|复现|adb|frida|smali|hook|反编译`)
	proofRe      = regexp.MustCompile(`(?i)返回|结果|样例|数据|日志|证明|截图|toast`)
)

type evidenceCheckItem struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
}

type evidenceReviewFinding struct {
	ID       int64               `json:"id"`
	Name     string              `json:"name"`
	Severity string              `json:"severity"`
	Status   string              `json:"status"`
	Score    int                 `json:"score"`
	Max      int                 `json:"max"`
	Ready    bool                `json:"ready"`
	Checks   []evidenceCheckItem `json:"checks"`
}

type evidenceReviewResponse struct {
	TaskID   string                  `json:"task_id"`
	TaskName string                  `json:"task_name"`
	Total    int                     `json:"total"`
	Ready    int                     `json:"ready"`
	Weak     []evidenceReviewFinding `json:"weak"`
	Ok       []evidenceReviewFinding `json:"ok"`
}

func checkEvidence(name, vulnclass, summary, evidence, report, severity string) ([]evidenceCheckItem, int, bool) {
	title := firstNonEmpty(name, vulnclass)
	clientSide := clientSideRe.MatchString(vulnclass + " " + name)
	requestCheck := evidenceCheckItem{Key: "request", Label: "证据含原始请求", OK: httpRequestRe.MatchString(evidence)}
	responseCheck := evidenceCheckItem{Key: "response", Label: "证据含响应/状态码", OK: httpResponseRe.MatchString(evidence)}
	if clientSide {
		requestCheck = evidenceCheckItem{Key: "request", Label: "证据含复现步骤/调用链", OK: reproStepRe.MatchString(evidence)}
		responseCheck = evidenceCheckItem{Key: "response", Label: "证据含结果/数据样例", OK: proofRe.MatchString(evidence)}
	}
	checks := []evidenceCheckItem{
		{Key: "title", Label: "有可读标题", OK: strings.TrimSpace(title) != ""},
		{Key: "summary", Label: "描述 ≥ 40 字", OK: len([]rune(strings.TrimSpace(summary))) >= 40,
			Detail: strconv.Itoa(len([]rune(strings.TrimSpace(summary)))) + " 字"},
		{Key: "evidence", Label: "证据 ≥ 300 字", OK: len([]rune(strings.TrimSpace(evidence))) >= 300,
			Detail: strconv.Itoa(len([]rune(strings.TrimSpace(evidence)))) + " 字"},
		requestCheck,
		responseCheck,
		{Key: "report", Label: "报告 ≥ 500 字", OK: len([]rune(strings.TrimSpace(report))) >= 500,
			Detail: strconv.Itoa(len([]rune(strings.TrimSpace(report)))) + " 字"},
		{Key: "report_parts", Label: "报告含复现/影响结构", OK: reportPartRe.MatchString(report)},
		{Key: "severity", Label: "等级 ≥ 中危", OK: severityRank(severity) >= severityRank("medium"),
			Detail: severityOrUnknown(severity)},
	}
	passed := 0
	for _, c := range checks {
		if c.OK {
			passed++
		}
	}
	return checks, passed, passed == len(checks)
}

func (s *Server) taskEvidenceReview(w http.ResponseWriter, r *http.Request) {
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
	rows, err := s.m.pg.QueryContext(r.Context(), `SELECT id, vulnclass, COALESCE(name,''), severity, COALESCE(summary,''),
COALESCE(evidence,''), COALESCE(report,''), status
FROM findings WHERE task_id=$1 AND status IN ('pending','confirmed') ORDER BY id`, tid)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer rows.Close()

	resp := evidenceReviewResponse{TaskID: task.ID, TaskName: task.Name, Weak: []evidenceReviewFinding{}, Ok: []evidenceReviewFinding{}}
	for rows.Next() {
		var id int64
		var vulnclass, name, severity, summary, evidence, report, status string
		if err := rows.Scan(&id, &vulnclass, &name, &severity, &summary, &evidence, &report, &status); err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		checks, passed, ready := checkEvidence(name, vulnclass, summary, evidence, report, severity)
		item := evidenceReviewFinding{
			ID:       id,
			Name:     firstNonEmpty(name, vulnclass),
			Severity: severity,
			Status:   status,
			Score:    passed,
			Max:      len(checks),
			Ready:    ready,
			Checks:   checks,
		}
		resp.Total++
		if ready {
			resp.Ready++
			resp.Ok = append(resp.Ok, item)
		} else {
			resp.Weak = append(resp.Weak, item)
		}
	}
	if err := rows.Err(); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, resp)
}

func (s *Server) taskEvidenceReviewApply(w http.ResponseWriter, r *http.Request) {
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
		IDs    []int64 `json:"ids"`
		Action string  `json:"action"` // confirm = 推进为已确认；ignore = 忽略
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, 400, "bad json: "+err.Error())
		return
	}
	status := ""
	switch body.Action {
	case "confirm":
		status = "confirmed"
	case "ignore":
		status = "ignored"
	default:
		writeErr(w, 400, "action must be confirm or ignore")
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
	res, err := s.m.pg.ExecContext(r.Context(), `UPDATE findings SET status=$2
WHERE task_id=$1 AND status='pending' AND id = ANY($3)`, tid, status, body.IDs)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	n, _ := res.RowsAffected()
	writeJSON(w, 200, map[string]any{"ok": true, "updated": n, "status": status})
}
