# Contributing to Keen Code

Thank you for contributing. This project records human-AI interactions that shapes the codebase. When you work on an issue, you must preserve the prompts you send and the outputs the agent produces so the development history stays complete and transparent.

## Running Locally

This project is built with Go. To run it locally, ensure you have Go installed on your system.

1. Install Go dependencies:
   ```bash
   go mod download
   go mod tidy
   ```

2. Run the application:
   ```bash
   go run cmd/main.go
   ```

3. Run the tests to ensure everything is working:
   ```bash
   go test ./...
   ```

## Debugging

The application uses structured logging through `slog`. Control the log verbosity with the `KEEN_LOG_LEVEL` environment variable:

- `KEEN_LOG_LEVEL=debug` — verbose debug output
- `KEEN_LOG_LEVEL=info` — informational messages (default)
- `KEEN_LOG_LEVEL=warn` or `KEEN_LOG_LEVEL=warning` — warnings only
- `KEEN_LOG_LEVEL=error` — errors only

Logs are written to `~/.keen/logs/keen-code-<timestamp>.log`. For example, to run with debug logging:

```bash
KEEN_LOG_LEVEL=debug go run cmd/main.go
```

## Prerequisites

- You need to have access to at least one AI coding agent
- All PRs must be written by an AI agent
- All prompts must be written by a human
- Prompts and output docs are critical for the development history so they must be saved as outlined in this document

## Where to save task files

Save everything under `.ai-interactions/tasks/`.

## Folder per issue

Every issue gets its own folder under `.ai-interactions/tasks/<issue-name>/`.

Follow the naming convention for prompt and output files:

- `prompt-1_that-feature.md`
- `output-1_plan-for-that-feature.md`

## What to include

- **Prompts** — every message or instruction you send to the agent that was needed to implement the issue
- **Outputs** — every plan, design doc, code review, or task breakdown the agent returns

Do not edit outputs after the agent produces them. Save them as-is.

## Example

```
.ai-interactions/
└── tasks/
    ├── issue-5/
    │   └── prompt-1_issue-5-feature.md
    │   └── output-1_plan-for-issue-5-feature.md
    └── issue-42/
        ├── prompt-1_issue-42-feature.md
        ├── output-1_plan-for-issue-42-feature.md
        ├── prompt-2_issue-42-feature.md
        └── output-2_plan-for-issue-42-feature.md
```

## Workflow

1. Open or create a GitHub issue describing the bug or feature.
2. Work with the agent iteratively. Save each turn.
3. Place the final prompts and outputs in `.ai-interactions/tasks/` following the rules above.
4. Open a pull request that includes both the code changes and the task files.
