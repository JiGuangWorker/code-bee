// Package pipeline 定义 code-bee 四阶段 loop 使用的结果契约与文件读取逻辑。
//
// 核心功能:
// 1. 统一定义 intake / coding / review / issue-post 四类结果文件的结构
// 2. 提供最小的 JSON 文件加载与状态校验能力，避免 orchestrator 直接解析自由文本
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-04
// 更新时间: 2026-07-04
package pipeline

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const (
	issueStatusReady   = "READY"
	issueStatusBlocked = "BLOCKED"

	coderStatusDone       = "DONE"
	coderStatusInProgress = "IN_PROGRESS"
	coderStatusBlocked    = "BLOCKED"

	reviewStatusPass    = "PASS"
	reviewStatusFail    = "FAIL"
	reviewStatusUnknown = "UNKNOWN"
	reviewStatusBlocked = "BLOCKED"

	postStatusPosted   = "POSTED"
	postStatusRejected = "REJECTED"
	postStatusBlocked  = "BLOCKED"

	loopJudgeDecisionContinue    = "CONTINUE"
	loopJudgeDecisionShrinkTask  = "SHRINK_TASK"
	loopJudgeDecisionStopManual  = "STOP_MANUAL"
	loopJudgeDecisionStopBlocked = "STOP_BLOCKED"
)

// Result 表示 code-bee 外层 loop 的最终执行结果。
type Result struct {
	// Success 表示整个 loop 是否顺利完成，没有出现系统级错误。
	Success bool

	// Completed 表示当前 Issue 已被 reviewer 判定通过，且最终回复已成功提交。
	Completed bool

	// Blocked 表示流程因外部条件不足或智能体明确阻塞而暂停。
	Blocked bool

	// ManualRequired 表示自动循环已经触达兜底阈值，需要人工接管。
	ManualRequired bool

	// Output 保留最后一个阶段的原始输出，便于排障和问题归因。
	Output string
}

// IssueHandlingResult 表示 Issue 处理阶段写入的结构化结果文件。
type IssueHandlingResult struct {
	// Status 表示当前 Issue 是否可进入开发。
	Status string `json:"status"`

	// Agent 是本次被选择的编码角色名称，不包含 @ 前缀。
	Agent string `json:"agent"`

	// Summary 是对当前 Issue 的任务理解摘要。
	Summary string `json:"summary"`

	// Acceptance 是后续编码与审查必须遵循的验收标准或完成依据。
	Acceptance string `json:"acceptance"`

	// NextAction 是进入下一阶段前的直接动作说明。
	NextAction string `json:"next_action"`

	// CommentBody 是准备交给 Issue 提交智能体发布的接单/阻塞评论正文。
	CommentBody string `json:"comment_body"`
}

// Ready 返回当前 Issue 结果是否允许进入编码阶段。
func (r *IssueHandlingResult) Ready() bool {
	return r != nil && r.Status == issueStatusReady
}

// BlockedStatus 返回当前 Issue 结果是否处于阻塞态。
func (r *IssueHandlingResult) BlockedStatus() bool {
	return r != nil && r.Status == issueStatusBlocked
}

// CodingResult 表示编码阶段每轮写入的结构化结果文件。
type CodingResult struct {
	// Status 表示本轮编码是否完成、继续中或阻塞。
	Status string `json:"status"`

	// Summary 是本轮编码工作总结。
	Summary string `json:"summary"`

	// Evidence 是可供 reviewer 验证的证据清单。
	Evidence string `json:"evidence"`

	// AcceptanceCheck 是编码智能体对验收项逐条自检的结果。
	AcceptanceCheck string `json:"acceptance_check"`

	// NextAction 是下一轮建议动作。
	NextAction string `json:"next_action"`
}

// Done 返回 coder 是否认为已准备好接受审查。
func (r *CodingResult) Done() bool {
	return r != nil && r.Status == coderStatusDone
}

// BlockedStatus 返回 coder 是否已经阻塞。
func (r *CodingResult) BlockedStatus() bool {
	return r != nil && r.Status == coderStatusBlocked
}

// ReviewResult 表示审查阶段每轮写入的结构化结果文件。
type ReviewResult struct {
	// Status 表示审查结论。
	Status string `json:"status"`

	// Summary 是本轮审查总结。
	Summary string `json:"summary"`

	// CheckResult 是对验收标准逐条判断的结果。
	CheckResult string `json:"check_result"`

	// Missing 描述仍缺失的实现或证据。
	Missing string `json:"missing"`

	// NextAction 是要求 coder 下一轮执行的动作。
	NextAction string `json:"next_action"`

	// CommentBody 是当审查通过时准备发布到 Issue 的完成评论正文。
	CommentBody string `json:"comment_body"`
}

// Passed 返回 reviewer 是否明确判定通过。
func (r *ReviewResult) Passed() bool {
	return r != nil && r.Status == reviewStatusPass
}

