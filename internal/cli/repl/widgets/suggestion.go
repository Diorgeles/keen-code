package widgets

import (
	"strings"

	"charm.land/lipgloss/v2"
	replcommands "github.com/user/keen-code/internal/cli/repl/commands"
	repltheme "github.com/user/keen-code/internal/cli/repl/theme"
)

type suggestionMode int

const (
	commandMode suggestionMode = iota
	fileMode
)

// SuggestionItem is a generic suggestion entry used for both slash commands and file paths.
type SuggestionItem struct {
	Name        string
	Description string
}

type SuggestionModel struct {
	visible  bool
	items    []SuggestionItem
	selected int
	mode     suggestionMode
}

func NewSuggestionModel() SuggestionModel {
	return SuggestionModel{}
}

// Refresh filters slash commands matching input and shows them.
func (s *SuggestionModel) Refresh(input string) {
	s.mode = commandMode
	cmds := replcommands.Filter(input)
	s.items = make([]SuggestionItem, len(cmds))
	for i, c := range cmds {
		s.items[i] = SuggestionItem{Name: c.Name, Description: c.Description}
	}
	if len(s.items) > 0 {
		s.visible = true
		s.selected = 0
	} else {
		s.visible = false
		s.items = nil
	}
}

// RefreshFiles sets file path suggestions directly.
func (s *SuggestionModel) RefreshFiles(paths []string) {
	s.mode = fileMode
	if len(paths) == 0 {
		s.visible = false
		s.items = nil
		return
	}
	s.items = make([]SuggestionItem, len(paths))
	for i, p := range paths {
		s.items[i] = SuggestionItem{Name: p}
	}
	s.visible = true
	s.selected = 0
}

func (s *SuggestionModel) Hide() {
	s.visible = false
	s.items = nil
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

func (s SuggestionModel) Current() *SuggestionItem {
	if !s.visible || len(s.items) == 0 {
		return nil
	}
	return &s.items[s.selected]
}

func (s SuggestionModel) First() *SuggestionItem {
	if len(s.items) == 0 {
		return nil
	}
	return &s.items[0]
}

func (s SuggestionModel) IsFileMode() bool {
	return s.mode == fileMode
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

		var row string
		if item.Description != "" {
			row = lipgloss.JoinHorizontal(lipgloss.Left,
				cmdStyle.Render(item.Name),
				descStyle.Render(item.Description),
			)
		} else {
			row = cmdStyle.Render(item.Name)
		}
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
