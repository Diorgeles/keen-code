# Keen vs OpenCode Benchmark Plan

## Goal

Build an initial black-box benchmark that compares Keen against OpenCode on the same target repository, using the public CLI for both tools:

- Keen: `keen run`
- OpenCode: `opencode run`

The first benchmark only executes tasks and saves normalized outputs. Usage and cost will be checked manually in the OpenCode dashboard using session IDs.

## Requirements

- Benchmark target repository must be configurable.
- Benchmark harness lives under `bench/` in the `keen-code` repository.
- Use `keen` and `opencode` from `PATH`.
- Create worktrees from the target repository's `main` branch.
- Use two separate worktrees for the benchmark:
  - one for Keen
  - one for OpenCode
- Each run is created under `bench/<run_id>/`.
- Worktrees are created under `bench/<run_id>/worktrees/`.
- Fail if `bench/<run_id>/` already exists.
- Delete worktrees when the benchmark finishes.
- Run tasks sequentially.
- Generate tasks before running the benchmark, based on the target repository.
- Task generation is not part of the benchmark script.
- The benchmark script assumes the tasks file already exists.
- For now, generate 5 read-only multi-turn tasks.
- Each task should contain 5 to 10 user turns.
- Task definitions will live in `bench/tasks.json`.
- Each task produces one combined JSONL output file containing both Keen and OpenCode turn results.
- Do not create separate output files for Keen and OpenCode.
- Do not create separate raw output files per tool. If raw stdout/stderr is needed for debugging, embed it in the JSONL record for that tool turn.
- Multi-turn state must be preserved by passing the session ID returned by each tool into later turns.
- No permission prompt should be required.
- If either tool requests permission, the benchmark should fail that task/run.
- Print all distinct Keen and OpenCode session IDs at the end.
- OpenCode session IDs are especially important because the usage dashboard is checked by session ID.

## CLI Configuration

### Keen

Keen uses the local Keen config for provider/model selection.

Command shape:

```bash
keen run --format json "prompt"
keen run --format json --session "$KEEN_SESSION_ID" "next prompt"
```

Keen does not currently have a run id or title argument. The benchmark run id will be recorded only in the benchmark output metadata for Keen.

### OpenCode

OpenCode should use:

```bash
opencode run --format json --thinking --model opencode-go/kimi-k2.6 --title "$TASK_RUN_ID" "prompt"
opencode run --format json --thinking --model opencode-go/kimi-k2.6 --session "$OPENCODE_SESSION_ID" "next prompt"
```

`--thinking` is supported by OpenCode. In non-interactive mode it defaults to false, so the benchmark must pass it explicitly.

OpenCode emits newline-delimited JSON events, not a single JSON object. The harness must parse the stream and normalize it into one turn result.

## Directory Layout

```text
bench/
  benchmark-plan.md
  run.sh
  run.go
  tasks.json
  usage.csv
  <run_id>/
    tasks.json
    results/
      summary.json
      sessions.txt
      task-01.jsonl
      task-02.jsonl
      task-03.jsonl
      task-04.jsonl
      task-05.jsonl
    worktrees/
      keen/
      opencode/
```

`bench/<run_id>/worktrees/` should be removed after the benchmark finishes, even if a task fails. `bench/<run_id>/tasks.json` and `bench/<run_id>/results/` should remain. `bench/usage.csv` stores the latest pulled OpenCode usage export and is replaced independently of benchmark runs.

## Command Shape

Runner interface:

```bash
go run bench/run.go --repo /path/to/target/repo --run-id bench-20260512-001
```

Optional flags that are useful but not required in the first version:

```bash
go run bench/run.go \
  --repo /path/to/target/repo \
  --run-id bench-20260512-001 \
  --tasks bench/tasks.json \
  --opencode-model opencode-go/kimi-k2.6
```

Defaults:

- `--tasks`: `bench/tasks.json`
- `--opencode-model`: `opencode-go/kimi-k2.6`

## Task Generation

Tasks should be generated before the benchmark run, but they must be based on the repository being benchmarked. This is a separate preparation step, not part of the runner.

The intended flow is:

