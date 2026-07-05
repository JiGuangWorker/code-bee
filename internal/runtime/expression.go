// 本文件实现条件表达式求值，支持 when 和 exit_when 两种场景。
//
// 两种表达式形态:
// 1. 结构化 ExitCondition: stage + field + operator + value
//    支持 6 个 operator: equals/not_equals/in/contains/exists/regex
// 2. 字符串表达式: "round >= 2"、"review.status == \"PASS\""
//    支持比较运算符: ==, !=, >=, <=, >, <
//
// 设计说明:
// - 字符串表达式用最小解析器，后续可替换为 expr-lang/expr
// - 特殊变量 round 引用 CurrentLoop().Round
// - stage.field 形式通过 ExecutionContext.Lookup 取值
// - 数字比较统一转 float64，避免 int/float64 类型差异
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-05

package runtime

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/JiGuangWorker/code-bee/internal/schema"
)

// EvalExitCondition 求值单个结构化退出条件。
//
// 返回 true 表示条件满足（应退出 loop）。
func EvalExitCondition(ec *ExecutionContext, cond schema.ExitCondition) (bool, error) {
	val, ok := ec.Lookup(cond.Stage, cond.Field)

	switch cond.Operator {
	case "equals":
		if !ok {
			return false, nil
		}
		return tolerantEquals(val, cond.Value), nil

	case "not_equals":
		if !ok {
			return true, nil
		}
		return !tolerantEquals(val, cond.Value), nil

	case "in":
		if !ok {
			return false, nil
		}
		list, err := toAnySlice(cond.Value)
		if err != nil {
			return false, fmt.Errorf("operator in: value not a list: %w", err)
		}
		for _, item := range list {
			if tolerantEquals(val, item) {
				return true, nil
			}
		}
		return false, nil

	case "contains":
		if !ok {
			return false, nil
		}
		return containsValue(val, cond.Value), nil

	case "exists":
		return ok, nil

	case "regex":
		return evalRegexCondition(ok, val, cond.Value)
	}

	return false, fmt.Errorf("unknown operator: %s", cond.Operator)
}

// evalRegexCondition 处理 regex 操作符的匹配逻辑。
func evalRegexCondition(ok bool, val, patternVal any) (bool, error) {
	if !ok {
		return false, nil
	}
	s, err := toString(val)
	if err != nil {
		return false, fmt.Errorf("operator regex: value not a string: %w", err)
	}
	pattern, err := toString(patternVal)
	if err != nil {
		return false, fmt.Errorf("operator regex: pattern not a string: %w", err)
	}
	matched, err := regexp.MatchString(pattern, s)
	if err != nil {
		return false, fmt.Errorf("operator regex: invalid pattern %q: %w", pattern, err)
	}
	return matched, nil
}

// EvalCondition 求值 Condition（支持 Expr 和 Structured 两种形态）。
//
// cond 为 nil 时返回 true（无条件视为满足）。
func EvalCondition(ec *ExecutionContext, cond *schema.Condition) (bool, error) {
	if cond == nil {
		return true, nil
	}

	if cond.Expr != "" {
		return EvalExpr(cond.Expr, ec)
	}

	if cond.Structured != nil {
		return EvalExitCondition(ec, *cond.Structured)
	}

	return true, nil
}

// AnyExitConditionMatched 检查一组退出条件是否任一满足。
//
// 用于 loop 的 exit_when 检查：任一条件满足即退出。
func AnyExitConditionMatched(ec *ExecutionContext, conds []schema.ExitCondition) (bool, error) {
	for _, c := range conds {
		matched, err := EvalExitCondition(ec, c)
		if err != nil {
			return false, fmt.Errorf("exit condition [stage=%s field=%s]: %w", c.Stage, c.Field, err)
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
}

// 字符串表达式解析

var exprPattern = regexp.MustCompile(`^\s*(\w+(?:\.\w+)*)\s*(==|!=|>=|<=|>|<)\s*(.+?)\s*$`)

// EvalExpr 求值字符串表达式。
//
// 支持的格式:
//   - round >= 2
//   - review.status == "PASS"
//   - review.status != "BLOCKED"
//
// 特殊变量:
//   - round: 当前 loop 的轮次（非 loop 内为 0）
func EvalExpr(expr string, ec *ExecutionContext) (bool, error) {
	m := exprPattern.FindStringSubmatch(expr)
	if m == nil {
		return false, fmt.Errorf("invalid expression: %q", expr)
	}

	leftPath := m[1]
	op := m[2]
	rightStr := m[3]

	left, err := resolveExprValue(ec, leftPath)
	if err != nil {
		return false, fmt.Errorf("resolve left %q: %w", leftPath, err)
	}

	right, err := parseLiteral(rightStr)
	if err != nil {
		return false, fmt.Errorf("parse right %q: %w", rightStr, err)
	}

	return compareValues(left, op, right)
}

// resolveExprValue 解析表达式中的变量引用。
func resolveExprValue(ec *ExecutionContext, path string) (any, error) {
	if path == "round" {
		loop := ec.CurrentLoop()
		if loop == nil {
			return 0, nil
		}
		return loop.Round, nil
	}

	// stage.field 形式
	idx := strings.Index(path, ".")
	if idx > 0 {
		stageName := path[:idx]
		fieldPath := path[idx+1:]
		val, ok := ec.Lookup(stageName, fieldPath)
		if !ok {
			return nil, nil
		}
		return val, nil
	}

	return nil, fmt.Errorf("unknown variable: %s", path)
}

// parseLiteral 解析表达式中的字面量。
func parseLiteral(s string) (any, error) {
	s = strings.TrimSpace(s)

	// 字符串字面量
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1], nil
		}
	}

	// 数字
	if n, err := strconv.Atoi(s); err == nil {
		return n, nil
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f, nil
	}

	// 布尔
	if s == "true" {
		return true, nil
	}
	if s == "false" {
		return false, nil
	}

	// 否则当作字符串
	return s, nil
}

