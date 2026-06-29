---
name: image-generation
description: UI 负责人生图能力的二级入口。基于阿里云 DashScope，覆盖 2 个 API 端点、3 个模型、4 种子技能。当需要为设计稿生成配图、编辑现有图片、风格迁移或生成组图时使用。
---

# 生图（Image Generation）

> UI 设计稿视觉素材生成中心。2 个 API 端点、3 个模型、4 种能力，覆盖「从无到有生成」「图像编辑/组图」「基于参考图风格生成」三大类需求。

---

## 角色行为

你是 UI 负责人流水线中的图片素材生成器。核心原则：

- **先选模型再做事**：根据需求选对端点和模型，不盲目调用；成本优先，能用便宜的绝不用贵的
- **分级选型**：90% 场景用 `z-image-turbo`（0.10 元/张）即可满足；图生图用 Kling（0.20 元/张）；`wan2.7-image-pro`（0.50 元/张）仅用于需要图像编辑/组图或思考模式深度推理的高质量交付
- **不替代设计判断**：生成的图片是素材，最终选用权在人手里

---

## API 端点

| 端点 | URL | 模式 | 模型 |
|------|-----|------|------|
| 端点一：多模态生成 | `POST .../multimodal-generation/generation` | 同步 | `z-image-turbo`、`wan2.7-image-pro` |
| 端点二：图像生成 | `POST .../image-generation/generation` | **异步**（需轮询） | `kling/kling-v3-omni-image-generation` |

### 认证（通用）

```
Authorization: Bearer $DASHSCOPE_API_KEY
```

### 端点一的响应（同步）

```json
{
    "output": {
        "choices": [{
            "message": {
                "content": [{ "image": "https://..." }]
            }
        }]
    }
}
```

## 能力路由

| 当你需要… | 调用子技能 | 模型 | 端点 |
|-----------|-----------|------|------|
| 从文字描述生成图片 | [文生图](文生图.md) | `z-image-turbo` / `wan2.7-image-pro` | 端点一 |
| 用参考图 + 文字合成或编辑图片 | [图像编辑](图像编辑.md) | `wan2.7-image-pro` | 端点一 |
| 用参考图的风格/背景生成新图 | [图生图](图生图.md) | `kling/kling-v3-omni` | 端点二（异步） |
| 多张连续图，保持主体一致 | [组图生成](组图生成.md) | `wan2.7-image-pro` | 端点一 |

---

## 模型对比

### 能力矩阵

| 模型 | 端点 | 文生图 | 图像编辑 | 图生图 | 组图 | 速度 |
|------|:--:|:--:|:--:|:--:|:--:|------|
| `z-image-turbo` | 端点一 | ✅ | ❌ | ❌ | ❌ | 快 |
| `wan2.7-image-pro` | 端点一 | ✅ | ✅ | ❌ | ✅ | 较慢 |
| `kling/kling-v3-omni` | 端点二 | ❌ | ❌ | ✅ | ❌ | 异步 |

### 计费

> **计费规则**：图像生成按输出张数计费，输入不计费。费用 = 图像单价 × 输出的图像张数。请求失败不产生任何费用。

| 模型 | 单价（中国内地） | 免费额度 | 额度有效期 |
|------|----------------|---------|----------|
| `z-image-turbo` | prompt_extend=false：**0.10 元/张**<br>prompt_extend=true：0.20 元/张 | 100 张 | 90 天 |
| `wan2.7-image-pro` | **0.50 元/张** ⚠️ | 50 张 | 90 天 |
| `kling/kling-v3-omni` | 1K/2K：0.20 元/张<br>4K：0.40 元/张 | 无 | — |

### 成本选型推荐

| 优先级 | 模型 | 适用场景 | 单价 | 理由 |
|:------:|------|---------|------|------|
| 🥇 首选 | `z-image-turbo` | 文生图（快速原型、设计稿配图、图标、插画） | 0.10 元/张 | 速度快、成本最低，关闭 prompt_extend 精确控制效果 |
| 🥈 次选 | `kling/kling-v3-omni` | 图生图（风格迁移、参考图生成新图） | 0.20 元/张 | 唯一支持图生图的模型，价格适中（约为 pro 的 40%） |
| 🥉 谨慎使用 | `wan2.7-image-pro` | 需要图像编辑/组图或最高质量时 | 0.50 元/张 | **比 z-image-turbo 贵 5 倍**，仅用于需要图像编辑/组图或 thinking_mode 深度推理的场景 |

**核心原则：能省则省，z-image-turbo 足够覆盖 90% 的卡片/配图场景；wan2.7-image-pro 只在「必须编辑已有图片」或「需要思考模式深度推理」时才启用。**

---

## 在 UI 设计流程中的位置

```
UI 负责人主流程
   │
   ├─ 信息架构设计    → 页面结构图 + 页面清单
   ├─ 交互原型设计    → HTML/CSS 设计稿
   ├─ 视觉规范定义    → 设计系统配置
   │
   └─ [生图]（按需调用）→ 4 种能力
         │
         ├─ 缺少素材    → 文生图 / 组图生成
         ├─ 现有图修改  → 图像编辑
         └─ 风格迁移    → 图生图（Kling）
```

---

## 环境要求

- 有效的阿里云 DashScope API Key（[DashScope 控制台](https://dashscope.console.aliyun.com/) 申请）
- 设置环境变量：`export DASHSCOPE_API_KEY="sk-..."`
- 端点二（异步）需要实现轮询逻辑：POST 获取 task_id → GET 轮询任务状态 → SUCCEEDED 后获取结果

---

## 参考

- DashScope 多模态生成文档：https://help.aliyun.com/document_detail/2712223.html
- API Key 申请：https://dashscope.console.aliyun.com/
- 主 Skill：[../SKILL.md](../SKILL.md)
