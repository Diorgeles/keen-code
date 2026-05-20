# MCP Phase 2: Startup Status and User-Initiated OAuth

In this phase, Keen will move MCP authentication from automatic startup resolution to an explicit user action. On startup, Keen will only try to connect to configured MCP servers using credentials that are already available: no auth, configured API keys, or OAuth access tokens stored on disk. If a server cannot connect for any reason, Keen will mark it disconnected and notify the user in the REPL with a suggestion to run `/mcp refresh <tool-name>`.

OAuth authorization will happen only when the user runs the refresh command. At that point, Keen can open the browser, complete the authorization-code flow, exchange the code for tokens, store the tokens on disk, and reconnect the MCP server. The implementation should reuse the existing OAuth and token storage code in `internal/auth/oauth.go` and `internal/auth/store.go` where possible.

## Todo

1. Inspect current MCP implementation and startup wiring:
   - `internal/mcp`
   - `internal/cli/cmd/root.go`
   - REPL command structure
   - existing auth files: `internal/auth/oauth.go`, `internal/auth/store.go`

2. Review current OAuth helper capabilities:
   - how browser opening works
   - how callback/code exchange is handled
   - how tokens are stored and loaded
   - whether token refresh is already supported

3. Define the MCP auth lifecycle:
   - startup only attempts connection with available credentials
   - startup does not open browser
   - startup does not initiate OAuth
   - failed OAuth, missing token, or expired token marks server disconnected
   - `/mcp refresh <tool-name>` initiates OAuth when needed

4. Update MCP config/auth model if needed:
   - support no auth
   - support API key auth
   - support OAuth using disk-backed token lookup
   - make sure config names map cleanly to `/mcp refresh <tool-name>`

5. Add disk-backed OAuth token support for MCP:
   - reuse `internal/auth/store.go`
   - define token keying strategy per MCP server
   - load token during MCP startup
   - save token after successful OAuth flow
   - avoid logging token values

6. Refactor MCP manager startup behavior:
   - attempt connect for each configured server
   - use API key when configured
   - use stored OAuth token when available
   - do not trigger OAuth flow automatically
   - record disconnected state and reason on failure

7. Add user-facing REPL notification for startup failures:
   - collect MCP connection failures
   - show concise REPL notices after startup
   - suggest `/mcp refresh <tool-name>`
   - avoid noisy repeated messages

8. Add `/mcp` REPL command support:
   - `/mcp status`
   - `/mcp refresh <tool-name>`
   - possibly `/mcp list` if status is not enough
   - validate unknown tool or server names clearly

9. Implement `/mcp refresh <tool-name>` flow:
   - find configured MCP server
   - detect auth type
   - for OAuth: open browser and perform authorization-code flow
   - store received token on disk
   - reconnect the MCP server
   - update status and notify user
   - for API key or no-auth: retry connection without OAuth

10. Integrate OAuth with MCP SDK correctly:
    - ensure authorization and token endpoints follow MCP auth spec discovery
    - use existing OAuth helpers where possible
    - keep MCP protocol behavior inside `internal/mcp`
    - keep CLI, browser, and user prompts in REPL or CLI layer

11. Handle token refresh:
    - if a refresh token exists, try refresh before requiring browser flow
    - if refresh fails, mark disconnected
    - `/mcp refresh <tool-name>` should recover by running browser flow

12. Improve MCP status model:
    - connected
    - disconnected
    - auth required
    - auth failed
    - config error
    - include last error safely redacted

13. Add logging:
    - startup connection attempts
    - successful connection and tool count
    - disconnected or auth-required states
    - refresh attempts and outcomes
    - no secrets in logs

14. Add tests for auth and startup behavior:
    - startup with no-auth server succeeds
    - startup with API key succeeds
    - startup with OAuth stored token succeeds
    - startup with OAuth missing token does not open browser
    - startup failure returns user-facing notification
    - failed connect marks server disconnected
    - missing config removes or ignores server as intended

15. Add tests for `/mcp refresh`:
    - OAuth refresh opens flow only on command
    - token is stored after success
    - server reconnects after token save
    - API key and no-auth refresh retry connection
    - unknown server or tool name returns useful error

16. Validate end-to-end configs:
    - public no-auth MCP server
    - OAuth MCP server, PostHog
    - API-key MCP server, Context7
    - keep local test fixture usable and documented

17. Run required cleanup and validation:
    - `gofmt` on modified Go files
    - `go mod tidy`
    - `go test ./...`
    - manual `go run cmd/main.go` sanity check if needed

18. Review git state:
    - separate our changes from pre-existing dirty files
    - summarize tracked and untracked changes clearly
    - do not revert unrelated user changes
