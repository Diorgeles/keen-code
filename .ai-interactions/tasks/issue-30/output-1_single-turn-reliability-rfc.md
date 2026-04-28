# RFC: Single-Turn Reliability via Pending Tool State

## Status

Draft.

## Summary

When an agent turn fails mid-loop (API error, empty response, user interruption, or max-turn exhaustion), all accumulated turn data is lost — assistant messages, tool call requests, and tool outputs. The conversation history in `appState` only retains partial assistant text, not the full turn transcript. On the next user message the model has no memory of the work it already performed.

This RFC proposes storing a pending turn state on each LLM client struct, in the client's native provider format. On failure the state is retained; on the next `StreamChat` call it is injected into the conversation so the model can resume. On successful completion the state is cleared.

## Goals

- Preserve the full turn transcript (assistant messages, tool calls, tool outputs) across turn failures.
- Allow the model to resume from where it left off on the next user message.
- Avoid duplicating turn transcript data in `appState.messages`.
- Keep the change internal to each client — no changes to `LLMClient`, `StreamChat`, or `Message`.

## Non-Goals

- Persisting pending turn state to the session file. The state is in-memory only and does not survive process crashes.
- Cross-provider state recovery. If the user switches providers between a failed turn and a retry, the pending state is discarded.
- Re-executing tool calls. Tools have side effects; the state captures outputs, not re-execution intent.

## Current State

Each `StreamChat` implementation runs a goroutine with a `for range maxToolTurns` loop. The accumulated turn data (assistant messages, tool call requests, and tool results) lives as local variables inside the goroutine:

| Client | Local variable | Type |
|---|---|---|
| `GenkitClient` | `aiMessages` | `[]*ai.Message` |
| `AnthropicClient` | `msgParams` | `[]anthropic.MessageParam` |
| `OpenAICompatibleClient` | `oaiMessages` | `[]openai.ChatCompletionMessageParamUnion` |
| `OpenAIResponsesClient` | `input` + `previousResponseID` | `[]responses.ResponseInputItemUnionParam` + `string` |
| `OpenAICodexClient` | `input` | `[]responses.ResponseInputItemUnionParam` |

When the goroutine exits on error, these local variables are discarded. The REPL's `handleLLMError` saves partial assistant text to `appState.messages`, but the assistant messages, tool call requests, and tool outputs from loop iterations are gone.

### Terminal States of an Agent Turn

A turn can end in five ways:

| # | State | Current event | Accumulated turn data? |
|---|---|---|---|
| 1 | Normal completion (model responds with no tool calls) | `StreamEventTypeDone` | N/A |
| 2 | API/stream error | `StreamEventTypeError` | Maybe |
| 3 | Empty/nil response mid-loop | `StreamEventTypeDone` | Maybe |
| 4 | Max tool turns exhausted | `StreamEventTypeDone` | Yes |
| 5 | User interruption (Esc) | Handled by `interruptStream()` | Maybe |

States 2–5 can lose accumulated turn data. States 3 and 4 are currently indistinguishable from normal completion at the REPL level.

## Proposed Design

### 1. Pending State Field on Each Client

Each client struct gets a field storing the messages generated during the loop — the delta beyond the initial converted `[]Message`. The type is provider-native:

```go
// GenkitClient
pendingState []*ai.Message

// AnthropicClient
pendingState []anthropic.MessageParam

// OpenAICompatibleClient
pendingState []openai.ChatCompletionMessageParamUnion

// OpenAIResponsesClient
pendingState         []responses.ResponseInputItemUnionParam
pendingResponseID    string

// OpenAICodexClient
pendingState []responses.ResponseInputItemUnionParam
```

The `OpenAIResponsesClient` additionally stores `pendingResponseID` to preserve the `previousResponseID` optimization for context caching.

### 2. Saving Pending State

At the end of each loop iteration, after tool execution, the client checks whether turn data has accumulated — that is, any messages (assistant messages, tool call requests, or tool outputs) were appended to the local message list beyond the initial conversion. When the turn ends abnormally and turn data has accumulated, the client saves the delta to its pending state field.

The rule: if any turn data has accumulated, save pending state. If not, don't.

### 3. Injecting Pending State

On the next `StreamChat` call, if pending state exists, the client:

1. Converts the incoming `[]Message` to provider format (existing logic, unchanged).
2. Inserts the pending state before the last message (the new user message).
3. Clears the pending state.
4. Runs the loop as normal.

The "before the last message" placement is correct because the REPL is sequential: each user submission triggers exactly one `StreamChat` call, so there is at most one new user message after a failure. The pending state belongs after the messages that triggered the failed turn and before the new user message.

### 4. New Event Type: `StreamEventTypeIncomplete`

A new `StreamEventType` is introduced to signal that work was done but the turn did not complete normally:

```go
StreamEventTypeIncomplete StreamEventType = "incomplete"
```

