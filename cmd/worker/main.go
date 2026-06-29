// code-bee · CLI 入口
//
// 用法:
//
//	code-bee --repo owner/repo --issue 42
//
// code-bee 只做一件事：告诉编码智能体去看哪个仓库的哪个 Issue。
// 所有编码操作（读 Issue、写代码、提 PR）由智能体自行完成。
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/JiGuangWorker/code-bee/internal/agent"
	"github.com/JiGuangWorker/code-bee/pkg/version"
)

func main() {
	repo := flag.String("repo", "", "仓库地址，如 owner/repo")
	issueNumber := flag.Int("issue", 0, "Issue 编号")
	showVersion := flag.Bool("version", false, "输出版本信息")
	flag.Parse()

	if *showVersion {
		fmt.Println(version.Info())
		return
	}

	if *repo == "" || *issueNumber <= 0 {
		fmt.Fprintf(os.Stderr, "用法: code-bee --repo <owner/repo> --issue <number>\n\n")
		fmt.Fprintf(os.Stderr, "示例:\n")
		fmt.Fprintf(os.Stderr, "  code-bee --repo owner/repo --issue 42\n")
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	fmt.Printf("🐝 code-bee %s\n", version.Version)
	fmt.Printf("📦 仓库: %s | Issue: #%d\n\n", *repo, *issueNumber)

	// 生成一句话任务，让智能体去看 Issue
	task := fmt.Sprintf(
		"请查看 %s 仓库的 #%d Issue，并完成其中的编码任务。",
		*repo, *issueNumber,
	)

	fmt.Printf("🚀 正在通知编码智能体: %s\n", task)

	runner := agent.New()
	result, err := runner.Run(ctx, task)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n❌ 执行失败: %v\n", err)
		os.Exit(1)
	}

	if result.Success {
		fmt.Println("\n✅ 任务执行完成")
	} else {
		fmt.Fprintf(os.Stderr, "\n❌ 智能体返回失败:\n%s\n", result.Output)
		os.Exit(1)
	}
}
