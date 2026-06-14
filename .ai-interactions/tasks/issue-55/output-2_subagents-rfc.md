# RFC: Skill-Like Subagents for Keen Code

## Summary

Implement Keen subagents using a skill-like authoring and discovery mechanism:
Markdown files with YAML frontmatter, discovered from project and user
directories, surfaced to the main agent through a catalog, and invoked through a
dedicated `delegate_task` tool.

The important distinction is runtime behavior:

- A skill is loaded into the main agent's context and changes how the main agent
  works.
- A subagent profile configures a separate child agent that runs in its own
  context and returns a compact result to the parent.

This keeps the authoring model familiar while preserving Keen's existing context
discipline.

## Goals

- Let users define reusable subagents with a lightweight Markdown + YAML format.
- Reuse the mental model and parser/discovery patterns of Keen skills where it
  makes sense.
- Give the main agent a tool for explicit bounded delegation.
- Keep noisy exploration out of the parent agent's live context.
- Start with read-only subagents to avoid file conflicts and permission
  ambiguity.
- Preserve Keen's existing tool permission model.
- Keep implementation provider-neutral across Keen's LLM clients.
- Support the profile fields needed for model routing:
  `provider`, `model`, and `thinking_effort`.
- Keep subagents independent from Keen skills and MCP support.

## Non-Goals

- Do not implement autonomous agent teams.
- Do not let subagents recursively spawn subagents in the first version.
- Do not implement worktree or filesystem isolation in this RFC.
- Do not delegate gratuitously or for ordinary prompts that the main agent can handle directly.
- Do not replace skills, turn memory, compaction, or the `/adversary` command.
- Do not add write-capable parallel workers until read-only delegation is
  stable.

## Motivation

Keen already has strong cross-turn context management through `TurnMemory`.
Completed tool-heavy turns are compressed into small durable summaries instead
of replaying every file read, grep result, and command output in later turns.

Subagents still add value in a different place: within a single active turn.
During a long turn, the parent agent can still accumulate noisy intermediate
state while exploring broad areas of the codebase. A child agent can do that
work in a separate context and return only the distilled findings.

The most valuable first use cases are:

- broad codebase exploration
- review passes
- test failure analysis
- log summarization
- dependency or API research
- adversarial critique of a proposed plan

These are usually read-heavy and bounded, which makes them a good fit for a
safe MVP.

## Design Principles

1. Deliberate delegation.
   The main agent may choose to use a subagent on its own when the work is
   bounded, independent, and benefits from a narrower role or reduced parent
   context. Users can also explicitly request a named subagent. The main agent
   should not delegate gratuitously or for every task.

2. Skill-like authoring, separate execution.
   Users should define subagents in simple Markdown files, but invocation should
   start a separate child context.

3. Read-only first.
   Start with agents that can inspect and summarize. Writing can come later with
   stronger isolation.

4. Permissions are an intersection.
   A subagent should never gain tools or access that the parent session does not
   have.

5. Summaries, not transcripts.
   The parent should receive a concise structured result. Raw child tool output
   should be inspectable separately, not injected wholesale into the parent
   context.

6. Provider-neutral orchestration.
   Subagent orchestration should live above provider clients, not inside a
   specific Anthropic/OpenAI/Genkit implementation.

## User Model

Users create subagent profiles in files such as:

```text
.keen/agents/reviewer.md
.agents/agents/reviewer.md
~/.keen/agents/reviewer.md
~/.agents/agents/reviewer.md
```

Example:

```markdown
---
name: reviewer
description: Reviews code for correctness, security, regressions, and missing tests.
tools: [read_file, glob, grep]
provider: inherit
model: inherit
thinking_effort: inherit
---

Review like a maintainer.
Prioritize correctness, security, behavior regressions, and missing tests.
Return only actionable findings with file references.
Do not edit files.
```

The main agent sees a subagent catalog in the system prompt and can call:

```text
delegate_task(agent: "reviewer", task: "Review this branch for regressions")
```

The subagent runs separately, then returns a compact result to the parent agent.

## Profile Format

Subagent profiles are Markdown files with YAML frontmatter followed by
instructions.

Required fields:

| Field | Type | Purpose |
| --- | --- | --- |
| `name` | string | Stable subagent identifier used by `delegate_task` |
| `description` | string | When this subagent should be used |

Supported optional fields:

