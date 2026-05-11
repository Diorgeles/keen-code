# `/btw` Command — Implementation Plan

## Summary

Add a `/btw <question>` slash command that sends a one-shot question to the LLM
in a separate context (not polluting the main conversation), streams the response,
and renders it in a visually distinct boxed section with clear start/end boundaries.

**Key constraints:**
- Only read tools available: `read_file`, `glob`, `grep`
- Tool calls are visible to the user (same rendering as main conversation)
- Response is rendered with markdown (code blocks, bold, etc.) same as main conversation
- No session persistence — the exchange is ephemeral
- No conversation history pollution — does not touch `appState.messages`
- No pending state corruption — uses `OneShot: true` on `StreamOptions`
- Streamed response with explicit visual start/end markers
- Empty input shows: "Usage: /btw <question>"
- Uses a separate, focused system prompt (no skills catalog, no project instructions)

---

## Architecture

```
User types "/btw how does X work?"
  → dispatchCommand → handleBtwCommand()
    → appState.StreamBtw(ctx, cfg, question)
      → client.StreamChat(ctx, [system, user], readOnlyRegistry, StreamOptions{OneShot: true})
        → clients skip injectPendingState / savePendingIfAccumulated
  → streamHandler starts; events flow as usual
  → handlers detect isBtw → skip AppendMessage, skip session writes
  → on done/error/interrupt: render btw bottom rule, reset isBtw
```

---

## Detailed Changes

### 1. `internal/llm/client.go` — Add `OneShot` to StreamOptions

```go
type StreamOptions struct {
    SessionID string
    OneShot   bool
}
```

No interface change. All clients already receive `StreamOptions` via variadic.

---

### 2. `internal/llm/genkit.go` — Guard pending state

In `StreamChat` goroutine:

```go
oneShot := streamOptions(opts).OneShot

aiMessages := toGenkitMessages(messages)
var injectedPending []*ai.Message
if !oneShot {
    aiMessages, injectedPending = c.injectPendingState(aiMessages)
}
turnStartLen := len(aiMessages)
```

In `exitIncomplete`:
```go
// Only called when !oneShot (caller passes injectedPending=nil for oneShot)
```

Actually simpler: when `oneShot`, pass `nil` for `injectedPending` and skip calling
`exitIncomplete`. Instead, emit Done/Error directly:

```go
if err != nil {
    if oneShot {
        eventCh <- StreamEvent{Type: StreamEventTypeError, Error: err}
        return
    }
    c.exitIncomplete(eventCh, aiMessages, turnStartLen, injectedPending, err)
    return
}
```

And at the end of tool loop:
```go
if oneShot {
    eventCh <- StreamEvent{Type: StreamEventTypeDone}
    return
}
c.exitIncomplete(eventCh, aiMessages, turnStartLen, injectedPending, nil)
```

---

### 3. `internal/llm/anthropic.go` — Same pattern

Guard `injectPendingState` and `exitIncomplete` behind `!oneShot`. Same approach
as genkit: skip inject, skip save, emit terminal events directly.

---

### 4. `internal/llm/openai.go` — Same pattern

---

### 5. `internal/llm/openai_responses.go` — Same pattern

---

### 6. `internal/llm/openai_codex.go` — Same pattern

---

### 7. `internal/cli/repl/tooling/tool_registry.go` — Add `NewReadOnlyRegistry`

```go
func NewReadOnlyRegistry(
    workingDir string,
    permissionRequester *replpermissions.Requester,
) *tools.Registry {
    gitAwareness := filesystem.NewGitAwareness()
    _ = gitAwareness.LoadGitignore(filepath.Join(workingDir, ".gitignore"))
    guard := filesystem.NewGuard(workingDir, gitAwareness)

    registry := tools.NewRegistry()
    registry.Register(tools.NewReadFileTool(guard, permissionRequester))
    registry.Register(tools.NewGlobTool(guard, permissionRequester))
    registry.Register(tools.NewGrepTool(guard, permissionRequester))
    return registry
}
```

---

### 8. `internal/llm/systemprompt.go` — Add `BuildBtwPrompt`

A separate, focused system prompt for btw — does not include project instructions,
skills catalog, or the full coding-agent personality. Focuses on concise answers
with read-only tool access.

```go
const btwStaticPrompt = `You are a helpful assistant answering a side question in a coding session.
You have access to read-only tools (read_file, glob, grep) to explore the codebase.

