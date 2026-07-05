// 本文件实现 Engine，Runtime 引擎的顶层入口。
//
// 职责:
// 1. 持有 workflow 配置和 ToolRegistry
// 2. 提供 Run 方法，遍历 workflow.Pipeline 并通过 dispatch 分发执行
// 3. 根据 StepResult 的 Blocked/ManualRequired/Completed 决定是否提前终止
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-05

package runtime

import (
	"context"
	"fmt"

	"github.com/JiGuangWorker/code-bee/internal/config"
	"github.com/JiGuangWorker/code-bee/internal/schema"
)

// Engine 是 Runtime 引擎的顶层入口。
type Engine struct {
	workflow *schema.Workflow
	registry *ToolRegistry
	pb       *PromptBuilder
	stage    *StageExecutor
	parallel *ParallelExecutor
	loop     *LoopExecutor
}

// NewEngine 创建 Engine 实例。
//
// 构造逻辑:
// 1. 加载 PromptBuilder（内置模板）
// 2. 根据 workflow.Tools 构建 ToolRegistry
// 3. 创建三种 Executor，通过 dispatch 函数支持嵌套
func NewEngine(wf *schema.Workflow, runner Runner) (*Engine, error) {
	if wf == nil {
		return nil, fmt.Errorf("runtime.NewEngine: workflow is nil")
	}
	if runner == nil {
		return nil, fmt.Errorf("runtime.NewEngine: runner is nil")
	}

	pb, err := NewPromptBuilder()
	if err != nil {
		return nil, fmt.Errorf("runtime.NewEngine: %w", err)
	}

	registry, err := NewToolRegistry(wf, runner, pb)
	if err != nil {
		return nil, fmt.Errorf("runtime.NewEngine: %w", err)
	}

	e := &Engine{
		workflow: wf,
		registry: registry,
		pb:       pb,
	}

	// dispatch 函数用于 ParallelExecutor 和 LoopExecutor 的嵌套分发
	dispatch := func(ctx context.Context, step schema.PipelineStep, ec *ExecutionContext) (*StepResult, error) {
		return e.dispatch(ctx, step, ec)
	}

	e.stage = NewStageExecutor(registry)
	e.parallel = NewParallelExecutor(dispatch)
	e.loop = NewLoopExecutor(registry, dispatch)

	return e, nil
}

// RunDependencies 是每次 Run 时需要的运行时依赖。
//
// 不同 issue 有不同的 ArtifactSet 和 Config，所以这些不放在 Engine 构造时。
type RunDependencies struct {
	Artifacts ArtifactResolver
	Platform  PlatformContext
	Config    *config.Config
}

// Run 执行 workflow，返回最终结果。
//
// 执行流程:
// 1. 创建 ExecutionContext
// 2. 遍历 workflow.Pipeline，逐个 dispatch
// 3. 遇到 Blocked/ManualRequired/Completed 时提前返回
func (e *Engine) Run(ctx context.Context, deps RunDependencies) (*EngineResult, error) {
	ec := NewExecutionContext(e.workflow, deps.Artifacts, deps.Platform, deps.Config)

	for _, step := range e.workflow.Pipeline {
		result, err := e.dispatch(ctx, step, ec)
		if err != nil {
			return toEngineResult(result, err), err
		}

		if result == nil {
			continue
		}

		// 遇到终止性状态提前返回
		if result.Blocked {
			return &EngineResult{
				Success: true,
				Blocked: true,
				Output:  result.Output,
			}, nil
		}

		if result.ManualRequired {
			return &EngineResult{
				Success:        true,
				ManualRequired: true,
				Output:         result.Output,
			}, nil
		}

		if result.Completed {
			return &EngineResult{
				Success:   true,
				Completed: true,
				Output:    result.Output,
			}, nil
		}
	}

	// 所有 step 执行完毕，无终止性状态
	return &EngineResult{
		Success:   true,
		Completed: true,
	}, nil
}

// dispatch 根据 step 类型选择 executor 并执行。
func (e *Engine) dispatch(ctx context.Context, step schema.PipelineStep, ec *ExecutionContext) (*StepResult, error) {
	exec := pickExecutor(step, e.stage, e.parallel, e.loop)
	if exec == nil {
		return nil, errInvalidStep
	}
	return exec.Execute(ctx, step, ec)
}

// toEngineResult 把 StepResult + error 转成 EngineResult。
func toEngineResult(r *StepResult, err error) *EngineResult {
	if r == nil {
		return &EngineResult{Success: false}
	}
	return &EngineResult{
		Success:        r.Success,
		Completed:      r.Completed,
		Blocked:        r.Blocked,
		ManualRequired: r.ManualRequired,
		Output:         r.Output,
	}
}
