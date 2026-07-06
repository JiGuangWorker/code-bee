// Package state 覆盖 schema 与 editor state 的双向映射逻辑。
//
// 核心功能:
// 1. 验证标准 schema 能稳定映射为编辑器状态，并可重新组装回合法 schema
// 2. 验证 UI 坐标等展示元数据与节点语义边界清晰，避免互相污染
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-06
// 更新时间: 2026-07-06
package state

import (
	"testing"

	"github.com/JiGuangWorker/code-bee/internal/editor/schema"
)

// TestFromSchemaAndToSchemaRoundTrip 验证默认 schema 能稳定完成 state 双向 round-trip。
func TestFromSchemaAndToSchemaRoundTrip(t *testing.T) {
	sourceDocument := schema.BuildDefaultDocument()

	editorState, err := FromSchema(sourceDocument)
	if err != nil {
		t.Fatalf("FromSchema() unexpected error: %v", err)
	}

	rebuiltDocument, err := ToSchema(editorState)
	if err != nil {
		t.Fatalf("ToSchema() unexpected error: %v", err)
	}

	if err := schema.Validate(rebuiltDocument); err != nil {
		t.Fatalf("schema.Validate(ToSchema(FromSchema(...))) unexpected error: %v", err)
	}

	if rebuiltDocument.Workflow.StartNodeID != sourceDocument.Workflow.StartNodeID {
		t.Fatalf(
			"Workflow.StartNodeID = %q, want %q",
			rebuiltDocument.Workflow.StartNodeID,
			sourceDocument.Workflow.StartNodeID,
		)
	}

	if len(rebuiltDocument.Workflow.Nodes) != len(sourceDocument.Workflow.Nodes) {
		t.Fatalf(
			"len(Workflow.Nodes) = %d, want %d",
			len(rebuiltDocument.Workflow.Nodes),
			len(sourceDocument.Workflow.Nodes),
		)
	}

	if len(rebuiltDocument.Workflow.Edges) != len(sourceDocument.Workflow.Edges) {
		t.Fatalf(
			"len(Workflow.Edges) = %d, want %d",
			len(rebuiltDocument.Workflow.Edges),
			len(sourceDocument.Workflow.Edges),
		)
	}
}

// TestFromSchemaMergesNodeUIIntoPosition 验证 schema.UI 中的坐标会合并进节点状态。
func TestFromSchemaMergesNodeUIIntoPosition(t *testing.T) {
	sourceDocument := schema.BuildDefaultDocument()

	editorState, err := FromSchema(sourceDocument)
	if err != nil {
		t.Fatalf("FromSchema() unexpected error: %v", err)
	}

	for _, node := range editorState.Nodes {
		if node.ID == schema.DefaultCodingNodeID {
			if node.Position.X != 620 || node.Position.Y != 120 {
				t.Fatalf("coding_main position = (%d,%d), want (620,120)", node.Position.X, node.Position.Y)
			}
			return
		}
	}

	t.Fatal("coding_main node not found in editor state")
}

// TestToSchemaWritesNodePositionBackToUI 验证节点位置修改后只回写到 schema.UI。
func TestToSchemaWritesNodePositionBackToUI(t *testing.T) {
	editorState, err := FromSchema(schema.BuildDefaultDocument())
	if err != nil {
		t.Fatalf("FromSchema() unexpected error: %v", err)
	}

	for nodeIndex := range editorState.Nodes {
		if editorState.Nodes[nodeIndex].ID == schema.DefaultReviewNodeID {
			editorState.Nodes[nodeIndex].Position = NodePosition{X: 1000, Y: 480}
			break
		}
	}

	rebuiltDocument, err := ToSchema(editorState)
	if err != nil {
		t.Fatalf("ToSchema() unexpected error: %v", err)
	}

	reviewUI, exists := rebuiltDocument.UI.Nodes[schema.DefaultReviewNodeID]
	if !exists {
		t.Fatalf("UI.Nodes[%q] not found", schema.DefaultReviewNodeID)
	}

	if reviewUI.X != 1000 || reviewUI.Y != 480 {
		t.Fatalf("UI position = (%d,%d), want (1000,480)", reviewUI.X, reviewUI.Y)
	}

	for _, node := range rebuiltDocument.Workflow.Nodes {
		if node.ID == schema.DefaultReviewNodeID {
			if node.Type != schema.NodeTypeReview {
				t.Fatalf("review node type = %q, want %q", node.Type, schema.NodeTypeReview)
			}
			return
		}
	}

	t.Fatal("review_main node not found in rebuilt schema")
}

// TestToSchemaRejectsUnknownStartNodeID 验证非法 startNodeId 会在状态层被阻断。
func TestToSchemaRejectsUnknownStartNodeID(t *testing.T) {
	editorState, err := FromSchema(schema.BuildDefaultDocument())
	if err != nil {
		t.Fatalf("FromSchema() unexpected error: %v", err)
	}

	editorState.StartNodeID = "ghost"

	if _, err := ToSchema(editorState); err == nil {
		t.Fatal("ToSchema() error = nil, want error for unknown start node")
	}
}

// TestToSchemaRejectsDuplicateNodeID 验证重复节点 ID 会在状态层被阻断。
func TestToSchemaRejectsDuplicateNodeID(t *testing.T) {
	editorState, err := FromSchema(schema.BuildDefaultDocument())
	if err != nil {
		t.Fatalf("FromSchema() unexpected error: %v", err)
	}

	editorState.Nodes[1].ID = editorState.Nodes[0].ID

	if _, err := ToSchema(editorState); err == nil {
		t.Fatal("ToSchema() error = nil, want error for duplicate node id")
	}
}

// TestToSchemaRejectsInvalidNodeSemanticField 验证节点语义字段仍会通过 schema.Validate 被阻断。
func TestToSchemaRejectsInvalidNodeSemanticField(t *testing.T) {
	editorState, err := FromSchema(schema.BuildDefaultDocument())
	if err != nil {
		t.Fatalf("FromSchema() unexpected error: %v", err)
	}

	editorState.Nodes[0].RoleRef = "randomAgent"

	if _, err := ToSchema(editorState); err == nil {
		t.Fatal("ToSchema() error = nil, want error for invalid node semantic field")
	}
}
