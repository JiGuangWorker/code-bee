package schema

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestLoader_LoadFromBytes_Valid 全部合法配置应成功加载并返回结构体。
func TestLoader_LoadFromBytes_Valid(t *testing.T) {
	l := newLoader(t)

	cases := loadCases(t, "testdata/valid")

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			data, err := os.ReadFile(tc.path)
			if err != nil {
				t.Fatalf("read testdata: %v", err)
			}

			// Act
			wf, err := l.LoadFromBytes(data)

			// Assert
			if err != nil {
				t.Fatalf("expected valid, got error: %v", err)
			}
			if wf == nil {
				t.Fatal("workflow is nil")
			}
			if wf.Version != "1" {
				t.Errorf("version = %q, want \"1\"", wf.Version)
			}
			if len(wf.Tools) == 0 {
				t.Error("tools is empty")
			}
			if len(wf.Pipeline) == 0 {
				t.Error("pipeline is empty")
			}
		})
	}
}

// TestLoader_LoadFromBytes_ToolAlias 验证工具别名查找：
// FindTool 应能通过 name 和任一 alias 找到同一个 Tool。
func TestLoader_LoadFromBytes_ToolAlias(t *testing.T) {
	l := newLoader(t)

	// Arrange
	data := []byte(`
version: "1"
name: alias-lookup
tools:
  - name: reviewer
    display_name: 代码审核员
    aliases:
      - QA负责人
      - 审查员
    type: agent
    skill: "QA负责人Skill/SKILL.md"
    prompt_template: review
  - name: coder
    type: agent
    skill: "开发者Skill/SKILL.md"
    prompt_template: coding
pipeline:
  - stage:
      name: coding
      tool: coder
  - stage:
      name: review
      tool: 审查员
`)

	// Act
	wf, err := l.LoadFromBytes(data)

	// Assert
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	// 通过 name 查找
	r := wf.FindTool("reviewer")
	if r == nil {
		t.Fatal("FindTool by name failed")
	}
	if r.DisplayName != "代码审核员" {
		t.Errorf("display_name = %q, want 代码审核员", r.DisplayName)
	}

	// 通过 alias 查找（中文）
	r2 := wf.FindTool("QA负责人")
	if r2 == nil || r2.Name != "reviewer" {
		t.Errorf("FindTool by alias QA负责人 failed")
	}

	// 通过另一个 alias 查找
	r3 := wf.FindTool("审查员")
	if r3 == nil || r3.Name != "reviewer" {
		t.Errorf("FindTool by alias 审查员 failed")
	}

	// 不存在的引用
	r4 := wf.FindTool("不存在")
	if r4 != nil {
		t.Error("FindTool should return nil for unknown ref")
	}

	// pipeline 中 stage.tool 用 alias 也应通过校验
	if len(wf.Pipeline) != 2 {
		t.Fatalf("pipeline length = %d, want 2", len(wf.Pipeline))
	}
	if wf.Pipeline[1].Stage.Tool != "审查员" {
		t.Errorf("pipeline[1].stage.tool = %q, want 审查员", wf.Pipeline[1].Stage.Tool)
	}
}

// TestLoader_SemanticErrors 验证语义校验能识别 schema 校验不到的问题。
func TestLoader_SemanticErrors(t *testing.T) {
	l := newLoader(t)

	cases := []struct {
		name       string
		data       string
		wantErrIs  error
		wantSubstr string
	}{
		{
			name: "duplicate_tool_name",
			data: `
version: "1"
name: dup-name
tools:
  - name: coder
    type: agent
    skill: "s.md"
    prompt_template: c
  - name: coder
    type: agent
    skill: "s2.md"
    prompt_template: c
pipeline:
  - stage:
      name: coding
      tool: coder
`,
			wantErrIs:  errToolNameConflict,
			wantSubstr: "duplicate tool name",
		},
		{
			name: "alias_conflicts_with_name",
			data: `
version: "1"
name: alias-conflict
tools:
  - name: coder
    type: agent
    skill: "s.md"
    prompt_template: c
  - name: reviewer
    type: agent
    skill: "r.md"
    prompt_template: r
    aliases:
      - coder
pipeline:
  - stage:
      name: coding
      tool: coder
`,
			wantErrIs:  errToolNameConflict,
			wantSubstr: "conflicts with a tool name",
		},
		{
			name: "duplicate_alias",
			data: `
version: "1"
name: dup-alias
tools:
  - name: coder
    type: agent
    skill: "s.md"
    prompt_template: c
    aliases:
      - 开发者
  - name: reviewer
    type: agent
    skill: "r.md"
    prompt_template: r
    aliases:
      - 开发者
pipeline:
  - stage:
      name: coding
      tool: coder
`,
			wantErrIs:  errToolNameConflict,
			wantSubstr: "duplicate alias",
		},
		{
			name: "unknown_tool_ref",
			data: `
version: "1"
name: unknown-tool
tools:
  - name: coder
    type: agent
    skill: "s.md"
    prompt_template: c
pipeline:
  - stage:
      name: coding
      tool: ghost
`,
			wantErrIs:  errUnknownToolRef,
			wantSubstr: "ghost",
		},
		{
			name: "input_from_unknown_stage",
			data: `
version: "1"
name: bad-input-from
tools:
  - name: coder
    type: agent
    skill: "s.md"
    prompt_template: c
  - name: reviewer
    type: agent
    skill: "r.md"
    prompt_template: r
pipeline:
  - stage:
      name: review
      tool: reviewer
      input_from: ghost
`,
			wantErrIs:  errUnknownStageRef,
			wantSubstr: "input_from",
		},
		{
			name: "loop_exit_when_unknown_stage",
			data: `
version: "1"
name: bad-exit-when
tools:
  - name: coder
    type: agent
    skill: "s.md"
    prompt_template: c
pipeline:
  - loop:
      id: dev-loop
      max_iterations: 3
      exit_when:
        - stage: ghost
          field: status
          operator: equals
          value: PASS
      body:
        - stage:
            name: coding
            tool: coder
`,
			wantErrIs:  errUnknownStageRef,
			wantSubstr: "ghost",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Act
			_, err := l.LoadFromBytes([]byte(tc.data))

			// Assert
			if err == nil {
				t.Fatalf("expected semantic error, got nil")
			}
			if !errors.Is(err, tc.wantErrIs) {
				t.Errorf("error type = %v, want %v", err, tc.wantErrIs)
			}
			if tc.wantSubstr != "" && !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Errorf("error message %q does not contain %q", err.Error(), tc.wantSubstr)
			}
		})
	}
}

