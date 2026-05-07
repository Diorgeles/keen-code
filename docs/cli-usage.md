# CLI Usage

Keen Code provides slash commands (prefixed with `/`) for controlling the agent. Type `/` and press `Tab` to see command suggestions.

## Command Reference

| Command | Description |
|---------|-------------|
| `/help` | Show available commands |
| `/model` | Change provider or model |
| `/thinking <effort>` | Set thinking effort for the current model |
| `/show-thinking [on\|off]` | Show, hide, or inspect thinking token display |
| `/sessions` or `/resume` | Open the saved-session picker for the current directory |
| `/skills [list\|reload\|<name> enable\|disable]` | List, reload, enable, or disable skills |
| `/<skill-name> [args...]` | Activate an enabled skill |
| `/compact [prompt]` | Compact conversation context; provide a prompt to guide what to retain |
| `/clear` or `/new` | Clear the current session and start a new one |
| `/logout` | Sign out of the current OAuth provider |
| `/exit` | Quit Keen Code |

## `/help`

Displays a formatted list of built-in slash commands with descriptions.

```text
/help
```

## `/model`

Opens an interactive model selector to change the AI provider and model. Use this command when you want to:

- Switch between providers (Anthropic, OpenAI, Google AI, etc.)
- Change to a different model within the same provider
- Configure API keys for a provider
- Configure an optional custom base URL for API-key providers that support it
- Sign in to OAuth providers such as Codex (ChatGPT OAuth)

```text
/model
```

Navigation:

| Key | Action |
|-----|--------|
| `↑` / `↓` or `k` / `j` | Move through provider, model, or thinking-effort lists |
| `Enter` | Select the highlighted item or confirm input |
| `Esc` | Cancel model selection |

API keys are masked while typed. If an API key already exists for the provider, press `Enter` on an empty API-key prompt to keep it.

## `/thinking <effort>`

Sets the thinking effort level for the current model. The accepted values come from the selected model, not just the provider. If the current model does not support configurable thinking, Keen shows an error.

Common supported efforts include:

| Provider/model family | Efforts |
|-----------------------|---------|
| Anthropic Claude Opus/Sonnet | `low`, `medium`, `high`, `max` |
| OpenAI / Codex GPT models | `none`, `low`, `medium`, `high`, `xhigh` |
| Google AI Gemini Pro | `low`, `medium`, `high` |
| Google AI Gemini Flash / Flash-Lite | `minimal`, `low`, `medium`, `high` |
| DeepSeek V4 | `off`, `high`, `max` |
| Z.ai GLM | `enabled`, `disabled` |
| OpenCode Go selected models | varies by model, including `enabled`/`disabled` or `off`/`high`/`max` |

```text
/thinking high
```

If no effort is specified, or the effort is invalid for the current model, Keen prints the valid options.

## `/show-thinking [on|off]`

Controls whether thinking/reasoning tokens are displayed in the output.

```text
/show-thinking on   # Show thinking tokens
/show-thinking off  # Hide thinking tokens
/show-thinking      # Show the current setting
```

The setting is saved in Keen's global config.

## `/sessions` / `/resume`

Opens the saved-session picker for the current working directory. Sessions are stored in `~/.keen/sessions/` and namespaced by directory.

```text
/sessions
/resume
```

Navigation:

| Key | Action |
|-----|--------|
| `↑` / `↓` or `k` / `j` | Move through sessions |
| `Enter` | Resume the highlighted session |
| `Esc` | Close the picker |

If there are no saved sessions for the directory, Keen prints a message instead of opening the picker.

## `/skills [list|reload|<name> enable|disable]`

Manages skill plugins. Without arguments, lists all available skills.

```text
/skills
/skills list              # List all skills
/skills reload            # Reload skills from disk
/skills <name> enable     # Enable a skill
/skills <name> disable    # Disable a skill
```

Skills are discovered from:

