# Keen Code Roadmap - April 2026

## Purpose

This roadmap turns the current product discussion into a concrete direction for
making Keen Code realistically useful for software engineers and competitive
with modern CLI-based coding agents.

The product is intentionally minimal today. That is a strength as long as the
next features are chosen carefully. The roadmap below keeps that constraint in
mind:

- Preserve a lightweight, terminal-first experience
- Add capabilities that improve daily engineering workflows
- Avoid feature bloat that can be handled through extensibility
- Prefer strong infrastructure over shipping lots of bundled behavior

## Current Baseline

Keen Code currently provides:

- A REPL-based coding agent UI
- A small built-in toolset: `read_file`, `glob`, `grep`, `write_file`,
  `edit_file`, `bash`
- Filesystem guard and interactive permission prompts
- Model/provider selection
- Context compaction
- Basic project-instruction loading from files like `AGENTS.md`

This is enough for focused single-agent tasks, but not yet enough for long,
high-trust, team-oriented engineering workflows.

## Product Principles

These principles should shape roadmap decisions:

- Keep the core small and composable
- Prefer infrastructure over hardcoded workflows
- Make risky behavior explicit and reviewable
- Optimize for real software engineering tasks, not toy demos
- Support team adoption through repo-local configuration and conventions

## Tier Definitions

- Tier 1: Required for daily usability and trust
- Tier 2: High-leverage features that expand capability significantly
- Tier 3: Advanced workflow features that improve competitiveness and polish

## Tier 1: Core Usability And Trust

### 1. Persistent Sessions And Resume ✅

**What it should do**

Allow users to persist conversation state and later resume it from disk. A
session should capture enough context to continue work without starting over.

**Why it matters**

Serious engineering tasks often span hours or days. Losing state when the
process exits makes the tool less practical than competing agents.

**Key behaviors**

- Save session history locally
- Reopen the most recent session or choose from prior sessions
- Preserve compacted state and relevant metadata
- Support explicit session naming for longer tasks

**Considerations**

- Storage format should be stable and inspectable
- Session history may contain sensitive code or prompts, so local storage
  behavior must be clear
- Large sessions need compaction-aware persistence

### 2. Autopilot Mode

**What it should do**

Add an explicit `autopilot` mode that bypasses interactive permission prompts
for non-`bash` tools while keeping the existing permission behavior as the
default mode.

**Why it matters**

Interactive prompts are useful by default, but they create friction during
trusted in-repo editing workflows. An explicit autopilot mode reduces that
friction without making the permission model ambiguous.

**Key behaviors**

- Keep the current permission system unchanged in normal mode
- Add an explicit `autopilot` mode users can opt into
- In `autopilot`, bypass interactive permission prompts for non-`bash` tools
- Keep `bash` permission handling separate from autopilot
- Surface the active mode clearly in the UI

**Considerations**

- Autopilot should bypass prompts, not the core filesystem guard
- Blocked paths should remain blocked
- The behavior must be easy to explain: normal mode prompts, autopilot
  auto-allows non-`bash` actions within existing guardrails
- The UI should make autopilot highly visible so users do not forget it is on

### 3. Web Search And Fetch

**What it should do**

Allow the agent to retrieve external documentation and web content when the
task requires current or external information.

**Why it matters**

Modern software work regularly depends on upstream docs, release notes,
libraries, API references, and issue trackers.

**Key behaviors**

- Search the web for relevant sources
- Fetch pages or documentation content
- Present citations or source references in responses
- Respect user approval and clear network access rules

**Considerations**

- Network access should remain explicit and policy-controlled
- Source quality matters; official docs should be preferred when possible
- Output should distinguish retrieved facts from model inference

## Tier 2: Capability Expansion

### 4. Native Code Review Mode

**What it should do**

Provide a first-class way to review local changes, diffs, or specific files in
review mode instead of implementation mode.

**Why it matters**

Code review is one of the most common and highest-value software engineering
workflows for coding agents.

**Key behaviors**

- Review unstaged changes, staged changes, or arbitrary diffs
- Report findings ordered by severity
- Focus on bugs, regressions, risks, and missing tests
- Optionally generate fix suggestions without applying them automatically

**Considerations**

- The output format should be concise and trustworthy
- The review mode should avoid drifting into summary-heavy responses
- Good review quality may require diff-aware prompting and git-aware helpers

### 5. MCP Support

**What it should do**

Add Model Context Protocol support so Keen Code can connect to external tools
and systems through a standard integration layer.

**Why it matters**

MCP is the cleanest path to extensibility without turning Keen Code into a
collection of one-off integrations.

**Key behaviors**

- Discover and register MCP servers
- Expose MCP-provided tools and resources to the model
- Respect existing permission and approval flows
- Surface connection and failure status in the UI

**Considerations**

- Tool trust and provenance must be visible to the user
- Authentication and local configuration should be straightforward
- MCP should complement built-in tools, not create overlapping confusion

### 6. Subagents And Delegation

**What it should do**

Let the agent delegate bounded subtasks to additional agents when parallel or
role-specific work would improve throughput.

**Why it matters**

Large engineering tasks benefit from splitting exploration, implementation,
testing, and review instead of forcing one linear agent thread.

**Key behaviors**

- Spawn subagents for scoped tasks
- Keep ownership boundaries clear
- Allow result collection and integration into the main thread
- Make delegated work visible in the interface

**Considerations**

- Delegation should not become chaotic or opaque
- Approval and filesystem boundaries still apply
- The main thread should remain understandable to the user

### 7. LSP And Diagnostics Integration

**What it should do**

Integrate with language servers or equivalent diagnostics sources to provide
semantic navigation and actionable feedback.

