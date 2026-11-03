package agent

import (
	"strings"
	"testing"

	"vulnhunt/db"
)

func TestFalsePositivePatternFormatting(t *testing.T) {
	msg := formatFalsePositivePatterns([]db.FalsePositivePattern{{VulnClass: "信息泄露", Count: 3, Reasons: "仅暴露公开信息；响应无敏感字段"}})
	if !strings.Contains(msg, "信息泄露") || !strings.Contains(msg, "仅暴露公开信息") || !strings.Contains(msg, "不阻止登记") {
		t.Fatalf("unexpected warning: %s", msg)
	}
	if got := formatFalsePositivePatterns(nil); got != "" {
		t.Fatalf("empty patterns should render empty, got %q", got)
	}
}
