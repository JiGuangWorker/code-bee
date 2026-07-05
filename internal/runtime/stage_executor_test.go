package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/JiGuangWorker/code-bee/internal/config"
	"github.com/JiGuangWorker/code-bee/internal/schema"
)

func newStageExecutorForTest(t *testing.T, template string) (*StageExecutor, *scriptedRunner, *ExecutionContext) {
	t.Helper()
	pb, _ := NewPromptBuilder()
	runner := &scriptedRunner{output: "ok"}
	wf := &schema.Workflow{
		Tools: []schema.Tool{
			{Name: "agent-tool", Type: "agent", PromptTemplate: template, Skill: "s/SKILL.md"},
		},
	}
	registry, err := NewToolRegistry(wf, runner, pb)
	if err != nil {
		t.Fatalf("NewToolRegistry: %v", err)
	}
	fa := &fakeArtifacts{results: map[string]map[string]any{}}
	ec := NewExecutionContext(wf, fa, PlatformContext{}, &config.Config{Repo: "o/r", IssueNumber: 1})
	return NewStageExecutor(registry), runner, ec
}

func TestStageExecutor_Execute_Success(t *testing.T) {
	exec, _, ec := newStageExecutorForTest(t, promptCoding)

	// 预置 issue-handling 结果
	ec.SetResult("issue-handling", &StepResult{
		Data: map[string]any{"agent": "dev", "summary": "s", "acceptance": "a"},
	})
	ec.Artifacts.(*fakeArtifacts).results["coding"] = map[string]any{"status": "DONE"}

	step := schema.PipelineStep{Stage: &schema.Stage{
		Name: "coding",
		Tool: "agent-tool",
	}}

	result, err := exec.Execute(context.Background(), step, ec)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if result.Status != "DONE" {
		t.Errorf("got status %q, want DONE", result.Status)
	}
	// 结果应存储到 EC
	got, ok := ec.GetResult("coding")
	if !ok {
		t.Fatal("expected coding result in EC")
	}
	if got.Status != "DONE" {
		t.Errorf("EC result status %s, want DONE", got.Status)
	}
}

func TestStageExecutor_Execute_WhenSkipped(t *testing.T) {
	exec, runner, ec := newStageExecutorForTest(t, promptCoding)

	// when 条件不满足（round < 2，但当前 round=1）
	ec.PushLoop(&LoopState{ID: "test", Round: 1, MaxIterations: 3})

	step := schema.PipelineStep{Stage: &schema.Stage{
		Name: "judge",
		Tool: "agent-tool",
		When: &schema.Condition{Expr: "round >= 2"},
	}}

	result, err := exec.Execute(context.Background(), step, ec)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.Skipped {
		t.Error("expected Skipped=true")
	}
	if len(runner.calls) != 0 {
		t.Errorf("runner should not be called, got %d calls", len(runner.calls))
	}
}

func TestStageExecutor_Execute_WhenMatched(t *testing.T) {
	exec, runner, ec := newStageExecutorForTest(t, promptCoding)
	ec.SetResult("issue-handling", &StepResult{
		Data: map[string]any{"agent": "dev", "summary": "s", "acceptance": "a"},
	})
	ec.Artifacts.(*fakeArtifacts).results["coding"] = map[string]any{"status": "DONE"}
	ec.PushLoop(&LoopState{ID: "test", Round: 2, MaxIterations: 3})

	step := schema.PipelineStep{Stage: &schema.Stage{
		Name: "coding",
		Tool: "agent-tool",
		When: &schema.Condition{Expr: "round >= 2"},
	}}

	result, err := exec.Execute(context.Background(), step, ec)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Skipped {
		t.Error("expected Skipped=false")
	}
	if len(runner.calls) != 1 {
		t.Errorf("runner should be called once, got %d", len(runner.calls))
	}
}

func TestStageExecutor_Execute_ToolNotFound(t *testing.T) {
	pb, _ := NewPromptBuilder()
	wf := &schema.Workflow{Tools: []schema.Tool{}}
	registry, _ := NewToolRegistry(wf, &scriptedRunner{}, pb)
	ec := NewExecutionContext(wf, &fakeArtifacts{}, PlatformContext{}, &config.Config{})

	exec := NewStageExecutor(registry)
	step := schema.PipelineStep{Stage: &schema.Stage{
		Name: "test",
		Tool: "nonexistent",
	}}

	_, err := exec.Execute(context.Background(), step, ec)
	if err == nil {
		t.Fatal("expected error for missing tool")
	}
}

