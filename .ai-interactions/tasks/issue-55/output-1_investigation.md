# Subagents in Coding Agents: Investigation for Keen Code

Date: 2026-06-09

## Scope

This investigation looks at how current coding agents use subagents or adjacent
parallel-agent workflows, then maps those patterns onto Keen Code's architecture.
The focus is practical product and implementation design, not a general survey of
multi-agent research.

Primary sources checked:

- Claude Code docs: <https://code.claude.com/docs/en/sub-agents>
- Claude Code agent teams: <https://code.claude.com/docs/en/agent-teams>
- Claude Code agent view: <https://code.claude.com/docs/en/agent-view>
- Claude Code worktrees: <https://code.claude.com/docs/en/worktrees>
- Codex official manual, fetched 2026-06-09 from
  <https://developers.openai.com/codex/codex-manual.md>
- OpenCode agents docs: <https://opencode.ai/docs/agents/>
- GitHub Copilot cloud agent docs:
  <https://docs.github.com/en/copilot/concepts/agents/cloud-agent/about-cloud-agent>
- GitHub Copilot CLI `/fleet` docs:
  <https://docs.github.com/en/copilot/concepts/agents/copilot-cli/fleet>
- GitHub Copilot custom agents docs:
  <https://docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/create-custom-agents-for-cli>
- GitHub Copilot custom agents configuration:
  <https://docs.github.com/en/copilot/reference/custom-agents-configuration>
- Gemini CLI repo/docs:
  <https://github.com/google-gemini/gemini-cli>

## Executive Summary

The mature pattern is consistent across Claude Code, Codex, OpenCode, and
GitHub Copilot CLI:

1. A "custom agent" is a reusable profile: name, description, instructions,
   model, and tool/permission limits.
2. A "subagent" is a runtime worker spawned by the main agent to perform a
   bounded task in its own context window.
3. The parent agent should receive a distilled result, not raw logs.
4. Read-heavy subagents are the safest and highest-value starting point.
5. Write-heavy parallelism needs isolation, usually worktrees, plus conflict
   management.
6. Delegation must be capped by depth, concurrency, time, and permissions.

For Keen Code, the best first move is not "agent teams." It is a small,
explicit, read-only `task` or `delegate_task` tool backed by reusable agent
profiles. Keen already has the necessary substrate: provider-neutral LLM
clients, a tool registry with filtering, plan/build mode, `/btw`, `/adversary`,
skills, MCP, permissions, and event-sourced sessions. Subagents can be added as
a generalization of `/btw` and `/adversary`, not as a rewrite of the main loop.

Recommended direction:

- Phase 1: explicit read-only subagents for exploration, review, test-log
  summarization, and adversarial critique.
- Phase 2: custom agent profiles in `.keen/agents/*.md` with YAML frontmatter.
- Phase 3: parallel execution, parent-visible progress, and transcript support.
- Phase 4: write-capable workers only with strong permission inheritance and,
  later, optional git worktree isolation.

## What Existing Agents Do

### Claude Code

Claude Code has the richest subagent system in this survey.

Core model:

- Subagents are specialized assistants with their own context window, custom
  prompt, tool access, and permission controls.
- Claude can delegate automatically based on a subagent description, and users
  can invoke agents explicitly.
- Built-ins include Explore, Plan, and general-purpose agents.
- Custom subagents are Markdown files with YAML frontmatter. Project agents live
  in `.claude/agents/`; user agents live in `~/.claude/agents/`.
- Supported fields include `name`, `description`, `tools`, `disallowedTools`,
  `model`, `permissionMode`, `maxTurns`, `skills`, `mcpServers`, `hooks`,
  `memory`, `background`, `effort`, `isolation`, `color`, and `initialPrompt`.
- Subagents start with fresh isolated context. Claude sends a delegation message
  containing the task. Results return to the main conversation.
- Subagents cannot spawn other subagents. Claude recommends chaining from the
  main conversation if needed.
- Subagents can be resumed by ID.
- Worktree isolation is supported with `isolation: worktree`; temporary
  subagent worktrees are cleaned up when possible.

Important design choices:

