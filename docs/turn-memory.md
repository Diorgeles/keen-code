# TurnMemory In Keen

## Table of Contents

- [The Idea](#the-idea)
- [Memory Layers](#memory-layers)
- [Lifecycle Of A Turn](#lifecycle-of-a-turn)
- [What The Next Turn Gets](#what-the-next-turn-actually-gets)
- [Historical Tool Activity](#historical-tool-activity)
- [Why Keen Does This](#why-keen-does-this)
- [Tradeoffs](#tradeoffs)
- [Assistant Turn Reliability](#assistant-turn-reliability)
- [Bottom Line](#bottom-line)

## The Idea

`TurnMemory` addresses a simple problem: a coding agent needs detailed tool state while it is actively working, but keeping every tool call and result forever makes later turns noisy, expensive, and harder to reason about.

Keen therefore treats the end of a turn as a compression boundary. The raw execution trace is discarded after successful completion and replaced by a small, provider-neutral summary.

`TurnMemory` is not a transcript, hidden chain of thought, or planner database. It retains only:

- durable outcomes that are likely to matter later, such as changed files and failed bash commands
- compact historical tool activity that preserves where real tool execution occurred between assistant prose segments

It never retains file contents, search results, command output, arbitrary tool input, MCP arguments, or MCP result content.

## Memory Layers

Keen uses four related forms of state:

| Layer | Lifetime | Contents | Purpose |
|---|---|---|---|
| Current-turn execution | One active assistant turn | Provider-native tool calls and results | High-fidelity tool-loop reasoning |
| Durable outcome memory | Later turns and persisted sessions | Changed files and failed bash commands | Preserve materially useful outcomes |
| Historical tool activity | Later turns and persisted sessions | Tool, bounded target, success/error status, prose offset | Preserve the protocol shape of real execution |
| Pending provider state | Until an interrupted turn resumes or completes | Provider-native in-progress messages | Recover incomplete tool loops without lossy conversion |

The historical activity layer does not retain what a tool returned. A later turn that needs a file, command result, search result, MCP response, or external state must query it again.

## Lifecycle Of A Turn

1. A new user turn starts with retained conversation messages and `TurnMemory` from earlier assistant turns.
2. During the active turn, the provider may emit assistant prose, tool calls, and tool results over several loop iterations.
3. The REPL keeps an ordered stream of assistant, tool, bash, permission, and diff segments.
4. On completion, Keen walks those segments and records each completed tool at the number of assistant-text bytes emitted before it.
5. Keen stores the flattened assistant prose separately from the compact `TurnMemory`.
6. When formatting that assistant message for a later provider request, Keen inserts provider-native historical tool-call/result blocks at the saved offsets and appends durable outcome memory.
7. The visible response and session transcript continue to show the original assistant prose and normal tool rendering, not the reconstructed blocks.
8. If the turn fails mid-loop, provider-native pending state remains the recovery mechanism.

## What The Next Turn Actually Gets

On the next user turn, Keen sends:

- prior user messages
- prior assistant prose
- provider-native historical tool-call/result blocks inserted between the relevant prior assistant prose segments
- durable `Tool memory` outcomes
- pending provider-native state from a prior failed turn, when present

This preserves a compact causal pattern:

```text
assistant intent
→ historical record of an actual tool invocation
→ assistant conclusion
```

It replays structured tool calls with empty placeholder arguments and one of two status-aware placeholder results, not the original arguments or outputs. The model can see whether an earlier invocation completed or failed, but it cannot rely on the discarded result as current evidence.

## Historical Tool Activity

A provider-facing exchange is reconstructed conceptually like this:

```text
assistant: Let me inspect the stream handler.
assistant tool call: read_file({})
tool result: Historical tool data intentionally removed; call required tools with valid arguments for actual results.
assistant: The terminal event is handled after content blocks finish.
```

Failed invocations instead use `Historical tool invocation failed and details were removed; call required tools with valid arguments if needed.` Unknown status values are treated as failures.

The call and result use each provider's native protocol and are not part of `Message.Content`. Empty arguments are placeholders rather than valid examples; current-turn work still requires a real tool call with schema-valid arguments. Synthetic call IDs are generated while formatting so each provider can pair calls with results; original provider call IDs are not retained.

### Placement

Each activity stores a byte offset into the flattened assistant prose. The offset equals the cumulative byte length of assistant segments preceding the completed tool segment. Formatting uses that offset to restore the activity between prose segments without storing a duplicate copy of the prose in `TurnMemory`.

Multiple tools may share an offset. Their original execution order is retained and they are replayed as one grouped batch at that point. Negative, out-of-range, out-of-order, non-UTF-8-boundary, or nameless persisted activities are ignored rather than causing formatting to fail.

### Fields

| Field | Meaning |
|---|---|
| `text_offset` | Byte position in assistant prose where native historical blocks are inserted |
| `tool` | Keen tool name, or logical MCP tool name |
| `status` | `success` when the invocation completed without a tool error; otherwise `error` |
| `target` | Optional allowlisted, bounded target such as a path, pattern, command, URL, or subagent name |
| `server` | MCP server name when the invocation used `call_mcp_tool` |

Targets are allowlisted by tool type and length-limited. File paths are made relative to the workspace when possible. Web URLs omit credentials, query parameters, and fragments. Raw outputs, complete errors, replacement text, written content, MCP arguments, and arbitrary input maps are not retained.

### MCP calls

MCP wrapper calls retain their logical server and tool in memory, but provider replay uses the registered `call_mcp_tool` wrapper with empty arguments. The retained server/tool values remain compact metadata and are not reconstructed into those placeholder arguments.

This records that the invocation occurred. It does not retain the MCP arguments, response, preview, or artifact path, and does not establish that the external information is still current or factually correct.

### What status means

`success` means only that Keen completed the tool invocation without a reported tool error. It does not guarantee that:

- the tool output was factually correct
- a search found useful results
- an external mutation had the desired broader effect
- the underlying workspace or service remains unchanged

`error` records only failure, not the full error text.

## Why Keen Does This

The design balances three pressures:

- the active agent needs high-fidelity tool state while solving the current task
- later turns need continuity and a truthful record that actions actually occurred
- conversation context should not accumulate large, stale, or untrusted tool outputs

Retaining prose while deleting every sign of tool activity can produce a misleading history in which the assistant appears to announce an action and then claim completion without executing anything. Provider-native historical blocks repair that protocol shape without turning `TurnMemory` into a raw execution archive.

The durable outcome layer remains intentionally narrow. Changed files and failed commands carry useful continuity, while read/search/MCP results are expected to be refreshed when needed.

## Tradeoffs

### Benefits

- Smaller context than full tool-trace retention
- Better distinction between narrated intent and actual prior execution
- No persistence of large or untrusted tool results
- Native tool protocol shape for each provider
- Status-aware placeholders distinguish completed and failed invocations
- Fresh reads of mutable workspace and external state
- Legible, bounded cross-turn memory

### Costs

- Rich investigative details from prior outputs are still lost
- Later turns may repeat reads, searches, commands, and MCP calls
- Empty historical arguments may be imitated and fail schema validation
- Byte offsets require validation when loading persisted state
- Compact history can reduce prompt/KV-cache continuity compared with retaining a full trace

This design works best when the workspace is the source of truth, read-only tools are cheap to repeat, and lean context is preferred over exhaustive replay. A fuller trace or richer planner state may suit long investigations with expensive, irreproducible external observations.

## Assistant Turn Reliability

### Pending Turn State

A single assistant turn can involve many provider tool-loop iterations. If it ends abnormally after tool work has accumulated, converting that partial exchange into generic conversation messages would be lossy and could invite side-effect re-execution.

Each LLM client therefore stores pending state in its provider-native message format. On the next `StreamChat` call, that state is injected before the new user message so the model can resume from the prior work.

| Event | Meaning | Pending state action |
|---|---|---|
| `Done` | Normal completion | Cleared or never saved |
| `Incomplete` | Turn ended abnormally after work occurred | Saved for the next call |
| `Error` | Turn failed before recoverable provider work accumulated | Not saved |

Pending state is in-memory only, does not survive process crashes, avoids re-executing completed tools, and is cleared after successful recovery. Persisted transcript and `TurnMemory` may still describe the visible partial turn, but provider-native pending state—not historical annotations—is authoritative for resuming the incomplete tool loop.

Retries within the same active turn rewind trailing unsealed assistant/reasoning segments. Historical activity is collected only from the final surviving segment list, so abandoned retry prose is not retained.

## Bottom Line

`TurnMemory` is a compact execution summary rather than a transcript.

Inside a turn, Keen keeps rich provider-native tool state. Across completed turns, it retains durable outcomes plus bounded records of where real tools ran. The records establish prior invocation, not retained evidence or current state. Failed turns use temporary provider-native pending state for recovery.