// compareValues 按运算符比较两个值。
func compareValues(left any, op string, right any) (bool, error) {
	switch op {
	case "==":
		return tolerantEquals(left, right), nil
	case "!=":
		return !tolerantEquals(left, right), nil
	case ">=", "<=", ">", "<":
		return compareNumeric(left, op, right)
	}
	return false, fmt.Errorf("unknown operator: %s", op)
}

// compareNumeric 比较两个数值。
func compareNumeric(left any, op string, right any) (bool, error) {
	l, err := toFloat(left)
	if err != nil {
		return false, fmt.Errorf("left not numeric: %v", left)
	}
	r, err := toFloat(right)
	if err != nil {
		return false, fmt.Errorf("right not numeric: %v", right)
	}

	switch op {
	case ">=":
		return l >= r, nil
	case "<=":
		return l <= r, nil
	case ">":
		return l > r, nil
	case "<":
		return l < r, nil
	}
	return false, fmt.Errorf("unknown operator: %s", op)
}

// tolerantEquals 宽松相等比较，处理数字类型差异。
//
// JSON 往返后数字是 float64，YAML 解析后可能是 int/int64/float64。
// 此函数把两边都尝试转 float64 比较，转不了再用 reflect.DeepEqual。
func tolerantEquals(a, b any) bool {
	// 尝试数字比较
	af, aErr := toFloat(a)
	bf, bErr := toFloat(b)
	if aErr == nil && bErr == nil {
		return af == bf
	}

	// 字符串比较
	as, aOk := toString(a)
	bs, bOk := toString(b)
	if aOk == nil && bOk == nil {
		return as == bs
	}

	// 兜底用 DeepEqual
	return reflect.DeepEqual(a, b)
}

// toFloat 把任意值转为 float64。
func toFloat(v any) (float64, error) {
	switch n := v.(type) {
	case int:
		return float64(n), nil
	case int8:
		return float64(n), nil
	case int16:
		return float64(n), nil
	case int32:
		return float64(n), nil
	case int64:
		return float64(n), nil
	case uint:
		return float64(n), nil
	case uint64:
		return float64(n), nil
	case float32:
		return float64(n), nil
	case float64:
		return n, nil
	}
	return 0, fmt.Errorf("not numeric: %T", v)
}

// toString 把任意值转为 string。
func toString(v any) (string, error) {
	switch s := v.(type) {
	case string:
		return s, nil
	case []byte:
		return string(s), nil
	}
	return "", fmt.Errorf("not string: %T", v)
}

// toAnySlice 把任意值转为 []any。
func toAnySlice(v any) ([]any, error) {
	if v == nil {
		return nil, fmt.Errorf("nil")
	}
	// YAML/JSON 解析后的数组都是 []any
	if s, ok := v.([]any); ok {
		return s, nil
	}
	// 反射处理其他 slice 类型
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Slice {
		out := make([]any, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			out[i] = rv.Index(i).Interface()
		}
		return out, nil
	}
	return nil, fmt.Errorf("not a slice: %T", v)
}

// containsValue 检查 val 是否包含 target。
//
// - val 是 string: strings.Contains
// - val 是 []any: 遍历用 tolerantEquals
func containsValue(val, target any) bool {
	if s, err := toString(val); err == nil {
		if t, err := toString(target); err == nil {
			return strings.Contains(s, t)
		}
	}

	if list, err := toAnySlice(val); err == nil {
		for _, item := range list {
			if tolerantEquals(item, target) {
				return true
			}
		}
	}

	return false
}
