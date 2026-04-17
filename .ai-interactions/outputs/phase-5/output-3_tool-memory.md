# Implementation Plan: Tool Memory

## Purpose

This document defines the implementation plan for the tool memory feature based
on the approved PRD and the selected minimal-change architecture.

The chosen design is:

- tool memory is emitted by the main assistant turn itself
- the output contract is enforced through the system prompt
- the tool memory is wrapped in a `<keen_memory>...</keen_memory>`
  block
- the REPL filters that block out of visible rendering only
- the raw assistant message, including the block, remains intact in normal
  conversation history

This keeps the implementation small while ensuring tool memory naturally stays
in later LLM context.

## Goals

- Generate tool memory as part of the main assistant turn
- Keep tool memory out of the visible REPL transcript
- Let tool memory naturally remain in later LLM context through normal
  conversation history
- Preserve tool memory across session resume
- Minimize code changes

## Non-Goals

- Showing tool memory in the REPL UI
- Storing raw tool input/output in long-term conversation history
- Adding a second LLM call for summarization
- Introducing a separate hidden memory model
- Redesigning the session model

## Design Summary

The system prompt will instruct the main model:

- if the turn used one or more tools, append exactly one
  `<keen_memory>...</keen_memory>` block at the very end
- if the turn used no tools, emit no such block
- the block must summarize outcomes, not raw tool I/O
- the block must stay short

Keen will then:

1. Stream the assistant response as usual.
2. Detect and suppress the XML tool-memory block from visible REPL rendering.
3. Keep the raw assistant message unchanged in normal conversation history.
4. Persist the raw assistant message unchanged so resumed sessions keep the
   tool memory in context.

Because the raw assistant message is already part of normal conversation
history, the tool memory naturally remains in the context window for later
turns.

## Core Decisions

### 1. Keep Tool Memory Inside The Normal Assistant Message

Do not introduce:

- a new `llm.Message` kind
- a dedicated hidden-memory store
- a new tool-memory field in session events

Instead:

- the assistant message saved in `AppState.messages` remains the raw model
  output, including the XML tool-memory block
- the REPL strips the block only for display purposes

This is the main simplification.

### 2. Use XML Tags As The Output Contract

The main system prompt in `internal/llm/systemprompt.go` should define the fixed
tool-memory delimiters:

```text
<keen_memory>...</keen_memory>
```

The XML block must appear only at the very end of the assistant turn.

### 3. Rely On Prompting And Best-Effort Parsing

The chosen solution is prompt-based rather than protocol-enforced.

That means:

- Keen will instruct the model to emit the block
- Keen will filter it out when present
- Keen will not fail the turn if the block is missing or malformed

This keeps v1 simple. Reliability is best-effort.

## Minimal Code Changes

### 1. `internal/llm/systemprompt.go`

Ensure the system prompt clearly requires:

- emit one `<keen_memory>` block at end of turn if tools were used
- emit no block if no tools were used
- summarize durable outcomes only
- do not include raw tool input/output
- keep it short

### 2. `internal/cli/repl/streaming.go`

Add streaming-aware parsing so assistant rendering excludes the XML block while
the raw response is still preserved.

The parser should:

- accumulate the raw assistant response
- detect the opening `<keen_memory>` tag, including across chunk
  boundaries
- stop rendering the XML block into visible assistant content
- keep buffering until `</keen_memory>` appears

This is the main implementation work.

### 3. `internal/cli/repl/streaming.go`

Update `HandleDone()` so it returns both:

- visible assistant text for transcript rendering
- raw assistant text for conversation history

A shape like this is sufficient:

```go
type doneResult struct {
	Lines          []string
	VisibleMessage string
	RawMessage     string
}
```

The important part is the split between what gets rendered and what gets stored.

### 4. `internal/cli/repl/handlers.go`

Update turn finalization so:

- `AppState` stores the raw assistant message
- the REPL transcript uses the cleaned visible assistant message

This is the other key change.

### 5. `internal/cli/repl/session_state.go`

Persist the raw assistant message, not the cleaned visible one.

This is necessary so resumed sessions keep the tool-memory block in the rebuilt
conversation state.

No new session-event field is required if the existing assistant message field
stores the raw message.

### 6. `internal/session/projection.go`

No special tool-memory logic should be needed.

Projection can continue rebuilding conversation state from the persisted
assistant message. Because that message is raw, the tool memory naturally
survives resume.

## Runtime Flow

### Normal Turn Completion

At the end of a turn:

1. `StreamHandler` has the raw assistant response.
2. The visible transcript is built from the cleaned response with the XML block
   removed.
3. `AppState` stores the raw assistant response.
4. Session persistence stores the raw assistant response.

### Resume

On session resume:

1. Transcript replay uses rendered transcript content, so the XML block stays
   hidden.
2. Conversation projection uses the raw assistant message, so the tool memory is
   present in later LLM context.

## Compaction Caveat

This design keeps tool memory embedded in normal assistant messages, so
compaction is the main caveat.

`AppState.Compact()` currently summarizes prior conversation history. If left
unchanged, tool-memory blocks may be lost or blurred during compaction.

Minimal-change v1 approach:

- accept the current compaction behavior for now
- optionally strengthen the compaction prompt later if preserving tool-memory
  content becomes important

Compaction-specific tool-memory preservation is out of scope for the minimal
implementation.

## Parsing And Failure Handling

Parser behavior must be forgiving.

Rules:

- if no XML block is found, render and store the response normally
- if the opening tag is found but the closing tag is missing, avoid showing the
  partial block in the UI when possible
- malformed or missing tool memory must not fail the assistant turn

V1 does not require a repair path.

## Suggested Implementation Sequence

### Phase 1: Prompt And Stream Filtering

1. Finalize the XML block contract in `internal/llm/systemprompt.go`
2. Add XML parsing and filtering logic to `StreamHandler`
3. Ensure visible assistant rendering excludes the XML block

### Phase 2: Turn Finalization And Persistence

1. Update `HandleDone()` to split visible vs raw assistant text
2. Update `handleLLMDone()` to render the visible text but store the raw text
3. Ensure session persistence stores the raw assistant message

### Phase 3: Resume Verification

1. Verify resumed sessions rebuild conversation state with raw assistant
   messages intact
2. Verify replayed transcript stays filtered

## Tests

### `internal/cli/repl/streaming_test.go`

- XML tool-memory blocks are stripped from visible assistant output
- opening and closing tags split across chunks are handled correctly
- malformed or partial XML blocks do not leak into the visible transcript

### `internal/cli/repl/handlers_test.go`

- completed turn renders cleaned visible text and stores raw assistant text
- turns without XML blocks behave normally
- malformed XML does not break normal turn completion

### `internal/session/projection_test.go`

- resumed sessions rebuild raw assistant messages unchanged

### `internal/session/store_test.go`

- assistant turn events persist the raw assistant message including the XML
  block

## Acceptance Criteria

The feature is complete when all of the following are true:

- a turn that emits a valid XML tool-memory block stores that block in the raw
  assistant message
- the XML block is not rendered in the REPL UI
- subsequent LLM turns naturally receive tool memory through the existing
  conversation history
- resumed sessions preserve tool memory because the raw assistant message is
  restored unchanged
- malformed or missing XML does not break normal turn completion
