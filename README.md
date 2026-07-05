<p align="center">
  <img src="https://raw.githubusercontent.com/JiGuangWorker/code-bee/main/docs/images/logo/code-bee-logo.png" alt="code-bee logo" width="200">
</p>

# 🐝 code-bee

> 在 GitHub Issue 中 @agent 下达任务，AI 编码智能体自动执行、校验、提 PR 并回复结果。

**code-bee** 是一个极简的 AI 编码调度器。它不写代码，只做一件事：发现 Issue 中的任务，交给编码智能体去完成。像蜂巢中的工蜂一样，每个 Agent 各司其职，而你只需要在 Issue 中 `@agent-name` 描述需求。

![code-bee 工作流](https://raw.githubusercontent.com/JiGuangWorker/code-bee/main/docs/images/readme/01-what-is-codebee.png)

---

## ✨ 为什么选择 code-bee？

- **Issue 即任务面板** —— 不需要额外工具，GitHub Issue 就是你的任务系统
- **@ mention 即调度** —— `@agent-frontend` 重构组件、`@agent-backend` 修 bug，像 @ 同事一样自然
- **零侵入** —— code-bee 本身不触碰你的代码，所有操作由智能体独立完成
- **场景驱动调度** —— 角色体系和管线流程由 YAML workflow 配置驱动，不再硬编码；内置默认开箱即用，`--workflow` 支持完全自定义

---

## 🧱 技术栈 & 依赖

| 组件 | 项目 | 说明 |
|------|------|------|
| 🧠 编码智能体 | [Reasonix](https://github.com/tryreasonix/reasonix) × [DeepSeek](https://www.deepseek.com/) | 接收任务指令，自主完成读 Issue → 编码 → 提 PR 全流程 |
| 🔧 Git 操作 | [GitHub CLI (`gh`)](https://cli.github.com/) | 智能体通过 `gh` 完成 fork、clone、commit、push、create PR |
| 🏗️ 调度器 | **code-bee** (Go 1.23+) | 解析 Issue 中的 @agent，按 workflow 配置编排 stage/parallel/loop，驱动智能体执行 |
| ✅ 代码校验 | [golangci-lint](https://golangci-lint.run/) + [conform](https://github.com/talos-systems/conform) | 静态分析 + 目录结构校验 |

### 架构一览

code-bee 通过 **workflow 配置**驱动整个调度流程：读取 Issue → 判断接单/阻塞 → 编码-审查循环 → 提交评论。角色体系和管线流程完全可配置，内置默认 workflow 开箱即用。

```
Issue @agent → runtime.Engine 遍历 workflow.Pipeline
                 │
                 ├─ stage: issue-handling    (判断 READY/BLOCKED)
                 ├─ stage: issue-post-intake  (when: READY → 提交接单评论)
                 └─ loop: coding-review       (max_iterations: 3)
                      ├─ stage: coding
                      ├─ stage: review
                      ├─ stage: issue-post-completion (when: PASS → 提交完成评论)
                      └─ judge: loop-judge    (价值评估，决定 continue/stop)
```

![四阶段架构](https://raw.githubusercontent.com/JiGuangWorker/code-bee/main/docs/images/readme/02-architecture.png)

---

## 🚀 快速开始

### 前置要求

- [Go](https://go.dev/dl/) 1.23+
- [Reasonix](https://github.com/tryreasonix/reasonix) 已安装并配置
- [GitHub CLI](https://cli.github.com/) 已安装并登录 (`gh auth login`)

### 安装

```bash
git clone https://github.com/JiGuangWorker/code-bee.git
cd code-bee
make build
```

### 使用

1. 在 GitHub Issue 中用 `@agent-name` 下达任务：

```markdown
## 任务
@agent-frontend 请把 UserProfile 组件从 class 重构为 hooks

## 详细说明
- 使用 TypeScript
- 保持 API 兼容
- 补充单元测试

## 验收标准
- [ ] npm run lint 通过
- [ ] npm run test 通过
- [ ] npm run build 通过
```

2. 运行 code-bee 指向该 Issue：

```bash
# 使用内置默认 workflow（开箱即用）
code-bee --repo your-org/your-repo --issue 42

# 或指定自定义 workflow
code-bee --repo your-org/your-repo --issue 42 --workflow ./my-workflow.yaml
```

3. code-bee 自动完成后续流程：

```
🐝 code-bee 0.1.0
📦 仓库: your-org/your-repo | Issue: #42

🚀 正在执行 workflow 驱动的调度流程...

✅ reviewer 已明确 PASS，且最终 Issue 回复已提交，任务执行完成
```

---

## 📂 项目结构

```
.
├── cmd/worker/              # CLI 入口，加载 workflow 并启动调度
├── internal/
│   ├── agent/               # 编码智能体执行器（runner）
│   ├── config/              # 运行时配置（Repo/IssueNumber/Token）
│   ├── parser/              # Issue 解析
│   ├── pipeline/            # 文件契约 + runtime.Engine 薄封装 + ArtifactResolver 适配层
│   ├── platform/            # 平台适配（GitHub 等）
│   ├── runtime/             # 可编排执行引擎（Engine/Tool/Executor/PromptBuilder + 默认 workflow）
│   └── schema/              # 三层 JSON Schema + YAML 加载器 + 语义校验
├── pkg/version/             # 版本信息
├── Makefile                 # 统一操作入口
├── .conform.yaml            # 目录结构校验
└── .golangci.yml            # 代码规范检查
```

---

## ⚙️ 自定义 Workflow

code-bee 的调度策略完全由 YAML workflow 配置驱动。无 `--workflow` flag 时使用内置默认配置（等价于原四阶段 harness），提供自定义 YAML 即可重定义角色体系和管线流程。

### 配置结构

一个 workflow 包含两部分：**工具集**（tools）和**管线编排**（pipeline）。

```yaml
# 工具集：定义可用角色及其能力
tools:
  - name: coder                    # 内部标识，pipeline 引用此名
    display_name: 开发者            # 类人展示名，渲染进 prompt
    aliases: [开发者]               # @mention 别名，Issue 中 @ 任一别名都可触发
    type: agent
    prompt_template: coding         # 引用内置 prompt 模板

  - name: reviewer
    display_name: QA负责人
    aliases: [QA负责人, 代码审核员]
    type: agent
    prompt_template: review

# 管线编排：用 stage / parallel / loop 三种原语组合流程
pipeline:
  - stage:
      name: issue-handling
      tool: issue-handling
      output: issue_intake_result.json

  - loop:
      id: coding-review
      max_iterations: 3             # 硬上限，防止死循环
      exit_when:
        - stage: review
          field: status
          operator: equals
          value: PASS
      body:
        - stage: { name: coding, tool: coder, input_from: issue-handling }
        - stage: { name: review, tool: reviewer, input_from: coding }
      judge:                        # 可选：价值评估员
        tool: loop-judge
        start_round: 1
```

### 三种编排原语

| 原语 | 语义 | 适用场景 |
|------|------|---------|
| `stage` | 串行单步 | 顺序执行的单个阶段 |
| `parallel` | 并行执行（fork-join） | 多个独立检查同时跑 |
| `loop` | 循环（含 `max_iterations` / `exit_when` / `judge`） | coder-reviewer 迭代 |

支持嵌套：loop body 内可含 parallel，parallel 内可含 stage。阶段可通过 `when` 条件实现跳过。

### 三种工具类型

| 类型 | 用途 | 示例 |
|------|------|------|
| `agent` | 调用 AI 智能体执行任务 | 编码、审查、Issue 提交 |
| `command` | 执行 shell 命令 | `golangci-lint run`、`npm test` |
| `function` | 调用注册的 Go 函数 | 内置扩展点（预留） |

### 角色名派生

prompt 模板中的角色展示名（如 `@{{.DefaultAgent}}`）从 `workflow.Tools` 按 `prompt_template` 自动派生，不再硬编码。用户只需在 tool 定义中设置 `display_name`，prompt 渲染时自动取用。

### 更多细节

- [Schema 定义](internal/schema/schemas/v1/README.md) —— 三层 JSON Schema 与校验机制
- [默认 workflow](internal/runtime/default_workflow.yaml) —— 内置配置参考
- [Prompt 模板](internal/runtime/prompts/) —— 5 个内置模板文件

---

## 🔮 路线图

code-bee 的场景驱动调度已落地，未来计划扩展的方向包括：

- [x] **场景驱动调度** —— 角色、管线、循环参数完全由 YAML workflow 配置驱动（[#11](https://github.com/JiGuangWorker/code-bee/issues/11)）
- [x] **可编排管线** —— stage / parallel / loop 三种原语，支持嵌套与条件跳过
- [x] **工具别名** —— `@mention` 别名机制，角色体系完全可自定义
- [ ] **阻塞评论恢复** —— 增强 Engine 支持 `on_blocked: continue`，恢复 issue-post-blocked 阶段
- [ ] **多智能体支持** —— 除 Reasonix 外，接入 Qoder、Cline 等更多编码智能体
- [ ] **GitLab 适配** —— 将 Issue → MR 的调度能力扩展到 GitLab 平台
- [ ] **Webhook 触发** —— 支持 Issue 事件自动触发，无需手动执行 CLI
- [ ] **校验管线** —— 用 `command` 工具组合 lint → test → build → security scan 流程

> 💡 欢迎提 Issue / PR 一起建设！每个想法都值得被讨论。

---

## 🤝 贡献

如果你对「让 AI 像同事一样协作编码」这件事感兴趣，欢迎加入：

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feat/amazing-idea`)
3. 提交变更 (`git commit -m 'feat: add amazing idea'`)
4. 推送到分支 (`git push origin feat/amazing-idea`)
5. 创建 Pull Request

### 开发命令

```bash
make build              # 编译
make test               # 运行测试
make lint               # 代码检查
make check-structure    # 目录结构校验
make check-all          # 全部校验
```

---

## 📄 License

MIT © [JiGuangWorker](https://github.com/JiGuangWorker)
