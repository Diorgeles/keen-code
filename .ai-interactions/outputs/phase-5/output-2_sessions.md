# RFC: Persistent Sessions For Keen Code

## Purpose

This RFC defines a persistent session system for Keen Code so users can stop a
REPL session, reopen it later, and continue work without losing context.

The design explicitly separates:

- The **rendered transcript** needed to restore the visible REPL state
- The **conversation state** needed to rebuild `AppState.messages` for the next
  LLM turn

That distinction is necessary because Keen should resume the full visible
session, including tool activity, while only sending user and assistant
messages back to the model.

## Solution Overview

Keen will persist each saved session as a single append-only JSONL transcript
file stored under a working-directory-scoped session folder:

`~/.keen/sessions/<working-dir-sanitized>-<hash>/<timestamp>-<session-id>/transcript_events.jsonl`

The core model is:

- A session is created lazily, only after the first conversation message is
  persisted
- Every persisted event receives a strictly monotonic `seq`
- The session transcript is the source of truth for both:
  - replaying the visible REPL state on resume
  - rebuilding the conversation state for the next LLM request
- Compaction is represented as a `compaction_applied` event that includes the
  replacement conversation state
- Permission prompts are never persisted
- Provider and model are never persisted; the resumed session uses the current
  runtime configuration

On resume, Keen will:

1. Discover sessions only for the current working directory
2. Let the user pick a prior session with `/sessions`
3. Replay `transcript_events.jsonl` to rebuild the visible REPL transcript
4. Rebuild `AppState.messages` by projecting only the conversation-relevant
   events and applying compaction replacement events when present

This preserves the user-visible session history without polluting future model
context with UI-only artifacts.

## Goals

- Preserve REPL state across process exits
- Restore the visible transcript as closely as possible
- Preserve enough conversation state to continue the session correctly
- Keep session storage inspectable and deterministic
- Scope session discovery to the current working directory
- Support future evolution of the event format without redesigning the system

## Non-Goals

- Persisting provider or model selection
- Persisting context window calculations
- Persisting permission prompts or permission decisions
- Truncating large bash outputs or diffs for v1
- Merging sessions across working directories
- Supporting cross-session branching in v1

## Storage Layout

Each working directory gets its own session namespace:

`~/.keen/sessions/<working-dir-sanitized>-<hash>/`

Each session lives in a child directory:

`<timestamp>-<session-id>/`

Where:

- `working-dir-sanitized` is the absolute working directory transformed into a
  filesystem-safe readable slug
- `hash` is a short hash suffix derived from the absolute working directory
- `timestamp` is the session creation timestamp
- `session-id` is a generated UUID

Each session directory contains:

- `transcript_events.jsonl`

No additional files are required for v1.

## Session Discovery

Session discovery is scoped to the current working directory only.

Keen will:

- Derive the current working-directory namespace from the current absolute cwd
- Scan only that directory under `~/.keen/sessions/`
- Load sessions by reading `transcript_events.jsonl`
- Sort sessions by last updated sequence or timestamp, newest first

Because discovery is already cwd-scoped, resume does not need to warn about
cross-directory mismatches in v1.

## User Experience

### Creating A Session

A session is created only when there is at least one conversation message.

That means:

- starting the REPL does not create a session
- viewing help does not create a session
- permission prompts do not create a session
- the first persisted user message causes the session directory and transcript
  file to be created

### Listing Sessions

Users invoke `/sessions` to open a session picker.

The picker will:

- show only sessions for the current working directory
- sort most recently updated sessions first
- allow navigation with arrow keys
- resume the selected session on Enter

Each item shows:

- created-at timestamp
- updated-at timestamp
- preview of the first user message, trimmed for display

If no sessions exist for the current directory, Keen shows an empty-state
message instead of opening the picker.

### Resume Flow

When a session is resumed, Keen restores the visible REPL transcript, including:

- user messages
- assistant messages
- interrupted assistant messages
- thinking/reasoning text
- tool interactions
- diffs
- bash commands and outputs
- compaction status entries

The resumed session does not restore:

- provider
- model
- context status
- permission prompts

After replay completes, the user can continue the session immediately. The next
LLM request uses the rebuilt `AppState.messages` derived from the transcript
projection rules in this RFC.

## Event Log Format

`transcript_events.jsonl` is an append-only JSONL file. Each line is a single
self-contained event.

Every event has:

- `seq`: strictly monotonic sequence number within the session
- `kind`: event type
- `payload`: event-specific data

Recommended envelope:

```json
{
  "seq": 7,
  "kind": "assistant_message",
  "payload": {
    "content": "Updated the parser and added tests."
  }
}
```

The event stream is the only persisted state for the session.

## Event Types

The v1 event kinds are:

- `session_started`
- `user_message`
- `assistant_message`
- `assistant_interrupted`
- `reasoning_message`
- `tool_start`
- `tool_end`
- `bash_start`
- `bash_end`
- `diff`
- `compaction_applied`

