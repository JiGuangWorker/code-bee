// Package schema 覆盖标准 YAML Schema 的解析、组装与静态校验逻辑。
//
// 核心功能:
// 1. 验证标准 YAML 能稳定解析为 Document，并能重新编码回合法 YAML
// 2. 验证非法枚举、非法引用、非法条件会在解析或编码阶段被及时阻断
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-06
// 更新时间: 2026-07-06
package schema

import (
	"path/filepath"
	"testing"
)

// TestDecodeYAMLAndEncodeYAMLRoundTrip 验证标准 YAML 能稳定完成解析与组装闭环。
func TestDecodeYAMLAndEncodeYAMLRoundTrip(t *testing.T) {
	doc, err := DecodeYAML(mustEncodeDefaultSchemaYAML(t))
	if err != nil {
		t.Fatalf("DecodeYAML() unexpected error: %v", err)
	}

	content, err := EncodeYAML(doc)
	if err != nil {
		t.Fatalf("EncodeYAML() unexpected error: %v", err)
	}

	decodedAgain, err := DecodeYAML(content)
	if err != nil {
		t.Fatalf("DecodeYAML(encoded content) unexpected error: %v", err)
	}

	if decodedAgain.Workflow.StartNodeID != "issue_handling" {
		t.Fatalf("Workflow.StartNodeID = %q, want %q", decodedAgain.Workflow.StartNodeID, "issue_handling")
	}

	if len(decodedAgain.Workflow.Nodes) != 6 {
		t.Fatalf("len(Workflow.Nodes) = %d, want %d", len(decodedAgain.Workflow.Nodes), 6)
	}
}

// TestDecodeYAMLRejectsUnknownField 验证未知字段会被严格解析器阻断。
func TestDecodeYAMLRejectsUnknownField(t *testing.T) {
	content := `apiVersion: codebee/v1
kind: Orchestration
unknownField: true
metadata:
  name: default-issue-flow
workflow:
  startNodeId: issue_handling
  nodes: []
  edges: []
runtime:
  maxCodingReviewRounds: 3
  maxConsecutiveUnknownReview: 2
  maxIssuePostAttempts: 2
  loopJudgeStartRound: 0
roles:
  defaultAgent: 开发者
  reviewerAgent: QA负责人
  issuePostAgent: 产品经理
  loopJudgeAgent: 技术负责人
artifacts:
  issueHandlingResult: issue_intake_result.json
  codingResult: coding_result.json
  reviewResult: review_result.json
  issuePostResultPattern: issue_post_result_{purpose}.json
  loopJudgeResult: loop_judge_result.json
  loopHistory: loop_history.json
`

	if _, err := DecodeYAML([]byte(content)); err == nil {
		t.Fatal("DecodeYAML() error = nil, want error for unknown field")
	}
}

// TestDecodeYAMLRejectsInvalidRoleRef 验证非法角色引用会被静态校验阻断。
func TestDecodeYAMLRejectsInvalidRoleRef(t *testing.T) {
	doc := BuildDefaultDocument()
	doc.Workflow.Nodes[2].RoleRef = "randomAgent"

	if _, err := EncodeYAML(doc); err == nil {
		t.Fatal("DecodeYAML() error = nil, want error for invalid roleRef")
	}
}

// TestDecodeYAMLRejectsMultipleEdgeConditions 验证单条边不能同时声明多个条件字段。
func TestDecodeYAMLRejectsMultipleEdgeConditions(t *testing.T) {
	doc := BuildDefaultDocument()
	doc.Workflow.Edges[1].When.ReviewStatus = ReviewStatusPass

	if _, err := EncodeYAML(doc); err == nil {
		t.Fatal("EncodeYAML() error = nil, want error for multiple edge conditions")
	}
}

// TestLoadFileAndSaveFile 验证标准 YAML 文件可稳定保存并重新读取。
func TestLoadFileAndSaveFile(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "orchestration.yaml")

	doc, err := DecodeYAML(mustEncodeDefaultSchemaYAML(t))
	if err != nil {
		t.Fatalf("DecodeYAML() unexpected error: %v", err)
	}

	if err := SaveFile(filePath, doc); err != nil {
		t.Fatalf("SaveFile() unexpected error: %v", err)
	}

	loadedDoc, err := LoadFile(filePath)
	if err != nil {
		t.Fatalf("LoadFile() unexpected error: %v", err)
	}

	if loadedDoc.Metadata.Name != "default-issue-flow" {
		t.Fatalf("Metadata.Name = %q, want %q", loadedDoc.Metadata.Name, "default-issue-flow")
	}
}