1. Inspect the target repo.
2. Identify important read-only investigation areas, such as CLI entry points, configuration flow, session handling, tool registration, tests, or package boundaries.
3. Generate 5 fixed multi-turn tasks for that specific repo.
4. Save them to `bench/tasks.json`.
5. Run the benchmark using that generated `bench/tasks.json`.

Task generation should happen once per benchmark target/revision. After generation, both Keen and OpenCode must receive the exact same saved tasks.

The benchmark script must not generate or modify tasks. It should only validate that the configured tasks file exists and has the expected schema.

The generated tasks should be preserved with the benchmark results so the run is reproducible.

## Task File

`bench/tasks.json` should contain fixed benchmark tasks generated from the target repo.

Proposed schema:

```json
{
  "tasks": [
    {
      "id": "task-01",
      "title": "Map CLI command flow",
      "turns": [
        "Find the entry point for the CLI and summarize how commands are registered.",
        "Now identify where configuration is loaded and explain the data flow.",
        "Which files would need to change to add a new read-only command?"
      ]
    }
  ]
}
```

Rules for tasks:

- Read-only only.
- Based on the target repository's actual structure and source files.
- Avoid requests to edit files, run formatters, commit changes, install dependencies, or change config.
- Prefer repository-understanding tasks that require navigating code.
- Make Keen and OpenCode answer the same prompts in the same order.
- Keep prompts deterministic and avoid asking for subjective recommendations unless the code context is enough.

## Worktree Setup

For a target repo `/path/to/repo` and run id `bench-20260512-001`:

1. Verify `/path/to/repo` is a git repo.
2. Verify `main` exists.
3. Fail if target repo has no `main` branch.
4. Fail if `bench/bench-20260512-001` exists.
5. Create:

```bash
git -C /path/to/repo worktree add /path/to/keen-code/bench/bench-20260512-001/worktrees/keen main
git -C /path/to/repo worktree add /path/to/keen-code/bench/bench-20260512-001/worktrees/opencode main
```

6. Record the checked-out commit for each worktree.

If the target repo already has `main` checked out and git refuses to add duplicate worktrees for the same branch, the implementation should create detached worktrees from `main`:

```bash
git -C /path/to/repo worktree add --detach <path> main
```

Detached worktrees are acceptable because benchmark tasks are read-only.

## Execution Flow

For each task:

1. Start with empty Keen and OpenCode session IDs.
2. For each turn:
   1. Run Keen in the Keen worktree.
   2. Parse Keen output.
   3. Store Keen session ID from the first turn and pass it to later turns.
   4. Run OpenCode in the OpenCode worktree.
   5. Parse OpenCode NDJSON.
   6. Store OpenCode session ID from the first event and pass it to later turns.
   7. Append normalized output for both tools to the same task JSONL file.
   8. Append normalized turn results.
   9. Check `git status --short` in both worktrees.
   10. If either worktree is dirty, mark the task as failed because the task was expected to be read-only.
3. Append one normalized JSON object per tool turn to `bench/<run_id>/results/<task_id>.jsonl`.

The per-task JSONL file should contain both tools in the same file, ordered by execution:

```text
{"run_id":"bench-20260512-001","task_id":"task-01","tool":"keen","turn":1,...}
{"run_id":"bench-20260512-001","task_id":"task-01","tool":"opencode","turn":1,...}
{"run_id":"bench-20260512-001","task_id":"task-01","tool":"keen","turn":2,...}
{"run_id":"bench-20260512-001","task_id":"task-01","tool":"opencode","turn":2,...}
```

Sequential ordering should be:

```text
task-01 turn-01 keen
task-01 turn-01 opencode
task-01 turn-02 keen
task-01 turn-02 opencode
...
task-02 turn-01 keen
task-02 turn-01 opencode
```

## Normalized Output

Each task output should be JSONL. Every line is one normalized tool-turn result.

