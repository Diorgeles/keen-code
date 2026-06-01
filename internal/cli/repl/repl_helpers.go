package repl

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	replpermissions "github.com/user/keen-code/internal/cli/repl/permissions"
	repltheme "github.com/user/keen-code/internal/cli/repl/theme"
	repltooling "github.com/user/keen-code/internal/cli/repl/tooling"
	replwidgets "github.com/user/keen-code/internal/cli/repl/widgets"
	"github.com/user/keen-code/internal/llm"
	keenmcp "github.com/user/keen-code/internal/mcp"
	"github.com/user/keen-code/internal/mcpskills"
	"github.com/user/keen-code/internal/skills"
	"github.com/user/keen-code/internal/updater"

	"github.com/user/keen-code/internal/config"
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

func (m *replModel) startLoading(text string) {
	m.showSpinner = true
	m.spinner.Spinner = nextLoadingSpinner()
	m.loadingText = text
	m.loadingStartedAt = time.Now()
}

func (m *replModel) stopLoading() {
	m.showSpinner = false
	m.loadingStartedAt = time.Time{}
}

func waitForMCPStartup(runtime keenmcp.Runtime) tea.Cmd {
	if runtime == nil {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = runtime.WaitInitialScan(ctx)
		return mcpStartupStatusMsg{Statuses: runtime.Servers()}
	}
}

func (m *replModel) handleMCPStartupStatus(statuses []keenmcp.ServerStatus) {
	m.syncMCPSkills(statuses)

	var failed []keenmcp.ServerStatus
	for _, status := range statuses {
		if isMCPFailureState(status.State) {
			failed = append(failed, status)
		}
	}
	if len(failed) == 0 {
		return
	}
	for _, status := range failed {
		msg := "  MCP connection failed for " + status.Name + ". Try `/mcp connect " + status.Name + "` to connect."
		if status.LastError != "" {
			msg += " (" + status.LastError + ")"
		}
		m.output.AddLine(wrapTextWithStyle(msg, repltheme.ErrorStyle, m.messageWidth()))
	}
	m.output.AddEmptyLine()
	m.updateViewportContent()
	m.viewport.GotoBottom()
}

func (m *replModel) messageWidth() int {
	width := m.width
	if width <= 0 && m.viewport.Width() > 0 {
		width = m.viewport.Width()
	}
	if width <= 0 {
		width = defaultWidth
	}
	return max(width-4, 1)
}

func wrapTextWithStyle(text string, style lipgloss.Style, width int) string {
	if width < 1 {
		width = 1
	}
	return lipgloss.NewStyle().Width(width).Render(style.Render(text))
}

func isMCPFailureState(state keenmcp.ServerState) bool {
	return state == keenmcp.StateDisconnected || state == keenmcp.StateAuthRequired || state == keenmcp.StateAuthFailed
}

func (m *replModel) syncMCPSkills(statuses []keenmcp.ServerStatus) {
	configured := map[string]bool{}
	reloadSkills := false
	for _, status := range statuses {
		configured[status.Name] = true
		if status.State == keenmcp.StateConnected {
			if m.refreshMCPSkill(status.Name, status.Description) {
				reloadSkills = true
			}
			continue
		}
		if isMCPFailureState(status.State) {
			_ = m.appState.SetSkillStatus(mcpskills.SkillName(status.Name), skills.StatusDisabled)
			reloadSkills = true
		}
	}
	if m.removeUnconfiguredMCPSkillStatuses(configured) {
		reloadSkills = true
	}
	if reloadSkills {
		m.appState.ReloadSkills()
	}
}

func (m *replModel) removeUnconfiguredMCPSkillStatuses(configured map[string]bool) bool {
	changed := false
	for name := range m.appState.GetSkillsConfig().IsEnabled {
		if !mcpskills.IsSkillName(name) {
			continue
		}
		server := mcpskills.ServerName(name)
		if configured[server] {
			continue
		}
		_ = m.appState.RemoveSkillStatus(name)
		_ = mcpskills.Remove(server)
		changed = true
	}
	return changed
}

func (m *replModel) refreshMCPSkill(server, description string) bool {
	if m.ctx == nil || m.ctx.mcp == nil {
		return false
	}
	tools, err := m.ctx.mcp.ListTools(context.Background(), server)
	if err != nil {
		slog.Default().Debug("mcpskills list tools failed", "server", server, "error", err)
		return false
	}
	if err := mcpskills.Generate(server, description, tools); err != nil {
		slog.Default().Debug("mcpskills generate failed", "server", server, "error", err)
		return false
	}
	return true
}

