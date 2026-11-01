package report

import (
	"strings"
	"testing"
	"time"

	"vulnhunt/db"
)

func sampleFinding() *db.DBFinding {
	return &db.DBFinding{
		Name:      "导出游戏 WebView 任意文件读",
		VulnClass: "账号接管",
		Severity:  "high",
		Summary:   "承接事实 #15197 的读链",
		Evidence:  "GET /res_task?fileName=../bili.account.storage",
		Report:    "# 正文\n\n门禁绕过细节……",
		CreatedAt: time.Date(2026, 10, 31, 20, 0, 0, 0, time.Local),
	}
}

func TestSubmissionMarkdownButian(t *testing.T) {
	md := SubmissionMarkdown(sampleFinding(), "butian", time.Now())
	for _, want := range []string{"漏洞名称", "漏洞类型", "危害等级", "高危", "漏洞详情", "修复建议"} {
		if !strings.Contains(md, want) {
			t.Fatalf("butian template missing %q:\n%s", want, md)
		}
	}
	if strings.Contains(md, "#15197") {
		t.Fatalf("internal handle leaked into submission template:\n%s", md)
	}
}

func TestSubmissionMarkdownVulbox(t *testing.T) {
	md := SubmissionMarkdown(sampleFinding(), "vulbox", time.Now())
	for _, want := range []string{"基本信息", "等级", "高危", "漏洞描述", "修复方案"} {
		if !strings.Contains(md, want) {
			t.Fatalf("vulbox template missing %q:\n%s", want, md)
		}
	}
}

func TestSubmissionPackSeparatesFindings(t *testing.T) {
	second := sampleFinding()
	second.Name = "另一个漏洞"
	md := SubmissionPack([]*db.DBFinding{sampleFinding(), second}, "generic", time.Now())
	if strings.Count(md, "---") < 1 || !strings.Contains(md, "另一个漏洞") {
		t.Fatalf("pack did not include both findings:\n%s", md)
	}
	if !strings.Contains(md, "漏洞数量：2") {
		t.Fatalf("pack header missing count:\n%s", md)
	}
}
