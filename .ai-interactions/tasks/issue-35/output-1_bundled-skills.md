# Plan: Ship bundled skills with the keen CLI

## Context

Keen ships as a single static Go binary (via GoReleaser) plus a thin npm wrapper that downloads that binary post-install. There is no sidecar file distribution. The skills system has three discovery roots — project, `~/.agents/skills/`, and `~/.keen/skills/` — and the third was clearly intended as the "built-in" slot, but nothing populates it. Users who run `keen` get an empty skill list out of the box.

We want a curated set of skills to ship in the binary itself (e.g. `git-commit`, `echo`, future additions), available the moment the user runs `keen`, with no install step. User-defined skills live in four well-known disk locations and can override the bundled set by name.

## Approach

**Bundled skills:** Source lives at `internal/skills/bundled/<name>/SKILL.md` and is `//go:embed`ed into the binary. At startup, the binary extracts each embedded SKILL.md into `~/.keen/skills/bundled/<name>/SKILL.md`, overwriting on every launch so the bundled set always matches the binary version. `~/.keen/skills/bundled/` becomes the lowest-priority discovery root.

**User-defined skills:** Four disk discovery roots, in override order (highest first):
1. `<cwd>/.agents/skills/` — project, vendor-neutral
2. `<cwd>/.keen/skills/` — project, keen-specific *(new)*
3. `~/.agents/skills/` — global, vendor-neutral
4. `~/.keen/skills/` — global, keen-specific
5. `~/.keen/skills/bundled/` — bundled (binary-managed) *(new)*

Existing `Discover`/`LoadMetadata` precedence (first-found wins on dirname) means a project skill named `git-commit` automatically shadows the bundled one without any new logic.

### Why extract to disk, not virtual FS

The catalog entry `→ read <abs path>` is a contract with the model — it can `read_file` the path for model-driven activation. Virtual paths break that. Extracting to disk keeps the contract intact, lets the existing filesystem guard work, and adds zero code paths in `Discover`/`LoadMetadata`/`Catalog`/`ActivationMessage`.

### Why `~/.keen/skills/bundled/` and not `~/.keen/skills/` directly

`~/.keen/skills/` is a user-managed root — users put skills there with `<name>/SKILL.md` directly. Mixing binary-managed files with user files there would be destructive (the bundled ones would clobber on every launch). The `bundled/` subdirectory is its own namespace, scanned as a separate discovery root via `~/.keen/skills/bundled/*/SKILL.md`. Discovery's existing glob (`<root>/*/SKILL.md`) won't match `~/.keen/skills/bundled/<name>/SKILL.md` from the parent root, so the two namespaces stay cleanly separate.

Edge case: a user-defined skill at `~/.keen/skills/bundled/SKILL.md` (a skill literally named `bundled`) would conflict with our namespace directory. Document the reserved name; not worth more code.

## Files to modify

**New:** `internal/skills/bundled.go`
- `//go:embed all:bundled` on an `embed.FS`.
- `EnsureBundled() (string, error)` — resolves `~/.keen/skills/bundled/` via `os.UserHomeDir()`, walks the embedded tree, writes each `SKILL.md` (overwriting), returns the bundled root path. Returns `("", nil)` if the home dir can't be resolved (degrade gracefully — bundled skills just won't appear).

**New:** `internal/skills/bundled/git-commit/SKILL.md`, `internal/skills/bundled/echo/SKILL.md`
- Move from `.agents/skills/` to here.

**Modify:** `internal/skills/discover.go:116-125` (`discoveryRoots`)
- Take an extra arg or call `EnsureBundled` directly. Add `<cwd>/.keen/skills` as the second root, append `~/.keen/skills/bundled` as the last root.
- Cleanest: change `discoveryRoots` to accept `(workingDir, bundledDir string)` and have the caller (`Discover`) compute `bundledDir` via `EnsureBundled`.

**Modify:** `internal/cli/repl/appstate/state.go:56-65` (`ReloadSkills`)
- Resolve bundled dir via `skills.EnsureBundled()` before `Discover`. Pass into `Discover` (which now takes both workingDir + bundledDir).

**Delete:** `.agents/skills/` (echo, git-commit) — they ship in the binary now. Self-hosting development still works because the project-level `.agents/skills/` root takes priority if someone wants to customize while developing keen itself.

**Tests:**
- `internal/skills/bundled_test.go` — embedded FS non-empty; `EnsureBundled` writes expected files into `<HOME>/.keen/skills/bundled/`; overwrites prior contents.
- `internal/skills/discover_test.go` — extend `TestDiscover_*` to cover the new project `.keen/skills` root and the bundled root, with override precedence (project beats global beats bundled). Verify the `bundled/` subdirectory does not bleed into the parent `~/.keen/skills/` glob.

**No changes** to `Catalog`, `ActivationMessage`, `ParseSkillMetadata`, the filesystem guard (`~/.keen/` is read-everywhere by default for the model), GoReleaser config, or the npm wrapper.

## Pattern reuse

- `providers/loader.go:9-10` — established `//go:embed` + `embed.FS` pattern in this repo.
- Existing `Discover` collision logic — first-found-by-dirname wins. Bundled becomes a regular root; no special-casing needed.

## Verification

1. `go build ./...` and `go test ./...` pass.
2. `rm -rf ~/.keen/skills/bundled && ./keen` — verify it gets re-created with the bundled set.
3. In a directory with no `.agents/skills/` or `.keen/skills/`, launch `keen`, run `/skills list`, see bundled `git-commit` and `echo`.
4. Type `/git-commit` in REPL — confirm activation message uses bundled SKILL.md content.
5. Create `<cwd>/.keen/skills/git-commit/SKILL.md` with different content — confirm project version wins (verifies the new project root + override precedence).
6. Create `~/.keen/skills/foo/SKILL.md` — confirm it loads alongside bundled skills, and that nothing in `~/.keen/skills/bundled/` leaks into the user-global root.
7. Edit a file in `~/.keen/skills/bundled/` directly, relaunch — confirm overwrite (documents the binary-managed contract).
