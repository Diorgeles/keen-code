# Plan: Add Benchmark-First `keen run`

## Goal

Add a simple non-interactive `keen run` command so Keen can be benchmarked against `opencode run` without PTY-driving the REPL.

The first version should be benchmark-scoped, additive, and low-risk. It should reuse Keen's existing session, context, tool, and turn-memory behavior instead of creating a new agent architecture.

## Non-Goals

- Do not replace or refactor the REPL.
- Do not build a full interactive or TUI-less product surface yet.
- Do not implement `--continue`, `--fork`, session picker behavior, or slash commands.
- Do not persist provider-specific `pendingState`.
- Do not support cross-process recovery of incomplete provider tool loops.
- Do not change existing session projection or context-management behavior.
- Do not change tool implementations or REPL permission behavior.

## Command Shape

Initial CLI:

```sh
keen run [--session <session-id>] [--format text|json] <message...>
```

Behavior:

- Without `--session`, create a new Keen session.
- With `--session`, load that existing session and rebuild the conversation from its transcript.
- Read piped stdin when present.
- If both args and stdin are present, join them with a newline.
- Default output is final assistant text only.
- JSON output emits one final JSON object, not streaming JSONL.

Suggested JSON output:

```json
{
  "session_id": "keen-session-id",
  "opencode_session_id": "keen-session-id-without-hyphens",
  "text": "assistant response",
  "usage": {
    "input_tokens": 0,
    "output_tokens": 0,
    "reasoning_tokens": 0,
    "total_tokens": 0,
    "cached_tokens": 0
  }
}
```

`opencode_session_id` should match the value Keen sends in the `x-opencode-session` header for OpenCode Go, currently the local session ID with hyphens removed.

## Implementation Location

Use a small headless runner inside the existing `repl` package:

```text
internal/cli/repl/headless_run.go
```

This is intentionally pragmatic. The behavior needed for `keen run` already lives in package-private REPL helpers:

- `newReplSessionState`
- `buildAssistantTurnEvent`
- `cloneStreamSegments`
- `newTurnMemoryAccumulator`
- `StreamHandler`
- diff event handling types

Keeping the first pass in `internal/cli/repl` avoids premature extraction and avoids exporting internal details solely for a benchmark command.

Add the Cobra subcommand in:

```text
internal/cli/cmd/root.go
```

## Headless Runner Flow

High-level call chain:

```text
keen run
  -> repl.RunHeadless(...)
  -> AppState.StreamChat(...)
  -> llmClient.StreamChat(...)
```

Do not call `llmClient.StreamChat` directly. `AppState.StreamChat` is the right layer because it adds Keen's system prompt, skills catalog, message history, and registered tools.

Flow:

1. Load provider registry and resolved config using the same root-command setup path.
2. Create `llm.NewClient(cfg)`.
3. Create `appstate.New(client, workingDir)`.
4. Create an auto-approving REPL permission requester.
5. Create `tooling.NewDiffEmitter()`.
6. Call `tooling.SetupToolRegistry(workingDir, appState, permissionRequester, diffEmitter)`.
7. Create `newReplSessionState(workingDir)`.
8. If `--session` is provided:
   - find the matching summary from `sessions.listSessions()`
   - `sessions.load(summary)`
   - rebuild context with `session.BuildConversation(loaded.Events)`
   - call `appState.ReplaceMessages(...)`
9. Append the user message to session storage.
10. Append the user message to `appState`.
11. Start the stream with `AppState.StreamChat(ctx, cfg, llm.StreamOptions{SessionID: sessions.currentID()})`.
12. Consume LLM events and diff requests until done/error/incomplete.
13. Persist the assistant turn with turn memory.
14. Print final text or JSON.

## Conversation Handling Across Commands

`AppState` is in-memory, so every `keen run --session <id>` invocation must rebuild it from session storage before sending the next prompt.

Use the same projection the REPL uses for session resume:

```go
appState.ReplaceMessages(session.BuildConversation(loaded.Events))
```

This preserves Keen's current conservative context behavior:

- includes prior user messages
- includes assistant messages
- includes assistant `TurnMemory`
- respects compaction replacement events
- does not replay raw tool traces into future LLM context

## Stream Handling

Reuse `StreamHandler` as a turn accumulator, not as a UI renderer.

Create it with no markdown renderer:

```go
handler := NewStreamHandler(nil)
handler.Start(eventCh, "")
handler.showThinking = false
```

Feed stream events into the same methods the REPL uses:

```go
case llm.StreamEventTypeChunk:
    handler.HandleChunk(event.Content)

case llm.StreamEventTypeReasoningChunk:
    handler.HandleReasoningChunk(event.Content)

case llm.StreamEventTypeToolStart:
    if event.ToolCall.Name == "bash" {
        command, _ := event.ToolCall.Input["command"].(string)
        summary, _ := event.ToolCall.Input["summary"].(string)
        handler.HandleBashStart(command, summary)
    } else {
        handler.HandleToolStart(event.ToolCall)
    }

case llm.StreamEventTypeToolEnd:
    turnMemory.RecordToolEnd(event.ToolCall)
    if event.ToolCall.Name == "bash" {
        handler.HandleBashEnd(event.ToolCall)
    } else {
        handler.HandleToolEnd(event.ToolCall)
    }

case llm.StreamEventTypeUsage:
    lastUsage = event.Usage

case llm.StreamEventTypeRetry:
    handler.RewindForRetry()
```

