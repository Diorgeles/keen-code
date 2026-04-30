package repl

import (
	"context"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	keenauth "github.com/user/keen-code/internal/auth"
	reploutput "github.com/user/keen-code/internal/cli/repl/output"
	repltheme "github.com/user/keen-code/internal/cli/repl/theme"
	replwidgets "github.com/user/keen-code/internal/cli/repl/widgets"
	"github.com/user/keen-code/internal/config"
)

func (m *replModel) dispatchCommand(input string) (replModel, tea.Cmd, bool) {
	switch {
	case input == exitCommand:
		m.quitting = true
		_ = m.history.Flush()
		return *m, tea.Quit, true

	case input == helpCommand:
		m.output.AddLine(getHelpText())
		m.output.AddEmptyLine()
		m.textarea.Reset()
		m.updateViewportContent()
		m.viewport.GotoBottom()
		return *m, nil, true

	case input == modelCommand:
		m.textarea.Reset()
		return m.startModelSelection(), nil, true

	case input == logoutCommand:
		m.textarea.Reset()
		result := m.handleLogoutCommand()
		return result, nil, true

	case input == sessionsCommand || input == resumeCommand:
		m.textarea.Reset()
		summaries, err := m.sessions.listSessions()
		if err != nil {
			m.output.AddError("Failed to load sessions: "+err.Error(), repltheme.ErrorStyle)
			m.updateViewportContent()
			m.viewport.GotoBottom()
			return *m, nil, true
		}
		if len(summaries) == 0 {
			m.output.AddStyledLine("  No saved sessions for this directory.", lipgloss.NewStyle().Foreground(repltheme.MutedColor))
			m.output.AddEmptyLine()
			m.updateViewportContent()
			m.viewport.GotoBottom()
			return *m, nil, true
		}
		m.sessionPicker = replwidgets.NewSessionPicker(summaries)
		m.updateViewportContent()
		m.viewport.GotoBottom()
		return *m, nil, true

	case input == clearCommand || input == newCommand:
		m.textarea.Reset()
		result := m.handleClearCommand()
		return result, nil, true

	case input == thinkingCommand || strings.HasPrefix(input, thinkingCommand+" "):
		m.textarea.Reset()
		result, cmd := m.handleThinkingCommand(input)
		return result, cmd, true

	case input == compactCommand || strings.HasPrefix(input, compactCommand+" "):
		extraPrompt := strings.TrimSpace(strings.TrimPrefix(input, compactCommand))
		if !m.appState.IsClientReady(m.ctx.cfg) {
			m.output.AddError("LLM client not initialized. Use /model to configure.", repltheme.ErrorStyle)
			m.textarea.Reset()
			m.updateViewportContent()
			m.viewport.GotoBottom()
			return *m, nil, true
		}
		m.textarea.Reset()
		result, cmd := m.startCompaction(extraPrompt)
		return result, cmd, true

	default:
		return *m, nil, false
	}
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

func (m *replModel) handleLogoutCommand() replModel {
	if m.ctx == nil || m.ctx.cfg == nil || m.ctx.cfg.Provider == "" {
		m.output.AddError("No provider is configured.", repltheme.ErrorStyle)
		m.updateViewportContent()
		m.viewport.GotoBottom()
		return *m
	}
	if config.AuthModeForProvider(m.ctx.cfg.Provider) != config.AuthModeOAuth {
		m.output.AddError("Current provider does not use OAuth.", repltheme.ErrorStyle)
		m.updateViewportContent()
		m.viewport.GotoBottom()
		return *m
	}
	if err := keenauth.NewStore().Remove(m.ctx.cfg.Provider); err != nil {
		m.output.AddError("Failed to remove OAuth credentials: "+err.Error(), repltheme.ErrorStyle)
		m.updateViewportContent()
		m.viewport.GotoBottom()
		return *m
	}
	m.appState.UpdateClient(nil)
	m.output.AddStyledLine("  ✓ Signed out of "+m.ctx.cfg.Provider, repltheme.HighlightStyle)
	m.output.AddEmptyLine()
	m.updateViewportContent()
	m.viewport.GotoBottom()
	return *m
}

func (m *replModel) handleClearCommand() replModel {
	m.appState.ClearMessages()
	m.appState.ClearContextMetrics()
	m.sessions.resetSession()
	if m.permissionRequester != nil {
		m.permissionRequester.ResetSessionPermissions()
	}
	m.history.Reset()

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

func getHelpText() string {
	cmds := []struct{ cmd, desc string }{
		{"/clear", "Start a new session (also /new)"},
		{"/compact", "Compact conversation context"},
		{"/help", "Show available commands"},
		{"/logout", "Sign out of the current OAuth provider"},
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
