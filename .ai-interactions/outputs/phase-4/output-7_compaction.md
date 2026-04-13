# Compaction Implementation Plan

## Context
As conversations grow, the full message history is sent to the LLM on every request.
Without compaction, context windows fill up and requests eventually fail. This plan adds
a `/compact` slash command (manual trigger) and a suggestion nudge at 70%. Compaction summarizes the full conversation history into a
single user message, preserves the last 20 messages verbatim, and replaces `AppState.messages`
with `[summary_message] + last_20_messages`.

### Thread safety and cancellation
No mutex is needed. The summary string is fully accumulated from the LLM stream before
`s.messages` is written — there is no partial write. If the context is cancelled
mid-stream, `Compact()` returns an error and `messages` is left untouched, so
cancellation is safe at any point during compaction.

---

## Files to Modify / Create

| File | Change |
|------|--------|
| `internal/cli/repl/state.go` | Add `Compact()` method + compaction system prompt |
| `internal/cli/repl/repl.go` | Add `isCompacting` field; handle `/compact` command; block input during compaction |
| `internal/cli/repl/handlers.go` | No changes needed |
| `internal/cli/repl/commands.go` | Register `/compact` in `allSlashCommands` |
| `internal/cli/repl/context_status.go` | Add `ShouldSuggestCompaction()` helper |
| `internal/cli/repl/styles.go` | Add style for compaction suggestion hint |
| `internal/cli/repl/repl_test.go` | Tests for `/compact`, cancel flow, help text, spinner/meta rendering |
| `internal/cli/repl/compaction_test.go` | Tests for `Compact()` logic |

---

## Implementation Steps

### 1. Add compaction helper to `context_status.go`

Add one threshold helper:

```go
const compactionSuggestThreshold = 70.0

func (s contextStatus) ShouldSuggestCompaction() bool {
    return s.KnownWindow && s.Percent >= compactionSuggestThreshold
}
```

---

### 2. Add `Compact()` method to `state.go`

`Compact()` is the core compaction logic. It runs a no-tools LLM request using a dedicated
compaction system prompt, collects the full response, then replaces `messages`.

```go
const compactKeepLast = 20

func (s *AppState) Compact(ctx context.Context, cfg *config.ResolvedConfig, extraPrompt string) error
```

**Steps inside `Compact()`:**

1. Take a snapshot of current `s.messages`.
2. Split into `tail = messages[max(0, len(messages)-compactKeepLast):]`.
   - If `len(messages) == 0`, return nil early.
3. Build a special request:
   - System message: compaction-specific prompt (see below).
   - Messages: the full snapshot of `s.messages` (the summary must reflect the whole history, including the last 20 messages that will be preserved verbatim afterward).
   - Final user message: the compaction instruction (+ `extraPrompt` if provided).
4. Call `s.llmClient.StreamChat(ctx, fullMsgs, nil)` — pass `nil` tool registry (no tools).
5. Consume all `StreamEventTypeChunk` events, accumulate into `summary`.
6. On `StreamEventTypeDone`, replace `s.messages` with:
   ```
   [ {RoleUser, summary} ] + tail
   ```
7. On `StreamEventTypeError`, return the error without modifying messages.

**Compaction system prompt** (inline constant):

```
You are an AI agent for compacting long conversation history. Your task is to produce a concise but complete
summary of the conversation provided. The summary will replace the earlier part of
the conversation so that work can continue without losing important context. The summary has to be useful and concise.

Structure your summary as follows:

## Goal
What goal(s) is the user trying to accomplish?

## Key Instructions
Important instructions or constraints given by the user.

## Discoveries
Notable things learned (about the codebase, requirements, etc.).

## Accomplished
What has been completed, what is in progress, and what remains.

## Relevant Files
A structured list of files that are still important to continue the task.

Be concise. Omit repetition. Do not include tool outputs verbatim. The summary must be useful for continuing the current task after the earlier messages are removed.
```

If the caller passes a non-empty `extraPrompt`, append it after the default instruction:
`"Additionally: " + extraPrompt`.

