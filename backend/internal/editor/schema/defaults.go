// Package schema 提供标准 YAML Schema 的默认文档构造能力。
//
// 核心功能:
// 1. 将当前 CodeB 已稳定下来的默认编排事实收口为一份标准文档对象
// 2. 为后续 editor state 与 UI 层提供统一、可复用的默认输入，而不是散落在测试中的字符串样例
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-06
// 更新时间: 2026-07-06
package schema

const (
	// DefaultDocumentName 是默认编排文档的稳定名称。
	DefaultDocumentName = "default-issue-flow"

	// DefaultDocumentDescription 是默认编排文档的稳定说明。
	DefaultDocumentDescription = "CodeB 当前固定四阶段流程与 coder-reviewer loop"

	// DefaultStartNodeID 是默认编排图的起始节点 ID。
	DefaultStartNodeID = "issue_handling"

	// DefaultIssueHandlingNodeID 是 Issue 处理节点的稳定 ID。
	DefaultIssueHandlingNodeID = "issue_handling"

	// DefaultIssuePostIntakeNodeID 是接单回帖节点的稳定 ID。
	DefaultIssuePostIntakeNodeID = "issue_post_intake"

	// DefaultCodingNodeID 是编码主节点的稳定 ID。
	DefaultCodingNodeID = "coding_main"

	// DefaultReviewNodeID 是审查主节点的稳定 ID。
	DefaultReviewNodeID = "review_main"

	// DefaultIssuePostCompletionNodeID 是完成回帖节点的稳定 ID。
	DefaultIssuePostCompletionNodeID = "issue_post_completion"

	// DefaultLoopJudgeNodeID 是价值评估节点的稳定 ID。
	DefaultLoopJudgeNodeID = "loop_judge_main"

	// DefaultIssueHandlingArtifact 是 Issue 处理结果文件名。
	DefaultIssueHandlingArtifact = "issue_intake_result.json"

	// DefaultCodingArtifact 是编码结果文件名。
	DefaultCodingArtifact = "coding_result.json"

	// DefaultReviewArtifact 是审查结果文件名。
	DefaultReviewArtifact = "review_result.json"

	// DefaultIssuePostIntakeArtifact 是接单回帖结果文件名。
	DefaultIssuePostIntakeArtifact = "issue_post_result_intake.json"

	// DefaultIssuePostCompletionArtifact 是完成回帖结果文件名。
	DefaultIssuePostCompletionArtifact = "issue_post_result_completion.json"

	// DefaultIssuePostPatternArtifact 是通用 Issue 提交结果文件命名模式。
	DefaultIssuePostPatternArtifact = "issue_post_result_{purpose}.json"

	// DefaultLoopJudgeArtifact 是价值评估结果文件名。
	DefaultLoopJudgeArtifact = "loop_judge_result.json"

	// DefaultLoopHistoryArtifact 是循环历史聚合文件名。
	DefaultLoopHistoryArtifact = "loop_history.json"

	// DefaultMaxCodingReviewRounds 是默认 coder-reviewer 最大轮数。
	DefaultMaxCodingReviewRounds = 3

	// DefaultMaxConsecutiveUnknownReview 是默认连续 UNKNOWN 阈值。
	DefaultMaxConsecutiveUnknownReview = 2

	// DefaultMaxIssuePostAttempts 是默认 Issue 提交最大重试次数。
	DefaultMaxIssuePostAttempts = 2

	// DefaultLoopJudgeStartRound 是默认价值评估触发起始轮。
	DefaultLoopJudgeStartRound = 0

	// DefaultIssueHandlingPositionX 是 Issue 处理节点的默认画布横坐标。
	DefaultIssueHandlingPositionX = 120

	// DefaultIssueHandlingPositionY 是 Issue 处理节点的默认画布纵坐标。
	DefaultIssueHandlingPositionY = 120

	// DefaultIssuePostIntakePositionX 是接单回帖节点的默认画布横坐标。
	DefaultIssuePostIntakePositionX = 340

	// DefaultIssuePostIntakePositionY 是接单回帖节点的默认画布纵坐标。
	DefaultIssuePostIntakePositionY = 120

	// DefaultCodingPositionX 是编码节点的默认画布横坐标。
	DefaultCodingPositionX = 620

	// DefaultCodingPositionY 是编码节点的默认画布纵坐标。
	DefaultCodingPositionY = 120

	// DefaultReviewPositionX 是审查节点的默认画布横坐标。
	DefaultReviewPositionX = 900

	// DefaultReviewPositionY 是审查节点的默认画布纵坐标。
	DefaultReviewPositionY = 120

	// DefaultIssuePostCompletionPositionX 是完成回帖节点的默认画布横坐标。
	DefaultIssuePostCompletionPositionX = 1180

	// DefaultIssuePostCompletionPositionY 是完成回帖节点的默认画布纵坐标。
	DefaultIssuePostCompletionPositionY = 120

	// DefaultLoopJudgePositionX 是价值评估节点的默认画布横坐标。
	DefaultLoopJudgePositionX = 900

	// DefaultLoopJudgePositionY 是价值评估节点的默认画布纵坐标。
	DefaultLoopJudgePositionY = 320
)

