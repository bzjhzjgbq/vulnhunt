package report

import (
	"regexp"

	"github.com/Autumn-27/artex/db"
)

// Submission-ready export: ARTEX reports are written against its internal graph
// (finding/fact/intent/work ids). Those handles are meaningless — and noisy — on a
// bounty platform, so "clean" exports replace them with neutral wording while
// leaving the evidence (manifest lines, smali offsets, HTTP snippets) untouched.
//
// Only the human-facing prose is rewritten; request/response bytes inside evidence
// blocks are not pattern-matched because they never contain these handles.

type submissionRule struct {
	re   *regexp.Regexp
	repl string
}

var submissionRules = []submissionRule{
	{regexp.MustCompile(`(?i)\bfinding\s*#?\d+`), "相关发现"},
	{regexp.MustCompile(`(?i)\bfacts?\s*#?\d+`), "既有事实"},
	{regexp.MustCompile(`(?i)\bintents?\s*#?\d+`), "相关意图"},
	{regexp.MustCompile(`(?i)\bwork\s*#\d+`), "worker"},
	{regexp.MustCompile(`(?i)\bnodes?\s*#\d+`), "探索节点"},
	{regexp.MustCompile(`(?i)\bassets?\s+\d+`), "目标资产"},
	{regexp.MustCompile(`(?i)get_finding_traffic\s*\([^)]*\)`), "流量绑定记录"},
	{regexp.MustCompile(`探索节点\s*#\d+`), "探索节点"},
	{regexp.MustCompile(`事实节点\s*#\d+`), "既有事实"},
	{regexp.MustCompile(`事实\s*#\d+`), "既有事实"},
	{regexp.MustCompile(`意图\s*#\d+`), "相关意图"},
	{regexp.MustCompile(`work#\d+`), "worker"},
	// A bare handle that survived the rules above ("见 #15197", "读链/#15401"):
	// drop it wherever it sits so no dangling id is left behind.
	{regexp.MustCompile(`#\d{3,}\b`), ""},
}

// CleanForSubmission rewrites internal tracker handles in one text blob.
func CleanForSubmission(text string) string {
	if text == "" {
		return text
	}
	for _, rule := range submissionRules {
		text = rule.re.ReplaceAllString(text, rule.repl)
	}
	return text
}

// SanitizeFinding cleans every human-facing field of a finding in place.
func SanitizeFinding(f *db.DBFinding) {
	if f == nil {
		return
	}
	f.Name = CleanForSubmission(f.Name)
	f.Summary = CleanForSubmission(f.Summary)
	f.Evidence = CleanForSubmission(f.Evidence)
	f.Report = CleanForSubmission(f.Report)
}

// SanitizeFindings cleans a whole export batch.
func SanitizeFindings(fs []*db.DBFinding) {
	for _, f := range fs {
		SanitizeFinding(f)
	}
}
