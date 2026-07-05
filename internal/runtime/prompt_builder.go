// 本文件实现 PromptBuilder，从 embed FS 加载 prompt 模板并渲染。
//
// 设计说明:
// - prompt 模板用 Go text/template 语法，命名参数替代位置敏感的 %s/%d
// - 模板文件位于 internal/runtime/prompts/*.md，通过 go:embed 打包
// - PromptData 结构体覆盖所有模板的字段需求，不同模板用不同子集
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-05

package runtime

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"
)

//go:embed prompts/*.md
var promptFS embed.FS

// 内置 prompt 模板名称。
const (
	promptIssueHandling = "issue-handling"
	promptCoding        = "coding"
	promptReview        = "review"
	promptIssuePost     = "issue-post"
	promptLoopJudge     = "loop-judge"
)

// PromptData 包含所有 prompt 模板可能用到的参数。
//
// 不同模板使用不同字段子集：
// - issue-handling: WorkerID, DefaultAgent, Repo, IssueNumber, IssueURL, PlatformName, PlatformGuide, ResultFilePath, PostFeedback
// - coding: Agent, ResultFilePath, WorkerID, Repo, IssueNumber, IssueURL, PlatformName, Round, MaxRounds, PlatformGuide, IssueSummary, Acceptance, ReviewerFeedback
// - review: ReviewerAgent, ResultFilePath, WorkerID, Repo, IssueNumber, IssueURL, PlatformName, Round, MaxRounds, ConsecutiveUnknowns, PlatformGuide, IssueSummary, Acceptance, CodingSummary, CodingEvidence, CodingAcceptanceCheck
// - issue-post: IssuePostAgent, SourceFilePath, ResultFilePath, WorkerID, Repo, IssueNumber, IssueURL, PlatformName, Purpose, PlatformGuide, PreviousFeedback
// - loop-judge: HistoryFilePath, ResultFilePath, WorkerID, Repo, IssueNumber, IssueURL, PlatformName, Round, MaxRounds, PlatformGuide
type PromptData struct {
	// 公共字段
	WorkerID      string
	IssueURL      string
	PlatformName  string
	PlatformGuide string
	Repo          string
	IssueNumber   int

	// 结果文件路径（多数模板都需要）
	ResultFilePath string

	// Issue handling 模板
	DefaultAgent string
	PostFeedback string

	// Coding 模板
	Agent            string
	Round            int
	MaxRounds        int
	ReviewerFeedback string
	IssueSummary     string
	Acceptance       string

	// Review 模板
	ReviewerAgent         string
	ConsecutiveUnknowns   int
	CodingSummary         string
	CodingEvidence        string
	CodingAcceptanceCheck string

	// Issue post 模板
	IssuePostAgent   string
	SourceFilePath   string
	Purpose          string
	PreviousFeedback string

	// Loop judge 模板
	HistoryFilePath string
}

// PromptBuilder 从 embed FS 加载 prompt 模板并渲染。
type PromptBuilder struct {
	templates map[string]*template.Template
}

// NewPromptBuilder 加载所有内置 prompt 模板。
func NewPromptBuilder() (*PromptBuilder, error) {
	templates := make(map[string]*template.Template)

	names := []string{
		promptIssueHandling,
		promptCoding,
		promptReview,
		promptIssuePost,
		promptLoopJudge,
	}

	for _, name := range names {
		path := "prompts/" + name + ".md"
		data, err := promptFS.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("runtime.NewPromptBuilder: load %s: %w", name, err)
		}

		tmpl, err := template.New(name).Parse(string(data))
		if err != nil {
			return nil, fmt.Errorf("runtime.NewPromptBuilder: parse %s: %w", name, err)
		}
		templates[name] = tmpl
	}

	return &PromptBuilder{templates: templates}, nil
}

// Build 渲染指定模板，返回最终 prompt 字符串。
func (b *PromptBuilder) Build(templateName string, data PromptData) (string, error) {
	tmpl, ok := b.templates[templateName]
	if !ok {
		return "", fmt.Errorf("runtime.PromptBuilder.Build: unknown template %q", templateName)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("runtime.PromptBuilder.Build: execute %q: %w", templateName, err)
	}

	return buf.String(), nil
}

// mapTemplateToKind 把 prompt 模板名映射到 agent.TaskKind。
//
// agent.Runner.Run 的 kind 参数仅用于错误归因，不影响执行逻辑，
// 但保持映射一致有助于日志和错误信息可读。
func mapTemplateToKind(templateName string) string {
	switch templateName {
	case promptIssueHandling:
		return "issue-handling"
	case promptCoding:
		return "coding"
	case promptReview:
		return "review"
	case promptIssuePost:
		return "issue-post"
	case promptLoopJudge:
		return "loop-judge"
	default:
		return "unknown"
	}
}
