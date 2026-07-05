// 本文件实现 CommandTool，通过 exec.CommandContext 执行 shell 命令。
//
// 核心流程:
// 1. 用 tool.Run 作为 shell 命令
// 2. 注入 tool.Env 环境变量
// 3. 执行命令并捕获 stdout
// 4. 尝试把 stdout 解析为 JSON（如果失败则当作纯文本输出）
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-05

package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/JiGuangWorker/code-bee/internal/schema"
)

// execCommandContext 是 exec.CommandContext 的可替换别名，便于单元测试注入 mock。
var execCommandContext = exec.CommandContext

// CommandTool 通过 shell 命令执行工具。
type CommandTool struct {
	def *schema.Tool
}

// NewCommandTool 创建一个 CommandTool 实例。
func NewCommandTool(def *schema.Tool) (*CommandTool, error) {
	if def.Type != "command" {
		return nil, fmt.Errorf("CommandTool: tool type must be command, got %s", def.Type)
	}
	if def.Run == "" {
		return nil, fmt.Errorf("CommandTool: run required for tool %s", def.Name)
	}
	return &CommandTool{def: def}, nil
}

// Definition 返回工具的 schema 定义。
func (t *CommandTool) Definition() *schema.Tool {
	return t.def
}

// Execute 执行 shell 命令。
func (t *CommandTool) Execute(ctx context.Context, inv Invocation) (*StepResult, error) {
	cmd := execCommandContext(ctx, "sh", "-c", t.def.Run)

	// 注入环境变量
	if len(t.def.Env) > 0 {
		cmd.Env = append(cmd.Env, envSlice(t.def.Env)...)
	}

	output, err := cmd.CombinedOutput()
	out := string(output)

	result := &StepResult{
		StageName: inv.StageName,
		Output:    out,
	}

	if err != nil {
		result.Success = false
		return result, fmt.Errorf("CommandTool.Execute[%s]: %w\n%s", t.def.Name, err, out)
	}

	result.Success = true

	// 尝试解析 stdout 为 JSON
	if trimmed := strings.TrimSpace(out); trimmed != "" {
		var data map[string]any
		if jsonErr := json.Unmarshal([]byte(trimmed), &data); jsonErr == nil {
			result.Data = data
			result.Status = extractStatus(data)
			result.Blocked = result.Status == "BLOCKED"
		}
		// 非 JSON 输出不报错，Data 留空
	}

	return result, nil
}

// envSlice 把 map[string]string 转成 "KEY=VALUE" 切片。
func envSlice(env map[string]string) []string {
	out := make([]string, 0, len(env))
	for k, v := range env {
		out = append(out, k+"="+v)
	}
	return out
}
