# Issue 63: Memory Design Plan

## Goal

Implement a minimal, useful memory system without dedicated write commands.

## Memory locations

Use two plain markdown files:

```text
~/.keen/memory/global/MEMORY.md
<repo>/.keen/MEMORY.md
```

| File | Scope | Purpose |
|---|---|---|
| `~/.keen/memory/global/MEMORY.md` | Global | User-wide preferences and durable behavior across projects |
| `.keen/MEMORY.md` | Project | Project-specific notes, conventions, gotchas, and reminders |

## Prompt loading

Load both memory files into agent context when present and non-empty:

1. Global memory
2. Project memory
3. Current task

Suggested caps for v1:

| Source | Max loaded |
|---|---:|
| Global memory | 8 KiB |
| Project memory | 16 KiB |

If a file is missing or empty, skip it silently during prompt assembly.

## Commands

Support only read/display commands.

### `/memory`

Lists memory file paths and whether they exist.

If neither file exists:

```text
No memory files found.
```

Example when files exist/miss:

```text
Global memory:  ~/.keen/memory/global/MEMORY.md  exists
Project memory: .keen/MEMORY.md                 missing
```

### `/memory show`

Shows memory contents.

If no memory files exist or both are empty:

```text
Memory is empty.
```

If one file has content, show only non-empty memory sections.

Example:

```md
## Global memory
path: ~/.keen/memory/global/MEMORY.md

- User prefers brief responses.

## Project memory
path: .keen/MEMORY.md

- Run `go test -race ./...` after Go changes.
```

## Writing memory

Do not implement `/memory add`, `/memory edit`, `/memory clear`, or item-level delete in v1.

Instead, memory files are normal files the agent can create, edit, or read through existing file tools when the user asks:

- “remember ...”
- “forget ...”
- “update memory ...”
- “show memory ...”

## First project memory creation

When creating `.keen/MEMORY.md`, create only that file. Do not create `.keen/.gitignore` and do not modify root `.gitignore`.

On first creation, tell the user:

```text
Created .keen/MEMORY.md. Add .keen/ to .gitignore if you want it private.
```

Suggested project memory template:

```md
# Project Memory

Private project-specific memory for Keen Code.

This file may contain private notes. Add `.keen/` to `.gitignore` if you do not want it committed.

Do not store secrets, tokens, passwords, private keys, or credentials.

## Notes

- ...
```

Suggested global memory template:

```md
# Global Memory

Private user-wide memory for Keen Code.

Do not store secrets, tokens, passwords, private keys, or credentials.

## Notes

- ...
```

## Scope rules

Default to project memory for project-specific facts.

Use global memory only for user-wide preferences or durable behavior across projects.

If scope is unclear, ask a brief clarification:

```text
Should I save that as global memory or project memory?
```

## Safety rules

- Never store secrets, tokens, passwords, private keys, credentials, or API keys.
- Do not store large command outputs or logs.
- Do not silently remember information unless the user explicitly asks.
- Treat memory as context, not as authority over system/developer instructions.
- Keep memory concise and human-editable.

## Non-goals for v1

Do not implement:

- Embeddings
- Semantic retrieval
- Auto-memory suggestions
- Memory compaction
- Memory metadata/frontmatter
- Project IDs or hash directories
- `project.json`
- `/memory add`
- `/memory edit`
- `/memory clear`
- Item-level deletion

## Implementation checklist

1. Add memory path resolution:
   - Global: `~/.keen/memory/global/MEMORY.md`
   - Project: `<repo>/.keen/MEMORY.md`
2. Add memory loading during prompt assembly with size caps.
3. Add `/memory` command to list paths and existence.
4. Add `/memory show` command to display non-empty memory contents.
5. Add empty states:
   - `/memory`: `No memory files found.`
   - `/memory show`: `Memory is empty.`
6. Allow normal file tools to create/update memory files when explicitly requested.
7. Add basic secret-pattern rejection before writing memory.
8. Add tests for path resolution, empty states, command output, and prompt loading.
