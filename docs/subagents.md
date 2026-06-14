# Subagents

Subagents are focused assistants that Keen Code can delegate bounded tasks to during a conversation. They are useful when a task is separable from the main work and benefits from a narrower role or custom instructions.

The first bundled subagent is `explorer`.

## What Subagents Are For

Use subagents for focused, read-only work such as:

- investigating a specific area of a project
- reviewing code, docs, or configuration against a narrow checklist
- comparing related files and reporting concise findings
- tracing references to a function, type, command, or config option
- summarizing relevant context before the main agent makes a decision
- applying project-specific or domain-specific analysis instructions

Subagents are not meant for:

- editing files
- running shell commands
- using skills
- using MCP tools
- handling broad, vague tasks without direction
- replacing the main agent's judgment

The main agent decides when to call a subagent and should provide clear instructions with relevant paths or inputs whenever possible.

## Bundled Subagents

### `explorer`

`explorer` investigates the codebase using read-only tools and returns concise, organized findings.

It can use:

- `read_file`
- `glob`
- `grep`

It cannot use write tools, shell commands, skills, MCP tools, or other subagents.

Good delegation examples:

```text
Use explorer to inspect internal/tools and summarize how tool registration works.
```

```text
Use explorer to review docs/ and internal/config for where provider configuration is documented and loaded.
```

```text
Use explorer to trace references to StreamChat in internal/llm and internal/subagents. Return the relevant files and responsibilities.
```

Less useful examples:

```text
Use explorer to understand the whole repo.
```

```text
Use explorer to fix the bug.
```

```text
Use explorer to read one known file.
```

## Adding Your Own Subagents

Create a Markdown file in one of these directories:

1. `<project>/.agents/agents/`
2. `<project>/.keen/agents/`
3. `<project>/.claude/agents/`
4. `~/.agents/agents/`
5. `~/.keen/agents/`
6. `~/.claude/agents/`

Project-level subagents are useful for repository-specific workflows. User-level subagents are available across projects.

Each subagent is a single `.md` file with YAML frontmatter followed by the subagent's system prompt.

Example:

```markdown
---
name: api-reviewer
description: Reviews API-related code and docs for consistency, correctness, and missing edge cases.
---

You are an API review subagent.

Your role is to inspect API-related files using read-only tools and return concise findings to the parent agent.

Guidelines:
- Stay within the delegated task.
- Focus on paths provided by the parent agent first.
- Check routing, handlers, request/response types, validation, errors, and documentation when relevant.
- Return a short summary, relevant files, and key findings with `path:line` references.
- Do not edit files.
- Do not ask the user questions directly; report blockers to the parent agent.
```

## Frontmatter Fields

Required fields:

| Field | Description |
|---|---|
| `name` | Unique subagent name used by the main agent. |
| `description` | Short description shown to the main agent. |

Optional fields:

| Field | Description |
|---|---|
| `tools` | Restrict the read-only tools available to the subagent. Only `read_file`, `glob`, and `grep` are supported. |
| `timeout_seconds` | Runtime timeout for the subagent. If omitted, Keen uses a default timeout. |
| `hidden` | If `true`, the subagent is loaded but not listed in the main agent's subagent catalog. |
| `provider` | Reserved for model/provider override support. Omit this unless documented for your version. |
| `model` | Reserved for model override support. Omit this unless documented for your version. |
| `thinking_effort` | Reserved for model reasoning-effort override support. Omit this unless documented for your version. |

For most custom subagents, keep the frontmatter minimal:

```yaml
---
name: my-subagent
description: Briefly describe when the main agent should use this subagent.
---
```

By default, subagents inherit the main agent's model and provider configuration.

## Writing Good Subagent Prompts

A good subagent prompt should explain:

- the subagent's focused role
- what kind of tasks it should handle
- how it should explore files
- what it should avoid
- how it should format the final result

Recommended result format:

```text
Summary:
- ...

Relevant files:
- path/to/file.go:10 — why it matters

Key findings:
- ...

Open questions:
- ...
```

Keep prompts specific. A subagent should have a narrow purpose rather than a broad instruction like “help with coding tasks.”

## Limitations

Subagents currently have these limitations:

- They are read-only.
- They do not receive the full parent conversation history.
- They only receive the delegated task and repository context they inspect themselves.
- They do not support skills.
- They do not support MCP tools.
- Their result is returned to the main agent after completion, not streamed directly to the user.

These limits are intentional: subagents should produce concise findings that the main agent can review, synthesize, and act on.
