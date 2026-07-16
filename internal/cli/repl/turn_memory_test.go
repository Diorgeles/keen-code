package repl

import (
	"path/filepath"
	"testing"

	replappstate "github.com/user/keen-code/internal/cli/repl/appstate"
	reploutput "github.com/user/keen-code/internal/cli/repl/output"
	"github.com/user/keen-code/internal/llm"
)

func TestHandleLLMDone_AttachesTurnMemoryToAssistantMessage(t *testing.T) {
	workingDir := t.TempDir()
	sh := NewStreamHandler(nil)
	sh.Start(make(<-chan llm.StreamEvent), "Loading...")
	sh.HandleChunk("working")
	sh.HandleToolStart(&llm.ToolCall{Name: "edit_file", Input: map[string]any{"path": "nested/a.go"}})
	sh.HandleToolEnd(&llm.ToolCall{Name: "edit_file", Input: map[string]any{"path": "nested/a.go"}})
	sh.HandleChunk("done")

	m := replModel{
		streamHandler: sh,
		showSpinner:   true,
		width:         80,
		appState:      replappstate.New(nil, workingDir),
		output:        reploutput.NewOutputBuilder(80, ""),
	}
	m.startAssistantTurnMemory()
	relativeFile := filepath.Join("nested", "a.go")
	sh.HandleToolEnd(&llm.ToolCall{
		Name:   "edit_file",
		Output: map[string]any{"file_changed": filepath.Join(workingDir, relativeFile)},
	})
	sh.HandleBashStart("go test ./...", "")
	sh.HandleBashEnd(&llm.ToolCall{
		Name:   "bash",
		Output: map[string]any{"failed_command": "go test ./...", "exit_code": 1},
	})

	updated, _ := m.handleLLMDone()

	messages := updated.appState.GetMessages()
	if len(messages) != 1 {
		t.Fatalf("expected one stored message, got %#v", messages)
	}
	if messages[0].TurnMemory == nil {
		t.Fatal("expected assistant turn memory")
	}
	activities := messages[0].TurnMemory.ToolActivity
	if len(activities) != 3 {
		t.Fatalf("unexpected tool activity %#v", activities)
	}
	if activities[1].FileChanged != relativeFile {
		t.Fatalf("unexpected file change %#v", activities[1])
	}
	if activities[2].FailedCommand != "go test ./..." || activities[2].ExitCode == nil || *activities[2].ExitCode != 1 {
		t.Fatalf("unexpected failed bash activity %#v", activities[2])
	}
}

func TestCollectHistoricalToolActivity_UsesRelativeChangedPath(t *testing.T) {
	workingDir := t.TempDir()
	targetPath := filepath.Join(workingDir, "dir", "file.go")
	activities := collectHistoricalToolActivity([]streamSegment{{
		kind: segmentToolEnd,
		toolCall: &llm.ToolCall{
			Name:   "write_file",
			Output: map[string]any{"file_changed": targetPath},
		},
	}}, workingDir)

	if len(activities) != 1 || activities[0].FileChanged != filepath.Join("dir", "file.go") {
		t.Fatalf("expected relative changed file path, got %#v", activities)
	}
}

func TestCollectHistoricalToolActivity_RecordsOffsetsTargetsAndStatus(t *testing.T) {
	workingDir := t.TempDir()
	segments := []streamSegment{
		{kind: segmentToolEnd, toolCall: &llm.ToolCall{Name: "glob", Input: map[string]any{"path": workingDir, "pattern": "**/*.go"}}},
		{kind: segmentAssistant, content: "Inspecting. "},
		{kind: segmentToolEnd, toolCall: &llm.ToolCall{Name: "read_file", Input: map[string]any{"path": filepath.Join(workingDir, "a.go")}}},
		{kind: segmentToolEnd, toolCall: &llm.ToolCall{Name: "edit_file", Error: "failed", Input: map[string]any{"path": filepath.Join(workingDir, "a.go")}}},
		{kind: segmentAssistant, content: "Done."},
		{kind: segmentBash, command: "go test ./...", toolCall: &llm.ToolCall{Name: "bash"}},
	}

	got := collectHistoricalToolActivity(segments, workingDir)
	if len(got) != 4 {
		t.Fatalf("expected four activities, got %#v", got)
	}
	if got[0].TextOffset != 0 || got[0].Target != "**/*.go" {
		t.Fatalf("unexpected glob activity %#v", got[0])
	}
	if got[1].TextOffset != len("Inspecting. ") || got[1].Target != "a.go" || got[1].Status != "success" {
		t.Fatalf("unexpected read activity %#v", got[1])
	}
	if got[2].TextOffset != got[1].TextOffset || got[2].Status != "error" {
		t.Fatalf("unexpected edit activity %#v", got[2])
	}
	if got[3].TextOffset != len("Inspecting. Done.") || got[3].Target != "go test ./..." {
		t.Fatalf("unexpected bash activity %#v", got[3])
	}
}

