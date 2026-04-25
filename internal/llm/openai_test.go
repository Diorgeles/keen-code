package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/respjson"
	"github.com/user/keen-code/internal/config"
	"github.com/user/keen-code/internal/tools"
)

type fakeChatStream struct {
	chunks []openai.ChatCompletionChunk
	idx    int
	err    error
}

func (s *fakeChatStream) Next() bool {
	if s.idx >= len(s.chunks) {
		return false
	}
	s.idx++
	return true
}

func (s *fakeChatStream) Current() openai.ChatCompletionChunk {
	if s.idx == 0 || s.idx > len(s.chunks) {
		return openai.ChatCompletionChunk{}
	}
	return s.chunks[s.idx-1]
}

func (s *fakeChatStream) Err() error {
	return s.err
}

func (s *fakeChatStream) Close() error { return nil }

type successToolOAI struct{}

func (t *successToolOAI) Name() string {
	return "read_file"
}

func (t *successToolOAI) Description() string {
	return "reads a file"
}

func (t *successToolOAI) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{"type": "string"},
		},
	}
}

func (t *successToolOAI) Execute(ctx context.Context, input any) (any, error) {
	return map[string]any{"content": "module github.com/user/keen-code"}, nil
}

func makeToolCallChunk() openai.ChatCompletionChunk {
	chunk := openai.ChatCompletionChunk{
		Choices: []openai.ChatCompletionChunkChoice{
			{
				Index: 0,
				Delta: openai.ChatCompletionChunkChoiceDelta{
					Role: "assistant",
					ToolCalls: []openai.ChatCompletionChunkChoiceDeltaToolCall{
						{
							Index: 0,
							ID:    "call_1",
							Type:  "function",
							Function: openai.ChatCompletionChunkChoiceDeltaToolCallFunction{
								Name:      "read_file",
								Arguments: `{"path":"go.mod"}`,
							},
						},
					},
				},
				FinishReason: "tool_calls",
			},
		},
	}
	chunk.Choices[0].Delta.JSON.ExtraFields = map[string]respjson.Field{
		"reasoning_content": respjson.NewField(`"reasoning-step"`),
	}
	return chunk
}

func makeContentChunk(content string) openai.ChatCompletionChunk {
	return openai.ChatCompletionChunk{
		Choices: []openai.ChatCompletionChunkChoice{
			{
				Index: 0,
				Delta: openai.ChatCompletionChunkChoiceDelta{
					Role:    "assistant",
					Content: content,
				},
				FinishReason: "stop",
			},
		},
	}
}