**Why it matters**

Text search is useful, but semantic code understanding is much stronger for
non-trivial codebases.

**Key behaviors**

- Retrieve diagnostics
- Support definitions, references, and symbols
- Support rename previews and other semantic actions where safe
- Feed compiler and editor-quality feedback into the model loop

**Considerations**

- Language support should degrade gracefully when no server is available
- Diagnostics should be presented compactly
- Integration should avoid making startup heavy or fragile

### 8. Custom Commands And Reusable Workflows

**What it should do**

Allow users and teams to define reusable commands that package common prompts,
instructions, or workflow entry points.

**Why it matters**

Teams often repeat the same tasks: fixing tests, preparing releases, reviewing
PRs, or auditing changes. Commands reduce friction.

**Key behaviors**

- Support project-local and user-global custom commands
- Expose command descriptions and discoverability in the UI
- Allow commands to compose with built-in product features
- Keep commands easy to version in a repository

**Considerations**

- Commands should remain inspectable and not hide dangerous behavior
- Naming and precedence rules should be simple
- Commands should complement core features instead of replacing them
- Can be implemented using agent skills

### 9. Persistent Memory Beyond Project Instructions

**What it should do**

Provide a way to retain useful learned context across sessions beyond static
instruction files like `AGENTS.md`.

**Why it matters**

Teams accumulate local knowledge that is too specific for the base prompt but
too valuable to rediscover repeatedly.

**Key behaviors**

- Store durable notes or structured memory
- Separate project memory from user-global memory
- Make stored memory visible and editable
- Allow explicit opt-in and cleanup

**Considerations**

- Memory should be reviewable and not silently mutate behavior
- Stored context must not become stale or misleading
- Repo-local memory should be easy to version or ignore intentionally

## Tier 3: Competitiveness And Workflow Depth

### 10. Hooks And Automation

**What it should do**

Provide hook points around agent and tool activity so users can run validation,
formatting, notifications, or policy checks automatically.

**Why it matters**

Hooks make the agent fit established engineering workflows instead of requiring
teams to work around it manually.

**Key behaviors**

- Support pre-tool and post-tool hooks
- Support validation hooks after edits
- Support notifications or logging hooks
- Keep hook execution visible and debuggable

**Considerations**

- Hook failures must be understandable
- Hooks should not silently introduce unsafe behavior
- Execution order and retry behavior need clear rules

### 11. Richer Built-In Tools

**What it should do**

Expand the built-in toolset only where the capability is foundational and too
common to leave entirely to external integrations.

**Why it matters**

A small toolset is good, but some missing primitives create unnecessary model
friction and poor reliability.

**Candidate additions**

- Directory listing
- Move or rename files
- Better diff inspection
- Task or todo tracking
- Structured patch application
- Test runner helpers

**Considerations**

- Every new built-in tool adds maintenance and prompt complexity
- Avoid adding tools that are better handled through MCP
- Prefer tools that materially improve reliability over convenience-only tools

### 12. Worktree And Branch Isolation

**What it should do**

Support isolated work environments for tasks that should not modify the current
working tree directly.

**Why it matters**

Isolation improves safety for larger autonomous tasks, experiments, and review
flows.

**Key behaviors**

- Create or target dedicated worktrees or branches
- Keep task state associated with the isolated workspace
- Make isolation explicit in the UI

**Considerations**

- Git interactions become more complex quickly
- Users need clear visibility into where edits are happening
- This feature becomes more valuable after sessions and delegation exist

### 13. Session Sharing And Export

**What it should do**

Allow users to export or share session artifacts for collaboration, debugging,
or reproducibility.

**Why it matters**

Shared sessions help teams understand agent behavior, review outputs, and file
useful bug reports.

**Key behaviors**

- Export session transcripts and metadata
- Share compacted context and decisions
- Support local export first, remote sharing later

**Considerations**

- Shared sessions may include sensitive data
- Export format should be easy to inspect
- Sharing is lower priority than local usability and trust features

### 14. Image Input And Multimodal Workflows

**What it should do**

Allow the agent to use screenshots or image inputs when supported by the active
model and workflow.

**Why it matters**

This is especially useful for frontend work, design implementation, and certain
debugging tasks.

**Key behaviors**

- Attach local images to prompts
- Surface model capability constraints clearly
- Keep image use optional and lightweight

**Considerations**

- Not all providers or models will support this equally
- The UI and prompt pipeline should degrade gracefully
- This is useful, but not as foundational as the higher-priority items

## Proposed Implementation Sequence

The roadmap should not be built strictly by tier order alone. Dependencies and
infrastructure matter. A practical sequence is:

1. Persistent sessions and resume
2. Autopilot mode
3. Web search and fetch
4. Native code review mode
5. MCP support
6. Persistent memory
7. LSP and diagnostics
8. Custom commands and reusable workflows
9. Subagents and delegation
10. Hooks and automation
11. Richer built-in tools
12. Worktree and branch isolation
13. Session sharing and export
14. Image input

## Out Of Scope For The Near Term

These ideas may become useful later, but should not distract from the roadmap
above:

- Heavy cloud task orchestration
- Complex auto-triggered workflows
- Provider-specific feature divergence where it harms consistency
- Large numbers of built-in niche tools

## Success Criteria

Keen Code should feel competitive when a software engineer can:

- Start work in a repo and safely let the agent explore and edit
- Pause and resume multi-hour tasks without losing context
- Review code changes as easily as generating them
- Pull in external documentation when needed
- Reuse local team workflows through product features and custom commands
- Extend the tool through MCP instead of waiting on core changes

That should remain the bar for roadmap decisions: improve real engineering
utility without compromising clarity, control, or product simplicity.
