// Package pipeline 串联 code-bee 的四阶段 harness 与 coder-reviewer loop。
//
// 核心功能:
// 1. 先执行 Issue 处理阶段，由智能体自行查看 Issue 并把结果写入文件
// 2. 再进入“编码 -> 审查”的外层循环，由 code-bee 负责状态机调度
// 3. 所有 Issue 评论都交给专门的 Issue 提交智能体，避免编码阶段直接对外提交
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-04
// 更新时间: 2026-07-04
package pipeline

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/JiGuangWorker/code-bee/internal/agent"
	"github.com/JiGuangWorker/code-bee/internal/config"
	"github.com/JiGuangWorker/code-bee/internal/platform"
)

const (
	maxConsecutiveUnknownReview = 2
	maxIssuePostAttempts        = 2
)

// agentRunner 抽象出智能体运行能力，便于在测试中注入假实现。
type agentRunner interface {
	// Run 调用指定阶段的智能体任务并返回结构化执行结果。
	//
	// 输入参数:
	// - ctx: 控制当前任务生命周期的上下文
	// - kind: 当前阶段类型，用于标识 issue-handling / coding / review / issue-post / loop-judge
	// - task: 传递给智能体的完整提示词
	//
	// 返回值:
	// - *agent.RunResult: 智能体原始输出
	// - error: 当命令执行失败或被中断时返回错误
	Run(ctx context.Context, kind agent.TaskKind, task string) (*agent.RunResult, error)
}

// Service 负责串联平台技能包和四阶段调度流程。
type Service struct {
	platformClient platform.Client
	runner         agentRunner
}

// dispatchContext 统一收纳整个 harness 运行中会重复使用的上下文。
type dispatchContext struct {
	// workerID 是当前实例的稳定标识，用于写入所有阶段的提示词。
	workerID string

	// issueURL 是目标 Issue 的直接入口。
	issueURL string

	// platformName 是当前代码托管平台名称。
	platformName string

	// platformGuide 是当前平台提供给智能体的技能包说明。
	platformGuide string

	// artifacts 是本次运行对应的文件工件集合。
	artifacts *ArtifactSet
}

// codingReviewLoopState 表示 coder-reviewer 外层循环在轮次之间传递的最小状态。
type codingReviewLoopState struct {
	// lastOutput 是上一轮对调度器最有价值的原始输出，优先保留 judge 输出，其次保留 review 输出。
	lastOutput string

	// reviewerFeedback 是下一轮 coder 要消费的反馈摘要。
	reviewerFeedback string

	// consecutiveUnknowns 记录 reviewer 连续给出 UNKNOWN 的次数，用于触发证据优先模式。
	consecutiveUnknowns int
}

// NewService 创建最小执行流水线服务。
func NewService(platformClient platform.Client, runner agentRunner) *Service {
	return &Service{
		platformClient: platformClient,
		runner:         runner,
	}
}

// Dispatch 执行四阶段 harness。
//
// 核心逻辑:
// - 第一步执行 Issue 处理阶段，并通过专门的 Issue 提交智能体完成接单/阻塞回复
// - 第二步进入 coder-reviewer loop，保证“编码”和“审查”是两个独立任务
// - 第三步仅以 reviewer PASS 且最终 completion 回复成功提交作为退出条件
func (s *Service) Dispatch(ctx context.Context, cfg *config.Config) (*Result, error) {
	dispatchCtx, err := s.newDispatchContext(cfg)
	if err != nil {
		return &Result{Success: false}, fmt.Errorf("pipeline.Service.Dispatch: %w", err)
	}

	issueResult, issueOutput, err := s.runIssueHandling(ctx, cfg, dispatchCtx)
	if err != nil {
		return &Result{
			Success: false,
			Output:  issueOutput,
		}, fmt.Errorf("pipeline.Service.Dispatch: %w", err)
	}

	if issueResult.BlockedStatus() {
		return &Result{
			Success: true,
			Blocked: true,
			Output:  issueOutput,
		}, nil
	}

	loopResult, loopErr := s.runCodingReviewLoop(ctx, cfg, dispatchCtx, issueResult)
	if loopErr != nil {
		return loopResult, fmt.Errorf("pipeline.Service.Dispatch: %w", loopErr)
	}

	return loopResult, nil
}

