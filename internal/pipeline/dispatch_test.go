// Package pipeline 覆盖四阶段 harness 中的文件契约与最小状态逻辑。
//
// 核心功能:
// 1. 验证 JSON 结果文件能够被稳定加载和校验
// 2. 验证 repo 到工件目录的映射稳定可预测
// 3. 验证 Service.Dispatch 委托 runtime.Engine 的端到端接线
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-04
// 更新时间: 2026-07-05
package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JiGuangWorker/code-bee/internal/agent"
	"github.com/JiGuangWorker/code-bee/internal/config"
	"github.com/JiGuangWorker/code-bee/internal/runtime"
	"github.com/JiGuangWorker/code-bee/internal/schema"
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

// TestToPipelineResult 验证 EngineResult → pipeline.Result 的字段映射。
func TestToPipelineResult(t *testing.T) {
	cases := []struct {
		name string
		in   *runtime.EngineResult
		want Result
	}{
		{
			name: "nil-input",
			in:   nil,
			want: Result{Success: false},
		},
		{
			name: "completed",
			in:   &runtime.EngineResult{Success: true, Completed: true, Output: "done"},
			want: Result{Success: true, Completed: true, Output: "done"},
		},
		{
			name: "blocked",
			in:   &runtime.EngineResult{Success: true, Blocked: true, Output: "stuck"},
			want: Result{Success: true, Blocked: true, Output: "stuck"},
		},
		{
			name: "manual-required",
			in:   &runtime.EngineResult{Success: true, ManualRequired: true, Output: "need human"},
			want: Result{Success: true, ManualRequired: true, Output: "need human"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := toPipelineResult(tc.in)
			if got.Success != tc.want.Success || got.Completed != tc.want.Completed ||
				got.Blocked != tc.want.Blocked || got.ManualRequired != tc.want.ManualRequired ||
				got.Output != tc.want.Output {
				t.Fatalf("toPipelineResult() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestServiceDispatchBlockedPath 验证 issue-handling 返回 BLOCKED 时，Service.Dispatch 返回 Blocked 结果。
//
// 端到端接线: Service → Engine → StageExecutor → AgentTool → ArtifactResolverAdapter → Result 转换
func TestServiceDispatchBlockedPath(t *testing.T) {
	withTempWorkingDir(t, func() {
		wf, err := runtime.LoadDefaultWorkflow()
		if err != nil {
			t.Fatalf("LoadDefaultWorkflow() unexpected error: %v", err)
		}

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
			},
		}

		service, err := NewService(fakePlatformClient{}, runner, wf)
		if err != nil {
			t.Fatalf("NewService() unexpected error: %v", err)
		}

		cfg := config.New("owner/repo", 51)
		result, err := service.Dispatch(context.Background(), cfg)
		if err != nil {
			t.Fatalf("Dispatch() unexpected error: %v", err)
		}

		if !result.Success || !result.Blocked || result.Completed {
			t.Fatalf("Dispatch() result = %+v, want blocked success result", result)
		}
	})
}

// TestServiceDispatchCompletedPath 验证 READY → 编码 → 审查 PASS → 完成评论提交 的完整路径。
func TestServiceDispatchCompletedPath(t *testing.T) {
	withTempWorkingDir(t, func() {
		wf, err := runtime.LoadDefaultWorkflow()
		if err != nil {
			t.Fatalf("LoadDefaultWorkflow() unexpected error: %v", err)
		}

		runner := &scriptedRunner{
			t: t,
			steps: []scriptedStep{
				// 1. issue-handling READY
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
						return &agent.RunResult{Success: true, Output: "issue handling ready"}, nil
					},
				},
				// 2. issue-post-intake POSTED
				{
					kind: agent.TaskKindIssuePost,
					run: func(task string) (*agent.RunResult, error) {
						writeJSONFromPrompt(t, task, "- 提交结果文件:", `{
  "status": "POSTED",
  "summary": "接单回执已提交。",
  "feedback": "已成功提交。",
  "next_action": "继续后续流程。"
}`)
						return &agent.RunResult{Success: true, Output: "issue post intake ok"}, nil
					},
				},
				// 3. coding DONE
				{
					kind: agent.TaskKindCoding,
					run: func(task string) (*agent.RunResult, error) {
						writeJSONFromPrompt(t, task, "- 结果文件:", `{
  "status": "DONE",
  "summary": "编码完成。",
  "evidence": "已补充实现与验证证据。",
  "acceptance_check": "按验收标准完成自检。",
  "next_action": "等待审查。"
}`)
						return &agent.RunResult{Success: true, Output: "coding done"}, nil
					},
				},
				// 4. review PASS
				{
					kind: agent.TaskKindReview,
					run: func(task string) (*agent.RunResult, error) {
						writeJSONFromPrompt(t, task, "- 结果文件:", `{
  "status": "PASS",
  "summary": "审查通过。",
  "check_result": "验收项全部通过。",
  "missing": "无",
  "next_action": "允许结束 loop。",
  "comment_body": "任务已完成。"
}`)
						return &agent.RunResult{Success: true, Output: "review pass"}, nil
					},
				},
				// 5. issue-post-completion POSTED
				{
					kind: agent.TaskKindIssuePost,
					run: func(task string) (*agent.RunResult, error) {
						writeJSONFromPrompt(t, task, "- 提交结果文件:", `{
  "status": "POSTED",
  "summary": "完成评论已提交。",
  "feedback": "已成功提交完成评论。",
  "next_action": "任务结束。"
}`)
						return &agent.RunResult{Success: true, Output: "issue post completion ok"}, nil
					},
				},
			},
		}

		service, err := NewService(fakePlatformClient{}, runner, wf)
		if err != nil {
			t.Fatalf("NewService() unexpected error: %v", err)
		}

		cfg := config.New("owner/repo", 52)
		result, err := service.Dispatch(context.Background(), cfg)
		if err != nil {
			t.Fatalf("Dispatch() unexpected error: %v", err)
		}

		if !result.Success || !result.Completed {
			t.Fatalf("Dispatch() result = %+v, want completed success result", result)
		}
	})
}

// TestServiceDispatchCustomWorkflow 验证用户自定义 workflow 配置（非默认）能驱动 Service.Dispatch，
// 且 --prompts-dir overlay 的自定义模板内容能到达 runner。
//
// 端到端覆盖:
// - 自定义 tool name（my-coder，不在默认 workflow 中）
// - 自定义 prompt 模板内容（通过 WithPromptsDir overlay 覆盖 coding.md）
// - 单 stage 无 loop 的最小 pipeline
func TestServiceDispatchCustomWorkflow(t *testing.T) {
	withTempWorkingDir(t, func() {
		// 1. 准备外部 prompts 目录，覆盖 coding.md
		promptsDir := t.TempDir()
		customTemplate := "自定义标记-CODING\n仓库: {{.Repo}}\nIssue: #{{.IssueNumber}}\n结果文件: {{.ResultFilePath}}\n"
		if err := os.WriteFile(filepath.Join(promptsDir, "coding.md"), []byte(customTemplate), testFilePermission); err != nil {
			t.Fatalf("os.WriteFile(coding.md) unexpected error: %v", err)
		}

		// 2. 编程式构造最小自定义 workflow
		wf := &schema.Workflow{
			Version: "1",
			Name:    "test-custom-workflow",
			Tools: []schema.Tool{{
				Name:           "my-coder",
				Type:           "agent",
				Skill:          "自定义编码技能",
				PromptTemplate: "coding",
				DisplayName:    "自定义开发者",
				Aliases:        []string{"自定义开发者"},
			}},
			Pipeline: []schema.PipelineStep{{
				Stage: &schema.Stage{
					Name:   "custom-coding",
					Tool:   "my-coder",
					Output: "custom_result.json",
				},
			}},
		}

		// 3. scriptedRunner 验证自定义 prompt 内容到达
		runner := &scriptedRunner{
			t: t,
			steps: []scriptedStep{
				{
					kind: agent.TaskKindCoding,
					run: func(task string) (*agent.RunResult, error) {
						if !strings.Contains(task, "自定义标记-CODING") {
							t.Errorf("custom prompt marker missing, task=\n%s", task)
						}
						if !strings.Contains(task, "owner/custom-repo") {
							t.Errorf("repo not rendered in prompt, task=\n%s", task)
						}
						writeJSONFromPrompt(t, task, "结果文件:", `{
  "status": "DONE",
  "summary": "自定义编码完成。"
}`)
						return &agent.RunResult{Success: true, Output: "custom coding done"}, nil
					},
				},
			},
		}

		// 4. 构造 service，传入 WithPromptsDir
		service, err := NewService(fakePlatformClient{}, runner, wf, runtime.WithPromptsDir(promptsDir))
		if err != nil {
			t.Fatalf("NewService() unexpected error: %v", err)
		}

		// 5. 执行 Dispatch
		cfg := config.New("owner/custom-repo", 77)
		result, err := service.Dispatch(context.Background(), cfg)
		if err != nil {
			t.Fatalf("Dispatch() unexpected error: %v", err)
		}

		if !result.Success || !result.Completed {
			t.Fatalf("Dispatch() result = %+v, want completed success result", result)
		}
	})
}
