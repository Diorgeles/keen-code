# Provider-Backed Context Status Redesign

As of 2026-04-21.

## Goal

Replace the current local word-count heuristic used by `context status` with provider-backed token data.

The first-pass redesign should make the REPL status bar report context usage from:

- provider-reported request usage when a model call has happened

The main UI metric remains:

`context in use = input_tokens / model_context_window`

The difference is that `input_tokens` should come from the provider, not from local text estimation.

This version does not require provider token-counting APIs for idle-state prediction.

## Current State

Today, `context status` is computed locally in:

- `internal/cli/repl/context_status.go`

It approximates:

- system prompt text
- persisted conversation messages
- partial streamed assistant text
- registered tool schemas

Then it converts words to tokens with a rough `words * 4 / 3` heuristic.

This has several problems:

- it does not match provider tokenization
- it does not reflect provider-side context handling exactly
- it can drift from reality when tools, reasoning, system formatting, or multimodal/provider-specific inputs matter
- it is not authoritative enough for compaction suggestions

## Design Principles

1. Provider data is the source of truth when available.
2. The status bar should represent per-request context usage, not cumulative turn cost.
3. The first pass should optimize for simplicity: track the latest actual model request, not a separately counted future request.
4. If a provider does not expose usage, the UI should show `0.0%` rather than a heuristic or `N/A`.

## Key Product Decision

The redesign should use one authoritative metric:

- latest actual request input tokens reported by the provider

If a turn contains multiple internal model calls:

- first sampling call
- one or more post-tool follow-up calls

the status should display the latest request's input-token count, because the context window applies to that specific request.

Do not sum input tokens across loop iterations for the meter. That would describe spend, not context occupancy.

When the REPL is idle, keep showing the latest authoritative request usage. If no provider-backed usage has been observed yet, show `0.0%`.

## Provider Capability Model

The plan should treat providers in two buckets.

### 1. Providers with trustworthy request usage

- OpenAI Responses
- Anthropic Messages
- Google AI via Genkit when the underlying plugin exposes usage

These can support:

- authoritative request usage for the latest model call

### 2. Providers with no trustworthy request usage in the current integration

- OpenAI-compatible chat providers when the SDK/provider does not expose prompt usage reliably in streaming or final responses

These should render `0.0%` until or unless trustworthy usage support is added.

## Architecture Changes

## Step 1: Replace heuristic status state with provider-backed status state

**Files:**

- `internal/cli/repl/context_status.go`
- `internal/cli/repl/repl.go`

Redefine `contextStatus` so it models provider-backed measurements rather than estimated text size.

Suggested shape:

- `CurrentTokens int`
- `ContextWindow int`
- `Percent float64`
- `KnownWindow bool`
- `KnownTokens bool`
- `Source string` or enum:
  - `provider_usage`
  - `unknown`
- `Pending bool`

`computeContextStatus()` should stop rebuilding prompt text entirely.

Instead, it should:

- read the current model context window from `providers.Registry`
- read the latest provider-backed usage measurement from app/session state
- compute percent from that usage when both tokens and context window are known
- otherwise render `0.0%`

This means the following helpers become removable from the main path:

- `estimateTokensFromWordCount`
- `countWords`
- `estimateToolDefinitionTokens`
- `buildConversationForEstimation`

They can be removed outright once the migration is complete.

## Step 2: Introduce provider token accounting types

**Files:**

- `internal/llm/message.go`
- `internal/llm/client.go`

Add a provider-neutral token usage model.

Suggested new types:

```go
type TokenUsage struct {
    InputTokens     int
    OutputTokens    int
    TotalTokens     int
    ReasoningTokens int
    CachedTokens    int
}
```

Add a new stream event type for usage emitted from actual model calls:

- `StreamEventTypeUsage`

and extend `StreamEvent` with:

- `Usage *TokenUsage`

`LLMClient` does not need a count-tokens method in the first pass.

## Step 3: Persist provider-backed context metrics in app state

**Files:**

- `internal/cli/repl/appstate/state.go`

Add state to hold the latest authoritative context measurements.

Suggested fields:

- `lastUsage *llm.TokenUsage`

Suggested semantics:

- `lastUsage` = latest actual model request usage emitted during the current or most recent completed turn

Expose methods like:

- `SetLastUsage(*llm.TokenUsage)`
- `GetLastUsage() *llm.TokenUsage`
- `ClearContextMetrics()`

## Step 4: Teach providers to emit request usage during streaming

### 4a. OpenAI Responses

**File:**

- `internal/llm/openai_responses.go`

After each `response.completed`, emit a usage event using the response's `usage` object.

That usage should correspond to the actual request just completed in the current loop iteration.

This is the best source for active-turn status on the OpenAI path.

### 4b. Anthropic

**File:**

- `internal/llm/anthropic.go`

Anthropic streaming exposes cumulative token usage on `message_delta`, and the final `Message` object also includes usage.

Emit a usage event from the latest authoritative value during the stream, and/or from the final accumulated message if that is simpler and more stable.

Use:

- `input_tokens` for context occupancy
- `output_tokens` as supporting telemetry only

