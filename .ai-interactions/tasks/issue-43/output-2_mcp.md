# MCP Package Implementation Plan

Keen should first implement a standalone MCP layer before integrating MCP tools with generated skills or the LLM tool registry.

The first iteration supports MCP servers over Streamable HTTP and stdio using the official Go SDK. It does not support generated skills or prompt-time tool disclosure yet.

## Goals

- Load MCP server configuration from `~/.keen/mcp/configs.json`.
- Connect to configured Streamable HTTP and stdio MCP servers.
- Support Streamable HTTP with three auth modes:
  - no auth,
  - custom API key HTTP auth,
  - MCP-standard OAuth.
- Initialize MCP sessions.
- Fetch `tools/list` and log discovered tools with `slog.Debug`.
- Call MCP tools by server and tool name.
- Track runtime connection/auth state per server.

## Non-Goals

- Deprecated HTTP+SSE transport.
- Generated MCP skills.
- `call_mcp_tool` native LLM tool registration.
- Resources, prompts, sampling, elicitation, completion, and tasks.
- Dynamic refresh from `notifications/tools/list_changed`.

## Protocol Basis

Use the latest stable MCP protocol version: `2025-11-25`.

Use the official Go SDK:

```text
github.com/modelcontextprotocol/go-sdk
```

Keen should rely on the SDK for MCP protocol handling, Streamable HTTP transport, stdio transport, lifecycle/session management, tool listing, tool calls, and OAuth helpers where possible.

Keen-specific code should handle config loading, auth-mode wiring, server state, logging, and the public package API.

## Config

Read MCP config from:

```text
~/.keen/mcp/configs.json
```

Initial config shape:

```json
{
  "version": 1,
  "servers": {
    "deepwiki": {
      "url": "https://mcp.deepwiki.com/mcp",
      "transport": "streamable_http",
      "auth": {
        "type": "none"
      }
    },
    "internal": {
      "url": "https://mcp.example.com/mcp",
      "transport": "streamable_http",
      "auth": {
        "type": "api_key",
        "header": "Authorization",
        "scheme": "Bearer",
        "key": "..."
      }
    },
    "github": {
      "url": "https://github.example.com/mcp",
      "transport": "streamable_http",
      "auth": {
        "type": "oauth",
        "scopes": ["repo", "issues"]
      }
    },
    "local-files": {
      "transport": "stdio",
      "command": "local-files-mcp",
      "args": ["--workspace", "/Users/me/project"],
      "env": {
        "LOG_LEVEL": "info"
      }
    }
  }
}
```

Validation rules:

- `version` must be supported.
- Server names must be stable identifiers suitable for `call_mcp_tool`.
- `transport` must be `streamable_http` or `stdio`.
- Streamable HTTP servers must include `url`.
- HTTP `url` must be a valid HTTP or HTTPS URL.
- Streamable HTTP `auth.type` must be `none`, `api_key`, or `oauth`.
- API key auth must include `key`.
- OAuth may include configured scopes.
- stdio servers must include `command`.
- stdio servers may include `args` and `env`.
- stdio servers do not use HTTP auth config.

If Keen creates or writes this config file later, it should use `0600` permissions because API keys may be stored directly in the file.

## Transports

### Streamable HTTP

Streamable HTTP is used for remote MCP servers.

Keen should:

- POST JSON-RPC messages to the configured MCP endpoint,
- send `Accept: application/json, text/event-stream`,
- support JSON and SSE responses from POST,
- store and resend `MCP-Session-Id` when provided,
- send `MCP-Protocol-Version: 2025-11-25` after initialization,
- use configured auth mode for each request.

### stdio

stdio is used for local MCP servers launched as child processes.

Keen should:

- start the configured command with optional args and env,
- communicate with the server over stdin/stdout using MCP JSON-RPC framing through the SDK,
- capture stderr for debug logs and connection diagnostics,
- terminate the child process on manager shutdown,
- mark the server disconnected if the process exits unexpectedly.

