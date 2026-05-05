# StackChan HA Add-ons

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

[English](README.md) | 中文

## 项目介绍

**StackChan HA Add-ons** 让你的 [StackChan](https://github.com/m5stack/StackChan) 桌面机器人成为与智能家居深度集成的 AI 语音助手——无需小智账号。

StackChan 是基于 M5Stack CoreS3（ESP32-S3）的掌心大小机器人，出厂搭载开源的 [xiaozhi-esp32](https://github.com/78/xiaozhi-esp32) 固件，该固件原本依赖小智云提供语音识别、语言模型和语音合成服务。本插件将小智云替换为 Home Assistant 插件：语音数据发往 **OpenAI** 而非小智，Home Assistant 则完全运行在你的本地网络中。

**设备固件无需任何修改**——插件使用设备已支持的 Xiaozhi WebSocket 协议 v3 进行通信。语音指令由 **OpenAI Realtime API** 或 **Google Gemini Live API** 处理（流式语音对话，延迟约 0.5–1.5 秒，可在插件 UI 中切换），机器人可通过语音控制任意 Home Assistant 设备。

**核心功能：**
- 基于 OpenAI Realtime API 流式传输，端到端延迟约 0.5–1.5 秒
- 语音控制灯光、空调、窗帘、媒体播放器及脚本
- 支持区域控制（如"把客厅所有灯关掉"）
- 无需小智账号——语音由 OpenAI 处理，HA 保留在本地局域网
- 作为标准 Home Assistant 插件一键安装

## 工作原理

本插件完全取代小智云服务。StackChan 设备以为自己在和小智服务器通信，实际上连接的是运行在你的 Home Assistant 上的本地服务器。

```
StackChan ESP32-S3（未修改的 xiaozhi-esp32 固件）
    │  Xiaozhi WebSocket 协议 v3（OPUS 音频 + JSON）
    ▼
StackChan AI Server（本插件，运行在 HA 的 12800 端口）
    ├─ /xiaozhi/ota/  → 返回本地 WebSocket 地址
    └─ /xiaozhi/ws    → WebSocket 会话
         ├─ OpenAI Realtime API（STT + LLM + TTS，流式）
         └─ Home Assistant WebSocket API（设备控制）
```

**音频流水线（流式，延迟约 0.5–1.5 秒）：**

```
设备 OPUS（16kHz）→ PCM → OpenAI Realtime API
                              ↓ 服务端 VAD 检测到说话结束
                         流式 PCM 回复（24kHz）
                              ↓
                         OPUS 编码 → 设备扬声器
```

无需小智账号，除 OpenAI 外无其他云端依赖。

---

## 插件列表

### StackChan AI Server

基于 **OpenAI Realtime API**（`gpt-realtime-1.5`）实现低延迟语音对话，并通过自然语言控制 Home Assistant 设备。

**功能特性：**
- 约 0.5–1.5 秒响应延迟（服务端 VAD + 流式音频）
- 语音控制 HA 设备：灯光、空调、窗帘、媒体播放器、脚本等
- 支持区域控制（如"把客厅所有灯关掉"）
- 同一会话内多轮对话保持上下文
- 在插件 UI 中通过下拉菜单选择模型和语音

---

## 安装

1. 在 Home Assistant 中进入 **设置 → 插件 → 插件商店**
2. 点击右上角三点菜单 → **仓库**
3. 添加以下 URL：
   ```
   https://github.com/rudyll/stackchan_ha_addons
   ```
4. 在商店中找到 **StackChan AI Server**，点击 **安装**
5. 进入插件的 **配置** 选项卡，填写必要字段（见下文）
6. 启动插件

---

## 配置项

通过 `ai_provider` 选择**其中一个** AI 后端，只需填入对应的 API Key，另一家的字段可留空。

| 选项 | 必填 | 说明 |
|------|------|------|
| `local_host` | ✅ | Home Assistant 的局域网 IP（如 `192.168.1.100`）。设备通过此 IP 连接。 |
| `ha_mcp_token` | ✅ | HA 长期访问令牌。在 **个人资料 → 安全 → 长期访问令牌** 中创建。 |
| `ai_provider` | ✅ | `openai`（默认）或 `gemini`。选择由谁处理语音 + LLM + TTS。 |
| `system_prompt` | | 助手的自定义角色设定或指令。 |
| **OpenAI**（当 `ai_provider=openai`） | | |
| `openai_api_key` | ✅ | OpenAI API Key，在 [platform.openai.com](https://platform.openai.com) 获取。 |
| `openai_realtime_model` | | Realtime 模型，默认 `gpt-realtime-1.5`。Mini（更便宜）：`gpt-realtime-mini`、`gpt-4o-mini-realtime-preview`。 |
| `openai_tts_voice` | | TTS 语音，默认 `alloy`。女声推荐：`nova`、`shimmer`、`coral`、`sage`、`cedar`、`marin`、`cove`。 |
| **Gemini**（当 `ai_provider=gemini`） | | |
| `gemini_api_key` | ✅ | Google AI Studio API Key，在 [aistudio.google.com](https://aistudio.google.com/app/apikey) 获取。 |
| `gemini_model` | | Gemini Live 模型，默认 `gemini-2.5-flash-preview-native-audio-dialog`。 |
| `gemini_voice` | | TTS 语音，默认 `Aoede`。可选：`Aoede`、`Charon`、`Fenrir`、`Kore`、`Puck`。 |

---

## 固件配置

设备固件需要知道你的本地服务器地址，而不是小智云。有以下两种方式。

> **选哪种方式？**
> 大多数情况下使用**方式 A（NVS 写入）**——重新注入只需两条命令，无需重新编译。
> 只有在需要同时进行其他固件定制时，才使用**方式 B（源码编译）**。
>
> ⚠️ **注意：** 官方 xiaozhi-esp32 的 OTA 升级会写入完整 flash 镜像，**NVS 分区也会被覆盖**。每次固件升级后需重新执行方式 A 的第三、四步写入 NVS，这比重新编译固件要快得多。
>
> 💡 **一键写入：** 运行 `python3 flash_nvs.py`，交互式引导完成全部四个步骤（支持中英文）。

### 方式 A — 写入 NVS（推荐）

固件启动时会先检查 NVS（非易失性存储）中是否有 OTA 地址覆盖值，优先于内置默认值。此设置**在固件 OTA 升级后依然保留**，只需操作一次即可。

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
ota_url,data,string,http://<你的HA_IP>:12800/xiaozhi/ota/
EOF
```
将 `<你的HA_IP>` 替换为 Home Assistant 的局域网 IP（与插件配置中的 `local_host` 相同）。

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
   - 改为 `http://<你的HA_IP>:12800/xiaozhi/ota/`
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
4. 连网后，设备会使用你配置的 OTA 地址（通过 NVS 写入或 menuconfig）连接本地插件，而不是小智云

### 固件 OTA 升级后

官方 xiaozhi-esp32 的 OTA 升级会写入完整的 flash 镜像，**NVS 分区也会被覆盖**。因此无论使用哪种方式，固件升级后都需要重新执行方式 A 的第三、四步，重新写入 NVS。这比重新编译固件要快得多。

---

## 端口说明

| 端口 | 用途 |
|------|------|
| `12800/tcp` | 主 WebSocket 服务器（OTA 地址返回 + 语音会话） |
| `443/tcp` | 旧版 HTTPS 拦截（已废弃，不使用） |

---

## 许可证

MIT — 详见 [LICENSE](LICENSE)