- Claude strongly separates subagents from skills. Skills are reusable workflows
  that run in the main context; subagents are isolated workers.
- Explore and Plan are read-only and skip some heavier startup context to stay
  fast.
- The docs explicitly recommend subagents for verbose, bounded work: tests,
  docs fetching, logs, code search, and parallel research.
- Claude also has adjacent but distinct features:
  - Agent view: multiple background sessions visible from one TUI.
  - Agent teams: experimental independent Claude sessions with shared task lists
    and inter-agent communication.
  - Worktrees: isolated parallel workspaces.

Takeaway for Keen:

Claude's subagent feature is broad, but the highest-value subset is simple:
isolated context, restricted tools, summaries back to parent, and optional
profile files. Agent teams are a later feature, not an MVP requirement.

### Codex CLI and Codex App

Codex has official subagent workflows, but with a more explicit triggering model
than Claude Code.

Core model:

- Codex can spawn specialized agents in parallel and collect their results.
- Codex only spawns subagents when explicitly asked. Example prompts include
  "spawn one agent per point" or "delegate this work in parallel."
- The CLI exposes `/agent` to switch between active agent threads and inspect
  them.
- Built-in agents include `default`, `worker`, and `explorer`.
- Custom agents are TOML files under `~/.codex/agents/` or `.codex/agents/`.
- Required custom-agent fields are `name`, `description`, and
  `developer_instructions`.
- Custom agent files can override normal config fields such as model, reasoning
  effort, sandbox mode, MCP servers, and skills config.
- Global controls include:
  - `agents.max_threads`
  - `agents.max_depth`
  - `agents.job_max_runtime_seconds`
- The default depth is one direct child. This prevents recursive fan-out.
- Subagents inherit the current sandbox and approval policy. In interactive
  sessions, approval requests can surface from inactive child threads.

Important design choices:

- Codex documents subagents primarily as a solution for context pollution and
  context rot.
- Codex recommends starting with read-heavy parallel tasks, and being more
  careful with parallel write-heavy work.
- Codex treats custom agent files as configuration overlays for spawned
  sessions. This is powerful, but heavier than a dedicated agent manifest.
- Codex app worktrees are separate but related. They let background tasks run
  without disturbing local state.

Takeaway for Keen:

Codex's explicit-only policy is attractive for Keen. It reduces surprise,
protects token budget, and fits a CLI product where users expect visible
control. Keen should copy Codex's `max_depth = 1` default and explicit user
intent gate.

### OpenCode

OpenCode exposes agents as a first-class concept with two modes: primary agents
and subagents.

Core model:

- Primary agents are the agents users interact with directly.
- Subagents are invoked by primary agents for specific tasks, or manually via
  `@` mention.
- Built-in primary agents include Build and Plan.
- Built-in subagents include General, Explore, and Scout.
- Explore is read-only for codebase search and understanding.
- Scout is read-only for external docs/dependency research.
- General can do multi-step work and can make changes.
- Subagents create child sessions. The UI has keybindings to enter a child
  session, cycle child sessions, and return to the parent.
- Agent config supports JSON and Markdown.
- `mode` can be `primary`, `subagent`, or `all`.
- `hidden: true` hides a subagent from autocomplete while preserving
  programmatic invocation.
- `permission.task` controls which subagents an agent may invoke, with glob
  patterns and allow/deny/ask behavior.

Important design choices:

- OpenCode treats Plan/Build as primary modes, not only prompts.
- It exposes subagent navigation directly in the UI.
- It adds task-level permission controls, so an orchestrator can be prevented
  from spawning arbitrary agents.

Takeaway for Keen:

OpenCode's primary/subagent distinction maps cleanly to Keen's existing
plan/build mode. Keen should add an agent permission surface early, even if the
first version only supports a small allowlist.

### GitHub Copilot

GitHub now has several agent surfaces. Two are most relevant here: Copilot cloud
agent and Copilot CLI subagents.

Cloud agent:

- Copilot cloud agent works independently in a GitHub Actions-powered ephemeral
  development environment.
- It can research a repository, create plans, make branch changes, run tests,
  and optionally open pull requests.
