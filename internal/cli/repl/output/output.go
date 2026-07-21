package output

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
	repltheme "github.com/user/keen-code/internal/cli/repl/theme"
	"github.com/user/keen-code/internal/llm"
)

type OutputBuilder struct {
	lines      []string
	width      int
	workingDir string
}

func NewOutputBuilder(width int, workingDir string) *OutputBuilder {
	return &OutputBuilder{
		lines:      []string{},
		width:      width,
		workingDir: workingDir,
	}
}

func (ob *OutputBuilder) SetLines(lines []string) {
	ob.lines = lines
}

func (ob *OutputBuilder) GetLines() []string {
	return ob.lines
}

func (ob *OutputBuilder) AddLine(line string) {
	ob.lines = append(ob.lines, line)
}

func (ob *OutputBuilder) AddEmptyLine() {
	ob.lines = append(ob.lines, "")
}

func (ob *OutputBuilder) SetWidth(width int) {
	ob.width = width
}

func (ob *OutputBuilder) AddUserInput(input string, promptStyle lipgloss.Style) {
	inputLines := strings.Split(input, "\n")
	const (
		promptWidth            = 3
		blockHorizontalPadding = 2
	)
	blockContentWidth := ob.width - blockHorizontalPadding
	if blockContentWidth < 1 {
		blockContentWidth = 1
	}
	wrapWidth := blockContentWidth - promptWidth
	if wrapWidth < 1 {
		wrapWidth = 1
	}

	bg := repltheme.UserInputBlockBackground
	wrapStyle := lipgloss.NewStyle().Width(wrapWidth).Background(bg)
	indentStyle := lipgloss.NewStyle().Background(bg)
	prompt := promptStyle.UnsetMarginTop().Background(bg).Render(" ▶ ")

	bodyLines := make([]string, 0, len(inputLines))
	for i, inputLine := range inputLines {
		prefix := indentStyle.Render("   ")
		if i == 0 {
			prefix = prompt
		}

		wrappedLines := strings.Split(wrapStyle.Render(inputLine), "\n")
		for j, wrappedLine := range wrappedLines {
			if j > 0 {
				prefix = indentStyle.Render("   ")
			}
			bodyLines = append(bodyLines, prefix+wrappedLine)
		}
	}
	body := strings.Join(bodyLines, "\n")

	rendered := repltheme.UserInputBlockStyle.Width(ob.width).Render(body)
	for line := range strings.SplitSeq(rendered, "\n") {
		ob.lines = append(ob.lines, line)
	}
	ob.AddEmptyLine()
}

func (ob *OutputBuilder) AddAssistantResponse(response string, assistantStyle lipgloss.Style) {
	wrapStyle := lipgloss.NewStyle().Width(ob.width - 4)
	responseLines := strings.SplitSeq(response, "\n")
	for line := range responseLines {
		ob.lines = append(ob.lines, "  "+wrapStyle.Render(assistantStyle.Render(line)))
	}
	ob.AddEmptyLine()
}

func (ob *OutputBuilder) AddError(err string, errorStyle lipgloss.Style) {
	wrapStyle := lipgloss.NewStyle().Width(ob.width - 4)
	ob.lines = append(ob.lines, wrapStyle.Render(errorStyle.Render("  Error: "+err)))
	ob.AddEmptyLine()
}

func (ob *OutputBuilder) AddStyledLine(content string, style lipgloss.Style) {
	ob.lines = append(ob.lines, style.Render(content))
}

func (ob *OutputBuilder) Join() string {
	if len(ob.lines) == 0 {
		return ""
	}
	return strings.Join(ob.lines, "\n")
}

func (ob *OutputBuilder) IsEmpty() bool {
	return len(ob.lines) == 0
}

func (ob *OutputBuilder) AddToolStart(toolCall *llm.ToolCall) {
	ob.lines = append(ob.lines, FormatToolStart(toolCall, ob.workingDir))
}

func (ob *OutputBuilder) AddToolEnd(toolCall *llm.ToolCall) {
	ob.lines = append(ob.lines, FormatToolEnd(toolCall))
}

func FormatToolStart(toolCall *llm.ToolCall, workingDir string) string {
	inputDisplay := FormatToolInput(toolCall.Name, toolCall.Input, workingDir)
	return "  " + repltheme.ToolStartStyle.Render(fmt.Sprintf("⚙ %s → %s...", toolCall.Name, inputDisplay))
}

func FormatToolDone(startCall, endCall *llm.ToolCall, workingDir string) string {
	inputDisplay := FormatToolInput(startCall.Name, startCall.Input, workingDir)
	if endCall.Error != "" {
		return "  " + repltheme.ToolErrorStyle.Render(fmt.Sprintf("✗ %s → %s failed: %s", startCall.Name, inputDisplay, endCall.Error))
	}
	if startCall.Name == "delegate_task" {
		completed, failed, completedByAgent, failedByAgent, ok := delegateResultCounts(endCall.Output)
		if ok {
			status := fmt.Sprintf("%d completed", completed)
			if agents := formatAgentCounts(completedByAgent); agents != "" {
				status += " (" + agents + ")"
			}
			if failed > 0 {
				status += fmt.Sprintf(", %d failed", failed)
				if agents := formatAgentCounts(failedByAgent); agents != "" {
					status += " (" + agents + ")"
				}
				return "  " + repltheme.ToolErrorStyle.Render(fmt.Sprintf("✗ %s → %s", startCall.Name, status))
			}
			return "  " + repltheme.ToolSuccessStyle.Render(fmt.Sprintf("✓ %s → %s", startCall.Name, status))
		}
	}
	return "  " + repltheme.ToolSuccessStyle.Render(fmt.Sprintf("✓ %s → %s", startCall.Name, inputDisplay))
}

