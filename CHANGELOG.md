# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.19.1] - 2026-05-23

### Changed
- Preserved MCP skill enablement preferences during server sync while removing stale generated skills for deleted servers.
- Simplified MCP tool display and capped skill descriptions in REPL list output.
- Used MCP server instructions for generated skill descriptions.
- Store logs in `~/.keen/logs` instead of `~/.keen-code/logs`.
- Refreshed documentation landing page styling and clarified skill activation persistence.

## [0.19.0] - 2026-05-21

### Added
- `web_fetch` tool to fetch URL content and convert HTML pages to Markdown for LLM consumption.
- MCP server support with configurable transports, authentication, connection management, and tool discovery.
- MCP tool-calling support through generated MCP skills and the `call_mcp_tool` tool.
- Documentation for MCP servers, skill-driven MCP integration, and OAuth-authenticated MCP servers.
- GitHub Pages documentation site powered by MkDocs Material.
- Suggested subcommands for `/mcp connect`, `/mcp status`, `/skills list`, `/skills enable`, and `/skills disable`.

### Changed
- Streamlined README intro section.
- Updated REPL mode glyphs and removed mode-change confirmation messages.
- MCP skills now enable or disable based on connection status while preserving generated skill files.
- Improved docs site styling, navigation, badges, fonts, and local preview support.

### Fixed
- Render markdown table row rules safely.
- Fixed broken or misleading documentation links and labels.

## [0.18.0] - 2026-05-16

### Added
- Plan and build modes for structured REPL interaction workflows.

## [0.17.0] - 2026-05-15

### Added
- Project-level tool allow lists for pre-approved permission checks.
- Anthropic prompt caching support and improved token usage tracking.
- Benchmark runner with updated benchmark documentation and demo assets.

### Changed
- Improved REPL markdown rendering with width-aware horizontal rules, wrapped tables, connected table borders, and outer table frames.
- Refined assistant formatting guidance to prefer semantic GitHub-flavored markdown.
- Updated CLI usage and permission system documentation.

## [0.16.3] - 2026-05-13

### Added
- Paginate `read_file` output and add line number prefixes.
- OpenCode usage scripts and restructured benchmark output with usage timestamp filtering.
- Refined system prompt exploration guidance for efficient tool use.

### Changed
- Restructured benchmark layout.

## [0.16.2] - 2026-05-12

### Added
- Toggle focus between input and viewport via Tab and mouse clicks.
- Route up/down keys based on focused region.
- Dim input chrome and prompt glyph when focus is in the viewport.

### Changed
- Merged PR #41: add basic benchmark.

## [0.16.1] - 2026-05-12

### Added
- `keen run` headless command for non-interactive task execution.
- `--provider` and `--model` flags to override LLM configuration in `keen run`.

## [0.16.0] - 2026-05-11

### Added
- Bundled workflow skills for common agent tasks.
- `/btw` side questions for asking context-aware questions without interrupting the main conversation.
- Documentation for `/btw` side questions.

### Changed
- Constrained REPL suggestion list height to fit the available viewport.

## [0.15.3] - 2026-05-08

### Changed
- Moved release guide from README into a local skill at `.agents/skills/release/`.

### Added
- Documentation for turn memory KV cache and token cost analysis.

## [0.15.2] - 2026-05-08

### Changed
- Avoid repeated file suggestion cache rebuilds in large repositories.

## [0.15.1] - 2026-05-07

### Added
- Horizontal padding for submitted user input blocks.

### Changed
- Use `git ls-files` for faster cached file suggestions.
- Improved REPL status display and usage documentation.

### Fixed
- REPL submitted input wrapping test expectation.

## [0.15.0] - 2026-05-07

### Added
- MiniMax provider support for M2.7 and M2.5 via the Anthropic-compatible API.

## [0.14.0] - 2026-05-06