- `<working-dir>/.agents/skills/`
- `<working-dir>/.keen/skills/`
- `~/.agents/skills/`
- `~/.keen/skills/`
- Bundled skills embedded in the binary

Skills are enabled by default unless disabled in the skills config.

## `/<skill-name> [args...]`

Enabled skills can be activated like slash commands:

```text
/review
/review docs/cli-usage.md
```

When a skill is activated, Keen injects that skill's `SKILL.md` body into the conversation as an activation message. Arguments are substituted into the skill body using `$ARGUMENTS`, `$1`, `$2`, etc. See [`docs/skills-system.md`](skills-system.md) for the skill file format.

Skill names participate in slash-command autocomplete alongside built-in commands.

## `/compact [prompt]`

Triggers context compaction to reduce conversation history while preserving important information. Optionally includes a prompt to guide what to retain.

```text
/compact
/compact Focus on the recent API changes
```

Compaction requires an initialized LLM client. While compaction is running, press `Esc` to cancel it.

Compaction is useful when context is filling up and you want to summarize the conversation before continuing. Keen may also suggest `Try /compact` in the status line when context usage is high.

## `/clear` / `/new`

Clears the current session and starts fresh. This clears:

- Conversation history
- Turn memory
- Session state
- Session-scoped permission approvals
- Current input history state

```text
/clear
/new
```

## `/logout`

Signs out of the current OAuth provider. This is useful for providers that use browser-based OAuth, currently Codex (ChatGPT OAuth). It removes stored OAuth credentials and clears the active LLM client.

```text
/logout
```

If the current provider does not use OAuth, Keen shows an error.

## `/exit`

Quits Keen Code.

```text
/exit
```

## File Mentions

Use `@` mentions to reference files in prompts. Type `@` followed by part of a filename or path, then use autocomplete to insert a match.

```text
Review @docs/cli-usage.md
```

File mention autocomplete:

- Starts when `@<token>` appears at the beginning of input or after a space
- Searches for relative paths whose names contain the token
- Respects Keen's filesystem guard and ignored/blocked paths
- Inserts the selected path as `@path ` with a trailing space

## Autocomplete

Type a partial slash command and press `Tab` to see suggestions:

```text
/mo<Tab> → /model
/th<Tab> → /thinking
```

Autocomplete supports:

- Built-in slash commands
- Enabled skills as slash commands
- File mentions with `@<token>`

When suggestions are visible:

| Key | Action |
|-----|--------|
| `↑` / `↓` | Move through suggestions |
| `Tab` | Accept the highlighted suggestion |
| `Enter` | Accept the highlighted suggestion |
| `Esc` | Hide suggestions when no response is streaming |

Slash-command autocomplete matches command prefixes case-insensitively. Command execution itself uses the command names shown above.

## Keyboard Shortcuts

These shortcuts are available in the REPL:

| Shortcut | Action |
|----------|--------|
| `Enter` | Submit the current input |
| `Shift+Enter` | Insert a newline in the input box |
| `Ctrl+C` | Clear non-empty input; quit when input is empty; copy selected text when a selection is active |
| `Ctrl+D` | Clear non-empty input; quit when input is empty |
| `Esc` | Interrupt an active response; cancel compaction/model/session pickers; hide suggestions; clear selections |
| `Tab` | Show or accept autocomplete suggestions |
| `↑` / `↓` | Navigate input history when the cursor is at the top/bottom of input; otherwise move the cursor; scroll output when history cannot move |
| `PageUp` / `PageDown` | Scroll output by a half page |
| `Home` / `End` | Jump to top/bottom of output |

## Mouse Selection

Keen supports mouse selection in the output and input areas:

| Action | Result |
|--------|--------|
| Click and drag | Select text |
| Double-click | Select word |
| Triple-click | Select line |
| `Ctrl+C` / `Cmd+C` with a selection | Copy selected text |
| `Esc` | Clear selection |
