package repl

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	reploutput "github.com/user/keen-code/internal/cli/repl/output"
	repltheme "github.com/user/keen-code/internal/cli/repl/theme"
	"github.com/user/keen-code/internal/tools"
)

const bashOutputMaxLines = 30

func (sh *StreamHandler) renderViewLines(width int) []string {
	lines := make([]string, 0)

	lastAssistantIdx := -1
	lastReasoningIdx := -1
	for i := range sh.segments {
		if sh.segments[i].kind == segmentAssistant {
			lastAssistantIdx = i
		}
		if sh.segments[i].kind == segmentReasoning {
			lastReasoningIdx = i
		}
	}

	for i := range sh.segments {
		seg := &sh.segments[i]
		switch seg.kind {
		case segmentToolStart:
			if seg.toolCall != nil {
				if i+1 < len(sh.segments) && sh.segments[i+1].kind == segmentToolEnd {
					continue
				}
				lines = append(lines, reploutput.FormatToolStart(seg.toolCall, sh.workingDir))
			}
		case segmentToolEnd:
			if seg.toolCall != nil {
				if i > 0 && sh.segments[i-1].kind == segmentToolStart && sh.segments[i-1].toolCall != nil {
					lines = append(lines, reploutput.FormatToolDone(sh.segments[i-1].toolCall, seg.toolCall, sh.workingDir))
				} else {
					lines = append(lines, reploutput.FormatToolEnd(seg.toolCall))
				}
			}
		case segmentBash:
			lines = append(lines, sh.renderBashSegment(seg, width)...)
		case segmentAssistant:
			if seg.renderedLines == nil || i == lastAssistantIdx {
				seg.renderedLines = sh.renderAssistantViewLines(seg.content, width)
			}
			lines = append(lines, seg.renderedLines...)
		case segmentReasoning:
			if seg.renderedLines == nil || i == lastReasoningIdx {
				seg.renderedLines = sh.renderReasoningViewLines(seg.content, width)
			}
			lines = append(lines, seg.renderedLines...)
		case segmentPermission:
			if seg.permissionReq != nil {
				lines = append(lines, renderPermissionCard(seg, width)...)
			}
		case segmentDiff:
			lines = append(lines, renderDiffSegment(seg, width)...)
		}
	}

	return lines
}

func (sh *StreamHandler) renderTranscriptLines() []string {
	lines := make([]string, 0)

	for i := range sh.segments {
		seg := &sh.segments[i]
		switch seg.kind {
		case segmentToolStart:
			if seg.toolCall != nil {
				if i+1 < len(sh.segments) && sh.segments[i+1].kind == segmentToolEnd {
					continue
				}
				lines = append(lines, reploutput.FormatToolStart(seg.toolCall, sh.workingDir))
			}
		case segmentToolEnd:
			if seg.toolCall != nil {
				if i > 0 && sh.segments[i-1].kind == segmentToolStart && sh.segments[i-1].toolCall != nil {
					lines = append(lines, reploutput.FormatToolDone(sh.segments[i-1].toolCall, seg.toolCall, sh.workingDir))
				} else {
					lines = append(lines, reploutput.FormatToolEnd(seg.toolCall))
				}
			}
		case segmentBash:
			lines = append(lines, sh.renderBashSegment(seg, 0)...)
		case segmentAssistant:
			lines = append(lines, sh.renderAssistantTranscriptLines(seg.content)...)
		case segmentReasoning:
			lines = append(lines, sh.renderReasoningTranscriptLines(seg.content)...)
		case segmentPermission:
			if seg.permissionReq != nil {
				lines = append(lines, renderPermissionResolved(seg.permissionReq)...)
			}
		case segmentDiff:
			lines = append(lines, renderDiffSegment(seg, sh.lastWidth)...)
		}
	}

	return lines
}

func (sh *StreamHandler) renderAssistantViewLines(content string, width int) []string {
	if content == "" {
		return nil
	}

	if sh.mdRenderer != nil {
		rendered := sh.mdRenderer.Render(content)
		if rendered == "" {
			return nil
		}
		rawLines := strings.Split(strings.TrimRight(rendered, "\n"), "\n")
		formatted := make([]string, 0, len(rawLines))
		for _, line := range rawLines {
			formatted = append(formatted, "  "+line)
		}
		return formatted
	}

	responseLines := strings.Split(content, "\n")
	wrapWidth := width - 4
	if wrapWidth < 1 {
		wrapWidth = 1
	}
	wrapStyle := lipgloss.NewStyle().Width(wrapWidth)
	formatted := make([]string, 0, len(responseLines))
	for _, line := range responseLines {
		formatted = append(formatted, "  "+wrapStyle.Render(repltheme.AssistantStyle.Render(line)))
	}
	return formatted
}

func (sh *StreamHandler) renderAssistantTranscriptLines(content string) []string {
	if content == "" {
		return nil
	}

	if sh.mdRenderer != nil {
		rendered := sh.mdRenderer.Render(content)
		if rendered == "" {
			return nil
		}
		rawLines := strings.Split(strings.TrimRight(rendered, "\n"), "\n")
		formatted := make([]string, 0, len(rawLines))
		for _, line := range rawLines {
			formatted = append(formatted, "  "+line)
		}
		return formatted
	}

	return formatResponseLines(content)
}

