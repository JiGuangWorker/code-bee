// Package schema 提供标准 YAML Schema 的静态校验逻辑。
//
// 核心功能:
// 1. 在编辑器加载 YAML 时尽早发现结构问题、枚举值错误和引用关系错误
// 2. 在编辑器保存 YAML 前阻断非法文档落盘，保证输出内容稳定、可预期
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-06
// 更新时间: 2026-07-06
package schema

import (
	"fmt"
	"strings"
)

// Validate 校验一份标准 YAML Schema 文档的最小完整性。
//
// 输入参数:
// - doc: 待校验的标准 YAML 文档对象
//
// 返回值:
// - error: 当顶层字段、节点、边、枚举、引用关系任一不合法时返回详细错误
//
// 核心实现逻辑:
// - 先校验顶层与 runtime、roles、artifacts 等稳定块
// - 再校验 workflow 节点图，确保 startNodeId、节点唯一性和边引用完整
// - 最后校验 UI 元数据，仅允许引用已存在节点
//
// 调用注意事项:
// - Validate 只做静态结构校验，不执行任何运行时行为
func Validate(doc *Document) error {
	if doc == nil {
		return fmt.Errorf("nil schema document")
	}

	if doc.APIVersion != APIVersion {
		return fmt.Errorf("invalid apiVersion %q", doc.APIVersion)
	}

	if doc.Kind != Kind {
		return fmt.Errorf("invalid kind %q", doc.Kind)
	}

	if strings.TrimSpace(doc.Metadata.Name) == "" {
		return fmt.Errorf("empty metadata.name")
	}

	if err := validateRuntime(doc.Runtime); err != nil {
		return err
	}

	if err := validateRoles(doc.Roles); err != nil {
		return err
	}

	if err := validateArtifacts(doc.Artifacts); err != nil {
		return err
	}

	nodeIndex, err := validateWorkflow(doc.Workflow)
	if err != nil {
		return err
	}

	if err := validateUI(doc.UI, nodeIndex); err != nil {
		return err
	}

	return nil
}

// validateRuntime 校验 runtime 配置块。
func validateRuntime(runtime Runtime) error {
	// 这些阈值都直接影响编辑器输出的 YAML 是否仍处于可用区间，因此统一在这里收口。
	if runtime.MaxCodingReviewRounds <= 0 {
		return fmt.Errorf("invalid runtime.maxCodingReviewRounds %d", runtime.MaxCodingReviewRounds)
	}

	if runtime.MaxConsecutiveUnknownReview <= 0 {
		return fmt.Errorf(
			"invalid runtime.maxConsecutiveUnknownReview %d",
			runtime.MaxConsecutiveUnknownReview,
		)
	}

	if runtime.MaxIssuePostAttempts <= 0 {
		return fmt.Errorf("invalid runtime.maxIssuePostAttempts %d", runtime.MaxIssuePostAttempts)
	}

	if runtime.LoopJudgeStartRound < 0 {
		return fmt.Errorf("invalid runtime.loopJudgeStartRound %d", runtime.LoopJudgeStartRound)
	}

	return nil
}

// validateRoles 校验 roles 配置块。
func validateRoles(roles Roles) error {
	if strings.TrimSpace(roles.DefaultAgent) == "" {
		return fmt.Errorf("empty roles.defaultAgent")
	}

	if strings.TrimSpace(roles.ReviewerAgent) == "" {
		return fmt.Errorf("empty roles.reviewerAgent")
	}

	if strings.TrimSpace(roles.IssuePostAgent) == "" {
		return fmt.Errorf("empty roles.issuePostAgent")
	}

	if strings.TrimSpace(roles.LoopJudgeAgent) == "" {
		return fmt.Errorf("empty roles.loopJudgeAgent")
	}

	return nil
}

// validateArtifacts 校验 artifacts 配置块。
func validateArtifacts(artifacts Artifacts) error {
	if strings.TrimSpace(artifacts.IssueHandlingResult) == "" {
		return fmt.Errorf("empty artifacts.issueHandlingResult")
	}

	if strings.TrimSpace(artifacts.CodingResult) == "" {
		return fmt.Errorf("empty artifacts.codingResult")
	}

	if strings.TrimSpace(artifacts.ReviewResult) == "" {
		return fmt.Errorf("empty artifacts.reviewResult")
	}

	if strings.TrimSpace(artifacts.IssuePostResult) == "" {
		return fmt.Errorf("empty artifacts.issuePostResultPattern")
	}

	if strings.TrimSpace(artifacts.LoopJudgeResult) == "" {
		return fmt.Errorf("empty artifacts.loopJudgeResult")
	}

	if strings.TrimSpace(artifacts.LoopHistory) == "" {
		return fmt.Errorf("empty artifacts.loopHistory")
	}

	return nil
}

