## Supporting Skills in Keen Code

1. Hi, check the issue: https://github.com/mochow13/keen-code/issues/34. Don't do any work. Just read it.
2. Now review the two linked specificiations.
3. Does open standard not support arguments?
4. We want to support skills in Keen Code. Following are the requirements we need to implement:
    - Keen will look for skills according to the open standard in `.agents` directory both for global and project level
    - Skills will be loaded according to the open standard (progressive disclosure)
    - Collisions, parsing, validation will be handled by the open standard
    - Skill catalog should be simple bulleted list with name and description only. No other fields.
    - Catalog should be placed in the system prompt
    - Instruct the model in the system prompt on how to access individual skills using `read_file` tool
    - If no skill is found, nothing will be added to the system prompt
    - Skills can be invoked by `/<skill-name>` or by models naturally deciding to invoke it
    - For now, skills will support only these frontmatter fields: `name`, `description`, `location`. Other fields will be ignored.
    - Skills will also support arguments. We will follow the Claude Code approach for this
    - File references will be relative to the skill root directory
    - `/skills` is a REPL command that will be used to manage skills
    - User can enable/disable skills by simply running a command `/skills <skill-name> <enable|disable>`
    - Enable/disable operation will show a confirmation message to the user
    - Enabled/disabled skills will be persisted across sessions, so we need to store them in a config in ~/.keen/skills/config.json
    - `/skills list` will list all available skills along with their status