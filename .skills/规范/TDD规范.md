# TDD 规范（测试驱动开发）

> **适用角色：** 所有工程师
> **对标原则：** 《第一条 交付诚信原则》
> **版本：** v1.0

---

## 一、TDD 的核心原则

> **先写测试，再写实现。测试不通过，代码不算完。**

TDD 不是"写了代码再补测试"，而是**用测试驱动设计**。

---

## 二、TDD 三步循环

```
Red  →  写一个失败的测试（描述期望行为）
Green →  写最少代码让测试通过
Refactor →  重构代码，保持测试通过
```

| 阶段 | 做什么 | 禁止做什么 |
|------|--------|------------|
| **Red** | 写测试用例，运行确认失败 | 不能还没红就写实现 |
| **Green** | 写最简单实现让测试通过 | 不能过度设计，测试通过就停 |
| **Refactor** | 消除重复、优化结构 | 不能改行为，所有已有测试必须仍绿 |

---

## 三、Go 后端测试规范

### 3.1 测试文件组织

```
backend/internal/
├── service/
│   ├── user.go
│   └── user_test.go          # 与源文件同目录
├── handler/
│   ├── user.go
│   └── user_test.go
└── repository/
    ├── user_repo.go
    └── user_repo_test.go
```

> 测试文件与源文件同目录，命名规则：`[源文件名]_test.go`

---

### 3.2 测试分层与工具

| 层级 | 测试类型 | 工具 | Mock 策略 |
|------|----------|------|-----------|
| Handler | HTTP 集成测试 | `httptest` | Mock Service |
| Service | 单元测试 | `testing` + `testify` | Mock Repository |
| Repository | 数据访问测试 | `sqlmock` / 测试数据库 | Mock DB 或 用 Docker 起真实 DB |

### 3.3 单元测试命名

```
func Test[函数名]_[场景]_[期望结果](t *testing.T)
```

**示例：**

```go
func TestCreateUser_ValidInput_ReturnsUser(t *testing.T) {
    // Red: 定义期望
    // Green: 调用实现
    // Assert: 验证结果
}

func TestCreateUser_DuplicateEmail_ReturnsError(t *testing.T) { }

func TestCreateUser_EmptyName_ReturnsValidationError(t *testing.T) { }
```

---

### 3.4 测试结构：AAA 模式

每个测试函数按 **Arrange-Act-Assert** 三段式组织：

```go
func TestApproveOrder_ValidOrder_UpdatesStatus(t *testing.T) {
    // Arrange（准备数据）
    mockRepo := new(MockOrderRepo)
    svc := NewOrderService(mockRepo)
    order := &Order{ID: 1, Status: "pending"}

    // Act（执行操作）
    err := svc.Approve(order.ID)

    // Assert（验证结果）
    assert.NoError(t, err)
    assert.Equal(t, "approved", order.Status)
}
```

---

### 3.5 覆盖率要求

| 层级 | 最低覆盖率 | 说明 |
|------|-----------|------|
| Service 层 | ≥ 80% | 核心业务逻辑必须高覆盖 |
| Handler 层 | ≥ 60% | 至少覆盖正常+异常路径 |
| Repository 层 | ≥ 50% | SQL 至少覆盖 CRUD 主流程 |

> **覆盖率不是目的，但不能低于底线。** 低于底线的代码不得合并。

### 3.6 必测场景清单

对每个函数，至少覆盖以下场景：

| 场景类别 | 说明 | 示例 |
|----------|------|------|
| ✅ 正常路径 | 输入正确，返回预期结果 | 创建用户成功 |
| ❌ 参数异常 | 空值、超长、非法格式 | 邮箱格式错误 |
| ❌ 业务异常 | 重复数据、状态不符 | 重复邮箱注册 |
| ❌ 边界值 | 最大值、最小值、零值 | 分页第 0 页 |
| ❌ 依赖异常 | DB 挂了、超时 | 数据库连接失败 |

---

## 四、React 前端测试规范

### 4.1 测试文件组织

```
frontend/src/
├── components/
│   └── Button/
│       ├── index.tsx
│       ├── style.module.css
│       └── index.test.tsx      # 与组件同目录
├── hooks/
│   ├── useAuth.ts
│   └── useAuth.test.ts        # 与 Hook 同目录
└── utils/
    ├── format.ts
    └── format.test.ts
```

---

### 4.2 测试分层与工具

| 层级 | 测试类型 | 工具 |
|------|----------|------|
| 组件 | 渲染 + 交互测试 | `vitest` + `@testing-library/react` |
| Hook | 逻辑测试 | `vitest` + `renderHook` |
| 工具函数 | 单元测试 | `vitest` |
| API 层 | 接口 Mock 测试 | `vitest` + `msw` |

---

### 4.3 组件测试规范

```tsx
// 测试命名：describe([组件名]) + it([场景])
describe('Button', () => {
  // 渲染测试
  it('renders with correct label', () => {
    render(<Button label="提交" />);
    expect(screen.getByText('提交')).toBeInTheDocument();
  });

  // 交互测试
  it('calls onClick when clicked', async () => {
    const onClick = vi.fn();
    render(<Button label="提交" onClick={onClick} />);
    await userEvent.click(screen.getByRole('button'));
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  // 状态测试
  it('disables button when loading', () => {
    render(<Button label="提交" loading />);
    expect(screen.getByRole('button')).toBeDisabled();
  });
});
```

### 4.4 前端必测场景

| 组件类型 | 必测场景 |
|----------|----------|
| 展示组件 | 正常渲染、空数据、加载态、错误态 |
| 表单组件 | 输入、校验、提交、提交失败重试 |
| 列表组件 | 有数据、空列表、分页、搜索过滤 |
| Hook | 正常逻辑、异常处理、边界值 |

---

## 五、TDD 执行纪律

| 规则 | 说明 |
|------|------|
| ❗ Red 没看到失败之前，不准写实现 | 提交前确认测试确实失败（不是语法错误） |
| ❗ 测试不通过，代码不得提交 | CI 管线必须跑测试，失败 = 阻断合并 |
| ❗ 测试必须可重复运行 | 不依赖执行顺序、不依赖外部状态、不依赖时间 |
| ❗ Bug 修复必须先写复现测试 | 能复现 Bug 的测试 → 修复代码 → 测试通过 |
| ❗ 禁止注释掉失败的测试 | 要么修代码让测试通过，要么确认测试写错了再改测试 |

---

## 六、CI 测试流水线

```
代码推送 → 自动跑单元测试 → 生成覆盖率报告 → 检查覆盖率阈值 → 通过=允许合并
```

> 任何提交导致已有测试变红，**必须立即修复**，不得积压。

---

## 七、测试代码质量标准

测试代码与生产代码同等质量要求：

| 规则 | 说明 |
|------|------|
| 一个测试只测一件事 | 禁止一个测试函数里 assert 多个不相关的行为 |
| 测试名称要自解释 | 看测试名就知道测什么场景，不依赖注释 |
| 避免测试间共享可变状态 | 每个测试独立准备数据，不依赖执行顺序 |
| 不测试第三方库的行为 | 只测自己的逻辑，不测框架或库的功能 |