# Guidelines
- Be concise and direct. Use GitHub-flavored markdown.
- Use tools to look up information when needed before answering.
- Reference code as file_path:line_number when relevant.
- Do not make changes to any files — you only have read access.`

func BuildBtwPrompt(workingDir string) string {
    return fmt.Sprintf("%s\n\nWorking directory: %s", btwStaticPrompt, workingDir)
}
```

---

### 9. `internal/cli/repl/appstate/state.go` — Add `StreamBtw`

```go
func (s *AppState) StreamBtw(
    ctx context.Context,
    cfg *config.ResolvedConfig,
    question string,
    readOnlyRegistry *tools.Registry,
    opts ...llm.StreamOptions,
) (<-chan llm.StreamEvent, error) {
    if s.llmClient == nil {
        return nil, nil
    }
    systemMsg := llm.Message{
        Role:    llm.RoleSystem,
        Content: llm.BuildBtwPrompt(s.workingDir),
    }
    userMsg := llm.Message{
        Role:    llm.RoleUser,
        Content: question,
    }
    messages := []llm.Message{systemMsg, userMsg}

    // Ensure OneShot is set
    streamOpts := llm.StreamOptions{OneShot: true}
    if len(opts) > 0 {
        streamOpts = opts[0]
        streamOpts.OneShot = true
    }
    return s.llmClient.StreamChat(ctx, messages, readOnlyRegistry, streamOpts)
}
```

---

### 10. `internal/cli/repl/commands/commands.go` — Add `Btw` constant

```go
const (
    Btw = "/btw"
    // ... existing
)
```

Add to `All`:
```go
{Btw, "Ask a side question without affecting conversation context"},
```

---

### 11. `internal/cli/repl/repl.go` — Add `isBtw` field

```go
type replModel struct {
    // ... existing fields
    isBtw            bool
    btwReadRegistry  *tools.Registry
}
```

`btwReadRegistry` is lazily created on first `/btw` invocation (or once during init).

---

### 12. `internal/cli/repl/command_handlers.go` — Add btw dispatch + handler

In `dispatchCommand`:
```go
case strings.HasPrefix(input, replcommands.Btw+" "):
    m.textarea.Reset()
    result, cmd := m.handleBtwCommand(input)
    return result, cmd, true
```

New method:
```go
func (m *replModel) handleBtwCommand(input string) (replModel, tea.Cmd) {
    question := strings.TrimSpace(strings.TrimPrefix(input, replcommands.Btw))
    if question == "" {
        m.output.AddError("Usage: /btw <question>", repltheme.ErrorStyle)
        m.updateViewportContent()
        m.viewport.GotoBottom()
        return *m, nil
    }

    if !m.appState.IsClientReady(m.ctx.cfg) {
        m.output.AddError("LLM client not initialized. Use /model to configure.", repltheme.ErrorStyle)
        m.updateViewportContent()
        m.viewport.GotoBottom()
        return *m, nil
    }

    if m.btwReadRegistry == nil {
        m.btwReadRegistry = repltooling.NewReadOnlyRegistry(
            m.ctx.workingDir,
            m.permissionRequester,
        )
    }

    ctx := m.startStreamContext()
    eventCh, err := m.appState.StreamBtw(ctx, m.ctx.cfg, question, m.btwReadRegistry)
    if err != nil {
        m.clearStreamCancel()
        m.output.AddError(err.Error(), repltheme.ErrorStyle)
        m.updateViewportContent()
        m.viewport.GotoBottom()
        return *m, nil
    }

    m.isBtw = true
    m.startLoading("btw...")
    m.streamHandler.Start(eventCh, m.loadingText)
    m.userScrolled = false
    m.adjustTextareaHeight()
    m.updateViewportContent()
    m.viewport.GotoBottom()

    return *m, tea.Batch(m.spinner.Tick, m.waitForAsyncEvent())
}
```

---

### 13. `internal/cli/repl/handlers.go` — Guard persistence in btw mode

**`handleLLMDone`:**
```go
func (m *replModel) handleLLMDone() (replModel, tea.Cmd) {
    if m.isCompacting {
        return m.handleCompactionDone()
    }
    if m.isBtw {
        return m.handleBtwDone()
    }
    // ... existing code
}
```

