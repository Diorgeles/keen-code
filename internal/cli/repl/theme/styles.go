package theme

import (
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"
)

var (
	PrimaryColor = compat.AdaptiveColor{
		Light: lipgloss.Color("#3F51B5"),
		Dark:  lipgloss.Color("#5C6BC0"),
	}
	SecondaryColor = compat.AdaptiveColor{
		Light: lipgloss.Color("#00897B"),
		Dark:  lipgloss.Color("#4DB6AC"),
	}
	MutedColor = compat.AdaptiveColor{
		Light: lipgloss.Color("#757575"),
		Dark:  lipgloss.Color("#BDBDBD"),
	}
	AccentColor = compat.AdaptiveColor{
		Light: lipgloss.Color("#FF8F00"),
		Dark:  lipgloss.Color("#FFB300"),
	}

	TextPrimaryColor = compat.AdaptiveColor{
		Light: lipgloss.Color("#242323"),
		Dark:  lipgloss.Color("#BDBDBD"),
	}
	TextSecondaryColor = compat.AdaptiveColor{
		Light: lipgloss.Color("#424242"),
		Dark:  lipgloss.Color("#BDBDBD"),
	}
	TextDimColor = compat.AdaptiveColor{
		Light: lipgloss.Color("#5e5d5d"),
		Dark:  lipgloss.Color("#B3B3B3"),
	}
	RuleColor = compat.AdaptiveColor{
		Light: lipgloss.Color("#bdbdbd"),
		Dark:  lipgloss.Color("#3b3b3b"),
	}
	UserInputBlockBackground = compat.AdaptiveColor{
		Light: lipgloss.Color("#E0E0E0"),
		Dark:  lipgloss.Color("#2E2E2E"),
	}

	ErrorColor = compat.AdaptiveColor{Light: lipgloss.Color("#D32F2F"), Dark: lipgloss.Color("#EF5350")}
	WhiteColor = compat.AdaptiveColor{Light: lipgloss.Color("#FFFFFF"), Dark: lipgloss.Color("#FFFFFF")}

	DiffAddColor = compat.AdaptiveColor{
		Light: lipgloss.Color("#2E7D32"),
		Dark:  lipgloss.Color("#66BB6A"),
	}
	DiffRemoveColor = compat.AdaptiveColor{
		Light: lipgloss.Color("#C62828"),
		Dark:  lipgloss.Color("#EF5350"),
	}
	DiffContextColor = compat.AdaptiveColor{
		Light: lipgloss.Color("#616161"),
		Dark:  lipgloss.Color("#9E9E9E"),
	}
	DiffHunkColor = compat.AdaptiveColor{
		Light: lipgloss.Color("#1565C0"),
		Dark:  lipgloss.Color("#42A5F5"),
	}

	NormalStyle    = lipgloss.NewStyle()
	TitleStyle     = lipgloss.NewStyle().Bold(true).Foreground(PrimaryColor)
	TipStyle       = lipgloss.NewStyle().Foreground(TextDimColor).Italic(true)
	HintStyle      = lipgloss.NewStyle().Foreground(TextDimColor)
	UsageHintStyle = lipgloss.NewStyle().Foreground(SecondaryColor).Bold(true)
	BoxStyle       = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(TextDimColor).
			Padding(1, 2).
			MarginTop(1)
	HighlightStyle = lipgloss.NewStyle().Foreground(SecondaryColor)

	PromptStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(PrimaryColor).
			MarginTop(2)
	InputRuleStyle      = lipgloss.NewStyle().Foreground(PrimaryColor)
	UserInputBlockStyle = lipgloss.NewStyle().
				Background(UserInputBlockBackground).
				Padding(1, 1)

	InfoLabelStyle = lipgloss.NewStyle().Foreground(MutedColor).Width(18)
	InfoValueStyle = lipgloss.NewStyle().Foreground(TextDimColor)

	HelpCmdStyle   = lipgloss.NewStyle().Foreground(SecondaryColor).Bold(true).Width(12)
	HelpDescStyle  = lipgloss.NewStyle().Foreground(TextSecondaryColor)
	TimestampStyle = lipgloss.NewStyle().Foreground(TextDimColor)

	AssistantStyle           = lipgloss.NewStyle().Foreground(TextPrimaryColor)
	ReasoningStyle           = lipgloss.NewStyle().Foreground(TextDimColor).Faint(true)
	ErrorStyle               = lipgloss.NewStyle().Foreground(ErrorColor)
	InterruptedStyle         = lipgloss.NewStyle().Foreground(PrimaryColor).Bold(true)
	LoadingTextStyled        = lipgloss.NewStyle().Foreground(PrimaryColor)
	LoadingTimerStyle        = lipgloss.NewStyle().Foreground(TextDimColor).Faint(true)
	CompactionSuccessStyle   = lipgloss.NewStyle().Foreground(SecondaryColor)
	CompactionErrorStyle     = lipgloss.NewStyle().Foreground(ErrorColor)
	CompactionCancelledStyle = lipgloss.NewStyle().Foreground(TextDimColor)

	ToolStartStyle    = lipgloss.NewStyle().Foreground(TextDimColor).Bold(true)
	ToolSuccessStyle  = lipgloss.NewStyle().Foreground(SecondaryColor)
	ToolErrorStyle    = lipgloss.NewStyle().Foreground(ErrorColor)
	WarningTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(ErrorColor)

	BashCommandStyle = lipgloss.NewStyle().Foreground(SecondaryColor)
	BashOutputStyle  = lipgloss.NewStyle().Foreground(TextDimColor)
	BashSummaryStyle = lipgloss.NewStyle().Foreground(MutedColor)

	DiffAddStyle     = lipgloss.NewStyle().Foreground(DiffAddColor)
	DiffRemoveStyle  = lipgloss.NewStyle().Foreground(DiffRemoveColor)
	DiffContextStyle = lipgloss.NewStyle().Foreground(DiffContextColor)
	DiffHunkStyle    = lipgloss.NewStyle().Foreground(DiffHunkColor).Bold(true)
	DiffLineNumStyle = lipgloss.NewStyle().Foreground(TextDimColor)
	RuleStyle        = lipgloss.NewStyle().Foreground(RuleColor)

	ModelSelectionStyle      = lipgloss.NewStyle().Foreground(SecondaryColor).Bold(true)
	ModelSelectionRuleStyle  = lipgloss.NewStyle().Foreground(SecondaryColor)
	UserPromptCardStyle      = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(SecondaryColor).Padding(1, 2)
	UserPromptStyle          = lipgloss.NewStyle().Bold(true).Foreground(SecondaryColor)
	UserPromptSelectionStyle = lipgloss.NewStyle().Foreground(SecondaryColor).Bold(true)

	SuggestionContainerStyle = lipgloss.NewStyle().
					BorderStyle(lipgloss.RoundedBorder()).
					BorderForeground(MutedColor).
					Padding(0, 1)
	SuggestionCmdStyle  = lipgloss.NewStyle().Foreground(SecondaryColor)
	SuggestionDescStyle = lipgloss.NewStyle().
				Foreground(MutedColor).
				PaddingLeft(2)
	SuggestionSelectedCmdStyle = lipgloss.NewStyle().
					Foreground(WhiteColor).
					Background(PrimaryColor).
					Bold(true)
	SuggestionSelectedDescStyle = lipgloss.NewStyle().
					Foreground(WhiteColor).
					Background(PrimaryColor).
					PaddingLeft(2)

	MetaLabelStyle                    = lipgloss.NewStyle().Foreground(TextDimColor)
	ContextStatusLabelStyle           = lipgloss.NewStyle().Foreground(TextDimColor)
	ContextStatusPercentStyle         = lipgloss.NewStyle().Foreground(SecondaryColor)
	ContextStatusPercentWarnStyle     = lipgloss.NewStyle().Foreground(AccentColor)
	ContextStatusPercentCriticalStyle = lipgloss.NewStyle().Foreground(ErrorColor)
	ContextStatusUnknownStyle         = lipgloss.NewStyle().Foreground(TextDimColor)
	CompactionSuggestionStyle         = lipgloss.NewStyle().Foreground(AccentColor)

	UpdateAvailableStyle = lipgloss.NewStyle().Foreground(AccentColor).Bold(true)
	UpdateCommandStyle   = lipgloss.NewStyle().Foreground(TextDimColor)

	BtwBorderStyle = lipgloss.NewStyle().Foreground(AccentColor)
	BtwLabelStyle  = lipgloss.NewStyle().Foreground(AccentColor).Bold(true)
	BtwHintStyle   = lipgloss.NewStyle().Foreground(MutedColor).Faint(true)
)
