package repl

import (
	"fmt"
	"strings"

	"github.com/user/keen-code/internal/session"
)

type SessionPicker struct {
	summaries []session.Summary
	cursor    int
}

func NewSessionPicker(summaries []session.Summary) *SessionPicker {
	return &SessionPicker{summaries: summaries}
}

func (p *SessionPicker) Move(delta int) {
	if p == nil || len(p.summaries) == 0 {
		return
	}

	p.cursor += delta
	if p.cursor < 0 {
		p.cursor = 0
	}
	if p.cursor >= len(p.summaries) {
		p.cursor = len(p.summaries) - 1
	}
}

func (p *SessionPicker) Current() *session.Summary {
	if p == nil || len(p.summaries) == 0 {
		return nil
	}
	return &p.summaries[p.cursor]
}

func formatSessionPickerCard(picker *SessionPicker) string {
	if picker == nil {
		return ""
	}

	var body strings.Builder
	body.WriteString(userPromptStyle.Render("Saved Sessions"))
	body.WriteString("\n\n")

	for i, summary := range picker.summaries {
		prefix := "  "
		style := normalStyle
		if i == picker.cursor {
			prefix = "> "
			style = userPromptSelectionStyle
		}

		preview := strings.TrimSpace(summary.LastUserMessage)
		if preview == "" {
			preview = "(no user message)"
		}
		if len(preview) > 72 {
			preview = preview[:69] + "..."
		}

		body.WriteString(style.Render(prefix + preview))
		body.WriteString("\n")
		body.WriteString(timestampStyle.Render(fmt.Sprintf(
			"    Created: %s   Updated: %s",
			summary.CreatedAt.Local().Format("2006-01-02 15:04"),
			summary.UpdatedAt.Local().Format("2006-01-02 15:04"),
		)))
		body.WriteString("\n\n")
	}

	body.WriteString(hintStyle.Render("[↑/↓ navigate  Enter to resume  Esc to cancel]"))

	boxed := userPromptCardStyle.Render(body.String())
	lines := strings.Split(strings.TrimRight(boxed, "\n"), "\n")

	var out strings.Builder
	out.WriteString("\n")
	for _, line := range lines {
		out.WriteString("  " + line + "\n")
	}
	return out.String()
}
