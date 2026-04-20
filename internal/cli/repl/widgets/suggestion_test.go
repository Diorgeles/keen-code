package widgets

import (
	"testing"

	replcommands "github.com/user/keen-code/internal/cli/repl/commands"
)

func TestFilterCommandsEmpty(t *testing.T) {
	if got := replcommands.Filter(""); len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

func TestFilterCommandsSlashOnly(t *testing.T) {
	got := replcommands.Filter("/")
	if len(got) != 8 {
		t.Fatalf("expected 8 commands, got %d", len(got))
	}
	if got[0].Name != "/clear" || got[1].Name != "/compact" || got[2].Name != "/exit" || got[3].Name != "/help" || got[4].Name != "/model" || got[5].Name != "/new" || got[6].Name != "/resume" || got[7].Name != "/sessions" {
		t.Errorf("unexpected order: %v", got)
	}
}

func TestFilterCommandsC(t *testing.T) {
	got := replcommands.Filter("/c")
	if len(got) != 2 || got[0].Name != "/clear" || got[1].Name != "/compact" {
		t.Errorf("expected /clear and /compact, got %v", got)
	}
}

func TestFilterCommandsH(t *testing.T) {
	got := replcommands.Filter("/h")
	if len(got) != 1 || got[0].Name != "/help" {
		t.Errorf("expected /help only, got %v", got)
	}
}

func TestFilterCommandsM(t *testing.T) {
	got := replcommands.Filter("/m")
	if len(got) != 1 || got[0].Name != "/model" {
		t.Errorf("expected /model only, got %v", got)
	}
}

func TestFilterCommandsE(t *testing.T) {
	got := replcommands.Filter("/e")
	if len(got) != 1 || got[0].Name != "/exit" {
		t.Errorf("expected /exit only, got %v", got)
	}
}

func TestFilterCommandsNoMatch(t *testing.T) {
	if got := replcommands.Filter("/xyz"); len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

func TestFilterCommandsCaseInsensitive(t *testing.T) {
	got := replcommands.Filter("/EXIT")
	if len(got) != 1 || got[0].Name != "/exit" {
		t.Errorf("expected /exit, got %v", got)
	}
}

func TestFilterCommandsExactMatch(t *testing.T) {
	got := replcommands.Filter("/help")
	if len(got) != 1 || got[0].Name != "/help" {
		t.Errorf("expected exactly /help, got %v", got)
	}
}

func TestSuggestionMoveDown(t *testing.T) {
	s := NewSuggestionModel()
	s.Refresh("/")
	s.selected = 0
	s.MoveDown()
	if s.selected != 1 {
		t.Errorf("expected 1, got %d", s.selected)
	}
	s.selected = len(s.items) - 1
	s.MoveDown()
	if s.selected != 0 {
		t.Errorf("expected wrap to 0, got %d", s.selected)
	}
}

func TestSuggestionMoveUp(t *testing.T) {
	s := NewSuggestionModel()
	s.Refresh("/")
	s.selected = 2
	s.MoveUp()
	if s.selected != 1 {
		t.Errorf("expected 1, got %d", s.selected)
	}
	s.selected = 0
	s.MoveUp()
	if s.selected != len(s.items)-1 {
		t.Errorf("expected wrap to %d, got %d", len(s.items)-1, s.selected)
	}
}

func TestSuggestionCurrentNilWhenInvisible(t *testing.T) {
	s := NewSuggestionModel()
	if s.Current() != nil {
		t.Error("expected nil when not visible")
	}
}

func TestSuggestionHeight(t *testing.T) {
	s := NewSuggestionModel()
	if s.Height() != 0 {
		t.Errorf("expected 0 when not visible, got %d", s.Height())
	}
	s.Refresh("/")
	if s.Height() != len(s.items)+2 {
		t.Errorf("expected %d, got %d", len(s.items)+2, s.Height())
	}
}

func TestSuggestionRefreshSlash(t *testing.T) {
	s := NewSuggestionModel()
	s.Refresh("/")
	if !s.visible {
		t.Error("expected visible after refresh('/')")
	}
	if len(s.items) == 0 {
		t.Error("expected items populated")
	}
}

func TestSuggestionRefreshEmpty(t *testing.T) {
	s := NewSuggestionModel()
	s.Refresh("/")
	s.Refresh("")
	if s.visible {
		t.Error("expected not visible after refresh('')")
	}
}
