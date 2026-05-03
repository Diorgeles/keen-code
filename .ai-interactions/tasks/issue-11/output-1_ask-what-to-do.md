# Plan: "Ask what to do instead" Permission Option (Issue #11)

## Summary

Add a fourth option to the permission selector: "Ask what to do instead". When selected, the user provides a text instruction that interrupts the current agent turn and is sent as a new user message to the LLM.

---

## Current State

- Permission choices are defined in `internal/cli/repl/permissions/selector.go`
  - Non-dangerous: `Allow`, `Allow for this session`, `Deny`
  - Dangerous: `Allow`, `Deny`
- Permission key handling is in `internal/cli/repl/handlers.go` → `handlePermissionKeyMsg`
- Permission card rendering is in `internal/cli/repl/stream_permission.go`
- The `Requester` in `permissions/requester.go` sends a boolean (`allowed`) back to the tool
- Interrupt logic already exists: `interruptStream()` in `handlers.go` cancels the stream context and preserves partial state

---

## Design Decisions

1. **New choice**: `ChoiceAskWhatToDo` added after `ChoiceDeny`
2. **UI flow**: When user selects "Ask what to do instead" and presses Enter:
   - Permission card resolves with a new status (e.g. `StatusRedirected`)
   - The pending tool gets denied (returns `false` to the tool)
   - The stream is interrupted (same as Esc interrupt)
   - A text input prompt appears for the user to type their instruction
   - On submit, the instruction is appended as a user message and a new LLM turn starts
3. **Requester response**: The tool receives `false` (denied), same as `ChoiceDeny`. The difference is purely in the REPL flow that follows.
4. **Text input**: Reuse the existing textarea. After interrupt, focus returns to textarea with a hint that the user should type a new instruction.

---

## Implementation Plan

### Step 1: Add the new choice to the selector

**File**: `internal/cli/repl/permissions/selector.go`

- Add `ChoiceAskWhatToDo Choice = 3`
- Update `Choices()` to include `"Ask what to do instead"` as the last option (both dangerous and non-dangerous)
- Update `ChoiceAt()` to map the new cursor position

### Step 2: Add new status

**File**: `internal/cli/repl/permissions/requester.go`

- Add `StatusRedirected Status = "redirected"` for the resolved card display

### Step 3: Handle the new choice in permission key handler

**File**: `internal/cli/repl/handlers.go` → `handlePermissionKeyMsg`

- When `choice == ChoiceAskWhatToDo`:
  1. Resolve permission with `StatusRedirected`
  2. Send `ChoiceDeny` response to the requester (tool gets denied)
  3. Call `interruptStream()` to cancel the current turn
  4. Set a flag on replModel (e.g. `awaitingRedirectInput bool`) to indicate the next Enter should send the user's text as a follow-up message

### Step 4: Handle the redirect input submission

**File**: `internal/cli/repl/handlers.go` → `handleEnterKey`

- If `m.awaitingRedirectInput` is true when user presses Enter:
  - Clear the flag
  - Take textarea value as the user instruction
  - Append it as a user message to appState
  - Start a new LLM turn (same as normal message submission)
  - This naturally continues from the preserved partial assistant state

### Step 5: Update permission card rendering

**File**: `internal/cli/repl/stream_permission.go`

- Add rendering for the new choice in the selector list
- Add resolved status line for `StatusRedirected` (e.g. "↩ Redirected for <toolName>")

### Step 6: Update hint text

- Update the hint line in the permission card to reflect the new option

---

## Edge Cases

1. **User types empty instruction**: Treat same as no-op, re-show textarea prompt or ignore
2. **Esc during redirect input**: Cancels the redirect, returns to idle state (no message sent)
3. **Multiple permission requests queued**: Only the active pending one can be redirected; queued ones remain pending

---

## Testing

- Unit test: `ChoiceAt` correctly maps new cursor positions
- Unit test: `Choices()` returns correct options for dangerous/non-dangerous
- Integration: Selecting "Ask what to do instead" interrupts the stream and awaits user input
- Integration: Submitting redirect input starts a new LLM turn with the instruction as user message

---

## Files to Modify

1. `internal/cli/repl/permissions/selector.go` — new choice constant + updated lists
2. `internal/cli/repl/permissions/requester.go` — new status constant
3. `internal/cli/repl/handlers.go` — permission key handling + enter key handling
4. `internal/cli/repl/stream_permission.go` — card rendering for new choice/status
5. `internal/cli/repl/repl.go` — add `awaitingRedirectInput` field to `replModel`
