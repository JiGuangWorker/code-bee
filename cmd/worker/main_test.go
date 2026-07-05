// Package main 覆盖 CLI 入口的单元测试。
//
// 核心功能:
// 1. 验证参数解析、版本输出、使用说明和退出码映射是否符合预期
// 2. 验证不同调度结果会被稳定映射为用户可见消息，避免 CLI 行为漂移
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-04
// 更新时间: 2026-07-04
package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/JiGuangWorker/code-bee/internal/config"
	"github.com/JiGuangWorker/code-bee/internal/pipeline"
	"github.com/JiGuangWorker/code-bee/pkg/version"
)

// TestRunCLIReturnsVersion 验证 --version 会输出版本信息并以 0 退出。
func TestRunCLIReturnsVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runCLI([]string{"--version"}, &stdout, &stderr, func() (dispatchService, error) {
		t.Fatal("service factory should not be called for --version")
		return nil, nil
	})

	if exitCode != 0 {
		t.Fatalf("runCLI(--version) exitCode = %d, want 0", exitCode)
	}

	if !strings.Contains(stdout.String(), version.Info()) {
		t.Fatalf("runCLI(--version) stdout = %q, want contains %q", stdout.String(), version.Info())
	}
}

// TestRunCLIRejectsMissingRequiredArgs 验证缺少 repo 或 issue 时会输出使用说明并返回失败。
func TestRunCLIRejectsMissingRequiredArgs(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runCLI([]string{"--repo", "owner/repo"}, &stdout, &stderr, func() (dispatchService, error) {
		t.Fatal("service factory should not be called when args are invalid")
		return nil, nil
	})

	if exitCode != 1 {
		t.Fatalf("runCLI(invalid args) exitCode = %d, want 1", exitCode)
	}

	if !strings.Contains(stderr.String(), "用法: code-bee") {
		t.Fatalf("runCLI(invalid args) stderr = %q, want usage text", stderr.String())
	}
}

// TestRunCLIReturnsSuccessWhenDispatchCompleted 验证调度成功完成时 CLI 以 0 退出。
func TestRunCLIReturnsSuccessWhenDispatchCompleted(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runCLI([]string{"--repo", "owner/repo", "--issue", "8"}, &stdout, &stderr, func() (dispatchService, error) {
		return fakeDispatchService{
			result: &pipeline.Result{
				Success:   true,
				Completed: true,
			},
		}, nil
	})

	if exitCode != 0 {
		t.Fatalf("runCLI(success) exitCode = %d, want 0", exitCode)
	}

	if !strings.Contains(stdout.String(), "任务执行完成") {
		t.Fatalf("runCLI(success) stdout = %q, want completion message", stdout.String())
	}
}

// TestRunCLIReturnsBlockedMessage 验证阻塞态会输出阻塞说明并返回失败退出码。
func TestRunCLIReturnsBlockedMessage(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runCLI([]string{"--repo", "owner/repo", "--issue", "9"}, &stdout, &stderr, func() (dispatchService, error) {
		return fakeDispatchService{
			result: &pipeline.Result{
				Success: true,
				Blocked: true,
				Output:  "缺少外部接口权限。",
			},
		}, nil
	})

	if exitCode != 1 {
		t.Fatalf("runCLI(blocked) exitCode = %d, want 1", exitCode)
	}

	if !strings.Contains(stderr.String(), "缺少外部接口权限") {
		t.Fatalf("runCLI(blocked) stderr = %q, want blocked output", stderr.String())
	}
}

// TestRunCLIReturnsManualRequiredMessage 验证人工介入态会输出人工接管说明并返回失败退出码。
func TestRunCLIReturnsManualRequiredMessage(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runCLI([]string{"--repo", "owner/repo", "--issue", "10"}, &stdout, &stderr, func() (dispatchService, error) {
		return fakeDispatchService{
			result: &pipeline.Result{
				Success:        true,
				ManualRequired: true,
				Output:         "自动循环已无价值。",
			},
		}, nil
	})

	if exitCode != 1 {
		t.Fatalf("runCLI(manual required) exitCode = %d, want 1", exitCode)
	}

	if !strings.Contains(stderr.String(), "自动循环已触达兜底阈值") {
		t.Fatalf("runCLI(manual required) stderr = %q, want manual-required message", stderr.String())
	}
}

// TestRunCLIReturnsDispatchError 验证调度器返回系统级错误时 CLI 会输出错误并返回失败退出码。
func TestRunCLIReturnsDispatchError(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runCLI([]string{"--repo", "owner/repo", "--issue", "11"}, &stdout, &stderr, func() (dispatchService, error) {
		return fakeDispatchService{
			err: errors.New("dispatch failed"),
		}, nil
	})

	if exitCode != 1 {
		t.Fatalf("runCLI(dispatch error) exitCode = %d, want 1", exitCode)
	}

	if !strings.Contains(stderr.String(), "dispatch failed") {
		t.Fatalf("runCLI(dispatch error) stderr = %q, want contains dispatch error", stderr.String())
	}
}

// TestRunCLIReturnsFactoryError 验证 factory 返回错误时 CLI 会输出初始化失败并返回失败退出码。
func TestRunCLIReturnsFactoryError(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runCLI([]string{"--repo", "owner/repo", "--issue", "12"}, &stdout, &stderr, func() (dispatchService, error) {
		return nil, errors.New("workflow load failed")
	})

	if exitCode != 1 {
		t.Fatalf("runCLI(factory error) exitCode = %d, want 1", exitCode)
	}

	if !strings.Contains(stderr.String(), "初始化失败") {
		t.Fatalf("runCLI(factory error) stderr = %q, want contains 初始化失败", stderr.String())
	}
}

// fakeDispatchService 是供 CLI 单元测试注入的最小假调度器。
type fakeDispatchService struct {
	result *pipeline.Result
	err    error
}

// Dispatch 返回预设结果，用于覆盖 CLI 的退出码映射。
func (f fakeDispatchService) Dispatch(context.Context, *config.Config) (*pipeline.Result, error) {
	return f.result, f.err
}