// newDispatchContext 构造本次 harness 运行所需的稳定上下文。
func (s *Service) newDispatchContext(cfg *config.Config) (dispatchContext, error) {
	artifacts, err := NewArtifactSet(cfg.Repo, cfg.IssueNumber)
	if err != nil {
		return dispatchContext{}, err
	}

	return dispatchContext{
		workerID:     resolveWorkerID(),
		issueURL:     s.platformClient.BuildIssueURL(cfg.Repo, cfg.IssueNumber),
		platformName: s.platformClient.Name(),
		platformGuide: s.platformClient.BuildSkillInstruction(
			cfg.Repo,
			cfg.IssueNumber,
		),
		artifacts: artifacts,
	}, nil
}

// runIssueHandling 执行第一阶段的 Issue 处理任务，并确保接单/阻塞回复由 Issue 提交智能体完成。
func (s *Service) runIssueHandling(
	ctx context.Context,
	cfg *config.Config,
	dispatchCtx dispatchContext,
) (*IssueHandlingResult, string, error) {
	var lastOutput string

	if err := resetResultFile(dispatchCtx.artifacts.IssueHandlingResultPath()); err != nil {
		return nil, "", err
	}

	issuePrompt := buildIssueHandlingPrompt(
		cfg,
		dispatchCtx.workerID,
		dispatchCtx.issueURL,
		dispatchCtx.platformName,
		dispatchCtx.platformGuide,
		dispatchCtx.artifacts.IssueHandlingResultPath(),
		"",
	)

	issueRunResult, err := s.runner.Run(ctx, agent.TaskKindIssueHandling, issuePrompt)
	if err != nil {
		return nil, safeOutput(issueRunResult), fmt.Errorf("issue handling failed: %w", err)
	}
	lastOutput = issueRunResult.Output

	issueResult, loadErr := loadIssueHandlingResult(dispatchCtx.artifacts.IssueHandlingResultPath())
	if loadErr != nil {
		return nil, lastOutput, fmt.Errorf("load issue handling result: %w", loadErr)
	}

	postResult, postOutput, postErr := s.runIssuePostWithRetry(
		ctx,
		cfg,
		dispatchCtx,
		"intake",
		dispatchCtx.artifacts.IssueHandlingResultPath(),
	)
	lastOutput = postOutput
	if postErr != nil {
		return nil, lastOutput, postErr
	}

	if postResult.BlockedStatus() {
		issueResult.Status = issueStatusBlocked
	}

	return issueResult, lastOutput, nil
}

// runCodingReviewLoop 执行 coder-reviewer 外层循环，并在 PASS 后调用 Issue 提交智能体提交最终回复。
func (s *Service) runCodingReviewLoop(
	ctx context.Context,
	cfg *config.Config,
	dispatchCtx dispatchContext,
	issueResult *IssueHandlingResult,
) (*Result, error) {
	history := &LoopHistory{
		Repo:        cfg.Repo,
		IssueNumber: cfg.IssueNumber,
		MaxRounds:   cfg.MaxCodingReviewRounds,
	}
	loopState := codingReviewLoopState{}

	for round := 1; round <= cfg.MaxCodingReviewRounds; round++ {
		nextState, loopResult, roundErr := s.runCodingReviewIteration(
			ctx,
			cfg,
			dispatchCtx,
			issueResult,
			history,
			round,
			loopState,
		)
		loopState = nextState
		if roundErr != nil {
			return &Result{Success: false, Output: loopState.lastOutput}, roundErr
		}
		if loopResult != nil {
			return loopResult, nil
		}
	}

	return &Result{Success: true, ManualRequired: true, Output: loopState.lastOutput}, nil
}

