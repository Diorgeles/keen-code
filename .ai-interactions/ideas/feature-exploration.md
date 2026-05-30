# Keen Code Feature Exploration

Date: 2026-05-26

## Research Scope

This proposal is based on:

- Local Keen Code docs and code: `README.md`, `ROADMAP.md`, `docs/cli-usage.md`, `docs/tools.md`, `docs/architecture.md`, `docs/permission-system.md`, `docs/session-management.md`, `docs/skills-system.md`, `docs/mcp-servers.md`, `docs/mcp-skills.md`, `docs/turn-memory.md`, `docs/ai-providers.md`, and `CHANGELOG.md`.
- Product docs and repositories for Claude Code, OpenAI Codex CLI, Gemini CLI, Aider, OpenCode, and Pi Coding Agent.
- Public discussion signals from Hacker News, Reddit `r/ChatGPTCoding`, and GitHub issues for Aider, Codex, Gemini CLI, and Pi.

The goal is not to make Keen match every competing agent. Keen's current positioning is a small, terminal-first, Go-native coding agent with a narrow tool surface. The strongest opportunities are features that preserve that identity while improving trust, control, verification, and workflow fit.

## What Keen Code Already Does

Keen Code is a terminal-based AI coding agent implemented in Go. It has a Bubble Tea REPL, a Genkit/OpenAI/Anthropic-compatible LLM layer, an event-sourced session store, and a filesystem guard that mediates tool access.

Current shipped capabilities include:

- Multi-provider model support: Anthropic, OpenAI, OpenAI Codex OAuth, Google AI, Kimi, DeepSeek, Z.ai, MiniMax, and OpenCode Go.
- Thinking effort configuration where providers expose it.
- A lean built-in tool set: `read_file`, `write_file`, `edit_file`, `glob`, `grep`, `bash`, `web_fetch`, and `call_mcp_tool`.
- Tool permissions with filesystem policy checks, project-scoped pre-approval, and special handling for dangerous bash commands.
- MCP tool support through generated `mcp:<server>` skills and one generic `call_mcp_tool`.
- Skills as reusable workflows, including bundled workflows such as review, refactor, explain, cleanup, fix-tests, plan, clarify, and commit.
- Plan/build modes, where plan mode removes write-capable tools.
- Persistent sessions under `~/.keen/sessions`, resume support, and `/compact`.
- Compact cross-turn `TurnMemory` that keeps changed files and failed commands instead of replaying raw tool traces.
- Pending turn recovery for incomplete or interrupted assistant turns.
- `/btw` side questions that do not enter the main conversation.
- Headless `keen run` with text or JSON output and provider/model overrides.
- REPL ergonomics: slash-command suggestions, file suggestions, model picker, session picker, copy selection, mode chips, markdown rendering, and update notices.

The main product principle is minimalism. The best new features should reinforce that: fewer tools when possible, strong defaults, and visible control.

## Competitor Feature Scan

### Claude Code

Claude Code has moved beyond a plain terminal agent into a broader agent platform. Public docs describe terminal, IDE, desktop, web, mobile, Slack, GitHub Actions, GitLab CI/CD, Chrome debugging, remote control, scheduled tasks/routines, `/loop`, session teleport, memory, MCP, subagents, hooks, custom slash commands, and an Agent SDK.

Distinctive ideas:

- Cross-surface continuity: terminal, IDE, desktop, web, mobile, and chat integrations share the same agent engine.
- Remote control and teleporting sessions between local and cloud surfaces.
- Scheduled and recurring tasks.
- Browser/Chrome debugging workflows.
- Subagents and custom agents.
- First-class automation around PR review, issue triage, and CI.

Relevant sources:

- `https://code.claude.com/docs/en/overview`
- `https://code.claude.com/docs/en/remote-control`
- `https://code.claude.com/docs/en/github-actions`
- `https://code.claude.com/docs/en/chrome`
- `https://code.claude.com/docs/en/subagents`

