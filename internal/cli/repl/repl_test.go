package repl

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	replappstate "github.com/user/keen-code/internal/cli/repl/appstate"
	reploutput "github.com/user/keen-code/internal/cli/repl/output"
	replpermissions "github.com/user/keen-code/internal/cli/repl/permissions"
	repltheme "github.com/user/keen-code/internal/cli/repl/theme"
	repltooling "github.com/user/keen-code/internal/cli/repl/tooling"
	replwidgets "github.com/user/keen-code/internal/cli/repl/widgets"
	"github.com/user/keen-code/internal/config"
	"github.com/user/keen-code/internal/llm"
	"github.com/user/keen-code/internal/session"
	"github.com/user/keen-code/internal/tools"
	"github.com/user/keen-code/providers"
)

func newTestModel() replModel {
	ta := textarea.New()
	ta.Focus()
	ta.SetWidth(80)
	ta.SetHeight(maxHeight)
	ta.MaxHeight = 0
	vp := viewport.New(viewport.WithWidth(80), viewport.WithHeight(20))
	return replModel{
		textarea:            ta,
		viewport:            vp,
		ctx:                 &replContext{cfg: &config.ResolvedConfig{}},
		mode:                llm.ModeBuild,
		appState:            replappstate.New(nil, ""),
		output:              reploutput.NewOutputBuilder(80, ""),
		streamHandler:       NewStreamHandler(nil),
		permissionRequester: replpermissions.NewRequester(nil),
		projectPerms:        config.NewProjectPermissions(),
		diffEmitter:         repltooling.NewDiffEmitter(),
		sessions:            newReplSessionState(""),
		spinner:             spinner.New(),
		width:               80,
		height:              30,
		showThinking:        true,
	}
}

func scrollViewportAwayFromBottom(t *testing.T, m *replModel) int {
	t.Helper()

	m.viewport.SetHeight(6)
	for range 40 {
		m.output.AddLine("existing output")
	}
	m.updateViewportContent()
	m.viewport.GotoBottom()
	if m.viewport.AtTop() {
		t.Fatal("expected test viewport to be scrollable")
	}
	m.viewport.ScrollUp(4)
	if m.viewport.AtBottom() {
		t.Fatal("expected test viewport to be above bottom")
	}
	m.userScrolled = true
	return m.viewport.YOffset()
}

func TestUpdate_InlinePermission_AllowsToolStartEvent(t *testing.T) {
	sh := NewStreamHandler(nil)
	eventCh := make(chan llm.StreamEvent)
	sh.Start(eventCh, "Loading...")

	req := &replpermissions.Request{
		RequestID:    "1",
		ToolName:     "read_file",
		Path:         "../foo.txt",
		ResolvedPath: "/tmp/foo.txt",
		Status:       replpermissions.StatusPending,
		ResponseChan: make(chan bool, 1),
	}
	sh.HandlePermissionRequest(req)

	m := replModel{
		streamHandler: sh,
		showSpinner:   true,
		width:         80,
		output:        reploutput.NewOutputBuilder(80, ""),
	}

	toolCall := &llm.ToolCall{Name: "read_file", Input: map[string]any{"path": "../foo.txt"}}
	updatedModel, cmd := m.Update(llmToolStartMsg{toolCall: toolCall})

	updated, ok := updatedModel.(*replModel)
	if !ok {
		t.Fatalf("expected *replModel, got %T", updatedModel)
	}

	if !updated.showSpinner {
		t.Error("expected showSpinner to remain true after tool start while permission is pending")
	}

	if len(updated.output.GetLines()) != 0 {
		t.Errorf("expected no persisted output line for tool start, got %d", len(updated.output.GetLines()))
	}

	if cmd == nil {
		t.Error("expected non-nil cmd when handling tool start event")
	}
}

func TestAdjustTextareaHeight(t *testing.T) {
	m := newTestModel()
	m.textarea.SetHeight(1)
	m.adjustTextareaHeight()

	if m.textarea.Height() != maxHeight {
		t.Errorf("expected textarea height %d, got %d", maxHeight, m.textarea.Height())
	}
	expectedVPHeight := m.height - m.textarea.Height() - 4
	if m.viewport.Height() != expectedVPHeight {
		t.Errorf("expected viewport height %d, got %d", expectedVPHeight, m.viewport.Height())
	}
}