| Field | Type | Default | Purpose |
| --- | --- | --- | --- |
| `tools` | string[] | read-only default | Tool allowlist. MVP supports only `read_file`, `glob`, and `grep`; requested tools are intersected with parent availability. |
| `provider` | string | `inherit` | Provider override or inherited parent provider |
| `model` | string | `inherit` | Model override or inherited parent model |
| `thinking_effort` | string | `inherit` | Reasoning/thinking effort override when the selected model supports it |
| `timeout_seconds` | number | `600` | Child runtime cap in seconds |
| `hidden` | bool | `false` | Hide from catalog/autocomplete but allow direct internal use |

Future fields:

| Field | Purpose |
| --- | --- |
| `mode` | `subagent`, `primary`, or `all` if Keen later supports primary agents |
| `disallowed_tools` | Remove tools from inherited/default tool sets |
| `permission_mode` | `inherit`, `read-only`, `ask`, or `auto` |
| `color` | Optional UI color hint |

Unknown fields should produce warnings, not hard failures, unless they make the
profile unsafe or ambiguous. This allows partial compatibility with other
agent-file ecosystems.

The minimal useful profile is therefore:

```markdown
---
name: reviewer
description: Reviews code for correctness, regressions, security, and missing tests.
---

Review like a maintainer.
Return actionable findings with file references.
```

All omitted optional fields inherit from the main agent's resolved runtime
environment or global agent defaults. This keeps profile files small while still
allowing explicit overrides when a subagent needs a different model, tool
surface, timeout, or thinking effort.

Subagents do not have Keen skills or MCP support. Profile frontmatter must not
include `skills` or `mcp_skills`; if those fields appear, they are treated as
unknown fields and reported as warnings.

## Discovery

Use a discovery flow similar to skills:

1. Find candidate Markdown files under configured roots.
2. Parse YAML frontmatter.
3. Validate required fields.
4. Resolve duplicates by priority.
5. Return a catalog plus warnings.

Suggested root priority:

1. `<working-dir>/.agents/agents/`
2. `<working-dir>/.keen/agents/`
3. `<working-dir>/.claude/agents/` for compatible subset import
4. `~/.agents/agents/`
5. `~/.keen/agents/`
6. `~/.claude/agents/` for compatible subset import
7. bundled Keen agents

This mirrors the current skill discovery philosophy while keeping subagents in
their own namespace.

## Built-In Subagents

Keen should ship the first MVP with one bundled read-only profile:

| Name | Tools | Purpose |
| --- | --- | --- |
| `explorer` | `read_file`, `glob`, `grep` | Investigate code, docs, or configuration and return concise findings with references |

Additional bundled read-only profiles such as reviewers or analyzers can be added
later after the execution model is stable. Do not include a write-capable
`worker` in the default MVP.

## Invocation Tool

Add a built-in tool:

```text
delegate_task
```

Suggested input schema:

```json
{
  "type": "object",
  "required": ["agent", "task"],
  "properties": {
    "agent": {
      "type": "string",
      "description": "Name of the subagent profile to run."
    },
    "task": {
      "type": "string",
      "description": "Bounded task for the subagent."
    },
    "timeout_seconds": {
      "type": "integer",
      "description": "Optional lower timeout than the profile/global timeout."
    }
  }
}
```

Tool output should be structured:

```json
{
  "agent": "review",
  "status": "completed",
  "summary": "...",
  "findings": [
    {
      "severity": "high",
      "file": "internal/foo.go",
      "line": 42,
      "message": "..."
    }
  ],
  "commands_run": [],
  "errors": [],
  "session_id": "child-session-id"
}
```

The exact Go output type can be simpler at first, but the model-facing result
should be predictable and concise.

## Parent Prompt Guidance

The main system prompt should gain a small section explaining subagents:

- Use `delegate_task` for bounded, independent work that would otherwise create
  noisy context or benefit from a narrower role.
- Prefer subagents when the task is separable, has a clear objective, and matches
  a listed subagent's description.
- Good uses vary by subagent and can include investigation, review, comparison,
  summarization, tracing, or domain-specific analysis.
- Do not use subagents for quick file reads, direct edits, ambiguous requests that
  need clarification, or tightly coupled implementation work.
- Ask subagents for concise results with relevant references.
- Synthesize results yourself after child agents return.

This keeps the feature useful without encouraging gratuitous fan-out.

## Child Prompt Contract

Each child agent gets a system prompt from the selected subagent markdown file
plus a delegated user task. The markdown body after the YAML frontmatter is the
subagent's system prompt.

The child system prompt is composed from:

1. The selected agent profile body, from the markdown after frontmatter.
2. Runtime context such as the working directory.
3. No skill or MCP catalog; subagents use only the tools provided by their
   filtered child registry.

The delegated task is sent as the child user message, not embedded in the
system prompt.

Any global subagent constraints should live in the bundled/profile markdown
itself, not in a separate Keen base child-agent prompt.

