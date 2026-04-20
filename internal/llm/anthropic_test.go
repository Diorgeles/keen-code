package llm

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/packages/ssestream"
	"github.com/user/keen-code/internal/tools"
)

// mockAnthropicStream implements anthropicStream for testing.
type mockAnthropicStream struct {
	events []anthropic.MessageStreamEventUnion
	pos    int
	err    error
}

func (m *mockAnthropicStream) Next() bool {
	if m.err != nil {
		return false
	}
	m.pos++
	return m.pos <= len(m.events)
}

func (m *mockAnthropicStream) Current() anthropic.MessageStreamEventUnion {
	if m.pos < 1 || m.pos > len(m.events) {
		return anthropic.MessageStreamEventUnion{}
	}
	return m.events[m.pos-1]
}

func (m *mockAnthropicStream) Err() error   { return m.err }
func (m *mockAnthropicStream) Close() error { return nil }

// Satisfy the interface used by ssestream.Stream — only used to verify the
// sdkAnthropicStream wrapper compiles; not exercised in unit tests.
var _ anthropicStream = (*sdkAnthropicStream)(nil)
var _ anthropicStream = (*mockAnthropicStream)(nil)

// Verify sdkAnthropicStream wraps ssestream.Stream correctly (compile check).
var _ *ssestream.Stream[anthropic.MessageStreamEventUnion] = (*ssestream.Stream[anthropic.MessageStreamEventUnion])(nil)

func makeTextDeltaEvent(index int64, text string) anthropic.MessageStreamEventUnion {
	raw := json.RawMessage(`{"type":"content_block_delta","index":` +
		string(rune('0'+index)) + `,"delta":{"type":"text_delta","text":` +
		mustMarshalJSON(text) + `}}`)
	var ev anthropic.MessageStreamEventUnion
	_ = json.Unmarshal(raw, &ev)
	return ev
}

func makeThinkingDeltaEvent(index int64, thinking string) anthropic.MessageStreamEventUnion {
	raw := json.RawMessage(`{"type":"content_block_delta","index":` +
		string(rune('0'+index)) + `,"delta":{"type":"thinking_delta","thinking":` +
		mustMarshalJSON(thinking) + `}}`)
	var ev anthropic.MessageStreamEventUnion
	_ = json.Unmarshal(raw, &ev)
	return ev
}

func makeContentBlockStopEvent(index int64) anthropic.MessageStreamEventUnion {
	raw := json.RawMessage(`{"type":"content_block_stop","index":` +
		string(rune('0'+index)) + `}`)
	var ev anthropic.MessageStreamEventUnion
	_ = json.Unmarshal(raw, &ev)
	return ev
}

func makeToolUseStartEvent(index int64, id, name string) anthropic.MessageStreamEventUnion {
	raw := json.RawMessage(`{"type":"content_block_start","index":` +
		string(rune('0'+index)) +
		`,"content_block":{"type":"tool_use","id":` + mustMarshalJSON(id) +
		`,"name":` + mustMarshalJSON(name) + `,"input":{}}}`)
	var ev anthropic.MessageStreamEventUnion
	_ = json.Unmarshal(raw, &ev)
	return ev
}

func makeInputJSONDeltaEvent(index int64, partial string) anthropic.MessageStreamEventUnion {
	raw := json.RawMessage(`{"type":"content_block_delta","index":` +
		string(rune('0'+index)) + `,"delta":{"type":"input_json_delta","partial_json":` +
		mustMarshalJSON(partial) + `}}`)
	var ev anthropic.MessageStreamEventUnion
	_ = json.Unmarshal(raw, &ev)
	return ev
}

func mustMarshalJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func newTestAnthropicClient(events []anthropic.MessageStreamEventUnion) *AnthropicClient {
	c := &AnthropicClient{model: "claude-3-haiku-20240307"}
	c.streamImpl = func(ctx context.Context, params anthropic.MessageNewParams) anthropicStream {
		return &mockAnthropicStream{events: events}
	}
	return c
}

