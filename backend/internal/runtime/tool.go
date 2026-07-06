// 本文件定义 Runtime 引擎的核心抽象：Tool 接口、StepResult、Invocation。
//
// 设计原则:
// 1. runtime 包不依赖 pipeline 包（避免循环依赖）
// 2. Tool 统一 agent/command/function 三种工具类型
// 3. StepResult.Status 是唯一状态字段，收敛现有散落的 BlockedStatus/Passed/Unknown
// 4. 文件契约通过 ArtifactResolver 接口抽象，由 pipeline 层注入实现
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-05

package runtime

import (
	"context"

	"github.com/JiGuangWorker/code-bee/internal/agent"
	"github.com/JiGuangWorker/code-bee/internal/config"
	"github.com/JiGuangWorker/code-bee/internal/schema"
)

// Runner 是 agent.Runner 的抽象接口。
//
// runtime 包通过此接口调用智能体，避免直接依赖 agent 包的具体实现。
// agent.Runner struct 自动满足此接口（duck typing）。
type Runner interface {
	Run(ctx context.Context, kind agent.TaskKind, task string) (*agent.RunResult, error)
}

// Tool 统一 agent/command/function 三种工具类型的执行接口。
//
// 实现类:
// - AgentTool: 包装 Runner，调用 reasonix 执行 AI 智能体
// - CommandTool: 通过 exec.CommandContext 执行 shell 命令
// - FunctionTool: 调用内置注册函数
type Tool interface {
	// Definition 返回工具的 schema 定义（name/aliases/skill 等）。
	Definition() *schema.Tool

	// Execute 执行工具并返回结构化结果。
	Execute(ctx context.Context, inv Invocation) (*StepResult, error)
}

// Invocation 封装工具执行时所需的全部运行时上下文。
type Invocation struct {
	// StageName 是当前 stage 的名称，用于结果存储和日志。
	StageName string

	// Tool 是工具的 schema 定义，供 PromptBuilder 渲染时引用。
	Tool *schema.Tool

	// InputFrom 是 input_from stage 的结构化数据，可为 nil。
	InputFrom map[string]any

	// Args 是 stage 配置中的参数，如 issue-post 的 purpose。
	Args map[string]any

	// ResultFilePath 是 agent 端契约文件路径，agent 把 JSON 结果写入此文件。
	ResultFilePath string

	// Loop 是当前 loop 状态，非 loop 内执行时为 nil。
	Loop *LoopState

	// Platform 是平台上下文（worker_id/issue_url 等）。
	Platform PlatformContext

	// Config 是 code-bee 配置，供 prompt 渲染引用。
	Config *config.Config

	// EC 是执行上下文引用，供 Tool 访问 Artifacts/Lookup 等能力。
	EC *ExecutionContext
}

// PlatformContext 封装平台相关的上下文信息。
type PlatformContext struct {
	WorkerID      string
	IssueURL      string
	PlatformName  string
	PlatformGuide string
}

// StepResult 是单个 stage 执行后的结构化结果。
//
// 关键设计:
// - Status 是唯一状态字段，从 result file 的 "status" 字段提取
// - Data 是完整的结果 map，供 exit_when/when 条件求值时按 dot path 取值
// - Blocked/ManualRequired/Completed 由 Executor 层根据 Status 和 loop 语义设置
type StepResult struct {
	// StageName 是对应的 stage 名称。
	StageName string

	// Success 表示工具调用本身是否成功（无系统级错误）。
	Success bool

	// Blocked 表示业务流程被阻塞（status=BLOCKED）。
	Blocked bool

	// ManualRequired 表示需要人工接管（loop 超限或 judge STOP_MANUAL）。
	ManualRequired bool

	// Completed 表示流程已完成（如 review PASS 或 issue-post POSTED）。
	Completed bool

	// Output 是工具的原始 stdout 输出。
	Output string

	// Data 是从结果文件加载的结构化数据。
	Data map[string]any

	// Status 是 Data["status"] 的快捷引用，统一状态判断。
	Status string

	// Skipped 表示因 when 条件不满足而跳过。
	Skipped bool
}

// EngineResult 是 Runtime 引擎的最终执行结果。
//
// 与 pipeline.Result 结构对齐，由 pipeline 层做类型转换。
type EngineResult struct {
	// Success 表示整个流程顺利完成，没有系统级错误。
	Success bool

	// Completed 表示 Issue 已被判定通过且最终回复已提交。
	Completed bool

	// Blocked 表示流程因外部条件不足而暂停。
	Blocked bool

	// ManualRequired 表示自动循环触达兜底阈值，需要人工接管。
	ManualRequired bool

	// Output 是最终输出文本。
	Output string
}

// ArtifactResolver 抽象文件契约的路径计算和结果加载。
//
// 由 pipeline 层实现并注入，runtime 包不直接依赖 pipeline.ArtifactSet。
type ArtifactResolver interface {
	// ResolveResultFile 返回指定 stage 的结果文件路径。
	// loopRound 为 0 表示非 loop 内的 stage。
	ResolveResultFile(stageName string, args map[string]any, loopRound int) string

	// ResetResultFile 删除旧的结果文件，防止读到上一轮残留。
	ResetResultFile(path string) error

	// LoadResult 从结果文件加载结构化数据。
	// stageName 用于路由到对应的 Result 结构（如 coding → CodingResult）。
	LoadResult(stageName string, path string) (map[string]any, error)

	// LoopHistoryPath 返回 loop 历史文件路径。
	LoopHistoryPath() string
}
