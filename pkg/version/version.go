// Package version 提供构建版本信息。
package version

import "fmt"

// 构建时通过 -ldflags 注入，零值时使用默认值。
var (
	Version   = "0.1.0"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

// Info 返回完整的版本信息字符串。
func Info() string {
	return fmt.Sprintf("code-bee v%s (commit: %s, built: %s)", Version, GitCommit, BuildTime)
}
