// Package platform 定义代码托管平台的统一接口。
// 调用方（pipeline）定义接口，各平台（github/gitcode/...）实现。
package platform

import "context"

// Issue 表示一个平台 Issue，包含调度所需的关键信息。
type Issue struct {
	Number int
	Title  string
	Body   string
	State  string
	Labels []string
	Author string
}

// Platform 是代码托管平台的抽象接口。
// 由调用方（pipeline）定义，各平台实现各自的 Adapter。
type Platform interface {
	// Name 返回平台标识，如 "github"、"gitcode"。
	Name() string

	// FetchIssue 获取指定仓库的 Issue 详情。
	FetchIssue(ctx context.Context, owner, repo string, number int) (*Issue, error)

	// AddComment 在 Issue 下添加评论。
	AddComment(ctx context.Context, owner, repo string, number int, body string) error

	// CreatePR 创建分支、推送代码并创建 Pull Request。
	// 返回创建的 PR URL。
	CreatePR(ctx context.Context, prReq CreatePRRequest) (prURL string, err error)
}

// CreatePRRequest 是创建 PR 的请求参数。
type CreatePRRequest struct {
	Owner  string // 仓库所有者
	Repo   string // 仓库名
	Branch string // 源分支名
	Title  string // PR 标题
	Body   string // PR 描述
	Base   string // 目标分支，默认 "main"
}
