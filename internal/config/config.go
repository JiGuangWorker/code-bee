// Package config 管理 code-bee 的运行时配置。
//
// 配置来源优先级：命令行参数 > 环境变量 > 默认值。
package config

import "os"

const (
	defaultMaxCodingReviewRounds = 10
	defaultLoopJudgeStartRound   = 3
)

// Config 表示 code-bee 的完整运行时配置。
type Config struct {
	// Repo 是目标仓库，格式 owner/repo。
	Repo string

	// IssueNumber 是目标 Issue 编号。
	IssueNumber int

	// GitHubToken 用于访问 GitHub API。
	// 默认从环境变量 GITHUB_TOKEN 读取。
	GitHubToken string

	// DefaultAgent 是当 Issue 中未发现 @agent-name 时使用的默认智能体。
	DefaultAgent string

	// ReviewerAgent 是 coder-reviewer loop 中默认的审查智能体。
	// 当前版本固定使用内置角色，避免主流程里散落硬编码字符串。
	ReviewerAgent string

	// IssuePostAgent 是专门负责 Issue 提交的智能体。
	// 该角色不直接编码，而是负责校验结果文件并完成最终评论提交。
	IssuePostAgent string

	// LoopJudgeAgent 是专门负责评估“继续自动循环是否还有价值”的智能体。
	// 该角色不判断业务实现细节是否正确，只基于多轮历史决定是否继续自动 loop。
	LoopJudgeAgent string

	// MaxCodingReviewRounds 是 coder-reviewer 外层循环允许执行的最大轮数。
	// 当轮数达到上限仍未通过时，调度器将停止自动循环并要求人工接管。
	MaxCodingReviewRounds int

	// LoopJudgeStartRound 是价值评估员开始介入的最小轮次。
	// 小于该值时不触发 loop judge，避免过早评估导致正常任务被打断。
	LoopJudgeStartRound int
}

// AgentRole 定义了一个可调度的智能体角色。
type AgentRole struct {
	// Name 是 @ 语法中的名称，如 "开发者"、"技术负责人"。
	Name string

	// SkillPath 是该角色对应的 Skill 文件路径（相对于 .skills/ 目录）。
	SkillPath string

	// Description 是角色的简短描述。
	Description string
}

// BuiltinAgents 返回内置的智能体角色列表。
// 这些角色与 .skills/ 目录中的 Skill 定义一一对应。
func BuiltinAgents() []AgentRole {
	return []AgentRole{
		{Name: "开发者", SkillPath: "开发者Skill/SKILL.md", Description: "负责将 Issue 转化为符合规范的代码"},
		{Name: "技术负责人", SkillPath: "技术负责人Skill/SKILL.md", Description: "负责工程团队的交付质量与效率"},
		{Name: "架构师", SkillPath: "架构师Skill/SKILL.md", Description: "负责数据结构设计与逻辑流程设计"},
		{Name: "产品经理", SkillPath: "产品经理Skill/SKILL.md", Description: "负责需求捕捉、PRD 编写与评审主持"},
		{Name: "QA负责人", SkillPath: "QA负责人Skill/SKILL.md", Description: "负责测试用例生成与验收放行"},
		{Name: "UI负责人", SkillPath: "UI负责人Skill/SKILL.md", Description: "负责生图与配音"},
	}
}

// AgentByName 根据名称查找内置智能体角色。
// 如果未找到，返回 false。
func AgentByName(name string) (AgentRole, bool) {
	for _, a := range BuiltinAgents() {
		if a.Name == name {
			return a, true
		}
	}
	return AgentRole{}, false
}

// AgentNames 返回所有内置智能体的名称列表。
func AgentNames() []string {
	agents := BuiltinAgents()
	names := make([]string, len(agents))
	for i, a := range agents {
		names[i] = a.Name
	}
	return names
}

// New 创建一个带有默认值的 Config。
func New(repo string, issueNumber int) *Config {
	return &Config{
		Repo:                  repo,
		IssueNumber:           issueNumber,
		GitHubToken:           os.Getenv("GITHUB_TOKEN"),
		DefaultAgent:          "开发者",
		ReviewerAgent:         "QA负责人",
		IssuePostAgent:        "产品经理",
		LoopJudgeAgent:        "技术负责人",
		MaxCodingReviewRounds: defaultMaxCodingReviewRounds,
		LoopJudgeStartRound:   defaultLoopJudgeStartRound,
	}
}
