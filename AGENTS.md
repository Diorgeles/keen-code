## Keen Code
CLI-based coding agent powered by AI using Firebase Genkit for LLM interactions.

## Architecture
- **internal/tools** - LLM tools (read_file, write_file, edit_file, glob, grep, bash)
- **internal/filesystem** - Guard for safe file access
- **internal/cli/repl** - Interactive REPL UI
- **internal/llm** - Genkit-based LLM client

## Permission System
Guard checks paths before filesystem operations:
- `PermissionGranted` - Allowed (working directory)
- `PermissionPending` - User approval required (outside working dir)
- `PermissionDenied` - Blocked (system paths, .gitignore files)

## Releasing
1. Bump versions together:
   - `cmd/main.go`
   - `npm/package.json`
2. Update `CHANGELOG.md`:
   - Move `[Unreleased]` entries under a new `[X.Y.Z] - YYYY-MM-DD` heading
   - Add a new empty `[Unreleased]` section at the top
   - Update the comparison links at the bottom of the file
3. Run the tests:
   - `go test ./...`
4. Verify the npm wrapper package:
   - `cd npm && npm pack --dry-run`
5. Commit the version bump.
6. Create and push a tag in the form `vX.Y.Z`.
6. Push `main` and the tag to GitHub.
7. GitHub Actions will:
   - run GoReleaser for the tagged release
   - publish the npm package from `npm/` after the release job succeeds
8. The Git tag must match `npm/package.json` version exactly.

## Important Guidelines
- Minimal comments only when strictly necessary
- Test critical paths, not aiming for 100% coverage
- Always run the tests after each change
- Always run `go mod tidy` after each change
- Always run `gofmt` on modified Go files before committing
- Commit messages should be concise and focus on the key changes with bullet points
- Commit messages should follow the `feat(category): description` format
- Always check both tracked and untracked files for creating the commit message
- Never add co-authors or made-with AI tags to the commit message

## Behavioral Guidelines

Behavioral guidelines to reduce common LLM coding mistakes. Merge with project-specific instructions as needed.

Tradeoff: These guidelines bias toward caution over speed. For trivial tasks, use judgment.

### 1. Think Before Coding
Don't assume. Don't hide confusion. Surface tradeoffs.

Before implementing:

- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them - don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

### 2. Simplicity First
Minimum code that solves the problem. Nothing speculative.

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.

Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

### 3. Surgical Changes
Touch only what you must. Clean up only your own mess.

When editing existing code:

- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it - don't delete it.

When your changes create orphans:

- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.
- The test: Every changed line should trace directly to the user's request.

### 4. Goal-Driven Execution
Define success criteria. Loop until verified.

Transform tasks into verifiable goals:

- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"
For multi-step tasks, state a brief plan:

```
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
```

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.

These guidelines are working if: fewer unnecessary changes in diffs, fewer rewrites due to overcomplication, and clarifying questions come before implementation rather than after mistakes.