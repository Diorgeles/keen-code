# Tool Memory Redesign Implementation Plan

## Goal

Replace the current `<keen_memory>...</keen_memory>` in-band assistant markup with a structured per-turn tool memory model stored in `AppState.messages`, then render that memory into assistant message content only when building provider requests.

## Agreed Requirements

- After every assistant turn, create a `TurnMemory` object for that turn.
- `TurnMemory` stores:
  - `FilesChanged`: all files written or edited during the turn, deduplicated and kept in stable first-seen order
  - `FailedBash`: bash commands from the turn with `exit_code != 0`, including the command string and exit code
- Store `TurnMemory` in `AppState.messages` alongside the assistant message.
- While converting internal `llm.Message` objects to OpenAI or Genkit messages, render `TurnMemory` as part of the assistant message content.
- Do not store raw stdout/stderr in turn memory.

## Proposed Data Model

```go
type Message struct {
	Role       Role
	Content    string
	TurnMemory *TurnMemory
}

type TurnMemory struct {
	FilesChanged []string
	FailedBash   []FailedBashCommand
}

type FailedBashCommand struct {
	Command  string
	ExitCode int
}
```

## Rendering Contract

Assistant message content remains the source of user-visible text. `TurnMemory` is rendered into provider-facing assistant content only when non-empty.

Proposed format:

```text
<assistant content>

Tool memory:
- Files changed: path/a.go, path/b.go
- Failed bash: go test ./... (exit 1)
```

Constraints:

- Render `Tool memory:` only if there is at least one memory entry.
- Omit empty sections.
- Keep rendering deterministic.
- Never reintroduce `<keen_memory>` tags.
- Use the relative path for files changed

## Implementation Plan

### 1. Introduce structured tool memory types

Update `internal/llm/message.go` to add:

- `TurnMemory *TurnMemory` on `Message`
- `TurnMemory`
- `FailedBashCommand`

This is the core schema change. It should be minimal and backward-compatible for existing messages that have no memory.

### 2. Add turn-level accumulation in the REPL flow

Track tool-memory candidates during each assistant turn in the REPL/model layer where tool events are already observed.

Capture:

- file paths from `write_file`
- file paths from `edit_file`
- failed bash commands from `bash` tool results where `exit_code != 0`

Do not capture:

- `read_file`
- `grep`
- `glob`
- successful bash commands
- raw stdout/stderr

Use a deduplicating accumulator for changed files and an append-only list for failed bash commands.

### 3. Finalize `TurnMemory` at assistant turn completion

When the assistant turn ends successfully:

- build a `TurnMemory` object from the accumulated turn state
- attach it to the stored assistant `llm.Message`

When the turn has no retained memory:

- store `TurnMemory` as `nil`

Decide whether interrupted/error turns should store partial memory. Recommended first version:

- store memory only on completed assistant turns
- skip storing partial memory on interrupt/error for now

### 4. Update `AppState.messages` usage

Ensure all `AppState` flows preserve `TurnMemory`:

- `AddMessage`
- `GetMessages`
- `ReplaceMessages`
- compaction request building
- any session persistence or replay path that serializes/deserializes messages

Anything that copies `llm.Message` values must continue to preserve the new field.

### 5. Render `TurnMemory` only at provider boundary

Update provider conversion paths so structured memory becomes part of assistant content only when sending messages to the model:

- `internal/llm/genkit.go`
- `internal/llm/openai.go`
- `internal/llm/openai_responses.go`

Introduce one shared formatter helper so the rendering logic is defined once and reused consistently.

Recommended helper shape:

```go
func FormatMessageForProvider(msg Message) string
```

Behavior:

- for non-assistant messages, return `Content`
- for assistant messages without `TurnMemory`, return `Content`
- for assistant messages with `TurnMemory`, append the rendered `Tool memory:` block

### 6. Remove `<keen_memory>` prompt instructions and parsing dependency

Delete the old model instruction that asks the LLM to emit `<keen_memory>...</keen_memory>`.

Update:

- `internal/llm/systemprompt.go`

Then remove the legacy parsing/extraction path once the new flow is wired:

