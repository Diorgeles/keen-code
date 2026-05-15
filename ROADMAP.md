# Roadmap

Keen Code is intentionally minimal. The goal is not to match every feature of every coding agent — it is to stay focused on what makes a terminal-first agent genuinely useful for day-to-day software engineering.

This roadmap is organized into three tiers. Tier 1 is what needs to exist before the tool is really trustworthy for serious work. Tier 2 expands capability meaningfully. Tier 3 is about depth and polish.

Items marked ✅ are shipped.

---

## Tier 1 — Core Usability and Trust

### Autopilot Mode
An opt-in mode that bypasses interactive permission prompts for file operations, while keeping `bash` gated as always. The filesystem guard and blocked-path rules remain in place — autopilot just removes the friction of approving every read and write within the working directory. The active mode will be clearly surfaced in the UI.

### Web Search and Fetch
Let the agent pull in external documentation, release notes, or API references when a task requires current information that is not in the codebase. Network access will remain explicit and policy-controlled.

---

## Tier 2 — Capability Expansion

### Native Code Review Mode

✅ - Implemented using Agents Skills.

A first-class way to review changes rather than implement them. Point it at unstaged changes, a diff, or specific files and get findings ordered by severity — bugs, regressions, risks, missing tests. Optionally generate fix suggestions without applying them automatically.

### MCP Support

Add [Model Context Protocol](https://modelcontextprotocol.io) support so Keen can connect to external tools and systems through a standard layer. This is the extensibility path — instead of bundling one-off integrations, MCP lets users bring their own tools without waiting on core changes.

### Custom Commands and Reusable Workflows

✅
Define reusable slash commands that package common prompts or workflow entry points. Commands can live in the repo (team-shared) or in user config (personal). Think `/review`, `/release-notes`, `/fix-tests` — the kind of repeated tasks every team has. Agent `skills` can be leveraged for this.

### Persistent Memory
Retain useful context across sessions beyond static instruction files like `AGENTS.md`. Project-local and user-global memory, kept visible and editable so it never silently mutates behavior.

### Subagents and Delegation
Let the agent delegate bounded subtasks — exploration, implementation, testing, review — to additional agents running in parallel. Keeps the main thread clean while handling larger tasks. All existing permission and filesystem boundaries still apply.

---

## Tier 3 — Workflow Depth

### Hooks and Automation
Hook points around tool activity for running formatters, validators, linters, or notifications automatically after edits. Keeps Keen Code fitting into established engineering workflows rather than working around them.

### Session Sharing and Export
Export or share session transcripts for collaboration, debugging, or reproducibility. Local export first; remote sharing later.

### Image Input
Attach local images or screenshots to prompts for models that support it. Most useful for frontend work and visual debugging tasks.

---

## Out of Scope (for now)

- Cloud task orchestration or remote agent execution
- Complex auto-triggered workflows
- Large collections of bundled niche tools
- Provider-specific features that would create inconsistent behavior across providers

---

## Contributing

If something on this list matters to you, issues and PRs are open. The [issue templates](https://github.com/mochow13/keen-code/issues/new/choose) include a provider/model request form for additions to `providers/registry.yaml`, and `AGENTS.md` has the short guide for adding a new tool.
