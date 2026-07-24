package subagents

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/user/keen-code/internal/config"
	"github.com/user/keen-code/internal/llm"
	"github.com/user/keen-code/internal/tools"
)

type recordingClient struct {
	messages []llm.Message
	registry *tools.Registry
	options  []llm.StreamOptions
	timeout  time.Duration
}

func (c *recordingClient) StreamChat(ctx context.Context, messages []llm.Message, registry *tools.Registry, opts ...llm.StreamOptions) (<-chan llm.StreamEvent, error) {
	c.messages = llm.CloneMessages(messages)
	c.registry = registry
	c.options = append([]llm.StreamOptions(nil), opts...)
	if deadline, ok := ctx.Deadline(); ok {
		c.timeout = time.Until(deadline)
	}
	ch := make(chan llm.StreamEvent, 2)
	ch <- llm.StreamEvent{Type: llm.StreamEventTypeChunk, Content: "summary"}
	ch <- llm.StreamEvent{Type: llm.StreamEventTypeDone}
	close(ch)
	return ch, nil
}

func (c *recordingClient) Reset() {}

type namedTool struct{ name string }

func (t namedTool) Name() string                              { return t.name }
func (t namedTool) Description() string                       { return t.name }
func (t namedTool) InputSchema() map[string]any               { return map[string]any{"type": "object"} }
func (t namedTool) Execute(context.Context, any) (any, error) { return "ok", nil }

func TestRunnerUsesReadOnlyRegistryAndProfilePrompt(t *testing.T) {
	parentRegistry := tools.NewRegistry()
	for _, name := range []string{"read_file", "glob", "grep", "write_file", "delegate_task"} {
		if err := parentRegistry.Register(namedTool{name: name}); err != nil {
			t.Fatalf("register %s: %v", name, err)
		}
	}

	client := &recordingClient{}
	runner := &Runner{
		WorkingDir: "/repo",
		Config:     &config.ResolvedConfig{Provider: config.ProviderOpenAI, Model: "model", APIKey: "key"},
		Profiles: []Profile{{
			Name:         "explorer",
			Description:  "Explore code",
			Instructions: "Explore only what was asked.",
		}},
		NewClient: func(*config.ResolvedConfig) (llm.LLMClient, error) { return client, nil },
		Registry:  parentRegistry,
	}

	result, err := runner.Run(context.Background(), "explorer", "Inspect internal/subagents")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result.Status != "completed" || result.Result != "summary" {
		t.Fatalf("unexpected result: %+v", result)
	}
	for _, name := range []string{"read_file", "glob", "grep"} {
		if _, ok := client.registry.Get(name); !ok {
			t.Fatalf("expected child registry to contain %s", name)
		}
	}
	for _, name := range []string{"write_file", "delegate_task"} {
		if _, ok := client.registry.Get(name); ok {
			t.Fatalf("expected child registry to exclude %s", name)
		}
	}
	if len(client.messages) != 2 {
		t.Fatalf("expected 2 child messages, got %d", len(client.messages))
	}
	if got := client.messages[0].Content; !containsAll(got, []string{"Explore only what was asked.", "Working directory: /repo"}) {
		t.Fatalf("system prompt missing expected content: %s", got)
	}
	if got := client.messages[1].Content; !containsAll(got, []string{"Delegated task:", "Inspect internal/subagents"}) {
		t.Fatalf("user task missing expected content: %s", got)
	}
	if len(client.options) != 1 || !client.options[0].OneShot {
		t.Fatalf("expected subagent stream to be one-shot, got %#v", client.options)
	}
}

func TestRunnerUsesLiveProfileProvider(t *testing.T) {
	client := &recordingClient{}
	profiles := []Profile{{Name: "old", Description: "Old", Instructions: "Old prompt."}}
	runner := &Runner{
		WorkingDir: "/repo",
		Config:     &config.ResolvedConfig{Provider: config.ProviderOpenAI, Model: "model", APIKey: "key"},
		GetProfiles: func() []Profile {
			return profiles
		},
		NewClient: func(*config.ResolvedConfig) (llm.LLMClient, error) { return client, nil },
		Registry:  tools.NewRegistry(),
	}

	profiles = []Profile{{Name: "explorer", Description: "Explore", Instructions: "Fresh prompt."}}
	result, err := runner.Run(context.Background(), "explorer", "Inspect files")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result.Status != "completed" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if got := client.messages[0].Content; !strings.Contains(got, "Fresh prompt.") {
		t.Fatalf("expected prompt from live profiles, got %s", got)
	}
}

func TestRunnerReturnsErrorsForInvalidInputs(t *testing.T) {
	runner := &Runner{
		Profiles: []Profile{{Name: "explorer", Description: "Explore", Instructions: "Prompt."}},
		Config:   &config.ResolvedConfig{Provider: config.ProviderOpenAI, Model: "model", APIKey: "key"},
	}
	if result, err := runner.Run(context.Background(), "missing", "Task"); err == nil || result.Status != "error" {
		t.Fatalf("expected unknown subagent error, got result=%+v err=%v", result, err)
	}
	if result, err := runner.Run(context.Background(), "explorer", ""); err == nil || result.Status != "error" {
		t.Fatalf("expected missing task error, got result=%+v err=%v", result, err)
	}

	runner.Config = nil
	if result, err := runner.Run(context.Background(), "explorer", "Task"); err == nil || result.Status != "error" {
		t.Fatalf("expected missing config error, got result=%+v err=%v", result, err)
	}
}

func TestRunnerAppliesProfileTimeout(t *testing.T) {
	newRunner := func(client *recordingClient, profileTimeout int) *Runner {
		return &Runner{
			WorkingDir: "/repo",
			Config:     &config.ResolvedConfig{Provider: config.ProviderOpenAI, Model: "model", APIKey: "key"},
			Profiles:   []Profile{{Name: "explorer", Description: "Explore", Instructions: "Prompt.", TimeoutSeconds: profileTimeout}},
			NewClient:  func(*config.ResolvedConfig) (llm.LLMClient, error) { return client, nil },
			Registry:   tools.NewRegistry(),
		}
	}

	client := &recordingClient{}
	if _, err := newRunner(client, 30).Run(context.Background(), "explorer", "Task"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if client.timeout <= 29*time.Second || client.timeout > 30*time.Second {
		t.Fatalf("expected profile timeout near 30s, got %v", client.timeout)
	}

	client = &recordingClient{}
	if _, err := newRunner(client, 0).Run(context.Background(), "explorer", "Task"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	want := time.Duration(defaultTimeoutSeconds) * time.Second
	if client.timeout <= want-time.Second || client.timeout > want {
		t.Fatalf("expected default timeout near %v, got %v", want, client.timeout)
	}
}

func TestCollectResultReturnsPartialTextOnError(t *testing.T) {
	events := make(chan llm.StreamEvent, 2)
	events <- llm.StreamEvent{Type: llm.StreamEventTypeChunk, Content: "partial"}
	events <- llm.StreamEvent{Type: llm.StreamEventTypeIncomplete}
	close(events)

	text, err := collectResult(context.Background(), events)
	if err == nil {
		t.Fatal("expected incomplete stream error")
	}
	if text != "partial" {
		t.Fatalf("expected partial text, got %q", text)
	}
}

func containsAll(text string, parts []string) bool {
	for _, part := range parts {
		if !strings.Contains(text, part) {
			return false
		}
	}
	return true
}
