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
	"github.com/joho/godotenv"
	"github.com/user/keen-code/internal/tools"
)

const anthropicMaxTokens = 16192

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
	client     anthropic.Client
	model      string
	streamImpl anthropicStreamFactory
}

func NewAnthropicClient(cfg *ClientConfig) (*AnthropicClient, error) {
	env, _ := godotenv.Read(".env")
	baseURL := env["ANTHROPIC_BASE_URL"]

	opts := []option.RequestOption{
		option.WithAPIKey(cfg.APIKey),
	}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}

	client := anthropic.NewClient(opts...)

	c := &AnthropicClient{
		client: client,
		model:  cfg.Model,
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
			msgParams = append(msgParams, anthropic.NewUserMessage(anthropic.NewTextBlock(content)))
		case RoleAssistant:
			msgParams = append(msgParams, anthropic.NewAssistantMessage(anthropic.NewTextBlock(content)))
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
) ([]anthropic.ContentBlockParamUnion, []toolUseEntry, error) {
	stream := c.streamImpl(ctx, params)

	// track open tool_use blocks by index so we can accumulate partial JSON
	type toolUseState struct {
		id          string
		name        string
		inputBuffer []byte
	}
	toolStates := map[int64]*toolUseState{}

	var assistantBlocks []anthropic.ContentBlockParamUnion

	for stream.Next() {
		ev := stream.Current()

		switch ev.Type {
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
		return nil, nil, fmt.Errorf("stream error: %w", err)
	}

	// Collect tool use entries for execution
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

	return assistantBlocks, toolUses, nil
}

type toolUseEntry struct {
	id    string
	name  string
	input map[string]any
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
			params := anthropic.MessageNewParams{
				Model:     c.model,
				MaxTokens: anthropicMaxTokens,
				Messages:  msgParams,
				Thinking: anthropic.ThinkingConfigParamUnion{
					OfAdaptive: &anthropic.ThinkingConfigAdaptiveParam{},
				},
				OutputConfig: anthropic.OutputConfigParam{
					Effort: anthropic.OutputConfigEffortHigh,
				},
			}
			if len(systemBlocks) > 0 {
				params.System = systemBlocks
			}
			if len(anthropicTools) > 0 {
				params.Tools = anthropicTools
			}

			assistantBlocks, toolUses, err := c.collectTurn(ctx, params, eventCh)
			if err != nil {
				eventCh <- StreamEvent{Type: StreamEventTypeError, Error: err}
				return
			}

			if len(toolUses) == 0 {
				eventCh <- StreamEvent{Type: StreamEventTypeDone}
				return
			}

			// Append assistant message with all blocks (text + tool_use)
			msgParams = append(msgParams, anthropic.NewAssistantMessage(assistantBlocks...))

			// Execute tools and collect results
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
