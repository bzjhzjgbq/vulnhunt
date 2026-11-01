package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"vulnhunt/db"
	"golang.org/x/net/publicsuffix"
)

// SRC bounty catalog: browse the public SRC index, then turn one program into an
// VULNHUNT task with an explicit scope and an explicit authorization acknowledgement.
// The catalog itself is reference data; nothing here starts testing on its own.

const defaultSRCCatalogURL = "https://www.ver.dpdns.org/api/public/src"
const srcCatalogBodyLimit = 8 << 20 // 8 MiB

func (s *Server) srcPrograms(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	programs, err := s.m.pg.ListSRCPrograms(r.Context(), db.SRCProgramFilter{
		Query:           q.Get("q"),
		Tag:             q.Get("tag"),
		Status:          q.Get("status"),
		Region:          q.Get("region"),
		IncludeArchived: q.Get("include_archived") == "1",
	})
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"programs": programs, "total": len(programs)})
}

// srcProgramArchive marks a platform as retired (or restores it). Retired rows are
// hidden from the default list and cannot be launched.
func (s *Server) srcProgramArchive(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, 400, "bad program id")
		return
	}
	var body struct {
		Archived bool   `json:"archived"`
		Reason   string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, 400, "bad json: "+err.Error())
		return
	}
	ok, err := s.m.pg.SetSRCProgramArchived(r.Context(), id, body.Archived, body.Reason)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if !ok {
		writeErr(w, 404, "program not found")
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "archived": body.Archived})
}

// srcCatalogSync pulls the public catalog and upserts it. Failures are reported,
// never fatal: an empty catalog just means "sync again later".
func (s *Server) srcCatalogSync(w http.ResponseWriter, r *http.Request) {
	url := strings.TrimSpace(os.Getenv("VULNHUNT_SRC_CATALOG_URL"))
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
// authorization acknowledgement and a non-empty scope: VULNHUNT must never start
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
	if program.Archived || program.Status == "offline" || program.Status == "closed" {
		writeErr(w, 400, "该项目已停止运营（已下线），不能开测；如确认恢复请在平台库中取消下线标记")
		return
	}

	name := strings.TrimSpace(body.Name)
	if name == "" {
		name = program.Name
	}
	scopeText := strings.Join(scopes, "、")
	std := resolveSRCStandard(program)
	desc := fmt.Sprintf("%s\n项目主页：%s\n标签：%s\n热度：%d\n来源：SRC 平台库（h0day 目录）\n收录标准：%s（%s）",
		program.Name, program.TargetURL, strings.Join(program.Tags, "、"), program.Heat, std.Name, std.Confidence)
	goal := fmt.Sprintf("在「%s」的授权收录范围内做 SRC 漏洞挖掘，产出可直接提交、符合平台收录标准的中/高危漏洞报告。"+
		"用户声明的测试范围：%s。只测上述范围；每条 finding 必须给出可复现请求与原样响应片段。\n\n%s",
		program.Name, scopeText, std.promptText())

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
	// 把标准落成任务约束，规划者/Worker 每轮都会看到，避免挖到一半跑偏。
	if min := std.MinSeverity; min != "" {
		_, _ = t.Store.AddConstraint("deny", "只产出 "+min+" 及以上等级的漏洞；无法证明真实影响的方向直接跳过，不要凑 finding。", "policy")
	}
	if len(std.Rejects) > 0 {
		limit := len(std.Rejects)
		if limit > 8 {
			limit = 8
		}
		_, _ = t.Store.AddConstraint("deny", "平台不收、禁止产出与测试的类型："+strings.Join(std.Rejects[:limit], "；"), "policy")
	}
	s.launchTask(t, desc+" "+goal, false)
	writeJSON(w, 201, map[string]any{"ok": true, "task_id": t.ID, "program_id": program.ID, "scopes": scopes,
		"standard": std.ID, "confidence": std.Confidence})
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

// ---- scope candidates -------------------------------------------------------

var scopeDomainRe = regexp.MustCompile(`(?i)\b(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,24}\b`)
var scopeCIDRRe = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}/\d{1,2}\b`)
var scopeURLRe = regexp.MustCompile(`(?i)https?://[a-z0-9._~:/?#\[\]@!$&'()*+,;=%-]+`)

// scopeCommonTLD keeps the visible-text pass from treating JS property chains
// (value.open, prize.id, donation.money) as hosts. URL hosts are not limited.
var scopeCommonTLD = map[string]bool{
	"com": true, "cn": true, "net": true, "org": true, "io": true, "co": true,
	"me": true, "ai": true, "app": true, "dev": true, "cloud": true, "tech": true,
	"top": true, "xyz": true, "info": true, "biz": true, "site": true, "online": true,
	"shop": true, "store": true, "vip": true, "cc": true, "tv": true, "gov": true,
	"edu": true, "mil": true, "us": true, "uk": true, "jp": true, "hk": true,
	"tw": true, "sg": true, "kr": true, "de": true, "fr": true, "ru": true,
	"in": true, "au": true, "ca": true, "br": true, "nl": true, "se": true,
	"no": true, "fi": true, "dk": true, "it": true, "es": true, "pl": true,
	"tr": true, "il": true, "ae": true, "sa": true, "my": true, "th": true,
	"vn": true, "ph": true, "nz": true, "za": true, "mx": true, "ar": true,
	"cl": true,
}

