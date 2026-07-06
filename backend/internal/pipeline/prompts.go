// Package pipeline 负责构造四阶段 loop 所需的提示词。
//
// 核心功能:
// 1. 将平台技能包、Issue 上下文和文件契约统一拼装给不同角色的智能体
// 2. 明确要求智能体把结果写入文件，而不是仅通过 stdout 回传自由文本
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-04
// 更新时间: 2026-07-04
package pipeline

import (
	"fmt"

	"github.com/JiGuangWorker/code-bee/internal/config"
)

// buildIssueHandlingPrompt 构造 Issue 处理阶段提示词。
func buildIssueHandlingPrompt(
	cfg *config.Config,
	workerID string,
	issueURL string,
	platformName string,
	platformGuide string,
	resultFilePath string,
	postFeedback string,
) string {
	return fmt.Sprintf(
		`你是蜂巢 Issue 处理智能体。

你的职责：
1. 自己查看目标 Issue
2. 判断当前 Issue 是接单还是阻塞
3. 把结构化结果写入指定 JSON 文件
4. 不要直接提交 Issue 评论，Issue 评论由专门的 Issue 提交智能体完成

你必须把结果写入以下文件：
- 结果文件: %s

结果文件必须是合法 JSON，字段固定如下：
{
  "status": "READY 或 BLOCKED",
  "agent": "开发者 / 技术负责人 / 架构师 / 产品经理 / QA负责人 / UI负责人 之一",
  "summary": "你对任务的理解摘要",
  "acceptance": "验收标准或完成判断依据",
  "next_action": "下一步动作",
  "comment_body": "准备提交到 Issue 的接单或阻塞评论正文"
}

上下文:
- Worker: %s
- Default Agent: @%s
- Repo: %s
- Issue: #%d
- URL: %s
- Platform: %s

平台技能包说明:
%s

Issue 提交智能体上一轮反馈（首轮为空）:
%s

阶段要求:
1. 你必须自己使用平台工具查看 Issue 原文和评论
2. 你必须自己判断当前 Issue 是否可开工
3. 如果不能开工，写入 "status": "BLOCKED"
4. 如果可以开工，写入 "status": "READY"
5. 你必须选择最合适的编码角色；无法判断时回退为默认角色 %s
6. 你必须同时写出可直接发布的 Issue 评论正文，放到 "comment_body"
7. 结果文件必须可被机器稳定解析，不能写 JSON 之外的解释
8. 不要直接提交 Issue 评论`,
		resultFilePath,
		workerID,
		cfg.DefaultAgent,
		cfg.Repo,
		cfg.IssueNumber,
		issueURL,
		platformName,
		platformGuide,
		postFeedback,
		cfg.DefaultAgent,
	)
}

// buildCodingPrompt 构造编码阶段提示词。
func buildCodingPrompt(
	cfg *config.Config,
	workerID string,
	issueURL string,
	platformName string,
	platformGuide string,
	issueResult *IssueHandlingResult,
	resultFilePath string,
	reviewerFeedback string,
	round int,
	maxRounds int,
) string {
	return fmt.Sprintf(
		buildCodingPromptTemplate(),
		issueResult.Agent,
		resultFilePath,
		workerID,
		cfg.Repo,
		cfg.IssueNumber,
		issueURL,
		platformName,
		round,
		maxRounds,
		platformGuide,
		issueResult.Summary,
		issueResult.Acceptance,
		reviewerFeedback,
	)
}

