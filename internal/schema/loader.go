// Package schema 的配置加载器。
//
// 核心职责:
// 1. 从 YAML 字节流或文件加载 Workflow 配置
// 2. 先做 JSON Schema 校验（字段级），再做语义校验（跨字段引用）
// 3. 提供工具查找接口（按 name 或 alias），供 runtime 调度器使用
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-05
// 更新时间: 2026-07-05
package schema

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Loader 负责加载并校验 workflow 配置。
type Loader struct {
	validator *Validator
}

// NewLoader 创建一个加载器实例。
func NewLoader() (*Loader, error) {
	v, err := NewValidator()
	if err != nil {
		return nil, fmt.Errorf("schema.NewLoader: %w", err)
	}
	return &Loader{validator: v}, nil
}

// LoadFromBytes 从 YAML 字节流加载并校验一份 workflow 配置。
//
// 校验顺序:
// 1. JSON Schema 校验（字段类型、枚举、必填、条件必填）
// 2. YAML 反序列化到强类型结构体
// 3. 语义校验（工具引用完整性、别名唯一性、stage 引用合法性）
func (l *Loader) LoadFromBytes(data []byte) (*Workflow, error) {
	if err := l.validator.ValidateWorkflow(data); err != nil {
		return nil, fmt.Errorf("schema.LoadFromBytes: validate: %w", err)
	}

	var wf Workflow
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return nil, fmt.Errorf("schema.LoadFromBytes: unmarshal: %w", err)
	}

	if err := wf.Validate(); err != nil {
		return nil, fmt.Errorf("schema.LoadFromBytes: semantic: %w", err)
	}

	return &wf, nil
}

// LoadFromFile 从 YAML 文件加载并校验一份 workflow 配置。
func (l *Loader) LoadFromFile(path string) (*Workflow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("schema.LoadFromFile: read %s: %w", path, err)
	}
	return l.LoadFromBytes(data)
}

// Validate 做跨字段语义校验。
//
// 校验项:
// 1. tools 中 name 唯一
// 2. aliases 不与任何 name 或其他 alias 冲突
// 3. pipeline 中所有 stage.tool / judge.tool 引用都能在 tools 中找到
// 4. PipelineStep 的 stage/parallel/loop 互斥
// 5. join 仅在 parallel 模式下有效
// 6. input_from / exit_when 引用的 stage 在可见范围内存在
func (w *Workflow) Validate() error {
	registry, err := buildToolRegistry(w.Tools)
	if err != nil {
		return fmt.Errorf("build tool registry: %w", err)
	}

	if err := validatePipeline(w.Pipeline, registry, make(map[string]bool)); err != nil {
		return fmt.Errorf("validate pipeline: %w", err)
	}

	return nil
}

// FindTool 按 name 或 alias 查找工具，找不到返回 nil。
//
// runtime 调度器通过此方法解析 stage.tool 和 judge.tool 引用。
func (w *Workflow) FindTool(ref string) *Tool {
	for i := range w.Tools {
		t := &w.Tools[i]
		if t.Name == ref {
			return t
		}
		for _, a := range t.Aliases {
			if a == ref {
				return t
			}
		}
	}
	return nil
}

// toolRegistry 是 name→Tool 的索引，O(1) 查找。
type toolRegistry struct {
	byName   map[string]*Tool
	byAlias  map[string]*Tool
	allNames map[string]bool
}

// buildToolRegistry 构建 tool 索引并检测 name/alias 冲突。
func buildToolRegistry(tools []Tool) (*toolRegistry, error) {
	r := &toolRegistry{
		byName:   make(map[string]*Tool),
		byAlias:  make(map[string]*Tool),
		allNames: make(map[string]bool),
	}

	for i := range tools {
		t := &tools[i]

		if _, exists := r.byName[t.Name]; exists {
			return nil, fmt.Errorf("%w: duplicate tool name %q", errToolNameConflict, t.Name)
		}
		r.byName[t.Name] = t
		r.allNames[t.Name] = true

		for _, alias := range t.Aliases {
			// alias 与已有 tool name 冲突
			if _, exists := r.byName[alias]; exists {
				return nil, fmt.Errorf("%w: alias %q conflicts with a tool name", errToolNameConflict, alias)
			}
			// alias 与已有 alias 冲突
			if _, exists := r.byAlias[alias]; exists {
				return nil, fmt.Errorf("%w: duplicate alias %q", errToolNameConflict, alias)
			}
			r.byAlias[alias] = t
			r.allNames[alias] = true
		}
	}

	return r, nil
}

