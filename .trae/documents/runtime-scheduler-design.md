# Runtime 调度器设计与实现计划

## Context

### 问题
code-bee 当前的调度策略硬编码在 `internal/pipeline/dispatch.go` 中：
- 角色固定（`BuiltinAgents()` 写死 6 个角色）
- 流程固定（`runCodingReviewLoop` 写死 coding→review→judge 的 3 轮循环）
- Loop 次数固定（`maxCodingReviewRounds=3`、`maxConsecutiveUnknownReview=2`）
- Prompt 模板固定（`prompts.go` 5 个 builder 函数，参数位置敏感）

这导致非标准场景（无审查流、需要安全审查、不同角色分工等）无法适配。Issue #11 提出应让调度策略由场景驱动、外部加载。

### 已完成
- `internal/schema/`：三层 YAML Schema（tool/stage/pipeline_step/loop/loop_judge/workflow）+ 配置加载器 + 语义校验
- 工具别名支持（name + display_name + aliases）
- 三种编排原语（stage/parallel/loop）的 Schema 定义

### 本次目标
实现 Runtime 调度器，基于 `*schema.Workflow` 配置驱动执行：
1. 新建 `internal/runtime/` 包，实现通用执行引擎
2. **直接替换** `dispatch.go` 的硬编码逻辑（不共存）
3. **同步外部化** prompt 模板（从 `prompts.go` 抽到模板文件）
4. 复用现有文件契约（`.code-bee/runs/...`）和 `agent.Runner`

## 设计决策

| 决策点 | 选择 | 理由 |
|--------|------|------|
| 与 dispatch.go 关系 | 直接替换 | 彻底统一，避免两套代码维护 |
| Prompt 模板 | 同步外部化 | 一步到位，避免临时方案 |
| 文件契约 | 复用现有 | agent 端零改动 |
| Tool 抽象 | 统一 Agent/Command/Function | 智能体本质是工具 |
| 状态收敛 | StepResult.Status 单字段 | 替代散落的 BlockedStatus/Passed/Unknown |

## 包结构

```
internal/runtime/
├── doc.go                  # 包注释
├── tool.go                 # Tool 接口 + StepResult + Invocation
├── tool_registry.go        # ToolRegistry: 由 *schema.Workflow 构建
├── agent_tool.go           # AgentTool: 包装 agent.Runner
├── command_tool.go         # CommandTool: exec.CommandContext
├── context.go              # ExecutionContext + LoopState
├── expression.go           # when / exit_when 表达式求值
├── executor.go             # Executor 接口
├── stage_executor.go       # StageExecutor
├── parallel_executor.go    # ParallelExecutor (errgroup)
├── loop_executor.go        # LoopExecutor
├── engine.go               # Engine: 顶层入口
├── prompt_builder.go       # PromptBuilder 接口 + 模板加载
└── *_test.go               # 单元测试

internal/pipeline/
├── contracts.go            # 保留（Result 结构 + 状态枚举）
├── artifacts.go            # 保留（ArtifactSet）
├── dispatch.go             # 重构：Service.Dispatch 调用 runtime.Engine
├── prompts.go              # 删除（模板移到 prompts/ 目录）
└── history.go              # 保留（LoopHistory）

prompts/                    # 新增：外部化 prompt 模板
├── coding.md
├── review.md
├── issue-handling.md
├── issue-post.md
└── loop-judge.md
```

## 核心接口

### Tool 接口

```go
type Tool interface {
    Definition() *schema.Tool
    Execute(ctx context.Context, inv Invocation) (*StepResult, error)
}

type StepResult struct {
    StageName string
    Success   bool
    Blocked   bool
    Output    string
    Data      map[string]any  // 从 result file 加载的结构化结果
    Status    string          // = Data["status"]，统一状态字段
    Skipped   bool            // when=false 时
}
```

### Executor 接口

```go
type Executor interface {
    Execute(ctx context.Context, step schema.PipelineStep, ec *ExecutionContext) (*StepResult, error)
}
```

