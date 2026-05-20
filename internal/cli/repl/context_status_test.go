package repl

import (
	"strings"
	"testing"
)

func TestUsagePercent(t *testing.T) {
	if got := usagePercent(1000, 2000); got != 50.0 {
		t.Fatalf("usagePercent(1000, 2000) = %f, want 50", got)
	}
	if got := usagePercent(2500, 2000); got != 100.0 {
		t.Fatalf("usagePercent should clamp to 100, got %f", got)
	}
	if got := usagePercent(100, 0); got != 0.0 {
		t.Fatalf("usagePercent with zero context window should be 0, got %f", got)
	}
}

func TestRenderContextStatusUnknownWindow(t *testing.T) {
	got := renderContextStatus(contextStatus{KnownWindow: false, KnownTokens: true, CurrentTokens: 42})
	if !strings.Contains(got, "◷") {
		t.Fatalf("expected context glyph, got %q", got)
	}
	if !strings.Contains(got, "N/A") {
		t.Fatalf("expected N/A for unknown window, got %q", got)
	}
}

func TestRenderContextStatusUnknownTokens(t *testing.T) {
	got := renderContextStatus(contextStatus{KnownWindow: true, ContextWindow: 100000, KnownTokens: false})
	if !strings.Contains(got, "0.0%") {
		t.Fatalf("expected 0.0%% for unknown tokens, got %q", got)
	}
}

func TestRenderContextStatusKnownIncludesPercent(t *testing.T) {
	got := renderContextStatus(contextStatus{
		CurrentTokens: 1000,
		ContextWindow: 2000,
		Percent:       50.0,
		KnownWindow:   true,
		KnownTokens:   true,
	})
	if !strings.Contains(got, "50%") {
		t.Fatalf("expected percent in status, got %q", got)
	}
}

func TestRenderContextStatusKnownShowsTwoDecimalPlacesWhenNeeded(t *testing.T) {
	got := renderContextStatus(contextStatus{
		CurrentTokens: 1,
		ContextWindow: 3,
		Percent:       33.3333,
		KnownWindow:   true,
		KnownTokens:   true,
	})
	if !strings.Contains(got, "33.33%") {
		t.Fatalf("expected 33.33%% in status, got %q", got)
	}
}

func TestContextStatus_ShouldSuggestCompaction(t *testing.T) {
	if !(contextStatus{KnownWindow: true, KnownTokens: true, Percent: 70}).ShouldSuggestCompaction() {
		t.Fatal("expected compaction suggestion at 70%")
	}
	if (contextStatus{KnownWindow: true, KnownTokens: true, Percent: 69.99}).ShouldSuggestCompaction() {
		t.Fatal("did not expect compaction suggestion below 70%")
	}
	if (contextStatus{KnownWindow: false, KnownTokens: true, Percent: 90}).ShouldSuggestCompaction() {
		t.Fatal("did not expect compaction suggestion when context window is unknown")
	}
	if (contextStatus{KnownWindow: true, KnownTokens: false, Percent: 90}).ShouldSuggestCompaction() {
		t.Fatal("did not expect compaction suggestion when tokens are unknown")
	}
}