func (m *replModel) handleMCPConnectDone(msg mcpConnectDoneMsg) {
	m.stopLoading()
	changed := false
	if msg.Err != nil {
		m.output.AddError("MCP connect failed for "+msg.Server+": "+msg.Err.Error(), repltheme.ErrorStyle)
		_ = m.appState.SetSkillStatus(mcpskills.SkillName(msg.Server), skills.StatusDisabled)
		changed = true
	} else {
		m.output.AddStyledLine("  ✓ MCP server connected: "+msg.Server, repltheme.HighlightStyle)
		description := ""
		if m.ctx != nil && m.ctx.mcp != nil {
			description = m.ctx.mcp.Status(msg.Server).Description
		}
		if m.refreshMCPSkill(msg.Server, description) {
			changed = true
		}
		_ = m.appState.SetSkillStatus(mcpskills.SkillName(msg.Server), skills.StatusEnabled)
		changed = true
	}
	if changed {
		m.appState.ReloadSkills()
	}
	m.output.AddEmptyLine()
	m.updateViewportContent()
	m.viewport.GotoBottom()
}

func (m replModel) loadingElapsedText() string {
	if m.loadingStartedAt.IsZero() {
		return "0:00"
	}
	return formatLoadingElapsed(time.Since(m.loadingStartedAt))
}

func formatLoadingElapsed(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	totalSeconds := int(d.Seconds())
	minutes := totalSeconds / 60
	seconds := totalSeconds % 60
	return fmt.Sprintf("%d:%02d", minutes, seconds)
}

func (m *replModel) currentMode() llm.AgentMode {
	if m.mode == "" {
		return llm.ModeBuild
	}
	return m.mode
}

func (m *replModel) setMode(mode llm.AgentMode) {
	if mode != llm.ModePlan {
		mode = llm.ModeBuild
	}
	m.mode = mode
	if m.appState != nil {
		m.appState.SetMode(mode)
	}
}

func (m *replModel) toggleMode() {
	if m.currentMode() == llm.ModePlan {
		m.setMode(llm.ModeBuild)
	} else {
		m.setMode(llm.ModePlan)
	}
	m.updateViewportContent()
	m.viewport.GotoBottom()
}

func abbreviateHome(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if after, ok := strings.CutPrefix(path, home); ok {
		return "~" + after
	}
	return path
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
		"Use /help for all commands, `/skills list` for available skills",
		"Use /model to change provider and model",
		"Use /mode plan|build or Shift+Tab to switch modes",
		"Press enter to send, shift+enter for new line",
		"Press ctrl+C or cmd+C for copying, tab to switch focus",
	}
	tipsBox := repltheme.BoxStyle.Render(repltheme.TipStyle.Render(strings.Join(tips, "\n")))
	lines = append(lines, tipsBox)
	lines = append(lines, "")

	return lines
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

func formatModelSelectionCard(ms *replwidgets.Model, width int) string {
	ruleWidth := defaultWidth
	if width > 0 {
		ruleWidth = width
	}
	if ruleWidth < 1 {
		ruleWidth = 1
	}

	contentWidth := max(ruleWidth-2, 1)

	rule := repltheme.ModelSelectionRuleStyle.Render(strings.Repeat("─", ruleWidth))
	lines := strings.Split(strings.TrimRight(ms.ViewString(), "\n"), "\n")
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(rule + "\n\n")
	for _, l := range lines {
		wrapped := wrapTextWithStyle(l, lipgloss.NewStyle(), contentWidth)
		for _, wrappedLine := range strings.Split(wrapped, "\n") {
			sb.WriteString("  " + wrappedLine + "\n")
		}
	}
	sb.WriteString("\n")
	sb.WriteString(rule + "\n")
	return sb.String()
}

func renderInputArea(content string, width int, focused bool) string {
	ruleWidth := defaultWidth
	if width > 0 {
		ruleWidth = width
	}
	if ruleWidth < 1 {
		ruleWidth = 1
	}

	ruleStyle := repltheme.InputRuleStyle
	if !focused {
		ruleStyle = repltheme.InputRuleBlurredStyle
	}
	rule := ruleStyle.Render(strings.Repeat("─", ruleWidth))
	return rule + "\n" + content + "\n" + rule
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
			case llm.StreamEventTypeIncomplete:
				return llmIncompleteMsg{err: event.Error}
			case llm.StreamEventTypeToolStart:
				return llmToolStartMsg{toolCall: event.ToolCall}
			case llm.StreamEventTypeToolEnd:
				return llmToolEndMsg{toolCall: event.ToolCall}
			case llm.StreamEventTypeUsage:
				return llmUsageMsg{usage: event.Usage}
			case llm.StreamEventTypeRetry:
				return llmRetryMsg{err: event.Error, attempt: event.Attempt}
			default:
				return llmDoneMsg{}
			}
		}
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

func (m *replModel) focusInput() tea.Cmd {
	m.suggestion.Hide()
	return m.textarea.Focus()
}

func (m *replModel) blurInput() {
	m.suggestion.Hide()
	m.textarea.Blur()
}

func (m *replModel) toggleInputFocus() tea.Cmd {
	if m.textarea.Focused() {
		m.blurInput()
		return nil
	}
	return m.focusInput()
}