### OpenAI Codex CLI

Codex CLI runs locally, supports ChatGPT login and API-key auth, has IDE and desktop app surfaces, supports MCP, web search, subagents, approval modes, Codex Cloud task handoff, and non-interactive scripting via `exec`.

Distinctive ideas:

- Tight integration with ChatGPT subscription auth and Codex cloud tasks.
- Subagents for parallel task decomposition.
- Approval modes as an explicit user posture.
- Web search as a first-class coding-agent capability.
- Desktop app and IDE integration alongside the CLI.

Relevant sources:

- `https://github.com/openai/codex`
- `https://developers.openai.com/codex/cli`
- `https://developers.openai.com/codex/cli/features`
- `https://developers.openai.com/codex/noninteractive`
- `https://developers.openai.com/codex/mcp`

### Gemini CLI

Gemini CLI is open source and emphasizes a free tier, 1M context, Google Search grounding, multimodal input, MCP, custom commands, checkpointing, sandboxing, trusted folders, GitHub Actions, headless JSON/stream-json output, IDE integration, telemetry, and enterprise settings.

Distinctive ideas:

- Checkpoint/restore using a shadow Git repository plus conversation state.
- Sandboxing across macOS Seatbelt, Docker/Podman, gVisor, LXC, and Windows native mechanisms.
- Sandbox expansion prompts when a command needs extra access.
- `stream-json` output for automation.
- Large-context `@file`/directory workflows and Google Search grounding.
- GitHub Action for PR review, issue triage, and mention-triggered assistance.

Relevant sources:

- `https://github.com/google-gemini/gemini-cli`
- `https://raw.githubusercontent.com/google-gemini/gemini-cli/main/docs/cli/checkpointing.md`
- `https://raw.githubusercontent.com/google-gemini/gemini-cli/main/docs/cli/sandbox.md`
- `https://github.com/google-github-actions/run-gemini-cli`

### Aider

Aider is the strongest example of a minimal terminal coding workflow focused on Git and explicit context. It supports repo maps, automatic commits, `/diff`, `/undo`, `/add`, `/drop`, `/read-only`, `/ask`, `/architect`, `/lint`, `/test`, `/voice`, images, web pages, clipboard paste, editor prompt entry, and extensive model/provider support.

Distinctive ideas:

- Repo map built with tree-sitter to orient the model without loading every file.
- Git-native safety: automatic commits, diffs, undo, and familiar Git review flows.
- Explicit file inclusion and read-only context control.
- Automatic lint/test loops after edits.
- Voice input and image/web context.
- Architect/editor mode using separate models.

Relevant sources:

- `https://github.com/Aider-AI/aider`
- `https://aider.chat/docs/repomap.html`
- `https://aider.chat/docs/git.html`
- `https://raw.githubusercontent.com/Aider-AI/aider/main/aider/website/docs/usage/commands.md`

### OpenCode

OpenCode is an open-source terminal coding agent with plan/build agents, a general subagent, desktop app, multi-provider support, and strong terminal UX.

Distinctive ideas:

- Built-in build and read-only plan agents.
- A general subagent for complex searches and multistep tasks.
- Open-source desktop companion.
- Broad install and packaging support.

Relevant sources:

- `https://github.com/anomalyco/opencode`
- `https://opencode.ai/docs/agents`

### Pi Coding Agent

Pi is a minimal, open-source terminal coding agent built around hackability. Its docs explicitly frame the product as "primitives, not features": a small core plus TypeScript extensions, skills, prompts, themes, and installable packages.

Distinctive ideas:

