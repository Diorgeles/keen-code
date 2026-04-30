# REPL Command Handlers Refactor — Plan

## Summary

Move slash-command dispatch and command-specific business logic out of `repl.go` and
`handlers.go` into a dedicated `command_handlers.go` file. After the refactor:

- `repl.go` → model construction, `RunREPL`, rendering, viewport/textarea glue
- `handlers.go` → Bubble Tea message routing (key events, LLM stream events, spinner ticks)
- `command_handlers.go` → slash-command dispatch + each command's logic

---

## Current state

| Function | Current file | Responsibility |
|----------|-------------|----------------|
| `handleEnterKey` | `repl.go` | Dispatches slash commands AND starts LLM chat stream |
| `handleThinkingCommand` | `repl.go` | `/thinking` business logic |
| `handleLogout` | `repl.go` | `/logout` business logic |
| `handleClear` | `repl.go` | `/clear` and `/new` business logic |
| `startCompaction` | `repl.go` | `/compact` business logic |
| `startModelSelection` | `repl.go` | `/model` UI trigger |
| `getHelpText` | `repl.go` | `/help` text generation |
| `handleKeyMsg` | `handlers.go` | Bubble Tea key routing (delegates to `handleEnterKey`) |
| `handleLLM*` | `handlers.go` | LLM stream event handling |
| `handlePermissionKeyMsg` | `handlers.go` | Permission dialog key handling |
| `handleSessionPickerKeyMsg` | `handlers.go` | Session picker key handling |

The boundary is fuzzy — `handleEnterKey` in `repl.go` contains both the dispatch
table and the general "send to LLM" path, while `handlers.go` contains both Bubble Tea
routing and some command-adjacent logic.

---

## Proposed file layout after refactor

### `repl.go` (model + lifecycle)
- `replContext`, `replModel` struct definitions
- `initialModel()`, `RunREPL()`
- `Init()`, `View()`
- Viewport/textarea helpers: `applyWindowSize`, `adjustTextareaHeight`, `updateViewportContent`, `scrollToBottomIfFollowing`, `renderInputArea`, `inputMetaView`
- `startStreamContext`, `clearStreamCancel`
- `updateLLMClient`
- `replayLoadedSession`
- Constants, loading texts, loading spinners

### `handlers.go` (Bubble Tea message/key routing)
- `Update`, `updateNormalMode`
- `handleKeyMsg` — key dispatch only (enter → call into command_handlers)
- `handleLLMStreamMsg`, `handleLLMChunk`, `handleLLMReasoningChunk`, `handleLLMDone`, `handleLLMError`, `handleLLMIncomplete`, `handleLLMRetry`, `handleLLMUsage`
- `handleToolStart`, `handleToolEnd`
- `handleCompactionDone`, `handleCompactionError`
- `handleSpinnerTick`
- `consumeModelSelectionResult`
- `handlePermissionKeyMsg`
- `handleSessionPickerKeyMsg`
- `handleSuggestionKeyMsg`, `handleFileModeSelection`
- `handleUpdateCheckMsg`
- `interruptStream`
- `waitForAsyncEvent`

### `command_handlers.go` (NEW — slash-command dispatch + logic)
- `dispatchCommand(input string) (replModel, tea.Cmd)` — the slash-command routing
  extracted from `handleEnterKey`
- `handleHelpCommand() (replModel, tea.Cmd)`
- `handleModelCommand() (replModel, tea.Cmd)`
- `handleLogoutCommand() (replModel, tea.Cmd)` (renamed from `handleLogout`)
- `handleClearCommand() (replModel, tea.Cmd)` (renamed from `handleClear`)
- `handleThinkingCommand(input string) (replModel, tea.Cmd)` (moved as-is)
- `handleCompactCommand(input string) (replModel, tea.Cmd)` (wraps `startCompaction`)
- `handleSessionsCommand() (replModel, tea.Cmd)`
- `handleExitCommand() (replModel, tea.Cmd)`
- `getHelpText() string` (moved)
- `startModelSelection() replModel` (moved)
- `startCompaction(extraPrompt string) (replModel, tea.Cmd)` (moved)

