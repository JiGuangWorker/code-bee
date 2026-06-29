// Package github 提供 GitHub 平台的 Platform 接口实现。
package github

import (
	"context"
	"net/http"

	"github.com/google/go-github/v68/github"
	"golang.org/x/oauth2"
)

// Option 是 GitHub Client 的函数选项。
type Option func(*Client)

// WithHTTPClient 设置自定义 HTTP Client。
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) { c.httpClient = httpClient }
}

// Client 是 GitHub 平台的 Adapter，实现 platform.Platform 接口。
type Client struct {
	client     *github.Client
	httpClient *http.Client
}

// New 创建 GitHub Client，token 为必填参数。
func New(token string, opts ...Option) *Client {
	c := &Client{}

	for _, opt := range opts {
		opt(c)
	}

	if c.httpClient == nil {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		)
		c.httpClient = oauth2.NewClient(context.Background(), ts)
	}

	c.client = github.NewClient(c.httpClient)
	return c
}

// Name 返回平台标识。
func (c *Client) Name() string { return "github" }