// TestLoader_LoadFromFile 验证从文件加载。
func TestLoader_LoadFromFile(t *testing.T) {
	l := newLoader(t)

	// Arrange
	path := filepath.Join("testdata", "valid", "minimal.yaml")

	// Act
	wf, err := l.LoadFromFile(path)

	// Assert
	if err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}
	if wf.Name != "minimal" {
		t.Errorf("name = %q, want minimal", wf.Name)
	}
}

// TestLoader_LoadFromFile_NotExist 验证文件不存在的错误。
func TestLoader_LoadFromFile_NotExist(t *testing.T) {
	l := newLoader(t)

	// Act
	_, err := l.LoadFromFile("testdata/nonexistent.yaml")

	// Assert
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
	if !strings.Contains(err.Error(), "nonexistent.yaml") {
		t.Errorf("error should mention file path: %v", err)
	}
}

// TestWorkflow_FindTool_ByAlias 覆盖 FindTool 的三种命中路径。
func TestWorkflow_FindTool_ByAlias(t *testing.T) {
	// Arrange
	wf := &Workflow{
		Tools: []Tool{
			{Name: "coder", Aliases: []string{"开发者"}},
			{Name: "reviewer", DisplayName: "代码审核员"},
		},
	}

	cases := []struct {
		name    string
		ref     string
		wantNil bool
		wantKey string
	}{
		{name: "by_name", ref: "coder", wantKey: "coder"},
		{name: "by_alias", ref: "开发者", wantKey: "coder"},
		{name: "by_name_no_alias", ref: "reviewer", wantKey: "reviewer"},
		{name: "unknown", ref: "ghost", wantNil: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Act
			got := wf.FindTool(tc.ref)

			// Assert
			if tc.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected non-nil for ref %q", tc.ref)
			}
			if got.Name != tc.wantKey {
				t.Errorf("name = %q, want %q", got.Name, tc.wantKey)
			}
		})
	}
}

// newLoader 是测试用的便捷构造函数。
func newLoader(t *testing.T) *Loader {
	t.Helper()
	l, err := NewLoader()
	if err != nil {
		t.Fatalf("NewLoader: %v", err)
	}
	return l
}

// TestCondition_UnmarshalYAML 验证 Condition 的三种 YAML 形态：字符串、对象、非法类型。
func TestCondition_UnmarshalYAML(t *testing.T) {
	cases := []struct {
		name      string
		yaml      string
		wantExpr  string
		wantStage string
		wantErr   bool
	}{
		{
			name:     "string_form",
			yaml:     `"round >= 2"`,
			wantExpr: "round >= 2",
		},
		{
			name: "object_form",
			yaml: `stage: review
field: status
operator: equals
value: PASS`,
			wantStage: "review",
		},
		{
			name: "sequence_invalid",
			yaml: `- foo
- bar`,
			wantErr: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			var c Condition
			data := []byte(tc.yaml)

			// Act
			err := yaml.Unmarshal(data, &c)

			// Assert
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantExpr != "" && c.Expr != tc.wantExpr {
				t.Errorf("expr = %q, want %q", c.Expr, tc.wantExpr)
			}
			if tc.wantStage != "" {
				if c.Structured == nil {
					t.Fatal("structured is nil")
				}
				if c.Structured.Stage != tc.wantStage {
					t.Errorf("stage = %q, want %q", c.Structured.Stage, tc.wantStage)
				}
			}
		})
	}
}
