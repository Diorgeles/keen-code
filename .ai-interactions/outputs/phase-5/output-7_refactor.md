# REPL Refactor Plan

## Goal

Reduce the `internal/cli/repl` package's cognitive load by reorganizing it around responsibilities instead of letting `repl.go`, `handlers.go`, and `streaming.go` remain the main catch-all files.

## Target Boundaries

Use these responsibility buckets as the target architecture:

- `app`: Bubble Tea model, top-level `Update`, bootstrapping, layout
- `input`: slash commands, key handling, submit flow, suggestions
- `stream`: active-turn transcript state, stream event application, stream rendering
- `runtime`: app state, session persistence, tool wiring, turn memory
- `view`: passive rendering helpers and styles
- `setup`: model-selection flow
- `sessionui`: session picker and replay UI

## Plan

### 1. Stabilize boundaries before moving files

Treat the target architecture as the migration rule for every existing function.

This is mainly to stop the refactor from turning into random file shuffling.

Current files driving the need for this:

- `internal/cli/repl/repl.go`
- `internal/cli/repl/handlers.go`
- `internal/cli/repl/streaming.go`

### 2. Extract `stream` first

This is the largest cohesive unit and the safest first split.

Create:

- `internal/cli/repl/stream/handler.go`
- `internal/cli/repl/stream/segments.go`
- `internal/cli/repl/stream/render.go`
- `internal/cli/repl/stream/permission.go`

Move from current `streaming.go`:

- `StreamHandler`
- `streamSegment` types
- stream mutation methods
- transcript rendering
- permission card rendering
- diff rendering

Why first:

- it is large
- it already has a strong internal concept boundary
- it reduces pressure on the rest of the package immediately

### 3. Split input handling from model orchestration

Move command submission and keyboard handling out of the root model.

Create:

- `internal/cli/repl/input/submit.go`
- `internal/cli/repl/input/keys.go`
- keep command/suggestion code with this area

Move here:

- slash command handling from `repl.go`
- submit flow from `handleEnterKey`
- key handling from `handlers.go`
- command metadata from `commands.go`
- suggestion logic from `suggestion.go`

Goal:

- a single place that explains user input behavior
- the root model dispatches rather than implementing every interaction inline

### 4. Pull runtime/state out of UI-heavy files

Move non-visual state management into `runtime`.

Create:

- `internal/cli/repl/runtime/app_state.go`
- `internal/cli/repl/runtime/sessions.go`
- `internal/cli/repl/runtime/turn_memory.go`
- `internal/cli/repl/runtime/tools.go`
- `internal/cli/repl/runtime/async.go`

Move here:

- `state.go`
- `session_state.go`
- `turn_memory.go`
- `tool_registry.go`
- permission requester
- diff emitter plumbing

Goal:

- persistent/session/tool concerns are separated from Bubble Tea presentation code

### 5. Reduce `repl.go` to shell-only responsibilities

After the above extractions, leave only:

- `RunREPL`
- root model type
- `Init`
- top-level `Update` dispatch
- top-level `View`
- minimal bootstrap helpers

Anything more detailed should be delegated into subpackages.

Target outcome:

- `repl.go` becomes composition and wiring
- it stops being the main home for business logic

### 6. Move passive view helpers last

Once behavior is stable, move rendering-only helpers into `view`.

Create:

- `internal/cli/repl/view/output.go`
- `internal/cli/repl/view/styles.go`
- `internal/cli/repl/view/markdown.go`
- `internal/cli/repl/view/context_status.go`
- `internal/cli/repl/view/compaction_status.go`

Move here:

- `output.go`
- `styles.go`
- `markdown.go`
- `context_status.go`
- `compaction_status.go`

Do this late so rendering code is not moving while behavior is still being reshaped.

### 7. Keep already-cohesive flows mostly intact

These are lower-priority moves:

- `model_selection.go` → `setup`
- `session_picker.go` and `session_replay.go` → `sessionui`

They are already more focused than the main REPL files, so they should not be disturbed first.

## Execution Order

1. Extract `stream`
2. Extract `input`
3. Extract `runtime`
4. Thin `repl.go`
5. Move passive `view` helpers
6. Clean up APIs, naming, and imports

## Guardrails

- Keep behavior identical after each step.
- Run `go mod tidy` and `go test ./...` after each extraction.
- Prefer moving code first, then renaming symbols once tests pass.
- Do not redesign session format or stream segment schema during this refactor.
- Keep `view` passive.
- Keep `runtime` UI-free.
- Avoid circular dependencies between `app`, `stream`, `input`, and `runtime`.

## Suggested Target Tree

```text
internal/cli/repl/
  repl.go

  app/
    model.go
    bootstrap.go
    update.go
    view.go

  input/
    commands.go
    submit.go
    keys.go
    suggestion.go

  stream/
    handler.go
    segments.go
    render.go
    permission.go

  runtime/
    app_state.go
    sessions.go
    tools.go
    turn_memory.go
    async.go

  sessionui/
    picker.go
    replay.go

  setup/
    model_selection.go

  view/
    output.go
    styles.go
    markdown.go
    context_status.go
    compaction_status.go
```

## Recommendation

Start with `stream`.

It has the best ratio of:

- high package complexity reduction
- low behavioral risk
- clear ownership boundary

After that, split `input`, then `runtime`, and only then thin `repl.go`.