### Engine 顶层

```go
func New(wf *schema.Workflow, deps Dependencies) (*Engine, error)

func (e *Engine) Run(ctx context.Context) (*pipeline.Result, error)
```

## 实现步骤

### 阶段 1：Prompt 模板外部化

**目标**：把 `prompts.go` 的 5 个 builder 抽到 `prompts/` 目录的模板文件

**改动**：
1. 创建 `prompts/` 目录，写 5 个 `.md` 模板文件（用 Go `text/template` 语法）
2. `internal/runtime/prompt_builder.go`：实现 PromptBuilder，从文件加载模板渲染
3. 删除 `internal/pipeline/prompts.go`
4. 调整 `dispatch.go` 中对 `buildXxxPrompt` 的调用（临时保留，阶段 5 会删）

**关键**：模板文件用 `{{.WorkerID}}`、`{{.IssueURL}}`、`{{.Round}}` 等命名参数，替代位置敏感的 `%s/%d`

### 阶段 2：Runtime 核心数据结构（P1）

**文件**：`tool.go`、`tool_registry.go`、`context.go`

**内容**：
- `Tool` 接口、`StepResult`、`Invocation`、`PlatformContext`
- `ToolRegistry`：由 `*schema.Workflow` 构建，支持 name + alias 查找（复用 `Workflow.FindTool`）
- `ExecutionContext`：`stages map[string]*StepResult` + `loopStack []*LoopState`
- `LoopState`：`Round`、`MaxIterations`、`ConsecutiveUnknown`、`LastFeedback`
- `Lookup(stageName, fieldPath)` 支持 dot path 嵌套取值
- `Snapshot()` / `Merge()` 用于 parallel 写隔离

### 阶段 3：表达式求值（P2）

**文件**：`expression.go`

**内容**：
- 结构化 `ExitCondition` 求值：6 个 operator（equals/not_equals/in/contains/exists/regex）
- 字符串表达式最小解析器：支持 `round >= 2`、`review.status == "PASS"` 等
- 复用 `ExecutionContext.Lookup` 取值

### 阶段 4：三种 Tool 实现（P3）

**文件**：`agent_tool.go`、`command_tool.go`、`prompt_builder.go`

**内容**：
- `AgentTool`：包装 `agent.Runner`，调 `runner.Run(ctx, kind, prompt)` → 加载 result file → `StepResult`
- `CommandTool`：`exec.CommandContext("sh", "-c", run)` → 解析 stdout JSON → `StepResult`
- `PromptBuilder`：从 `prompts/` 加载模板，用 `Invocation` 渲染
- `mapTemplateToKind`：`coding→TaskKindCoding` 等映射表

### 阶段 5：StageExecutor（P4）

**文件**：`stage_executor.go`

**流程**：
1. `registry.Resolve(stage.Tool)` → Tool
2. 求 `stage.When`，false 则返回 `Skipped`
3. `ec.Lookup(stage.InputFrom, "")` 拿上游数据
4. `tool.Execute(ctx, inv)` → `StepResult`
5. `ec.SetResult(stage.Name, result)`
6. 处理 `on_blocked`：stop/skip/continue

### 阶段 6：ParallelExecutor（P5）

**文件**：`parallel_executor.go`

**流程**：
1. `errgroup.WithContext(ctx)` 并发执行分支
2. 每分支用 `ec.CloneForBranch()` 写隔离
3. Join 策略：
   - `all`：全成功才成功
   - `any`：任一成功即成功
   - `first_success`：首个成功后 `context.WithCancel` 取消其他
4. 成功分支结果 `Merge` 回主 ec

### 阶段 7：LoopExecutor（P6）

**文件**：`loop_executor.go`

