# Add Thinking Effort Configuration

## Context

Users have no way to control thinking/reasoning effort in Keen Code. This adds:
1. A `StepThinking` in the `/model` setup flow for models whose registry entry has non-empty `thinking_efforts`
2. A `/thinking` runtime command to change effort on the fly using the current model's supported values
3. Persistent `thinking_effort` in `~/.keen/configs.json`

The registry stores provider-native effort values. The CLI adds an `off` choice on top of that:
- Anthropic UI: `off` + `low|medium|high|max`
- OpenAI UI: `off` + `low|medium|high|xhigh`
- Google AI UI: `off` + `low|medium|high`

Persist the provider-native value in config; represent `off` as `""` in config/runtime state so request builders can omit provider-specific reasoning fields cleanly.

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
    case "low", "medium", "high", "max":
        thinking := anthropic.ThinkingConfigParamUnion{
            OfAdaptive: &anthropic.ThinkingConfigAdaptiveParam{},
        }
        outCfg := anthropic.OutputConfigParam{
            Effort: anthropic.OutputConfigEffort(effort),
        }
        return thinking, outCfg, 32768
    default: // off / unset
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
```

`ThinkingConfigDisabledParam` can be constructed directly as
`&anthropic.ThinkingConfigDisabledParam{}` (the `Type` field is a `constant.Disabled`
which zero-marshals correctly), or via `anthropic.NewThinkingConfigDisabledParam()`.

The important change from Step 1 is that Anthropic should consume the raw registry
value directly; this layer should not collapse `max` back to `high`.

---

### 3b. Google AI — `internal/llm/genkit.go`

Add `thinkingEffort string` to `GenkitClient`. In `StreamChat`, append a
`ai.WithConfig(&genai.GenerateContentConfig{...})` option when effort is set:

```go
import "google.golang.org/genai"

if c.thinkingEffort != "" {
    opts = append(opts, ai.WithConfig(&genai.GenerateContentConfig{
        ThinkingConfig: &genai.ThinkingConfig{
            IncludeThoughts: true,
            ThinkingBudget:  budgetForEffort(c.thinkingEffort),
        },
    }))
}
```

Check the exact `google.golang.org/genai` API available in `go.mod` (`v1.41.0`) for
the exact helper signature, but in the installed SDK the relevant fields are
`ThinkingConfig.IncludeThoughts` and `ThinkingConfig.ThinkingBudget`.

Gemini is the only provider in this plan where the registry value is still a label
that Keen maps to an internal numeric budget. Keep that mapping local to the Google
client; do not push budgets into the registry.

---

### 3c. OpenAI — `internal/llm/openai_responses.go`

Add `thinkingEffort string` to `OpenAIResponsesClient`. In `StreamChat`, set
`params.Reasoning` when effort is non-empty:

```go
import "github.com/openai/openai-go/shared"

if c.thinkingEffort != "" {
    params.Reasoning = shared.ReasoningParam{
        Effort: reasoningEffortForLevel(c.thinkingEffort),
    }
}

