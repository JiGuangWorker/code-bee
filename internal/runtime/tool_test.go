package runtime

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/JiGuangWorker/code-bee/internal/agent"
	"github.com/JiGuangWorker/code-bee/internal/config"
	"github.com/JiGuangWorker/code-bee/internal/schema"
)

// scriptedRunner 是测试用的 Runner mock。
type scriptedRunner struct {
	output string
	err    error
	calls  []string // 记录每次调用的 task
}

func (r *scriptedRunner) Run(ctx context.Context, kind agent.TaskKind, task string) (*agent.RunResult, error) {
	r.calls = append(r.calls, task)
	if r.err != nil {
		return &agent.RunResult{Success: false, Output: r.output}, r.err
	}
	return &agent.RunResult{Success: true, Output: r.output}, nil
}

// fakeArtifacts 是测试用的 ArtifactResolver mock。
type fakeArtifacts struct {
	results map[string]map[string]any // stageName → data
	loaded  []string                  // 记录被加载的 stageName
}

func (f *fakeArtifacts) ResolveResultFile(stageName string, args map[string]any, loopRound int) string {
	return "/tmp/fake_" + stageName + ".json"
}

func (f *fakeArtifacts) ResetResultFile(path string) error { return nil }

func (f *fakeArtifacts) LoadResult(stageName string, path string) (map[string]any, error) {
	f.loaded = append(f.loaded, stageName)
	if data, ok := f.results[stageName]; ok {
		return data, nil
	}
	return nil, errors.New("no result for " + stageName)
}

func (f *fakeArtifacts) LoopHistoryPath() string { return "/tmp/loop_history.json" }

func newTestAgentTool(t *testing.T, template string) (*AgentTool, *scriptedRunner, *PromptBuilder) {
	t.Helper()
	pb, err := NewPromptBuilder()
	if err != nil {
		t.Fatalf("NewPromptBuilder: %v", err)
	}
	runner := &scriptedRunner{output: "agent output"}
	def := &schema.Tool{
		Name:           "test-agent",
		Type:           "agent",
		PromptTemplate: template,
	}
	tool, err := NewAgentTool(def, runner, pb)
	if err != nil {
		t.Fatalf("NewAgentTool: %v", err)
	}
	return tool, runner, pb
}

func TestAgentTool_Execute_IssueHandling(t *testing.T) {
	tool, runner, _ := newTestAgentTool(t, promptIssueHandling)

	ec := NewExecutionContext(nil, &fakeArtifacts{}, PlatformContext{
		WorkerID:      "w1",
		IssueURL:      "https://github.com/o/r/issues/1",
		PlatformName:  "github",
		PlatformGuide: "guide",
	}, &config.Config{Repo: "o/r", IssueNumber: 1, DefaultAgent: "开发者"})

	inv := Invocation{
		StageName:      "issue-handling",
		Tool:           tool.Definition(),
		ResultFilePath: "/tmp/issue_intake_result.json",
		Platform:       ec.Platform,
		Config:         ec.Config,
		EC:             ec,
	}

	// fakeArtifacts 返回 READY 状态
	ec.Artifacts.(*fakeArtifacts).results = map[string]map[string]any{
		"issue-handling": {"status": "READY", "agent": "开发者"},
	}

	result, err := tool.Execute(context.Background(), inv)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if result.Status != "READY" {
		t.Errorf("got status %q, want READY", result.Status)
	}
	if len(runner.calls) != 1 {
		t.Errorf("expected 1 runner call, got %d", len(runner.calls))
	}
	// 验证 prompt 包含关键字段
	if !strings.Contains(runner.calls[0], "开发者") {
		t.Error("prompt should contain default agent")
	}
}

