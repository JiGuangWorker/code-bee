package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/JiGuangWorker/code-bee/internal/schema"
)

// makeDispatch 创建 mock dispatch 函数，按 stage name 返回预设结果。
func makeDispatch(results map[string]*StepResult) dispatchFunc {
	return func(ctx context.Context, step schema.PipelineStep, ec *ExecutionContext) (*StepResult, error) {
		if step.Stage == nil {
			return nil, errors.New("stage is nil")
		}
		name := step.Stage.Name
		r, ok := results[name]
		if !ok {
			return nil, errors.New("no result for " + name)
		}
		ec.SetResult(name, r)
		return r, nil
	}
}

func TestParallelExecutor_AllJoin_Success(t *testing.T) {
	dispatch := makeDispatch(map[string]*StepResult{
		"a": {StageName: "a", Success: true, Status: "PASS"},
		"b": {StageName: "b", Success: true, Status: "PASS"},
	})
	exec := NewParallelExecutor(dispatch)
	ec := NewExecutionContext(nil, &fakeArtifacts{}, PlatformContext{}, nil)

	step := schema.PipelineStep{
		Parallel: []schema.PipelineStep{
			{Stage: &schema.Stage{Name: "a", Tool: "t"}},
			{Stage: &schema.Stage{Name: "b", Tool: "t"}},
		},
		Join: "all",
	}

	result, err := exec.Execute(context.Background(), step, ec)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.Success {
		t.Error("expected success for all join")
	}
	if result.Status != "PASS" {
		t.Errorf("got status %q, want PASS", result.Status)
	}
}

func TestParallelExecutor_AllJoin_OneFails(t *testing.T) {
	dispatch := makeDispatch(map[string]*StepResult{
		"a": {StageName: "a", Success: true, Status: "PASS"},
		"b": {StageName: "b", Success: false, Status: "FAIL"},
	})
	exec := NewParallelExecutor(dispatch)
	ec := NewExecutionContext(nil, &fakeArtifacts{}, PlatformContext{}, nil)

	step := schema.PipelineStep{
		Parallel: []schema.PipelineStep{
			{Stage: &schema.Stage{Name: "a", Tool: "t"}},
			{Stage: &schema.Stage{Name: "b", Tool: "t"}},
		},
		Join: "all",
	}

	result, err := exec.Execute(context.Background(), step, ec)
	// all 模式下 b 失败不应返回 error（除非有 err），但应不 success
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure when one branch fails in all mode")
	}
}

func TestParallelExecutor_AnyJoin_OneSucceeds(t *testing.T) {
	dispatch := makeDispatch(map[string]*StepResult{
		"a": {StageName: "a", Success: false, Status: "FAIL"},
		"b": {StageName: "b", Success: true, Status: "PASS"},
	})
	exec := NewParallelExecutor(dispatch)
	ec := NewExecutionContext(nil, &fakeArtifacts{}, PlatformContext{}, nil)

	step := schema.PipelineStep{
		Parallel: []schema.PipelineStep{
			{Stage: &schema.Stage{Name: "a", Tool: "t"}},
			{Stage: &schema.Stage{Name: "b", Tool: "t"}},
		},
		Join: "any",
	}

	result, err := exec.Execute(context.Background(), step, ec)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.Success {
		t.Error("expected success for any join when one succeeds")
	}
}

func TestParallelExecutor_AnyJoin_AllFail(t *testing.T) {
	dispatch := makeDispatch(map[string]*StepResult{
		"a": {StageName: "a", Success: false, Status: "FAIL"},
		"b": {StageName: "b", Success: false, Status: "FAIL"},
	})
	exec := NewParallelExecutor(dispatch)
	ec := NewExecutionContext(nil, &fakeArtifacts{}, PlatformContext{}, nil)

	step := schema.PipelineStep{
		Parallel: []schema.PipelineStep{
			{Stage: &schema.Stage{Name: "a", Tool: "t"}},
			{Stage: &schema.Stage{Name: "b", Tool: "t"}},
		},
		Join: "any",
	}

	result, _ := exec.Execute(context.Background(), step, ec)
	if result.Success {
		t.Error("expected failure when all branches fail in any mode")
	}
}