```json
{
  "run_id": "bench-20260512-001",
  "task_id": "task-01",
  "task_title": "Map CLI command flow",
  "tool": "keen",
  "turn": 1,
  "prompt": "Find the entry point...",
  "target_repo": {
    "path": "/path/to/repo",
    "ref": "main",
    "commit": "abc123"
  },
  "command": ["keen", "run", "--format", "json", "..."],
  "cwd": "/path/to/worktree",
  "started_at": "2026-05-12T10:00:00Z",
  "finished_at": "2026-05-12T10:00:10Z",
  "duration_ms": 10000,
  "exit_code": 0,
  "session_id": "session-id",
  "text": "assistant response",
  "usage": {
    "input": 0,
    "output": 0,
    "reasoning": 0,
    "cache_read": 0,
    "cache_write": 0,
    "total": 0,
    "cost": 0
  },
  "raw_stdout": "{\"session_id\":\"...\"}\n",
  "raw_stderr": "",
  "dirty_status": ""
}
```

Usage fields may be zero or omitted if the tool does not provide them reliably. Dashboard usage remains the source of truth for cost comparison.

`raw_stdout` and `raw_stderr` may be omitted on successful turns if the normalized fields are enough. On parse errors, permission failures, command failures, or non-zero exits, include them in the JSONL record so the single task output file still contains everything needed to debug that turn.

## Keen Normalization

Expected Keen output is one JSON object.

Extract:

- `session_id`
- `text`
- usage fields when present

If Keen emits invalid JSON, mark the turn failed and save stdout/stderr.

## OpenCode Normalization

OpenCode emits NDJSON events.

Extract:

- `sessionID` from the first event that has it.
- Assistant text from events where `type == "text"` and `part.text` exists.
- Usage from all events where `type == "step_finish"` and `part.tokens` exists.
- Cost from all `step_finish.part.cost` values.

Usage mapping:

- `part.tokens.input` -> `input`
- `part.tokens.output` -> `output`
- `part.tokens.reasoning` -> `reasoning`
- `part.tokens.cache.read` -> `cache_read`
- `part.tokens.cache.write` -> `cache_write`
- `part.tokens.total` -> `total`

If OpenCode emits a permission event, fail:

- `type == "permission.asked"`

If OpenCode emits `session.error`, fail the turn and include the error in normalized output.

## Failure Handling

The benchmark should fail fast for setup errors:

- missing `keen`
- missing `opencode`
- missing `jq` if the shell implementation depends on it
- target repo is not a git repo
- target repo has no `main`
- result directory already exists
- worktree directory already exists
- invalid tasks file

During task execution:

- If one tool fails a turn, record the failure in the task output.
- Stop the current task.
- Continue to cleanup.
- The first implementation may stop the whole benchmark after a failed task to keep behavior simple.

Worktree cleanup should happen in all cases.

## Session ID Summary

At the end, print distinct session IDs in a copyable format:

```text
Benchmark run: bench-20260512-001

Keen sessions:
task-01  <keen-session-id>
task-02  <keen-session-id>

OpenCode sessions:
task-01  ses_...
task-02  ses_...

Results:
bench/bench-20260512-001/results
```

Also save the same information to:

```text
bench/<run_id>/results/sessions.txt
```

## Initial Implementation Choice

Use the Go runner in `bench/run.go`:

- It directly exercises the real CLI commands.
- It validates `tasks.json`.
- It parses Keen JSON and OpenCode NDJSON.
- It writes normalized JSONL results, summary metadata, and session IDs.

## Verification

Manual smoke test:

```bash
go run bench/run.go --repo /path/to/repo --run-id bench-smoke-001 --smoke
```

Verify:

- Worktrees are created from `main`.
- Worktrees are removed after completion.
- `bench/<run_id>/tasks.json` exists.
- `bench/<run_id>/results/task-*.jsonl` files exist.
- There are no separate Keen/OpenCode output files.
- Parse failures or command failures include raw stdout/stderr in the task JSONL record.
- `sessions.txt` contains all Keen and OpenCode session IDs.
- OpenCode session IDs match sessions visible in the OpenCode dashboard.
- Dirty worktree status is empty for both tools after every turn.

Because the benchmark uses real LLM calls, automated tests should focus on parser helpers if the implementation grows. The first shell version can be verified with one short smoke task before running all 5 tasks.
