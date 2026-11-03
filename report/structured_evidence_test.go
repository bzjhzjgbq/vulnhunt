package report

import (
	"strings"
	"testing"
	"time"

	"vulnhunt/db"
)

func TestStructuredEvidenceRendering(t *testing.T) {
	f := sampleFinding()
	f.ID = 42
	f.StructuredEvidence = []db.FindingEvidence{{
		ID:          7,
		Kind:        "command",
		Label:       "../../nmap scan",
		Note:        "anonymous upload",
		Content:     "preamble\n```\nHTTP/1.1 200 OK\n```\ntrailer",
		ContentHash: "deadbeef",
	}}

	md := SingleFindingMarkdown(f, time.Now())
	for _, want := range []string{
		"## 结构化证据",
		"### [command] ../../nmap scan",
		"anonymous upload",
		"deadbeef",
		"HTTP/1.1 200 OK",
		"[7-nmap_scan.txt](evidence/42/structured/7-nmap_scan.txt)",
		"````", // the content's triple backticks must not close the block
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("single md missing %q:\n%s", want, md)
		}
	}

	sub := SubmissionMarkdown(f, "butian", time.Now())
	if !strings.Contains(sub, "## 结构化证据") || !strings.Contains(sub, "HTTP/1.1 200 OK") {
		t.Fatalf("submission draft missing structured evidence:\n%s", sub)
	}
	if strings.Contains(sub, "evidence/42/structured/") {
		t.Fatalf("submission draft should inline evidence, not link archive files:\n%s", sub)
	}
}

func TestStructuredEvidenceFilenameIsArchiveSafe(t *testing.T) {
	cases := []struct {
		ev   db.FindingEvidence
		want string
	}{
		{db.FindingEvidence{ID: 1, Kind: "http", Label: "proof / login"}, "1-proof_login.http"},
		{db.FindingEvidence{ID: 2, Kind: "file", Label: " C:\\temp\\a.txt "}, "2-C_temp_a.txt.txt"},
		{db.FindingEvidence{ID: 3, Kind: "text", Label: "..", ContentHash: "x"}, "3-evidence.txt"},
		{db.FindingEvidence{ID: 4, Kind: "screenshot", Label: ""}, "4-screenshot.txt"},
	}
	for _, c := range cases {
		if got := StructuredEvidenceFilename(c.ev); got != c.want {
			t.Fatalf("filename(%+v)=%q want %q", c.ev, got, c.want)
		}
	}
}
