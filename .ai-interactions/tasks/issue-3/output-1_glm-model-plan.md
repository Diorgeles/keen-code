# Implementation Plan: Support Z.ai GLM Models

## Issue Summary
GitHub Issue [#3](https://github.com/mochow13/keen-code/issues/3) — GLM models
from Z.ai are supported through the OpenAI completions API. We can add Z.ai as
another OpenAI-compatible provider, following the same pattern as DeepSeek and
MoonshotAI.

Z.ai (ZhipuAI) exposes an OpenAI-compatible Chat Completions endpoint at
`https://api.z.ai/api/paas/v4/`.

---

## Current Architecture (relevant touch-points)

| Layer | File | Role |
|---|---|---|
| Provider constants | `internal/config/config.go` | `ProviderDeepSeek`, `ProviderMoonshotAI`, etc. |
| Config types | `internal/config/config.go` | `ProviderConfig`, `ResolvedConfig`, `Resolve()` |
| LLM factory | `internal/llm/models.go` | `NewClient()` dispatches to client constructors |
| OpenAI-compat client | `internal/llm/openai.go` | `OpenAICompatibleClient`, `openAICompatibleBaseURL()` |
| Model selection UI | `internal/cli/repl/widgets/model_selection.go` | Provider list shown to user |

---

## Implementation Plan

### 1. Add provider constant

**File:** `internal/config/config.go`

- [ ] Add `ProviderZAI = "zai"` constant alongside the existing providers.

---

### 2. Register Z.ai in the LLM factory

**File:** `internal/llm/models.go`

- [ ] Add `config.ProviderZAI` to the existing `case` that already handles
      `config.ProviderDeepSeek` and `config.ProviderMoonshotAI`, since all three
      use `NewOpenAICompatibleClient`:

```go
case config.ProviderDeepSeek,
    config.ProviderMoonshotAI,
    config.ProviderZAI:
```

No new constructor or client type is needed.

---

### 3. Add default base URL for Z.ai

**File:** `internal/llm/openai.go`

- [ ] Add a case to `openAICompatibleBaseURL()`:

```go
case Provider(config.ProviderZAI):
    return "https://api.z.ai/api/paas/v4/", nil
```

This follows the exact pattern used for DeepSeek and MoonshotAI. When a user has
configured a custom `base_url` in their provider config, it will already take
precedence (the `NewOpenAICompatibleClient` constructor checks `cfg.BaseURL`
first before falling back to `openAICompatibleBaseURL()`).

---

### 4. Surface Z.ai in the model selection UI

**File:** `internal/cli/repl/widgets/model_selection.go`

- [ ] Add Z.ai to the provider list so it appears in the `/model` wizard.
      Find where providers are listed (the `StepProvider` options) and add an
      entry for `config.ProviderZAI` with a display name like `"Z.ai (GLM)"`.

---

### 5. Tests

**File:** `internal/llm/openai_test.go`

- [ ] Add test: `openAICompatibleBaseURL` returns the correct URL for
      `Provider(config.ProviderZAI)`.
- [ ] Add test: `NewOpenAICompatibleClient` with `ProviderZAI` succeeds and
      uses the default base URL.

**File:** `internal/llm/models_test.go`

- [ ] Add test: `NewClient` with `config.ProviderZAI` returns a non-nil client
      (follows existing test pattern if present).

**File:** `internal/config/config_test.go`

- [ ] No changes needed — config is provider-agnostic; existing tests cover the
      generic `Resolve()` flow.

---

### 6. Housekeeping

- [ ] Run `go test ./...` — all tests pass.
- [ ] Run `go mod tidy`.
- [ ] Update `CHANGELOG.md` `[Unreleased]` section:
      `- feat(llm): add Z.ai (GLM) as an OpenAI-compatible provider (#3)`

---

## File Change Summary

| File | Change type |
|---|---|
| `internal/config/config.go` | Edit — add `ProviderZAI` constant |
| `internal/llm/models.go` | Edit — add `config.ProviderZAI` to existing OpenAI-compat case |
| `internal/llm/openai.go` | Edit — add Z.ai base URL to `openAICompatibleBaseURL()` |
| `internal/cli/repl/widgets/model_selection.go` | Edit — add Z.ai to provider list |
| `internal/llm/openai_test.go` | Edit — add base URL and client creation tests |
| `CHANGELOG.md` | Edit — add unreleased entry |

---

## Notes

- **No new dependencies required.** Z.ai uses the standard OpenAI Chat
  Completions API, so the existing `openai-go` SDK handles it.
- **No new client type needed.** `OpenAICompatibleClient` already supports
  streaming, tool calling, and `reasoning_content` extraction — all of which
  work with the Z.ai API.
- **Reasoning content:** GLM models may or may not support the
  `reasoning_content` extension field. The existing extraction logic
  (`extractJSONStringField`) is a no-op when the field is absent, so no
  special handling is required.
- **Scope:** This is a minimal, low-risk change — approximately 6 lines of
  production code across 4 files, plus tests.
