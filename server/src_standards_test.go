package server

import (
	"strings"
	"testing"

	"vulnhunt/db"
)

func TestResolveSRCStandardMergesProgramOverride(t *testing.T) {
	std := resolveSRCStandard(&db.SRCProgram{Name: "Airbnb爱彼迎", Host: "airbnb.cn"})
	if std.Platform != "vulbox" {
		t.Fatalf("platform = %q, want vulbox", std.Platform)
	}
	if std.MinSeverity != "medium" {
		t.Fatalf("min severity = %q", std.MinSeverity)
	}
	joined := strings.Join(std.Rejects, "\n")
	// Platform rejects (vulbox) and program-specific rejects (Airbnb) must both be present.
	if !strings.Contains(joined, "无法利用的漏洞") {
		t.Fatalf("vulbox platform reject missing:\n%s", joined)
	}
	if !strings.Contains(joined, "Airbnb") && !strings.Contains(joined, "内部已知漏洞") {
		t.Fatalf("airbnb program reject missing:\n%s", joined)
	}
	if len(std.Scope) == 0 || !strings.Contains(strings.Join(std.Scope, ";"), "airbnb.cn") {
		t.Fatalf("scope missing: %v", std.Scope)
	}
	if std.Confidence != "verified" {
		t.Fatalf("confidence = %q, want verified", std.Confidence)
	}
}

func TestResolveSRCStandardFallsBackToGeneric(t *testing.T) {
	std := resolveSRCStandard(&db.SRCProgram{Name: "某不知名SRC", Host: "unknown.example"})
	if std.Platform != "generic-vendor" {
		t.Fatalf("platform = %q, want generic-vendor", std.Platform)
	}
	if !strings.Contains(strings.Join(std.Rejects, "\n"), "版本号") {
		t.Fatalf("generic rejects missing version disclosure rule")
	}
}

func TestStandardPromptMentionsCoreRules(t *testing.T) {
	std := resolveSRCStandard(&db.SRCProgram{Name: "哔哩哔哩 BILISRC", Host: "security.bilibili.com"})
	text := std.promptText()
	for _, want := range []string{"最低等级", "不收", "自检", "必剪"} {
		if !strings.Contains(text, want) {
			t.Fatalf("prompt missing %q:\n%s", want, text)
		}
	}
}
