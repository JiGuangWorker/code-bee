---
name: audio-generation
description: UI 负责人配音能力的入口。基于 AiSounds MCP，覆盖 TTS 语音合成、音效生成、音乐生成、视频配音、视频配乐、音频清理 6 种能力。当需要为设计稿、原型演示、产品视频生成配音、音效或背景音乐时使用。
---

# 配音（Audio Generation）

> UI 设计稿音频素材生成中心。6 种能力，覆盖「文字转语音」「音效」「背景音乐」「视频配音/配乐」「音频清理」五大类需求。

---

## 角色行为

你是 UI 负责人流水线中的音频素材生成器。核心原则：

- **先草稿再提交**：TTS / SFX / Music 三类支持 draft（不扣费预览）→ 人确认 → submit（扣费生成），避免浪费点数
- **视频类直接生成**：Video Voice / Video BGM 需要上传本地视频文件，无草稿阶段
- **不替代听觉判断**：生成的音频是素材，最终选用权在人手里

---

## MCP 工具清单

| 工具 | 用途 | 扣费 |
|------|------|:--:|
| `get_audio_capabilities` | 查看当前可用的音频能力 | — |
| `get_account_balance` | 查询账户点数余额 | — |
| `list_tts_voices` | 列出可用的 TTS 音色（可按 gender 筛选） | — |
| `draft_tts_task` | 创建 TTS 草稿（不扣费预览） | ❌ |
| `submit_tts_task` | 提交 TTS 任务（扣费生成） | ✅ |
| `draft_sfx_task` | 创建音效草稿（不扣费预览） | ❌ |
| `submit_sfx_task` | 提交音效任务（扣费生成） | ✅ |
| `draft_music_task` | 创建音乐草稿（不扣费预览） | ❌ |
| `submit_music_task` | 提交音乐任务（扣费生成） | ✅ |
| `create_video_voice_task` | 给本地视频配音（上传视频 + 生成） | ✅ |
| `create_video_bgm_task` | 给本地视频生成配乐（上传视频 + 生成） | ✅ |
| `create_audio_cleanup_task` | 音频清理/降噪 | ✅ |
| `get_task_status` | 查询任务状态（按 type + jobId） | — |
| `get_recent_jobs` | 查看最近任务列表 | — |
| `get_recent_assets` | 查看最近生成的素材 | — |
| `download_asset` | 获取素材下载链接 | — |

---

## 能力路由

| 当你需要… | 调用工具 | 关键参数 | 产出格式 |
|-----------|---------|---------|---------|
| 将文字转为语音（旁白、产品介绍、提示音） | `draft_tts_task` → `submit_tts_task` | `text`, `voiceId` | MP3/WAV |
| 生成 UI 交互音效（点击、切换、通知等） | `draft_sfx_task` → `submit_sfx_task` | `prompt`, `durationSeconds` | WAV |
| 生成背景音乐（演示视频、产品宣传片） | `draft_music_task` → `submit_music_task` | `prompt`, `durationSeconds`, `genre` | MP3 |
| 给已有视频添加 AI 配音 | `create_video_voice_task` | `filePath`, `script`, `voiceId`, `ttsJobId` | 含配音的视频 |
| 给已有视频添加背景音乐 | `create_video_bgm_task` | `filePath`, `prompt` | 含 BGM 的视频/MP3 |
| 清理音频噪声 | `create_audio_cleanup_task` | 音频文件 | 清理后的音频 |

---

## 工作流

### TTS（文字转语音）

```
1. list_tts_voices          → 浏览可选音色（男/女/中性）
2. draft_tts_task           → 生成草稿预览（不扣费）
3. 人确认音色和效果
4. submit_tts_task          → 提交生成（扣费）
5. get_task_status          → 轮询任务状态
6. download_asset           → 下载音频文件
```

### SFX（音效）/ Music（音乐）

```
1. draft_sfx_task / draft_music_task   → 生成草稿预览（不扣费）
2. 人确认效果
3. submit_sfx_task / submit_music_task → 提交生成（扣费）
4. get_task_status                     → 轮询任务状态
5. download_asset                      → 下载音频文件
```

### Video Voice（视频配音）/ Video BGM（视频配乐）

```
1. create_video_voice_task / create_video_bgm_task  → 上传视频 + 创建任务（扣费）
2. get_task_status                                  → 轮询任务状态
3. download_asset                                   → 下载成品
```

> ⚠️ Video Voice 需要先完成 TTS 生成（拿到 `ttsJobId`），再将 TTS 结果与视频合成。

---

## 在 UI 设计流程中的位置

```
UI 负责人主流程
   │
   ├─ 信息架构设计 → 页面结构图 + 页面清单
   ├─ 交互原型设计 → HTML/CSS 设计稿
   ├─ 视觉规范定义 → 设计系统配置
   │
   ├─ [生图]（按需） → 图片素材
   └─ [配音]（按需） → 音频素材
         │
         ├─ 页面语音提示     → TTS
         ├─ 交互反馈音效     → SFX
         ├─ 产品演示视频配乐 → Music / Video BGM
         └─ 产品介绍旁白     → TTS / Video Voice
```

---

## 环境要求

- AiSounds MCP 已配置（通过 `aisounds` MCP server）
- 有效的 AiSounds API Key（[AiSounds 控制台](https://aisounds.cn) 申请）
- CLI 初始化（可选）：`npx "@aisounds.cn/cli" auth login --api-key ask_xxx`

---

## 参考

- AiSounds 官网：https://aisounds.cn
- CLI 文档：`npx "@aisounds.cn/cli" --help`
- 生图 Skill：[../生图/SKILL.md](../生图/SKILL.md)
- 主 Skill：[../../SKILL.md](../../SKILL.md)
