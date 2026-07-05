// 本文件实现 LoopExecutor，执行循环编排原语。
//
// 执行流程:
// 1. 初始化 LoopState（max_iterations / max_consecutive_unknown / judge）
// 2. 每轮执行 body 中的 step 序列
// 3. 每步后检查 exit_when，命中则退出
// 4. 维护 ConsecutiveUnknown，超限则 ManualRequired 退出
// 5. 若配了 judge 且 round >= startRound，执行 judge 并按决策处理
// 6. 跑完 maxIterations 仍无退出 → ManualRequired
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-05

package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/JiGuangWorker/code-bee/internal/schema"
)

// LoopExecutor 执行循环。
type LoopExecutor struct {
	registry *ToolRegistry
	dispatch dispatchFunc
}

// NewLoopExecutor 创建 LoopExecutor。
func NewLoopExecutor(registry *ToolRegistry, dispatch dispatchFunc) *LoopExecutor {
	return &LoopExecutor{registry: registry, dispatch: dispatch}
}

// Execute 执行 loop 步骤。
func (e *LoopExecutor) Execute(ctx context.Context, step schema.PipelineStep, ec *ExecutionContext) (*StepResult, error) {
	loop := step.Loop
	if loop == nil {
		return nil, fmt.Errorf("LoopExecutor: step.Loop is nil")
	}

	// 1. 初始化 LoopState
	loopState := &LoopState{
		ID:                    loop.ID,
		MaxIterations:         loop.MaxIterations,
		MaxConsecutiveUnknown: loop.MaxConsecutiveUnknown,
		Judge:                 loop.Judge,
	}
	if loopState.MaxIterations == 0 {
		loopState.MaxIterations = 3
	}
	if loopState.MaxConsecutiveUnknown == 0 {
		loopState.MaxConsecutiveUnknown = 2
	}

	// 2. timeout
	if loop.Timeout != "" {
		timeout, err := time.ParseDuration(loop.Timeout)
		if err != nil {
			return nil, fmt.Errorf("loop %s: invalid timeout %q: %w", loop.ID, loop.Timeout, err)
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	ec.PushLoop(loopState)
	defer ec.PopLoop()

	var lastResult *StepResult

	for round := 1; round <= loopState.MaxIterations; round++ {
		loopState.Round = round

		// 3. 执行 body
		var err error
		lastResult, err = e.executeBody(ctx, loop, ec, lastResult)
		if err != nil {
			return lastResult, err
		}
		// executeBody 返回的终端结果需要立即退出
		if lastResult != nil && (lastResult.Completed || lastResult.Blocked) {
			return lastResult, nil
		}

		// 4. 维护 ConsecutiveUnknown
		if lastResult != nil && lastResult.Status == "UNKNOWN" {
			loopState.ConsecutiveUnknown++
		} else {
			loopState.ConsecutiveUnknown = 0
		}
		if loopState.ConsecutiveUnknown >= loopState.MaxConsecutiveUnknown {
			return &StepResult{
				Success:        true,
				ManualRequired: true,
				StageName:      "loop",
				Output:         fmt.Sprintf("loop %s: max consecutive unknown (%d) reached", loop.ID, loopState.MaxConsecutiveUnknown),
			}, nil
		}

		// 5. 执行 judge
		judgeResult, err := e.handleJudge(ctx, ec, loop, loopState, lastResult, round)
		if err != nil {
			return judgeResult, err
		}
		if judgeResult != nil {
			return judgeResult, nil
		}

		// 6. 更新 LastFeedback
		if lastResult != nil {
			fb := buildFeedbackFromResult(lastResult)
			if fb != "" {
				loopState.LastFeedback = fb
			}
		}
	}

	// 跑完 maxIterations 仍无退出
	return &StepResult{
		Success:        true,
		ManualRequired: true,
		StageName:      "loop",
		Output:         fmt.Sprintf("loop %s: max iterations (%d) reached", loop.ID, loopState.MaxIterations),
	}, nil
}

// executeBody 执行 loop body 中的所有 step，处理 blocked 和 exit_when。
func (e *LoopExecutor) executeBody(ctx context.Context, loop *schema.Loop, ec *ExecutionContext, lastResult *StepResult) (*StepResult, error) {
	for _, bodyStep := range loop.Body {
		r, err := e.dispatch(ctx, bodyStep, ec)
		if err != nil {
			if r == nil {
				r = &StepResult{Success: false, StageName: "loop"}
			}
			return r, err
		}
		if r != nil {
			lastResult = r
			if r.Blocked && bodyStep.Stage != nil {
				onBlocked := bodyStep.Stage.OnBlocked
				if onBlocked == "" {
					onBlocked = "stop"
				}
				if onBlocked == "stop" {
					return r, nil
				}
			}
		}

		if len(loop.ExitWhen) > 0 {
			matched, err := AnyExitConditionMatched(ec, loop.ExitWhen)
			if err != nil {
				return nil, fmt.Errorf("loop %s exit_when: %w", loop.ID, err)
			}
			if matched {
				return e.makeExitResult(lastResult, "exit_when matched"), nil
			}
		}
	}
	return lastResult, nil
}

// handleJudge 执行价值评估并处理决策。
// 返回 nil result 表示 CONTINUE；返回非 nil 表示 EXIT。
func (e *LoopExecutor) handleJudge(ctx context.Context, ec *ExecutionContext, loop *schema.Loop, loopState *LoopState, _ *StepResult, round int) (*StepResult, error) {
	if loop.Judge == nil || round < judgeStartRound(loop.Judge) {
		return nil, nil
	}

	judgeResult, err := e.runJudge(ctx, ec, loop.Judge, loopState)
	if err != nil {
		return judgeResult, err
	}
	if judgeResult == nil {
		return nil, nil
	}

	decision := getDecision(judgeResult)
	action := mapJudgeDecision(decision, loop.Judge.OnDecision)

	if action == "exit" {
		return &StepResult{
			Success:        true,
			ManualRequired: decision == "STOP_MANUAL",
			Blocked:        decision == "STOP_BLOCKED",
			StageName:      "loop",
			Output:         judgeResult.Output,
			Data:           judgeResult.Data,
		}, nil
	}

	if decision == "SHRINK_TASK" {
		fb := buildShrinkFeedback(judgeResult)
		if fb != "" {
			loopState.LastFeedback = fb + "\n\n" + loopState.LastFeedback
		}
	}
	return nil, nil
}

// runJudge 执行价值评估员。
//
// 通过 dispatch 执行，便于测试 mock。dispatch 会路由到 StageExecutor，
// StageExecutor 负责解析 tool、计算路径、调用 tool.Execute。
func (e *LoopExecutor) runJudge(ctx context.Context, ec *ExecutionContext, judge *schema.LoopJudge, _ *LoopState) (*StepResult, error) {
	judgeStep := schema.PipelineStep{
		Stage: &schema.Stage{
			Name: "loop-judge",
			Tool: judge.Tool,
		},
	}
	return e.dispatch(ctx, judgeStep, ec)
}

// makeExitResult 构造 exit_when 退出时的结果。
func (e *LoopExecutor) makeExitResult(lastResult *StepResult, reason string) *StepResult {
	if lastResult == nil {
		return &StepResult{
			Success:   true,
			Completed: true,
			StageName: "loop",
			Output:    reason,
		}
	}
	// 基于最后一个 stage 的结果构造
	return &StepResult{
		Success:   true,
		Completed: lastResult.Status == "PASS" || lastResult.Status == "POSTED",
		Blocked:   lastResult.Blocked,
		StageName: "loop",
		Output:    reason,
		Data:      lastResult.Data,
		Status:    lastResult.Status,
	}
}

// judgeStartRound 返回 judge 触发起始轮。
// StartRound <= 0 表示始终触发（返回 1）。
func judgeStartRound(judge *schema.LoopJudge) int {
	if judge.StartRound <= 0 {
		return 1
	}
	return judge.StartRound
}

// mapJudgeDecision 把 judge 的 decision 映射到 action（continue/exit）。
//
// on_decision 是用户自定义映射，未配置时用默认值:
// - CONTINUE/SHRINK_TASK → continue
// - STOP_MANUAL/STOP_BLOCKED → exit
func mapJudgeDecision(decision string, onDecision map[string]string) string {
	if action, ok := onDecision[decision]; ok {
		return action
	}

	switch decision {
	case "CONTINUE", "SHRINK_TASK":
		return "continue"
	case "STOP_MANUAL", "STOP_BLOCKED":
		return "exit"
	}
	return "continue"
}

// getDecision 从 judge 结果中提取 decision 字段。
func getDecision(result *StepResult) string {
	if result == nil || result.Data == nil {
		return ""
	}
	if v, ok := result.Data["decision"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// buildFeedbackFromResult 从结果中提取 next_action 作为下一轮反馈。
func buildFeedbackFromResult(result *StepResult) string {
	if result == nil || result.Data == nil {
		return ""
	}
	if v, ok := result.Data["next_action"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// buildShrinkFeedback 从 judge 结果中提取 reason 作为 SHRINK_TASK 反馈。
func buildShrinkFeedback(result *StepResult) string {
	if result == nil || result.Data == nil {
		return ""
	}
	if v, ok := result.Data["reason"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
