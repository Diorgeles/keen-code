# MCP + Skills Integration Plan

## Overview

4 new/modified areas: a new `internal/mcpskills` package, a new `call_mcp_tool` in `internal/tools`, wiring in `internal/cli/repl/tooling/tool_registry.go`, and skill generation hooks in `repl_helpers.go`.

---

## 1. New package: `internal/mcpskills`

Handles all MCP skill file management. No imports from `internal/skills` (avoids circular deps).

**Functions:**

| Function | Responsibility |
|---|---|
| `SkillDir(server string) (string, error)` | Returns `~/.keen/skills/mcp-<server>/` |
| `Generate(server string, tools []mcp.Tool) error` | Atomically writes `SKILL.md`, `.keen-generated-mcp.json`, `schemas/*.json` |
| `Cleanup(configuredServers []string) error` | Deletes managed dirs (has `.keen-generated-mcp.json` with `managed_by: "keen-mcp"`) whose server is not in `configuredServers` |

**Atomic write strategy for `Generate`:**
1. Write to a temp dir `~/.keen/skills/.mcp-<server>-tmp-<pid>/`
2. `os.Rename(tmp, mcp-<server>/)` — temp dir is inside `~/.keen/skills/` so rename is same-filesystem

**Generated `SKILL.md` shape:**
```markdown
---
name: mcp-<server>
description: <server description>
---
## When to use
Use this skill to interact with the `<server>` MCP server.

## Available tools
| Tool | Description |
|------|-------------|
| create_issue | Create a GitHub issue |
| list_issues | List issues in a repository |
```

No generated-metadata header, no schema locations, no invocation section.

**Generated `.keen-generated-mcp.json`:**
```json
{
  "managed_by": "keen-mcp",
  "server": "github",
  "status": "connected",
  "tool_count": 12,
  "last_successful_refresh": "2026-05-17T10:00:00Z",
  "last_error": ""
}
```

`status` is one of `"connected"` or `"disconnected"`. Written by `Generate` as `"connected"`. Not updated on failure — stale files keep their last-known status, which reflects the last successful generation.

Schema files are written to `schemas/<tool>.json`. `call_mcp_tool` finds them deterministically via `SkillDir(server)/schemas/<tool>.json`.

**Disconnect behavior:** No `Disable()` function. If a server fails to connect, stale skill files from a prior successful connection are left in place. The LLM can attempt to use the skill and the MCP layer returns the connection error — letting the user decide to run `/mcp refresh`.

---

## 2. New tool: `internal/tools/call_mcp_tool.go`

```go
type callMCPInput struct {
    Server     string         `json:"server"`
    Tool       string         `json:"tool"`
    Arguments  map[string]any `json:"arguments,omitempty"`
    CheckCache bool           `json:"checkCache,omitempty"` // reserved, no-op for now
}

type CallMCPTool struct {
    manager             mcp.Runtime
    permissionRequester PermissionRequester
}

func NewCallMCPTool(manager mcp.Runtime, pr PermissionRequester) *CallMCPTool
```

**`Execute` flow:**
1. Parse `server`, `tool`, `arguments`, `checkCache` from input map (`checkCache` is a no-op: `_ = input.CheckCache`)
2. Call `permissionRequester.RequestPermission(ctx, "call_mcp_tool", server+"/"+tool, argsJSON, false)` — `isDangerous=false`
3. On denial: return error
4. Call `manager.CallTool(ctx, server, tool, arguments)`
5. Format `ToolResult.Content` into a string response

**Permission flow:** `isDangerous=false` — same as `read_file`/`edit_file`. Supports session-allow and `/allow-permission`. No changes to permission card rendering needed.

**Safety checks** (delegate to `mcp.Manager`):
- Server not configured → `ErrServerNotConfigured` from `CallTool`
- Server disconnected → state error from `CallTool`
- Tool not found → `ErrToolNotFound` from `CallTool`

---

## 3. Wire up: `internal/cli/repl/tooling/tool_registry.go`

Add `mcpRuntime keenmcp.Runtime` parameter to `SetupToolRegistry`. Register `call_mcp_tool` when runtime is non-nil:

```go
func SetupToolRegistry(workingDir string, appState *replappstate.AppState,
    permReq *replpermissions.Requester, diffEmitter *DiffEmitter,
    mcpRuntime keenmcp.Runtime) {
    ...
    if mcpRuntime != nil {
        mcpTool := tools.NewCallMCPTool(mcpRuntime, permReq)
        appState.RegisterTool(mcpTool)
    }
}
```

Update call sites: `repl.go:initialModel` (pass `ctx.mcp`) and tests (pass `nil`).

---

## 4. Skill generation hooks: `internal/cli/repl/repl_helpers.go`

**`handleMCPStartupStatus`** — extend after existing status display:
```
for each status in statuses:
    if connected:
        tools, _ = m.ctx.mcp.ListTools(context.Background(), status.Name)
        mcpskills.Generate(status.Name, tools)   // best-effort, log on error

configuredNames = [status.Name for all statuses]
mcpskills.Cleanup(configuredNames)

m.appState.ReloadSkills()
```

`ListTools` reads from in-memory cache — safe to call synchronously here.

**`handleMCPRefreshDone`** — after existing success/failure output:
```
if msg.Err == nil:
    tools, _ = m.ctx.mcp.ListTools(context.Background(), msg.Server)
    mcpskills.Generate(msg.Server, tools)
    m.appState.ReloadSkills()
// on error: no-op — stale skill stays, connection error surfaces at call time
```

`ReloadSkills()` is only called when something actually changed (on successful connect/refresh).

---

## 5. No changes needed

| Package | Reason |
|---|---|
| `internal/skills/*` | Skills without a `SKILL.md` are simply not found; no disable mechanism needed |
| `internal/mcp/*` | Already complete |
| `internal/cli/repl/permissions/` | Existing `isDangerous=false` path handles session-allow and `/allow-permission` |

---

## Risks & mitigations

| Risk | Mitigation |
|---|---|
| Skill generation fails for one server | Best-effort: log error, continue for other servers |
| Atomic rename fails on different filesystems | Write temp dir inside `~/.keen/skills/` — same filesystem guaranteed |
| `Cleanup` deletes user files by mistake | Only delete dirs containing `.keen-generated-mcp.json` with `"managed_by": "keen-mcp"` |
| LLM tries a disconnected server's tool | `CallTool` returns a clear error; user can `/mcp refresh` |
| `checkCache` field causes confusion later | Field is defined in schema now; implementation is a no-op `_ = input.CheckCache` |
| Large tool list → large `SKILL.md` | Cap tool table at 100 rows |

---

## Verification steps

1. `go test ./internal/mcpskills/...` — Generate/Cleanup logic
2. `go test ./internal/tools/...` — `call_mcp_tool` with mock runtime
3. `go test -race ./...` — no races on skill reload after MCP events
4. Manual: configure a stdio MCP server, start Keen, verify `~/.keen/skills/mcp-<server>/` is created and skill appears in catalog