func TestAgentTool_Execute_Coding(t *testing.T) {
	tool, runner, _ := newTestAgentTool(t, promptCoding)

	fa := &fakeArtifacts{results: map[string]map[string]any{
		"coding": {"status": "DONE", "summary": "implemented"},
	}}
	ec := NewExecutionContext(nil, fa, PlatformContext{WorkerID: "w1"}, &config.Config{
		Repo: "o/r", IssueNumber: 1,
	})
	ec.SetResult("issue-handling", &StepResult{
		Data: map[string]any{
			"agent":      "开发者",
			"summary":    "fix bug",
			"acceptance": "tests pass",
		},
	})

	inv := Invocation{
		StageName:      "coding",
		Tool:           tool.Definition(),
		ResultFilePath: "/tmp/coding_result.json",
		Platform:       ec.Platform,
		Config:         ec.Config,
		EC:             ec,
		Loop:           &LoopState{Round: 1, MaxIterations: 3, LastFeedback: "add tests"},
	}

	result, err := tool.Execute(context.Background(), inv)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Status != "DONE" {
		t.Errorf("got status %q, want DONE", result.Status)
	}
	// 验证 prompt 包含 issue 摘要和反馈
	prompt := runner.calls[0]
	if !strings.Contains(prompt, "fix bug") {
		t.Error("prompt should contain issue summary")
	}
	if !strings.Contains(prompt, "add tests") {
		t.Error("prompt should contain reviewer feedback")
	}
	if !strings.Contains(prompt, "Round: 1/3") {
		t.Error("prompt should contain round info")
	}
}

func TestAgentTool_Execute_Blocked(t *testing.T) {
	tool, _, _ := newTestAgentTool(t, promptCoding)

	fa := &fakeArtifacts{results: map[string]map[string]any{
		"coding": {"status": "BLOCKED"},
	}}
	ec := NewExecutionContext(nil, fa, PlatformContext{}, &config.Config{})

	inv := Invocation{
		StageName:      "coding",
		Tool:           tool.Definition(),
		ResultFilePath: "/tmp/coding_result.json",
		EC:             ec,
	}

	result, err := tool.Execute(context.Background(), inv)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.Blocked {
		t.Error("expected Blocked=true for BLOCKED status")
	}
}

func TestAgentTool_Execute_RunnerError(t *testing.T) {
	pb, _ := NewPromptBuilder()
	runner := &scriptedRunner{err: errors.New("runner failed")}
	def := &schema.Tool{Name: "bad", Type: "agent", PromptTemplate: promptCoding}
	tool, _ := NewAgentTool(def, runner, pb)

	ec := NewExecutionContext(nil, &fakeArtifacts{}, PlatformContext{}, &config.Config{})
	inv := Invocation{
		StageName: "coding",
		Tool:      def,
		EC:        ec,
	}

	_, err := tool.Execute(context.Background(), inv)
	if err == nil {
		t.Fatal("expected error from runner")
	}
	if !strings.Contains(err.Error(), "runner failed") {
		t.Errorf("error should contain 'runner failed', got: %v", err)
	}
}

