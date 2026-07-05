// Package main 提供 code-bee 的 CLI 入口。
//
// 核心功能:
// 1. 解析命令行参数并初始化运行时上下文
// 2. 加载 workflow 配置（内置默认或 --workflow 指定的外部文件）
// 3. 调度由 workflow 驱动的 runtime.Engine 执行
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-04
// 更新时间: 2026-07-05
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/JiGuangWorker/code-bee/internal/agent"
	"github.com/JiGuangWorker/code-bee/internal/config"
	"github.com/JiGuangWorker/code-bee/internal/pipeline"
	platformgithub "github.com/JiGuangWorker/code-bee/internal/platform/github"
	"github.com/JiGuangWorker/code-bee/internal/runtime"
	"github.com/JiGuangWorker/code-bee/internal/schema"
	"github.com/JiGuangWorker/code-bee/pkg/version"
)

// dispatchService 是调度器的抽象接口，使 CLI 逻辑可独立于具体的 pipeline.Service 进行测试。
type dispatchService interface {
	Dispatch(context.Context, *config.Config) (*pipeline.Result, error)
}

// serviceFactory 创建调度服务实例，可能因 workflow 加载失败返回错误。
type serviceFactory func() (dispatchService, error)

// main 是 code-bee 的程序入口。
//
// 核心逻辑:
// - 校验命令行参数，避免进入无效执行
// - 创建支持 Ctrl+C 中断的上下文
// - 加载 workflow 配置（内置默认或 --workflow 外部文件）
// - 调用 runtime.Engine 驱动的调度流程
func main() {
	os.Exit(runCLI(os.Args[1:], os.Stdout, os.Stderr, func() (dispatchService, error) {
		return buildService(os.Args[1:])
	}))
}

// buildService 构造真实的 pipeline.Service，包含 workflow 加载。
//
// workflowPath 为空时使用内置默认 workflow。
func buildService(args []string) (*pipeline.Service, error) {
	workflowPath := parseWorkflowFlag(args)

	wf, err := loadWorkflow(workflowPath)
	if err != nil {
		return nil, fmt.Errorf("加载 workflow 失败: %w", err)
	}

	service, err := pipeline.NewService(
		platformgithub.NewClient(),
		agent.New(),
		wf,
	)
	if err != nil {
		return nil, fmt.Errorf("创建调度服务失败: %w", err)
	}

	return service, nil
}

// parseWorkflowFlag 从参数中解析 --workflow flag，不解析其他 flag。
//
// 单独解析是为了避免与 runCLI 的 flag.Parse 冲突（仅提取 workflow 路径）。
func parseWorkflowFlag(args []string) string {
	fs := flag.NewFlagSet("workflow-probe", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	workflowPath := fs.String("workflow", "", "workflow 配置文件路径（留空则使用内置默认）")
	_ = fs.Parse(args)
	return *workflowPath
}

// loadWorkflow 加载 workflow 配置。
//
// 加载策略:
// - path 非空: 从外部 YAML 文件加载（支持用户自定义）
// - path 为空: 加载内置默认 workflow（go:embed）
func loadWorkflow(path string) (*schema.Workflow, error) {
	if path != "" {
		loader, err := schema.NewLoader()
		if err != nil {
			return nil, fmt.Errorf("创建 schema loader: %w", err)
		}
		return loader.LoadFromFile(path)
	}

	return runtime.LoadDefaultWorkflow()
}

// usageError 当参数缺失或解析失败时，向 stderr 输出统一的使用说明。
func usageError(stderr io.Writer) int {
	fmt.Fprintf(stderr, "用法: code-bee --repo <owner/repo> --issue <number> [--workflow <path>]\n\n")
	fmt.Fprintf(stderr, "示例:\n")
	fmt.Fprintf(stderr, "  code-bee --repo owner/repo --issue 42\n")
	fmt.Fprintf(stderr, "  code-bee --repo owner/repo --issue 42 --workflow ./my-workflow.yaml\n")
	return 1
}

// runCLI 是 CLI 入口的可测试版本，接收注入的 io.Writer 和 serviceFactory。
//
// 返回值:
// - 0: 成功完成
// - 1: 参数错误、调度失败或需要人工介入
func runCLI(args []string, stdout, stderr io.Writer, factory serviceFactory) int {
	fs := flag.NewFlagSet("code-bee", flag.ContinueOnError)
	fs.SetOutput(stderr)

	repo := fs.String("repo", "", "仓库地址，如 owner/repo")
	issueNumber := fs.Int("issue", 0, "Issue 编号")
	showVersion := fs.Bool("version", false, "输出版本信息")
	// --workflow flag 仅用于文档展示，实际解析在 buildService 中完成
	_ = fs.String("workflow", "", "workflow 配置文件路径（留空则使用内置默认）")

	if err := fs.Parse(args); err != nil {
		return usageError(stderr)
	}

	if *showVersion {
		fmt.Fprintln(stdout, version.Info())
		return 0
	}

	if *repo == "" || *issueNumber <= 0 {
		return usageError(stderr)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	fmt.Fprintf(stdout, "🐝 code-bee %s\n", version.Version)
	fmt.Fprintf(stdout, "📦 仓库: %s | Issue: #%d\n\n", *repo, *issueNumber)

	service, err := factory()
	if err != nil {
		fmt.Fprintf(stderr, "\n❌ 初始化失败: %v\n", err)
		return 1
	}

	fmt.Fprintln(stdout, "🚀 正在执行 workflow 驱动的调度流程...")

	cfg := config.New(*repo, *issueNumber)
	result, err := service.Dispatch(ctx, cfg)
	if err != nil {
		fmt.Fprintf(stderr, "\n❌ 执行失败: %v\n", err)
		return 1
	}

	return reportDispatchResult(result, stdout, stderr)
}

// reportDispatchResult 将调度结果映射为用户可见消息与退出码。
//
// 返回值:
// - 0: 任务顺利完成
// - 1: 阻塞、需要人工介入或智能体失败
func reportDispatchResult(result *pipeline.Result, stdout, stderr io.Writer) int {
	if result.Success && result.Completed {
		fmt.Fprintln(stdout, "\n✅ reviewer 已明确 PASS，且最终 Issue 回复已提交，任务执行完成")
		return 0
	}

	if result.Success && result.Blocked {
		fmt.Fprintf(stderr, "\n⏸️ 流程已阻塞，需要补充信息或人工介入:\n%s\n", result.Output)
		return 1
	}

	if result.Success {
		if result.ManualRequired {
			fmt.Fprintf(stderr, "\n🧑‍🔧 自动循环已触达兜底阈值，需要人工介入:\n%s\n", result.Output)
			return 1
		}

		fmt.Fprintf(stderr, "\n⏳ 自动循环结束但 reviewer 未 PASS:\n%s\n", result.Output)
		return 1
	}

	fmt.Fprintf(stderr, "\n❌ 智能体返回失败:\n%s\n", result.Output)
	return 1
}