- It supports multiple custom agents specialized for task types.
- This is more like "background coding sessions" than in-process CLI
  subagents.

Copilot CLI `/fleet`:

- `/fleet` breaks a complex request into smaller independent subtasks.
- The main Copilot agent acts as an orchestrator and runs subagents in parallel
  when dependencies allow.
- Each subagent has its own context window.
- Subagents can use custom agents when their profiles match the work.
- The docs warn that subagents increase credit usage.
- `/fleet` is often paired with autopilot, but they are distinct features.

Custom agents:

- Custom agents for Copilot CLI are Markdown files with an `.agent.md`
  extension.
- Project agents live in `.github/agents/`; user agents live in
  `~/.copilot/agents/`.
- Copilot may infer when to use one from its description, or the user can select
  it explicitly.
- Frontmatter can include `description`, `tools`, `model`,
  `disable-model-invocation`, `user-invocable`, `mcp-servers`, and metadata.
- The `agent` tool alias allows one custom agent to invoke another custom
  agent.

Takeaway for Keen:

Copilot reinforces that subagents are most useful after a plan exists and can be
split into independent subtasks. Keen should avoid automatic decomposition until
the product has good observability and cost controls.

### Gemini CLI

Gemini CLI is relevant mostly as a contrast.

Documented strengths:

- Large context window.
- Built-in file operations, shell commands, web fetching/search, MCP, and
  Google Search grounding.
- Conversation checkpointing.
- `GEMINI.md` context files.
- Custom commands.
- Headless mode with JSON and streaming JSON output.
- GitHub Action workflows.

I did not find first-class "subagent" documentation in the checked Gemini CLI
README, configuration docs, or command docs. Gemini appears to lean more on
large context, checkpointing, custom commands, and automation rather than a
documented subagent abstraction.

Takeaway for Keen:

Subagents are not the only answer to large tasks. Keen should still keep
compaction, skills, headless runs, and concise context management strong. A
subagent feature should complement these, not replace them.

## Cross-Agent Design Patterns

### 1. Subagents Are Context Isolation First

The clearest common value is keeping noisy work out of the parent context.
Search results, test logs, dependency docs, stack traces, and broad scans are
useful, but they damage the main planning thread if pasted raw into it.

Keen implication:

The `task` tool should return a short structured result by default:

- summary
- key findings
- file references
- commands run
- errors or uncertainty
- optional raw transcript/session ID for inspection

It should not dump raw subagent logs into the parent turn.

### 2. Read-Only Subagents Are the Best MVP

Every product makes read-heavy workers a first-class path:

- Claude Explore and Plan
- Codex explorer
- OpenCode Explore and Scout
- Copilot research/planning before pull requests

Keen implication:

Start with read-only built-ins:

- `explore`: code search and architecture understanding
- `review`: code review findings, no edits
- `test-analyzer`: summarize failing tests/logs
- `adversary`: challenge the main approach

Do not start with parallel code writers.

### 3. Profiles Need Descriptions, Tool Limits, and Model Control

All mature systems use a profile file with a description and instructions.
Most include model selection and tool restrictions.

Keen implication:

Use Markdown with YAML frontmatter, not TOML, for agent profiles. Keen already
has a skills parser and users of coding agents are familiar with Markdown
agent files.

Suggested profile format:

```markdown
---
name: review
description: Reviews code for correctness, security, regressions, and missing tests.
mode: subagent
tools: [read_file, glob, grep]
model: inherit
max_turns: 6
---

Review like a senior maintainer. Return only actionable findings with
file references. Do not modify files.
```

Suggested discovery roots:

1. `.keen/agents/`
2. `.agents/agents/`
3. `~/.keen/agents/`
4. `~/.agents/agents/`
5. Optional compatibility import from `.claude/agents/` and
   `.github/agents/*.agent.md` for fields Keen understands.

### 4. Permission Inheritance Must Be an Intersection

Subagents should never be more powerful than the parent session unless the user
explicitly grants that.

Keen implication:

Effective permissions should be:

```text
effective_tools = parent_available_tools intersect agent_profile_tools intersect mode_tools
```

Other rules:

