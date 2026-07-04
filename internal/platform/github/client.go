// Package github 提供 GitHub 平台技能包实现。
//
// 核心功能:
// 1. 向编码智能体说明 GitHub 平台下可使用的 GH 工具与入口
// 2. 保持 code-bee 只做调度，不直接执行 Issue 读取和评论动作
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-04
// 更新时间: 2026-07-04
package github

import (
	"fmt"

	"github.com/JiGuangWorker/code-bee/internal/platform"
)

// Client 封装 GitHub 平台技能包说明。
type Client struct{}

// NewClient 创建 GitHub 平台技能包实例。
//
// 返回值:
// - *Client: 可用于向编码智能体提供 GitHub 平台说明的技能包
//
// 注意事项:
// - 这里只生成提示词说明，不直接执行 gh 命令
func NewClient() *Client {
	return &Client{}
}

// Name 返回平台名称。
//
// 返回值:
// - string: 当前平台名称 "GitHub"
func (c *Client) Name() string {
	return "GitHub"
}

// BuildIssueURL 返回 GitHub Issue 的直接访问链接。
//
// 输入参数:
// - repo: 目标仓库，格式必须为 owner/repo
// - issueNumber: 目标 Issue 编号
//
// 返回值:
// - string: 可直接访问的 Issue URL
func (c *Client) BuildIssueURL(repo string, issueNumber int) string {
	return fmt.Sprintf("https://github.com/%s/issues/%d", repo, issueNumber)
}

// BuildSkillInstruction 返回 GitHub 平台下建议编码智能体使用的工具说明。
//
// 输入参数:
// - repo: 目标仓库，格式必须为 owner/repo
// - issueNumber: 目标 Issue 编号
//
// 返回值:
// - string: 给编码智能体阅读的平台工具说明
//
// 设计说明:
// - 这里只告诉智能体“可以怎么调用 GH”，具体命令执行由智能体自己完成
func (c *Client) BuildSkillInstruction(repo string, issueNumber int) string {
	issueURL := c.BuildIssueURL(repo, issueNumber)

	return fmt.Sprintf(
		`当前平台为 GitHub。

你可以自行使用 GitHub CLI (gh) 完成任务，不需要 code-bee 代替你操作。

建议入口:
- Issue URL: %s

常用命令示例:
- 查看 Issue: gh issue view %d --repo %s
- 查看评论: gh issue view %d --repo %s --comments
- 回复 Issue: gh issue comment %d --repo %s --body '...'
- 查看 PR: gh pr list --repo %s
- 创建 PR: gh pr create --repo %s --title '...' --body '...'

你需要自己决定:
- 是否接单
- 是否回复阻塞说明
- 如何实现与自验
- 何时创建或更新 PR`,
		issueURL,
		issueNumber, repo,
		issueNumber, repo,
		issueNumber, repo,
		repo,
		repo,
	)
}

var _ platform.Client = (*Client)(nil)
