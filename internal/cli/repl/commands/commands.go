package commands

import "strings"

type SlashCommand struct {
	Name        string
	Description string
}

var all = []SlashCommand{
	{"/clear", "Clear the session and create a new one (also /new)"},
	{"/compact", "Compact conversation context"},
	{"/exit", "Quit Keen"},
	{"/help", "Show available commands"},
	{"/logout", "Sign out of the current OAuth provider"},
	{"/model", "Change provider or model"},
	{"/new", "Start a new session (also /clear)"},
	{"/resume", "Open the session picker"},
	{"/sessions", "List saved sessions for this directory"},
	{"/thinking", "Change thinking effort for the current model"},
}

func Filter(input string) []SlashCommand {
	if input == "" || !strings.HasPrefix(input, "/") {
		return nil
	}
	prefix := strings.ToLower(strings.TrimPrefix(input, "/"))
	var results []SlashCommand
	for _, cmd := range all {
		name := strings.TrimPrefix(cmd.Name, "/")
		if strings.HasPrefix(name, prefix) {
			results = append(results, cmd)
		}
	}
	return results
}
