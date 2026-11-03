package report

import (
	"fmt"
	"strings"
	"time"

	"vulnhunt/db"
)

// Submission templates: the bounty platforms each want a slightly different
// header, but the evidence body is the same. These layouts wrap a finding's
// report with the fields a submitter would otherwise retype by hand.

func severityCN(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "critical":
		return "严重"
	case "high":
		return "高危"
	case "medium":
		return "中危"
	case "low":
		return "低危"
	default:
		return "未评级"
	}
}

func findingAssetText(f *db.DBFinding) string {
	// AssetIDs are numeric handles; the human-facing URL usually lives in the
	// report body, so keep this honest rather than inventing one.
	return ""
}

// submissionBody is the evidence body shared by all templates.
func submissionBody(f *db.DBFinding) string {
	body := strings.TrimSpace(f.Report)
	if body == "" {
		body = strings.TrimSpace(f.Summary)
	}
	if ev := strings.TrimSpace(f.Evidence); ev != "" && !strings.Contains(body, ev) {
		body += "\n\n## 原始证据\n\n" + ev
	}
	return CleanForSubmission(body)
}

// SubmissionMarkdown renders one finding in the chosen platform layout.
func SubmissionMarkdown(f *db.DBFinding, template string, now time.Time) string {
	title := findingTitle(f)
	sev := severityCN(f.Severity)
	body := submissionBody(f)
	asset := findingAssetText(f)

	var b strings.Builder
	switch template {
	case "butian":
		b.WriteString("# " + title + "\n\n")
		b.WriteString("## 漏洞信息\n\n")
		b.WriteString("| 字段 | 内容 |\n| --- | --- |\n")
		fmt.Fprintf(&b, "| 漏洞名称 | %s |\n", title)
		fmt.Fprintf(&b, "| 漏洞类型 | %s |\n", nz(f.VulnClass, "其他"))
		fmt.Fprintf(&b, "| 危害等级 | %s |\n", sev)
		if asset != "" {
			fmt.Fprintf(&b, "| 漏洞URL | %s |\n", asset)
		}
		fmt.Fprintf(&b, "| 发现时间 | %s |\n\n", f.CreatedAt.Format("2006-01-02"))
		b.WriteString("## 漏洞详情\n\n")
		b.WriteString(body)
		b.WriteString("\n\n## 修复建议\n\n请参考报告内「修复建议」章节，按优先级修复并做回归验证。\n")
	case "vulbox":
		b.WriteString("# " + title + "\n\n")
		b.WriteString("## 基本信息\n\n")
		b.WriteString("| 字段 | 内容 |\n| --- | --- |\n")
		fmt.Fprintf(&b, "| 标题 | %s |\n", title)
		fmt.Fprintf(&b, "| 类型 | %s |\n", nz(f.VulnClass, "其他"))
		fmt.Fprintf(&b, "| 等级 | %s |\n", sev)
		fmt.Fprintf(&b, "| 资产 | %s |\n", nz(asset, "见报告正文"))
		fmt.Fprintf(&b, "| 提交时间 | %s |\n\n", now.Format("2006-01-02"))
		b.WriteString("## 漏洞描述\n\n")
		b.WriteString(body)
		b.WriteString("\n\n## 修复方案\n\n请参考报告内「修复建议」章节。\n")
	default:
		b.WriteString("# " + title + "\n\n")
		fmt.Fprintf(&b, "- **漏洞类型**：%s\n", nz(f.VulnClass, "其他"))
		fmt.Fprintf(&b, "- **危害等级**：%s\n", sev)
		fmt.Fprintf(&b, "- **发现时间**：%s\n\n", f.CreatedAt.Format("2006-01-02 15:04:05"))
		b.WriteString(body)
		b.WriteString("\n")
	}
	// Manually attached evidence travels with the report in every template, so
	// the copied draft and the batch submission pack stay equivalent.
	b.WriteString(structuredEvidenceMarkdown(f, false, 3))
	return b.String()
}

// SubmissionPack renders a batch as one document with clear separators.
func SubmissionPack(fs []*db.DBFinding, template string, now time.Time) string {
	items := append([]*db.DBFinding(nil), fs...)
	sortFindingsForExport(items)
	var b strings.Builder
	fmt.Fprintf(&b, "# 漏洞提交材料（%s）\n\n- 生成时间：%s\n- 漏洞数量：%d\n\n---\n\n",
		templateLabel(template), now.Format("2006-01-02 15:04:05"), len(items))
	for i, f := range items {
		if i > 0 {
			b.WriteString("\n---\n\n")
		}
		b.WriteString(SubmissionMarkdown(f, template, now))
	}
	return b.String()
}

func templateLabel(t string) string {
	switch t {
	case "butian":
		return "补天"
	case "vulbox":
		return "漏洞盒子"
	default:
		return "通用"
	}
}
