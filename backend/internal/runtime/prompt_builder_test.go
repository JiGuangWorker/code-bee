package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewPromptBuilder_LoadsAllTemplates(t *testing.T) {
	pb, err := NewPromptBuilder("")
	if err != nil {
		t.Fatalf("NewPromptBuilder() error: %v", err)
	}

	for _, name := range []string{promptIssueHandling, promptCoding, promptReview, promptIssuePost, promptLoopJudge} {
		if _, ok := pb.templates[name]; !ok {
			t.Errorf("template %q not loaded", name)
		}
	}
}

func TestPromptBuilder_Build_IssueHandling(t *testing.T) {
	pb, err := NewPromptBuilder("")
	if err != nil {
		t.Fatalf("NewPromptBuilder() error: %v", err)
	}

	data := PromptData{
		WorkerID:       "worker-123",
		DefaultAgent:   "开发者",
		Repo:           "owner/repo",
		IssueNumber:    42,
		IssueURL:       "https://github.com/owner/repo/issues/42",
		PlatformName:   "github",
		PlatformGuide:  "guide content",
		ResultFilePath: "/tmp/issue_intake_result.json",
		PostFeedback:   "previous feedback",
	}

	out, err := pb.Build(promptIssueHandling, data)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	// 验证关键字段都被渲染
	checks := []string{
		"worker-123",
		"开发者",
		"owner/repo",
		"#42",
		"https://github.com/owner/repo/issues/42",
		"/tmp/issue_intake_result.json",
		"previous feedback",
		"guide content",
	}
	for _, s := range checks {
		if !strings.Contains(out, s) {
			t.Errorf("output missing %q", s)
		}
	}
}

func TestPromptBuilder_Build_Coding(t *testing.T) {
	pb, err := NewPromptBuilder("")
	if err != nil {
		t.Fatalf("NewPromptBuilder() error: %v", err)
	}

	data := PromptData{
		Agent:            "开发者",
		ResultFilePath:   "/tmp/coding_result.json",
		WorkerID:         "worker-1",
		Repo:             "owner/repo",
		IssueNumber:      7,
		IssueURL:         "https://github.com/owner/repo/issues/7",
		PlatformName:     "github",
		Round:            2,
		MaxRounds:        3,
		PlatformGuide:    "guide",
		IssueSummary:     "fix bug",
		Acceptance:       "tests pass",
		ReviewerFeedback: "please add tests",
	}

	out, err := pb.Build(promptCoding, data)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	checks := []string{
		"@开发者",
		"/tmp/coding_result.json",
		"Round: 2/3",
		"fix bug",
		"tests pass",
		"please add tests",
	}
	for _, s := range checks {
		if !strings.Contains(out, s) {
			t.Errorf("output missing %q", s)
		}
	}
}

func TestPromptBuilder_Build_Review(t *testing.T) {
	pb, err := NewPromptBuilder("")
	if err != nil {
		t.Fatalf("NewPromptBuilder() error: %v", err)
	}

	data := PromptData{
		ReviewerAgent:         "QA负责人",
		ResultFilePath:        "/tmp/review_result.json",
		WorkerID:              "worker-1",
		Repo:                  "owner/repo",
		IssueNumber:           7,
		IssueURL:              "https://github.com/owner/repo/issues/7",
		PlatformName:          "github",
		Round:                 1,
		MaxRounds:             3,
		ConsecutiveUnknowns:   2,
		PlatformGuide:         "guide",
		IssueSummary:          "summary",
		Acceptance:            "acceptance",
		CodingSummary:         "coding summary",
		CodingEvidence:        "evidence",
		CodingAcceptanceCheck: "self check",
	}

	out, err := pb.Build(promptReview, data)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	checks := []string{
		"@QA负责人",
		"/tmp/review_result.json",
		"Round: 1/3",
		"Consecutive UNKNOWN Before This Round: 2",
		"coding summary",
		"evidence",
		"self check",
	}
	for _, s := range checks {
		if !strings.Contains(out, s) {
			t.Errorf("output missing %q", s)
		}
	}
}

func TestPromptBuilder_Build_IssuePost(t *testing.T) {
	pb, err := NewPromptBuilder("")
	if err != nil {
		t.Fatalf("NewPromptBuilder() error: %v", err)
	}

	data := PromptData{
		IssuePostAgent:   "产品经理",
		SourceFilePath:   "/tmp/coding_result.json",
		ResultFilePath:   "/tmp/issue_post_result.json",
		WorkerID:         "worker-1",
		Repo:             "owner/repo",
		IssueNumber:      7,
		IssueURL:         "https://github.com/owner/repo/issues/7",
		PlatformName:     "github",
		Purpose:          "completion",
		PlatformGuide:    "guide",
		PreviousFeedback: "prev feedback",
	}

	out, err := pb.Build(promptIssuePost, data)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	checks := []string{
		"@产品经理",
		"/tmp/coding_result.json",
		"/tmp/issue_post_result.json",
		"Purpose: completion",
		"prev feedback",
	}
	for _, s := range checks {
		if !strings.Contains(out, s) {
			t.Errorf("output missing %q", s)
		}
	}
}

