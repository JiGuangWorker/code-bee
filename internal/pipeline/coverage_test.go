// Package pipeline 覆盖角色级单元测试与状态机场景测试。
//
// 核心功能:
// 1. 补齐 artifacts、contracts、history、feedback helper 的单元测试
// 2. 构造 issue-handling / coding / review / issue-post / loop-judge 的关键场景，验证调度器状态机
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-04
// 更新时间: 2026-07-04
package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JiGuangWorker/code-bee/internal/agent"
	"github.com/JiGuangWorker/code-bee/internal/config"
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
		"CodingResultPath":    artifacts.CodingResultPath(2),
		"ReviewResultPath":    artifacts.ReviewResultPath(2),
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

// TestNextReviewerFeedbackAddsEvidenceHintAfterUnknownThreshold 验证 reviewer 连续 UNKNOWN 时会强制切换到证据优先模式。
func TestNextReviewerFeedbackAddsEvidenceHintAfterUnknownThreshold(t *testing.T) {
	feedback, consecutiveUnknowns := nextReviewerFeedback(&ReviewResult{
		Status:      reviewStatusUnknown,
		Summary:     "暂时无法确认。",
		CheckResult: "证据不足。",
		Missing:     "缺少测试结果。",
		NextAction:  "补充证据。",
	}, maxConsecutiveUnknownReview-1)

	if consecutiveUnknowns != maxConsecutiveUnknownReview {
		t.Fatalf("consecutiveUnknowns = %d, want %d", consecutiveUnknowns, maxConsecutiveUnknownReview)
	}

	if !strings.Contains(feedback, "下一轮优先补充证据") {
		t.Fatalf("feedback = %q, want evidence-first hint", feedback)
	}
}

// TestShouldRunLoopJudgeThreshold 验证价值评估员触发阈值判断逻辑。
func TestShouldRunLoopJudgeThreshold(t *testing.T) {
	cfg := config.New("owner/repo", 1)
	cfg.LoopJudgeStartRound = 3

	if shouldRunLoopJudge(cfg, 2) {
		t.Fatal("shouldRunLoopJudge(round=2) = true, want false")
	}

	if !shouldRunLoopJudge(cfg, 3) {
		t.Fatal("shouldRunLoopJudge(round=3) = false, want true")
	}

	if !shouldRunLoopJudge(&config.Config{LoopJudgeStartRound: 0}, 1) {
		t.Fatal("shouldRunLoopJudge(startRound=0) = false, want true")
	}
}

// TestDispatchBlockedWhenIssueHandlingBlocked 验证 intake 阶段直接阻塞时，流程会在进入 coding 前终止。
func TestDispatchBlockedWhenIssueHandlingBlocked(t *testing.T) {
	withTempWorkingDir(t, func() {
		cfg := config.New("owner/repo", 31)
		runner := &scriptedRunner{
			t: t,
			steps: []scriptedStep{
				{
					kind: agent.TaskKindIssueHandling,
					run: func(task string) (*agent.RunResult, error) {
						writeJSONFromPrompt(t, task, "- 结果文件:", `{
  "status": "BLOCKED",
  "agent": "开发者",
  "summary": "缺少明确验收标准。",
  "acceptance": "暂无。",
  "next_action": "等待补充验收标准。",
  "comment_body": "当前缺少明确验收标准，暂不进入开发。"
}`)
						return &agent.RunResult{Success: true, Output: "issue handling blocked"}, nil
					},
				},
				{
					kind: agent.TaskKindIssuePost,
					run: func(task string) (*agent.RunResult, error) {
						writeJSONFromPrompt(t, task, "- 提交结果文件:", `{
  "status": "POSTED",
  "summary": "阻塞说明已提交。",
  "feedback": "已发布阻塞说明。",
  "next_action": "等待 issue 补充信息。"
}`)
						return &agent.RunResult{Success: true, Output: "issue post blocked"}, nil
					},
				},
			},
		}

		result, err := NewService(fakePlatformClient{}, runner).Dispatch(context.Background(), cfg)
		if err != nil {
			t.Fatalf("Dispatch() unexpected error: %v", err)
		}

		if !result.Success || !result.Blocked || result.Completed {
			t.Fatalf("Dispatch() result = %+v, want blocked success result", result)
		}
	})
}

