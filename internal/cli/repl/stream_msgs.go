package repl

import (
	replpermissions "github.com/user/keen-code/internal/cli/repl/permissions"
	repltooling "github.com/user/keen-code/internal/cli/repl/tooling"
	"github.com/user/keen-code/internal/llm"
	keenmcp "github.com/user/keen-code/internal/mcp"
)

type llmChunkMsg string
type llmReasoningChunkMsg string
type llmDoneMsg struct{}
type llmIncompleteMsg struct {
	err error
}
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
type mcpStartupStatusMsg struct {
	Statuses []keenmcp.ServerStatus
}
type mcpConnectDoneMsg struct {
	Server string
	Status keenmcp.ServerStatus
	Err    error
}

type btwChunkMsg string
type btwDoneMsg struct{}
type btwErrorMsg struct {
	err error
}

type adversaryChunkMsg string
type adversaryDoneMsg  struct{}
type adversaryErrorMsg struct {
	err error
}
