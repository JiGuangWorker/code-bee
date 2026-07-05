// Package schema 提供 code-bee workflow 配置的 JSON Schema 加载与校验。
//
// 设计目标:
// 1. 把 schemas/v1/*.json 通过 go:embed 打包进二进制，避免外部文件依赖
// 2. 用 santhosh-tekuri/jsonschema/v5 做完整 draft-07 校验
// 3. 对外暴露最小接口：NewValidator + ValidateWorkflow
//
// 开发维护: AI Assistant
// 创建时间: 2026-07-05
// 更新时间: 2026-07-05
package schema

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v5"
	"gopkg.in/yaml.v3"
)

//go:embed schemas/v1/*.json
var schemaFS embed.FS

const (
	// workflowSchemaPath 是顶层 schema 在 embed FS 中的路径。
	workflowSchemaPath = "schemas/v1/workflow.json"

	// schemaIDBase 是所有 v1 schema 的 $id 前缀。
	schemaIDBase = "https://codebee.dev/schemas/v1/"
)

// Validator 负责加载所有 v1 schema 并对外提供校验入口。
type Validator struct {
	// compiler 是已注册全部 v1 schema 资源的编译器实例。
	compiler *jsonschema.Compiler

	// workflow 是编译后的顶层 workflow schema。
	workflow *jsonschema.Schema
}

// NewValidator 创建并初始化一个 schema 校验器。
//
// 核心逻辑:
// - 遍历 embed FS 中的 schemas/v1/*.json，逐个注册到 compiler
// - 编译 workflow.json 作为顶层入口
func NewValidator() (*Validator, error) {
	c := jsonschema.NewCompiler()

	if err := loadEmbeddedSchemas(c); err != nil {
		return nil, fmt.Errorf("schema.NewValidator: load embedded schemas: %w", err)
	}

	workflow, err := c.Compile(schemaIDBase + "workflow.json")
	if err != nil {
		return nil, fmt.Errorf("schema.NewValidator: compile workflow: %w", err)
	}

	return &Validator{compiler: c, workflow: workflow}, nil
}

// loadEmbeddedSchemas 把 embed FS 中所有 v1 schema 文件注册到 compiler。
//
// 关键点:
// - $id 是文件名（如 tool.json），$ref 通过 https://codebee.dev/schemas/v1/<name>.json 引用
// - 必须先注册全部资源再 Compile，否则跨文件 $ref 会失败
func loadEmbeddedSchemas(c *jsonschema.Compiler) error {
	entries, err := fs.ReadDir(schemaFS, "schemas/v1")
	if err != nil {
		return fmt.Errorf("read schema dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, readErr := fs.ReadFile(schemaFS, "schemas/v1/"+entry.Name())
		if readErr != nil {
			return fmt.Errorf("read schema %s: %w", entry.Name(), readErr)
		}

		url := schemaIDBase + entry.Name()
		c.AddResource(url, bytes.NewReader(data))
	}

	return nil
}

// ValidateWorkflow 校验一份 YAML 格式的 workflow 配置。
//
// 设计说明:
// - 用户配置文件统一用 YAML（可读性好、支持注释、与 workflow 生态一致）
// - 不接受 JSON 格式的用户配置；JSON 仅用于 schema 定义文件本身
//
// 输入参数:
//   - data: YAML 字节流
//
// 返回值:
//   - error: 校验失败时返回 *ValidationError，包含全部字段级错误
func (v *Validator) ValidateWorkflow(data []byte) error {
	doc, err := decodeConfig(data)
	if err != nil {
		return fmt.Errorf("schema.ValidateWorkflow: decode: %w", err)
	}

	if err := v.workflow.Validate(doc); err != nil {
		return newValidationError(err)
	}

	return nil
}

// ValidateTool 单独校验一份 tool 配置，便于工具注册场景增量校验。
func (v *Validator) ValidateTool(data []byte) error {
	doc, err := decodeConfig(data)
	if err != nil {
		return fmt.Errorf("schema.ValidateTool: decode: %w", err)
	}

	sch := v.compiler.MustCompile(schemaIDBase + "tool.json")
	if err := sch.Validate(doc); err != nil {
		return newValidationError(err)
	}

	return nil
}

// decodeConfig 把 YAML 字节流解码为 interface{}。
//
// 设计说明:
// - 用户配置文件统一用 YAML（可读性好、支持注释和多行字符串、与 K8s/GitHub Actions 生态一致）
// - JSON Schema 定义文件本身仍是 JSON，但那是 schema 标准格式，与用户配置无关
// - YAML 解出的数值类型与 jsonschema 期望不一致，需通过 JSON 往返统一类型
func decodeConfig(data []byte) (any, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, errors.New("empty config data")
	}

	var doc any
	if err := yaml.Unmarshal(trimmed, &doc); err != nil {
		return nil, fmt.Errorf("yaml unmarshal: %w", err)
	}

	// YAML 解出的 map 类型是 map[string]interface{}，但数值类型可能为 int64，
	// jsonschema 期望 float64 或 json.Number。通过 JSON 往返统一类型。
	jsonBytes, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("yaml normalize marshal: %w", err)
	}

	var unified any
	if err := json.Unmarshal(jsonBytes, &unified); err != nil {
		return nil, fmt.Errorf("yaml normalize unmarshal: %w", err)
	}

	return unified, nil
}

// ValidationError 描述一份配置的全部校验错误。
type ValidationError struct {
	// Errors 是字段级错误列表。
	Errors []FieldError
}

// FieldError 描述单个字段的校验错误。
type FieldError struct {
	// Location 是错误字段的 JSON Pointer 路径，如 /tools/0/name。
	Location string

	// Message 是错误描述。
	Message string
}

// Error 实现 error 接口。
func (e *ValidationError) Error() string {
	if len(e.Errors) == 0 {
		return "validation failed"
	}

	var b strings.Builder
	b.WriteString("validation failed:")
	for _, fe := range e.Errors {
		b.WriteString("\n  at ")
		if fe.Location == "" {
			b.WriteString("(root)")
		} else {
			b.WriteString(fe.Location)
		}
		b.WriteString(": ")
		b.WriteString(fe.Message)
	}
	return b.String()
}

// newValidationError 把 jsonschema 库返回的错误转换成 ValidationError。
func newValidationError(err error) *ValidationError {
	ve := &ValidationError{}

	var valErr *jsonschema.ValidationError
	if errors.As(err, &valErr) {
		collectValidationError(valErr, &ve.Errors)
		return ve
	}

	ve.Errors = append(ve.Errors, FieldError{
		Location: "",
		Message:  err.Error(),
	})
	return ve
}

// collectValidationError 递归收集所有叶子错误。
func collectValidationError(err *jsonschema.ValidationError, out *[]FieldError) {
	if len(err.Causes) == 0 {
		*out = append(*out, FieldError{
			Location: err.InstanceLocation,
			Message:  err.Message,
		})
		return
	}

	for _, cause := range err.Causes {
		collectValidationError(cause, out)
	}
}
