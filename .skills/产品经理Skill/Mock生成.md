# Mock 生成

> 当 PRD 编写完成（阶段 6 输出完整 PRD 后），或对方说"PRD 写完了，生成 Mock"时使用。将第 4 节的 GWT 场景自动映射为 API Mock 定义。

---

## 角色行为

你的目标是把 PRD 中的每个 GWT 场景翻译成 **OpenAPI 3.0 规范文件**（YAML 格式）。**PRD 定稿 = Mock 就绪**——生成的 `openapi.yaml` 可直接导入 API Fox / Swagger / Postman 等工具，前后端无需等待后端完成即可并行开发。

---

## 工作流程

### 第 1 步：确认 PRD 已定稿

在生成 Mock 之前，确认对方：
- PRD 7 个部分全部填写完成？
- PM 和工程师都已确认 GWT 内容？
- 功能范围（Must/Should/Won't）已划定？

如果有遗漏，让他先回 [PRD编写](PRD编写.md) 补全。

### 第 2 步：遍历 GWT，逐场景映射

针对 PRD 第 4 节的每个 GWT 场景，映射为 OpenAPI path 条目。

映射时遵循以下结构：

```yaml
paths:
  /api/v1/[资源名]/[动作]:
    [method（post/get/put/delete）]:
      summary: "[场景的一句话描述]"
      description: "关联 GWT 场景：{场景编号} {场景名称}"
      parameters:         # 基于 Given 中的路径/查询参数
        - name: ...
          in: path / query
          required: true
          schema:
            type: string
      requestBody:        # 基于 Given 中的请求体数据
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                ...
      responses:          # 基于 Then 的预期结果
        '200':
          description: "正常响应描述"
          content:
            application/json:
              schema:
                ...
```

**Method 推断规则：**
- When 是"查询/查看/获取" → GET
- When 是"新增/创建/提交" → POST
- When 是"修改/更新/编辑" → PUT 或 PATCH
- When 是"删除/移除" → DELETE

### 第 3 步：GWT → OpenAPI Schema 映射规则

| GWT 要素 | OpenAPI 映射位置 | 映射方法 |
|----------|-----------------|----------|
| Given（前置数据/状态） | `parameters` + `requestBody` | 路径参数 → parameters；业务数据 → requestBody schema |
| When（用户操作） | HTTP Method + Path | 动作类型 → Method；操作对象 → Path |
| Then（预期结果） | `responses` → `schema` | 系统层变化 → data 字段；用户层变化 → msg/description |

**映射示例：**

```
场景 1（正常审批）：
Given：工单 #1001 状态为"待审批"，属于标准退货类型
When：客服点击"审批通过"按钮
Then：工单状态更新为"已通过"；生成退货单号；列表刷新
```

→ 映射为 OpenAPI path：

```yaml
  /api/v1/tickets/{ticket_id}/approve:
    post:
      summary: "审批工单"
      description: "关联 GWT 场景：场景 1（正常审批）"
      parameters:
        - name: ticket_id
          in: path
          required: true
          description: "工单ID"
          schema:
            type: string
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [ticket_id]
              properties:
                ticket_id:
                  type: string
                  description: "工单ID"
                  example: "1001"
      responses:
        '200':
          description: "审批成功，生成退货单号"
          content:
            application/json:
              schema:
                type: object
                properties:
                  code:
                    type: integer
                    example: 0
                  msg:
                    type: string
                    example: "审批成功"
                  data:
                    type: object
                    properties:
                      ticket_id:
                        type: string
                        example: "1001"
                      status:
                        type: string
                        example: "approved"
                      return_ticket_no:
                        type: string
                        example: "RT20260627001"
```

**Schema 类型推断规则：**

根据 Then 中的字段内容推断 JSON Schema type：
- ID/编号类 → `type: string`，加 `example`
- 状态/枚举类 → `type: string`，加 `enum: [...]`
- 数量/金额类 → `type: integer` 或 `type: number`
- 是/否类 → `type: boolean`
- 时间日期类 → `type: string, format: date-time`

### 第 4 步：异常场景映射错误响应

每个异常场景在 OpenAPI path 的 `responses` 中生成对应的错误响应条目：

| 异常类型 | HTTP 状态码 | 响应体结构 |
|----------|------------|-----------|
| 数据已被他人修改 | `'409'` | `code`（业务错误码）+ `msg`（提示文案：已处理/请刷新） |
| 权限不足 | `'403'` | `code` + `msg`（提示文案：无操作权限） |
| 数据不存在 | `'404'` | `code` + `msg`（提示文案：数据不存在/已删除） |
| 参数校验失败 | `'422'` | `code` + `msg`（提示文案：参数校验不通过） |
| 服务异常 | `'500'` | `code` + `msg`（提示文案：服务异常，请稍后再试） |

**示例：**

```
场景 2（异常：已被他人审批）：
Given：工单状态为"已通过"
When：客服点击"审批通过"
Then：页面提示"该工单已被处理，请刷新列表"
```

