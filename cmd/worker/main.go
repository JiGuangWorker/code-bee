// code-bee · CLI 入口
//
// 用法:
//
//	code-bee --repo JiGuangWorker/DeepSeek-Reasonix --issue 42
//
// 通过 Issue 驱动 AI Agent 执行编码任务，并自动创建 PR。
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/JiGuangWorker/code-bee/internal/pipeline"
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
		fmt.Fprintf(os.Stderr, "  export GITHUB_TOKEN=ghp_xxx\n")
		fmt.Fprintf(os.Stderr, "  code-bee --repo JiGuangWorker/DeepSeek-Reasonix --issue 42\n")
		os.Exit(1)
	}

	// 捕获中断信号，支持优雅退出
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	fmt.Printf("🐝 code-bee %s\n", version.Version)
	fmt.Printf("📦 仓库: %s | Issue: #%d\n\n", *repo, *issueNumber)

	if err := pipeline.Run(ctx, *repo, *issueNumber); err != nil {
		fmt.Fprintf(os.Stderr, "\n❌ 执行失败: %v\n", err)
		os.Exit(1)
	}
}