- Tree-structured sessions: `/tree` can jump to an earlier point and continue on a new branch, with labels/bookmarks, filtering, HTML export, and GitHub gist sharing.
- Runtime steering: pressing Enter while the agent is working sends a steering message after the current tool and interrupts remaining tools; `Alt+Enter` queues follow-up work after the current turn.
- Broad provider support: Anthropic, OpenAI, Google, Azure, Bedrock, Mistral, Groq, Cerebras, xAI, Hugging Face, Kimi, MiniMax, OpenRouter, Ollama, and custom providers/models.
- Mid-session model switching with context handoff across provider-native formats.
- Headless modes: print mode, JSON event streams, RPC over stdin/stdout, and an SDK for embedding Pi in other tools.
- Extension surface for tools, slash commands, keyboard shortcuts, events, TUI views, context injection/filtering, permission gates, MCP, subagents, protected paths, and sandboxing.
- Context engineering through minimal prompts, `AGENTS.md`, `SYSTEM.md`, skills, markdown prompt templates, automatic compaction, and extension-driven history filtering.

Pi deliberately omits some features from core: built-in MCP, subagents, plan mode, TODOs, permission popups, and background bash. Its position is that those should be extensions, visible files, tmux/container workflows, or external tools.

Relevant sources:

- `https://pi.dev`
- `https://mariozechner.at/posts/2025-11-30-pi-coding-agent/`
- `https://hn.algolia.com/?query=pi.dev%20coding%20agent`

## User Discussion Signals

Across public forums and issue trackers, the strongest themes were:

- Users love agents that can iterate through implementation, testing, and fixes without needing constant micromanagement.
- Users still want tight control: clear diffs, edit approval, command approval, command history, and reliable rollback.
- Token burn and quota opacity are recurring pain points. Codex GitHub issues around fast usage drain attracted hundreds of comments/reactions.
- Provider flexibility matters. Users dislike being locked into one vendor or one subscription model, and Aider's highly discussed issues include Copilot/local/other provider requests.
- Large-context codebase analysis is valued, but users often combine tools manually: for example, using Gemini CLI as a read-only large-context helper while Claude/Codex performs edits.
- Sandboxing and credential isolation are becoming urgent. Public discussions mention prompt injection, malicious README/package attacks, syscall-level wrappers, and policy engines for agent actions.
- Frontend users want the agent to inspect real browser state: screenshots, console logs, DOM, and Playwright-style flows.
- Verification is a decisive unlock. Users repeatedly say coding agents become useful when they can run tests, lint, browser automation, fuzzing, or other executable checks.
- There is appetite for agent orchestration, but mostly as a pragmatic way to split exploration, compare model opinions, or run long tasks in parallel.
- Pi-related discussions show a strong countercurrent toward minimal, hackable agents: users want the core to stay understandable while extensions/packages handle specialized workflows.
- Queueing and steering matter more than they first appear. Pi's "True Queue" discussion highlights that exposing future tasks to the model can cause goal anchoring, so queued work should sometimes be hidden until the current task is done.
- Local/open model workflows remain important. Pi is discussed as a useful harness for Kimi, GLM, Gemma, Qwen, Ollama, and other local or OpenAI-compatible setups.

Sources sampled:

- HN: Claude Code/Codex/Aider coding-agent discussions via Algolia.
- HN: security/permission threads for AI coding agents and sandbox wrappers.
- HN: Pi minimal terminal coding harness and pi.dev coding-agent discussions.
- Reddit `r/ChatGPTCoding`: threads comparing Claude Code, Codex CLI, Gemini CLI, Aider, GLM, and workflows that chain multiple agents.
- GitHub: Aider issue `#2227` Copilot provider, `#172` other/local LLMs, `#3362` Claude Code inspiration, `#3086` reasoning visibility.
- GitHub: Codex issues around token burn, remote development, desktop support, auth, and stream reliability.
- GitHub: Gemini CLI issues around subscription detection, capacity/429s, model updates, and command UX.
- Pi docs and Mario Zechner's Pi design/benchmark writeup.

## Feature Ideas Already Present Elsewhere But Missing or Partial in Keen

### 1. Checkpoint and Restore

Add automatic workspace checkpoints before write tools and dangerous bash commands. A checkpoint should capture:

