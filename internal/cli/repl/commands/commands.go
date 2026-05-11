package commands

import "strings"

const (
	Btw          = "/btw"
	Clear        = "/clear"
	Compact      = "/compact"
	Exit         = "/exit"
	Help         = "/help"
	Logout       = "/logout"
	Model        = "/model"
	New          = "/new"
	Resume       = "/resume"
	Sessions     = "/sessions"
	ShowThinking = "/show-thinking"
	Skills       = "/skills"
	Thinking     = "/thinking"
)

type SlashCommand struct {
	Name        string
	Description string
}

var All = []SlashCommand{
	{Btw, "Ask a quick side question (not added to conversation)"},
	{Clear, "Clear the session and create a new one (also /new)"},
	{Compact, "Compact conversation context"},
	{Exit, "Quit Keen"},
	{Help, "Show available commands"},
	{Logout, "Sign out of the current OAuth provider"},
	{Model, "Change provider or model"},
	{New, "Start a new session (also /clear)"},
	{Resume, "Open the session picker"},
	{Sessions, "List saved sessions for this directory"},
	{ShowThinking, "Toggle thinking token display (on|off)"},
	{Skills, "List, reload, enable, or disable skills"},
	{Thinking, "Change thinking effort for the current model"},
}

func Filter(input string) []SlashCommand {
	if input == "" || !strings.HasPrefix(input, "/") {
		return nil
	}
	prefix := strings.ToLower(strings.TrimPrefix(input, "/"))
	var results []SlashCommand
	for _, cmd := range All {
		name := strings.TrimPrefix(cmd.Name, "/")
		if strings.HasPrefix(name, prefix) {
			results = append(results, cmd)
		}
	}
	return results
}