func TestParallelExecutor_FirstSuccess(t *testing.T) {
	dispatch := makeDispatch(map[string]*StepResult{
		"a": {StageName: "a", Success: true, Status: "PASS"},
		"b": {StageName: "b", Success: true, Status: "PASS"},
	})
	exec := NewParallelExecutor(dispatch)
	ec := NewExecutionContext(nil, &fakeArtifacts{}, PlatformContext{}, nil)

	step := schema.PipelineStep{
		Parallel: []schema.PipelineStep{
			{Stage: &schema.Stage{Name: "a", Tool: "t"}},
			{Stage: &schema.Stage{Name: "b", Tool: "t"}},
		},
		Join: "first_success",
	}

	result, err := exec.Execute(context.Background(), step, ec)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.Success {
		t.Error("expected success for first_success")
	}
}

func TestParallelExecutor_WriteIsolation(t *testing.T) {
	dispatch := makeDispatch(map[string]*StepResult{
		"a": {StageName: "a", Success: true, Status: "PASS", Data: map[string]any{"from": "a"}},
		"b": {StageName: "b", Success: true, Status: "PASS", Data: map[string]any{"from": "b"}},
	})
	exec := NewParallelExecutor(dispatch)
	ec := NewExecutionContext(nil, &fakeArtifacts{}, PlatformContext{}, nil)

	step := schema.PipelineStep{
		Parallel: []schema.PipelineStep{
			{Stage: &schema.Stage{Name: "a", Tool: "t"}},
			{Stage: &schema.Stage{Name: "b", Tool: "t"}},
		},
		Join: "all",
	}

	_, err := exec.Execute(context.Background(), step, ec)
	if err != nil {
		t.Fatal(err)
	}

	// 两个分支的结果都应 Merge 回主 EC
	rA, ok := ec.GetResult("a")
	if !ok {
		t.Fatal("expected result a in EC after merge")
	}
	if rA.Data["from"] != "a" {
		t.Errorf("a.Data[from] = %v, want a", rA.Data["from"])
	}

	rB, ok := ec.GetResult("b")
	if !ok {
		t.Fatal("expected result b in EC after merge")
	}
	if rB.Data["from"] != "b" {
		t.Errorf("b.Data[from] = %v, want b", rB.Data["from"])
	}
}

func TestParallelExecutor_DefaultJoin(t *testing.T) {
	// 不设 join，默认应为 all
	dispatch := makeDispatch(map[string]*StepResult{
		"a": {StageName: "a", Success: true, Status: "PASS"},
		"b": {StageName: "b", Success: true, Status: "PASS"},
	})
	exec := NewParallelExecutor(dispatch)
	ec := NewExecutionContext(nil, &fakeArtifacts{}, PlatformContext{}, nil)

	step := schema.PipelineStep{
		Parallel: []schema.PipelineStep{
			{Stage: &schema.Stage{Name: "a", Tool: "t"}},
			{Stage: &schema.Stage{Name: "b", Tool: "t"}},
		},
	}

	result, err := exec.Execute(context.Background(), step, ec)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Error("expected success for default (all) join")
	}
}

func TestParallelExecutor_NoBranches(t *testing.T) {
	exec := NewParallelExecutor(makeDispatch(nil))
	ec := NewExecutionContext(nil, &fakeArtifacts{}, PlatformContext{}, nil)

	step := schema.PipelineStep{Parallel: []schema.PipelineStep{}}

	_, err := exec.Execute(context.Background(), step, ec)
	if err == nil {
		t.Fatal("expected error for empty parallel")
	}
}
