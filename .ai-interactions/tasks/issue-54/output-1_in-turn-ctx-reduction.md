# In-turn Context Reduction Plan

## Problem

During a single agent turn, provider loops keep appending tool calls and tool results to the context sent back to the model. If tool turns are long or tool outputs are large, the next model request can exceed the model context window and fail.

Affected provider loops include:

- `internal/llm/genkit.go`
- `internal/llm/openai.go`
- `internal/llm/anthropic.go`
- `internal/llm/openai_responses.go`
- `internal/llm/openai_codex.go`

## Goal

Before every model request, reduce the request context if it is estimated to exceed the usable model context budget.

Initial reduction strategy:

- Use the provider-reported input token count from the previous request when available.
- Estimate only the token delta between the previous request context and the next request context.
- Compute the next request size as:

```text
previous_provider_input_tokens + estimated_context_delta
```

- If the computed size exceeds the usable budget, replace oldest tool result contents with a short placeholder.
- Maintain a running token estimate while reducing.
- Stop as soon as the estimated size is within budget.
- Do not delete tool-call or tool-result messages/items, because that can break provider protocol validity.

## Placeholder

Use a short placeholder:

```text
Tool result removed to fit context.
```

## Budget

The reducer compares estimated request input tokens against:

```text
usable_input_budget = model_context_window - output_reserve - safety_margin
```

### Notes

- Because provider input usage counts the full previous request, it already includes system prompts, tool schemas, message formatting, and provider overhead.
- Therefore, when provider usage is available, do not subtract system prompts or tools again.
- System prompts and tool schemas only need to be estimated for the first request or any case where previous provider usage is unavailable.

### Safety margin

Use a small safety margin to account for:

- approximate delta token estimation;
- provider-specific message framing overhead;
- model alias/context-window ambiguity;
- tokenizer differences across text types.

Suggested first default:

```text
safety_margin = max(1024, model_context_window / 100)
```

### Output reserve

Reserve output room so the request does not consume the entire context window.

Use the same first default for all providers:

```text
output_reserve = 8192
```

Do not use large provider-specific max output limits as the reserve by default. For example, even if Anthropic is configured with a high max output limit, it is unlikely to produce 64k tokens in one response for normal tasks, and reserving that much would over-reduce context unnecessarily.

## Design

Create one new shared file:

```text
internal/llm/context_reducer.go
```

This file should contain:

- model context-window lookup;
- usable input budget calculation;
- conservative token estimation helpers;
- shared reduction result type;
- provider-specific context reducer functions.

Provider files should only make small call-site changes:

1. Keep the previous request context that was actually sent to the provider.
2. Keep the previous provider-reported input token count when available.
3. Before the next provider request, pass the previous context, previous input token count, and next full context to the reducer.
4. Send the reduced context returned by the reducer.
5. After a successful provider response with usage, update the previous context/token baseline from the exact context that was sent and the returned input token count.

## Reducer flow

For each provider-specific reducer:

```text
input:
  model
  previous request context
  previous provider input token count, if available
  next full request context

steps:
  1. Compute usable input budget for model.
  2. Estimate current next request size:
       if previous provider input tokens are available:
         current = previous_input_tokens + estimate_delta(previous_context, next_context)
       else:
         current = estimate_full_request(next_context)
  3. If current <= budget, return context unchanged.
  4. Iterate oldest tool results in next_context.
  5. For each tool result:
       old_tokens = estimate(tool_result_content)
       new_tokens = estimate(placeholder)
       replace tool result content with placeholder
       current = current - old_tokens + new_tokens
       stop if current <= budget
  6. Return reduced context and reduction metadata.
```

## Reduction metadata

Use a small result struct similar to:

```go
type contextReduction struct {
    OriginalTokens     int
    ReducedTokens      int
    RemovedToolResults int
    EstimatedDelta     int
    FitsBudget         bool
}
```

If `FitsBudget` is false after all tool results are replaced, the provider should avoid making the model request and return a graceful failure/incomplete result instead.

## Token estimation

Do not rely on `len(text) / 4` as the main estimate. It can undercount code, JSON, logs, Unicode, hashes, and stack traces.

For a first implementation, use a conservative byte-based estimate for deltas and replaced result contents, for example:

```go
func estimateContextTokens(text string) int {
    if text == "" {
        return 0
    }
    return max(1, (len(text)+2)/3)
}
```

Provider-reported input usage should be preferred whenever available because it is more accurate and includes provider-specific overhead.

## Delta estimation

Reducers should estimate only the difference between the previous context and the next context when a previous provider input token count is available.

Because provider loops generally append new assistant/tool context during a tool turn, the first implementation can use common-prefix detection:

```text
1. Find the common prefix between previous context and next context.
2. Estimate tokens for the suffix added to next context.
3. Add that estimate to previous provider input tokens.
```

If common-prefix detection is not reliable for a provider/context shape, fall back to estimating the full next request.

## Provider-specific replacement targets

Reducers must preserve provider protocol structure and replace only tool result content.

| Provider | Replace |
|---|---|
| OpenAI chat-compatible | tool role message content |
| Anthropic | `tool_result` block content |
| OpenAI Responses | `function_call_output.output` |
| OpenAI Codex | `function_call_output.output` |
| Genkit | tool response output/content |

## Baseline update

The reduced context automatically becomes the next baseline if it is the exact context sent to the provider.

Provider logic should follow this rule:

```text
Only update previous context/input-token baseline after a successful provider response with usage.
```

If the request fails before usage is returned, do not update the baseline.

## Minimal provider changes

Each provider loop should add:

- local state for previous sent context;
- local state for previous provider input tokens;
- one reducer call before model request;
- one baseline update after successful response usage.

The reducer can expose separate provider-specific functions to minimize changes in provider files and avoid introducing a large generic provider abstraction.

## Verification

Add focused tests for:

1. Context under budget remains unchanged.
2. Context over budget replaces oldest tool results first.
3. Reduction stops as soon as the budget is met.
4. Tool result messages/items remain structurally present.
5. Provider input-token baseline plus estimated delta is used when available.
6. Full request estimate is used when no provider input-token baseline exists.
7. Reducer reports `FitsBudget=false` when replacing all tool results is still insufficient.

After implementation, run:

```bash
go mod tidy
gofmt -w <modified-go-files>
go test -race ./...
```
