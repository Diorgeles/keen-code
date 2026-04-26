# Input History Navigation — Implementation Plan

## Issue Summary
GitHub issue #4: Navigate previously submitted inputs using the Up/Down arrow keys,
similar to shell history (Up = older, Down = newer / restore draft).

---

## Current Behaviour

In `handlers.go → handleKeyMsg`:

- `keyUp` / `keyShiftUp` — when the textarea cursor is on the **first line**, scroll
  the viewport up by one line.
- `keyDown` / `keyShiftDown` — when the cursor is on the **last line**, scroll the
  viewport down by one line.

There is no history tracking at all today.

---

## Desired Behaviour

| Key     | Cursor position      | Action                                          |
|---------|----------------------|-------------------------------------------------|
| Up      | First line of input  | Replace textarea content with the previous (older) history entry |
| Down    | Last line of input, in history mode | Replace textarea content with the next (newer) entry, or restore the draft |
| Up/Down | Any other cursor position | Existing textarea cursor-movement (no change) |
| Up/Down | Suggestion widget visible | Already handled by suggestion logic — no change needed |

A **draft** is the in-progress text the user was typing before they started pressing
Up to navigate history. Pressing Down past the most-recent entry restores the draft.

---

## Implementation

### 1. New file: `internal/cli/repl/input_history.go`

Encapsulate history state in a small, testable struct.

```go
type inputHistory struct {
    entries  []string // oldest → newest
    idx      int      // -1 = draft mode; 0..len-1 = viewing a history entry
    draft    string   // text saved when entering history mode
    filePath string   // ~/.keen/input-history
}

const maxHistorySize = 1000
```

Methods:

| Method | Description |
|---|---|
| `push(entry string)` | Append entry (skip blanks, exact duplicates of last, and slash commands). Trim to `maxHistorySize`. Reset `idx = -1`, clear `draft`. Then call `appendToFile`. |
| `navigateUp(current string) (value string, ok bool)` | Save `current` as `draft` on first call (when `idx == -1`). Decrement `idx` toward 0. Return the entry at the new index. Returns `ok=false` if already at oldest entry. |
| `navigateDown() (value string, ok bool)` | Increment `idx`. If we pass the newest entry (`idx == -1`), return `draft` and reset to draft mode. Returns `ok=false` if already in draft mode and nothing to restore. |
| `reset()` | `idx = -1`, `draft = ""`. Called on `/clear` and session load. |
| `loadFromFile(path string) error` | Reads `~/.keen/input-history`, one entry per line (with `\n` literals unescaped). Populates `entries`. Deduplicates consecutive identical lines. If file is missing, returns silently. Stores `path` in `filePath`. |
| `appendToFile(entry string) error` | Appends a single escaped entry (newlines → `\n` literals) to `filePath`. Non-fatal: failure is silently ignored so the session is never blocked. |

### 2. Add field to `replModel` in `repl.go`

```go
type replModel struct {
    // ...existing fields...
    history inputHistory
}
```

In `initialModel()`, call `history.loadFromFile(path)` where `path` is
`filepath.Join(os.UserHomeDir(), ".keen", "input-history")`. Use `os.MkdirAll` to
create `~/.keen/` if it doesn't exist. A load failure is non-fatal — start with empty
history.

### 3. Modify `handleEnterKey` in `repl.go`

After validating `input != ""` and **before** `m.textarea.Reset()`, push the input
to history and reset the navigation index:

```go
m.history.push(input)
```

Only push messages and skip slash commands.

### 4. Modify `handleKeyMsg` in `handlers.go`

Replace the existing `keyUp` and `keyDown` cases:

```go
case keyUp, keyShiftUp:
    if m.isAtTopOfInput() {
        if val, ok := m.history.navigateUp(m.textarea.Value()); ok {
            m.textarea.SetValue(val)
            m.textarea.MoveToBeginning() // keep cursor at top
            m.adjustTextareaHeight()
            return *m, nil
        }
        // at oldest entry or empty history — fall through to viewport scroll
        m.viewport.ScrollUp(1)
        m.userScrolled = !m.viewport.AtBottom()
        return *m, nil
    }

case keyDown, keyShiftDown:
    if m.isAtBottomOfInput() {
        if val, ok := m.history.navigateDown(); ok {
            m.textarea.SetValue(val)
            m.textarea.MoveToEnd()
            m.adjustTextareaHeight()
            return *m, nil
        }
        // in draft mode with nothing newer — fall through to viewport scroll
        m.viewport.ScrollDown(1)
        m.userScrolled = !m.viewport.AtBottom()
        return *m, nil
    }
```

No changes are needed to the suggestion-widget intercept path — it already fires
before the Up/Down cases and consumes those keys.

### 5. New test file: `internal/cli/repl/input_history_test.go`

Cover:

- `push`: blank entry is ignored; duplicate of last entry is ignored; max size trim; `appendToFile` is called.
- `loadFromFile`: missing file is a no-op; consecutive duplicates are deduplicated; multi-line entries are unescaped correctly.
- `appendToFile`: multi-line entries are escaped to `\n` literals; write failure does not panic.
- `navigateUp`: saves draft on first call; steps through entries; returns `ok=false`
  at oldest.
- `navigateDown`: steps back toward draft; restores draft; returns `ok=false` when
  already in draft mode.
- Full round-trip: push several entries → Up × N → Down × N → confirm draft restored.

---

## Files Changed

| File | Change |
|---|---|
| `internal/cli/repl/input_history.go` | **New** — `inputHistory` struct + methods including `loadFromFile` and `appendToFile` |
| `internal/cli/repl/input_history_test.go` | **New** — unit tests |
| `internal/cli/repl/repl.go` | Add `history inputHistory` field; call `loadFromFile` + `os.MkdirAll` in `initialModel()` |
| `internal/cli/repl/handlers.go` | Update `keyUp` / `keyDown` cases; push in `handleEnterKey`; reset in `handleClear` |
| `internal/cli/repl/repl.go` | Also call `m.history.reset()` inside `replayLoadedSession()` |

---

## Edge Cases

| Scenario | Handling |
|---|---|
| Empty textarea on submission | Already guarded (`if input == ""`); not pushed |
| Consecutive identical submissions | Skipped in `push` (no duplicate of last entry) |
| Multi-line input (Ctrl+Enter) | Stored and restored as-is; textarea supports multi-line |
| Suggestion widget visible | Keys consumed by suggestion logic before reaching history code |
| Stream active | User can still navigate history (read-only); cannot submit until stream ends |
| Session load / resume | `reset()` clears navigation state (`idx`, `draft`) and is called in both `handleClear()` and `replayLoadedSession()`; history entries persist across sessions via `~/.keen/input-history` |
| History overflow | Capped at `maxHistorySize = 1000`; oldest entries dropped on load if file exceeds cap (rewritten once at startup) |
| Concurrent instances | Append-only writes are safe; a second instance won't see new entries until restarted (matches shell behaviour) |
| Write failure | `appendToFile` failure is silently ignored — history is a UX nicety, never fatal |
