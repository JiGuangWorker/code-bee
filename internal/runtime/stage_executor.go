// 本文件实现 StageExecutor，执行单个串行 stage。
//
// 执行流程:
// 1. 求值 stage.When，不满足则跳过
// 2. 通过 ToolRegistry 解析 tool
// 3. 计算结果文件路径并重置
// 4. 从 input_from stage 获取上游数据
// 5. 构造 Invocation 并执行 Tool
// 6. 处理 on_blocked 策略（stop/skip/continue）
// 7. 存储结果到 ExecutionContext
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-05

package runtime

import (
	"context"
	"fmt"

	"github.com/JiGuangWorker/code-bee/internal/schema"
)

// StageExecutor 执行单个 stage。
type StageExecutor struct {
	registry *ToolRegistry
}

// NewStageExecutor 创建 StageExecutor。
func NewStageExecutor(registry *ToolRegistry) *StageExecutor {
	return &StageExecutor{registry: registry}
}

// Execute 执行 stage 步骤。
func (e *StageExecutor) Execute(ctx context.Context, step schema.PipelineStep, ec *ExecutionContext) (*StepResult, error) {
	stage := step.Stage
	if stage == nil {
		return nil, fmt.Errorf("StageExecutor: step.Stage is nil")
	}

	// 1. 求值 when 条件
	matched, err := EvalCondition(ec, stage.When)
	if err != nil {
		return nil, fmt.Errorf("stage %s: when: %w", stage.Name, err)
	}
	if !matched {
		result := &StepResult{
			StageName: stage.Name,
			Success:   true,
			Skipped:   true,
		}
		ec.SetResult(stage.Name, result)
		return result, nil
	}

	// 2. 解析 tool
	tool, ok := e.registry.Resolve(stage.Tool)
	if !ok {
		return nil, fmt.Errorf("stage %s: tool %q not found", stage.Name, stage.Tool)
	}

	// 3. 计算结果文件路径
	if ec.Artifacts == nil {
		return nil, fmt.Errorf("stage %s: artifacts resolver is nil", stage.Name)
	}
	loopRound := 0
	if loop := ec.CurrentLoop(); loop != nil {
		loopRound = loop.Round
	}
	resultFilePath := ec.Artifacts.ResolveResultFile(stage.Name, stage.Args, loopRound)

	// 4. 重置结果文件
	if err := ec.Artifacts.ResetResultFile(resultFilePath); err != nil {
		return nil, fmt.Errorf("stage %s: reset result file: %w", stage.Name, err)
	}

	// 5. 获取 input_from 数据
	var inputFrom map[string]any
	if stage.InputFrom != "" {
		if val, ok := ec.Lookup(stage.InputFrom, ""); ok {
			if m, ok := val.(map[string]any); ok {
				inputFrom = m
			}
		}
	}

	// 6. 构造 Invocation
	inv := Invocation{
		StageName:      stage.Name,
		Tool:           tool.Definition(),
		InputFrom:      inputFrom,
		Args:           stage.Args,
		ResultFilePath: resultFilePath,
		Loop:           ec.CurrentLoop(),
		Platform:       ec.Platform,
		Config:         ec.Config,
		EC:             ec,
	}

	// 7. 执行 Tool
	result, err := tool.Execute(ctx, inv)
	if err != nil {
		if result == nil {
			result = &StepResult{StageName: stage.Name, Success: false}
		}
		ec.SetResult(stage.Name, result)
		return result, err
	}

	// 8. 确保 StageName 正确
	if result.StageName == "" {
		result.StageName = stage.Name
	}

	// 9. 处理 on_blocked
	if result.Blocked {
		onBlocked := stage.OnBlocked
		if onBlocked == "" {
			onBlocked = "stop"
		}
		switch onBlocked {
		case "skip":
			result.Skipped = true
			// 继续存储并返回
		case "continue":
			// 继续存储并返回
		case "stop":
			// 默认行为：存储后返回，上层 Engine 会终止
		default:
			// 未知值默认 stop
		}
	}

	// 10. 存储结果
	ec.SetResult(stage.Name, result)

	return result, nil
}
