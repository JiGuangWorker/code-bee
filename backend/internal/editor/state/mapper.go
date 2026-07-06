// Package state 提供 schema 与 editor state 之间的双向映射逻辑。
//
// 核心功能:
// 1. 将标准 YAML Schema 映射为画布和属性面板更容易直接消费的内存状态
// 2. 将编辑器状态重新组装为标准 YAML Schema，供保存和后续模块复用
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-06
// 更新时间: 2026-07-06
package state

import (
	"fmt"

	"github.com/JiGuangWorker/code-bee/internal/editor/schema"
)

// FromSchema 将标准 YAML Schema 文档映射为编辑器状态。
//
// 输入参数:
// - doc: 已通过 schema.Validate 的标准文档对象
//
// 返回值:
// - *State: 可直接供编辑器画布和属性面板消费的最小状态对象
// - error: 当输入文档为空或 schema 校验失败时返回错误
//
// 核心实现逻辑:
// - 先复用 schema.Validate 兜底结构合法性
// - 将 workflow 节点和 ui 节点位置合并到单个 NodeState 中
// - 边、runtime、roles、artifacts 保持一一映射，不新增额外运行语义
//
// 调用注意事项:
// - 若 schema 文档未提供某个节点的 UI 坐标，则默认使用零值位置
func FromSchema(doc *schema.Document) (*State, error) {
	if err := schema.Validate(doc); err != nil {
		return nil, fmt.Errorf("map schema to editor state: %w", err)
	}

	state := &State{
		APIVersion: doc.APIVersion,
		Kind:       doc.Kind,
		Metadata: MetadataState{
			Name:        doc.Metadata.Name,
			Description: doc.Metadata.Description,
		},
		StartNodeID: doc.Workflow.StartNodeID,
		Nodes:       make([]NodeState, 0, len(doc.Workflow.Nodes)),
		Edges:       make([]EdgeState, 0, len(doc.Workflow.Edges)),
		Runtime:     doc.Runtime,
		Roles:       doc.Roles,
		Artifacts:   doc.Artifacts,
	}

	for _, node := range doc.Workflow.Nodes {
		position := NodePosition{}
		if nodeUI, exists := doc.UI.Nodes[node.ID]; exists {
			position = NodePosition{
				X: nodeUI.X,
				Y: nodeUI.Y,
			}
		}

		state.Nodes = append(state.Nodes, NodeState{
			ID:             node.ID,
			Type:           node.Type,
			Name:           node.Name,
			RoleRef:        node.RoleRef,
			PromptTemplate: node.PromptTemplate,
			ResultContract: node.ResultContract,
			Inputs:         cloneStrings(node.Inputs),
			Outputs:        cloneStrings(node.Outputs),
			Position:       position,
		})
	}

	for _, edge := range doc.Workflow.Edges {
		state.Edges = append(state.Edges, EdgeState{
			From:      edge.From,
			To:        edge.To,
			Condition: edge.When,
		})
	}

	return state, nil
}

// ToSchema 将编辑器状态重新组装为标准 YAML Schema 文档。
//
// 输入参数:
// - editorState: 编辑器侧的最小状态对象
//
// 返回值:
// - *schema.Document: 可直接用于 schema.Validate、EncodeYAML、SaveFile 的标准文档
// - error: 当状态为空、节点定义非法或组装后 schema 校验失败时返回错误
//
// 核心实现逻辑:
// - 先校验状态的最小完整性，尽早阻断明显错误
// - 将 NodeState 中的 Position 拆回 schema.UI，并把节点语义部分拆回 workflow.nodes
// - 最终复用 schema.Validate，保证输出一定仍符合标准 YAML 契约
//
// 调用注意事项:
// - ToSchema 不会对节点和边重新排序，保持调用方当前状态顺序
func ToSchema(editorState *State) (*schema.Document, error) {
	if err := validateState(editorState); err != nil {
		return nil, fmt.Errorf("map editor state to schema: %w", err)
	}

	doc := &schema.Document{
		APIVersion: editorState.APIVersion,
		Kind:       editorState.Kind,
		Metadata: schema.Metadata{
			Name:        editorState.Metadata.Name,
			Description: editorState.Metadata.Description,
		},
		Workflow: schema.Workflow{
			StartNodeID: editorState.StartNodeID,
			Nodes:       make([]schema.Node, 0, len(editorState.Nodes)),
			Edges:       make([]schema.Edge, 0, len(editorState.Edges)),
		},
		Runtime:   editorState.Runtime,
		Roles:     editorState.Roles,
		Artifacts: editorState.Artifacts,
		UI: schema.UISettings{
			Nodes: make(map[string]schema.NodeUI, len(editorState.Nodes)),
		},
	}

	for _, node := range editorState.Nodes {
		doc.Workflow.Nodes = append(doc.Workflow.Nodes, schema.Node{
			ID:             node.ID,
			Type:           node.Type,
			Name:           node.Name,
			RoleRef:        node.RoleRef,
			PromptTemplate: node.PromptTemplate,
			ResultContract: node.ResultContract,
			Inputs:         cloneStrings(node.Inputs),
			Outputs:        cloneStrings(node.Outputs),
		})

		doc.UI.Nodes[node.ID] = schema.NodeUI{
			X: node.Position.X,
			Y: node.Position.Y,
		}
	}

	for _, edge := range editorState.Edges {
		doc.Workflow.Edges = append(doc.Workflow.Edges, schema.Edge{
			From: edge.From,
			To:   edge.To,
			When: edge.Condition,
		})
	}

	if err := schema.Validate(doc); err != nil {
		return nil, fmt.Errorf("map editor state to schema: %w", err)
	}

	return doc, nil
}

// validateState 校验 editor state 的最小完整性。
func validateState(editorState *State) error {
	if editorState == nil {
		return fmt.Errorf("nil editor state")
	}

	if editorState.APIVersion == "" {
		return fmt.Errorf("empty state apiVersion")
	}

	if editorState.Kind == "" {
		return fmt.Errorf("empty state kind")
	}

	if editorState.Metadata.Name == "" {
		return fmt.Errorf("empty state metadata.name")
	}

	if editorState.StartNodeID == "" {
		return fmt.Errorf("empty state startNodeId")
	}

	if len(editorState.Nodes) == 0 {
		return fmt.Errorf("empty state nodes")
	}

	nodeIndex := make(map[string]struct{}, len(editorState.Nodes))
	for nodeIndexValue, node := range editorState.Nodes {
		if node.ID == "" {
			return fmt.Errorf("empty state nodes[%d].id", nodeIndexValue)
		}

		if _, exists := nodeIndex[node.ID]; exists {
			return fmt.Errorf("duplicate state node id %q", node.ID)
		}

		nodeIndex[node.ID] = struct{}{}
	}

	if _, exists := nodeIndex[editorState.StartNodeID]; !exists {
		return fmt.Errorf("unknown state startNodeId %q", editorState.StartNodeID)
	}

	return nil
}

// cloneStrings 复制字符串切片，避免上游状态和下游 schema 共享底层数组。
func cloneStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	clonedValues := make([]string, len(values))
	copy(clonedValues, values)

	return clonedValues
}
