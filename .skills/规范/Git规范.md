# Git 规范

> **适用角色：** 所有工程师
> **对标原则：** 《第一条 交付诚信原则》
> **版本：** v1.0

---

## 一、分支策略

采用 **Trunk-Based** 模式：`main` 为主线，功能在短分支上开发，快速合并。

```
main ─────────────────────────●──────●──────●────  (始终保持可发布)
        \                    /      /      /
feature/xxx ───●──●──●─────       /      /
                                  /      /
fix/xxx ────────●──●────────────       /
                                       /
release/x.x ─────────●──●────────────
```

### 分支命名

| 类型 | 格式 | 示例 | 说明 |
|------|------|------|------|
| 功能开发 | `feature/<模块>-<简述>` | `feature/user-login` | 新功能 |
| Bug 修复 | `fix/<模块>-<简述>` | `fix/order-status` | 修 Bug |
| 紧急修复 | `hotfix/<简述>` | `hotfix/payment-crash` | 线上 P0，从 main 拉 |
| 发布分支 | `release/<版本号>` | `release/v1.2.0` | 发布前冻结 |

> **命名规则：** 全小写，连字符分隔，动词+对象或模块+简述。不超过 40 个字符。

---

## 二、Commit Message 规范

### 格式

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Type（必填）

| Type | 说明 | 示例 |
|------|------|------|
| `feat` | 新功能 | `feat(user): add login API` |
| `fix` | Bug 修复 | `fix(order): correct status after refund` |
| `refactor` | 重构（不增功能不修 Bug） | `refactor(user): extract validation` |
| `perf` | 性能优化 | `perf(order): add index for query` |
| `test` | 测试相关 | `test(user): add login test cases` |
| `docs` | 文档 | `docs(api): update endpoint description` |
| `chore` | 构建/工具/依赖 | `chore(deps): bump gin to v1.9` |
| `style` | 格式（不影响逻辑） | `style(user): format with gofmt` |

### Scope（必填）

模块名或影响范围，全小写。例如：`user`、`order`、`auth`、`ci`、`deps`。

### Subject（必填）

- 用英文，50 字符以内
- 动词开头，现在时（add / fix / remove / update / refactor）
- 首字母小写，结尾不加句号

### Body（可选，建议）

说明 **做了什么、为什么这样做**，不写"怎么做"（代码自身说明）。72 字符换行。

### Footer（条件必填）

- 关联 Issue：`Closes #123` 或 `Refs #456`
- **破坏性变更必须标注：** `BREAKING CHANGE: <描述>`

### 示例

**简单提交：**
```
feat(user): add login API with JWT
```

**完整提交：**
```
fix(order): correct status transition after refund

The order status was stuck in 'refunding' after the refund
callback timed out. Added a timeout handler that rolls back
to the previous status.

Closes #234
```

**破坏性变更：**
```
refactor(user): change auth middleware signature

Adopts the new AuthContext interface across all handlers.

BREAKING CHANGE: AuthMiddleware now requires AuthProvider
instead of raw token string.
```

---

## 三、PR / Merge Request 流程

### 3.1 提 PR 前置条件

| 条件 | 检查方式 |
|------|----------|
| 单测全部通过 | CI 绿 |
| 目录结构校验通过 | `make check-structure` 通过 |
| 无遗留 TODO / 调试代码 | 自查 |
| 关联的 PRD 验收标准已覆盖 | 对照 PRD GWT |
| Commit 已 squash 为有意义的分组 | 不提交无意义的 "fix typo" × 10 |

### 3.2 PR 标题格式

```
<type>(<scope>): <简述>
```

与 Commit Message 格式一致。

### 3.3 PR 描述模板

```markdown
## 概述
一句话说清楚这个 PR 做了什么。

## 变更内容
- 变更点 1
- 变更点 2

## 关联
- PRD：[链接]
- Issue：Closes #123

## 测试
- [ ] 单测通过
- [ ] 手动验证了正常 + 异常路径
- [ ] 目录结构校验通过

## 截图（前端）
| Before | After |
|--------|-------|
| ![before](url) | ![after](url) |
```

