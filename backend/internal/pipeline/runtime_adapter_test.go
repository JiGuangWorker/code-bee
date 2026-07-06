// Package pipeline 覆盖 ArtifactResolverAdapter 的单元测试。
//
// 核心功能:
// 1. 验证 stageName → 文件路径的映射符合 default_workflow.yaml 约定
// 2. 验证 LoadResult 按 stageName 路由到对应的 typed loader
// 3. 验证 ResetResultFile 和 LoopHistoryPath 的行为
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-05
package pipeline

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestAdapterResolveResultFileMapping 验证各 stageName 的结果文件路径映射。
func TestAdapterResolveResultFileMapping(t *testing.T) {
	artifacts, err := NewArtifactSet("owner/repo", 42)
	if err != nil {
		t.Fatalf("NewArtifactSet() unexpected error: %v", err)
	}

	adapter := NewArtifactResolverAdapter(artifacts)

	cases := []struct {
		name       string
		stageName  string
		args       map[string]any
		loopRound  int
		wantSuffix string
	}{
		{name: "issue-handling", stageName: "issue-handling", wantSuffix: "issue_intake_result.json"},
		{name: "coding-non-loop", stageName: "coding", loopRound: 0, wantSuffix: "coding_result.json"},
		{name: "coding-in-loop", stageName: "coding", loopRound: 2, wantSuffix: "coding_result.json"},
		{name: "review-non-loop", stageName: "review", loopRound: 0, wantSuffix: "review_result.json"},
		{name: "loop-judge-round-1", stageName: "loop-judge", loopRound: 1, wantSuffix: filepath.Join("round-01", "loop_judge_result.json")},
		{name: "loop-judge-round-2", stageName: "loop-judge", loopRound: 2, wantSuffix: filepath.Join("round-02", "loop_judge_result.json")},
		{name: "issue-post-intake", stageName: "issue-post-intake", args: map[string]any{"purpose": "intake"}, wantSuffix: "issue_post_result_intake.json"},
		{name: "issue-post-completion", stageName: "issue-post-completion", args: map[string]any{"purpose": "completion"}, wantSuffix: "issue_post_result_completion.json"},
		{name: "unknown-stage", stageName: "custom-stage", wantSuffix: "custom-stage.json"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := adapter.ResolveResultFile(tc.stageName, tc.args, tc.loopRound)
			if !strings.HasSuffix(got, tc.wantSuffix) {
				t.Fatalf("ResolveResultFile(%q) = %q, want suffix %q", tc.stageName, got, tc.wantSuffix)
			}
		})
	}
}

// TestAdapterLoadResultRouting 验证 LoadResult 按 stageName 路由到正确的 typed loader。
func TestAdapterLoadResultRouting(t *testing.T) {
	artifacts, err := NewArtifactSet("owner/repo", 42)
	if err != nil {
		t.Fatalf("NewArtifactSet() unexpected error: %v", err)
	}

	adapter := NewArtifactResolverAdapter(artifacts)

	// 写入一个合法的 issue-handling 结果文件
	path := adapter.ResolveResultFile("issue-handling", nil, 0)
	writeTestFile(t, path, `{
  "status": "READY",
  "agent": "开发者",
  "summary": "任务可以开工。",
  "acceptance": "验收标准。",
  "next_action": "进入编码。",
  "comment_body": "已接单。"
}`)

	m, err := adapter.LoadResult("issue-handling", path)
	if err != nil {
		t.Fatalf("LoadResult(issue-handling) unexpected error: %v", err)
	}

	if m["status"] != "READY" {
		t.Fatalf("LoadResult(issue-handling) status = %v, want READY", m["status"])
	}
	if m["agent"] != "开发者" {
		t.Fatalf("LoadResult(issue-handling) agent = %v, want 开发者", m["agent"])
	}
}

// TestAdapterLoadResultRejectsInvalidFile 验证 LoadResult 会透传 typed loader 的校验错误。
func TestAdapterLoadResultRejectsInvalidFile(t *testing.T) {
	artifacts, err := NewArtifactSet("owner/repo", 42)
	if err != nil {
		t.Fatalf("NewArtifactSet() unexpected error: %v", err)
	}

	adapter := NewArtifactResolverAdapter(artifacts)

	// 写入一个非法的 coding 结果文件（status=PASS 不合法）
	path := adapter.ResolveResultFile("coding", nil, 0)
	writeTestFile(t, path, `{
  "status": "PASS",
  "summary": "非法状态。",
  "evidence": "无",
  "acceptance_check": "无",
  "next_action": "无"
}`)

	if _, err := adapter.LoadResult("coding", path); err == nil {
		t.Fatal("LoadResult(coding) error = nil, want non-nil error for invalid status")
	}
}

// TestAdapterLoadResultUnknownStage 验证未知 stageName 走 raw map 加载（无校验）。
func TestAdapterLoadResultUnknownStage(t *testing.T) {
	artifacts, err := NewArtifactSet("owner/repo", 42)
	if err != nil {
		t.Fatalf("NewArtifactSet() unexpected error: %v", err)
	}

	adapter := NewArtifactResolverAdapter(artifacts)

	path := adapter.ResolveResultFile("custom-stage", nil, 0)
	writeTestFile(t, path, `{"foo": "bar", "count": 42}`)

	m, err := adapter.LoadResult("custom-stage", path)
	if err != nil {
		t.Fatalf("LoadResult(custom-stage) unexpected error: %v", err)
	}

	if m["foo"] != "bar" {
		t.Fatalf("LoadResult(custom-stage) foo = %v, want bar", m["foo"])
	}
}

// TestAdapterResetResultFile 验证 ResetResultFile 删除文件且对不存在的文件不报错。
func TestAdapterResetResultFile(t *testing.T) {
	artifacts, err := NewArtifactSet("owner/repo", 42)
	if err != nil {
		t.Fatalf("NewArtifactSet() unexpected error: %v", err)
	}

	adapter := NewArtifactResolverAdapter(artifacts)

	path := adapter.ResolveResultFile("issue-handling", nil, 0)
	writeTestFile(t, path, `{"status": "READY"}`)

	if err := adapter.ResetResultFile(path); err != nil {
		t.Fatalf("ResetResultFile(existing) unexpected error: %v", err)
	}

	// 再次删除（文件已不存在）不应报错
	if err := adapter.ResetResultFile(path); err != nil {
		t.Fatalf("ResetResultFile(non-existent) unexpected error: %v", err)
	}
}

// TestAdapterLoopHistoryPath 验证 LoopHistoryPath 返回稳定路径。
func TestAdapterLoopHistoryPath(t *testing.T) {
	artifacts, err := NewArtifactSet("owner/repo", 42)
	if err != nil {
		t.Fatalf("NewArtifactSet() unexpected error: %v", err)
	}

	adapter := NewArtifactResolverAdapter(artifacts)

	got := adapter.LoopHistoryPath()
	if !strings.HasSuffix(got, "loop_history.json") {
		t.Fatalf("LoopHistoryPath() = %q, want suffix loop_history.json", got)
	}
}
