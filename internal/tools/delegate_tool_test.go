package tools

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type delegateCall struct {
	agent          string
	task           string
	timeoutSeconds int
}

type mockSubagentRunner struct {
	mu sync.Mutex

	result any
	err    error
	calls  []delegateCall

	started chan struct{}
	release chan struct{}
}

func (m *mockSubagentRunner) RunDelegate(ctx context.Context, agent, task string, timeoutSeconds int) (any, error) {
	m.mu.Lock()
	m.calls = append(m.calls, delegateCall{agent: agent, task: task, timeoutSeconds: timeoutSeconds})
	m.mu.Unlock()
	if m.started != nil {
		m.started <- struct{}{}
	}
	if m.release != nil {
		select {
		case <-m.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return m.result, m.err
}

func (m *mockSubagentRunner) recordedCalls() []delegateCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]delegateCall(nil), m.calls...)
}

func TestDelegateTool_Metadata(t *testing.T) {
	tool := NewDelegateTool(&mockSubagentRunner{})

	if tool.Name() != "delegate_task" {
		t.Fatalf("Name() = %q, want %q", tool.Name(), "delegate_task")
	}
	if !strings.Contains(tool.Description(), "up to 10") || !strings.Contains(tool.Description(), "parallel") {
		t.Fatalf("Description() = %q, want parallel limit", tool.Description())
	}
}

func TestDelegateTool_InputSchema(t *testing.T) {
	tool := NewDelegateTool(&mockSubagentRunner{})
	schema := tool.InputSchema()

	if schema["type"] != "object" {
		t.Fatalf("schema type = %v, want object", schema["type"])
	}
	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatalf("required type = %T, want []string", schema["required"])
	}
	if !reflect.DeepEqual(required, []string{"tasks"}) {
		t.Fatalf("required = %v, want [tasks]", required)
	}

	properties := schema["properties"].(map[string]any)
	tasks := properties["tasks"].(map[string]any)
	if tasks["minItems"] != 1 || tasks["maxItems"] != maxDelegateTasks {
		t.Fatalf("task bounds = %v..%v, want 1..%d", tasks["minItems"], tasks["maxItems"], maxDelegateTasks)
	}
	items := tasks["items"].(map[string]any)
	if !reflect.DeepEqual(items["required"], []string{"agent", "task"}) {
		t.Fatalf("item required = %v, want [agent task]", items["required"])
	}
	itemProperties := items["properties"].(map[string]any)
	for _, name := range []string{"agent", "task", "timeout_seconds"} {
		if _, ok := itemProperties[name]; !ok {
			t.Fatalf("item properties missing %q", name)
		}
	}
}

func TestDelegateTool_ExecutePassesTasksToRunner(t *testing.T) {
	runner := &mockSubagentRunner{result: map[string]any{"status": "completed"}}
	tool := NewDelegateTool(runner)

	result, err := tool.Execute(context.Background(), delegateTasks(
		map[string]any{"agent": "explorer", "task": "Inspect internal/tools.", "timeout_seconds": 30},
	))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	calls := runner.recordedCalls()
	if len(calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(calls))
	}
	wantCall := delegateCall{agent: "explorer", task: "Inspect internal/tools.", timeoutSeconds: 30}
	if calls[0] != wantCall {
		t.Fatalf("call = %+v, want %+v", calls[0], wantCall)
	}
	output := result.(map[string]any)
	results := output["results"].([]delegateResult)
	if len(results) != 1 || !reflect.DeepEqual(results[0].Result, runner.result) {
		t.Fatalf("results = %#v, want runner result", results)
	}
	if output["completed"] != 1 || output["failed"] != 0 {
		t.Fatalf("counts = completed:%v failed:%v, want 1 and 0", output["completed"], output["failed"])
	}
	if !reflect.DeepEqual(output["completed_by_agent"], map[string]int{"explorer": 1}) {
		t.Fatalf("completed_by_agent = %#v, want explorer:1", output["completed_by_agent"])
	}
	if !reflect.DeepEqual(output["failed_by_agent"], map[string]int{}) {
		t.Fatalf("failed_by_agent = %#v, want empty", output["failed_by_agent"])
	}
}

