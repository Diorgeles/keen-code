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
| `read_file` | Read file contents | `path` |
| `write_file` | Create/overwrite files | `path`, `content` |
| `edit_file` | Targeted string replacement | `path`, `oldString`, `newString` |
| `glob` | Find files by pattern | `pattern`, `path` |
| `grep` | Search file contents | `pattern`, `path`, `include` |
| `bash` | Execute shell commands | `command`, `isDangerous`, `summary` |

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

**Validation:**
- File must be valid UTF-8 text
- File must be under 10MB
- Binary files are rejected

**Returns:**
```json
{
  "path": "/absolute/path/to/file",
  "content": "file contents...",
  "bytes_read": 1234
}
```

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

Executes shell commands with timeout and output limits.

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
- Timeout: 180 seconds
- Output: 10MB max (truncated if exceeded)

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
  "stderr": "",
  "summary": "Run Go tests"
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