## Bundled Subagent Implementation
1. We have a plan for subagents in `@.ai-interactions/tasks/issue-55/output-2_subagents-rfc.md`.
   In the first iteration, we want to build a single subagent: explorer.
   This subagent will have read-only tools. It's YAML frontmatter will be minimal. It will inherit the model, provider, etc. fields from the main model, so we will not set those optional fields in the YAML frontmatter.
   Its system prompt will explain its role. It will be used to explore codebase either fully or based on specific directory paths or list of file paths. It will receive file paths as a part of the task if main agent decides to provide.
   The result from the delegate tool will be concise, to the point. The design proposes to return a JSON structured response from the tool. But we don't have to do it. As long as the content can be passed back to the main agent and main agent can read them easily like tool messages, we are good.
   The main agent needs to understand that calling `explorer` subagent has to be targeted. It must pass clear and specific instructions, with paths to specific directories if needed. Explorer subagent will have a clear and specific system prompt in the markdown file. It won't have user and main agent conversation context. Its role is simply:
   - take in the specific task asked by the main agent
   - explore the codebase based on what was asked
   - return a concise and organised result instead of full raw results
2. I think we should name the package `subagents`.
3. Subagents won't have skills. So no MCP support. Let's update the plan and the implementation both.
4. Subagent prompt comes from the subagent markdown file. Why are we separately having a prompt in `@internal/llm/systemprompt.go`?
5. In `@internal/llm/systemprompt.go` we have `Build`, `BuildForMode`, `BuildWithCatalogs` - not sure this chaining is useful.
6. Some fields in `@internal/subagents/delegate_tool.go` are optional. What if main agent doesn't set them?
7. Actually, we don't need `maxTurns` either. `StreamChat` internally has `maxToolTurns` which keeps looping.
8. We should move `delegate_tool` to `@internal/tools/` package since all tools are there.
9. Add a section to the plan in `@.ai-interactions/tasks/issue-55/output-2_subagents-rfc.md` on what we have done here. Keep it brief.
10. Walk me through the implementation. From starting point to subagent trigger.
11. Do we need `Location` in `Profile` struct?
12. Just like how we have a `@docs/skills-system.md` doc, write a `subagents.md` doc. Add content that are relevant for end users, avoid internal implementation details. Also mention what users need to do to add their own subagents.
13. Are we guiding the main agent enough on when to use a subagent?
14. Let's make the subagent guidance more generic. We are currently focusing on codebase exploration only. Users might have more granular usecase.
15. We are allowing subagents to be triggered by the main agent on its own decision. But in the plan we mentioned users have to ask for it. Let's update the plan.
16. Let's remove stale reference on the plan with fields for tools and Profiles that we don't want to support anymore. No tools is an important field.
17. Runner holds a snapshot of profiles at startup. `internal/cli/repl/tooling/tool_registry.go:56` passes `appState.GetSubagents().Profiles` to the runner at init time. If `ReloadSubagents()` is called later (e.g., user adds a new `.md` file), the runner still sees the stale snapshot. The runner should either accept a profile getter or be re-wired on reload.
18. Make sure in our subagent implementation, we are not breaking prefix caching in subsequent calls.
19. Add unit tests for missing files in `@internal/subagents`.
20. Add tests for `@internal/tools/delegate_tool.go`.
21. How are we showing the subagent invocation on UI?
22. Let's only show the `agent` argument.
23. Well should we put this guideline here? Or should such agent-specific guideline be in the agent description itself, for instance here in `@internal/subagents/bundled/explorer.md`?
24. No I think the guidelines are for the main agent. Main agent sees the description, not the system prompt for the subagent.
25. Add a `/subagents list` command.
26. Make sure tests you added are not modifying the actual config directory.