**New `handleBtwDone`:**
```go
func (m *replModel) handleBtwDone() (replModel, tea.Cmd) {
    m.stopLoading()
    m.clearStreamCancel()
    m.adjustTextareaHeight()
    responseLines, _ := m.streamHandler.HandleDone()
    m.isBtw = false

    m.output.AddLine(renderBtwRuleTop(m.btwRuleWidth()))
    for _, line := range responseLines {
        m.output.AddLine(line)
    }
    m.output.AddLine(renderBtwRuleBottom(m.btwRuleWidth()))
    m.output.AddEmptyLine()
    m.updateViewportContent()
    m.scrollToBottomIfFollowing()
    return *m, nil
}
```

**`handleLLMError` / `handleLLMIncomplete`:**
Add early return for `isBtw` that skips `AppendMessage` and session writes,
renders partial content between btw rules, and resets `m.isBtw = false`.

**`interruptStream`:**
Add guard:
```go
if m.isBtw {
    // skip AppendMessage, skip session persist
    m.isBtw = false
    // still show partial content + interrupted label inside btw rules
    ...
    return
}
```

---

### 14. `internal/cli/repl/theme/styles.go` — Add btw styles

Reuse `AccentColor` for both the rule lines and the "btw" label:

```go
BtwRuleStyle = lipgloss.NewStyle().
    Foreground(AccentColor)

BtwLabelStyle = lipgloss.NewStyle().
    Foreground(AccentColor).
    Bold(true)
```

No new color constant needed — leverages the existing `AccentColor`.

---

### 15. `internal/cli/repl/btw_render.go` (NEW) — Btw rule rendering

```go
package repl

func renderBtwRuleTop(width int) string {
    // "── btw " + repeat("─", remaining width)
    label := "── btw "
    remaining := width - len(label)
    if remaining < 0 {
        remaining = 0
    }
    return BtwLabelStyle.Render(label) + BtwRuleStyle.Render(strings.Repeat("─", remaining))
}

func renderBtwRuleBottom(width int) string {
    return BtwRuleStyle.Render(strings.Repeat("─", width))
}

func (m *replModel) btwRuleWidth() int {
    w := m.width
    if w <= 0 {
        w = m.viewport.Width()
    }
    if w <= 0 {
        w = 80
    }
    return w
}
```

The streaming header shown live during the stream will use `renderBtwRuleTop` as
a prefix in `updateViewportContent()` when `m.isBtw` is true. The stream handler's
live text appears below this top rule. On done, the full block (top rule + content +
bottom rule) replaces it. Rules expand to the full terminal width with no margins.

---

## Rendering Behavior

Full-width straight rule lines that extend to both edges of the terminal window.

**During streaming (live):**
```
── btw ────────────────────────────────────────────────────────────────────────
<streaming content rendered normally>
...
```

**After done:**
```
── btw ────────────────────────────────────────────────────────────────────────
The answer rendered with markdown...
...
───────────────────────────────────────────────────────────────────────────────
```

The rules use straight `─` characters and expand to the full terminal width.
The "btw" label appears inline in the top rule. Both the rules and label use
`AccentColor` (the existing project accent) to visually separate the block from
the main conversation output. No side borders — just top and bottom horizontal rules.

---

## Implementation Order

1. Add `OneShot` to `StreamOptions` + guard in all 5 clients → verify: existing tests pass
2. Add `BuildBtwPrompt` in `systemprompt.go` → verify: compiles
3. Add `NewReadOnlyRegistry` in tooling package → verify: compiles
4. Add `StreamBtw` in appstate → verify: compiles
5. Add `/btw` command constant + `All` entry → verify: compiles
6. Add `isBtw` + `btwReadRegistry` to replModel → verify: compiles
7. Add btw theme styles → verify: compiles
8. Add `btw_render.go` with rule rendering → verify: compiles
9. Add `handleBtwCommand` in command_handlers → verify: compiles
10. Add `handleBtwDone` + guards in handlers.go → verify: existing tests pass
11. Full test suite: `go test ./...`

---

## Risks & Mitigations

| Risk | Mitigation |
|------|-----------|
| Pending state corruption if OneShot logic is wrong | Unit test: call StreamChat with OneShot, verify pendingState stays nil |
| Stream handler reuse — btw and main could collide | Guard: btw cannot start while stream is active (same check as compaction) |
| Visual glitch if btw rules aren't closed on error | All error/interrupt paths reset `isBtw` and emit bottom rule |

---

## Out of Scope

- Session persistence for btw exchanges
- Full tool access (write/edit/bash) in btw mode
- Multi-turn btw conversations (it's always one-shot)
