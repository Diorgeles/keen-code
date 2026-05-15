## Supporting Plan and Build Modes

1. We want to support plan and build modes. Build mode will be as it is. In plan mode, the agent will not have access to write_file and edit_file tools. The agent will have access to bash tool but strictly for non-write operations. I am not sure how to restrict that in bash. Perhaps, in the system prompt?
2. For plan mode, we will use the almost same system prompt but tailored for plan mode. So it makes sense to have a separate system prompt for plan mode.
3. For the build mode, it will be as it is. We can encourage the agent to lean towards building.
4. If agent is in plan mode but user asks to build or write something, the agent should ask user to switch to build mode.
5. Users can toggle between plan and build modes using `/mode [plan|build]` command as well as `shift+tab` in the REPL.