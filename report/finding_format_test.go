package report

import (
	"strings"
	"testing"
)

const compliantReport = "# iac.oocl.com 客服会话越权\n" +
	"- 漏洞等级：高危（匿名可读他人私密会话）\n\n" +
	"## 一句话结论\n" +
	"`iac.oocl.com` 的 `his_session` 用会话头验签却按 param 取数，任意匿名者可读任意访客会话。\n\n" +
	"## 成立前提\n" +
	"匿名 `create-chat` 即可自建会话；目标 sessionId 为毫秒时间戳 + 2 位序号，可推算。\n\n" +
	"## 关键动作\n" +
	"`GET /ccrm/incall/outer/api/his_session?param=...`，X-Session-Id 用 A、`param.sessionId` 换成 B。\n\n" +
	"## 影响范围\n" +
	"任意访客的完整客服对话与其中的姓名/电话/运单号（PII）。\n"

func TestValidateFindingReportAcceptsCompliant(t *testing.T) {
	if missing := ValidateFindingReport(compliantReport); len(missing) != 0 {
		t.Fatalf("合规报告被拒，缺：%v", missing)
	}
}

func TestValidateFindingReportRejectsMissingSections(t *testing.T) {
	cases := map[string]string{
		"缺标题":  "- 漏洞等级：高危\n\n## 一句话结论\nx\n## 成立前提\nx\n## 关键动作\nx\n## 影响范围\nx\n",
		"缺等级":  "# 标题\n\n## 一句话结论\nx\n## 成立前提\nx\n## 关键动作\nx\n## 影响范围\nx\n",
		"缺结论":  "# 标题\n- 漏洞等级：高危\n## 成立前提\nx\n## 关键动作\nx\n## 影响范围\nx\n",
		"缺前提":  "# 标题\n- 漏洞等级：高危\n## 一句话结论\nx\n## 关键动作\nx\n## 影响范围\nx\n",
		"缺动作":  "# 标题\n- 漏洞等级：高危\n## 一句话结论\nx\n## 成立前提\nx\n## 影响范围\nx\n",
		"缺影响":  "# 标题\n- 漏洞等级：高危\n## 一句话结论\nx\n## 成立前提\nx\n## 关键动作\nx\n",
		"空标题":  "#   \n- 漏洞等级：高危\n## 一句话结论\nx\n## 成立前提\nx\n## 关键动作\nx\n## 影响范围\nx\n",
		"旧模板":  "## 概述\n旧格式\n## 影响与危害\nx\n## 修复建议\nx\n",
	}
	for name, md := range cases {
		if missing := ValidateFindingReport(md); len(missing) == 0 {
			t.Errorf("%s：应判为不合规", name)
		}
	}
}

func TestValidateFindingReportLegacyTemplateMissesAll(t *testing.T) {
	missing := ValidateFindingReport("## 概述\n旧模板报告")
	if len(missing) != len(findingReportSections) {
		t.Fatalf("旧模板应缺全部 %d 段，实际：%v", len(findingReportSections), missing)
	}
	if strings.Join(missing, ",") == "" {
		t.Fatal("缺段列表不应为空")
	}
}
