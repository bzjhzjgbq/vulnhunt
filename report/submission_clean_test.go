package report

import (
	"strings"
	"testing"

	"vulnhunt/db"
)

func TestCleanForSubmissionRemovesInternalHandles(t *testing.T) {
	in := strings.Join([]string{
		"该链末端引用 finding #15197，并承接事实 #15297。",
		"详见意图 #15843 / work#2，资产 asset 19013。",
		"`get_finding_traffic(335)` 返回空绑定。",
		"同组件读链（#334 / 节点 #15197）为另一条路径，见 #15197。",
		"本链把 reading/#15401 删链的前置降为无条件。",
	}, "\n")
	out := CleanForSubmission(in)

	for _, forbidden := range []string{"15197", "15297", "15843", "19013", "get_finding_traffic"} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("cleaned text still contains %q:\n%s", forbidden, out)
		}
	}
	for _, want := range []string{"相关发现", "既有事实", "相关意图", "目标资产", "流量绑定记录"} {
		if !strings.Contains(out, want) {
			t.Fatalf("cleaned text lost %q:\n%s", want, out)
		}
	}
}

func TestCleanForSubmissionKeepsEvidenceIntact(t *testing.T) {
	in := "AndroidManifest 第 1503 行、smali offsets 0x28、HTTP/1.1 200、CVE-2021-44228、`/api/v3/graphql`"
	if out := CleanForSubmission(in); out != in {
		t.Fatalf("evidence text was rewritten:\n got: %s\nwant: %s", out, in)
	}
}

func TestSanitizeFindingTouchesAllHumanFields(t *testing.T) {
	f := &db.DBFinding{
		Name:     "账号接管（finding #15197）",
		Summary:  "承接事实 #15297",
		Evidence: "见 #15197",
		Report:   "# 标题\n\n引用 finding #15197 与 意图 #15843",
	}
	SanitizeFinding(f)
	for _, field := range []string{f.Name, f.Summary, f.Evidence, f.Report} {
		if strings.Contains(field, "#15") {
			t.Fatalf("field not cleaned: %q", field)
		}
	}
}
