# Persistent Memory In Keen

## Overview

Persistent memory lets Keen retain useful context across sessions beyond static instruction files like `AGENTS.md`. It is stored as plain, human-editable markdown files and loaded into the agent's system prompt at the start of every session.

## Memory Files

| File | Scope | Purpose | Cap |
|---|---|---|---|
| `~/.keen/memory/global/MEMORY.md` | Global | User-wide preferences and durable behavior across projects | 8 KiB |
| `<repo>/.keen/MEMORY.md` | Project | Project-specific notes, conventions, gotchas | 16 KiB |

Missing or empty files are skipped silently during prompt assembly.

## How Memory Differs From AGENTS.md

- **AGENTS.md** is static, hand-written by the user, and acts as developer/system instructions (high authority).
- **Memory** is accumulated by the agent when the user asks it to remember or forget things, and is treated as context — subordinate to system and developer instructions.

## Commands

### `/memory`

Lists memory file paths and whether they exist.

```text
Global memory:  ~/.keen/memory/global/MEMORY.md  exists
Project memory: .keen/MEMORY.md                 missing
```

If neither file exists:

```text
No memory files found.
```

### `/memory show`

Displays non-empty memory contents. Supports an optional scope argument:

- `/memory show` — shows all non-empty memory
- `/memory show global` — shows only global memory
- `/memory show project` — shows only project memory

If no memory files exist or both are empty:

```text
Memory is empty.
```

## Writing Memory

There are no dedicated write commands (`/memory add`, `/memory edit`, etc.). Memory files are normal files that the agent creates or edits through the existing file tools (`write_file`, `edit_file`) when the user asks:

- "remember ..."
- "forget ..."
- "update memory ..."
- "show memory ..."

### Scope Rules

- Default to **project memory** for project-specific facts.
- Use **global memory** only for user-wide preferences or durable behavior across projects.
- If the scope is unclear, the agent asks: "Should I save that as global memory or project memory?"

### First Project Memory Creation

When first creating `.keen/MEMORY.md`, the agent tells the user:

```text
Created .keen/MEMORY.md. Add .keen/ to .gitignore if you want it private.
```

The agent does not modify `.gitignore` itself.

## Safety

- Never store secrets, tokens, passwords, private keys, credentials, or API keys in memory files.
- The `write_file` and `edit_file` tools reject writes to memory paths if the content matches known secret patterns (API keys, bearer tokens, private key blocks, etc.).
- Do not store large command outputs or logs in memory.
- The agent never silently remembers information unless the user explicitly asks.
- Memory is treated as context, not as authority over system or developer instructions.
- Memory is kept concise and human-editable.

## Implementation

- `internal/memory` — path resolution, loading with size caps, secret-pattern detection.
- `internal/llm/systemprompt.go` — memory loaded into the system prompt during `Build`.
- `internal/filesystem/guard.go` — `~/.keen/memory/` is allowed for reads (granted) and writes (pending user approval).
- `internal/tools/write_file.go`, `internal/tools/edit_file.go` — secret-pattern rejection before writing memory paths.
- `internal/cli/repl/commands/commands.go` — `/memory` and `/memory show` command registration.
- `internal/cli/repl/command_handlers.go` — `handleMemoryCommand` implementation.