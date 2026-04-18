# Add Thinking Effort Configuration

## Context

Users have no way to control thinking/reasoning effort in Keen Code. This adds:
1. A `StepThinking` in the `/model` setup flow (for models with `supports_thinking`)
2. A `/thinking` runtime command to change effort on the fly
3. Persistent `thinking_effort` in `~/.keen/configs.json`

Effort levels: `off` / `low` / `medium` / `high` (default cursor: `high`)

---

## Step 0: Upgrade anthropic-sdk-go

`go.mod` currently pins `github.com/anthropics/anthropic-sdk-go v1.19.0`. Upgrade to latest (≥ v1.37.0) to get typed `ThinkingConfigAdaptiveParam`, `ThinkingConfigDisabledParam`, and `OutputConfigParam` with effort enums.

```
go get github.com/anthropics/anthropic-sdk-go@latest
go mod tidy
go test ./...
```

---

## Step 1: Registry — add `supports_thinking` per model

**File:** `providers/registry.yaml`

Add `supports_thinking: true` to:
- Anthropic: `claude-opus-4-6`, `claude-sonnet-4-6` (not `claude-haiku-4-5`)
- OpenAI: `gpt-5.4`, `gpt-5.4-mini` (not `gpt-5.3-codex`)
- Google AI: `gemini-3.1-pro-preview`, `gemini-3-flash-preview`
- DeepSeek / Moonshot: **omit** (always-on, not user-configurable)

**File:** `providers/loader.go`

```go
// Add to Model struct:
SupportsThinking bool `yaml:"supports_thinking"`

// Add helper to *Registry:
func (r *Registry) GetModel(providerID, modelID string) (Model, bool)
```

---

## Step 2: Config — add `ThinkingEffort`

**File:** `internal/config/config.go`

```go
// GlobalConfig — add field:
ThinkingEffort string `json:"thinking_effort,omitempty"`

// ResolvedConfig — add field:
ThinkingEffort string

// Resolve() — copy into resolved:
resolved.ThinkingEffort = global.ThinkingEffort
```

**File:** `internal/cli/cmd/root.go`

The manual `ResolvedConfig` construction (~line 44) bypasses `Resolve()`. Add:
```go
resolvedCfg = &config.ResolvedConfig{
    Provider:       globalCfg.ActiveProvider,
    Model:          globalCfg.ActiveModel,
    APIKey:         providerCfg.APIKey,
    ThinkingEffort: globalCfg.ThinkingEffort,  // ADD
}
```

---

## Step 3: LLM clients — thread thinking effort through

**File:** `internal/llm/models.go`

```go
// ClientConfig — add field:
ThinkingEffort string

// NewClient — pass cfg.ThinkingEffort when constructing each client
```

**File:** `internal/llm/genkit.go`

Add `thinkingEffort string` to `GenkitClient`. Replace the hardcoded Anthropic block (~line 132):

```go
if c.provider == config.ProviderAnthropic {
    params := &anthropicsdk.MessageNewParams{MaxTokens: 16192}
    switch c.thinkingEffort {
    case "low", "medium", "high":
        params.Thinking = anthropicsdk.ThinkingConfigAdaptiveParam{}
        params.OutputConfig = anthropicsdk.OutputConfigParam{
            Effort: effortEnum(c.thinkingEffort), // maps to OutputConfigEffortLow/Medium/High
        }
        params.MaxTokens = 32768
    default: // "off" or ""
        params.Thinking = anthropicsdk.ThinkingConfigDisabledParam{}
    }
    opts = append(opts, ai.WithConfig(params))
}
```

For **Google AI**, map effort to `genai.ThinkingLevel`:
```go
if c.provider == config.ProviderGoogleAI {
    level := thinkingLevelForEffort(c.thinkingEffort) // OFF/LOW/MEDIUM/HIGH
    opts = append(opts, ai.WithConfig(&genai.GenerateContentConfig{
        ThinkingConfig: &genai.ThinkingConfig{ThinkingLevel: level},
    }))
}
```

**File:** `internal/llm/openai_responses.go`

Add `thinkingEffort string` to `OpenAIResponsesClient`. In `StreamChat`, if effort ≠ `""` and ≠ `"off"`:
```go
params.Reasoning = shared.ReasoningParam{Effort: reasoningEffort(c.thinkingEffort)}
// reasoningEffort maps low/medium/high → ReasoningEffortLow/Medium/High
```

**File:** `internal/llm/openai.go`

Add `thinkingEffort string` field for symmetry; no-op in request building (DeepSeek/Moonshot have always-on reasoning).

---

## Step 4: Model selection — add `StepThinking`

**File:** `internal/cli/repl/model_selection.go`

