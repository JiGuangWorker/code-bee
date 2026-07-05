// 本文件定义 Executor 接口，三种编排原语的执行器都实现此接口。
//
// Executor 是统一的执行入口，Engine 根据 PipelineStep 类型选择对应执行器。
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-05

package runtime

import (
	"context"
	"fmt"

	"github.com/JiGuangWorker/code-bee/internal/schema"
)

// Executor 是编排原语执行器的统一接口。
type Executor interface {
	// Execute 执行一个 pipeline step，返回执行结果。
	Execute(ctx context.Context, step schema.PipelineStep, ec *ExecutionContext) (*StepResult, error)
}

// pickExecutor 根据 PipelineStep 的类型选择对应的执行器。
//
// 返回 nil 表示 step 配置无效（三个字段都为空）。
func pickExecutor(step schema.PipelineStep, stage *StageExecutor, parallel *ParallelExecutor, loop *LoopExecutor) Executor {
	switch {
	case step.Stage != nil:
		return stage
	case len(step.Parallel) > 0:
		return parallel
	case step.Loop != nil:
		return loop
	}
	return nil
}

// errInvalidStep 表示 pipeline step 配置无效。
var errInvalidStep = fmt.Errorf("pipeline step has no stage/parallel/loop")
