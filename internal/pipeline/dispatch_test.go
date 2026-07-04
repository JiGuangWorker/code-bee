// Package pipeline 覆盖四阶段 harness 中的文件契约与最小状态逻辑。
//
// 核心功能:
// 1. 验证 JSON 结果文件能够被稳定加载和校验
// 2. 验证 repo 到工件目录的映射稳定可预测
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-04
// 更新时间: 2026-07-04
package pipeline

import (
	"os"
	"path/filepath"
	"testing"
)

const testFilePermission = 0o600

// TestLoadIssueHandlingResultReady 验证 intake 结果文件能够被正确读取。
func TestLoadIssueHandlingResultReady(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "issue_intake_result.json")
	writeTestFile(t, filePath, `{
  "status": "READY",
  "agent": "开发者",
  "summary": "登录功能可以开工。",
  "acceptance": "正常登录成功；错误密码被拦截。",
  "next_action": "进入编码阶段。",
  "comment_body": "我已接单，将按验收标准开始处理。"
}`)

	result, err := loadIssueHandlingResult(filePath)
	if err != nil {
		t.Fatalf("loadIssueHandlingResult() unexpected error: %v", err)
	}

	if !result.Ready() || result.BlockedStatus() {
		t.Fatal("loadIssueHandlingResult() should return READY result")
	}

	if result.Agent != "开发者" {
		t.Fatalf("loadIssueHandlingResult() Agent = %q, want %q", result.Agent, "开发者")
	}
}

// TestLoadReviewResultRejectsInvalidStatus 验证非法状态会在读取 review 结果时被及时阻断。
func TestLoadReviewResultRejectsInvalidStatus(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "review_result.json")
	writeTestFile(t, filePath, `{
  "status": "DONE",
  "summary": "非法状态。",
  "check_result": "无",
  "missing": "无",
  "next_action": "无",
  "comment_body": ""
}`)

	if _, err := loadReviewResult(filePath); err == nil {
		t.Fatal("loadReviewResult() error = nil, want non-nil error")
	}
}

// TestLoadReviewResultPassRequiresCommentBody 验证 PASS 结果必须携带可提交评论正文。
func TestLoadReviewResultPassRequiresCommentBody(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "review_result.json")
	writeTestFile(t, filePath, `{
  "status": "PASS",
  "summary": "审查通过。",
  "check_result": "验收项全部通过。",
  "missing": "无",
  "next_action": "允许结束 loop。",
  "comment_body": ""
}`)

	if _, err := loadReviewResult(filePath); err == nil {
		t.Fatal("loadReviewResult() error = nil, want non-nil error when PASS lacks comment_body")
	}
}

// TestIssuePostResultStates 验证 Issue 提交结果的状态辅助方法。
func TestIssuePostResultStates(t *testing.T) {
	result := &IssuePostResult{
		Status:     postStatusRejected,
		Summary:    "评论格式不合理。",
		Feedback:   "请补充更清晰的完成说明。",
		NextAction: "重新整理提交内容。",
	}

	if result.Posted() {
		t.Fatal("IssuePostResult.Posted() = true, want false")
	}

	if !result.Rejected() {
		t.Fatal("IssuePostResult.Rejected() = false, want true")
	}
}

// TestSanitizeRepoName 验证 repo 名称会被映射成稳定目录名。
func TestSanitizeRepoName(t *testing.T) {
	got := sanitizeRepoName("owner/repo")
	want := "owner__repo"
	if got != want {
		t.Fatalf("sanitizeRepoName() = %q, want %q", got, want)
	}
}

// writeTestFile 向测试临时文件写入内容。
func writeTestFile(t *testing.T, filePath string, content string) {
	t.Helper()

	if err := os.WriteFile(filePath, []byte(content), testFilePermission); err != nil {
		t.Fatalf("os.WriteFile(%s) unexpected error: %v", filePath, err)
	}
}
