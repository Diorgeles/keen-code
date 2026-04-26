package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"github.com/anthropics/anthropic-sdk-go/packages/ssestream"
	"github.com/user/keen-code/internal/tools"
)

const anthropicMaxTokens = 65536

type anthropicStream interface {
	Next() bool
	Current() anthropic.MessageStreamEventUnion
	Err() error
	Close() error
}

type anthropicStreamFactory func(ctx context.Context, params anthropic.MessageNewParams) anthropicStream

type sdkAnthropicStream struct {
	stream *ssestream.Stream[anthropic.MessageStreamEventUnion]
}

func (s *sdkAnthropicStream) Next() bool {
	return s.stream.Next()
}

func (s *sdkAnthropicStream) Current() anthropic.MessageStreamEventUnion {
	return s.stream.Current()
}

func (s *sdkAnthropicStream) Err() error {
	return s.stream.Err()
}

func (s *sdkAnthropicStream) Close() error {
	return s.stream.Close()
}

type AnthropicClient struct {
	client         anthropic.Client
	model          string
	thinkingEffort string
	streamImpl     anthropicStreamFactory
}

func NewAnthropicClient(cfg *ClientConfig) (*AnthropicClient, error) {
	opts := []option.RequestOption{
		option.WithAPIKey(cfg.APIKey),
	}
	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	}

	client := anthropic.NewClient(opts...)

	c := &AnthropicClient{
		client:         client,
		model:          cfg.Model,
		thinkingEffort: cfg.ThinkingEffort,
	}
	c.streamImpl = func(ctx context.Context, params anthropic.MessageNewParams) anthropicStream {
		return &sdkAnthropicStream{stream: c.client.Messages.NewStreaming(ctx, params)}
	}

	return c, nil
}

func toAnthropicMessages(messages []Message) ([]anthropic.TextBlockParam, []anthropic.MessageParam) {
	var systemBlocks []anthropic.TextBlockParam
	var msgParams []anthropic.MessageParam

	for _, m := range messages {
		content := FormatMessageForProvider(m)
		switch m.Role {
		case RoleSystem:
			systemBlocks = append(systemBlocks, anthropic.TextBlockParam{Text: content})
		case RoleUser:
			if content != "" {
				msgParams = append(msgParams, anthropic.NewUserMessage(anthropic.NewTextBlock(content)))
			}
		case RoleAssistant:
			if content != "" {
				msgParams = append(msgParams, anthropic.NewAssistantMessage(anthropic.NewTextBlock(content)))
			}
		}
	}

	return systemBlocks, msgParams
}

func toAnthropicTools(registry *tools.Registry) []anthropic.ToolUnionParam {
	if registry == nil {
		return nil
	}

	all := registry.All()
	result := make([]anthropic.ToolUnionParam, 0, len(all))
	for _, t := range all {
		schema := t.InputSchema()
		inputSchema := anthropic.ToolInputSchemaParam{}
		if props, ok := schema["properties"]; ok {
			inputSchema.Properties = props
		}
		if req, ok := schema["required"].([]string); ok {
			inputSchema.Required = req
		}

		result = append(result, anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        t.Name(),
				Description: param.NewOpt(t.Description()),
				InputSchema: inputSchema,
			},
		})
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