---

## Detailed changes

### 1. Create `internal/cli/repl/command_handlers.go`

- [ ] Add package declaration and required imports
- [ ] Move `getHelpText()` from `repl.go`
- [ ] Move `startModelSelection()` from `repl.go`
- [ ] Move `handleThinkingCommand()` from `repl.go`
- [ ] Move `handleLogout()` → rename to `handleLogoutCommand()` for consistency
- [ ] Move `handleClear()` → rename to `handleClearCommand()`
- [ ] Move `startCompaction()` from `repl.go`
- [ ] Extract a new `dispatchCommand(input string) (replModel, tea.Cmd, bool)`:
  - Contains the slash-command `if/switch` chain currently in `handleEnterKey`
  - Returns `(model, cmd, handled)` — if `handled == false`, the caller sends to LLM
- [ ] Extract `handleExitCommand`, `handleHelpCommand`, `handleModelCommand`,
  `handleSessionsCommand`, `handleCompactCommand` as thin wrappers if warranted,
  or leave them inlined in `dispatchCommand` where the logic is ≤5 lines

### 2. Simplify `handleEnterKey` in `repl.go`

- [ ] After extracting command dispatch, `handleEnterKey` becomes:
  ```go
  func (m *replModel) handleEnterKey() (replModel, tea.Cmd) {
      input := m.textarea.Value()
      if input == "" || m.streamHandler.IsActive() || m.isCompacting {
          return *m, nil
      }
      m.output.AddUserInput(input, repltheme.PromptStyle)
      m.history.Push(input)

      if updated, cmd, handled := m.dispatchCommand(input); handled {
          return updated, cmd
      }

      // ... existing "send to LLM" logic stays here
  }
  ```
- [ ] Move `handleEnterKey` to stay in `repl.go` (it's lifecycle/textarea plumbing)
  OR move it to `handlers.go` (it's called from `handleKeyMsg`). Decision: keep in
  `handlers.go` since it's triggered by key events and closely related to message routing.

### 3. Move `handleEnterKey` from `repl.go` to `handlers.go`

- [ ] This puts all key-triggered actions in one file
- [ ] The command dispatch call goes to `command_handlers.go`

### 4. Create/update tests

- [ ] Create `internal/cli/repl/command_handlers_test.go`:
  - Move tests for `/help`, `/model`, `/clear`, `/thinking`, `/logout`, `/compact`
    from `repl_test.go` to the new test file
  - Add a test for `dispatchCommand` routing (unknown commands fall through)
- [ ] Keep LLM stream and key-routing tests in `handlers_test.go`
- [ ] Keep model construction and rendering tests in `repl_test.go`

### 5. Verify

- [ ] `go test ./internal/cli/repl/...` — all pass
- [ ] `go test ./...` — full suite passes
- [ ] `go mod tidy`
- [ ] No behavior changes — purely structural refactor

---

## Implementation order

1. Create `command_handlers.go` with moves (no renames yet) — verify tests pass
2. Extract `dispatchCommand` from `handleEnterKey`
3. Move `handleEnterKey` to `handlers.go`
4. Rename handlers for consistency (`handleLogout` → `handleLogoutCommand`, etc.)
5. Reorganize tests into the appropriate `_test.go` files
6. Final `go test ./...`

---

## Risk mitigation

- **No behavior changes** — this is a pure code-motion refactor
- **Incremental moves** — each step leaves tests green
- **No new dependencies** — same package, same imports reshuffled

---

## Acceptance criteria (from issue)

- [x] Slash-command routing has a single obvious home → `command_handlers.go`
- [x] `handlers.go` no longer owns command-specific business logic
- [x] Existing command behavior is preserved
- [x] Existing tests pass, with focused tests added where command dispatch moves
