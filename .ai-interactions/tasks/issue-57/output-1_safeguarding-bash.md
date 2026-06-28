# Plan: Improve Bash Tool Safety with Deterministic Danger Classification

## Problem

The `bash` tool currently relies on the LLM to set an `isDangerous` flag.
LLMs are non-deterministic and can:
- omit the flag,
- misclassify a command,
- label a destructive command as safe.

The current default of `isDangerous=false` means a missed flag can lead to
irreversible data loss, privilege escalation, or secret exposure.

## Goal

Make the `bash` tool’s dangerous-command detection deterministic and
fail-safe. The LLM’s `isDangerous` flag should be treated as a hint, but the
final decision must be made server-side. If the classifier is uncertain, it
should prompt the user rather than allow the command silently.

## Out of Scope

- Replacing the bash tool with a restricted shell.
- Sandboxing command execution at the OS level.
- Natural-language classification of command intent.

## Approach

1. Add a server-side command classifier in `internal/tools/bash.go`.
2. Parse the command to extract:
   - the base command(s),
   - flags,
   - write redirections (`>`, `>>`, `>|`),
   - environment variable references (`$VAR`, `${VAR}`),
   - privilege escalation,
   - compound command boundaries (`;`, `&&`, `||`, `|`),
   - command arguments and paths for sensitive-pattern matching.
3. Run classification before execution. Require explicit approval when:
   - the LLM sets `isDangerous=true`, **or**
   - the classifier detects a gated command, flag, or pattern.
4. Never allow `isDangerous=false` to override a dangerous classification.
5. Add unit tests for every gated category and for safe-by-default cases.

## Gated Commands

The following commands require explicit user approval. Commands are grouped by
category with the exact trigger and the reason for gating.

### Irreversible deletion

| Command / Pattern | Gated Condition | Reason |
|---|---|---|
| `rm` | always | Deletes files permanently. |
| `rmdir` | always | Deletes directories permanently. |
| `shred` | always | Overwrites files to make recovery impossible. |
| `dd` | always | Can overwrite disks or destroy data (`dd if=/dev/zero of=/dev/sda`). |
| `mkfs` / `mkfs.*` | always | Formats filesystems; destroys all data on target. |
| `unlink` | always | Deletes a single file directly. |
| `git rm` | always | Removes tracked files from working tree and index. |
| `git clean` | always | Removes untracked files; irreversible. |

### Overwrite / move that can destroy data

| Command / Pattern | Gated Condition | Reason |
|---|---|---|
| `mv` | always | Can overwrite an existing destination file. |
| `install` | always | Copies files and sets permissions; can overwrite. |
| `>` redirection | always | Overwrites file contents silently. |

### Privilege escalation

| Command / Pattern | Gated Condition | Reason |
|---|---|---|
| `sudo` | always | Escapes user permissions; can modify system. |
| `su` | always | Switches user identity. |
| `doas` | always | Alternative privilege escalation. |
| `pkexec` | always | Graphical privilege escalation. |
| `chroot` | always | Changes root filesystem context. |

### Permission / ownership modification

| Command / Pattern | Gated Condition | Reason |
|---|---|---|
| `chmod` | always | Modifies file permissions; no read-only mode exists. |
| `chown` | always | Modifies file ownership; no read-only mode exists. |

### Environment / secrets access

| Command / Pattern | Gated Condition | Reason |
|---|---|---|
| `env` | bare `env` or `env` with no command | Dumps all environment variables, may contain secrets. |
| `printenv` | bare `printenv` (no argument), or `printenv <sensitive-var>` | Full dump may contain secrets; sensitive names match the scoping above. Reading common variables like `printenv HOME` is safe. |
| `export` | always | Modifies shell environment state. |
| `unset` | always | Removes environment variables. |
| `$VAR` or `${VAR}` | only sensitive names (`AWS_*`, `SECRET_*`, `TOKEN_*`, `PASSWORD_*`, `PRIVATE_*`, `GITHUB_TOKEN`, etc.) | Common variables like `$HOME` or `$PATH` are safe; these patterns may leak credentials. |
| `cat ~/.ssh/*` | always | SSH private keys are sensitive. |
| `cat ~/.aws/credentials` | always | AWS credentials are sensitive. |
| `cat ~/.netrc` | always | May contain stored credentials. |
| `cat ~/.git-credentials` | always | May contain stored credentials. |
| `cat /etc/shadow` | always | System password hashes. |
| `cat /etc/sudoers` | always | Privilege configuration. |
| `cat /proc/*/environ` | always | Environment of other processes. |

### Dangerous git operations

| Command / Pattern | Gated Condition | Reason |
|---|---|---|
| `git push` | always | Publishes local history to a remote; `--force` can overwrite history. |
| `git reset` | always | Moves HEAD; `--hard` discards working tree changes permanently. |
| `git rebase` | always | Rewrites commit history. |
| `git merge` | always | Creates merge commits; can introduce conflicts. |
| `git checkout -f` / `--hard` | always | Discards local changes or forces checkout. |
| `git branch -D` / `git branch -d` | always | Deletes a branch and its commits. |
| `git tag -d` | always | Deletes a tag. |
| `git cherry-pick` | always | Creates a new commit from an existing one; alters current branch state. |

