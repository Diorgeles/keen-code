package repl

import "github.com/user/keen-code/internal/llm"

type llmChunkMsg string
type llmReasoningChunkMsg string
type llmDoneMsg struct{}
type llmErrorMsg struct {
	err error
}
type llmToolStartMsg struct {
	toolCall *llm.ToolCall
}
type llmToolEndMsg struct {
	toolCall *llm.ToolCall
}
type permissionReadyMsg struct {
	req *PermissionRequest
}
type diffReadyMsg struct {
	req diffEmitRequest
}
type compactionDoneMsg struct{}
type compactionErrMsg struct {
	err error
}