var scopeNoiseDomains = map[string]bool{
	"w3.org": true, "google.com": true, "googleapis.com": true, "gstatic.com": true,
	"cloudflare.com": true, "jsdelivr.net": true, "github.com": true, "githubusercontent.com": true,
	"twitter.com": true, "x.com": true, "facebook.com": true, "youtube.com": true,
	"beian.miit.gov.cn": true, "miit.gov.cn": true, "aliyun.com": true, "qq.com": true,
	"baidu.com": true, "bootcss.com": true, "jquery.com": true, "fontawesome.com": true,
	"w3schools.com": true, "mozilla.org": true, "schema.org": true, "apache.org": true,
	"nginx.org": true, "openssl.org": true, "windows.net": true, "microsoft.com": true,
}

var scopeFileExt = map[string]bool{
	"png": true, "jpg": true, "jpeg": true, "gif": true, "svg": true, "webp": true,
	"css": true, "js": true, "html": true, "htm": true, "php": true, "asp": true,
	"aspx": true, "jsp": true, "json": true, "xml": true, "txt": true, "pdf": true,
	"zip": true, "woff": true, "woff2": true, "ttf": true, "ico": true, "map": true,
}

// srcProgramScopeCandidates fetches the program's own public page and extracts
// candidate domains / CIDRs for the launch dialog. It is a reading aid: the user
// still edits and confirms the list, and only the program page is fetched.
func (s *Server) srcProgramScopeCandidates(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, 400, "bad program id")
		return
	}
	program, err := s.m.pg.GetSRCProgram(r.Context(), id)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if program == nil || program.TargetURL == "" {
		writeErr(w, 404, "program has no target url")
		return
	}
	if !safePublicURL(program.TargetURL) {
		writeErr(w, 400, "只允许抓取公开的 http(s) 页面")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, program.TargetURL, nil)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; VULNHUNT-SRC/1.0)")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeErr(w, 502, "抓取项目页失败: "+err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		writeErr(w, 502, fmt.Sprintf("项目页返回 %d", resp.StatusCode))
		return
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		writeErr(w, 502, err.Error())
		return
	}
	candidates := extractScopeCandidates(string(body))
	writeJSON(w, 200, map[string]any{
		"program_id": program.ID,
		"source":     program.TargetURL,
		"candidates": candidates,
	})
}

type scopeCandidate struct {
	Value string `json:"value"`
	Kind  string `json:"kind"`
}

// extractScopeCandidates pulls deduplicated domains and CIDRs out of a page.
// Noise (asset hosts, standards bodies, CDNs) is filtered; the result is capped so
// the dialog stays usable.
func extractScopeCandidates(text string) []scopeCandidate {
	seen := map[string]bool{}
	out := make([]scopeCandidate, 0, 64)
	addRoot := func(raw string) bool {
		d := strings.ToLower(strings.TrimSuffix(raw, "."))
		if len(d) > 253 || d == "" || scopeNoiseDomains[d] {
			return len(out) >= 100
		}
		if labels := strings.Split(d, "."); scopeFileExt[labels[len(labels)-1]] {
			return len(out) >= 100
		}
		if _, icann := publicsuffix.PublicSuffix(d); !icann {
			return len(out) >= 100
		}
		root, err := publicsuffix.EffectiveTLDPlusOne(d)
		if err != nil || scopeNoiseDomains[root] || seen[root] {
			return len(out) >= 100
		}
		seen[root] = true
		out = append(out, scopeCandidate{Value: root, Kind: "root_domain"})
		return len(out) >= 100
	}

	for _, m := range scopeCIDRRe.FindAllString(text, 200) {
		if _, _, err := net.ParseCIDR(m); err != nil || seen[m] {
			continue
		}
		seen[m] = true
		out = append(out, scopeCandidate{Value: m, Kind: "cidr"})
		if len(out) >= 100 {
			return out
		}
	}

	// Pass 1: real URL hosts (src/href). These are trustworthy.
	for _, m := range scopeURLRe.FindAllString(text, 2000) {
		u, err := url.Parse(m)
		if err != nil || u.Hostname() == "" {
			continue
		}
		if addRoot(u.Hostname()) {
			return out
		}
	}
	// Pass 2: visible-text domains, limited to common TLDs. JS property chains
	// (value.open, prize.id) are excluded by this allowlist.
	for _, m := range scopeDomainRe.FindAllString(text, 4000) {
		d := strings.ToLower(strings.TrimSuffix(m, "."))
		labels := strings.Split(d, ".")
		if !scopeCommonTLD[labels[len(labels)-1]] {
			continue
		}
		if addRoot(d) {
			break
		}
	}
	return out
}

// safePublicURL rejects non-http(s) and private/loopback destinations, so a
// tampered catalog row cannot turn this endpoint into an SSRF probe.
func safePublicURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return false
	}
	ips, err := net.LookupIP(u.Hostname())
	if err != nil || len(ips) == 0 {
		return false
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
			return false
		}
	}
	return true
}
