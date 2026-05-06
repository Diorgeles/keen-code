# Support OpenCode Go Provider

## Summary

Add `opencode-go` as a first-class provider in Keen Code using OpenCode Go's documented endpoints:

- OpenAI-compatible chat completions for GLM, Kimi, DeepSeek, MiMo, and Qwen models.
- Anthropic-compatible messages for MiniMax M2.5 and M2.7.
- Default base URL: `https://opencode.ai/zen/go/v1/`.

Source: OpenCode Go docs, last checked May 6, 2026: https://opencode.ai/docs/go/

## Key Changes

- Add `ProviderOpenCodeGo = "opencode-go"` to config provider constants.
- Add OpenCode Go to `providers/registry.yaml` with the currently documented models:
  - `glm-5.1`, `glm-5`
  - `kimi-k2.6`, `kimi-k2.5`
  - `deepseek-v4-pro`, `deepseek-v4-flash`
  - `mimo-v2-pro`, `mimo-v2-omni`, `mimo-v2.5-pro`, `mimo-v2.5`
  - `minimax-m2.7`, `minimax-m2.5`
  - `qwen3.6-plus`, `qwen3.5-plus`
- Keep model IDs raw in Keen config, such as `kimi-k2.6`, under provider `opencode-go`. Do not store `opencode-go/<model-id>` because Keen already stores provider separately.

## Implementation Details

- Route `opencode-go` models in `NewClient` by model family:
  - `minimax-m2.*` -> `NewAnthropicClient`
  - all other OpenCode Go models -> `NewOpenAICompatibleClient`
- Add OpenCode Go default base URL handling:
  - OpenAI-compatible client uses `https://opencode.ai/zen/go/v1/`, letting the SDK append `/chat/completions`.
  - Anthropic client uses the same base URL and relies on the SDK's Messages path behavior for `/messages`.
- Add a small helper, for example `isOpenCodeGoAnthropicModel(model string) bool`, so protocol routing is explicit and testable.
- Do not introduce a new LLM client type unless SDK base URL behavior makes it unavoidable.

## Thinking Behavior

- DeepSeek OpenCode Go models:
  - `off` sends `thinking: {"type":"disabled"}`.
  - `high` and `max` send `thinking: {"type":"enabled"}` plus `reasoning_effort`.
- GLM and Kimi OpenCode Go models:
  - support `enabled` / `disabled` via `thinking: {"type": effort}`.
- Qwen OpenCode Go models:
  - support `enabled` / `disabled` using `enable_thinking: true|false`.
- MiMo models:
  - expose `thinking_efforts` only if verified from `/models`; otherwise omit thinking controls and still stream any `reasoning_content` returned by the provider.
- MiniMax models:
  - do not send Keen thinking controls initially.
  - preserve and replay returned Anthropic thinking blocks through the existing Anthropic client path.

## Tests

- Config/client routing:
  - `opencode-go` non-MiniMax model creates an `OpenAICompatibleClient`.
  - `opencode-go` MiniMax model creates an `AnthropicClient`.
  - missing API key is rejected like other API-key providers.
- Base URL:
  - OpenAI-compatible OpenCode Go client defaults to `https://opencode.ai/zen/go/v1/`.
  - custom `base_url` overrides the default.
- Thinking params:
  - DeepSeek OpenCode Go maps `off`, `high`, and `max` correctly.
  - GLM/Kimi OpenCode Go maps `enabled` and `disabled` correctly.
  - Qwen OpenCode Go maps `enabled`/`disabled` to `enable_thinking`.
- Registry:
  - provider `opencode-go` loads from `providers/registry.yaml`.
  - listed models have expected IDs, names, context windows, and thinking efforts where supported.
- Full verification:
  - run `gofmt` on modified Go files.
  - run `go mod tidy`.
  - run `go test ./...`.

## Assumptions

- The default OpenCode Go API key is a normal bearer API key and should use Keen's existing API-key auth path.
- The model list should be static in `providers/registry.yaml` for this change; dynamic loading from `/models` is out of scope.
- OpenCode Go remains beta, so provider-specific thinking controls should be conservative and covered by unit tests.
