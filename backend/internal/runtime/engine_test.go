package runtime

import (
	"context"
	"errors"
	"os/exec"
	"testing"

	"github.com/JiGuangWorker/code-bee/internal/config"
	"github.com/JiGuangWorker/code-bee/internal/schema"
)

func TestNewEngine_Success(t *testing.T) {
	wf := &schema.Workflow{
		Version: "1",
		Name:    "test",
		Tools: []schema.Tool{
			{Name: "agent-tool", Type: "agent", PromptTemplate: "coding", Skill: "s"},
		},
	}

	eng, err := NewEngine(wf, &scriptedRunner{})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if eng.workflow != wf {
		t.Error("workflow not set")
	}
	if eng.registry == nil {
		t.Error("registry not set")
	}
	if eng.stage == nil || eng.parallel == nil || eng.loop == nil {
		t.Error("executors not set")
	}
}

func TestNewEngine_NilWorkflow(t *testing.T) {
	_, err := NewEngine(nil, &scriptedRunner{})
	if err == nil {
		t.Fatal("expected error for nil workflow")
	}
}

func TestNewEngine_NilRunner(t *testing.T) {
	wf := &schema.Workflow{Tools: []schema.Tool{}}
	_, err := NewEngine(wf, nil)
	if err == nil {
		t.Fatal("expected error for nil runner")
	}
}

func TestEngine_Run_SimplePipeline(t *testing.T) {
	// 简单 pipeline：一个 stage，返回 PASS
	wf := &schema.Workflow{
		Version: "1",
		Tools: []schema.Tool{
			{Name: "test-tool", Type: "command", Run: "echo"},
		},
		Pipeline: []schema.PipelineStep{
			{Stage: &schema.Stage{Name: "test", Tool: "test-tool"}},
		},
	}

	eng, err := NewEngine(wf, &scriptedRunner{})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	// 用 mock artifacts 让 stage 返回 PASS
	fa := &fakeArtifacts{results: map[string]map[string]any{
		"test": {"status": "PASS"},
	}}

	// 但 command tool 会真的执行 echo，需要 mock exec
	oldExec := execCommandContext
	defer func() { execCommandContext = oldExec }()
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("true")
	}

	result, err := eng.Run(context.Background(), RunDependencies{
		Artifacts: fa,
		Platform:  PlatformContext{},
		Config:    &config.Config{},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
}

func TestEngine_Run_Blocked(t *testing.T) {
	// pipeline 返回 BLOCKED → 提前终止
	wf := &schema.Workflow{
		Version: "1",
		Tools: []schema.Tool{
			{Name: "test-tool", Type: "command", Run: "echo"},
		},
		Pipeline: []schema.PipelineStep{
			{Stage: &schema.Stage{Name: "test", Tool: "test-tool"}},
		},
	}

	eng, _ := NewEngine(wf, &scriptedRunner{})

	fa := &fakeArtifacts{}

	oldExec := execCommandContext
	defer func() { execCommandContext = oldExec }()
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("echo", `{"status":"BLOCKED"}`)
	}

	result, err := eng.Run(context.Background(), RunDependencies{
		Artifacts: fa,
		Config:    &config.Config{},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !result.Blocked {
		t.Error("expected Blocked=true")
	}
}

func TestEngine_Run_DispatchError(t *testing.T) {
	// 用一个 tool 不存在的 stage 触发 dispatch 错误
	wf := &schema.Workflow{
		Version: "1",
		Tools:   []schema.Tool{},
		Pipeline: []schema.PipelineStep{
			{Stage: &schema.Stage{Name: "test", Tool: "nonexistent"}},
		},
	}

	eng, _ := NewEngine(wf, &scriptedRunner{})

	_, err := eng.Run(context.Background(), RunDependencies{
		Artifacts: &fakeArtifacts{},
		Config:    &config.Config{},
	})
	if err == nil {
		t.Fatal("expected error for missing tool")
	}
}

func TestEngine_Run_EmptyPipeline(t *testing.T) {
	wf := &schema.Workflow{
		Version:  "1",
		Tools:    []schema.Tool{},
		Pipeline: []schema.PipelineStep{},
	}

	eng, _ := NewEngine(wf, &scriptedRunner{})

	result, err := eng.Run(context.Background(), RunDependencies{
		Artifacts: &fakeArtifacts{},
		Config:    &config.Config{},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !result.Success || !result.Completed {
		t.Error("expected Success and Completed for empty pipeline")
	}
}

func TestToEngineResult(t *testing.T) {
	// nil result
	r := toEngineResult(nil, errors.New("err"))
	if r.Success {
		t.Error("expected Success=false for nil result")
	}

	// 正常 result
	r = toEngineResult(&StepResult{
		Success:   true,
		Completed: true,
		Output:    "done",
	}, nil)
	if !r.Success || !r.Completed {
		t.Error("expected Success and Completed")
	}
	if r.Output != "done" {
		t.Errorf("got output %q, want done", r.Output)
	}
}