func TestActivateSkillInput(t *testing.T) {
	home := t.TempDir()
	work := t.TempDir()
	t.Setenv("HOME", home)
	skillDir := filepath.Join(work, ".agents", "skills", "demo")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: demo\ndescription: Demo skill\n---\n# Demo\nargs=$ARGUMENTS"), 0644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	m := newTestModel()
	m.ctx.workingDir = work
	m.appState = replappstate.New(nil, work)
	activated, ok := m.activateSkillInput("/demo thing")
	if !ok {
		t.Fatal("expected skill activation")
	}
	if !strings.Contains(activated, "[Activate skill: demo]") || !strings.Contains(activated, "# Demo") || !strings.Contains(activated, "args=thing") {
		t.Fatalf("unexpected activation message: %q", activated)
	}
}

func TestActivateSkillInput_UsesFrontmatterNameNotDir(t *testing.T) {
	home := t.TempDir()
	work := t.TempDir()
	t.Setenv("HOME", home)
	skillDir := filepath.Join(work, ".agents", "skills", "any-dir")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: real-name\ndescription: Demo skill\n---\nbody=$ARGUMENTS"), 0644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	m := newTestModel()
	m.ctx.workingDir = work
	m.appState = replappstate.New(nil, work)

	if _, ok := m.activateSkillInput("/any-dir foo"); ok {
		t.Fatal("expected /<dirname> to NOT activate")
	}

	activated, ok := m.activateSkillInput("/real-name foo")
	if !ok {
		t.Fatal("expected /<frontmatter-name> to activate")
	}
	if !strings.Contains(activated, "[Activate skill: real-name]") || !strings.Contains(activated, "body=foo") {
		t.Fatalf("unexpected activation message: %q", activated)
	}
}

func TestAdjustTextareaHeight_ZeroHeight(t *testing.T) {
	m := newTestModel()
	m.height = 0
	m.adjustTextareaHeight()

	if m.textarea.Height() != maxHeight {
		t.Errorf("expected textarea height %d, got %d", maxHeight, m.textarea.Height())
	}
}

func TestIsAtTopOfInput(t *testing.T) {
	m := newTestModel()
	m.textarea.SetValue("line1")

	if !m.isAtTopOfInput() {
		t.Error("expected isAtTopOfInput to be true for single line")
	}
}

func TestIsAtBottomOfInput(t *testing.T) {
	m := newTestModel()
	m.textarea.SetValue("line1")

	if !m.isAtBottomOfInput() {
		t.Error("expected isAtBottomOfInput to be true for single line")
	}
}

func TestUpdateNormalMode_WindowResize(t *testing.T) {
	m := newTestModel()

	resizeMsg := tea.WindowSizeMsg{Width: 100, Height: 40}
	newM, cmd := m.updateNormalMode(resizeMsg)

	if newM.width != 100 {
		t.Errorf("expected width 100, got %d", newM.width)
	}
	if newM.height != 40 {
		t.Errorf("expected height 40, got %d", newM.height)
	}
	if cmd != nil {
		t.Error("expected nil cmd for window resize")
	}
}

func TestUpdateNormalMode_WindowResizeWhileModelSelectionActive(t *testing.T) {
	m := newTestModel()
	m.modelSelection = &replwidgets.Model{}

	resizeMsg := tea.WindowSizeMsg{Width: 100, Height: 40}
	newM, cmd := m.updateNormalMode(resizeMsg)

	if newM.width != 100 {
		t.Errorf("expected width 100, got %d", newM.width)
	}
	if newM.height != 40 {
		t.Errorf("expected height 40, got %d", newM.height)
	}
	if newM.viewport.Height() != 33 {
		t.Errorf("expected viewport height 33, got %d", newM.viewport.Height())
	}
	if cmd != nil {
		t.Error("expected nil cmd for window resize")
	}
}

