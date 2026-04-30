package repl

import (
	"path/filepath"
	"strings"
	"testing"

	replappstate "github.com/user/keen-code/internal/cli/repl/appstate"
	replwidgets "github.com/user/keen-code/internal/cli/repl/widgets"
	"github.com/user/keen-code/internal/config"
	"github.com/user/keen-code/internal/llm"
	"github.com/user/keen-code/providers"
)

func TestHandleEnterKey_EmptyInput(t *testing.T) {
	m := newTestModel()
	m.textarea.SetValue("")

	newM, cmd := m.handleEnterKey()

	if cmd != nil {
		t.Error("expected nil cmd for empty input")
	}
	if len(newM.output.GetLines()) != 0 {
		t.Error("expected no output for empty input")
	}
}

func TestHandleEnterKey_ActiveStream(t *testing.T) {
	m := newTestModel()
	m.textarea.SetValue("some input")
	eventCh := make(chan llm.StreamEvent)
	m.streamHandler.Start(eventCh, "Loading...")

	newM, cmd := m.handleEnterKey()

	if cmd != nil {
		t.Error("expected nil cmd when stream is active")
	}
	if newM.textarea.Value() != "some input" {
		t.Error("expected textarea to remain unchanged when stream is active")
	}
}

func TestHandleEnterKey_ExitCommand(t *testing.T) {
	m := newTestModel()
	m.textarea.SetValue(exitCommand)

	newM, cmd := m.handleEnterKey()

	if !newM.quitting {
		t.Error("expected quitting to be true")
	}
	if cmd == nil {
		t.Fatal("expected tea.Quit cmd")
	}
}

func TestHandleEnterKey_HelpCommand(t *testing.T) {
	m := newTestModel()
	m.textarea.SetValue(helpCommand)

	newM, _ := m.handleEnterKey()

	if !strings.Contains(newM.output.Join(), "Available Commands") {
		t.Error("expected help text in output")
	}
	if newM.textarea.Value() != "" {
		t.Error("expected textarea to be reset after help command")
	}
}

func TestHandleEnterKey_ModelCommand(t *testing.T) {
	m := newTestModel()
	m.ctx.registry = &providers.Registry{Providers: []providers.Provider{}}
	m.ctx.globalCfg = &config.GlobalConfig{}
	m.ctx.loader = config.NewLoader()
	m.textarea.SetValue(modelCommand)

	newM, _ := m.handleEnterKey()

	if newM.modelSelection == nil {
		t.Error("expected model selection to be started")
	}
	if newM.textarea.Value() != "" {
		t.Error("expected textarea to be reset")
	}
}

func TestHandleEnterKey_SessionsCommand_EmptyState(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	m := newTestModel()
	m.sessions = newReplSessionState(filepath.Join(tmp, "project"))
	m.textarea.SetValue(sessionsCommand)

	newM, cmd := m.handleEnterKey()

	if cmd != nil {
		t.Fatal("expected nil cmd")
	}
	if newM.sessionPicker != nil {
		t.Fatal("expected no session picker for empty state")
	}
	if !strings.Contains(newM.output.Join(), "No saved sessions for this directory.") {
		t.Fatalf("expected empty state message, got %q", newM.output.Join())
	}
}

func TestHandleEnterKey_CompactCommandStartsCompaction(t *testing.T) {
	m := newTestModel()
	m.ctx.cfg = &config.ResolvedConfig{APIKey: "key", Model: "model"}
	m.appState = replappstate.New(&mockLLMClient{}, "")
	m.appState.AddMessage(llm.RoleUser, "hello")
	m.textarea.SetValue("/compact Keep business logic details")

	newM, cmd := m.handleEnterKey()

	if !newM.isCompacting {
		t.Fatal("expected compaction mode to start")
	}
	if !newM.showSpinner {
		t.Fatal("expected spinner to be visible during compaction")
	}
	if newM.loadingText != "Compacting..." {
		t.Fatalf("expected compaction loading text, got %q", newM.loadingText)
	}
	if newM.textarea.Value() != "" {
		t.Fatal("expected textarea to be reset")
	}
	if newM.compactionCancel == nil {
		t.Fatal("expected compaction cancel func to be set")
	}
	if !newM.streamHandler.IsActive() {
		t.Fatal("expected compaction to use the stream handler")
	}
	if cmd == nil {
		t.Fatal("expected async compaction command")
	}
}

func TestHandleEnterKey_ClientNotReady(t *testing.T) {
	m := newTestModel()
	m.textarea.SetValue("hello there")

	newM, _ := m.handleEnterKey()

	found := false
	for _, line := range newM.output.GetLines() {
		if strings.Contains(line, "LLM client not initialized") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected error about LLM client not initialized")
	}
	if newM.textarea.Value() != "" {
		t.Error("expected textarea to be reset")
	}
}

func TestGetHelpText(t *testing.T) {
	text := getHelpText()

	if !strings.Contains(text, "/compact") {
		t.Error("expected /compact in help text")
	}
	if !strings.Contains(text, "/help") {
		t.Error("expected /help in help text")
	}
	if !strings.Contains(text, "/model") {
		t.Error("expected /model in help text")
	}
	if !strings.Contains(text, "/exit") {
		t.Error("expected /exit in help text")
	}
	if !strings.Contains(text, "/resume") {
		t.Error("expected /resume in help text")
	}
	if !strings.Contains(text, "/sessions") {
		t.Error("expected /sessions in help text")
	}
}

