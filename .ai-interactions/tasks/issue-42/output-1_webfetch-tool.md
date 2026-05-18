# Plan: Add `web_fetch` Tool

## Context
Keen's LLM agent needs the ability to retrieve web content during task execution — for reading documentation, fetching API specs, or checking URLs referenced in code. HTML responses are converted to Markdown using `html-to-markdown` for cleaner LLM consumption. The tool follows the same `Tool` interface pattern as all existing tools.

## New Dependency

Add `github.com/JohannesKaufmann/html-to-markdown` for HTML-to-Markdown conversion. Run `go get github.com/JohannesKaufmann/html-to-markdown` and `go mod tidy` after implementation.

## Implementation

### New file: `internal/tools/web_fetch.go`

Implement `WebFetchTool` struct — no `guard` or `permissionRequester` needed (read-only network operation, no filesystem involvement):

```go
type WebFetchTool struct{}

func NewWebFetchTool() *WebFetchTool
func (t *WebFetchTool) Name() string        // "web_fetch"
func (t *WebFetchTool) Description() string
func (t *WebFetchTool) InputSchema() map[string]any
func (t *WebFetchTool) Execute(ctx context.Context, input any) (any, error)
```

**Input schema** — single required parameter:
- `url` (string, required): The URL to fetch

**Execute logic:**
1. Assert `input.(map[string]any)`, extract and validate `url` is a non-empty string
2. `http.NewRequestWithContext(ctx, "GET", url, nil)` with a `User-Agent` header
3. Use `http.Client{Timeout: 30 * time.Second}`
4. Read body, truncate at 512KB with `... (content truncated)` suffix
5. If `Content-Type` contains `text/html`, convert body to Markdown using `html-to-markdown`; otherwise use body as-is
6. Return `map[string]any{"url": url, "status_code": resp.StatusCode, "content": content}`
7. Non-2xx responses: return body + status code (not an error — LLM should see the status)
8. Network errors: return `error`

**Limitations to document in `Description()`:** JavaScript-rendered pages (SPAs) will return the pre-JS skeleton, not the full content.

### New file: `internal/tools/web_fetch_test.go`

Tests using `httptest.NewServer` to avoid real network calls:
- `Name()` returns `"web_fetch"`
- `Description()` is non-empty
- `InputSchema()` has correct shape (`type: object`, `url` property, `required: ["url"]`, `additionalProperties: false`)
- `Execute()` with missing `url` returns error
- `Execute()` with invalid URL type returns error
- `Execute()` with HTML response: assert `status_code` == 200 and `content` is non-empty Markdown (not raw HTML)
- `Execute()` with plain text / JSON response: returns body as-is
- `Execute()` with non-2xx response: returns `status_code` + body without error
- `Execute()` truncates large responses at 512KB

### Wiring: `internal/cli/repl/tooling/tool_registry.go`

Add after `bashTool` registration:
```go
webFetchTool := tools.NewWebFetchTool()
appState.RegisterTool(webFetchTool)
```

## Critical Files

| File | Change |
|------|--------|
| `internal/tools/web_fetch.go` | New — tool implementation |
| `internal/tools/web_fetch_test.go` | New — unit tests |
| `internal/cli/repl/tooling/tool_registry.go` | Register the new tool |
| `go.mod` / `go.sum` | Add `html-to-markdown` dependency |

## Verification

```bash
go get github.com/JohannesKaufmann/html-to-markdown
go mod tidy
go test ./internal/tools/... -run TestWebFetch -v
go test ./internal/tools/...
go test ./...
gofmt -w internal/tools/web_fetch.go
```
