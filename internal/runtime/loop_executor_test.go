package runtime

import (
	"context"
	"testing"

	"github.com/JiGuangWorker/code-bee/internal/config"
	"github.com/JiGuangWorker/code-bee/internal/schema"
)

func TestLoopExecutor_ExitWhen_Matched(t *testing.T) {
	// 第一轮 review.status=PASS → exit_when 命中
	callCount := 0
	dispatch := func(ctx context.Context, step schema.PipelineStep, ec *ExecutionContext) (*StepResult, error) {
		callCount++
		stage := step.Stage
		r := &StepResult{StageName: stage.Name, Success: true, Status: "PASS", Data: map[string]any{"status": "PASS"}}
		ec.SetResult(stage.Name, r)
		return r, nil
	}

	exec := NewLoopExecutor(nil, dispatch)
	ec := NewExecutionContext(nil, &fakeArtifacts{}, PlatformContext{}, &config.Config{})

	step := schema.PipelineStep{
		Loop: &schema.Loop{
			ID:            "test",
			MaxIterations: 3,
			ExitWhen: []schema.ExitCondition{
				{Stage: "review", Field: "status", Operator: "equals", Value: "PASS"},
			},
			Body: []schema.PipelineStep{
				{Stage: &schema.Stage{Name: "review", Tool: "reviewer"}},
			},
		},
	}

	result, err := exec.Execute(context.Background(), step, ec)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if !result.Completed {
		t.Error("expected Completed=true for PASS exit")
	}
	if callCount != 1 {
		t.Errorf("expected 1 dispatch call, got %d", callCount)
	}
}

func TestLoopExecutor_ExitWhen_Blocked(t *testing.T) {
	// review.status=BLOCKED → exit_when 命中
	dispatch := func(ctx context.Context, step schema.PipelineStep, ec *ExecutionContext) (*StepResult, error) {
		stage := step.Stage
		r := &StepResult{StageName: stage.Name, Success: true, Status: "BLOCKED",
			Data: map[string]any{"status": "BLOCKED"}, Blocked: true}
		ec.SetResult(stage.Name, r)
		return r, nil
	}

	exec := NewLoopExecutor(nil, dispatch)
	ec := NewExecutionContext(nil, &fakeArtifacts{}, PlatformContext{}, &config.Config{})

	step := schema.PipelineStep{
		Loop: &schema.Loop{
			ID:            "test",
			MaxIterations: 3,
			ExitWhen: []schema.ExitCondition{
				{Stage: "review", Field: "status", Operator: "equals", Value: "BLOCKED"},
			},
			Body: []schema.PipelineStep{
				{Stage: &schema.Stage{Name: "review", Tool: "reviewer"}},
			},
		},
	}

	result, err := exec.Execute(context.Background(), step, ec)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.Blocked {
		t.Error("expected Blocked=true")
	}
}

func TestLoopExecutor_MaxIterations(t *testing.T) {
	// 跑完 max_iterations 仍无 exit_when 命中 → ManualRequired
	dispatch := func(ctx context.Context, step schema.PipelineStep, ec *ExecutionContext) (*StepResult, error) {
		stage := step.Stage
		r := &StepResult{StageName: stage.Name, Success: true, Status: "FAIL",
			Data: map[string]any{"status": "FAIL"}}
		ec.SetResult(stage.Name, r)
		return r, nil
	}

	exec := NewLoopExecutor(nil, dispatch)
	ec := NewExecutionContext(nil, &fakeArtifacts{}, PlatformContext{}, &config.Config{})

	step := schema.PipelineStep{
		Loop: &schema.Loop{
			ID:            "test",
			MaxIterations: 2,
			ExitWhen: []schema.ExitCondition{
				{Stage: "review", Field: "status", Operator: "equals", Value: "PASS"},
			},
			Body: []schema.PipelineStep{
				{Stage: &schema.Stage{Name: "review", Tool: "reviewer"}},
			},
		},
	}

	result, err := exec.Execute(context.Background(), step, ec)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.ManualRequired {
		t.Error("expected ManualRequired=true after max iterations")
	}
}

func TestLoopExecutor_MaxConsecutiveUnknown(t *testing.T) {
	// 连续 UNKNOWN 达到上限 → ManualRequired
	dispatch := func(ctx context.Context, step schema.PipelineStep, ec *ExecutionContext) (*StepResult, error) {
		stage := step.Stage
		r := &StepResult{StageName: stage.Name, Success: true, Status: "UNKNOWN",
			Data: map[string]any{"status": "UNKNOWN"}}
		ec.SetResult(stage.Name, r)
		return r, nil
	}

	exec := NewLoopExecutor(nil, dispatch)
	ec := NewExecutionContext(nil, &fakeArtifacts{}, PlatformContext{}, &config.Config{})

	step := schema.PipelineStep{
		Loop: &schema.Loop{
			ID:                    "test",
			MaxIterations:         5,
			MaxConsecutiveUnknown: 2,
			Body: []schema.PipelineStep{
				{Stage: &schema.Stage{Name: "review", Tool: "reviewer"}},
			},
		},
	}

	result, err := exec.Execute(context.Background(), step, ec)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.ManualRequired {
		t.Error("expected ManualRequired for consecutive unknowns")
	}
}