// resolve 解析一个 tool 引用，name 或 alias 均可。
func (r *toolRegistry) resolve(ref string) (*Tool, error) {
	if t, ok := r.byName[ref]; ok {
		return t, nil
	}
	if t, ok := r.byAlias[ref]; ok {
		return t, nil
	}
	return nil, fmt.Errorf("%w: %q", errUnknownToolRef, ref)
}

// validatePipeline 递归校验 pipeline 步骤序列。
//
// 输入参数:
//   - steps: 待校验的步骤列表
//   - registry: tool 索引
//   - scopeStages: 当前可见的 stage name 集合，会被本函数修改（加入本层 stage）
func validatePipeline(steps []PipelineStep, registry *toolRegistry, scopeStages map[string]bool) error {
	for i := range steps {
		if err := validateStep(&steps[i], registry, scopeStages); err != nil {
			return fmt.Errorf("step[%d]: %w", i, err)
		}
	}
	return nil
}

// validateStep 校验单个 PipelineStep。
func validateStep(step *PipelineStep, registry *toolRegistry, scopeStages map[string]bool) error {
	if err := checkStepMutualExclusive(step); err != nil {
		return err
	}

	switch {
	case step.Stage != nil:
		return validateStage(step.Stage, registry, scopeStages)
	case step.Parallel != nil:
		return validateParallel(step, registry, scopeStages)
	case step.Loop != nil:
		return validateLoop(step.Loop, registry, scopeStages)
	}
	return nil
}

// checkStepMutualExclusive 校验 stage/parallel/loop 互斥，以及 join 仅在 parallel 时有效。
func checkStepMutualExclusive(step *PipelineStep) error {
	n := 0
	if step.Stage != nil {
		n++
	}
	if step.Parallel != nil {
		n++
	}
	if step.Loop != nil {
		n++
	}
	if n != 1 {
		return fmt.Errorf("%w: got %d", errStepMutualExclusive, n)
	}
	if step.Join != "" && step.Parallel == nil {
		return errJoinWithoutParallel
	}
	return nil
}

// validateStage 校验单个 stage。
func validateStage(st *Stage, registry *toolRegistry, scopeStages map[string]bool) error {
	if _, err := registry.resolve(st.Tool); err != nil {
		return fmt.Errorf("stage %q: %w", st.Name, err)
	}

	if st.InputFrom != "" && !scopeStages[st.InputFrom] {
		return fmt.Errorf("%w: stage %q input_from %q not in scope", errUnknownStageRef, st.Name, st.InputFrom)
	}

	// 把本 stage 加入可见集合，供后续步骤引用
	scopeStages[st.Name] = true
	return nil
}

// validateParallel 校验并行块。
//
// 语义: parallel 内的 stage 互相不可见（并发执行），但都对外层后续可见。
func validateParallel(step *PipelineStep, registry *toolRegistry, scopeStages map[string]bool) error {
	// parallel 子步骤共享一个 innerScope，互相不可见
	innerScope := make(map[string]bool)
	for k, v := range scopeStages {
		innerScope[k] = v
	}

	for i := range step.Parallel {
		// 每个并行分支独立 copy，避免互相可见
		branchScope := make(map[string]bool)
		for k, v := range innerScope {
			branchScope[k] = v
		}
		if err := validateStep(&step.Parallel[i], registry, branchScope); err != nil {
			return fmt.Errorf("parallel[%d]: %w", i, err)
		}
		// 分支内的 stage 对外层可见
		for k, v := range branchScope {
			if v {
				scopeStages[k] = true
			}
		}
	}
	return nil
}

// validateLoop 校验循环块。
//
// 语义: loop body 内的 stage 互相可见，且对 loop 后续步骤可见。
func validateLoop(lp *Loop, registry *toolRegistry, scopeStages map[string]bool) error {
	// loop body 共享一个 innerScope，body 内 stage 互相可见
	innerScope := make(map[string]bool)
	for k, v := range scopeStages {
		innerScope[k] = v
	}

	if err := validatePipeline(lp.Body, registry, innerScope); err != nil {
		return fmt.Errorf("loop %q body: %w", lp.ID, err)
	}

	// 校验 exit_when 引用的 stage 在 body 内存在
	for i, ec := range lp.ExitWhen {
		if !innerScope[ec.Stage] {
			return fmt.Errorf("%w: loop %q exit_when[%d] stage %q not in body",
				errUnknownStageRef, lp.ID, i, ec.Stage)
		}
	}

	// 校验 judge.tool 引用
	if lp.Judge != nil {
		if _, err := registry.resolve(lp.Judge.Tool); err != nil {
			return fmt.Errorf("loop %q judge: %w", lp.ID, err)
		}
	}

	// loop body 内的 stage 对外层后续可见
	for k, v := range innerScope {
		if v {
			scopeStages[k] = true
		}
	}
	return nil
}
