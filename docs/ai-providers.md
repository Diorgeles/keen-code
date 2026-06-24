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
| Amazon Bedrock | `amazon-bedrock` | AWS credentials | Claude Opus 4.8, Opus 4.6, Sonnet 4.6, Haiku 4.5 |
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
  "adversary_provider": "anthropic",
  "adversary_model": "claude-sonnet-4-6",
  "providers": {
    "opencode-go": {
      "models": ["kimi-k2.6"],
      "api_key": "oc_..."
    }
  }
}
```

`adversary_provider` and `adversary_model` are set via `/adversary model` and control which model is used for adversarial reviews. They are independent of `active_provider`/`active_model` and can point to any configured provider.

### Custom Headers

You can attach custom HTTP headers to a provider by adding a `headers` object to that provider's config in `~/.keen/configs.json`. These headers are sent with every request to that provider.

```json
{
  "active_provider": "deepseek",
  "active_model": "deepseek-v4-pro",
  "providers": {
    "deepseek": {
      "models": ["deepseek-v4-pro"],
      "api_key": "sk-...",
      "headers": {
        "x_header_1": "val1",
        "x_header_2": "val2"
      }
    }
  }
}
```

Notes:

- Header names and values are plain strings.
- They are set per-provider; different providers can have different headers.
- Custom headers must be added by editing the config file directly. The `/model` UI does not provide a field for them.
- Applied to all clients: Anthropic, OpenAI, Codex, DeepSeek, Moonshot AI, Z.ai, MiniMax, OpenCode Go, Google AI (Genkit), and Amazon Bedrock.

### Config Resolution

Keen loads `~/.keen/configs.json` through `internal/config.Loader`, then builds the runtime `ResolvedConfig` in `internal/cli/cmd/root.go`.

Resolution order for the default interactive and headless paths:
1. Provider: `global.active_provider`
2. Model: `global.active_model`
3. API key: `providers.{provider}.api_key_helper` → `providers.{provider}.api_key`

For `keen run --provider`, the selected provider's config replaces the active provider for that invocation. If `--model` is omitted, Keen uses the selected provider's first configured model. The API key is still resolved through `api_key_helper` first, then `api_key`.

## Authentication

### API Key Authentication

Most providers use API key authentication. Keys are stored in the global config under `providers.{provider}.api_key`.

Instead of storing a key, a provider can define `api_key_helper`. Keen executes the helper locally when resolving the provider config, trims stdout, and uses that value as the in-memory API key for the current run/session. When `api_key_helper` is set, it always wins over `api_key`; `api_key` can be empty and Keen does not write the helper output back to `~/.keen/configs.json`.

```json
{
  "active_provider": "anthropic",
  "active_model": "claude-sonnet-4-6",
  "providers": {
    "anthropic": {
      "models": ["claude-sonnet-4-6"],
      "api_key": "",
      "api_key_helper": "example-auth token || (example-auth login > /dev/null 2>&1 && example-auth token)"
    }
  }
}
```

> **Security note:** `api_key_helper` is executed as a shell command with the privileges of the running process. Treat it as executable code: never paste untrusted strings into this field, audit any helper script before use, and keep `~/.keen` permissions strict (e.g. `chmod 700 ~/.keen` and `chmod 600 ~/.keen/configs.json`) so other local users cannot inject or read its contents.

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

### AWS Authentication (Amazon Bedrock)

Amazon Bedrock uses AWS credential authentication via the AWS SDK:

```go
// internal/config/config.go
const AuthModeAWS = "aws"
```

Credentials are loaded from the standard AWS credential chain (`~/.aws/credentials`, environment variables, IAM roles, etc.). No API key is stored in Keen's global config. An optional custom `base_url` can be configured to override the Bedrock endpoint.

The default AWS region is `us-east-1` if none is configured in the environment.

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

### BedrockClient (`internal/llm/bedrock.go`)

AWS SDK integration for Amazon Bedrock:
- Streaming via `bedrockruntime.ConverseStream`
- Tool conversion to Bedrock tool format
- Reasoning content support (thinking text, signatures, redacted content)
- Prompt caching with cache points on system prompts, tools, and messages
- Cached token tracking

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
| Amazon Bedrock | low, medium, high, max |
| Z.ai | enabled, disabled |
| OpenCode Go DeepSeek | off, high, max |
| OpenCode Go GLM/Kimi/OpenAI-compatible Qwen | enabled, disabled |

The thinking effort is set via config and passed to the LLM client, which configures the provider's thinking parameters.

OpenCode Go thinking controls are model-family specific:
- DeepSeek sends `thinking.type` plus `reasoning_effort` for enabled efforts.
- GLM and Kimi send `thinking.type`.
- OpenAI-compatible Qwen sends `enable_thinking`.
- MiMo and MiniMax do not receive a Keen-sent thinking control; returned reasoning is still streamed when the provider exposes it.