// runCodingReviewIteration 执行单轮 coder-reviewer 工作，并把结果折叠回循环状态。
//
// 输入参数:
// - ctx: 控制本轮所有智能体生命周期的上下文
// - cfg: 运行时配置，包含最大轮数与 loop judge 阈值
// - dispatchCtx: 当前运行的稳定上下文
// - issueResult: intake 阶段产出的任务理解与验收标准
// - history: 当前运行中的聚合历史对象
// - round: 当前轮次编号
// - loopState: 上一轮传入的最小循环状态
//
// 返回值:
// - codingReviewLoopState: 更新后的循环状态
// - *Result: 当本轮已经触发最终退出条件时返回最终结果；否则返回 nil
// - error: 当 coding、review、history 落盘或 loop judge 任一环节失败时返回错误
func (s *Service) runCodingReviewIteration(
	ctx context.Context,
	cfg *config.Config,
	dispatchCtx dispatchContext,
	issueResult *IssueHandlingResult,
	history *LoopHistory,
	round int,
	loopState codingReviewLoopState,
) (codingReviewLoopState, *Result, error) {
	codingResult, codingOutput, codingErr := s.runCodingRound(
		ctx,
		cfg,
		dispatchCtx,
		issueResult,
		loopState.reviewerFeedback,
		round,
	)
	if codingErr != nil {
		loopState.lastOutput = codingOutput
		return loopState, nil, codingErr
	}

	if codingResult.BlockedStatus() {
		return loopState, &Result{
			Success: true,
			Blocked: true,
			Output:  codingOutput,
		}, nil
	}

	reviewResult, reviewOutput, reviewErr := s.runReviewRound(
		ctx,
		cfg,
		dispatchCtx,
		issueResult,
		codingResult,
		round,
		loopState.consecutiveUnknowns,
	)
	if reviewErr != nil {
		loopState.lastOutput = reviewOutput
		return loopState, nil, reviewErr
	}

	if historyErr := persistLoopHistoryRound(
		dispatchCtx.artifacts.LoopHistoryPath(),
		history,
		round,
		codingResult,
		reviewResult,
	); historyErr != nil {
		loopState.lastOutput = reviewOutput
		return loopState, nil, historyErr
	}

	if reviewResult.Passed() {
		completedResult, finishErr := s.finishSuccessfulReview(ctx, cfg, dispatchCtx, round)
		return loopState, completedResult, finishErr
	}

	if reviewResult.BlockedStatus() {
		return loopState, &Result{
			Success:        true,
			Blocked:        true,
			ManualRequired: true,
			Output:         reviewOutput,
		}, nil
	}

	return s.finalizeCodingReviewIteration(
		ctx,
		cfg,
		dispatchCtx,
		reviewResult,
		reviewOutput,
		round,
		loopState,
	)
}

// finishSuccessfulReview 在 reviewer 明确 PASS 后，调用 Issue 提交智能体提交最终完成评论。
func (s *Service) finishSuccessfulReview(
	ctx context.Context,
	cfg *config.Config,
	dispatchCtx dispatchContext,
	round int,
) (*Result, error) {
	postResult, postOutput, postErr := s.runIssuePostWithRetry(
		ctx,
		cfg,
		dispatchCtx,
		"completion",
		dispatchCtx.artifacts.ReviewResultPath(round),
	)
	if postErr != nil {
		return &Result{
			Success:        true,
			ManualRequired: true,
			Output:         postOutput,
		}, postErr
	}

	if postResult.BlockedStatus() {
		return &Result{
			Success:        true,
			Blocked:        true,
			ManualRequired: true,
			Output:         postOutput,
		}, nil
	}

	return &Result{Success: true, Completed: true, Output: postOutput}, nil
}

// runCodingRound 执行单轮编码任务，并从结果文件中读取结构化结果。
func (s *Service) runCodingRound(
	ctx context.Context,
	cfg *config.Config,
	dispatchCtx dispatchContext,
	issueResult *IssueHandlingResult,
	reviewerFeedback string,
	round int,
) (*CodingResult, string, error) {
	if err := dispatchCtx.artifacts.EnsureRoundDir(round); err != nil {
		return nil, "", err
	}

	resultFilePath := dispatchCtx.artifacts.CodingResultPath(round)
	if err := resetResultFile(resultFilePath); err != nil {
		return nil, "", err
	}

	codingPrompt := buildCodingPrompt(
		cfg,
		dispatchCtx.workerID,
		dispatchCtx.issueURL,
		dispatchCtx.platformName,
		dispatchCtx.platformGuide,
		issueResult,
		resultFilePath,
		reviewerFeedback,
		round,
		cfg.MaxCodingReviewRounds,
	)

	codingRunResult, err := s.runner.Run(ctx, agent.TaskKindCoding, codingPrompt)
	if err != nil {
		return nil, safeOutput(codingRunResult), fmt.Errorf("coding round %d failed: %w", round, err)
	}

	codingResult, loadErr := loadCodingResult(resultFilePath)
	if loadErr != nil {
		return nil, codingRunResult.Output, fmt.Errorf("load coding round %d result: %w", round, loadErr)
	}

	return codingResult, codingRunResult.Output, nil
}