// validateWorkflow 校验节点图主体，并返回节点索引用于后续边和 UI 校验。
func validateWorkflow(workflow Workflow) (map[string]struct{}, error) {
	if strings.TrimSpace(workflow.StartNodeID) == "" {
		return nil, fmt.Errorf("empty workflow.startNodeId")
	}

	if len(workflow.Nodes) == 0 {
		return nil, fmt.Errorf("empty workflow.nodes")
	}

	nodeIndex := make(map[string]struct{}, len(workflow.Nodes))
	for nodeIndexValue, node := range workflow.Nodes {
		if err := validateNode(node, nodeIndexValue); err != nil {
			return nil, err
		}

		if _, exists := nodeIndex[node.ID]; exists {
			return nil, fmt.Errorf("duplicate workflow node id %q", node.ID)
		}

		nodeIndex[node.ID] = struct{}{}
	}

	if _, exists := nodeIndex[workflow.StartNodeID]; !exists {
		return nil, fmt.Errorf("unknown workflow.startNodeId %q", workflow.StartNodeID)
	}

	for edgeIndex, edge := range workflow.Edges {
		if err := validateEdge(edge, edgeIndex, nodeIndex); err != nil {
			return nil, err
		}
	}

	return nodeIndex, nil
}

// validateNode 校验单个节点的结构完整性。
func validateNode(node Node, nodeIndex int) error {
	if strings.TrimSpace(node.ID) == "" {
		return fmt.Errorf("empty workflow.nodes[%d].id", nodeIndex)
	}

	if strings.TrimSpace(node.Name) == "" {
		return fmt.Errorf("empty workflow.nodes[%d].name", nodeIndex)
	}

	if !isValidNodeType(node.Type) {
		return fmt.Errorf("invalid workflow.nodes[%d].type %q", nodeIndex, node.Type)
	}

	if !isValidRoleRef(node.RoleRef) {
		return fmt.Errorf("invalid workflow.nodes[%d].roleRef %q", nodeIndex, node.RoleRef)
	}

	if !isValidPromptTemplate(node.PromptTemplate) {
		return fmt.Errorf(
			"invalid workflow.nodes[%d].promptTemplate %q",
			nodeIndex,
			node.PromptTemplate,
		)
	}

	if !isValidResultContract(node.ResultContract) {
		return fmt.Errorf(
			"invalid workflow.nodes[%d].resultContract %q",
			nodeIndex,
			node.ResultContract,
		)
	}

	if err := validateArtifactsList(node.ID, "inputs", node.Inputs); err != nil {
		return err
	}

	if len(node.Outputs) == 0 {
		return fmt.Errorf("empty workflow node %q outputs", node.ID)
	}

	if err := validateArtifactsList(node.ID, "outputs", node.Outputs); err != nil {
		return err
	}

	return nil
}

// validateArtifactsList 校验节点的输入或输出工件列表。
func validateArtifactsList(nodeID string, fieldName string, artifacts []string) error {
	for artifactIndex, artifact := range artifacts {
		if strings.TrimSpace(artifact) == "" {
			return fmt.Errorf(
				"empty workflow node %q %s[%d]",
				nodeID,
				fieldName,
				artifactIndex,
			)
		}
	}

	return nil
}

// validateEdge 校验单条边及其条件块。
func validateEdge(edge Edge, edgeIndex int, nodeIndex map[string]struct{}) error {
	if strings.TrimSpace(edge.From) == "" {
		return fmt.Errorf("empty workflow.edges[%d].from", edgeIndex)
	}

	if strings.TrimSpace(edge.To) == "" {
		return fmt.Errorf("empty workflow.edges[%d].to", edgeIndex)
	}

	if _, exists := nodeIndex[edge.From]; !exists {
		return fmt.Errorf("unknown workflow.edges[%d].from %q", edgeIndex, edge.From)
	}

	if _, exists := nodeIndex[edge.To]; !exists {
		return fmt.Errorf("unknown workflow.edges[%d].to %q", edgeIndex, edge.To)
	}

	if err := validateEdgeCondition(edge.When, edgeIndex); err != nil {
		return err
	}

	return nil
}