func TestUpdateViewportContent_UsesViewportWidthWhenModelStartsWithoutResize(t *testing.T) {
	m := newTestModel()
	m.width = 0
	eventCh := make(chan llm.StreamEvent)
	m.streamHandler.Start(eventCh, "Loading...")
	m.streamHandler.HandleReasoningChunk("thinking")

	m.updateViewportContent()

	content := m.viewport.View()
	if strings.Contains(content, "  t\n  h\n  i") {
		t.Fatalf("expected reasoning to use viewport width fallback, got %q", content)
	}
	if !strings.Contains(content, "thinking") {
		t.Fatalf("expected reasoning content to be rendered, got %q", content)
	}
}

func TestBuildInitialScreen_HighlightsModelOnly(t *testing.T) {
	ctx := &replContext{
		version:    "0.2.1",
		workingDir: "/tmp/project",
		cfg: &config.ResolvedConfig{
			Provider: "openai",
			Model:    "gpt-5.4",
		},
	}

	lines := buildInitialScreen(ctx)
	rendered := strings.Join(lines, "\n")

	if strings.Contains(rendered, repltheme.HighlightStyle.Render("openai")) {
		t.Fatalf("expected provider in initial screen to not use highlight style, got %q", rendered)
	}
	if !strings.Contains(rendered, repltheme.HighlightStyle.Render("gpt-5.4")) {
		t.Fatalf("expected model in initial screen to use highlight style, got %q", rendered)
	}
}

func TestInitialModel_DimsBlurredPromptGlyph(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := initialModel(&replContext{version: "test", workingDir: t.TempDir(), cfg: &config.ResolvedConfig{}}, nil, false)
	styles := m.textarea.Styles()

	got := styles.Blurred.Prompt.Render(" ▶ ")
	want := repltheme.InputRuleBlurredStyle.Render(" ▶ ")
	if got != want {
		t.Fatalf("expected blurred prompt glyph to use blurred input style, got %q want %q", got, want)
	}
}

func TestRenderInputArea_UsesViewportWidthRules(t *testing.T) {
	focusedWide := renderInputArea("▶ hello", 80, true)
	blurredWide := renderInputArea("▶ hello", 80, false)
	if focusedWide == blurredWide {
		t.Fatal("expected focused and blurred input areas to render differently")
	}

	wide := focusedWide
	wideLines := strings.Split(strings.TrimRight(wide, "\n"), "\n")
	if len(wideLines) != 3 {
		t.Fatalf("expected 3 input-area lines, got %v", wideLines)
	}
	if !strings.Contains(wideLines[0], "─") || !strings.Contains(wideLines[2], "─") {
		t.Fatalf("expected top and bottom input rules, got %q", wide)
	}
	if wideRuleWidth := lipgloss.Width(wideLines[0]); wideRuleWidth != 80 {
		t.Fatalf("expected wide input rules to match viewport width, got width %d", wideRuleWidth)
	}

	narrow := renderInputArea("▶ hi", 24, true)
	narrowLines := strings.Split(strings.TrimRight(narrow, "\n"), "\n")
	if len(narrowLines) != 3 {
		t.Fatalf("expected 3 narrow input-area lines, got %v", narrowLines)
	}
	if narrowRuleWidth := lipgloss.Width(narrowLines[0]); narrowRuleWidth != 24 {
		t.Fatalf("expected narrow input rules to match viewport width, got width %d", narrowRuleWidth)
	}
}

func TestFormatModelSelectionCard_UsesViewportWidthRules(t *testing.T) {
	card := formatModelSelectionCard(&replwidgets.Model{}, 24)
	lines := strings.Split(strings.TrimRight(card, "\n"), "\n")
	nonEmpty := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			nonEmpty = append(nonEmpty, line)
		}
	}
	if len(nonEmpty) < 2 {
		t.Fatalf("expected ruled model selection output, got %v", nonEmpty)
	}
	if !strings.Contains(nonEmpty[0], "─") || !strings.Contains(nonEmpty[len(nonEmpty)-1], "─") {
		t.Fatalf("expected top and bottom rules, got %q", card)
	}
	if ruleWidth := lipgloss.Width(nonEmpty[0]); ruleWidth != 24 {
		t.Fatalf("expected rules to match viewport width, got width %d", ruleWidth)
	}
	if strings.TrimSpace(lines[2]) != "" {
		t.Fatalf("expected blank line after top rule, got %q", lines[2])
	}
	if strings.TrimSpace(lines[len(lines)-2]) != "" {
		t.Fatalf("expected blank line before bottom rule, got %q", lines[len(lines)-2])
	}
}

