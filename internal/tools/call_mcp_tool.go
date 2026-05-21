package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	keenmcp "github.com/user/keen-code/internal/mcp"
)

type CallMCPTool struct {
	manager             keenmcp.Runtime
	permissionRequester PermissionRequester
}

func NewCallMCPTool(manager keenmcp.Runtime, permissionRequester PermissionRequester) *CallMCPTool {
	return &CallMCPTool{
		manager:             manager,
		permissionRequester: permissionRequester,
	}
}

func (t *CallMCPTool) Name() string {
	return "call_mcp_tool"
}

func (t *CallMCPTool) Description() string {
	return `Call a tool on a connected MCP (Model Context Protocol) server.

Before calling, read the server's skill file to discover available tools, then read
the tool's schema file to understand the required arguments:
- Skill file:   ~/.keen/skills/mcp-<server>/SKILL.md
- Schema file:  ~/.keen/skills/mcp-<server>/schemas/<tool>.json

IMPORTANT:
- The server name must match a configured MCP server.
- Arguments must match the tool's input schema exactly.
- Set checkCache to false or omit it (reserved for future use).`
}

func (t *CallMCPTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"server": map[string]any{
				"type":        "string",
				"description": "The MCP server name as configured",
			},
			"tool": map[string]any{
				"type":        "string",
				"description": "The exact tool name to call on the server",
			},
			"arguments": map[string]any{
				"type":        "object",
				"description": "Key-value arguments matching the tool's input schema",
			},
			"checkCache": map[string]any{
				"type":        "boolean",
				"description": "Reserved for future caching; set to false or omit",
			},
		},
		"required":             []string{"server", "tool"},
		"additionalProperties": false,
	}
}

func (t *CallMCPTool) Execute(ctx context.Context, input any) (any, error) {
	params, ok := input.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid input: expected map[string]any, got %T", input)
	}

	server, err := requiredString(params, "server")
	if err != nil {
		return nil, err
	}
	tool, err := requiredString(params, "tool")
	if err != nil {
		return nil, err
	}

	var arguments map[string]any
	if raw, exists := params["arguments"]; exists && raw != nil {
		if m, ok := raw.(map[string]any); ok {
			arguments = m
		}
	}

	_ = params["checkCache"] // reserved, no-op

	argsJSON := ""
	if len(arguments) > 0 {
		data, jsonErr := json.MarshalIndent(arguments, "", "  ")
		if jsonErr == nil {
			argsJSON = string(data)
		}
	}

	if t.permissionRequester == nil {
		return nil, fmt.Errorf("permission denied: user approval required but not available")
	}
	allowed, err := t.permissionRequester.RequestPermission(ctx, t.Name(), server+"/"+tool, argsJSON, false)
	if err != nil {
		return nil, fmt.Errorf("permission request failed: %w", err)
	}
	if !allowed {
		return nil, fmt.Errorf("permission denied by user: call_mcp_tool rejected for %s/%s", server, tool)
	}

	result, err := t.manager.CallTool(ctx, server, tool, arguments)
	if err != nil {
		if result != nil && len(result.Content) > 0 {
			content := formatMCPContent(result.Content)
			if content != "" {
				return nil, fmt.Errorf("%w\n%s", err, content)
			}
		}
		return nil, err
	}

	return map[string]any{
		"server":  server,
		"tool":    tool,
		"content": formatMCPContent(result.Content),
	}, nil
}

func requiredString(params map[string]any, name string) (string, error) {
	v, ok := params[name]
	if !ok {
		return "", fmt.Errorf("invalid input: missing required %q parameter", name)
	}
	s, ok := v.(string)
	if !ok || s == "" {
		return "", fmt.Errorf("invalid input: %q must be a non-empty string", name)
	}
	return s, nil
}

func formatMCPContent(content []mcpsdk.Content) string {
	parts := make([]string, 0, len(content))
	for _, item := range content {
		switch c := item.(type) {
		case *mcpsdk.TextContent:
			if c.Text != "" {
				parts = append(parts, c.Text)
			}
		default:
			data, err := json.Marshal(item)
			if err == nil {
				parts = append(parts, string(data))
			}
		}
	}
	return strings.Join(parts, "\n")
}
