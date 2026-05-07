package filesearch

import (
	"os"
	"os/exec"
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

func TestGitLsFilesFiltersBlockedPaths(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	writeFile(t, dir, ".gitignore", "secret.txt\n")
	writeFile(t, dir, "secret.txt", "secret")
	writeFile(t, dir, "visible.txt", "visible")
	runGit(t, dir, "init")
	runGit(t, dir, "add", "-f", "secret.txt")

	ga := filesystem.NewGitAwareness()
	if err := ga.LoadGitignore(filepath.Join(dir, ".gitignore")); err != nil {
		t.Fatalf("load gitignore: %v", err)
	}
	guard := filesystem.NewGuard(dir, ga)

	paths, ok := gitLsFiles(dir, guard)
	if !ok {
		t.Fatal("expected git ls-files to succeed")
	}
	for _, path := range paths {
		if path == "secret.txt" {
			t.Fatalf("expected blocked tracked file to be filtered, got %v", paths)
		}
	}
	if !containsPath(paths, "visible.txt") {
		t.Fatalf("expected unblocked file in results, got %v", paths)
	}
}

func TestGitLsFilesHandlesSpecialPaths(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	pathWithNewline := "dir/file\nname.txt"
	pathWithSpaces := " dir/spaced name.txt "
	writeFile(t, dir, pathWithNewline, "newline")
	writeFile(t, dir, pathWithSpaces, "spaces")
	runGit(t, dir, "init")

	paths, ok := gitLsFiles(dir, filesystem.NewGuard(dir, nil))
	if !ok {
		t.Fatal("expected git ls-files to succeed")
	}
	if !containsPath(paths, pathWithNewline) {
		t.Fatalf("expected newline path in results, got %q", paths)
	}
	if !containsPath(paths, pathWithSpaces) {
		t.Fatalf("expected space-padded path in results, got %q", paths)
	}
}

func writeFile(t *testing.T, dir, path, content string) {
	t.Helper()
	full := filepath.Join(dir, path)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func containsPath(paths []string, want string) bool {
	for _, path := range paths {
		if path == want {
			return true
		}
	}
	return false
}
