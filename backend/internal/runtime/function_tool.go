// 本文件实现 FunctionTool，调用内置注册函数。
//
// 当前为桩实现，后续可扩展为函数注册表。
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-05

package runtime

import (
	"context"
	"fmt"

	"github.com/JiGuangWorker/code-bee/internal/schema"
)

// FunctionTool 调用内置函数。
type FunctionTool struct {
	def *schema.Tool
}

// NewFunctionTool 创建一个 FunctionTool 实例。
func NewFunctionTool(def *schema.Tool) (*FunctionTool, error) {
	if def.Type != "function" {
		return nil, fmt.Errorf("FunctionTool: tool type must be function, got %s", def.Type)
	}
	if def.Function == "" {
		return nil, fmt.Errorf("FunctionTool: function required for tool %s", def.Name)
	}
	return &FunctionTool{def: def}, nil
}

// Definition 返回工具的 schema 定义。
func (t *FunctionTool) Definition() *schema.Tool {
	return t.def
}

// Execute 执行内置函数。
//
// 当前未实现具体函数，返回错误。
func (t *FunctionTool) Execute(ctx context.Context, inv Invocation) (*StepResult, error) {
	return &StepResult{StageName: inv.StageName, Success: false},
		fmt.Errorf("FunctionTool.Execute: function %q not yet implemented", t.def.Function)
}
