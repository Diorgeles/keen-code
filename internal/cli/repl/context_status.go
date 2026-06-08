package repl

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	repltheme "github.com/user/keen-code/internal/cli/repl/theme"
)

const (
	compactionSuggestThreshold = 70.0
)

type contextStatus struct {
	CurrentTokens int
	ContextWindow int
	Percent       float64
	KnownWindow   bool
	KnownTokens   bool
}

func (s contextStatus) ShouldSuggestCompaction() bool {
	return s.KnownWindow && s.KnownTokens && s.Percent >= compactionSuggestThreshold
}

func usagePercent(currentTokens, contextWindow int) float64 {
	if currentTokens <= 0 || contextWindow <= 0 {
		return 0
	}
	percent := (float64(currentTokens) * 100.0) / float64(contextWindow)
	if percent > 100 {
		return 100
	}
	if percent < 0 {
		return 0
	}
	return percent
}

func (m replModel) computeContextStatus() contextStatus {
	var providerID, modelID string
	if m.ctx != nil && m.ctx.cfg != nil {
		providerID = m.ctx.cfg.Provider
		modelID = m.ctx.cfg.Model
	}

	var contextWindow int
	var knownWindow bool
	if m.ctx != nil && m.ctx.registry != nil && providerID != "" && modelID != "" {
		contextWindow, knownWindow = m.ctx.registry.GetModelContextWindow(providerID, modelID)
	}

	var currentTokens int
	var knownTokens bool
	if m.appState != nil {
		if usage := m.appState.GetLastUsage(); usage != nil {
			currentTokens = usage.InputTokens
			knownTokens = true
		}
	}

	status := contextStatus{
		CurrentTokens: currentTokens,
		ContextWindow: contextWindow,
		KnownWindow:   knownWindow,
		KnownTokens:   knownTokens,
	}
	if knownWindow && knownTokens {
		status.Percent = usagePercent(currentTokens, contextWindow)
	}
	return status
}

func (m *replModel) refreshContextStatus() {
	if m == nil {
		return
	}
	m.contextStatus = m.computeContextStatus()
}

func formatPercent(percent float64) string {
	p := strconv.FormatFloat(percent, 'f', 2, 64)
	p = strings.TrimRight(p, "0")
	p = strings.TrimRight(p, ".")
	return p + "%"
}

func contextPercentStyle(percent float64) lipgloss.Style {
	if percent >= 95 {
		return repltheme.ContextStatusPercentCriticalStyle
	}
	if percent >= 80 {
		return repltheme.ContextStatusPercentWarnStyle
	}
	return repltheme.ContextStatusPercentStyle
}

func renderContextStatus(status contextStatus) string {
	label := repltheme.ContextStatusLabelStyle.Render(" ◒")
	if !status.KnownWindow || status.ContextWindow <= 0 {
		return label + " " + repltheme.ContextStatusUnknownStyle.Render("N/A")
	}
	if !status.KnownTokens {
		return label + " " + repltheme.ContextStatusPercentStyle.Render("0.0%")
	}

	percent := contextPercentStyle(status.Percent).Render(formatPercent(status.Percent))
	return label + " " + percent
}