stdio servers do not use OAuth or API key auth. Any credentials needed by a stdio server should be provided through its command args, environment, or that server's own config.

## HTTP Auth Modes

MCP does not define a standard client config file schema. `~/.keen/mcp/configs.json` is Keen's local configuration format. It should map cleanly to MCP-standard transports and authorization behavior.

MCP authorization is optional. For HTTP transports, MCP-standard authorization means OAuth as defined by the MCP HTTP authorization spec. API key auth is supported as Keen-configured custom HTTP authentication, not as the MCP OAuth flow.

stdio servers do not use MCP HTTP auth. Any credentials needed by a stdio server should be provided through environment variables, command args, or that server's own config.

### None

No auth headers are added.

If the server responds with an auth challenge, mark the server disconnected or auth-required with a clear error.

### API Key

API key auth is custom HTTP auth, not the MCP OAuth flow.

Keen injects the configured key into each HTTP request:

- default `header`: `Authorization`
- default `scheme`: `Bearer`
- if `scheme` is empty, send the raw key value

Examples:

```text
Authorization: Bearer <key>
X-API-Key: <key>
```

Logs must never include the API key or full auth header value.

### OAuth

OAuth should follow the MCP `2025-11-25` HTTP authorization spec.

Keen should use the SDK OAuth support, including:

- OAuth Protected Resource Metadata discovery,
- Authorization Server Metadata and OIDC Discovery support,
- Authorization Code + PKCE,
- resource parameter handling,
- bearer token injection,
- token refresh,
- step-up authorization for insufficient scope when supported.

OAuth token and registration persistence should be separate from generated skills or MCP metadata. The exact token store can be introduced behind an interface so it can move from files to keychain later.

## Package Shape

Add an `internal/mcp` package.

Suggested public API:

```go
type Manager struct {
    // implementation fields
}

func NewManager(configPath string, opts ...Option) (*Manager, error)
func (m *Manager) Start(ctx context.Context) error
func (m *Manager) Close() error

func (m *Manager) Status(server string) ServerStatus
func (m *Manager) Servers() []ServerStatus
func (m *Manager) ListTools(ctx context.Context, server string) ([]Tool, error)
func (m *Manager) CallTool(ctx context.Context, server, tool string, arguments map[string]any) (*ToolResult, error)
```

The package should stay headless:

- no REPL code,
- no skill generation,
- no LLM tool registration,
- no UI prompts except through injected OAuth/browser hooks.

## Runtime State

Track state per configured server:

- `configured`
- `connecting`
- `connected`
- `disconnected`
- `auth_required`
- `auth_failed`

Keep runtime state separate from config.

Each server should retain:

- server name,
- endpoint URL,
- auth type,
- current status,
- last successful connection time,
- last successful tool refresh time,
- last error,
- discovered tools,
- active SDK session when connected.

## Startup Flow

```text
Keen starts
→ internal/mcp loads ~/.keen/mcp/configs.json
→ Manager starts a background discovery goroutine
→ Discovery starts one goroutine per configured server
→ Each server goroutine creates the configured transport
→ For Streamable HTTP, auth provider is attached based on config
→ For stdio, configured command is started
→ SDK client connects and initializes
→ Manager fetches tools/list, following pagination
→ Manager logs discovered tools with slog.Debug
→ Server state becomes connected
```

If a server fails to connect, keep it configured but mark it disconnected, auth-required, or auth-failed depending on the failure.

## Tool Listing

On successful connection, fetch all tools through `tools/list`, including pagination.

Store the discovered tools in memory per server.

Debug logging should be compact and redacted:

- server name,
- tool name,
- optional truncated description,
- optional input schema hash or top-level property names.

Avoid dumping full schemas by default because schemas can be large.

## Tool Calls

`CallTool` should:

- require both server name and tool name,
- fail if the server is not configured,
- fail if the server is not connected,
- fail if the tool is not in the latest discovered tool list,
- validate arguments against the cached input schema where practical,
- call the MCP server through the SDK,
- preserve normal tool results and MCP tool error results.

Normalize errors into Keen-friendly categories:

