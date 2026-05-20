package markdown

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/glamour"
	repltheme "github.com/user/keen-code/internal/cli/repl/theme"
)

type Renderer struct {
	renderer *glamour.TermRenderer
	width    int
}

func wordWrapWidth(width int) int {
	wordWrap := width - 4
	if wordWrap < 1 {
		return 1
	}
	return wordWrap
}

func newGlamourRenderer(width int) (*glamour.TermRenderer, error) {
	wordWrap := wordWrapWidth(width)

	return glamour.NewTermRenderer(
		glamour.WithStyles(repltheme.MarkdownStyleConfig(wordWrap)),
		glamour.WithChromaFormatter("terminal256"),
		glamour.WithWordWrap(wordWrap),
		glamour.WithTableWrap(true),
		glamour.WithInlineTableLinks(true),
	)
}

func New(width int) (*Renderer, error) {
	renderer, err := newGlamourRenderer(width)
	if err != nil {
		return nil, err
	}

	return &Renderer{
		renderer: renderer,
		width:    width,
	}, nil
}

func (r *Renderer) Render(markdown string) string {
	if markdown == "" {
		return ""
	}

	rendered, err := r.renderer.Render(markdown)
	if err != nil {
		return markdown
	}
	return addTableOuterBorders(rendered)
}

func (r *Renderer) UpdateWidth(width int) error {
	if r.width == width {
		return nil
	}

	renderer, err := newGlamourRenderer(width)
	if err != nil {
		return err
	}

	r.renderer = renderer
	r.width = width
	return nil
}

func addTableOuterBorders(rendered string) string {
	lines := strings.Split(rendered, "\n")
	out := make([]string, 0, len(lines))

	for i := 0; i < len(lines); {
		if !isTableLine(lines[i]) {
			out = append(out, lines[i])
			i++
			continue
		}

		start := i
		hasSeparator := false
		for i < len(lines) && isTableLine(lines[i]) {
			hasSeparator = hasSeparator || isTableSeparatorLine(lines[i])
			i++
		}

		block := lines[start:i]
		if hasSeparator {
			out = append(out, renderTableBlockWithOuterBorders(block)...)
			continue
		}
		out = append(out, block...)
	}

	return strings.Join(out, "\n")
}

func renderTableBlockWithOuterBorders(lines []string) []string {
	if len(lines) == 0 {
		return lines
	}

	indent := tableIndent(lines)
	bodies := make([]string, len(lines))
	width := 0
	separator := ""
	for i, line := range lines {
		body := strings.TrimPrefix(line, indent)
		bodies[i] = body
		width = max(width, lipgloss.Width(body))
		if separator == "" && isTableSeparatorLine(line) {
			separator = body
		}
	}
	if separator == "" {
		separator = strings.Repeat("─", width)
	}
	separator = normalizeTableSeparator(separator, width)

	framed := make([]string, 0, len(lines)+2)
	framed = append(framed, indent+"┌"+strings.ReplaceAll(separator, "┼", "┬")+"┐")
	for i, body := range bodies {
		if isTableSeparatorLine(lines[i]) {
			framed = append(framed, indent+"├"+normalizeTableSeparator(body, width)+"┤")
			continue
		}
		framed = append(framed, indent+"│"+padTableBody(body, width)+"│")
		if nextTableLineIsBody(lines[i+1:]) {
			framed = append(framed, indent+"├"+separator+"┤")
		}
	}
	framed = append(framed, indent+"└"+strings.ReplaceAll(separator, "┼", "┴")+"┘")
	return framed
}

func nextTableLineIsBody(lines []string) bool {
	return len(lines) > 0 && isTableLine(lines[0]) && !isTableSeparatorLine(lines[0])
}

func isTableLine(line string) bool {
	return strings.Contains(line, "│") || isTableSeparatorLine(line)
}

func isTableSeparatorLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || !strings.Contains(trimmed, "─") || !strings.Contains(trimmed, "┼") {
		return false
	}
	return strings.Trim(trimmed, "─┼") == ""
}

func tableIndent(lines []string) string {
	indent := ""
	found := false
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		current := leadingSpaces(line)
		if !found || len(current) < len(indent) {
			indent = current
			found = true
		}
	}
	return indent
}

func leadingSpaces(line string) string {
	return line[:len(line)-len(strings.TrimLeft(line, " "))]
}

func padTableBody(value string, width int) string {
	if missing := width - lipgloss.Width(value); missing > 0 {
		return value + strings.Repeat(" ", missing)
	}
	return value
}

func normalizeTableSeparator(value string, width int) string {
	value = strings.ReplaceAll(value, " ", "─")
	if missing := width - lipgloss.Width(value); missing > 0 {
		return value + strings.Repeat("─", missing)
	}
	return value
}
