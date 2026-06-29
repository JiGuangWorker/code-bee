// Package agent 管理 Docker 容器中的 Reasonix Agent 执行。
package agent

import (
	"context"
	"fmt"
	"os/exec"
)

// RunResult 表示 Agent 执行结果。
type RunResult struct {
	Success bool   // 是否成功
	Output  string // 执行输出
}

// Runner 管理 Docker Agent 的执行。
type Runner struct {
	agentName string
	image     string
}

// New 创建 Agent 执行器。
func New(agentName string) *Runner {
	return &Runner{
		agentName: agentName,
		image:     fmt.Sprintf("jiguang-agent-%s:latest", agentName),
	}
}

// Run 在 Docker 容器中执行 Reasonix。
// task 是自然语言描述的任务内容。
func (r *Runner) Run(ctx context.Context, task string) (*RunResult, error) {
	// V1 阶段：直接调用 reasonix run，后续替换为 Docker 方式
	cmd := exec.CommandContext(ctx,
		"reasonix", "run", task,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return &RunResult{
			Success: false,
			Output:  string(output),
		}, fmt.Errorf("agent.Run(%s): reasonix failed: %w\n%s", r.agentName, err, string(output))
	}

	return &RunResult{
		Success: true,
		Output:  string(output),
	}, nil
}
