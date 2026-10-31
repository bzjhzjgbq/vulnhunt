package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Autumn-27/artex/db"
)

// SRC bounty catalog: browse the public SRC index, then turn one program into an
// ARTEX task with an explicit scope and an explicit authorization acknowledgement.
// The catalog itself is reference data; nothing here starts testing on its own.

const defaultSRCCatalogURL = "https://www.ver.dpdns.org/api/public/src"
const srcCatalogBodyLimit = 8 << 20 // 8 MiB

func (s *Server) srcPrograms(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	programs, err := s.m.pg.ListSRCPrograms(r.Context(), db.SRCProgramFilter{
		Query:  q.Get("q"),
		Tag:    q.Get("tag"),
		Status: q.Get("status"),
		Region: q.Get("region"),
	})
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"programs": programs, "total": len(programs)})
}

// srcCatalogSync pulls the public catalog and upserts it. Failures are reported,
// never fatal: an empty catalog just means "sync again later".
func (s *Server) srcCatalogSync(w http.ResponseWriter, r *http.Request) {
	url := strings.TrimSpace(os.Getenv("ARTEX_SRC_CATALOG_URL"))
	if url == "" {
		url = defaultSRCCatalogURL
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeErr(w, 502, "拉取 SRC 目录失败: "+err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		writeErr(w, 502, fmt.Sprintf("SRC 目录返回 %d", resp.StatusCode))
		return
	}
	var payload struct {
		Regions []db.SRCRegionInput `json:"regions"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, srcCatalogBodyLimit)).Decode(&payload); err != nil {
		writeErr(w, 502, "解析 SRC 目录失败: "+err.Error())
		return
	}
	res, err := s.m.pg.SyncSRCPrograms(r.Context(), payload.Regions)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "source": url, "result": res})
}

// srcProgramLaunch creates a task for one program. It refuses without an explicit
// authorization acknowledgement and a non-empty scope: ARTEX must never start
// probing a company just because its SRC page is listed.
func (s *Server) srcProgramLaunch(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, 400, "bad program id")
		return
	}
	var body struct {
		Scope      []string `json:"scope"`
		Authorized bool     `json:"authorized"`
		Name       string   `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, 400, "bad json: "+err.Error())
		return
	}
	if !body.Authorized {
		writeErr(w, 400, "需要先确认已加入该项目并获得测试授权")
		return
	}
	scopes := normalizeSRCScopes(body.Scope)
	if len(scopes) == 0 {
		writeErr(w, 400, "请至少填写一条测试范围（域名 / IP / CIDR）")
		return
	}
	program, err := s.m.pg.GetSRCProgram(r.Context(), id)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if program == nil {
		writeErr(w, 404, "program not found")
		return
	}

	name := strings.TrimSpace(body.Name)
	if name == "" {
		name = program.Name
	}
	scopeText := strings.Join(scopes, "、")
	desc := fmt.Sprintf("%s\n项目主页：%s\n标签：%s\n热度：%d\n来源：SRC 平台库（h0day 目录）",
		program.Name, program.TargetURL, strings.Join(program.Tags, "、"), program.Heat)
	goal := fmt.Sprintf("在「%s」的授权收录范围内做 SRC 漏洞挖掘，产出可直接提交的中/高危漏洞报告。"+
		"用户声明的测试范围：%s。只测上述范围；每条 finding 必须给出可复现请求与原样响应片段；"+
		"按该 SRC 的评级口径组织报告。", program.Name, scopeText)

	t, err := s.m.CreateTaskWithOptions(desc, goal, db.TaskCreateOptions{Name: name})
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	taskID, _ := strconv.ParseInt(t.ID, 10, 64)
	if err := s.m.pg.SetTaskSRCProgram(r.Context(), taskID, program.ID); err != nil {
		_ = s.m.pg.DeleteTask(taskID)
		writeErr(w, 500, "关联 SRC 项目失败: "+err.Error())
		return
	}
	as := s.m.Assets()
	if as == nil {
		_ = s.m.pg.DeleteTask(taskID)
		writeErr(w, 503, "asset store 未启用，无法写入测试范围")
		return
	}
	for _, value := range scopes {
		if _, err := as.AddAgentScope(taskID, inferScopeKind(value), value, "SRC 平台库导入", "manual"); err != nil {
			_ = s.m.pg.DeleteTask(taskID)
			writeErr(w, 400, fmt.Sprintf("范围 %q 无效：%v", value, err))
			return
		}
	}
	s.launchTask(t, desc+" "+goal, false)
	writeJSON(w, 201, map[string]any{"ok": true, "task_id": t.ID, "program_id": program.ID, "scopes": scopes})
}

// normalizeSRCScopes trims, de-duplicates and strips URL/wildcard decoration so
// "https://*.Example.com/path" becomes "example.com".
func normalizeSRCScopes(raw []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		v := strings.TrimSpace(item)
		if v == "" || strings.HasPrefix(v, "#") {
			continue
		}
		v = strings.TrimPrefix(v, "https://")
		v = strings.TrimPrefix(v, "http://")
		// Keep CIDR prefixes intact ("10.0.0.0/24"); only strip a URL path when the
		// value is not itself a valid CIDR ("example.com/path").
		if _, _, err := net.ParseCIDR(v); err != nil {
			if i := strings.IndexAny(v, "/?#"); i >= 0 {
				v = v[:i]
			}
		}
		v = strings.TrimPrefix(v, "*.")
		v = strings.TrimSuffix(v, ".")
		v = strings.ToLower(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

// inferScopeKind maps one scope line to the asset store's kind vocabulary.
func inferScopeKind(value string) string {
	if strings.Contains(value, "/") {
		if _, _, err := net.ParseCIDR(value); err == nil {
			return "cidr"
		}
	}
	if net.ParseIP(value) != nil {
		return "ip"
	}
	return "root_domain"
}