// BuildDefaultDocument 构造一份可直接用于编辑器初始化的标准默认文档。
//
// 输入参数:
// - 无；该函数内部直接使用当前源码映射稿中已经明确的默认节点图、角色、工件命名和 runtime 参数
//
// 返回值:
// - *Document: 一份可直接参与 Validate、EncodeYAML、DecodeYAML、SaveFile/LoadFile 闭环的默认文档
//
// 核心实现逻辑:
// - 顶层结构固定为 codebee/v1 + Orchestration
// - workflow、runtime、roles、artifacts、ui 分别由独立 helper 构造，避免默认事实散落
// - 所有默认值严格对齐当前源码映射稿，不额外发明运行时语义
//
// 调用注意事项:
// - 返回对象仅作为编辑器的默认输入，不代表当前运行时会直接消费该文档
func BuildDefaultDocument() *Document {
	return &Document{
		APIVersion: APIVersion,
		Kind:       Kind,
		Metadata: Metadata{
			Name:        DefaultDocumentName,
			Description: DefaultDocumentDescription,
		},
		Workflow:  buildDefaultWorkflow(),
		Runtime:   buildDefaultRuntime(),
		Roles:     buildDefaultRoles(),
		Artifacts: buildDefaultArtifacts(),
		UI:        buildDefaultUI(),
	}
}

// buildDefaultWorkflow 构造默认节点图主体。
func buildDefaultWorkflow() Workflow {
	return Workflow{
		StartNodeID: DefaultStartNodeID,
		Nodes:       buildDefaultNodes(),
		Edges:       buildDefaultEdges(),
	}
}

// buildDefaultNodes 构造默认编排中的固定节点列表。
func buildDefaultNodes() []Node {
	return []Node{
		{
			ID:             DefaultIssueHandlingNodeID,
			Type:           NodeTypeIssueHandling,
			Name:           "Issue 处理",
			RoleRef:        RoleRefIssuePostAgent,
			PromptTemplate: PromptTemplateIssueHandling,
			ResultContract: ResultContractIssueHandling,
			Outputs:        []string{DefaultIssueHandlingArtifact},
		},
		{
			ID:             DefaultIssuePostIntakeNodeID,
			Type:           NodeTypeIssuePost,
			Name:           "接单回帖",
			RoleRef:        RoleRefIssuePostAgent,
			PromptTemplate: PromptTemplateIssuePost,
			ResultContract: ResultContractIssuePost,
			Inputs:         []string{DefaultIssueHandlingArtifact},
			Outputs:        []string{DefaultIssuePostIntakeArtifact},
		},
		{
			ID:             DefaultCodingNodeID,
			Type:           NodeTypeCoding,
			Name:           "编码实现",
			RoleRef:        RoleRefDefaultAgent,
			PromptTemplate: PromptTemplateCoding,
			ResultContract: ResultContractCoding,
			Inputs:         []string{DefaultIssueHandlingArtifact, DefaultReviewArtifact},
			Outputs:        []string{DefaultCodingArtifact},
		},
		{
			ID:             DefaultReviewNodeID,
			Type:           NodeTypeReview,
			Name:           "结果审查",
			RoleRef:        RoleRefReviewerAgent,
			PromptTemplate: PromptTemplateReview,
			ResultContract: ResultContractReview,
			Inputs:         []string{DefaultIssueHandlingArtifact, DefaultCodingArtifact},
			Outputs:        []string{DefaultReviewArtifact},
		},
		{
			ID:             DefaultIssuePostCompletionNodeID,
			Type:           NodeTypeIssuePost,
			Name:           "完成回帖",
			RoleRef:        RoleRefIssuePostAgent,
			PromptTemplate: PromptTemplateIssuePost,
			ResultContract: ResultContractIssuePost,
			Inputs:         []string{DefaultReviewArtifact},
			Outputs:        []string{DefaultIssuePostCompletionArtifact},
		},
		{
			ID:             DefaultLoopJudgeNodeID,
			Type:           NodeTypeLoopJudge,
			Name:           "价值评估",
			RoleRef:        RoleRefLoopJudgeAgent,
			PromptTemplate: PromptTemplateLoopJudge,
			ResultContract: ResultContractLoopJudge,
			Inputs:         []string{DefaultLoopHistoryArtifact},
			Outputs:        []string{DefaultLoopJudgeArtifact},
		},
	}
}

