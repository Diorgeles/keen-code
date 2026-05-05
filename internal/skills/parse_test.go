package skills

import (
	"path/filepath"
	"testing"
)

func TestParseSkillMetadata_WithFrontmatter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "demo", "SKILL.md")
	skill, ok, err := ParseSkillMetadata(path, "demo", []byte("---\nname: demo\ndescription: Demo skill\n---\nBody"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected skill to load")
	}
	if skill.Name != "demo" || skill.Description != "Demo skill" {
		t.Fatalf("unexpected skill: %#v", skill)
	}
	if !filepath.IsAbs(skill.Location) {
		t.Fatalf("expected absolute location, got %q", skill.Location)
	}
}

func TestParseSkillMetadata_NoFrontmatterUsesFallbacks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "demo", "SKILL.md")
	skill, ok, err := ParseSkillMetadata(path, "demo", []byte("plain markdown"))
	if err != nil || !ok {
		t.Fatalf("ParseSkillMetadata() ok=%v err=%v", ok, err)
	}
	if skill.Name != "demo" || skill.Description != "demo" {
		t.Fatalf("unexpected skill: %#v", skill)
	}
}

func TestParseSkillMetadata_EmptySkipped(t *testing.T) {
	_, ok, err := ParseSkillMetadata(filepath.Join(t.TempDir(), "SKILL.md"), "demo", []byte(" \n\t"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected empty skill to be skipped")
	}
}

func TestParseSkillMetadata_InvalidYAML(t *testing.T) {
	_, _, err := ParseSkillMetadata(filepath.Join(t.TempDir(), "SKILL.md"), "demo", []byte("---\nname: [\n---\nBody"))
	if err == nil {
		t.Fatal("expected YAML parse error")
	}
}

func TestParseSkillMetadata_MissingFieldsFallback(t *testing.T) {
	skill, ok, err := ParseSkillMetadata(filepath.Join(t.TempDir(), "SKILL.md"), "demo", []byte("---\nother: field\n---\nBody"))
	if err != nil || !ok {
		t.Fatalf("ParseSkillMetadata() ok=%v err=%v", ok, err)
	}
	if skill.Name != "demo" || skill.Description != "demo" {
		t.Fatalf("expected fallbacks, got %#v", skill)
	}
}