The client emits this instead of `Done` or `Error` when the turn ends abnormally **and** turn data has accumulated. The decision logic inside the goroutine:

| Condition | Event emitted |
|---|---|
| Normal completion (no tool calls) | `StreamEventTypeDone` |
| Abnormal exit, turn data accumulated | `StreamEventTypeIncomplete` |
| Abnormal exit, no turn data | `StreamEventTypeError` |

This covers terminal states 2, 3, and 4 cleanly. The REPL does not need to inspect the client's internal state — the event type tells it everything.

### 5. Error Emission in `OpenAICompatibleClient`

`collectTurnWithRetry` currently emits `StreamEventTypeError` directly to `eventCh` before returning the error to `StreamChat`. This means `StreamChat` never gets the chance to check for accumulated turn data and emit `StreamEventTypeIncomplete` instead.

The fix: `collectTurnWithRetry` stops emitting `StreamEventTypeError`. It returns the error to `StreamChat`, which decides the appropriate event based on accumulated state. `StreamEventTypeRetry` events remain in `collectTurnWithRetry` since they are purely informational and do not affect state decisions.

### 6. Changes to `appState.messages`

| Event received | appState action |
|---|---|
| `StreamEventTypeDone` | Append full assistant message (unchanged) |
| `StreamEventTypeIncomplete` | Do not append. Pending state on the client has the full turn transcript (assistant messages, tool calls, tool outputs). |
| `StreamEventTypeError` | Append partial assistant text if any (unchanged, first-iteration errors only) |

This avoids duplication: the turn transcript lives either in `appState.messages` (on success) or in the client's pending state (on failure), never both.

### 7. User Interruption (Case 5)

User interruption is handled by `interruptStream()` in the REPL, which runs synchronously and bypasses the event system. The goroutine's eventual event is dropped because `streamHandler.IsActive()` returns false after interruption.

Two changes:

1. **Client side**: When the goroutine detects `context.Canceled` and turn data has accumulated, it saves the pending state. The emitted event is dropped by the REPL but the state persists on the client struct.
2. **REPL side**: `interruptStream()` stops calling `m.appState.AppendMessage(...)`. The rest of the function (UI handling, showing "[interrupted]" text, resetting spinner) remains unchanged.

### 8. Clearing Pending State

Pending state is cleared in two places:

- At the start of a successful injection (step 3 above), after the state has been spliced into the message list.
- Implicitly, when the client struct is replaced (user switches providers via `/model`).

### 9. Concurrency

`StreamChat` runs a goroutine that writes the pending state. The next `StreamChat` call reads it. Since the REPL is sequential (waits for one call to finish before starting another), no synchronization is needed.

## Affected Files

| File | Change |
|---|---|
| `internal/llm/message.go` | Add `StreamEventTypeIncomplete` constant |
| `internal/llm/genkit.go` | Add `pendingState` field, save/inject/clear logic |
| `internal/llm/anthropic.go` | Add `pendingState` field, save/inject/clear logic |
| `internal/llm/openai.go` | Add `pendingState` field, save/inject/clear logic; remove `StreamEventTypeError` emission from `collectTurnWithRetry` |
| `internal/llm/openai_responses.go` | Add `pendingState` + `pendingResponseID` fields, save/inject/clear logic |
| `internal/llm/openai_codex.go` | Add `pendingState` field, save/inject/clear logic |
| `internal/cli/repl/handlers.go` | Handle `StreamEventTypeIncomplete` (no appState append), update `interruptStream()` |
| `internal/cli/repl/repl.go` | Map `StreamEventTypeIncomplete` to a new REPL message type |
| `internal/cli/repl/stream_msgs.go` | Add `llmIncompleteMsg` type |

## Example Flow

### Turn Failure and Recovery

1. User sends "refactor the auth module" → added to `appState.messages`.
2. `StreamChat` called. Client converts messages, starts loop.
3. Iteration 1: Model calls `read_file(auth.go)`. Tool executes, result appended to local messages.
4. Iteration 2: Model calls `edit_file(auth.go, ...)`. Tool executes, result appended.
5. Iteration 3: `collectTurn` fails (API 500 error). Client saves iterations 1–2 turn data (assistant messages, tool calls, tool outputs) as pending state. Emits `StreamEventTypeIncomplete`.
6. REPL receives `Incomplete`. Does not append to `appState`. Shows error to user.
7. User sends "continue" → added to `appState.messages`.
8. `StreamChat` called. Client sees pending state. Converts messages, inserts pending state before "continue", clears pending state, starts loop.
9. Model sees the full history: original request, assistant messages and tool calls/outputs from iterations 1–2, and "continue". Resumes work.

### Normal Completion (No Change)

1. User sends a message. `StreamChat` called. No pending state.
2. Loop runs, model finishes with no tool calls. Client emits `StreamEventTypeDone`.
3. REPL appends full assistant message to `appState`. Same as today.