### Added
- OpenCode Go provider support routed through OpenAI-compatible or Anthropic clients, including provider registry entries and thinking parameter handling
- REPL session IDs propagated through LLM stream calls and attached as hyphenless OpenCode Go request headers (Anthropic and OpenAI-compatible)
- Architecture and system documentation covering AI providers, permission system, session management, skills system, tools, and turn memory

### Changed
- Preserve Anthropic thinking blocks across tool continuations
- Simplified LLM test coverage by removing redundant provider, thinking effort, and system prompt tests

## [0.13.0] - 2026-05-06

### Added
- Agent skills discovery, slash-command invocation, frontmatter validation, and argument substitution
- Bundled commit skill embedded in the binary and extracted to the user skills directory at runtime
- Additional model registry entries

### Changed
- Reset LLM provider state when starting new REPL sessions

## [0.12.2] - 2026-05-03

### Added
- Permission option to ask what the agent should do instead, interrupting the current stream while preserving partial state

## [0.12.1] - 2026-05-02

### Changed
- Improved REPL loading status display with elapsed time

### Fixed
- Indent wrapped submitted user input lines in the REPL transcript

## [0.12.0] - 2026-05-01

### Added
- In-app text selection for REPL output and input, with copy support for active selections via `Ctrl+C` or forwarded `Cmd+C`

## [0.11.2] - 2026-04-30

### Changed
- Split REPL command handling into dedicated command handler components

## [0.11.1] - 2026-04-30

### Added
- Shaded background block for the echoed user input that grows with line count and resizes responsively with the viewport

### Changed
- Refreshed the prompt glyph from `>` to `▶` across the textarea, echoed input, model selection inputs, session picker, and permission card cursor

## [0.11.0] - 2026-04-30

### Added
- Retry support across streaming clients for improved LLM reliability
- Pending tool state preservation across turns
- Pending state recovery for all LLM clients

## [0.10.0] - 2026-04-27

### Added
- ChatGPT OAuth support for the Codex provider

## [0.9.0] - 2026-04-27

## [0.8.0] - 2026-04-25

### Added
- Retry transient LLM stream errors with backoff for OpenAI-compatible clients

## [0.7.0] - 2026-04-25

### Added
- Startup update checker that notifies REPL users when a newer version is available

## [0.6.1] - 2026-04-25

### Fixed
- Improved assistant markdown colors on light terminals while preserving inline code color and syntax highlighting

## [0.6.0] - 2025-07-18

