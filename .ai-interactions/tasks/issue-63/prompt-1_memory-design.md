## Agent Memory

1. How to design a memory system for coding agents? For now, skip checking the local project. Figure out how other coding agents like Claude Code, Codex, OpenCode do this. Use Exa websearch.
2. We don't have any memory mechanism: what can we implement as a first version that's useful but not too complicated?
3. why do MEMORY.md be in ~/.config instead of locally in project, for example in ./.keen/?
4. Ok we can put it in ~/.keen/memory/<project|global>/MEMORY.md. What do you think?
5. why do we need project-id? The project directory name should be enough. And what's the purpose of project.json?
6. I think it's better then we put local memory in ./.keen/MEMORY.md and users can always gitignore `.keen`
7. we should let users put it in gitignore themeselves
8. what commands should we support?
9. Agent can simply edit/update/write the memory files. We don't need commands like /memory add or so.
10. Yes I like `/memory` and `/memory show` commands. `/memory` will list the paths, `/memory show` will show the content. What do you think?
11. If no memory files or content, just message that "Memory is empty" or "No memory files found"
12. ok so create a plan in @.ai-interactions/tasks/issue-63 (the dir doesn't exist) and save as output-1_memory-design.md
