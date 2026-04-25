package repl

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	replappstate "github.com/user/keen-code/internal/cli/repl/appstate"
	replfilesearch "github.com/user/keen-code/internal/cli/repl/filesearch"
	replmarkdown "github.com/user/keen-code/internal/cli/repl/markdown"
	reploutput "github.com/user/keen-code/internal/cli/repl/output"
	replpermissions "github.com/user/keen-code/internal/cli/repl/permissions"
	repltheme "github.com/user/keen-code/internal/cli/repl/theme"
	repltooling "github.com/user/keen-code/internal/cli/repl/tooling"
	replwidgets "github.com/user/keen-code/internal/cli/repl/widgets"
	"github.com/user/keen-code/internal/config"
	"github.com/user/keen-code/internal/filesystem"
	"github.com/user/keen-code/internal/llm"
	"github.com/user/keen-code/internal/session"
	"github.com/user/keen-code/internal/updater"
	"github.com/user/keen-code/providers"
)

const (
	exitCommand     = "/exit"
	helpCommand     = "/help"
	modelCommand    = "/model"
	compactCommand  = "/compact"
	sessionsCommand = "/sessions"
	resumeCommand   = "/resume"
	clearCommand    = "/clear"
	newCommand      = "/new"
	thinkingCommand = "/thinking"

	defaultWidth = 120
	maxHeight    = 3
)

var loadingTexts = []string{
	"Accio...",
	"Aguamenti...",
	"Alohomora...",
	"Anapneo...",
	"Aparecium...",
	"Ascendio...",
	"Avis...",
	"Bombarda...",
	"Colloportus...",
	"Confundo...",
	"Confringo...",
	"Defodio...",
	"Depulso...",
	"Descendo...",
	"Diffindo...",
	"Duro...",
	"Engorgio...",
	"Episkey...",
	"Evanesco...",
	"Expelliarmus...",
	"Expulso...",
	"Ferula...",
	"Finite...",
	"Flagrate...",
	"Flipendo...",
	"Geminio...",
	"Homenum Revelio...",
	"Impedimenta...",
	"Impervius...",
	"Incendio...",
	"Langlock...",
	"Levicorpus...",
	"Liberacorpus...",
	"Locomotor...",
	"Lumos...",
	"Muffliato...",
	"Nox...",
	"Obliviate...",
	"Obscuro...",
	"Oppugno...",
	"Orchideous...",
	"Petrificus Totalus...",
	"Protego...",
	"Quietus...",
	"Reducio...",
	"Reducto...",
	"Reparo...",
	"Revelio...",
	"Rictusempra...",
	"Ridikulus...",
	"Scourgify...",
	"Sectumsempra...",
	"Serpensortia...",
	"Silencio...",
	"Sonorus...",
	"Stupefy...",
	"Tarantallegra...",
	"Tergeo...",
	"Waddiwasi...",
	"Wingardium Leviosa...",
}

var loadingSpinners = []spinner.Spinner{
	spinner.Line,
	spinner.Dot,
	spinner.MiniDot,
	spinner.Jump,
	spinner.Pulse,
	spinner.Points,
	spinner.Meter,
	spinner.Hamburger,
}

func nextLoadingText() string {
	return loadingTexts[rand.Intn(len(loadingTexts))]
}

func nextLoadingSpinner() spinner.Spinner {
	return loadingSpinners[rand.Intn(len(loadingSpinners))]
}

type replContext struct {
	version    string
	workingDir string
	cfg        *config.ResolvedConfig
	globalCfg  *config.GlobalConfig
	loader     *config.Loader
	registry   *providers.Registry
}

type replModel struct {
	textarea            textarea.Model
	viewport            viewport.Model
	ctx                 *replContext
	appState            *replappstate.AppState
	output              *reploutput.OutputBuilder
	modelSelection      *replwidgets.Model
	permissionRequester *replpermissions.Requester
	diffEmitter         *repltooling.DiffEmitter
	sessions            *replSessionState
	sessionPicker       *replwidgets.SessionPicker
	suggestion          replwidgets.SuggestionModel
	fileSearcher        *replfilesearch.FileSearcher
	quitting            bool
	streamHandler       *StreamHandler
	mdRenderer          *replmarkdown.Renderer
	width               int
	height              int
	spinner             spinner.Model
	showSpinner         bool
	loadingText         string
	userScrolled        bool
	streamCancel        context.CancelFunc
	turnMemory          *turnMemoryAccumulator
	isCompacting        bool
	compactionCancel    context.CancelFunc
	contextStatus       contextStatus
}