### `session_started`

Contains static session metadata:

- `session_id`
- `created_at`
- `cwd`

This is the first event in every session file.

### `user_message`

Contains:

- `content`

Used for transcript replay and conversation-state rebuilding.

### `assistant_message`

Contains:

- `content`

Used for transcript replay and conversation-state rebuilding.

### `assistant_interrupted`

Contains:

- `content`

Represents a partial assistant response persisted when the user interrupts the
stream. This event is rendered in the transcript and included in rebuilt
conversation state because that matches current runtime behavior.

### `reasoning_message`

Contains:

- `content`

Used only for transcript replay. This event does not contribute to
`AppState.messages`.

### `tool_start`

Contains enough data to reproduce the visible tool-start line:

- `name`
- `input`

This event is transcript-only.

### `tool_end`

Contains enough data to reproduce the visible tool-end line:

- `name`
- `input`
- `output`
- `error`
- `duration_ns`

This event is transcript-only.

### `bash_start`

Contains:

- `command`
- `summary`

This event is transcript-only.

### `bash_end`

Contains:

- `command`
- `summary`
- `output`
- `error`
- `duration_ns`

This event is transcript-only.

### `diff`

Contains the rendered diff lines needed to reproduce the visible diff output.

This event is transcript-only.

### `compaction_applied`

Contains:

- a status message or compacted summary representation for transcript replay
- the full replacement conversation state as `[]llm.Message`

This event is special:

- transcript replay shows a compaction status entry
- conversation rebuild replaces the accumulated message slice with the payload
  in this event

This is how Keen preserves compaction semantics without rendering the compacted
message content as if it were ordinary visible chat.

## Replay Model

The session file supports two separate projections.

### Transcript Projection

The transcript projection rebuilds the visible REPL state.

It:

- replays user and assistant messages
- replays interrupted assistant messages
- replays reasoning text
- replays tool start/end entries
- replays bash start/end entries
- replays diffs
- replays compaction status entries
- skips permission prompts entirely

This projection is responsible for rebuilding the output shown in the viewport
when a session resumes.

### Conversation Projection

The conversation projection rebuilds `AppState.messages`.

It:

- appends `user_message` as `llm.RoleUser`
- appends `assistant_message` as `llm.RoleAssistant`
- appends `assistant_interrupted` as `llm.RoleAssistant`
- ignores reasoning events
- ignores tool events
- ignores bash events
- ignores diff events
- ignores permission prompts
- replaces the accumulated message slice when a `compaction_applied` event is
  encountered

This is the state used for the next `StreamChat()` call.

## Rendering Semantics

To make replay reliable, persisted events must capture the content needed to
reconstruct the UI without depending on live streaming state.

That means:

- tool events are persisted as completed transcript events, not as in-flight UI
  state
- bash events capture the final rendered output
- diff events store the same diff lines that were shown in the REPL
- interrupted assistant events capture the exact persisted text that the user
  saw after interruption
- compaction replay uses a stable synthetic status line

The replay layer should not attempt to restore spinner state, cursor state, or
streaming state.

## Ordering And Durability

Events are ordered only by `seq`.

Requirements:

- `seq` starts at `1` for a new session
- each appended event increments `seq` by `1`
- replay uses file order and `seq` consistency checks
- event appends are atomic at the line level from Keen's point of view

Keen should write events promptly after the corresponding state change so a
crash loses as little session history as possible.

## Corruption Handling

Session replay must be tolerant of malformed lines.

On load:

- read the file line by line
- attempt to decode each JSON object independently
- if a line is malformed, skip it and continue
- do not fail the whole session because of one bad event

Replay must also tolerate orphaned transcript events, for example:

- `tool_end` without a matching `tool_start`
- `bash_end` without a matching `bash_start`

In those cases Keen should still render the surviving event as sensibly as
possible instead of failing the resume flow.

## Current Runtime Mapping

The design maps cleanly to the current REPL architecture:

- `AppState` remains the source of in-memory conversation messages
- `OutputBuilder` remains the source of persisted non-stream transcript lines
- `StreamHandler` already knows how to represent reasoning, tool activity,
  bash output, permission requests, and diffs while streaming
- persistent sessions add a storage and replay layer around those structures

The key architectural change is that Keen will persist transcript events at the
same points where it currently mutates in-memory state or renders durable
transcript output.

## Concrete Implementation Plan

### 1. Add A Session Package

Create a new package under `internal/session` to encapsulate:

- working-directory namespace derivation
- session path generation
- session creation
- JSONL event append
- session discovery
- session loading
- transcript and conversation projection logic

This keeps persistence concerns out of the REPL view code.

### 2. Define Event Types And Codecs

Add Go types for:

- event envelope
- each event payload
- session summary used by the picker

Implement:

- JSON encoding for appends
- line-by-line JSON decoding for replay
- tolerant malformed-line handling

