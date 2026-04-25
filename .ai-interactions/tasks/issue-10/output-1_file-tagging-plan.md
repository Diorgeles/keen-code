# File Tagging (`@`) — Implementation Plan

## Overview

Add `@`-based file tagging to the REPL input. When the user types `@` followed by at
least one character, a dropdown appears showing up to 10 matching file paths from the
working directory. The dropdown reuses the existing `SuggestionModel` infrastructure
(styles, navigation, rendering) to avoid duplication. On selection, the relative path
replaces the `@<query>` token in the textarea. Multiple `@` tags are supported.

---

## Architecture

### How it works

```
┌──────────────────────────────────────┐
│  viewport                            │
├──────────────────────────────────────┤
│  > Fix the bug in @src/m             │  ← textarea with cursor after @src/m
├──────────────────────────────────────┤
│  src/main.go                         │  ← file suggestion dropdown (max 10)
│  src/model.go                        │
│  src/middleware.go                    │
├──────────────────────────────────────┤
│  model: openai/gpt-4o · ctx: 12%    │  ← metadata
└──────────────────────────────────────┘
```

- The user types `@src/m` → dropdown shows files matching `src/m` prefix/substring
- Up/Down to navigate, Tab/Enter to select, Esc to dismiss
- On selection, `@src/m` is replaced with `@src/main.go` (or the selected path)
- The user can type `@` again elsewhere to tag another file

### Key design decisions

| Topic | Decision |
|-------|----------|
| Trigger | `@` character followed by ≥1 character |
| File source | Live glob using `doublestar.GlobWalk` with `Guard.IsBlocked` to skip gitignored/blocked dirs — no in-memory cache |
| Includes | Both files and directories |
| Pattern construction | Query `foo` → glob pattern `**/*foo*` (case-insensitive via `doublestar` options or lowercase normalization) |
| Matching | Substring match on file/directory names via glob wildcards; always fresh results (no stale cache) |
| Max results | 10 (stop walking after 10 matches) |
| Dropdown UI | Reuse `SuggestionModel` rendering logic and theme styles |
| Selection behavior | Replace the `@<query>` token at cursor with the selected relative path |
| Multiple tags | Each `@` token is independent; only the one at/before cursor is active |

---

## Files to create / modify

| File | Action | Purpose |
|------|--------|---------|
| `internal/cli/repl/filesearch/filesearch.go` | **Create** | Live glob search using `doublestar.GlobWalk` with guard filtering |
| `internal/cli/repl/filesearch/filesearch_test.go` | **Create** | Unit tests for file search |
| `internal/cli/repl/widgets/suggestion.go` | **Modify** | Generalize to support both slash commands and file suggestions |
| `internal/cli/repl/widgets/suggestion_test.go` | **Modify** | Add tests for file suggestion mode |
| `internal/cli/repl/repl.go` | **Modify** | Initialize file searcher, pass to suggestion model |
| `internal/cli/repl/handlers.go` | **Modify** | Detect `@` token, switch suggestion source, handle selection replacement |

---

## Granular Todo Items

### 1. Create `filesearch/filesearch.go` — live glob-based file search

- [ ] Define `FileSearcher` struct holding `workingDir string` and
  `guard *filesystem.Guard`
- [ ] Implement `NewFileSearcher(workingDir string, guard *filesystem.Guard) *FileSearcher`
- [ ] Implement `(s *FileSearcher) Search(query string, limit int) []string`:
  - If query is empty, return nil
  - Construct glob pattern: `**/*{query}*` (wraps the query in wildcards for substring matching)
  - Use `doublestar.GlobWalk(os.DirFS(workingDir), pattern, ...)` to walk the filesystem
  - Inside the walk callback, skip paths where `guard.IsBlocked(absPath)` is true
  - Collect both file and directory relative paths; stop early after `limit` matches (return `fs.SkipAll`)
  - Return the collected paths
- [ ] Handle special glob characters in the query by escaping them (e.g. `[`, `]`, `{`, `}`)
  before constructing the pattern

### 2. Create `filesearch/filesearch_test.go`

- [ ] Test `Search("")` returns nil
- [ ] Test `Search("main")` returns file and directory paths containing "main"
- [ ] Test `Search("xyz_nonexistent")` returns empty
- [ ] Test limit is respected (query matching many files returns at most `limit`)
- [ ] Test directories are included in results (not just files)
- [ ] Test special characters in query are handled (e.g. `[`, `]`)
- [ ] Use `t.TempDir()` with a known directory structure for deterministic tests

### 3. Generalize `SuggestionModel` in `widgets/suggestion.go`

The current `SuggestionModel` is tightly coupled to `SlashCommand`. We need to
generalize it to support two suggestion sources: slash commands and file paths.

