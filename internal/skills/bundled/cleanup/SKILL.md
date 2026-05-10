---
name: cleanup
description: Clean up clearly bounded code by removing dead, duplicated, stale, or noisy implementation details.
---

# Cleanup Skill

Cleanup target (optional): $ARGUMENTS

## Steps

1. **Define the boundary.** Identify the specific files, package, feature, or
   change set the cleanup applies to. If the request is broad, choose a small
   safe scope and state it before editing.

2. **Classify cleanup candidates.** Look for unused code, stale comments,
   duplicated logic, unnecessary helpers, formatting drift, or dead tests within
   the stated boundary.

3. **Avoid behavior changes.**

   - Do not alter public APIs, CLI behavior, file formats, or user-visible
     output unless explicitly requested.
   - Do not delete code unless it is clearly unreachable, unused, or made
     obsolete by the current cleanup.
   - Do not refactor unrelated nearby code.

4. **Edit in small steps.** Prefer simple removals and local simplifications.
   Keep style consistent with the surrounding code.

5. **Verify.** Run relevant tests and required formatters. Use broader tests when
   cleanup touches shared helpers, package boundaries, or public behavior.

6. **Report.** Summarize what was removed or simplified and which verification
   commands passed.