// Unknown 返回 reviewer 是否暂时无法判断。
func (r *ReviewResult) Unknown() bool {
	return r != nil && r.Status == reviewStatusUnknown
}

// BlockedStatus 返回 reviewer 是否已经阻塞。
func (r *ReviewResult) BlockedStatus() bool {
	return r != nil && r.Status == reviewStatusBlocked
}

// IssuePostResult 表示 Issue 提交智能体写入的结构化结果文件。
type IssuePostResult struct {
	// Status 表示提交结果：已提交、拒绝或阻塞。
	Status string `json:"status"`

	// Summary 是本次 Issue 提交动作的总结。
	Summary string `json:"summary"`

	// Feedback 是在拒绝提交时给上游阶段的修正反馈。
	Feedback string `json:"feedback"`

	// NextAction 是本次提交后的下一步建议动作。
	NextAction string `json:"next_action"`
}

// LoopJudgeResult 表示价值评估员基于多轮历史做出的结构化判断。
type LoopJudgeResult struct {
	// Decision 表示价值评估员对自动循环的动作建议。
	Decision string `json:"decision"`

	// Reason 是本次判断的简要理由，要求能让人工快速理解为何继续或停止。
	Reason string `json:"reason"`

	// Evidence 是本次判断引用的轮次证据，要求明确说明依据来自哪些历史记录。
	Evidence string `json:"evidence"`

	// Confidence 是本次判断的置信度，便于后续扩展更保守的兜底策略。
	Confidence string `json:"confidence"`

	// NextAction 是给 orchestrator 或下游智能体的明确下一步动作说明。
	NextAction string `json:"next_action"`
}

// Continue 返回价值评估员是否建议继续自动循环。
func (r *LoopJudgeResult) Continue() bool {
	return r != nil && r.Decision == loopJudgeDecisionContinue
}

// ShrinkTask 返回价值评估员是否建议收缩任务后再继续执行。
func (r *LoopJudgeResult) ShrinkTask() bool {
	return r != nil && r.Decision == loopJudgeDecisionShrinkTask
}

// StopManual 返回价值评估员是否建议停止自动循环并转人工。
func (r *LoopJudgeResult) StopManual() bool {
	return r != nil && r.Decision == loopJudgeDecisionStopManual
}

// StopBlocked 返回价值评估员是否建议因外部阻塞而停止自动循环。
func (r *LoopJudgeResult) StopBlocked() bool {
	return r != nil && r.Decision == loopJudgeDecisionStopBlocked
}

// Posted 返回 Issue 提交是否已成功完成。
func (r *IssuePostResult) Posted() bool {
	return r != nil && r.Status == postStatusPosted
}

// Rejected 返回 Issue 提交是否被拒绝。
func (r *IssuePostResult) Rejected() bool {
	return r != nil && r.Status == postStatusRejected
}

// BlockedStatus 返回 Issue 提交阶段是否阻塞。
func (r *IssuePostResult) BlockedStatus() bool {
	return r != nil && r.Status == postStatusBlocked
}

