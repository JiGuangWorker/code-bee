// Package github 覆盖 GitHub 平台技能包实现的单元测试。
//
// 核心功能:
// 1. 验证 GitHub 平台名称、Issue URL 和技能说明文本的稳定性
// 2. 避免提示词中的仓库、Issue 与 GH 命令样例漂移，导致智能体拿到错误平台上下文
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-04
// 更新时间: 2026-07-04
package github

import (
	"strings"
	"testing"
)

// TestClientName 验证 GitHub 平台名称输出稳定。
func TestClientName(t *testing.T) {
	client := NewClient()

	if client.Name() != "GitHub" {
		t.Fatalf("Client.Name() = %q, want %q", client.Name(), "GitHub")
	}
}

// TestBuildIssueURL 验证 GitHub Issue URL 拼装符合 owner/repo 约定。
func TestBuildIssueURL(t *testing.T) {
	client := NewClient()
	got := client.BuildIssueURL("owner/repo", 12)
	want := "https://github.com/owner/repo/issues/12"
	if got != want {
		t.Fatalf("Client.BuildIssueURL() = %q, want %q", got, want)
	}
}

// TestBuildSkillInstructionContainsKeyContext 验证技能包说明包含关键命令与目标上下文。
func TestBuildSkillInstructionContainsKeyContext(t *testing.T) {
	client := NewClient()
	instruction := client.BuildSkillInstruction("owner/repo", 12)

	requiredFragments := []string{
		"当前平台为 GitHub",
		"https://github.com/owner/repo/issues/12",
		"gh issue view 12 --repo owner/repo",
		"gh issue comment 12 --repo owner/repo",
		"gh pr list --repo owner/repo",
		"gh pr create --repo owner/repo",
	}

	for _, fragment := range requiredFragments {
		if !strings.Contains(instruction, fragment) {
			t.Fatalf("BuildSkillInstruction() missing fragment %q", fragment)
		}
	}
}