// runReviewRound 执行单轮审查任务，并从结果文件中读取结构化结果。
func (s *Service) runReviewRound(
	ctx context.Context,
	cfg *config.Config,
	dispatchCtx dispatchContext,
	issueResult *IssueHandlingResult,
	codingResult *CodingResult,
	round int,
	consecutiveUnknowns int,
) (*ReviewResult, string, error) {
	if err := dispatchCtx.artifacts.EnsureRoundDir(round); err != nil {
		return nil, "", err
	}

	resultFilePath := dispatchCtx.artifacts.ReviewResultPath(round)
	if err := resetResultFile(resultFilePath); err != nil {
		return nil, "", err
	}

	reviewPrompt := buildReviewPrompt(
		cfg,
		dispatchCtx.workerID,
		dispatchCtx.issueURL,
		dispatchCtx.platformName,
		dispatchCtx.platformGuide,
		issueResult,
		codingResult,
		resultFilePath,
		round,
		cfg.MaxCodingReviewRounds,
		consecutiveUnknowns,
	)

	reviewRunResult, err := s.runner.Run(ctx, agent.TaskKindReview, reviewPrompt)
	if err != nil {
		return nil, safeOutput(reviewRunResult), fmt.Errorf("review round %d failed: %w", round, err)
	}

	reviewResult, loadErr := loadReviewResult(resultFilePath)
	if loadErr != nil {
		return nil, reviewRunResult.Output, fmt.Errorf("load review round %d result: %w", round, loadErr)
	}

	return reviewResult, reviewRunResult.Output, nil
}

// runLoopJudgeRound 在达到阈值后执行价值评估任务，并读取其结构化决策。
//
// 输入参数:
// - ctx: 控制本轮价值评估智能体生命周期的上下文
// - cfg: 运行时配置，包含 loop judge 角色和最大轮数
// - dispatchCtx: 当前运行的稳定上下文，包含平台信息与工件目录
// - round: 当前刚完成的 coder-reviewer 轮次
//
// 返回值:
// - *LoopJudgeResult: 结构化的价值评估结论
// - string: 价值评估智能体的原始输出，便于排障和上抛
// - error: 当执行失败或结果文件不合法时返回错误
func (s *Service) runLoopJudgeRound(
	ctx context.Context,
	cfg *config.Config,
	dispatchCtx dispatchContext,
	round int,
) (*LoopJudgeResult, string, error) {
	resultFilePath := dispatchCtx.artifacts.LoopJudgeResultPath(round)
	if err := resetResultFile(resultFilePath); err != nil {
		return nil, "", err
	}

	judgePrompt := buildLoopJudgePrompt(
		cfg,
		dispatchCtx.workerID,
		dispatchCtx.issueURL,
		dispatchCtx.platformName,
		dispatchCtx.platformGuide,
		dispatchCtx.artifacts.LoopHistoryPath(),
		resultFilePath,
		round,
		cfg.MaxCodingReviewRounds,
	)

	judgeRunResult, err := s.runner.Run(ctx, agent.TaskKindLoopJudge, judgePrompt)
	if err != nil {
		return nil, safeOutput(judgeRunResult), fmt.Errorf("loop judge round %d failed: %w", round, err)
	}

	judgeResult, loadErr := loadLoopJudgeResult(resultFilePath)
	if loadErr != nil {
		return nil, judgeRunResult.Output, fmt.Errorf("load loop judge round %d result: %w", round, loadErr)
	}

	return judgeResult, judgeRunResult.Output, nil
}