- server not configured,
- disconnected,
- auth required,
- auth failed,
- tool not found,
- invalid arguments,
- timeout,
- remote protocol error,
- remote tool error.

## Timeouts and Cancellation

All network operations must accept `context.Context`.

Recommended defaults:

- connect/initialize timeout,
- `tools/list` timeout,
- `tools/call` timeout.

Tool call timeout may need to be longer than startup discovery timeout.

Manager shutdown should cancel in-flight server discovery and close active sessions.

## Notifications

For v1, Keen does not need dynamic metadata refresh.

It should still tolerate and log server notifications from the SDK where available.

If `notifications/tools/list_changed` is received, log it. A later iteration can use that notification to refresh tool metadata.

## Testing

Add focused tests for `internal/mcp` using fake HTTP servers:

- config parsing and validation,
- no-auth connection/list/call path,
- API key header injection,
- API key redaction in logs/errors,
- OAuth auth wiring at unit boundaries,
- stdio command launch and shutdown,
- stdio process exit state,
- connection failure state,
- auth failure state,
- `tools/list` pagination,
- unknown server/tool errors,
- disconnected server call errors,
- remote tool error result handling,
- timeout/cancellation behavior.

Tests should cover critical paths rather than aiming for exhaustive protocol coverage.

## End-to-End Server Configs

Use these real MCP servers for manual end-to-end validation once the manager can connect, list tools, and call tools.

### No Auth: DeepWiki

DeepWiki provides a public no-auth MCP server over Streamable HTTP.

```json
{
  "deepwiki": {
    "url": "https://mcp.deepwiki.com/mcp",
    "transport": "streamable_http",
    "auth": {
      "type": "none"
    }
  }
}
```

Expected validation:

- `initialize` succeeds without auth.
- `tools/list` returns DeepWiki tools such as `read_wiki_structure`, `read_wiki_contents`, and `ask_question`.
- `tools/call` can call a read-only public-repository query.

### OAuth: PostHog

PostHog provides a hosted MCP server over Streamable HTTP. Its normal path uses PostHog's authentication server, while API-key auth is documented as a fallback for clients that do not support OAuth.

```json
{
  "posthog-oauth": {
    "url": "https://mcp.posthog.com/mcp",
    "transport": "streamable_http",
    "auth": {
      "type": "oauth"
    }
  }
}
```

Expected validation:

- Initial connection receives an OAuth challenge.
- Keen opens the browser authorization flow.
- Token material is persisted outside `configs.json`.
- Retry after authorization succeeds.
- `tools/list` returns PostHog MCP tools.
- `tools/call` can call a low-risk read-only PostHog operation, such as documentation search or project/user metadata.

### API Key: Context7

Context7 provides a hosted Streamable HTTP MCP server that accepts API key authentication. This validates Keen's custom API key HTTP auth path with a server separate from the OAuth E2E target.

```json
{
  "context7-api-key": {
    "url": "https://mcp.context7.com/mcp",
    "transport": "streamable_http",
    "auth": {
      "type": "api_key",
      "header": "CONTEXT7_API_KEY",
      "scheme": "",
      "key": "YOUR_CONTEXT7_API_KEY"
    }
  }
}
```

Expected validation:

- Keen sends `CONTEXT7_API_KEY: <api-key>` on each request.
- `initialize` succeeds without browser OAuth.
- `tools/list` returns Context7 MCP tools such as `resolve-library-id` and `get-library-docs`.
- `tools/call` can call a low-risk read-only documentation operation, such as resolving `nextjs` or fetching docs for a known library ID.
- Logs and errors never include the API key.

Combined E2E config:

```json
{
  "version": 1,
  "servers": {
    "deepwiki": {
      "url": "https://mcp.deepwiki.com/mcp",
      "transport": "streamable_http",
      "auth": {
        "type": "none"
      }
    },
    "posthog-oauth": {
      "url": "https://mcp.posthog.com/mcp",
      "transport": "streamable_http",
      "auth": {
        "type": "oauth"
      }
    },
    "context7-api-key": {
      "url": "https://mcp.context7.com/mcp",
      "transport": "streamable_http",
      "auth": {
        "type": "api_key",
        "header": "CONTEXT7_API_KEY",
        "scheme": "",
        "key": "YOUR_CONTEXT7_API_KEY"
      }
    }
  }
}
```

