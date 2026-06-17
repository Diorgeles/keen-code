# Tools

Keen Code provides a set of built-in tools that the LLM can use to interact with the codebase. Tools are registered in a central registry and exposed to the LLM with their schemas.

## Tool Registry

```go
// internal/tools/tool.go
type Tool interface {
    Name() string
    Description() string
    InputSchema() map[string]any
    Execute(ctx context.Context, input any) (any, error)
}

type Registry struct {
    tools map[string]Tool
}
```

The registry manages all available tools and converts them to provider-specific tool formats (Anthropic, OpenAI, etc.).

## Available Tools

| Tool | Purpose | Key Parameters |
|------|---------|----------------|
| `read_file` | Read file contents | `path`, `offset`, `limit` |
| `write_file` | Create/overwrite files | `path`, `content` |
| `edit_file` | Targeted string replacement | `path`, `oldString`, `newString` |
| `glob` | Find files by pattern | `pattern`, `path` |
| `grep` | Search file contents | `pattern`, `path`, `include` |
| `bash` | Execute shell commands | `command`, `isDangerous`, `summary` |
| `web_fetch` | Fetch content from a URL | `url` |
| `call_mcp_tool` | Call a tool on an MCP server | `server`, `tool`, `arguments` |

## read_file

Reads a UTF-8 text file with permission checks and validation.

```go
type ReadFileTool struct {
    guard               *filesystem.Guard
    permissionRequester PermissionRequester
}
```

**Parameters:**
- `path` (string, required): Absolute or relative path to the file
- `offset` (integer, optional): 1-based line number to start reading from (defaults to 1)
- `limit` (integer, optional): Maximum number of lines to return (defaults to 1000)

**Validation:**
- File must be valid UTF-8 text
- File must be under 25MB; `offset` and `limit` bound returned lines, not the initial file-size check
- Binary files are rejected
- Long lines are truncated to 1000 runes to keep tool results bounded

**Returns:**
```json
{
  "path": "/absolute/path/to/file",
  "content": "1: file contents...",
  "bytes_read": 1234,
  "offset": 1,
  "limit": 1000,
  "total_lines": 10,
  "truncated": false
}
```

Files under `~/.keen/bash/` can be read without an extra permission prompt. Use the returned `stdout_file` and `stderr_file` paths from `bash` rather than guessing artifact names.

## write_file

Creates a new file or overwrites existing content.

```go
type WriteFileTool struct {
    guard               *filesystem.Guard
    diffEmitter         DiffEmitter
    permissionRequester PermissionRequester
}
```

**Parameters:**
- `path` (string, required): Target file path
- `content` (string, required): Content to write

**Behavior:**
- Creates parent directories if needed
- Overwrites existing files completely
- Emits diff for display via `DiffEmitter`

**Returns:**
```json
{
  "path": "/absolute/path/to/file",
  "bytes_written": 1234,
  "created": true
}
```

## edit_file

Performs targeted string replacement in existing files.

```go
type EditFileTool struct {
    guard               *filesystem.Guard
    diffEmitter         DiffEmitter
    permissionRequester PermissionRequester
}
```

**Parameters:**
- `path` (string, required): Target file path
- `oldString` (string, required): Exact text to find and replace
- `newString` (string, required): Replacement text
- `shouldReplaceAll` (boolean, optional): Replace all occurrences (default: false)

**Behavior:**
- File must already exist
- `oldString` must match exactly (including whitespace)
- Uses `go-udiff` for unified diff output
- Emits diff via `DiffEmitter`

**Returns:**
```json
{
  "success": true,
  "path": "/absolute/path/to/file",
  "replacementCount": 1
}
```

## glob

Finds files matching a glob pattern.

```go
type GlobTool struct {
    guard               *filesystem.Guard
    permissionRequester PermissionRequester
}
```

**Parameters:**
- `pattern` (string, required): Glob pattern (e.g., `*.go`, `**/*.md`)
- `path` (string, optional): Base directory (defaults to working directory)

**Limits:**
- Maximum 1000 files returned

**Returns:**
```json
{
  "pattern": "*.go",
  "base_path": "/project",
  "files": ["/project/main.go", "/project/pkg/foo.go"],
  "count": 2
}
```

## grep

Searches file contents using regular expressions.

```go
type GrepTool struct {
    guard               *filesystem.Guard
    permissionRequester PermissionRequester
}
```

**Parameters:**
- `pattern` (string, required): Regex pattern (Go/RE2 syntax)
- `path` (string, optional): Base directory
- `include` (string, optional): Glob filter for file types
- `output_mode` (string, optional): `"file"` or `"content"` (default)

**Limits:**
- Maximum 1000 matches

