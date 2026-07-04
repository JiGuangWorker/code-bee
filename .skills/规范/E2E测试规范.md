# E2E 测试规范（基于 Mock）

> **适用角色：** 前端工程师 + 后端工程师
> **适用技术：** Playwright + Go Mock 服务
> **对标原则：** 《第一条 交付诚信原则》
> **关联规范：** [后端单元测试规范](后端单元测试规范.md)、[前端单元测试规范](前端单元测试规范.md)、[编码规范](编码规范.md)
> **版本：** v1.0

---

## 一、定位

本规范定义**基于 Mock 实现的端到端（E2E）自动化测试标准**。核心思路：

```
接口定义（Go interface）
    ├── 真实实现 → 生产环境
    └── Mock 实现 → E2E 测试环境
                         ↓
               Playwright 浏览器自动化
                   测试前端页面 + Mock 后端服务
```

> **不需要真实数据库、不需要真实后端、不需要外部依赖。** 前端是真实的，后端用 Mock 服务替代。

---

## 二、为什么基于 Mock

| 对比 | 传统 E2E（真实后端） | Mock E2E |
|------|---------------------|----------|
| 启动成本 | 需要数据库、Redis、外部 API | 零依赖，Mock 服务在进程内启动 |
| 测试速度 | 慢（DB 读写 + 网络） | 快（内存级 Mock） |
| 数据污染 | 需要清理/重置 | 无，每次测试独立启动 |
| 稳定性 | 受外部服务波动影响 | 完全可控，不受外部影响 |
| 覆盖场景 | 仅限正常路径 | 可精确模拟各种异常（超时、错误码、并发冲突） |

> Mock E2E 验证的是**前端行为正确性**——页面渲染、组件联动、用户流程、错误处理。后端逻辑的正确性由后端的单元测试保证。

---

## 三、前置条件：接口分离

Mock E2E 能跑起来的前提是——**后端代码中接口与实现分离**。

```
internal/module/order/
├── service/
│   ├── service.go           # 定义 OrderService 接口 + 真实实现
│   └── service_test.go
├── repository/
│   ├── repository.go        # 定义 Repository 接口
│   ├── mysql.go             # MySQL 实现
│   └── repository_test.go
└── ...
```

```go
// service/service.go —— 调用方定义接口
package service

type Service interface {
    ListOrders(ctx context.Context, status string) ([]model.Order, error)
    GetOrder(ctx context.Context, id string) (*model.Order, error)
    ApproveOrder(ctx context.Context, id string) error
}

type serviceImpl struct {
    repo repository.Repository
}

func NewService(repo repository.Repository) Service {
    return &serviceImpl{repo: repo}
}
```

> 接口由调用方定义，真实实现和 Mock 实现是同一接口的两个实例。这使得 E2E 测试可以**无缝替换**底层服务。

---

## 四、Mock 服务搭建

### 4.1 创建 Mock 实现

```go
// backend/tests/e2e/mock/mock_services.go
package mock

import (
    "context"
    "project/internal/module/order/service"
    "project/internal/module/order/model"
)

// MockOrderService 实现 service.Service 接口
type MockOrderService struct {
    Orders map[string]*model.Order
}

func (m *MockOrderService) ListOrders(ctx context.Context, status string) ([]model.Order, error) {
    var result []model.Order
    for _, o := range m.Orders {
        if status == "" || o.Status == status {
            result = append(result, *o)
        }
    }
    return result, nil
}

func (m *MockOrderService) GetOrder(ctx context.Context, id string) (*model.Order, error) {
    o, ok := m.Orders[id]
    if !ok {
        return nil, ErrNotFound
    }
    return o, nil
}

func (m *MockOrderService) ApproveOrder(ctx context.Context, id string) error {
    o, ok := m.Orders[id]
    if !ok {
        return ErrNotFound
    }
    if o.Status == "approved" {
        return ErrAlreadyApproved // 模拟竞争条件
    }
    o.Status = "approved"
    return nil
}

// 测试数据工厂
func NewMockOrderService() *MockOrderService {
    return &MockOrderService{
        Orders: map[string]*model.Order{
            "1001": {ID: "1001", Status: "pending", Customer: "张三"},
            "1002": {ID: "1002", Status: "approved", Customer: "李四"},
            "1003": {ID: "1003", Status: "pending", Customer: "王五"},
        },
    }
}
```

