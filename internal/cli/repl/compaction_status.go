package repl

import "charm.land/lipgloss/v2"

func addCompactionSuccessStatus(output *OutputBuilder, status string) {
	addCompactionStatus(output, "✓", status, compactionSuccessStyle)
}

func addCompactionErrorStatus(output *OutputBuilder, status string) {
	addCompactionStatus(output, "✗", status, compactionErrorStyle)
}

func addCompactionCancelledStatus(output *OutputBuilder, status string) {
	addCompactionStatus(output, "", status, compactionCancelledStyle)
}

func addCompactionStatus(output *OutputBuilder, icon, status string, style lipgloss.Style) {
	if output == nil || status == "" {
		return
	}

	line := "  "
	if icon != "" {
		line += icon + " "
	}
	line += status

	output.AddStyledLine(line, style)
	output.AddEmptyLine()
}
