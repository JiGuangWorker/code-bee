package github

import (
	"context"
	"fmt"

	"github.com/google/go-github/v68/github"

	"github.com/JiGuangWorker/code-bee/internal/platform"
)

// FetchIssue 获取指定仓库的 Issue 详情。
func (c *Client) FetchIssue(ctx context.Context, owner, repo string, number int) (*platform.Issue, error) {
	issue, _, err := c.client.Issues.Get(ctx, owner, repo, number)
	if err != nil {
		return nil, fmt.Errorf("github.FetchIssue(%s/%s#%d): %w", owner, repo, number, err)
	}

	result := &platform.Issue{
		Number: issue.GetNumber(),
		Title:  issue.GetTitle(),
		Body:   issue.GetBody(),
		State:  issue.GetState(),
		Author: issue.GetUser().GetLogin(),
	}

	for _, label := range issue.Labels {
		result.Labels = append(result.Labels, label.GetName())
	}

	return result, nil
}

// AddComment 在 Issue 下添加评论。
func (c *Client) AddComment(ctx context.Context, owner, repo string, number int, body string) error {
	comment := &github.IssueComment{Body: github.Ptr(body)}
	_, _, err := c.client.Issues.CreateComment(ctx, owner, repo, number, comment)
	if err != nil {
		return fmt.Errorf("github.AddComment(%s/%s#%d): %w", owner, repo, number, err)
	}

	return nil
}