func TestPromptBuilder_Build_LoopJudge(t *testing.T) {
	pb, err := NewPromptBuilder("")
	if err != nil {
		t.Fatalf("NewPromptBuilder() error: %v", err)
	}

	data := PromptData{
		HistoryFilePath: "/tmp/loop_history.json",
		ResultFilePath:  "/tmp/loop_judge_result.json",
		WorkerID:        "worker-1",
		Repo:            "owner/repo",
		IssueNumber:     7,
		IssueURL:        "https://github.com/owner/repo/issues/7",
		PlatformName:    "github",
		Round:           2,
		MaxRounds:       3,
		PlatformGuide:   "guide",
	}

	out, err := pb.Build(promptLoopJudge, data)
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}

	checks := []string{
		"/tmp/loop_history.json",
		"/tmp/loop_judge_result.json",
		"Round: 2/3",
	}
	for _, s := range checks {
		if !strings.Contains(out, s) {
			t.Errorf("output missing %q", s)
		}
	}
}

func TestPromptBuilder_Build_UnknownTemplate(t *testing.T) {
	pb, err := NewPromptBuilder("")
	if err != nil {
		t.Fatalf("NewPromptBuilder() error: %v", err)
	}

	_, err = pb.Build("nonexistent", PromptData{})
	if err == nil {
		t.Fatal("expected error for unknown template, got nil")
	}
	if !strings.Contains(err.Error(), "unknown template") {
		t.Errorf("error should mention unknown template, got: %v", err)
	}
}

func TestMapTemplateToKind(t *testing.T) {
	cases := []struct {
		template string
		want     string
	}{
		{promptIssueHandling, "issue-handling"},
		{promptCoding, "coding"},
		{promptReview, "review"},
		{promptIssuePost, "issue-post"},
		{promptLoopJudge, "loop-judge"},
		{"unknown", "unknown"},
	}

	for _, c := range cases {
		got := mapTemplateToKind(c.template)
		if got != c.want {
			t.Errorf("mapTemplateToKind(%q) = %q, want %q", c.template, got, c.want)
		}
	}
}

// TestNewPromptBuilder_OverlayExternalDir 验证外部目录的 .md 模板会覆盖同名内置模板，
// 而未覆盖的模板仍用内置默认。
func TestNewPromptBuilder_OverlayExternalDir(t *testing.T) {
	// 创建临时目录，写入自定义 coding.md
	tempDir := t.TempDir()
	customContent := "自定义 coding 模板：{{.Repo}} #{{.IssueNumber}}"
	if err := os.WriteFile(filepath.Join(tempDir, "coding.md"), []byte(customContent), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error: %v", err)
	}

	pb, err := NewPromptBuilder(tempDir)
	if err != nil {
		t.Fatalf("NewPromptBuilder(%q) error: %v", tempDir, err)
	}

	// coding 模板应被覆盖
	output, err := pb.Build(promptCoding, PromptData{Repo: "test/repo", IssueNumber: 7})
	if err != nil {
		t.Fatalf("Build(coding) error: %v", err)
	}
	if !strings.Contains(output, "自定义 coding 模板") {
		t.Errorf("coding template should be overridden, got: %s", output)
	}
	if !strings.Contains(output, "test/repo #7") {
		t.Errorf("coding template should render data, got: %s", output)
	}

	// review 模板应仍用内置默认（未被覆盖）
	reviewOutput, err := pb.Build(promptReview, PromptData{ReviewerAgent: "审查员"})
	if err != nil {
		t.Fatalf("Build(review) error: %v", err)
	}
	if !strings.Contains(reviewOutput, "审查") {
		t.Errorf("review template should use builtin, got: %s", reviewOutput)
	}
}

// TestNewPromptBuilder_ExternalDirNotFound 验证不存在的目录会返回错误。
func TestNewPromptBuilder_ExternalDirNotFound(t *testing.T) {
	_, err := NewPromptBuilder("/nonexistent/path/that/should/not/exist")
	if err == nil {
		t.Fatal("NewPromptBuilder() with nonexistent dir should return error")
	}
	if !strings.Contains(err.Error(), "read prompts dir") {
		t.Errorf("error should mention prompts dir, got: %v", err)
	}
}

// TestNewPromptBuilder_EmptyDirUsesBuiltinOnly 验证空字符串目录只加载内置模板。
func TestNewPromptBuilder_EmptyDirUsesBuiltinOnly(t *testing.T) {
	pb, err := NewPromptBuilder("")
	if err != nil {
		t.Fatalf("NewPromptBuilder(\"\") error: %v", err)
	}

	for _, name := range []string{promptIssueHandling, promptCoding, promptReview, promptIssuePost, promptLoopJudge} {
		if _, ok := pb.templates[name]; !ok {
			t.Errorf("builtin template %q not loaded", name)
		}
	}
}
