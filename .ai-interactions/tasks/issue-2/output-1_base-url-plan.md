# Implementation Plan: Support `baseUrl` as a Model Setting Param

## Issue Summary
Users should be able to provide an optional custom `baseUrl` during the `/model`
setup flow. If no value is supplied, the SDK's default URL is used. A basic URL
format validation must be applied to the input.

---

## Current Architecture (relevant touch-points)

| Layer | File | What it does |
|---|---|---|
| Config types | `internal/config/config.go` | `ProviderConfig`, `GlobalConfig`, `ResolvedConfig`, `Resolve()` |
| Config persistence | `internal/config/loader.go` | JSON read/write to `~/.keen/configs.json` |
| LLM factory | `internal/llm/models.go` | `ClientConfig`, `NewClient()` |
| Anthropic client | `internal/llm/anthropic.go` | reads `ANTHROPIC_BASE_URL` from `.env` today |
| OpenAI Responses | `internal/llm/openai_responses.go` | `NewOpenAIResponsesClient` — no baseURL today |
| OpenAI-compat | `internal/llm/openai.go` | `openAICompatibleBaseURL()` hard-codes provider URLs |
| Genkit (Google) | `internal/llm/genkit.go` | `NewGenkitClient` — no baseURL today |
| Model selection UI | `internal/cli/repl/widgets/model_selection.go` | multi-step wizard: Provider → Model → Thinking → APIKey |
| REPL wiring | `internal/cli/repl/repl.go` | calls `replwidgets.New(...)` and `onComplete` callback |

---

## Implementation Plan

### 1. Config layer — store `baseUrl` per provider

**File:** `internal/config/config.go`

- [ ] Add `BaseURL string` field to `ProviderConfig` (JSON tag `"base_url,omitempty"`).
- [ ] Add `BaseURL string` field to `ResolvedConfig`.
- [ ] In `Resolve()`, propagate `ProviderConfig.BaseURL` into `ResolvedConfig.BaseURL`
      (no session-level override needed for now — it is always persisted globally).

---

### 2. LLM layer — thread `BaseURL` through `ClientConfig` and all clients

**File:** `internal/llm/models.go`

- [ ] Add `BaseURL string` to `ClientConfig`.
- [ ] Pass `cfg.BaseURL` when constructing each `ClientConfig` inside `NewClient()`.

**File:** `internal/llm/anthropic.go`

- [ ] Remove the `.env`-based `ANTHROPIC_BASE_URL` lookup.
- [ ] Read `cfg.BaseURL` from `ClientConfig`; apply `option.WithBaseURL` only when non-empty.

**File:** `internal/llm/openai_responses.go`

- [ ] Accept `cfg.BaseURL` in `NewOpenAIResponsesClient`; apply `option.WithBaseURL`
      only when non-empty.

**File:** `internal/llm/openai.go`

- [ ] Accept `cfg.BaseURL` in `NewOpenAICompatibleClient`.
- [ ] When `cfg.BaseURL` is non-empty, use it instead of the value returned by
      `openAICompatibleBaseURL()`.
- [ ] Keep `openAICompatibleBaseURL()` as the fallback for providers that have
      a well-known default URL.

**File:** `internal/llm/genkit.go`