func TestUpdate_RoutesToNormalMode(t *testing.T) {
	m := newTestModel()

	result, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	updated := result.(*replModel)

	if updated.width != 100 {
		t.Errorf("expected width 100, got %d", updated.width)
	}
}

func TestUpdate_RoutesToPermissionHandling(t *testing.T) {
	m := newTestModel()
	eventCh := make(chan llm.StreamEvent)
	m.streamHandler.Start(eventCh, "Loading...")

	req := &replpermissions.Request{
		RequestID:    "1",
		ToolName:     "read_file",
		Path:         "foo.txt",
		ResolvedPath: "/resolved/foo.txt",
		Status:       replpermissions.StatusPending,
		ResponseChan: make(chan bool, 1),
	}
	m.streamHandler.HandlePermissionRequest(req)

	if !m.streamHandler.HasPendingPermission() {
		t.Fatal("expected pending permission")
	}

	// Pressing 'j' should move the cursor down
	result, _ := m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	updated := result.(*replModel)

	if !updated.streamHandler.HasPendingPermission() {
		t.Error("expected pending permission to remain after 'j' key")
	}
}

func TestHandleLLMStreamMsg_UnknownMsg(t *testing.T) {
	m := newTestModel()
	_, _, handled := m.handleLLMStreamMsg(tea.WindowSizeMsg{})

	if handled {
		t.Error("expected unknown msg to not be handled")
	}
}

func TestHandleLLMStreamMsg_RoutesChunk(t *testing.T) {
	m := newTestModel()
	eventCh := make(chan llm.StreamEvent)
	m.streamHandler.Start(eventCh, "Loading...")
	m.showSpinner = true

	newM, _, handled := m.handleLLMStreamMsg(llmChunkMsg("hello"))

	if !handled {
		t.Error("expected chunk msg to be handled")
	}
	if !newM.showSpinner {
		t.Error("expected showSpinner to remain true after chunk")
	}
}

func TestUpdateNormalMode_PermissionReadyRendersImmediately(t *testing.T) {
	m := newTestModel()
	eventCh := make(chan llm.StreamEvent)
	m.streamHandler.Start(eventCh, "Loading...")
	m.showSpinner = true

	req := &replpermissions.Request{
		RequestID:    "1",
		ToolName:     "read_file",
		Path:         "../foo.txt",
		ResolvedPath: "/tmp/foo.txt",
		Status:       replpermissions.StatusPending,
		ResponseChan: make(chan bool, 1),
	}

	newM, cmd := m.updateNormalMode(permissionReadyMsg{req: req})

	if !newM.streamHandler.HasPendingPermission() {
		t.Fatal("expected pending permission to be rendered immediately")
	}
	if !newM.showSpinner {
		t.Fatal("expected spinner to remain active when permission prompt appears")
	}
	if cmd == nil {
		t.Fatal("expected async waiter to be re-armed")
	}
}

func TestUpdateNormalMode_PermissionReadyPreservesUserScroll(t *testing.T) {
	m := newTestModel()
	eventCh := make(chan llm.StreamEvent)
	m.streamHandler.Start(eventCh, "Loading...")
	m.showSpinner = true
	offset := scrollViewportAwayFromBottom(t, &m)

	req := &replpermissions.Request{
		RequestID:    "1",
		ToolName:     "read_file",
		Path:         "../foo.txt",
		ResolvedPath: "/tmp/foo.txt",
		Status:       replpermissions.StatusPending,
		ResponseChan: make(chan bool, 1),
	}

	newM, _ := m.updateNormalMode(permissionReadyMsg{req: req})

	if got := newM.viewport.YOffset(); got != offset {
		t.Fatalf("expected permission prompt to preserve scroll offset %d, got %d", offset, got)
	}
}

