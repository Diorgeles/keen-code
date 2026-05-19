# MCP Phase 2
1. Here's what we want to do:
   - On startup, Keen should simply try to connect using whatever credentials are already available.
   - If connection fails, Keen should notify the user in the REPL and suggest `/mcp refresh <tool-name>`.
   - OAuth tokens must be stored on disk and reused.
   - Reuse the existing OAuth flow in `internal/auth/oauth.go` and `internal/auth/store.go`.
   - Startup must not try to resolve auth or open the browser.
   - User-triggered refresh should initiate the full OAuth flow and open the browser.
2. Save a comprehensive phase 2 todo list in `.ai-interactions/tasks/issue-43/output-2_mcp-phase-2.md`, first describing what this phase is doing and then listing all tasks.
3. Implement phase 2.
4. Reuse existing OAuth code where possible. Explain what was missing if it is not reused directly.
5. `/mcp refresh` should not be only for OAuth. It should refresh the configured server using whatever auth mode applies.
6. `mcpRuntime` should be part of the REPL context or app state only if that matches existing architecture. Prefer declaring MCP runtime-related types in the `mcp` package, like config/providers patterns.
7. Put MCP-specific OAuth helpers in the `mcp` package and reuse generic pieces from `internal/auth`. Keep Keen provider OAuth and MCP OAuth separated.
8. The startup wait timeout is for the UI status command, not the underlying MCP connection attempts. Avoid making the REPL wait too long.
9. In `handleMCPStartupStatus`, wrap messages within the window and reuse existing wrapping helpers where sensible.
10. `internal/cli/repl/mcp.go` should not exist. Move those types into `internal/mcp/types.go`.
11. Remove premature synchronization from `Manager.Start`. Keen will not call `Start` twice inside the same process.
12. Remove manager-level `wg`; use local wait groups only.
13. `Close()` should sequentially close in-memory server sessions. Do not add locking unless it is needed.
14. Closing Streamable HTTP sessions on Keen shutdown is unnecessary and slows exit. Stop attempting to close Streamable HTTP on shutdown; still close stdio sessions.
15. Simplify `closeSession` after skipping Streamable HTTP in `Close`.
16. MCP can use its own logger; do not propagate the root logger into the MCP manager.
17. Since `slog.Default()` is the logger initialized in `cmd/main.go`, use `slog.Default()` directly in MCP.
18. Add proper MCP logs and review the latest log file to verify startup/connection visibility.
19. Add proper MCP logs and review the latest log file to verify startup/connection visibility.
20. `/mcp status` should use the same table style as the skills status table. Reuse existing table rendering where possible.
21. In `/mcp status`, `disconnected` should use `ErrorStyle`.
22. Startup should show failed MCPs early; do not wait for the full connection timeout before notifying the user.
23. Manual `/mcp refresh` needs enough time for OAuth browser authentication. Use a named refresh timeout constant and apply it to both the command context and refresh connect timeout.
24. Explain the MCP authentication flow from entry point to final authentication success.
25. Simplify MCP authentication:
   - Load OAuth tokens from `auth.json` once into manager memory.
   - API keys remain available from MCP config.
   - Startup goroutines read in-memory credentials.
   - `/mcp refresh <server>` opens browser for OAuth, saves the token to `auth.json`, and updates memory.
   - Remove unnecessary memory/disk token-store split.
26. Remove `internal/mcp/doc.go`.
27. In `loadOAuthTokens`, only load auth store entries with the `mcp:` prefix.
28. Simplify the MCP package by moving REPL-specific messages into `internal/cli/repl/stream_msgs.go`.
29. Remove `runtimeSnapshot` from `Manager`.
30. Remove local MCP tool argument schema validation and rely on server-side validation.
31. Remove unused `Manager` fields `configPath` and `config`.
32. Inspect the newly introduced `serverRuntime` `RWMutex`, explain whether races are real, and identify every scenario where `serverRuntime` can be concurrently accessed.
33. Fix the unsynchronized `rt.status` read in `discoverServer` by moving it into the existing locked commit section.
34. Run race tests and confirm `go test -race ./...` passes.