- Plan-mode parent cannot spawn write-capable subagents.
- Read-only agents get no `write_file`, `edit_file`, `bash`, or
  `call_mcp_tool` by default.
- If `bash` is allowed, dangerous command handling stays unchanged.
- If MCP is allowed, agent profiles should support an allowlist later.
- Approval prompts must show the child agent label.

### 5. Fan-Out Needs Hard Caps

Codex explicitly caps depth and concurrent threads. OpenCode has task
permissions. Copilot warns about cost. Claude blocks nested subagents.

Keen implication:

Defaults:

```text
agents.max_depth = 1
agents.max_concurrency = 3
agents.max_turns = 8
agents.default_timeout_seconds = 600
agents.allow_auto_spawn = false
```

The first version should not allow subagents to spawn subagents.

### 6. UI Matters Once Agents Run Concurrently

The initial UI can be simple, but parallel work quickly needs observability.
Claude has agent view; Codex has `/agent`; OpenCode has child-session
navigation.

Keen implication:

MVP:

- Render subagent start/end as a normal tool call with the agent name.
- Show a compact status line while a subagent is running.
- Persist the subagent transcript separately.

Later:

- `/agents` lists active/completed child agents.
- `/agent <id>` opens a child transcript.
- Approval prompts include agent identity.
- Headless JSON emits `subagent_start`, `subagent_event`, `subagent_done`.

### 7. Worktree Isolation Is Valuable, But Not First

Worktrees are a strong answer for parallel writers, but they introduce branch,
cleanup, ignored-file, and dependency setup complexity.

Keen implication:

Do not block read-only subagents on worktrees. Add worktree isolation only when
Keen supports write-capable workers.

Future profile field:

```yaml
isolation: worktree
```

Potential worktree root:

```text
~/.keen/worktrees/<repo-hash>/<agent-id>/
```

This avoids polluting the main checkout with `.keen/worktrees/` directories.
Keen would need cleanup, lock, and restore rules before making this default.

## Keen Code Architecture Fit

Relevant existing pieces:

- `internal/llm.LLMClient` already streams events from a message list and a
  tool registry.
- `llm.StreamOptions` already has `SessionID` and `OneShot`.
- `internal/tools.Registry` already supports `Without(...)`, which is enough
  for read-only tool filtering.
- `AppState.StreamBtw` already runs an isolated no-tool side conversation.
- `AppState.StreamAdversary` already runs a separate model with a read-only
  registry.
- Plan mode already removes write tools from the registry.
- The permission system already centralizes filesystem guard checks and user
  approval.
- Session storage is event-sourced and can be extended for child transcripts.
- Skills and MCP already provide extension surfaces.
- Headless mode already has text and JSON output paths.

Keen's unique strengths:

1. Provider neutrality. Keen can route subagents to Anthropic, OpenAI,
   OpenAI-compatible providers, Google AI via Genkit, OpenCode Go, MiniMax,
   Moonshot, DeepSeek, and Z.ai. This enables cheap explorer agents and stronger
   reviewer agents without hardcoding one vendor.
2. Existing side-agent affordances. `/btw` and `/adversary` are already
   user-facing examples of isolated helper contexts. Subagents can be explained
   as "generalized `/btw` with tools and a reusable role."
3. Strong permission architecture. The guard/requester model is a good base for
   labeled child-agent approvals.
4. Skills compatibility. Keen can let agent profiles preload skills or invoke
   skills, while keeping skills distinct from subagents.
5. CLI and headless symmetry. A subagent runner can serve interactive TUI,
   headless JSON, and future automation flows.

## Recommended Approach for Keen Code

### Product Rule

Keen should make subagents explicit by default:

> Use subagents only when the user asks for agents/parallel delegation, invokes
> a named agent, or uses a dedicated command/tool. Do not silently fan out for
> ordinary prompts.

This matches Codex, reduces surprise, and keeps cost predictable.

### MVP: Read-Only Task Delegation

Add a built-in tool exposed to the main agent:

```text
delegate_task
```

Suggested input schema:

```json
{
  "agent": "explore",
  "task": "Find how permissions are enforced for write_file.",
  "return_format": "Summarize files, functions, and risks with line references.",
  "max_turns": 6
}
```