- [ ] Accept `cfg.BaseURL` in `NewGenkitClient`.
- [ ] Pass it through to the `compat_oai.OpenAICompatible` / `googlegenai.GoogleAI`
      plugin config when non-empty (check each plugin's struct for a `BaseURL` field).

---

### 3. UI layer — add `StepBaseURL` to the model selection wizard

**File:** `internal/cli/repl/widgets/model_selection.go`

- [ ] Add `StepBaseURL Step` constant (insert after `StepProvider`, before `StepModel`
      — or after `StepAPIKey`, before completion — whichever feels most natural in UX;
      **proposed order: Provider → Model → Thinking → BaseURL → APIKey**).
- [ ] Add fields to `Model`:
  - `BaseURLInput string`
  - `BaseURLError string`
- [ ] Update `handleKeyMsg` for `StepBaseURL`:
  - `backspace` / printable text → edit `BaseURLInput`.
  - `enter` → validate; if valid (or empty) advance to `StepAPIKey`; else set
    `BaseURLError`.
  - `esc` → emit `modelSelectionCancelMsg`.
- [ ] Add `handlePasteMsg` support for `StepBaseURL` (same pattern as `StepAPIKey`).
- [ ] Add URL validation helper `isValidBaseURL(s string) bool`:
  - Accept empty string (optional field).
  - Use `net/url.Parse` + check `scheme` is `http` or `https` and `Host` is non-empty.
- [ ] Add `renderBaseURLInput()` view method (mirroring `renderAPIKeyInput` style,
      but **no masking** — base URLs are not secrets).
  - Show existing saved value as a hint: `(press Enter to keep: <url>)`.
  - Show `BaseURLError` in red when non-empty.
- [ ] Update `ViewString()` to handle `StepBaseURL`.
- [ ] Update `complete()` to:
  - Read `BaseURLInput`; fall back to existing `ProviderConfig.BaseURL` when empty.
  - Store in `ProviderConfig.BaseURL` and `m.resolvedCfg.BaseURL`.
- [ ] Update `New(...)` / `onComplete` signature: the callback currently receives
      `(provider, model, apiKey string)` — extend to also pass `baseURL string`, **or**
      simply rely on `resolvedCfg` which is already mutated in place before `onComplete`
      is called. The latter requires no signature change.

**File:** `internal/cli/repl/repl.go`

- [ ] No signature change needed if `resolvedCfg` mutation approach is used.
- [ ] Verify `updateLLMClient()` re-reads `m.ctx.cfg` (which is `resolvedCfg`) — it
      already does via `llm.NewClient(m.ctx.cfg)`.
- [ ] Optionally show `baseUrl` in `buildInitialScreen` info block when non-empty.

---

### 4. Validation helper (shared or inline)

- [ ] Implement `isValidBaseURL` in `model_selection.go` (keeping it local; no need
      for a separate package given it is UI-only validation).
- [ ] Rules:
  - Empty → valid (field is optional).
  - Must parse without error via `url.Parse`.
  - Scheme must be `"http"` or `"https"`.
  - Host must be non-empty.

---

### 5. Tests

**File:** `internal/config/config_test.go`

- [ ] Add test: `Resolve` propagates `ProviderConfig.BaseURL` into `ResolvedConfig`.
- [ ] Add test: `Resolve` leaves `ResolvedConfig.BaseURL` empty when `ProviderConfig`
      has no `BaseURL`.

**File:** `internal/llm/anthropic_test.go`

- [ ] Add test: `NewAnthropicClient` uses custom `BaseURL` when provided.
- [ ] Add test: `NewAnthropicClient` does not set base URL when `cfg.BaseURL` is empty.

**File:** `internal/llm/openai_responses_test.go` / `internal/llm/openai_test.go`

- [ ] Same coverage pattern as Anthropic tests above.

**File:** (new or existing widget test)

- [ ] Add test for `isValidBaseURL`:
  - `""` → valid.
  - `"https://api.example.com"` → valid.
  - `"http://localhost:8080"` → valid.
  - `"ftp://bad.com"` → invalid.
  - `"not-a-url"` → invalid.
  - `"https://"` (missing host) → invalid.
- [ ] Add test: `StepBaseURL` advances to `StepAPIKey` on Enter with valid URL.
- [ ] Add test: `StepBaseURL` stays on step and sets error on Enter with invalid URL.
- [ ] Add test: `complete()` persists `BaseURLInput` into `ProviderConfig.BaseURL`.

---

### 6. Housekeeping

- [ ] Run `go test ./...` — all tests pass.
- [ ] Run `go mod tidy`.
- [ ] Update `CHANGELOG.md` `[Unreleased]` section with the new feature entry.

---

## File Change Summary

| File | Change type |
|---|---|
| `internal/config/config.go` | Edit — add `BaseURL` to `ProviderConfig` and `ResolvedConfig`; update `Resolve()` |
| `internal/config/config_test.go` | Edit — new tests |
| `internal/llm/models.go` | Edit — add `BaseURL` to `ClientConfig`; pass through in `NewClient()` |
| `internal/llm/anthropic.go` | Edit — remove `.env` lookup; use `cfg.BaseURL` |
| `internal/llm/anthropic_test.go` | Edit — new tests |
| `internal/llm/openai_responses.go` | Edit — use `cfg.BaseURL` |
| `internal/llm/openai_responses_test.go` | Edit — new tests |
| `internal/llm/openai.go` | Edit — use `cfg.BaseURL` with fallback |
| `internal/llm/openai_test.go` | Edit — new tests |
| `internal/llm/genkit.go` | Edit — use `cfg.BaseURL` where plugin supports it |
| `internal/cli/repl/widgets/model_selection.go` | Edit — new `StepBaseURL`, validation, render |
| `internal/cli/repl/repl.go` | Edit (minor) — optionally display baseURL in initial screen |
| `CHANGELOG.md` | Edit — add unreleased entry |
