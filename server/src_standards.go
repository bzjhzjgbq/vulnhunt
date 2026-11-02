package server

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"vulnhunt/db"
)

// SRC 收录标准库：把各平台的「收什么 / 不收什么 / 最低等级 / 范围与特殊规则」做成结构化数据，
// 开测时写进任务目标与约束，产出时作为自检口径，避免大量低价值 finding。

//go:embed src_standards.json
var srcStandardsJSON []byte

// SRCStandard 是一条平台级或项目级标准。
type SRCStandard struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Platform    string   `json:"platform,omitempty"`
	Match       []string `json:"match,omitempty"`
	Source      string   `json:"source,omitempty"`
	Confidence  string   `json:"confidence,omitempty"`
	MinSeverity string   `json:"min_severity,omitempty"`
	Accepts     []string `json:"accepts,omitempty"`
	Rejects     []string `json:"rejects,omitempty"`
	Scope       []string `json:"scope,omitempty"`
	Notes       []string `json:"notes,omitempty"`
	// RejectKeywords 供「标准体检」匹配：命中即视为平台不收的类型（来自 rejects 的可判定子集）。
	RejectKeywords []string `json:"reject_keywords,omitempty"`
}

type srcStandardBook struct {
	Version   int           `json:"version"`
	UpdatedAt string        `json:"updated_at"`
	Platforms []SRCStandard `json:"platforms"`
	Programs  []SRCStandard `json:"programs"`
}

var (
	srcStdOnce sync.Once
	srcStdBook srcStandardBook
)

func loadSRCStandards() *srcStandardBook {
	srcStdOnce.Do(func() {
		if err := json.Unmarshal(srcStandardsJSON, &srcStdBook); err != nil {
			log.Printf("[src-standards] 解析内置标准库失败: %v", err)
		}
	})
	return &srcStdBook
}

func severityRank(s string) int {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "critical":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	}
	return 0
}

func maxSeverity(a, b string) string {
	if severityRank(a) >= severityRank(b) {
		if a != "" {
			return a
		}
		return b
	}
	return b
}

// resolveSRCStandard merges the program override with its platform default.
func resolveSRCStandard(program *db.SRCProgram) *SRCStandard {
	book := loadSRCStandards()
	needle := strings.ToLower(program.Name + " " + program.Host)
	var override *SRCStandard
	for i := range book.Programs {
		for _, m := range book.Programs[i].Match {
			if m != "" && strings.Contains(needle, strings.ToLower(m)) {
				override = &book.Programs[i]
				break
			}
		}
		if override != nil {
			break
		}
	}
	baseID := "generic-vendor"
	if override != nil && override.Platform != "" {
		baseID = override.Platform
	}
	var base *SRCStandard
	for i := range book.Platforms {
		if book.Platforms[i].ID == baseID {
			base = &book.Platforms[i]
			break
		}
	}
	if base == nil && len(book.Platforms) > 0 {
		base = &book.Platforms[0]
	}
	if base == nil {
		return &SRCStandard{ID: "generic-vendor", Name: "通用口径", MinSeverity: "medium"}
	}
	out := &SRCStandard{
		ID:             base.ID,
		Name:           base.Name,
		Platform:       base.ID,
		Source:         base.Source,
		Confidence:     base.Confidence,
		MinSeverity:    base.MinSeverity,
		Accepts:        append([]string(nil), base.Accepts...),
		Rejects:        append([]string(nil), base.Rejects...),
		Notes:          append([]string(nil), base.Notes...),
		RejectKeywords: append([]string(nil), base.RejectKeywords...),
	}
	if override != nil {
		out.ID = override.ID
		out.Name = override.Name
		out.Scope = override.Scope
		if len(override.Accepts) > 0 {
			out.Accepts = override.Accepts
		}
		out.Rejects = append(out.Rejects, override.Rejects...)
		out.Notes = append(out.Notes, override.Notes...)
		out.RejectKeywords = append(out.RejectKeywords, override.RejectKeywords...)
		out.MinSeverity = maxSeverity(out.MinSeverity, override.MinSeverity)
		if override.Source != "" {
			out.Source = override.Source
		}
		if override.Confidence == "verified" {
			out.Confidence = "verified"
		}
	}
	return out
}

// namedPlatform returns the platform standard explicitly named in needle
// (e.g. "漏洞盒子SRC", "补天专属SRC"). Task names are usually
// "<vendor>｜<platform>SRC"; the explicit platform must win over the vendor's
// program override when choosing which standard the task is reviewed against.
func namedPlatform(needle string) *SRCStandard {
	book := loadSRCStandards()
	lower := strings.ToLower(needle)
	for i := range book.Platforms {
		for _, m := range book.Platforms[i].Match {
			if m != "" && strings.Contains(lower, strings.ToLower(m)) {
				return &book.Platforms[i]
			}
		}
	}
	return nil
}

// standardPrompt renders the standard as instructions the planner/worker must follow.
func (std *SRCStandard) promptText() string {
	var b strings.Builder
	b.WriteString("【收录标准（必须遵守，来自平台公开规则）】\n")
	if std.MinSeverity != "" {
		fmt.Fprintf(&b, "- 只产出 %s 及以上等级的漏洞；低于该等级不要写 finding。\n", std.MinSeverity)
	}
	if len(std.Scope) > 0 {
		fmt.Fprintf(&b, "- 范围：%s。\n", strings.Join(std.Scope, "；"))
	}
	if len(std.Accepts) > 0 {
		b.WriteString("- 可收（优先级从高到低）：\n")
		for _, a := range std.Accepts {
			fmt.Fprintf(&b, "  · %s\n", a)
		}
	}
	if len(std.Rejects) > 0 {
		b.WriteString("- 不收（不要测试、更不要产出 finding）：\n")
		for _, r := range std.Rejects {
			fmt.Fprintf(&b, "  · %s\n", r)
		}
	}
	if len(std.Notes) > 0 {
		b.WriteString("- 特殊规则：\n")
		for _, n := range std.Notes {
			fmt.Fprintf(&b, "  · %s\n", n)
		}
	}
	b.WriteString("- 上报前自检：类型是否在可收清单内、是否有真实影响的可复现证据、是否达到最低等级；任一项不满足就继续换方向，不要凑数。\n")
	return b.String()
}

func (s *Server) srcStandards(w http.ResponseWriter, r *http.Request) {
	book := loadSRCStandards()
	writeJSON(w, 200, book)
}

// srcProgramStandard returns the resolved standard for one catalog program.
func (s *Server) srcProgramStandard(w http.ResponseWriter, r *http.Request) {
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
	if program == nil {
		writeErr(w, 404, "program not found")
		return
	}
	std := resolveSRCStandard(program)
	writeJSON(w, 200, map[string]any{
		"program_id": program.ID,
		"program":    program.Name,
		"standard":   std,
	})
}
