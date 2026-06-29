// Package config 提供统一的配置管理。
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/JiGuangWorker/code-bee/internal/platform"
	"github.com/JiGuangWorker/code-bee/internal/platform/github"
)

// Config 聚合所有运行时配置。
type Config struct {
	// Repo 仓库地址，如 "JiGuangWorker/DeepSeek-Reasonix"
	Repo string
	// IssueNumber Issue 编号
	IssueNumber int
	// Token 平台 API Token
	Token string
	// Platform 根据 Repo 自动识别的平台 Adapter
	Platform platform.Platform
}

// Load 从环境变量和参数加载配置，并自动识别平台。
func Load(repo string, issueNumber int) (*Config, error) {
	if repo == "" {
		return nil, errors.New("repo is required")
	}
	if issueNumber <= 0 {
		return nil, errors.New("issue number must be positive")
	}

	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, errors.New("GITHUB_TOKEN environment variable is required")
	}

	cfg := &Config{
		Repo:        repo,
		IssueNumber: issueNumber,
		Token:       token,
	}

	p, err := detectPlatform(cfg)
	if err != nil {
		return nil, err
	}
	cfg.Platform = p

	return cfg, nil
}

// detectPlatform 根据仓库地址自动识别并创建对应的 Platform 实例。
func detectPlatform(cfg *Config) (platform.Platform, error) {
	switch {
	case strings.Contains(cfg.Repo, "github.com"):
		return github.New(cfg.Token), nil
	default:
		return nil, fmt.Errorf("unsupported platform for repo: %s", cfg.Repo)
	}
}
