## Supporting `keen run` Command

1. I want to compare Keen Code against OpenCode. I have an OpenCode Go subscription. That subscription can run both on OpenCode CLI and Keen Code. Since Keen Code takes a conservative approach in context management, I expect to see significant token usage savings in Keen compared to OpenCode CLI. My experiments show the same. But how can I compare these two reliably? I also have opencode repo in `../opencode`.
2. Since both tools run on repl, how can we automatically test them?
3. Would `keen run` directly invoke StreamChat function?
4. Is opencode run synchronous? How does it output to stdin?
5. Okay explore keen. Then suggest me the most simple approach for supporting `keen run` commands. It should be simple, reuse existing code, avoid extra work as much as possible. I prefer isolating benchmark path if that makes things simpler and easier to work on.
6. Do we have to change anything that impacts existing functionalities?
7. In this flow, how is the conversation handled in subsequent turn? Right now, appstate stores the conversation and passes it to LLM every time a user sends a new message. But appstate is in memory. When we will run with `keen run`, we execute one command, receive a response, and execute another command. But who would enrich LLM context with the converstaion so far?
8. Ok based on our conversation, let's create a concrete plan and save in `.ai-interactions/tasks/issue-40` as `output-1_keen-run-plan.md`
9. Let's work on the plan.