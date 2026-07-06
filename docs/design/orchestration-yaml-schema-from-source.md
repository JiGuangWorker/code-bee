# CodeB 编排 YAML Schema（源码映射稿）

## 1. 目的

本文档不是重新发明一套运行时协议，而是把当前 `code-bee` 已经写死在源码里的编排事实，整理成一份可供编辑器解析、展示、编辑、再组装回 YAML 的标准结构。

这份 Schema 的定位只有一个：

- 给前端编辑器作为标准输入输出格式

当前明确**不包含**：

- 不改 `dispatch`
- 不改 scheduler
- 不让运行时直接读取这份 YAML 执行
- 不把 CodeB 变成通用工作流引擎

## 2. 来源边界

这份 Schema 只允许映射当前源码里已经稳定存在的事实，主要来自以下文件：

- [`internal/pipeline/dispatch.go`](file:///Users/lihongyang/Desktop/code-bee-tools/internal/pipeline/dispatch.go)
- [`internal/pipeline/contracts.go`](file:///Users/lihongyang/Desktop/code-bee-tools/internal/pipeline/contracts.go)
- [`internal/pipeline/artifacts.go`](file:///Users/lihongyang/Desktop/code-bee-tools/internal/pipeline/artifacts.go)
- [`internal/pipeline/history.go`](file:///Users/lihongyang/Desktop/code-bee-tools/internal/pipeline/history.go)
- [`internal/pipeline/prompts.go`](file:///Users/lihongyang/Desktop/code-bee-tools/internal/pipeline/prompts.go)
- [`internal/config/config.go`](file:///Users/lihongyang/Desktop/code-bee-tools/internal/config/config.go)

因此它能稳定表达的只有：

- 固定阶段
- 固定角色
- 固定结果契约
- 固定边条件
- 固定 runtime 参数
- 固定工件命名

它**不能**从现有源码中稳定推导的内容包括：

- 画布坐标
- 节点分组/颜色/折叠状态
- 任意自定义节点类型
- 通用表达式引擎
- 任意脚本型边条件

所以，编辑器可以增加自己的 UI 元数据，但那必须放在独立字段里，不能污染这份核心 Schema。

## 3. 总体原则

这份 YAML Schema 必须满足 4 条原则：

1. 只表达当前源码已有事实，不补脑
2. 能被前端稳定解析成节点图
3. 能从前端编辑态稳定组装回 YAML
4. 不要求当前运行时直接消费它

## 4. 顶层结构

标准顶层结构建议如下：

```yaml
apiVersion: codebee/v1
kind: Orchestration
metadata:
  name: default-issue-flow
  description: CodeB 当前固定四阶段流程

workflow:
  startNodeId: issue_handling
  nodes: []
  edges: []

runtime:
  maxCodingReviewRounds: 3
  maxConsecutiveUnknownReview: 2
  maxIssuePostAttempts: 2
  loopJudgeStartRound: 0

roles:
  defaultAgent: 开发者
  reviewerAgent: QA负责人
  issuePostAgent: 产品经理
  loopJudgeAgent: 技术负责人

artifacts:
  issueHandlingResult: issue_intake_result.json
  codingResult: coding_result.json
  reviewResult: review_result.json
  issuePostResultPattern: issue_post_result_{purpose}.json
  loopJudgeResult: loop_judge_result.json
  loopHistory: loop_history.json

ui:
  nodes: {}
```

## 5. 顶层字段定义

### 5.1 `apiVersion`

固定值：

```yaml
apiVersion: codebee/v1
```

说明：

- 表示这是给 CodeB 编排编辑器使用的 YAML 表达版本
- 这里的版本是编辑器 Schema 版本，不是运行时协议版本

### 5.2 `kind`

固定值：

```yaml
kind: Orchestration
```

### 5.3 `metadata`

```yaml
metadata:
  name: default-issue-flow
  description: CodeB 当前固定四阶段流程
```

字段说明：

- `name`: 编排名称，必填
- `description`: 编排说明，选填

### 5.4 `workflow`

```yaml
workflow:
  startNodeId: issue_handling
  nodes: []
  edges: []
```

字段说明：

- `startNodeId`: 起始节点 ID，必填
- `nodes`: 节点列表，必填
- `edges`: 边列表，必填

### 5.5 `runtime`

```yaml
runtime:
  maxCodingReviewRounds: 3
  maxConsecutiveUnknownReview: 2
  maxIssuePostAttempts: 2
  loopJudgeStartRound: 0
```

字段说明：

- `maxCodingReviewRounds`: coder-reviewer 最大轮数，当前默认值来自 [`dispatch.go:L25`](file:///Users/lihongyang/Desktop/code-bee-tools/internal/pipeline/dispatch.go#L25)
- `maxConsecutiveUnknownReview`: 连续 `UNKNOWN` 阈值，当前默认值来自 [`dispatch.go:L26`](file:///Users/lihongyang/Desktop/code-bee-tools/internal/pipeline/dispatch.go#L26)
- `maxIssuePostAttempts`: Issue 提交最大重试次数，当前默认值来自 [`dispatch.go:L27`](file:///Users/lihongyang/Desktop/code-bee-tools/internal/pipeline/dispatch.go#L27)
- `loopJudgeStartRound`: 从第几轮开始触发价值评估员，默认值来自 [`config.go:L13-L15`](file:///Users/lihongyang/Desktop/code-bee-tools/internal/config/config.go#L13-L15)

### 5.6 `roles`

```yaml
roles:
  defaultAgent: 开发者
  reviewerAgent: QA负责人
  issuePostAgent: 产品经理
  loopJudgeAgent: 技术负责人
```

字段说明：

- `defaultAgent`: 编码主角色
- `reviewerAgent`: 审查角色
- `issuePostAgent`: Issue 提交角色
- `loopJudgeAgent`: 价值评估角色

默认值来自 [`config.go:L104-L110`](file:///Users/lihongyang/Desktop/code-bee-tools/internal/config/config.go#L104-L110)。

### 5.7 `artifacts`

```yaml
artifacts:
  issueHandlingResult: issue_intake_result.json
  codingResult: coding_result.json
  reviewResult: review_result.json
  issuePostResultPattern: issue_post_result_{purpose}.json
  loopJudgeResult: loop_judge_result.json
  loopHistory: loop_history.json
```

字段说明：

- 用于给编辑器展示每个节点对应的工件命名
- 来源于 [`artifacts.go`](file:///Users/lihongyang/Desktop/code-bee-tools/internal/pipeline/artifacts.go)

### 5.8 `ui`

```yaml
ui:
  nodes:
    issue_handling:
      x: 100
      y: 120
```

说明：

- `ui` 是编辑器专用元数据区
- 只允许放展示相关字段，如坐标、折叠状态、颜色
- 不允许放运行语义

## 6. `workflow.nodes` 结构

单个节点结构如下：

```yaml
- id: issue_handling
  type: issue-handling
  name: Issue 处理
  roleRef: issuePostAgent
  promptTemplate: issue-handling
  resultContract: issue-handling-result
  outputs:
    - issue_intake_result.json
```

字段定义：

- `id`: 节点唯一标识
- `type`: 节点类型
- `name`: 节点显示名称
- `roleRef`: 角色引用，引用 `roles` 中的 key
- `promptTemplate`: 提示词模板标识
- `resultContract`: 结果契约标识
- `inputs`: 输入工件列表，可选
- `outputs`: 输出工件列表，必填

## 7. 标准节点类型

当前只能从源码里稳定提取这 5 类节点：

```yaml
issue-handling
coding
review
issue-post
loop-judge
```

对应来源：

- `issue-handling`: [`runIssueHandling`](file:///Users/lihongyang/Desktop/code-bee-tools/internal/pipeline/dispatch.go#L111-L158)
- `coding`: [`runCodingRound`](file:///Users/lihongyang/Desktop/code-bee-tools/internal/pipeline/dispatch.go#L363-L401)
- `review`: [`runReviewRound`](file:///Users/lihongyang/Desktop/code-bee-tools/internal/pipeline/dispatch.go#L403-L442)
- `issue-post`: [`runIssuePostWithRetry`](file:///Users/lihongyang/Desktop/code-bee-tools/internal/pipeline/dispatch.go#L444-L503)
- `loop-judge`: [`runLoopJudgeIfNeeded`](file:///Users/lihongyang/Desktop/code-bee-tools/internal/pipeline/dispatch.go#L305-L339)

## 8. 推荐默认节点清单

基于当前源码，推荐固定为这 6 个节点：

```yaml
workflow:
  startNodeId: issue_handling
  nodes:
    - id: issue_handling
      type: issue-handling
      name: Issue 处理
      roleRef: issuePostAgent
      promptTemplate: issue-handling
      resultContract: issue-handling-result
      outputs:
        - issue_intake_result.json

    - id: issue_post_intake
      type: issue-post
      name: 接单回帖
      roleRef: issuePostAgent
      promptTemplate: issue-post
      resultContract: issue-post-result
      inputs:
        - issue_intake_result.json
      outputs:
        - issue_post_result_intake.json

    - id: coding_main
      type: coding
      name: 编码实现
      roleRef: defaultAgent
      promptTemplate: coding
      resultContract: coding-result
      inputs:
        - issue_intake_result.json
        - review_result.json
      outputs:
        - coding_result.json

    - id: review_main
      type: review
      name: 结果审查
      roleRef: reviewerAgent
      promptTemplate: review
      resultContract: review-result
      inputs:
        - issue_intake_result.json
        - coding_result.json
      outputs:
        - review_result.json

    - id: issue_post_completion
      type: issue-post
      name: 完成回帖
      roleRef: issuePostAgent
      promptTemplate: issue-post
      resultContract: issue-post-result
      inputs:
        - review_result.json
      outputs:
        - issue_post_result_completion.json

    - id: loop_judge_main
      type: loop-judge
      name: 价值评估
      roleRef: loopJudgeAgent
      promptTemplate: loop-judge
      resultContract: loop-judge-result
      inputs:
        - loop_history.json
      outputs:
        - loop_judge_result.json
```

## 9. `workflow.edges` 结构

单条边结构如下：

```yaml
- from: review_main
  to: coding_main
  when:
    reviewStatus: FAIL
```

字段定义：

- `from`: 起点节点 ID
- `to`: 终点节点 ID
- `when`: 迁移条件

`when` 当前只允许使用受控条件字段，不能写任意表达式。

## 10. 标准边条件字段

当前建议只允许这 5 类条件字段：

```yaml
issueStatus
codingStatus
reviewStatus
issuePostStatus
loopJudgeDecision
```

### 10.1 `issueStatus`

取值来自 [`contracts.go:L20-L21`](file:///Users/lihongyang/Desktop/code-bee-tools/internal/pipeline/contracts.go#L20-L21)：

```yaml
READY
BLOCKED
```

### 10.2 `codingStatus`

取值来自 [`contracts.go:L23-L25`](file:///Users/lihongyang/Desktop/code-bee-tools/internal/pipeline/contracts.go#L23-L25)：

```yaml
DONE
IN_PROGRESS
BLOCKED
```

### 10.3 `reviewStatus`

取值来自 [`contracts.go:L27-L30`](file:///Users/lihongyang/Desktop/code-bee-tools/internal/pipeline/contracts.go#L27-L30)：

```yaml
PASS
FAIL
UNKNOWN
BLOCKED
```

### 10.4 `issuePostStatus`

取值来自 [`contracts.go:L32-L34`](file:///Users/lihongyang/Desktop/code-bee-tools/internal/pipeline/contracts.go#L32-L34)：

```yaml
POSTED
REJECTED
BLOCKED
```

### 10.5 `loopJudgeDecision`

取值来自 [`contracts.go:L181-L184`](file:///Users/lihongyang/Desktop/code-bee-tools/internal/pipeline/contracts.go#L181-L184)：

```yaml
CONTINUE
SHRINK_TASK
STOP_MANUAL
STOP_BLOCKED
```

## 11. 推荐默认边

当前固定流程可整理为以下边：

```yaml
workflow:
  edges:
    - from: issue_handling
      to: issue_post_intake

    - from: issue_post_intake
      to: coding_main
      when:
        issuePostStatus: POSTED

    - from: coding_main
      to: review_main
      when:
        codingStatus: DONE

    - from: review_main
      to: issue_post_completion
      when:
        reviewStatus: PASS

    - from: review_main
      to: coding_main
      when:
        reviewStatus: FAIL

    - from: review_main
      to: coding_main
      when:
        reviewStatus: UNKNOWN

    - from: review_main
      to: loop_judge_main
      when:
        reviewStatus: FAIL

    - from: review_main
      to: loop_judge_main
      when:
        reviewStatus: UNKNOWN

    - from: loop_judge_main
      to: coding_main
      when:
        loopJudgeDecision: CONTINUE

    - from: loop_judge_main
      to: coding_main
      when:
        loopJudgeDecision: SHRINK_TASK
```

说明：

- `STOP_MANUAL` 和 `STOP_BLOCKED` 是终止决策，不再回流到 `coding_main`
- `issue_handling -> issue_post_intake` 可以没有 `when`，因为这一步是固定串联

## 12. 标准结果契约标识

编辑器里不需要展开每个 JSON 字段定义到节点内部，但需要知道节点绑定的是哪种结果契约。

推荐使用以下标识：

```yaml
issue-handling-result
coding-result
review-result
issue-post-result
loop-judge-result
```

它们分别映射到源码中的结构体：

- `issue-handling-result` -> [`IssueHandlingResult`](file:///Users/lihongyang/Desktop/code-bee-tools/internal/pipeline/contracts.go#L47-L66)
- `coding-result` -> [`CodingResult`](file:///Users/lihongyang/Desktop/code-bee-tools/internal/pipeline/contracts.go#L80-L96)
- `review-result` -> [`ReviewResult`](file:///Users/lihongyang/Desktop/code-bee-tools/internal/pipeline/contracts.go#L108-L129)
- `issue-post-result` -> [`IssuePostResult`](file:///Users/lihongyang/Desktop/code-bee-tools/internal/pipeline/contracts.go#L149-L161)
- `loop-judge-result` -> [`LoopJudgeResult`](file:///Users/lihongyang/Desktop/code-bee-tools/internal/pipeline/contracts.go#L187-L201)

## 13. 标准提示词模板标识

为避免把整段长 prompt 直接写进 YAML，建议只保留模板标识：

```yaml
issue-handling
coding
review
issue-post
loop-judge
```

这些模板对应当前源码中的构造函数：

- `issue-handling` -> [`buildIssueHandlingPrompt`](file:///Users/lihongyang/Desktop/code-bee-tools/internal/pipeline/prompts.go#L14-L84)
- `coding` -> [`buildCodingPrompt`](file:///Users/lihongyang/Desktop/code-bee-tools/internal/pipeline/prompts.go#L86-L114)
- `review` -> [`buildReviewPrompt`](file:///Users/lihongyang/Desktop/code-bee-tools/internal/pipeline/prompts.go#L181-L220)
- `issue-post` -> [`buildIssuePostPrompt`](file:///Users/lihongyang/Desktop/code-bee-tools/internal/pipeline/prompts.go#L264-L323)
- `loop-judge` -> [`buildLoopJudgePrompt`](file:///Users/lihongyang/Desktop/code-bee-tools/internal/pipeline/prompts.go#L325-L389)

## 14. 完整 YAML 示例

```yaml
apiVersion: codebee/v1
kind: Orchestration

metadata:
  name: default-issue-flow
  description: CodeB 当前固定四阶段流程与 coder-reviewer loop

workflow:
  startNodeId: issue_handling
  nodes:
    - id: issue_handling
      type: issue-handling
      name: Issue 处理
      roleRef: issuePostAgent
      promptTemplate: issue-handling
      resultContract: issue-handling-result
      outputs:
        - issue_intake_result.json

    - id: issue_post_intake
      type: issue-post
      name: 接单回帖
      roleRef: issuePostAgent
      promptTemplate: issue-post
      resultContract: issue-post-result
      inputs:
        - issue_intake_result.json
      outputs:
        - issue_post_result_intake.json

    - id: coding_main
      type: coding
      name: 编码实现
      roleRef: defaultAgent
      promptTemplate: coding
      resultContract: coding-result
      inputs:
        - issue_intake_result.json
        - review_result.json
      outputs:
        - coding_result.json

    - id: review_main
      type: review
      name: 结果审查
      roleRef: reviewerAgent
      promptTemplate: review
      resultContract: review-result
      inputs:
        - issue_intake_result.json
        - coding_result.json
      outputs:
        - review_result.json

    - id: issue_post_completion
      type: issue-post
      name: 完成回帖
      roleRef: issuePostAgent
      promptTemplate: issue-post
      resultContract: issue-post-result
      inputs:
        - review_result.json
      outputs:
        - issue_post_result_completion.json

    - id: loop_judge_main
      type: loop-judge
      name: 价值评估
      roleRef: loopJudgeAgent
      promptTemplate: loop-judge
      resultContract: loop-judge-result
      inputs:
        - loop_history.json
      outputs:
        - loop_judge_result.json

  edges:
    - from: issue_handling
      to: issue_post_intake

    - from: issue_post_intake
      to: coding_main
      when:
        issuePostStatus: POSTED

    - from: coding_main
      to: review_main
      when:
        codingStatus: DONE

    - from: review_main
      to: issue_post_completion
      when:
        reviewStatus: PASS

    - from: review_main
      to: coding_main
      when:
        reviewStatus: FAIL

    - from: review_main
      to: coding_main
      when:
        reviewStatus: UNKNOWN

    - from: review_main
      to: loop_judge_main
      when:
        reviewStatus: FAIL

    - from: review_main
      to: loop_judge_main
      when:
        reviewStatus: UNKNOWN

    - from: loop_judge_main
      to: coding_main
      when:
        loopJudgeDecision: CONTINUE

    - from: loop_judge_main
      to: coding_main
      when:
        loopJudgeDecision: SHRINK_TASK

runtime:
  maxCodingReviewRounds: 3
  maxConsecutiveUnknownReview: 2
  maxIssuePostAttempts: 2
  loopJudgeStartRound: 0

roles:
  defaultAgent: 开发者
  reviewerAgent: QA负责人
  issuePostAgent: 产品经理
  loopJudgeAgent: 技术负责人

artifacts:
  issueHandlingResult: issue_intake_result.json
  codingResult: coding_result.json
  reviewResult: review_result.json
  issuePostResultPattern: issue_post_result_{purpose}.json
  loopJudgeResult: loop_judge_result.json
  loopHistory: loop_history.json

ui:
  nodes:
    issue_handling:
      x: 120
      y: 120
    issue_post_intake:
      x: 340
      y: 120
    coding_main:
      x: 620
      y: 120
    review_main:
      x: 900
      y: 120
    issue_post_completion:
      x: 1180
      y: 120
    loop_judge_main:
      x: 900
      y: 320
```

## 15. 编辑器实现时的边界

编辑器下一步只应该做这些事：

1. 解析这份 YAML
2. 转成前端节点/边状态
3. 编辑后重新组装成这份 YAML
4. 对枚举和值域做静态校验

编辑器当前不应该做这些事：

1. 不驱动运行时执行
2. 不替换 `dispatch.go`
3. 不把任意表达式塞进 `when`
4. 不引入超出当前源码事实的新节点类型

## 16. 结论

从现有源码可以稳定反推出一份标准 YAML Schema，而且这份 Schema 足够支撑一个编排编辑器。

但它的身份必须保持清楚：

- 它是**源码事实的外部表达**
- 不是新的调度内核
- 不是新的运行时协议
- 更不是通用工作流平台 DSL

所以，后续最合理的实现顺序是：

1. 基于本文档实现 YAML 解析/组装
2. 基于解析结果实现前端编辑态
3. 最后再做画布和属性面板
