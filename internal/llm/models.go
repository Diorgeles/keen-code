package llm

import (
	"fmt"
	"strings"

	"github.com/user/keen-code/internal/config"
)

type Provider string

const (
	deepSeekBaseURL   = "https://api.deepseek.com/"
	moonshotAIBaseURL = "https://api.moonshot.ai/v1/"
	zaiBaseURL        = "https://api.z.ai/api/paas/v4/"
	miniMaxBaseURL    = "https://api.minimax.io/anthropic"
	openCodeGoBaseURL = "https://opencode.ai/zen/go"
)

type ClientConfig struct {
	Provider       Provider
	APIKey         string
	Model          string
	ThinkingEffort string
	BaseURL        string
	MaxRetries     int
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
	case config.ProviderMiniMax:
		return NewAnthropicClient(&ClientConfig{
			Provider: Provider(cfg.Provider),
			APIKey:   cfg.APIKey,
			Model:    cfg.Model,
			BaseURL:  cfg.BaseURL,
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
	case config.ProviderOpenCodeGo:
		if isOpenCodeGoAnthropicModel(cfg.Model) {
			return NewAnthropicClient(&ClientConfig{
				Provider: Provider(cfg.Provider),
				APIKey:   cfg.APIKey,
				Model:    cfg.Model,
				BaseURL:  cfg.BaseURL,
			})
		}
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

func isOpenCodeGoMiniMaxModel(model string) bool {
	return strings.HasPrefix(model, "minimax-m2.")
}

func isOpenCodeGoAnthropicModel(model string) bool {
	return isOpenCodeGoMiniMaxModel(model) || model == "qwen3.7-max"
}

func isOpenCodeGoDeepSeekModel(model string) bool {
	return strings.HasPrefix(model, "deepseek-")
}

func isOpenCodeGoGLMModel(model string) bool {
	return strings.HasPrefix(model, "glm-")
}

func isOpenCodeGoKimiModel(model string) bool {
	return strings.HasPrefix(model, "kimi-")
}

func isOpenCodeGoQwenModel(model string) bool {
	return strings.HasPrefix(model, "qwen")
}
