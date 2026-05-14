package markdown

import (
	"regexp"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestRendererRenderPlainTextUsesTerminalForeground(t *testing.T) {
	renderer, err := New(80)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	rendered := renderer.Render("plain assistant response")
	if hasForegroundColorEscape(rendered) {
		t.Fatalf("expected no foreground color escapes, got %q", rendered)
	}
}

func TestRendererRenderCodeBlockUsesSyntaxHighlighting(t *testing.T) {
	renderer, err := New(80)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	rendered := renderer.Render("```go\nfunc main() {\n\tprintln(\"hello\")\n}\n```")
	if !hasForegroundColorEscape(rendered) {
		t.Fatalf("expected syntax foreground color escapes, got %q", rendered)
	}
}

func TestRendererRenderInlineCodeUsesColor(t *testing.T) {
	renderer, err := New(80)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	rendered := renderer.Render("Use `keen --help` for details.")
	if !hasForegroundColorEscape(rendered) {
		t.Fatalf("expected inline code foreground color escapes, got %q", rendered)
	}
}

func TestRendererRenderHorizontalRuleUsesContentWidth(t *testing.T) {
	tests := []struct {
		name      string
		width     int
		wantWidth int
	}{
		{
			name:      "wide",
			width:     80,
			wantWidth: 72,
		},
		{
			name:      "narrow",
			width:     24,
			wantWidth: 16,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderer, err := New(tt.width)
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			rendered := stripANSI(renderer.Render("before\n\n---\n\nafter"))
			rule := findLineContaining(rendered, "─")
			if rule == "" {
				t.Fatalf("expected horizontal rule, got %q", rendered)
			}

			trimmed := strings.TrimSpace(rule)
			if got := lipgloss.Width(trimmed); got != tt.wantWidth {
				t.Fatalf("expected rule width %d, got %d in %q", tt.wantWidth, got, rendered)
			}
			if strings.Trim(trimmed, "─") != "" {
				t.Fatalf("expected rule to contain only rule characters, got %q", trimmed)
			}
		})
	}
}

func TestRendererRenderTableUsesRulesAndColumns(t *testing.T) {
	renderer, err := New(80)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	rendered := stripANSI(renderer.Render("| Name | Status |\n| --- | --- |\n| Build | Passing |"))
	if !strings.Contains(rendered, "│") {
		t.Fatalf("expected table column separators, got %q", rendered)
	}
	if !strings.Contains(rendered, "┼") {
		t.Fatalf("expected table row separator intersections, got %q", rendered)
	}
	for _, want := range []string{"┌", "┬", "┐", "├", "┤", "└", "┴", "┘"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected table outer border %q, got %q", want, rendered)
		}
	}
}

func TestRendererRenderTableStaysWithinWidth(t *testing.T) {
	width := 40
	renderer, err := New(width)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	rendered := stripANSI(renderer.Render("| Name | Description |\n| --- | --- |\n| Alpha | This description is deliberately long so it must wrap within the markdown viewport. |"))
	for _, line := range strings.Split(rendered, "\n") {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("expected line width <= %d, got %d for %q in %q", width, got, line, rendered)
		}
	}
}

func TestRendererRenderTableOuterBordersDoNotLeaveRightGap(t *testing.T) {
	renderer, err := New(80)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	rendered := stripANSI(renderer.Render("| ID | Name | Score | Status |\n| --- | --- | --- | --- |\n| 1 | Alice | 87 | Active |\n| 2 | Bob | 92 | Active |\n| 3 | Charlie | 78 | Pending |\n| 4 | Diana | 95 | Active |\n| 5 | Edward | 64 | Inactive |\n| 6 | Fiona | 88 | Active |\n| 7 | George | 71 | Pending |\n| 8 | Hannah | 99 | Active |"))
	for _, line := range strings.Split(rendered, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "┌") && !strings.HasPrefix(trimmed, "├") && !strings.HasPrefix(trimmed, "└") {
			continue
		}
		if strings.Contains(trimmed, " ┐") || strings.Contains(trimmed, " ┤") || strings.Contains(trimmed, " ┘") {
			t.Fatalf("expected border line without right gap, got %q in %q", line, rendered)
		}
	}
}

var ansiEscapePattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(value string) string {
	return ansiEscapePattern.ReplaceAllString(value, "")
}

func findLineContaining(value, needle string) string {
	for _, line := range strings.Split(value, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return ""
}

func hasForegroundColorEscape(value string) bool {
	return strings.Contains(value, "\x1b[38;") ||
		strings.Contains(value, "\x1b[30m") ||
		strings.Contains(value, "\x1b[31m") ||
		strings.Contains(value, "\x1b[32m") ||
		strings.Contains(value, "\x1b[33m") ||
		strings.Contains(value, "\x1b[34m") ||
		strings.Contains(value, "\x1b[35m") ||
		strings.Contains(value, "\x1b[36m") ||
		strings.Contains(value, "\x1b[37m") ||
		strings.Contains(value, "\x1b[90m") ||
		strings.Contains(value, "\x1b[91m") ||
		strings.Contains(value, "\x1b[92m") ||
		strings.Contains(value, "\x1b[93m") ||
		strings.Contains(value, "\x1b[94m") ||
		strings.Contains(value, "\x1b[95m") ||
		strings.Contains(value, "\x1b[96m") ||
		strings.Contains(value, "\x1b[97m") ||
		strings.Contains(value, "\x1b[39m")
}
