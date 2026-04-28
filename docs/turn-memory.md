# TurnMemory In Keen

## Table of Contents

- [The Idea](#the-idea)
- [How To Think About It](#how-to-think-about-it)
- [Lifecycle Of A Turn](#lifecycle-of-a-turn)
- [Why Keen Does This](#why-keen-does-this)
- [What The Next Turn Actually Gets](#what-the-next-turn-actually-gets)
- [How This Differs From Other Coding Agents](#how-this-differs-from-other-coding-agents)
- [What Is Distinctive About Keen's Approach](#what-is-distinctive-about-keens-approach)
- [Pros](#pros)
- [Cons](#cons)
- [When This Design Works Well](#when-this-design-works-well)
- [When Another Design Might Work Better](#when-another-design-might-work-better)
- [Bottom Line](#bottom-line)
- [Assistant Turn Reliability](#assistant-turn-reliability)

## The Idea

`TurnMemory` addresses a simple problem: a coding agent needs detailed tool state while it is actively working, but keeping every tool call forever makes later turns noisy, expensive, and harder to reason about.

So Keen splits memory into two layers:

- a temporary working memory for the current assistant turn
- a small summary that survives into later turns

That summary is `TurnMemory`.

In practice, the current turn may involve many tool calls, many tool results, and several internal loop iterations. But once the turn is over, Keen does not carry forward that entire raw trace. It keeps only a small summary of the parts most likely to matter later.

## How To Think About It

`TurnMemory` is not a transcript. It is not a full execution log, nor a hidden chain of thought. It is closer to a short note about the turn: "what changed during this turn that the next turn may need to know?"

Today that summary is intentionally narrow. It remembers facts such as:

- which files were changed
- which bash commands failed

That means Keen preserves outcomes rather than the entire path taken to reach them.

## Lifecycle Of A Turn

The model to keep in mind is:

1. A new user turn starts.
2. Keen gives the model the conversation history it has retained so far.
3. Inside that turn, the model may call tools repeatedly.
4. Those raw tool calls are available while the turn is still running.
5. When the turn finishes successfully, Keen summarizes the useful state into `TurnMemory`.
6. When the turn fails mid-loop, Keen saves the accumulated turn data as pending state (see "Assistant Turn Reliability" below).
7. On normal completion, the raw tool-call trace does not become part of the next-turn conversation state.

This means tool calls matter during execution, but they are temporary as long-term memory — unless the turn fails, in which case they are preserved for recovery.

## Why Keen Does This

The design is trying to balance three pressures:

- the agent needs high-fidelity state while solving the current task
- the next turn needs some continuity
- the conversation should not bloat with low-value tool chatter

Without a mechanism like `TurnMemory`, Keen would have two options:

- keep all tool traffic forever, which pollutes context
- keep none of it, which loses continuity after edits or failed commands

`TurnMemory` is a middle ground. It keeps some continuity without keeping the entire trace. Today it is limited to file changes and failed bash commands. But it can be extended to remember other useful information in the future.

## What The Next Turn Actually Gets

On the next user turn, Keen does not replay the full prior tool trace back to the model as structured tool-call history.

Instead, the next turn gets:

- prior user messages
- prior assistant responses
- the compact turn-memory summary from earlier assistant turns
- any pending turn state from a prior failed turn (injected in provider-native format)

That matters because it changes the agent's behavior. Keen is implicitly saying:

- if a read-only fact is needed again, re-read it
- if a search result is needed again, re-run the search
- if a file was changed, remember that it changed
- if a shell command failed, remember that it failed
- if the prior turn failed mid-loop, resume from where it left off

So continuity is based on retained outcomes, not retained execution traces — with the exception of failed turns, where the full in-progress transcript is preserved for recovery.

## How This Differs From Other Coding Agents

Many coding agents fall into one of three common patterns.

### 1. Full-trace retention

Some agents keep a large amount of prior tool activity in the next prompt, either directly or through aggressive transcript replay. In those systems, the model may see a long history of file reads, searches, command output, and edits from earlier turns.

Keen is more selective than that. It does not treat the raw tool trace as long-term conversation state.

### 2. Hidden persistent scratchpad

Some agents keep a private server-side memory or execution state that survives across turns without being visible as part of the normal conversation. The user may see the agent "remember" prior tool work, even though that memory is not represented as plain conversation history.

Keen is more explicit and narrower. Its cross-turn memory is small and legible rather than broad and opaque.

### 3. Planner-state retention

Some agents retain structured state such as task graphs, subgoals, artifact inventories, or execution plans across turns. In those systems, long-term memory is not just conversation plus tools; it is an explicit task-state machine.

Keen's `TurnMemory` is simpler than that. It is not trying to be a planner database. It is a lightweight per-turn summary.

## What Is Distinctive About Keen's Approach

Keen's approach is distinctive in three ways:

- it separates "what the agent needed while thinking this turn" from "what should survive into future turns"
- it treats most tool activity as disposable unless it changed the workspace or exposed a failed command
- it keeps cross-turn memory small enough to be understandable by both the model and the user

This makes `TurnMemory` closer to a working summary than an archive.

## Pros

### Smaller context

One advantage is context discipline. Tool-heavy coding sessions can grow quickly if every file read, grep result, and command output is kept forever. `TurnMemory` limits that growth.

### Better signal-to-noise ratio

Most tool calls are not worth remembering verbatim. A search that found nothing, a file read, or a successful listing command usually matters only in the moment. By dropping those traces, Keen keeps later turns focused on what materially changed.

### More predictable continuity

Because the retained memory is narrow, it is easier to understand what the agent will actually carry forward. That can make multi-turn behavior easier to reason about.

### Encourages fresh reads of mutable state

This is valuable in coding work. Files may have changed, generated outputs may be stale, and search results may no longer reflect the current codebase. A design that favors re-reading over trusting old tool output can reduce stale-context errors. And guess what? Agents frequently reread files and search results in later turns anyway.

## Cons

### Loss of rich historical context

The obvious downside is that the next turn no longer has the full investigative trail. If an earlier tool output contained an important nuance and it was not reflected in the final answer or `TurnMemory`, that nuance is gone from the model-facing conversation state.

### More repeated tool work

Because read-only tool calls are not retained across turns, the agent may need to read files or search again in later turns. That is often the right tradeoff, but it can increase repeated work.

### Can reduce KV-cache continuity

Because Keen carries forward a compact summary instead of the prior raw tool-call trace, the next turn may not line up as closely with the previous prompt prefix. Depending on the provider and how prompt caching or KV reuse works, that can reduce cache continuity across turns.

This may increase cost or latency, but it is not guaranteed. The actual effect depends on the model provider, the caching strategy, and how much of the prompt still remains stable from turn to turn. Also tool outputs are frequently large so it is not obvious that the cost savings will outweigh the overhead of rerunning tools.

### Limited support for long, interdependent investigations

For very long debugging sessions, full raw traces can sometimes help because the agent can refer back to earlier experiments without redoing them. Keen's design is less suited to that style of persistent investigative memory.

## When This Design Works Well

`TurnMemory` works especially well when:

- the workspace is the main source of truth
- read-only tool calls are cheap to repeat
- the important cross-turn facts are mostly "what changed" and "what failed"
- the product values lean context over exhaustive replay

That is a good fit for many coding sessions.

## When Another Design Might Work Better

A heavier memory model may be better when:

- the agent does long investigations that depend on earlier observations
- external systems produce important outputs that are expensive or impossible to reproduce
- the product wants the model to reason over prior execution traces directly
- planning state is as important as file state

In those settings, a fuller transcript memory or a richer structured state model may work better than a narrow `TurnMemory`.

## Bottom Line

`TurnMemory` is a simple compression boundary.

Inside a turn, Keen allows rich tool-driven execution.

Across turns, Keen keeps only a small summary of the tool execution.

If a turn fails, Keen preserves the full in-progress transcript as pending state so the model can resume on the next attempt — but that state is temporary and in-memory only.

Compared with many other coding agents, Keen is more conservative about what deserves durable memory. The upside is cleaner context and clearer continuity. The downside is that some useful history is discarded.

## Assistant Turn Reliability

### The Problem

A single assistant turn can involve many loop iterations — reading files, running commands, editing code — before producing a final response. If the turn fails partway through (API error, empty response, max tool turns exhausted, or user interruption), all of that accumulated work is lost. The local variables holding the turn transcript are discarded when the goroutine exits. On the next user message, the model has no memory of the work it already performed and starts over from scratch.

### The Solution: Pending Turn State

Each LLM client stores a pending state field in the provider's native message format. When a turn ends abnormally and tool work has accumulated, the client saves the delta (everything beyond the initial message conversion) to this field. On the next `StreamChat` call, the pending state is injected into the conversation — spliced in before the new user message — so the model sees its prior work and can resume.

### Terminal States

A turn can end in three ways from the REPL's perspective:

| Event | Meaning | Pending state action |
|---|---|---|
| `Done` | Normal completion | Cleared (or never saved) |
| `Incomplete` | Turn ended abnormally but work was done | Saved for next call |
| `Error` | Turn failed before any tool work accumulated | Not saved |

The `Incomplete` event is the key addition. It tells the REPL that work happened but the turn did not finish — so the REPL should not append anything to `appState.messages` (the pending state on the client already has the full transcript).

### Characteristics

- **In-memory only.** Pending state does not survive process crashes or get written to the session file.
- **Provider-native format.** Each client stores messages in its own type, avoiding lossy conversion.
- **No re-execution.** Tools have side effects; the state captures outputs, not re-execution intent.
- **Sequential safety.** The REPL is single-threaded with respect to `StreamChat` calls, so no synchronization is needed.
- **Cleared on success.** If the recovery turn completes normally, pending state is cleared and the full assistant response goes into `appState.messages` as usual.

### Why Not Just Save to appState?

Saving the partial turn transcript into `appState.messages` would mean converting provider-native messages into the generic `Message` type and back — a lossy round-trip. It would also mean `appState` carries incomplete turn data that is not a proper assistant response. Keeping the state on the client avoids both problems.