// applyLoopJudgeDecision 在达到阈值后执行价值评估，并将评估结论映射为 loop 状态机动作。
//
// 输入参数:
// - ctx: 控制本轮价值评估生命周期的上下文
// - cfg: 运行时配置，包含 loop judge 角色与阈值
// - dispatchCtx: 当前运行的稳定上下文
// - reviewerFeedback: 当前准备传给下一轮 coder 的 reviewer 反馈
// - round: 当前刚完成的 coder-reviewer 轮次
//
// 返回值:
// - string: 更新后的下一轮 coder 反馈
// - string: 本轮价值评估智能体的原始输出；若未触发评估则返回空字符串
// - *Result: 当价值评估结论要求立即终止循环时，返回最终 Result；否则返回 nil
// - error: 当价值评估执行失败或结果文件非法时返回错误
func (s *Service) applyLoopJudgeDecision(
	ctx context.Context,
	cfg *config.Config,
	dispatchCtx dispatchContext,
	reviewerFeedback string,
	round int,
) (string, string, *Result, error) {
	if !shouldRunLoopJudge(cfg, round) {
		return reviewerFeedback, "", nil, nil
	}

	judgeResult, judgeOutput, judgeErr := s.runLoopJudgeRound(ctx, cfg, dispatchCtx, round)
	if judgeErr != nil {
		return reviewerFeedback, judgeOutput, nil, judgeErr
	}

	switch {
	case judgeResult.Continue():
		return reviewerFeedback, judgeOutput, nil, nil
	case judgeResult.ShrinkTask():
		return mergeLoopJudgeFeedback(reviewerFeedback, judgeResult), judgeOutput, nil, nil
	case judgeResult.StopManual():
		return reviewerFeedback, judgeOutput, &Result{
			Success:        true,
			ManualRequired: true,
			Output:         judgeOutput,
		}, nil
	case judgeResult.StopBlocked():
		return reviewerFeedback, judgeOutput, &Result{
			Success:        true,
			Blocked:        true,
			ManualRequired: true,
			Output:         judgeOutput,
		}, nil
	default:
		return reviewerFeedback, judgeOutput, nil, nil
	}
}

// persistLoopHistoryRound 将单轮 coding/review 结果追加到聚合历史文件。
//
// 输入参数:
// - historyFilePath: 聚合历史文件路径
// - history: 当前运行中的历史对象
// - round: 当前轮次编号
// - codingResult: 本轮编码结果
// - reviewResult: 本轮审查结果
//
// 返回值:
// - error: 当追加历史或写文件失败时返回错误
func persistLoopHistoryRound(
	historyFilePath string,
	history *LoopHistory,
	round int,
	codingResult *CodingResult,
	reviewResult *ReviewResult,
) error {
	if err := history.appendRound(round, codingResult, reviewResult); err != nil {
		return err
	}

	return saveLoopHistory(historyFilePath, history)
}

// finalizeCodingReviewIteration 将 review 结论与 loop judge 结论折叠回循环状态。
//
// 输入参数:
// - ctx: 控制价值评估智能体生命周期的上下文
// - cfg: 运行时配置，包含 loop judge 阈值
// - dispatchCtx: 当前运行的稳定上下文
// - reviewResult: 当前轮 review 结果
// - reviewOutput: 当前轮 review 原始输出
// - round: 当前轮次编号
// - loopState: 待更新的循环状态
//
// 返回值:
// - codingReviewLoopState: 更新后的循环状态
// - *Result: 当价值评估要求立即退出时返回最终结果；否则返回 nil
// - error: 当价值评估执行失败时返回错误
func (s *Service) finalizeCodingReviewIteration(
	ctx context.Context,
	cfg *config.Config,
	dispatchCtx dispatchContext,
	reviewResult *ReviewResult,
	reviewOutput string,
	round int,
	loopState codingReviewLoopState,
) (codingReviewLoopState, *Result, error) {
	loopState.reviewerFeedback, loopState.consecutiveUnknowns = nextReviewerFeedback(
		reviewResult,
		loopState.consecutiveUnknowns,
	)

	judgeFeedback, judgeOutput, loopResult, judgeErr := s.applyLoopJudgeDecision(
		ctx,
		cfg,
		dispatchCtx,
		loopState.reviewerFeedback,
		round,
	)
	loopState.reviewerFeedback = judgeFeedback
	loopState.lastOutput = reviewOutput
	if judgeOutput != "" {
		loopState.lastOutput = judgeOutput
	}

	return loopState, loopResult, judgeErr
}