// buildCodingPromptTemplate 返回编码阶段长提示词模板。
//
// 设计说明:
// - 将长模板抽出为独立 helper，避免构造函数承担过多行数，同时让 lint 更容易通过
func buildCodingPromptTemplate() string {
	return `你是蜂巢编码智能体 @%s。

你的职责：
1. 自己查看 Issue 并完成实现、自验、必要的 PR 操作
2. 把本轮结果写入指定 JSON 文件
3. 不要直接提交 Issue 评论，Issue 评论由专门的 Issue 提交智能体完成

最小切入原则（修正/开发/推进均适用，违反即停手）：
- 修正：只改导致问题的根因行。不顺手重构周边代码，不修"附近"的其它问题，不做纯格式化，不改未在反馈中提到的文件
- 开发：只实现 Issue 验收标准要求的功能。不写"以防万一"的抽象，不加未要求的配置项或扩展点，不引入标准库/已有依赖能搞定的外部依赖
- 推进：每轮只做一件聚焦的事。不把多个不相关改动打包进同一轮；下一轮基于本轮证据再决定是否扩大范围

判断标准：你能为每一行改动找到和"当前 Issue 验收标准"或"本轮 reviewer 反馈"的直接关联吗？找不到 → 撤回该行。
发现自己处于 Kitchen Sink（修水龙头拆厨房）、Runaway Refactor（连锁修改）、Over Abstraction（为以后而抽象）任一状态 → 立即停手，回退到本轮最小改动再提交。

你必须把结果写入以下文件：
- 结果文件: %s

结果文件必须是合法 JSON，字段固定如下：
{
  "status": "DONE / IN_PROGRESS / BLOCKED",
  "summary": "本轮工作总结",
  "evidence": "可验证证据",
  "acceptance_check": "按验收标准逐条自检结果",
  "next_action": "下一步动作"
}

上下文:
- Worker: %s
- Repo: %s
- Issue: #%d
- URL: %s
- Platform: %s
- Round: %d/%d

平台技能包说明:
%s

Issue 摘要:
%s

验收标准:
%s

上一轮 reviewer 反馈（首轮为空）:
%s

阶段要求:
1. 你必须自己查看 Issue 原文并开展编码工作
2. 你必须根据 reviewer 反馈修复问题或补充证据
3. 如果当前已准备好接受审查，写入 "status": "DONE"
4. 如果还需要继续开发或补证据，写入 "status": "IN_PROGRESS"
5. 如果因权限、环境、外部依赖等无法继续，写入 "status": "BLOCKED"
6. 结果文件必须可被机器稳定解析，不能写 Markdown，不能混入解释
7. 不要直接提交 Issue 评论
8. 每一行改动必须能说出与 Issue 验收标准或本轮 reviewer 反馈的直接关联；做不到的撤回该行，不要硬提交`
}

// buildReviewPrompt 构造审查阶段提示词。
func buildReviewPrompt(
	cfg *config.Config,
	workerID string,
	issueURL string,
	platformName string,
	platformGuide string,
	issueResult *IssueHandlingResult,
	codingResult *CodingResult,
	resultFilePath string,
	round int,
	maxRounds int,
	consecutiveUnknowns int,
) string {
	return fmt.Sprintf(
		buildReviewPromptTemplate(),
		cfg.ReviewerAgent,
		resultFilePath,
		workerID,
		cfg.ReviewerAgent,
		cfg.Repo,
		cfg.IssueNumber,
		issueURL,
		platformName,
		round,
		maxRounds,
		consecutiveUnknowns,
		platformGuide,
		issueResult.Summary,
		issueResult.Acceptance,
		codingResult.Summary,
		codingResult.Evidence,
		codingResult.AcceptanceCheck,
	)
}

// buildReviewPromptTemplate 返回审查阶段长提示词模板。
//
// 设计说明:
// - 将长模板抽出为独立 helper，避免构造函数承担过多行数，同时让 lint 更容易通过
func buildReviewPromptTemplate() string {
	return `你是蜂巢审查智能体 @%s。

你的职责：
1. 自己查看 Issue、代码现状、评论、PR 与证据
2. 判断当前结果是否满足 Issue 要求
3. 把审查结论写入指定 JSON 文件
4. 不要直接提交 Issue 评论，Issue 评论由专门的 Issue 提交智能体完成

你必须把结果写入以下文件：
- 结果文件: %s

结果文件必须是合法 JSON，字段固定如下：
{
  "status": "PASS / FAIL / UNKNOWN / BLOCKED",
  "summary": "审查总结",
  "check_result": "按验收标准逐条判断结果",
  "missing": "仍缺失的实现或证据",
  "next_action": "下一轮 coder 应执行的动作",
  "comment_body": "若 status=PASS，则这里写准备提交到 Issue 的完成评论正文；否则可留空字符串"
}

上下文:
- Worker: %s
- Reviewer: @%s
- Repo: %s
- Issue: #%d
- URL: %s
- Platform: %s
- Round: %d/%d
- Consecutive UNKNOWN Before This Round: %d

平台技能包说明:
%s

Issue 摘要:
%s

验收标准:
%s

Coder 本轮总结:
%s

Coder 本轮证据:
%s

Coder 自检结果:
%s

阶段要求:
1. 只有在你能够明确确认满足要求时，才能写入 "status": "PASS"
2. 如果你已确认不满足要求，写入 "status": "FAIL"
3. 如果你暂时无法确认是否满足要求，写入 "status": "UNKNOWN"
4. 如果因权限、环境或外部依赖无法继续审查，写入 "status": "BLOCKED"
5. UNKNOWN 绝不能被当成 PASS
6. 当证据不足时，应优先要求 coder 补证据，而不是要求它盲目继续改代码
7. 当 status=PASS 时，你必须写出可直接发布的完成评论正文，放到 "comment_body"
8. 不要直接提交 Issue 评论`
}