func (c *AnthropicClient) collectTurn(
	ctx context.Context,
	params anthropic.MessageNewParams,
	eventCh chan<- StreamEvent,
) ([]anthropic.ContentBlockParamUnion, []toolUseEntry, *TokenUsage, error) {
	stream := c.streamImpl(ctx, params)

	// track open tool_use blocks by index so we can accumulate partial JSON
	type toolUseState struct {
		id          string
		name        string
		inputBuffer []byte
	}
	toolStates := map[int64]*toolUseState{}

	var assistantBlocks []anthropic.ContentBlockParamUnion
	var usage *TokenUsage

	for stream.Next() {
		ev := stream.Current()

		switch ev.Type {
		case "message_start":
			ms := ev.AsMessageStart()
			slog.Debug(
				"Anthropic stream usage",
				"event_type", "message_start",
				"input_tokens", ms.Message.Usage.InputTokens,
				"output_tokens", ms.Message.Usage.OutputTokens,
				"cache_creation_input_tokens", ms.Message.Usage.CacheCreationInputTokens,
				"cache_read_input_tokens", ms.Message.Usage.CacheReadInputTokens,
			)
			if ms.Message.Usage.InputTokens > 0 {
				totalInputTokens := int(ms.Message.Usage.InputTokens + ms.Message.Usage.CacheCreationInputTokens + ms.Message.Usage.CacheReadInputTokens)
				cachedTokens := int(ms.Message.Usage.CacheCreationInputTokens + ms.Message.Usage.CacheReadInputTokens)
				usage = &TokenUsage{
					InputTokens:  totalInputTokens,
					OutputTokens: int(ms.Message.Usage.OutputTokens),
					TotalTokens:  totalInputTokens + int(ms.Message.Usage.OutputTokens),
					CachedTokens: cachedTokens,
				}
			}

		case "message_delta":
			md := ev.AsMessageDelta()
			slog.Debug(
				"Anthropic stream usage",
				"event_type", "message_delta",
				"input_tokens", md.Usage.InputTokens,
				"output_tokens", md.Usage.OutputTokens,
				"cache_creation_input_tokens", md.Usage.CacheCreationInputTokens,
				"cache_read_input_tokens", md.Usage.CacheReadInputTokens,
			)
			if usage == nil && md.Usage.InputTokens > 0 {
				usage = &TokenUsage{}
			}
			if usage != nil {
				totalInputTokens := int(md.Usage.InputTokens + md.Usage.CacheCreationInputTokens + md.Usage.CacheReadInputTokens)
				cachedTokens := int(md.Usage.CacheCreationInputTokens + md.Usage.CacheReadInputTokens)
				usage.InputTokens = totalInputTokens
				usage.OutputTokens = int(md.Usage.OutputTokens)
				usage.CachedTokens = cachedTokens
				usage.TotalTokens = totalInputTokens + usage.OutputTokens
			}

		case "content_block_start":
			cbs := ev.AsContentBlockStart()
			switch cbs.ContentBlock.Type {
			case "tool_use":
				tu := cbs.ContentBlock.AsToolUse()
				toolStates[cbs.Index] = &toolUseState{
					id:   tu.ID,
					name: tu.Name,
				}
			}

		case "content_block_delta":
			cbd := ev.AsContentBlockDelta()
			switch cbd.Delta.Type {
			case "text_delta":
				if cbd.Delta.Text != "" {
					eventCh <- StreamEvent{
						Type:    StreamEventTypeChunk,
						Content: cbd.Delta.Text,
					}
				}
			case "thinking_delta":
				if cbd.Delta.Thinking != "" {
					eventCh <- StreamEvent{
						Type:    StreamEventTypeReasoningChunk,
						Content: cbd.Delta.Thinking,
					}
				}
			case "input_json_delta":
				if state, ok := toolStates[cbd.Index]; ok {
					state.inputBuffer = append(state.inputBuffer, []byte(cbd.Delta.PartialJSON)...)
				}
			}

		case "content_block_stop":
			cbs := ev.AsContentBlockStop()
			if state, ok := toolStates[cbs.Index]; ok {
				var inputRaw json.RawMessage = state.inputBuffer
				if len(inputRaw) == 0 {
					inputRaw = json.RawMessage("{}")
				}
				assistantBlocks = append(assistantBlocks, anthropic.NewToolUseBlock(state.id, inputRaw, state.name))
				delete(toolStates, cbs.Index)
			}
		}
	}
	_ = stream.Close()

	if err := stream.Err(); err != nil {
		return nil, nil, nil, fmt.Errorf("stream error: %w", err)
	}

	var toolUses []toolUseEntry
	for _, block := range assistantBlocks {
		if block.OfToolUse == nil {
			continue
		}
		tu := block.OfToolUse
		var input map[string]any
		if err := json.Unmarshal(tu.Input.(json.RawMessage), &input); err != nil {
			input = map[string]any{}
		}
		toolUses = append(toolUses, toolUseEntry{
			id:    tu.ID,
			name:  tu.Name,
			input: input,
		})
	}

	return assistantBlocks, toolUses, usage, nil
}

type toolUseEntry struct {
	id    string
	name  string
	input map[string]any
}

func anthropicThinkingParams(effort string) (anthropic.ThinkingConfigParamUnion, anthropic.OutputConfigParam, int64) {
	switch effort {
	case "low", "medium", "high", "max":
		thinking := anthropic.ThinkingConfigParamUnion{
			OfAdaptive: &anthropic.ThinkingConfigAdaptiveParam{},
		}
		outCfg := anthropic.OutputConfigParam{
			Effort: anthropic.OutputConfigEffort(effort),
		}
		return thinking, outCfg, anthropicMaxTokens
	default:
		thinking := anthropic.ThinkingConfigParamUnion{
			OfDisabled: &anthropic.ThinkingConfigDisabledParam{},
		}
		return thinking, anthropic.OutputConfigParam{}, anthropicMaxTokens
	}
}

