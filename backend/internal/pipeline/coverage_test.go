// Package pipeline 覆盖文件契约与历史工具的单元测试。
//
// 核心功能:
// 1. 补齐 artifacts、contracts、history 的单元测试
// 2. 验证结果文件加载与校验逻辑的稳定性
//
// 注: 原覆盖 dispatch 状态机的测试已在阶段 8 重构中删除，
// 新的 Service.Dispatch 行为由 internal/runtime 包的引擎测试覆盖。
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-04
// 更新时间: 2026-07-05
package pipeline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestArtifactSetRoundPaths 验证逐轮工件路径与聚合历史路径稳定可预测。
func TestArtifactSetRoundPaths(t *testing.T) {
	artifacts, err := NewArtifactSet("owner/repo", 21)
	if err != nil {
		t.Fatalf("NewArtifactSet() unexpected error: %v", err)
	}

	if err := artifacts.EnsureRoundDir(2); err != nil {
		t.Fatalf("EnsureRoundDir() unexpected error: %v", err)
	}

	requiredSuffixes := map[string]string{
		"CodingResultPath":    filepath.Join("round-02", "coding_result.json"),
		"ReviewResultPath":    filepath.Join("round-02", "review_result.json"),
		"LoopJudgeResultPath": filepath.Join("round-02", "loop_judge_result.json"),
		"LoopHistoryPath":     "loop_history.json",
	}

	gotPaths := map[string]string{
		"CodingResultPath":    artifacts.CodingResultPathForRound(2),
		"ReviewResultPath":    artifacts.ReviewResultPathForRound(2),
		"LoopJudgeResultPath": artifacts.LoopJudgeResultPath(2),
		"LoopHistoryPath":     artifacts.LoopHistoryPath(),
	}

	for name, suffix := range requiredSuffixes {
		if !strings.HasSuffix(gotPaths[name], suffix) {
			t.Fatalf("%s = %q, want suffix %q", name, gotPaths[name], suffix)
		}
	}
}

// TestLoadCodingResultRejectsInvalidStatus 验证 coding 结果非法状态会被结构化读取层及时阻断。
func TestLoadCodingResultRejectsInvalidStatus(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "coding_result.json")
	writeTestFile(t, filePath, `{
  "status": "PASS",
  "summary": "非法状态。",
  "evidence": "无",
  "acceptance_check": "无",
  "next_action": "无"
}`)

	if _, err := loadCodingResult(filePath); err == nil {
		t.Fatal("loadCodingResult() error = nil, want non-nil error")
	}
}

// TestLoadIssuePostResultRejectsEmptyFeedback 验证 Issue 提交结果缺少 feedback 时会被阻断。
func TestLoadIssuePostResultRejectsEmptyFeedback(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "issue_post_result.json")
	writeTestFile(t, filePath, `{
  "status": "REJECTED",
  "summary": "拒绝提交。",
  "feedback": "",
  "next_action": "补充评论正文。"
}`)

	if _, err := loadIssuePostResult(filePath); err == nil {
		t.Fatal("loadIssuePostResult() error = nil, want non-nil error")
	}
}

// TestLoadLoopJudgeResultRequiresEvidence 验证价值评估结果必须引用明确证据。
func TestLoadLoopJudgeResultRequiresEvidence(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "loop_judge_result.json")
	writeTestFile(t, filePath, `{
  "decision": "CONTINUE",
  "reason": "还可以继续。",
  "evidence": "",
  "confidence": "MEDIUM",
  "next_action": "继续下一轮。"
}`)

	if _, err := loadLoopJudgeResult(filePath); err == nil {
		t.Fatal("loadLoopJudgeResult() error = nil, want non-nil error")
	}
}

// TestLoopHistoryAppendRoundRejectsNilReview 验证逐轮历史在缺少 review 结果时不会悄悄落盘。
func TestLoopHistoryAppendRoundRejectsNilReview(t *testing.T) {
	history := &LoopHistory{}
	err := history.appendRound(1, &CodingResult{
		Status:          coderStatusDone,
		Summary:         "ok",
		Evidence:        "ok",
		AcceptanceCheck: "ok",
		NextAction:      "ok",
	}, nil)
	if err == nil {
		t.Fatal("LoopHistory.appendRound() error = nil, want non-nil error")
	}
}

// TestSaveLoopHistoryWritesReadableJSON 验证聚合历史会被稳定写成可读 JSON 文件。
func TestSaveLoopHistoryWritesReadableJSON(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "loop_history.json")
	history := &LoopHistory{
		Repo:        "owner/repo",
		IssueNumber: 36,
		MaxRounds:   10,
		Rounds: []LoopHistoryRound{
			{
				Round: 1,
				CodingResult: &CodingResult{
					Status:          coderStatusDone,
					Summary:         "第一轮编码完成。",
					Evidence:        "新增代码。",
					AcceptanceCheck: "已做自检。",
					NextAction:      "等待审查。",
				},
				ReviewResult: &ReviewResult{
					Status:      reviewStatusFail,
					Summary:     "第一轮未通过。",
					CheckResult: "验收不完整。",
					Missing:     "缺少边界测试。",
					NextAction:  "补测试。",
				},
			},
		},
	}

	if err := saveLoopHistory(filePath, history); err != nil {
		t.Fatalf("saveLoopHistory() unexpected error: %v", err)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("os.ReadFile() unexpected error: %v", err)
	}

	if !strings.Contains(string(content), "\"issue_number\": 36") {
		t.Fatalf("saveLoopHistory() content = %q, want contains issue_number", string(content))
	}
}
