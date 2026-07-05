// Package schema 提供 code-bee workflow 配置的 JSON Schema 加载与校验。
//
// 设计目标:
// 1. 把 schemas/v1/*.yaml 通过 go:embed 打包进二进制，避免外部文件依赖
// 2. 用 santhosh-tekuri/jsonschema/v5 做完整 draft-07 校验
// 3. 对外暴露最小接口：NewValidator + ValidateWorkflow
//
// Schema 文件格式说明:
//   - schema 定义文件用 YAML 格式（与用户配置统一），加载时转 JSON 喂给校验库
//   - $id 和 $ref 中的 URL 保持 .json 后缀，作为 URI 标识符（JSON Schema 标准约定）
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

//go:embed schemas/v1/*.yaml
var schemaFS embed.FS

const (
	// workflowSchemaPath 是顶层 schema 在 embed FS 中的路径。
	workflowSchemaPath = "schemas/v1/workflow.yaml"

	// schemaIDBase 是所有 v1 schema 的 $id 前缀。
	// 注意：$id URL 保持 .json 后缀作为 URI 标识符（JSON Schema 标准约定），
	// 实际文件用 .yaml 格式，加载时转 JSON 注册到 compiler。
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
// - 遍历 embed FS 中的 schemas/v1/*.yaml，逐个转 JSON 后注册到 compiler
// - 编译 workflow schema 作为顶层入口
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
// - schema 文件用 YAML 格式存储，加载时转 JSON 喂给 santhosh-tekuri/jsonschema
// - $id 和 $ref URL 保持 .json 后缀，作为 URI 标识符（与 JSON Schema 标准约定一致）
// - 必须先注册全部资源再 Compile，否则跨文件 $ref 会失败
func loadEmbeddedSchemas(c *jsonschema.Compiler) error {
	entries, err := fs.ReadDir(schemaFS, "schemas/v1")
	if err != nil {
		return fmt.Errorf("read schema dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		data, readErr := fs.ReadFile(schemaFS, "schemas/v1/"+entry.Name())
		if readErr != nil {
			return fmt.Errorf("read schema %s: %w", entry.Name(), readErr)
		}

		// YAML → JSON 转换：santhosh-tekuri/jsonschema 只接受 JSON 输入
		var doc any
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return fmt.Errorf("yaml unmarshal schema %s: %w", entry.Name(), err)
		}

		jsonBytes, err := json.Marshal(doc)
		if err != nil {
			return fmt.Errorf("json marshal schema %s: %w", entry.Name(), err)
		}

		// URL 用 .json 后缀（与 schema 文件内 $id 和 $ref 中的引用一致）
		baseName := strings.TrimSuffix(entry.Name(), ".yaml")
		url := schemaIDBase + baseName + ".json"
		c.AddResource(url, bytes.NewReader(jsonBytes))
	}

	return nil
}

// ValidateWorkflow 校验一份 YAML 格式的 workflow 配置。
//
// 设计说明:
// - 用户配置文件统一用 YAML（可读性好、支持注释、与 workflow 生态一致）
// - schema 定义文件也用 YAML，加载时统一转 JSON 喂给校验库
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
