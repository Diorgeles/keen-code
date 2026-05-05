package llm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/user/keen-code/internal/skills"
)

func TestBuild_ContainsIdentity(t *testing.T) {
	dir := t.TempDir()
	result := Build(dir, "")
	if !strings.Contains(result, "Keen Code") {
		t.Error("expected output to contain 'Keen Code'")
	}
}

func TestBuild_ContainsWorkingDir(t *testing.T) {
	dir := t.TempDir()
	result := Build(dir, "")
	if !strings.Contains(result, dir) {
		t.Errorf("expected output to contain working dir %q", dir)
	}
}

func TestBuild_AgentsMd_Found(t *testing.T) {
	dir := t.TempDir()
	content := "## My Project\nSome instructions here."
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(content), 0644)

	result := Build(dir, "")
	if !strings.Contains(result, "# Project Instructions") {
		t.Error("expected project instructions section")
	}
	if !strings.Contains(result, "My Project") {
		t.Error("expected AGENTS.md content in output")
	}
}

func TestBuild_AgentsMd_WalkUp(t *testing.T) {
	parent := t.TempDir()
	child := filepath.Join(parent, "subdir")
	os.MkdirAll(child, 0755)
	os.WriteFile(filepath.Join(parent, "AGENTS.md"), []byte("parent instructions"), 0644)

	result := Build(child, "")
	if !strings.Contains(result, "parent instructions") {
		t.Error("expected AGENTS.md from parent directory")
	}
}

func TestBuild_ClaudeMd_Fallback(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("claude instructions"), 0644)

	result := Build(dir, "")
	if !strings.Contains(result, "claude instructions") {
		t.Error("expected CLAUDE.md content as fallback")
	}
}

func TestBuild_NoInstructionFile(t *testing.T) {
	dir := t.TempDir()
	result := Build(dir, "")
	if strings.Contains(result, "# Project Instructions") {
		t.Error("expected no project instructions section when no file exists")
	}
}

func TestBuild_AgentsMd_Truncation(t *testing.T) {
	dir := t.TempDir()
	content := strings.Repeat("x", 10*1024)
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(content), 0644)

	result := Build(dir, "")
	if !strings.Contains(result, "[truncated") {
		t.Error("expected truncation note for large AGENTS.md")
	}
}

func TestBuild_AgentsMd_Empty(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(""), 0644)

	result := Build(dir, "")
	if strings.Contains(result, "# Project Instructions") {
		t.Error("expected no project instructions for empty AGENTS.md")
	}
}

func TestBuild_FreshOnEachCall(t *testing.T) {
	dir := t.TempDir()
	result1 := Build(dir, "")
	result2 := Build(dir, "")
	if result1 != result2 {
		t.Error("expected identical output from two Build calls with same args")
	}
}

func TestBuild_IncludesSkillsCatalog(t *testing.T) {
	dir := t.TempDir()
	catalog := skills.Catalog([]skills.Skill{{Name: "demo", Description: "Demo skill", Location: "/tmp/demo/SKILL.md"}}, skills.Config{})

	result := Build(dir, catalog)
	if !strings.Contains(result, "## Available Skills") {
		t.Fatal("expected skills catalog")
	}
	if !strings.Contains(result, "- demo: Demo skill") {
		t.Fatalf("expected demo skill in catalog, got %q", result)
	}
}
