package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBundledFS_HasCommit(t *testing.T) {
	names := bundledNames()
	found := false
	for _, n := range names {
		if n == "commit" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected commit to be bundled, got %v", names)
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

	commitPath := filepath.Join(root, "commit", "SKILL.md")
	data, err := os.ReadFile(commitPath)
	if err != nil {
		t.Fatalf("expected %s: %v", commitPath, err)
	}
	if !strings.Contains(string(data), "name: commit") {
		t.Fatalf("commit SKILL.md missing frontmatter name, got %q", string(data))
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
	found := false
	for _, s := range result.Skills {
		if s.Name == "commit" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected bundled commit skill in discovery, got %#v", result.Skills)
	}
}