func (c *AnthropicClient) StreamChat(
	ctx context.Context,
	messages []Message,
	toolRegistry *tools.Registry,
) (<-chan StreamEvent, error) {
	eventCh := make(chan StreamEvent)

	go func() {
		defer close(eventCh)

		systemBlocks, msgParams := toAnthropicMessages(messages)
		anthropicTools := toAnthropicTools(toolRegistry)

		for range maxToolTurns {
			thinking, outCfg, maxTok := anthropicThinkingParams(c.thinkingEffort)
			params := anthropic.MessageNewParams{
				Model:        c.model,
				MaxTokens:    maxTok,
				Messages:     msgParams,
				Thinking:     thinking,
				OutputConfig: outCfg,
			}
			if len(systemBlocks) > 0 {
				params.System = systemBlocks
			}
			if len(anthropicTools) > 0 {
				params.Tools = anthropicTools
			}

			assistantBlocks, toolUses, usage, err := c.collectTurn(ctx, params, eventCh)
			if err != nil {
				eventCh <- StreamEvent{Type: StreamEventTypeError, Error: err}
				return
			}

			if usage != nil {
				slog.Debug(
					"Anthropic usage emitted",
					"input_tokens", usage.InputTokens,
					"output_tokens", usage.OutputTokens,
					"total_tokens", usage.TotalTokens,
					"cached_tokens", usage.CachedTokens,
				)
				eventCh <- StreamEvent{Type: StreamEventTypeUsage, Usage: usage}
			} else {
				slog.Debug("Anthropic usage unavailable for turn")
			}

			if len(toolUses) == 0 {
				eventCh <- StreamEvent{Type: StreamEventTypeDone}
				return
			}

			msgParams = append(msgParams, anthropic.NewAssistantMessage(assistantBlocks...))

			toolResultBlocks := c.executeTools(ctx, toolUses, toolRegistry, eventCh)
			msgParams = append(msgParams, anthropic.NewUserMessage(toolResultBlocks...))
		}

		eventCh <- StreamEvent{Type: StreamEventTypeDone}
	}()

	return eventCh, nil
}

func (c *AnthropicClient) executeTools(
	ctx context.Context,
	toolUses []toolUseEntry,
	registry *tools.Registry,
	eventCh chan<- StreamEvent,
) []anthropic.ContentBlockParamUnion {
	var resultBlocks []anthropic.ContentBlockParamUnion

	for _, tu := range toolUses {
		start := time.Now()

		slog.Debug("Tool request", "tool", tu.name, "input", tu.input)
		eventCh <- StreamEvent{
			Type: StreamEventTypeToolStart,
			ToolCall: &ToolCall{
				Name:  tu.name,
				Input: tu.input,
			},
		}

		var output any
		var execErr error

		if registry == nil {
			execErr = fmt.Errorf("tool registry not available")
		} else if tool, exists := registry.Get(tu.name); !exists {
			execErr = fmt.Errorf("tool %q not found", tu.name)
		} else {
			output, execErr = tool.Execute(ctx, tu.input)
		}

		duration := time.Since(start)
		toolCall := &ToolCall{
			Name:     tu.name,
			Input:    tu.input,
			Output:   output,
			Duration: duration,
		}

		var resultContent string
		if execErr != nil {
			toolCall.Error = execErr.Error()
			slog.Debug("Tool response", "tool", tu.name, "error", execErr.Error(), "duration", duration)
			eventCh <- StreamEvent{Type: StreamEventTypeToolEnd, ToolCall: toolCall}
			resultContent = fmt.Sprintf(`{"error":%q}`, execErr.Error())
		} else {
			slog.Debug("Tool response", "tool", tu.name, "duration", duration)
			eventCh <- StreamEvent{Type: StreamEventTypeToolEnd, ToolCall: toolCall}
			if output == nil {
				output = map[string]any{}
			}
			b, err := json.Marshal(output)
			if err != nil {
				resultContent = "{}"
			} else {
				resultContent = string(b)
			}
		}

		resultBlocks = append(resultBlocks, anthropic.NewToolResultBlock(tu.id, resultContent, execErr != nil))
	}

	return resultBlocks
}
