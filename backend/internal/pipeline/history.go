// Package pipeline 管理 coder-reviewer loop 的历史快照文件。
//
// 核心功能:
// 1. 将每一轮的 coding/review 结果聚合为稳定的历史文件，供 loop judge 与测试回放复用
// 2. 避免调度器仅依赖最后一轮结果，导致无法判断问题是否收敛或是否进入空转
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-04
// 更新时间: 2026-07-04
package pipeline

import (
	"encoding/json"
	"fmt"
	"os"
)

const historyFilePermission = 0o600

// LoopHistory 表示整个 coder-reviewer loop 的历史聚合结果。
type LoopHistory struct {
	// Repo 是当前运行关联的仓库名，格式为 owner/repo。
	Repo string `json:"repo"`

	// IssueNumber 是当前运行关联的 Issue 编号。
	IssueNumber int `json:"issue_number"`

	// MaxRounds 是本次自动循环允许执行的最大轮数。
	MaxRounds int `json:"max_rounds"`

	// Rounds 是按时间顺序累计的逐轮历史记录。
	Rounds []LoopHistoryRound `json:"rounds"`
}

// LoopHistoryRound 表示单轮 coder-reviewer 结果快照。
type LoopHistoryRound struct {
	// Round 是当前快照对应的轮次编号，从 1 开始。
	Round int `json:"round"`

	// CodingResult 是当前轮的编码结果结构化快照。
	CodingResult *CodingResult `json:"coding_result"`

	// ReviewResult 是当前轮的审查结果结构化快照。
	ReviewResult *ReviewResult `json:"review_result"`
}

// appendRound 将单轮 coding/review 快照追加到历史中。
//
// 输入参数:
// - round: 当前轮次编号
// - codingResult: 当前轮编码结果，不能为空
// - reviewResult: 当前轮审查结果，不能为空
//
// 返回值:
// - error: 当输入为空或轮次非法时返回错误
func (h *LoopHistory) appendRound(round int, codingResult *CodingResult, reviewResult *ReviewResult) error {
	if round <= 0 {
		return fmt.Errorf("invalid history round %d", round)
	}

	if codingResult == nil {
		return fmt.Errorf("nil coding result for history round %d", round)
	}

	if reviewResult == nil {
		return fmt.Errorf("nil review result for history round %d", round)
	}

	h.Rounds = append(h.Rounds, LoopHistoryRound{
		Round:        round,
		CodingResult: codingResult,
		ReviewResult: reviewResult,
	})

	return nil
}

// saveLoopHistory 将历史聚合结果稳定写入 JSON 文件。
//
// 输入参数:
// - filePath: 历史文件输出路径
// - history: 待写入的历史数据
//
// 返回值:
// - error: 当序列化或写文件失败时返回错误
func saveLoopHistory(filePath string, history *LoopHistory) error {
	if history == nil {
		return fmt.Errorf("nil loop history")
	}

	content, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal loop history %s: %w", filePath, err)
	}

	if err := os.WriteFile(filePath, content, historyFilePermission); err != nil {
		return fmt.Errorf("write loop history %s: %w", filePath, err)
	}

	return nil
}
