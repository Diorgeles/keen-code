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
const diffRightPadding = 2

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
	rule := repltheme.RuleStyle.Render(strings.Repeat("─", ruleWidth))

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

func renderWrappedDiffLine(prefix string, content string, contentStyle lipgloss.Style, width int) []string {
	renderedPrefix := prefix
	if width <= 0 {
		return []string{renderedPrefix + contentStyle.Render(content)}
	}

	contentWidth := width - lipgloss.Width(renderedPrefix) - diffRightPadding
	if contentWidth < 1 {
		contentWidth = 1
	}

	wrapped := lipgloss.NewStyle().Width(contentWidth).Render(contentStyle.Render(content))
	wrappedLines := strings.Split(strings.TrimRight(wrapped, "\n"), "\n")
	if len(wrappedLines) == 0 {
		return []string{renderedPrefix}
	}

	lines := make([]string, 0, len(wrappedLines))
	lines = append(lines, renderedPrefix+wrappedLines[0])

	continuationPrefix := strings.Repeat(" ", lipgloss.Width(renderedPrefix))
	for _, line := range wrappedLines[1:] {
		lines = append(lines, continuationPrefix+line)
	}

	return lines
}

func renderDiffLines(dl tools.EditDiffLine, width int) []string {
	switch dl.Kind {
	case tools.DiffLineHunk:
		return renderWrappedDiffLine("  ", dl.Content, repltheme.DiffHunkStyle, width)
	case tools.DiffLineAdded:
		lineNum := fmt.Sprintf("%4d", dl.NewLineNum)
		prefix := repltheme.DiffLineNumStyle.Render("     "+lineNum) + " " + repltheme.DiffAddStyle.Render("+ ")
		return renderWrappedDiffLine(prefix, dl.Content, repltheme.DiffAddStyle, width)
	case tools.DiffLineRemoved:
		lineNum := fmt.Sprintf("%4d", dl.OldLineNum)
		prefix := repltheme.DiffLineNumStyle.Render(lineNum+"     ") + " " + repltheme.DiffRemoveStyle.Render("- ")
		return renderWrappedDiffLine(prefix, dl.Content, repltheme.DiffRemoveStyle, width)
	default:
		prefix := repltheme.DiffLineNumStyle.Render(fmt.Sprintf("%4d %4d", dl.OldLineNum, dl.NewLineNum)) + " " + repltheme.DiffContextStyle.Render("  ")
		return renderWrappedDiffLine(prefix, dl.Content, repltheme.DiffContextStyle, width)
	}
}

func renderDiffSegment(seg *streamSegment, width int) []string {
	if len(seg.diffLines) == 0 {
		return nil
	}

	rendered := make([]string, 0, len(seg.diffLines))
	for _, dl := range seg.diffLines {
		rendered = append(rendered, renderDiffLines(dl, width)...)
	}

	ruleWidth := defaultWidth - 2 - diffRightPadding
	if width > 0 {
		ruleWidth = width - 2 - diffRightPadding
	}
	if ruleWidth < 1 {
		ruleWidth = 1
	}

	rule := "  " + repltheme.RuleStyle.Render(strings.Repeat("─", ruleWidth))
	lines := make([]string, 0, len(rendered)+3)
	lines = append(lines, "")
	lines = append(lines, rule)
	lines = append(lines, rendered...)
	lines = append(lines, rule)
	return lines
}
