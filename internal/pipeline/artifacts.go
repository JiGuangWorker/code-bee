// Package pipeline 管理 code-bee 在本地工作区中的阶段工件目录。
//
// 核心功能:
// 1. 为每个 repo/issue 组合创建稳定的运行目录，便于智能体通过文件交接结果
// 2. 统一提供四阶段结果文件路径，避免调度层散落硬编码路径
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-04
// 更新时间: 2026-07-04
package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const runArtifactsRoot = ".code-bee/runs"
const artifactDirPermission = 0o755

// ArtifactSet 表示单个 repo/issue 在本地工作区下的工件目录集合。
type ArtifactSet struct {
	// BaseDir 是本次运行对应的根目录，所有阶段结果文件都写在这里。
	BaseDir string
}

// NewArtifactSet 为指定 repo/issue 创建稳定的工件目录。
//
// 输入参数:
// - repo: 目标仓库名，格式通常为 owner/repo
// - issueNumber: 当前 Issue 编号
//
// 返回值:
// - *ArtifactSet: 可用于获取各阶段结果文件路径的目录描述对象
// - error: 当创建目录失败时返回错误
func NewArtifactSet(repo string, issueNumber int) (*ArtifactSet, error) {
	baseDir := filepath.Join(
		runArtifactsRoot,
		sanitizeRepoName(repo),
		fmt.Sprintf("issue-%d", issueNumber),
	)

	if err := os.MkdirAll(baseDir, artifactDirPermission); err != nil {
		return nil, fmt.Errorf("create artifact directory %s: %w", baseDir, err)
	}

	return &ArtifactSet{BaseDir: baseDir}, nil
}

// IssueHandlingResultPath 返回 intake 结果文件路径。
func (a *ArtifactSet) IssueHandlingResultPath() string {
	return filepath.Join(a.BaseDir, "issue_intake_result.json")
}

// RoundDirPath 返回指定轮次的历史工件目录。
//
// 输入参数:
// - round: 当前 coder-reviewer loop 的轮次，从 1 开始
//
// 返回值:
// - string: 当前轮次对应的稳定目录路径
//
// 调用注意事项:
// - round 小于等于 0 时会回退到 round-00，避免出现非法空目录名
func (a *ArtifactSet) RoundDirPath(round int) string {
	if round < 0 {
		round = 0
	}

	return filepath.Join(a.BaseDir, fmt.Sprintf("round-%02d", round))
}

// EnsureRoundDir 为指定轮次创建历史工件目录。
//
// 输入参数:
// - round: 当前 coder-reviewer loop 的轮次
//
// 返回值:
// - error: 当目录创建失败时返回错误
func (a *ArtifactSet) EnsureRoundDir(round int) error {
	roundDir := a.RoundDirPath(round)
	if err := os.MkdirAll(roundDir, artifactDirPermission); err != nil {
		return fmt.Errorf("create round artifact directory %s: %w", roundDir, err)
	}

	return nil
}

// CodingResultPath 返回指定轮次的 coding 结果文件路径。
func (a *ArtifactSet) CodingResultPath(round int) string {
	return filepath.Join(a.RoundDirPath(round), "coding_result.json")
}

// ReviewResultPath 返回指定轮次的 review 结果文件路径。
func (a *ArtifactSet) ReviewResultPath(round int) string {
	return filepath.Join(a.RoundDirPath(round), "review_result.json")
}

// LoopJudgeResultPath 返回指定轮次的 loop judge 结果文件路径。
func (a *ArtifactSet) LoopJudgeResultPath(round int) string {
	return filepath.Join(a.RoundDirPath(round), "loop_judge_result.json")
}

// LoopHistoryPath 返回跨轮次聚合历史文件路径。
func (a *ArtifactSet) LoopHistoryPath() string {
	return filepath.Join(a.BaseDir, "loop_history.json")
}

// IssuePostResultPath 返回指定 purpose 下的 Issue 提交结果文件路径。
func (a *ArtifactSet) IssuePostResultPath(purpose string) string {
	safePurpose := strings.TrimSpace(purpose)
	if safePurpose == "" {
		safePurpose = "generic"
	}

	return filepath.Join(a.BaseDir, fmt.Sprintf("issue_post_result_%s.json", safePurpose))
}

// sanitizeRepoName 将 owner/repo 形式的仓库名转换为安全目录名。
func sanitizeRepoName(repo string) string {
	replacer := strings.NewReplacer("/", "__", "\\", "__", " ", "_", ":", "_")
	sanitized := replacer.Replace(strings.TrimSpace(repo))
	if sanitized == "" {
		return "unknown_repo"
	}

	return sanitized
}
