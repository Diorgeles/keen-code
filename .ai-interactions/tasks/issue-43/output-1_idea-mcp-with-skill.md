# Progressive MCP Tool Disclosure via Skills

Keen Code should support MCP servers without adding every MCP tool schema to the model prompt by default. MCP servers can expose many tools, and including all tool descriptions and schemas all the time increases prompt size even when the tools are irrelevant.

The proposed design is to use Keen's skills system as a lightweight discovery layer for configured MCP servers, while keeping MCP execution fully standard.

For the first iteration, Keen treats configured MCP servers as trusted user-selected extensions. Users decide which MCP servers to configure. Keen limits execution through the configured MCP server registry and the `call_mcp_tool` gateway.

When Keen starts, it will scan configured MCP servers in the background. For each configured server, Keen connects to the server, calls the standard MCP `tools/list` method, and generates local skill/schema documentation for that server. The generated skill is not the execution mechanism. Its role is to help the agent discover that the server exists, understand when it is useful, see a compact list of available tools, and find exact schema files for individual tools.

The actual invocation happens through one Keen-native tool: `call_mcp_tool`.

```json
{
  "server": "github",
  "tool": "create_issue",
  "arguments": {
    "owner": "acme",
    "repo": "web",
    "title": "Bug report"
  }
}
```

Under the hood, `call_mcp_tool` invokes the selected MCP tool using the standard MCP `tools/call` protocol.

## Generated Files

For each configured and connected MCP server, Keen generates one managed skill directory:

```text
mcp-github/
├── SKILL.md
├── .keen-generated-mcp.json
└── schemas/
    ├── create_issue.json
    ├── list_pull_requests.json
    └── ...
```

Generated MCP skills should live under a dedicated Keen-managed location, separate from user-authored skills. Each generated directory must include metadata that identifies it as a Keen-managed MCP skill so cleanup never removes user files by mistake.

`SKILL.md` contains:

- skill frontmatter, e.g. `name: mcp-github`
- a short description of when to use the server
- server name and generated timestamp
- compact list of available tools
- path to each tool's schema JSON
- instruction to invoke tools through `call_mcp_tool`

`.keen-generated-mcp.json` contains state used by Keen, for example:

```json
{
  "managed_by": "keen-mcp",
  "server": "github",
  "configured": true,
  "status": "connected",
  "last_successful_refresh": "2026-05-17T10:00:00Z",
  "last_error": ""
}
```

Each schema JSON contains the MCP `tools/list` metadata for one tool:

```json
{
  "server": "github",
  "name": "create_issue",
  "description": "Create a GitHub issue",
  "inputSchema": {
    "type": "object",
    "properties": {}
  }
}
```

## Proposed Flow

```text
Keen starts
→ Keen starts a background MCP discovery goroutine
→ Discovery scans configured MCP servers
→ Keen starts one goroutine per configured server
→ Each server goroutine connects and initializes the MCP client
→ Each connected server goroutine calls tools/list
→ Keen atomically generates or refreshes SKILL.md, metadata, and schema JSON files
→ Generated skill appears in Available Skills when the server is connected
→ Agent reads the skill only when relevant
→ Agent reads the schema JSON for the chosen tool
→ Agent calls call_mcp_tool
→ Keen validates and invokes MCP tools/call
→ Result is returned to the agent
```

## Startup Refresh Behavior

For the first iteration, Keen refreshes MCP tool metadata only on startup.

Startup refresh should run in the background so Keen can open without blocking on slow or unavailable external MCP servers. Keen may reload the skill catalog before new turns or notify app state when MCP discovery finishes so newly generated MCP skills become available after startup.

Each server refresh should use a per-server timeout and app-shutdown cancellation. File writes should be atomic, such as generating into a temporary directory and then renaming into place, so the agent never reads partially written skill or schema files.

After successful refresh, Keen rewrites `SKILL.md`, `.keen-generated-mcp.json`, and the per-tool schema JSON files so the agent can inspect current tool names and schemas on demand.

## Server State and Cleanup

Keen should distinguish configured-but-disconnected servers from removed servers:

- `configured + connected`: generate or refresh `SKILL.md`, metadata, and schemas; the generated skill is enabled/discoverable.
- `configured + connection failed`: keep the generated files, mark the server as `disconnected`, and hide or disable the generated skill so the agent does not try to use stale tools.
- `not configured anymore`: delete the corresponding Keen-managed MCP skill directory and schemas.

Deletion must only target directories marked as Keen-managed MCP output, such as directories containing `.keen-generated-mcp.json` with `managed_by: "keen-mcp"`.

Authentication should use the configured mechanism for the MCP server, such as an API key or OAuth. Secrets must not be written into generated skill, metadata, or schema files.

## Safety Notes

Generated skill/schema files are discovery material only. They should not grant execution authority by themselves.

`call_mcp_tool` is the privileged gateway, so it must enforce:

- server is configured and enabled,
- server is currently connected,
- requested tool exists on that server,
- arguments validate against the cached MCP input schema where possible,
- risky tools can require confirmation if Keen adds permission controls.

Even if a stale skill appears in context, execution should still fail when the server is disconnected or no longer configured. For example, `call_mcp_tool` should return an actionable error such as: `MCP server "github" is disconnected; reconnect or restart Keen after fixing the server.`

## Summary

This design is not a new MCP protocol. It is progressive disclosure on top of standard MCP:

```text
SKILL.md for server discovery
schema JSON files for per-tool input docs
startup tools/list for metadata refresh
call_mcp_tool for execution
tools/call for protocol-compliant invocation
```

The result is a smaller default prompt, a familiar skills-based UX, managed startup discovery for configured MCP servers, and standard MCP compatibility without registering every MCP tool as a native model tool upfront.
