package widgets

import (
	"strings"

	"charm.land/lipgloss/v2"
	replcommands "github.com/user/keen-code/internal/cli/repl/commands"
	repltheme "github.com/user/keen-code/internal/cli/repl/theme"
)

type SuggestionModel struct {
	visible  bool
	items    []replcommands.SlashCommand
	selected int
}

func NewSuggestionModel() SuggestionModel {
	return SuggestionModel{}
}

func (s *SuggestionModel) Refresh(input string) {
	s.items = replcommands.Filter(input)
	if len(s.items) > 0 {
		s.visible = true
		s.selected = 0
	} else {
		s.visible = false
		s.items = nil
	}
}

func (s *SuggestionModel) MoveDown() {
	if len(s.items) == 0 {
		return
	}
	s.selected = (s.selected + 1) % len(s.items)
}

func (s *SuggestionModel) MoveUp() {
	if len(s.items) == 0 {
		return
	}
	s.selected = (s.selected - 1 + len(s.items)) % len(s.items)
}

func (s SuggestionModel) Current() *replcommands.SlashCommand {
	if !s.visible || len(s.items) == 0 {
		return nil
	}
	return &s.items[s.selected]
}

func (s SuggestionModel) First() *replcommands.SlashCommand {
	if len(s.items) == 0 {
		return nil
	}
	return &s.items[0]
}

func (s SuggestionModel) Height() int {
	if !s.visible {
		return 0
	}
	return len(s.items) + 2
}

func (s SuggestionModel) Visible() bool {
	return s.visible
}

func (s SuggestionModel) View(width int) string {
	if !s.visible {
		return ""
	}

	cmdColWidth := 0
	for _, item := range s.items {
		if len(item.Name) > cmdColWidth {
			cmdColWidth = len(item.Name)
		}
	}
	cmdColWidth += 2

	var rows []string
	for i, item := range s.items {
		isSelected := i == s.selected

		var cmdStyle, descStyle lipgloss.Style
		if isSelected {
			cmdStyle = repltheme.SuggestionSelectedCmdStyle.Width(cmdColWidth)
			descStyle = repltheme.SuggestionSelectedDescStyle
		} else {
			cmdStyle = repltheme.SuggestionCmdStyle.Width(cmdColWidth)
			descStyle = repltheme.SuggestionDescStyle
		}

		row := lipgloss.JoinHorizontal(lipgloss.Left,
			cmdStyle.Render(item.Name),
			descStyle.Render(item.Description),
		)
		rows = append(rows, row)
	}

	inner := strings.Join(rows, "\n")

	hasSelection := s.selected >= 0 && len(s.items) > 0
	containerStyle := repltheme.SuggestionContainerStyle
	if hasSelection {
		containerStyle = containerStyle.BorderForeground(repltheme.PrimaryColor)
	}

	box := containerStyle.Render(inner)

	boxWidth := lipgloss.Width(box)
	if boxWidth < width {
		lines := strings.Split(box, "\n")
		var padded []string
		for _, l := range lines {
			lw := lipgloss.Width(l)
			if lw < width {
				padded = append(padded, l+strings.Repeat(" ", width-lw))
			} else {
				padded = append(padded, l)
			}
		}
		return strings.Join(padded, "\n")
	}
	return box
}
