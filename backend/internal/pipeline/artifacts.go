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

// CodingResultPath 返回 coding 结果文件路径。
func (a *ArtifactSet) CodingResultPath() string {
	return filepath.Join(a.BaseDir, "coding_result.json")
}

// ReviewResultPath 返回 review 结果文件路径。
func (a *ArtifactSet) ReviewResultPath() string {
	return filepath.Join(a.BaseDir, "review_result.json")
}

// IssuePostResultPath 返回指定 purpose 下的 Issue 提交结果文件路径。
func (a *ArtifactSet) IssuePostResultPath(purpose string) string {
	safePurpose := strings.TrimSpace(purpose)
	if safePurpose == "" {
		safePurpose = "generic"
	}

	return filepath.Join(a.BaseDir, fmt.Sprintf("issue_post_result_%s.json", safePurpose))
}

// EnsureRoundDir 为指定轮次创建工件子目录。
func (a *ArtifactSet) EnsureRoundDir(round int) error {
	dir := filepath.Join(a.BaseDir, fmt.Sprintf("round-%02d", round))
	if err := os.MkdirAll(dir, artifactDirPermission); err != nil {
		return fmt.Errorf("create round directory %s: %w", dir, err)
	}
	return nil
}

// CodingResultPathForRound 返回指定轮次的 coding 结果文件路径。
func (a *ArtifactSet) CodingResultPathForRound(round int) string {
	return filepath.Join(a.BaseDir, fmt.Sprintf("round-%02d", round), "coding_result.json")
}

// ReviewResultPathForRound 返回指定轮次的 review 结果文件路径。
func (a *ArtifactSet) ReviewResultPathForRound(round int) string {
	return filepath.Join(a.BaseDir, fmt.Sprintf("round-%02d", round), "review_result.json")
}

// LoopJudgeResultPath 返回指定轮次的 loop judge 结果文件路径。
func (a *ArtifactSet) LoopJudgeResultPath(round int) string {
	return filepath.Join(a.BaseDir, fmt.Sprintf("round-%02d", round), "loop_judge_result.json")
}

// LoopHistoryPath 返回聚合历史文件路径。
func (a *ArtifactSet) LoopHistoryPath() string {
	return filepath.Join(a.BaseDir, "loop_history.json")
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
