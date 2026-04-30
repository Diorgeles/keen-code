package repl

import (
	"context"
	"math/rand"
	"os"
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	replpermissions "github.com/user/keen-code/internal/cli/repl/permissions"
	repltheme "github.com/user/keen-code/internal/cli/repl/theme"
	repltooling "github.com/user/keen-code/internal/cli/repl/tooling"
	replwidgets "github.com/user/keen-code/internal/cli/repl/widgets"
	"github.com/user/keen-code/internal/llm"
	"github.com/user/keen-code/internal/updater"
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
		"Use /help  for available commands",
		"Use /model to change provider or model",
		"Press Enter to send, Shift+Enter for new line",
		"Shift+click to select and copy text",
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

func (m *replModel) scrollToBottomIfFollowing() {
	if !m.userScrolled {
		m.viewport.GotoBottom()
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
