// Package agent 管理不同阶段智能体任务的调用。
//
// 核心功能:
// 1. 将 code-bee 生成的阶段化提示词交给外部智能体执行
// 2. 统一收集 Issue 处理、编码、审查、Issue 提交四个阶段的原始输出
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-04
// 更新时间: 2026-07-04
package agent

import (
	"context"
	"fmt"
	"os/exec"
)

// execCommandContext 是对 exec.CommandContext 的最小封装。
//
// 设计说明:
// - 通过可替换变量而不是在代码里写死 exec.CommandContext，便于单元测试注入伪命令执行器
// - 正式运行时保持默认值，不改变生产行为
var execCommandContext = exec.CommandContext

// RunResult 表示编码智能体的执行结果。
type RunResult struct {
	// Success 表示本次智能体调用是否成功完成。
	Success bool

	// Output 是智能体返回的原始文本输出，可直接用于发布评论或日志排障。
	Output string
}

// TaskKind 表示当前调度的任务阶段。
type TaskKind string

const (
	// TaskKindIssueHandling 表示“读取 Issue 并回复”的任务阶段。
	TaskKindIssueHandling TaskKind = "issue-handling"

	// TaskKindCoding 表示“编码实现/修复”的任务阶段。
	TaskKindCoding TaskKind = "coding"

	// TaskKindReview 表示“审查是否满足要求”的任务阶段。
	TaskKindReview TaskKind = "review"

	// TaskKindIssuePost 表示“校验结果文件并提交 Issue 评论”的任务阶段。
	TaskKindIssuePost TaskKind = "issue-post"

	// TaskKindLoopJudge 表示“评估继续自动循环是否还有价值”的任务阶段。
	TaskKindLoopJudge TaskKind = "loop-judge"
)

// Runner 管理编码智能体的调用。
type Runner struct{}

// New 创建编码智能体执行器。
//
// 返回值:
// - *Runner: 可用于执行回执生成、阻塞回复和正式编码任务的执行器
func New() *Runner {
	return &Runner{}
}

// Run 调用指定阶段的智能体任务并返回执行结果。
//
// 输入参数:
// - ctx: 控制外部命令生命周期的上下文
// - kind: 当前任务阶段，仅用于日志归因和错误信息增强
// - task: 传递给编码智能体的自然语言任务描述
//
// 返回值:
// - *RunResult: 封装执行是否成功以及智能体原始输出
// - error: 当 reasonix 命令执行失败时返回错误，并附带原始输出
//
// 核心逻辑:
// - 统一通过 reasonix run 执行所有阶段的提示词
// - 使用 CombinedOutput 保留完整现场，避免失败时无法定位问题
//
// 调用注意事项:
// - task 既可以是 Issue 处理任务，也可以是编码或审查任务
func (r *Runner) Run(ctx context.Context, kind TaskKind, task string) (*RunResult, error) {
	cmd := execCommandContext(ctx,
		"reasonix", "run", task,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		// 保留原始输出是为了让上层能够直接看到智能体失败现场，便于归因与重试。
		return &RunResult{
			Success: false,
			Output:  string(output),
		}, fmt.Errorf("agent.Run[%s]: reasonix failed: %w\n%s", kind, err, string(output))
	}

	return &RunResult{
		Success: true,
		Output:  string(output),
	}, nil
}