### 4c. Genkit

**File:**

- `internal/llm/genkit.go`

When the final `ModelResponse` is available, read `GenerateResponse.usage` and emit a usage event if `inputTokens` is populated.

Do not assume all Genkit providers fill every field. If missing, emit nothing and leave the status unknown for that request.

### 4d. OpenAI-compatible Chat Providers

**File:**

- `internal/llm/openai.go`

Treat this as a second-phase integration.

The compatibility layer should only emit usage if the SDK/provider path can be made reliable for:

- prompt/input token usage
- streaming or final-message access

If not, leave usage unsupported and report `0.0%` for those providers.

Do not guess.

## Step 5: Refresh lifecycle in the REPL

**Files:**

- `internal/cli/repl/repl.go`
- `internal/cli/repl/handlers.go`

### On REPL startup / session resume / model change

Do not issue any background count-tokens request in the first pass.

Instead:

- retain the latest observed `lastUsage` if it is still relevant to the current in-memory state
- clear it when session state is reset or replaced
- render `0.0%` if there is no provider-backed usage yet

### On user send

Do not block the assistant request on any preflight counting step.

Keep the current displayed value until the first usage event for the new request arrives, then replace it with the provider-reported usage.

### During active turn

When a provider usage event arrives:

- store it in `lastUsage`
- refresh `contextStatus`

If the tool loop makes another model call later in the turn, the newer usage should overwrite the older one.

### On turn completion

After the assistant message is appended to conversation state, keep the latest `lastUsage` as the displayed idle value until a later request overwrites it.

## Step 6: Define status selection rules

**Files:**

- `internal/cli/repl/context_status.go`

The status bar should resolve its displayed token count in this order:

1. `lastUsage.InputTokens` when available
2. `0` when provider-backed data is unavailable

Always compute the displayed percent from that selected token count and the model context window.

Do not mix provider-backed counts with the old heuristic in the primary display path.

If the context window is unknown, render `0.0%` in the first pass rather than `N/A`.

## Step 7: Compaction suggestion should use authoritative data when available

**Files:**

- `internal/cli/repl/context_status.go`
- `internal/cli/repl/repl.go`

`ShouldSuggestCompaction()` should continue using the same threshold logic, but the percentage should now come from provider-backed usage when available.

This is one of the main benefits of the redesign:

- compaction prompts become grounded in real tokenization rather than local heuristics

When usage is unavailable and the UI falls back to `0.0%`, compaction should not trigger.

## Step 8: Session persistence policy

**Files:**

- `internal/session/*` only if persistence is desired

This should be a deliberate choice.

Recommended first pass:

- do not persist `lastUsage` into saved sessions
- after replay/load, leave status at `0.0%` until the next provider-backed request usage is observed

Reason:

- usage is model-specific
- usage becomes stale when provider/model changes
- keeping it transient avoids session format churn

## Step 9: Tests

### Unit tests

**Files:**

- `internal/cli/repl/context_status_test.go`
- `internal/cli/repl/handlers_test.go`
- `internal/cli/repl/appstate/state_test.go`
- `internal/llm/openai_responses_test.go`
- `internal/llm/anthropic_test.go`
- `internal/llm/genkit_test.go`

Add tests for:

- active-turn refresh from `usage` events
- overwrite behavior across multiple tool-loop request iterations
- unsupported providers showing `0.0%`
- startup/resume/model-change behavior when no usage has been observed yet
- compaction threshold behavior with authoritative values

### Migration tests

Ensure no test still depends on:

- word-count token estimation
- partial streamed assistant text being counted heuristically

## Rollout Strategy

## Phase A: Core plumbing

- add token usage/count types
- extend `LLMClient` stream events for usage
- store provider-backed metrics in app state
- switch `contextStatus` to read those values
- render `0.0%` when missing

At the end of this phase, the heuristic is no longer the primary source.

## Phase B: OpenAI + Anthropic

- emit usage events from real responses

This covers the strongest provider-backed paths first.

## Phase C: Google AI / Genkit

- add usage propagation where available

## Phase D: Compatibility providers

- validate whether DeepSeek / Moonshot can expose reliable prompt usage
- support them only if the data is trustworthy

Otherwise keep `0.0%`.

## Future Extension

If accurate idle-state status becomes important later, add a separate `CountTokens()` path as a follow-up phase. That should be treated as an enhancement, not as a prerequisite for replacing the heuristic.

## Risks

### Provider mismatch

Genkit or compatibility providers may not expose complete usage data.

Mitigation:

- treat unsupported as `0.0%`
- do not reintroduce silent heuristic fallback in the main display path

### Semantic confusion

“Latest request input tokens” and “current idle conversation occupancy” are not identical.

Mitigation:

- document that the first pass shows the latest observed authoritative request size
- add idle-state counting later if product requirements demand it

## Recommended Outcome

The end state should be:

- no local word-count-based context status
- authoritative provider-backed usage whenever available
- `0.0%` when provider-backed usage is unavailable
- no idle-state counting requirement in the first pass

That is a cleaner product contract than the current heuristic:

`context status reports provider-counted request occupancy against the selected model's context window`
