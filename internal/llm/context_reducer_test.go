package llm

import (
	"strings"
	"testing"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/firebase/genkit/go/ai"
	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/responses"
)

func TestContextFitsBudget(t *testing.T) {
	if !contextFitsBudget(10000, 700) {
		t.Fatal("expected context to fit budget")
	}
	if contextFitsBudget(10000, 800) {
		t.Fatal("expected context to exceed budget")
	}
}

func TestReduceToolResultsForRequest_RemovesOldestUntilBudgetFits(t *testing.T) {
	var removed []int
	targets := []toolResultReductionTarget{
		{tokenCount: 50, remove: func() { removed = append(removed, 0) }},
		{tokenCount: 100, remove: func() { removed = append(removed, 1) }},
		{tokenCount: 100, remove: func() { removed = append(removed, 2) }},
	}

	reduction := reduceToolResultsForRequest(10000, 850, targets)

	if !reduction.FitsBudget {
		t.Fatal("expected reduced context to fit budget")
	}
	if reduction.RemovedToolResults != 2 {
		t.Fatalf("expected 2 removed tool results, got %d", reduction.RemovedToolResults)
	}
	if reduction.OriginalTokenCount != 850 {
		t.Fatalf("expected original token count 850, got %d", reduction.OriginalTokenCount)
	}
	wantReduced := 850 - 50 - 100 + 2*estimateContextTokenCount(removedToolResultPlaceholder)
	if reduction.ReducedTokenCount != wantReduced {
		t.Fatalf("expected reduced token count %d, got %d", wantReduced, reduction.ReducedTokenCount)
	}
	if len(removed) != 2 || removed[0] != 0 || removed[1] != 1 {
		t.Fatalf("expected oldest two removals, got %v", removed)
	}
}

func TestReduceToolResultsForRequest_ReturnsNotFitAfterRemovingAllTargets(t *testing.T) {
	removed := 0
	targets := []toolResultReductionTarget{
		{tokenCount: 50, remove: func() { removed++ }},
	}

	reduction := reduceToolResultsForRequest(10000, 1000, targets)

	if reduction.FitsBudget {
		t.Fatal("expected reduced context to remain over budget")
	}
	if reduction.RemovedToolResults != 1 {
		t.Fatalf("expected 1 removed tool result, got %d", reduction.RemovedToolResults)
	}
	if removed != 1 {
		t.Fatalf("expected target removal to run once, got %d", removed)
	}
}

func TestReduceOpenAIContextForRequest_ReplacesOldestToolResults(t *testing.T) {
	longResult := repeatString("old ", 200)
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("inspect files"),
		openai.ToolMessage(longResult, "call_old"),
		openai.ToolMessage("recent result", "call_recent"),
	}

	reduced, reduction := reduceOpenAIContextForRequest(10000, 850, messages)

	if !reduction.FitsBudget {
		t.Fatal("expected reduced context to fit budget")
	}
	if reduction.RemovedToolResults != 1 {
		t.Fatalf("expected 1 removed tool result, got %d", reduction.RemovedToolResults)
	}
	if got := openAIToolContent(reduced[1].OfTool.Content); got != removedToolResultPlaceholder {
		t.Fatalf("expected oldest tool result placeholder, got %q", got)
	}
	if got := openAIToolContent(reduced[2].OfTool.Content); got != "recent result" {
		t.Fatalf("expected recent tool result to remain, got %q", got)
	}
}

func TestReduceResponsesContextForRequest_SkipsExistingPlaceholders(t *testing.T) {
	longResult := repeatString("old ", 200)
	input := []responses.ResponseInputItemUnionParam{
		responses.ResponseInputItemParamOfFunctionCallOutput("call_removed", removedToolResultPlaceholder),
		responses.ResponseInputItemParamOfFunctionCallOutput("call_old", longResult),
		responses.ResponseInputItemParamOfFunctionCallOutput("call_recent", "recent result"),
	}

	reduced, reduction := reduceResponsesContextForRequest(10000, 850, input)

	if reduction.RemovedToolResults != 1 {
		t.Fatalf("expected 1 removed tool result, got %d", reduction.RemovedToolResults)
	}
	if got := reduced[0].OfFunctionCallOutput.Output; got != removedToolResultPlaceholder {
		t.Fatalf("expected existing placeholder to remain, got %q", got)
	}
	if got := reduced[1].OfFunctionCallOutput.Output; got != removedToolResultPlaceholder {
		t.Fatalf("expected oldest non-placeholder result to be replaced, got %q", got)
	}
	if got := reduced[2].OfFunctionCallOutput.Output; got != "recent result" {
		t.Fatalf("expected recent tool result to remain, got %q", got)
	}
}

func TestReduceAnthropicContextForRequest_ReplacesToolResultContentOnly(t *testing.T) {
	longResult := repeatString("old ", 200)
	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(
			anthropic.NewToolResultBlock("toolu_old", longResult, false),
			anthropic.NewToolResultBlock("toolu_recent", "recent result", false),
		),
	}

	reduced, reduction := reduceAnthropicContextForRequest(10000, 850, messages)

	if reduction.RemovedToolResults != 1 {
		t.Fatalf("expected 1 removed tool result, got %d", reduction.RemovedToolResults)
	}
	oldResult := reduced[0].Content[0].OfToolResult
	if oldResult == nil {
		t.Fatal("expected first block to remain a tool result")
	}
	if oldResult.ToolUseID != "toolu_old" {
		t.Fatalf("expected tool use id to be preserved, got %q", oldResult.ToolUseID)
	}
	if got := anthropicToolResultContent(oldResult); got != removedToolResultPlaceholder {
		t.Fatalf("expected placeholder content, got %q", got)
	}
	if got := anthropicToolResultContent(reduced[0].Content[1].OfToolResult); got != "recent result" {
		t.Fatalf("expected recent tool result to remain, got %q", got)
	}
}

func TestReduceGenkitContextForRequest_ReplacesToolResponseOutput(t *testing.T) {
	messages := []*ai.Message{
		ai.NewMessage(ai.RoleTool, nil,
			ai.NewToolResponsePart(&ai.ToolResponse{
				Name:   "read_file",
				Ref:    "call_old",
				Output: map[string]any{"content": repeatString("old ", 200)},
			}),
			ai.NewToolResponsePart(&ai.ToolResponse{
				Name:   "grep",
				Ref:    "call_recent",
				Output: "recent result",
			}),
		),
	}

	reduced, reduction := reduceGenkitContextForRequest(10000, 850, messages)

	if reduction.RemovedToolResults != 1 {
		t.Fatalf("expected 1 removed tool result, got %d", reduction.RemovedToolResults)
	}
	oldResponse := reduced[0].Content[0].ToolResponse
	if oldResponse == nil {
		t.Fatal("expected first part to remain a tool response")
	}
	if oldResponse.Name != "read_file" || oldResponse.Ref != "call_old" {
		t.Fatalf("expected tool response identity to be preserved, got name=%q ref=%q", oldResponse.Name, oldResponse.Ref)
	}
	if got := oldResponse.Output; got != removedToolResultPlaceholder {
		t.Fatalf("expected placeholder output, got %#v", got)
	}
	if got := reduced[0].Content[1].ToolResponse.Output; got != "recent result" {
		t.Fatalf("expected recent tool output to remain, got %#v", got)
	}
}

func repeatString(s string, count int) string {
	var out strings.Builder
	for range count {
		out.WriteString(s)
	}
	return out.String()
}
