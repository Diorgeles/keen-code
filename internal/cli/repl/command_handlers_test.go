package repl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	replappstate "github.com/user/keen-code/internal/cli/repl/appstate"
	replcommands "github.com/user/keen-code/internal/cli/repl/commands"
	replwidgets "github.com/user/keen-code/internal/cli/repl/widgets"
	"github.com/user/keen-code/internal/config"
	"github.com/user/keen-code/internal/llm"
	"github.com/user/keen-code/internal/skills"
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
	m.textarea.SetValue(replcommands.Exit)

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
	m.textarea.SetValue(replcommands.Help)

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
	m.textarea.SetValue(replcommands.Model)

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
	m.textarea.SetValue(replcommands.Sessions)

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
	text := getHelpText(80)

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
	if !strings.Contains(text, "/skills") {
		t.Error("expected /skills in help text")
	}
}

func TestGetHelpTextWrapsCommandDescriptions(t *testing.T) {
	text := getHelpText(48)
	stripped := ansi.Strip(text)

	if !strings.Contains(stripped, "  Available Commands") {
		t.Fatalf("expected padded title, got %q", stripped)
	}
	if strings.Contains(stripped, "\n/show-thinking") {
		t.Fatalf("expected /show-thinking to stay in command column, got %q", stripped)
	}

	for _, line := range strings.Split(stripped, "\n") {
		if lipgloss.Width(line) > 46 {
			t.Fatalf("help line exceeds padded width (%d > %d): %q", lipgloss.Width(line), 46, line)
		}
	}
}

func TestDispatchCommand_UnknownCommandFallsThrough(t *testing.T) {
	m := newTestModel()

	_, _, handled := m.dispatchCommand("hello world")

	if handled {
		t.Error("expected unknown input to not be handled by dispatchCommand")
	}
}

func TestHandleSkillsCommandList(t *testing.T) {
	home := t.TempDir()
	work := t.TempDir()
	t.Setenv("HOME", home)
	skillDir := filepath.Join(work, ".agents", "skills", "demo")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: demo\ndescription: Demo skill\n---\nBody"), 0644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	m := newTestModel()
	m.ctx.workingDir = work
	m.appState = replappstate.New(nil, work)
	m.textarea.SetValue("/skills list")
	newM, _ := m.handleEnterKey()

	out := newM.output.Join()
	stripped := ansi.Strip(out)
	if !strings.Contains(out, "\x1b[1;38;2;189;189;189m  Available Skills") || strings.Contains(stripped, "Available Skills:") {
		t.Fatalf("expected header-colored skills title without colon, got %q", out)
	}
	if !strings.Contains(out, "38;2;92;107;192mdemo") {
		t.Fatalf("expected primary-colored skill name, got %q", out)
	}
	for _, expected := range []string{"Skill", "Status", "Description", "────", "demo", "✓ enabled", "Demo skill"} {
		if !strings.Contains(stripped, expected) {
			t.Fatalf("expected %q in skills list output, got %q", expected, stripped)
		}
	}
}

func TestHandleSkillsCommandListStylesDisabledStatus(t *testing.T) {
	home := t.TempDir()
	work := t.TempDir()
	t.Setenv("HOME", home)
	skillDir := filepath.Join(work, ".agents", "skills", "demo")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: demo\ndescription: Demo skill\n---\nBody"), 0644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	cfg := skills.Config{IsEnabled: map[string]bool{"demo": false}}
	if err := skills.SaveConfig(cfg); err != nil {
		t.Fatalf("save skills config: %v", err)
	}

	m := newTestModel()
	m.ctx.workingDir = work
	m.appState = replappstate.New(nil, work)
	m.textarea.SetValue("/skills list")
	newM, _ := m.handleEnterKey()

	out := newM.output.Join()
	if !strings.Contains(out, "38;2;255;179;0m✗ disabled") {
		t.Fatalf("expected accent-colored disabled status, got %q", out)
	}
	if !strings.Contains(ansi.Strip(out), "✗ disabled") {
		t.Fatalf("expected disabled status in skills list, got %q", out)
	}
}