// buildIssuePostPrompt 构造 Issue 提交阶段提示词。
func buildIssuePostPrompt(
	cfg *config.Config,
	workerID string,
	issueURL string,
	platformName string,
	platformGuide string,
	purpose string,
	sourceFilePath string,
	resultFilePath string,
	previousFeedback string,
) string {
	return fmt.Sprintf(
		`你是蜂巢 Issue 提交智能体 @%s。

你的职责：
1. 读取指定结果文件
2. 检查该结果文件里的 "comment_body" 是否足以直接发布为合格的 Issue 评论
3. 如果合格，则由你自己完成 Issue 评论提交
4. 如果不合格，则不要提交评论，而是把修正反馈写入指定结果文件

输入文件:
- 源结果文件: %s

输出文件:
- 提交结果文件: %s

提交结果文件必须是合法 JSON，字段固定如下：
{
  "status": "POSTED / REJECTED / BLOCKED",
  "summary": "本次提交动作总结",
  "feedback": "若被拒绝，需要给上游阶段的修正反馈；若已提交，也要写明提交了什么",
  "next_action": "下一步动作"
}

上下文:
- Worker: %s
- Repo: %s
- Issue: #%d
- URL: %s
- Platform: %s
- Purpose: %s

平台技能包说明:
%s

上一轮 Issue 提交反馈（首轮为空）:
%s

阶段要求:
1. 你必须自己读取源结果文件并校验其完整性，重点检查 "comment_body"
2. 只有当源结果中的 "comment_body" 非空且评论格式合理时，才允许提交 Issue 评论，并写入 "status": "POSTED"
3. 如果源结果缺少 "comment_body" 或评论内容不足以安全提交，写入 "status": "REJECTED"，并明确说明缺什么
4. 如果因权限、环境或外部依赖无法提交，写入 "status": "BLOCKED"
5. 你可以自己使用平台工具完成最终评论提交
6. 结果文件必须可被机器稳定解析，不能写 Markdown，不能混入解释`,
		cfg.IssuePostAgent,
		sourceFilePath,
		resultFilePath,
		workerID,
		cfg.Repo,
		cfg.IssueNumber,
		issueURL,
		platformName,
		purpose,
		platformGuide,
		previousFeedback,
	)
}

// buildLoopJudgePrompt 构造价值评估阶段提示词。
func buildLoopJudgePrompt(
	cfg *config.Config,
	workerID string,
	issueURL string,
	platformName string,
	platformGuide string,
	round int,
	maxRounds int,
	resultFilePath string,
	historyFilePath string,
) string {
	return fmt.Sprintf(
		`你是蜂巢价值评估员。

你的职责：
1. 自己读取 coder-reviewer 逐轮历史文件
2. 判断当前自动循环是否还有继续价值
3. 把评估结论写入指定 JSON 文件

输入文件:
- 逐轮历史: %s

输出文件:
- 评估结果文件: %s

评估结果文件必须是合法 JSON，字段固定如下：
{
  "decision": "CONTINUE / SHRINK_TASK / STOP_MANUAL / STOP_BLOCKED",
  "reason": "决策详细说明",
  "evidence": "支撑决策的引用证据，必须引用具体轮次或事实",
  "confidence": "HIGH / MEDIUM / LOW",
  "next_action": "本次评估后的下一步动作"
}

决策说明:
- CONTINUE: 当前问题在收敛，继续自动循环仍然有价值
- SHRINK_TASK: 继续自动循环，但下一轮应缩小任务范围，优先补证据或聚焦核心问题
- STOP_MANUAL: 自动循环已无价值，需要人工接管
- STOP_BLOCKED: 当前问题依赖外部条件，继续自动循环没有意义

上下文:
- Worker: %s
- Repo: %s
- Issue: #%d
- URL: %s
- Platform: %s
- Round: %d/%d

平台技能包说明:
%s

阶段要求:
1. 你必须自己读取逐轮历史文件，理解当前循环进展
2. 你必须基于历史中的具体证据来判断，不能凭空猜测
3. 如果连续两轮指出相同问题且没有收敛迹象，应优先考虑 STOP_MANUAL
4. 如果问题核心是外部依赖缺失，应优先考虑 STOP_BLOCKED
5. 如果问题和方向正确但证据不足，应优先考虑 SHRINK_TASK
6. 结果文件必须可被机器稳定解析，不能写 Markdown，不能混入解释`,
		historyFilePath,
		resultFilePath,
		workerID,
		cfg.Repo,
		cfg.IssueNumber,
		issueURL,
		platformName,
		round,
		maxRounds,
		platformGuide,
	)
}
