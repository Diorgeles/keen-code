# Bundled Skills Foundation

1. Keen Code implements agent skills. Explore how it's implemented. Then share your critical analysis.
2. Address these one by one:
    - The frontmatter name field is essentially decorative > let's use the name from the YAML instead of directory name
    - Slash-command argument handling is a string append > let's replace arguments in the content
    - The dual invocation paths leak > but we put the path in system prompt, right?
3. Okay now review skill changes again. What's the current status?
4. We also need to use `/<skill-name>` correctly from the name field of the SKILL.md file, not from directory name.
5. What's the simplest way to support bundled skills (skills that are supported in keen out of the box) so that users can simply use them when running `keen` as a CLI? Give me options.
6. Save the plan in `@.ai-interactions/tasks/issues-35/` as `output-1_bundled-skills.md` and use it as the source of truth.
7. Change of plan: bundled skills will be placed in `~/.keen/skills/bundled` directory. Update it in `@.ai-interactions/tasks/issues-35/output-1_bundled-skills.md`.
8. Ok let's create a `commit` skill for committing to git. Add necessary description and instructions.
9. User defined skills should take priority over Keen's bundled skills. Confirm this.
