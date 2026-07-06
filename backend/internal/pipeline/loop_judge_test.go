// Package pipeline 覆盖 loop judge 的契约校验与调度接线测试辅助。
//
// 核心功能:
// 1. 验证 loop judge 结果文件的结构化约束，避免非法决策污染状态机
// 2. 提供 scriptedRunner / fakePlatformClient 等测试辅助，供 dispatch 集成测试复用
//
// 注: 原覆盖 dispatch 状态机的两个测试（STOP_MANUAL / SHRINK_TASK）已在阶段 8 重构中删除，
// 新的 Service.Dispatch 行为由 internal/runtime 包的引擎测试覆盖。
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-04
// 更新时间: 2026-07-05
package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JiGuangWorker/code-bee/internal/agent"
)

// TestLoadLoopJudgeResultRejectsInvalidDecision 验证非法决策值会被读取层及时阻断。
func TestLoadLoopJudgeResultRejectsInvalidDecision(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "loop_judge_result.json")
	writeTestFile(t, filePath, `{
  "decision": "RETRY",
  "reason": "非法决策。",
  "evidence": "round-01",
  "confidence": "HIGH",
  "next_action": "继续。"
}`)

	if _, err := loadLoopJudgeResult(filePath); err == nil {
		t.Fatal("loadLoopJudgeResult() error = nil, want non-nil error")
	}
}

// scriptedRunner 用脚本化步骤模拟多阶段智能体执行。
//
// 按 kind 顺序匹配预设步骤，供 Service.Dispatch 集成测试复用。
type scriptedRunner struct {
	t     *testing.T
	steps []scriptedStep
	index int
}

// scriptedStep 表示一次预期的智能体调用。
type scriptedStep struct {
	kind agent.TaskKind
	run  func(task string) (*agent.RunResult, error)
}

// Run 按预设顺序模拟各阶段智能体执行。
func (r *scriptedRunner) Run(_ context.Context, kind agent.TaskKind, task string) (*agent.RunResult, error) {
	r.t.Helper()

	if r.index >= len(r.steps) {
		r.t.Fatalf("unexpected runner call kind=%s at index=%d", kind, r.index)
	}

	step := r.steps[r.index]
	r.index++

	if step.kind != kind {
		r.t.Fatalf("runner call kind=%s, want %s at index=%d", kind, step.kind, r.index-1)
	}

	return step.run(task)
}

// fakePlatformClient 提供最小平台技能包实现，避免测试依赖真实平台行为。
type fakePlatformClient struct{}

// Name 返回测试用平台名称。
func (fakePlatformClient) Name() string {
	return "GitHub"
}

// BuildIssueURL 返回稳定的测试 Issue URL。
func (fakePlatformClient) BuildIssueURL(repo string, issueNumber int) string {
	return fmt.Sprintf("https://example.com/%s/issues/%d", repo, issueNumber)
}

// BuildSkillInstruction 返回最小平台技能说明。
func (fakePlatformClient) BuildSkillInstruction(string, int) string {
	return "使用 gh 查看 issue 并按结果文件契约写入 JSON。"
}

// withTempWorkingDir 将测试过程切换到临时目录，避免污染真实工作区。
func withTempWorkingDir(t *testing.T, fn func()) {
	t.Helper()

	originDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() unexpected error: %v", err)
	}

	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("os.Chdir(%s) unexpected error: %v", tempDir, err)
	}

	t.Cleanup(func() {
		if err := os.Chdir(originDir); err != nil {
			t.Fatalf("restore working directory unexpected error: %v", err)
		}
	})

	fn()
}

// writeJSONFromPrompt 从提示词中提取结果文件路径并写入测试 JSON。
func writeJSONFromPrompt(t *testing.T, prompt string, prefix string, content string) {
	t.Helper()

	filePath := extractPathFromPrompt(t, prompt, prefix)
	if err := os.MkdirAll(filepath.Dir(filePath), artifactDirPermission); err != nil {
		t.Fatalf("os.MkdirAll(%s) unexpected error: %v", filepath.Dir(filePath), err)
	}

	if err := os.WriteFile(filePath, []byte(content), testFilePermission); err != nil {
		t.Fatalf("os.WriteFile(%s) unexpected error: %v", filePath, err)
	}
}

// extractPathFromPrompt 根据固定前缀从提示词中提取结果文件路径。
func extractPathFromPrompt(t *testing.T, prompt string, prefix string) string {
	t.Helper()

	for _, line := range strings.Split(prompt, "\n") {
		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedLine, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(trimmedLine, prefix))
		}
	}

	t.Fatalf("extractPathFromPrompt() cannot find prefix %q in prompt", prefix)
	return ""
}

// readLoopHistoryForTest 读取并反序列化 loop_history.json，供断言逐轮历史。
//
//nolint:unused // 保留供后续 loop 状态机测试启用时使用
func readLoopHistoryForTest(t *testing.T, filePath string) *LoopHistory {
	t.Helper()

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("os.ReadFile(%s) unexpected error: %v", filePath, err)
	}

	var history LoopHistory
	if err := json.Unmarshal(content, &history); err != nil {
		t.Fatalf("json.Unmarshal(%s) unexpected error: %v", filePath, err)
	}

	return &history
}
