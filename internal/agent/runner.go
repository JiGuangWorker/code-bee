// Package agent 管理编码智能体的调用。
//
// code-bee 只做一件事：生成任务指令，交给编码智能体执行。
// 智能体自行完成：读取 Issue、解析任务、编码、提交 PR。
package agent

import (
	"context"
	"fmt"
	"os/exec"
)

// RunResult 表示编码智能体的执行结果。
type RunResult struct {
	Success bool   // 是否成功
	Output  string // 执行输出
}

// Runner 管理编码智能体的调用。
type Runner struct{}

// New 创建编码智能体执行器。
func New() *Runner {
	return &Runner{}
}

// Run 调用编码智能体，传入任务指令。
// task 是一句自然语言任务描述，例如：
//
//	"请查看 owner/repo 仓库的 #42 Issue，并完成其中的编码任务。"
func (r *Runner) Run(ctx context.Context, task string) (*RunResult, error) {
	cmd := exec.CommandContext(ctx,
		"reasonix", "run", task,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return &RunResult{
			Success: false,
			Output:  string(output),
		}, fmt.Errorf("agent.Run: reasonix failed: %w\n%s", err, string(output))
	}

	return &RunResult{
		Success: true,
		Output:  string(output),
	}, nil
}