---

### 3. Add `/compact` command to `commands.go`

```go
{"/compact", "Compact conversation context (optional: /compact <focus hint>)"},
```

Also add the command constant to `repl.go`:
```go
compactCommand = "/compact"
```

---

### 4. Add compaction state fields to `replModel` in `repl.go`

```go
isCompacting      bool
compactionCancel  context.CancelFunc
```

---

### 5. Handle `/compact` in `handleEnterKey()` in `repl.go`

Add a `/compact` branch after `/help` and `/model`, but do **not** bypass LLM readiness validation. Compaction requires the client to be initialized.

```go
if strings.HasPrefix(input, compactCommand) {
    extraPrompt := strings.TrimSpace(strings.TrimPrefix(input, compactCommand))
    if !m.appState.IsClientReady(m.ctx.cfg) {
        m.output.AddError("LLM client not initialized. Use /model to configure.", errorStyle)
        m.textarea.Reset()
        m.updateViewportContent()
        m.viewport.GotoBottom()
        return *m, nil
    }
    m.textarea.Reset()
    return m.startCompaction(extraPrompt)
}
```

`startCompaction` creates a cancellable context, stores the cancel func, and returns a
`tea.Cmd` that runs the goroutine:

```go
func (m *replModel) startCompaction(extraPrompt string) (replModel, tea.Cmd) {
    ctx, cancel := context.WithCancel(context.Background())
    m.compactionCancel = cancel
    m.isCompacting = true
    m.showSpinner = true
    m.spinner.Spinner = nextLoadingSpinner()
    m.loadingText = "Compacting..."
    m.adjustTextareaHeight()
    m.updateViewportContent()
    m.viewport.GotoBottom()

    runCompaction := func() tea.Msg {
		err := m.appState.Compact(ctx, m.ctx.cfg, extraPrompt)
		if err != nil {
			return compactionErrMsg{err: err}
		}
		return compactionDoneMsg{}
	}

    return *m, tea.Batch(m.spinner.Tick, runCompaction)
}
```

---

### 6. Handle compaction messages in `Update()` / `handleLLMStreamMsg()`

Add a new `handleCompactionMsg` path in `updateNormalMode()`:

```go
case compactionDoneMsg:
    return m.handleCompactionDone()
case compactionErrMsg:
    return m.handleCompactionError(msg.err)
```

**`handleCompactionDone()`:**
1. Set `m.isCompacting = false`, `m.showSpinner = false`, `m.compactionCancel = nil`.
2. Call `m.refreshContextStatus(false)`.
3. Add an output line: `"  Context compacted."` (styled with `mutedColor`).
4. Call `m.adjustTextareaHeight()`.
5. `m.updateViewportContent()`, `m.viewport.GotoBottom()`.

**`handleCompactionError(err)`:**
1. Set `m.isCompacting = false`, `m.showSpinner = false`, `m.compactionCancel = nil`.
2. If `errors.Is(err, context.Canceled)`: show `"  Compaction cancelled."` styled with `mutedColor`.
3. Otherwise: `m.output.AddError("Compaction failed: " + err.Error(), errorStyle)`.
4. Call `m.adjustTextareaHeight()`.
5. Refresh, update, scroll.

---

### 7. Block user input during compaction; allow Esc to cancel

In `handleEnterKey()`, add at the top (alongside the existing stream-active check):

```go
if m.isCompacting {
    return *m, nil
}
```

In `handleKeyMsg()`, allow Esc to cancel compaction:

```go
if m.isCompacting {
    if keyMsg.String() == keyEsc {
        if m.compactionCancel != nil {
            m.compactionCancel()
            m.compactionCancel = nil
        }
    }
    return *m, nil
}

// Existing suggestion handling follows.

case keyEsc:
    if m.streamHandler != nil && m.streamHandler.IsActive() {
        m.interruptStream("Interrupted...what should the agent do instead?")
    }
    return *m, nil
```

