package repl

import "strings"

type slashCommand struct {
	Name        string
	Description string
}

var allSlashCommands = []slashCommand{
	{"/clear", "Clear the session and create a new one (also /new)"},
	{"/compact", "Compact conversation context"},
	{"/exit", "Quit Keen"},
	{"/help", "Show available commands"},
	{"/model", "Change provider or model"},
	{"/new", "Start a new session (also /clear)"},
	{"/resume", "Open the session picker"},
	{"/sessions", "List saved sessions for this directory"},
}

func filterCommands(input string) []slashCommand {
	if input == "" || !strings.HasPrefix(input, "/") {
		return nil
	}
	prefix := strings.ToLower(strings.TrimPrefix(input, "/"))
	var results []slashCommand
	for _, cmd := range allSlashCommands {
		name := strings.TrimPrefix(cmd.Name, "/")
		if strings.HasPrefix(name, prefix) {
			results = append(results, cmd)
		}
	}
	return results
}