func abbreviateHome(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}

func initialModel(ctx *replContext, llmClient llm.LLMClient, needsSetup bool) replModel {
	ta := textarea.New()
	ta.Placeholder = "What are we building?"
	ta.Focus()
	ta.CharLimit = 0
	ta.SetWidth(defaultWidth - 3)
	ta.SetHeight(maxHeight)
	ta.MaxHeight = 0
	ta.ShowLineNumbers = false
	ta.SetPromptFunc(2, func(info textarea.PromptInfo) string {
		if info.LineNumber == 0 {
			return "> "
		}
		return "  "
	})

	styles := ta.Styles()
	styles.Focused.Prompt = repltheme.PromptStyle
	styles.Focused.Text = lipgloss.NewStyle()
	styles.Focused.CursorLine = lipgloss.NewStyle()
	styles.Blurred.Prompt = repltheme.PromptStyle
	styles.Blurred.Text = lipgloss.NewStyle()
	styles.Blurred.CursorLine = lipgloss.NewStyle()
	ta.SetStyles(styles)

	ta.KeyMap.InsertNewline.SetKeys("ctrl+enter")
	ta.KeyMap.InsertNewline.SetEnabled(true)

	s := spinner.New()
	s.Spinner = spinner.Pulse
	s.Style = lipgloss.NewStyle().Foreground(repltheme.PrimaryColor)

	appState := replappstate.New(llmClient, ctx.workingDir)

	permissionRequester := replpermissions.NewRequester()
	diffEmitter := repltooling.NewDiffEmitter()
	sessions := newReplSessionState(ctx.workingDir)

	fileGitAwareness := filesystem.NewGitAwareness()
	_ = fileGitAwareness.LoadGitignore(filepath.Join(ctx.workingDir, ".gitignore"))
	fileGuard := filesystem.NewGuard(ctx.workingDir, fileGitAwareness)
	fileSearcher := replfilesearch.NewFileSearcher(ctx.workingDir, fileGuard)

	repltooling.SetupToolRegistry(ctx.workingDir, appState, permissionRequester, diffEmitter)

	mdRenderer, err := replmarkdown.New(defaultWidth)

	if err != nil {
		mdRenderer = nil
	}

	output := reploutput.NewOutputBuilder(defaultWidth, ctx.workingDir)
	initialLines := buildInitialScreen(ctx)
	for _, line := range initialLines {
		output.AddLine(line)
	}

	vp := viewport.New(viewport.WithWidth(defaultWidth), viewport.WithHeight(24))

	model := replModel{
		textarea:            ta,
		viewport:            vp,
		ctx:                 ctx,
		appState:            appState,
		output:              output,
		spinner:             s,
		streamHandler:       NewStreamHandler(mdRenderer),
		mdRenderer:          mdRenderer,
		permissionRequester: permissionRequester,
		diffEmitter:         diffEmitter,
		sessions:            sessions,
		suggestion:          replwidgets.NewSuggestionModel(),
		fileSearcher:        fileSearcher,
	}
	model.streamHandler.workingDir = ctx.workingDir
	model.refreshContextStatus()

	if needsSetup {
		welcomeStyle := lipgloss.NewStyle().Foreground(repltheme.PrimaryColor).Bold(true)
		model.output.AddEmptyLine()
		model.output.AddStyledLine(welcomeStyle.Render("👋 Welcome to Keen!"), lipgloss.NewStyle())
		model.output.AddEmptyLine()
		model.output.AddEmptyLine()
		model = model.startModelSelection()
	} else {
		model.updateViewportContent()
	}

	return model
}

