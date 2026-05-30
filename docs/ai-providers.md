# AI Providers

Keen Code supports multiple AI providers through a plugin-like architecture. The provider system handles model discovery, authentication, and communication with different LLM backends.

## Supported Providers

| Provider | ID | Authentication | Models |
|----------|-----|----------------|--------|
| Anthropic | `anthropic` | API Key | Claude Opus 4.7, Opus 4.6, Sonnet 4.6, Haiku 4.5 |
| OpenAI | `openai` | API Key | GPT-5.5, GPT-5.4, GPT-5.4-mini, GPT-5.3-codex |
| Codex | `openai-codex` | OAuth | GPT-5.5, GPT-5.4, GPT-5.4-mini, GPT-5.3-codex |
| Google AI | `googleai` | API Key | Gemini 3.1 Pro, 3.1 Flash-Lite, 3 Flash |
| Moonshot AI | `moonshotai` | API Key | Kimi K2.6, K2.5, K2 Thinking, K2 Thinking Turbo |
| DeepSeek | `deepseek` | API Key | DeepSeek V4 Flash, V4 Pro |
| Z.ai | `zai` | API Key | GLM-5.1, GLM-5, GLM-5 Turbo |
| MiniMax | `minimax` | API Key | MiniMax M2.7, M2.5 |
| OpenCode Go | `opencode-go` | API Key | GLM-5.1, GLM-5, Kimi K2.6, Kimi K2.5, DeepSeek V4 Pro, DeepSeek V4 Flash, MiMo-V2, MiniMax M2.7/M2.5, Qwen3 Plus/Max |

## Provider Registry

Provider and model metadata is stored in `providers/registry.yaml`. This includes:
- Context window sizes
- Supported thinking efforts
- Model display names

```go
// providers/loader.go
type Registry struct {
    Providers []Provider `yaml:"providers"`
}

type Provider struct {
    ID     string  `yaml:"id"`
    Name   string  `yaml:"name"`
    Models []Model `yaml:"models"`
}

type Model struct {
    ID              string   `yaml:"id"`
    Name            string   `yaml:"name"`
    ContextWindow   int      `yaml:"context_window"`
    ThinkingEfforts []string `yaml:"thinking_efforts"`
}
```

## Configuration

### Global Config (`~/.keen/configs.json`)

```json
{
  "active_provider": "opencode-go",
  "active_model": "kimi-k2.6",
  "thinking_effort": "enabled",
  "show_thinking": true,
  "providers": {
    "opencode-go": {
      "models": ["kimi-k2.6"],
      "api_key": "oc_..."
    }
  }
}
```

### Config Resolution

The `Resolve` function in `internal/config/config.go` determines the final configuration by merging global and session configs:

```go
func Resolve(global *GlobalConfig, session *SessionConfig) (*ResolvedConfig, error)
```

Resolution order:
1. Provider: session → global.active_provider → error if unset
2. API Key: session → provider global config → error if required
3. Model: session → global.active_model → provider's first configured model

## Authentication

### API Key Authentication

Most providers use API key authentication. Keys are stored in the global config under `providers.{provider}.api_key`.

MiniMax uses its Anthropic-compatible API. Users normally leave `base_url` unset. Keen uses `https://api.minimax.io/anthropic`, which the Anthropic SDK extends to `/v1/messages`.

OpenCode Go also uses API key authentication. Users normally leave `base_url` unset. Keen uses `https://opencode.ai/zen/go/v1/` for OpenAI-compatible models and `https://opencode.ai/zen/go` for MiniMax models through the Anthropic SDK, which appends `/v1/messages`.

### OAuth Authentication (OpenAI Codex)

OpenAI Codex uses OAuth with PKCE flow:

```go
// internal/auth/oauth.go
type OAuthManager struct {
    Store       *Store
    HTTPClient  *http.Client
    OpenBrowser BrowserOpener
}
```

Flow:
1. Generate PKCE verifier/challenge and state
2. Start local HTTP server on port 1455
3. Open browser to authorization URL
4. Receive callback, exchange code for tokens
5. Store refresh/access tokens

Token refresh is automatic when the access token expires.

## LLM Client Architecture

```go
// internal/llm/client.go
type LLMClient interface {
    StreamChat(ctx context.Context, messages []Message, toolRegistry *tools.Registry) (<-chan StreamEvent, error)
    Reset()
}
```

Three client implementations:

### AnthropicClient (`internal/llm/anthropic.go`)

Direct integration with Anthropic SDK:
- Streaming via `ssestream.Stream`
- Tool conversion to Anthropic tool format
- Thinking budget support (low/medium/high/max)
- Cached token tracking
- MiniMax models (`MiniMax-M2.7`, `MiniMax-M2.5`) through MiniMax's Anthropic-compatible `/messages` endpoint
- OpenCode Go MiniMax models (`minimax-m2.*`) and `qwen3.7-max` through the Anthropic-compatible `/messages` endpoint

### OpenAIResponsesClient (`internal/llm/openai_responses.go`)

OpenAI Responses API for:
- OpenAI (GPT models)

### OpenAICompatibleClient (`internal/llm/openai.go`)

OpenAI-compatible API for:
- DeepSeek
- Moonshot AI (Kimi)
- Z.ai (GLM)
- OpenCode Go GLM, Kimi, DeepSeek, MiMo, and OpenAI-compatible Qwen models

Handles provider-specific features like the `reasoning_content` extension and thinking controls for compatible providers.

### GenkitClient (`internal/llm/genkit.go`)

Firebase Genkit integration for Google AI (Gemini). Currently the only provider using Genkit.

## Stream Events

All clients emit a unified stream of events:

```go
type StreamEvent struct {
    Type       StreamEventType
    Content    string           // for Chunk, ReasoningChunk
    ToolCall   *ToolCall        // for ToolStart, ToolEnd
    Usage      *TokenUsage      // for Usage
    Error      error            // for Error, Retry
    Attempt    int              // for Retry
}
```

Event types:
- `StreamEventTypeChunk` - Text content delta
- `StreamEventTypeReasoningChunk` - Thinking/reasoning content
- `StreamEventTypeToolStart` - Tool execution begins
- `StreamEventTypeToolEnd` - Tool execution completes
- `StreamEventTypeUsage` - Token usage stats
- `StreamEventTypeDone` - Response complete
- `StreamEventTypeError` - Unrecoverable error
- `StreamEventTypeRetry` - Retrying after error
- `StreamEventTypeIncomplete` - Turn limit reached with pending state

## Thinking Efforts

Models support different thinking effort levels:

| Provider | Efforts |
|----------|---------|
| Anthropic | low, medium, high, max |
| OpenAI | none, low, medium, high, xhigh |
| Google AI | low, medium, high, minimal |
| DeepSeek | off, high, max |
| Z.ai | enabled, disabled |
| OpenCode Go DeepSeek | off, high, max |
| OpenCode Go GLM/Kimi/OpenAI-compatible Qwen | enabled, disabled |

The thinking effort is set via config and passed to the LLM client, which configures the provider's thinking parameters.

OpenCode Go thinking controls are model-family specific:
- DeepSeek sends `thinking.type` plus `reasoning_effort` for enabled efforts.
- GLM and Kimi send `thinking.type`.
- OpenAI-compatible Qwen sends `enable_thinking`.
- MiMo and MiniMax do not receive a Keen-sent thinking control; returned reasoning is still streamed when the provider exposes it.