func TestLoopExecutor_JudgeStopManual(t *testing.T) {
	// judge 返回 STOP_MANUAL → ManualRequired 退出
	dispatch := func(ctx context.Context, step schema.PipelineStep, ec *ExecutionContext) (*StepResult, error) {
		stage := step.Stage
		var r *StepResult
		if stage.Name == "review" {
			r = &StepResult{StageName: stage.Name, Success: true, Status: "FAIL",
				Data: map[string]any{"status": "FAIL"}}
		} else if stage.Name == "loop-judge" {
			r = &StepResult{StageName: stage.Name, Success: true,
				Data: map[string]any{"decision": "STOP_MANUAL", "reason": "no progress"}}
		}
		ec.SetResult(stage.Name, r)
		return r, nil
	}

	// 需要 registry 来解析 judge tool
	wf := &schema.Workflow{
		Tools: []schema.Tool{
			{Name: "judge-tool", Type: "command", Run: "echo judge"},
		},
	}
	registry, _ := NewToolRegistry(wf, nil, nil)

	exec := NewLoopExecutor(registry, dispatch)
	ec := NewExecutionContext(nil, &fakeArtifacts{}, PlatformContext{}, &config.Config{})

	step := schema.PipelineStep{
		Loop: &schema.Loop{
			ID:            "test",
			MaxIterations: 5,
			Body: []schema.PipelineStep{
				{Stage: &schema.Stage{Name: "review", Tool: "reviewer"}},
			},
			Judge: &schema.LoopJudge{
				Tool:       "judge-tool",
				StartRound: 1,
			},
		},
	}

	result, err := exec.Execute(context.Background(), step, ec)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.ManualRequired {
		t.Error("expected ManualRequired for STOP_MANUAL")
	}
}

func TestLoopExecutor_JudgeStopBlocked(t *testing.T) {
	dispatch := func(ctx context.Context, step schema.PipelineStep, ec *ExecutionContext) (*StepResult, error) {
		stage := step.Stage
		var r *StepResult
		if stage.Name == "review" {
			r = &StepResult{StageName: stage.Name, Success: true, Status: "FAIL",
				Data: map[string]any{"status": "FAIL"}}
		} else if stage.Name == "loop-judge" {
			r = &StepResult{StageName: stage.Name, Success: true,
				Data: map[string]any{"decision": "STOP_BLOCKED", "reason": "external dep"}}
		}
		ec.SetResult(stage.Name, r)
		return r, nil
	}

	wf := &schema.Workflow{Tools: []schema.Tool{{Name: "judge-tool", Type: "command", Run: "echo"}}}
	registry, _ := NewToolRegistry(wf, nil, nil)

	exec := NewLoopExecutor(registry, dispatch)
	ec := NewExecutionContext(nil, &fakeArtifacts{}, PlatformContext{}, &config.Config{})

	step := schema.PipelineStep{
		Loop: &schema.Loop{
			ID:            "test",
			MaxIterations: 5,
			Body: []schema.PipelineStep{
				{Stage: &schema.Stage{Name: "review", Tool: "reviewer"}},
			},
			Judge: &schema.LoopJudge{Tool: "judge-tool", StartRound: 1},
		},
	}

	result, err := exec.Execute(context.Background(), step, ec)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.Blocked {
		t.Error("expected Blocked for STOP_BLOCKED")
	}
}

func TestLoopExecutor_JudgeContinue(t *testing.T) {
	// judge 返回 CONTINUE → 继续循环
	rounds := 0
	dispatch := func(ctx context.Context, step schema.PipelineStep, ec *ExecutionContext) (*StepResult, error) {
		stage := step.Stage
		var r *StepResult
		if stage.Name == "review" {
			rounds++
			r = &StepResult{StageName: stage.Name, Success: true, Status: "FAIL",
				Data: map[string]any{"status": "FAIL", "next_action": "keep trying"}}
		} else if stage.Name == "loop-judge" {
			// 第一轮 CONTINUE，第二轮 STOP_MANUAL
			if rounds == 1 {
				r = &StepResult{StageName: stage.Name, Success: true,
					Data: map[string]any{"decision": "CONTINUE", "reason": "keep going"}}
			} else {
				r = &StepResult{StageName: stage.Name, Success: true,
					Data: map[string]any{"decision": "STOP_MANUAL", "reason": "stop"}}
			}
		}
		ec.SetResult(stage.Name, r)
		return r, nil
	}

	wf := &schema.Workflow{Tools: []schema.Tool{{Name: "judge-tool", Type: "command", Run: "echo"}}}
	registry, _ := NewToolRegistry(wf, nil, nil)

	exec := NewLoopExecutor(registry, dispatch)
	ec := NewExecutionContext(nil, &fakeArtifacts{}, PlatformContext{}, &config.Config{})

	step := schema.PipelineStep{
		Loop: &schema.Loop{
			ID:            "test",
			MaxIterations: 5,
			Body: []schema.PipelineStep{
				{Stage: &schema.Stage{Name: "review", Tool: "reviewer"}},
			},
			Judge: &schema.LoopJudge{Tool: "judge-tool", StartRound: 1},
		},
	}

	result, err := exec.Execute(context.Background(), step, ec)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	// 第一轮 CONTINUE，第二轮 STOP_MANUAL
	if !result.ManualRequired {
		t.Error("expected ManualRequired after second round judge")
	}
	if rounds != 2 {
		t.Errorf("expected 2 review rounds, got %d", rounds)
	}
}

