// Package schema 覆盖默认标准文档构造能力。
//
// 核心功能:
// 1. 验证 BuildDefaultDocument 生成的默认文档与当前源码映射稿保持一致
// 2. 验证默认文档可以稳定通过 Validate、EncodeYAML、DecodeYAML 闭环
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-06
// 更新时间: 2026-07-06
package schema

import "testing"

// TestBuildDefaultDocumentReturnsValidDocument 验证默认文档构造器直接返回一份合法文档。
func TestBuildDefaultDocumentReturnsValidDocument(t *testing.T) {
	doc := BuildDefaultDocument()

	if err := Validate(doc); err != nil {
		t.Fatalf("Validate(BuildDefaultDocument()) unexpected error: %v", err)
	}

	if doc.APIVersion != APIVersion {
		t.Fatalf("APIVersion = %q, want %q", doc.APIVersion, APIVersion)
	}

	if doc.Kind != Kind {
		t.Fatalf("Kind = %q, want %q", doc.Kind, Kind)
	}

	if doc.Metadata.Name != DefaultDocumentName {
		t.Fatalf("Metadata.Name = %q, want %q", doc.Metadata.Name, DefaultDocumentName)
	}
}

// TestBuildDefaultDocumentRoundTrip 验证默认文档可稳定完成 YAML round-trip。
func TestBuildDefaultDocumentRoundTrip(t *testing.T) {
	doc := BuildDefaultDocument()

	content, err := EncodeYAML(doc)
	if err != nil {
		t.Fatalf("EncodeYAML(BuildDefaultDocument()) unexpected error: %v", err)
	}

	decodedDoc, err := DecodeYAML(content)
	if err != nil {
		t.Fatalf("DecodeYAML(EncodeYAML(BuildDefaultDocument())) unexpected error: %v", err)
	}

	if decodedDoc.APIVersion != APIVersion {
		t.Fatalf("decoded APIVersion = %q, want %q", decodedDoc.APIVersion, APIVersion)
	}

	if decodedDoc.Kind != Kind {
		t.Fatalf("decoded Kind = %q, want %q", decodedDoc.Kind, Kind)
	}

	if decodedDoc.Metadata.Name != DefaultDocumentName {
		t.Fatalf("decoded Metadata.Name = %q, want %q", decodedDoc.Metadata.Name, DefaultDocumentName)
	}

	if decodedDoc.Workflow.StartNodeID != DefaultStartNodeID {
		t.Fatalf("decoded Workflow.StartNodeID = %q, want %q", decodedDoc.Workflow.StartNodeID, DefaultStartNodeID)
	}

	if len(decodedDoc.Workflow.Nodes) != len(doc.Workflow.Nodes) {
		t.Fatalf("len(decoded Workflow.Nodes) = %d, want %d", len(decodedDoc.Workflow.Nodes), len(doc.Workflow.Nodes))
	}

	if len(decodedDoc.Workflow.Edges) != len(doc.Workflow.Edges) {
		t.Fatalf("len(decoded Workflow.Edges) = %d, want %d", len(decodedDoc.Workflow.Edges), len(doc.Workflow.Edges))
	}
}
