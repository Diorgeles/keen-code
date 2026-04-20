# Add Thinking Effort Configuration

## Context

Users have no way to control thinking/reasoning effort in Keen Code. This adds:
1. A `StepThinking` in the `/model` setup flow (for models with `supports_thinking`)
2. A `/thinking` runtime command to change effort on the fly
3. Persistent `thinking_effort` in `~/.keen/configs.json`

Effort levels: `off` / `low` / `medium` / `high` (default cursor: `high`)

---

## Step 0: ~~Upgrade anthropic-sdk-go~~ (already done)

`go.mod` already pins `github.com/anthropics/anthropic-sdk-go v1.37.0`, which provides
`ThinkingConfigAdaptiveParam`, `ThinkingConfigDisabledParam`, `ThinkingConfigParamUnion`,
`OutputConfigParam`, and `OutputConfigEffort` enums. No upgrade needed.

---

## Step 1: Registry — add per-model `thinking_efforts`

**File:** `providers/registry.yaml`

Replace the boolean `supports_thinking` idea with explicit provider-native effort options:

- Anthropic:
  - `claude-opus-4-6`: `thinking_efforts: ["low", "medium", "high", "max"]`
  - `claude-sonnet-4-6`: `thinking_efforts: ["low", "medium", "high", "max"]`
  - `claude-haiku-4-5`: **omit** (`extended thinking` exists, but Anthropic's `effort` parameter is not supported on Haiku 4.5)
- OpenAI:
  - `gpt-5.4`: `thinking_efforts: ["low", "medium", "high", "xhigh"]`
  - `gpt-5.4-mini`: `thinking_efforts: ["low", "medium", "high", "xhigh"]`
  - `gpt-5.3-codex`: `thinking_efforts: ["low", "medium", "high", "xhigh"]`
- Google AI:
  - `gemini-3.1-pro-preview`: `thinking_efforts: ["low", "medium", "high"]`
  - `gemini-3-flash-preview`: `thinking_efforts: ["low", "medium", "high"]`
- DeepSeek / Moonshot: **omit** (always-on or not part of this effort-level work)

Keep the raw provider values in the registry. Do **not** normalize these to a shared `off/low/medium/high` set in the registry layer:
- Gemini does not get an `off`-equivalent registry value in this plan
- Anthropic uses `max`
- OpenAI uses `xhigh`; represent "no extra reasoning" by omitting `params.Reasoning` rather than sending `"none"`

**File:** `providers/loader.go`

```go
// Add to Model struct:
ThinkingEfforts []string `yaml:"thinking_efforts"`

func (m Model) SupportsThinkingEffort() bool {
    return len(m.ThinkingEfforts) > 0
}

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

### Architecture note
Anthropic is **not** routed through Genkit. It uses its own `AnthropicClient` backed
directly by `anthropic-sdk-go`. The routing in `internal/llm/models.go` is:

| Provider          | Client                  |
|-------------------|-------------------------|
| `anthropic`       | `AnthropicClient`       |
| `googleai`        | `GenkitClient`          |
| `openai`          | `OpenAIResponsesClient` |
| `deepseek` / `moonshotai` | `OpenAICompatibleClient` (no-op) |

**File:** `internal/llm/models.go`

```go
// ClientConfig — add field:
ThinkingEffort string

// NewClient — pass cfg.ThinkingEffort into each relevant constructor
```

---

### 3a. Anthropic — `internal/llm/anthropic.go`

Add `thinkingEffort string` to `AnthropicClient`. In `StreamChat`, set
`params.Thinking` and `params.OutputConfig` / `params.MaxTokens` before each turn:

```go
// Helper:
func anthropicThinkingParams(effort string) (anthropic.ThinkingConfigParamUnion, anthropic.OutputConfigParam, int64) {
    switch effort {
    case "low", "medium", "high":
        thinking := anthropic.ThinkingConfigParamUnion{
            OfAdaptive: &anthropic.ThinkingConfigAdaptiveParam{},
        }
        outCfg := anthropic.OutputConfigParam{
            Effort: effortToOutputConfigEffort(effort),
        }
        return thinking, outCfg, 32768
    default: // "off" or ""
        thinking := anthropic.ThinkingConfigParamUnion{
            OfDisabled: anthropic.NewThinkingConfigDisabledParam().Ptr(), // or just &anthropic.ThinkingConfigDisabledParam{}
        }
        return thinking, anthropic.OutputConfigParam{}, anthropicMaxTokens
    }
}

// In StreamChat, when building params:
thinking, outCfg, maxTok := anthropicThinkingParams(c.thinkingEffort)
params := anthropic.MessageNewParams{
    Model:        c.model,
    MaxTokens:    maxTok,
    Messages:     msgParams,
    Thinking:     thinking,
    OutputConfig: outCfg,
}

// Effort mapping:
func effortToOutputConfigEffort(effort string) anthropic.OutputConfigEffort {
    switch effort {
    case "low":    return anthropic.OutputConfigEffortLow
    case "medium": return anthropic.OutputConfigEffortMedium
    default:       return anthropic.OutputConfigEffortHigh
    }
}
```

`ThinkingConfigDisabledParam` can be constructed directly as
`&anthropic.ThinkingConfigDisabledParam{}` (the `Type` field is a `constant.Disabled`
which zero-marshals correctly), or via `anthropic.NewThinkingConfigDisabledParam()`.

---

### 3b. Google AI — `internal/llm/genkit.go`

Add `thinkingEffort string` to `GenkitClient`. In `StreamChat`, append a
`ai.WithConfig(&genai.GenerateContentConfig{...})` option when effort is set:

```go
import "google.golang.org/genai"

if c.thinkingEffort != "" && c.thinkingEffort != "off" {
    level := thinkingLevelForEffort(c.thinkingEffort) // maps to genai.ThinkingMode* constants
    opts = append(opts, ai.WithConfig(&genai.GenerateContentConfig{
        ThinkingConfig: &genai.ThinkingConfig{ThinkingBudget: budgetForLevel(level)},
    }))
}

// OR use the ThinkingMode enum if the googlegenai plugin exposes it:
// ThinkingModeEnabled / ThinkingModeDisabled / ThinkingModeDynamic
```

Check the exact `google.golang.org/genai` API available in `go.mod` (`v1.41.0`) for
the correct field name (`ThinkingBudget` vs `ThinkingMode`).

---

### 3c. OpenAI — `internal/llm/openai_responses.go`

Add `thinkingEffort string` to `OpenAIResponsesClient`. In `StreamChat`, set
`params.Reasoning` when effort ≠ `""` and ≠ `"off"`:

```go
import "github.com/openai/openai-go/shared"

if c.thinkingEffort != "" && c.thinkingEffort != "off" {
    params.Reasoning = shared.ReasoningParam{
        Effort: reasoningEffortForLevel(c.thinkingEffort),
    }
}

func reasoningEffortForLevel(effort string) shared.ReasoningEffort {
    switch effort {
    case "low":    return shared.ReasoningEffortLow
    case "medium": return shared.ReasoningEffortMedium
    default:       return shared.ReasoningEffortHigh
    }
}
```

---

### 3d. OpenAI-compatible (DeepSeek / Moonshot) — `internal/llm/openai.go`

Add `thinkingEffort string` field to `OpenAICompatibleClient` for symmetry; no-op
in request building (these providers have always-on reasoning).

---

## Step 4: Model selection — add `StepThinking`

**File:** `internal/cli/repl/model_selection.go`

Current: `StepProvider → StepModel → StepAPIKey`
New: `StepProvider → StepModel → StepThinking (if supports_thinking) → StepAPIKey`

Changes:
- Add `StepThinking` constant
- Add fields to `Model`: `ThinkingCursor int`, `ThinkingOptions []string` (`["off","low","medium","high"]`), `SelectedThinking string`
- After `StepModel` confirm: call `registry.GetModel(provider, model)` — if `SupportsThinking` → `StepThinking`; else → `StepAPIKey`
- Default cursor on `"high"` (index 3)
- Add `renderThinkingSelection()` view (same list-selector style as provider/model steps)
- In `complete()`: write `ThinkingEffort` to `GlobalConfig` and `ResolvedConfig`, then save

---

## Step 5: `/thinking` command

**File:** `internal/cli/repl/commands.go`

```go
{"/thinking", "Change thinking effort (off|low|medium|high)"},
```

**File:** `internal/cli/repl/repl.go`

Add constant `thinkingCommand = "/thinking"`.

In `handleEnterKey`, parse the argument from the input line:
```go
case thinkingCommand:
    effort := strings.TrimPrefix(strings.TrimSpace(input), "/thinking ")
    valid := map[string]bool{"off": true, "low": true, "medium": true, "high": true}
    if !valid[effort] {
        // show error: "Usage: /thinking off|low|medium|high"
        break
    }
    m, ok := ctx.registry.GetModel(ctx.cfg.Provider, ctx.cfg.Model)
    if !ok || !m.SupportsThinking {
        // show error: "Current model does not support configurable thinking"
        break
    }
    // Apply immediately:
    ctx.cfg.ThinkingEffort = effort
    ctx.globalCfg.ThinkingEffort = effort
    ctx.loader.Save(ctx.globalCfg)
    // Reinitialize LLM client with new effort
    newClient, err := llm.NewClient(llm.ClientConfig{...cfg fields..., ThinkingEffort: effort})
    // update appState.llmClient
    // show confirmation: "Thinking effort set to: EFFORT"
```

No new message type or UI selection flow required.

---

## Step 6: Display thinking status

**File:** `internal/cli/repl/repl.go`

`inputMetaView()`: append ` · thinking:EFFORT` to the status line when `cfg.ThinkingEffort` is set and not `"off"` and current model has `SupportsThinking`.

`buildInitialScreen()`: add a `Thinking: EFFORT` line (next to Model/Provider) when applicable.

---

## Critical Files

| File | Change |
|------|--------|
| `providers/registry.yaml` | Add `supports_thinking: true` per model |
| `providers/loader.go` | `SupportsThinking bool`; `GetModel()` helper |
| `internal/config/config.go` | `ThinkingEffort` in GlobalConfig + ResolvedConfig; copy in `Resolve()` |
| `internal/cli/cmd/root.go` | Set `ThinkingEffort` in manual ResolvedConfig construction |
| `internal/llm/models.go` | `ThinkingEffort` in ClientConfig + NewClient pass-through |
| `internal/llm/anthropic.go` | Set `params.Thinking` (ThinkingConfigParamUnion) + `params.OutputConfig.Effort` + MaxTokens 32768 when thinking on |
| `internal/llm/genkit.go` | Google AI ThinkingConfig via `ai.WithConfig`; no Anthropic code here |
| `internal/llm/openai_responses.go` | `params.Reasoning.Effort` when effort ≠ off |
| `internal/llm/openai.go` | Store field; no-op |
| `internal/cli/repl/model_selection.go` | StepThinking in full `/model` flow |
| `internal/cli/repl/commands.go` | Register `/thinking` |
| `internal/cli/repl/repl.go` | Handle `/thinking <effort>` argument; status display |

---

## SDK API Quick Reference

**anthropic-sdk-go v1.37.0**

```go
// Enable adaptive thinking with effort level:
params.Thinking = anthropic.ThinkingConfigParamUnion{
    OfAdaptive: &anthropic.ThinkingConfigAdaptiveParam{},
}
params.OutputConfig = anthropic.OutputConfigParam{
    Effort: anthropic.OutputConfigEffortHigh, // Low / Medium / High / Xhigh / Max
}
params.MaxTokens = 32768

// Disable thinking:
params.Thinking = anthropic.ThinkingConfigParamUnion{
    OfDisabled: &anthropic.ThinkingConfigDisabledParam{},
}
params.MaxTokens = 16192 // anthropicMaxTokens
```

**openai-go** (Responses API)

```go
params.Reasoning = shared.ReasoningParam{
    Effort: shared.ReasoningEffortHigh, // Low / Medium / High
}
```

**google.golang.org/genai v1.41.0** (via Genkit's `googlegenai` plugin)

Verify field names against the installed version before coding; likely:
```go
ai.WithConfig(&genai.GenerateContentConfig{
    ThinkingConfig: &genai.ThinkingConfig{
        ThinkingBudget: ptr(int32(budgetForEffort(effort))),
        // or IncludeThoughts: true
    },
})
```

---

## Verification

1. `go mod tidy && go test ./...` — all tests pass
2. `/model` → `claude-sonnet-4-6` → thinking step appears (`off/low/medium/high`, default `high`)
3. `/model` → `deepseek-chat` → thinking step **skipped**
4. `/thinking high` → change persists in session + `~/.keen/configs.json`; `/thinking bad` → usage error
5. `/thinking off` with `deepseek-chat` active → clear error message
6. Cold start: quit and relaunch `keen` → `ThinkingEffort` still applied (proves root.go fix)
7. Status line shows `thinking:high` when applicable
8. `claude-haiku-4-5`: `/model` skips thinking step; `/thinking` shows error