func TestHandleSkillsCommandListWrapsLongDescriptions(t *testing.T) {
	home := t.TempDir()
	work := t.TempDir()
	t.Setenv("HOME", home)
	skillDir := filepath.Join(work, ".agents", "skills", "demo")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	description := "This skill has a very long description that should wrap within the viewport boundary instead of overflowing horizontally."
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: demo\ndescription: "+description+"\n---\nBody"), 0644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	m := newTestModel()
	m.ctx.workingDir = work
	m.appState = replappstate.New(nil, work)
	m.width = 50
	m.viewport.SetWidth(50)
	m.output.SetWidth(50)
	m.textarea.SetValue("/skills list")
	newM, _ := m.handleEnterKey()

	for _, line := range strings.Split(ansi.Strip(newM.output.Join()), "\n") {
		if !strings.Contains(line, "demo") && !strings.Contains(line, description[:10]) {
			continue
		}
		if lipgloss.Width(line) > 48 {
			t.Fatalf("skill line exceeds padded width (%d > %d): %q", lipgloss.Width(line), 48, line)
		}
	}
}

func TestHandleSkillsCommandDisable(t *testing.T) {
	home := t.TempDir()
	work := t.TempDir()
	t.Setenv("HOME", home)
	skillDir := filepath.Join(work, ".agents", "skills", "demo")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: demo\ndescription: Demo skill\n---\nBody"), 0644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	m := newTestModel()
	m.ctx.workingDir = work
	m.appState = replappstate.New(nil, work)
	m.textarea.SetValue("/skills demo disable")
	newM, _ := m.handleEnterKey()

	if !strings.Contains(newM.output.Join(), "Skill \"demo\" disabled") {
		t.Fatalf("expected disable confirmation, got %q", newM.output.Join())
	}
	if newM.appState.GetSkillsConfig().Enabled("demo") {
		t.Fatal("expected appstate config to disable skill")
	}
}

