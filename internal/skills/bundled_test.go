package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func expectedBundledSkillNames() []string {
	return []string{
		"clarify",
		"cleanup",
		"commit",
		"debug",
		"explain",
		"fix-tests",
		"plan",
		"refactor",
		"review",
	}
}

func TestBundledFS_HasExpectedSkills(t *testing.T) {
	names := bundledNames()
	found := make(map[string]bool, len(names))
	for _, n := range names {
		found[n] = true
	}
	for _, want := range expectedBundledSkillNames() {
		if !found[want] {
			t.Fatalf("expected %s to be bundled, got %v", want, names)
		}
	}
}

func TestEnsureBundled_WritesFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	root, err := EnsureBundled()
	if err != nil {
		t.Fatalf("EnsureBundled error: %v", err)
	}
	wantRoot := filepath.Join(home, ".keen", "skills", "bundled")
	if root != wantRoot {
		t.Fatalf("root: got %q want %q", root, wantRoot)
	}

	for _, name := range expectedBundledSkillNames() {
		path := filepath.Join(root, name, "SKILL.md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("expected %s: %v", path, err)
		}
		if !strings.Contains(string(data), "name: "+name) {
			t.Fatalf("%s SKILL.md missing frontmatter name, got %q", name, string(data))
		}
	}
}

func TestEnsureBundled_OverwritesExistingFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	target := filepath.Join(home, ".keen", "skills", "bundled", "commit", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("STALE"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := EnsureBundled(); err != nil {
		t.Fatalf("EnsureBundled error: %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == "STALE" {
		t.Fatal("expected EnsureBundled to overwrite stale file")
	}
}

func TestEnsureBundled_DiscoveryPicksUpBundled(t *testing.T) {
	home := t.TempDir()
	work := t.TempDir()
	t.Setenv("HOME", home)

	bundledDir, err := EnsureBundled()
	if err != nil {
		t.Fatalf("EnsureBundled: %v", err)
	}
	result := LoadMetadata(Discover(work, bundledDir))
	found := make(map[string]bool, len(result.Skills))
	for _, s := range result.Skills {
		found[s.Name] = true
	}
	for _, want := range expectedBundledSkillNames() {
		if !found[want] {
			t.Fatalf("expected bundled %s skill in discovery, got %#v", want, result.Skills)
		}
	}
}
