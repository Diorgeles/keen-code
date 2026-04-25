package markdown

import (
	"strings"
	"testing"
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