func (sh *StreamHandler) renderReasoningViewLines(content string, width int) []string {
	if content == "" {
		return nil
	}

	responseLines := strings.Split(content, "\n")
	wrapWidth := width - 4
	if wrapWidth < 1 {
		wrapWidth = 1
	}
	wrapStyle := lipgloss.NewStyle().Width(wrapWidth)
	formatted := make([]string, 0, len(responseLines))
	for _, line := range responseLines {
		formatted = append(formatted, "  "+wrapStyle.Render(repltheme.ReasoningStyle.Render(line)))
	}
	return formatted
}

func (sh *StreamHandler) renderReasoningTranscriptLines(content string) []string {
	if content == "" {
		return nil
	}

	lines := strings.Split(content, "\n")
	wrapWidth := sh.lastWidth - 4
	if wrapWidth < 1 {
		wrapWidth = 120
	}
	wrapStyle := lipgloss.NewStyle().Width(wrapWidth)

	result := make([]string, 0, len(lines))
	for _, line := range lines {
		result = append(result, "  "+wrapStyle.Render(repltheme.ReasoningStyle.Render(line)))
	}
	return result
}

func formatResponseLines(response string) []string {
	lines := strings.Split(response, "\n")
	result := make([]string, len(lines))
	for i, line := range lines {
		result[i] = "  " + line
	}
	return result
}

func (sh *StreamHandler) renderBashSegment(seg *streamSegment, width int) []string {
	ruleWidth := defaultWidth
	if width > 0 {
		ruleWidth = width
	}
	if ruleWidth < 1 {
		ruleWidth = 1
	}
	rule := repltheme.DiffRuleStyle.Render(strings.Repeat("─", ruleWidth))

	lines := make([]string, 0)

	lines = append(lines, "")
	lines = append(lines, rule)
	lines = append(lines, repltheme.BashCommandStyle.Render("  $ "+seg.command))

	if seg.summary != "" {
		lines = append(lines, repltheme.BashSummaryStyle.Render("  › "+seg.summary))
	}

	lines = append(lines, "")

	if seg.output != "" {
		outputLines := strings.Split(seg.output, "\n")
		total := len(outputLines)
		visible := outputLines
		if total > bashOutputMaxLines {
			visible = outputLines[:bashOutputMaxLines]
		}
		for _, line := range visible {
			if width > 0 {
				wrapStyle := lipgloss.NewStyle().Width(width - 4)
				lines = append(lines, "  "+repltheme.BashOutputStyle.Render(wrapStyle.Render(line)))
			} else {
				lines = append(lines, "  "+repltheme.BashOutputStyle.Render(line))
			}
		}
		if total > bashOutputMaxLines {
			accentStyle := lipgloss.NewStyle().Foreground(repltheme.AccentColor)
			lines = append(lines, "  "+accentStyle.Render(fmt.Sprintf("→ %d more lines", total-bashOutputMaxLines)))
		}
	}

	lines = append(lines, rule)

	return lines
}

func renderDiffLine(dl tools.EditDiffLine) string {
	switch dl.Kind {
	case tools.DiffLineHunk:
		return "  " + repltheme.DiffHunkStyle.Render(dl.Content)
	case tools.DiffLineAdded:
		lineNum := fmt.Sprintf("%4d", dl.NewLineNum)
		return repltheme.DiffLineNumStyle.Render("     "+lineNum) + " " + repltheme.DiffAddStyle.Render("+ "+dl.Content)
	case tools.DiffLineRemoved:
		lineNum := fmt.Sprintf("%4d", dl.OldLineNum)
		return repltheme.DiffLineNumStyle.Render(lineNum+"     ") + " " + repltheme.DiffRemoveStyle.Render("- "+dl.Content)
	default:
		return repltheme.DiffLineNumStyle.Render(fmt.Sprintf("%4d %4d", dl.OldLineNum, dl.NewLineNum)) + " " + repltheme.DiffContextStyle.Render("  "+dl.Content)
	}
}

func renderDiffSegment(seg *streamSegment, width int) []string {
	if len(seg.diffLines) == 0 {
		return nil
	}

	rendered := make([]string, 0, len(seg.diffLines))
	for _, dl := range seg.diffLines {
		rendered = append(rendered, renderDiffLine(dl))
	}

	ruleWidth := defaultWidth - 2
	if width > 0 {
		ruleWidth = width - 2
	}
	if ruleWidth < 1 {
		ruleWidth = 1
	}

	rule := "  " + repltheme.DiffRuleStyle.Render(strings.Repeat("─", ruleWidth))
	lines := make([]string, 0, len(rendered)+3)
	lines = append(lines, "")
	lines = append(lines, rule)
	lines = append(lines, rendered...)
	lines = append(lines, rule)
	return lines
}
