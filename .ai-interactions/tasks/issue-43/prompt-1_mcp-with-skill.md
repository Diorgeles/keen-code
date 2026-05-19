## MCP Progressive Disclosure via Skills
- For Keen Code, I want to bring in support for MCP servers. So Keen Code will allow users to add mcp servers and
  enable/disable them.
  Before that, I want to discuss an idea with you.
  One of the issues with MCP servers is that they are part of the system prompt and they are always in the prompt even
   if I don't need them. Enabling/disabling them every time I need or don't need them is little inconvenient.
  I am thinking, can we activate them on-demand, through skills?
  The idea is, every time an MCP server is added, a new skill will be created *for the MCP server*. Whenever users
  activate the skill or agent decides to use it, the MCP server will be used. This way, MCP servers get enabled only
  when they are needed.
  What do you think?
- Why do we need an activate_mcp_server tool if we didn't need an actiavate_skill tool?
- What if we keep it simple?
  Skills will be created as soon as an MCP server is connected. If it's disabled, the skill will be disabled.
  The skill will be generated, and it's role is purely for progressive discovery.
  Other things about MCP server protocol will remain the same: Keen will create connection, discover tools, call tools etc. But the tool prompts won't be in system prompt by default. They will only be added when agent needs it or user uses a slash command.
- Reading a skill file by agent and taking that as a sign to activate the server feels fragile.
- Let's use a `call_mcp_tool` for allowing LLMs to invoke a particular tool in an mcp server? The tool will follow the standard protocol implementation to invoke the tool and return the result.
  This tool can be very simple.
  Wati, did I just reinvent search tool for mcp tools?
- Can these tools simply use MCP protocol standard mechanisms under the hood?
- Can we use skills to already pre-fetch the tools/list so that tools and schemas are available already? Every time an MCP server is connected, enabled, or refreshed by Keen, the schema file can be refreshed. So it's readily available.
  LLM can read the schema file when needed, just like another skill. Then it knows what the tool looks like.
  The final piece is invoking the tool. This should be simple as we can follow the standard protocol.
- So we need this new tool: `call_mcp_tool`?
- In MCP protocol, how does client keep refreshing the tools? Or how does it keep itself up to date in the standard protocol?
- Is this idea a new invention?
- Do you see any flaw? Think carefully.
- ok based on our discussion, give me a final draft note for the idea.
- The inner content of the SKILL.md file can contain the list of tools. We don't need a separate TOOLS.md
- No we should still have per-tool schema file.
- And schema files can be jsons. LLM can read json.
- How can we generate the SKILL.md file?
- ok update the draft you gave me earlier
- Save it in @.ai-interactions/tasks/issue-43 as output-1_mcp-with-skill.md