**Returns (content mode):**
```json
{
  "pattern": "func foo",
  "base_path": "/project",
  "output_mode": "content",
  "matches": [
    {"file": "/project/main.go", "line_number": 10, "line": "func foo() {"},
    {"file": "/project/main.go", "line_number": 25, "line": "func foo() error {"}
  ],
  "count": 2
}
```

## bash

Executes shell commands with timeout and bounded inline output. Large stdout is saved to an artifact file so the model can inspect it later without flooding the prompt.

```go
type BashTool struct {
    guard               *filesystem.Guard
    permissionRequester PermissionRequester
}
```

**Parameters:**
- `command` (string, required): Bash command to execute
- `isDangerous` (boolean, optional): Always prompts for permission if true
- `summary` (string, optional): Brief description for the UI

**Limits:**
- Timeout: 300 seconds
- Inline output: 64KB max per stream before truncation
- Truncated output preview: head/tail excerpt with omitted-byte count
- Full truncated stdout is written to randomly named files under `~/.keen/bash/`, such as `keen-bash-*.stdout`
- Stderr is returned only when the command exits non-zero; large captured stderr may be saved to `stderr_file`

When `truncated` is true, the agent should not rerun the same broad command just to see more output. It should inspect any returned `stdout_file` or `stderr_file` with `read_file` using targeted `offset`/`limit` values, or use `grep` for targeted follow-up.

**Dangerous commands (always prompt):**
- File removal (`rm`, `rm -rf`)
- Git operations that modify repo (`git commit`, `git push`, `git reset`, `git rebase`)
- Process termination (`kill`)
- System modifications

**Returns:**
```json
{
  "command": "go test ./...",
  "exit_code": 0,
  "stdout": "PASS\nok      github.com/user/keen-code    0.015s",
  "truncated": false,
  "summary": "Run Go tests"
}
```

**Returns (truncated output):**
```json
{
  "command": "grep -R plan.md ~/.keen/sessions",
  "exit_code": 0,
  "stdout": "first preview...\n\n... (1048576 bytes omitted; full stdout saved to /Users/alice/.keen/bash/keen-bash-abc123.stdout) ...\n\nlast preview...",
  "truncated": true,
  "stdout_file": "/Users/alice/.keen/bash/keen-bash-abc123.stdout"
}
```

## web_fetch

Fetches content from a URL and returns it as text.

```go
type WebFetchTool struct{}
```

**Parameters:**
- `url` (string, required): The URL to fetch

**Behavior:**
- HTML pages are automatically converted to Markdown for readability
- Other content types (JSON, plain text, XML) are returned as-is
- JavaScript-rendered pages (SPAs) return the pre-JS skeleton only

**Limits:**
- Timeout: 30 seconds
- Maximum response size: 128KB (truncated if exceeded)

**Returns:**
```json
{
  "url": "https://example.com",
  "status_code": 200,
  "content": "markdown or raw content..."
}
```

## call_mcp_tool

Calls a tool on a connected MCP (Model Context Protocol) server.

```go
type CallMCPTool struct {
    manager             keenmcp.Runtime
    permissionRequester PermissionRequester
}
```

**Parameters:**
- `server` (string, required): The MCP server name as configured
- `tool` (string, required): The exact tool name to call on the server
- `arguments` (object, optional): Key-value arguments matching the tool's input schema
- `checkCache` (boolean, optional): Reserved for future caching; set to false or omit

**Behavior:**
- Requires user permission before execution
- Server name must match a configured MCP server
- Arguments must match the tool's input schema exactly
- Skill file at `~/.keen/skills/mcp:<server>/SKILL.md` describes available tools
- Schema file at `~/.keen/skills/mcp:<server>/schemas/<tool>.json` describes required arguments

**Returns:**
```json
{
  "server": "server-name",
  "tool": "tool-name",
  "content": "tool output text"
}
```

## DiffEmitter

The `DiffEmitter` interface allows tools to emit diff output for display:

```go
// internal/tools/diff.go
type DiffEmitter interface {
    EmitDiff(lines []EditDiffLine)
}

type EditDiffLine struct {
    Kind       EditDiffLineKind
    OldLineNum int
    NewLineNum int
    Content    string
}

const (
    DiffLineContext EditDiffLineKind = iota
    DiffLineAdded
    DiffLineRemoved
    DiffLineHunk
)
```

## Permission Integration

All tools integrate with the permission system through `PermissionRequester`:

```go
// internal/tools/permission.go
type PermissionRequester interface {
    RequestPermission(ctx context.Context, toolName, path, resolvedPath string, isDangerous bool) (bool, error)
}
```

Tools check permissions before execution and may request user approval for:
- Paths outside the working directory
- Dangerous operations (marked with `isDangerous=true`)
- First-time access to certain paths
