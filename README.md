# code-bee

蜂巢式 AI 编码调度器 —— 在 GitHub Issue 中 @agent-name 下达任务，Agent 自动执行编码、通过校验后提交 PR 并回复结果。基于 Reasonix × DeepSeek。

> Hive-style AI coding scheduler — @mention an agent in a GitHub Issue, it clones the repo, writes code, passes checks, and submits a PR.

## 使用方式

```bash
# 1. 设置 API Token
export GITHUB_TOKEN=ghp_xxx

# 2. 在 Issue 中 @agent-name，如 @agent-frontend 请重构组件

# 3. 执行
code-bee --repo JiGuangWorker/DeepSeek-Reasonix --issue 42
```

## Issue 模板

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

## 执行流程

```
读取 Issue → 解析 @agent-xxx → 回复 Issue → 启动 Agent → 
校验通过 → 推分支 + 创建 PR → 回复结果
```

## License

MIT
