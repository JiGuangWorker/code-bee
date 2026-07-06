// Package config 覆盖运行时默认配置的单元测试。
//
// 核心功能:
// 验证 New 会返回稳定的默认配置（Repo/IssueNumber/GitHubToken）
//
// 注: 原 BuiltinAgents/AgentByName/AgentNames 测试已在场景驱动调度重构中删除，
// 角色体系完全由 workflow 配置驱动，不再需要内置角色查询。
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-04
// 更新时间: 2026-07-05
package config

import "testing"

// TestNewReturnsExpectedDefaults 验证 New 会返回稳定的默认配置。
func TestNewReturnsExpectedDefaults(t *testing.T) {
	cfg := New("owner/repo", 42)

	if cfg.Repo != "owner/repo" {
		t.Fatalf("Config.Repo = %q, want %q", cfg.Repo, "owner/repo")
	}

	if cfg.IssueNumber != 42 {
		t.Fatalf("Config.IssueNumber = %d, want %d", cfg.IssueNumber, 42)
	}

	// GitHubToken 来自环境变量，不在此断言具体值，只确认字段存在且不 panic
	_ = cfg.GitHubToken
}
