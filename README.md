# 🐝 code-bee

> 在 GitHub Issue 中 @agent 下达任务，AI 编码智能体自动执行、校验、提 PR 并回复结果。

**code-bee** 是一个极简的 AI 编码调度器。它不写代码，只做一件事：发现 Issue 中的任务，交给编码智能体去完成。像蜂巢中的工蜂一样，每个 Agent 各司其职，而你只需要在 Issue 中 `@agent-name` 描述需求。

![code-bee 工作流](https://raw.githubusercontent.com/JiGuangWorker/code-bee/main/docs/images/readme/01-what-is-codebee.png)

---

## ✨ 为什么选择 code-bee？

- **Issue 即任务面板** —— 不需要额外工具，GitHub Issue 就是你的任务系统
- **@ mention 即调度** —— `@agent-frontend` 重构组件、`@agent-backend` 修 bug，像 @ 同事一样自然
- **零侵入** —— code-bee 本身不触碰你的代码，所有操作由智能体独立完成
- **极简内核** —— 不到 100 行核心逻辑，易于理解、审计和贡献

---

## 🧱 技术栈 & 依赖

| 组件 | 项目 | 说明 |
|------|------|------|
| 🧠 编码智能体 | [Reasonix](https://github.com/tryreasonix/reasonix) × [DeepSeek](https://www.deepseek.com/) | 接收任务指令，自主完成读 Issue → 编码 → 提 PR 全流程 |
| 🔧 Git 操作 | [GitHub CLI (`gh`)](https://cli.github.com/) | 智能体通过 `gh` 完成 fork、clone、commit、push、create PR |
| 🏗️ 调度器 | **code-bee** (Go 1.23+) | 解析 Issue 中的 @agent，生成任务指令，交给智能体执行 |
| ✅ 代码校验 | [golangci-lint](https://golangci-lint.run/) + [conform](https://github.com/talos-systems/conform) | 静态分析 + 目录结构校验 |

### 架构一览

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
code-bee --repo your-org/your-repo --issue 42
```

3. code-bee 自动完成后续流程：

```
🐝 code-bee 0.1.0
📦 仓库: your-org/your-repo | Issue: #42

🚀 正在通知编码智能体: 请查看 your-org/your-repo 仓库的 #42 Issue...

✅ 任务执行完成 → PR 已提交 → Issue 已回复
```

---

## 📂 项目结构

```
.
├── cmd/worker/          # CLI 入口
├── internal/
│   ├── agent/           # 编码智能体调度
│   ├── config/          # 配置管理
│   ├── parser/          # Issue 解析
│   ├── pipeline/        # 执行管线
│   └── platform/        # 平台适配
├── pkg/version/         # 版本信息
├── Makefile             # 统一操作入口
├── .conform.yaml        # 目录结构校验
└── .golangci.yml        # 代码规范检查
```

---

## 🔮 路线图

code-bee 的核心已经稳定，未来计划扩展的方向包括：

- [ ] **多智能体支持** —— 除 Reasonix 外，接入 Qoder、Cline 等更多编码智能体
- [ ] **GitLab 适配** —— 将 Issue → MR 的调度能力扩展到 GitLab 平台
- [ ] **Webhook 触发** —— 支持 Issue 事件自动触发，无需手动执行 CLI
- [ ] **多 Agent 协作** —— 一个 Issue 中调度多个 Agent 协同完成复杂任务
- [ ] **校验管线** —— 可配置的 CI 校验步骤（lint → test → build → security scan）

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
