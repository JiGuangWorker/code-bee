package schema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestValidator_ValidateWorkflow_Valid 全部合法 workflow 配置应通过校验。
//
// 覆盖场景:
// - 最小化配置（仅一个 tool + 一个 stage）
// - 工具别名（中文类人名称）
// - 三种 tool type（agent / command / function）
// - parallel 编排
// - loop + judge 编排
// - 嵌套 loop
// - JSON 与 YAML 两种格式
func TestValidator_ValidateWorkflow_Valid(t *testing.T) {
	cases := loadCases(t, "testdata/valid")

	v := newValidator(t)

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			data, err := os.ReadFile(tc.path)
			if err != nil {
				t.Fatalf("read testdata %s: %v", tc.path, err)
			}

			// Act
			err = v.ValidateWorkflow(data)

			// Assert
			if err != nil {
				t.Fatalf("expected valid, got error: %v", err)
			}
		})
	}
}

// TestValidator_ValidateWorkflow_Invalid 全部非法 workflow 配置应被拒绝。
//
// 覆盖场景:
// - 缺少必填字段
// - agent 缺 skill
// - command 缺 run
// - 工具名非法
// - parallel 子步骤不足
// - loop 超限 / 缺 body
// - exit_when operator 非法
// - version 非法
// - 未知 pipeline step
// - aliases 重复
// - 空配置
func TestValidator_ValidateWorkflow_Invalid(t *testing.T) {
	cases := loadCases(t, "testdata/invalid")

	v := newValidator(t)

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			data, err := os.ReadFile(tc.path)
			if err != nil {
				t.Fatalf("read testdata %s: %v", tc.path, err)
			}

			// Act
			err = v.ValidateWorkflow(data)

			// Assert
			// 配置非法包含两类：
			// 1. decode 阶段失败（空配置 / YAML 语法错）→ 返回普通 wrapError
			// 2. schema 校验失败（字段缺失 / 枚举非法）→ 返回 *ValidationError
			// 两者都算"配置非法"，本测试只要求 err != nil
			if err == nil {
				t.Fatalf("expected validation error, got nil")
			}
		})
	}
}

// TestValidator_ValidateWorkflow_ToolAlias 验证工具别名功能：
// 含中文类人名称的 aliases 字段应被接受，且能正确解析。
func TestValidator_ValidateWorkflow_ToolAlias(t *testing.T) {
	v := newValidator(t)

	// Arrange: 含中文别名的工具定义
	data := []byte(`
version: "1"
name: alias-feature
tools:
  - name: reviewer
    display_name: 代码审核员
    aliases:
      - QA负责人
      - 审查员
    type: agent
    skill: "QA负责人Skill/SKILL.md"
    prompt_template: review
pipeline:
  - stage:
      name: review
      tool: reviewer
`)

	// Act
	err := v.ValidateWorkflow(data)

	// Assert
	if err != nil {
		t.Fatalf("tool with chinese aliases should be valid: %v", err)
	}
}

// TestValidator_ValidateTool 单独校验 tool 配置，验证 agent / command / function 三种类型的条件必填。
func TestValidator_ValidateTool(t *testing.T) {
	v := newValidator(t)

	cases := []struct {
		name    string
		data    string
		wantErr bool
	}{
		{
			name: "agent_with_skill_and_prompt",
			data: `
name: coder
type: agent
skill: "开发者Skill/SKILL.md"
prompt_template: coding
`,
			wantErr: false,
		},
		{
			name: "agent_without_skill",
			data: `
name: coder
type: agent
prompt_template: coding
`,
			wantErr: true,
		},
		{
			name: "command_with_run",
			data: `
name: lint
type: command
run: "golangci-lint run ./..."
`,
			wantErr: false,
		},
		{
			name: "command_without_run",
			data: `
name: lint
type: command
`,
			wantErr: true,
		},
		{
			name: "function_with_function_name",
			data: `
name: notify
type: function
function: send_webhook
`,
			wantErr: false,
		},
		{
			name: "function_without_function_name",
			data: `
name: notify
type: function
`,
			wantErr: true,
		},
		{
			name: "unknown_type",
			data: `
name: foo
type: unknown
`,
			wantErr: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Act
			err := v.ValidateTool([]byte(tc.data))

			// Assert
			if tc.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
		})
	}
}

