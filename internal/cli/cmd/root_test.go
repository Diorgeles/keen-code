package cmd

import (
	"testing"

	"github.com/user/keen-code/internal/config"
)

func TestNewRootCommand(t *testing.T) {
	cmd := NewRootCommand("0.1.0")

	if cmd.Use != "keen" {
		t.Errorf("command Use = %q, want 'keen'", cmd.Use)
	}

	if cmd.Version != "0.1.0" {
		t.Errorf("command Version = %q, want '0.1.0'", cmd.Version)
	}

	if cmd.Short == "" {
		t.Error("command Short should not be empty")
	}

	if cmd.Long == "" {
		t.Error("command Long should not be empty")
	}
}

func TestNewRootCommand_DifferentVersion(t *testing.T) {
	cmd := NewRootCommand("1.2.3")

	if cmd.Version != "1.2.3" {
		t.Errorf("command Version = %q, want '1.2.3'", cmd.Version)
	}
}

func TestNewRootCommand_HasRunCommand(t *testing.T) {
	cmd := NewRootCommand("0.1.0")

	runCmd, _, err := cmd.Find([]string{"run"})
	if err != nil {
		t.Fatalf("Find(run) error = %v", err)
	}
	if runCmd == nil || runCmd.Name() != "run" {
		t.Fatalf("expected run command, got %#v", runCmd)
	}
}

func TestNewRootCommand_RunCommandHasModelProviderFlags(t *testing.T) {
	cmd := NewRootCommand("0.1.0")

	runCmd, _, err := cmd.Find([]string{"run"})
	if err != nil {
		t.Fatalf("Find(run) error = %v", err)
	}

	for _, name := range []string{"model", "provider"} {
		if runCmd.Flags().Lookup(name) == nil {
			t.Fatalf("expected run command to have --%s flag", name)
		}
	}
}

func TestApplyRunOverrides(t *testing.T) {
	globalCfg := &config.GlobalConfig{
		Providers: map[string]config.ProviderConfig{
			config.ProviderAnthropic: {
				APIKey:  "anthropic-key",
				Models:  []string{"claude-3"},
				BaseURL: "https://anthropic.example",
			},
			config.ProviderOpenCodeGo: {
				APIKey:  "opencode-key",
				Models:  []string{"kimi-k2.6"},
				BaseURL: "https://opencode.example",
			},
		},
	}
	resolvedCfg := &config.ResolvedConfig{
		Provider: config.ProviderAnthropic,
		APIKey:   "anthropic-key",
		Model:    "claude-3",
		BaseURL:  "https://anthropic.example",
		AuthMode: config.AuthModeAPIKey,
	}

	err := applyRunOverrides(globalCfg, resolvedCfg, config.ProviderOpenCodeGo, "qwen3.6-plus")
	if err != nil {
		t.Fatalf("applyRunOverrides() error = %v", err)
	}

	if resolvedCfg.Provider != config.ProviderOpenCodeGo {
		t.Fatalf("Provider = %q, want %q", resolvedCfg.Provider, config.ProviderOpenCodeGo)
	}
	if resolvedCfg.APIKey != "opencode-key" {
		t.Fatalf("APIKey = %q, want opencode-key", resolvedCfg.APIKey)
	}
	if resolvedCfg.BaseURL != "https://opencode.example" {
		t.Fatalf("BaseURL = %q, want https://opencode.example", resolvedCfg.BaseURL)
	}
	if resolvedCfg.Model != "qwen3.6-plus" {
		t.Fatalf("Model = %q, want qwen3.6-plus", resolvedCfg.Model)
	}
}

func TestApplyRunOverrides_ProviderUsesFirstConfiguredModel(t *testing.T) {
	globalCfg := &config.GlobalConfig{
		Providers: map[string]config.ProviderConfig{
			config.ProviderOpenCodeGo: {
				APIKey: "opencode-key",
				Models: []string{"kimi-k2.6"},
			},
		},
	}
	resolvedCfg := &config.ResolvedConfig{
		Provider: config.ProviderAnthropic,
		Model:    "claude-3",
	}

	err := applyRunOverrides(globalCfg, resolvedCfg, config.ProviderOpenCodeGo, "")
	if err != nil {
		t.Fatalf("applyRunOverrides() error = %v", err)
	}

	if resolvedCfg.Model != "kimi-k2.6" {
		t.Fatalf("Model = %q, want kimi-k2.6", resolvedCfg.Model)
	}
}

func TestBuildRunPrompt(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		stdin string
		want  string
	}{
		{name: "args only", args: []string{"hello", "there"}, want: "hello there"},
		{name: "stdin only", stdin: " from stdin\n", want: "from stdin"},
		{name: "args and stdin", args: []string{"hello"}, stdin: "from stdin\n", want: "hello\nfrom stdin"},
		{name: "empty", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildRunPrompt(tt.args, tt.stdin)
			if got != tt.want {
				t.Fatalf("buildRunPrompt() = %q, want %q", got, tt.want)
			}
		})
	}
}
