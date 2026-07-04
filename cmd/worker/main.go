// Package main 提供 code-bee 的 CLI 入口。
//
// 核心功能:
// 1. 解析命令行参数并初始化运行时上下文
// 2. 调度 Issue 处理、编码、审查、Issue 提交四个阶段的独立任务
// 3. 以 reviewer PASS 且最终回复已成功提交作为 loop 唯一退出条件
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-04
// 更新时间: 2026-07-04
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
	"github.com/JiGuangWorker/code-bee/pkg/version"
)

// dispatchService 抽象出 CLI 依赖的最小调度能力，便于在测试中注入假实现。
type dispatchService interface {
	// Dispatch 执行完整的四阶段 harness，并返回调度结果。
	//
	// 输入参数:
	// - ctx: 控制整个 CLI 调度生命周期的上下文
	// - cfg: 运行时配置，包含 repo、issue 以及各角色默认配置
	//
	// 返回值:
	// - *pipeline.Result: 调度器最终状态
	// - error: 当调度过程中发生系统级错误时返回错误
	Dispatch(ctx context.Context, cfg *config.Config) (*pipeline.Result, error)
}

// serviceFactory 定义创建调度服务的工厂函数。
type serviceFactory func() dispatchService

// main 是 code-bee 的程序入口。
//
// 核心逻辑:
// - 校验命令行参数，避免进入无效执行
// - 创建支持 Ctrl+C 中断的上下文
// - 调用四阶段 harness，将读 Issue、编码、审查、Issue 提交拆成独立阶段
//
// 调用注意事项:
// - 需要本机已安装 reasonix，以便生成回执和执行编码任务
func main() {
	os.Exit(runCLI(os.Args[1:], os.Stdout, os.Stderr, newDefaultService))
}

// runCLI 执行 CLI 参数解析、调度调用和退出码映射。
//
// 输入参数:
// - args: 传入 CLI 的原始参数列表，不包含二进制名
// - stdout: 标准输出写入目标
// - stderr: 错误输出写入目标
// - newService: 调度服务工厂，便于在测试中替换为假实现
//
// 返回值:
// - int: 进程应返回的退出码，0 表示成功，1 表示失败或需要人工介入
func runCLI(args []string, stdout io.Writer, stderr io.Writer, newService serviceFactory) int {
	flagSet := flag.NewFlagSet("code-bee", flag.ContinueOnError)
	flagSet.SetOutput(stderr)

	repo := flagSet.String("repo", "", "仓库地址，如 owner/repo")
	issueNumber := flagSet.Int("issue", 0, "Issue 编号")
	showVersion := flagSet.Bool("version", false, "输出版本信息")

	if err := flagSet.Parse(args); err != nil {
		return 1
	}

	if *showVersion {
		fmt.Fprintln(stdout, version.Info())
		return 0
	}

	if *repo == "" || *issueNumber <= 0 {
		writeUsage(stderr)
		return 1
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	fmt.Fprintf(stdout, "🐝 code-bee %s\n", version.Version)
	fmt.Fprintf(stdout, "📦 仓库: %s | Issue: #%d\n\n", *repo, *issueNumber)

	cfg := config.New(*repo, *issueNumber)
	service := newService()

	fmt.Fprintln(stdout, "🚀 正在执行四阶段 harness：Issue 处理 -> 编码 -> 审查 -> Issue 提交...")

	result, err := service.Dispatch(ctx, cfg)
	if err != nil {
		fmt.Fprintf(stderr, "\n❌ 执行失败: %v\n", err)
		return 1
	}

	return mapDispatchResultToExitCode(result, stdout, stderr)
}

// newDefaultService 创建生产环境默认使用的调度服务。
//
// 返回值:
// - dispatchService: 基于 GitHub 平台技能包和 reasonix 运行器的正式调度服务
func newDefaultService() dispatchService {
	return pipeline.NewService(
		platformgithub.NewClient(),
		agent.New(),
	)
}

// writeUsage 向错误输出写入 CLI 使用说明。
//
// 输入参数:
// - stderr: 错误输出目标
func writeUsage(stderr io.Writer) {
	fmt.Fprintf(stderr, "用法: code-bee --repo <owner/repo> --issue <number>\n\n")
	fmt.Fprintf(stderr, "示例:\n")
	fmt.Fprintf(stderr, "  code-bee --repo owner/repo --issue 42\n")
}

// mapDispatchResultToExitCode 将 pipeline 结果映射为用户可见的消息与进程退出码。
//
// 输入参数:
// - result: 四阶段 harness 的最终执行结果
// - stdout: 标准输出目标
// - stderr: 错误输出目标
//
// 返回值:
// - int: 0 表示任务完成，1 表示失败、阻塞或需要人工介入
func mapDispatchResultToExitCode(result *pipeline.Result, stdout io.Writer, stderr io.Writer) int {
	if result == nil {
		fmt.Fprintln(stderr, "\n❌ 调度器返回了空结果")
		return 1
	}

	if result.Success && result.Completed {
		fmt.Fprintln(stdout, "\n✅ reviewer 已明确 PASS，且最终 Issue 回复已提交，任务执行完成")
		return 0
	}

	if result.Success && result.Blocked {
		fmt.Fprintf(stderr, "\n⏸️ 流程已阻塞，需要补充信息或人工介入:\n%s\n", result.Output)
		return 1
	}

	if result.Success && result.ManualRequired {
		fmt.Fprintf(stderr, "\n🧑‍🔧 自动循环已触达兜底阈值，需要人工介入:\n%s\n", result.Output)
		return 1
	}

	if result.Success {
		fmt.Fprintf(stderr, "\n⏳ 自动循环结束但 reviewer 未 PASS:\n%s\n", result.Output)
		return 1
	}

	fmt.Fprintf(stderr, "\n❌ 智能体返回失败:\n%s\n", result.Output)
	return 1
}
