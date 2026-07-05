// Package schema 的类型定义文件。
//
// 本文件定义与 schemas/v1/*.json 一一对应的 Go 数据结构。
// 设计原则:
// 1. 结构体字段顺序与 schema properties 顺序一致，便于对照
// 2. yaml tag 与 schema 字段名严格一致
// 3. 判别联合（PipelineStep/Condition）用自定义 UnmarshalYAML 处理
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-05
// 更新时间: 2026-07-05
package schema

import "gopkg.in/yaml.v3"

// Workflow 是顶层配置结构，对应 workflow.json。
type Workflow struct {
	Version     string         `yaml:"version"`
	Name        string         `yaml:"name"`
	Description string         `yaml:"description,omitempty"`
	Tools       []Tool         `yaml:"tools"`
	Pipeline    []PipelineStep `yaml:"pipeline"`
	Defaults    *Defaults      `yaml:"defaults,omitempty"`
}

// Defaults 是全局默认值。
type Defaults struct {
	Timeout   string `yaml:"timeout,omitempty"`
	OnBlocked string `yaml:"on_blocked,omitempty"`
}

// Tool 定义一个可调度的工具，对应 tool.json。
//
// 关键设计:
// - Name 是内部标识，pipeline 中引用此名
// - DisplayName 是类人展示名，用于评论和日志
// - Aliases 是 @mention 别名列表，Issue 中 @ 任一别名都可触发本工具
type Tool struct {
	Name           string            `yaml:"name"`
	DisplayName    string            `yaml:"display_name,omitempty"`
	Aliases        []string          `yaml:"aliases,omitempty"`
	Type           string            `yaml:"type"` // agent | command | function
	Skill          string            `yaml:"skill,omitempty"`
	PromptTemplate string            `yaml:"prompt_template,omitempty"`
	Run            string            `yaml:"run,omitempty"`
	Function       string            `yaml:"function,omitempty"`
	Timeout        string            `yaml:"timeout,omitempty"`
	Env            map[string]string `yaml:"env,omitempty"`
	Description    string            `yaml:"description,omitempty"`
}

// Stage 定义流水线中的一个执行阶段，对应 stage.json。
type Stage struct {
	Name      string         `yaml:"name"`
	Tool      string         `yaml:"tool"`
	InputFrom string         `yaml:"input_from,omitempty"`
	Output    string         `yaml:"output,omitempty"`
	When      *Condition     `yaml:"when,omitempty"`
	OnBlocked string         `yaml:"on_blocked,omitempty"`
	Args      map[string]any `yaml:"args,omitempty"`
}

// Condition 是条件表达式，对应 condition.json 的 oneOf。
//
// 两种形态:
// - Expr: 字符串表达式，如 "round >= 2"
// - Structured: 结构化条件，引用某 stage 的输出字段
type Condition struct {
	Expr       string
	Structured *ExitCondition
}

// UnmarshalYAML 让 Condition 支持字符串或对象两种 YAML 形态。
func (c *Condition) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		c.Expr = value.Value
		return nil
	case yaml.MappingNode:
		var ec ExitCondition
		if err := value.Decode(&ec); err != nil {
			return err
		}
		c.Structured = &ec
		return nil
	default:
		return errConditionKind
	}
}

// ExitCondition 是结构化退出条件，对应 exit_condition.json。
type ExitCondition struct {
	Stage    string `yaml:"stage"`
	Field    string `yaml:"field"`
	Operator string `yaml:"operator"`
	Value    any    `yaml:"value"`
}

// ResultContract 是结果契约，对应 result_contract.json。
type ResultContract struct {
	Schema      map[string]any `yaml:"schema"`
	StatusField string         `yaml:"status_field,omitempty"`
	StatusEnum  []string       `yaml:"status_enum,omitempty"`
}

// PipelineStep 是流水线步骤的判别联合，对应 pipeline_step.json。
//
// 三个字段互斥，由 Validate 保证恰好一个非空:
// - Stage: 串行单步
// - Parallel: 并行执行（至少 2 个子步骤）
// - Loop: 循环
//
// Join 仅在 Parallel 非 nil 时有效，表示汇合策略。
type PipelineStep struct {
	Stage    *Stage         `yaml:"stage,omitempty"`
	Parallel []PipelineStep `yaml:"parallel,omitempty"`
	Loop     *Loop          `yaml:"loop,omitempty"`
	Join     string         `yaml:"join,omitempty"`
}

// Loop 是循环编排原语，对应 loop.json。
type Loop struct {
	ID                    string          `yaml:"id"`
	MaxIterations         int             `yaml:"max_iterations,omitempty"`
	MaxConsecutiveUnknown int             `yaml:"max_consecutive_unknown,omitempty"`
	Timeout               string          `yaml:"timeout,omitempty"`
	ExitWhen              []ExitCondition `yaml:"exit_when,omitempty"`
	Body                  []PipelineStep  `yaml:"body"`
	Judge                 *LoopJudge      `yaml:"judge,omitempty"`
}

// LoopJudge 是价值评估员配置，对应 loop_judge.json。
type LoopJudge struct {
	Tool       string            `yaml:"tool"`
	StartRound int               `yaml:"start_round,omitempty"`
	OnDecision map[string]string `yaml:"on_decision,omitempty"`
}
