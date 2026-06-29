// Package pipeline 编排完整的 Issue → Agent → PR 流水线。
package pipeline

import (
	"context"
	"fmt"
	"log"

	"github.com/JiGuangWorker/code-bee/internal/agent"
	"github.com/JiGuangWorker/code-bee/internal/config"
	"github.com/JiGuangWorker/code-bee/internal/parser"
	"github.com/JiGuangWorker/code-bee/internal/platform"
)

// Run 执行完整的流水线。
//
// 流程：
//  1. 加载配置，识别平台
//  2. 读取 Issue 内容
//  3. 解析 Issue → 提取 Agent + 任务
//  4. 在 Issue 下回复"开始执行"
//  5. 启动 Agent 执行任务
//  6. 校验通过后，推分支 + 创建 PR
//  7. 在 Issue 下回复结果
func Run(ctx context.Context, repo string, issueNumber int) error {
	// 1. 加载配置
	cfg, err := config.Load(repo, issueNumber)
	if err != nil {
		return fmt.Errorf("pipeline.Run: load config: %w", err)
	}

	// 2. 读取 Issue
	owner, repoName := splitRepo(repo)
	issue, err := cfg.Platform.FetchIssue(ctx, owner, repoName, issueNumber)
	if err != nil {
		return fmt.Errorf("pipeline.Run: fetch issue: %w", err)
	}

	// 3. 解析 Issue → 提取任务
	task, err := parser.Parse(issue.Title, issue.Body)
	if err != nil {
		return fmt.Errorf("pipeline.Run: parse issue: %w", err)
	}

	log.Printf("📋 任务已解析: Agent=%s, 标题=%s", task.AgentName, task.Title)

	// 4. 回复"开始执行"
	startMsg := fmt.Sprintf("🤖 @agent-%s 收到任务，开始执行...", task.AgentName)
	if err := cfg.Platform.AddComment(ctx, owner, repoName, issueNumber, startMsg); err != nil {
		return fmt.Errorf("pipeline.Run: add start comment: %w", err)
	}

	// 5. 启动 Agent 执行任务
	runner := agent.New(task.AgentName)
	runTask := fmt.Sprintf("%s\n\n%s", task.Title, task.Body)
	result, err := runner.Run(ctx, runTask)
	if err != nil {
		// 失败时也回复 Issue
		failMsg := fmt.Sprintf("❌ 执行失败: %v", err)
		_ = cfg.Platform.AddComment(ctx, owner, repoName, issueNumber, failMsg)
		return fmt.Errorf("pipeline.Run: agent run: %w", err)
	}

	log.Printf("✅ Agent 执行完成: Success=%v", result.Success)

	// 6. 创建 PR
	branchName := fmt.Sprintf("agent/%s/issue-%d", task.AgentName, issueNumber)
	prTitle := fmt.Sprintf("[Agent:%s] %s", task.AgentName, task.Title)
	prBody := fmt.Sprintf("由 @agent-%s 执行 Issue #%d\n\n---\n\n%s", task.AgentName, issueNumber, task.Body)

	prURL, err := cfg.Platform.CreatePR(ctx, platform.CreatePRRequest{
		Owner:  owner,
		Repo:   repoName,
		Branch: branchName,
		Title:  prTitle,
		Body:   prBody,
	})
	if err != nil {
		return fmt.Errorf("pipeline.Run: create PR: %w", err)
	}

	// 7. 回复结果
	doneMsg := fmt.Sprintf("✅ 任务完成！\n- Agent: @agent-%s\n- PR: %s", task.AgentName, prURL)
	if err := cfg.Platform.AddComment(ctx, owner, repoName, issueNumber, doneMsg); err != nil {
		return fmt.Errorf("pipeline.Run: add done comment: %w", err)
	}

	log.Printf("🎉 流水线执行完成: PR=%s", prURL)
	return nil
}

// splitRepo 将 "owner/repo" 拆分为 owner 和 repo。
func splitRepo(fullRepo string) (owner, repo string) {
	for i := len(fullRepo) - 1; i >= 0; i-- {
		if fullRepo[i] == '/' {
			return fullRepo[:i], fullRepo[i+1:]
		}
	}
	return "", fullRepo
}