### State-modifying but logged, not prompted

These commands change state but are common in coding-agent workflows and are
reversible or low-impact. They should be logged, not prompted.

| Command / Pattern | Reason |
|---|---|
| `git commit` | Local-only; reversible with `git reset HEAD~1`. |
| `git revert` | Safe undo that creates a new commit without rewriting history. |

### Process / session control

| Command / Pattern | Gated Condition | Reason |
|---|---|---|
| `kill` | always | Terminates processes. |
| `killall` | always | Terminates matching processes. |
| `pkill` | always | Terminates matching processes. |
| `xkill` | always | Kills X11 clients. |
| `shutdown` | always | Shuts down the machine. |
| `reboot` | always | Reboots the machine. |
| `halt` | always | Halts the machine. |
| `poweroff` | always | Powers off the machine. |
| `systemctl stop` | always | Stops system services. |
| `systemctl restart` | always | Restarts system services. |
| `systemctl disable` | always | Disables system services. |

### Package / dependency removal or system mutation

| Command / Pattern | Gated Condition | Reason |
|---|---|---|
| `apt-get remove` / `apt remove` | always | Removes installed packages. |
| `apt-get purge` / `apt purge` | always | Removes packages and config files. |
| `apt-get autoremove` | always | Removes packages automatically. |
| `yum remove` | always | Removes packages. |
| `dnf remove` | always | Removes packages. |
| `pacman -R` | always | Removes packages. |
| `brew uninstall` | always | Removes packages. |
| `pip uninstall` | always | Removes Python packages. |
| `npm uninstall` | always | Removes Node packages. |
| `go clean -cache` / `-modcache` | always | Removes cached build or module data. |

### Conditional gating by flag

| Command | Safe Flags / Usage | Gated Flags / Usage | Reason |
|---|---|---|---|
| `cp` | without `-f`/`--force` | `-f`, `--force` | Force overwrite destroys data. |
| `rsync` | list / copy without deletion | `--delete`, `--force` | Can delete or overwrite files at destination. |
| `git checkout` | `git checkout <branch>`, `git checkout -b <branch>` | (force/hard variants are always gated above) | Discards local changes or forces checkout. |
| `docker` | `ps`, `logs`, `images`, `inspect` | `rm`, `rmi`, `run`, `stop`, `kill`, `exec` | Mutates containers and images. |
| `kubectl` | `get`, `describe`, `logs` | `apply`, `delete`, `patch`, `edit` | Mutates cluster state. |
| `make` | default build/test target | `install`, `clean`, `distclean` | Can install to system or delete build artifacts. |

### Subshell / eval gating

| Pattern | Gated Condition | Reason |
|---|---|---|
| `eval ...` | always | Executes arbitrary shell code. |
| `source <file>` / `. <file>` | only when file is outside the working directory | In-project files like `venv/bin/activate` are common and safe. |
| `bash -c ...` | always | Executes arbitrary shell code. |
| `sh -c ...` | always | Executes arbitrary shell code. |
| `zsh -c ...` | always | Executes arbitrary shell code. |
| `python -c ...` / `python3 -c ...` | always | Executes arbitrary code. |
| `perl -e ...` | always | Executes arbitrary code. |
| `ruby -e ...` | always | Executes arbitrary code. |
| `$(...)` / `` `...` `` | always when containing a gated command | Subshell can hide dangerous commands. |

## Argument-based Gating of Safe Commands

Some commands are safe by default but become gated when their arguments match
sensitive patterns. The classifier must inspect arguments and paths, not just
the base command name.

| Example | Why it is gated |
|---|---|
| `cat ~/.ssh/id_rsa` | Reads SSH private keys. |
| `cat /etc/shadow` | Reads system password hashes. |
| `cat /proc/*/environ` | Reads environment of other processes. |
| `printenv GITHUB_TOKEN` | Reads sensitive environment variables. |
| `source /etc/profile` | Sources a file outside the working directory. |

## Safe by Default

The following commands should be permitted without a prompt unless they
contain one of the gated patterns above.

| Category | Examples |
|---|---|
| Read-only inspection | `ls`, `cat`, `find`, `grep`, `head`, `tail`, `less`, `pwd`, `file`, `stat`, `which`, `git status`, `git log`, `git diff`, `git show`, `git branch`, `git remote -v` |
| Build / test | `go build`, `go test`, `go vet`, `npm test`, `pytest`, `cargo build`, `cargo test`, `make` (default target) |
| Safe git staging | `git add`, `git stash` |
| Safe package queries | `apt list`, `brew list`, `pip list`, `go list` |
| Creating new files | `mkdir`, `touch` |
| Appending to files | `echo "..." >> file` | Append never destroys existing data. |

## Files to Modify

- `internal/tools/bash.go` — add deterministic classifier and approval logic.
- `internal/tools/bash_test.go` — add unit tests for classification and gating.

## Success Criteria

- `rm`, `sudo`, `git push`, `>` redirection, and bare `env` are gated even if `isDangerous=false`.
- `ls`, `go test`, `git status`, `git commit`, and `echo "..." >> file` pass without a prompt.
- All new code is covered by unit tests.
- `go test -race ./...` passes.
- `go mod tidy` is run if new dependencies are added.
- `gofmt` is applied to modified Go files.