var toolDisplayFields = map[string][]string{
	"read_file":  {"path"},
	"write_file": {"path"},
	"edit_file":  {"path"},
	"grep":       {"path", "pattern"},
	"glob":       {"path", "pattern"},
}

func FormatToolInput(toolName string, input map[string]any, workingDir string) string {
	if input == nil {
		return ""
	}

	if toolName == "call_mcp_tool" {
		return formatMCPToolInput(input)
	}
	if toolName == "delegate_task" {
		return formatDelegateTaskInput(input)
	}

	fields, ok := toolDisplayFields[toolName]
	if !ok {
		return jsonMarshalCompact(input)
	}

	displayInput := make(map[string]any, len(fields))
	for _, field := range fields {
		value, ok := input[field]
		if !ok {
			continue
		}
		if field == "path" {
			if path, ok := value.(string); ok {
				displayInput[field] = formatToolPathForUI(path, workingDir)
			}
			continue
		}
		displayInput[field] = value
	}
	return jsonMarshalCompact(displayInput)
}

func FormatToolEnd(toolCall *llm.ToolCall) string {
	if toolCall.Error != "" {
		return "  " + repltheme.ToolErrorStyle.Render(fmt.Sprintf("✗ %s failed: %s", toolCall.Name, toolCall.Error))
	}
	return "  " + repltheme.ToolSuccessStyle.Render(fmt.Sprintf("✓ %s ➜ [%s]", toolCall.Name, toolCall.Duration))
}

func jsonMarshalCompact(v map[string]any) string {
	if v == nil {
		return ""
	}
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v[k]))
	}
	return strings.Join(parts, " • ")
}

func formatMCPToolInput(input map[string]any) string {
	server, _ := input["server"].(string)
	tool, _ := input["tool"].(string)
	if server == "" || tool == "" {
		return ""
	}
	return server + "/" + tool
}

func formatDelegateTaskInput(input map[string]any) string {
	tasks, ok := input["tasks"].([]any)
	if !ok {
		return "0 tasks"
	}
	label := "tasks"
	if len(tasks) == 1 {
		label = "task"
	}

	agentCounts := make(map[string]int)
	for _, task := range tasks {
		task, ok := task.(map[string]any)
		if !ok {
			continue
		}
		agent, _ := task["agent"].(string)
		if agent != "" {
			agentCounts[agent]++
		}
	}
	if len(agentCounts) == 0 {
		return fmt.Sprintf("%d %s", len(tasks), label)
	}

	return fmt.Sprintf("%d %s (%s)", len(tasks), label, formatAgentCounts(agentCounts))
}

func delegateResultCounts(output any) (completed, failed int, completedByAgent, failedByAgent map[string]int, ok bool) {
	result, ok := output.(map[string]any)
	if !ok {
		return 0, 0, nil, nil, false
	}
	completed, completedOK := intValue(result["completed"])
	failed, failedOK := intValue(result["failed"])
	completedByAgent, completedAgentsOK := agentCountsValue(result["completed_by_agent"])
	failedByAgent, failedAgentsOK := agentCountsValue(result["failed_by_agent"])
	return completed, failed, completedByAgent, failedByAgent, completedOK && failedOK && completedAgentsOK && failedAgentsOK
}

func agentCountsValue(value any) (map[string]int, bool) {
	counts := make(map[string]int)
	switch value := value.(type) {
	case map[string]int:
		for agent, count := range value {
			counts[agent] = count
		}
	case map[string]any:
		for agent, value := range value {
			count, ok := intValue(value)
			if !ok {
				return nil, false
			}
			counts[agent] = count
		}
	default:
		return nil, false
	}
	return counts, true
}

func formatAgentCounts(agentCounts map[string]int) string {
	agents := make([]string, 0, len(agentCounts))
	for agent := range agentCounts {
		agents = append(agents, agent)
	}
	sort.Strings(agents)
	parts := make([]string, 0, len(agents))
	for _, agent := range agents {
		parts = append(parts, fmt.Sprintf("%s ×%d", agent, agentCounts[agent]))
	}
	return strings.Join(parts, ", ")
}

func intValue(value any) (int, bool) {
	switch value := value.(type) {
	case int:
		return value, true
	case float64:
		return int(value), value == float64(int(value))
	default:
		return 0, false
	}
}

func formatToolPathForUI(path, workingDir string) string {
	if path == "" || workingDir == "" || !filepath.IsAbs(path) {
		return path
	}

	relPath, err := filepath.Rel(workingDir, path)
	if err != nil {
		return path
	}
	return relPath
}