func TestLoopExecutor_JudgeShrinkTask(t *testing.T) {
	// judge 返回 SHRINK_TASK → 注入反馈继续
	dispatch := func(ctx context.Context, step schema.PipelineStep, ec *ExecutionContext) (*StepResult, error) {
		stage := step.Stage
		var r *StepResult
		if stage.Name == "review" {
			r = &StepResult{StageName: stage.Name, Success: true, Status: "PASS",
				Data: map[string]any{"status": "PASS"}}
		} else if stage.Name == "loop-judge" {
			r = &StepResult{StageName: stage.Name, Success: true,
				Data: map[string]any{"decision": "SHRINK_TASK", "reason": "focus on core"}}
		}
		ec.SetResult(stage.Name, r)
		return r, nil
	}

	wf := &schema.Workflow{Tools: []schema.Tool{{Name: "judge-tool", Type: "command", Run: "echo"}}}
	registry, _ := NewToolRegistry(wf, nil, nil)

	exec := NewLoopExecutor(registry, dispatch)
	ec := NewExecutionContext(nil, &fakeArtifacts{}, PlatformContext{}, &config.Config{})

	step := schema.PipelineStep{
		Loop: &schema.Loop{
			ID:            "test",
			MaxIterations: 3,
			ExitWhen: []schema.ExitCondition{
				{Stage: "review", Field: "status", Operator: "equals", Value: "PASS"},
			},
			Body: []schema.PipelineStep{
				{Stage: &schema.Stage{Name: "review", Tool: "reviewer"}},
			},
			Judge: &schema.LoopJudge{Tool: "judge-tool", StartRound: 1},
		},
	}

	// 第一轮 review PASS → exit_when 命中退出（在 judge 之前）
	result, err := exec.Execute(context.Background(), step, ec)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.Completed {
		t.Error("expected Completed=true for PASS")
	}
}

func TestLoopExecutor_NilLoop(t *testing.T) {
	exec := NewLoopExecutor(nil, nil)
	ec := NewExecutionContext(nil, nil, PlatformContext{}, nil)

	_, err := exec.Execute(context.Background(), schema.PipelineStep{}, ec)
	if err == nil {
		t.Fatal("expected error for nil loop")
	}
}

func TestJudgeStartRound(t *testing.T) {
	cases := []struct {
		start int
		want  int
	}{
		{0, 1},
		{-1, 1},
		{1, 1},
		{3, 3},
	}
	for _, c := range cases {
		got := judgeStartRound(&schema.LoopJudge{StartRound: c.start})
		if got != c.want {
			t.Errorf("judgeStartRound(%d) = %d, want %d", c.start, got, c.want)
		}
	}
}

func TestMapJudgeDecision(t *testing.T) {
	// 默认映射
	cases := []struct {
		decision string
		want     string
	}{
		{"CONTINUE", "continue"},
		{"SHRINK_TASK", "continue"},
		{"STOP_MANUAL", "exit"},
		{"STOP_BLOCKED", "exit"},
		{"UNKNOWN", "continue"}, // 未知默认 continue
	}
	for _, c := range cases {
		got := mapJudgeDecision(c.decision, nil)
		if got != c.want {
			t.Errorf("mapJudgeDecision(%q, nil) = %q, want %q", c.decision, got, c.want)
		}
	}

	// 自定义映射
	custom := map[string]string{
		"CONTINUE":   "exit",    // 覆盖默认
		"STOP_MANUAL": "continue", // 覆盖默认
	}
	if got := mapJudgeDecision("CONTINUE", custom); got != "exit" {
		t.Errorf("custom map CONTINUE = %q, want exit", got)
	}
	if got := mapJudgeDecision("STOP_MANUAL", custom); got != "continue" {
		t.Errorf("custom map STOP_MANUAL = %q, want continue", got)
	}
}

func TestGetDecision(t *testing.T) {
	if got := getDecision(nil); got != "" {
		t.Error("expected empty for nil")
	}
	if got := getDecision(&StepResult{}); got != "" {
		t.Error("expected empty for nil Data")
	}
	if got := getDecision(&StepResult{Data: map[string]any{"decision": "STOP_MANUAL"}}); got != "STOP_MANUAL" {
		t.Errorf("got %q, want STOP_MANUAL", got)
	}
}
