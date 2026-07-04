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

// dispatchService 是调度器的抽象接口，使 CLI 逻辑可独立于具体的 pipeline.Service 进行测试。
type dispatchService interface {
	Dispatch(context.Context, *config.Config) (*pipeline.Result, error)
}

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
	os.Exit(runCLI(os.Args[1:], os.Stdout, os.Stderr, func() dispatchService {
		return pipeline.NewService(
			platformgithub.NewClient(),
			agent.New(),
		)
	}))
}

// runCLI 是 CLI 入口的可测试版本，接收注入的 io.Writer 和 serviceFactory。
//
// 返回值:
// - 0: 成功完成
// - 1: 参数错误、调度失败或需要人工介入
func runCLI(args []string, stdout, stderr io.Writer, serviceFactory func() dispatchService) int {
	fs := flag.NewFlagSet("code-bee", flag.ContinueOnError)
	fs.SetOutput(stderr)

	repo := fs.String("repo", "", "仓库地址，如 owner/repo")
	issueNumber := fs.Int("issue", 0, "Issue 编号")
	showVersion := fs.Bool("version", false, "输出版本信息")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "用法: code-bee --repo <owner/repo> --issue <number>\n\n")
		fmt.Fprintf(stderr, "示例:\n")
		fmt.Fprintf(stderr, "  code-bee --repo owner/repo --issue 42\n")
		return 1
	}

	if *showVersion {
		fmt.Fprintln(stdout, version.Info())
		return 0
	}

	if *repo == "" || *issueNumber <= 0 {
		fmt.Fprintf(stderr, "用法: code-bee --repo <owner/repo> --issue <number>\n\n")
		fmt.Fprintf(stderr, "示例:\n")
		fmt.Fprintf(stderr, "  code-bee --repo owner/repo --issue 42\n")
		return 1
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	fmt.Fprintf(stdout, "🐝 code-bee %s\n", version.Version)
	fmt.Fprintf(stdout, "📦 仓库: %s | Issue: #%d\n\n", *repo, *issueNumber)

	cfg := config.New(*repo, *issueNumber)
	service := serviceFactory()

	fmt.Fprintln(stdout, "🚀 正在执行四阶段 harness：Issue 处理 -> 编码 -> 审查 -> Issue 提交...")

	result, err := service.Dispatch(ctx, cfg)
	if err != nil {
		fmt.Fprintf(stderr, "\n❌ 执行失败: %v\n", err)
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