func TestUpdateNormalMode_DiffReadyRendersImmediately(t *testing.T) {
	m := newTestModel()
	eventCh := make(chan llm.StreamEvent)
	m.streamHandler.Start(eventCh, "Loading...")

	done := make(chan struct{})
	req := repltooling.DiffRequest{
		Lines: []tools.EditDiffLine{
			{Kind: tools.DiffLineAdded, Content: "hello", NewLineNum: 1},
		},
		Done: done,
	}

	newM, cmd := m.updateNormalMode(diffReadyMsg{req: req})

	if len(newM.streamHandler.segments) != 1 || newM.streamHandler.segments[0].kind != segmentDiff {
		t.Fatal("expected diff segment to be rendered immediately")
	}
	select {
	case <-done:
	default:
		t.Fatal("expected diff emitter to be unblocked immediately")
	}
	if cmd == nil {
		t.Fatal("expected async waiter to be re-armed")
	}
}

func TestUpdateNormalMode_DiffReadyPreservesUserScroll(t *testing.T) {
	m := newTestModel()
	eventCh := make(chan llm.StreamEvent)
	m.streamHandler.Start(eventCh, "Loading...")
	offset := scrollViewportAwayFromBottom(t, &m)

	done := make(chan struct{})
	req := repltooling.DiffRequest{
		Lines: []tools.EditDiffLine{
			{Kind: tools.DiffLineAdded, Content: "hello", NewLineNum: 1},
		},
		Done: done,
	}

	newM, _ := m.updateNormalMode(diffReadyMsg{req: req})

	if got := newM.viewport.YOffset(); got != offset {
		t.Fatalf("expected diff prompt to preserve scroll offset %d, got %d", offset, got)
	}
}

func TestHandleUpdateCheckMsg_PreservesUserScroll(t *testing.T) {
	m := newTestModel()
	offset := scrollViewportAwayFromBottom(t, &m)

	m.handleUpdateCheckMsg(updateCheckMsg{latest: "9.9.9"})

	if got := m.viewport.YOffset(); got != offset {
		t.Fatalf("expected update notice to preserve scroll offset %d, got %d", offset, got)
	}
}

func TestReplayLoadedSession_RebuildsOutputAndConversation(t *testing.T) {
	m := newTestModel()
	loaded := &session.LoadedSession{
		Events: []session.Event{
			{
				Kind:        session.KindUserMessage,
				UserMessage: &session.MessagePayload{Content: "hello"},
			},
			{
				Kind: session.KindAssistantTurn,
				AssistantTurn: &session.AssistantTurnPayload{
					Transcript: []session.TranscriptItem{
						{
							Kind:    session.TranscriptItemReasoning,
							Content: "thinking",
						},
						{
							Kind:    session.TranscriptItemText,
							Content: "world",
						},
					},
					Message: "world",
				},
			},
			{
				Kind: session.KindCompactionApplied,
				CompactionApplied: &session.CompactionAppliedPayload{
					Status: "Context compacted.",
					Transcript: []session.TranscriptItem{
						{
							Kind:    session.TranscriptItemText,
							Content: "summary",
						},
					},
					Messages: []llm.Message{
						{Role: llm.RoleUser, Content: "summary"},
					},
				},
			},
		},
	}

	m.replayLoadedSession(loaded)

	if !strings.Contains(m.output.Join(), "hello") {
		t.Fatalf("expected replayed user message, got %q", m.output.Join())
	}
	if !strings.Contains(m.output.Join(), "thinking") {
		t.Fatalf("expected replayed reasoning, got %q", m.output.Join())
	}
	if !strings.Contains(m.output.Join(), "world") {
		t.Fatalf("expected replayed assistant, got %q", m.output.Join())
	}
	if !strings.Contains(m.output.Join(), "summary") {
		t.Fatalf("expected replayed compaction transcript, got %q", m.output.Join())
	}

	messages := m.appState.GetMessages()
	if len(messages) != 1 || messages[0].Content != "summary" {
		t.Fatalf("expected compacted conversation state, got %#v", messages)
	}
}

