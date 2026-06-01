# `/adversary` Command — Implementation Plan

## Context

Users working with Keen Code want to catch bugs, security issues, and blind spots in the agent's
work without relying solely on the primary model. `/adversary` addresses this by invoking a
separately-configured LLM that adversarially reviews the full conversation — finding problems
rather than validating the primary agent's output. It is modelled closely on `/btw` as a
side-channel that never enters the main session history.

## Design Summary

- `/adversary` — default adversarial review (issues, bugs, risks, security)
- `/adversary <prompt>` — custom focus
- `/adversary model` — open the interactive model picker to configure the adversary model
- Full conversation context (not truncated like `/btw`'s last-10)
- Read-only tools: `read_file`, `glob`, `grep`, `web_fetch`. No `bash`, `write_file`, `edit_file`, `call_mcp_tool`
- Rendered as a `SecondaryColor` (teal) left-border block with an "adversary" chip label
- Separate spinner (`spinner.Points`, `SecondaryColor`) from main (`Pulse`) and btw (`MiniDot`)
- Output is NOT added to main session history; each invocation starts fresh
- Adversary provider/model stored separately in `GlobalConfig` (`adversary_provider`, `adversary_model`)

---

## Implementation Steps

### 1. `internal/config/config.go` — Add adversary config fields

Add two fields to `GlobalConfig`:

```go
AdversaryProvider string `json:"adversary_provider,omitempty"`
AdversaryModel    string `json:"adversary_model,omitempty"`
```

Add a helper:

```go
func ResolveAdversary(global *GlobalConfig) (*ResolvedConfig, error) {
    if global.AdversaryProvider == "" || global.AdversaryModel == "" {
        return nil, fmt.Errorf("adversary model not configured")
    }
    provCfg := global.Providers[global.AdversaryProvider]
    return &ResolvedConfig{
        Provider: global.AdversaryProvider,
        Model:    global.AdversaryModel,
        APIKey:   provCfg.APIKey,
        BaseURL:  provCfg.BaseURL,
        AuthMode: AuthModeForProvider(global.AdversaryProvider),
    }, nil
}
```

---

### 2. `internal/llm/systemprompt.go` — Add `BuildAdversaryPrompt`

```go
const adversaryPrompt = `You are an adversarial critic for Keen Code.
Your job is to find problems — in code, in ideas, in plans, and in reasoning.

When the conversation involves code changes: find bugs, logic errors, security vulnerabilities,
missing edge cases, and risks the main agent overlooked. Use your read tools to inspect files
when you need evidence. Reference file:line when citing code.

When the conversation involves ideas, suggestions, or plans: challenge the assumptions, question
the rationale, surface alternatives, and identify what could go wrong. Do not simply validate
what the main agent proposed.

Be direct and specific. If nothing significant is wrong, say so briefly — do not invent problems.`

func BuildAdversaryPrompt(workingDir string) string {
    return adversaryPrompt + fmt.Sprintf("\n\nWorking directory: %s", workingDir)
}
```

---

### 3. `internal/cli/repl/appstate/state.go` — Add adversary client and `StreamAdversary`

Add field to `AppState`:

```go
adversaryClient llm.LLMClient
```

Add methods:

```go
func (s *AppState) SetAdversaryClient(client llm.LLMClient)
func (s *AppState) IsAdversaryClientReady() bool  // returns adversaryClient != nil

func (s *AppState) StreamAdversary(ctx context.Context, focus string, opts ...llm.StreamOptions) (<-chan llm.StreamEvent, error) {
    // Full history — no truncation (unlike btwContext)
    history := s.GetMessages()
    messages := make([]llm.Message, 0, 2+len(history))
    messages = append(messages, llm.Message{Role: llm.RoleSystem, Content: llm.BuildAdversaryPrompt(s.workingDir)})
    messages = append(messages, history...)
    instruction := "Review this conversation based on your responsibilities."
    if focus != "" {
        instruction = focus
    }
    messages = append(messages, llm.Message{Role: llm.RoleUser, Content: instruction})
    readOnlyRegistry := s.toolRegistry.Without("write_file", "edit_file", "bash", "call_mcp_tool")
    return s.adversaryClient.StreamChat(ctx, messages, readOnlyRegistry, llm.StreamOptions{OneShot: true})
}
```

---

### 4. `internal/cli/repl/stream_msgs.go` — Add adversary message types

Append after the btw types (lines 54–57):

```go
type adversaryChunkMsg string
type adversaryDoneMsg  struct{}
type adversaryErrorMsg struct{ err error }
```

---

### 5. `internal/cli/repl/theme/styles.go` — Add adversary styles

Append using the existing `SecondaryColor`:

```go
AdversaryBorderStyle = lipgloss.NewStyle().Foreground(SecondaryColor)
AdversaryLabelStyle  = lipgloss.NewStyle().Foreground(TextPrimaryColor)
AdversaryChipStyle   = lipgloss.NewStyle().
    Background(SecondaryColor).
    Foreground(lipgloss.Color("#000000")).
    Bold(true).
    Padding(0, 1)
```

---

### 6. `internal/cli/repl/commands/commands.go` — Register the command

Add constants:

```go
Adversary      = "/adversary"
AdversaryModel = "/adversary model"
```

Add to both `All` (for help) and `Suggestions` (for autocomplete):

```go
{Adversary,      "Adversarially review the conversation for issues, bugs, risks, and security problems"},
{AdversaryModel, "Configure the adversary model"},
```

---

### 7. `internal/cli/repl/repl.go` — Extend `replContext`, `replModel`, `initialModel`

Declare a new struct to hold all adversary state, and add a single field to `replModel`:

```go
type adversaryState struct {
    streamHandler   *StreamHandler
    streamCancel    context.CancelFunc
    lines           []string
    focus           string
    showSpinner     bool
    spinner         spinner.Model
    modelSelection  *replwidgets.Model
}
```

```go
// in replModel
adversary adversaryState
```

In `initialModel`, initialize alongside the btw spinner (`bs`):

```go
as := spinner.New()
as.Spinner = spinner.Points
as.Style = lipgloss.NewStyle().Foreground(repltheme.SecondaryColor)
```

In `updateViewportContent`, append adversary block after btw block:

```go
if m.adversary.streamHandler != nil && m.adversary.streamHandler.IsActive() {
    content.WriteString(m.renderAdversaryInline(contentWidth))
} else if m.adversary.lines != nil {
    content.WriteString(m.renderAdversaryInlineFinished(contentWidth))
}
```

In `handleSpinnerTick`, add adversary spinner branch alongside the btw branch. Extend the early-return guard:

```go
if !m.showSpinner && !m.btwShowSpinner && !m.adversary.showSpinner {
    return m, nil, false
}
```

Add `consumeAdversaryModelSelectionResult` mirroring `consumeModelSelectionResult`, and wire it into `updateNormalMode`. Also add `m.adversary.modelSelection` key routing in `handleKeyMsg` mirroring the `modelSelection` routing block.

---

### 8. `internal/cli/repl/stream_render.go` — Add adversary render functions

Mirror the four btw render functions exactly, substituting adversary styles and the "adversary" chip label:

```go
func renderAdversaryQuestionHeader(focus string) string
func renderAdversaryLeftBorder(line string) string
func (m *replModel) renderAdversaryInline(width int) string
func (m *replModel) renderAdversaryInlineFinished(width int) string
```

The `adversaryFocus` is shown next to the chip only when non-empty (e.g. "adversary — look for SQL injection risks"). When empty, only the chip is rendered.

---

### 9. `internal/cli/repl/repl_helpers.go` — Add adversary helpers

```go
func waitForAdversaryEvent(ch <-chan llm.StreamEvent) tea.Cmd
// mirrors waitForBtwEvent; maps events to adversaryChunkMsg / adversaryDoneMsg / adversaryErrorMsg

func (m *replModel) cancelAdversaryStream()
// cancels context, resets m.adversary.lines, m.adversary.showSpinner

func (m *replModel) flushAdversaryToOutput()
// moves m.adversary.lines into the main output buffer, then clears it

func (m *replModel) buildAdversaryClient() error
// calls config.ResolveAdversary, then llm.NewClient, then m.appState.SetAdversaryClient
```

Also call `m.flushAdversaryToOutput()` in the same place `flushBtwToOutput()` is called when a new user turn begins (around `repl.go:239`).

---

### 10. `internal/cli/repl/handlers.go` — Add `handleAdversaryStreamMsg`

Mirror `handleBtwStreamMsg` (line 668), dispatching on `adversaryChunkMsg`, `adversaryDoneMsg`, `adversaryErrorMsg`. Wire into `handleLLMStreamMsg` immediately after the btw handler call:

```go
if updated, cmd, handled := m.handleAdversaryStreamMsg(msg); handled {
    return updated, cmd, true
}
```

---

### 11. `internal/cli/repl/command_handlers.go` — Wire `/adversary`

**In `handleEnterKey`**, intercept before `dispatchCommand` (same placement as the `/btw` intercept, line ~224):

```go
if input == replcommands.Adversary || strings.HasPrefix(input, replcommands.Adversary+" ") {
    m.history.Push(input)
    m.textarea.Reset()
    result, cmd := m.handleAdversaryCommand(input)
    return result, cmd
}
```

**Add `handleAdversaryCommand`**:

```go
func (m *replModel) handleAdversaryCommand(input string) (replModel, tea.Cmd) {
    arg := strings.TrimSpace(strings.TrimPrefix(input, replcommands.Adversary))

    if arg == "model" {
        return m.startAdversaryModelSelection(), nil
    }

    if m.ctx.globalCfg.AdversaryProvider == "" || m.ctx.globalCfg.AdversaryModel == "" {
        // show "Run /adversary model to configure an adversary model" in muted style
        return *m, nil
    }

    if !m.appState.IsAdversaryClientReady() {
        if err := m.buildAdversaryClient(); err != nil {
            // show error
            return *m, nil
        }
    }

    m.cancelAdversaryStream()  // cancel any in-flight stream
    m.flushAdversaryToOutput() // persist previous result to output

    ctx, cancel := context.WithCancel(context.Background())
    m.adversary.streamCancel = cancel

    eventCh, err := m.appState.StreamAdversary(ctx, arg)
    if err != nil { /* show error, cancel, return */ }

    m.adversary.lines = nil
    m.adversary.focus = arg
    m.adversary.streamHandler.Start(eventCh, nextLoadingText())
    m.adversary.showSpinner = true
    m.updateViewportContent()
    m.viewport.GotoBottom()

    return *m, tea.Batch(m.adversary.spinner.Tick, waitForAdversaryEvent(eventCh))
}
```

**Add `startAdversaryModelSelection`**:

```go
func (m *replModel) startAdversaryModelSelection() replModel {
    // Save current main model values — the widget will overwrite Active* before calling onComplete
    savedProvider := m.ctx.globalCfg.ActiveProvider
    savedModel    := m.ctx.globalCfg.ActiveModel

    onComplete := func(provider, model, apiKey string) error {
        // Move widget's Active* writes into Adversary* fields
        m.ctx.globalCfg.AdversaryProvider = provider
        m.ctx.globalCfg.AdversaryModel    = model
        // Restore main model — widget overwrote these
        m.ctx.globalCfg.ActiveProvider = savedProvider
        m.ctx.globalCfg.ActiveModel    = savedModel
        if err := m.ctx.loader.Save(m.ctx.globalCfg); err != nil {
            return err
        }
        return m.buildAdversaryClient()
    }
    adversaryResolved, _ := config.ResolveAdversary(m.ctx.globalCfg)
    if adversaryResolved == nil {
        adversaryResolved = &config.ResolvedConfig{}
    }
    m.adversary.modelSelection = replwidgets.New(
        m.ctx.registry, m.ctx.globalCfg, m.ctx.loader, adversaryResolved, onComplete,
    )
    m.updateViewportContent()
    m.viewport.GotoBottom()
    return *m
}
```

Add Esc handling: when `m.adversary.streamHandler.IsActive()`, Esc cancels the adversary stream (checked before the btw Esc block).

---

## Implementation Order

Dependencies flow in this order:

1. `config.go` — no dependencies
2. `systemprompt.go` — no dependencies
3. `appstate/state.go` — depends on 1, 2
4. `stream_msgs.go` — no dependencies
5. `theme/styles.go` — no dependencies
6. `commands/commands.go` — no dependencies
7. `stream_render.go` — depends on 5
8. `repl_helpers.go` — depends on 3, 4
9. `handlers.go` — depends on 3, 4, 7, 8
10. `repl.go` — depends on 5, 6, 7, 8, 9
11. `command_handlers.go` — depends on all prior

---

## Critical Files

| File | Change |
|------|--------|
| `internal/config/config.go` | Add `AdversaryProvider`, `AdversaryModel` to `GlobalConfig`; add `ResolveAdversary` |
| `internal/llm/systemprompt.go` | Add `BuildAdversaryPrompt` |
| `internal/cli/repl/appstate/state.go` | Add `adversaryClient`, `SetAdversaryClient`, `IsAdversaryClientReady`, `StreamAdversary` |
| `internal/cli/repl/stream_msgs.go` | Add 3 adversary message types |
| `internal/cli/repl/theme/styles.go` | Add 3 adversary styles using `SecondaryColor` |
| `internal/cli/repl/commands/commands.go` | Register `Adversary` and `AdversaryModel` constants |
| `internal/cli/repl/repl.go` | Add `adversaryState` struct and `adversary adversaryState` field to `replModel`; update viewport, spinner tick, key routing |
| `internal/cli/repl/stream_render.go` | Add 4 adversary render functions |
| `internal/cli/repl/repl_helpers.go` | Add `waitForAdversaryEvent`, `cancelAdversaryStream`, `flushAdversaryToOutput`, `buildAdversaryClient` |
| `internal/cli/repl/handlers.go` | Add `handleAdversaryStreamMsg`; wire into `handleLLMStreamMsg` |
| `internal/cli/repl/command_handlers.go` | Add `handleAdversaryCommand`, `startAdversaryModelSelection`; wire into `handleEnterKey` and `handleKeyMsg` |

---

## Verification

1. `go build ./...` — clean compile
2. `go test -race ./...` — all tests pass, including existing btw and model picker tests
3. **Config round-trip**: `/adversary model` → select a provider/model → inspect `~/.keen/configs.json`: `adversary_provider` and `adversary_model` written; `active_provider`/`active_model` unchanged
4. **No-config path**: with `adversary_provider` absent, `/adversary` prints "Run /adversary model to configure an adversary model" without crashing
5. **Stream rendering**: with adversary configured, `/adversary` renders a teal `▌` left-border block with an "adversary" chip; main conversation unchanged after stream completes
6. **Custom focus**: `/adversary look for SQL injection risks` uses that string as the chip label and as the final user message to the adversary model
7. **Tool restriction**: unit test that `toolRegistry.Without("write_file", "edit_file", "bash", "call_mcp_tool")` produces a registry containing only `read_file`, `glob`, `grep`, `web_fetch` (and any MCP tools if present — confirm `call_mcp_tool` exclusion is correct)
8. **Esc cancellation**: Esc while adversary is streaming cancels cleanly, no panic
9. **Concurrent btw**: `/btw` after `/adversary` runs without interfering with each other's spinner or line buffers
10. **Main model picker regression**: `/model` still correctly updates the main provider/model and reinitializes the main LLM client after the `onComplete` signature change
