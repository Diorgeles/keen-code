# RFC: Agent Skills Support in Keen Code

## Summary

Add support for the [Agent Skills open standard](https://agentskills.io/client-implementation/adding-skills-support.md) in Keen Code. Skills are markdown files with YAML frontmatter that extend the agent's capabilities. They follow **progressive disclosure**: a lightweight catalog in the system prompt, full instructions loaded on demand via `read_file`, and resources referenced as needed.

---

## Discovery

Keen scans two directories on session start:

| Level | Path |
|-------|------|
| Project | `<working_dir>/.agents/skills/` |
| Global | `~/.agents/skills/` |

Each subdirectory containing a `SKILL.md` file is a skill. The directory name is the skill name.

**Collision resolution:** If the same skill name exists at both levels, the project-level skill wins.

**Live reload:** Since `Build()` scans skill directories on every call, catalog changes
(add, remove, rename) are reflected on the next message. Content changes are always
fresh since the model reads `SKILL.md` via `read_file` at invocation time.

---

## Parsing

Each `SKILL.md` is parsed for:

1. **YAML frontmatter** — delimited by `---` at the top of the file
2. **Markdown body** — everything after the closing `---`

### Supported frontmatter fields

| Field | Required | Source | Description |
|-------|----------|--------|-------------|
| `name` | Yes | User | Skill name (used for display, must match directory name) |
| `description` | Yes | User | Short description for the catalog |

For each skill, the parser will generate a `location` field with the absolute path to the `SKILL.md` file. This is used to access the skill by the model.

All other user-provided fields are **ignored**.

### Validation

Per the open standard — **lenient**. Warn on minor issues but don't block loading:
- Missing `name` or `description` → warn, use directory name as fallback
- Unparseable YAML frontmatter → skill is **not loaded**; error message shown to user: `Skill <skill-name> failed to load due to YAML parsing issue`
- `location` is **auto-generated** by the parser (absolute path to the `SKILL.md` file), never read from user-provided YAML

---

## Progressive Disclosure

### Tier 1: Catalog (system prompt)

A bulleted list of all **enabled** skills is injected into the system prompt on every turn:

```
## Available Skills

You have access to specialized skills. To activate a skill, read its SKILL.md file
using the read_file tool, then follow the instructions within.

- my-skill: Brief description of what it does
- another-skill: Another description
```

If no skills are found, nothing is added.

### Tier 2: Instructions (on activation)

The model activates a skill by calling `read_file` on the skill's `SKILL.md` path. The skill body contains instructions the model follows.

### Tier 3: Resources

Skill instructions may reference files relative to the skill root directory. The model accesses these via standard tools.

---

## Skill Invocation

### Model-driven invocation

The model reads the catalog and decides when a skill is relevant. It invokes the skill by using `read_file` on that skill's `SKILL.md`.

### User-driven invocation (`/<skill-name>`)

When the user types `/<skill-name> [arguments...]`, the REPL intercepts this:

1. Look up the skill by name
2. Read the `SKILL.md` file
3. **Substitute argument placeholders** in the content (Claude Code convention):
   - `$ARGUMENTS` → all arguments joined with spaces
   - `$ARGUMENTS[N]` → single argument at index N (0-based, e.g., `$ARGUMENTS[0]` for first arg)
   - `$0`, `$1`, `$2`, ... `$9` → positional arguments (`$0` is the first argument)
   - If arguments are provided but none of the above placeholders appear in the content, they are silently ignored
4. Inject the processed content as a user message prefixed with context:

```
[Activated skill: my-skill]
Arguments: foo bar

<processed SKILL.md content>
```

The model then follows the instructions within.

---

## `/skills` REPL Command

A meta-command for managing skills. Intercepted by the REPL before LLM dispatch.

### `/skills list`

Displays all discovered skills with their status:

```
  Available Skills:
    my-skill        ✓ enabled  - Brief description
    another-skill   ✗ disabled - Another description
```

### `/skills <name> enable`

Enables a skill.

Confirmation: `  ✓ Skill "my-skill" enabled`

### `/skills <name> disable`

Disables a skill.

Confirmation: `  ✓ Skill "my-skill" disabled`

---

## Config Persistence

Skill enable/disable state is stored in `~/.keen/skills/config.json`:

```json
{
  "is_enabled": {
    "my-skill": true,
    "another-skill": false
  }
}
```
---

## System Prompt Changes

The `llm.Build()` function is extended to scan skills directories and append
the catalog. No signature change — callers remain unchanged.

`Build()` internally:

1. Scans `.agents/skills/` (project + global) for `SKILL.md` files
2. Loads `~/.keen/skills/config.json` for enable/disable state
3. Builds the catalog string and appends it after project instructions

The catalog section instructs the model on how to access skills:

```
## Available Skills

You have access to specialized skills. To activate a skill, use the read_file
tool to read the skill's SKILL.md file at one of these paths, then follow the
instructions within:

- my-skill: Brief description → read /absolute/path/to/project/.agents/skills/my-skill/SKILL.md
- another-skill: Another description → read /home/user/.agents/skills/another-skill/SKILL.md
```

### Disabled skills

Disabled skills are **not** included in the system prompt catalog. They are only visible via `/skills list`.

---

## File References in Skills

All file paths in skill instructions are relative to the skill's root directory (the directory containing `SKILL.md`). For example, if `SKILL.md` references `scripts/helper.sh`, the full path is:

```
.agents/skills/my-skill/scripts/helper.sh
```

The model is instructed in the catalog to resolve relative paths against the skill directory.

---

## Guard Allowlist for Skill Roots

The filesystem guard currently blocks global skills in two ways:

1. **`~/.agents` is blocked** — `guard.go:55` blocks all `~/.*` paths
2. **`PermissionPending` for reads outside working dir** — `CheckPath` returns `PermissionPending` for read operations outside the working directory, prompting the user on every invocation

To fix this, the guard gains an allowlist of discovered skill roots:

- `Guard` gets an `AddAllowlistRoot(path string)` method
- `CheckPath` treats any path within an allowlisted root as `PermissionGranted` for read operations, even outside the working dir
- The REPL passes discovered skill root directories to the guard after scanning, on every `Build()` call

This covers not only `SKILL.md` files but also any bundled resources (scripts, references) within the skill directory.

---

## Implementation Plan

### New files

| File | Purpose |
|------|---------|
| `internal/skills/discover.go` | Scan directories, find `SKILL.md` files |
| `internal/skills/parse.go` | Parse YAML frontmatter + markdown body |
| `internal/skills/catalog.go` | Build catalog string for system prompt |
| `internal/skills/config.go` | Load/save `~/.keen/skills/config.json` |
| `internal/skills/invoke.go` | Handle `/<skill-name>` argument substitution |

### Modified files

| File | Change |
|------|--------|
| `internal/llm/systemprompt.go` | Accept and append skills catalog |
| `internal/filesystem/guard.go` | Add allowlist for skill roots; override `~/.*` block + `PermissionPending` for allowed paths |
| `internal/cli/repl/commands/commands.go` | Add `/skills` command |
| `internal/cli/repl/command_handlers.go` | Handle `/skills list/enable/disable` |
| `internal/cli/repl/repl.go` | No changes needed (skills loaded on-demand by `Build()`) |
| `internal/cli/repl/handlers.go` | Intercept `/<skill-name>` before LLM dispatch |
| `internal/cli/repl/widgets/suggestion.go` | Auto-complete skill names after `/` |
| `internal/cli/repl/appstate/state.go` | Pass allowlisted skill roots to guard on each `Build()` call |

### Tests

| File | Purpose |
|------|---------|
| `internal/skills/discover_test.go` | Discovery with various directory layouts |
| `internal/skills/parse_test.go` | YAML parsing, validation, edge cases |
| `internal/skills/catalog_test.go` | Catalog generation, empty states |
| `internal/skills/config_test.go` | Config load/save, defaults |
| `internal/skills/invoke_test.go` | Argument substitution |
| `internal/filesystem/guard_test.go` | Allowlist grants read access to skill roots outside working dir, blocks `~/.*` paths that aren't allowlisted |
| `internal/cli/repl/command_handlers_test.go` | `/skills` command behavior |
| `internal/llm/systemprompt_test.go` | Catalog injection |

---

## Edge Cases

| Case | Behavior |
|------|----------|
| No `.agents/skills/` directory exists | No skills loaded, no catalog injected |
| `SKILL.md` has no YAML frontmatter | Treated as plain markdown; use directory name as skill name |
| `SKILL.md` has unparseable YAML frontmatter | Skill is not loaded; error message: `Skill <skill-name> failed to load due to YAML parsing issue` |
| `SKILL.md` is empty | Skill is skipped |
| Skill directory contains only `SKILL.md` (no resources) | Valid; catalog entry created normally |
| Same skill name in project and global | Project wins; global version is shadowed |
| User disables a skill, then removes the directory | Config key becomes orphaned (harmless — cleaned up if skill reappears) |
| `/<skill-name>` with no matching skill | Pass through to LLM as regular text (model may ignore or respond) |
| Skill with `$ARGUMENTS` but no arguments passed | `$ARGUMENTS` replaced with empty string |
| Skill invoked with arguments but no placeholder in content | Arguments are silently ignored |
| Project changes during session | Reflected on next message (catalog rescanned each turn) |
| Model tries to `read_file` a global skill | Guard allowlists discovered skill roots; reads granted automatically |
| Skill references a bundled resource (e.g., `scripts/foo.sh`) | Guard allows read because path is within allowlisted skill root |