// runIssuePostWithRetry 执行 Issue 提交阶段，并在被 REJECTED 时允许专门的提交智能体自我修正一次。
func (s *Service) runIssuePostWithRetry(
	ctx context.Context,
	cfg *config.Config,
	dispatchCtx dispatchContext,
	purpose string,
	sourceFilePath string,
) (*IssuePostResult, string, error) {
	var (
		lastOutput       string
		previousFeedback string
		resultFilePath   = dispatchCtx.artifacts.IssuePostResultPath(purpose)
	)

	for attempt := 1; attempt <= maxIssuePostAttempts; attempt++ {
		if err := resetResultFile(resultFilePath); err != nil {
			return nil, lastOutput, err
		}

		postPrompt := buildIssuePostPrompt(
			cfg,
			dispatchCtx.workerID,
			dispatchCtx.issueURL,
			dispatchCtx.platformName,
			dispatchCtx.platformGuide,
			purpose,
			sourceFilePath,
			resultFilePath,
			previousFeedback,
		)

		postRunResult, err := s.runner.Run(ctx, agent.TaskKindIssuePost, postPrompt)
		if err != nil {
			return nil, safeOutput(postRunResult), fmt.Errorf("issue post %s attempt %d failed: %w", purpose, attempt, err)
		}
		lastOutput = postRunResult.Output

		postResult, loadErr := loadIssuePostResult(resultFilePath)
		if loadErr != nil {
			return nil, lastOutput, fmt.Errorf("load issue post %s attempt %d result: %w", purpose, attempt, loadErr)
		}

		if postResult.Posted() || postResult.BlockedStatus() {
			return postResult, lastOutput, nil
		}

		previousFeedback = postResult.Feedback
	}

	return nil, lastOutput, fmt.Errorf("issue post %s rejected after %d attempts", purpose, maxIssuePostAttempts)
}

// nextReviewerFeedback 基于本轮 reviewer 结果生成下一轮 coder 要消费的反馈，并维护 UNKNOWN 计数。
func nextReviewerFeedback(reviewResult *ReviewResult, consecutiveUnknowns int) (string, int) {
	if reviewResult.Unknown() {
		consecutiveUnknowns++
	} else {
		consecutiveUnknowns = 0
	}

	feedback := buildReviewerFeedback(reviewResult)
	if consecutiveUnknowns >= maxConsecutiveUnknownReview {
		feedback += "\n\n强制要求：下一轮优先补充证据，不要继续盲改代码。"
	}

	return feedback, consecutiveUnknowns
}

// mergeLoopJudgeFeedback 将价值评估员的收缩任务建议拼接到 coder 下一轮输入中。
//
// 输入参数:
// - reviewerFeedback: 原始 reviewer 反馈摘要
// - judgeResult: 价值评估员的结构化判断结果
//
// 返回值:
// - string: 可直接传给下一轮 coder 的合并后反馈文本
func mergeLoopJudgeFeedback(reviewerFeedback string, judgeResult *LoopJudgeResult) string {
	if judgeResult == nil {
		return reviewerFeedback
	}

	judgeFeedback := fmt.Sprintf(
		"价值评估结论:\n%s\n\n依据:\n%s\n\n下一步要求:\n%s",
		judgeResult.Reason,
		judgeResult.Evidence,
		judgeResult.NextAction,
	)

	if strings.TrimSpace(reviewerFeedback) == "" {
		return judgeFeedback
	}

	return reviewerFeedback + "\n\n" + judgeFeedback
}

// buildReviewerFeedback 将 reviewer 结构化结果转成下一轮 coder 的输入摘要。
func buildReviewerFeedback(reviewResult *ReviewResult) string {
	return fmt.Sprintf(
		"审查结论:\n%s\n\n验收项判断:\n%s\n\n缺失项:\n%s\n\n下一步要求:\n%s",
		reviewResult.Summary,
		reviewResult.CheckResult,
		reviewResult.Missing,
		reviewResult.NextAction,
	)
}

// shouldRunLoopJudge 判断当前轮次是否应触发价值评估。
//
// 输入参数:
// - cfg: 运行时配置，包含 loop judge 介入阈值
// - round: 当前刚完成的 coder-reviewer 轮次
//
// 返回值:
// - bool: 当轮次达到或超过介入阈值时返回 true
func shouldRunLoopJudge(cfg *config.Config, round int) bool {
	if cfg == nil {
		return false
	}

	if cfg.LoopJudgeStartRound <= 0 {
		return true
	}

	return round >= cfg.LoopJudgeStartRound
}

// resetResultFile 在每轮调用前删除旧的结果文件，防止读取到上一次残留结果。
func resetResultFile(filePath string) error {
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reset result file %s: %w", filePath, err)
	}

	return nil
}

// resolveWorkerID 解析当前执行实例的稳定 Worker 标识。
func resolveWorkerID() string {
	if workerID := strings.TrimSpace(os.Getenv("WORKER_ID")); workerID != "" {
		return workerID
	}

	hostname, err := os.Hostname()
	if err == nil && strings.TrimSpace(hostname) != "" {
		return hostname
	}

	return "unknown-worker"
}

// safeOutput 在上层需要透传 Runner 输出时提供空值兜底。
func safeOutput(result *agent.RunResult) string {
	if result == nil {
		return ""
	}

	return result.Output
}