func TestDispatchCommand_UnknownCommandFallsThrough(t *testing.T) {
	m := newTestModel()

	_, _, handled := m.dispatchCommand("hello world")

	if handled {
		t.Error("expected unknown input to not be handled by dispatchCommand")
	}
}

func TestDispatchCommand_SlashPrefixedNonCommandFallsThrough(t *testing.T) {
	m := newTestModel()

	_, _, handled := m.dispatchCommand("/unknown")

	if handled {
		t.Error("expected unknown slash command to not be handled by dispatchCommand")
	}
}

func TestHandleEnterKey_ClearCommand(t *testing.T) {
	m := newTestModel()
	m.textarea.SetValue(clearCommand)

	newM, cmd := m.handleEnterKey()

	if cmd != nil {
		t.Fatal("expected nil cmd")
	}
	if !strings.Contains(newM.output.Join(), "New session started") {
		t.Fatalf("expected new session message, got %q", newM.output.Join())
	}
	if newM.textarea.Value() != "" {
		t.Error("expected textarea to be reset")
	}
}

func TestHandleEnterKey_LogoutCommand_NoProvider(t *testing.T) {
	m := newTestModel()
	m.ctx.cfg = &config.ResolvedConfig{Provider: ""}
	m.textarea.SetValue(logoutCommand)

	newM, _ := m.handleEnterKey()

	found := false
	for _, line := range newM.output.GetLines() {
		if strings.Contains(line, "No provider is configured") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected error about no provider configured")
	}
}

func TestStartModelSelection_SetsModelSelection(t *testing.T) {
	m := newTestModel()
	m.ctx.registry = &providers.Registry{Providers: []providers.Provider{}}
	m.ctx.globalCfg = &config.GlobalConfig{}
	m.ctx.loader = config.NewLoader()

	result := m.startModelSelection()

	if result.modelSelection == nil {
		t.Error("expected modelSelection to be set")
	}
}

func TestHandleEnterKey_NewCommand(t *testing.T) {
	m := newTestModel()
	m.textarea.SetValue(newCommand)

	newM, cmd := m.handleEnterKey()

	if cmd != nil {
		t.Fatal("expected nil cmd")
	}
	if !strings.Contains(newM.output.Join(), "New session started") {
		t.Fatalf("expected new session message, got %q", newM.output.Join())
	}
}

func TestHandleEnterKey_CompactCommandClientNotReady(t *testing.T) {
	m := newTestModel()
	m.ctx.cfg = &config.ResolvedConfig{}
	m.textarea.SetValue(compactCommand)

	newM, cmd := m.handleEnterKey()

	if cmd != nil {
		t.Fatal("expected nil cmd")
	}
	found := false
	for _, line := range newM.output.GetLines() {
		if strings.Contains(line, "LLM client not initialized") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected error about LLM client not initialized for /compact")
	}
}

func TestHandleEnterKey_ThinkingCommandNoSupport(t *testing.T) {
	m := newTestModel()
	m.ctx.registry = &providers.Registry{
		Providers: []providers.Provider{
			{
				ID: "openai",
				Models: []providers.Model{
					{ID: "gpt-4", ContextWindow: 2000},
				},
			},
		},
	}
	m.ctx.cfg = &config.ResolvedConfig{Provider: "openai", Model: "gpt-4"}
	m.textarea.SetValue("/thinking high")

	newM, _ := m.handleEnterKey()

	found := false
	for _, line := range newM.output.GetLines() {
		if strings.Contains(line, "does not support configurable thinking") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected error about model not supporting thinking")
	}
}

func TestHandleEnterKey_SessionsCommandWithSessions(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	workingDir := filepath.Join(tmp, "project")

	m := newTestModel()
	m.ctx.registry = &providers.Registry{Providers: []providers.Provider{}}
	m.ctx.globalCfg = &config.GlobalConfig{}
	m.ctx.loader = config.NewLoader()
	m.sessions = newReplSessionState(workingDir)
	if err := m.sessions.appendUserMessage("saved prompt"); err != nil {
		t.Fatalf("append user message: %v", err)
	}
	m.sessions.resetSession()
	m.textarea.SetValue(sessionsCommand)

	newM, cmd := m.handleEnterKey()

	if cmd != nil {
		t.Fatal("expected nil cmd")
	}
	if newM.sessionPicker == nil {
		t.Fatal("expected session picker for saved sessions")
	}
	if newM.textarea.Value() != "" {
		t.Fatal("expected textarea to be reset")
	}
}

func TestHandleEnterKey_ResumeCommand(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	m := newTestModel()
	m.sessions = newReplSessionState(filepath.Join(tmp, "project"))
	m.textarea.SetValue(resumeCommand)

	newM, _ := m.handleEnterKey()

	if !strings.Contains(newM.output.Join(), "No saved sessions for this directory.") {
		t.Fatalf("expected empty state message for /resume, got %q", newM.output.Join())
	}
}

func TestStartModelSelection_CallsOnComplete(t *testing.T) {
	m := newTestModel()
	m.ctx.registry = &providers.Registry{Providers: []providers.Provider{}}
	m.ctx.globalCfg = &config.GlobalConfig{}
	m.ctx.loader = config.NewLoader()

	result := m.startModelSelection()
	if result.modelSelection == nil {
		t.Fatal("expected model selection to be set")
	}

	// Verify the model selection widget was initialized
	ms := result.modelSelection
	if ms.Step != replwidgets.StepProvider {
		t.Fatalf("expected model selection to start at provider step, got %d", ms.Step)
	}
}
