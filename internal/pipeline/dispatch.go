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
	maxCodingReviewRounds       = 3
	maxConsecutiveUnknownReview = 2
	maxIssuePostAttempts        = 2
)

// Runner 定义智能体执行器的抽象，用于解耦 Service 与具体的 agent.Runner 实现。
type Runner interface {
	Run(ctx context.Context, kind agent.TaskKind, task string) (*agent.RunResult, error)
}

// Service 负责串联平台技能包和四阶段调度流程。
type Service struct {
	platformClient platform.Client
	runner         Runner
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

// NewService 创建最小执行流水线服务。
func NewService(platformClient platform.Client, runner Runner) *Service {
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
	var (
		lastOutput          string
		reviewerFeedback    string
		consecutiveUnknowns int
	)

	maxRounds := maxCodingReviewRounds
	if cfg.MaxCodingReviewRounds > 0 {
		maxRounds = cfg.MaxCodingReviewRounds
	}

	history := &LoopHistory{
		Repo:        cfg.Repo,
		IssueNumber: cfg.IssueNumber,
		MaxRounds:   maxRounds,
	}

	for round := 1; round <= maxRounds; round++ {
		codingResult, codingOutput, codingErr := s.runCodingRound(
			ctx,
			cfg,
			dispatchCtx,
			issueResult,
			reviewerFeedback,
			round,
		)
		if codingErr != nil {
			return &Result{Success: false, Output: codingOutput}, codingErr
		}

		if codingResult.BlockedStatus() {
			return &Result{Success: true, Blocked: true, Output: codingOutput}, nil
		}

		reviewResult, reviewOutput, reviewErr := s.runReviewRound(
			ctx,
			cfg,
			dispatchCtx,
			issueResult,
			codingResult,
			round,
			consecutiveUnknowns,
		)
		if reviewErr != nil {
			return &Result{Success: false, Output: reviewOutput}, reviewErr
		}
		lastOutput = reviewOutput

		if err := history.appendRound(round, codingResult, reviewResult); err != nil {
			return &Result{Success: false, Output: lastOutput}, fmt.Errorf("append history round %d: %w", round, err)
		}
		if err := saveLoopHistory(dispatchCtx.artifacts.LoopHistoryPath(), history); err != nil {
			return &Result{Success: false, Output: lastOutput}, fmt.Errorf("save loop history round %d: %w", round, err)
		}

		if reviewResult.Passed() {
			return s.finishSuccessfulReview(ctx, cfg, dispatchCtx)
		}

		if reviewResult.BlockedStatus() {
			return &Result{
				Success:        true,
				Blocked:        true,
				ManualRequired: true,
				Output:         reviewOutput,
			}, nil
		}

		reviewerFeedback, consecutiveUnknowns = nextReviewerFeedback(reviewResult, consecutiveUnknowns)

		if shouldRunLoopJudge(cfg, round) {
			judgeResult, judgeOutput, judgeErr := s.runLoopJudgeRound(
				ctx, cfg, dispatchCtx, round, maxRounds,
			)
			if judgeErr != nil {
				return &Result{Success: false, Output: judgeOutput}, judgeErr
			}
			lastOutput = judgeOutput

			switch judgeResult.Decision {
			case loopJudgeDecisionStopManual:
				return &Result{Success: true, ManualRequired: true, Output: judgeOutput}, nil
			case loopJudgeDecisionStopBlocked:
				return &Result{Success: true, Blocked: true, ManualRequired: true, Output: judgeOutput}, nil
			case loopJudgeDecisionShrinkTask:
				reviewerFeedback = buildLoopJudgeFeedback(judgeResult) + "\n\n" + reviewerFeedback
			case loopJudgeDecisionContinue:
			}
		}
	}

	return &Result{Success: true, ManualRequired: true, Output: lastOutput}, nil
}

// finishSuccessfulReview 在 reviewer 明确 PASS 后，调用 Issue 提交智能体提交最终完成评论。
func (s *Service) finishSuccessfulReview(
	ctx context.Context,
	cfg *config.Config,
	dispatchCtx dispatchContext,
) (*Result, error) {
	postResult, postOutput, postErr := s.runIssuePostWithRetry(
		ctx,
		cfg,
		dispatchCtx,
		"completion",
		dispatchCtx.artifacts.ReviewResultPath(),
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
	if err := resetResultFile(dispatchCtx.artifacts.CodingResultPath()); err != nil {
		return nil, "", err
	}

	codingPrompt := buildCodingPrompt(
		cfg,
		dispatchCtx.workerID,
		dispatchCtx.issueURL,
		dispatchCtx.platformName,
		dispatchCtx.platformGuide,
		issueResult,
		dispatchCtx.artifacts.CodingResultPath(),
		reviewerFeedback,
		round,
		maxCodingReviewRounds,
	)

	codingRunResult, err := s.runner.Run(ctx, agent.TaskKindCoding, codingPrompt)
	if err != nil {
		return nil, safeOutput(codingRunResult), fmt.Errorf("coding round %d failed: %w", round, err)
	}

	codingResult, loadErr := loadCodingResult(dispatchCtx.artifacts.CodingResultPath())
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
	if err := resetResultFile(dispatchCtx.artifacts.ReviewResultPath()); err != nil {
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
		dispatchCtx.artifacts.ReviewResultPath(),
		round,
		maxCodingReviewRounds,
		consecutiveUnknowns,
	)

	reviewRunResult, err := s.runner.Run(ctx, agent.TaskKindReview, reviewPrompt)
	if err != nil {
		return nil, safeOutput(reviewRunResult), fmt.Errorf("review round %d failed: %w", round, err)
	}

	reviewResult, loadErr := loadReviewResult(dispatchCtx.artifacts.ReviewResultPath())
	if loadErr != nil {
		return nil, reviewRunResult.Output, fmt.Errorf("load review round %d result: %w", round, loadErr)
	}

	return reviewResult, reviewRunResult.Output, nil
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

// runLoopJudgeRound 执行单轮价值评估任务。
func (s *Service) runLoopJudgeRound(
	ctx context.Context,
	cfg *config.Config,
	dispatchCtx dispatchContext,
	round int,
	maxRounds int,
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
		round,
		maxRounds,
		resultFilePath,
		dispatchCtx.artifacts.LoopHistoryPath(),
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

// buildLoopJudgeFeedback 将价值评估结果转成下一轮 coder 的输入摘要。
func buildLoopJudgeFeedback(judgeResult *LoopJudgeResult) string {
	return fmt.Sprintf(
		"价值评估结论:\n决策: %s\n原因: %s\n证据: %s\n下一步: %s",
		judgeResult.Decision,
		judgeResult.Reason,
		judgeResult.Evidence,
		judgeResult.NextAction,
	)
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

// shouldRunLoopJudge 判断当前轮次是否应触发价值评估员。
//
// 规则:
// - 如果 LoopJudgeStartRound <= 0，始终触发
// - 否则，当 round >= LoopJudgeStartRound 时触发
func shouldRunLoopJudge(cfg *config.Config, round int) bool {
	if cfg.LoopJudgeStartRound <= 0 {
		return true
	}
	return round >= cfg.LoopJudgeStartRound
}
