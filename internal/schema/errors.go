// Package schema 的错误定义。
package schema

import "errors"

// errConditionKind 表示 Condition 的 YAML 形态既不是字符串也不是对象。
var errConditionKind = errors.New("condition must be a string or a mapping")

// 语义校验错误。用错误类型而非固定字符串，便于调用方判断。
var (
	// errToolNameConflict 表示 tools 中 name 或 aliases 出现冲突。
	errToolNameConflict = errors.New("tool name or alias conflict")

	// errUnknownToolRef 表示 stage 或 judge 引用了未声明的 tool。
	errUnknownToolRef = errors.New("unknown tool reference")

	// errStepMutualExclusive 表示 PipelineStep 的 stage/parallel/loop 不满足互斥。
	errStepMutualExclusive = errors.New("pipeline step must have exactly one of stage/parallel/loop")

	// errJoinWithoutParallel 表示 join 字段在没有 parallel 时出现。
	errJoinWithoutParallel = errors.New("join only valid with parallel")

	// errUnknownStageRef 表示 input_from 或 exit_when 引用了不存在的 stage。
	errUnknownStageRef = errors.New("unknown stage reference")
)
