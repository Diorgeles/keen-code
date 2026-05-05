---
name: git-commit
description: Stage, commit, and push changes with an auto-generated commit message.
---

# Git Commit Skill

## Steps

1. Run `git status --short` to see the current state of the working tree.

2. Stage all tracked modifications:
   ```
   git add -u
   ```

3. If there are untracked files listed in step 1, show them to the user and ask:
   > "The following untracked files were found. Should any of them be included in this commit?"
   > (list the files)
   Wait for the user's response before continuing. Add only the files the user confirms.

4. Run `git diff --staged --stat` to review what is staged. If nothing is staged, tell the user and stop.

5. Inspect the staged diff to write a concise commit message:
   - First line: `type(scope): short summary` (50 chars or fewer)
   - Use types: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`
   - Add bullet points outlining the important changes
   - Never add co-author or AI attribution lines

6. Show the proposed commit message to the user and ask for confirmation or edits before committing.

7. Once confirmed, commit:
   ```
   git commit -m "<message>"
   ```

8. Ask the user whether to push. If yes, run:
   ```
   git push
   ```
   If the push is rejected because the remote branch does not exist yet, re-run with `--set-upstream origin <branch>`.

## Constraints

- Never run `git reset`, `git rebase`, or `git push --force`.
- Never commit files that look like they contain secrets (`.env`, credentials, private keys).
- If the user provided arguments when invoking this skill, treat them as additional context or instructions for the commit message.
