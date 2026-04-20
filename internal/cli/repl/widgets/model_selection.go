package widgets

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	repltheme "github.com/user/keen-code/internal/cli/repl/theme"
	"github.com/user/keen-code/internal/config"
	"github.com/user/keen-code/providers"
)

type Step int

const (
	StepProvider Step = iota
	StepModel
	StepThinking
	StepAPIKey
)

type modelSelectionCompleteMsg struct{}
type modelSelectionCancelMsg struct{}

type Model struct {
	Step             Step
	SelectedProvider string
	SelectedModel    string
	APIKeyInput      string
	ProviderCursor   int
	ModelCursor      int
	ThinkingCursor   int
	ThinkingOptions  []string
	SelectedThinking string
	ProviderList     []providers.Provider
	ModelList        []providers.Model
	ErrorMessage     string
	registry         *providers.Registry
	globalCfg        *config.GlobalConfig
	loader           *config.Loader
	resolvedCfg      *config.ResolvedConfig
	onComplete       func(provider, model, apiKey string) error
}

func New(registry *providers.Registry, globalCfg *config.GlobalConfig, loader *config.Loader, resolvedCfg *config.ResolvedConfig, onComplete func(provider, model, apiKey string) error) *Model {
	return &Model{
		Step:         StepProvider,
		ProviderList: registry.Providers,
		registry:     registry,
		globalCfg:    globalCfg,
		loader:       loader,
		resolvedCfg:  resolvedCfg,
		onComplete:   onComplete,
	}
}

func (m *Model) Update(msg tea.Msg) (*Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m.handleKeyMsg(msg)
	case tea.PasteMsg:
		return m.handlePasteMsg(msg)
	}
	return m, nil
}

func (m *Model) handleKeyMsg(msg tea.KeyPressMsg) (*Model, tea.Cmd) {
	switch m.Step {
	case StepProvider:
		switch msg.String() {
		case "up", "k":
			m.ProviderCursor = (m.ProviderCursor - 1 + len(m.ProviderList)) % len(m.ProviderList)
		case "down", "j":
			m.ProviderCursor = (m.ProviderCursor + 1) % len(m.ProviderList)
		case "enter":
			m.SelectedProvider = m.ProviderList[m.ProviderCursor].ID
			provider, _ := m.registry.GetProvider(m.SelectedProvider)
			m.ModelList = provider.Models
			m.ModelCursor = 0
			m.Step = StepModel
		case "esc":
			return m, func() tea.Msg { return modelSelectionCancelMsg{} }
		}

	case StepModel:
		switch msg.String() {
		case "up", "k":
			m.ModelCursor = (m.ModelCursor - 1 + len(m.ModelList)) % len(m.ModelList)
		case "down", "j":
			m.ModelCursor = (m.ModelCursor + 1) % len(m.ModelList)
		case "enter":
			m.SelectedModel = m.ModelList[m.ModelCursor].ID
			modelMeta, ok := m.registry.GetModel(m.SelectedProvider, m.SelectedModel)
			if ok && modelMeta.SupportsThinkingEffort() {
				m.ThinkingOptions = append([]string{"off"}, modelMeta.ThinkingEfforts...)
				m.ThinkingCursor = m.resolveThinkingCursor(m.ThinkingOptions)
				m.Step = StepThinking
			} else {
				m.Step = StepAPIKey
			}
		case "esc":
			return m, func() tea.Msg { return modelSelectionCancelMsg{} }
		}

	case StepThinking:
		switch msg.String() {
		case "up", "k":
			m.ThinkingCursor = (m.ThinkingCursor - 1 + len(m.ThinkingOptions)) % len(m.ThinkingOptions)
		case "down", "j":
			m.ThinkingCursor = (m.ThinkingCursor + 1) % len(m.ThinkingOptions)
		case "enter":
			m.SelectedThinking = m.ThinkingOptions[m.ThinkingCursor]
			m.Step = StepAPIKey
		case "esc":
			return m, func() tea.Msg { return modelSelectionCancelMsg{} }
		}

	case StepAPIKey:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return modelSelectionCancelMsg{} }
		case "enter":
			return m.complete()
		case "backspace":
			if len(m.APIKeyInput) > 0 {
				m.APIKeyInput = m.APIKeyInput[:len(m.APIKeyInput)-1]
			}
		default:
			if len(msg.Text) > 0 {
				m.APIKeyInput += msg.Text
			}
		}
	}

	return m, nil
}

func (m *Model) resolveThinkingCursor(options []string) int {
	currentEffort := ""
	if m.resolvedCfg != nil {
		currentEffort = m.resolvedCfg.ThinkingEffort
	}
	if currentEffort == "" {
		// Try "medium" default, else "off"
		if idx := slices.Index(options, "medium"); idx >= 0 {
			return idx
		}
		return 0 // "off"
	}
	if idx := slices.Index(options, currentEffort); idx >= 0 {
		return idx
	}
	// Saved value not compatible — prefer medium
	if idx := slices.Index(options, "medium"); idx >= 0 {
		return idx
	}
	return 0 // "off"
}

func (m *Model) handlePasteMsg(msg tea.PasteMsg) (*Model, tea.Cmd) {
	if m.Step == StepAPIKey && msg.Content != "" {
		m.APIKeyInput += msg.Content
	}
	return m, nil
}

