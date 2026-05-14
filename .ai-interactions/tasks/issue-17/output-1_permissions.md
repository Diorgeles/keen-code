# Plan: `/permission` Command with Project-Level Config

## Overview

Introduce a `/permission [allow|deny|default] <tool_names...>` REPL command that lets users grant or revoke specific tool permissions, persisted to `.keen/permissions.json` in the working directory. Overrides apply before the normal permission mechanism but after filesystem guard safety checks.

---

## Config File

**Path:** `<working_dir>/.keen/permissions.json`

```json
{
  "allow": ["bash", "write_file"],
  "deny": ["edit_file"]
}
```

- Tool present in `allow` → auto-grant (skip prompt, including dangerous), filesystem guard still applies
- Tool present in `deny` → always block, regardless of path
- Tool absent from both → normal Keen Code mechanism

---

## New Files

### `internal/config/project_permissions.go`

```go
type ToolSet map[string]struct{}

// MarshalJSON serializes as a JSON array: ["bash", "write_file"]
// UnmarshalJSON parses from a JSON array

type ProjectPermissions struct {
    Allow ToolSet `json:"allow"`
    Deny  ToolSet `json:"deny"`
}
```

`ToolSet` provides O(1) lookups directly and serializes cleanly to/from JSON arrays.

Functions:
- `LoadProjectPermissions(workingDir string) (*ProjectPermissions, error)` — reads `.keen/permissions.json`; returns empty struct (both sets initialized) if file not found
- `SaveProjectPermissions(workingDir string, perms *ProjectPermissions) error` — creates `.keen/` dir if needed, writes JSON

---

## Modified Files

### `internal/cli/repl/permissions/requester.go`

Add `projectPerms *config.ProjectPermissions` field to `Requester`.

At the top of `RequestPermission()`, before the existing `autoApprove` and `sessionAllowedTools` checks, insert:

```
if projectPerms.IsInDeny(toolName)  → return false
if projectPerms.IsInAllow(toolName) → return true   // guard already ran upstream; bypasses dangerous prompt too
```

Update `NewRequester(...)` to accept `*config.ProjectPermissions`.

### `internal/cli/repl/commands/commands.go`

Add:
```go
const Permission = "/permission"
```

Add to `All` slice:
```go
{Name: Permission, Description: "Set tool permissions: allow|deny|default <tool_names>"}
```

### `internal/cli/repl/command_handlers.go`

Add a case for `replcommands.Permission` in `dispatchCommand`. Handler logic:

1. Parse: `/permission <subcommand> <tool1> [tool2...]`
2. If no args or invalid subcommand → print usage + list available tool names (from `appState` registry)
3. Validate each tool name against the registry; warn on unknown names
4. `allow` → delete from deny set, add to allow set
5. `deny` → delete from allow set, add to deny set
6. `default` → delete from both sets
7. Persist updated `projectPerms` via `config.SaveProjectPermissions`
8. Print confirmation

Autocomplete hint: when user types `/permission ` (with trailing space), suggest `allow`, `deny`, `default`. After a subcommand is typed, suggest registered tool names.

### `internal/cli/repl/repl.go` / `internal/cli/repl/tooling/tool_registry.go`

In `SetupToolRegistry` (or just before it in `initialModel`):
- Call `config.LoadProjectPermissions(workingDir)` to get `*ProjectPermissions`
- Pass it through `AppState` or directly to `permissions.NewRequester(...)`
- Store reference on `replModel` so `dispatchCommand` can mutate and persist it

---

## Permission Check Flow (Updated)

```
tool.Execute -> guard.CheckPath(path, op)
  ├── PermissionDenied  → error (hard block, unchanged)
  ├── PermissionGranted → proceed (unchanged)
  └── PermissionPending → permissionRequester.RequestPermission(toolName, ...)
        ├── projectPerms.IsInDeny(toolName)  → false (new)
        ├── projectPerms.IsInAllow(toolName) → true  (new; bypasses dangerous prompt)
        ├── autoApprove                  → true  (existing)
        ├── sessionAllowedTools[toolName]→ true  (existing)
        └── prompt user                          (existing)
```

Note: `PermissionDenied` from the guard (system paths, gitignore, dotfiles) is never reached by `RequestPermission` — it short-circuits before that call. So `/permission allow` cannot bypass those hard blocks.

---

## Key Decisions

| Decision | Choice | Reason |
|---|---|---|
| Config scope | Project-level `.keen/permissions.json` | Per-repo trust model |
| Default key | Absent from both maps | Simpler; no sentinel value needed |
| Load strategy | Once at startup, sync in-memory on `/permission` calls | Avoids per-call file I/O |
| Filesystem guard | Always runs | Safety boundary cannot be overridden |
| Dangerous commands | `/permission allow bash` bypasses dangerous prompt | Bash non-dangerous is already auto-allowed; the only effect of allowing bash is skipping dangerous prompts |

---

## Files Touched Summary

| File | Change |
|---|---|
| `internal/config/project_permissions.go` | **New** — struct + load/save |
| `internal/cli/repl/permissions/requester.go` | Add project perms check + new constructor param |
| `internal/cli/repl/commands/commands.go` | Add `Permission` constant and `All` entry |
| `internal/cli/repl/command_handlers.go` | Add `/permission` dispatch + handler |
| `internal/cli/repl/repl.go` | Load project perms at startup, store on model |
| `internal/cli/repl/tooling/tool_registry.go` | Pass project perms to requester |

---

## Out of Scope

- Persisting session-level allows (`ChoiceAllowSession`) to the project config — that remains in-memory only
- Global (cross-project) permission overrides
- UI for listing current permission state (follow-up: `/permission list`)