### 3.4 评审规则

| 规则 | 说明 |
|------|------|
| ❗ 至少 1 人 Approve 才能 Merge | 禁止自批自合 |
| ❗ Review 24 小时内完成 | 超时升级到技术负责人 |
| ❗ PR 不超过 400 行变更 | 超过必须拆 PR |
| ❗ 禁止直接 push main | 所有变更必须走 PR |
| ❗ CI 不通过禁止 Merge | 红灯不能合 |

### 3.5 Code Review 检查清单

| 检查项 | 关注点 |
|--------|--------|
| 业务逻辑 | 是否正确覆盖 PRD 的 GWT？边界情况？异常处理？ |
| 编码规范 | 命名、error 处理、文件组织是否符合规范？ |
| AI 常见错误 | 魔法数字、any 类型、吞 error、过长函数？ |
| 测试覆盖 | 是否覆盖正常+异常+边界？ |
| 模块边界 | 是否跨模块直接调 repository？前端是否跨领域 import？ |
| 性能 | 有无 N+1 查询、未加索引、不必要的重渲染？ |

### 3.6 PR 驳回处理

PR 可能因以下原因被驳回（Request Changes 或直接 Close）：

| 驳回原因 | 说明 | 后续动作 |
|----------|------|----------|
| 方案设计有误 | 技术方案走不通或违背架构原则 | 与 Reviewer 重新讨论方案，必要时拉技术负责人介入 |
| 理解偏差 | 对需求理解有误，做出来的不是 PRD 描述的功能 | 重新对齐 PRD，确认 GWT 后重写 |
| 质量不达标 | 缺少测试、异常处理、或存在明显 Bug | 补齐后重新提交 |
| 范围越界 | PR 做了超出功能边界的事（如顺手重构了无关模块） | 拆分 PR，越界部分另起 PR 或移除 |

> **驳回后处理流程：**
> 1. 先与 Reviewer 当面/文字确认驳回原因，不要猜
> 2. 需要重新设计的：先出方案简稿，Reviewer 确认后再写代码
> 3. 修正后在原分支继续开发、commit、push → PR 自动更新
> 4. 如果原分支历史已乱，关闭原 PR，重新从 main 拉分支、重新开发

> ❗ **驳回不是否定你，是否定这次的实现方式。** 不要有心理负担，清楚原因后重来即可。

---

## 四、合并策略

| 分支流向 | 合并方式 | 说明 |
|----------|----------|------|
| `feature/*` → `main` | **Squash Merge** | 将一个功能的所有 commit 压成 1 个，保持 main 干净 |
| `hotfix/*` → `main` | **Squash Merge** | 同上 |
| `main` → `release/*` | Cherry-pick 或 Merge | 从 main 摘取需要的 commit 到发布分支 |

> ❗ **禁止在 main 上直接 commit。禁止 force push main。**

---

## 五、版本号

采用 [Semantic Versioning](https://semver.org/)：`MAJOR.MINOR.PATCH`

| 类型 | 说明 | 示例 |
|------|------|------|
| MAJOR | 破坏性变更（不兼容的 API 修改） | `1.0.0` → `2.0.0` |
| MINOR | 向后兼容的新功能 | `1.0.0` → `1.1.0` |
| PATCH | 向后兼容的 Bug 修复 | `1.0.0` → `1.0.1` |

---

## 六、Tag 规范

发布时在 main 上打 Tag：

```bash
git tag -a v1.2.0 -m "feat: add refund module"
git push origin v1.2.0
```

Tag 名称：`v<版本号>`，附注简要说明本次发布内容。

---

## 七、仓库配置

### .gitignore 模板

```gitignore
# Go
*.exe
*.test
*.out
vendor/
/tmp/

# Node
node_modules/
dist/
.env.local

# IDE
.idea/
.vscode/
*.swp

# OS
.DS_Store
Thumbs.db
```