// TestDispatchBlockedWhenCodingBlocked 验证 coder 明确 BLOCKED 时，流程会直接停止并返回阻塞态。
func TestDispatchBlockedWhenCodingBlocked(t *testing.T) {
	withTempWorkingDir(t, func() {
		cfg := config.New("owner/repo", 32)
		runner := &scriptedRunner{
			t: t,
			steps: []scriptedStep{
				makeIssueHandlingReadyStep(t),
				makeIssuePostPostedStep(t, "接单回执已提交。", "issue post intake ok"),
				{
					kind: agent.TaskKindCoding,
					run: func(task string) (*agent.RunResult, error) {
						writeJSONFromPrompt(t, task, "- 结果文件:", `{
  "status": "BLOCKED",
  "summary": "缺少仓库写权限。",
  "evidence": "push 被拒绝。",
  "acceptance_check": "尚未完成。",
  "next_action": "申请仓库写权限。"
}`)
						return &agent.RunResult{Success: true, Output: "coding blocked"}, nil
					},
				},
			},
		}

		result, err := NewService(fakePlatformClient{}, runner).Dispatch(context.Background(), cfg)
		if err != nil {
			t.Fatalf("Dispatch() unexpected error: %v", err)
		}

		if !result.Success || !result.Blocked || result.Output != "coding blocked" {
			t.Fatalf("Dispatch() result = %+v, want blocked result from coding", result)
		}
	})
}

// TestDispatchBlockedWhenReviewBlocked 验证 reviewer 明确 BLOCKED 时，流程会停止并要求人工介入。
func TestDispatchBlockedWhenReviewBlocked(t *testing.T) {
	withTempWorkingDir(t, func() {
		cfg := config.New("owner/repo", 33)
		runner := &scriptedRunner{
			t: t,
			steps: []scriptedStep{
				makeIssueHandlingReadyStep(t),
				makeIssuePostPostedStep(t, "接单回执已提交。", "issue post intake ok"),
				makeCodingDoneStep(t, "第一轮编码完成。", "coding round1 ok"),
				{
					kind: agent.TaskKindReview,
					run: func(task string) (*agent.RunResult, error) {
						writeJSONFromPrompt(t, task, "- 结果文件:", `{
  "status": "BLOCKED",
  "summary": "缺少测试环境。",
  "check_result": "无法继续验证。",
  "missing": "缺少联调环境与权限。",
  "next_action": "人工补齐环境后再审查。",
  "comment_body": ""
}`)
						return &agent.RunResult{Success: true, Output: "review blocked"}, nil
					},
				},
			},
		}

		result, err := NewService(fakePlatformClient{}, runner).Dispatch(context.Background(), cfg)
		if err != nil {
			t.Fatalf("Dispatch() unexpected error: %v", err)
		}

		if !result.Success || !result.Blocked || !result.ManualRequired {
			t.Fatalf("Dispatch() result = %+v, want blocked manual-required result", result)
		}
	})
}

// TestDispatchBlockedWhenLoopJudgeRequestsBlocked 验证价值评估员可直接将流程判定为外部阻塞。
func TestDispatchBlockedWhenLoopJudgeRequestsBlocked(t *testing.T) {
	withTempWorkingDir(t, func() {
		cfg := config.New("owner/repo", 34)
		cfg.LoopJudgeStartRound = 1
		runner := &scriptedRunner{
			t: t,
			steps: []scriptedStep{
				makeIssueHandlingReadyStep(t),
				makeIssuePostPostedStep(t, "接单回执已提交。", "issue post intake ok"),
				makeCodingDoneStep(t, "第一轮编码完成。", "coding round1 ok"),
				makeReviewFailStep(t, "第一轮审查未通过。", "缺少外部依赖响应。", "review round1 fail"),
				{
					kind: agent.TaskKindLoopJudge,
					run: func(task string) (*agent.RunResult, error) {
						writeJSONFromPrompt(t, task, "- 评估结果文件:", `{
  "decision": "STOP_BLOCKED",
  "reason": "问题核心依赖外部接口返回，继续自动循环没有意义。",
  "evidence": "round-01 已明确缺少外部依赖响应。",
  "confidence": "HIGH",
  "next_action": "停止自动循环，等待外部条件补齐。"
}`)
						return &agent.RunResult{Success: true, Output: "loop judge stop blocked"}, nil
					},
				},
			},
		}

		result, err := NewService(fakePlatformClient{}, runner).Dispatch(context.Background(), cfg)
		if err != nil {
			t.Fatalf("Dispatch() unexpected error: %v", err)
		}

		if !result.Success || !result.Blocked || !result.ManualRequired {
			t.Fatalf("Dispatch() result = %+v, want blocked manual-required result", result)
		}
	})
}