// validateEdgeCondition 校验受控边条件块。
func validateEdgeCondition(condition EdgeCondition, edgeIndex int) error {
	setCount := 0

	if strings.TrimSpace(condition.IssueStatus) != "" {
		setCount++
		if !isOneOf(condition.IssueStatus, IssueStatusReady, IssueStatusBlocked) {
			return fmt.Errorf(
				"invalid workflow.edges[%d].when.issueStatus %q",
				edgeIndex,
				condition.IssueStatus,
			)
		}
	}

	if strings.TrimSpace(condition.CodingStatus) != "" {
		setCount++
		if !isOneOf(condition.CodingStatus, CodingStatusDone, CodingStatusInProgress, CodingStatusBlocked) {
			return fmt.Errorf(
				"invalid workflow.edges[%d].when.codingStatus %q",
				edgeIndex,
				condition.CodingStatus,
			)
		}
	}

	if strings.TrimSpace(condition.ReviewStatus) != "" {
		setCount++
		if !isOneOf(
			condition.ReviewStatus,
			ReviewStatusPass,
			ReviewStatusFail,
			ReviewStatusUnknown,
			ReviewStatusBlocked,
		) {
			return fmt.Errorf(
				"invalid workflow.edges[%d].when.reviewStatus %q",
				edgeIndex,
				condition.ReviewStatus,
			)
		}
	}

	if strings.TrimSpace(condition.IssuePostStatus) != "" {
		setCount++
		if !isOneOf(
			condition.IssuePostStatus,
			IssuePostStatusPosted,
			IssuePostStatusRejected,
			IssuePostStatusBlocked,
		) {
			return fmt.Errorf(
				"invalid workflow.edges[%d].when.issuePostStatus %q",
				edgeIndex,
				condition.IssuePostStatus,
			)
		}
	}

	if strings.TrimSpace(condition.LoopJudgeDecision) != "" {
		setCount++
		if !isOneOf(
			condition.LoopJudgeDecision,
			LoopJudgeDecisionContinue,
			LoopJudgeDecisionShrinkTask,
			LoopJudgeDecisionStopManual,
			LoopJudgeDecisionStopBlocked,
		) {
			return fmt.Errorf(
				"invalid workflow.edges[%d].when.loopJudgeDecision %q",
				edgeIndex,
				condition.LoopJudgeDecision,
			)
		}
	}

	// 允许无条件边；但一旦写了条件，就必须保持“单条件字段”约束，避免出现多条件混写。
	if setCount > 1 {
		return fmt.Errorf("workflow.edges[%d].when has multiple condition fields", edgeIndex)
	}

	return nil
}

// validateUI 校验 UI 元数据块，确保它只引用已存在节点。
func validateUI(ui UISettings, nodeIndex map[string]struct{}) error {
	for nodeID := range ui.Nodes {
		if _, exists := nodeIndex[nodeID]; !exists {
			return fmt.Errorf("ui.nodes references unknown node %q", nodeID)
		}
	}

	return nil
}

// isValidNodeType 判断节点类型是否合法。
func isValidNodeType(value string) bool {
	return isOneOf(
		value,
		NodeTypeIssueHandling,
		NodeTypeCoding,
		NodeTypeReview,
		NodeTypeIssuePost,
		NodeTypeLoopJudge,
	)
}

// isValidRoleRef 判断角色引用键是否合法。
func isValidRoleRef(value string) bool {
	return isOneOf(
		value,
		RoleRefDefaultAgent,
		RoleRefReviewerAgent,
		RoleRefIssuePostAgent,
		RoleRefLoopJudgeAgent,
	)
}

// isValidPromptTemplate 判断提示词模板标识是否合法。
func isValidPromptTemplate(value string) bool {
	return isOneOf(
		value,
		PromptTemplateIssueHandling,
		PromptTemplateCoding,
		PromptTemplateReview,
		PromptTemplateIssuePost,
		PromptTemplateLoopJudge,
	)
}

// isValidResultContract 判断结果契约标识是否合法。
func isValidResultContract(value string) bool {
	return isOneOf(
		value,
		ResultContractIssueHandling,
		ResultContractCoding,
		ResultContractReview,
		ResultContractIssuePost,
		ResultContractLoopJudge,
	)
}

// isOneOf 判断目标值是否命中受控枚举。
func isOneOf(target string, allowed ...string) bool {
	for _, candidate := range allowed {
		if target == candidate {
			return true
		}
	}

	return false
}
