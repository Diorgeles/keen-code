# A Tour of Keen Code

This is not a README. It is a walkthrough for anyone who wants to understand how this project was built, what decisions were made along the way, and why the repository looks the way it does.

---

## The One-Sentence Summary

Keen Code is a terminal-based AI coding agent — and every line of its code was written by AI agents, not humans.

---

## The Experiment

The core idea behind Keen Code is a question: *can you build a coding agent using coding agents?*

The answer this project demonstrates is yes. The human role here was strictly that of an orchestrator — writing requirements, reviewing designs, giving feedback, and testing the product. The actual implementation was always delegated to an AI agent.

Over time, a range of agents were used: Cursor, Windsurf, Claude Code, OpenCode, Codex CLI, and Kimi CLI. Importantly, only one agent was active at any given time — no multi-agent orchestration.

---

## The `.ai-interactions` Folder

This is the most distinctive part of the repository. Everything in `.ai-interactions` is a record of the human-AI collaboration that produced the code.

```
.ai-interactions/
├── prompts/     ← what the human sent to the agent
├── outputs/     ← what the agent produced (plans, designs, specs)
└── tasks/       ← standalone task briefs for specific issues
```

**Prompts** are almost entirely preserved in chronological order. Reading through them is the closest thing to watching the project's thought process unfold in real time. They range from the original product idea to very granular follow-up questions inside a single feature.

**Outputs** are the plans, design documents, and architecture writeups that agents produced before writing any code. These are the "specs" that guided the actual implementation. They are not documentation written after the fact — they were written first, then reviewed, then used to build.

**Tasks** hold briefs for work that was scoped separately, usually tied to a GitHub issue.

---

## The Development Cycle

Every feature followed the same loop:

1. **Spec** — the human wrote a prompt describing what they wanted
2. **Plan** — the agent produced a design document saved in `outputs/`
3. **Review** — the human reviewed the plan, asked questions, pushed back
4. **Task** — the agent was given a concrete task list to implement
5. **Feedback** — the human tested the result and sent corrections inline

This cycle repeats within each phase and within individual features. A single prompt file often contains a dozen numbered follow-ups, capturing the back-and-forth of a real working session.

---

## The Phases

Development is organized into five phases, each building on the last.

### Phase 1 — Foundation

Started with a raw product idea in `prompts/phase-1/prd.md`. From there, an agent wrote a full system architecture RFC (`outputs/phase-1/output-1_rfc.md`), which was then reviewed, revised, and turned into a concrete implementation plan. This phase produced the project skeleton: configuration, file access guard, git-awareness, and a basic CLI shell.

### Phase 2 — The REPL and LLM Integration

This phase wired up the interactive terminal UI and connected it to a real LLM. There was also a pivot: the project initially considered `langchain-go`, evaluated alternatives, and landed on Firebase Genkit as the LLM framework. Genkit remains in use today, but only for Gemini. As the project matured, dedicated OpenAI and Anthropic SDKs were brought in directly — giving those providers full access to native features like reasoning content and prompt caching that a generic framework could not easily expose. Later in this phase, the main REPL file had grown large enough to warrant a full refactor — which was also agent-driven.

### Phase 3 — Tools

This is where the agent gained the ability to actually touch files and run commands. Each tool — `read_file`, `glob`, `grep`, `bash`, `write_file`, `edit_file` — was designed, reviewed, and built one at a time. The permission system (which asks the user before touching anything outside the working directory) also took shape here.

### Phase 4 — Providers, Polish, and Distribution

DeepSeek support was added in this phase, which required solving a non-obvious compatibility issue with the `deepseek-reasoner` model. Distribution was designed and shipped: GoReleaser for binaries, an npm wrapper, and an install script. Context management — a progress bar showing how full the model's context window is — was also added here, along with `/compact` to summarize and trim long conversations.

### Phase 5 — Sessions and Memory

Persistent sessions were introduced so conversations survive across restarts. A `/sessions` command lets users browse and resume past sessions. Tool memory was also explored here: a mechanism to give the model a compact record of what it changed across turns, without cluttering the visible chat.

---

## How Decisions Actually Got Made

Reading the prompts reveals something honest about how this kind of work goes. Decisions were not made upfront and then executed cleanly. They evolved.

For example:
- The project started with official provider SDKs, moved to `langchain-go`, evaluated alternatives, and settled on Genkit — all within Phase 2.
- Tool memory went through several redesigns: XML tags, then a "structured TurnMemory" system, with the human reversing course multiple times as the tradeoffs became clearer.
- The roadmap for Phase 5 originally included an "agent skills" system, then it was added back with defaults, then removed entirely. The prompts record every turn.

These are not mistakes. They are the natural shape of iterative development, just unusually visible here because the conversation was saved.

---

## What's in the Codebase Itself

```
cmd/           ← entry point
internal/
  cli/         ← REPL, model selection, streaming, widgets
  filesystem/  ← file access guard, git-awareness
  llm/         ← LLM clients (Anthropic, OpenAI, Genkit), system prompt, message formatting
  session/     ← persistent session storage and replay
  tools/       ← each built-in tool as its own file
  config/      ← configuration loading
  logging/     ← structured logging
providers/     ← registry of supported models and their context windows
npm/           ← npm wrapper for installation via `npm install -g keen-code`
scripts/       ← shell-based install script
.github/       ← CI and release workflows
```

The structure maps closely to the RFC that was written before a single line of Go existed.

---

## The Changelog as a Timeline

`CHANGELOG.md` is the most compact summary of what was built when. It starts at `v0.1.0` — the first working release — and shows how the project grew from a basic REPL with file tools to a full session-aware agent with context management, compaction, persistent memory, and thinking mode support.

Reading the phases in `.ai-interactions/` and the versions in `CHANGELOG.md` side by side gives the clearest picture of the project's arc.