func (m *Model) complete() (*Model, tea.Cmd) {
	existingKey := ""
	if providerCfg, exists := m.globalCfg.GetProviderConfig(m.SelectedProvider); exists {
		existingKey = providerCfg.APIKey
	}

	apiKey := m.APIKeyInput
	if apiKey == "" && existingKey != "" {
		apiKey = existingKey
	}

	if apiKey == "" {
		m.ErrorMessage = "API key is required"
		return m, nil
	}

	// Resolve the stored effort value ("off" → "")
	storedEffort := m.SelectedThinking
	if storedEffort == "off" {
		storedEffort = ""
	}

	// If model doesn't support configurable effort, clear any stale value
	modelMeta, ok := m.registry.GetModel(m.SelectedProvider, m.SelectedModel)
	if !ok || !modelMeta.SupportsThinkingEffort() {
		storedEffort = ""
	}

	m.globalCfg.ActiveProvider = m.SelectedProvider
	m.globalCfg.ActiveModel = m.SelectedModel
	m.globalCfg.ThinkingEffort = storedEffort

	providerCfg := config.ProviderConfig{
		APIKey: apiKey,
		Models: []string{m.SelectedModel},
	}
	m.globalCfg.SetProviderConfig(m.SelectedProvider, providerCfg)

	if err := m.loader.Save(m.globalCfg); err != nil {
		m.ErrorMessage = fmt.Sprintf("Failed to save config: %v", err)
		return m, nil
	}

	m.resolvedCfg.Provider = m.SelectedProvider
	m.resolvedCfg.Model = m.SelectedModel
	m.resolvedCfg.APIKey = apiKey
	m.resolvedCfg.ThinkingEffort = storedEffort

	if err := m.onComplete(m.SelectedProvider, m.SelectedModel, apiKey); err != nil {
		m.ErrorMessage = fmt.Sprintf("Failed to initialize LLM client: %v", err)
		return m, nil
	}

	return m, func() tea.Msg { return modelSelectionCompleteMsg{} }
}

func (m *Model) ViewString() string {
	switch m.Step {
	case StepProvider:
		return m.renderProviderSelection()
	case StepModel:
		return m.renderModelSelection()
	case StepThinking:
		return m.renderThinkingSelection()
	case StepAPIKey:
		return m.renderAPIKeyInput()
	}
	return ""
}

func (m *Model) renderProviderSelection() string {
	var view strings.Builder
	view.WriteString(repltheme.UserPromptStyle.Render("Select a provider:"))
	view.WriteString("\n\n")
	view.WriteString(m.renderList(m.ProviderCursor, func(i int) string { return m.ProviderList[i].Name }, len(m.ProviderList)))
	view.WriteString("\n" + repltheme.HintStyle.Render("[↑/↓ to navigate, Enter to select, Esc to cancel]"))
	return view.String()
}

func (m *Model) renderModelSelection() string {
	var view strings.Builder
	providerName := m.getProviderName(m.SelectedProvider)
	view.WriteString(repltheme.UserPromptStyle.Render(fmt.Sprintf("Select a model for %s:", providerName)))
	view.WriteString("\n\n")
	view.WriteString(m.renderList(m.ModelCursor, func(i int) string { return m.ModelList[i].Name }, len(m.ModelList)))
	view.WriteString("\n" + repltheme.HintStyle.Render("[↑/↓ to navigate, Enter to select, Esc to cancel]"))
	return view.String()
}

func (m *Model) renderThinkingSelection() string {
	var view strings.Builder
	view.WriteString(repltheme.UserPromptStyle.Render("Select thinking effort:"))
	view.WriteString("\n\n")
	view.WriteString(m.renderList(m.ThinkingCursor, func(i int) string { return m.ThinkingOptions[i] }, len(m.ThinkingOptions)))
	view.WriteString("\n" + repltheme.HintStyle.Render("[↑/↓ to navigate, Enter to select, Esc to cancel]"))
	return view.String()
}

func (m *Model) renderAPIKeyInput() string {
	var view strings.Builder
	providerName := m.getProviderName(m.SelectedProvider)
	existingKey := m.getExistingAPIKey(m.SelectedProvider)

	title := fmt.Sprintf("Enter API key for %s", providerName)
	if existingKey != "" {
		title += "\n" + repltheme.HintStyle.Render("(press Enter to keep existing key)")
	}
	view.WriteString(repltheme.UserPromptStyle.Render(title))
	view.WriteString("\n\n")

	maskedKey := strings.Repeat("•", len(m.APIKeyInput))
	view.WriteString(repltheme.PromptStyle.Render("> ") + maskedKey)
	view.WriteString("\n\n" + repltheme.HintStyle.Render("[Enter to confirm, Esc to cancel]"))

	if m.ErrorMessage != "" {
		view.WriteString("\n" + repltheme.ErrorStyle.Render(m.ErrorMessage))
	}
	return view.String()
}

func (m *Model) renderList(cursor int, getName func(int) string, count int) string {
	var view strings.Builder
	for i := 0; i < count; i++ {
		if i == cursor {
			view.WriteString(repltheme.ModelSelectionStyle.Render("> " + getName(i)))
			view.WriteString("\n")
			continue
		}
		view.WriteString("  " + repltheme.NormalStyle.Render(getName(i)) + "\n")
	}
	return view.String()
}

func (m *Model) getProviderName(providerID string) string {
	if m.registry == nil {
		return ""
	}
	if provider, ok := m.registry.GetProvider(providerID); ok {
		return provider.Name
	}
	return ""
}

func (m *Model) getExistingAPIKey(providerID string) string {
	if m.globalCfg == nil {
		return ""
	}
	if providerCfg, exists := m.globalCfg.GetProviderConfig(providerID); exists {
		return providerCfg.APIKey
	}
	return ""
}

func IsComplete(msg tea.Msg) bool {
	_, ok := msg.(modelSelectionCompleteMsg)
	return ok
}

func IsCancel(msg tea.Msg) bool {
	_, ok := msg.(modelSelectionCancelMsg)
	return ok
}