func (m *replModel) handleViewportFocusKeyMsg(msg tea.KeyPressMsg) bool {
	switch msg.String() {
	case keyUp, keyShiftUp:
		m.viewport.ScrollUp(1)
		m.userScrolled = !m.viewport.AtBottom()
		return true
	case keyDown, keyShiftDown:
		m.viewport.ScrollDown(1)
		m.userScrolled = !m.viewport.AtBottom()
		return true
	case keyPageUp:
		m.viewport.HalfPageUp()
		m.userScrolled = !m.viewport.AtBottom()
		return true
	case keyPageDown:
		m.viewport.HalfPageDown()
		m.userScrolled = !m.viewport.AtBottom()
		return true
	case keyHome:
		m.viewport.GotoTop()
		m.userScrolled = true
		return true
	case keyEnd:
		m.viewport.GotoBottom()
		m.userScrolled = false
		return true
	}
	return false
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

func (m *replModel) cancelBtwStream() {
	if m.btwStreamHandler != nil && m.btwStreamHandler.IsActive() {
		m.btwStreamHandler.HandleInterrupt()
	}
	if m.btwStreamCancel != nil {
		m.btwStreamCancel()
		m.btwStreamCancel = nil
	}
	m.btwLines = nil
	m.btwShowSpinner = false
}

func (m *replModel) flushBtwToOutput() {
	if m.btwLines == nil {
		return
	}
	m.output.AddLine(renderBtwLeftBorder(renderBtwQuestionHeader(m.btwQuestion)))
	for _, line := range m.btwLines {
		m.output.AddLine(renderBtwLeftBorder(line))
	}
	m.output.AddEmptyLine()
	m.btwLines = nil
	m.btwQuestion = ""
}

func (m *replModel) cancelAdversaryStream() {
	if m.adversary.streamHandler != nil && m.adversary.streamHandler.IsActive() {
		m.adversary.streamHandler.HandleInterrupt()
	}
	if m.adversary.streamCancel != nil {
		m.adversary.streamCancel()
		m.adversary.streamCancel = nil
	}
	m.adversary.lines = nil
	m.adversary.showSpinner = false
}

func (m *replModel) flushAdversaryToOutput() {
	if m.adversary.lines == nil {
		return
	}
	m.output.AddLine(renderAdversaryLeftBorder(renderAdversaryHeader(m.adversary.focus)))
	for _, line := range m.adversary.lines {
		m.output.AddLine(renderAdversaryLeftBorder(line))
	}
	m.output.AddEmptyLine()
	m.adversary.lines = nil
	m.adversary.focus = ""
}

func (m *replModel) buildAdversaryClient() error {
	resolved, err := config.ResolveAdversary(m.ctx.globalCfg)
	if err != nil {
		return err
	}
	client, err := llm.NewClient(resolved)
	if err != nil {
		return err
	}
	m.appState.SetAdversaryClient(client)
	return nil
}

func waitForAdversaryEvent(llmCh <-chan llm.StreamEvent) tea.Cmd {
	if llmCh == nil {
		return nil
	}

	return func() tea.Msg {
		for {
			event, ok := <-llmCh
			if !ok {
				return adversaryDoneMsg{}
			}

			switch event.Type {
			case llm.StreamEventTypeChunk:
				return adversaryChunkMsg(event.Content)
			case llm.StreamEventTypeDone:
				return adversaryDoneMsg{}
			case llm.StreamEventTypeError:
				return adversaryErrorMsg{err: event.Error}
			case llm.StreamEventTypeIncomplete:
				return adversaryErrorMsg{err: event.Error}
			default:
				continue
			}
		}
	}
}

func (m *replModel) scrollToBottomIfFollowing() {
	if !m.userScrolled {
		m.viewport.GotoBottom()
	}
}

func waitForBtwEvent(llmCh <-chan llm.StreamEvent) tea.Cmd {
	if llmCh == nil {
		return nil
	}

	return func() tea.Msg {
		for {
			event, ok := <-llmCh
			if !ok {
				return btwDoneMsg{}
			}

			switch event.Type {
			case llm.StreamEventTypeChunk:
				return btwChunkMsg(event.Content)
			case llm.StreamEventTypeDone:
				return btwDoneMsg{}
			case llm.StreamEventTypeError:
				return btwErrorMsg{err: event.Error}
			case llm.StreamEventTypeIncomplete:
				return btwErrorMsg{err: event.Error}
			default:
				continue
			}
		}
	}
}

func (m *replModel) applyWindowSize(msg tea.WindowSizeMsg) {
	m.width = msg.Width
	m.height = msg.Height
	m.textarea.SetWidth(msg.Width - 3)
	if m.mdRenderer != nil {
		m.mdRenderer.UpdateWidth(msg.Width)
	}
	if m.output != nil {
		m.output.SetWidth(msg.Width)
	}
	m.viewport.SetWidth(msg.Width)
	m.viewport.SetHeight(msg.Height - m.textarea.Height() - 4 - m.spinnerHeight() - m.suggestion.Height())
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
