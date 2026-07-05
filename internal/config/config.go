// Package config 管理 code-bee 的运行时配置。
//
// 配置来源优先级：命令行参数 > 环境变量 > 默认值。
//
// 在场景驱动调度重构后，角色体系和循环参数完全由 workflow 配置
// （internal/runtime/default_workflow.yaml 或 --workflow 指定的自定义 YAML）
// 驱动，Config 只保留 Repo/IssueNumber/GitHubToken 三个真正运行时参数。
package config

import "os"

// Config 表示 code-bee 的运行时配置。
//
// 角色展示名（DefaultAgent/ReviewerAgent/IssuePostAgent）和循环参数
// （MaxCodingReviewRounds/LoopJudgeStartRound）已迁移至 workflow 配置，
// 由 runtime.lookupDisplayNameByPromptTemplate 和 loop_executor 从
// *schema.Workflow 派生，不再在此硬编码。
type Config struct {
	// Repo 是目标仓库，格式 owner/repo。
	Repo string

	// IssueNumber 是目标 Issue 编号。
	IssueNumber int

	// GitHubToken 用于访问 GitHub API。
	// 默认从环境变量 GITHUB_TOKEN 读取。
	GitHubToken string
}

// New 创建一个带有默认值的 Config。
func New(repo string, issueNumber int) *Config {
	return &Config{
		Repo:        repo,
		IssueNumber: issueNumber,
		GitHubToken: os.Getenv("GITHUB_TOKEN"),
	}
}
