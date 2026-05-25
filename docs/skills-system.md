# Skills System

Skills extend Keen Code's capabilities through specialized instruction sets that can be activated during a conversation.

## Skill Structure

Each skill is a directory containing a `SKILL.md` file:

```
skills/
└── my-skill/
    └── SKILL.md
```

### SKILL.md Format

Skills use YAML frontmatter followed by markdown content:

```markdown
---
name: my-skill
description: Does something useful
---

Instructions for the skill...

Reference arguments with $ARGUMENTS, $1, $2, etc.
```

Required frontmatter:
- `name`: Unique identifier for the skill
- `description`: Brief description shown in the catalog

## Skill Discovery

Skills are discovered from multiple locations (`internal/skills/discover.go`):

```go
func discoveryRoots(workingDir, bundledDir string) []string {
    roots := []string{
        filepath.Join(workingDir, ".agents", "skills"),
        filepath.Join(workingDir, ".keen", "skills"),
        filepath.Join(workingDir, ".claude", "skills"),
        filepath.Join(home, ".agents", "skills"),
        filepath.Join(home, ".keen", "skills"),
        filepath.Join(home, ".claude", "skills"),
    }
    if bundledDir != "" {
        roots = append(roots, bundledDir)
    }
    return roots
}
```

Priority order:
1. `<working-dir>/.agents/skills/`
2. `<working-dir>/.keen/skills/`
3. `<working-dir>/.claude/skills/`
4. `~/.agents/skills/`
5. `~/.keen/skills/`
6. `~/.claude/skills/`
7. Bundled skills directory

## Skill Metadata Parsing

```go
// internal/skills/parse.go
type Skill struct {
    Name        string
    Description string
    Location    string
}

func ParseSkillMetadata(path string, data []byte) (Skill, error)
```

The parser:
1. Splits frontmatter from content (delimited by `---`)
2. Parses YAML frontmatter
3. Validates required fields
4. Returns absolute path for the skill

## Skill Configuration

```go
// internal/skills/config.go
type Config struct {
    IsEnabled map[string]bool `json:"is_enabled"`
}
```

Config file: `~/.keen/skills/config.json`

- Skills are enabled by default if not in the config
- Use `Config.Enabled(name)` to check if a skill is enabled
- Use `Config.SetEnabled(name, enabled)` to modify enabled state

## Bundled Skills

Bundled skills are embedded in the binary using Go's `embed` directive:

```go
//go:embed all:bundled
var bundledFS embed.FS
```

`EnsureBundled()` extracts them to `~/.keen/skills/bundled/` on each startup to ensure the binary's version is always available.

## Skill Catalog

The `Catalog()` function generates a markdown catalog for the system prompt:

```go
func Catalog(all []Skill, cfg Config) string
```

Output format:
```markdown
## Available Skills

You have access to specialized skills. To activate a skill, use the read_file tool
to read the skill's SKILL.md file at one of these paths, then follow the instructions
within...

- skill-name: Description → /path/to/SKILL.md

IMPORTANT: If any user message in this conversation begins with
`[Activate skill: <name>]`, the SKILL.md body for that skill has already been
provided inline in that message — do not call read_file on its path.
```

## Skill Activation

To activate a skill, the LLM reads the SKILL.md file. The content is injected as a user message prefixed with:

```
[Activate skill: <name>]

<SKILL.md content>
```

### Argument Substitution

Arguments passed during activation are substituted into the skill body:

- `$ARGUMENTS` → all arguments joined by space
- `$1`, `$2`, etc. → individual arguments

```go
// internal/skills/discover.go
var argPlaceholder = regexp.MustCompile(`\$ARGUMENTS\b|\$([1-9])\b`)

func substituteArgs(body string, args []string) string {
    return argPlaceholder.ReplaceAllStringFunc(body, func(match string) string {
        if match == "$ARGUMENTS" {
            return strings.Join(args, " ")
        }
        idx := int(match[1]-'0') - 1
        if idx < len(args) {
            return args[idx]
        }
        return ""
    })
}
```

## Discovery and Loading

The discovery process:

```go
func Discover(workingDir, bundledDir string) Discovery
func LoadMetadata(discovery Discovery) Discovery
```

1. `Discover()` finds all `SKILL.md` files across discovery roots
2. `LoadMetadata()` reads and parses each skill's frontmatter
3. Duplicate names are reported as warnings

```go
type Discovery struct {
    Skills   []Skill
    Warnings []string
}
```

## Finding Skills

```go
func Find(skills []Skill, name string) (Skill, bool)
```

Look up a skill by name from the discovered list.

## Example Skill Structure

```
~/.keen/skills/
├── git-helper/
│   └── SKILL.md
├── code-review/
│   └── SKILL.md
└── test-generator/
    └── SKILL.md
```

With content:

```markdown
---
name: git-helper
description: Assists with git operations
---

You are a git assistant. When the user asks about git, help them with:
- Creating commits (but prompt before executing dangerous ops)
- Viewing history
- Managing branches

Use the bash tool for git operations.
```