func TestAnthropicClient_StreamChat_TextChunks(t *testing.T) {
	events := []anthropic.MessageStreamEventUnion{
		makeTextDeltaEvent(0, "Hello"),
		makeTextDeltaEvent(0, " world"),
		makeTextDeltaEvent(0, "!"),
		makeContentBlockStopEvent(0),
	}

	client := newTestAnthropicClient(events)
	messages := []Message{{Role: RoleUser, Content: "Hi"}}

	eventCh, err := client.StreamChat(context.Background(), messages, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var chunks []string
	var doneReceived bool
	for event := range eventCh {
		switch event.Type {
		case StreamEventTypeChunk:
			chunks = append(chunks, event.Content)
		case StreamEventTypeDone:
			doneReceived = true
		case StreamEventTypeError:
			t.Fatalf("unexpected error: %v", event.Error)
		}
	}

	if !doneReceived {
		t.Error("expected done event")
	}
	if len(chunks) != 3 {
		t.Errorf("expected 3 chunks, got %d", len(chunks))
	}
	expected := []string{"Hello", " world", "!"}
	for i, exp := range expected {
		if i >= len(chunks) {
			break
		}
		if chunks[i] != exp {
			t.Errorf("chunk %d: expected %q, got %q", i, exp, chunks[i])
		}
	}
}

func TestAnthropicClient_StreamChat_ReasoningChunks(t *testing.T) {
	events := []anthropic.MessageStreamEventUnion{
		makeThinkingDeltaEvent(0, "step 1"),
		makeThinkingDeltaEvent(0, "step 2"),
		makeContentBlockStopEvent(0),
		makeTextDeltaEvent(1, "answer"),
		makeContentBlockStopEvent(1),
	}

	client := newTestAnthropicClient(events)
	messages := []Message{{Role: RoleUser, Content: "Think"}}

	eventCh, err := client.StreamChat(context.Background(), messages, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var reasoning []string
	var text []string
	for event := range eventCh {
		switch event.Type {
		case StreamEventTypeReasoningChunk:
			reasoning = append(reasoning, event.Content)
		case StreamEventTypeChunk:
			text = append(text, event.Content)
		case StreamEventTypeError:
			t.Fatalf("unexpected error: %v", event.Error)
		}
	}

	if len(reasoning) != 2 {
		t.Errorf("expected 2 reasoning chunks, got %d", len(reasoning))
	}
	if len(text) != 1 || text[0] != "answer" {
		t.Errorf("expected 1 text chunk 'answer', got %v", text)
	}
}

func TestAnthropicClient_StreamChat_StreamError(t *testing.T) {
	expectedErr := errors.New("network failure")
	c := &AnthropicClient{model: "claude-3-haiku-20240307"}
	c.streamImpl = func(ctx context.Context, params anthropic.MessageNewParams) anthropicStream {
		return &mockAnthropicStream{err: expectedErr}
	}

	eventCh, err := c.StreamChat(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var receivedErr error
	for event := range eventCh {
		if event.Type == StreamEventTypeError {
			receivedErr = event.Error
		}
	}

	if receivedErr == nil {
		t.Fatal("expected error event")
	}
}

func TestAnthropicClient_StreamChat_ToolInvocation(t *testing.T) {
	callCount := 0

	firstEvents := []anthropic.MessageStreamEventUnion{
		makeTextDeltaEvent(0, "using tool"),
		makeContentBlockStopEvent(0),
		makeToolUseStartEvent(1, "toolu_01", "success_tool"),
		makeInputJSONDeltaEvent(1, `{"message"`),
		makeInputJSONDeltaEvent(1, `:"hello"}`),
		makeContentBlockStopEvent(1),
	}
	secondEvents := []anthropic.MessageStreamEventUnion{
		makeTextDeltaEvent(0, "done"),
		makeContentBlockStopEvent(0),
	}

	c := &AnthropicClient{model: "claude-3-haiku-20240307"}
	c.streamImpl = func(ctx context.Context, params anthropic.MessageNewParams) anthropicStream {
		callCount++
		if callCount == 1 {
			return &mockAnthropicStream{events: firstEvents}
		}
		return &mockAnthropicStream{events: secondEvents}
	}

	registry := tools.NewRegistry()
	if err := registry.Register(&successTool{}); err != nil {
		t.Fatalf("register: %v", err)
	}

	eventCh, err := c.StreamChat(context.Background(), []Message{{Role: RoleUser, Content: "go"}}, registry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var toolStartReceived, toolEndReceived, doneReceived bool
	var textChunks []string
	for event := range eventCh {
		switch event.Type {
		case StreamEventTypeChunk:
			textChunks = append(textChunks, event.Content)
		case StreamEventTypeToolStart:
			toolStartReceived = true
			if event.ToolCall.Name != "success_tool" {
				t.Errorf("expected tool name success_tool, got %q", event.ToolCall.Name)
			}
		case StreamEventTypeToolEnd:
			toolEndReceived = true
			if event.ToolCall.Output == nil {
				t.Error("expected tool output in tool_end event")
			}
		case StreamEventTypeDone:
			doneReceived = true
		case StreamEventTypeError:
			t.Fatalf("unexpected error: %v", event.Error)
		}
	}

	if !toolStartReceived {
		t.Error("expected tool_start event")
	}
	if !toolEndReceived {
		t.Error("expected tool_end event")
	}
	if !doneReceived {
		t.Error("expected done event")
	}
	if callCount != 2 {
		t.Errorf("expected 2 stream calls, got %d", callCount)
	}
	// "using tool" from first turn, "done" from second
	if len(textChunks) != 2 {
		t.Errorf("expected 2 text chunks, got %d: %v", len(textChunks), textChunks)
	}
}

func TestAnthropicClient_executeTools_Success(t *testing.T) {
	c := &AnthropicClient{}

	registry := tools.NewRegistry()
	if err := registry.Register(&successTool{}); err != nil {
		t.Fatalf("register: %v", err)
	}

	uses := []toolUseEntry{{id: "id1", name: "success_tool", input: map[string]any{"message": "hi"}}}
	eventCh := make(chan StreamEvent, 4)
	blocks := c.executeTools(context.Background(), uses, registry, eventCh)

	if len(blocks) != 1 {
		t.Fatalf("expected 1 result block, got %d", len(blocks))
	}

	start := <-eventCh
	if start.Type != StreamEventTypeToolStart {
		t.Fatalf("expected tool_start, got %q", start.Type)
	}
	end := <-eventCh
	if end.Type != StreamEventTypeToolEnd {
		t.Fatalf("expected tool_end, got %q", end.Type)
	}
	if end.ToolCall.Error != "" {
		t.Fatalf("unexpected error: %s", end.ToolCall.Error)
	}
}

func TestAnthropicClient_executeTools_Error(t *testing.T) {
	c := &AnthropicClient{}

	registry := tools.NewRegistry()
	if err := registry.Register(&failingTool{}); err != nil {
		t.Fatalf("register: %v", err)
	}

	uses := []toolUseEntry{{id: "id2", name: "failing_tool", input: map[string]any{}}}
	eventCh := make(chan StreamEvent, 4)
	blocks := c.executeTools(context.Background(), uses, registry, eventCh)

	if len(blocks) != 1 {
		t.Fatalf("expected 1 result block, got %d", len(blocks))
	}

	<-eventCh // tool_start
	end := <-eventCh
	if end.Type != StreamEventTypeToolEnd {
		t.Fatalf("expected tool_end, got %q", end.Type)
	}
	if end.ToolCall.Error != "tool failed" {
		t.Fatalf("expected error 'tool failed', got %q", end.ToolCall.Error)
	}
}

func TestToAnthropicMessages_SystemAndConversation(t *testing.T) {
	messages := []Message{
		{Role: RoleSystem, Content: "be helpful"},
		{Role: RoleUser, Content: "hello"},
		{Role: RoleAssistant, Content: "hi there"},
	}

	systemBlocks, msgParams := toAnthropicMessages(messages)

	if len(systemBlocks) != 1 {
		t.Fatalf("expected 1 system block, got %d", len(systemBlocks))
	}
	if systemBlocks[0].Text != "be helpful" {
		t.Errorf("unexpected system text: %q", systemBlocks[0].Text)
	}
	if len(msgParams) != 2 {
		t.Fatalf("expected 2 message params, got %d", len(msgParams))
	}
}

func TestToAnthropicMessages_TurnMemoryRendered(t *testing.T) {
	messages := []Message{
		{
			Role:    RoleAssistant,
			Content: "done",
			TurnMemory: &TurnMemory{
				FilesChanged: []string{"main.go"},
			},
		},
	}

	_, msgParams := toAnthropicMessages(messages)

	if len(msgParams) != 1 {
		t.Fatalf("expected 1 message param, got %d", len(msgParams))
	}
}

func TestToAnthropicTools(t *testing.T) {
	registry := tools.NewRegistry()
	if err := registry.Register(&successTool{}); err != nil {
		t.Fatalf("register: %v", err)
	}

	result := toAnthropicTools(registry)

	if len(result) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(result))
	}
	if result[0].OfTool == nil {
		t.Fatal("expected OfTool to be set")
	}
	if result[0].OfTool.Name != "success_tool" {
		t.Errorf("expected tool name 'success_tool', got %q", result[0].OfTool.Name)
	}
}