## Runtime Architecture

Add a new package:

```text
internal/subagents
```

Suggested files:

```text
internal/subagents/profile.go
internal/subagents/parse.go
internal/subagents/discover.go
internal/subagents/catalog.go
internal/subagents/runner.go
internal/tools/delegate_tool.go
```

Core types:

```go
type Profile struct {
    Name           string
    Description    string
    Tools          []string
    Provider       string
    Model          string
    ThinkingEffort string
    TimeoutSeconds int
    Hidden         bool
    Instructions   string
}

type Runner struct {
    WorkingDir string
    Config     *config.ResolvedConfig
    Profiles   Catalog
    NewClient  ClientFactory
    Registry   *tools.Registry
}
```

The runner should:

- Resolve the profile.
- Build child messages.
- Build an effective tool registry.
- Create a fresh child LLM client.
- Resolve profile `provider`, `model`, and `thinking_effort` against parent
  config.
- Do not resolve or load skills for child agents.
- Apply timeout and cancellation.
- Capture child stream events.
- Return a compact result.
- Persist child transcript metadata.

## Client Lifecycle

Do not reuse the parent `LLMClient` instance for child agents unless it is
explicitly safe.

Reason:

- Some provider clients hold pending state for incomplete turns.
- Some clients may maintain provider-specific session state.
- Parallel subagents would create race risk if they share one mutable client.

Add a client factory that can instantiate a new client from a resolved config.
For MVP, subagents can inherit the parent provider/model but still use a fresh
client instance.

## Tool and Permission Model

Effective tools are an intersection:

```text
effective_tools =
  parent_available_tools
  intersect read_only_subagent_tools
  intersect profile.tools_or_default
  minus delegate_task
```

Rules:

- Child agents never receive `delegate_task` in the MVP.
- MVP child agents receive only read-only tools: `read_file`, `glob`, and `grep`.
- The `tools` frontmatter field can further restrict that read-only set, but it
  cannot grant write tools, `bash`, MCP tools, or recursive delegation.
- `call_mcp_tool` is never exposed to child agents because subagents do not have
  MCP support.
- Permission prompts must show the child agent label when a child triggers them.

This gives users predictable control and avoids privilege escalation.

Subagent profile prompts should make their own constraints clear. The child
prompt should not include the parent skill catalog or MCP catalog.

## Configuration

MVP runtime defaults:

```text
max_concurrency = 1
max_depth = 1
default_timeout_seconds = 600
```

For the first implementation, these are code-level defaults. A later iteration
can add a global `Agents` config section after the runner and transcript model
are proven.

## Session and Transcript Persistence

Child work should not disappear, but it also should not pollute parent context.

MVP approach:

- Create a normal session for each child run.
- Store metadata linking it to the parent session:
  - parent session ID
  - child session ID
  - agent name
  - task
  - status
  - started/ended timestamps
- Return the child session ID in `delegate_task` output.

Potential event additions:

```go
KindSubagentStarted
KindSubagentFinished
```

These can be added later if the UI needs richer replay. The MVP can keep the
parent transcript clean by representing subagent invocation as a normal tool
call result.

## TUI Behavior

MVP:

- Render `delegate_task` like any other tool call.
- Show the agent name and short task summary.
- On completion, show the compact result.

Later:

- `/agents` lists available profiles.
- `/agents reload` reloads profile files.
- `/agents status` lists child runs for the current session.
- `/agent <session-id>` opens or prints the child transcript.
- Permission overlays show the child label.

Avoid building a complex agent dashboard until the basic execution model is
stable.

## Headless Behavior

In text mode, include only the parent agent's final answer.

In JSON mode, eventually include child metadata:

```json
{
  "session_id": "parent",
  "text": "...",
  "subagents": [
    {
      "session_id": "child",
      "agent": "review",
      "status": "completed",
      "summary": "..."
    }
  ]
}
```

The first implementation can omit this and rely on the tool result in the
parent transcript, but the data model should not block JSON support.

## Relationship to Skills

Subagents should reuse skill-like machinery where practical:

- YAML frontmatter parsing
- Markdown body instructions
- discovery roots
- catalog generation
- enable/disable config pattern
- bundled profiles

Subagents should not reuse skill activation semantics directly:

- A skill is read or injected into the main context.
- A subagent starts a separate child context.
- A skill has no independent tool registry.
- A subagent has its own filtered tool registry.
- A skill does not produce a child transcript.
- A subagent should produce a child transcript or linked metadata.
- Subagents do not have skills or MCP support.

This distinction should be explicit in docs to avoid user confusion.

## Relationship to TurnMemory

`TurnMemory` remains Keen's cross-turn memory compression mechanism.

