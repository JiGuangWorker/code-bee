// Package schema 定义编排编辑器使用的标准 YAML Schema 数据模型。
//
// 核心功能:
// 1. 提供标准 YAML Schema 在 Go 内存中的稳定结构，作为“YAML <-> 编辑器状态”之间的中间表达
// 2. 将源码中已经稳定存在的编排事实，抽象为编辑器可解析、可编辑、可重新组装的结构体
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-06
// 更新时间: 2026-07-06
package schema

const (
	// APIVersion 是当前标准 YAML Schema 的固定版本号。
	APIVersion = "codebee/v1"

	// Kind 是当前标准 YAML Schema 的固定种类标识。
	Kind = "Orchestration"

	// NodeTypeIssueHandling 表示 Issue 处理节点。
	NodeTypeIssueHandling = "issue-handling"

	// NodeTypeCoding 表示编码节点。
	NodeTypeCoding = "coding"

	// NodeTypeReview 表示审查节点。
	NodeTypeReview = "review"

	// NodeTypeIssuePost 表示 Issue 评论提交节点。
	NodeTypeIssuePost = "issue-post"

	// NodeTypeLoopJudge 表示价值评估节点。
	NodeTypeLoopJudge = "loop-judge"

	// PromptTemplateIssueHandling 表示 Issue 处理阶段的提示词模板标识。
	PromptTemplateIssueHandling = "issue-handling"

	// PromptTemplateCoding 表示编码阶段的提示词模板标识。
	PromptTemplateCoding = "coding"

	// PromptTemplateReview 表示审查阶段的提示词模板标识。
	PromptTemplateReview = "review"

	// PromptTemplateIssuePost 表示 Issue 提交阶段的提示词模板标识。
	PromptTemplateIssuePost = "issue-post"

	// PromptTemplateLoopJudge 表示价值评估阶段的提示词模板标识。
	PromptTemplateLoopJudge = "loop-judge"

	// ResultContractIssueHandling 表示 Issue 处理结果契约标识。
	ResultContractIssueHandling = "issue-handling-result"

	// ResultContractCoding 表示编码结果契约标识。
	ResultContractCoding = "coding-result"

	// ResultContractReview 表示审查结果契约标识。
	ResultContractReview = "review-result"

	// ResultContractIssuePost 表示 Issue 提交结果契约标识。
	ResultContractIssuePost = "issue-post-result"

	// ResultContractLoopJudge 表示价值评估结果契约标识。
	ResultContractLoopJudge = "loop-judge-result"

	// RoleRefDefaultAgent 表示编码主角色引用键。
	RoleRefDefaultAgent = "defaultAgent"

	// RoleRefReviewerAgent 表示审查角色引用键。
	RoleRefReviewerAgent = "reviewerAgent"

	// RoleRefIssuePostAgent 表示 Issue 提交角色引用键。
	RoleRefIssuePostAgent = "issuePostAgent"

	// RoleRefLoopJudgeAgent 表示价值评估角色引用键。
	RoleRefLoopJudgeAgent = "loopJudgeAgent"

	// IssueStatusReady 表示当前 Issue 已准备好进入开发。
	IssueStatusReady = "READY"

	// IssueStatusBlocked 表示当前 Issue 因外部条件不足而阻塞。
	IssueStatusBlocked = "BLOCKED"

	// CodingStatusDone 表示本轮编码工作已准备好接受审查。
	CodingStatusDone = "DONE"

	// CodingStatusInProgress 表示本轮编码还未结束。
	CodingStatusInProgress = "IN_PROGRESS"

	// CodingStatusBlocked 表示本轮编码因外部条件不足而阻塞。
	CodingStatusBlocked = "BLOCKED"

	// ReviewStatusPass 表示审查通过。
	ReviewStatusPass = "PASS"

	// ReviewStatusFail 表示审查明确失败。
	ReviewStatusFail = "FAIL"

	// ReviewStatusUnknown 表示审查暂时无法判断。
	ReviewStatusUnknown = "UNKNOWN"

	// ReviewStatusBlocked 表示审查因外部条件不足而阻塞。
	ReviewStatusBlocked = "BLOCKED"

	// IssuePostStatusPosted 表示 Issue 评论已成功提交。
	IssuePostStatusPosted = "POSTED"

	// IssuePostStatusRejected 表示 Issue 评论内容不合格，被拒绝提交。
	IssuePostStatusRejected = "REJECTED"

	// IssuePostStatusBlocked 表示 Issue 提交受外部环境阻塞。
	IssuePostStatusBlocked = "BLOCKED"

	// LoopJudgeDecisionContinue 表示自动循环继续执行。
	LoopJudgeDecisionContinue = "CONTINUE"

	// LoopJudgeDecisionShrinkTask 表示自动循环继续，但下一轮应收缩任务。
	LoopJudgeDecisionShrinkTask = "SHRINK_TASK"

	// LoopJudgeDecisionStopManual 表示应立即停止自动循环，转人工接管。
	LoopJudgeDecisionStopManual = "STOP_MANUAL"

	// LoopJudgeDecisionStopBlocked 表示应立即停止自动循环，转阻塞状态。
	LoopJudgeDecisionStopBlocked = "STOP_BLOCKED"
)

