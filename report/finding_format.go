// Package report / finding_format.go —— 漏洞报告的固定 6 段结构校验。
//
// 规则（缺一不可）：标题、漏洞等级、一句话结论、成立前提、关键动作、影响范围。
// 「前提决定真伪」：若前提本身就是管理员权限、或越权所需对象 ID 根本拿不到，
// 报告不成立（正常功能），因此只认这 6 段，不再接受旧模板的概述/复现步骤/修复建议。
package report

import "strings"

// 固定报告结构的 6 个必需片段。
var findingReportSections = []struct {
	Label string
	Check func(string) bool
}{
	{"标题（首行 `# 标题`）", hasReportTitle},
	{"漏洞等级", func(md string) bool { return strings.Contains(md, "漏洞等级") }},
	{"## 一句话结论", func(md string) bool { return strings.Contains(md, "## 一句话结论") }},
	{"## 成立前提", func(md string) bool { return strings.Contains(md, "## 成立前提") }},
	{"## 关键动作", func(md string) bool { return strings.Contains(md, "## 关键动作") }},
	{"## 影响范围", func(md string) bool { return strings.Contains(md, "## 影响范围") }},
}

// ValidateFindingReport 返回缺失的段名；结构完整时返回 nil。
func ValidateFindingReport(md string) []string {
	var missing []string
	for _, s := range findingReportSections {
		if !s.Check(md) {
			missing = append(missing, s.Label)
		}
	}
	return missing
}

// hasReportTitle 判断首个非空行是否为非空的 `# ` 标题。
func hasReportTitle(md string) bool {
	for _, line := range strings.Split(md, "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		return strings.HasPrefix(t, "# ") && strings.TrimSpace(strings.TrimPrefix(t, "# ")) != ""
	}
	return false
}
