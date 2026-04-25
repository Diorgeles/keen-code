package filesearch

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/user/keen-code/internal/filesystem"
)

func newSearcher(t *testing.T, files []string) (*FileSearcher, string) {
	t.Helper()
	dir := t.TempDir()
	for _, f := range files {
		full := filepath.Join(dir, f)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte{}, 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	guard := filesystem.NewGuard(dir, nil)
	return NewFileSearcher(dir, guard), dir
}

func TestSearchEmptyQuery(t *testing.T) {
	s, _ := newSearcher(t, []string{"main.go"})
	if got := s.Search("", 10); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestSearchFindsSubstring(t *testing.T) {
	s, _ := newSearcher(t, []string{"main.go", "model.go", "middleware.go", "readme.md"})
	got := s.Search("main", 10)
	if len(got) != 1 || got[0] != "main.go" {
		t.Errorf("expected [main.go], got %v", got)
	}
}

func TestSearchNoMatch(t *testing.T) {
	s, _ := newSearcher(t, []string{"main.go", "model.go"})
	got := s.Search("xyz_nonexistent", 10)
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

func TestSearchLimitRespected(t *testing.T) {
	files := []string{"a1.go", "a2.go", "a3.go", "a4.go", "a5.go"}
	s, _ := newSearcher(t, files)
	got := s.Search("a", 3)
	if len(got) != 3 {
		t.Errorf("expected 3 results, got %d: %v", len(got), got)
	}
}

func TestSearchIncludesDirectories(t *testing.T) {
	s, _ := newSearcher(t, []string{"cmd/main.go", "cmd/other.go"})
	got := s.Search("cmd", 10)
	found := false
	for _, p := range got {
		if p == "cmd" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'cmd' directory in results, got %v", got)
	}
}

func TestSearchSpecialChars(t *testing.T) {
	s, _ := newSearcher(t, []string{"main.go"})
	// Should not panic or error; just return empty
	got := s.Search("[bracket]", 10)
	_ = got // no error expected
}

func TestSearchMultipleMatches(t *testing.T) {
	files := []string{"src/main.go", "src/model.go", "src/middleware.go"}
	s, _ := newSearcher(t, files)
	got := s.Search("m", 10)
	if len(got) < 3 {
		t.Errorf("expected at least 3 results for 'm', got %d: %v", len(got), got)
	}
}