func buildInitialScreen(ctx *replContext) []string {
	var lines []string

	asciiArt := []string{
		"░█░█░█▀▀░█▀▀░█▀█░░░█▀▀░█▀█░█▀▄░█▀▀",
		"░█▀▄░█▀▀░█▀▀░█░█░░░█░░░█░█░█░█░█▀▀",
		"░▀░▀░▀▀▀░▀▀▀░▀░▀░░░▀▀▀░▀▀▀░▀▀░░▀▀▀",
	}

	colors := []string{
		"#9FA8DA", "#7986CB", "#5C6BC0", "#3F51B5", "#3949AB", "#303F9F", "#283593",
	}

	lines = append(lines, "")
	for i, line := range asciiArt {
		color := colors[i%len(colors)]
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(line))
	}

	lines = append(lines, "")
	lines = append(lines, "  "+repltheme.TitleStyle.Render("✦︎ Keen v"+ctx.version+" .✦ ݁˖"))
	lines = append(lines, "")

	displayDir := abbreviateHome(ctx.workingDir)
	lines = append(lines, "  "+repltheme.InfoLabelStyle.Render("Directory:")+" "+repltheme.InfoValueStyle.Render(displayDir))
	lines = append(lines, "  "+repltheme.InfoLabelStyle.Render("Provider:")+" "+repltheme.InfoValueStyle.Render(ctx.cfg.Provider))
	lines = append(lines, "  "+repltheme.InfoLabelStyle.Render("Model:")+" "+repltheme.HighlightStyle.Render(ctx.cfg.Model))
	if ctx.cfg.ThinkingEffort != "" && ctx.registry != nil {
		if modelMeta, ok := ctx.registry.GetModel(ctx.cfg.Provider, ctx.cfg.Model); ok && modelMeta.SupportsThinkingEffort() {
			lines = append(lines, "  "+repltheme.InfoLabelStyle.Render("Thinking:")+" "+repltheme.InfoValueStyle.Render(ctx.cfg.ThinkingEffort))
		}
	}
	lines = append(lines, "")

	tips := []string{
		"Use /help  for available commands",
		"Use /model to change provider or model",
		"Press Enter to send, Ctrl+Enter for new line",
		"Shift+click to select and copy text",
	}
	tipsBox := repltheme.BoxStyle.Render(repltheme.TipStyle.Render(strings.Join(tips, "\n")))
	lines = append(lines, tipsBox)
	lines = append(lines, "")

	return lines
}

func (m replModel) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, checkForUpdate(m.ctx.version))
}

func checkForUpdate(currentVersion string) tea.Cmd {
	return func() tea.Msg {
		latest, newer, err := updater.CheckLatest(context.Background(), currentVersion, "mochow13", "keen-code")
		if err != nil || !newer {
			return updateCheckMsg{}
		}
		return updateCheckMsg{latest: latest}
	}
}

func (m *replModel) spinnerHeight() int {
	if m.showSpinner {
		return 2
	}
	return 0
}

func (m *replModel) adjustTextareaHeight() {
	if m.height <= 0 {
		return
	}
	m.textarea.SetHeight(maxHeight)
	m.viewport.SetHeight(m.height - m.textarea.Height() - 4 - m.spinnerHeight() - m.suggestion.Height())
}

func (m replModel) isAtTopOfInput() bool {
	return m.textarea.Line() == 0
}

func (m replModel) isAtBottomOfInput() bool {
	return m.textarea.Line() >= m.textarea.LineCount()-1
}

func (m *replModel) startModelSelection() replModel {
	onComplete := func(provider, model, apiKey string) error {
		return m.updateLLMClient()
	}
	m.modelSelection = replwidgets.New(
		m.ctx.registry,
		m.ctx.globalCfg,
		m.ctx.loader,
		m.ctx.cfg,
		onComplete,
	)
	m.updateViewportContent()
	m.viewport.GotoBottom()
	return *m
}

