# Keen Code REPL UI Enhancement Ideas

Date: 2026-06-01

Concrete UI/UX improvement ideas for `internal/cli/repl/`, ordered from launch through interaction.

## 1. Startup Screen

Tighten the banner; expand the orientation. The ASCII logo is nice but the tips box is the more useful real estate.

- ✅ Show **last session summary** ("Resuming? Last session: 'fix auth tests' • 2h ago • `/resume`") so the user can continue without remembering the command.
- ✅ Replace static tips with a **rotating "Did you know"** line — one tip per launch keeps it fresh and surfaces lesser-known features (`@`-mention, Shift+Tab, `/adversary`).

## 2. Input Area

- ✅ **Semantic prompt indicator**: change the `▶` color/shape based on mode (e.g. `▶` build, `◆` plan, `⚠` waiting on permission). Mode is currently only in the footer chip, easy to miss.

## 4. Streaming Feedback (biggest UX win)

✅ The Harry Potter spell names are charming but uninformative. Replace with **structured activity status**:

```
⠋ Reading internal/cli/repl/repl.go (1.2k lines)
⠋ Running: go test ./internal/llm/... (12s)
⠋ Thinking (243 tokens)
```

**Edit**: Replaced with Keen Code usage tips.

## 5. Tool Call Rendering

- **Collapse-by-default for long bash output**: show first 5 + last 5 lines, with `[+18 lines · press e to expand]`. The current 30-line hard cap is both too much (clutters) and too little (loses context).
- **Group consecutive tool calls** under a single fold: `▼ 4 file reads` expanding to the list. Long agentic turns currently scroll forever.
- **Diff summary line**: above each diff, show `edit_file: foo.go (+12 −3)` so a quick scroll gives you a changelog.

## 6. Footer / Status

- Add a **token-cost meter** ("$0.12 this turn / $1.40 session") — even approximate. Cost awareness is genuinely useful and currently invisible.
- **Keybind hint that fades**: show `Tab focus · Shift+Tab mode · Ctrl+R search` in the footer for the first ~30 seconds of idle then fade out. Surfaces shortcuts without permanent clutter.

## 7. Scrollback

Add **`Ctrl+R` reverse search across the current session's viewport** (or `/find`). Right now scrolling is the only way to locate a tool result you saw 200 lines ago. Even a basic incremental highlight is a big win.

## 8. Permissions

- **Risk badge**: colored chip on the permission card — `LOW` (read), `MED` (write in cwd), `HIGH` (write outside cwd, network, dangerous bash). Faster mental triage than reading the path.
- **"Allow this pattern this session"**: e.g. allow `read_file` for anything under `internal/`. Currently you re-approve per-path or session-wide.

## 9. Theming

- `/theme list`, `/theme set <name>` — ship 3-4 named themes (default, high-contrast, mono, solarized). Auto-detect is fine as default but a manual override is overdue.
- **Light terminal palette is washed-out**: bump saturation; many grays disappear on bright backgrounds.

## 10. Help

- **Per-command help**: `/help compact` shows just that command's doc + examples. The current monolithic table is hard to scan.
- **Discoverable from `/`**: when the suggestion popup is open, show the command's description in a side panel rather than truncating with `...`.

---

## Top Three Priorities

If only three things land, pick these — they change the feel of every session, every turn:

1. **Sticky header bar** with session / branch / model / mode (§2).
2. **Informative streaming status** replacing spell names with tool name + key arg (§4).
3. **`Ctrl+R` scrollback search** (§7).
