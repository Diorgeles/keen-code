package repl

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/user/keen-code/internal/config"
	"github.com/user/keen-code/internal/llm"
	"github.com/user/keen-code/internal/tools"
)

const compactionUserInstruction = "Please compact this conversation according to the system instructions."

type AppState struct {
	messages     []llm.Message
	llmClient    llm.LLMClient
	toolRegistry *tools.Registry
	workingDir   string
}

func NewAppState(client llm.LLMClient, workingDir string) *AppState {
	return &AppState{
		messages:     []llm.Message{},
		llmClient:    client,
		toolRegistry: tools.NewRegistry(),
		workingDir:   workingDir,
	}
}

func (s *AppState) AddMessage(role llm.Role, content string) {
	s.messages = append(s.messages, llm.Message{
		Role:    role,
		Content: content,
	})
}

func (s *AppState) GetMessages() []llm.Message {
	result := make([]llm.Message, len(s.messages))
	copy(result, s.messages)
	return result
}

func (s *AppState) ClearMessages() {
	s.messages = []llm.Message{}
}

func (s *AppState) ReplaceMessages(messages []llm.Message) {
	s.messages = make([]llm.Message, len(messages))
	copy(s.messages, messages)
}

func (s *AppState) StreamChat(ctx context.Context, cfg *config.ResolvedConfig) (<-chan llm.StreamEvent, error) {
	if s.llmClient == nil {
		return nil, nil
	}
	systemMsg := llm.Message{
		Role:    llm.RoleSystem,
		Content: llm.Build(s.workingDir),
	}
	messages := append([]llm.Message{systemMsg}, s.GetMessages()...)
	return s.llmClient.StreamChat(ctx, messages, s.toolRegistry)
}

func (s *AppState) Compact(ctx context.Context, cfg *config.ResolvedConfig, extraPrompt string) error {
	if len(s.messages) == 0 {
		return nil
	}
	if s.llmClient == nil {
		return fmt.Errorf("LLM client not initialized")
	}
	if cfg == nil || cfg.APIKey == "" || cfg.Model == "" {
		return fmt.Errorf("LLM client not initialized")
	}

	snapshot := s.GetMessages()

	requestMessages := make([]llm.Message, 0, len(snapshot)+2)
	requestMessages = append(requestMessages, llm.Message{
		Role:    llm.RoleSystem,
		Content: llm.BuildCompactionPrompt(extraPrompt),
	})
	requestMessages = append(requestMessages, snapshot...)
	requestMessages = append(requestMessages, llm.Message{
		Role:    llm.RoleUser,
		Content: compactionUserInstruction,
	})

	eventCh, err := s.llmClient.StreamChat(ctx, requestMessages, nil)
	if err != nil {
		return err
	}
	if eventCh == nil {
		return fmt.Errorf("compaction stream unavailable")
	}

	var summary strings.Builder
	for event := range eventCh {
		switch event.Type {
		case llm.StreamEventTypeChunk:
			summary.WriteString(event.Content)
		case llm.StreamEventTypeError:
			if event.Error != nil {
				return event.Error
			}
			return fmt.Errorf("compaction failed")
		case llm.StreamEventTypeDone:
			compacted := strings.TrimSpace(summary.String())
			if compacted == "" {
				return fmt.Errorf("compaction returned empty summary")
			}
			s.messages = []llm.Message{{
				Role:    llm.RoleUser,
				Content: compacted,
			}}
			return nil
		}
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	return errors.New("compaction ended without completion")
}

func (s *AppState) IsClientReady(cfg *config.ResolvedConfig) bool {
	return s.llmClient != nil && cfg.APIKey != "" && cfg.Model != ""
}

func (s *AppState) UpdateClient(client llm.LLMClient) {
	s.llmClient = client
}

func (s *AppState) GetClient() llm.LLMClient {
	return s.llmClient
}

func (s *AppState) GetToolRegistry() *tools.Registry {
	return s.toolRegistry
}

func (s *AppState) RegisterTool(tool tools.Tool) error {
	return s.toolRegistry.Register(tool)
}
