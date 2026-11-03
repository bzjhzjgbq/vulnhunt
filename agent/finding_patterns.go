package agent

import (
	"fmt"
	"strings"

	"vulnhunt/db"
)

// formatFalsePositivePatterns renders a short, non-blocking warning for the model.
// It is deliberately advisory: the historical verdict may be wrong, and a repeat
// of the same class can still be a real new bug. The point is to force a second
// look at the impact, not to suppress the report.
func formatFalsePositivePatterns(pats []db.FalsePositivePattern) string {
	if len(pats) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("⚠ 历史误报提醒：本任务已有同类/相关历史误报，登记前请再确认本次影响不是同一模式：")
	for _, p := range pats {
		fmt.Fprintf(&b, "\n- %s ×%d", p.VulnClass, p.Count)
		if r := strings.TrimSpace(p.Reasons); r != "" {
			runes := []rune(r)
			if len(runes) > 240 {
				r = string(runes[:240]) + "…"
			}
			b.WriteString("：" + r)
		}
	}
	b.WriteString("\n仅作提醒，不阻止登记；若本次确有不同根因或新增影响，请在 evidence 中写明差异。")
	return b.String()
}
