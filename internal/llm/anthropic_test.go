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

func makeMessageStartEvent(inputTokens, outputTokens, cacheCreationInputTokens, cacheReadInputTokens int64) anthropic.MessageStreamEventUnion {
	raw := json.RawMessage(`{"type":"message_start","message":{"usage":{"input_tokens":` +
		mustMarshalJSON(inputTokens) + `,"output_tokens":` +
		mustMarshalJSON(outputTokens) + `,"cache_creation_input_tokens":` +
		mustMarshalJSON(cacheCreationInputTokens) + `,"cache_read_input_tokens":` +
		mustMarshalJSON(cacheReadInputTokens) + `}}}`)
	var ev anthropic.MessageStreamEventUnion
	_ = json.Unmarshal(raw, &ev)
	return ev
}

func makeMessageDeltaUsageEvent(inputTokens, outputTokens, cacheCreationInputTokens, cacheReadInputTokens int64) anthropic.MessageStreamEventUnion {
	raw := json.RawMessage(`{"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"input_tokens":` +
		mustMarshalJSON(inputTokens) + `,"output_tokens":` +
		mustMarshalJSON(outputTokens) + `,"cache_creation_input_tokens":` +
		mustMarshalJSON(cacheCreationInputTokens) + `,"cache_read_input_tokens":` +
		mustMarshalJSON(cacheReadInputTokens) + `}}`)
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

func TestAnthropicClient_StreamChat_UsesMessageDeltaInputTokensWhenMessageStartIsZero(t *testing.T) {
	events := []anthropic.MessageStreamEventUnion{
		makeMessageStartEvent(0, 0, 0, 0),
		makeMessageDeltaUsageEvent(3546, 15, 0, 0),
		makeTextDeltaEvent(0, "ok"),
		makeContentBlockStopEvent(0),
	}

	client := newTestAnthropicClient(events)
	eventCh, err := client.StreamChat(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var usage *TokenUsage
	for event := range eventCh {
		switch event.Type {
		case StreamEventTypeUsage:
			usage = event.Usage
		case StreamEventTypeError:
			t.Fatalf("unexpected error: %v", event.Error)
		}
	}

	if usage == nil {
		t.Fatal("expected usage event")
	}
	if usage.InputTokens != 3546 {
		t.Fatalf("expected input tokens 3546, got %d", usage.InputTokens)
	}
	if usage.OutputTokens != 15 {
		t.Fatalf("expected output tokens 15, got %d", usage.OutputTokens)
	}
}

func TestAnthropicClient_StreamChat_IncludesCacheTokensInInputFootprint(t *testing.T) {
	events := []anthropic.MessageStreamEventUnion{
		makeMessageStartEvent(0, 0, 0, 0),
		makeMessageDeltaUsageEvent(3000, 20, 400, 100),
		makeTextDeltaEvent(0, "ok"),
		makeContentBlockStopEvent(0),
	}

	client := newTestAnthropicClient(events)
	eventCh, err := client.StreamChat(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var usage *TokenUsage
	for event := range eventCh {
		switch event.Type {
		case StreamEventTypeUsage:
			usage = event.Usage
		case StreamEventTypeError:
			t.Fatalf("unexpected error: %v", event.Error)
		}
	}

	if usage == nil {
		t.Fatal("expected usage event")
	}
	if usage.InputTokens != 3500 {
		t.Fatalf("expected input footprint 3500, got %d", usage.InputTokens)
	}
	if usage.CachedTokens != 500 {
		t.Fatalf("expected cached token breakdown 500, got %d", usage.CachedTokens)
	}
	if usage.TotalTokens != 3520 {
		t.Fatalf("expected total tokens 3520, got %d", usage.TotalTokens)
	}
}

func TestAnthropicClient_StreamChat_StreamError(t *testing.T) {
	expectedErr := errors.New("network failure")
	c := &AnthropicClient{model: "claude-3-haiku-20240307", maxRetries: 1}
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

func TestAnthropicClient_StreamChat_RetriesOnRetryableError(t *testing.T) {
	const testMaxRetries = 2
	expectedErr := errors.New("network failure")
	callCount := 0
	c := &AnthropicClient{model: "claude-3-haiku-20240307", maxRetries: testMaxRetries}
	c.streamImpl = func(ctx context.Context, params anthropic.MessageNewParams) anthropicStream {
		callCount++
		return &mockAnthropicStream{err: expectedErr}
	}

	eventCh, err := c.StreamChat(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var retryEvents []StreamEvent
	var receivedErr error
	for event := range eventCh {
		switch event.Type {
		case StreamEventTypeRetry:
			retryEvents = append(retryEvents, event)
		case StreamEventTypeError:
			receivedErr = event.Error
		}
	}

	if callCount != testMaxRetries {
		t.Fatalf("expected %d stream calls, got %d", testMaxRetries, callCount)
	}
	if len(retryEvents) != testMaxRetries-1 {
		t.Fatalf("expected %d retry events, got %d", testMaxRetries-1, len(retryEvents))
	}
	if retryEvents[0].Attempt != 1 {
		t.Fatalf("expected retry attempt 1, got %d", retryEvents[0].Attempt)
	}
	if receivedErr == nil {
		t.Fatal("expected final error event")
	}
}

func TestAnthropicClient_StreamChat_ToolInvocation(t *testing.T) {
	callCount := 0
	var seenParams []anthropic.MessageNewParams

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
		seenParams = append(seenParams, params)
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
	if len(seenParams) != 2 {
		t.Fatalf("expected 2 captured params, got %d", len(seenParams))
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

func TestAnthropicThinkingParams_OffByDefault(t *testing.T) {
	thinking, outCfg, maxTok := anthropicThinkingParams("")
	if thinking.OfDisabled == nil {
		t.Error("expected OfDisabled to be set when effort is empty")
	}
	if thinking.OfAdaptive != nil {
		t.Error("expected OfAdaptive to be nil when effort is empty")
	}
	if outCfg.Effort != "" {
		t.Errorf("expected empty effort in OutputConfig, got %q", outCfg.Effort)
	}
	if maxTok != anthropicMaxTokens {
		t.Errorf("expected anthropicMaxTokens %d, got %d", anthropicMaxTokens, maxTok)
	}
}

func TestAnthropicThinkingParams_EnabledEfforts(t *testing.T) {
	for _, effort := range []string{"low", "medium", "high", "max"} {
		thinking, outCfg, maxTok := anthropicThinkingParams(effort)
		if thinking.OfAdaptive == nil {
			t.Errorf("effort %q: expected OfAdaptive to be set", effort)
		}
		if thinking.OfDisabled != nil {
			t.Errorf("effort %q: expected OfDisabled to be nil", effort)
		}
		if string(outCfg.Effort) != effort {
			t.Errorf("effort %q: expected OutputConfig.Effort %q, got %q", effort, effort, outCfg.Effort)
		}
		if maxTok != anthropicMaxTokens {
			t.Errorf("effort %q: expected maxTok %d, got %d", effort, anthropicMaxTokens, maxTok)
		}
	}
}

func TestAnthropicClient_ThinkingEffort_UsedInParams(t *testing.T) {
	var capturedParams anthropic.MessageNewParams

	c := &AnthropicClient{
		model:          "claude-sonnet-4-6",
		thinkingEffort: "high",
	}
	c.streamImpl = func(ctx context.Context, params anthropic.MessageNewParams) anthropicStream {
		capturedParams = params
		return &mockAnthropicStream{events: []anthropic.MessageStreamEventUnion{
			makeTextDeltaEvent(0, "ok"),
			makeContentBlockStopEvent(0),
		}}
	}

	eventCh, err := c.StreamChat(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for range eventCh {
	}

	if capturedParams.Thinking.OfAdaptive == nil {
		t.Error("expected OfAdaptive to be set for effort 'high'")
	}
	if string(capturedParams.OutputConfig.Effort) != "high" {
		t.Errorf("expected OutputConfig.Effort 'high', got %q", capturedParams.OutputConfig.Effort)
	}
	if capturedParams.MaxTokens != anthropicMaxTokens {
		t.Errorf("expected MaxTokens %d, got %d", anthropicMaxTokens, capturedParams.MaxTokens)
	}
}

func TestAnthropicClient_NoThinkingEffort_DisablesThinking(t *testing.T) {
	var capturedParams anthropic.MessageNewParams

	c := &AnthropicClient{
		model:          "claude-sonnet-4-6",
		thinkingEffort: "",
	}
	c.streamImpl = func(ctx context.Context, params anthropic.MessageNewParams) anthropicStream {
		capturedParams = params
		return &mockAnthropicStream{events: []anthropic.MessageStreamEventUnion{
			makeTextDeltaEvent(0, "ok"),
			makeContentBlockStopEvent(0),
		}}
	}

	eventCh, err := c.StreamChat(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for range eventCh {
	}

	if capturedParams.Thinking.OfDisabled == nil {
		t.Error("expected OfDisabled to be set when thinkingEffort is empty")
	}
	if capturedParams.MaxTokens != anthropicMaxTokens {
		t.Errorf("expected anthropicMaxTokens %d, got %d", anthropicMaxTokens, capturedParams.MaxTokens)
	}
}

func TestAnthropicClient_PendingState_ErrorMidLoop(t *testing.T) {
	callCount := 0
	firstEvents := []anthropic.MessageStreamEventUnion{
		makeToolUseStartEvent(0, "toolu_01", "success_tool"),
		makeInputJSONDeltaEvent(0, `{"message":"hi"}`),
		makeContentBlockStopEvent(0),
	}

	c := &AnthropicClient{model: "claude-3-haiku-20240307", maxRetries: 1}
	c.streamImpl = func(ctx context.Context, params anthropic.MessageNewParams) anthropicStream {
		callCount++
		if callCount == 1 {
			return &mockAnthropicStream{events: firstEvents}
		}
		return &mockAnthropicStream{err: errors.New("API error")}
	}

	registry := tools.NewRegistry()
	if err := registry.Register(&successTool{}); err != nil {
		t.Fatalf("register: %v", err)
	}

	ch, err := c.StreamChat(context.Background(), []Message{{Role: RoleUser, Content: "go"}}, registry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var hasIncomplete bool
	var incompleteErr error
	for ev := range ch {
		switch ev.Type {
		case StreamEventTypeIncomplete:
			hasIncomplete = true
			incompleteErr = ev.Error
		case StreamEventTypeError:
			t.Fatalf("expected incomplete, got error: %v", ev.Error)
		}
	}

	if !hasIncomplete {
		t.Fatal("expected incomplete event")
	}
	if incompleteErr == nil {
		t.Fatal("expected error on incomplete event")
	}
	if len(c.pendingState) == 0 {
		t.Fatal("expected pending state to be saved")
	}
	// assistant message (tool use) + user message (tool result)
	if len(c.pendingState) != 2 {
		t.Fatalf("expected 2 pending messages, got %d", len(c.pendingState))
	}
}

func TestAnthropicClient_PendingState_InjectedOnNextCall(t *testing.T) {
	callCount := 0
	var capturedParams []anthropic.MessageNewParams

	firstEvents := []anthropic.MessageStreamEventUnion{
		makeToolUseStartEvent(0, "toolu_01", "success_tool"),
		makeInputJSONDeltaEvent(0, `{"message":"hi"}`),
		makeContentBlockStopEvent(0),
	}
	recoveryEvents := []anthropic.MessageStreamEventUnion{
		makeTextDeltaEvent(0, "recovered"),
		makeContentBlockStopEvent(0),
	}

	c := &AnthropicClient{model: "claude-3-haiku-20240307", maxRetries: 1}
	c.streamImpl = func(ctx context.Context, params anthropic.MessageNewParams) anthropicStream {
		callCount++
		capturedParams = append(capturedParams, params)
		switch callCount {
		case 1:
			return &mockAnthropicStream{events: firstEvents}
		case 2:
			return &mockAnthropicStream{err: errors.New("API error")}
		default:
			return &mockAnthropicStream{events: recoveryEvents}
		}
	}

	registry := tools.NewRegistry()
	if err := registry.Register(&successTool{}); err != nil {
		t.Fatalf("register: %v", err)
	}

	ch, err := c.StreamChat(context.Background(), []Message{{Role: RoleUser, Content: "go"}}, registry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for range ch {
	}

	if len(c.pendingState) == 0 {
		t.Fatal("expected pending state after failed turn")
	}
	savedLen := len(c.pendingState)

	ch, err = c.StreamChat(context.Background(), []Message{
		{Role: RoleUser, Content: "go"},
		{Role: RoleAssistant, Content: "working on it"},
		{Role: RoleUser, Content: "continue"},
	}, registry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var hasDone bool
	var streamed []string
	for ev := range ch {
		switch ev.Type {
		case StreamEventTypeDone:
			hasDone = true
		case StreamEventTypeChunk:
			streamed = append(streamed, ev.Content)
		case StreamEventTypeError:
			t.Fatalf("unexpected stream error: %v", ev.Error)
		}
	}

	if !hasDone {
		t.Fatal("expected done event")
	}
	if len(streamed) == 0 || streamed[0] != "recovered" {
		t.Fatalf("expected recovered chunk, got %v", streamed)
	}
	if len(c.pendingState) != 0 {
		t.Fatal("expected pending state to be cleared after injection")
	}

	// Recovery call: 3 original messages → inject savedLen before last → 2 + savedLen + 1
	recoveryParams := capturedParams[len(capturedParams)-1]
	if len(recoveryParams.Messages) != 2+savedLen+1 {
		t.Fatalf("expected %d messages in recovery call, got %d", 2+savedLen+1, len(recoveryParams.Messages))
	}
}

func TestAnthropicClient_PendingState_PreservedWhenRecoveryFailsBeforeProgress(t *testing.T) {
	c := &AnthropicClient{
		model:      "claude-3-haiku-20240307",
		maxRetries: 1,
		pendingState: []anthropic.MessageParam{
			anthropic.NewAssistantMessage(anthropic.NewTextBlock("prior tool use")),
			anthropic.NewUserMessage(anthropic.NewTextBlock("prior tool result")),
		},
	}

	wantLen := len(c.pendingState)
	c.streamImpl = func(ctx context.Context, params anthropic.MessageNewParams) anthropicStream {
		return &mockAnthropicStream{err: errors.New("API error")}
	}

	ch, err := c.StreamChat(context.Background(), []Message{
		{Role: RoleUser, Content: "go"},
		{Role: RoleUser, Content: "continue"},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var hasIncomplete bool
	for ev := range ch {
		switch ev.Type {
		case StreamEventTypeIncomplete:
			hasIncomplete = true
		case StreamEventTypeError:
			t.Fatalf("expected incomplete, got error: %v", ev.Error)
		}
	}

	if !hasIncomplete {
		t.Fatal("expected incomplete event")
	}
	if len(c.pendingState) != wantLen {
		t.Fatalf("expected pending state length %d preserved, got %d", wantLen, len(c.pendingState))
	}
}

func TestAnthropicClient_PendingState_NoAccumulation_EmitsError(t *testing.T) {
	c := &AnthropicClient{model: "claude-3-haiku-20240307", maxRetries: 1}
	c.streamImpl = func(ctx context.Context, params anthropic.MessageNewParams) anthropicStream {
		return &mockAnthropicStream{err: errors.New("API error")}
	}

	ch, err := c.StreamChat(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var hasError bool
	for ev := range ch {
		switch ev.Type {
		case StreamEventTypeError:
			hasError = true
		case StreamEventTypeIncomplete:
			t.Fatal("expected error, not incomplete")
		}
	}

	if !hasError {
		t.Fatal("expected error event")
	}
	if len(c.pendingState) != 0 {
		t.Fatal("expected no pending state when nothing accumulated")
	}
}

func TestAnthropicClient_PendingState_ClearedOnSuccess(t *testing.T) {
	c := &AnthropicClient{
		model: "claude-3-haiku-20240307",
		pendingState: []anthropic.MessageParam{
			anthropic.NewAssistantMessage(anthropic.NewTextBlock("prior tool use")),
			anthropic.NewUserMessage(anthropic.NewTextBlock("prior tool result")),
		},
	}

	successEvents := []anthropic.MessageStreamEventUnion{
		makeMessageStartEvent(100, 20, 0, 0),
		makeTextDeltaEvent(0, "done"),
		makeContentBlockStopEvent(0),
	}
	c.streamImpl = func(ctx context.Context, params anthropic.MessageNewParams) anthropicStream {
		return &mockAnthropicStream{events: successEvents}
	}

	ch, err := c.StreamChat(context.Background(), []Message{
		{Role: RoleUser, Content: "original"},
		{Role: RoleUser, Content: "continue"},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var hasDone bool
	for ev := range ch {
		if ev.Type == StreamEventTypeDone {
			hasDone = true
		}
	}

	if !hasDone {
		t.Fatal("expected done event")
	}
	if len(c.pendingState) != 0 {
		t.Fatal("expected pending state to be cleared after successful completion")
	}
}