func TestInputMetaView_ShowsContextPercent(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.ctx = &replContext{
		workingDir: "",
		cfg: &config.ResolvedConfig{
			Provider: "openai",
			Model:    "gpt-5.4",
		},
		registry: &providers.Registry{
			Providers: []providers.Provider{
				{
					ID: "openai",
					Models: []providers.Model{
						{ID: "gpt-5.4", ContextWindow: 2000},
					},
				},
			},
		},
	}
	m.appState = replappstate.New(nil, "")
	m.appState.SetLastUsage(&llm.TokenUsage{InputTokens: 1000})
	m.refreshContextStatus()

	meta := m.inputMetaView()
	if strings.Contains(meta, "model:") {
		t.Fatalf("did not expect model label, got %q", meta)
	}
	if !strings.Contains(meta, "◉") {
		t.Fatalf("expected model glyph, got %q", meta)
	}
	if !strings.Contains(meta, "openai") {
		t.Fatalf("expected provider text in combined model display, got %q", meta)
	}
	if !strings.Contains(meta, "◷") {
		t.Fatalf("expected context glyph, got %q", meta)
	}
	if strings.Contains(meta, "context in use:") {
		t.Fatalf("did not expect context label, got %q", meta)
	}
	if !strings.Contains(meta, "50%") {
		t.Fatalf("expected 50%% context usage, got %q", meta)
	}
	if !strings.Contains(meta, repltheme.HighlightStyle.Render("openai/gpt-5.4")) {
		t.Fatalf("expected provider/model to use the same highlight style, got %q", meta)
	}
	if !strings.Contains(meta, repltheme.PrimaryBoldStyle.Render("✦")+" "+repltheme.PrimaryBoldStyle.Render("build")) {
		t.Fatalf("expected build mode glyph and value to use primary bold style, got %q", meta)
	}
	if strings.Contains(meta, "Mode:") {
		t.Fatalf("did not expect mode label, got %q", meta)
	}
	contextIdx := strings.Index(meta, "50%")
	modeIdx := strings.Index(meta, "build")
	if contextIdx == -1 || modeIdx == -1 || modeIdx <= contextIdx {
		t.Fatalf("expected mode after context percentage, got %q", meta)
	}
	if !strings.Contains(meta, "·") {
		t.Fatalf("expected separator dot between model and context, got %q", meta)
	}
}

func TestInputMetaView_ShowsAnthropicAdaptiveEffort(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.ctx = &replContext{
		workingDir: "",
		cfg: &config.ResolvedConfig{
			Provider:       config.ProviderAnthropic,
			Model:          "claude-sonnet-4-6",
			ThinkingEffort: "high",
		},
		registry: &providers.Registry{
			Providers: []providers.Provider{
				{
					ID: config.ProviderAnthropic,
					Models: []providers.Model{
						{ID: "claude-sonnet-4-6", ContextWindow: 2000, ThinkingEfforts: []string{"low", "medium", "high", "max"}},
					},
				},
			},
		},
	}

	meta := m.inputMetaView()
	if !strings.Contains(meta, "∴") {
		t.Fatalf("expected thinking glyph for anthropic, got %q", meta)
	}
	if !strings.Contains(meta, "high (adaptive)") {
		t.Fatalf("expected adaptive effort text for anthropic, got %q", meta)
	}
	if strings.Contains(meta, "effort:") {
		t.Fatalf("did not expect effort label for anthropic, got %q", meta)
	}
	if strings.Contains(meta, "thinking:") {
		t.Fatalf("did not expect thinking label for anthropic, got %q", meta)
	}
}

