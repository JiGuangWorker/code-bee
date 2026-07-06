// Package state 定义编排编辑器使用的内存状态模型。
//
// 核心功能:
// 1. 为编辑器提供比标准 YAML Schema 更直接的内存结构，便于画布、属性面板和保存操作消费
// 2. 在不引入运行时语义扩张的前提下，将 UI 坐标等展示元数据合并到节点状态中，降低上层调用复杂度
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-06
// 更新时间: 2026-07-06
package state

import "github.com/JiGuangWorker/code-bee/internal/editor/schema"

// State 表示编排编辑器在 Go 侧使用的最小状态结构。
//
// 输入参数:
// - 无；该结构由 FromSchema 生成，或由调用方在内存中编辑后交给 ToSchema 重新组装
//
// 返回值:
// - 无；作为编辑器状态容器承载元信息、节点、边、运行参数、角色与工件命名
//
// 核心实现逻辑:
// - 将 schema.Document 中分散在 workflow 与 ui 的信息拍平成更易编辑的结构
// - 节点列表直接携带位置坐标，避免上层每次都去 workflow / ui 两处拼装
//
// 调用注意事项:
// - 该结构只服务编辑器状态转换，不应用于现有调度主流程
type State struct {
	APIVersion  string
	Kind        string
	Metadata    MetadataState
	StartNodeID string
	Nodes       []NodeState
	Edges       []EdgeState
	Runtime     schema.Runtime
	Roles       schema.Roles
	Artifacts   schema.Artifacts
}

// MetadataState 表示编辑器状态中的元信息块。
type MetadataState struct {
	Name        string
	Description string
}

// NodeState 表示编辑器中的单个节点状态。
//
// 设计说明:
// - 运行语义字段继续保持与 schema.Node 对齐
// - Position 将原本独立放在 schema.UI 中的坐标信息收口到节点级，便于画布层直接消费
type NodeState struct {
	ID             string
	Type           string
	Name           string
	RoleRef        string
	PromptTemplate string
	ResultContract string
	Inputs         []string
	Outputs        []string
	Position       NodePosition
}

// NodePosition 表示节点在编辑器画布中的位置。
type NodePosition struct {
	X int
	Y int
}

// EdgeState 表示编辑器中的单条边状态。
type EdgeState struct {
	From      string
	To        string
	Condition schema.EdgeCondition
}
