package widgets

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/user/keen-code/internal/auth"
	"github.com/user/keen-code/internal/config"
	"github.com/user/keen-code/providers"
)

func TestIsValidBaseURL_Empty(t *testing.T) {
	if err := isValidBaseURL(""); err != nil {
		t.Errorf("expected empty URL to be valid, got %v", err)
	}
}

func TestIsValidBaseURL_ValidHTTPS(t *testing.T) {
	cases := []string{
		"https://api.example.com",
		"https://api.example.com/v1",
		"http://localhost:8080",
		"http://localhost:8080/v1/",
	}
	for _, c := range cases {
		if err := isValidBaseURL(c); err != nil {
			t.Errorf("expected %q to be valid, got %v", c, err)
		}
	}
}

func TestIsValidBaseURL_InvalidScheme(t *testing.T) {
	cases := []string{
		"ftp://example.com",
		"example.com",
		"//example.com",
	}
	for _, c := range cases {
		if err := isValidBaseURL(c); err == nil {
			t.Errorf("expected %q to be invalid, got nil", c)
		}
	}
}

func TestIsValidBaseURL_MissingHost(t *testing.T) {
	if err := isValidBaseURL("https://"); err == nil {
		t.Error("expected URL with no host to be invalid")
	}
}

func TestModelSelection_OpenAICodexSkipsAPIKey(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	registry := &providers.Registry{
		Providers: []providers.Provider{
			{
				ID:   config.ProviderOpenAICodex,
				Name: "Codex (ChatGPT OAuth)",
				Models: []providers.Model{
					{
						ID:              "gpt-5.4",
						Name:            "GPT-5.4",
						ThinkingEfforts: []string{"low", "medium", "high", "xhigh"},
					},
				},
			},
		},
	}
	global := config.DefaultGlobalConfig()
	resolved := &config.ResolvedConfig{}
	store := auth.NewStoreAt(t.TempDir() + "/auth.json")
	if err := store.Set(config.ProviderOpenAICodex, auth.OAuthCredential{
		Type:         "oauth",
		AccessToken:  "access",
		RefreshToken: "refresh",
	}); err != nil {
		t.Fatalf("seed auth store: %v", err)
	}
	manager := auth.NewOAuthManager(store)

	completed := false
	m := NewWithAuthManager(registry, global, config.NewLoader(), resolved, manager, func(provider, model, apiKey string) error {
		completed = true
		if apiKey != "" {
			t.Fatalf("expected empty API key, got %q", apiKey)
		}
		return nil
	})

	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("did not expect OAuth command for existing credentials")
	}
	if m.Step != StepModel {
		t.Fatalf("expected StepModel, got %v", m.Step)
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.Step != StepThinking {
		t.Fatalf("expected StepThinking, got %v", m.Step)
	}
	m, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !completed {
		t.Fatal("expected completion")
	}
	if cmd == nil {
		t.Fatal("expected completion command")
	}
	if resolved.Provider != config.ProviderOpenAICodex || resolved.Model != "gpt-5.4" {
		t.Fatalf("unexpected resolved config: %+v", resolved)
	}
	if resolved.APIKey != "" || resolved.BaseURL != "" {
		t.Fatalf("expected no API key/base URL for Codex, got %+v", resolved)
	}
}