func TestInputMetaView_ShowsThinkingGlyphForNonAnthropic(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.ctx = &replContext{
		workingDir: "",
		cfg: &config.ResolvedConfig{
			Provider:       config.ProviderOpenAI,
			Model:          "gpt-5.4",
			ThinkingEffort: "high",
		},
		registry: &providers.Registry{
			Providers: []providers.Provider{
				{
					ID: config.ProviderOpenAI,
					Models: []providers.Model{
						{ID: "gpt-5.4", ContextWindow: 2000, ThinkingEfforts: []string{"low", "medium", "high", "xhigh"}},
					},
				},
			},
		},
	}

	meta := m.inputMetaView()
	if !strings.Contains(meta, "∴") {
		t.Fatalf("expected thinking glyph for non-anthropic provider, got %q", meta)
	}
	if strings.Contains(meta, "thinking:") {
		t.Fatalf("did not expect thinking label for non-anthropic provider, got %q", meta)
	}
	if strings.Contains(meta, "effort:") {
		t.Fatalf("did not expect effort label for non-anthropic provider, got %q", meta)
	}
}

func TestInputMetaView_UsesAccentStyleForPlanMode(t *testing.T) {
	m := newTestModel()
	m.mode = llm.ModePlan
	m.width = 120

	meta := m.inputMetaView()
	if !strings.Contains(meta, repltheme.AccentStyle.Render("✦")+" "+repltheme.AccentStyle.Render("plan")) {
		t.Fatalf("expected plan mode glyph and value to use accent style, got %q", meta)
	}
}

func TestInputMetaView_SuggestsCompactionAtSeventyPercent(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.contextStatus = contextStatus{
		KnownWindow:   true,
		KnownTokens:   true,
		ContextWindow: 100,
		CurrentTokens: 70,
		Percent:       70,
	}

	meta := m.inputMetaView()
	if !strings.Contains(meta, "Try /compact") {
		t.Fatalf("expected compaction hint, got %q", meta)
	}
}

func TestInputMetaView_DropsCompactionHintWhenWidthIsTight(t *testing.T) {
	m := newTestModel()
	m.width = 20
	m.contextStatus = contextStatus{
		KnownWindow:   true,
		KnownTokens:   true,
		ContextWindow: 100,
		CurrentTokens: 75,
		Percent:       75,
	}

	meta := m.inputMetaView()
	if strings.Contains(meta, "Try /compact") {
		t.Fatalf("expected compaction hint to be dropped for narrow width, got %q", meta)
	}
	if !strings.Contains(meta, "75%") {
		t.Fatalf("expected context status to remain visible, got %q", meta)
	}
}

func TestSpinnerHeight_IncludesCompactionSpinner(t *testing.T) {
	m := newTestModel()
	m.showSpinner = true
	m.isCompacting = true

	if got := m.spinnerHeight(); got != 2 {
		t.Fatalf("expected spinner height 2 during compaction, got %d", got)
	}
}

