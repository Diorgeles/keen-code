package repl

import (
	"errors"
	"time"

	"github.com/user/keen-code/internal/llm"
	"github.com/user/keen-code/internal/session"
)

type sessionReplay struct {
	output  *OutputBuilder
	handler *StreamHandler
}

func newSessionReplay(width int, mdRenderer *MarkdownRenderer, workingDir string) *sessionReplay {
	outputWidth := defaultWidth
	if width > 0 {
		outputWidth = width
	}

	handler := NewStreamHandler(mdRenderer)
	handler.lastWidth = width
	handler.workingDir = workingDir
	if handler.lastWidth <= 0 {
		handler.lastWidth = defaultWidth
	}

	return &sessionReplay{
		output:  &OutputBuilder{width: outputWidth, lines: []string{}, workingDir: workingDir},
		handler: handler,
	}
}

func (r *sessionReplay) applyEvent(event session.Event) {
	switch event.Kind {
	case session.KindUserMessage:
		r.flushDone()
		if event.UserMessage != nil {
			r.output.AddUserInput(event.UserMessage.Content, promptStyle)
		}
	case session.KindAssistantTurn:
		r.applyAssistantTurn(event.AssistantTurn)
	case session.KindCompactionApplied:
		r.flushDone()
		if event.CompactionApplied != nil {
			addCompactionSuccessStatus(r.output, event.CompactionApplied.Status)
		}
	}
}

func (r *sessionReplay) applyAssistantTurn(turn *session.AssistantTurnPayload) {
	if turn == nil {
		return
	}

	replayAssistantTurn(r.handler, turn)

	switch {
	case turn.Interrupted:
		r.flushInterrupt()
	case turn.Error != "":
		r.flushError(turn.Error)
	default:
		r.flushDone()
	}
}

func (r *sessionReplay) flushDone() {
	if !r.handler.HasContent() {
		return
	}
	lines, _ := r.handler.HandleDone()
	for _, line := range lines {
		r.output.AddLine(line)
	}
	r.output.AddEmptyLine()
}

func (r *sessionReplay) flushInterrupt() {
	if r.handler.HasContent() {
		lines := r.handler.HandleInterrupt()
		for _, line := range lines {
			r.output.AddLine(line)
		}
	}
	r.output.AddStyledLine("\n  "+interruptedPromptText, interruptedStyle)
	r.output.AddEmptyLine()
}

func (r *sessionReplay) flushError(errText string) {
	if r.handler.HasContent() {
		lines, _ := r.handler.HandleError(errors.New(errText))
		for _, line := range lines {
			r.output.AddLine(line)
		}
	}
	if errText != "" {
		r.output.AddError(errText, errorStyle)
	}
}

func replayAssistantTurn(handler *StreamHandler, turn *session.AssistantTurnPayload) {
	if handler == nil || turn == nil {
		return
	}

	for _, item := range turn.Transcript {
		switch item.Kind {
		case session.TranscriptItemText:
			handler.HandleChunk(item.Content)
		case session.TranscriptItemReasoning:
			handler.HandleReasoningChunk(item.Content)
		case session.TranscriptItemToolStart:
			handler.HandleToolStart(toolCallFromPayload(item.ToolStart))
		case session.TranscriptItemToolEnd:
			handler.HandleToolEnd(toolCallResultFromPayload(item.ToolEnd))
		case session.TranscriptItemBash:
			replayBashPayload(handler, item.Bash)
		case session.TranscriptItemDiff:
			if item.Diff != nil {
				handler.HandleDiff(item.Diff.Lines)
			}
		}
	}
}

func replayBashPayload(handler *StreamHandler, payload *session.BashPayload) {
	if handler == nil || payload == nil {
		return
	}

	handler.HandleBashStart(payload.Command, payload.Summary)
	handler.HandleBashEnd(&llm.ToolCall{
		Name:     "bash",
		Output:   map[string]any{"stdout": payload.Output},
		Error:    payload.Error,
		Duration: time.Duration(payload.DurationNS),
	})
}