func (m *replModel) handleEnterKey() (replModel, tea.Cmd) {
	input := m.textarea.Value()
	if input == "" {
		return *m, nil
	}

	if m.streamHandler.IsActive() {
		return *m, nil
	}

	if m.isCompacting {
		return *m, nil
	}

	m.output.AddUserInput(input, repltheme.PromptStyle)

	if input == exitCommand {
		m.quitting = true
		return *m, tea.Quit
	}

	if input == helpCommand {
		m.output.AddLine(getHelpText())
		m.output.AddEmptyLine()
		m.textarea.Reset()
		m.updateViewportContent()
		m.viewport.GotoBottom()
		return *m, nil
	}

	if input == modelCommand {
		m.textarea.Reset()
		return m.startModelSelection(), nil
	}

	if input == sessionsCommand || input == resumeCommand {
		m.textarea.Reset()
		summaries, err := m.sessions.listSessions()
		if err != nil {
			m.output.AddError("Failed to load sessions: "+err.Error(), repltheme.ErrorStyle)
			m.updateViewportContent()
			m.viewport.GotoBottom()
			return *m, nil
		}
		if len(summaries) == 0 {
			m.output.AddStyledLine("  No saved sessions for this directory.", lipgloss.NewStyle().Foreground(repltheme.MutedColor))
			m.output.AddEmptyLine()
			m.updateViewportContent()
			m.viewport.GotoBottom()
			return *m, nil
		}
		m.sessionPicker = replwidgets.NewSessionPicker(summaries)
		m.updateViewportContent()
		m.viewport.GotoBottom()
		return *m, nil
	}

	if input == clearCommand || input == newCommand {
		m.textarea.Reset()
		return m.handleClear(), nil
	}

	if input == thinkingCommand || strings.HasPrefix(input, thinkingCommand+" ") {
		m.textarea.Reset()
		return m.handleThinkingCommand(input)
	}

	if input == compactCommand || strings.HasPrefix(input, compactCommand+" ") {
		extraPrompt := strings.TrimSpace(strings.TrimPrefix(input, compactCommand))
		if !m.appState.IsClientReady(m.ctx.cfg) {
			m.output.AddError("LLM client not initialized. Use /model to configure.", repltheme.ErrorStyle)
			m.textarea.Reset()
			m.updateViewportContent()
			m.viewport.GotoBottom()
			return *m, nil
		}
		m.textarea.Reset()
		return m.startCompaction(extraPrompt)
	}

	if !m.appState.IsClientReady(m.ctx.cfg) {
		m.output.AddError("LLM client not initialized. Use /model to configure.", repltheme.ErrorStyle)
		m.textarea.Reset()
		m.updateViewportContent()
		m.viewport.GotoBottom()
		return *m, nil
	}

	if err := m.sessions.appendUserMessage(input); err != nil {
		m.output.AddError("Session persistence failed: "+err.Error(), repltheme.ErrorStyle)
		m.textarea.Reset()
		m.updateViewportContent()
		m.viewport.GotoBottom()
		return *m, nil
	}

	m.appState.AddMessage(llm.RoleUser, input)

	ctx := m.startStreamContext()
	eventCh, err := m.appState.StreamChat(ctx, m.ctx.cfg)
	if err != nil {
		m.clearStreamCancel()
		m.output.AddError(err.Error(), repltheme.ErrorStyle)
		m.textarea.Reset()
		m.updateViewportContent()
		m.viewport.GotoBottom()
		return *m, nil
	}

	m.showSpinner = true
	m.spinner.Spinner = nextLoadingSpinner()
	m.loadingText = nextLoadingText()
	m.startAssistantTurnMemory()
	m.streamHandler.Start(eventCh, m.loadingText)
	m.textarea.Reset()
	m.userScrolled = false
	m.adjustTextareaHeight()
	m.updateViewportContent()
	m.viewport.GotoBottom()

	return *m, tea.Batch(m.spinner.Tick, m.waitForAsyncEvent())
}