- [ ] Define a `SuggestionItem` interface or struct:
  ```go
  type SuggestionItem struct {
      Name        string   // display name (command name or file path)
      Description string   // description (command desc or empty for files)
  }
  ```
- [ ] Change `SuggestionModel.items` from `[]SlashCommand` to `[]SuggestionItem`
- [ ] Add a `mode` field to track the current suggestion type (`commandMode` /
  `fileMode`) — this determines behavior on selection
- [ ] Update `Refresh` to accept `[]SuggestionItem` directly (move filtering logic
  to the caller in handlers.go)
- [ ] Alternatively, keep `Refresh(input string)` for commands and add
  `RefreshFiles(items []SuggestionItem)` — whichever is cleaner
- [ ] Ensure `Current()` returns `*SuggestionItem`
- [ ] Ensure `View()` works correctly for both modes (file items have no description,
  so the desc column can be omitted or empty)
- [ ] Update all callers to use the new types

### 4. Update `handlers.go` — detect `@` token and handle selection

- [ ] Add a helper `extractAtToken(input string, cursorPos int) (token string, startIdx int, found bool)`:
  - Scan backwards from cursor position to find the nearest `@`
  - The token is the text between `@` and the cursor (exclusive of `@`)
  - Return `found = false` if no `@` is found or token is empty (< 1 char)
  - Stop scanning if we hit a space before finding `@` (the `@` must be preceded by
    a space or be at the start of the input)
- [ ] In the key handler fall-through (after textarea update), determine suggestion mode:
  - If input starts with `/` → command suggestions (existing behavior)
  - Else if `extractAtToken` finds a valid token → file suggestions
  - Else → hide suggestions
- [ ] On Tab/Enter when file suggestion is visible:
  - Get the current textarea value and cursor position
  - Use `extractAtToken` to find the `@token` range
  - Replace `@token` with `@selected_path ` (with trailing space) in the textarea value
  - Set the new value and adjust cursor position
- [ ] On Tab/Enter when command suggestion is visible: keep existing behavior

### 5. Update `repl.go` — initialize file searcher

- [ ] Add `fileSearcher *filesearch.FileSearcher` field to `replModel`
- [ ] In `initialModel()`, create the file searcher:
  ```go
  model.fileSearcher = filesearch.NewFileSearcher(ctx.workingDir, guard)
  ```
  - Note: we need access to a `Guard` instance. The guard is currently created inside
    `repltooling.SetupToolRegistry`. Either:
    - (a) Create the guard earlier and pass it to both `SetupToolRegistry` and `NewFileSearcher`
    - (b) Create a separate guard for the file searcher (simple, no permission checking needed)
  - **Decision**: Option (b) — create a lightweight guard with the same gitignore
    awareness just for walking. Or even simpler: reuse the same `filesystem.Guard`
    by extracting its creation.
- [ ] Pass `m.fileSearcher` to the suggestion refresh logic in handlers

### 6. Update `widgets/suggestion_test.go`

- [ ] Add tests for `SuggestionItem`-based items (file mode)
- [ ] Test `View()` with items that have no description
- [ ] Test `Current()` returns correct `SuggestionItem` in file mode
- [ ] Test navigation (MoveUp/MoveDown) works the same for both modes

### 7. Testing & verification

- [ ] Run `go test ./...` — all tests pass
- [ ] Run `go mod tidy`
- [ ] Manual smoke test:
  - Type `@s` → dropdown shows files and directories containing "s"
  - Type `@internal/cli` → dropdown narrows to files/dirs in that path
  - Press Down/Up → navigate suggestions
  - Press Tab → selected path replaces `@internal/cli` with `@internal/cli/repl/repl.go`
  - Press Enter on a suggestion → same replacement behavior
  - Press Esc → dropdown closes
  - Type `fix @main.go and @rea` → second `@` triggers new suggestions
  - `/help` still shows command suggestions (no regression)
  - After `/clear`, typing `@` still works (no stale state)

---

## Implementation order

1. **`filesearch/filesearch.go` + tests** — standalone, no dependencies on UI
2. **Generalize `SuggestionModel`** — refactor types, update existing callers
3. **`extractAtToken` helper + tests** — pure function, easy to test
4. **Wire into `repl.go` and `handlers.go`** — integration
5. **End-to-end manual testing**

---

## Open questions

1. **Should the `@path` token be stripped before sending to the LLM, or sent as-is?**
   If sent as-is, the LLM sees `@src/main.go` in the message. This is fine — the LLM
   can interpret it as a file reference. Alternatively, we could prepend the file
   contents automatically. **Current plan**: send as-is (the `@` prefix is just a UI
   convenience for typing paths). File content injection can be a follow-up feature.
