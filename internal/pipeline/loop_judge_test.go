// Package pipeline 覆盖 loop judge 的契约校验与调度接线测试。
//
// 核心功能:
// 1. 验证 loop judge 结果文件的结构化约束，避免非法决策污染状态机
// 2. 验证 value judge 接入后能够真实改变 harness 的继续、收缩与停止行为
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-04
// 更新时间: 2026-07-04
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
	"github.com/JiGuangWorker/code-bee/internal/config"
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

// TestDispatchStopsWhenLoopJudgeRequestsManual 验证 loop judge 返回 STOP_MANUAL 时，自动循环会被立即中断。
func TestDispatchStopsWhenLoopJudgeRequestsManual(t *testing.T) {
	withTempWorkingDir(t, func() {
		cfg := config.New("owner/repo", 7)
		cfg.LoopJudgeStartRound = 2
		cfg.MaxCodingReviewRounds = 5

		runner := &scriptedRunner{
			t: t,
			steps: []scriptedStep{
				{
					kind: agent.TaskKindIssueHandling,
					run: func(task string) (*agent.RunResult, error) {
						writeJSONFromPrompt(t, task, "- 结果文件:", `{
  "status": "READY",
  "agent": "开发者",
  "summary": "任务可以进入开发。",
  "acceptance": "验收标准存在。",
  "next_action": "进入编码。",
  "comment_body": "已接单。"
}`)
						return &agent.RunResult{Success: true, Output: "issue handling ok"}, nil
					},
				},
				{
					kind: agent.TaskKindIssuePost,
					run: func(task string) (*agent.RunResult, error) {
						writeJSONFromPrompt(t, task, "- 提交结果文件:", `{
  "status": "POSTED",
  "summary": "接单回执已提交。",
  "feedback": "已发布接单说明。",
  "next_action": "进入编码阶段。"
}`)
						return &agent.RunResult{Success: true, Output: "issue post intake ok"}, nil
					},
				},
				{
					kind: agent.TaskKindCoding,
					run: func(task string) (*agent.RunResult, error) {
						writeJSONFromPrompt(t, task, "- 结果文件:", `{
  "status": "DONE",
  "summary": "第一轮编码完成。",
  "evidence": "已修改代码。",
  "acceptance_check": "完成基础实现。",
  "next_action": "等待审查。"
}`)
						return &agent.RunResult{Success: true, Output: "coding round1 ok"}, nil
					},
				},
				{
					kind: agent.TaskKindReview,
					run: func(task string) (*agent.RunResult, error) {
						writeJSONFromPrompt(t, task, "- 结果文件:", `{
  "status": "FAIL",
  "summary": "第一轮审查未通过。",
  "check_result": "边界条件未覆盖。",
  "missing": "缺少异常路径说明。",
  "next_action": "补充异常路径与验证证据。",
  "comment_body": ""
}`)
						return &agent.RunResult{Success: true, Output: "review round1 ok"}, nil
					},
				},
				{
					kind: agent.TaskKindCoding,
					run: func(task string) (*agent.RunResult, error) {
						writeJSONFromPrompt(t, task, "- 结果文件:", `{
  "status": "DONE",
  "summary": "第二轮编码完成。",
  "evidence": "补充了更多代码说明。",
  "acceptance_check": "仍存在缺口。",
  "next_action": "再次等待审查。"
}`)
						return &agent.RunResult{Success: true, Output: "coding round2 ok"}, nil
					},
				},
				{
					kind: agent.TaskKindReview,
					run: func(task string) (*agent.RunResult, error) {
						writeJSONFromPrompt(t, task, "- 结果文件:", `{
  "status": "FAIL",
  "summary": "第二轮审查仍未通过。",
  "check_result": "同一问题仍未收敛。",
  "missing": "缺少明确验证证据。",
  "next_action": "不要继续盲改，先判断是否还有继续价值。",
  "comment_body": ""
}`)
						return &agent.RunResult{Success: true, Output: "review round2 ok"}, nil
					},
				},
				{
					kind: agent.TaskKindLoopJudge,
					run: func(task string) (*agent.RunResult, error) {
						writeJSONFromPrompt(t, task, "- 评估结果文件:", `{
  "decision": "STOP_MANUAL",
  "reason": "问题连续两轮重复，继续自动循环价值很低。",
  "evidence": "round-01 与 round-02 都指出缺少验证证据，且缺失项高度重复。",
  "confidence": "HIGH",
  "next_action": "停止自动循环，转人工介入。"
}`)
						return &agent.RunResult{Success: true, Output: "loop judge stop manual"}, nil
					},
				},
			},
		}

		service := NewService(fakePlatformClient{}, runner)
		result, err := service.Dispatch(context.Background(), cfg)
		if err != nil {
			t.Fatalf("Dispatch() unexpected error: %v", err)
		}

		if !result.Success || !result.ManualRequired || result.Completed || result.Blocked {
			t.Fatalf("Dispatch() result = %+v, want success=true manual_required=true completed=false blocked=false", result)
		}

		historyFilePath := filepath.Join(".code-bee", "runs", "owner__repo", "issue-7", "loop_history.json")
		history := readLoopHistoryForTest(t, historyFilePath)
		if len(history.Rounds) != 2 {
			t.Fatalf("loop history rounds = %d, want 2", len(history.Rounds))
		}
	})
}