// buildDefaultEdges 构造默认节点图中的固定边列表。
func buildDefaultEdges() []Edge {
	return []Edge{
		{
			From: DefaultIssueHandlingNodeID,
			To:   DefaultIssuePostIntakeNodeID,
		},
		{
			From: DefaultIssuePostIntakeNodeID,
			To:   DefaultCodingNodeID,
			When: EdgeCondition{
				IssuePostStatus: IssuePostStatusPosted,
			},
		},
		{
			From: DefaultCodingNodeID,
			To:   DefaultReviewNodeID,
			When: EdgeCondition{
				CodingStatus: CodingStatusDone,
			},
		},
		{
			From: DefaultReviewNodeID,
			To:   DefaultIssuePostCompletionNodeID,
			When: EdgeCondition{
				ReviewStatus: ReviewStatusPass,
			},
		},
		{
			From: DefaultReviewNodeID,
			To:   DefaultCodingNodeID,
			When: EdgeCondition{
				ReviewStatus: ReviewStatusFail,
			},
		},
		{
			From: DefaultReviewNodeID,
			To:   DefaultCodingNodeID,
			When: EdgeCondition{
				ReviewStatus: ReviewStatusUnknown,
			},
		},
		{
			From: DefaultReviewNodeID,
			To:   DefaultLoopJudgeNodeID,
			When: EdgeCondition{
				ReviewStatus: ReviewStatusFail,
			},
		},
		{
			From: DefaultReviewNodeID,
			To:   DefaultLoopJudgeNodeID,
			When: EdgeCondition{
				ReviewStatus: ReviewStatusUnknown,
			},
		},
		{
			From: DefaultLoopJudgeNodeID,
			To:   DefaultCodingNodeID,
			When: EdgeCondition{
				LoopJudgeDecision: LoopJudgeDecisionContinue,
			},
		},
		{
			From: DefaultLoopJudgeNodeID,
			To:   DefaultCodingNodeID,
			When: EdgeCondition{
				LoopJudgeDecision: LoopJudgeDecisionShrinkTask,
			},
		},
	}
}

// buildDefaultRuntime 构造默认 runtime 参数块。
func buildDefaultRuntime() Runtime {
	return Runtime{
		MaxCodingReviewRounds:       DefaultMaxCodingReviewRounds,
		MaxConsecutiveUnknownReview: DefaultMaxConsecutiveUnknownReview,
		MaxIssuePostAttempts:        DefaultMaxIssuePostAttempts,
		LoopJudgeStartRound:         DefaultLoopJudgeStartRound,
	}
}

// buildDefaultRoles 构造默认角色配置块。
func buildDefaultRoles() Roles {
	return Roles{
		DefaultAgent:   "开发者",
		ReviewerAgent:  "QA负责人",
		IssuePostAgent: "产品经理",
		LoopJudgeAgent: "技术负责人",
	}
}

// buildDefaultArtifacts 构造默认工件命名配置块。
func buildDefaultArtifacts() Artifacts {
	return Artifacts{
		IssueHandlingResult: DefaultIssueHandlingArtifact,
		CodingResult:        DefaultCodingArtifact,
		ReviewResult:        DefaultReviewArtifact,
		IssuePostResult:     DefaultIssuePostPatternArtifact,
		LoopJudgeResult:     DefaultLoopJudgeArtifact,
		LoopHistory:         DefaultLoopHistoryArtifact,
	}
}

// buildDefaultUI 构造默认画布元数据。
func buildDefaultUI() UISettings {
	return UISettings{
		Nodes: map[string]NodeUI{
			DefaultIssueHandlingNodeID: {
				X: DefaultIssueHandlingPositionX,
				Y: DefaultIssueHandlingPositionY,
			},
			DefaultIssuePostIntakeNodeID: {
				X: DefaultIssuePostIntakePositionX,
				Y: DefaultIssuePostIntakePositionY,
			},
			DefaultCodingNodeID: {
				X: DefaultCodingPositionX,
				Y: DefaultCodingPositionY,
			},
			DefaultReviewNodeID: {
				X: DefaultReviewPositionX,
				Y: DefaultReviewPositionY,
			},
			DefaultIssuePostCompletionNodeID: {
				X: DefaultIssuePostCompletionPositionX,
				Y: DefaultIssuePostCompletionPositionY,
			},
			DefaultLoopJudgeNodeID: {
				X: DefaultLoopJudgePositionX,
				Y: DefaultLoopJudgePositionY,
			},
		},
	}
}