func TestCollectHistoricalToolActivity_ExtractsMCPWithoutArguments(t *testing.T) {
	segments := []streamSegment{{
		kind: segmentToolEnd,
		toolCall: &llm.ToolCall{
			Name: "call_mcp_tool",
			Input: map[string]any{
				"server":    "requested-server",
				"tool":      "requested-tool",
				"arguments": map[string]any{"secret": "do not retain"},
			},
			Output: map[string]any{
				"server":  "context7",
				"tool":    "query-docs",
				"content": "do not retain",
			},
		},
	}}

	got := collectHistoricalToolActivity(segments, "")
	if len(got) != 1 {
		t.Fatalf("expected one activity, got %#v", got)
	}
	if got[0].Server != "context7" || got[0].Tool != "query-docs" || got[0].Target != "" {
		t.Fatalf("unexpected MCP activity %#v", got[0])
	}
}

func TestRebuildTurnMemoryFromSegments_KeepsSurvivingActivity(t *testing.T) {
	workingDir := t.TempDir()
	m := replModel{appState: replappstate.New(nil, workingDir)}
	m.startAssistantTurnMemory()
	m.turnMemory.toolActivity = []llm.HistoricalToolActivity{{Tool: "edit_file", FileChanged: "abandoned.go"}}

	m.rebuildTurnMemoryFromSegments([]streamSegment{
		{kind: segmentToolEnd, toolCall: &llm.ToolCall{Name: "write_file", Output: map[string]any{"file_changed": "kept.go"}}},
	})
	memory := m.consumeTurnMemory()
	if memory == nil || len(memory.ToolActivity) != 1 || memory.ToolActivity[0].FileChanged != "kept.go" {
		t.Fatalf("expected only surviving activity, got %#v", memory)
	}
}

func TestCollectHistoricalToolActivity_DoesNotInferRetainedOutcomesFromArguments(t *testing.T) {
	activities := collectHistoricalToolActivity([]streamSegment{
		{kind: segmentToolEnd, toolCall: &llm.ToolCall{Name: "write_file", Input: map[string]any{"path": "a.go"}, Output: map[string]any{"path": "a.go"}}},
		{kind: segmentBash, command: "go test ./...", toolCall: &llm.ToolCall{Name: "bash", Input: map[string]any{"command": "go test ./..."}, Output: map[string]any{"exit_code": 1}}},
	}, "")

	if activities[0].FileChanged != "" {
		t.Fatalf("expected file_changed to come only from tool output, got %#v", activities[0])
	}
	if activities[1].FailedCommand != "" || activities[1].ExitCode == nil || *activities[1].ExitCode != 1 {
		t.Fatalf("expected failed command to come only from tool output, got %#v", activities[1])
	}
}

func TestCollectHistoricalToolActivity_SanitizesTargets(t *testing.T) {
	segments := []streamSegment{
		{kind: segmentToolEnd, toolCall: &llm.ToolCall{Name: "web_fetch", Input: map[string]any{"url": "https://user:pass@example.com/docs?token=secret#section"}}},
		{kind: segmentBash, command: "curl -H 'Authorization: secret' example.com", toolCall: &llm.ToolCall{Name: "bash"}},
	}

	got := collectHistoricalToolActivity(segments, "")
	if got[0].Target != "https://example.com/docs" {
		t.Fatalf("expected sanitized URL, got %#v", got[0])
	}
	if got[1].Target != "" {
		t.Fatalf("expected sensitive command target to be omitted, got %#v", got[1])
	}
}