**流程**：
```
for round := 1; round <= maxIterations; round++ {
    执行 body 中每个 step
    每步后检查 exit_when（命中则退出）
    维护 ConsecutiveUnknown（超限则 ManualRequired 退出）
    若配了 judge 且 round >= startRound：执行 judge
        STOP_MANUAL → ManualRequired 退出
        STOP_BLOCKED → Blocked 退出
        SHRINK_TASK → 注入反馈继续
        CONTINUE → 继续
}
跑完 maxIterations → ManualRequired
```

### 阶段 8：Engine + 重构 dispatch.go（P7）

**文件**：`engine.go`、`dispatch.go`（重构）

**改动**：
1. `runtime.Engine.Run(ctx)`：遍历 `wf.Pipeline`，`pickExecutor` 分发执行
2. 重构 `pipeline.Service`：
   - `Dispatch(ctx, cfg)` 改为调用 `runtime.Engine`
   - 删除 `runCodingReviewLoop` / `runIssueHandling` / `runCodingRound` 等硬编码函数
   - 保留 `contracts.go`（Result 结构）、`artifacts.go`（ArtifactSet）、`history.go`（LoopHistory）
3. `cmd/worker/main.go`：加载 workflow.yaml → 构造 Engine → 调用 Service
4. 调整现有测试：`dispatch_test.go`、`loop_judge_test.go`、`coverage_test.go` 改为基于 workflow 配置测试

## 测试策略

### 单元测试（runtime 包）
- `tool_test.go`：AgentTool/CommandTool，用 scriptedRunner + fakeExec mock
- `context_test.go`：Lookup 嵌套字段、Snapshot/Merge 隔离、LoopStack
- `expression_test.go`：6 operator + 字符串表达式全覆盖
- `stage_executor_test.go`：when 跳过、on_blocked 三策略、input_from 注入
- `parallel_executor_test.go`：all/any/first_success + cancel 行为
- `loop_executor_test.go`：exit_when 命中、judge 四种决策、max_consecutive_unknown、max_iterations、嵌套 loop

### 集成测试
- 用 `schema/testdata/valid/loop_with_judge.yaml` 端到端跑通
- 用 `parallel_pipeline.yaml`、`nested_loop.yaml` 验证编排原语

### 重构后的 pipeline 测试
- `dispatch_test.go` 改为：加载 workflow.yaml → Engine.Run → 断言 Result
- 保持原有测试覆盖意图（blocked/manual/completion 等场景）

## 验证方法

1. **单元测试**：`go test ./internal/runtime/... -v -cover`，覆盖率 >= 80%
2. **集成测试**：`go test ./internal/pipeline/... -v`，原有场景全部通过
3. **全项目测试**：`go test ./...` 全绿
4. **构建验证**：`go build ./...` 无错误
5. **Lint**：`go vet ./...` 无警告

## 关键文件

| 文件 | 操作 | 说明 |
|------|------|------|
| `internal/runtime/*.go` | 新增 | 完整调度引擎 |
| `internal/pipeline/dispatch.go` | 重构 | 删除硬编码 loop，改调 runtime.Engine |
| `internal/pipeline/prompts.go` | 删除 | 模板移到 `prompts/` 目录 |
| `internal/pipeline/contracts.go` | 保留 | Result 结构 + 状态枚举 |
| `internal/pipeline/artifacts.go` | 保留 | ArtifactSet 复用 |
| `prompts/*.md` | 新增 | 5 个外部化 prompt 模板 |
| `cmd/worker/main.go` | 修改 | 加载 workflow.yaml |
| `internal/pipeline/dispatch_test.go` | 调整 | 改为基于 workflow 配置测试 |

## 风险与缓解

| 风险 | 缓解 |
|------|------|
| 重构破坏现有测试 | 分阶段提交，每阶段测试通过再继续 |
| Prompt 模板外部化改变行为 | 保持模板内容与原 prompts.go 等价，逐字对照 |
| Loop 行为不一致 | 保留 LoopHistory 持久化逻辑，状态迁移完整 |
| 文件契约破坏 | 复用 ArtifactSet，agent 端零改动 |
