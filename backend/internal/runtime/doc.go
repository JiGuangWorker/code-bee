// Package runtime 提供 code-bee 的可配置调度引擎。
//
// 设计目标:
// 1. 基于 *schema.Workflow 配置驱动执行
// 2. 支持 stage（串行）/ parallel（并行）/ loop（循环）三种编排原语
// 3. 统一 Tool 抽象（agent 和 command 都是 Tool）
// 4. 复用现有文件契约和 agent.Runner
//
// 核心抽象:
// - Tool: 统一 agent/command/function 的执行接口
// - Executor: 编排原语执行器（Stage/Parallel/Loop）
// - Engine: 顶层入口，消费 *schema.Workflow
// - ExecutionContext: stage 间数据传递和 loop 状态维护
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-05
package runtime