Behavior:

- Runs one child LLM conversation.
- Uses a fresh context window.
- Uses the selected agent profile's system prompt.
- Adds the user's task as the first user message.
- Uses a filtered read-only registry unless the profile and parent mode allow
  more.
- Returns a concise structured result as the tool output.
- Stores a child transcript for debugging.
- Does not add raw child conversation to the parent context.

Built-in profiles:

| Agent | Tools | Purpose |
| --- | --- | --- |
| `explore` | `read_file`, `glob`, `grep` | Codebase discovery and architecture mapping |
| `review` | `read_file`, `glob`, `grep` | Findings-only review |
| `test-analyzer` | `read_file`, `glob`, `grep` | Summarize logs and failing tests |
| `adversary` | `read_file`, `glob`, `grep` | Challenge assumptions and identify risks |

Keep `worker` out of the first release or hide it behind an experimental flag.

### Agent Profile Discovery

Add `internal/agents` with:

- `Agent` struct
- profile parser
- discovery roots
- duplicate handling
- validation
- tests

Suggested fields:

```yaml
name: string
description: string
mode: subagent | primary | all
tools: string[]
disallowed_tools: string[]
provider: inherit | string
model: inherit | string
thinking_effort: inherit | low | medium | high
max_turns: number
timeout_seconds: number
permission_mode: inherit | read-only | ask | auto
skills: string[]
mcp_servers: string[]
hidden: boolean
color: string
isolation: none | worktree
```

For MVP, implement only:

- `name`
- `description`
- `tools`
- `model`
- `max_turns`
- `hidden`
- Markdown body as instructions

Everything else can be accepted but ignored with warnings, or rejected until
implemented. Prefer warnings for cross-tool compatibility.

### Runner Architecture

Add an `AgentRunner` rather than embedding subagent orchestration inside
provider clients.

Proposed package:

```text
internal/agents
  profile.go
  discover.go
  runner.go
  task_tool.go
```

Runner responsibilities:

- Build child system prompt.
- Build filtered tool registry.
- Create a fresh LLM client for the child where possible.
- Enforce depth, turns, timeout, and cancellation.
- Stream child events to transcript and optional UI hooks.
- Return a compact result.

Important implementation note:

Do not assume current `LLMClient` instances are safe for concurrent child use.
Provider clients may hold response/session state. Add a client factory or
clone path from resolved config.

### Prompting Contract

The parent prompt should teach the main model:

- Use `delegate_task` only for bounded, independent work.
- Prefer it for noisy exploration, test/log analysis, and independent reviews.
- Do not use it for quick targeted edits.
- Do not spawn subagents unless the user asked for subagents/parallel work, or
  the task is explicitly a side investigation.
- Ask each subagent for concise findings with evidence.
- Wait for all delegated work before synthesizing.

The child prompt should teach the subagent:

- You are a child agent.
- Your output will be returned to a parent agent.
- Keep raw logs out of the final answer unless essential.
- Cite files and commands.
- State uncertainty.
- Do not continue beyond the delegated task.

### Session and Transcript Model

Add child-agent events rather than hiding everything in generic tool output.

Possible event additions:

```go
KindSubagentStarted
KindSubagentEvent
KindSubagentFinished
```

Or store child sessions with parent metadata:

```json
{
  "parent_session_id": "...",
  "agent_id": "...",
  "agent_name": "explore",
  "task": "...",
  "status": "completed"
}
```

The second option is simpler and aligns with current session storage.

### UI Plan

MVP UI:

- Show `explore started: <task summary>`.
- Show spinner/status.
- Show `explore completed`.
- Parent answer includes the synthesized result.

Next UI:

- `/agents` lists active and completed child runs.
- `/agent <id>` prints or opens a child transcript.
- Child approval prompts include labels like `[explore abc123]`.

### Permission Design

Rules:

1. Parent plan mode forces child read-only mode.
2. Agent profile can remove tools but cannot add tools unavailable to parent.
3. User approval is still required for pending paths and dangerous commands.
4. Background subagent approvals must be cancellable and attributable.
5. Subagents cannot call `delegate_task` until recursive delegation is designed.

