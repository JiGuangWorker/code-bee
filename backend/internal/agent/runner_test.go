// Package agent 覆盖智能体运行器的单元测试。
//
// 核心功能:
// 1. 验证 Runner 在外部命令成功和失败时都会保留原始输出
// 2. 避免 reasonix 执行失败后上层拿不到现场，导致问题归因中断
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-04
// 更新时间: 2026-07-04
package agent

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

// TestRunnerRunReturnsSuccessOutput 验证外部命令成功时，Runner 会返回成功状态和完整输出。
func TestRunnerRunReturnsSuccessOutput(t *testing.T) {
	originalExecCommandContext := execCommandContext
	execCommandContext = fakeExecCommandContext("success", "mock success output")
	t.Cleanup(func() {
		execCommandContext = originalExecCommandContext
	})

	runner := New()
	result, err := runner.Run(context.Background(), TaskKindCoding, "demo task")
	if err != nil {
		t.Fatalf("Runner.Run() unexpected error: %v", err)
	}

	if result == nil || !result.Success {
		t.Fatalf("Runner.Run() result = %+v, want success=true", result)
	}

	if result.Output != "mock success output" {
		t.Fatalf("Runner.Run() output = %q, want %q", result.Output, "mock success output")
	}
}

// TestRunnerRunReturnsFailureOutput 验证外部命令失败时，Runner 仍会返回原始输出供上层排障。
func TestRunnerRunReturnsFailureOutput(t *testing.T) {
	originalExecCommandContext := execCommandContext
	execCommandContext = fakeExecCommandContext("fail", "mock failure output")
	t.Cleanup(func() {
		execCommandContext = originalExecCommandContext
	})

	runner := New()
	result, err := runner.Run(context.Background(), TaskKindReview, "demo task")
	if err == nil {
		t.Fatal("Runner.Run() error = nil, want non-nil error")
	}

	if result == nil || result.Success {
		t.Fatalf("Runner.Run() result = %+v, want success=false", result)
	}

	if result.Output != "mock failure output" {
		t.Fatalf("Runner.Run() output = %q, want %q", result.Output, "mock failure output")
	}

	if !strings.Contains(err.Error(), "mock failure output") {
		t.Fatalf("Runner.Run() error = %q, want contains %q", err.Error(), "mock failure output")
	}
}

// fakeExecCommandContext 构造可控的命令执行器，供 Runner 单元测试注入成功或失败场景。
//
// 输入参数:
// - mode: success 表示辅助进程以 0 退出；fail 表示辅助进程以 1 退出
// - output: 辅助进程写到标准输出的内容
//
// 返回值:
// - func: 可替代 exec.CommandContext 的命令构造函数
func fakeExecCommandContext(mode string, output string) func(context.Context, string, ...string) *exec.Cmd {
	return func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		exitCode := "0"
		if mode == "fail" {
			exitCode = "1"
		}

		script := "printf '%s' \"$GO_HELPER_OUTPUT\"; exit " + exitCode
		cmd := exec.CommandContext(ctx, "/bin/sh", "-c", script)
		cmd.Env = append(cmd.Env, "GO_HELPER_OUTPUT="+output)
		return cmd
	}
}
