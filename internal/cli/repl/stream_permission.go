package repl

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

const permissionPreviewMaxLines = 120

func (sh *StreamHandler) HandlePermissionRequest(req *PermissionRequest) {
	sh.segments = append(sh.segments, streamSegment{
		kind:          segmentPermission,
		permissionReq: req,
	})
}

func (sh *StreamHandler) HasPendingPermission() bool {
	n := len(sh.segments)
	if n == 0 {
		return false
	}
	seg := &sh.segments[n-1]
	return seg.kind == segmentPermission &&
		seg.permissionReq != nil &&
		seg.permissionReq.Status == PermissionStatusPending
}

func (sh *StreamHandler) MovePendingCursor(delta int) {
	n := len(sh.segments)
	if n == 0 {
		return
	}
	seg := &sh.segments[n-1]
	if seg.kind != segmentPermission || seg.permissionReq == nil {
		return
	}
	choices := permissionChoices(seg.permissionReq.IsDangerous)
	newCursor := seg.permissionCursor + delta
	if newCursor < 0 {
		newCursor = 0
	}
	if newCursor >= len(choices) {
		newCursor = len(choices) - 1
	}
	seg.permissionCursor = newCursor
}

func (sh *StreamHandler) GetPendingChoice() PermissionChoice {
	n := len(sh.segments)
	if n == 0 {
		return PermissionChoiceDeny
	}
	seg := &sh.segments[n-1]
	if seg.kind != segmentPermission || seg.permissionReq == nil {
		return PermissionChoiceDeny
	}
	return permissionChoiceAt(seg.permissionCursor, seg.permissionReq.IsDangerous)
}

func (sh *StreamHandler) GetPendingPermissionRequest() *PermissionRequest {
	n := len(sh.segments)
	if n == 0 {
		return nil
	}
	seg := &sh.segments[n-1]
	if seg.kind != segmentPermission || seg.permissionReq == nil {
		return nil
	}
	return seg.permissionReq
}

func (sh *StreamHandler) ResolvePendingPermission(status PermissionStatus) {
	n := len(sh.segments)
	if n == 0 {
		return
	}
	seg := &sh.segments[n-1]
	if seg.kind != segmentPermission || seg.permissionReq == nil {
		return
	}
	seg.permissionReq.Status = status
	seg.renderedLines = nil
}

func renderPermissionCard(seg *streamSegment, width int) []string {
	req := seg.permissionReq
	if req == nil {
		return nil
	}

	if req.Status != PermissionStatusPending {
		return renderPermissionResolved(req)
	}

	cardWidth := width - 4
	if cardWidth < 20 {
		cardWidth = 20
	}
	cardStyle := userPromptCardStyle.MaxWidth(cardWidth)
	contentWidth := cardWidth - cardStyle.GetHorizontalFrameSize()
	if contentWidth < 1 {
		contentWidth = 1
	}

	labelWidth := lipgloss.Width(infoLabelStyle.Render("Resolved:"))
	if labelWidth < 1 {
		labelWidth = 1
	}
	valueWidth := contentWidth - labelWidth - 1
	if valueWidth < 1 {
		valueWidth = 1
	}

	var sb strings.Builder

	if req.IsDangerous {
		sb.WriteString(warningTitleStyle.Render("⚠  Allow Dangerous Command?"))
	} else {
		sb.WriteString(userPromptStyle.Render("Permission Required"))
	}
	sb.WriteString("\n\n")

	sb.WriteString(formatPermissionKeyValue("Tool:", req.ToolName, labelWidth, valueWidth))
	if req.IsDangerous {
		sb.WriteString(formatPermissionKeyValue("Command:", req.Path, labelWidth, valueWidth))
	} else {
		sb.WriteString(formatPermissionKeyValue("Path:", req.Path, labelWidth, valueWidth))
		if req.ResolvedPath != "" {
			sb.WriteString(formatPermissionKeyValue("Resolved:", req.ResolvedPath, labelWidth, valueWidth))
		}
	}

	if req.Preview != "" {
		previewStyle := lipgloss.NewStyle().Foreground(mutedColor)
		previewLines := strings.Split(req.Preview, "\n")
		total := len(previewLines)
		truncated := total > permissionPreviewMaxLines
		if truncated {
			previewLines = previewLines[:permissionPreviewMaxLines]
		}
		sb.WriteString("\n")
		for _, l := range previewLines {
			sb.WriteString(wrapTextWithStyle(l, previewStyle, contentWidth))
			sb.WriteString("\n")
		}
		if truncated {
			sb.WriteString(wrapTextWithStyle(fmt.Sprintf("... %d more preview lines omitted", total-permissionPreviewMaxLines), hintStyle, contentWidth))
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n")

	choices := permissionChoices(req.IsDangerous)
	for i, choice := range choices {
		if i == seg.permissionCursor {
			sb.WriteString(wrapTextWithStyle("> "+choice, userPromptSelectionStyle, contentWidth))
			sb.WriteString("\n")
		} else {
			sb.WriteString(wrapTextWithStyle("  "+choice, normalStyle, contentWidth))
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n")
	sb.WriteString(wrapTextWithStyle("[↑/↓ navigate  Enter confirm  Esc deny]", hintStyle, contentWidth))

	boxed := cardStyle.Render(sb.String())
	rawLines := strings.Split(strings.TrimRight(boxed, "\n"), "\n")
	result := make([]string, 0, len(rawLines)+1)
	result = append(result, "")
	for _, l := range rawLines {
		result = append(result, "  "+l)
	}
	return result
}

func wrapTextWithStyle(text string, style lipgloss.Style, width int) string {
	if width < 1 {
		width = 1
	}
	return lipgloss.NewStyle().Width(width).Render(style.Render(text))
}

func formatPermissionKeyValue(label, value string, labelWidth, valueWidth int) string {
	if labelWidth < 1 {
		labelWidth = 1
	}
	if valueWidth < 1 {
		valueWidth = 1
	}

	prefix := infoLabelStyle.Width(labelWidth).Render(label)
	continuation := strings.Repeat(" ", labelWidth+1)
	if value == "" {
		return prefix + " \n"
	}

	wrapped := wrapTextWithStyle(value, infoValueStyle, valueWidth)
	lines := strings.Split(strings.TrimRight(wrapped, "\n"), "\n")
	if len(lines) == 0 {
		return prefix + " \n"
	}

	var out strings.Builder
	out.WriteString(prefix + " " + lines[0] + "\n")
	for _, line := range lines[1:] {
		out.WriteString(continuation + line + "\n")
	}
	return out.String()
}

func renderPermissionResolved(req *PermissionRequest) []string {
	var line string
	switch req.Status {
	case PermissionStatusAllowed:
		line = "  " + highlightStyle.Render("✓ Permission granted for "+req.ToolName)
	case PermissionStatusAllowedSession:
		line = "  " + highlightStyle.Render("✓ Permission granted for "+req.ToolName+" (this session)")
	case PermissionStatusDenied:
		line = "  " + lipgloss.NewStyle().Foreground(mutedColor).Render("✗ Permission denied for "+req.ToolName)
	case PermissionStatusAutoAllowedSession:
		line = "  " + highlightStyle.Render("✓ Auto-approved for "+req.ToolName+" (session)")
	default:
		line = "  " + lipgloss.NewStyle().Foreground(mutedColor).Render("✗ Permission cancelled for "+req.ToolName)
	}
	return []string{line}
}
