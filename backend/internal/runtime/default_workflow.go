// 本文件提供内置默认 workflow 的加载入口。
//
// 设计说明:
// - default_workflow.yaml 通过 go:embed 内置到二进制，保证开箱即用
// - 用户可通过 --workflow flag 加载外部 YAML 覆盖默认配置
// - 加载时复用 schema.Loader 做完整的 JSON Schema + 语义校验
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-05

package runtime

import (
	_ "embed"
	"fmt"

	"github.com/JiGuangWorker/code-bee/internal/schema"
)

//go:embed default_workflow.yaml
var defaultWorkflowFS []byte

// LoadDefaultWorkflow 加载内置默认 workflow 配置。
//
// 返回值:
// - *schema.Workflow: 校验通过的 workflow 实例
// - error: 当内置 YAML 非法或语义校验失败时返回（属编程错误，正常不会发生）
func LoadDefaultWorkflow() (*schema.Workflow, error) {
	loader, err := schema.NewLoader()
	if err != nil {
		return nil, fmt.Errorf("runtime.LoadDefaultWorkflow: %w", err)
	}

	wf, err := loader.LoadFromBytes(defaultWorkflowFS)
	if err != nil {
		return nil, fmt.Errorf("runtime.LoadDefaultWorkflow: %w", err)
	}

	return wf, nil
}