When the cancelled context causes `Compact()` to return an error, `handleCompactionError`
will fire — the error will be `context.Canceled`, which should be shown as a soft
"Compaction cancelled." message rather than a red error.

While compaction is running:
- `Enter` must not send a new message
- the textarea does not need to be visually disabled
- all regular typing can be ignored to keep the mode simple and avoid slash-suggestion interference

---

### 8. Override spinner text during compaction in `View()`

Replace the spinner block in `View()`:

```go
if m.showSpinner {
    return 1
}
return 0
```

Update `spinnerHeight()` so compaction reserves vertical space even without an active `streamHandler`.

Then update the spinner block in `View()`:

```go
if m.showSpinner {
    var spinnerLabel string
    if m.isCompacting {
        spinnerLabel = "Compacting..."
    } else if m.streamHandler != nil && m.streamHandler.IsActive() {
        spinnerLabel = m.loadingText
    }
    if spinnerLabel != "" {
        spinnerText := m.spinner.View() + " " + loadingTextStyled.Render(spinnerLabel)
        // ... existing padding + write logic
    }
}
```

This same `showSpinner`-based logic must also be reflected in `adjustTextareaHeight()` and `applyWindowSize()`, since they currently depend on `spinnerHeight()`.

---

### 9. Add compaction suggestion to `inputMetaView()` in `repl.go`

When `contextStatus.ShouldSuggestCompaction()` is true, append a hint to the meta bar:

```go
if m.contextStatus.ShouldSuggestCompaction() {
    hint := compactionHintStyle.Render("  · type /compact to free up context")
    right = right + hint
}
```

Add `compactionHintStyle` in `styles.go`:
```go
compactionHintStyle = lipgloss.NewStyle().Foreground(accentColor).Italic(true)
```

For narrow widths, the hint should be best-effort only: if it does not fit alongside the existing context status, drop the hint and keep the current compact meta-bar fallback behavior.

---

### 10. Add `/compact` to help output

Update `getHelpText()` so `/help` shows:

```go
{"/compact", "Compact conversation context (optional: /compact <focus hint>)"},
```

This keeps slash-command suggestions and help text consistent.

---

### 11. Test Coverage

Add tests for:

- `AppState.Compact()` replaces history with `summary + last_20_messages`
- `AppState.Compact()` summarizes the full history, not only the prefix before the preserved tail
- `AppState.Compact()` leaves messages unchanged on error
- `AppState.Compact()` leaves messages unchanged on cancellation
- `/compact` parsing with and without an extra prompt
- `/compact` readiness failure when no LLM client is configured
- `Esc` cancels compaction even if slash suggestions would otherwise be visible
- spinner is shown during compaction even though `streamHandler` is inactive
- 70% context-usage hint appears when there is enough horizontal space
- 70% context-usage hint is dropped cleanly on narrow widths

---

### 10. Add `/compact` to `getHelpText()` in `repl.go`

```go
{"/compact", "Compact conversation context (optional focus hint)"},
```

---

### 11. Write tests in `internal/cli/repl/compaction_test.go`

- `TestCompact_NoOp_WhenFewMessages`: fewer than 10 messages → no change.
- `TestCompact_ReplacesSummaryAndPreservesTail`: mock LLM returns "SUMMARY", verify messages = `[{user, "SUMMARY"}] + last10`.
- `TestCompact_ReturnsError_OnLLMError`: mock LLM returns error → messages unchanged.
- `TestShouldSuggestCompaction`: 69% → false, 70% → true.

---

## Verification

1. **Run tests**: `go test ./...`
2. **Run `go mod tidy`** after any new imports.
3. **Manual smoke test**:
   - Start keen-code, send enough messages to reach 70% → confirm hint appears.
   - Send `/compact` → confirm "Compacting..." spinner, then "Context compacted." message.
   - Send `/compact keep the details about the file structure` → same flow, verify extra prompt is included in LLM request.
   - During compaction, try typing and pressing Enter → confirm it's blocked.
   - During compaction, press Esc → confirm "Compaction cancelled." message appears.
