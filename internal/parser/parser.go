// Package parser 负责解析 Issue 内容，提取 Agent 任务信息。
package parser

import (
	"fmt"
	"regexp"
	"strings"
)

// Task 是从 Issue 中解析出的任务描述。
type Task struct {
	AgentName string   // 目标 Agent，如 "frontend"
	Title     string   // 任务标题
	Body      string   // 任务描述
	Checks    []string // 验收标准
}

// agentMention 匹配 @agent-xxx 格式，提取 Agent 名称。
var agentMention = regexp.MustCompile(`@agent-(\w+)`)

// Parse 从 Issue 标题和正文中提取任务信息。
func Parse(issueTitle, issueBody string) (*Task, error) {
	content := issueTitle + "\n" + issueBody

	matches := agentMention.FindStringSubmatch(content)
	if len(matches) < 2 {
		return nil, fmt.Errorf("未找到 Agent 指派指令，请在 Issue 中使用 @agent-<name> 格式指定 Agent")
	}

	task := &Task{
		AgentName: matches[1],
		Title:     issueTitle,
		Body:      issueBody,
		Checks:    extractChecks(issueBody),
	}

	return task, nil
}

// extractChecks 从 Issue 正文中提取验收标准（- [ ] 格式的 checklist）。
func extractChecks(body string) []string {
	var checks []string
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- [ ]") {
			checks = append(checks, strings.TrimPrefix(trimmed, "- [ ] "))
		}
	}
	return checks
}