// TestDispatchShrinkTaskFeedbackFlowsToNextCodingRound 验证 SHRINK_TASK 会进入下一轮 coder 输入，而不是被调度器吞掉。
func TestDispatchShrinkTaskFeedbackFlowsToNextCodingRound(t *testing.T) {
	withTempWorkingDir(t, func() {
		cfg := config.New("owner/repo", 8)
		cfg.LoopJudgeStartRound = 1
		cfg.MaxCodingReviewRounds = 4

		runner := &scriptedRunner{
			t: t,
			steps: []scriptedStep{
				{
					kind: agent.TaskKindIssueHandling,
					run: func(task string) (*agent.RunResult, error) {
						writeJSONFromPrompt(t, task, "- 结果文件:", `{
  "status": "READY",
  "agent": "开发者",
  "summary": "任务可以进入开发。",
  "acceptance": "按验收标准完成并提供证据。",
  "next_action": "进入编码。",
  "comment_body": "已接单。"
}`)
						return &agent.RunResult{Success: true, Output: "issue handling ok"}, nil
					},
				},
				{
					kind: agent.TaskKindIssuePost,
					run: func(task string) (*agent.RunResult, error) {
						writeJSONFromPrompt(t, task, "- 提交结果文件:", `{
  "status": "POSTED",
  "summary": "接单回执已提交。",
  "feedback": "已发布接单说明。",
  "next_action": "进入编码阶段。"
}`)
						return &agent.RunResult{Success: true, Output: "issue post intake ok"}, nil
					},
				},
				{
					kind: agent.TaskKindCoding,
					run: func(task string) (*agent.RunResult, error) {
						writeJSONFromPrompt(t, task, "- 结果文件:", `{
  "status": "DONE",
  "summary": "第一轮编码完成。",
  "evidence": "完成基础实现。",
  "acceptance_check": "缺少验证证据。",
  "next_action": "等待审查。"
}`)
						return &agent.RunResult{Success: true, Output: "coding round1 ok"}, nil
					},
				},
				{
					kind: agent.TaskKindReview,
					run: func(task string) (*agent.RunResult, error) {
						writeJSONFromPrompt(t, task, "- 结果文件:", `{
  "status": "FAIL",
  "summary": "第一轮审查未通过。",
  "check_result": "实现方向基本正确，但证据不足。",
  "missing": "缺少测试与验证说明。",
  "next_action": "优先补证据。",
  "comment_body": ""
}`)
						return &agent.RunResult{Success: true, Output: "review round1 ok"}, nil
					},
				},
				{
					kind: agent.TaskKindLoopJudge,
					run: func(task string) (*agent.RunResult, error) {
						writeJSONFromPrompt(t, task, "- 评估结果文件:", `{
  "decision": "SHRINK_TASK",
  "reason": "当前更适合先补证据，而不是继续扩改代码。",
  "evidence": "round-01 的 review 已明确指出缺的是测试与验证说明。",
  "confidence": "HIGH",
  "next_action": "下一轮聚焦补测试、补验证结果，不要继续扩展实现范围。"
}`)
						return &agent.RunResult{Success: true, Output: "loop judge shrink task"}, nil
					},
				},
				{
					kind: agent.TaskKindCoding,
					run: func(task string) (*agent.RunResult, error) {
						if !strings.Contains(task, "价值评估结论:") {
							t.Fatal("second coding round prompt does not contain loop judge feedback")
						}

						if !strings.Contains(task, "下一轮聚焦补测试、补验证结果") {
							t.Fatal("second coding round prompt does not contain shrink-task next_action")
						}

						writeJSONFromPrompt(t, task, "- 结果文件:", `{
  "status": "DONE",
  "summary": "第二轮完成证据补齐。",
  "evidence": "新增测试结果与验证说明。",
  "acceptance_check": "验收项已有对应证据。",
  "next_action": "再次等待审查。"
}`)
						return &agent.RunResult{Success: true, Output: "coding round2 ok"}, nil
					},
				},
				{
					kind: agent.TaskKindReview,
					run: func(task string) (*agent.RunResult, error) {
						writeJSONFromPrompt(t, task, "- 结果文件:", `{
  "status": "PASS",
  "summary": "第二轮审查通过。",
  "check_result": "验收项与证据均已覆盖。",
  "missing": "无",
  "next_action": "允许结束 loop。",
  "comment_body": "任务已完成，验收通过。"
}`)
						return &agent.RunResult{Success: true, Output: "review round2 ok"}, nil
					},
				},
				{
					kind: agent.TaskKindIssuePost,
					run: func(task string) (*agent.RunResult, error) {
						writeJSONFromPrompt(t, task, "- 提交结果文件:", `{
  "status": "POSTED",
  "summary": "完成回执已提交。",
  "feedback": "已发布最终完成说明。",
  "next_action": "结束流程。"
}`)
						return &agent.RunResult{Success: true, Output: "issue post completion ok"}, nil
					},
				},
			},
		}

		service := NewService(fakePlatformClient{}, runner)
		result, err := service.Dispatch(context.Background(), cfg)
		if err != nil {
			t.Fatalf("Dispatch() unexpected error: %v", err)
		}

		if !result.Success || !result.Completed || result.ManualRequired || result.Blocked {
			t.Fatalf("Dispatch() result = %+v, want success=true completed=true manual_required=false blocked=false", result)
		}
	})
}

// scriptedRunner 用脚本化步骤模拟多阶段智能体执行。
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
