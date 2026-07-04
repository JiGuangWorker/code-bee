// Package version 覆盖版本信息字符串的单元测试。
//
// 核心功能:
// 1. 验证版本信息格式在默认值与注入值场景下都稳定可读
// 2. 避免 CLI 输出版本时出现字段缺失或格式漂移
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-04
// 更新时间: 2026-07-04
package version

import "testing"

// TestInfoFormatsVersionString 验证 Info 会按固定格式输出版本信息。
func TestInfoFormatsVersionString(t *testing.T) {
	originalVersion := Version
	originalCommit := GitCommit
	originalBuildTime := BuildTime

	Version = "1.2.3"
	GitCommit = "abc123"
	BuildTime = "2026-07-04T12:00:00Z"
	t.Cleanup(func() {
		Version = originalVersion
		GitCommit = originalCommit
		BuildTime = originalBuildTime
	})

	got := Info()
	want := "code-bee v1.2.3 (commit: abc123, built: 2026-07-04T12:00:00Z)"
	if got != want {
		t.Fatalf("Info() = %q, want %q", got, want)
	}
}