### Added
- Z.ai (GLM) as an OpenAI-compatible provider (#3)
- File suggestion with `@` prefix in the input textarea
- `filesearch` package with gitignore-aware file indexing and glob-escaped query matching

### Fixed
- Materialize partial message and TurnMemory on LLM error

### Changed
- Bumped Anthropic max output tokens

## [0.5.0] - 2026-04-23

### Added
- Configurable base URL per provider
- CONTRIBUTING.md for contributors
- Public-facing ROADMAP.md
- Turn-memory documentation
- Project tour, issue templates, and pull request template
- Demo GIF in README

### Changed
- Wrapped REPL diff output in a viewport
- Updated LLM configuration
- Refreshed the demo GIF rendering with Monaspace Argon NF

## [0.4.1] - 2026-04-22

### Fixed
- Retain tool memory on stream interrupt

## [0.4.0] - 2026-04-22

### Added
- Provider-backed context status replacing the local word-count heuristic
- Token usage events emitted from all provider clients (OpenAI Responses, Anthropic, Genkit/Google AI, DeepSeek/Moonshot)
- Cache-aware token accounting for Anthropic (includes cache creation and read tokens)
- Anthropic adaptive effort display in the status bar
- `N/A` display when context window is unknown instead of a misleading percentage

### Changed
- Context status now reports actual provider-counted token usage against the model context window
- Compaction suggestions are grounded in real tokenization rather than local estimates
- `/clear` and `/new` reset context metrics for new sessions
- Updated provider registry with new models and context windows

### Removed
- Local word-count token estimation helpers (`estimateTokensFromWordCount`, `countWords`, `estimateToolDefinitionTokens`, `buildConversationForEstimation`)

## [0.3.0] - 2026-04-21

### Added
- Configurable thinking effort selection in model setup and via the `/thinking` runtime command
- Direct Anthropic SDK client with expanded Anthropic streaming and tool-loop test coverage
- Refactored REPL helper packages for app state, output, permissions, theme, tooling, widgets, and streaming

### Changed
- Thread thinking effort configuration through OpenAI Responses, Anthropic, and Genkit clients
- Added Anthropic prompt caching support in the REPL streaming path
- Refreshed phase-5 design notes and removed stale scratch artifacts

## [0.2.3] - 2026-04-19

### Added
- Structured `TurnMemory` system to replace in-band XML tags for tracking durable tool outcomes
- `turnMemoryAccumulator` in REPL to automatically capture file changes and failed bash commands

### Changed
- Refactored LLM provider interface to deterministically append tool memory metadata
- Simplified system and compaction prompts by removing manual memory tag instructions
- Improved session persistence to support structured tool outcomes

### Removed
- Legacy `<keen_memory>` tag parsing and stripping logic

## [0.2.2] - 2026-04-17

### Added
- Hidden `keen_memory` blocks to preserve durable tool outcomes across turns without showing them in the REPL transcript

### Changed
- Session picker now constrains its visible list to the current viewport height

### Fixed
- Only extract trailing dedicated `keen_memory` blocks for logging and compaction-aware handling

## [0.2.1] - 2026-04-16

### Changed
- Simplified REPL context status display and metadata emphasis

## [0.2.0] - 2026-04-16

### Added
- Conversation session management with transcript persistence
- `/sessions` command to list recent sessions with metadata
- `/resume` command with interactive picker to restore conversations
- `/compact` command to summarize conversation history via LLM
- Event-sourced storage (session_started, user_message, assistant_turn, compaction_applied)
- Store tool outputs, bash results, and file diffs in transcript for full replay

## [0.1.7] - 2026-03-24

### Added
- REPL context status indicator with progress bar and percentage based on model context window
- Slash command autosuggestion dropdown for `/help`, `/model`, and `/exit`

### Changed
- Consolidated REPL styling for context status and suggestion UI

## [0.1.6] - 2026-03-22

### Changed
- Improved spinner UX with smoother feedback during LLM streaming
- Refined tool descriptions for better LLM tool selection
- Improved Genkit streaming reliability

## [0.1.5] - 2026-03-22

### Added
- Install script for easier local setup
- npm wrapper package documentation

## [0.1.4] - 2026-03-22

### Changed
- Switched npm publishing to trusted publishing (removes need for legacy token)

## [0.1.3] - 2026-03-22

### Fixed
- Release pipeline corrections from v0.1.2

## [0.1.2] - 2026-03-22

### Fixed
- Improved release flow and startup behavior

## [0.1.1] - 2026-03-22

### Fixed
- npm wrapper publish and install flow

## [0.1.0] - 2026-03-22

### Added
- Interactive REPL powered by Bubble Tea with streaming LLM responses
- Multi-turn tool calling with Genkit integration
- `read_file` tool with interactive permission system
- `write_file` tool with inline diff rendering
- `edit_file` tool with inline diff rendering
- `bash` tool with permission gating
- `glob` tool for file pattern searching
- `grep` tool for content search
- File guard with `.gitignore` awareness and permission levels (granted/pending/denied)
- Inline permission card UI (replaces full-screen modal)
- Dynamic system prompt generation with project context
- OpenAI-compatible client supporting DeepSeek (including reasoning/chain-of-thought)
- MoonshotAI provider via OpenAI-compatible client
- Dedicated OpenAI Responses API client
- GoReleaser config for cross-platform binary distribution
- npm wrapper package for installation via `npm install -g keen-code`

[Unreleased]: https://github.com/mochow13/keen-code/compare/v0.19.1...HEAD
[0.19.1]: https://github.com/mochow13/keen-code/compare/v0.19.0...v0.19.1
[0.19.0]: https://github.com/mochow13/keen-code/compare/v0.18.0...v0.19.0
[0.18.0]: https://github.com/mochow13/keen-code/compare/v0.17.0...v0.18.0
[0.17.0]: https://github.com/mochow13/keen-code/compare/v0.16.3...v0.17.0
[0.16.3]: https://github.com/mochow13/keen-code/compare/v0.16.2...v0.16.3
[0.16.2]: https://github.com/mochow13/keen-code/compare/v0.16.1...v0.16.2
[0.16.1]: https://github.com/mochow13/keen-code/compare/v0.16.0...v0.16.1
[0.16.0]: https://github.com/mochow13/keen-code/compare/v0.15.3...v0.16.0
[0.15.3]: https://github.com/mochow13/keen-code/compare/v0.15.2...v0.15.3
[0.15.2]: https://github.com/mochow13/keen-code/compare/v0.15.1...v0.15.2
[0.15.1]: https://github.com/mochow13/keen-code/compare/v0.15.0...v0.15.1
[0.15.0]: https://github.com/mochow13/keen-code/compare/v0.14.0...v0.15.0
[0.14.0]: https://github.com/mochow13/keen-code/compare/v0.13.0...v0.14.0
[0.13.0]: https://github.com/mochow13/keen-code/compare/v0.12.2...v0.13.0
[0.12.2]: https://github.com/mochow13/keen-code/compare/v0.12.1...v0.12.2
[0.12.1]: https://github.com/mochow13/keen-code/compare/v0.12.0...v0.12.1
[0.12.0]: https://github.com/mochow13/keen-code/compare/v0.11.2...v0.12.0
[0.11.2]: https://github.com/mochow13/keen-code/compare/v0.11.1...v0.11.2
[0.11.1]: https://github.com/mochow13/keen-code/compare/v0.11.0...v0.11.1
[0.11.0]: https://github.com/mochow13/keen-code/compare/v0.10.0...v0.11.0
[0.10.0]: https://github.com/mochow13/keen-code/compare/v0.9.0...v0.10.0
[0.9.0]: https://github.com/mochow13/keen-code/compare/v0.8.0...v0.9.0
[0.8.0]: https://github.com/mochow13/keen-code/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/mochow13/keen-code/compare/v0.6.1...v0.7.0
[0.6.1]: https://github.com/mochow13/keen-code/compare/v0.6.0...v0.6.1
[0.6.0]: https://github.com/mochow13/keen-code/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/mochow13/keen-code/compare/v0.4.1...v0.5.0
[0.4.1]: https://github.com/mochow13/keen-code/compare/v0.4.0...v0.4.1
[0.4.0]: https://github.com/mochow13/keen-code/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/mochow13/keen-code/compare/v0.2.3...v0.3.0
[0.2.3]: https://github.com/mochow13/keen-code/compare/v0.2.2...v0.2.3
[0.2.2]: https://github.com/mochow13/keen-code/compare/v0.2.1...v0.2.2
[0.2.1]: https://github.com/mochow13/keen-code/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/mochow13/keen-code/compare/v0.1.7...v0.2.0
[0.1.7]: https://github.com/mochow13/keen-code/compare/v0.1.6...v0.1.7
[0.1.6]: https://github.com/mochow13/keen-code/compare/v0.1.5...v0.1.6
[0.1.5]: https://github.com/mochow13/keen-code/compare/v0.1.4...v0.1.5
[0.1.4]: https://github.com/mochow13/keen-code/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/mochow13/keen-code/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/mochow13/keen-code/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/mochow13/keen-code/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/mochow13/keen-code/releases/tag/v0.1.0