- file snapshot, preferably through a shadow Git repository so it does not affect the user's repo
- conversation/session pointer
- pending tool call or command summary
- changed files and generated diff

Commands:

- `/checkpoint list`
- `/checkpoint restore <id>`
- `/checkpoint diff <id>`

Why it fits Keen:

- It strengthens trust without adding new model-facing tools.
- Keen already stores sessions and emits diffs.
- It complements the existing permission model.

Competitor reference: Gemini CLI checkpointing, Aider auto-commits and `/undo`.

Priority: high.

### 2. Tool Sandboxing Profiles

Add an optional sandbox execution layer for `bash` and eventually write/edit tools:

- `off`: current behavior
- `workspace-write`: default filesystem restrictions, current-style guard
- `strict-read`: no writes, no network, project read-only
- `container`: Docker/Podman-backed shell execution
- `seatbelt`: macOS sandbox-exec profile where available

Include "sandbox expansion" prompts when a command needs network or an outside path.

Why it fits Keen:

- Keen already has a filesystem guard but not process isolation.
- It addresses the main trust gap for autonomous command execution.
- It reduces pressure to approve every command manually.

Competitor reference: Gemini CLI sandboxing, Claude Code sandboxing docs, external tools like yolo-cage/Yu/grith.

Priority: high.

### 3. Git-Native Review and Rollback UX

Add first-class commands around the agent's changes:

- `/diff`: show current agent/user diff with file list
- `/undo`: revert last Keen-applied change set only
- `/commit`: generate a commit message and optionally run configured checks
- `/pr-summary`: summarize changed files for a pull request
- `/merge-help`: guided conflict resolution for rebase/merge states

Why it fits Keen:

- The repo is the developer's safety boundary.
- It is useful without cloud services.
- Keen already has a commit skill, review skill, diff emitter, and session memory.

Competitor reference: Aider Git integration, Claude/Gemini GitHub automation.

Priority: high.

### 4. Repo Map and Symbol-Aware Context

Build a lightweight repo map using tree-sitter, LSP, `ctags`, or a simple language-aware scanner. The map should summarize:

- top-level modules/packages
- exported symbols
- function/class names
- imports/dependency edges
- test files near implementation files

Expose it as an internal context source, not necessarily as a broad new LLM tool.

Why it fits Keen:

- Users value tight context control.
- It reduces repeated `grep`/`read_file` loops.
- It can stay local and deterministic.

Competitor reference: Aider repo map, Pi context engineering and extension-driven history filtering.

Priority: high.

### 5. Verification Hooks

Add project-local hooks that run after edits or before final response:

```json
{
  "after_edit": ["gofmt ./...", "go test ./..."],
  "before_final": ["go test -race ./..."],
  "on_failure": "feed_output_to_agent"
}
```

Start small:

- hook definitions in `.keen/hooks.json`
- command allowlist and timeouts
- visible output summary
- option to let the agent fix hook failures

Why it fits Keen:

- Keen's own AGENTS instructions already require repeated tests.
- Forum discussions repeatedly identify execution and verification as what makes agents valuable.
- It does not expand the model-facing tool surface.

Competitor reference: Aider `/lint` and `/test`, Claude Code hooks, Gemini GitHub Action workflows.

Priority: high.

### 6. Multimodal Image Input

Support attaching local images/screenshots to prompts when the selected model supports image input.

Use cases:

- frontend screenshots
- design mockups
- terminal screenshots
- error dialogs
- architecture diagrams

Why it fits Keen:

- The roadmap already lists image input.
- It fills a clear gap for UI/debugging work.

Competitor reference: Aider images, Gemini multimodal generation/debugging, Claude Code visual surfaces.

Priority: medium-high.

### 7. Browser Inspection Skill or Tooling

Add a browser workflow, likely through MCP first:

- inspect current page DOM
- capture screenshot
- read console errors
- run Playwright checks
- optionally watch a dev server URL