func TestStageExecutor_Execute_OnBlockedStop(t *testing.T) {
	exec, _, ec := newStageExecutorForTest(t, promptCoding)
	ec.SetResult("issue-handling", &StepResult{
		Data: map[string]any{"agent": "dev", "summary": "s", "acceptance": "a"},
	})
	ec.Artifacts.(*fakeArtifacts).results["coding"] = map[string]any{"status": "BLOCKED"}

	step := schema.PipelineStep{Stage: &schema.Stage{
		Name:      "coding",
		Tool:      "agent-tool",
		OnBlocked: "stop",
	}}

	result, err := exec.Execute(context.Background(), step, ec)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.Blocked {
		t.Error("expected Blocked=true")
	}
	if result.Skipped {
		t.Error("expected Skipped=false for on_blocked=stop")
	}
}

func TestStageExecutor_Execute_OnBlockedSkip(t *testing.T) {
	exec, _, ec := newStageExecutorForTest(t, promptCoding)
	ec.SetResult("issue-handling", &StepResult{
		Data: map[string]any{"agent": "dev", "summary": "s", "acceptance": "a"},
	})
	ec.Artifacts.(*fakeArtifacts).results["coding"] = map[string]any{"status": "BLOCKED"}

	step := schema.PipelineStep{Stage: &schema.Stage{
		Name:      "coding",
		Tool:      "agent-tool",
		OnBlocked: "skip",
	}}

	result, err := exec.Execute(context.Background(), step, ec)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.Blocked {
		t.Error("expected Blocked=true")
	}
	if !result.Skipped {
		t.Error("expected Skipped=true for on_blocked=skip")
	}
}

func TestStageExecutor_Execute_OnBlockedDefault(t *testing.T) {
	exec, _, ec := newStageExecutorForTest(t, promptCoding)
	ec.SetResult("issue-handling", &StepResult{
		Data: map[string]any{"agent": "dev", "summary": "s", "acceptance": "a"},
	})
	ec.Artifacts.(*fakeArtifacts).results["coding"] = map[string]any{"status": "BLOCKED"}

	// 不设 on_blocked，默认应为 stop
	step := schema.PipelineStep{Stage: &schema.Stage{
		Name: "coding",
		Tool: "agent-tool",
	}}

	result, err := exec.Execute(context.Background(), step, ec)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.Blocked {
		t.Error("expected Blocked=true")
	}
	if result.Skipped {
		t.Error("expected Skipped=false for default on_blocked")
	}
}

func TestStageExecutor_Execute_RunnerError(t *testing.T) {
	pb, _ := NewPromptBuilder()
	runner := &scriptedRunner{err: errors.New("runner failed")}
	wf := &schema.Workflow{
		Tools: []schema.Tool{
			{Name: "agent-tool", Type: "agent", PromptTemplate: promptCoding, Skill: "s"},
		},
	}
	registry, _ := NewToolRegistry(wf, runner, pb)
	ec := NewExecutionContext(wf, &fakeArtifacts{}, PlatformContext{}, &config.Config{})

	exec := NewStageExecutor(registry)
	step := schema.PipelineStep{Stage: &schema.Stage{
		Name: "coding",
		Tool: "agent-tool",
	}}

	_, err := exec.Execute(context.Background(), step, ec)
	if err == nil {
		t.Fatal("expected error from runner")
	}
}

func TestStageExecutor_Execute_NilStage(t *testing.T) {
	exec, _, _ := newStageExecutorForTest(t, promptCoding)
	ec := NewExecutionContext(nil, &fakeArtifacts{}, PlatformContext{}, &config.Config{})

	step := schema.PipelineStep{}
	_, err := exec.Execute(context.Background(), step, ec)
	if err == nil {
		t.Fatal("expected error for nil stage")
	}
}

func TestPickExecutor(t *testing.T) {
	stageExec := &StageExecutor{}
	parallelExec := &ParallelExecutor{}
	loopExec := &LoopExecutor{}

	cases := []struct {
		step schema.PipelineStep
		want Executor
	}{
		{schema.PipelineStep{Stage: &schema.Stage{}}, stageExec},
		{schema.PipelineStep{Parallel: []schema.PipelineStep{{}}}, parallelExec},
		{schema.PipelineStep{Loop: &schema.Loop{}}, loopExec},
		{schema.PipelineStep{}, nil},
	}
	for _, c := range cases {
		got := pickExecutor(c.step, stageExec, parallelExec, loopExec)
		if got != c.want {
			t.Errorf("pickExecutor() = %T, want %T", got, c.want)
		}
	}
}