## Later Integration

After this package works, later tasks can add:

- generated MCP skill/schema files,
- startup cleanup for removed configured servers,
- hiding/disabling generated skills for disconnected servers,
- native `call_mcp_tool` LLM tool,
- UI commands for MCP status and refresh,
- optional dynamic refresh from `notifications/tools/list_changed`.

## Granular Implementation Todo

1. Add the official MCP Go SDK dependency to `go.mod`.
2. Create the `internal/mcp` package directory.
3. Add config model types for `configs.json`: root config, server config, auth config, and transport/auth constants.
4. Add `DefaultConfigPath()` returning `~/.keen/mcp/configs.json`.
5. Implement config file loading with missing-file behavior that returns an empty config.
6. Implement JSON parsing for config files.
7. Implement config version validation.
8. Implement server name validation for stable public IDs.
9. Implement Streamable HTTP URL validation.
10. Implement stdio command validation.
11. Implement auth validation for `none`, `api_key`, and `oauth`.
12. Add config tests for missing file, invalid JSON, unsupported version, invalid server names, invalid URLs, missing API keys, and invalid stdio commands.
13. Define `ServerState` constants: `configured`, `connecting`, `connected`, `disconnected`, `auth_required`, and `auth_failed`.
14. Define `ServerStatus` with name, transport, auth type, state, timestamps, last error, and tool count.
15. Define MCP-facing `Tool` and `ToolResult` wrapper types used by Keen.
16. Define normalized error values or typed errors for not configured, disconnected, auth required, auth failed, tool not found, invalid arguments, timeout, protocol error, and remote tool error.
17. Add manager options for logger, HTTP client, token store, OAuth browser/callback hooks, and timeouts.
18. Implement `NewManager(configPath string, opts ...Option)`.
19. Store loaded server configs in the manager.
20. Initialize per-server runtime entries in `configured` state.
21. Add mutex-protected manager state for sessions, tools, status, and cancellation.
22. Implement `Servers() []ServerStatus`.
23. Implement `Status(server string) ServerStatus`.
24. Implement `Start(ctx context.Context) error`.
25. Make `Start` load config and create the top-level background discovery context.
26. Start one discovery goroutine per configured server.
27. Ensure repeated `Start` calls are either rejected or no-ops.
28. Implement per-server discovery timeout handling.
29. Implement `Close() error`.
30. Make `Close` cancel discovery, close active SDK sessions, stop stdio processes, and wait for goroutines.
31. Add tests for manager start/close lifecycle and state transitions.
32. Implement Streamable HTTP transport creation through the SDK.
33. Configure Streamable HTTP requests to follow MCP `2025-11-25` transport requirements through the SDK.
34. Implement no-auth HTTP mode.
35. Implement API key HTTP auth as a custom `RoundTripper`.
36. Add API key defaults for `Authorization` and `Bearer`.
37. Support raw API key header values when `scheme` is empty.
38. Ensure API key values are redacted from logs and errors.
39. Add tests that API key auth injects the expected header.
40. Add tests that API key values do not appear in captured logs or returned errors.
41. Implement OAuth transport wiring with the SDK OAuth handler.
42. Define a token store interface for OAuth token persistence.
43. Add a file-backed token store placeholder or in-memory token store for v1 tests.
44. Wire OAuth scopes from config into the SDK OAuth handler.
45. Wire OAuth browser-open and redirect/callback handling through injected hooks.
46. Map OAuth challenge failures to `auth_required` or `auth_failed`.
47. Add OAuth unit tests around handler wiring and state/error mapping.
48. Implement stdio transport creation through the SDK.
49. Pass configured stdio command args.
50. Merge configured stdio env with the process environment.
51. Capture stdio stderr for debug logs and diagnostics.
52. Track stdio process exit and mark the server `disconnected` when it exits unexpectedly.
53. Ensure stdio process cleanup happens during `Close`.
54. Add stdio tests using a fake test server process or SDK test harness.
55. Implement the per-server connection flow.
56. Set server state to `connecting` before attempting connection.
57. Create the configured transport.
58. Create the SDK client/session.
59. Run MCP initialization through the SDK.
60. Store negotiated server/session data needed by later calls.
61. Set server state to `connected` after successful initialization and tool fetch.
62. On connection failure, preserve server config and set a disconnected/auth state with last error.
63. Add tests for successful connection state.
64. Add tests for connection refused, invalid URL, auth failure, and stdio command-not-found state.
65. Implement `refreshTools(ctx, server)` for a connected session.
66. Call SDK `tools/list` after successful initialization.
67. Follow `tools/list` pagination until all tools are loaded.
68. Convert SDK tool definitions into Keen `Tool` wrappers.
69. Store discovered tools in the server runtime state.
70. Record last successful tool refresh time.
71. Log each discovered tool with `slog.Debug`.
72. Keep tool debug logs compact: server, tool name, truncated description, and optional schema summary/hash.
73. Avoid logging full schemas by default.
74. Add tests for paginated tool listing.
75. Add tests for debug logging of discovered tools.
76. Implement `ListTools(ctx, server string) ([]Tool, error)`.
77. Make `ListTools` fail for unknown servers.
78. Make `ListTools` fail for disconnected or auth-failed servers.
79. Make `ListTools` return a defensive copy of cached tools.
80. Add tests for `ListTools` success and failure cases.
81. Implement optional JSON Schema validation helper for tool arguments.
82. Decide whether unsupported schema features warn or fail closed.
83. Add tests for required fields, wrong types, and additional properties where supported.
84. Implement `CallTool(ctx, server, tool string, arguments map[string]any) (*ToolResult, error)`.
85. Validate that the server exists.
86. Validate that the server is currently connected.
87. Validate that the tool exists in the cached discovered tools.
88. Validate arguments against the cached input schema where practical.
89. Apply the configured tool-call timeout.
90. Call the SDK `tools/call` method.
91. Convert SDK tool call responses into Keen `ToolResult`.
92. Preserve successful content results.
93. Preserve MCP tool error results separately from transport/protocol errors.
94. Map SDK/protocol errors into Keen normalized errors.
95. Add tests for successful tool call.
96. Add tests for unknown server, disconnected server, unknown tool, invalid arguments, timeout, protocol error, and remote tool error.
97. Implement basic notification handling hooks exposed by the SDK.
98. Respond to or tolerate `ping` as supported by the SDK.
99. Log server notifications at debug level.
100. Log `notifications/tools/list_changed` without refreshing in v1.
101. Add tests for notification logging where practical.
102. Add package-level documentation describing v1 scope and non-goals.
103. Configure a real no-auth Streamable HTTP MCP server for end-to-end testing.
104. Configure a real OAuth Streamable HTTP MCP server for end-to-end testing.
105. Configure a real API-key Streamable HTTP MCP server for end-to-end testing.
106. Add an end-to-end `configs.json` example covering the no-auth, OAuth, and API-key servers.
107. Run the manager against the no-auth server and verify initialize, `tools/list`, and `tools/call`.
108. Run the manager against the API-key server and verify header auth, initialize, `tools/list`, and `tools/call`.
109. Run the manager against the OAuth server and verify browser auth, token persistence, initialize, `tools/list`, and `tools/call`.
110. Confirm end-to-end logs include discovered tools but never include API keys or OAuth tokens.
111. Add examples or test fixtures for no-auth HTTP, API key HTTP, OAuth HTTP, and stdio config.
112. Run `gofmt` on all modified Go files.
113. Run `go mod tidy`.
114. Run `go test ./...`.
115. Review logs and errors to confirm secrets are redacted.
116. Review public `internal/mcp` API to confirm it has no REPL, skills, or LLM tool coupling.