This could be a bundled `frontend-debug` skill that teaches the agent to use a configured Playwright/Chrome MCP server.

Why it fits Keen:

- It keeps core small by leaning on MCP.
- User discussions specifically ask for screenshots, DevTools console, and DOM inspection.
- It would make frontend work much more reliable.

Competitor reference: Claude Chrome integration, Playwright-driven community workflows.

Priority: medium-high.

### 8. Streaming Headless JSON Events and RPC

Extend `keen run --format json` with `--format stream-json` to emit newline-delimited events:

- assistant text chunk
- reasoning chunk
- tool start/end
- diff
- permission request
- command output summary
- final result

Then add a stable RPC mode over stdin/stdout for tools that want to embed Keen without scraping terminal output.

Why it fits Keen:

- It makes Keen easier to embed in scripts, CI, editor plugins, and dashboards.
- It is a natural extension of existing stream events.

Competitor reference: Gemini CLI `stream-json`, Codex scripting, Pi print/JSON/RPC/SDK modes.

Priority: medium.

### 9. Provider Health, Cost, and Budget Controls

Add visible cost/usage controls:

- per-turn token and estimated cost summary
- session cost summary
- provider health indicators after repeated failures
- `/budget set <amount|tokens>`
- "prefer cheap model for exploration, strong model for edits" mode
- warning before compaction or huge context sends

Why it fits Keen:

- User discussions show quota/token opacity is a major source of frustration.
- Keen already tracks usage events and provider metadata.
- This would differentiate Keen as predictable and economical.

Competitor reference: Aider token/cost displays; Codex/Gemini issue volume shows pain.

Priority: medium.

### 10. Persistent Project Memory

Implement explicit, editable memory files:

- `.keen/memory.md` for project-local memory
- `~/.keen/memory.md` for user-global preferences
- optional `.keen/system.md` for project-specific system prompt additions or overrides
- `/memory add`, `/memory edit`, `/memory show`

Rules:

- never silently mutate memory
- show memory in `/status`
- include a timestamp and source turn when adding memory

Why it fits Keen:

- The roadmap already includes persistent memory.
- It aligns with Keen's preference for legible context over opaque hidden state.

Competitor reference: Claude memory/CLAUDE.md workflows, Gemini `GEMINI.md`, Aider context controls, Pi `AGENTS.md`/`SYSTEM.md` and prompt templates.

Priority: medium.

### 11. Local Model and Router Support

Add Ollama, LM Studio, and generic OpenAI-compatible local endpoints as first-class provider entries.

Useful defaults:

- local model warnings when tool-calling support is weak
- read-only mode recommendation for small/local models
- model capability metadata: tool calling, image input, reasoning, context size
- model registry import from OpenRouter/models.dev-style metadata where practical
- mid-session model switching that keeps a best-effort provider-neutral context

Why it fits Keen:

- Forum users want vendor flexibility and cost control.
- Keen already has OpenAI-compatible clients.
- This keeps Keen usable when subscription/provider policy changes.

Competitor reference: Aider local/provider support, OpenCode, Gemini API/Vertex options, Pi provider registry and local/Ollama support.

Priority: medium.

### 12. Subagents and Delegation

Add bounded parallel workers:

- `explorer`: read-only code search and summary
- `reviewer`: review a diff or plan
- `tester`: run verification and report failures
- `implementer`: own a narrow file/module write set

Keep this explicit and sparse. Avoid making multi-agent orchestration the core product.

Why it fits Keen:

- Roadmap already lists subagents.
- Users are already comparing multiple agents manually.
- It helps on large codebases without bloating one main context.

Competitor reference: Claude Code subagents, Codex subagents, OpenCode general subagent, Pi extension/self-spawn patterns through print mode or tmux.

Priority: medium.

### 13. Team/CI Automation

Add a small GitHub Action wrapper around `keen run`:

- PR review
- issue triage
- release notes
- failed CI analysis
- mention-triggered help

Why it fits Keen:

- It builds on headless mode.
- It lets Keen participate in team workflows without needing a cloud service.

Competitor reference: Gemini CLI GitHub Action, Claude GitHub Actions/GitLab CI.

Priority: medium-low unless Keen wants team adoption.

### 14. Session Trees, Export, and Share

Add:

- `/session tree` to show branches and checkpoints in the current conversation
- `/session branch <event-id>` to continue from an earlier point without losing the current branch
- `/session export markdown`
- `/session export jsonl`
- `/session export html`
- optional redaction of secrets and command output
- reproducible "debug bundle" for bug reports

Why it fits Keen:

- The project already values paper trails in `.ai-interactions`.
- It helps users review and share agent work.
- Branching improves exploration: users can ask "what if we tried a different approach?" without throwing away the current path.

Competitor reference: Pi session trees, HTML export, and gist sharing; Claude session sharing/remote continuity; Keen roadmap.

Priority: medium.

### 15. Steering and Follow-Up Queue

Add a way to steer the agent while it is working:

- mid-run steering message delivered after the current tool finishes
- queued follow-up task that is hidden from the model until the current task completes
- visible queue with `/queue`, `/queue cancel <id>`, and `/queue reorder`
- separation between "interrupt/steer current work" and "do this next"

Why it fits Keen:

- Keen already has `/btw` for side questions, but it does not fully cover steering or hidden follow-up queues.
- Users often notice a problem while the agent is running and need to redirect without killing the whole turn.
- Hiding future tasks can reduce goal anchoring and rushed intermediate work.

Competitor reference: Pi runtime steering and True Queue extension discussion.

Priority: medium-high.

### 16. Extension and Package System

Turn skills into a broader extension surface while keeping the core conservative:

- installable packages containing skills, slash commands, hooks, themes, and MCP server recipes
- typed extension API for registering commands, context providers, event listeners, and status-bar/TUI elements
- explicit permission declarations for extension capabilities
- project-local and user-global package scopes

Why it fits Keen:

- Keen already has skills and MCP integration, so a package boundary would make workflows easier to share.
- A small extension API lets specialized features grow outside the core binary.
- Permission declarations fit Keen's guard-oriented architecture.

Competitor reference: Pi packages/extensions, Claude custom commands/hooks/SDK.

Priority: medium.

## Ideas I Did Not Find as First-Class Features in Mainstream Coding Agents

These may exist in niche tools or user scripts, but I did not find them as prominent first-class features in the agent docs and discussion sources sampled.

### A. Evidence Ledger

Keen could maintain a visible `EvidenceLedger` per session:

- requirements extracted from the user prompt
- assumptions made
- files read
- files changed
- commands run
- checks passed/failed
- unresolved risks
- decisions and reversals

Unlike raw transcript replay, this would be a structured artifact the user can inspect and the agent can update. It could live in session state and optionally export to `.ai-interactions/`.

Why it is valuable:

- Users want agents to be auditable, especially in professional settings.
- It complements Keen's existing `TurnMemory` philosophy.
- It makes handoff to humans or future agents easier.

Suggested command:

- `/evidence`
- `/evidence export`

### B. Verification Contract

Before making non-trivial edits, Keen could generate a small verification contract:

- expected behavior
- affected modules
- minimum checks
- optional deeper checks
- what cannot be verified locally

After implementation, Keen reports against that contract rather than only saying tests passed.

Example:

```markdown
Verification contract:
- Unit tests covering config resolution pass.
- Race tests pass for session store.
- Manual risk: OAuth callback behavior not exercised locally.
```

Why it is valuable:

- It turns vague "I tested it" into a concrete acceptance record.
- It fits coding work better than generic confidence scores.

### C. Context Budget Planner

Before a large task, Keen can estimate context strategy:

- direct file reads
- repo map
- grep-first search
- external docs fetch
- subagent exploration
- compaction timing

It can then show a compact plan like:

```text
Context plan: repo map + targeted reads; avoid loading generated files; reserve 30% window for test output.
```

Why it is valuable:

- Users complain about token burn and context waste.
- Keen's lean context model is a natural foundation for this.
- Pi has dynamic context hooks and automatic compaction, but a visible budget planner would make the strategy explicit to users before the spend happens.

### D. Agent Action Policy File

Introduce `.keen/policy.yaml` as a repo-visible policy for agent actions:

```yaml
allow:
  read:
    - "."
  write:
    - "internal/**"
    - "docs/**"
deny:
  bash:
    - "curl * | sh"
    - "npm install -g *"
require_approval:
  bash:
    - "git push *"
    - "go get *"
```

This is narrower and more actionable than a general instruction file. It could also honor community files such as `.no-llm`, `AI_POLICY.md`, or a future `.llm-permissions`.

Why it is valuable:

- Forum discussions show maintainers want repo-level AI contribution policies.
- It keeps permission decisions deterministic and reviewable.

### E. Cross-Provider Second Opinion

Add a command that sends the same plan/diff/question to a second configured model:

- `/second-opinion plan`
- `/second-opinion diff`
- `/second-opinion bug`

This is not the same as subagents. It is specifically for adversarial review across models/providers.

Why it is valuable:

- Users already compare Claude, Codex, Gemini, and GLM manually.
- It reduces model monoculture failures.
- Keen's multi-provider support makes this easier than for single-vendor agents.
- Pi supports mid-session model switching and provider handoff; Keen could differentiate by making the second model explicitly adversarial and scoped to a plan, diff, or bug hypothesis.

### F. Failure Replay Pack

When a task fails, Keen can produce a local replay bundle:

- user prompt
- model/provider
- relevant session events
- changed files list
- failed commands and truncated output
- environment summary
- reproducible command where possible

Suggested command:

- `/failure-pack`

Why it is valuable:

- Agent failures are hard to report and debug.
- Keen already stores most of this data in sessions and turn memory.

### G. Agent-Friendly Accessibility Checks

Create a frontend verification workflow that goes beyond screenshots:

- run Playwright
- run axe-core
- optionally drive screen-reader tooling where available
- produce user-task-based accessibility findings

Why it is valuable:

- Public discussion specifically called out a gap around AI-assisted screen-reader testing.
- Few coding agents make accessibility validation a first-class coding loop.

## Recommended Roadmap

### Near Term

1. Checkpoint/restore.
2. `/diff`, `/undo`, and improved Git change review.
3. Verification hooks.
4. Repo map.
5. Cost/token budget visibility.
6. Steering and hidden follow-up queue.

These are highest leverage because they improve trust and daily ergonomics without changing Keen's core architecture much.

### Medium Term

1. Sandboxing profiles.
2. Persistent project memory.
3. Image input.
4. Browser inspection through MCP or bundled skill.
5. Streaming headless JSON events.
6. Local model/router providers.
7. Session tree/export/share.
8. Extension/package boundary for skills and hooks.

These expand Keen's usefulness while still preserving a compact core.

### Later

1. Subagents and cross-provider second opinion.
2. GitHub Action wrapper.
3. Evidence ledger and verification contract.
4. Agent action policy file.

These are valuable but should come after the safety and verification foundation is solid.

## Highest-Impact Proposal

The best coherent product direction is:

> Make Keen the terminal coding agent with the clearest control loop: small tool surface, visible diffs, automatic rollback, deterministic verification, and explicit policy.

That suggests a first feature bundle:

- Checkpoints before edits.
- `/diff` and `/undo`.
- Hook-based verification.
- Cost/token visibility.
- Steering/follow-up queue.
- A simple repo map.

This bundle directly addresses what users praise and complain about in other agents: they want autonomy, but only when the tool is easy to audit, revert, and verify.