Current: `StepProvider → StepModel → StepAPIKey`
New: `StepProvider → StepModel → StepThinking (if supports_thinking) → StepAPIKey`

Changes:
- Add `StepThinking` constant
- Add fields to `Model`: `ThinkingCursor int`, `ThinkingOptions []string` (`["off","low","medium","high"]`), `SelectedThinking string`, `thinkingOnly bool`
- After `StepModel` confirm: call `registry.GetModel(provider, model)` — if `SupportsThinking` → `StepThinking`; else → `StepAPIKey`
- Default cursor on `"high"`
- Add `renderThinkingSelection()` view (same list-selector style as provider/model steps)
- In `complete()`: write `ThinkingEffort` to `GlobalConfig` and `ResolvedConfig`, then save
- Add `thinkingOnComplete func(effort string)` callback for thinking-only mode

**Thinking-only constructor** (for `/thinking` command):
```go
func NewThinkingOnly(
    cfg *config.ResolvedConfig,
    reg *providers.Registry,
    loader *config.Loader,
    globalCfg *config.GlobalConfig,
    onComplete func(effort string),
) *Model
```
Starts at `StepThinking`, skips all other steps. On confirm: saves `GlobalConfig.ThinkingEffort`, calls `onComplete(effort)`.

---

## Step 5: `/thinking` command

**File:** `internal/cli/repl/commands.go`

```go
{"/thinking", "Change thinking effort level"},
```

**File:** `internal/cli/repl/repl.go`

Add constant `thinkingCommand = "/thinking"`.

In `handleEnterKey` (after existing command cases):
```go
case thinkingCommand:
    m, ok := ctx.registry.GetModel(ctx.cfg.Provider, ctx.cfg.Model)
    if !ok || !m.SupportsThinking {
        // show inline error: "Current model does not support configurable thinking"
        break
    }
    return startThinkingSelection(ctx)
```

`startThinkingSelection` creates `NewThinkingOnly(...)` with callback that:
1. Sets `ctx.cfg.ThinkingEffort = effort`
2. Saves via `ctx.loader.Save(ctx.globalCfg)`
3. Reinitializes LLM client via `llm.NewClient(...)` and updates `appState`
4. Returns `thinkingSelectionCompleteMsg`

Handle `thinkingSelectionCompleteMsg` in `Update()` analogously to `modelSelectionCompleteMsg`.

---

## Step 6: Display thinking status

**File:** `internal/cli/repl/repl.go`

`inputMetaView()`: append ` · thinking:EFFORT` to the status line when `cfg.ThinkingEffort` is set and not `"off"` and current model has `SupportsThinking`.

`buildInitialScreen()`: add a `Thinking: EFFORT` line (next to Model/Provider) when applicable.

---

## Critical Files

| File | Change |
|------|--------|
| `go.mod` | Upgrade anthropic-sdk-go to ≥ v1.37.0 |
| `providers/registry.yaml` | Add `supports_thinking: true` per model |
| `providers/loader.go` | `SupportsThinking bool`; `GetModel()` helper |
| `internal/config/config.go` | `ThinkingEffort` in GlobalConfig + ResolvedConfig; copy in `Resolve()` |
| `internal/cli/cmd/root.go` | Set `ThinkingEffort` in manual ResolvedConfig construction |
| `internal/llm/models.go` | `ThinkingEffort` in ClientConfig + NewClient pass-through |
| `internal/llm/genkit.go` | Anthropic adaptive thinking + OutputConfig.Effort; Google ThinkingLevel; MaxTokens 32768 when thinking on |
| `internal/llm/openai_responses.go` | Reasoning.Effort when effort ≠ off |
| `internal/llm/openai.go` | Store field; no-op |
| `internal/cli/repl/model_selection.go` | StepThinking; full vs thinking-only constructor |
| `internal/cli/repl/commands.go` | Register `/thinking` |
| `internal/cli/repl/repl.go` | Handle `/thinking`; status display; `startThinkingSelection()` |

---

## Verification

1. `go mod tidy && go test ./...` — all tests pass
2. `/model` → `claude-sonnet-4-6` → thinking step appears (`off/low/medium/high`, default `high`)
3. `/model` → `deepseek-chat` → thinking step **skipped**
4. `/thinking` → shows selector only (no API key prompt); change persists in session + `~/.keen/configs.json`
5. `/thinking` with `deepseek-chat` active → clear error message
6. Cold start: quit and relaunch `keen` → `ThinkingEffort` still applied (proves root.go fix)
7. Status line shows `thinking:high` when applicable
8. `claude-haiku-4-5`: `/model` skips thinking step; `/thinking` shows error