→ 添加到同一个 path 的 `responses` 下：

```yaml
        '409':
          description: "工单已被其他客服处理"
          content:
            application/json:
              schema:
                type: object
                properties:
                  code:
                    type: integer
                    example: 10001
                  msg:
                    type: string
                    example: "该工单已被处理，请刷新列表"
                  data:
                    type: object
                    nullable: true
                    example: null
```

**强制要求：每个接口至少包含 5 个 4xx/5xx 响应（409 / 403 / 404 / 422 / 500），不足的用通用兜底响应补充，引用 `$ref: '#/components/schemas/ErrorResponse'`。**

### 第 5 步：汇总输出 OpenAPI 文件

将所有 path 条目汇总为一个完整的 OpenAPI 3.0 YAML 文件：

```yaml
openapi: 3.0.3
info:
  title: "[项目名称] API"
  description: "来源：[项目名称] PRD v1.0 | 生成日期：[日期]"
  version: "1.0.0"

servers:
  - url: http://localhost:8080
    description: "本地开发 Mock 服务"

paths:
  # ===== 第 2 步 + 第 3 步：正常场景 =====
  /api/v1/tickets/{ticket_id}/approve:
    post:
      summary: "审批工单"
      description: "关联 GWT 场景：场景 1（正常审批）"
      parameters:
        - name: ticket_id
          in: path
          required: true
          schema:
            type: string
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                ticket_id:
                  type: string
                  example: "1001"
      responses:
        '200':
          description: "审批成功，生成退货单号"
          content:
            application/json:
              schema:
                type: object
                properties:
                  code: { type: integer, example: 0 }
                  msg: { type: string, example: "审批成功" }
                  data:
                    type: object
                    properties:
                      ticket_id: { type: string }
                      status: { type: string, example: "approved" }
                      return_ticket_no: { type: string }
        # ===== 第 4 步：异常场景（5 个 4xx/5xx 覆盖） =====
        '403':
          description: "权限不足"
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '404':
          description: "工单不存在"
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '409':
          description: "工单已被处理"
          content:
            application/json:
              schema:
                type: object
                properties:
                  code: { type: integer, example: 10001 }
                  msg: { type: string, example: "该工单已被处理，请刷新列表" }
                  data: { type: object, nullable: true, example: null }
        '422':
          description: "参数校验失败"
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'
        '500':
          description: "服务内部异常"
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ErrorResponse'

components:
  schemas:
    ErrorResponse:
      type: object
      properties:
        code:
          type: integer
          description: "业务错误码"
        msg:
          type: string
          description: "错误提示文案"
        data:
          type: object
          nullable: true
```

**生成规则：**
1. 所有正常 API 响应必须包含统一结构：`{ code: 0, msg: string, data: object }`
2. 有特殊 `msg` 文案的异常响应可以内联 schema；无特殊文案的（如 403/404/422/500）统一引用 `$ref: '#/components/schemas/ErrorResponse'`，避免重复定义
3. 生成文件命名为 `openapi.yaml`，与 PRD 放在同一目录

---

## 输出示例

以"AI 客服退货审批 PRD"为例，最终产出文件结构：

```
[项目名] PRD 文档/
├── [项目名] PRD.md          # PRD 文档
└── openapi.yaml             # OpenAPI Mock 规范文件（第 5 步输出）
```

`openapi.yaml` 包含完整的 OpenAPI 3.0 接口定义，可直接拖入 API Fox 导入，一键生成所有接口 Mock。

每个接口的 `responses` 至少包含：
- 1 个 `'200'`（正常场景，与 PRD GWT 正常路径一一对应）
- 5 个 4xx/5xx 状态码（异常场景覆盖：403 / 404 / 409 / 422 / 500）

---

## 输出完成后

提醒 PM：
1. 将 `openapi.yaml` 导入 API Fox（项目设置 → 导入数据 → OpenAPI/Swagger → 选择文件），自动生成所有接口和 Mock
2. 在 API Fox 中开启 Mock 服务，获取 Mock URL
3. Mock URL 发给前端，前端可立即开始开发
4. 后端按 `openapi.yaml` 中的响应 schema 实现接口
5. QA 按 PRD 第 4 节 GWT 场景编写测试用例
6. 所有角色基于同一份 GWT → OpenAPI → Mock 闭环工作

**导入验证：** 生成后自检——"把这份 `openapi.yaml` 拖进 API Fox，能否一次性导入成功且所有接口 Mock 可用？"

---

## 参考

- [PRD 规范](../规范/PRD规范.md)（Mock 联动章节）
- OpenAPI 3.0 规范：[OpenAPI Specification](https://spec.openapis.org/oas/v3.0.3)
- 推荐工具：[API Fox](https://apifox.com/)（支持 OpenAPI 文件导入，一键生成 Mock）
- `openapi.yaml` 导入 API Fox 路径：项目设置 → 导入数据 → OpenAPI/Swagger → 选择文件