### 3. Integrate Session Writing Into REPL State Changes

Persist events at the durable state boundaries:

- when the first user message is added
- when an assistant message completes
- when an assistant message is interrupted
- when reasoning text needs to be part of the replayed transcript
- when tool start/end events are rendered
- when bash start/end events are rendered
- when diffs are emitted
- when compaction succeeds

Permission requests are intentionally not persisted.

### 4. Add Session Replay

On resume:

- load and decode the JSONL event stream
- rebuild the transcript projection into the REPL output
- rebuild the conversation projection into `AppState.messages`
- recompute context status from the rebuilt `AppState`

The current provider/model configuration is used after replay completes.

### 5. Add Session Picker UI

Add a new `/sessions` command that:

- loads session summaries for the current directory
- shows the picker in the REPL
- supports up/down navigation and Enter to resume

Optional follow-up:

- `/resume` can become an alias that opens the same picker

### 6. Add Tests

Add tests for:

- working-directory slug and hash generation
- lazy session creation
- event append ordering
- malformed-line tolerance
- transcript replay
- conversation rebuild
- compaction replacement semantics
- interrupted assistant persistence
- picker sorting and preview generation

## Granular Todo List

### Session Storage

- Create `internal/session/` package
- Add helper to derive the session namespace from the current cwd
- Add helper to sanitize the cwd for use in a directory name
- Add helper to compute the short cwd hash suffix
- Add helper to generate session directory names from timestamp and UUID
- Add helper to lazily create the session directory on first persisted message
- Add helper to resolve `transcript_events.jsonl`
- Add append logic for writing one JSON event per line
- Add load logic for reading one JSON event per line
- Add malformed-line skip behavior in the loader

### Event Schema

- Define event envelope with `seq`, `kind`, and typed payload
- Define payload for `session_started`
- Define payload for `user_message`
- Define payload for `assistant_message`
- Define payload for `assistant_interrupted`
- Define payload for `reasoning_message`
- Define payload for `tool_start`
- Define payload for `tool_end`
- Define payload for `bash_start`
- Define payload for `bash_end`
- Define payload for `diff`
- Define payload for `compaction_applied`
- Add encode/decode tests for each payload type

### Replay And Projections

- Add transcript replay projection
- Add conversation rebuild projection
- Add compaction replacement handling in the conversation projection
- Add transcript rendering for reasoning events
- Add transcript rendering for tool events
- Add transcript rendering for bash events
- Add transcript rendering for diff events
- Add transcript rendering for compaction status entries
- Add orphaned-event tolerance for replay

### REPL Integration

- Add session manager/state to the REPL model
- Initialize session state lazily instead of at REPL startup
- Persist `session_started` when the first conversation message is saved
- Persist `user_message` after the user submits a normal prompt
- Persist `assistant_message` when streaming completes
- Persist `assistant_interrupted` when the stream is cancelled by the user
- Persist reasoning text so it can be replayed on resume
- Persist tool interaction events from the stream handlers
- Persist bash events including final output
- Persist diff events emitted during edits
- Persist `compaction_applied` with replacement `[]llm.Message`
- Rebuild `AppState.messages` from a loaded session
- Rebuild the visible output from a loaded session
- Recompute context status after session replay

### Commands And UI

- Add `/sessions` to the slash command list
- Add picker model/state for saved sessions
- Add empty-state handling when no sessions exist
- Add session item rendering with created-at, updated-at, and preview
- Add keyboard navigation for the picker
- Add Enter-to-resume behavior
- Add `/resume` as an alias or dedicated entry point to the picker

### Testing

- Add unit tests for the session package
- Add replay tests for malformed JSONL lines
- Add tests for session ordering by most recent update
- Add tests for first-message preview generation
- Add tests for lazy creation semantics
- Add tests for compaction replacing prior conversation state
- Add tests for interrupted assistant replay
- Add REPL tests for `/sessions`
- Add REPL tests for successful resume into an existing transcript

### Verification

- Run `go mod tidy`
- Run `go test ./...`
- Review the RFC and implementation plan against the current REPL flow before
  starting code changes

## Open Questions

These do not block the RFC, but they should be settled during implementation:

- Should `/resume` be a visible alias for `/sessions`, or should only one
  command remain in the final UX?
  - I think `/resume` can be an alias for `/sessions`.
- Should `updated_at` be derived from the last event when listing sessions, or
  should the session package maintain a cached summary during append?
  - We should derive the `updated_at` from the last event when listing sessions.
- Should the session picker render local times using the terminal locale or a
  fixed format?
  - We should render local times using the terminal locale.

## Recommendation

Proceed with the single-file JSONL session log design described here.

It matches Keen's current architecture well:

- it preserves full visible session replay
- it keeps future LLM context limited to meaningful conversation state
- it handles compaction cleanly
- it avoids persisting transient permission UI
- it stays inspectable and debuggable on disk

This is the right foundation for persistent sessions in Keen Code.
