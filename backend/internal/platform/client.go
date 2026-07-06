// Package platform 定义 code-bee 使用的平台技能包抽象。
//
// 核心功能:
// 1. 描述不同代码托管平台下，编码智能体可使用的工具与入口
// 2. 让 code-bee 只负责把平台说明交给智能体，而不控制具体平台操作
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-04
// 更新时间: 2026-07-04
package platform

// Client 定义统一的平台技能包接口。
//
// 调用约束:
// - 这里只描述“智能体可以怎么做”，不负责替智能体执行平台动作
// - Issue 如何理解、是否接单、如何回复，均由编码智能体自行完成
type Client interface {
	// Name 返回平台名称，便于在提示词中明确当前操作的平台上下文。
	Name() string

	// BuildIssueURL 返回当前 Issue 的直接入口链接，供智能体定位目标任务。
	BuildIssueURL(repo string, issueNumber int) string

	// BuildSkillInstruction 返回平台工具说明。
	// 这里不是命令执行器，而是给编码智能体阅读的技能说明。
	BuildSkillInstruction(repo string, issueNumber int) string
}
