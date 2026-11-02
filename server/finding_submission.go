package server

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	reportpkg "vulnhunt/report"
)

// GET /api/exploration/findings/{id}/submission?template=butian|vulbox|generic
//
// Returns the platform-specific submission draft so the UI can copy it without
// reimplementing the template in TypeScript.
func (s *Server) findingSubmission(w http.ResponseWriter, r *http.Request) {
	id := int64(atoiDefault(r.PathValue("id"), 0))
	if id <= 0 {
		writeErr(w, 400, "bad finding id")
		return
	}
	template := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("template")))
	if template == "" {
		template = "generic"
	}
	switch template {
	case "generic", "butian", "vulbox":
	default:
		writeErr(w, 400, "template 只支持 generic / butian / vulbox")
		return
	}
	f, err := s.m.pg.GetFinding(id)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if f == nil {
		writeErr(w, 404, "finding not found")
		return
	}
	md := reportpkg.SubmissionMarkdown(f, template, time.Now())
	// Structured evidence is independent of the report body; append it so the
	// copied submission includes command output / file hashes / screenshot notes.
	if evs, err := s.m.pg.ListFindingEvidence(r.Context(), id); err == nil && len(evs) > 0 {
		var b strings.Builder
		b.WriteString(md)
		b.WriteString("\n\n## 结构化证据\n\n")
		for _, ev := range evs {
			fmt.Fprintf(&b, "### [%s] %s\n\n", ev.Kind, ev.Label)
			if ev.Note != "" {
				b.WriteString(ev.Note + "\n\n")
			}
			b.WriteString("```\n")
			b.WriteString(ev.Content)
			if !strings.HasSuffix(ev.Content, "\n") {
				b.WriteString("\n")
			}
			b.WriteString("```\n\n")
		}
		md = b.String()
	}
	writeJSON(w, 200, map[string]any{
		"template": template,
		"markdown": md,
		"filename": fmt.Sprintf("finding-%d-%s.md", id, template),
	})
}
