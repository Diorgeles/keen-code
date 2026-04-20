package appstate

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/user/keen-code/internal/config"
	"github.com/user/keen-code/internal/llm"
	"github.com/user/keen-code/internal/tools"
)

type mockLLMClient struct {
	streamChatFunc func(ctx context.Context, messages []llm.Message, toolRegistry *tools.Registry) (<-chan llm.StreamEvent, error)
}

func (m *mockLLMClient) StreamChat(ctx context.Context, messages []llm.Message, toolRegistry *tools.Registry) (<-chan llm.StreamEvent, error) {
	if m.streamChatFunc != nil {
		return m.streamChatFunc(ctx, messages, toolRegistry)
	}
	ch := make(chan llm.StreamEvent)
	close(ch)
	return ch, nil
}

func TestNewAppState(t *testing.T) {
	client := &mockLLMClient{}
	state := New(client, t.TempDir())

	if state == nil {
		t.Fatal("expected non-nil AppState")
	}
	if state.llmClient != client {
		t.Error("expected llmClient to be set")
	}
	if len(state.messages) != 0 {
		t.Errorf("expected empty messages, got %d", len(state.messages))
	}
}

func TestNewAppState_NilClient(t *testing.T) {
	state := New(nil, t.TempDir())

	if state == nil {
		t.Fatal("expected non-nil AppState")
	}
	if state.llmClient != nil {
		t.Error("expected nil llmClient")
	}
}

func TestAppState_AddMessage(t *testing.T) {
	state := New(nil, t.TempDir())

	state.AddMessage(llm.RoleUser, "Hello")
	if len(state.messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(state.messages))
	}
	if state.messages[0].Role != llm.RoleUser {
		t.Errorf("expected role %s, got %s", llm.RoleUser, state.messages[0].Role)
	}
	if state.messages[0].Content != "Hello" {
		t.Errorf("expected content %q, got %q", "Hello", state.messages[0].Content)
	}

	state.AddMessage(llm.RoleAssistant, "Hi there")
	if len(state.messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(state.messages))
	}
	if state.messages[1].Role != llm.RoleAssistant {
		t.Errorf("expected role %s, got %s", llm.RoleAssistant, state.messages[1].Role)
	}
}

func TestAppState_GetMessages(t *testing.T) {
	state := New(nil, t.TempDir())

	messages := state.GetMessages()
	if len(messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(messages))
	}

	state.AddMessage(llm.RoleUser, "Test")
	messages = state.GetMessages()
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if messages[0].Content != "Test" {
		t.Errorf("expected content %q, got %q", "Test", messages[0].Content)
	}
}

func TestAppState_GetMessages_ReturnsCopy(t *testing.T) {
	state := New(nil, t.TempDir())
	state.AppendMessage(llm.Message{
		Role:    llm.RoleAssistant,
		Content: "Original",
		TurnMemory: &llm.TurnMemory{
			FilesChanged: []string{"a.go"},
		},
	})

	messages := state.GetMessages()
	messages[0].Content = "Modified"
	messages[0].TurnMemory.FilesChanged[0] = "b.go"

	original := state.GetMessages()
	if original[0].Content != "Original" {
		t.Error("GetMessages should return a copy, but original was modified")
	}
	if original[0].TurnMemory.FilesChanged[0] != "a.go" {
		t.Error("GetMessages should deep-clone turn memory, but original was modified")
	}
}

func TestAppState_ClearMessages(t *testing.T) {
	state := New(nil, t.TempDir())

	state.AddMessage(llm.RoleUser, "Hello")
	state.AddMessage(llm.RoleAssistant, "Hi")
	if len(state.messages) != 2 {
		t.Fatalf("expected 2 messages before clear, got %d", len(state.messages))
	}

	state.ClearMessages()
	if len(state.messages) != 0 {
		t.Errorf("expected 0 messages after clear, got %d", len(state.messages))
	}
}

func TestAppState_ClearMessages_EmptyState(t *testing.T) {
	state := New(nil, t.TempDir())

	state.ClearMessages()
	if len(state.messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(state.messages))
	}
}

