// 本文件实现 ToolRegistry，管理 workflow 中定义的全部工具实例。
//
// 职责:
// 1. 根据 *schema.Workflow.Tools 构建 Tool 实例（agent/command/function）
// 2. 维护 name → Tool 和 alias → name 的映射
// 3. 对外提供 Resolve 方法，按 name 或 alias 查找 Tool
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-05

package runtime

import (
	"fmt"

	"github.com/JiGuangWorker/code-bee/internal/schema"
)

// ToolRegistry 管理全部 Tool 实例，支持 name 和 alias 查找。
type ToolRegistry struct {
	tools   map[string]Tool
	aliases map[string]string // alias → tool name
}

// NewToolRegistry 根据 workflow 配置构建所有 Tool 实例。
//
// 构建逻辑:
// - agent: 用 runner + promptBuilder 创建 AgentTool
// - command: 创建 CommandTool
// - function: 创建 FunctionTool（暂不支持，返回错误）
func NewToolRegistry(wf *schema.Workflow, runner Runner, pb *PromptBuilder) (*ToolRegistry, error) {
	if wf == nil {
		return nil, fmt.Errorf("ToolRegistry: workflow is nil")
	}

	registry := &ToolRegistry{
		tools:   make(map[string]Tool),
		aliases: make(map[string]string),
	}

	for i := range wf.Tools {
		def := &wf.Tools[i]

		if _, exists := registry.tools[def.Name]; exists {
			return nil, fmt.Errorf("ToolRegistry: duplicate tool name %q", def.Name)
		}

		var tool Tool
		var err error
		switch def.Type {
		case "agent":
			tool, err = NewAgentTool(def, runner, pb)
		case "command":
			tool, err = NewCommandTool(def)
		case "function":
			tool, err = NewFunctionTool(def)
		default:
			err = fmt.Errorf("unknown tool type %q for tool %s", def.Type, def.Name)
		}
		if err != nil {
			return nil, fmt.Errorf("ToolRegistry: build tool %s: %w", def.Name, err)
		}

		registry.tools[def.Name] = tool

		// 注册别名
		for _, alias := range def.Aliases {
			if alias == def.Name {
				continue // 别名与 name 重复时跳过
			}
			if existing, ok := registry.aliases[alias]; ok {
				return nil, fmt.Errorf("ToolRegistry: alias %q conflict between %s and %s", alias, existing, def.Name)
			}
			registry.aliases[alias] = def.Name
		}
	}

	return registry, nil
}

// Resolve 按 name 或 alias 查找 Tool。
func (r *ToolRegistry) Resolve(ref string) (Tool, bool) {
	// 先按 name 查
	if tool, ok := r.tools[ref]; ok {
		return tool, true
	}

	// 再按 alias 查
	if name, ok := r.aliases[ref]; ok {
		tool, ok := r.tools[name]
		return tool, ok
	}

	return nil, false
}

// MustResolve 按 name 或 alias 查找 Tool，找不到时 panic。
//
// 仅用于内部调用，外部调用应使用 Resolve。
func (r *ToolRegistry) MustResolve(ref string) Tool {
	tool, ok := r.Resolve(ref)
	if !ok {
		panic(fmt.Sprintf("ToolRegistry: tool %q not found", ref))
	}
	return tool
}

// Names 返回所有已注册的工具 name 列表。
func (r *ToolRegistry) Names() []string {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}