- streaming parser logic for hidden memory tags
- extraction helpers
- related logging around extracted keen memory

If safer, do this in two steps:

- first stop producing `<keen_memory>`
- then remove parsing code after tests are updated

### 7. Update compaction behavior

Compaction should rely on structured `TurnMemory`, not hidden tags in assistant content.

Update the compaction prompt and request-building assumptions so:

- hidden tag language is removed
- provider-formatted assistant messages already contain rendered tool memory where applicable

This keeps compaction behavior working without a separate memory channel.

### 8. Update tests

Add and update tests for:

- `Message` formatting with and without `TurnMemory`
- changed files deduplication
- failed bash capture with `exit_code != 0`
- successful bash exclusion
- provider conversion for OpenAI, Genkit, and OpenAI Responses
- `AppState` copy/preserve behavior
- session persistence/replay if messages are serialized
- removal of `<keen_memory>` parsing behavior

Delete or rewrite tests that assert tag-based hidden memory.

## Granular Todo List

- [ ] Add `TurnMemory` and `FailedBashCommand` types to `internal/llm/message.go`
- [ ] Add `TurnMemory *TurnMemory` to `llm.Message`
- [ ] Find every place `llm.Message` is constructed and verify the new field is preserved
- [ ] Add a REPL turn-memory accumulator type
- [ ] Reset the accumulator at the start of each assistant turn
- [ ] Record file paths for `write_file` tool completions
- [ ] Record file paths for `edit_file` tool completions
- [ ] Record failed bash commands for `bash` tool completions with `exit_code != 0`
- [ ] Deduplicate `FilesChanged` while preserving first-seen order
- [ ] Ignore successful bash executions
- [ ] Ignore read-only tools entirely
- [ ] Build `TurnMemory` from the accumulator in the assistant turn completion path
- [ ] Attach `TurnMemory` to the stored assistant message in `AppState.messages`
- [ ] Decide and implement behavior for interrupted/error turns; first pass should skip partial memory
- [ ] Add a shared formatter helper for provider-facing message text
- [ ] Render `FilesChanged` in a deterministic single line
- [ ] Render each failed bash entry as `command (exit N)`
- [ ] Update Genkit message conversion to use the shared formatter
- [ ] Update OpenAI-compatible chat conversion to use the shared formatter
- [ ] Update OpenAI Responses conversion to use the shared formatter
- [ ] Remove `<keen_memory>` instructions from the system prompt
- [ ] Remove compaction prompt references to hidden `<keen_memory>` blocks
- [ ] Remove `<keen_memory>` extraction from completed assistant message handling
- [ ] Remove `<keen_memory>`-specific streaming/parser logic
- [ ] Remove obsolete keen-memory logging hooks
- [ ] Rewrite streaming tests that currently depend on hidden tag parsing
- [ ] Add unit tests for `TurnMemory` rendering
- [ ] Add unit tests for failed bash detection from `exit_code`
- [ ] Add unit tests for changed-file deduplication and stable ordering
- [ ] Add unit tests to verify provider-boundary formatting includes rendered tool memory
- [ ] Add/update session persistence tests if `llm.Message` is serialized anywhere
- [ ] Run `go mod tidy`
- [ ] Run `go test ./...`

## Suggested Delivery Order

1. Add the new message schema and provider formatter.
2. Add turn-memory accumulation and assistant-turn storage.
3. Update provider conversions to include rendered memory.
4. Remove `<keen_memory>` prompt instructions.
5. Remove legacy tag parsing/extraction.
6. Update compaction and session-related paths.
7. Finish with tests, `go mod tidy`, and `go test ./...`.

## Open Decisions

- Whether interrupted assistant turns should retain partial memory. Recommended now: no.
- Whether `write_file` should be rendered as “created” and `edit_file` as “updated”, or both should simply feed `FilesChanged`. Current agreed requirement points to a single `FilesChanged` list.
- Whether failed bash commands should also be deduplicated if the same failing command appears multiple times in one turn. Recommended now: keep all failures in occurrence order unless repeated duplicates become noisy.