Subagents add within-turn isolation:

- Parent context receives a compact result.
- Child raw file reads and searches stay in the child transcript.
- Later parent turns still retain only the parent assistant response plus normal
  turn memory.

Do not use subagents as a replacement for compaction or turn memory.

## Rollout Plan

### Phase 1: Read-Only MVP

- Add `internal/subagents` parser/discovery/catalog.
- Add bundled `explorer` profile.
- Add serial `Runner`.
- Add `delegate_task` tool.
- Use fresh child clients.
- Use read-only child registries.
- Support `tools` as a read-only allowlist.
- Support `provider`, `model`, and `thinking_effort` in profile resolution.
- Do not support skills or MCP in subagents.
- Disable recursive delegation.
- Add unit tests.

### Phase 2: Commands and Config

- Add `/agents list`.
- Add `/agents reload`.
- Add `AgentsConfig`.
- Add enable/disable support if needed.
- Document profile format.

### Phase 3: Better Persistence and Headless Metadata

- Link child sessions to parent sessions.
- Add child metadata to headless JSON.
- Add child transcript lookup.

### Phase 4: Parallel Subagents

- Raise `max_concurrency`.
- Add cancellation for multiple active children.
- Add clear TUI status.
- Run race tests heavily.

### Phase 5: Write-Capable Workers

- Add experimental write-capable profiles.
- Enforce a single-writer rule.
- Add conflict detection.
- Consider filesystem or worktree isolation only in a separate future RFC.

## Implementation Status

Completed in the first iteration:

- Added `internal/subagents` for profile parsing, bundled discovery/catalog, and the serial runner.
- Added bundled `explorer` as the first read-only subagent, using only `read_file`, `glob`, and `grep`.
- Added `delegate_task` in `internal/tools`, with `agent`, `task`, and optional `timeout_seconds`.
- Wired the parent prompt to show available subagents and registered `delegate_task` for the main agent only.
- Child agents use the selected markdown body as their system prompt, inherit runtime model/provider settings, run through a fresh client, and do not receive skills, MCP, bash, write tools, or recursive delegation.
- The delegate tool buffers the child stream and returns a concise final result to the parent.

## Testing Plan

Parser/discovery:

- Valid profile parses.
- Missing `name` fails.
- Missing `description` fails.
- Invalid YAML returns clear error.
- Unknown fields produce warnings.
- Duplicate names resolve by root priority.
- Hidden profiles are omitted from catalog.

Tool registry:

- Child registry contains only allowed read-only tools.
- `tools` can restrict the read-only set.
- `delegate_task` is never present in child registry.
- Unknown profile returns a tool error.
- Profile cannot grant tools unavailable to parent.
- `call_mcp_tool` is never present in child registry.

Runner:

- Child messages include the markdown body system prompt and delegated task.
- Timeout cancels the child run.
- Child result is summarized.
- Child errors are returned to parent as structured errors.
- Fresh client factory is called for each child.
- Provider/model/thinking overrides are applied to the child config.
- Child prompt does not include the parent skill catalog or MCP catalog.

Permissions:

- Read-only profile cannot write.
- Permission prompts include agent identity once child prompts are wired through
  the requester.

Integration:

- Main agent can call `delegate_task` and use the result.
- Headless run can complete with a delegated child task.
- Session transcript stores the parent tool result.

Concurrency, later:

- Parallel children respect max concurrency.
- Race test passes with multiple child runs.
- Canceling parent cancels children.

## Open Questions

1. Should `.agents/agents/` or `.keen/agents/` have higher priority?
   The RFC proposes `.agents/agents/` first for ecosystem compatibility, but
   `.keen/agents/` first would make Keen-specific behavior more predictable.

2. Should `bash` be allowed for specialized analyzer subagents later?
   The safer answer is no for the MVP. A later phase can add read-only command
   classes or command allowlists.

3. Should hidden profiles be callable by explicit user prompt?
   The MVP should treat `hidden` as "not in catalog" rather than "not callable,"
   unless a later `user_invocable` field is added.

4. Should child sessions be visible in `/sessions`?
   They should probably be hidden by default and reachable from parent metadata.

## Decision

Proceed with skill-like subagent profiles plus a `delegate_task` tool.

The first implementation should be deliberately conservative:

- read-only
- serial
- deliberate delegation
- max depth 1
- fresh child client
- provider/model/thinking override support
- no skills or MCP in child agents
- no recursive delegation
- no bash in child agents
- compact result returned to parent

This gives Keen the useful part of subagents while preserving the core design
values already present in the codebase: small context, explicit memory,
provider-neutral execution, and strict tool permissions.
