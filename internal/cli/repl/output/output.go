package output

import (
	"fmt"
	"maps"
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
	wrapWidth := ob.width - 3
	if wrapWidth < 1 {
		wrapWidth = 1
	}

	bg := repltheme.UserInputBlockBackground
	wrapStyle := lipgloss.NewStyle().Width(wrapWidth).Background(bg)
	indentStyle := lipgloss.NewStyle().Background(bg)
	prompt := promptStyle.UnsetMarginTop().Background(bg).Render(" ▶ ")

	bodyLines := []string{prompt + wrapStyle.Render(inputLines[0])}
	for i := 1; i < len(inputLines); i++ {
		bodyLines = append(bodyLines, indentStyle.Render("   ")+wrapStyle.Render(inputLines[i]))
	}
	body := strings.Join(bodyLines, "\n")

	rendered := repltheme.UserInputBlockStyle.Width(ob.width).Render(body)
	for _, line := range strings.Split(rendered, "\n") {
		ob.lines = append(ob.lines, line)
	}
	ob.AddEmptyLine()
}

func (ob *OutputBuilder) AddAssistantResponse(response string, assistantStyle lipgloss.Style) {
	wrapStyle := lipgloss.NewStyle().Width(ob.width - 4)
	responseLines := strings.Split(response, "\n")
	for _, line := range responseLines {
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
	return "\n  " + repltheme.ToolStartStyle.Render(fmt.Sprintf("⚙ %s(%s)...", toolCall.Name, inputDisplay))
}

func FormatToolDone(startCall, endCall *llm.ToolCall, workingDir string) string {
	inputDisplay := FormatToolInput(startCall.Name, startCall.Input, workingDir)
	if endCall.Error != "" {
		return "  " + repltheme.ToolErrorStyle.Render(fmt.Sprintf("✗ %s(%s) failed: %s", startCall.Name, inputDisplay, endCall.Error))
	}
	return "  " + repltheme.ToolSuccessStyle.Render(fmt.Sprintf("✓ %s(%s) ➜ [%s]", startCall.Name, inputDisplay, endCall.Duration))
}

func FormatToolInput(toolName string, input map[string]any, workingDir string) string {
	if input == nil {
		return ""
	}

	displayInput := make(map[string]any, len(input))
	maps.Copy(displayInput, input)
	if path, ok := displayInput["path"].(string); ok {
		displayInput["path"] = formatToolPathForUI(path, workingDir)
	}

	switch toolName {
	case "write_file", "edit_file":
		if path, ok := displayInput["path"]; ok {
			return fmt.Sprintf("path=%v", path)
		}
		return ""
	}

	return jsonMarshalCompact(displayInput)
}

func FormatToolEnd(toolCall *llm.ToolCall) string {
	if toolCall.Error != "" {
		return "  " + repltheme.ToolErrorStyle.Render(fmt.Sprintf("✗ %s failed: %s", toolCall.Name, toolCall.Error))
	}
	return "  " + repltheme.ToolSuccessStyle.Render(fmt.Sprintf("✓ %s ➜ [%s]", toolCall.Name, toolCall.Duration)) + "\n"
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
	return strings.Join(parts, ", ")
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
