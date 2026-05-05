package skills

import (
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Skill struct {
	Name        string
	Description string
	Location    string
}

type frontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func ParseSkillMetadata(path, dirName string, data []byte) (Skill, bool, error) {
	content := string(data)
	if strings.TrimSpace(content) == "" {
		return Skill{}, false, nil
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return Skill{}, false, err
	}
	skill := Skill{
		Name:        dirName,
		Description: dirName,
		Location:    absPath,
	}

	fmText, _, hasFrontmatter, err := splitFrontmatter(content)
	if err != nil {
		return Skill{}, false, err
	}
	if !hasFrontmatter {
		return skill, true, nil
	}

	var fm frontmatter
	if err := yaml.Unmarshal([]byte(fmText), &fm); err != nil {
		return Skill{}, false, fmt.Errorf("parse frontmatter: %w", err)
	}
	if strings.TrimSpace(fm.Name) != "" && strings.TrimSpace(fm.Name) == dirName {
		skill.Name = strings.TrimSpace(fm.Name)
	}
	if strings.TrimSpace(fm.Description) != "" {
		skill.Description = strings.TrimSpace(fm.Description)
	}
	return skill, true, nil
}

func splitFrontmatter(content string) (string, string, bool, error) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(strings.TrimSuffix(lines[0], "\r")) != "---" {
		return "", "", false, nil
	}

	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(strings.TrimSuffix(lines[i], "\r")) == "---" {
			return strings.Join(lines[1:i], "\n"), strings.Join(lines[i+1:], "\n"), true, nil
		}
	}

	return "", "", false, fmt.Errorf("missing closing frontmatter delimiter")
}
