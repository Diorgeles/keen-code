package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Discovery struct {
	Skills   []Skill
	Warnings []string
}

func Discover(workingDir string) Discovery {
	var result Discovery
	loaded := map[string]Skill{}

	for _, root := range discoveryRoots(workingDir) {
		matches, err := filepath.Glob(filepath.Join(root, "*", "SKILL.md"))
		if err != nil {
			continue
		}
		sort.Strings(matches)
		for _, skillPath := range matches {
			name := filepath.Base(filepath.Dir(skillPath))
			if _, exists := loaded[name]; exists {
				continue
			}

			absPath, err := filepath.Abs(skillPath)
			if err != nil {
				continue
			}
			loaded[name] = Skill{
				Name:        name,
				Description: name,
				Location:    absPath,
			}
		}
	}

	for _, skill := range loaded {
		result.Skills = append(result.Skills, skill)
	}
	sort.Slice(result.Skills, func(i, j int) bool {
		return result.Skills[i].Name < result.Skills[j].Name
	})
	return result
}

func LoadMetadata(discovery Discovery) Discovery {
	result := Discovery{Warnings: append([]string(nil), discovery.Warnings...)}
	result.Skills = make([]Skill, 0, len(discovery.Skills))

	for _, discovered := range discovery.Skills {
		data, err := os.ReadFile(discovered.Location)
		if err != nil {
			result.Skills = append(result.Skills, discovered)
			continue
		}
		skill, ok, err := ParseSkillMetadata(discovered.Location, discovered.Name, data)
		if err != nil {
			result.Warnings = append(result.Warnings, "Skill "+discovered.Name+" failed to load due to YAML parsing issue")
			continue
		}
		if !ok {
			continue
		}
		result.Skills = append(result.Skills, skill)
	}

	sort.Slice(result.Skills, func(i, j int) bool {
		return result.Skills[i].Name < result.Skills[j].Name
	})
	return result
}

func Catalog(all []Skill, cfg Config) string {
	enabled := make([]Skill, 0, len(all))
	for _, skill := range all {
		if cfg.Enabled(skill.Name) {
			enabled = append(enabled, skill)
		}
	}
	if len(enabled) == 0 {
		return ""
	}

	sort.Slice(enabled, func(i, j int) bool {
		return enabled[i].Name < enabled[j].Name
	})

	var sb strings.Builder
	sb.WriteString("## Available Skills\n\n")
	sb.WriteString(`You have access to specialized skills. To activate a skill, use the read_file tool
to read the skill's SKILL.md file at one of these paths, then follow the instructions
within. Resolve relative paths in skill instructions against the skill directory
containing SKILL.md:
`)
	for _, skill := range enabled {
		sb.WriteString("- ")
		sb.WriteString(skill.Name)
		sb.WriteString(": ")
		sb.WriteString(skill.Description)
		sb.WriteString(" → read ")
		sb.WriteString(skill.Location)
		sb.WriteString("\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

func discoveryRoots(workingDir string) []string {
	roots := []string{filepath.Join(workingDir, ".agents", "skills")}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		roots = append(roots,
			filepath.Join(home, ".agents", "skills"),
			filepath.Join(home, ".keen", "skills"),
		)
	}
	return roots
}

func Find(skills []Skill, name string) (Skill, bool) {
	for _, skill := range skills {
		if skill.Name == name {
			return skill, true
		}
	}
	return Skill{}, false
}

func ActivationMessage(skill Skill, args []string) (string, error) {
	content, err := os.ReadFile(skill.Location)
	if err != nil {
		return "", fmt.Errorf("read skill %s: %w", skill.Name, err)
	}

	var sb strings.Builder
	sb.WriteString("[Activate skill: ")
	sb.WriteString(skill.Name)
	sb.WriteString("]")
	if len(args) > 0 {
		sb.WriteString("\nArguments: ")
		sb.WriteString(strings.Join(args, " "))
	}
	sb.WriteString("\n\n")
	sb.WriteString(strings.TrimSpace(string(content)))
	return sb.String(), nil
}