func TestNewOpenAICompatibleClient_DeepSeek(t *testing.T) {
	client, err := NewOpenAICompatibleClient(&ClientConfig{
		Provider: Provider(config.ProviderDeepSeek),
		APIKey:   "test-key",
		Model:    "deepseek-chat",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("expected client")
	}
	if client.model != "deepseek-chat" {
		t.Fatalf("expected model deepseek-chat, got %s", client.model)
	}
}

func TestNewOpenAICompatibleClient_CustomBaseURL(t *testing.T) {
	client, err := NewOpenAICompatibleClient(&ClientConfig{
		Provider: Provider(config.ProviderDeepSeek),
		APIKey:   "test-key",
		Model:    "deepseek-chat",
		BaseURL:  "https://custom.example.com/v1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("expected client")
	}
}

func TestNewOpenAICompatibleClient_OpenAIProviderRejected(t *testing.T) {
	client, err := NewOpenAICompatibleClient(&ClientConfig{
		Provider: Provider(config.ProviderOpenAI),
		APIKey:   "test-key",
		Model:    "gpt-4.1-mini",
	})
	if err == nil {
		t.Fatalf("expected error, got client: %+v", client)
	}
}

func TestOpenAICompatibleClient_StreamChat_InjectsReasoningContentAcrossToolTurns(t *testing.T) {
	client := &OpenAICompatibleClient{
		provider: Provider(config.ProviderDeepSeek),
		model:    "deepseek-reasoner",
	}

	var requests []string
	callCount := 0
	client.streamImpl = func(ctx context.Context, params openai.ChatCompletionNewParams, opts ...option.RequestOption) chatStream {
		body, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("failed to marshal params: %v", err)
		}
		requests = append(requests, string(body))

		callCount++
		if callCount == 1 {
			return &fakeChatStream{
				chunks: []openai.ChatCompletionChunk{
					makeToolCallChunk(),
				},
			}
		}
		return &fakeChatStream{
			chunks: []openai.ChatCompletionChunk{
				makeContentChunk("done"),
			},
		}
	}

	registry := tools.NewRegistry()
	if err := registry.Register(&successToolOAI{}); err != nil {
		t.Fatalf("register tool: %v", err)
	}

	eventCh, err := client.StreamChat(context.Background(), []Message{
		{Role: RoleUser, Content: "read go.mod"},
	}, registry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var hasDone bool
	var toolStartCount int
	var toolEndCount int
	var streamed strings.Builder
	var reasoning strings.Builder
	for ev := range eventCh {
		switch ev.Type {
		case StreamEventTypeDone:
			hasDone = true
		case StreamEventTypeChunk:
			streamed.WriteString(ev.Content)
		case StreamEventTypeReasoningChunk:
			reasoning.WriteString(ev.Content)
		case StreamEventTypeToolStart:
			toolStartCount++
		case StreamEventTypeToolEnd:
			toolEndCount++
		case StreamEventTypeError:
			t.Fatalf("unexpected stream error: %v", ev.Error)
		}
	}

	if !hasDone {
		t.Fatal("expected done event")
	}
	if toolStartCount != 1 || toolEndCount != 1 {
		t.Fatalf("expected 1 tool start/end, got start=%d end=%d", toolStartCount, toolEndCount)
	}
	if len(requests) != 2 {
		t.Fatalf("expected two requests, got %d", len(requests))
	}
	if !strings.Contains(requests[1], `"reasoning_content":"reasoning-step"`) {
		t.Fatalf("expected reasoning_content in second request, got: %s", requests[1])
	}
	if reasoning.String() != "reasoning-step" {
		t.Fatalf("expected reasoning stream, got: %q", reasoning.String())
	}
	if streamed.String() != "done" {
		t.Fatalf("expected assistant-only chunk stream, got: %q", streamed.String())
	}
}

func TestNewOpenAICompatibleClient_ZAI(t *testing.T) {
	client, err := NewOpenAICompatibleClient(&ClientConfig{
		Provider: Provider(config.ProviderZAI),
		APIKey:   "test-key",
		Model:    "glm-4-plus",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("expected client")
	}
	if client.model != "glm-4-plus" {
		t.Fatalf("expected model glm-4-plus, got %s", client.model)
	}
}

func TestOpenAICompatibleClient_StoresThinkingEffort(t *testing.T) {
	client, err := NewOpenAICompatibleClient(&ClientConfig{
		Provider:       Provider(config.ProviderDeepSeek),
		APIKey:         "test-key",
		Model:          "deepseek-v4-pro",
		ThinkingEffort: "high",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.thinkingEffort != "high" {
		t.Errorf("expected thinkingEffort 'high' stored, got %q", client.thinkingEffort)
	}
}

func TestOpenAICompatibleClient_DeepSeek_ThinkingEffort(t *testing.T) {
	client := &OpenAICompatibleClient{
		provider:       Provider(config.ProviderDeepSeek),
		model:          "deepseek-v4-pro",
		thinkingEffort: "high",
	}

	var capturedParams openai.ChatCompletionNewParams
	client.streamImpl = func(ctx context.Context, params openai.ChatCompletionNewParams, opts ...option.RequestOption) chatStream {
		capturedParams = params
		return &fakeChatStream{
			chunks: []openai.ChatCompletionChunk{makeContentChunk("hello")},
		}
	}

	eventCh, err := client.StreamChat(context.Background(), []Message{
		{Role: RoleUser, Content: "test"},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for range eventCh {
	}

	if string(capturedParams.ReasoningEffort) != "high" {
		t.Fatalf("expected reasoning_effort 'high', got %q", capturedParams.ReasoningEffort)
	}
	extra := capturedParams.ExtraFields()
	if extra == nil {
		t.Fatal("expected extra fields with enabled thinking config")
	}
	thinking, ok := extra["thinking"]
	if !ok {
		t.Fatal("expected 'thinking' in extra fields")
	}
	thinkingMap, ok := thinking.(map[string]any)
	if !ok {
		t.Fatalf("expected thinking to be map[string]any, got %T", thinking)
	}
	if thinkingMap["type"] != "enabled" {
		t.Fatalf("expected thinking type 'enabled', got %v", thinkingMap["type"])
	}
}

func TestOpenAICompatibleClient_DeepSeek_ThinkingOff(t *testing.T) {
	client := &OpenAICompatibleClient{
		provider:       Provider(config.ProviderDeepSeek),
		model:          "deepseek-v4-pro",
		thinkingEffort: "off",
	}

	var capturedParams openai.ChatCompletionNewParams
	client.streamImpl = func(ctx context.Context, params openai.ChatCompletionNewParams, opts ...option.RequestOption) chatStream {
		capturedParams = params
		return &fakeChatStream{
			chunks: []openai.ChatCompletionChunk{makeContentChunk("hello")},
		}
	}

	eventCh, err := client.StreamChat(context.Background(), []Message{
		{Role: RoleUser, Content: "test"},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for range eventCh {
	}

	if capturedParams.ReasoningEffort != "" {
		t.Fatalf("expected empty reasoning_effort when thinking is off, got %q", capturedParams.ReasoningEffort)
	}
	extra := capturedParams.ExtraFields()
	if extra == nil {
		t.Fatal("expected extra fields with disabled thinking config")
	}
	thinking, ok := extra["thinking"]
	if !ok {
		t.Fatal("expected 'thinking' in extra fields")
	}
	thinkingMap, ok := thinking.(map[string]any)
	if !ok {
		t.Fatalf("expected thinking to be map[string]any, got %T", thinking)
	}
	if thinkingMap["type"] != "disabled" {
		t.Fatalf("expected thinking type 'disabled', got %v", thinkingMap["type"])
	}
}

func TestOpenAICompatibleClient_ZAI_ThinkingEnabled(t *testing.T) {
	client := &OpenAICompatibleClient{
		provider:       Provider(config.ProviderZAI),
		model:          "glm-5.1",
		thinkingEffort: "enabled",
	}

	var capturedParams openai.ChatCompletionNewParams
	client.streamImpl = func(ctx context.Context, params openai.ChatCompletionNewParams, opts ...option.RequestOption) chatStream {
		capturedParams = params
		return &fakeChatStream{
			chunks: []openai.ChatCompletionChunk{makeContentChunk("hello")},
		}
	}

	eventCh, err := client.StreamChat(context.Background(), []Message{
		{Role: RoleUser, Content: "test"},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for range eventCh {
	}

	extra := capturedParams.ExtraFields()
	if extra == nil {
		t.Fatal("expected extra fields with thinking config")
	}
	thinking, ok := extra["thinking"]
	if !ok {
		t.Fatal("expected 'thinking' in extra fields")
	}
	thinkingMap, ok := thinking.(map[string]any)
	if !ok {
		t.Fatalf("expected thinking to be map[string]any, got %T", thinking)
	}
	if thinkingMap["type"] != "enabled" {
		t.Fatalf("expected thinking type 'enabled', got %v", thinkingMap["type"])
	}
}

func TestOpenAICompatibleClient_ZAI_ThinkingDisabled(t *testing.T) {
	client := &OpenAICompatibleClient{
		provider:       Provider(config.ProviderZAI),
		model:          "glm-5.1",
		thinkingEffort: "",
	}

	var capturedParams openai.ChatCompletionNewParams
	client.streamImpl = func(ctx context.Context, params openai.ChatCompletionNewParams, opts ...option.RequestOption) chatStream {
		capturedParams = params
		return &fakeChatStream{
			chunks: []openai.ChatCompletionChunk{makeContentChunk("hello")},
		}
	}

	eventCh, err := client.StreamChat(context.Background(), []Message{
		{Role: RoleUser, Content: "test"},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for range eventCh {
	}

	extra := capturedParams.ExtraFields()
	if extra != nil {
		if _, ok := extra["thinking"]; ok {
			t.Fatal("expected no thinking config when thinkingEffort is empty")
		}
	}
}

func TestToOpenAIMessages_RendersTurnMemoryForAssistant(t *testing.T) {
	messages := toOpenAIMessages([]Message{
		{
			Role:    RoleAssistant,
			Content: "done",
			TurnMemory: &TurnMemory{
				FilesChanged: []string{"a.go"},
			},
		},
	})

	body, err := json.Marshal(messages)
	if err != nil {
		t.Fatalf("marshal messages: %v", err)
	}
	if !strings.Contains(string(body), "Tool memory:") || !strings.Contains(string(body), "Files changed: a.go") {
		t.Fatalf("expected rendered turn memory in OpenAI message payload, got %s", string(body))
	}
}

func TestOpenAICompatibleClient_buildAssistantMessage_AttachesReasoningWithoutToolCalls(t *testing.T) {
	client := &OpenAICompatibleClient{}

	msg := openai.ChatCompletionMessage{
		Role:    "assistant",
		Content: "answer",
	}

	assistant := client.buildAssistantMessage(msg, "reasoning-step")
	extra := assistant.ExtraFields()
	if extra == nil {
		t.Fatal("expected extra fields to be set")
	}
	v, ok := extra["reasoning_content"]
	if !ok {
		t.Fatalf("expected reasoning_content in extra fields, got: %#v", extra)
	}
	if got, _ := v.(string); got != "reasoning-step" {
		t.Fatalf("expected reasoning_content=reasoning-step, got %#v", v)
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{"nil", nil, false},
		{"context canceled", context.Canceled, false},
		{"context deadline", context.DeadlineExceeded, false},
		{"generic error", fmt.Errorf("connection reset"), true},
		{"429 rate limit", &openai.Error{StatusCode: 429}, true},
		{"500 server error", &openai.Error{StatusCode: 500}, true},
		{"502 bad gateway", &openai.Error{StatusCode: 502}, true},
		{"503 service unavailable", &openai.Error{StatusCode: 503}, true},
		{"504 gateway timeout", &openai.Error{StatusCode: 504}, true},
		{"400 bad request", &openai.Error{StatusCode: 400}, false},
		{"401 unauthorized", &openai.Error{StatusCode: 401}, false},
		{"403 forbidden", &openai.Error{StatusCode: 403}, false},
		{"404 not found", &openai.Error{StatusCode: 404}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRetryableError(tt.err); got != tt.retryable {
				t.Errorf("isRetryableError(%v) = %v, want %v", tt.err, got, tt.retryable)
			}
		})
	}
}

func TestOpenAICompatibleClient_StreamChat_RetriesOnRetryableError(t *testing.T) {
	client := &OpenAICompatibleClient{
		provider: Provider(config.ProviderDeepSeek),
		model:    "deepseek-chat",
	}

	callCount := 0
	client.streamImpl = func(ctx context.Context, params openai.ChatCompletionNewParams, opts ...option.RequestOption) chatStream {
		callCount++
		if callCount <= 2 {
			return &fakeChatStream{err: fmt.Errorf("connection reset")}
		}
		return &fakeChatStream{
			chunks: []openai.ChatCompletionChunk{makeContentChunk("recovered")},
		}
	}

	eventCh, err := client.StreamChat(context.Background(), []Message{
		{Role: RoleUser, Content: "test"},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var retryEvents []StreamEvent
	var streamed strings.Builder
	var hasDone bool
	for ev := range eventCh {
		switch ev.Type {
		case StreamEventTypeRetry:
			retryEvents = append(retryEvents, ev)
		case StreamEventTypeChunk:
			streamed.WriteString(ev.Content)
		case StreamEventTypeDone:
			hasDone = true
		case StreamEventTypeError:
			t.Fatalf("unexpected error event: %v", ev.Error)
		}
	}

	if !hasDone {
		t.Fatal("expected done event")
	}
	if callCount != 3 {
		t.Fatalf("expected 3 calls (2 retries + 1 success), got %d", callCount)
	}
	if len(retryEvents) != 2 {
		t.Fatalf("expected 2 retry events, got %d", len(retryEvents))
	}
	if retryEvents[0].Attempt != 1 || retryEvents[1].Attempt != 2 {
		t.Fatalf("expected retry attempts 1,2; got %d,%d", retryEvents[0].Attempt, retryEvents[1].Attempt)
	}
	if streamed.String() != "recovered" {
		t.Fatalf("expected 'recovered', got %q", streamed.String())
	}
}

func TestOpenAICompatibleClient_StreamChat_NoRetryOnNonRetryableError(t *testing.T) {
	client := &OpenAICompatibleClient{
		provider: Provider(config.ProviderDeepSeek),
		model:    "deepseek-chat",
	}

	callCount := 0
	client.streamImpl = func(ctx context.Context, params openai.ChatCompletionNewParams, opts ...option.RequestOption) chatStream {
		callCount++
		return &fakeChatStream{err: &openai.Error{StatusCode: 401}}
	}

	eventCh, err := client.StreamChat(context.Background(), []Message{
		{Role: RoleUser, Content: "test"},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var hasError bool
	var retryCount int
	for ev := range eventCh {
		switch ev.Type {
		case StreamEventTypeRetry:
			retryCount++
		case StreamEventTypeError:
			hasError = true
		}
	}

	if !hasError {
		t.Fatal("expected error event")
	}
	if callCount != 1 {
		t.Fatalf("expected 1 call (no retry), got %d", callCount)
	}
	if retryCount != 0 {
		t.Fatalf("expected 0 retry events, got %d", retryCount)
	}
}
