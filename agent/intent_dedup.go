package agent

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

// Duplicate-direction guard.
//
// A planner that keeps re-discovering the same direction burns a worker slot and
// floods the graph with near-identical intents (the live 2026-10-31 run had 112
// exhausted intents, many restating earlier ones). Exact duplicates are obvious;
// this guard catches the "same sentence, new punctuation" class while leaving
// genuinely different directions alone.

// intentDupThreshold is deliberately high: only near-identical summaries are
// rejected. A planner re-stating a direction with a new angle usually differs in
// wording enough to fall well below this.
const intentDupThreshold = 0.90

// intentDupMinRunes avoids comparing trivially short strings ("test SSRF").
const intentDupMinRunes = 12

// normalizeIntentText lowercases and keeps letters/digits (including CJK),
// dropping whitespace, punctuation and markdown noise so formatting differences do
// not mask a duplicate.
func normalizeIntentText(s string) []rune {
	out := make([]rune, 0, len(s))
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			out = append(out, r)
		}
	}
	return out
}

// runeGrams returns the set of n-character shingles of s.
func runeGrams(s []rune, n int) map[string]struct{} {
	if len(s) < n {
		return map[string]struct{}{string(s): {}}
	}
	set := make(map[string]struct{}, len(s)-n+1)
	for i := 0; i+n <= len(s); i++ {
		set[string(s[i:i+n])] = struct{}{}
	}
	return set
}

// intentSimilarity is the Jaccard similarity of the two summaries' character
// 3-grams (0..1). Character shingles work for Chinese without a tokenizer.
func intentSimilarity(a, b string) float64 {
	ra, rb := normalizeIntentText(a), normalizeIntentText(b)
	if len(ra) < intentDupMinRunes || len(rb) < intentDupMinRunes {
		return 0
	}
	ga, gb := runeGrams(ra, 3), runeGrams(rb, 3)
	if len(ga) == 0 || len(gb) == 0 {
		return 0
	}
	inter := 0
	for g := range ga {
		if _, ok := gb[g]; ok {
			inter++
		}
	}
	union := len(ga) + len(gb) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

// duplicateActiveIntent reports the newest open/running intent whose summary is
// near-identical to summary. Store errors are swallowed: the guard is advisory and
// must never block an intent just because the lookup failed.
func (t *ToolSet) duplicateActiveIntent(summary string) (int64, float64, bool) {
	if t.ts == nil || strings.TrimSpace(summary) == "" {
		return 0, 0, false
	}
	nodes, err := t.ts.ListActiveIntents(300)
	if err != nil {
		return 0, 0, false
	}
	for _, n := range nodes {
		var p struct {
			Summary string `json:"summary"`
		}
		if err := json.Unmarshal(n.Payload, &p); err != nil || p.Summary == "" {
			continue
		}
		if sim := intentSimilarity(summary, p.Summary); sim >= intentDupThreshold {
			return n.ID, sim, true
		}
	}
	return 0, 0, false
}

// checkDuplicateIntent rejects a new direction that restates one already waiting or
// executing. Returns nil when the guard does not fire.
func (t *ToolSet) checkDuplicateIntent(summary string) error {
	id, sim, dup := t.duplicateActiveIntent(summary)
	if !dup {
		return nil
	}
	preview := []rune(summary)
	if len(preview) > 60 {
		preview = append(preview[:60], '…')
	}
	return fmt.Errorf("与已在排队/执行的意图 #%d 高度重复（相似度 %.0f%%）：%s。请换一个方向，或写清与它的差异（目标资产/测试手法/判据）",
		id, sim*100, string(preview))
}
