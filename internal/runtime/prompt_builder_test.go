package runtime

import (
	"strings"
	"testing"
)

func TestNewPromptBuilder_LoadsAllTemplates(t *testing.T) {
	pb, err := NewPromptBuilder()
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
	pb, err := NewPromptBuilder()
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
	pb, err := NewPromptBuilder()
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
	pb, err := NewPromptBuilder()
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
	pb, err := NewPromptBuilder()
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
	pb, err := NewPromptBuilder()
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
	pb, err := NewPromptBuilder()
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
