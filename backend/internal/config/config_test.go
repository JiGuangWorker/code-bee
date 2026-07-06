// Package config 覆盖运行时默认配置与内置角色的单元测试。
//
// 核心功能:
// 1. 验证默认配置是否按预期初始化，避免 loop 参数和角色默认值漂移
// 2. 验证内置角色查询与名称列表输出是否稳定，避免提示词引用角色时失配
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-04
// 更新时间: 2026-07-04
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

	if cfg.DefaultAgent != "开发者" {
		t.Fatalf("Config.DefaultAgent = %q, want %q", cfg.DefaultAgent, "开发者")
	}

	if cfg.ReviewerAgent != "QA负责人" {
		t.Fatalf("Config.ReviewerAgent = %q, want %q", cfg.ReviewerAgent, "QA负责人")
	}

	if cfg.IssuePostAgent != "产品经理" {
		t.Fatalf("Config.IssuePostAgent = %q, want %q", cfg.IssuePostAgent, "产品经理")
	}

	if cfg.LoopJudgeAgent != "技术负责人" {
		t.Fatalf("Config.LoopJudgeAgent = %q, want %q", cfg.LoopJudgeAgent, "技术负责人")
	}

	if cfg.MaxCodingReviewRounds != defaultMaxCodingReviewRounds {
		t.Fatalf("Config.MaxCodingReviewRounds = %d, want %d", cfg.MaxCodingReviewRounds, defaultMaxCodingReviewRounds)
	}

	if cfg.LoopJudgeStartRound != defaultLoopJudgeStartRound {
		t.Fatalf("Config.LoopJudgeStartRound = %d, want %d", cfg.LoopJudgeStartRound, defaultLoopJudgeStartRound)
	}
}

// TestAgentByNameFoundAndMissing 验证内置角色查询在命中和未命中场景下的返回值。
func TestAgentByNameFoundAndMissing(t *testing.T) {
	agentRole, ok := AgentByName("开发者")
	if !ok {
		t.Fatal("AgentByName(开发者) ok = false, want true")
	}

	if agentRole.SkillPath != "开发者Skill/SKILL.md" {
		t.Fatalf("AgentByName(开发者).SkillPath = %q, want %q", agentRole.SkillPath, "开发者Skill/SKILL.md")
	}

	if _, ok := AgentByName("不存在的角色"); ok {
		t.Fatal("AgentByName(不存在的角色) ok = true, want false")
	}
}

// TestAgentNamesContainsBuiltinRoles 验证角色名称列表与内置角色数量一致，且包含关键角色。
func TestAgentNamesContainsBuiltinRoles(t *testing.T) {
	names := AgentNames()
	agents := BuiltinAgents()

	if len(names) != len(agents) {
		t.Fatalf("len(AgentNames()) = %d, want %d", len(names), len(agents))
	}

	requiredNames := map[string]bool{
		"开发者":   false,
		"技术负责人": false,
		"QA负责人": false,
		"产品经理":  false,
	}

	for _, name := range names {
		if _, ok := requiredNames[name]; ok {
			requiredNames[name] = true
		}
	}

	for name, found := range requiredNames {
		if !found {
			t.Fatalf("AgentNames() missing required role %q", name)
		}
	}
}
