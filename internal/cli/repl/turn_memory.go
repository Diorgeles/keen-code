package repl

import (
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/user/keen-code/internal/llm"
)

const maxHistoricalToolTargetBytes = 256

var sensitiveTargetPattern = regexp.MustCompile(`(?i)(api[_-]?key|token|secret|password|passwd|authorization|credential)`)

type turnMemoryAccumulator struct {
	toolActivity []llm.HistoricalToolActivity
}

func newTurnMemoryAccumulator() *turnMemoryAccumulator {
	return &turnMemoryAccumulator{}
}

func (a *turnMemoryAccumulator) RecordToolActivity(segments []streamSegment, workingDir string) {
	if a == nil {
		return
	}
	a.toolActivity = collectHistoricalToolActivity(segments, workingDir)
}

func collectHistoricalToolActivity(segments []streamSegment, workingDir string) []llm.HistoricalToolActivity {
	textOffset := 0
	activities := make([]llm.HistoricalToolActivity, 0)

	for _, segment := range segments {
		switch segment.kind {
		case segmentAssistant:
			textOffset += len(segment.content)
		case segmentToolEnd:
			if segment.toolCall != nil {
				activities = append(activities, historicalToolActivity(segment.toolCall, textOffset, workingDir, ""))
			}
		case segmentBash:
			if segment.toolCall != nil {
				activities = append(activities, historicalToolActivity(segment.toolCall, textOffset, workingDir, segment.command))
			}
		}
	}

	return activities
}

func historicalToolActivity(toolCall *llm.ToolCall, textOffset int, workingDir, bashCommand string) llm.HistoricalToolActivity {
	activity := llm.HistoricalToolActivity{
		TextOffset: textOffset,
		Tool:       toolCall.Name,
		Status:     "success",
	}
	if toolCall.Error != "" {
		activity.Status = "error"
	}

	if toolCall.Name == "call_mcp_tool" {
		activity.Server = boundedTarget(toolStringField(toolCall, "server"))
		activity.Tool = boundedTarget(toolStringField(toolCall, "tool"))
		if activity.Tool == "" {
			activity.Tool = toolCall.Name
		}
		return activity
	}

	activity.Target = historicalToolTarget(toolCall, workingDir, bashCommand)
	if activity.Status != "success" {
		return activity
	}

	switch toolCall.Name {
	case "write_file", "edit_file":
		activity.FileChanged = boundedTarget(relativizePath(extractStringField(toolCall.Output, "file_changed"), workingDir))
	case "bash":
		exitCode, ok := extractIntField(toolCall.Output, "exit_code")
		if !ok || exitCode == 0 {
			return activity
		}
		command := extractStringField(toolCall.Output, "failed_command")
		activity.FailedCommand = boundedTarget(command)
		activity.ExitCode = &exitCode
	}
	return activity
}

func historicalToolTarget(toolCall *llm.ToolCall, workingDir, bashCommand string) string {
	var target string
	switch toolCall.Name {
	case "read_file", "write_file", "edit_file":
		target = relativizePath(inputStringField(toolCall, "path"), workingDir)
	case "glob", "grep":
		path := relativizePath(inputStringField(toolCall, "path"), workingDir)
		pattern := inputStringField(toolCall, "pattern")
		target = joinTarget(path, pattern)
	case "bash":
		target = bashCommand
		if target == "" {
			target = toolStringField(toolCall, "command")
		}
	case "web_fetch":
		target = sanitizedURL(inputStringField(toolCall, "url"))
	case "delegate_task":
		target = inputStringField(toolCall, "agent")
	}
	return boundedTarget(target)
}

func toolStringField(toolCall *llm.ToolCall, key string) string {
	if toolCall == nil {
		return ""
	}
	if value := extractStringField(toolCall.Output, key); value != "" {
		return value
	}
	return inputStringField(toolCall, key)
}

func inputStringField(toolCall *llm.ToolCall, key string) string {
	if toolCall == nil || toolCall.Input == nil {
		return ""
	}
	value, _ := toolCall.Input[key].(string)
	return value
}

func joinTarget(path, detail string) string {
	if path == "" || path == "." {
		return detail
	}
	if detail == "" {
		return path
	}
	return path + " :: " + detail
}

func sanitizedURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func boundedTarget(target string) string {
	target = strings.Join(strings.Fields(target), " ")
	if sensitiveTargetPattern.MatchString(target) {
		return ""
	}
	if len(target) <= maxHistoricalToolTargetBytes {
		return target
	}

	limit := maxHistoricalToolTargetBytes - len("...")
	for limit > 0 && !utf8.RuneStart(target[limit]) {
		limit--
	}
	return target[:limit] + "..."
}

func (a *turnMemoryAccumulator) Build() *llm.TurnMemory {
	if a == nil || len(a.toolActivity) == 0 {
		return nil
	}

	return &llm.TurnMemory{
		ToolActivity: append([]llm.HistoricalToolActivity(nil), a.toolActivity...),
	}
}

func extractStringField(output any, key string) string {
	result, ok := output.(map[string]any)
	if !ok {
		return ""
	}
	value, _ := result[key].(string)
	return value
}

func extractIntField(output any, key string) (int, bool) {
	result, ok := output.(map[string]any)
	if !ok {
		return 0, false
	}

	switch value := result[key].(type) {
	case int:
		return value, true
	case int32:
		return int(value), true
	case int64:
		return int(value), true
	case float64:
		return int(value), true
	default:
		return 0, false
	}
}

func (m *replModel) startAssistantTurnMemory() {
	if m == nil {
		return
	}
	m.turnMemory = newTurnMemoryAccumulator()
}

func (m *replModel) recordHistoricalToolActivity(segments []streamSegment) {
	if m == nil || m.turnMemory == nil {
		return
	}
	m.turnMemory.RecordToolActivity(segments, m.turnMemoryWorkingDir())
}

func (m *replModel) rebuildTurnMemoryFromSegments(segments []streamSegment) {
	if m == nil || m.turnMemory == nil {
		return
	}

	m.turnMemory = newTurnMemoryAccumulator()
	m.recordHistoricalToolActivity(segments)
}

func (m *replModel) consumeTurnMemory() *llm.TurnMemory {
	if m == nil || m.turnMemory == nil {
		return nil
	}
	memory := m.turnMemory.Build()
	m.turnMemory = nil
	return memory
}

func (m *replModel) clearTurnMemory() {
	if m == nil {
		return
	}
	m.turnMemory = nil
}

func (m *replModel) turnMemoryWorkingDir() string {
	if m == nil {
		return ""
	}
	if m.appState != nil && m.appState.WorkingDir() != "" {
		return m.appState.WorkingDir()
	}
	if m.ctx != nil {
		return m.ctx.workingDir
	}
	return ""
}

func relativizePath(path string, workingDir string) string {
	if path == "" || workingDir == "" || !filepath.IsAbs(path) {
		return path
	}

	relativePath, err := filepath.Rel(workingDir, path)
	if err != nil || relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) {
		return path
	}
	return relativePath
}