func TestFormatLoadingElapsed(t *testing.T) {
	tests := []struct {
		name string
		in   time.Duration
		want string
	}{
		{name: "zero", in: 0, want: "0:00"},
		{name: "under minute", in: 9*time.Second + 900*time.Millisecond, want: "0:09"},
		{name: "minute", in: time.Minute + 5*time.Second, want: "1:05"},
		{name: "negative", in: -time.Second, want: "0:00"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatLoadingElapsed(tt.in); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestView_RendersSpinnerOnLeftWithTopPadding(t *testing.T) {
	m := newTestModel()
	m.showSpinner = true
	m.loadingText = "Accio..."
	m.viewport.SetHeight(1)
	m.viewport.SetContent("assistant output")

	view := m.View().Content
	lines := strings.Split(view, "\n")

	outputLine := -1
	spinnerLine := -1
	for i, line := range lines {
		if strings.Contains(line, "assistant output") {
			outputLine = i
		}
		if strings.Contains(line, "Accio...") {
			spinnerLine = i
		}
	}

	if outputLine == -1 || spinnerLine == -1 {
		t.Fatalf("expected view to contain output and spinner text, got %q", view)
	}
	if spinnerLine != outputLine+2 {
		t.Fatalf("expected blank spacer line before spinner, got %q", view)
	}
	if strings.TrimSpace(lines[outputLine+1]) != "" {
		t.Fatalf("expected blank spacer line before spinner, got %q", view)
	}
	if !strings.HasPrefix(lines[spinnerLine], " ") {
		t.Fatalf("expected spinner text to preserve left padding, got %q", lines[spinnerLine])
	}
	if !strings.Contains(lines[spinnerLine], "| ") {
		t.Fatalf("expected spacing after spinner glyph, got %q", lines[spinnerLine])
	}
	if strings.Contains(lines[spinnerLine], "0:00") {
		t.Fatalf("expected spinner line not to include elapsed timer, got %q", lines[spinnerLine])
	}
}

func TestInputMetaView_RendersElapsedTimerLast(t *testing.T) {
	m := newTestModel()
	m.showSpinner = true
	m.loadingStartedAt = time.Now().Add(-65 * time.Second)
	m.contextStatus = contextStatus{
		KnownWindow:   true,
		KnownTokens:   true,
		ContextWindow: 100,
		CurrentTokens: 50,
		Percent:       50,
	}

	meta := m.inputMetaView()
	contextIdx := strings.Index(meta, "◷")
	modeIdx := strings.Index(meta, "build")
	timerIdx := strings.Index(meta, "1:05")
	if contextIdx == -1 || modeIdx == -1 || timerIdx == -1 {
		t.Fatalf("expected context status, mode, and elapsed timer in meta, got %q", meta)
	}
	if modeIdx <= contextIdx {
		t.Fatalf("expected mode after context status, got %q", meta)
	}
	if timerIdx <= modeIdx {
		t.Fatalf("expected elapsed timer after mode, got %q", meta)
	}
	if !strings.Contains(meta[modeIdx:timerIdx], "⏱") {
		t.Fatalf("expected timer icon before timer, got %q", meta)
	}
	if !strings.Contains(meta[modeIdx:timerIdx], "·") {
		t.Fatalf("expected dot separator between mode and timer, got %q", meta)
	}
}

func TestHandleCompactionDone_StopsCompactionAndRefreshesOutput(t *testing.T) {
	m := newTestModel()
	m.isCompacting = true
	m.showSpinner = true
	m.compactionCancel = func() {}
	m.contextStatus = contextStatus{KnownWindow: true, Percent: 10}
	m.streamHandler.Start(make(chan llm.StreamEvent), "Compacting...")
	m.streamHandler.HandleChunk("compacted summary")

	newM, cmd := m.handleCompactionDone()

	if newM.isCompacting || newM.showSpinner {
		t.Fatal("expected compaction mode to stop")
	}
	if newM.compactionCancel != nil {
		t.Fatal("expected compaction cancel func to be cleared")
	}
	if !strings.Contains(newM.output.Join(), "compacted summary") {
		t.Fatalf("expected streamed compaction summary, got %q", newM.output.Join())
	}
	compacted := newM.appState.GetMessages()
	if len(compacted) != 1 || compacted[0].Role != llm.RoleUser || compacted[0].Content != "compacted summary" {
		t.Fatalf("expected compacted state to keep summary as single user message, got %#v", compacted)
	}
	if cmd != nil {
		t.Fatal("expected nil cmd")
	}
}

func TestHandleCompactionError_CancelledShowsSoftMessage(t *testing.T) {
	m := newTestModel()
	m.isCompacting = true
	m.showSpinner = true
	m.compactionCancel = func() {}

	newM, cmd := m.handleCompactionError(context.Canceled)

	if newM.isCompacting || newM.showSpinner {
		t.Fatal("expected compaction mode to stop")
	}
	if !strings.Contains(newM.output.Join(), "Compaction cancelled.") {
		t.Fatalf("expected cancellation message, got %q", newM.output.Join())
	}
	if cmd != nil {
		t.Fatal("expected nil cmd")
	}
}

func TestInputMetaView_UnknownContextWindowShowsNA(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.ctx = &replContext{
		workingDir: "",
		cfg: &config.ResolvedConfig{
			Provider: "openai",
			Model:    "unknown-model",
		},
		registry: &providers.Registry{
			Providers: []providers.Provider{
				{
					ID: "openai",
					Models: []providers.Model{
						{ID: "gpt-5.4", ContextWindow: 2000},
					},
				},
			},
		},
	}

	m.refreshContextStatus()
	meta := m.inputMetaView()
	if !strings.Contains(meta, "N/A") {
		t.Fatalf("expected N/A for unknown context window, got %q", meta)
	}
}