func reasoningEffortForLevel(effort string) shared.ReasoningEffort {
    switch effort {
    case "low":    return shared.ReasoningEffortLow
    case "medium": return shared.ReasoningEffortMedium
    case "high":   return shared.ReasoningEffortHigh
    case "xhigh":  return shared.ReasoningEffort("xhigh")
    default:       return ""
    }
}
```

This needs to stay aligned with Step 1's registry values. The currently installed
`openai-go` version exposes constants only for `low|medium|high`, so `xhigh` must
either be passed as a raw string cast or be handled after an SDK upgrade.

---

### 3d. OpenAI-compatible (DeepSeek / Moonshot) — `internal/llm/openai.go`

Add `thinkingEffort string` field to `OpenAICompatibleClient` for symmetry; no-op
in request building (these providers have always-on reasoning).

---

## Step 4: Model selection — add `StepThinking`

**File:** `internal/cli/repl/model_selection.go`

Current: `StepProvider → StepModel → StepAPIKey`
New: `StepProvider → StepModel → StepThinking (if the selected model has registry-defined thinking efforts) → StepAPIKey`

Changes:
- Add `StepThinking` constant
- Add fields to `Model`: `ThinkingCursor int`, `ThinkingOptions []string`, `SelectedThinking string`
- After `StepModel` confirm: call `registry.GetModel(provider, model)` — if `SupportsThinkingEffort()` → build `ThinkingOptions` as `append([]string{"off"}, model.ThinkingEfforts...)` and go to `StepThinking`; else go to `StepAPIKey`
- Initial selection logic:
  - if current saved `resolvedCfg.ThinkingEffort == ""`, preselect `off`
  - else if current saved `resolvedCfg.ThinkingEffort` is in `ThinkingOptions`, preselect it
  - else if `medium` is supported, preselect `medium`
  - else preselect `off`
- Add `renderThinkingSelection()` view (same list-selector style as provider/model steps)
- In `complete()`:
  - store the selected provider-native effort in `GlobalConfig` / `ResolvedConfig`
  - map UI choice `off` to `""`
  - if the chosen model does not support configurable effort, clear `ThinkingEffort` to `""` so stale incompatible values do not carry across model switches

---

## Step 5: `/thinking` command

**File:** `internal/cli/repl/commands.go`

```go
{"/thinking", "Change thinking effort for the current model"},
```

**File:** `internal/cli/repl/repl.go`

Add constant `thinkingCommand = "/thinking"`.

In `handleEnterKey`, parse the argument from the input line:
```go
if input == thinkingCommand || strings.HasPrefix(input, thinkingCommand+" ") {
    modelMeta, ok := m.ctx.registry.GetModel(m.ctx.cfg.Provider, m.ctx.cfg.Model)
    if !ok || !modelMeta.SupportsThinkingEffort() {
        // show error: "Current model does not support configurable thinking"
        ...
    }

    effort := strings.TrimSpace(strings.TrimPrefix(input, thinkingCommand))
    allowed := append([]string{"off"}, modelMeta.ThinkingEfforts...)
    if !slices.Contains(allowed, effort) {
        // show error: "Usage: /thinking " + strings.Join(allowed, "|")
        ...
    }

    storedEffort := effort
    if effort == "off" {
        storedEffort = ""
    }

    m.ctx.cfg.ThinkingEffort = storedEffort
    m.ctx.globalCfg.ThinkingEffort = storedEffort
    if err := m.ctx.loader.Save(m.ctx.globalCfg); err != nil { ... }

    // Reinitialize LLM client with new effort
    newClient, err := llm.NewClient(m.ctx.cfg)
    // update appState.llmClient and show confirmation: "Thinking effort set to: EFFORT"
```

No new message type or UI selection flow required.

---

## Step 6: Display thinking status

**File:** `internal/cli/repl/repl.go`

`inputMetaView()`: append ` · thinking:EFFORT` to the status line when `cfg.ThinkingEffort != ""` and the current model has `SupportsThinkingEffort()`.

`buildInitialScreen()`: add a `Thinking: EFFORT` line (next to Model/Provider) when `cfg.ThinkingEffort != ""` and the current model supports configurable effort.

---

## Critical Files

| File | Change |
|------|--------|
| `providers/registry.yaml` | Add per-model `thinking_efforts` using provider-native values |
| `providers/loader.go` | `ThinkingEfforts []string`; `SupportsThinkingEffort()`; `GetModel()` helper |
| `internal/config/config.go` | `ThinkingEffort` in GlobalConfig + ResolvedConfig; copy in `Resolve()` |
| `internal/cli/cmd/root.go` | Set `ThinkingEffort` in manual ResolvedConfig construction |
| `internal/llm/models.go` | `ThinkingEffort` in ClientConfig + NewClient pass-through |
| `internal/llm/anthropic.go` | Enable/disable thinking based on non-empty effort; pass through `low|medium|high|max` directly |
| `internal/llm/genkit.go` | Map `low|medium|high` to Gemini `ThinkingBudget`; omit config when effort is empty |
| `internal/llm/openai_responses.go` | Set `params.Reasoning.Effort` from `low|medium|high|xhigh`; omit when effort is empty |
| `internal/llm/openai.go` | Store field; no-op |
| `internal/cli/repl/model_selection.go` | StepThinking in full `/model` flow using per-model option lists |
| `internal/cli/repl/commands.go` | Register `/thinking` |
| `internal/cli/repl/repl.go` | Handle `/thinking <effort>` against current model's option list; status display |

---

## SDK API Quick Reference

**anthropic-sdk-go v1.37.0**

```go
// Enable adaptive thinking with effort level:
params.Thinking = anthropic.ThinkingConfigParamUnion{
    OfAdaptive: &anthropic.ThinkingConfigAdaptiveParam{},
}
params.OutputConfig = anthropic.OutputConfigParam{
    Effort: anthropic.OutputConfigEffortMax, // Low / Medium / High / Xhigh / Max
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

The installed `openai-go` (`v1.8.2`) models `ReasoningEffort` as a string type but
only exposes constants for `low|medium|high`. If Step 1 keeps `xhigh` in the
registry, use `shared.ReasoningEffort("xhigh")` or upgrade the SDK before coding.

**google.golang.org/genai v1.41.0** (via Genkit's `googlegenai` plugin)

Installed SDK fields:
```go
ai.WithConfig(&genai.GenerateContentConfig{
    ThinkingConfig: &genai.ThinkingConfig{
        IncludeThoughts: true,
        ThinkingBudget:  ptr(int32(budgetForEffort(effort))),
    },
})
```

---

## Verification

1. `go mod tidy && go test ./...` — all tests pass
2. `/model` → `claude-sonnet-4-6` → thinking step appears with `off|low|medium|high|max`; if no saved compatible value exists, default is `medium`
3. `/model` → `deepseek-chat` → thinking step **skipped**
4. `/model` → `gpt-5.4` → thinking step appears with `off|low|medium|high|xhigh`
5. `/thinking max` on `claude-sonnet-4-6` and `/thinking xhigh` on `gpt-5.4` both persist in session + `~/.keen/configs.json`
6. `/thinking bad` shows usage with the current model's valid values, not a hard-coded global list
7. `/thinking off` clears `ThinkingEffort` in runtime/config and omits provider reasoning config on the next request
8. Switching from a model with `xhigh` or `max` to one that does not support that value preselects a compatible fallback (`medium` if available, else `off`)
9. Cold start: quit and relaunch `keen` → `ThinkingEffort` still applied (proves root.go fix)
10. Status line shows `thinking:max`, `thinking:xhigh`, etc. using the actual stored provider-native value
11. `claude-haiku-4-5`: `/model` skips thinking step; `/thinking` shows error
