# Improve MCP Tool UI: Collapsible Inputs

## Problem

`formatMCPToolInput` (output.go:188) currently renders **all** MCP arguments as multi-line output. For tools like `context7/query-docs` with large query strings, this clutters the terminal every time the tool fires.

## Recommended Approach

Make MCP tool calls **compact by default** (`server/tool` on one line) and **expandable on click** in the active stream. Historical output stays compact.

### Why this scope?

- **Active stream** is where the clutter problem is acute (rapid-fire tool calls during a turn).
- `StreamHandler` already stores structured `streamSegment`s, so adding toggle state is cheap.
- `OutputBuilder` bakes history into plain `[]string`. Making those interactive requires either storing structured segments (bigger refactor) or fragile line-index mapping across viewport wrapping. The payoff is low once a turn is done.

---

## Implementation Plan

### 1. Compact default rendering

**File:** `internal/cli/repl/output/output.go`

- Change `formatMCPToolInput` to return **only** `server/tool` when arguments exist.
- Extract the current multi-line argument formatting into `formatMCPToolInputExpanded` (used when a segment is toggled open).
- Update `FormatToolStart` and `FormatToolDone` to accept an `expanded bool` and branch between compact and full rendering.

**File:** `internal/cli/repl/output/output_test.go`

- Update `TestFormatToolInput_CallMCPToolFormatsArgumentsPrettily` to expect the compact one-line form by default.
- Add a new test for the expanded multi-line form.

### 2. Expansion state in the active stream

**File:** `internal/cli/repl/stream_segments.go`

```go
type streamSegment struct {
    kind             streamSegmentType
    expanded         bool          // NEW
    // ... existing fields
}
```

**File:** `internal/cli/repl/stream_handler.go`

Add:

```go
func (sh *StreamHandler) ToggleSegmentExpanded(segmentIdx int) bool
```

This flips `segments[segmentIdx].expanded` and returns true if that segment is a tool start/end (so the UI knows to re-render).

### 3. Render active stream with expansion

**File:** `internal/cli/repl/stream_render.go`

In `renderViewLines`, for `segmentToolStart` and `segmentToolEnd`:
- If `seg.expanded`, call `FormatToolStart(toolCall, expanded=true)` which uses `formatMCPToolInputExpanded`.
- Otherwise, use `FormatToolStart(toolCall, expanded=false)` which shows just `server/tool`.

Also build and cache a **line-to-segment mapping** during `renderViewLines` so mouse clicks can be resolved:

```go
// In StreamHandler
segmentLineMap []int   // maps rendered line index -> segment index
```

### 4. Mouse click handling

**File:** `internal/cli/repl/repl.go`

In `updateNormalMode`, inside the `tea.MouseClickMsg` handler, add logic after selection handling:

```go
if !m.selection.hasSelection() && m.streamHandler.IsActive() {
    if segIdx, ok := m.streamHandler.SegmentAtLine(streamLine); ok {
        if m.streamHandler.ToggleSegmentExpanded(segIdx) {
            m.updateViewportContent()
            return m, nil
        }
    }
}
```

Where `streamLine` is derived from the click Y minus the output region line count.

**File:** `internal/cli/repl/viewport_selection.go`

No changes needed to the selection system itself. The click-to-expand logic should run when the selection system determines the click was a **single click without drag** (i.e., not a text-selection gesture). The existing `clickCount`/`registerClick` logic can be used: if `clickCount == 1` and the mouse release happens on the same line without drag, treat it as a potential expand toggle.

### 5. Visual affordance

**File:** `internal/cli/repl/theme/styles.go` (or inline in output.go)

Add a small prefix to MCP tool lines so users know they're clickable:
- Collapsed: `▶ context7/query-docs`
- Expanded: `▼ context7/query-docs`
  (followed by indented arguments on subsequent lines)

Use `repltheme.MutedColor` or similar for the arrow so it doesn't dominate.

---

## Files to modify

| File | Change |
|------|--------|
| `internal/cli/repl/output/output.go` | Compact default MCP display; add expanded variant |
| `internal/cli/repl/output/output_test.go` | Update/add tests for new MCP formatting |
| `internal/cli/repl/stream_segments.go` | Add `expanded bool` |
| `internal/cli/repl/stream_handler.go` | `ToggleSegmentExpanded`, `SegmentAtLine` |
| `internal/cli/repl/stream_render.go` | Branch on `expanded` when rendering tool segments; build line map |
| `internal/cli/repl/repl.go` | Wire mouse click to toggle expansion |

## Risks & Mitigations

| Risk | Mitigation |
|------|--------|
| Click conflicts with text selection | Only toggle if the click was a clean single-click (no drag). Selection already uses drag detection. |
| Active stream line mapping drifts | Rebuild `segmentLineMap` on every `renderViewLines` call, which happens every frame during streaming. |
| Expanded state lost on stream end | Acceptable — once the turn completes, the compact lines are flushed to `OutputBuilder`. The clutter problem is solved; full data is still in the transcript/session file. |
| Lipgloss wrapping breaks line math | The line map is built from the *rendered* lines (after lipgloss wrapping in `renderViewLines`), so it matches viewport lines exactly. |

## Verification

1. Run `go test ./internal/cli/repl/output/...`
2. Run `go test ./internal/cli/repl/...`
3. Manual test: trigger an MCP tool call in the REPL and click the tool line — it should expand to show arguments, click again to collapse.
