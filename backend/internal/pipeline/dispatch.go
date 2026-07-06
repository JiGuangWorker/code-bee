// Package pipeline 串联 code-bee 的四阶段 harness 与 coder-reviewer loop。
//
// 本包在阶段 8 重构后，调度流程完全由 runtime.Engine 驱动：
//   - Service 持有 *runtime.Engine 和 *schema.Workflow
//   - Dispatch 构造 ArtifactResolverAdapter + PlatformContext，委托 engine.Run
//   - 硬编码的 runIssueHandling / runCodingReviewLoop 等函数已删除
//   - prompt 模板外部化至 internal/runtime/prompts/*.md
//
// 保留的职责:
// 1. 文件契约（ArtifactSet / 历史 / 结果校验）—— contracts.go / artifacts.go / history.go
// 2. runtime.ArtifactResolver 适配层 —— runtime_adapter.go
// 3. EngineResult → pipeline.Result 类型转换
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-04
// 更新时间: 2026-07-05
package pipeline

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/JiGuangWorker/code-bee/internal/agent"
	"github.com/JiGuangWorker/code-bee/internal/config"
	"github.com/JiGuangWorker/code-bee/internal/platform"
	"github.com/JiGuangWorker/code-bee/internal/runtime"
	"github.com/JiGuangWorker/code-bee/internal/schema"
)

// Runner 定义智能体执行器的抽象，用于解耦 Service 与具体的 agent.Runner 实现。
type Runner interface {
	Run(ctx context.Context, kind agent.TaskKind, task string) (*agent.RunResult, error)
}

// Service 负责串联平台技能包和 runtime.Engine 驱动的调度流程。
//
// 在阶段 8 重构后，Service 仅保留薄封装：
//   - 持有 *runtime.Engine（构造时一次性创建）
//   - Dispatch 时构造文件契约和平台上下文，委托 engine.Run
type Service struct {
	platformClient platform.Client
	runner         Runner
	engine         *runtime.Engine
	workflow       *schema.Workflow
}

// NewService 创建基于 runtime.Engine 的调度服务。
//
// 参数:
// - platformClient: 平台客户端（GitHub 等），用于构造 Issue URL 和技能包说明
// - runner: 智能体执行器，由 runtime.AgentTool 包装调用
// - wf: 已校验的 workflow 配置，驱动整个调度流程
// - opts: 可选的 EngineOption（如 runtime.WithPromptsDir），透传给 runtime.NewEngine
//
// 返回值:
// - *Service: 可用于 Dispatch 的服务实例
// - error: 当 runtime.Engine 构造失败时返回
func NewService(platformClient platform.Client, runner Runner, wf *schema.Workflow, opts ...runtime.EngineOption) (*Service, error) {
	engine, err := runtime.NewEngine(wf, runner, opts...)
	if err != nil {
		return nil, fmt.Errorf("pipeline.NewService: %w", err)
	}

	return &Service{
		platformClient: platformClient,
		runner:         runner,
		engine:         engine,
		workflow:       wf,
	}, nil
}

// Dispatch 执行 workflow 配置驱动的调度流程。
//
// 核心逻辑:
// 1. 创建本次运行的 ArtifactSet（文件契约目录）
// 2. 构造 ArtifactResolverAdapter（适配 runtime.ArtifactResolver 接口）
// 3. 构造 PlatformContext（workerID / issueURL / 平台技能包）
// 4. 委托 engine.Run 执行 workflow.Pipeline
// 5. 把 EngineResult 转成 pipeline.Result 返回
func (s *Service) Dispatch(ctx context.Context, cfg *config.Config) (*Result, error) {
	artifacts, err := NewArtifactSet(cfg.Repo, cfg.IssueNumber)
	if err != nil {
		return &Result{Success: false}, fmt.Errorf("pipeline.Service.Dispatch: %w", err)
	}

	resolver := NewArtifactResolverAdapter(artifacts)

	platformCtx := runtime.PlatformContext{
		WorkerID:      resolveWorkerID(),
		IssueURL:      s.platformClient.BuildIssueURL(cfg.Repo, cfg.IssueNumber),
		PlatformName:  s.platformClient.Name(),
		PlatformGuide: s.platformClient.BuildSkillInstruction(cfg.Repo, cfg.IssueNumber),
	}

	deps := runtime.RunDependencies{
		Artifacts: resolver,
		Platform:  platformCtx,
		Config:    cfg,
	}

	engineResult, err := s.engine.Run(ctx, deps)
	result := toPipelineResult(engineResult)
	if err != nil {
		return result, fmt.Errorf("pipeline.Service.Dispatch: %w", err)
	}

	return result, nil
}

// toPipelineResult 把 runtime.EngineResult 转成 pipeline.Result。
//
// 两个结构体字段对齐，转换是纯数据拷贝。
func toPipelineResult(r *runtime.EngineResult) *Result {
	if r == nil {
		return &Result{Success: false}
	}
	return &Result{
		Success:        r.Success,
		Completed:      r.Completed,
		Blocked:        r.Blocked,
		ManualRequired: r.ManualRequired,
		Output:         r.Output,
	}
}

// resetResultFile 在每轮调用前删除旧的结果文件，防止读取到上一次残留结果。
//
// 由 ArtifactResolverAdapter.ResetResultFile 复用。
func resetResultFile(filePath string) error {
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reset result file %s: %w", filePath, err)
	}

	return nil
}

// resolveWorkerID 解析当前执行实例的稳定 Worker 标识。
//
// 优先级: WORKER_ID 环境变量 > hostname > "unknown-worker"。
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
