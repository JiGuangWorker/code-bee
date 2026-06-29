package github

import (
	"context"
	"fmt"

	"github.com/google/go-github/v68/github"

	"github.com/JiGuangWorker/code-bee/internal/platform"
)

// CreatePR 创建分支、推送代码并创建 Pull Request。
func (c *Client) CreatePR(ctx context.Context, req platform.CreatePRRequest) (string, error) {
	base := req.Base
	if base == "" {
		base = "main"
	}

	newPR := &github.NewPullRequest{
		Title: github.Ptr(req.Title),
		Body:  github.Ptr(req.Body),
		Head:  github.Ptr(req.Branch),
		Base:  github.Ptr(base),
	}

	pr, _, err := c.client.PullRequests.Create(ctx, req.Owner, req.Repo, newPR)
	if err != nil {
		return "", fmt.Errorf("github.CreatePR(%s/%s): %w", req.Owner, req.Repo, err)
	}

	return pr.GetHTMLURL(), nil
}
