## Improve Tool UI

1. Some mcp tools can have very large input. We want to show them but we don't want to clutter our terminal. What's the best way to solve this? @internal/mcpskills @internal/tools/call_mcp_tool.go @internal/cli/repl/output/output.go 
2. If users click it, it will show the full input argument. Otherwise, it will just show one line with tool name and server/tool. How can this be achieved? 