func TestHandleSkillsCommandReload(t *testing.T) {
	home := t.TempDir()
	work := t.TempDir()
	t.Setenv("HOME", home)

	m := newTestModel()
	m.ctx.workingDir = work
	m.appState = replappstate.New(nil, work)

	writeSkillDir := filepath.Join(work, ".agents", "skills", "demo")
	if err := os.MkdirAll(writeSkillDir, 0755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(writeSkillDir, "SKILL.md"), []byte("---\nname: demo\ndescription: Demo skill\n---\nBody"), 0644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	m.textarea.SetValue("/skills reload")
	newM, _ := m.handleEnterKey()

	if !strings.Contains(newM.output.Join(), "Skills reloaded") {
		t.Fatalf("expected reload confirmation, got %q", newM.output.Join())
	}
	if _, ok := skills.Find(newM.appState.GetSkills().Skills, "demo"); !ok {
		t.Fatal("expected reloaded appstate to include new skill")
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
	client := &mockLLMClient{}
	m.appState = replappstate.New(client, "")
	m.appState.AddMessage(llm.RoleUser, "previous")
	m.textarea.SetValue(replcommands.Clear)

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
	if len(newM.appState.GetMessages()) != 0 {
		t.Fatal("expected messages to be cleared")
	}
	if client.resetCount != 1 {
		t.Fatalf("expected LLM client reset once, got %d", client.resetCount)
	}
}

func TestHandleEnterKey_LogoutCommand_NoProvider(t *testing.T) {
	m := newTestModel()
	m.ctx.cfg = &config.ResolvedConfig{Provider: ""}
	m.textarea.SetValue(replcommands.Logout)

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
	client := &mockLLMClient{}
	m.appState = replappstate.New(client, "")
	m.textarea.SetValue(replcommands.New)

	newM, cmd := m.handleEnterKey()

	if cmd != nil {
		t.Fatal("expected nil cmd")
	}
	if !strings.Contains(newM.output.Join(), "New session started") {
		t.Fatalf("expected new session message, got %q", newM.output.Join())
	}
	if client.resetCount != 1 {
		t.Fatalf("expected LLM client reset once, got %d", client.resetCount)
	}
}

func TestHandleEnterKey_CompactCommandClientNotReady(t *testing.T) {
	m := newTestModel()
	m.ctx.cfg = &config.ResolvedConfig{}
	m.textarea.SetValue(replcommands.Compact)

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
	m.textarea.SetValue(replcommands.Sessions)

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
	m.textarea.SetValue(replcommands.Resume)

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

func TestHandleShowThinkingCommand_On(t *testing.T) {
	m := newTestModel()
	m.showThinking = false
	m.streamHandler.showThinking = false

	result := m.handleShowThinkingCommand("/show-thinking on")

	if !result.showThinking {
		t.Error("expected showThinking to be true after /show-thinking on")
	}
	if !result.streamHandler.showThinking {
		t.Error("expected streamHandler.showThinking to be true after /show-thinking on")
	}
	if !strings.Contains(result.output.Join(), "Thinking tokens shown") {
		t.Fatalf("expected confirmation message, got %q", result.output.Join())
	}
}

func TestHandleShowThinkingCommand_Off(t *testing.T) {
	m := newTestModel()

	result := m.handleShowThinkingCommand("/show-thinking off")

	if result.showThinking {
		t.Error("expected showThinking to be false after /show-thinking off")
	}
	if result.streamHandler.showThinking {
		t.Error("expected streamHandler.showThinking to be false after /show-thinking off")
	}
	if !strings.Contains(result.output.Join(), "Thinking tokens hidden") {
		t.Fatalf("expected confirmation message, got %q", result.output.Join())
	}
}

func TestHandleShowThinkingCommand_NoArgShowsStatus(t *testing.T) {
	m := newTestModel()

	result := m.handleShowThinkingCommand("/show-thinking")
	if !strings.Contains(result.output.Join(), "shown") {
		t.Fatalf("expected status message for shown state, got %q", result.output.Join())
	}

	m2 := newTestModel()
	m2.showThinking = false
	m2.streamHandler.showThinking = false

	result2 := m2.handleShowThinkingCommand("/show-thinking")
	if !strings.Contains(result2.output.Join(), "hidden") {
		t.Fatalf("expected status message for hidden state, got %q", result2.output.Join())
	}
}

func TestHandleShowThinkingCommand_PersistsToGlobalConfig(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	m := newTestModel()
	m.ctx.globalCfg = &config.GlobalConfig{}
	m.ctx.loader = config.NewLoader()

	_ = m.handleShowThinkingCommand("/show-thinking off")

	if m.ctx.globalCfg.ShowThinking == nil || *m.ctx.globalCfg.ShowThinking {
		t.Error("expected globalCfg.ShowThinking to be false after /show-thinking off")
	}

	_ = m.handleShowThinkingCommand("/show-thinking on")

	if m.ctx.globalCfg.ShowThinking == nil || !*m.ctx.globalCfg.ShowThinking {
		t.Error("expected globalCfg.ShowThinking to be true after /show-thinking on")
	}
}

func TestHandleEnterKey_BtwCommandStartsStream(t *testing.T) {
	m := newTestModel()
	m.ctx.cfg = &config.ResolvedConfig{APIKey: "key", Model: "model"}
	m.appState = replappstate.New(&mockLLMClient{}, "")
	m.appState.AddMessage(llm.RoleUser, "context message")
	m.btwStreamHandler = NewStreamHandler(nil)
	m.textarea.SetValue("/btw what is this?")

	newM, cmd := m.handleEnterKey()

	if !newM.isBtw {
		t.Fatal("expected btw overlay to be active")
	}
	if !newM.btwShowSpinner {
		t.Fatal("expected btw spinner to be visible")
	}
	if newM.btwQuestion != "what is this?" {
		t.Fatalf("expected btw question %q, got %q", "what is this?", newM.btwQuestion)
	}
	if newM.textarea.Value() != "" {
		t.Fatal("expected textarea to be reset")
	}
	if cmd == nil {
		t.Fatal("expected async btw command")
	}
}

func TestHandleEnterKey_BtwCommandDuringActiveStream(t *testing.T) {
	m := newTestModel()
	m.ctx.cfg = &config.ResolvedConfig{APIKey: "key", Model: "model"}
	m.appState = replappstate.New(&mockLLMClient{}, "")
	m.btwStreamHandler = NewStreamHandler(nil)
	eventCh := make(chan llm.StreamEvent)
	m.streamHandler.Start(eventCh, "Loading...")
	m.textarea.SetValue("/btw quick question")

	newM, cmd := m.handleEnterKey()

	if !newM.isBtw {
		t.Fatal("expected btw to work even during active main stream")
	}
	if cmd == nil {
		t.Fatal("expected async btw command")
	}
}

func TestHandleEnterKey_BtwCommandNoQuestion(t *testing.T) {
	m := newTestModel()
	m.btwStreamHandler = NewStreamHandler(nil)
	m.textarea.SetValue("/btw")

	newM, cmd := m.handleEnterKey()

	if newM.isBtw {
		t.Fatal("expected btw overlay not to show without history")
	}
	if cmd != nil {
		t.Fatal("expected nil cmd for /btw without question and no history")
	}
	found := false
	for _, line := range newM.output.GetLines() {
		if strings.Contains(line, "Usage:") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected usage hint for /btw without question")
	}
}

func TestHandleEnterKey_BtwCommandNoQuestionWithHistory(t *testing.T) {
	m := newTestModel()
	m.btwStreamHandler = NewStreamHandler(nil)
	m.btwHistory = []string{"previous answer"}
	m.textarea.SetValue("/btw")

	newM, _ := m.handleEnterKey()

	if !newM.isBtw {
		t.Fatal("expected btw overlay to show with history")
	}
	if newM.btwQuestion != "" {
		t.Fatal("expected empty btw question when just viewing history")
	}
}

func TestHandleEnterKey_BtwCommandClientNotReady(t *testing.T) {
	m := newTestModel()
	m.ctx.cfg = &config.ResolvedConfig{}
	m.btwStreamHandler = NewStreamHandler(nil)
	m.textarea.SetValue("/btw question")

	newM, cmd := m.handleEnterKey()

	if newM.isBtw {
		t.Fatal("expected btw not to activate when client is not ready")
	}
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
		t.Error("expected error about LLM client not initialized for /btw")
	}
}

func TestDispatchCommand_BtwNotHandled(t *testing.T) {
	m := newTestModel()

	_, _, handled := m.dispatchCommand("/btw question")

	if handled {
		t.Error("expected /btw to not be handled by dispatchCommand (handled via early return in handleEnterKey)")
	}
}

func TestDismissBtw_ResetsUserScrolled(t *testing.T) {
	m := newTestModel()
	m.isBtw = true
	m.btwShowSpinner = true
	m.btwStreamHandler = NewStreamHandler(nil)
	m.userScrolled = true
	m.viewport.SetContent("enough content\n" + strings.Repeat("line\n", 50))
	m.viewport.GotoBottom()

	m.dismissBtw()

	if m.isBtw {
		t.Fatal("expected btw to be dismissed")
	}
	if m.userScrolled {
		t.Fatal("expected userScrolled to be false when viewport is at bottom")
	}
}

func TestDismissBtw_PreservesUserScrolledWhenNotAtBottom(t *testing.T) {
	m := newTestModel()
	m.isBtw = true
	m.btwStreamHandler = NewStreamHandler(nil)
	m.viewport.SetHeight(5)
	m.viewport.SetContent(strings.Repeat("line\n", 50))
	m.viewport.GotoTop()

	m.dismissBtw()

	if m.userScrolled != true {
		t.Fatal("expected userScrolled to remain true when viewport is not at bottom")
	}
}
