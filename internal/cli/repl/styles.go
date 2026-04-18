package repl

import (
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"
)

var (
	// ── Base palette ────────────────────────────────────────────────────────────

	primaryColor = compat.AdaptiveColor{
		Light: lipgloss.Color("#3F51B5"),
		Dark:  lipgloss.Color("#5C6BC0"),
	}
	secondaryColor = compat.AdaptiveColor{
		Light: lipgloss.Color("#00897B"),
		Dark:  lipgloss.Color("#4DB6AC"),
	}
	mutedColor = compat.AdaptiveColor{
		Light: lipgloss.Color("#757575"),
		Dark:  lipgloss.Color("#BDBDBD"),
	}
	accentColor = compat.AdaptiveColor{
		Light: lipgloss.Color("#FF8F00"),
		Dark:  lipgloss.Color("#FFB300"),
	}

	// ── Text ────────────────────────────────────────────────────────────────────

	textPrimaryColor = compat.AdaptiveColor{
		Light: lipgloss.Color("#757575"),
		Dark:  lipgloss.Color("#BDBDBD"),
	}
	textSecondaryColor = compat.AdaptiveColor{
		Light: lipgloss.Color("#424242"),
		Dark:  lipgloss.Color("#BDBDBD"),
	}
	textDimColor = compat.AdaptiveColor{
		Light: lipgloss.Color("#9E9E9E"),
		Dark:  lipgloss.Color("#757575"),
	}

	// ── State ───────────────────────────────────────────────────────────────────

	errorColor = compat.AdaptiveColor{Light: lipgloss.Color("#D32F2F"), Dark: lipgloss.Color("#EF5350")}
	whiteColor = compat.AdaptiveColor{Light: lipgloss.Color("#FFFFFF"), Dark: lipgloss.Color("#FFFFFF")}

	// ── Diff ────────────────────────────────────────────────────────────────────

	diffAddColor = compat.AdaptiveColor{
		Light: lipgloss.Color("#2E7D32"),
		Dark:  lipgloss.Color("#66BB6A"),
	}
	diffRemoveColor = compat.AdaptiveColor{
		Light: lipgloss.Color("#C62828"),
		Dark:  lipgloss.Color("#EF5350"),
	}
	diffContextColor = compat.AdaptiveColor{
		Light: lipgloss.Color("#616161"),
		Dark:  lipgloss.Color("#9E9E9E"),
	}
	diffHunkColor = compat.AdaptiveColor{
		Light: lipgloss.Color("#1565C0"),
		Dark:  lipgloss.Color("#42A5F5"),
	}

	// ── General ─────────────────────────────────────────────────────────────────

	normalStyle = lipgloss.NewStyle()
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(primaryColor)
	tipStyle    = lipgloss.NewStyle().Foreground(textDimColor).Italic(true)
	hintStyle   = lipgloss.NewStyle().Foreground(textDimColor)
	boxStyle    = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(textDimColor).
			Padding(1, 2).
			MarginTop(1)
	highlightStyle = lipgloss.NewStyle().Foreground(secondaryColor)

	// ── Input ───────────────────────────────────────────────────────────────────

	promptStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			MarginTop(2)
	inputRuleStyle = lipgloss.NewStyle().Foreground(primaryColor)

	// ── Info / metadata ─────────────────────────────────────────────────────────

	infoLabelStyle = lipgloss.NewStyle().Foreground(mutedColor).Width(18)
	infoValueStyle = lipgloss.NewStyle().Foreground(textSecondaryColor)

	// ── Help ────────────────────────────────────────────────────────────────────

	helpCmdStyle   = lipgloss.NewStyle().Foreground(secondaryColor).Bold(true).Width(12)
	helpDescStyle  = lipgloss.NewStyle().Foreground(textSecondaryColor)
	timestampStyle = lipgloss.NewStyle().Foreground(textDimColor)

	// ── Assistant output ────────────────────────────────────────────────────────

	assistantStyle           = lipgloss.NewStyle().Foreground(textPrimaryColor)
	reasoningStyle           = lipgloss.NewStyle().Foreground(textDimColor).Faint(true)
	errorStyle               = lipgloss.NewStyle().Foreground(errorColor)
	interruptedStyle         = lipgloss.NewStyle().Foreground(primaryColor).Bold(true)
	loadingTextStyled        = lipgloss.NewStyle().Foreground(primaryColor)
	compactionSuccessStyle   = lipgloss.NewStyle().Foreground(secondaryColor)
	compactionErrorStyle     = lipgloss.NewStyle().Foreground(errorColor)
	compactionCancelledStyle = lipgloss.NewStyle().Foreground(textDimColor)

	// ── Tools ───────────────────────────────────────────────────────────────────

	toolStartStyle    = lipgloss.NewStyle().Foreground(textDimColor).Bold(true)
	toolSuccessStyle  = lipgloss.NewStyle().Foreground(secondaryColor)
	toolErrorStyle    = lipgloss.NewStyle().Foreground(errorColor)
	warningTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(errorColor)

	// ── Bash ────────────────────────────────────────────────────────────────────

	bashCommandStyle = lipgloss.NewStyle().Foreground(secondaryColor)
	bashOutputStyle  = lipgloss.NewStyle().Foreground(textDimColor)
	bashSummaryStyle = lipgloss.NewStyle().Foreground(mutedColor)

	// ── Diff ────────────────────────────────────────────────────────────────────

	diffAddStyle     = lipgloss.NewStyle().Foreground(diffAddColor)
	diffRemoveStyle  = lipgloss.NewStyle().Foreground(diffRemoveColor)
	diffContextStyle = lipgloss.NewStyle().Foreground(diffContextColor)
	diffHunkStyle    = lipgloss.NewStyle().Foreground(diffHunkColor).Bold(true)
	diffLineNumStyle = lipgloss.NewStyle().Foreground(textDimColor)
	diffRuleStyle    = lipgloss.NewStyle().Foreground(textDimColor)

	// ── Model selection / user prompt ────────────────────────────────────────────

	modelSelectionStyle      = lipgloss.NewStyle().Foreground(secondaryColor).Bold(true)
	userPromptCardStyle      = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(secondaryColor).Padding(1, 2)
	userPromptStyle          = lipgloss.NewStyle().Bold(true).Foreground(secondaryColor)
	userPromptSelectionStyle = lipgloss.NewStyle().Foreground(secondaryColor).Bold(true)

	// ── Suggestion dropdown ──────────────────────────────────────────────────────

	suggestionContainerStyle = lipgloss.NewStyle().
					BorderStyle(lipgloss.RoundedBorder()).
					BorderForeground(mutedColor).
					Padding(0, 1)
	suggestionCmdStyle         = lipgloss.NewStyle().Foreground(secondaryColor)
	suggestionDescStyle        = lipgloss.NewStyle().Foreground(mutedColor).PaddingLeft(2)
	suggestionSelectedCmdStyle = lipgloss.NewStyle().
					Foreground(whiteColor).
					Background(primaryColor).
					Bold(true)
	suggestionSelectedDescStyle = lipgloss.NewStyle().
					Foreground(whiteColor).
					Background(primaryColor).
					PaddingLeft(2)

	// ── Context status ───────────────────────────────────────────────────────────

	metaLabelStyle                    = lipgloss.NewStyle().Foreground(textDimColor)
	contextStatusLabelStyle           = lipgloss.NewStyle().Foreground(textDimColor)
	contextStatusPercentStyle         = lipgloss.NewStyle().Foreground(secondaryColor)
	contextStatusPercentWarnStyle     = lipgloss.NewStyle().Foreground(accentColor)
	contextStatusPercentCriticalStyle = lipgloss.NewStyle().Foreground(errorColor)
	contextStatusUnknownStyle         = lipgloss.NewStyle().Foreground(textDimColor)
	compactionSuggestionStyle         = lipgloss.NewStyle().Foreground(accentColor)
)