func TestDelegateTool_ExecuteRunsTasksInParallel(t *testing.T) {
	const taskCount = 3
	runner := &mockSubagentRunner{
		result:  "ok",
		started: make(chan struct{}, taskCount),
		release: make(chan struct{}),
	}
	tool := NewDelegateTool(runner)
	done := make(chan error, 1)

	go func() {
		_, err := tool.Execute(context.Background(), delegateTasks(
			map[string]any{"agent": "explorer", "task": "one"},
			map[string]any{"agent": "explorer", "task": "two"},
			map[string]any{"agent": "explorer", "task": "three"},
		))
		done <- err
	}()

	for i := 0; i < taskCount; i++ {
		select {
		case <-runner.started:
		case <-time.After(time.Second):
			t.Fatal("tasks did not start in parallel")
		}
	}
	close(runner.release)
	if err := <-done; err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
}

func TestDelegateTool_ExecuteReturnsPerTaskErrors(t *testing.T) {
	wantErr := errors.New("subagent failed")
	runner := &mockSubagentRunner{
		result: map[string]any{"status": "error"},
		err:    wantErr,
	}
	tool := NewDelegateTool(runner)

	result, err := tool.Execute(context.Background(), delegateTasks(
		map[string]any{"agent": "explorer", "task": "Inspect docs."},
	))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	output := result.(map[string]any)
	results := output["results"].([]delegateResult)
	if len(results) != 1 || results[0].Error != wantErr.Error() {
		t.Fatalf("results = %#v, want per-task error", results)
	}
	if !reflect.DeepEqual(results[0].Result, runner.result) {
		t.Fatalf("result = %#v, want %#v", results[0].Result, runner.result)
	}
	if output["completed"] != 0 || output["failed"] != 1 {
		t.Fatalf("counts = completed:%v failed:%v, want 0 and 1", output["completed"], output["failed"])
	}
	if !reflect.DeepEqual(output["completed_by_agent"], map[string]int{}) {
		t.Fatalf("completed_by_agent = %#v, want empty", output["completed_by_agent"])
	}
	if !reflect.DeepEqual(output["failed_by_agent"], map[string]int{"explorer": 1}) {
		t.Fatalf("failed_by_agent = %#v, want explorer:1", output["failed_by_agent"])
	}
}

func TestDelegateTool_ValidateInputRejectsInvalidInput(t *testing.T) {
	tooMany := make([]any, maxDelegateTasks+1)
	for i := range tooMany {
		tooMany[i] = map[string]any{"agent": "explorer", "task": "Inspect docs."}
	}
	tests := []struct {
		name    string
		input   any
		wantErr string
	}{
		{name: "missing tasks", input: map[string]any{}, wantErr: `missing required "tasks" parameter`},
		{name: "empty tasks", input: delegateTasks(), wantErr: "at least one task"},
		{name: "too many tasks", input: map[string]any{"tasks": tooMany}, wantErr: "at most 10 tasks"},
		{name: "missing agent", input: delegateTasks(map[string]any{"task": "Inspect docs."}), wantErr: "tasks[0].agent"},
		{name: "missing task", input: delegateTasks(map[string]any{"agent": "explorer"}), wantErr: "tasks[0].task"},
		{name: "non-integer timeout", input: delegateTasks(map[string]any{"agent": "explorer", "task": "Inspect docs.", "timeout_seconds": "30"}), wantErr: "cannot unmarshal"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &mockSubagentRunner{}
			tool := NewDelegateTool(runner)

			err := tool.ValidateInput(context.Background(), tt.input)
			if err == nil {
				t.Fatal("ValidateInput() expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateInput() error = %v, want containing %q", err, tt.wantErr)
			}
			if len(runner.recordedCalls()) != 0 {
				t.Fatal("runner should not be called for invalid input")
			}
		})
	}
}

func TestDelegateTool_ExecuteRejectsMissingRunner(t *testing.T) {
	tool := NewDelegateTool(nil)

	_, err := tool.Execute(context.Background(), delegateTasks(
		map[string]any{"agent": "explorer", "task": "Inspect docs."},
	))
	if err == nil {
		t.Fatal("Execute() expected error")
	}
	if !strings.Contains(err.Error(), "subagent runner not configured") {
		t.Fatalf("Execute() error = %v, want runner configuration error", err)
	}
}

func delegateTasks(tasks ...map[string]any) map[string]any {
	items := make([]any, len(tasks))
	for i, task := range tasks {
		items[i] = task
	}
	return map[string]any{"tasks": items}
}
