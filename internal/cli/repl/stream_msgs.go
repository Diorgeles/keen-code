package repl

import (
	replpermissions "github.com/user/keen-code/internal/cli/repl/permissions"
	repltooling "github.com/user/keen-code/internal/cli/repl/tooling"
	"github.com/user/keen-code/internal/llm"
)

type llmChunkMsg string
type llmReasoningChunkMsg string
type llmDoneMsg struct{}
type llmErrorMsg struct {
	err error
}
type llmRetryMsg struct {
	err     error
	attempt int
}
type llmToolStartMsg struct {
	toolCall *llm.ToolCall
}
type llmToolEndMsg struct {
	toolCall *llm.ToolCall
}
type llmUsageMsg struct {
	usage *llm.TokenUsage
}
type permissionReadyMsg struct {
	req *replpermissions.Request
}
type diffReadyMsg struct {
	req repltooling.DiffRequest
}
type compactionDoneMsg struct{}
type compactionErrMsg struct {
	err error
}
type updateCheckMsg struct {
	latest string
}