On done, snapshot segments before resetting the handler:

```go
segments := cloneStreamSegments(handler.segments)
_, response := handler.HandleDone()

assistant := llm.Message{
    Role:       llm.RoleAssistant,
    Content:    response,
    TurnMemory: turnMemory.Build(),
}

appState.AppendMessage(assistant)
sessions.appendAssistantTurn(segments, assistant, false, "")
```

Optional cleanup after the first pass: add a small `StreamHandler.Finish()` helper that returns the final response and resets state without rendering transcript lines.

## Permission Policy

Headless benchmark mode should skip the interactive permission flow entirely.

Keep `tooling.SetupToolRegistry` and all tool constructor signatures unchanged. Add an auto-approve mode to the existing REPL permission requester:

```go
func NewAutoApproveRequester() *Requester {
    r := NewRequester()
    r.autoApprove = true
    return r
}

func (r *Requester) RequestPermission(ctx context.Context, toolName, path, resolvedPath string, isDangerous bool) (bool, error) {
    if r.autoApprove {
        return true, nil
    }

    // Existing interactive request path stays unchanged.
}
```

Then `keen run` can keep using the current registry API:

```go
permissionRequester := permissions.NewAutoApproveRequester()
tooling.SetupToolRegistry(workingDir, appState, permissionRequester, diffEmitter)
```

This keeps REPL permission behavior unchanged while making `keen run` independent of permission channels. In auto-approve mode, `RequestPermission` returns before creating a `Request`, writing to `requestChan`, or waiting for a response.

This skips user approval prompts, including dangerous bash-command approval. It does not bypass hard filesystem policy unless we intentionally change the guard. `filesystem.Guard` should still block denied paths such as ignored files, system paths, and hidden home directories. For benchmarking, keeping hard policy blocks is safer and keeps behavior closer to the existing tool contract.

## Diff Handling

`write_file` and `edit_file` emit diffs before permission approval. The existing `DiffEmitter` blocks until its `Done` channel is closed, so the headless loop must drain diff requests:

```go
case diffReq := <-diffEmitter.GetDiffChan():
    handler.HandleDiff(diffReq.Lines)
    close(diffReq.Done)
```

Skipping this will deadlock file edits.

## Incomplete Turns

Do not persist provider-specific `pendingState` in this first version.

Current pending state lives inside the provider client instance. That works in the REPL because the same process and `llmClient` survive after an incomplete turn. `keen run` starts a fresh process each time, so that state would be lost.

For benchmark mode, treat `StreamEventTypeIncomplete` as a failed run:

- persist partial transcript if useful for debugging
- print an error
- exit non-zero
- do not expect the next `keen run --session` to recover it

When resuming a session, optionally reject sessions whose last assistant turn has an error or interruption:

```text
cannot resume incomplete headless session
```

This is simpler and more honest for benchmarking than silently comparing a degraded recovery path.

## Root Command Wiring

`internal/cli/cmd/root.go` should add a `run` subcommand while preserving the existing default REPL behavior.

Suggested structure:

```go
root := &cobra.Command{... existing REPL RunE ...}
root.AddCommand(newRunCommand(version))
```

`newRunCommand` should reuse the same config-loading logic as the root command. If duplication becomes annoying, extract a small helper like:

```go
func loadRuntimeConfig() (*providers.Registry, *config.Loader, *config.GlobalConfig, *config.ResolvedConfig, bool, error)
```

Keep that helper mechanical; do not refactor unrelated root command behavior.

## Stdin Handling

Use standard input only when it is piped. Since `golang.org/x/term` is already present indirectly, either use it or a simple file stat check.

Behavior:

```text
args only      -> prompt = joined args
stdin only     -> prompt = stdin
args + stdin   -> prompt = joined args + "\n" + stdin
empty prompt   -> usage error
```

## Tests

Focused tests only:

1. Headless run creates a new session and persists user + assistant events using a fake LLM client.
2. Headless run with `--session` rebuilds conversation with `session.BuildConversation`.
3. Permission-pending tool path is auto-approved in headless mode.
4. Dangerous bash permission request is auto-approved in headless mode.
5. Diff requests are drained and do not block edits.
6. Incomplete event returns a non-zero/error result.
7. CLI prompt assembly handles args, stdin, and args + stdin.

Avoid real provider calls in tests.

## Verification

After implementation:

```sh
gofmt -w <modified-go-files>
go mod tidy
go test ./...
```

Manual smoke tests:

```sh
keen run --format json "Say hello"
keen run "Inspect the repo and summarize it in one sentence"
keen run --session <session-id> "Now list the most relevant files you saw"
echo "Say hello from stdin" | keen run --format json
```

For OpenCode Go benchmark attribution, confirm the JSON output's `opencode_session_id` matches the session shown in the OpenCode web usage interface.

## Expected Minimal File Changes

Likely files:

```text
internal/cli/cmd/root.go
internal/cli/repl/headless_run.go
internal/cli/repl/headless_run_test.go
internal/cli/cmd/root_test.go
```

Optional:

```text
internal/cli/repl/stream_handler.go
```

Only add a `Finish()` helper if it materially simplifies the headless runner.

## Why This Approach

This path keeps the benchmark command isolated and additive while still exercising the same context-management behavior as the REPL. It avoids brittle PTY automation, avoids broad refactors, and avoids turning `keen run` into a full product surface before the benchmark need is proven.
