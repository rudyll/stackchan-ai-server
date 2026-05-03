# StackChan HA Add-ons

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

[English](README.md) | 中文

适用于 StackChan 的 Home Assistant 插件仓库。StackChan 是基于 M5Stack CoreS3 的 AI 语音助手机器人，使用未经修改的 [xiaozhi-esp32](https://github.com/78/xiaozhi-esp32) 固件。

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

| 选项 | 必填 | 说明 |
|------|------|------|
| `local_host` | ✅ | Home Assistant 的局域网 IP（如 `192.168.1.100`）。设备通过此 IP 连接。 |
| `ha_mcp_token` | ✅ | HA 长期访问令牌。在 **个人资料 → 安全 → 长期访问令牌** 中创建。 |
| `openai_api_key` | ✅ | OpenAI API Key，在 [platform.openai.com](https://platform.openai.com) 获取。 |
| `openai_realtime_model` | | 使用的 Realtime 模型，默认 `gpt-realtime-1.5`。Mini（更便宜）：`gpt-realtime-mini`、`gpt-4o-mini-realtime-preview`。 |
| `openai_tts_voice` | | TTS 语音，默认 `alloy`。女声推荐：`nova`、`shimmer`、`coral`、`sage`、`cedar`、`marin`、`cove`。 |
| `system_prompt` | | 助手的自定义角色设定或指令。 |

---

## 固件配置

设备固件需要知道你的本地服务器地址，而不是小智云。有以下两种方式。

### 方式 A — 从源码编译（推荐）

适用于自行编译固件的情况。

1. 克隆并准备 [StackChan 固件](https://github.com/m5stack/StackChan/tree/main/firmware)：
   ```bash
   git clone https://github.com/m5stack/StackChan.git
   cd StackChan/firmware
   python3 fetch_repos.py
   ```

2. 在 menuconfig 中设置 OTA 地址：
   ```bash
   idf.py menuconfig
   ```
   - 按 `/` 搜索 `OTA_URL`
   - 改为 `http://<你的HA_IP>:12800/xiaozhi/ota/`
   - 将 `<你的HA_IP>` 替换为 Home Assistant 的局域网 IP（与插件配置中的 `local_host` 相同）

3. 编译并烧录：
   ```bash
   idf.py set-target esp32s3
   idf.py build
   idf.py -p /dev/tty.usbserial-XXXX -b 921600 flash
   ```
   将 `/dev/tty.usbserial-XXXX` 替换为你的设备串口。

### 方式 B — 写入 NVS（无需重新编译）

固件启动时会先检查 NVS（非易失性存储）中是否有 OTA 地址，优先使用 NVS 中的值。

**前提：** 已安装并激活 ESP-IDF（运行 `. $HOME/esp/esp-idf/export.sh`）

**第一步 — 查询 NVS 分区大小：**
```bash
python3 $IDF_PATH/components/partition_table/parttool.py \
    --port /dev/tty.usbserial-XXXX \
    get_partition_info --partition-name nvs
```
记录显示的 `size` 值（通常为 `0x4000` 或 `0x6000`）。

**第二步 — 创建 NVS 数据文件：**
```bash
cat > nvs.csv << 'EOF'
key,type,encoding,value
wifi,namespace,,
ota_url,data,string,http://<你的HA_IP>:12800/xiaozhi/ota/
EOF
```
将 `<你的HA_IP>` 替换为 Home Assistant 的局域网 IP。

**第三步 — 生成 NVS 二进制文件**（将 `0x4000` 替换为第一步查到的大小）：
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

### 查找串口设备名

- **macOS：** `ls /dev/tty.usb*`
- **Linux：** `ls /dev/ttyUSB* /dev/ttyACM*`

### 首次 Wi-Fi 配网

如果设备没有 Wi-Fi 信息（恢复出厂或首次烧录）：

1. 下载**小智**App（iOS / Android）
2. 打开 App，按照"添加设备"流程操作
3. App 通过蓝牙将 Wi-Fi 信息推送到设备
4. 连网后，设备会使用你配置的 OTA 地址（通过 menuconfig 或 NVS 写入）连接本地插件，而不是小智云

---

## 端口说明

| 端口 | 用途 |
|------|------|
| `12800/tcp` | 主 WebSocket 服务器（OTA 地址返回 + 语音会话） |
| `443/tcp` | 旧版 HTTPS 拦截（已废弃，不使用） |

---

## 许可证

MIT — 详见 [LICENSE](LICENSE)