### 4.2 启动带 Mock 服务的测试 Server

```go
// backend/tests/e2e/server.go
package e2e

import (
    "net/http"
    "testing"
    "project/backend/tests/e2e/mock"
    "project/internal/router"
)

// StartMockServer 启动 HTTP Server，所有业务服务使用 Mock 实现
func StartMockServer(t *testing.T) string {
    t.Helper()

    mockOrderSvc := mock.NewMockOrderService()
    // mockUserSvc := mock.NewMockUserService()   // 其他模块同理
    // mockPaymentSvc := mock.NewMockPaymentService()

    // 用 Mock 服务构建路由
    r := router.Setup(router.Deps{
        OrderService:   mockOrderSvc,
        // UserService:    mockUserSvc,
        // PaymentService: mockPaymentSvc,
    })

    srv := httptest.NewServer(r)
    t.Cleanup(srv.Close)
    return srv.URL
}
```

### 4.3 路由支持依赖注入

```go
// internal/router/router.go —— 生产环境和测试环境共用
package router

type Deps struct {
    OrderService   order.Service
    // ... 其他服务
}

func Setup(deps Deps) http.Handler {
    mux := http.NewServeMux()
    // 注入真实或 Mock 服务
    orderHandler := order_handler.NewHandler(deps.OrderService)
    mux.HandleFunc("/api/orders/", orderHandler.ServeHTTP)
    return mux
}
```

> **关键设计**：路由层接收接口依赖，不关心是真实实现还是 Mock 实现。测试时传 Mock，生产时传真实——同一套路由代码。

---

## 五、前端 E2E 目录结构

```
frontend/
└── tests/
    ├── e2e/                          # Playwright 流程测试
    │   ├── return-order.spec.ts       #   退货审批完整流程
    │   ├── order-list.spec.ts         #   工单列表操作
    │   └── create-order.spec.ts       #   创建工单流程
    ├── visual/                        # 截图基线
    ├── page-objects/                  # Page Object 封装
    │   ├── login.po.ts
    │   └── order-list.po.ts
    ├── fixtures/                      # 测试数据工厂（前端侧）
    └── playwright.config.ts           # Playwright 配置
```

---

## 六、Playwright 配置

```typescript
// frontend/tests/playwright.config.ts
import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  timeout: 30000,
  retries: 1,
  use: {
    baseURL: 'http://localhost:5173',
    screenshot: 'only-on-failure',
    testIdAttribute: 'data-testid',
  },
  snapshotDir: './visual',

  // 关键：启动前端 dev server + Mock 后端 server
  webServer: [
    {
      // 1. 启动 Mock 后端
      command: 'cd ../backend && go run ./tests/e2e/mock-server.go',
      port: 8080,
      reuseExistingServer: false,
    },
    {
      // 2. 启动前端
      command: 'pnpm --filter @project/app dev',
      port: 5173,
      reuseExistingServer: false,
    },
  ],
});
```

---

## 七、E2E 测试写法

### 7.1 页面流程测试

```typescript
// frontend/tests/e2e/return-order.spec.ts
import { test, expect } from '@playwright/test';

test('标准退货工单审批——完整流程', async ({ page }) => {
  // 1. 登录
  await page.goto('/login');
  await page.fill('[data-testid="username"]', 'agent01');
  await page.fill('[data-testid="password"]', 'test123');
  await page.click('[data-testid="login-btn"]');

  // 2. 验证进入工单列表
  await expect(page).toHaveURL(/\/orders/);
  await expect(page.locator('[data-testid="order-list"]')).toBeVisible();

  // 3. Mock 服务有 #1001 状态为 pending，验证列表显示
  const orderRow = page.locator('[data-testid="order-row"]')
    .filter({ hasText: '#1001' });
  await expect(orderRow.locator('[data-testid="status-badge"]'))
    .toHaveText('待审批');

  // 4. 点击进入详情
  await orderRow.click();
  await expect(page.locator('[data-testid="approve-btn"]')).toBeEnabled();

  // 5. 审批通过
  await page.click('[data-testid="approve-btn"]');

  // 6. 验证结果
  await expect(page.locator('[data-testid="toast-success"]'))
    .toContainText('审批成功');
  await expect(page.locator('[data-testid="status-badge"]'))
    .toHaveText('已通过');
});

test('审批失败——工单已被他人审批', async ({ page }) => {
  // Mock 服务中的 #1002 是 approved 状态
  await page.goto('/orders/1002');
  await page.click('[data-testid="approve-btn"]');

  await expect(page.locator('[data-testid="toast-error"]'))
    .toContainText('该工单已被处理');
  await expect(page.locator('[data-testid="status-badge"]'))
    .toHaveText('已通过'); // 状态未变
});
```