For MVP, remove `delegate_task` from child registries unconditionally.

### Configuration

Suggested global config:

```json
{
  "agents": {
    "max_concurrency": 3,
    "max_depth": 1,
    "default_timeout_seconds": 600,
    "allow_auto_spawn": false
  }
}
```

Keen's current global config is JSON, so this fits existing storage better than
a new TOML file.

## Phased Implementation Plan

### Phase 1: Single Read-Only Subagent

- Add built-in `explore` and `review` profiles in code.
- Add `delegate_task` tool.
- Run child agents serially first.
- Use current model/provider.
- Use read-only registry.
- Return structured text.
- Unit test runner prompt construction, registry filtering, and depth blocking.

### Phase 2: Agent Profile Files

- Add `.keen/agents/*.md` discovery.
- Add user-level `~/.keen/agents/*.md`.
- Parse YAML frontmatter.
- Add `/agents list` or `/agents reload`.
- Add tests for precedence, duplicates, invalid fields, and disabled/hidden
  agents.

### Phase 3: Parallel Delegation

- Allow the parent model to call multiple `delegate_task` tools in one turn, or
  add a `delegate_tasks` batch input.
- Add concurrency limit.
- Add cancellation.
- Add child transcript IDs in the returned result.
- Extend headless JSON output.
- Run `go test -race ./...` aggressively here, because this phase touches
  concurrency.

### Phase 4: Write-Capable Workers

- Add experimental `worker` profile.
- Require explicit user opt-in.
- Keep parent permission prompts.
- Add conflict detection.
- Consider single-writer rule before worktree support:
  - only one write-capable subagent at a time
  - no concurrent parent edits while worker runs

### Phase 5: Worktree Isolation

- Add `isolation: worktree`.
- Create worktrees under a managed Keen directory.
- Track ownership and cleanup.
- Surface changed files and patch summaries to parent.
- Add handoff/apply workflow.

## Risks and Mitigations

| Risk | Mitigation |
| --- | --- |
| Token/cost explosion | Explicit trigger, max depth 1, max concurrency, max turns, concise output contract |
| Context pollution returns | Structured summaries only; child transcript linked separately |
| Permission confusion | Effective permissions are intersection; label prompts with agent name |
| Concurrent client races | Fresh client per child; race tests |
| File conflicts | Read-only MVP; later single-writer or worktree isolation |
| UI overload | Start with tool-call rendering; add `/agents` only after runner is stable |
| Recursive fan-out | Remove delegate tool from child registries |
| Provider inconsistency | Keep orchestration provider-agnostic; test with fake client first |
| MCP side effects | MCP disabled in MVP child agents; profile allowlist later |

## Testing Strategy

Critical tests:

- Agent profile parser handles valid/invalid frontmatter.
- Discovery precedence works.
- `delegate_task` rejects unknown or hidden agents when user-invocation is off.
- Plan mode strips write tools from child registries.
- Child registries never contain `delegate_task`.
- Depth and concurrency limits are enforced.
- Timeout cancels child runs.
- Permission prompts include child identity.
- Child result is persisted without adding raw transcript to parent messages.
- Headless JSON includes child metadata once streaming support is added.

Use fake LLM clients for deterministic tests. Add race tests when parallel
execution lands.

## Recommendation

Build Keen subagents as explicit, profile-backed, read-only task delegation
first. Make it boring and reliable:

1. `delegate_task` tool.
2. Built-in `explore`, `review`, `test-analyzer`, and `adversary` profiles.
3. Fresh child context.
4. Filtered tools.
5. Summary-only return.
6. Depth 1.
7. No write-capable parallelism until the lifecycle, transcript, UI, and
   permission model are proven.

This uses Keen's current strengths instead of chasing the largest competitor
feature set immediately. Claude Code shows the long-term ceiling. Codex shows
the safest default trigger model. OpenCode shows the clean primary/subagent UI
split. Copilot shows how parallelization should follow a decomposable plan.
Keen can combine those lessons with its provider-neutral runtime and existing
side-agent affordances.
