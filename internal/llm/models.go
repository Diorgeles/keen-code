package llm

import (
	"fmt"

	"github.com/user/keen-code/internal/config"
)

type Provider string

type ClientConfig struct {
	Provider       Provider
	APIKey         string
	Model          string
	ThinkingEffort string
	BaseURL        string
}

func NewClient(cfg *config.ResolvedConfig) (LLMClient, error) {
	if config.RequiresAPIKey(cfg.Provider) && cfg.APIKey == "" {
		return nil, fmt.Errorf("API key is required")
	}
	if cfg.Model == "" {
		return nil, fmt.Errorf("model is required")
	}

	switch cfg.Provider {
	case config.ProviderAnthropic:
		return NewAnthropicClient(&ClientConfig{
			Provider:       Provider(cfg.Provider),
			APIKey:         cfg.APIKey,
			Model:          cfg.Model,
			ThinkingEffort: cfg.ThinkingEffort,
			BaseURL:        cfg.BaseURL,
		})
	case config.ProviderGoogleAI:
		return NewGenkitClient(&ClientConfig{
			Provider:       Provider(cfg.Provider),
			APIKey:         cfg.APIKey,
			Model:          cfg.Model,
			ThinkingEffort: cfg.ThinkingEffort,
			BaseURL:        cfg.BaseURL,
		})
	case config.ProviderOpenAI:
		return NewOpenAIResponsesClient(&ClientConfig{
			Provider:       Provider(cfg.Provider),
			APIKey:         cfg.APIKey,
			Model:          cfg.Model,
			ThinkingEffort: cfg.ThinkingEffort,
			BaseURL:        cfg.BaseURL,
		})
	case config.ProviderOpenAICodex:
		return NewOpenAICodexClient(&ClientConfig{
			Provider:       Provider(cfg.Provider),
			Model:          cfg.Model,
			ThinkingEffort: cfg.ThinkingEffort,
		})
	case config.ProviderDeepSeek,
		config.ProviderMoonshotAI,
		config.ProviderZAI:
		return NewOpenAICompatibleClient(&ClientConfig{
			Provider:       Provider(cfg.Provider),
			APIKey:         cfg.APIKey,
			Model:          cfg.Model,
			ThinkingEffort: cfg.ThinkingEffort,
			BaseURL:        cfg.BaseURL,
		})
	default:
		return nil, fmt.Errorf("unsupported provider: %s", cfg.Provider)
	}
}
