# 后端 Mock 与分层测试规范

> **适用角色：** 后端工程师
> **适用语言：** Go
> **对标原则：** 《第一条 交付诚信原则》
> **关联规范：** [编码规范](编码规范.md)、[后端单元测试规范](后端单元测试规范.md)、[E2E测试规范](E2E测试规范.md)
> **版本：** v1.0

---

## 一、定位

[后端单元测试规范](后端单元测试规范.md) 解决**"测什么"**——方法级覆盖、场景覆盖。本规范解决**"怎么 Mock"**——三层各自如何隔离依赖，使每个方法能独立测试。

```
后端单元测试规范    →   测什么：每个方法至少一个测试
后端Mock与分层测试规范 →   怎么测：三层各用什么 Mock 策略
E2E测试规范         →   怎么端到端验证：Mock 服务 + Playwright
```

---

## 二、核心前提：接口优先

> **接口由调用方定义，Mock 是对接口的另一个实现。**

```go
// ✅ service.go —— 调用方定义接口
package service

type Repository interface {
    FindByID(ctx context.Context, id string) (*model.Order, error)
    Create(ctx context.Context, order *model.Order) error
}

// 真实构造器接收接口
func NewService(repo Repository) *Service {
    return &Service{repo: repo}
}
```

有了接口，Mock 就只是接口的另一个实现——与真实实现的唯一区别是：Mock 返回预设数据，不连数据库。

---

## 三、三层 Mock 策略总览

```
Handler 层  ──Mock──▶  Service 层  ──Mock──▶  Repository 层  ──Mock──▶  Database
   │                      │                      │
   │ Mock 工具：          │ Mock 工具：          │ Mock 工具：
   │ testify/mock         │ testify/mock         │ sqlmock
   │                      │                      │
   │ 验证：参数校验       │ 验证：业务逻辑       │ 验证：SQL 正确性
   │       HTTP 状态码    │       状态变更       │       结果映射
   │       响应体格式     │       错误传递       │       错误处理
```

> 每一层只 Mock 它的**直接依赖**，不跨层 Mock。Handler Mock Service，不 Mock Repository。

---

## 四、Service 层 Mock

> **Mock Repository，验证业务逻辑。**

```go
// internal/module/order/service/service_test.go
package service

import (
    "context"
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
)

// ==========================================
// Step 1：定义 Mock Repository
// ==========================================
type MockOrderRepo struct {
    mock.Mock
}

func (m *MockOrderRepo) FindByID(ctx context.Context, id string) (*model.Order, error) {
    args := m.Called(ctx, id)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*model.Order), args.Error(1)
}

func (m *MockOrderRepo) Create(ctx context.Context, order *model.Order) error {
    args := m.Called(ctx, order)
    return args.Error(0)
}

// ==========================================
// Step 2：用 Mock 编写测试
// ==========================================
func TestApprove_ValidOrder_UpdatesStatus(t *testing.T) {
    mockRepo := new(MockOrderRepo)
    order := &model.Order{ID: "1001", Status: "pending"}

    // 预设 Mock 行为
    mockRepo.On("FindByID", mock.Anything, "1001").Return(order, nil)
    mockRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

    svc := NewService(mockRepo)

    err := svc.Approve(context.Background(), "1001")

    assert.NoError(t, err)
    assert.Equal(t, "approved", order.Status)
    mockRepo.AssertExpectations(t) // 验证 Mock 方法被正确调用
}

func TestApprove_AlreadyApproved_ReturnsError(t *testing.T) {
    mockRepo := new(MockOrderRepo)
    order := &model.Order{ID: "1001", Status: "approved"}
    mockRepo.On("FindByID", mock.Anything, "1001").Return(order, nil)

    svc := NewService(mockRepo)

    err := svc.Approve(context.Background(), "1001")
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "已被处理")
}
```

| 规则 | 说明 |
|------|------|
| ❗ Mock 结构体命名：`Mock` + 接口名 | `MockOrderRepo`、`MockPaymentService` |
| ❗ 使用 `testify/mock` | 统一 Mock 框架，禁止手写假结构体 |
| ❗ `On().Return()` 预设行为 | 在 Arrange 阶段设置，不在 Act 后修改 |
| ❗ `AssertExpectations(t)` 结尾必调 | 验证所有预设的 Mock 方法都被调用 |
| ❗ Service 不连真实 DB | 100% Mock，不启动数据库 |

---

## 五、Handler 层 Mock

> **Mock Service，用 `httptest` 发 HTTP 请求，验证参数校验和响应格式。**

