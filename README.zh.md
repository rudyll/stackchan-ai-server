# StackChan AI Server

<p align="center"><img src="stackchan-server/logo.png" alt="戴手绘皇冠、带天使翅膀的 StackChan" width="200" height="200"></p>

[![License: AGPL-3.0-only](https://img.shields.io/badge/License-AGPL--3.0--only-blue.svg)](LICENSE)

[English](README.md) | 中文

## 项目介绍

**StackChan AI Server** 让你的 [StackChan](https://github.com/m5stack/StackChan) 桌面机器人变成可配置的实时或 OpenAI-compatible 语音助手，并可选集成 Home Assistant——无需小智账号，无需维护意图脚本。

StackChan 是基于 M5Stack CoreS3（ESP32-S3）的掌心大小机器人，官方固件原本依赖小智云提供语音识别、语言模型和语音合成服务。本项目用自托管服务替代该依赖：可以作为 Home Assistant add-on 或 standalone Docker 服务运行，选择原生实时 provider，或分别配置 OpenAI-compatible 的 STT、LLM、TTS；只有连接 HA runtime 时才启用智能家居控制。

### 为什么不用 HA Assist？

| | **StackChan AI Server** | **HA Assist** |
|---|---|---|
| **理解能力** | GPT-4o / Gemini 2.5——理解自然、口语化的表达 | 规则匹配——只识别预定义的短语模板 |
| **多轮对话** | 整个会话全程保持上下文 | 无状态——每句话独立处理，无记忆 |
| **模糊指令** | 问一个关键问题再执行（如"好热"→"在哪个房间？"→开空调） | 指令不匹配就报错或随机执行 |
| **多设备消歧** | 列出匹配设备，询问控制哪一个或全部 | 不支持消歧 |
| **语音质量** | 实时神经音频——自然、低延迟 | STT→LLM→TTS 管道，有明显延迟 |
| **配置成本** | 一个 system prompt，无需写脚本 | 每个设备操作都需定义意图和脚本 |
| **场景/脚本/自动化** | 按名称自动搜索并激活 | 需要手动编写对应意图 |

**核心功能：**
- 可选 AI 后端：**OpenAI Realtime API**、**Google Gemini Live API**，或 OpenAI 兼容的 STT → LLM → TTS 管线（TokenHub、OpenRouter、任意兼容端点）
- 全程多轮对话——同一会话内 AI 记住上下文
- 语音控制灯光、空调、窗帘、媒体播放器、脚本、场景及自动化
- 支持区域控制（如"把客厅所有灯关掉"）
- Home Assistant 可选——standalone Docker 可以在没有 HA 的情况下提供语音对话
- 无需小智账号——音频交给你配置的 provider 处理，启用 HA 时设备控制仍留在局域网
- 使用官方固件，无需重新编译

## 工作原理

本项目完全取代小智云服务。StackChan 设备以为自己在和小智服务器通信，实际上连接的是运行在 Home Assistant add-on 或 standalone Docker 中的自托管服务器。

```
StackChan ESP32-S3（未修改的 xiaozhi-esp32 固件）
    │  Xiaozhi WebSocket 协议 v3（OPUS 音频 + JSON）
    ▼
StackChan AI Server（HA add-on 或 standalone Docker，使用 12800 端口）
    ├─ /xiaozhi/ota/  → 返回本地 WebSocket 地址
    └─ /xiaozhi/ws    → WebSocket 会话
         ├─ OpenAI Realtime API  ─┐
         │                        ├─ STT + LLM + TTS，流式（任选其一）
         └─ Gemini Live API     ──┘
         └─ Home Assistant WebSocket API（可选的设备控制）
```

设备的音频和 WebSocket 流量是直接发给 StackChan AI Server 的，Home Assistant 不承担中转。启用 HA 时，Server 只会为了智能家居控制另外建立带认证的 HA WebSocket API 连接；standalone 也可以选择建立这条直连，但默认不连接 HA。

**音频流水线（流式，延迟约 0.5–1.5 秒）：**

```
设备 OPUS（16kHz）→ PCM → OpenAI Realtime / Gemini Live
                              ↓ 服务端 VAD 检测到说话结束
                         流式 PCM 回复（24kHz）
                              ↓
                         OPUS 编码 → 设备扬声器
```

无需小智账号，云端依赖取决于你实际选择的 provider 或兼容端点。

---

## 运行方式

### StackChan AI Server

低延迟语音对话，可选 **OpenAI Realtime API**（`gpt-realtime`）或 **Google Gemini Live API**（`gemini-2.5-flash-native-audio-latest`），在插件 UI 中切换。两种后端都支持自然语言控制 Home Assistant 设备。

**功能特性：**
- AI 后端可切换：OpenAI Realtime / Gemini Live（下拉选择）
- 约 0.5–1.5 秒响应延迟（服务端 VAD + 流式音频）
- 语音控制 HA 设备：灯光、空调、窗帘、媒体播放器、脚本、场景及自动化
- 支持区域控制（如"把客厅所有灯关掉"）
- 全程多轮对话——同一会话内 AI 记住上下文
- OpenAI 与 Gemini 的模型名均可自由填写

同一套 server 也支持 standalone Docker；配置方式见下方说明。

---

## 安装

1. 在 Home Assistant 中进入 **设置 → 插件 → 插件商店**
2. 点击右上角三点菜单 → **仓库**
3. 添加以下 URL：
   ```
   https://github.com/rudyll/stackchan-ai-server
   ```
4. 在商店中找到 **StackChan AI Server**，点击 **安装**
5. 进入插件的 **配置** 选项卡，填写必要字段（见下文）
6. 启动插件

**HA 更新：** [2.8.0-beta.3](https://github.com/rudyll/stackchan-ai-server/releases/tag/v2.8.0-beta.3) 将共享图形化设置页、常驻 NVS 指引和皇冠翅膀图标正式带入插件。请先备份，刷新插件商店，再更新并重启；保留原有配置和数据。**打开 Web UI** 是共享图形化页面，HA 自带的**配置**选项卡是另一份表单。只更新仓库图片不会更新已安装的服务程序。

### Standalone Docker（Beta）

Home Assistant 不是必需项。使用 Docker Compose 时，复制 `stackchan-server/.env.standalone.example` 为 `.env`，将 `STACKCHAN_LOCAL_HOST` 设置为 Docker 宿主机的局域网 IP，填写 AI API Key，然后运行：

```bash
cd stackchan-server
cp .env.standalone.example .env
docker compose -f docker-compose.standalone.yml up --build -d
```

**Docker 更新：** 先取得最新源码，保留 `.env` 和挂载的数据目录，再运行上面的命令。当前通过源码在本地构建，没有可直接拉取的预构建镜像。[容器自动检查](https://github.com/rudyll/stackchan-ai-server/actions/workflows/containers.yml) 会构建声明支持的四种 HA 架构并运行两条启动路径，但不能代替真实设备和 HA 环境测试。

如果 `STACKCHAN_SETTINGS_TOKEN` 留空，打开 GUI 前先从首次启动日志中取出自动生成的 token：

```bash
docker compose -f docker-compose.standalone.yml logs --no-color --tail=50 stackchan
```

默认情况下，语音 WebSocket 服务发布在宿主机 `12800` 端口（容器内部仍监听 `12800`）；设置页默认绑定宿主机 `127.0.0.1:8099`。如果设置了 `STACKCHAN_SETTINGS_PORT`，请改为打开对应的宿主机端口（容器内部仍监听 `8099`），然后输入首次启动日志显示的 Bearer token 登录；浏览器随后使用短期 HttpOnly cookie，API 请求仍受保护。Token 会持久化到 `./data/settings-token`。GUI 提供 OpenAI Realtime、Gemini Live、TokenHub、OpenRouter 和 OpenAI-compatible 独立入口，并可检测 Provider、获取模型名称和填充对应的声音目录；standalone 默认不连接 Home Assistant。如需控制实体，在 GUI 的 **Standalone → Home Assistant bridge** 中启用，填写 HA 地址和长期访问令牌，再开启对应 Provider 的 HA tools；Server 会直接连接 HA Core WebSocket API，用于发现实体、查询状态和调用允许的服务，设备音频及 WebSocket 不会经过 HA 中转。standalone 不启动 443 端口 OTA 劫持；设备 OTA 地址配置为 `http://<服务器局域网IP>:12800/xiaozhi/ota/`。

如果同一台主机上同时运行 HA add-on 和 standalone Docker，建议让 add-on 继续使用 `12800`，在启动 Compose 前将 `STACKCHAN_WS_PORT=12801`（如有需要再将 `STACKCHAN_SETTINGS_PORT=8100`）。此时设备应配置为 `http://<服务器局域网IP>:12801/xiaozhi/ota/`。容器内部仍监听 `12800`，启动脚本会把选定的宿主机端口写入设备后续使用的 WebSocket 地址。

每台设备同时只会使用一个 OTA/WebSocket 目标：配置 HA add-on 地址的设备连接 add-on，配置 standalone 地址的设备连接 standalone。如果既没有写入 NVS 覆盖值，也没有在编译固件时设置 `OTA_URL`，官方固件仍会使用原来的云端/HA 劫持路径，不会自动发现 standalone。

### macOS — 下载现成 DMG（预览版）

从 [GitHub Releases](https://github.com/rudyll/stackchan-ai-server/releases/tag/macos-v0.1.1) 下载 [macOS 0.1.1 通用 DMG](https://github.com/rudyll/stackchan-ai-server/releases/download/macos-v0.1.1/StackChan-AI-Server-0.1.1-macos-universal.dmg)，将 **StackChan AI Server.app** 拖进**应用程序**后打开。**用户不需要自行构建，也不用安装 Docker、Go、Homebrew 或 OPUS。** 安装包同时包含 Apple Silicon 和 Intel 两种架构，构建目标为 macOS 12+，音频库已静态打包；Release 同时提供 SHA-256 校验文件。

应用会选择空闲端口、打开本地设置页，并在首次运行时显示登录 Token。新的 3D StackChan 图标也用于 HA / standalone 共用网页界面。这是**仅做临时签名、尚无 Developer ID 签名和 Apple 公证的预览版**；确认下载来源后，若提示无法验证开发者，可按 [Apple 官方说明](https://support.apple.com/en-us/102445)在**系统设置 → 隐私与安全性 → 仍要打开**中允许。不要绕过恶意软件警告，也不要全局关闭 Gatekeeper。macOS 防火墙询问时，如需局域网设备连接，请允许传入连接。预览版尚无菜单栏控制，升级前请在“活动监视器”中退出对应的 `stackchan-server` 进程。详见[安装说明与可选开发者构建方法](stackchan-server/macos/README.md)。

可选的 `STACKCHAN_STANDALONE_HA_ENABLED`、`STACKCHAN_STANDALONE_HA_URL` 和 `STACKCHAN_STANDALONE_HA_TOKEN` 可以在首次启动时预配置 standalone 到 HA 的直连。`STACKCHAN_DEVICE_PROFILES`、`STACKCHAN_SYSTEM_PROMPT`、`STACKCHAN_AUDIO_PREBUFFER_MS` 和 `STACKCHAN_AUDIO_PREBUFFER_MAX_WAIT_MS` 是其他首次启动默认值。服务启动后建议直接通过 GUI 修改；保存后的值会持久化到挂载的数据目录。HA Token 不会由设置 API 返回；密码框留空会保留已保存的 Token。

---

## 配置项

推荐在启动 add-on 后点击 **Open Web UI** 进行设置。该受 Home Assistant Ingress 保护的页面将配置分为“基础 / 语音管线 / 后台任务 / 设备 Profile”；标准 add-on「配置」页仍保留作兼容用途。

通过 `ai_provider` 选择**其中一个** AI 后端，只需填入对应的 API Key，另一家的字段可留空。

| 选项 | 必填 | 说明 |
|------|------|------|
| `local_host` | ✅ | 运行 StackChan AI Server 的主机局域网 IP（如 `192.168.1.100`）。add-on 通常填写 HA 主机；standalone 填写 Docker 宿主机。 |
| `ha_enabled` | | 是否启用 Home Assistant 工具和后台任务。HA add-on 默认为 `true`；standalone runtime 将默认为 `false`。 |
| `ha_mcp_token` | 启用 HA 时必填 | HA 长期访问令牌。在 **个人资料 → 安全 → 长期访问令牌** 中创建；standalone 模式留空。 |
| `standalone_ha_enabled` | | 是否让 standalone 直连 Home Assistant，默认 `false`。 |
| `standalone_ha_url` | 启用 standalone HA 时 | Home Assistant 地址，例如 `http://homeassistant.local:8123`；Server 会自动补上 `/api/websocket`。 |
| `standalone_ha_token` | 启用 standalone HA 时 | HA 长期访问令牌。它保存在 standalone 数据目录中，设置 API 不会返回该值。 |
| `ai_provider` | ✅ | `openai`（默认）、`gemini`、`tokenhub`、`openrouter` 或 `openai_compatible`。选择由谁处理语音 + LLM + TTS。 |
| `system_prompt` | | 助手的自定义角色设定或指令。 |
| **OpenAI**（当 `ai_provider=openai`） | | |
| `openai_api_key` | ✅ | OpenAI API Key，在 [platform.openai.com](https://platform.openai.com) 获取。 |
| `openai_realtime_model` | | Realtime 模型名（自由输入），默认 `gpt-realtime`。账号可用的其他模型 ID 也可直接填写。 |
| `openai_tts_voice` | | Realtime 声音，默认 `alloy`。内置声音包括 `alloy`、`ash`、`ballad`、`coral`、`echo`、`sage`、`shimmer`、`verse`、`marin` 和 `cedar`。 |
| **Gemini**（当 `ai_provider=gemini`） | | |
| `gemini_api_key` | ✅ | Google AI Studio API Key，在 [aistudio.google.com](https://aistudio.google.com/app/apikey) 获取。 |
| `gemini_model` | | Gemini Live 模型名（自由输入），默认 `gemini-2.5-flash-native-audio-latest`。Gemini Live 支持的其他模型 ID 也可直接填写。 |
| `gemini_voice` | | TTS 语音，默认 `Aoede`。共 30 种原生音频语音可选（下拉菜单）。 |
| `gemini_enable_tools` | | 启用 Gemini 的 HA 设备控制工具。add-on 默认开启；standalone 默认关闭。 |
| `gemini_enable_search` | | 启用 Gemini 的 Google Search 联网搜索（默认关闭）。**⚠️ 与 `gemini_enable_tools` 互斥** — Gemini 不支持同时使用 grounding（联网搜索）和 function calling（HA 工具调用），两者同时开启会导致 1011 连接错误。如需联网搜索，请将 `gemini_enable_tools` 设为关闭。 |
| **OpenAI-compatible 管线**（当 `ai_provider=tokenhub`、`openrouter` 或 `openai_compatible`） | | 这是逐句的 STT → LLM → TTS 管线，不是 OpenAI Realtime WebSocket；延迟应以插件 `[LAT]` 日志实测为准。 |
| `stt_*` | | 可选的独立 STT Base URL、API Key 和模型。 |
| `llm_*` | | 可选的独立 LLM Base URL、API Key 和模型。 |
| `tts_*` | | 可选的独立 TTS Base URL、API Key、模型和音色。 |
| `compatible_*` | | 三段均未单独填写时的向后兼容回退值。 |
| TokenHub | | 选择 `tokenhub`，填写 `tokenhub_base_url`、`tokenhub_api_key`，再按需要配置 `stt_*`、`llm_*`、`tts_*` 三段字段。 |
| OpenRouter | | 选择 `openrouter`，填写 `openrouter_api_key`，再配置 LLM 模型和 STT/TTS 字段；Base URL 自动使用 `https://openrouter.ai/api/v1`。 |
| 通用兼容端点 | | 选择 `openai_compatible`，在 GUI 中使用 `llm_*`、`stt_*`、`tts_*` 字段；旧的 `compatible_*` 仍作为回退值保留。 |
| **后台任务（Beta）**（当前仅 `ai_provider=openai`） | | 长任务进入每台设备独立的后台队列，实时语音会话可继续对话。 |
| `background_tasks_enabled` | | 是否启用后台任务，默认关闭。 |
| `background_agent_base_url` | | 支持 `/v1/chat/completions` 的 OpenAI-compatible Base URL。 |
| `background_agent_api_key` | | 后台 Agent Key；留空时依次复用 `llm_api_key`、`compatible_api_key` 或 `openai_api_key`。 |
| `background_agent_model` | 启用时 ✅ | 后台 Chat Completions 模型，不可填写 Realtime 音频模型。 |
| `background_agent_timeout_seconds` | | 单项任务超时，默认 300 秒。 |

每次语音完成后，插件日志会输出 `[LAT]`，包含从设备 `listen:stop` 到 STT、LLM、TTS 开始和首个音频包的毫秒数，可据此比较小智、Realtime 和兼容管线的实际差异。

### 本地历史与唤醒待机（Standalone）

在设置页 **Provider → 对话记忆与唤醒** 中开启文字历史后，服务会按设备保存本地 JSON
文件，支持导出 Markdown 和清空。默认保留 90 天、每台设备最多 2000 条消息；重连时
只带入最近 20 条，文字总量最多 12000 字符。消息数设为 0 可只归档、不带入模型。
历史保存默认关闭；开启上下文复用后，近期文字会发给所选 AI Provider。这项功能不保存
原始录音，也不等于能自动回忆全部旧对话。

Standalone 默认在回复音频发送完成、静默 15 秒后关闭语音通道，标准小智固件随即回到
原有的本地唤醒词/按键待机；短窗口内可以直接追问。静音数据包不会延长窗口。
`conversation_idle_seconds=0` 可保留不限时连续对话，HA add-on 仍默认使用这一旧行为。
窗口内的旁人说话或噪声仍可能触发回复；自定义唤醒词需要固件支持，不能靠修改提示词实现。
更多配置、存储路径和固件限制见[对话记忆与唤醒说明](docs/conversation-memory-and-wake.md)。

### 连续对话与后台任务（Beta）

> **Beta 提醒：** 此功能已通过自动化测试，但仍需要大家使用实体 StackChan、真实 Home Assistant 环境和不同的 OpenAI-compatible 后台模型进行验证。

启用后台任务后，OpenAI Realtime 会获得创建、查询和取消后台任务的工具。开灯、查询温度等短 HA 操作仍直接执行；分析历史数据或多步骤操作可进入后台 FIFO 队列，前台会立即确认并继续对话。任务状态保存在 `/data/background-tasks.json`，按设备 `Device-Id` 隔离。standalone 使用时必须先启用 HA bridge，再启用后台任务。

任务完成后，结果会等待用户没有说话、模型没有回复且设备音频队列已经播放完毕时播报。结果在播报前会被单一会话领取；断线或打断会释放领取状态，重连后可再次投递，成功播报后不会重复。服务重启时仍在运行且不能恢复的任务会转为失败并明确通知。

当前后台执行器使用 OpenAI-compatible Chat Completions，并可调用现有 Home Assistant 工具；它不自动获得网页搜索或代码执行能力。此功能当前只接入 OpenAI Realtime 前台，Gemini Live 和逐句兼容语音管线不会显示后台任务工具。

### Provider 选择与中国内地延迟

`openai` 和 `gemini` 是原生的双向实时音频接入，分别需要 OpenAI Realtime 或 Gemini Live API Key。在中国内地访问其海外端点时，跨境网络可能增加延迟或造成音频 chunk 间歇到达；实际体验取决于网络路由，不只取决于模型。

`tokenhub`、`openrouter` 和 `openai_compatible` 使用逐句 HTTP 管线。现在可分别配置 `stt_*`、`llm_*`、`tts_*` 来混用提供者；每段端点须分别支持 `/v1/audio/transcriptions`、`/v1/chat/completions` 或 `/v1/audio/speech`。仅提供文本 Chat 的 TokenHub 账号可承担 LLM 段，但还需要单独配置 STT 与 TTS；OpenRouter 的路由和“OpenAI-compatible”标签也不保证中国内地低延迟。

想优先获得小智那种顺畅体验，较合理的方向是“本地或国内 STT + 国内 LLM + 本地或国内 TTS”。现在已可通过独立三段设置实现这种组合；只有某段留空时，才回退使用旧 `compatible_*` 端点。

### 连续播放与多设备 Profile

`audio_prebuffer_ms`（默认 300ms）是开始播放前累计的音频长度。提高它可明显减少上游网络或模型音频 chunk 抖动造成的断续，但会增加相同量级的首次出声延迟；`audio_prebuffer_max_wait_ms`（默认 900ms）限制最长等待时间。网络不稳定时建议从 `300 / 900` 开始，仍断续可试 `480 / 1200`；追求最快开口可设为 `120 / 500`。

`device_profiles` 是一个以设备 WebSocket `Device-Id` 为键的 JSON 对象。未填写的字段自动继承全局设置；API Key 保持全局，不应放进 profile。例如：

```json
{
  "AA:BB:CC:DD:EE:FF": {
    "system_prompt": "你叫客厅小柴。始终用简短自然的中文回答。",
    "openai_tts_voice": "coral",
    "openai_realtime_model": "gpt-realtime-mini"
  },
  "11:22:33:44:55:66": {
    "provider": "gemini",
    "system_prompt": "你叫卧室小柴，夜间回答更轻柔简短。",
    "gemini_voice": "Aoede"
  }
}
```

可覆盖字段：`provider`、`system_prompt`、`openai_realtime_model`、`openai_tts_voice`、`gemini_model`、`gemini_voice`、`compatible_model`、`compatible_stt_model`、`compatible_tts_model`、`compatible_tts_voice`。设备连接时会在日志中显示其 `Device-Id`。

### 唤醒词、待机和省电

唤醒词检测和“始终待机”属于 StackChan 固件，不由本 add-on 或官方 App 的 prompt 控制；本 add-on 只能在固件发送 `listen:detect/start/stop` 后处理音频。官方文档也说明自定义唤醒词尚未上线。

若要减少误唤醒、打扰和耗电，建议先在设备侧关闭 AI Agent 自动启动，改为点击屏幕/触控唤醒；或者保留唤醒词但在固件中增加空闲超时后停止麦克风、仅在触控或按键时恢复。后一种需要定制固件，并应保留设备端的舵机安全、Wi-Fi 配网和 OTA/USB 恢复功能。

---

## 固件配置

设备固件需要知道你的本地服务器地址，而不是小智云。服务器可以是 Home Assistant add-on、standalone Docker 或 macOS 原生应用；有以下两种方式。

**图形化接入指引：** 打开设置页侧栏的 **设备接入 / NVS 注入**，或点击首次连接提示。相同服务器版本下，HA 的 **打开 Web UI** 与 standalone 共用这套页面；HA 自带的插件 **配置** 标签页仍是另一套表单。接入指引长期保留，显示当前服务器地址、设备端口、可复制的 OTA URL、脚本入口和 USB / ESP-IDF 操作步骤；不会自动烧录，也不会修改网络配置。地址来自服务器的只读配置，不会误用浏览器或 HA Ingress 地址；显示 URL 不代表已经验证连通性。

注入脚本要在通过 USB 连接设备的电脑上运行，不是在 HA 或服务器容器里运行。分别填写页面上的 host 和 port，不要填设置页端口 `8099` 或 HA 管理端口 `8123`。同机 HA 使用 `12800` 时，standalone 应使用 `12801` 等其他空闲端口。当前 add-on 通告给设备的端口固定为 `12800`，其宿主机映射也请保留 `12800`。Docker 如需更改，修改 `.env` 中的 `STACKCHAN_LOCAL_HOST` / `STACKCHAN_WS_PORT` 后重新创建容器；macOS 请退出应用，修改 `~/Library/Application Support/StackChan AI Server/runtime.env` 中对应字段，再重新打开。设备一次只使用一个目标；启用 standalone 的 HA 实体控制桥接不会改变这个目标。

> **选哪种方式？**
> 大多数情况下使用**方式 A（NVS 写入）**——重新注入只需两条命令，无需重新编译。
> 只有在需要同时进行其他固件定制时，才使用**方式 B（源码编译）**。
>
> ⚠️ **注意：** 现有注入脚本及下方手动操作都会**重写整个 NVS 分区**，覆盖已有 Wi-Fi 和其他 NVS 设置。需要保留时请先备份 NVS，写入后可能需要重新配网。后续重刷或升级若覆盖了 NVS，再重新注入服务器地址；并非每次固件升级都必然覆盖 NVS。
>
> 💡 **一键写入：** 运行 `python3 flash_nvs.py`，交互式引导完成全部四个步骤（支持中英文）。工具支持填写服务器局域网 IPv4 地址或设备可解析的局域网主机名（例如 `stackchan.local`），以及对外 TCP 端口。条件允许时建议给服务器做 DHCP 保留；如果 standalone 与 HA add-on 同机运行且 add-on 使用 `12800`，这里应填写 standalone 的 `12801`。

设备启动时需要先访问一个 OTA 地址，因此目前 NVS 仍必须写入 standalone 主机地址和端口。官方固件路径不会自动发现局域网中的 Docker 服务；若使用 mDNS 或 UDP 自动发现，需要修改固件，并额外处理 VLAN、防火墙以及局域网中同时存在多个服务的问题。因此我们保留显式写入 NVS 作为可靠路径。

### 方式 A — 写入 NVS（推荐）

固件启动时会先检查 NVS（非易失性存储）中是否有 OTA 地址覆盖值，优先于内置默认值。该设置会保留到 NVS 被擦除或覆盖；如果发生这种情况，使用当前服务器地址和端口重新运行注入脚本。

#### 前置条件

需要先安装 ESP-IDF，参考[官方安装指南](https://docs.espressif.com/projects/esp-idf/en/stable/esp32s3/get-started/index.html)。

**每次打开新终端都必须先激活 ESP-IDF 环境**，否则 `parttool.py` 脚本将无法找到。

- **macOS / Linux：**
  ```bash
  . $HOME/esp/esp-idf/export.sh
  ```
- **Windows（PowerShell）：**
  ```powershell
  C:\esp\v6.0.1\esp-idf\export.ps1
  ```
- **Windows（命令提示符）：**
  ```cmd
  C:\esp\v6.0.1\esp-idf\export.bat
  ```

验证激活状态：`idf.py --version` 能正常输出版本号即表示成功。

#### 操作步骤

**第一步 — 查询 NVS 分区大小：**
```bash
python3 $IDF_PATH/components/partition_table/parttool.py \
    --port /dev/tty.usbserial-XXXX \
    get_partition_info --partition-name nvs
```
记录显示的 `size` 值（通常为 `0x4000` 或 `0x6000`）。将 `/dev/tty.usbserial-XXXX` 替换为你的设备串口（参见下方[查找串口设备名](#查找串口设备名)）。

**第二步 — 创建 NVS 数据文件：**
```bash
cat > nvs.csv << 'EOF'
key,type,encoding,value
wifi,namespace,,
ota_url,data,string,http://<你的服务器主机地址>:<对外 WebSocket 端口>/xiaozhi/ota/
EOF
```
将 `<你的服务器主机地址>` 替换为运行 StackChan AI Server 的主机局域网 IPv4 地址或设备可解析的主机名（如 `192.168.1.100` 或 `stackchan.local`），并将 `<对外 WebSocket 端口>` 替换为设备可访问的端口（默认 `12800`；standalone 使用自定义宿主机端口时填写 `STACKCHAN_WS_PORT`）。使用 add-on 时主机通常是 Home Assistant 主机；使用 standalone 时是 Docker 宿主机。NVS 工具会校验 IPv4 地址和主机名格式，但不会执行服务自动发现。

**第三步 — 生成 NVS 二进制文件**（将 `0x4000` 替换为第一步查到的实际大小）：
```bash
python3 $IDF_PATH/components/nvs_flash/nvs_partition_generator/nvs_partition_gen.py \
    generate nvs.csv nvs.bin 0x4000
```

**第四步 — 写入设备：**
```bash
python3 $IDF_PATH/components/partition_table/parttool.py \
    --port /dev/tty.usbserial-XXXX \
    write_partition --partition-name nvs --input nvs.bin
```

### 方式 B — 从源码编译

仅在需要同时进行其他固件定制时使用此方式。注意，通过 `menuconfig` 内置的 OTA 地址**会在设备执行固件 OTA 升级时被覆盖**——届时仍需重做方式 A 的第三、四步。

#### 前置条件

与方式 A 相同，需要安装并激活 ESP-IDF 环境（见上文）。

#### 操作步骤

1. 克隆并准备 [StackChan 固件](https://github.com/m5stack/StackChan/tree/main/firmware)：
   ```bash
   git clone https://github.com/m5stack/StackChan.git
   cd StackChan/firmware
   python3 fetch_repos.py
   ```

2. 安装第三方组件依赖：
   ```bash
   idf.py add-dependency "bblanchon/arduinojson"
   idf.py update-dependencies
   ```
   **不要跳过此步骤**——它会安装 `ArduinoJson` 及 `idf_component.yml` 中声明的其他组件。跳过此步骤会导致编译时报 `Failed to resolve component 'ArduinoJson'` 错误。

3. 在 menuconfig 中设置 OTA 地址：
   ```bash
   idf.py menuconfig
   ```
   - 按 `/` 搜索 `OTA_URL`
   - 改为 `http://<你的服务器局域网IP>:<对外 WebSocket 端口>/xiaozhi/ota/`（默认是 `12800`；standalone 使用自定义宿主机端口时填写 `STACKCHAN_WS_PORT`）
   - 保存并退出

4. 编译并烧录：
   ```bash
   idf.py set-target esp32s3
   idf.py build
   idf.py -p /dev/tty.usbserial-XXXX -b 921600 flash
   ```

### 查找串口设备名

- **macOS：** `ls /dev/tty.usb*`
- **Linux：** `ls /dev/ttyUSB* /dev/ttyACM*`

### 首次 Wi-Fi 配网

如果设备没有 Wi-Fi 信息（恢复出厂或首次烧录）：

1. 下载 **StackChan World** App（iOS / Android）
2. 打开 App，按照"添加设备"流程操作
3. App 通过蓝牙将 Wi-Fi 信息推送到设备
4. 连网后，设备会使用你配置的 OTA 地址（通过 NVS 写入或 menuconfig）连接本地 StackChan AI Server，而不是小智云

### 固件 OTA 升级后

先确认设备是否仍使用你的本地 OTA 地址。升级或重刷若擦除、覆盖了 NVS，再使用当前服务器地址和端口重新运行注入脚本。重新注入也会覆盖 NVS，请先备份需要保留的设置，并准备重新配网。

---

## 端口说明

| 端口 | 用途 |
|------|------|
| `12800/tcp` | 主 WebSocket 服务器（OTA 地址返回 + 语音会话） |
| `443/tcp` | 旧版 HTTPS 拦截（已废弃，不使用） |

---

## 许可证

当前组合项目采用 **AGPL-3.0-only**，详见 [LICENSE](LICENSE)、
[版权及保留的第三方权利](stackchan-server/NOTICE.md)和[贡献规则](CONTRIBUTING.md)。
遵守许可证即可商业使用；没有强制捐赠、利润分成或向本仓库提交 PR 的要求。

已经发布的 MIT 版本，包括 HA `v2.8.0-beta.3` 和可下载的 `macos-v0.1.1` DMG，
保留原有许可。这次调整适用于当前源码和未来适用版本，不会覆盖旧版发布文件。
分发和网络服务的源码提供要求见[许可说明](docs/licensing.md)。

## 支持项目

[赞助完全自愿](SPONSORING.md)，用于持续维护、设备和 AI 服务测试、打包等工作。
不赞助不影响许可证授予的权利。目前尚未公布已核实的收款渠道；PayPal 及可选
加密货币收款信息只会在维护者确认后发布。