// Document 表示标准 YAML Schema 的完整文档结构。
//
// 输入参数:
// - 无；该结构体由 YAML 解码产生，或由调用方在内存中组装
//
// 返回值:
// - 无；作为顶层数据容器承载 metadata、workflow、runtime、roles、artifacts、ui 六块信息
//
// 核心实现逻辑:
// - workflow 保存节点图主体
// - runtime 保存运行参数
// - roles / artifacts 保存编辑器需要展示和引用的受控配置
// - ui 保存纯展示元数据，禁止承载运行时语义
//
// 调用注意事项:
// - Document 只用于编辑器 YAML 解析与组装，不应用于现有调度主流程
type Document struct {
	APIVersion string     `yaml:"apiVersion"`
	Kind       string     `yaml:"kind"`
	Metadata   Metadata   `yaml:"metadata"`
	Workflow   Workflow   `yaml:"workflow"`
	Runtime    Runtime    `yaml:"runtime"`
	Roles      Roles      `yaml:"roles"`
	Artifacts  Artifacts  `yaml:"artifacts"`
	UI         UISettings `yaml:"ui,omitempty"`
}

// Metadata 表示编排文档的元信息。
type Metadata struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
}

// Workflow 表示节点图主体。
type Workflow struct {
	StartNodeID string `yaml:"startNodeId"`
	Nodes       []Node `yaml:"nodes"`
	Edges       []Edge `yaml:"edges"`
}

// Node 表示单个编排节点。
type Node struct {
	ID             string   `yaml:"id"`
	Type           string   `yaml:"type"`
	Name           string   `yaml:"name"`
	RoleRef        string   `yaml:"roleRef"`
	PromptTemplate string   `yaml:"promptTemplate"`
	ResultContract string   `yaml:"resultContract"`
	Inputs         []string `yaml:"inputs,omitempty"`
	Outputs        []string `yaml:"outputs"`
}

// Edge 表示节点之间的有向连线。
type Edge struct {
	From string        `yaml:"from"`
	To   string        `yaml:"to"`
	When EdgeCondition `yaml:"when,omitempty"`
}

// EdgeCondition 表示受控边条件。
//
// 设计说明:
// - 当前只允许 5 类受控条件字段，禁止引入任意表达式
// - 一个条件块内最多只允许设置 1 个字段，避免语义歧义
type EdgeCondition struct {
	IssueStatus       string `yaml:"issueStatus,omitempty"`
	CodingStatus      string `yaml:"codingStatus,omitempty"`
	ReviewStatus      string `yaml:"reviewStatus,omitempty"`
	IssuePostStatus   string `yaml:"issuePostStatus,omitempty"`
	LoopJudgeDecision string `yaml:"loopJudgeDecision,omitempty"`
}

// Runtime 表示与循环相关的运行参数块。
type Runtime struct {
	MaxCodingReviewRounds       int `yaml:"maxCodingReviewRounds"`
	MaxConsecutiveUnknownReview int `yaml:"maxConsecutiveUnknownReview"`
	MaxIssuePostAttempts        int `yaml:"maxIssuePostAttempts"`
	LoopJudgeStartRound         int `yaml:"loopJudgeStartRound"`
}

// Roles 表示标准 YAML 中固定暴露给编辑器的角色配置。
type Roles struct {
	DefaultAgent   string `yaml:"defaultAgent"`
	ReviewerAgent  string `yaml:"reviewerAgent"`
	IssuePostAgent string `yaml:"issuePostAgent"`
	LoopJudgeAgent string `yaml:"loopJudgeAgent"`
}

// Artifacts 表示编辑器展示和组装时需要引用的工件命名约定。
type Artifacts struct {
	IssueHandlingResult string `yaml:"issueHandlingResult"`
	CodingResult        string `yaml:"codingResult"`
	ReviewResult        string `yaml:"reviewResult"`
	IssuePostResult     string `yaml:"issuePostResultPattern"`
	LoopJudgeResult     string `yaml:"loopJudgeResult"`
	LoopHistory         string `yaml:"loopHistory"`
}

// UISettings 表示编辑器专用的 UI 元数据块。
type UISettings struct {
	Nodes map[string]NodeUI `yaml:"nodes,omitempty"`
}

// NodeUI 表示单个节点在画布中的展示位置。
type NodeUI struct {
	X int `yaml:"x"`
	Y int `yaml:"y"`
}