// TestValidationError_Error 验证 ValidationError 的字符串格式包含字段路径。
func TestValidationError_Error(t *testing.T) {
	// Arrange
	ve := &ValidationError{
		Errors: []FieldError{
			{Location: "/tools/0/name", Message: "invalid pattern"},
			{Location: "", Message: "missing pipeline"},
		},
	}

	// Act
	got := ve.Error()

	// Assert
	if !strings.Contains(got, "/tools/0/name") {
		t.Errorf("error output missing location: %s", got)
	}
	if !strings.Contains(got, "(root)") {
		t.Errorf("error output should mark empty location as (root): %s", got)
	}
}

// TestNewValidator_LoadsAllSchemas 验证 NewValidator 能加载全部 9 个 schema 文件且无错误。
func TestNewValidator_LoadsAllSchemas(t *testing.T) {
	// Act
	v, err := NewValidator()

	// Assert
	if err != nil {
		t.Fatalf("NewValidator failed: %v", err)
	}
	if v == nil {
		t.Fatal("NewValidator returned nil")
	}
	if v.workflow == nil {
		t.Fatal("workflow schema not compiled")
	}
}

// TestValidateWorkflow_YAMLFormat 验证 YAML 格式（含注释）能正确解码。
func TestValidateWorkflow_YAMLFormat(t *testing.T) {
	v := newValidator(t)

	// Arrange: 含注释和多行结构的 YAML
	data := []byte(`# code-bee workflow 配置
version: "1"
name: yaml-with-comments
tools:
  - name: coder           # 编码智能体
    type: agent
    skill: "开发者Skill/SKILL.md"
    prompt_template: coding
pipeline:
  - stage:
      name: coding
      tool: coder
`)

	// Act
	err := v.ValidateWorkflow(data)

	// Assert
	if err != nil {
		t.Fatalf("YAML with comments should be valid: %v", err)
	}
}

// TestValidateWorkflow_DecodeError 验证 decode 阶段的错误会被包装返回。
func TestValidateWorkflow_DecodeError(t *testing.T) {
	v := newValidator(t)

	cases := []struct {
		name string
		data string
	}{
		{name: "empty", data: ""},
		{name: "whitespace_only", data: "   \n\t  "},
		{name: "invalid_yaml", data: "version: 1\n  bad: : : indent"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Act
			err := v.ValidateWorkflow([]byte(tc.data))

			// Assert
			if err == nil {
				t.Fatalf("expected decode error, got nil")
			}
			if !strings.Contains(err.Error(), "decode") {
				t.Fatalf("expected decode error message, got: %v", err)
			}
		})
	}
}

// testCase 表示一个测试用例的元信息。
type testCase struct {
	name string
	path string
}

// loadCases 遍历指定目录下的所有 .yaml/.yml/.json 文件作为测试用例。
//
// 设计说明:
// - 用文件名（去后缀）作为子测试名，避免硬编码用例列表
// - 新增 testdata 文件即自动加入测试，无需改测试代码
func loadCases(t *testing.T, dir string) []testCase {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read testdata dir %s: %v", dir, err)
	}

	var cases []testCase
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := filepath.Ext(entry.Name())
		if ext != ".yaml" && ext != ".yml" && ext != ".json" {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ext)
		cases = append(cases, testCase{
			name: name,
			path: filepath.Join(dir, entry.Name()),
		})
	}

	if len(cases) == 0 {
		t.Fatalf("no test cases found in %s", dir)
	}
	return cases
}

// newValidator 是测试用的便捷构造函数。
func newValidator(t *testing.T) *Validator {
	t.Helper()
	v, err := NewValidator()
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}
	return v
}