```go
// internal/module/order/handler/handler_test.go
package handler

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
)

// Mock Service
type MockOrderService struct {
    mock.Mock
}

func (m *MockOrderService) Approve(ctx context.Context, id string) error {
    args := m.Called(ctx, id)
    return args.Error(0)
}

// ========== 正常路径 ==========
func TestApproveHandler_ValidRequest_Returns200(t *testing.T) {
    mockSvc := new(MockOrderService)
    mockSvc.On("Approve", mock.Anything, "1001").Return(nil)
    handler := NewHandler(mockSvc)

    req := httptest.NewRequest("POST", "/orders/1001/approve", nil)
    w := httptest.NewRecorder()
    handler.Approve(w, req)

    assert.Equal(t, http.StatusOK, w.Code)
}

// ========== 参数校验（不依赖 Service）==========
func TestApproveHandler_EmptyID_Returns400(t *testing.T) {
    handler := NewHandler(nil) // 参数校验不需要 Service

    req := httptest.NewRequest("POST", "/orders//approve", nil)
    w := httptest.NewRecorder()
    handler.Approve(w, req)

    assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ========== 业务错误传递 ==========
func TestApproveHandler_ServiceError_Returns500(t *testing.T) {
    mockSvc := new(MockOrderService)
    mockSvc.On("Approve", mock.Anything, "1001").Return(assert.AnError)
    handler := NewHandler(mockSvc)

    req := httptest.NewRequest("POST", "/orders/1001/approve", nil)
    w := httptest.NewRecorder()
    handler.Approve(w, req)

    assert.Equal(t, http.StatusInternalServerError, w.Code)
}
```

| 规则 | 说明 |
|------|------|
| ❗ Handler 测试用 `httptest` | 不启动真实 HTTP Server |
| ❗ 参数校验用例不传 Mock Service | 传 `nil` 证明校验在 Service 调用之前 |
| ❗ 响应体格式必须断言 | 不只断言状态码，JSON body 也要验证 |
| ❗ Handler 不 Mock Repository | 只 Mock 直接依赖 Service |

---

## 六、Repository 层 Mock

> **用 `sqlmock` 模拟数据库，验证 SQL 语句正确性和结果映射。**

```go
// internal/module/order/repository/mysql_test.go
package repository

import (
    "context"
    "database/sql"
    "testing"

    "github.com/DATA-DOG/go-sqlmock"
    "github.com/stretchr/testify/assert"
)

// ========== 查询成功 ==========
func TestFindByID_ExistingOrder_ReturnsOrder(t *testing.T) {
    db, mockDB, _ := sqlmock.New()
    defer db.Close()

    // 预设 SQL 期望
    rows := sqlmock.NewRows([]string{"id", "status"}).
        AddRow("1001", "pending")
    mockDB.ExpectQuery("SELECT (.+) FROM orders WHERE id = ?").
        WithArgs("1001").
        WillReturnRows(rows)

    repo := NewMySQLRepo(db)

    order, err := repo.FindByID(context.Background(), "1001")
    assert.NoError(t, err)
    assert.Equal(t, "1001", order.ID)
    assert.Equal(t, "pending", order.Status)

    // 验证所有期望的 SQL 都被执行
    assert.NoError(t, mockDB.ExpectationsWereMet())
}

// ========== 查询不到 ==========
func TestFindByID_NotFound_ReturnsError(t *testing.T) {
    db, mockDB, _ := sqlmock.New()
    defer db.Close()

    mockDB.ExpectQuery("SELECT (.+) FROM orders WHERE id = ?").
        WithArgs("9999").
        WillReturnError(sql.ErrNoRows)

    repo := NewMySQLRepo(db)

    _, err := repo.FindByID(context.Background(), "9999")
    assert.Error(t, err)
}

// ========== 插入成功 ==========
func TestCreate_ValidOrder_InsertsRow(t *testing.T) {
    db, mockDB, _ := sqlmock.New()
    defer db.Close()

    mockDB.ExpectExec("INSERT INTO orders").
        WithArgs("1001", "pending").
        WillReturnResult(sqlmock.NewResult(1, 1))

    repo := NewMySQLRepo(db)

    err := repo.Create(context.Background(), &model.Order{ID: "1001", Status: "pending"})
    assert.NoError(t, err)
    assert.NoError(t, mockDB.ExpectationsWereMet())
}
```

| 规则 | 说明 |
|------|------|
| ❗ Repository 用 `sqlmock` | 不连真实数据库 |
| ❗ 必须验证 SQL 模板 + 参数 | `ExpectQuery` / `ExpectExec` + `WithArgs` |
| ❗ `ExpectationsWereMet()` 结尾必调 | 验证所有预期的 SQL 都被执行 |
| ❗ 覆盖 CRUD 全部场景 | 正常 + 找不到 + 写失败 |

---

## 七、强制规则

| 规则 | 说明 |
|------|------|
| ❗ 接口必须先于实现定义 | 调用方定义接口，Mock 和真实实现是同一接口的两个实例 |
| ❗ 统一 `testify/mock` | 禁止手写假结构体 |
| ❗ Service 层 Mock Repository | 不连真实 DB |
| ❗ Handler 层 Mock Service | 用 httptest，不启动真实 Server |
| ❗ Repository 层用 `sqlmock` | 不连真实 DB |
| ❗ 每层只 Mock 直接依赖 | Handler 不 Mock Repository |
| ❗ `AssertExpectations` / `ExpectationsWereMet` 结尾必调 | 验证依赖调用正确 |

---

## 八、Makefile 入口

```makefile
test-backend-unit:
	cd backend && go test ./internal/... -short -count=1 -coverprofile=coverage.out
```

> 单元测试统一入口，包含三层所有 Mock 测试。
