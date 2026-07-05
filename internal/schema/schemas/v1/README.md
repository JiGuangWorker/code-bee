# code-bee Workflow Schema v1

三层 JSON Schema，描述可配置的蜂巢调度策略。

## 层次结构

```
Layer 3: workflow.json            (顶层)
              │
Layer 2: pipeline_step.json       (编排原语：stage / parallel / loop 判别联合)
         loop.json
         loop_judge.json
              │
Layer 1: tool.json                (基础类型)
         stage.json
         condition.json
         exit_condition.json
         result_contract.json
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