// TestEncodeYAMLRejectsInvalidDocument 验证非法文档在编码前会被阻断。
func TestEncodeYAMLRejectsInvalidDocument(t *testing.T) {
	doc, err := DecodeYAML(mustEncodeDefaultSchemaYAML(t))
	if err != nil {
		t.Fatalf("DecodeYAML() unexpected error: %v", err)
	}

	doc.Workflow.Nodes[0].Outputs = nil

	if _, err := EncodeYAML(doc); err == nil {
		t.Fatal("EncodeYAML() error = nil, want error for invalid document")
	}
}

// TestValidateRejectsInvalidAPIVersion 验证非法 apiVersion 会被静态校验阻断。
func TestValidateRejectsInvalidAPIVersion(t *testing.T) {
	doc := BuildDefaultDocument()
	doc.APIVersion = "codebee/v0"

	if err := Validate(doc); err == nil {
		t.Fatal("Validate() error = nil, want error for invalid apiVersion")
	}
}

// TestValidateRejectsInvalidKind 验证非法 kind 会被静态校验阻断。
func TestValidateRejectsInvalidKind(t *testing.T) {
	doc := BuildDefaultDocument()
	doc.Kind = "Workflow"

	if err := Validate(doc); err == nil {
		t.Fatal("Validate() error = nil, want error for invalid kind")
	}
}

// TestValidateRejectsEmptyMetadataName 验证 metadata.name 不能为空。
func TestValidateRejectsEmptyMetadataName(t *testing.T) {
	doc := BuildDefaultDocument()
	doc.Metadata.Name = ""

	if err := Validate(doc); err == nil {
		t.Fatal("Validate() error = nil, want error for empty metadata.name")
	}
}

// TestValidateRejectsEmptyStartNodeID 验证 startNodeId 不能为空。
func TestValidateRejectsEmptyStartNodeID(t *testing.T) {
	doc := BuildDefaultDocument()
	doc.Workflow.StartNodeID = ""

	if err := Validate(doc); err == nil {
		t.Fatal("Validate() error = nil, want error for empty workflow.startNodeId")
	}
}

// TestValidateRejectsUnknownStartNodeID 验证 startNodeId 指向未知节点时会被阻断。
func TestValidateRejectsUnknownStartNodeID(t *testing.T) {
	doc := BuildDefaultDocument()
	doc.Workflow.StartNodeID = "unknown"

	if err := Validate(doc); err == nil {
		t.Fatal("Validate() error = nil, want error for unknown workflow.startNodeId")
	}
}

// TestValidateRejectsEmptyWorkflowNodes 验证节点列表不能为空。
func TestValidateRejectsEmptyWorkflowNodes(t *testing.T) {
	doc := BuildDefaultDocument()
	doc.Workflow.Nodes = nil

	if err := Validate(doc); err == nil {
		t.Fatal("Validate() error = nil, want error for empty workflow.nodes")
	}
}

// TestValidateRejectsDuplicateNodeID 验证重复节点 ID 会被阻断。
func TestValidateRejectsDuplicateNodeID(t *testing.T) {
	doc := BuildDefaultDocument()
	doc.Workflow.Nodes[1].ID = doc.Workflow.Nodes[0].ID

	if err := Validate(doc); err == nil {
		t.Fatal("Validate() error = nil, want error for duplicate node id")
	}
}

// TestValidateRejectsUnknownEdgeNode 验证边引用未知节点会被阻断。
func TestValidateRejectsUnknownEdgeNode(t *testing.T) {
	doc := BuildDefaultDocument()
	doc.Workflow.Edges[0].To = "unknown"

	if err := Validate(doc); err == nil {
		t.Fatal("Validate() error = nil, want error for unknown edge node")
	}
}

// TestValidateRejectsInvalidReviewStatus 验证非法状态枚举会被阻断。
func TestValidateRejectsInvalidReviewStatus(t *testing.T) {
	doc := BuildDefaultDocument()
	doc.Workflow.Edges[3].When.ReviewStatus = "MAYBE"

	if err := Validate(doc); err == nil {
		t.Fatal("Validate() error = nil, want error for invalid review status")
	}
}

// TestValidateRejectsUnknownUINodeReference 验证 UI 元数据引用未知节点会被阻断。
func TestValidateRejectsUnknownUINodeReference(t *testing.T) {
	doc := BuildDefaultDocument()
	doc.UI.Nodes["ghost"] = NodeUI{X: 1, Y: 2}

	if err := Validate(doc); err == nil {
		t.Fatal("Validate() error = nil, want error for unknown UI node reference")
	}
}

// mustEncodeDefaultSchemaYAML 将默认文档编码为标准 YAML，供 round-trip 测试复用。
func mustEncodeDefaultSchemaYAML(t *testing.T) []byte {
	t.Helper()

	content, err := EncodeYAML(BuildDefaultDocument())
	if err != nil {
		t.Fatalf("EncodeYAML(BuildDefaultDocument()) unexpected error: %v", err)
	}

	return content
}