func (m *replModel) startCompaction(extraPrompt string) (replModel, tea.Cmd) {
	if m.compactionCancel != nil {
		m.compactionCancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	eventCh, err := m.appState.StreamCompact(ctx, m.ctx.cfg, extraPrompt)
	if err != nil {
		cancel()
		m.output.AddError(err.Error(), repltheme.ErrorStyle)
		m.updateViewportContent()
		m.viewport.GotoBottom()
		return *m, nil
	}
	if eventCh == nil {
		cancel()
		m.output.AddError("compaction stream unavailable", repltheme.ErrorStyle)
		m.updateViewportContent()
		m.viewport.GotoBottom()
		return *m, nil
	}

	m.compactionCancel = cancel
	m.isCompacting = true
	m.showSpinner = true
	m.spinner.Spinner = nextLoadingSpinner()
	m.loadingText = "Compacting..."
	m.clearTurnMemory()
	m.streamHandler.Start(eventCh, m.loadingText)
	m.userScrolled = false
	m.adjustTextareaHeight()
	m.updateViewportContent()
	m.viewport.GotoBottom()

	return *m, tea.Batch(m.spinner.Tick, m.waitForAsyncEvent())
}

func (m *replModel) startStreamContext() context.Context {
	if m.streamCancel != nil {
		m.streamCancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.streamCancel = cancel
	return ctx
}

func (m *replModel) clearStreamCancel() {
	m.streamCancel = nil
}

func (m *replModel) updateViewportContent() {
	if m.viewport.Width() == 0 {
		return
	}

	contentWidth := m.width
	if contentWidth <= 0 {
		contentWidth = m.viewport.Width()
	}

	var content strings.Builder

	if m.output != nil && !m.output.IsEmpty() {
		content.WriteString(m.output.Join())
	}

	if m.streamHandler != nil && m.streamHandler.IsActive() {
		content.WriteString(m.streamHandler.View(contentWidth))
	}

	if m.modelSelection != nil {
		content.WriteString(formatModelSelectionCard(m.modelSelection, m.viewport.Width()))
	}

	if m.sessionPicker != nil {
		content.WriteString(replwidgets.FormatSessionPickerCard(m.sessionPicker, m.viewport.Width(), m.viewport.Height()))
	}

	m.viewport.SetContent(content.String())
}

func (m replModel) waitForAsyncEvent() tea.Cmd {
	if m.streamHandler == nil || !m.streamHandler.IsActive() || m.streamHandler.eventCh == nil {
		return nil
	}
	var permissionCh <-chan *replpermissions.Request
	if m.permissionRequester != nil {
		permissionCh = m.permissionRequester.GetRequestChan()
	}
	var diffCh <-chan repltooling.DiffRequest
	if m.diffEmitter != nil {
		diffCh = m.diffEmitter.GetDiffChan()
	}
	return waitForAsyncEvent(
		m.streamHandler.eventCh,
		permissionCh,
		diffCh,
	)
}

func waitForAsyncEvent(llmCh <-chan llm.StreamEvent, permissionCh <-chan *replpermissions.Request, diffCh <-chan repltooling.DiffRequest) tea.Cmd {
	if llmCh == nil {
		return nil
	}

	return func() tea.Msg {
		select {
		case req := <-permissionCh:
			return permissionReadyMsg{req: req}
		case req := <-diffCh:
			return diffReadyMsg{req: req}
		case event, ok := <-llmCh:
			if !ok {
				return llmDoneMsg{}
			}

			switch event.Type {
			case llm.StreamEventTypeChunk:
				return llmChunkMsg(event.Content)
			case llm.StreamEventTypeReasoningChunk:
				return llmReasoningChunkMsg(event.Content)
			case llm.StreamEventTypeDone:
				return llmDoneMsg{}
			case llm.StreamEventTypeError:
				return llmErrorMsg{err: event.Error}
			case llm.StreamEventTypeToolStart:
				return llmToolStartMsg{toolCall: event.ToolCall}
			case llm.StreamEventTypeToolEnd:
				return llmToolEndMsg{toolCall: event.ToolCall}
			case llm.StreamEventTypeUsage:
				return llmUsageMsg{usage: event.Usage}
			default:
				return llmDoneMsg{}
			}
		}
	}
}

func formatModelSelectionCard(ms *replwidgets.Model, width int) string {
	ruleWidth := defaultWidth
	if width > 0 {
		ruleWidth = width
	}
	if ruleWidth < 1 {
		ruleWidth = 1
	}

	rule := repltheme.ModelSelectionRuleStyle.Render(strings.Repeat("─", ruleWidth))
	lines := strings.Split(strings.TrimRight(ms.ViewString(), "\n"), "\n")
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(rule + "\n\n")
	for _, l := range lines {
		sb.WriteString("  " + l + "\n")
	}
	sb.WriteString("\n")
	sb.WriteString(rule + "\n")
	return sb.String()
}

func renderInputArea(content string, width int) string {
	ruleWidth := defaultWidth
	if width > 0 {
		ruleWidth = width
	}
	if ruleWidth < 1 {
		ruleWidth = 1
	}

	rule := repltheme.InputRuleStyle.Render(strings.Repeat("─", ruleWidth))
	return rule + "\n" + content + "\n" + rule
}

func (m *replModel) applyWindowSize(msg tea.WindowSizeMsg) {
	m.width = msg.Width
	m.height = msg.Height
	m.textarea.SetWidth(msg.Width - 3)
	if m.mdRenderer != nil {
		m.mdRenderer.UpdateWidth(msg.Width)
	}
	m.viewport.SetWidth(msg.Width)
	m.viewport.SetHeight(msg.Height - m.textarea.Height() - 4 - m.spinnerHeight() - m.suggestion.Height())
}

func (m replModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	updatedModel, cmd := m.updateNormalMode(msg)
	return &updatedModel, cmd
}

func (m replModel) updateNormalMode(msg tea.Msg) (replModel, tea.Cmd) {
	if updated, cmd, handled := m.handleLLMStreamMsg(msg); handled {
		return updated, cmd
	}

	if updated, cmd, handled := m.consumeModelSelectionResult(msg); handled {
		return updated, cmd
	}

	if sizeMsg, ok := msg.(tea.WindowSizeMsg); ok {
		m.applyWindowSize(sizeMsg)
		if m.modelSelection != nil {
			m.updateViewportContent()
		}
		return m, nil
	}

	if m.modelSelection != nil {
		return m.handleKeyMsg(msg)
	}

	switch msg := msg.(type) {
	case compactionDoneMsg:
		return m.handleCompactionDone()
	case compactionErrMsg:
		return m.handleCompactionError(msg.err)
	case updateCheckMsg:
		m.handleUpdateCheckMsg(msg)
		return m, nil
	case diffReadyMsg:
		m.streamHandler.HandleDiff(msg.req.Lines)
		close(msg.req.Done)
		m.updateViewportContent()
		m.viewport.GotoBottom()
		return m, m.waitForAsyncEvent()

	case permissionReadyMsg:
		m.streamHandler.HandlePermissionRequest(msg.req)
		m.updateViewportContent()
		m.viewport.GotoBottom()
		return m, m.waitForAsyncEvent()

	case spinner.TickMsg:
		if updated, cmd, handled := m.handleSpinnerTick(msg); handled {
			return updated, cmd
		}

	case tea.WindowSizeMsg:
		m.applyWindowSize(msg)
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKeyMsg(msg)

	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelUp:
			m.viewport.ScrollUp(3)
			m.userScrolled = !m.viewport.AtBottom()
		case tea.MouseWheelDown:
			m.viewport.ScrollDown(3)
			m.userScrolled = !m.viewport.AtBottom()
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	m.adjustTextareaHeight()
	return m, cmd
}

func (m replModel) consumeModelSelectionResult(msg tea.Msg) (replModel, tea.Cmd, bool) {
	if m.modelSelection == nil {
		return m, nil, false
	}

	if replwidgets.IsComplete(msg) {
		successMsg := "✓ Updated to " + m.modelSelection.SelectedProvider + " / " + m.modelSelection.SelectedModel
		m.output.AddStyledLine("  "+successMsg, repltheme.HighlightStyle)
		m.output.AddEmptyLine()
		m.modelSelection = nil
		m.refreshContextStatus()
		m.updateViewportContent()
		m.viewport.GotoBottom()
		return m, nil, true
	}

	if replwidgets.IsCancel(msg) {
		cancelStyle := lipgloss.NewStyle().Foreground(repltheme.MutedColor)
		m.output.AddStyledLine("  Model selection cancelled", cancelStyle)
		m.output.AddEmptyLine()
		m.modelSelection = nil
		m.updateViewportContent()
		m.viewport.GotoBottom()
		return m, nil, true
	}

	return m, nil, false
}

func (m replModel) handleSpinnerTick(msg spinner.TickMsg) (replModel, tea.Cmd, bool) {
	if !m.showSpinner {
		return m, nil, false
	}

	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	m.updateViewportContent()
	return m, cmd, true
}

func (m replModel) View() tea.View {
	var content string

	if m.quitting {
		content = lipgloss.NewStyle().Foreground(repltheme.MutedColor).Render("\n  Goodbye!\n")
	} else {
		var view strings.Builder

		view.WriteString(m.viewport.View())
		view.WriteString("\n")

		if m.showSpinner {
			spinnerText := " " + m.spinner.View() + " " + repltheme.LoadingTextStyled.Render(m.loadingText)
			view.WriteString("\n")
			view.WriteString(spinnerText)
			view.WriteString("\n")
		}

		view.WriteString(renderInputArea(m.textarea.View(), m.width))
		view.WriteString("\n")
		if m.suggestion.Visible() {
			view.WriteString(m.suggestion.View(m.width))
			view.WriteString("\n")
		}
		view.WriteString(m.inputMetaView())

		content = view.String()
	}

	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (m replModel) inputMetaView() string {
	provider := "-"
	model := "-"

	if m.ctx != nil && m.ctx.cfg != nil {
		if m.ctx.cfg.Provider != "" {
			provider = m.ctx.cfg.Provider
		}
		if m.ctx.cfg.Model != "" {
			model = m.ctx.cfg.Model
		}
	}

	modelText := repltheme.MetaLabelStyle.Render("model:") + " " + repltheme.HighlightStyle.Render(provider+"/"+model)
	if m.ctx != nil && m.ctx.cfg != nil && m.ctx.cfg.ThinkingEffort != "" && m.ctx.registry != nil {
		if modelMeta, ok := m.ctx.registry.GetModel(m.ctx.cfg.Provider, m.ctx.cfg.Model); ok && modelMeta.SupportsThinkingEffort() {
			effortLabel := "thinking:"
			effortValue := m.ctx.cfg.ThinkingEffort
			if m.ctx.cfg.Provider == config.ProviderAnthropic {
				effortLabel = "effort:"
				effortValue += " (adaptive)"
			}
			modelText += " " + repltheme.MetaLabelStyle.Render("·") + " " + repltheme.MetaLabelStyle.Render(effortLabel) + " " + repltheme.HighlightStyle.Render(effortValue)
		}
	}
	contextText := renderContextStatus(m.contextStatus)
	separator := repltheme.MetaLabelStyle.Render("·")
	left := modelText + " " + separator + " " + contextText
	right := ""
	if m.contextStatus.ShouldSuggestCompaction() {
		right = repltheme.CompactionSuggestionStyle.Render("Try /compact")
	}

	const leftPad = "  "
	if m.width <= 0 {
		if right == "" {
			return leftPad + left
		}
		return leftPad + left + "   " + right
	}

	available := m.width - lipgloss.Width(leftPad)
	if right == "" {
		return leftPad + left
	}
	if available < lipgloss.Width(left)+lipgloss.Width(right)+1 {
		right = ""
	}
	if right == "" {
		return leftPad + left
	}
	if available <= lipgloss.Width(right)+1 {
		return leftPad + left
	}

	space := available - lipgloss.Width(left) - lipgloss.Width(right)
	if space >= 1 {
		return leftPad + left + strings.Repeat(" ", space) + right
	}

	compactLeft := repltheme.MetaLabelStyle.Render("M:") + " " + repltheme.HighlightStyle.Render(provider+"/"+model) +
		" " + separator + " " + repltheme.MetaLabelStyle.Render("C:") + " " + contextPercentStyle(m.contextStatus.Percent).Render(formatPercent(m.contextStatus.Percent))
	if !m.contextStatus.KnownWindow || m.contextStatus.ContextWindow <= 0 {
		compactLeft = repltheme.MetaLabelStyle.Render("M:") + " " + repltheme.HighlightStyle.Render(provider+"/"+model) +
			" " + separator + " " + repltheme.MetaLabelStyle.Render("C:") + " " + repltheme.ContextStatusUnknownStyle.Render("N/A")
	}
	space = available - lipgloss.Width(compactLeft) - lipgloss.Width(right)
	if space >= 1 {
		return leftPad + compactLeft + strings.Repeat(" ", space) + right
	}

	return leftPad + left
}

func getHelpText() string {
	cmds := []struct{ cmd, desc string }{
		{"/clear", "Start a new session (also /new)"},
		{"/compact", "Compact conversation context"},
		{"/help", "Show available commands"},
		{"/model", "Change provider or model"},
		{"/new", "Start a new session (also /clear)"},
		{"/resume", "Open the session picker"},
		{"/sessions", "List saved sessions for this directory"},
		{"/thinking", "Change thinking effort for the current model"},
		{"/exit", "Quit Keen"},
	}

	var lines []string
	lines = append(lines, repltheme.TitleStyle.Render("Available Commands"))
	lines = append(lines, "")
	for _, c := range cmds {
		lines = append(lines, "  "+repltheme.HelpCmdStyle.Render(c.cmd)+" "+repltheme.HelpDescStyle.Render(c.desc))
	}

	return strings.Join(lines, "\n")
}

func (m *replModel) handleThinkingCommand(input string) (replModel, tea.Cmd) {
	effort := strings.TrimSpace(strings.TrimPrefix(input, thinkingCommand))

	modelMeta, ok := m.ctx.registry.GetModel(m.ctx.cfg.Provider, m.ctx.cfg.Model)
	if !ok || !modelMeta.SupportsThinkingEffort() {
		m.output.AddError("Current model does not support configurable thinking", repltheme.ErrorStyle)
		m.updateViewportContent()
		m.viewport.GotoBottom()
		return *m, nil
	}

	if !slices.Contains(modelMeta.ThinkingEfforts, effort) {
		m.output.AddError("Usage: /thinking "+strings.Join(modelMeta.ThinkingEfforts, "|"), repltheme.ErrorStyle)
		m.updateViewportContent()
		m.viewport.GotoBottom()
		return *m, nil
	}

	m.ctx.cfg.ThinkingEffort = effort
	m.ctx.globalCfg.ThinkingEffort = effort
	if err := m.ctx.loader.Save(m.ctx.globalCfg); err != nil {
		m.output.AddError("Failed to save config: "+err.Error(), repltheme.ErrorStyle)
		m.updateViewportContent()
		m.viewport.GotoBottom()
		return *m, nil
	}

	if err := m.updateLLMClient(); err != nil {
		m.output.AddError("Failed to reinitialize LLM client: "+err.Error(), repltheme.ErrorStyle)
		m.updateViewportContent()
		m.viewport.GotoBottom()
		return *m, nil
	}

	m.output.AddStyledLine("  ✓ Thinking effort set to: "+effort, repltheme.HighlightStyle)
	m.output.AddEmptyLine()
	m.updateViewportContent()
	m.viewport.GotoBottom()
	return *m, nil
}

func (m *replModel) handleClear() replModel {
	m.appState.ClearMessages()
	m.appState.ClearContextMetrics()
	m.sessions.resetSession()

	newOutput := reploutput.NewOutputBuilder(m.width, m.ctx.workingDir)
	initialLines := buildInitialScreen(m.ctx)
	for _, line := range initialLines {
		newOutput.AddLine(line)
	}
	newOutput.AddStyledLine("  ✓ New session started", repltheme.CompactionSuccessStyle)
	newOutput.AddEmptyLine()
	m.output = newOutput

	m.refreshContextStatus()
	m.updateViewportContent()
	m.viewport.GotoBottom()
	return *m
}

func (m *replModel) updateLLMClient() error {
	client, err := llm.NewClient(m.ctx.cfg)
	if err != nil {
		return err
	}
	m.appState.UpdateClient(client)
	return nil
}

func (m *replModel) handleSessionPersistenceError(err error) {
	if err == nil {
		return
	}
	m.output.AddError("Session persistence failed: "+err.Error(), repltheme.ErrorStyle)
}

func (m *replModel) replayLoadedSession(loaded *session.LoadedSession) {
	if loaded == nil {
		return
	}

	replay := newSessionReplay(m.width, m.mdRenderer, m.ctx.workingDir)

	for _, event := range loaded.Events {
		replay.applyEvent(event)
	}

	replay.flushDone()

	m.output = replay.output
	m.appState.ReplaceMessages(session.BuildConversation(loaded.Events))
	m.sessionPicker = nil
	m.refreshContextStatus()
	m.updateViewportContent()
	m.viewport.GotoBottom()
}

func RunREPL(
	version string,
	workingDir string,
	cfg *config.ResolvedConfig,
	loader *config.Loader,
	globalCfg *config.GlobalConfig,
	registry *providers.Registry,
	needsSetup bool,
) error {
	ctx := &replContext{
		version:    version,
		workingDir: workingDir,
		cfg:        cfg,
		globalCfg:  globalCfg,
		loader:     loader,
		registry:   registry,
	}

	var llmClient llm.LLMClient
	if cfg.APIKey != "" && cfg.Model != "" {
		client, err := llm.NewClient(cfg)
		if err != nil {
			return fmt.Errorf("failed to initialize LLM client: %w", err)
		}
		llmClient = client
	}

	m := initialModel(ctx, llmClient, needsSetup)
	p := tea.NewProgram(&m)
	if _, err := p.Run(); err != nil {
		return err
	}

	return nil
}
