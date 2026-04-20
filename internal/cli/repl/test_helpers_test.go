package repl

import (
	"context"

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