func TestAppState_StreamChat_WithClient(t *testing.T) {
	expectedEvents := []llm.StreamEvent{
		{Type: llm.StreamEventTypeChunk, Content: "Hello"},
		{Type: llm.StreamEventTypeDone},
	}

	client := &mockLLMClient{
		streamChatFunc: func(ctx context.Context, messages []llm.Message, toolRegistry *tools.Registry) (<-chan llm.StreamEvent, error) {
			ch := make(chan llm.StreamEvent)
			go func() {
				defer close(ch)
				for _, e := range expectedEvents {
					ch <- e
				}
			}()
			return ch, nil
		},
	}

	state := New(client, t.TempDir())
	state.AddMessage(llm.RoleUser, "Hi")

	cfg := &config.ResolvedConfig{APIKey: "key", Model: "model"}
	eventCh, err := state.StreamChat(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var received []llm.StreamEvent
	for e := range eventCh {
		received = append(received, e)
	}

	if len(received) != len(expectedEvents) {
		t.Errorf("expected %d events, got %d", len(expectedEvents), len(received))
	}
}

func TestAppState_StreamChat_NilClient(t *testing.T) {
	state := New(nil, t.TempDir())
	state.AddMessage(llm.RoleUser, "Hi")

	cfg := &config.ResolvedConfig{APIKey: "key", Model: "model"}
	eventCh, err := state.StreamChat(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if eventCh != nil {
		t.Error("expected nil event channel when client is nil")
	}
}

func TestAppState_StreamChat_ClientError(t *testing.T) {
	expectedErr := errors.New("stream error")
	client := &mockLLMClient{
		streamChatFunc: func(ctx context.Context, messages []llm.Message, toolRegistry *tools.Registry) (<-chan llm.StreamEvent, error) {
			return nil, expectedErr
		},
	}

	state := New(client, t.TempDir())
	cfg := &config.ResolvedConfig{APIKey: "key", Model: "model"}

	_, err := state.StreamChat(context.Background(), cfg)
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

func TestAppState_IsClientReady(t *testing.T) {
	client := &mockLLMClient{}

	tests := []struct {
		name     string
		client   llm.LLMClient
		cfg      *config.ResolvedConfig
		expected bool
	}{
		{
			name:     "ready with all fields",
			client:   client,
			cfg:      &config.ResolvedConfig{APIKey: "key", Model: "model"},
			expected: true,
		},
		{
			name:     "not ready with nil client",
			client:   nil,
			cfg:      &config.ResolvedConfig{APIKey: "key", Model: "model"},
			expected: false,
		},
		{
			name:     "not ready with empty API key",
			client:   client,
			cfg:      &config.ResolvedConfig{APIKey: "", Model: "model"},
			expected: false,
		},
		{
			name:     "not ready with empty model",
			client:   client,
			cfg:      &config.ResolvedConfig{APIKey: "key", Model: ""},
			expected: false,
		},
		{
			name:     "not ready with all empty",
			client:   nil,
			cfg:      &config.ResolvedConfig{APIKey: "", Model: ""},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := New(tt.client, t.TempDir())
			got := state.IsClientReady(tt.cfg)
			if got != tt.expected {
				t.Errorf("IsClientReady() = %v, expected %v", got, tt.expected)
			}
		})
	}
}

func TestAppState_UpdateClient(t *testing.T) {
	oldClient := &mockLLMClient{}
	state := New(oldClient, t.TempDir())

	if state.llmClient != oldClient {
		t.Error("expected old client to be set initially")
	}

	newClient := &mockLLMClient{}
	state.UpdateClient(newClient)

	if state.llmClient != newClient {
		t.Error("expected new client to be set after update")
	}
}

func TestAppState_UpdateClient_ToNil(t *testing.T) {
	client := &mockLLMClient{}
	state := New(client, t.TempDir())

	state.UpdateClient(nil)

	if state.llmClient != nil {
		t.Error("expected client to be nil after update")
	}
}

func TestAppState_GetClient(t *testing.T) {
	client := &mockLLMClient{}
	state := New(client, t.TempDir())

	got := state.GetClient()
	if got != client {
		t.Error("GetClient() returned unexpected client")
	}
}

func TestAppState_GetClient_Nil(t *testing.T) {
	state := New(nil, t.TempDir())

	got := state.GetClient()
	if got != nil {
		t.Error("GetClient() expected nil, got non-nil")
	}
}

func TestAppState_StreamCompactBuildsCompactionRequest(t *testing.T) {
	var capturedMessages []llm.Message
	var capturedRegistry *tools.Registry

	client := &mockLLMClient{
		streamChatFunc: func(ctx context.Context, messages []llm.Message, toolRegistry *tools.Registry) (<-chan llm.StreamEvent, error) {
			capturedMessages = append([]llm.Message(nil), messages...)
			capturedRegistry = toolRegistry

			ch := make(chan llm.StreamEvent, 2)
			ch <- llm.StreamEvent{Type: llm.StreamEventTypeChunk, Content: "compacted summary"}
			ch <- llm.StreamEvent{Type: llm.StreamEventTypeDone}
			close(ch)
			return ch, nil
		},
	}

	state := New(client, t.TempDir())
	original := make([]llm.Message, 0, 25)
	for i := 0; i < 25; i++ {
		role := llm.RoleUser
		if i%2 == 1 {
			role = llm.RoleAssistant
		}
		msg := llm.Message{Role: role, Content: "message " + strings.Repeat("x", i+1)}
		original = append(original, msg)
		state.AddMessage(role, msg.Content)
	}

	eventCh, err := state.StreamCompact(context.Background(), &config.ResolvedConfig{
		APIKey: "key",
		Model:  "model",
	}, "Keep business logic details")
	if err != nil {
		t.Fatalf("StreamCompact() returned error: %v", err)
	}
	if eventCh == nil {
		t.Fatal("expected compaction stream")
	}

	if capturedRegistry != nil {
		t.Fatal("expected compaction to disable tools")
	}
	if len(capturedMessages) != len(original)+2 {
		t.Fatalf("expected %d compaction request messages, got %d", len(original)+2, len(capturedMessages))
	}
	if capturedMessages[0].Role != llm.RoleSystem {
		t.Fatalf("expected first compaction message to be system, got %s", capturedMessages[0].Role)
	}
	if !strings.Contains(capturedMessages[0].Content, "Keep business logic details") {
		t.Fatalf("expected extra prompt in system prompt, got %q", capturedMessages[0].Content)
	}
	for i, msg := range original {
		got := capturedMessages[i+1]
		if got != msg {
			t.Fatalf("expected compaction request message %d to match original history", i)
		}
	}
	last := capturedMessages[len(capturedMessages)-1]
	if last.Role != llm.RoleUser {
		t.Fatalf("expected final compaction message to be user, got %s", last.Role)
	}
	if last.Content != compactionUserInstruction {
		t.Fatalf("unexpected final compaction instruction: %q", last.Content)
	}
}

func TestAppState_ApplyCompactionReplacesHistoryWithSingleSummaryMessage(t *testing.T) {
	state := New(&mockLLMClient{}, t.TempDir())
	state.AddMessage(llm.RoleUser, "hello")
	state.AddMessage(llm.RoleAssistant, "world")

	if err := state.ApplyCompaction("  compacted summary  "); err != nil {
		t.Fatalf("ApplyCompaction() returned error: %v", err)
	}

	compacted := state.GetMessages()
	if len(compacted) != 1 {
		t.Fatalf("expected compacted history to contain one summary message, got %d", len(compacted))
	}
	if compacted[0].Role != llm.RoleUser || compacted[0].Content != "compacted summary" {
		t.Fatalf("unexpected summary message: %#v", compacted[0])
	}
}

func TestAppState_ApplyCompactionLeavesMessagesUntouchedOnError(t *testing.T) {
	state := New(&mockLLMClient{}, t.TempDir())
	state.AddMessage(llm.RoleUser, "hello")
	state.AddMessage(llm.RoleAssistant, "world")
	original := state.GetMessages()

	err := state.ApplyCompaction(" \n\t ")
	if err == nil || err.Error() != "compaction returned empty summary" {
		t.Fatalf("expected empty summary error, got %v", err)
	}

	if got := state.GetMessages(); len(got) != len(original) || got[0] != original[0] || got[1] != original[1] {
		t.Fatalf("expected messages to remain unchanged, got %#v", got)
	}
}

func TestAppState_StreamCompactLeavesMessagesUntouchedOnCancel(t *testing.T) {
	state := New(&mockLLMClient{}, t.TempDir())
	state.AddMessage(llm.RoleUser, "hello")
	original := state.GetMessages()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	eventCh, err := state.StreamCompact(ctx, &config.ResolvedConfig{
		APIKey: "key",
		Model:  "model",
	}, "")
	if err != nil {
		t.Fatalf("expected nil error from StreamCompact, got %v", err)
	}
	if eventCh == nil {
		t.Fatal("expected compaction stream")
	}

	if got := state.GetMessages(); len(got) != len(original) || got[0] != original[0] {
		t.Fatalf("expected messages to remain unchanged, got %#v", got)
	}
}
