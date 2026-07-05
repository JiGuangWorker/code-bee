// 本文件实现 ParallelExecutor，并行执行多个分支。
//
// 执行流程:
// 1. 拍摄基线快照，每个分支用 CloneForBranch 创建独立写空间
// 2. 用 WaitGroup + context.WithCancel 并发执行所有分支
// 3. 按 join 策略汇合:
//    - all: 全成功才成功
//    - any: 任一成功即成功
//    - first_success: 首个成功后 cancel 其他
// 4. 成功分支的结果 Merge 回主 EC
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-05

package runtime

import (
	"context"
	"fmt"
	"sync"

	"github.com/JiGuangWorker/code-bee/internal/schema"
)

// dispatchFunc 是 step 分发函数，用于处理嵌套编排原语。
type dispatchFunc func(ctx context.Context, step schema.PipelineStep, ec *ExecutionContext) (*StepResult, error)

// ParallelExecutor 并行执行多个分支。
type ParallelExecutor struct {
	dispatch dispatchFunc
}

// NewParallelExecutor 创建 ParallelExecutor。
func NewParallelExecutor(dispatch dispatchFunc) *ParallelExecutor {
	return &ParallelExecutor{dispatch: dispatch}
}

// Execute 并行执行所有分支。
func (e *ParallelExecutor) Execute(ctx context.Context, step schema.PipelineStep, ec *ExecutionContext) (*StepResult, error) {
	branches := step.Parallel
	if len(branches) == 0 {
		return nil, fmt.Errorf("ParallelExecutor: no branches")
	}

	join := step.Join
	if join == "" {
		join = "all"
	}

	// 基线快照
	base := ec.Snapshot()

	// 并发执行
	results := make([]*StepResult, len(branches))
	errs := make([]error, len(branches))

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	var firstSuccessMu sync.Mutex
	firstSuccessIdx := -1

	for i, branch := range branches {
		i, branch := i, branch

		wg.Add(1)
		go func() {
			defer wg.Done()
			branchEC := ec.CloneForBranch(base)
			r, err := e.dispatch(ctx, branch, branchEC)
			results[i] = r
			errs[i] = err

			// first_success: 首个成功后 cancel 其他
			if join == "first_success" && err == nil && r != nil && r.Success && !r.Skipped {
				firstSuccessMu.Lock()
				if firstSuccessIdx < 0 {
					firstSuccessIdx = i
					cancel() // 取消其他分支
				}
				firstSuccessMu.Unlock()
			}
		}()
	}
	wg.Wait()

	// 按 join 策略汇合
	return e.joinResults(results, errs, join, ec, base, firstSuccessIdx)
}

// joinResults 按 join 策略汇合分支结果。
func (e *ParallelExecutor) joinResults(
	results []*StepResult,
	errs []error,
	join string,
	ec *ExecutionContext,
	_ map[string]*StepResult,
	firstSuccessIdx int,
) (*StepResult, error) {
	// 收集成功分支
	var successBranches []int
	var failedErrs []error
	for i, r := range results {
		if errs[i] != nil {
			failedErrs = append(failedErrs, errs[i])
			continue
		}
		if r != nil && r.Success {
			successBranches = append(successBranches, i)
		}
	}

	// 判断整体成功/失败
	overallSuccess := false
	switch join {
	case "all":
		overallSuccess = len(successBranches) == len(results) && len(failedErrs) == 0
	case "any":
		overallSuccess = len(successBranches) > 0
	case "first_success":
		overallSuccess = firstSuccessIdx >= 0
	default:
		overallSuccess = len(successBranches) == len(results) && len(failedErrs) == 0
	}

	// 即使失败也尝试 Merge 成功分支的结果（便于后续诊断）
	for _, i := range successBranches {
		if results[i] != nil && results[i].StageName != "" {
			ec.SetResult(results[i].StageName, results[i])
		}
	}

	// 构造聚合结果
	agg := &StepResult{
		StageName: "parallel",
		Success:   overallSuccess,
	}

	if !overallSuccess {
		// 收集错误信息
		if len(failedErrs) > 0 {
			return agg, fmt.Errorf("parallel join=%s: %d branches failed, first error: %w",
				join, len(failedErrs), failedErrs[0])
		}
		// 没有错误但没成功（如 all 模式下某些分支 !Success）
		return agg, nil
	}

	// 成功：聚合 status
	agg.Status = aggregateStatus(results, join, firstSuccessIdx, successBranches)

	return agg, nil
}

// aggregateStatus 按 join 策略聚合多个分支的 status。
func aggregateStatus(results []*StepResult, join string, firstSuccessIdx int, successBranches []int) string {
	if join == "all" {
		for _, r := range results {
			if r == nil || r.Status != "PASS" {
				if r != nil && r.Status != "" && r.Status != "PASS" {
					return ""
				}
			}
		}
		if len(results) > 0 {
			return "PASS"
		}
		return ""
	}
	if firstSuccessIdx >= 0 && results[firstSuccessIdx] != nil {
		return results[firstSuccessIdx].Status
	}
	if len(successBranches) > 0 && results[successBranches[0]] != nil {
		return results[successBranches[0]].Status
	}
	return ""
}
