---
name: plan
description: Turn a request into a concise implementation plan with assumptions, risks, and verification steps.
---

# Plan Skill

Task to plan (optional): $ARGUMENTS

## Steps

1. **Clarify the goal.** Restate the requested outcome and identify confirmed
   requirements, assumptions, and any open questions that block a safe plan.

2. **Inspect before planning.** For code tasks, read the relevant files,
   commands, tests, and project conventions needed to ground the plan.

3. **Choose the smallest viable approach.**

   - Prefer existing patterns and local helpers.
   - Avoid speculative features or unrelated cleanup.
   - Surface tradeoffs when multiple reasonable paths exist.

4. **Write a verifiable plan.** Each step should include how it will be checked,
   such as a test command, formatter, manual workflow, or file inspection.

5. **Stop at the plan.** Do not edit files unless the user explicitly asks you to
   implement after seeing the plan.

## Output Format

Use this structure:

**Assumptions**

- List only assumptions that affect the plan.

**Open Questions**

- List only questions that must be answered before implementation can start.

**Plan**

1. Do the first concrete step.
   Verify by running or checking `<specific command, test, file, or behavior>`.
2. Do the next concrete step.
   Verify by running or checking `<specific command, test, file, or behavior>`.
3. Identify the final verification needed before the work is considered done.
   Verify by naming the project's required commands or manual checks.

Omit the assumptions or open-questions sections when they do not apply. Keep each
plan step action-oriented and pair it with an explicit verification method.
