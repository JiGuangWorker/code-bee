# code-bee Workflow Schema v1

三层 JSON Schema（draft-07），描述可配置的蜂巢调度策略。

> **文件格式说明**：schema 定义文件统一用 YAML 格式（`.yaml`），加载时转 JSON 喂给校验库。`$id` 和 `$ref` 中的 URL 保持 `.json` 后缀作为 URI 标识符（JSON Schema 标准约定）。

## 层次结构

```
Layer 3: workflow.yaml            (顶层)
              │
Layer 2: pipeline_step.yaml       (编排原语：stage / parallel / loop 判别联合)
         loop.yaml
         loop_judge.yaml
              │
Layer 1: tool.yaml                (基础类型)
         stage.yaml
         condition.yaml
         exit_condition.yaml
         result_contract.yaml
```

## 关键设计

### 工具别名（@mention）

每个 tool 同时支持：

- `name`：内部标识，英文小写，pipeline 中引用此名
- `display_name`：类人展示名（如「代码审核员」），用于评论和日志
- `aliases`：@mention 别名数组，Issue 中 @ 任一别名都可触发本工具

```yaml
tools:
  - name: "reviewer"
    display_name: "代码审核员"
    aliases: ["QA负责人", "审查员"]
    type: "agent"
    skill: "QA负责人Skill/SKILL.md"
    prompt_template: "review"
```

### 编排原语

- **stage**：串行单步
- **parallel**：并行执行（fork-join）
- **loop**：循环（含 max_iterations / exit_when / judge）

```yaml
pipeline:
  - stage: {name: "intake", tool: "issue-handler"}
  - loop:
      id: "dev-loop"
      max_iterations: 3
      exit_when:
        - stage: "review"
          field: "status"
          operator: "equals"
          value: "PASS"
      body:
        - stage: {name: "coding", tool: "coder"}
        - stage: {name: "review", tool: "reviewer"}
  - stage: {name: "post", tool: "issue-post", when: "review.status == \"PASS\""}
```

## 校验

```go
import "github.com/JiGuangWorker/code-bee/internal/schema"

validator, err := schema.NewValidator()
err = validator.ValidateWorkflow(yamlBytes)
```