// loadIssueHandlingResult 从文件中读取并校验 Issue 处理结果。
func loadIssueHandlingResult(filePath string) (*IssueHandlingResult, error) {
	var result IssueHandlingResult
	if err := loadJSONFile(filePath, &result); err != nil {
		return nil, err
	}

	if err := validateIssueHandlingResult(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// loadCodingResult 从文件中读取并校验编码阶段结果。
func loadCodingResult(filePath string) (*CodingResult, error) {
	var result CodingResult
	if err := loadJSONFile(filePath, &result); err != nil {
		return nil, err
	}

	if err := validateCodingResult(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// loadReviewResult 从文件中读取并校验审查阶段结果。
func loadReviewResult(filePath string) (*ReviewResult, error) {
	var result ReviewResult
	if err := loadJSONFile(filePath, &result); err != nil {
		return nil, err
	}

	if err := validateReviewResult(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// loadIssuePostResult 从文件中读取并校验 Issue 提交结果。
func loadIssuePostResult(filePath string) (*IssuePostResult, error) {
	var result IssuePostResult
	if err := loadJSONFile(filePath, &result); err != nil {
		return nil, err
	}

	if err := validateIssuePostResult(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// loadLoopJudgeResult 从文件中读取并校验价值评估结果。
func loadLoopJudgeResult(filePath string) (*LoopJudgeResult, error) {
	var result LoopJudgeResult
	if err := loadJSONFile(filePath, &result); err != nil {
		return nil, err
	}

	if err := validateLoopJudgeResult(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// loadJSONFile 读取指定 JSON 文件并反序列化到目标结构。
//
// 输入参数:
// - filePath: 结果文件路径，要求该文件已经由对应智能体写出
// - target: 反序列化目标结构体指针
//
// 返回值:
// - error: 当文件不存在、为空或 JSON 非法时返回错误
func loadJSONFile(filePath string, target any) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read result file %s: %w", filePath, err)
	}

	trimmedContent := strings.TrimSpace(string(content))
	if trimmedContent == "" {
		return fmt.Errorf("result file %s is empty", filePath)
	}

	if err := json.Unmarshal([]byte(trimmedContent), target); err != nil {
		return fmt.Errorf("decode result file %s: %w", filePath, err)
	}

	return nil
}

// validateIssueHandlingResult 校验 intake 结果文件的最小完整性。
func validateIssueHandlingResult(result *IssueHandlingResult) error {
	switch result.Status {
	case issueStatusReady, issueStatusBlocked:
	default:
		return fmt.Errorf("invalid issue handling status %q", result.Status)
	}

	if strings.TrimSpace(result.Agent) == "" {
		return fmt.Errorf("empty issue handling agent")
	}

	if strings.TrimSpace(result.Summary) == "" {
		return fmt.Errorf("empty issue handling summary")
	}

	if strings.TrimSpace(result.Acceptance) == "" {
		return fmt.Errorf("empty issue handling acceptance")
	}

	if strings.TrimSpace(result.NextAction) == "" {
		return fmt.Errorf("empty issue handling next_action")
	}

	if strings.TrimSpace(result.CommentBody) == "" {
		return fmt.Errorf("empty issue handling comment_body")
	}

	return nil
}

// validateCodingResult 校验 coding 结果文件的最小完整性。
func validateCodingResult(result *CodingResult) error {
	switch result.Status {
	case coderStatusDone, coderStatusInProgress, coderStatusBlocked:
	default:
		return fmt.Errorf("invalid coding status %q", result.Status)
	}

	if strings.TrimSpace(result.Summary) == "" {
		return fmt.Errorf("empty coding summary")
	}

	if strings.TrimSpace(result.Evidence) == "" {
		return fmt.Errorf("empty coding evidence")
	}

	if strings.TrimSpace(result.AcceptanceCheck) == "" {
		return fmt.Errorf("empty coding acceptance_check")
	}

	if strings.TrimSpace(result.NextAction) == "" {
		return fmt.Errorf("empty coding next_action")
	}

	return nil
}

// validateReviewResult 校验 review 结果文件的最小完整性。
func validateReviewResult(result *ReviewResult) error {
	switch result.Status {
	case reviewStatusPass, reviewStatusFail, reviewStatusUnknown, reviewStatusBlocked:
	default:
		return fmt.Errorf("invalid review status %q", result.Status)
	}

	if strings.TrimSpace(result.Summary) == "" {
		return fmt.Errorf("empty review summary")
	}

	if strings.TrimSpace(result.CheckResult) == "" {
		return fmt.Errorf("empty review check_result")
	}

	if strings.TrimSpace(result.Missing) == "" {
		return fmt.Errorf("empty review missing")
	}

	if strings.TrimSpace(result.NextAction) == "" {
		return fmt.Errorf("empty review next_action")
	}

	if result.Status == reviewStatusPass && strings.TrimSpace(result.CommentBody) == "" {
		return fmt.Errorf("empty review comment_body when status is PASS")
	}

	return nil
}

// validateIssuePostResult 校验 Issue 提交结果文件的最小完整性。
func validateIssuePostResult(result *IssuePostResult) error {
	switch result.Status {
	case postStatusPosted, postStatusRejected, postStatusBlocked:
	default:
		return fmt.Errorf("invalid issue post status %q", result.Status)
	}

	if strings.TrimSpace(result.Summary) == "" {
		return fmt.Errorf("empty issue post summary")
	}

	if strings.TrimSpace(result.Feedback) == "" {
		return fmt.Errorf("empty issue post feedback")
	}

	if strings.TrimSpace(result.NextAction) == "" {
		return fmt.Errorf("empty issue post next_action")
	}

	return nil
}

// validateLoopJudgeResult 校验价值评估结果的最小完整性。
func validateLoopJudgeResult(result *LoopJudgeResult) error {
	switch result.Decision {
	case loopJudgeDecisionContinue, loopJudgeDecisionShrinkTask, loopJudgeDecisionStopManual, loopJudgeDecisionStopBlocked:
	default:
		return fmt.Errorf("invalid loop judge decision %q", result.Decision)
	}

	if strings.TrimSpace(result.Reason) == "" {
		return fmt.Errorf("empty loop judge reason")
	}

	if strings.TrimSpace(result.Evidence) == "" {
		return fmt.Errorf("empty loop judge evidence")
	}

	switch strings.TrimSpace(result.Confidence) {
	case "HIGH", "MEDIUM", "LOW":
	default:
		return fmt.Errorf("invalid loop judge confidence %q", result.Confidence)
	}

	if strings.TrimSpace(result.NextAction) == "" {
		return fmt.Errorf("empty loop judge next_action")
	}

	return nil
}
