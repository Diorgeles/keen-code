---
name: debug
description: Investigate, reproduce, and fix bugs using focused evidence and relevant tests.
---

# Debug Skill

Bug or failure to debug (optional): $ARGUMENTS

## Steps

1. **Understand the failure.** Identify the reported symptom, expected behavior,
   observed behavior, inputs, environment, and any error output.

2. **Reproduce or narrow it.** Run the smallest command, test, or workflow that
   demonstrates the problem. If reproduction is not possible, gather enough
   evidence to state the most likely failure path.

3. **Find the cause.** Read the relevant code and tests. Trace data flow and
   control flow from the failing behavior back to the source of the bug.

4. **Fix surgically.**

   - Change only the code needed to address the confirmed cause.
   - Prefer existing patterns, helpers, and error-handling style.
   - Add or update a focused regression test when practical.

5. **Verify.** Rerun the reproducer first, then run broader tests when shared
   behavior or public contracts changed. Run required formatters and dependency
   tidy commands for the project.

6. **Report.** Summarize the root cause, fix, and verification commands. If the
   issue cannot be fully fixed, state the blocker and remaining risk.