func TestNewAgentTool_Validation(t *testing.T) {
	pb, _ := NewPromptBuilder()
	runner := &scriptedRunner{}

	cases := []struct {
		name string
		def  *schema.Tool
	}{
		{"wrong type", &schema.Tool{Name: "t", Type: "command", PromptTemplate: "coding"}},
		{"missing template", &schema.Tool{Name: "t", Type: "agent"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := NewAgentTool(c.def, runner, pb)
			if err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestCommandTool_Execute_Success(t *testing.T) {
	// 用 fake exec
	oldExec := execCommandContext
	defer func() { execCommandContext = oldExec }()

	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		// 模拟输出 JSON
		cmd := exec.Command("echo", `{"status":"PASS","summary":"lint clean"}`)
		return cmd
	}

	def := &schema.Tool{Name: "lint", Type: "command", Run: "golangci-lint run"}
	tool, err := NewCommandTool(def)
	if err != nil {
		t.Fatal(err)
	}

	result, err := tool.Execute(context.Background(), Invocation{StageName: "lint"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if result.Status != "PASS" {
		t.Errorf("got status %q, want PASS", result.Status)
	}
}

func TestCommandTool_Execute_Failure(t *testing.T) {
	oldExec := execCommandContext
	defer func() { execCommandContext = oldExec }()

	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		// 模拟命令失败
		return exec.Command("false")
	}

	def := &schema.Tool{Name: "fail", Type: "command", Run: "exit 1"}
	tool, _ := NewCommandTool(def)

	_, err := tool.Execute(context.Background(), Invocation{StageName: "fail"})
	if err == nil {
		t.Fatal("expected error for failed command")
	}
}

func TestNewCommandTool_Validation(t *testing.T) {
	_, err := NewCommandTool(&schema.Tool{Name: "t", Type: "agent"})
	if err == nil {
		t.Error("expected error for wrong type")
	}

	_, err = NewCommandTool(&schema.Tool{Name: "t", Type: "command"})
	if err == nil {
		t.Error("expected error for missing run")
	}
}

func TestToolRegistry_BuildAndResolve(t *testing.T) {
	pb, _ := NewPromptBuilder()
	runner := &scriptedRunner{}

	wf := &schema.Workflow{
		Version: "1",
		Name:    "test",
		Tools: []schema.Tool{
			{Name: "coder", Type: "agent", PromptTemplate: "coding", Skill: "dev/SKILL.md"},
			{Name: "reviewer", Type: "agent", PromptTemplate: "review", Skill: "qa/SKILL.md", Aliases: []string{"QA负责人", "审查员"}},
			{Name: "lint", Type: "command", Run: "golangci-lint run"},
		},
	}

	registry, err := NewToolRegistry(wf, runner, pb)
	if err != nil {
		t.Fatalf("NewToolRegistry: %v", err)
	}

	// 按 name 查
	tool, ok := registry.Resolve("coder")
	if !ok {
		t.Fatal("expected to resolve coder")
	}
	if tool.Definition().Name != "coder" {
		t.Errorf("got name %s, want coder", tool.Definition().Name)
	}

	// 按 alias 查
	tool, ok = registry.Resolve("QA负责人")
	if !ok {
		t.Fatal("expected to resolve QA负责人 alias")
	}
	if tool.Definition().Name != "reviewer" {
		t.Errorf("got name %s, want reviewer", tool.Definition().Name)
	}

	// 查 command
	tool, ok = registry.Resolve("lint")
	if !ok {
		t.Fatal("expected to resolve lint")
	}

	// 未知工具
	_, ok = registry.Resolve("unknown")
	if ok {
		t.Error("expected not found for unknown")
	}
}

func TestToolRegistry_DuplicateName(t *testing.T) {
	pb, _ := NewPromptBuilder()
	wf := &schema.Workflow{
		Tools: []schema.Tool{
			{Name: "dup", Type: "command", Run: "echo 1"},
			{Name: "dup", Type: "command", Run: "echo 2"},
		},
	}
	_, err := NewToolRegistry(wf, nil, pb)
	if err == nil {
		t.Fatal("expected error for duplicate name")
	}
}

func TestToolRegistry_AliasConflict(t *testing.T) {
	pb, _ := NewPromptBuilder()
	wf := &schema.Workflow{
		Tools: []schema.Tool{
			{Name: "a", Type: "command", Run: "echo 1", Aliases: []string{"shared"}},
			{Name: "b", Type: "command", Run: "echo 2", Aliases: []string{"shared"}},
		},
	}
	_, err := NewToolRegistry(wf, nil, pb)
	if err == nil {
		t.Fatal("expected error for alias conflict")
	}
}

func TestFunctionTool_NotImplemented(t *testing.T) {
	def := &schema.Tool{Name: "fn", Type: "function", Function: "do-something"}
	tool, err := NewFunctionTool(def)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tool.Execute(context.Background(), Invocation{})
	if err == nil {
		t.Fatal("expected error for unimplemented function")
	}
}

func TestExtractStatus(t *testing.T) {
	cases := []struct {
		data map[string]any
		want string
	}{
		{map[string]any{"status": "PASS"}, "PASS"},
		{map[string]any{"status": 123}, ""},
		{map[string]any{}, ""},
		{nil, ""},
	}
	for _, c := range cases {
		got := extractStatus(c.data)
		if got != c.want {
			t.Errorf("extractStatus(%v) = %q, want %q", c.data, got, c.want)
		}
	}
}

// 确保 fakeArtifacts 满足 ArtifactResolver 接口
var _ ArtifactResolver = (*fakeArtifacts)(nil)