// TestDispatchReturnsErrorWhenCompletionIssuePostRejectedTwice 验证完成回帖连续两次 REJECTED 时，调度器会返回错误而不是误判成功。
func TestDispatchReturnsErrorWhenCompletionIssuePostRejectedTwice(t *testing.T) {
	withTempWorkingDir(t, func() {
		cfg := config.New("owner/repo", 35)
		runner := &scriptedRunner{
			t: t,
			steps: []scriptedStep{
				makeIssueHandlingReadyStep(t),
				makeIssuePostPostedStep(t, "接单回执已提交。", "issue post intake ok"),
				makeCodingDoneStep(t, "第一轮编码完成。", "coding round1 ok"),
				{
					kind: agent.TaskKindReview,
					run: func(task string) (*agent.RunResult, error) {
						writeJSONFromPrompt(t, task, "- 结果文件:", `{
  "status": "PASS",
  "summary": "第一轮审查通过。",
  "check_result": "验收项均通过。",
  "missing": "无",
  "next_action": "允许结束 loop。",
  "comment_body": "任务已完成。"
}`)
						return &agent.RunResult{Success: true, Output: "review pass"}, nil
					},
				},
				makeIssuePostRejectedStep(t, "完成评论信息不充分。", "issue post completion rejected #1"),
				makeIssuePostRejectedStep(t, "完成评论信息仍不充分。", "issue post completion rejected #2"),
			},
		}

		_, err := NewService(fakePlatformClient{}, runner).Dispatch(context.Background(), cfg)
		if err == nil {
			t.Fatal("Dispatch() error = nil, want non-nil error")
		}

		if !strings.Contains(err.Error(), "issue post completion rejected after 2 attempts") {
			t.Fatalf("Dispatch() error = %q, want retry exhausted error", err.Error())
		}
	})
}

// makeIssueHandlingReadyStep 构造通用的 intake READY 测试步骤。
func makeIssueHandlingReadyStep(t *testing.T) scriptedStep {
	t.Helper()

	return scriptedStep{
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
	}
}

// makeIssuePostPostedStep 构造通用的 Issue 提交成功测试步骤。
func makeIssuePostPostedStep(t *testing.T, summary string, output string) scriptedStep {
	t.Helper()

	return scriptedStep{
		kind: agent.TaskKindIssuePost,
		run: func(task string) (*agent.RunResult, error) {
			writeJSONFromPrompt(t, task, "- 提交结果文件:", `{
  "status": "POSTED",
  "summary": "`+summary+`",
  "feedback": "已成功提交。",
  "next_action": "继续后续流程。"
}`)
			return &agent.RunResult{Success: true, Output: output}, nil
		},
	}
}

// makeIssuePostRejectedStep 构造通用的 Issue 提交拒绝测试步骤。
func makeIssuePostRejectedStep(t *testing.T, feedback string, output string) scriptedStep {
	t.Helper()

	return scriptedStep{
		kind: agent.TaskKindIssuePost,
		run: func(task string) (*agent.RunResult, error) {
			writeJSONFromPrompt(t, task, "- 提交结果文件:", `{
  "status": "REJECTED",
  "summary": "评论内容不符合提交要求。",
  "feedback": "`+feedback+`",
  "next_action": "重新整理评论正文。"
}`)
			return &agent.RunResult{Success: true, Output: output}, nil
		},
	}
}

// makeCodingDoneStep 构造通用的 coding DONE 测试步骤。
func makeCodingDoneStep(t *testing.T, summary string, output string) scriptedStep {
	t.Helper()

	return scriptedStep{
		kind: agent.TaskKindCoding,
		run: func(task string) (*agent.RunResult, error) {
			writeJSONFromPrompt(t, task, "- 结果文件:", `{
  "status": "DONE",
  "summary": "`+summary+`",
  "evidence": "已补充实现与验证证据。",
  "acceptance_check": "按验收标准完成自检。",
  "next_action": "等待审查。"
}`)
			return &agent.RunResult{Success: true, Output: output}, nil
		},
	}
}

// makeReviewFailStep 构造通用的 review FAIL 测试步骤。
func makeReviewFailStep(t *testing.T, summary string, missing string, output string) scriptedStep {
	t.Helper()

	return scriptedStep{
		kind: agent.TaskKindReview,
		run: func(task string) (*agent.RunResult, error) {
			writeJSONFromPrompt(t, task, "- 结果文件:", `{
  "status": "FAIL",
  "summary": "`+summary+`",
  "check_result": "当前尚未满足全部验收要求。",
  "missing": "`+missing+`",
  "next_action": "补足缺失项后再次审查。",
  "comment_body": ""
}`)
			return &agent.RunResult{Success: true, Output: output}, nil
		},
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
