// 本文件把 pipeline.ArtifactSet 适配为 runtime.ArtifactResolver 接口。
//
// 设计说明:
// 1. runtime 包不依赖 pipeline（避免循环依赖），通过接口抽象文件契约
// 2. stageName → 文件路径的映射集中在此处，复用 ArtifactSet 的现有方法
// 3. LoadResult 路由到 contracts.go 的 typed loader，保留结果校验能力
// 4. coding/review 使用非轮次路径（每轮覆盖），与原 dispatch.go 行为一致；
//    loop-judge 使用轮次路径，便于回放
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-05

package pipeline

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/JiGuangWorker/code-bee/internal/runtime"
)

// ArtifactResolverAdapter 把 ArtifactSet 适配为 runtime.ArtifactResolver。
//
// stageName 映射约定（与 default_workflow.yaml 中的 stage name 对齐）:
// - issue-handling: 接单阶段结果
// - coding: 编码阶段结果（每轮覆盖）
// - review: 审查阶段结果（每轮覆盖）
// - issue-post-intake / issue-post-blocked / issue-post-completion: Issue 提交结果，按 purpose 区分文件
// - loop-judge: 价值评估结果（按轮次存盘）
type ArtifactResolverAdapter struct {
	artifacts *ArtifactSet
}

// NewArtifactResolverAdapter 创建适配器实例。
func NewArtifactResolverAdapter(a *ArtifactSet) *ArtifactResolverAdapter {
	return &ArtifactResolverAdapter{artifacts: a}
}

// 编译期断言：确保 ArtifactResolverAdapter 实现 runtime.ArtifactResolver 接口。
var _ runtime.ArtifactResolver = (*ArtifactResolverAdapter)(nil)

// ResolveResultFile 返回指定 stage 的结果文件路径。
//
// 参数:
// - stageName: workflow.yaml 中 stage 的 name
// - args: stage 的 args map（issue-post 用 purpose 字段）
// - loopRound: 当前 loop 轮次，0 表示非 loop 内执行
func (a *ArtifactResolverAdapter) ResolveResultFile(stageName string, args map[string]any, loopRound int) string {
	switch stageName {
	case "issue-handling":
		return a.artifacts.IssueHandlingResultPath()

	case "coding":
		// 编码结果每轮覆盖，与原 dispatch.go 行为一致
		return a.artifacts.CodingResultPath()

	case "review":
		// 审查结果每轮覆盖，与原 dispatch.go 行为一致
		return a.artifacts.ReviewResultPath()

	case "loop-judge":
		if loopRound <= 0 {
			loopRound = 1
		}
		return a.artifacts.LoopJudgeResultPath(loopRound)
	}

	// issue-post-* 系列：按 purpose 区分文件
	if strings.HasPrefix(stageName, "issue-post-") {
		purpose := getStringFromArgs(args, "purpose")
		if purpose == "" {
			purpose = "generic"
		}
		return a.artifacts.IssuePostResultPath(purpose)
	}

	// 兜底：BaseDir/<stageName>.json
	return filepath.Join(a.artifacts.BaseDir, stageName+".json")
}

// ResetResultFile 删除旧的结果文件，防止读到上一轮残留。
func (a *ArtifactResolverAdapter) ResetResultFile(path string) error {
	return resetResultFile(path)
}

// LoadResult 从结果文件加载结构化数据。
//
// 按 stageName 路由到对应的 typed loader（保留校验），再转成 map[string]any。
// 未知 stageName 直接 json.Unmarshal 到 map（无校验）。
func (a *ArtifactResolverAdapter) LoadResult(stageName string, path string) (map[string]any, error) {
	var typed any
	var err error

	switch stageName {
	case "issue-handling":
		typed, err = loadIssueHandlingResult(path)
	case "coding":
		typed, err = loadCodingResult(path)
	case "review":
		typed, err = loadReviewResult(path)
	case "loop-judge":
		typed, err = loadLoopJudgeResult(path)
	default:
		if strings.HasPrefix(stageName, "issue-post-") {
			typed, err = loadIssuePostResult(path)
		} else {
			return loadRawMap(path)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("LoadResult[%s]: %w", stageName, err)
	}

	return structToMap(typed)
}

// LoopHistoryPath 返回 loop 历史文件路径。
func (a *ArtifactResolverAdapter) LoopHistoryPath() string {
	return a.artifacts.LoopHistoryPath()
}

// getStringFromArgs 从 args map 中取字符串值。
func getStringFromArgs(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	v, ok := args[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

// structToMap 把 typed struct 通过 JSON 序列化/反序列化转成 map[string]any。
//
// 用于把 contracts.go 的 typed Result（如 *CodingResult）转成 runtime 期望的 map 形式，
// 保留字段名（由 json tag 决定）。
func structToMap(v any) (map[string]any, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("structToMap marshal: %w", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("structToMap unmarshal: %w", err)
	}

	return m, nil
}

// loadRawMap 直接把文件内容反序列化到 map（无业务校验）。
//
// 用于 workflow.yaml 中未识别的 stageName，保证向前兼容。
func loadRawMap(path string) (map[string]any, error) {
	var m map[string]any
	if err := loadJSONFile(path, &m); err != nil {
		return nil, fmt.Errorf("loadRawMap: %w", err)
	}
	return m, nil
}
