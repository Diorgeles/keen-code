package repl

import (
	"context"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	keenauth "github.com/user/keen-code/internal/auth"
	replcommands "github.com/user/keen-code/internal/cli/repl/commands"
	reploutput "github.com/user/keen-code/internal/cli/repl/output"
	repltheme "github.com/user/keen-code/internal/cli/repl/theme"
	replwidgets "github.com/user/keen-code/internal/cli/repl/widgets"
	"github.com/user/keen-code/internal/config"
)

func (m *replModel) dispatchCommand(input string) (replModel, tea.Cmd, bool) {
	switch {
	case input == replcommands.Exit:
		m.quitting = true
		_ = m.history.Flush()
		return *m, tea.Quit, true

	case input == replcommands.Help:
		m.output.AddLine(getHelpText())
		m.output.AddEmptyLine()
		m.textarea.Reset()
		m.updateViewportContent()
		m.viewport.GotoBottom()
		return *m, nil, true

	case input == replcommands.Model:
		m.textarea.Reset()
		return m.startModelSelection(), nil, true

	case input == replcommands.Logout:
		m.textarea.Reset()
		result := m.handleLogoutCommand()
		return result, nil, true

	case input == replcommands.Sessions || input == replcommands.Resume:
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

	case input == replcommands.Clear || input == replcommands.New:
		m.textarea.Reset()
		result := m.handleClearCommand()
		return result, nil, true

	case input == replcommands.Thinking || strings.HasPrefix(input, replcommands.Thinking+" "):
		m.textarea.Reset()
		result, cmd := m.handleThinkingCommand(input)
		return result, cmd, true

	case input == replcommands.ShowThinking || strings.HasPrefix(input, replcommands.ShowThinking+" "):
		m.textarea.Reset()
		result := m.handleShowThinkingCommand(input)
		return result, nil, true

	case input == replcommands.Compact || strings.HasPrefix(input, replcommands.Compact+" "):
		extraPrompt := strings.TrimSpace(strings.TrimPrefix(input, replcommands.Compact))
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
	effort := strings.TrimSpace(strings.TrimPrefix(input, replcommands.Thinking))

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

func (m *replModel) handleShowThinkingCommand(input string) replModel {
	arg := strings.TrimSpace(strings.TrimPrefix(input, replcommands.ShowThinking))

	switch arg {
	case "on":
		m.showThinking = true
		m.streamHandler.showThinking = true
		m.saveShowThinking(true)
		m.output.AddStyledLine("  ✓ Thinking tokens shown", repltheme.HighlightStyle)
	case "off":
		m.showThinking = false
		m.streamHandler.showThinking = false
		m.saveShowThinking(false)
		m.output.AddStyledLine("  ✓ Thinking tokens hidden", repltheme.HighlightStyle)
	default:
		if m.showThinking {
			m.output.AddStyledLine("  Thinking tokens: shown (use /show-thinking off to hide)", repltheme.HighlightStyle)
		} else {
			m.output.AddStyledLine("  Thinking tokens: hidden (use /show-thinking on to show)", repltheme.HighlightStyle)
		}
	}

	m.output.AddEmptyLine()
	m.updateViewportContent()
	m.viewport.GotoBottom()
	return *m
}

func (m *replModel) saveShowThinking(val bool) {
	if m.ctx == nil || m.ctx.globalCfg == nil || m.ctx.loader == nil {
		return
	}
	m.ctx.globalCfg.ShowThinking = &val
	_ = m.ctx.loader.Save(m.ctx.globalCfg)
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
	var lines []string
	lines = append(lines, repltheme.TitleStyle.Render("Available Commands"))
	lines = append(lines, "")
	for _, c := range replcommands.All {
		lines = append(lines, "  "+repltheme.HelpCmdStyle.Render(c.Name)+" "+repltheme.HelpDescStyle.Render(c.Description))
	}

	return strings.Join(lines, "\n")
}
