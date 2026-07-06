// 本文件实现 ExecutionContext 和 LoopState，负责 stage 间数据传递和 loop 状态维护。
//
// 设计说明:
// 1. stages map 维护每个 stage 的最新 StepResult，供 exit_when/when 条件求值用
// 2. loopStack 支持嵌套 loop，栈顶为当前 loop
// 3. Snapshot/Merge/CloneForBranch 用于 parallel 分支的写隔离
// 4. Lookup 支持 dot path 嵌套取值，如 "check_result.passed"
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-05

package runtime

import (
	"fmt"
	"sync"

	"github.com/JiGuangWorker/code-bee/internal/config"
	"github.com/JiGuangWorker/code-bee/internal/schema"
)

// ExecutionContext 是贯穿整个 workflow 执行的上下文容器。
//
// 职责:
// - 持有 workflow 配置、平台上下文、配置等全局信息
// - 维护 stage 结果 map，供后续 stage 和条件求值引用
// - 维护 loop 栈，支持嵌套 loop
// - 提供 parallel 分支的写隔离能力
type ExecutionContext struct {
	// Workflow 是校验后的 workflow 配置。
	Workflow *schema.Workflow

	// Artifacts 是文件契约解析器，由 pipeline 层注入。
	Artifacts ArtifactResolver

	// Platform 是平台上下文。
	Platform PlatformContext

	// Config 是 code-bee 配置。
	Config *config.Config

	mu        sync.RWMutex
	stages    map[string]*StepResult // stage name → 最新结果
	loopStack []*LoopState           // 嵌套 loop 栈，栈顶为当前 loop
}

// NewExecutionContext 创建新的执行上下文。
func NewExecutionContext(wf *schema.Workflow, artifacts ArtifactResolver, platform PlatformContext, cfg *config.Config) *ExecutionContext {
	return &ExecutionContext{
		Workflow:  wf,
		Artifacts: artifacts,
		Platform:  platform,
		Config:    cfg,
		stages:    make(map[string]*StepResult),
	}
}

// Lookup 按 dot path 从指定 stage 的结果中取值。
//
// 用法:
// - stageName="review", fieldPath="status" → review 结果的 status 字段
// - stageName="review", fieldPath="check_result.passed" → 嵌套取值
// - stageName="review", fieldPath="" → 返回整个 Data map（作为 any）
//
// 返回值:
// - val: 找到的值（fieldPath="" 时返回 map[string]any）
// - ok: 是否找到
func (ec *ExecutionContext) Lookup(stageName, fieldPath string) (any, bool) {
	ec.mu.RLock()
	defer ec.mu.RUnlock()

	r, ok := ec.stages[stageName]
	if !ok || r == nil || r.Data == nil {
		return nil, false
	}

	if fieldPath == "" {
		return r.Data, true
	}

	return lookupByPath(r.Data, fieldPath)
}

// lookupByPath 按 dot path 从 map 中取值。
func lookupByPath(data map[string]any, path string) (any, bool) {
	keys := splitDotPath(path)
	var cur any = data
	for _, k := range keys {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		v, ok := m[k]
		if !ok {
			return nil, false
		}
		cur = v
	}
	return cur, true
}

// splitDotPath 把 "a.b.c" 拆成 ["a","b","c"]。
func splitDotPath(path string) []string {
	var keys []string
	start := 0
	for i := 0; i < len(path); i++ {
		if path[i] == '.' {
			if i > start {
				keys = append(keys, path[start:i])
			}
			start = i + 1
		}
	}
	if start < len(path) {
		keys = append(keys, path[start:])
	}
	return keys
}

// SetResult 记录 stage 的执行结果。
func (ec *ExecutionContext) SetResult(stageName string, r *StepResult) {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	ec.stages[stageName] = r
}

// GetResult 获取 stage 的最新结果。
func (ec *ExecutionContext) GetResult(stageName string) (*StepResult, bool) {
	ec.mu.RLock()
	defer ec.mu.RUnlock()
	r, ok := ec.stages[stageName]
	return r, ok
}

// Snapshot 返回当前所有 stage 结果的快照（浅拷贝）。
//
// 用于 parallel 分支启动前的基线，分支内的写操作不影响其他分支。
func (ec *ExecutionContext) Snapshot() map[string]*StepResult {
	ec.mu.RLock()
	defer ec.mu.RUnlock()
	snap := make(map[string]*StepResult, len(ec.stages))
	for k, v := range ec.stages {
		snap[k] = v
	}
	return snap
}

// Merge 把快照中的结果合并回主上下文。
//
// 用于 parallel 分支完成后回写成功分支的结果。
func (ec *ExecutionContext) Merge(snap map[string]*StepResult) {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	for k, v := range snap {
		ec.stages[k] = v
	}
}

// CloneForBranch 创建一个用于 parallel 分支的子上下文。
//
// 子上下文共享 Workflow/Artifacts/Platform/Config，但有独立的 stages map
// （以 base 为初始值）和共享的 loopStack（loop 内的 parallel 仍属于同一个 loop）。
func (ec *ExecutionContext) CloneForBranch(base map[string]*StepResult) *ExecutionContext {
	ec.mu.RLock()
	defer ec.mu.RUnlock()

	branchStages := make(map[string]*StepResult, len(base))
	for k, v := range base {
		branchStages[k] = v
	}

	return &ExecutionContext{
		Workflow:  ec.Workflow,
		Artifacts: ec.Artifacts,
		Platform:  ec.Platform,
		Config:    ec.Config,
		stages:    branchStages,
		loopStack: ec.loopStack, // 共享 loop 栈，parallel 分支内仍在同一 loop 上下文
	}
}

// CurrentLoop 返回栈顶的 loop 状态，非 loop 内执行时返回 nil。
func (ec *ExecutionContext) CurrentLoop() *LoopState {
	ec.mu.RLock()
	defer ec.mu.RUnlock()
	if len(ec.loopStack) == 0 {
		return nil
	}
	return ec.loopStack[len(ec.loopStack)-1]
}

// PushLoop 把一个 loop 状态压栈。
func (ec *ExecutionContext) PushLoop(s *LoopState) {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	ec.loopStack = append(ec.loopStack, s)
}

// PopLoop 弹出栈顶 loop 状态。
func (ec *ExecutionContext) PopLoop() {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	if len(ec.loopStack) == 0 {
		return
	}
	ec.loopStack = ec.loopStack[:len(ec.loopStack)-1]
}

// LoopState 维护单个 loop 的执行状态。
type LoopState struct {
	// ID 是 loop 的唯一标识。
	ID string

	// Round 是当前轮次（从 1 开始）。
	Round int

	// MaxIterations 是硬上限。
	MaxIterations int

	// MaxConsecutiveUnknown 是连续 UNKNOWN 状态的软上限。
	MaxConsecutiveUnknown int

	// ConsecutiveUnknown 是当前连续 UNKNOWN 计数。
	ConsecutiveUnknown int

	// LastFeedback 是给下一轮 coder 的反馈，等价老 dispatch 的 reviewerFeedback。
	LastFeedback string

	// Judge 是 loop 的价值评估员配置。
	Judge *schema.LoopJudge
}

// String 返回可读的 loop 状态描述。
func (s *LoopState) String() string {
	return fmt.Sprintf("loop[%s] round=%d/%d unknown=%d/%d",
		s.ID, s.Round, s.MaxIterations, s.ConsecutiveUnknown, s.MaxConsecutiveUnknown)
}