### 7.2 组件联动校验

```typescript
test('售后类型联动：选择退货类型后下拉原因仅显示退货相关', async ({ page }) => {
  await page.goto('/orders/create');

  // 选择售后类型 = "退货退款"
  await page.selectOption('[data-testid="after-sale-type"]', 'return');

  // 打开原因下拉
  await page.click('[data-testid="reason-select"]');
  const options = page.locator('[data-testid="reason-option"]');

  // 断言：只包含退货相关原因
  await expect(options).toHaveCount(3);
  await expect(options.nth(0)).toHaveText('质量问题');
  await expect(options.nth(1)).toHaveText('商品与描述不符');
  await expect(options.nth(2)).toHaveText('发错货');

  // 断言：不包含仅退款原因
  await expect(options.filter({ hasText: '不想要了' })).toHaveCount(0);
});

test('切换售后类型后原因下拉即时更新', async ({ page }) => {
  await page.goto('/orders/create');

  // 先选"仅退款"
  await page.selectOption('[data-testid="after-sale-type"]', 'refund-only');
  await page.click('[data-testid="reason-select"]');
  await expect(page.locator('[data-testid="reason-option"]'))
    .toContainText('不想要了');

  // 切换到"退货退款"
  await page.selectOption('[data-testid="after-sale-type"]', 'return');
  await page.click('[data-testid="reason-select"]');

  // 验证联动：不应出现"不想要了"
  await expect(page.locator('[data-testid="reason-option"]'))
    .not.toContainText('不想要了');
  await expect(page.locator('[data-testid="reason-option"]'))
    .toContainText('质量问题');
});
```

### 7.3 视觉回归

```typescript
test('工单列表页视觉回归', async ({ page }) => {
  await page.goto('/orders');
  await expect(page.locator('[data-testid="order-list"]')).toBeVisible();
  await expect(page).toHaveScreenshot('order-list.png', { fullPage: true });
});
```

---

## 八、Page Object 模式

> 重复的选择器和操作封装为 Page Object，不散落各测试。

```typescript
// frontend/tests/page-objects/login.po.ts
import { Page, expect } from '@playwright/test';

export class LoginPage {
  constructor(private page: Page) {}

  async goto() {
    await this.page.goto('/login');
  }

  async login(username: string, password: string) {
    await this.page.fill('[data-testid="username"]', username);
    await this.page.fill('[data-testid="password"]', password);
    await this.page.click('[data-testid="login-btn"]');
  }

  async expectLoginSuccess() {
    await expect(this.page).toHaveURL(/\/orders/);
  }
}
```

---

## 九、强制规则

| 规则 | 说明 |
|------|------|
| ❗ E2E 测试使用 Mock 后端 | 不连真实数据库、不连外部 API |
| ❗ Mock 实现与真实实现共享同一接口 | 接口由调用方定义，Mock 是实现方的另一个实例 |
| ❗ 路由层支持依赖注入 | 测试和生产的唯一区别是注入的依赖不同 |
| ❗ E2E 测试按业务流程拆分 | 不按页面拆分，一个文件一条用户路径 |
| ❗ 组件必须挂 `data-testid` | Playwright 首选定位策略 |
| ❗ 每个测试独立，不依赖执行顺序 | Mock 服务每次独立初始化 |
| ❗ 统一 Playwright，禁用其他 E2E 框架 | Cypress、Selenium 等禁止引入 |
| ❗ 测试通过 Makefile 统一入口 | `make test-e2e` |

---

## 十、Makefile 入口

```makefile
# E2E 测试（启动 Mock 后端 + 前端 dev server）
test-e2e:
	cd frontend && npx playwright test --config tests/playwright.config.ts

# 视觉回归基线更新
test-e2e-visual-update:
	cd frontend && npx playwright test --config tests/playwright.config.ts --update-snapshots
```
