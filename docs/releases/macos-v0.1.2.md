# StackChan AI Server for macOS 0.1.2 — Third-party LLMs on stock firmware

## 更新内容 / Changes

- 回合制管线（`openai_compatible` / `tokenhub` / `openrouter`）现在由服务端判断说话结束。
  出厂固件以 `auto` 模式监听、从不发送 `listen:stop`，此前双方互等，永远不会触发 STT。
  设备若自己发来 `listen:stop`，仍以先到者为准。新增 `compatible_server_vad`（默认开启）
  和 `compatible_vad_silence_ms`（默认 800 毫秒）。
- 修复 Base URL 写成 `.../v1` 时请求打到 `/v1/v1/...` 的问题，STT、LLM、TTS 三段以及
  内置的 OpenRouter 默认值都受影响。现在带不带 `/v1` 都能正确解析。
- Realtime 地址可配置（`openai_base_url`），可指向任何实现 OpenAI Realtime 协议的服务。
  第三方 realtime 端点未经验证。
- 日志新增上行音频帧数/字节数统计和设备声明的监听模式，用来区分“没收到音频”和
  “收到音频但没触发回合”。没有产生音频的回合现在也会上报结束。

- The turn-based pipeline (`openai_compatible`, `tokenhub`, `openrouter`) now detects
  the end of speech server-side. Firmware listening in `auto` mode never sends
  `listen:stop`, so both sides waited and no turn ever started. A device's own
  `listen:stop` still ends the turn when it arrives first. New `compatible_server_vad`
  (default on) and `compatible_vad_silence_ms` (default 800 ms).
- Fixed base URLs written as `.../v1` producing `/v1/v1/...` request paths across all
  three stages, including the built-in OpenRouter default. Both spellings now work.
- Added a configurable `openai_base_url` for the realtime provider. Third-party
  realtime endpoints are untested.
- Added incoming-audio counters and the announced listening mode to the log, and an
  end-of-turn report for turns that produce no audio.

## 下载与升级 / Download and upgrade

下载 **StackChan-AI-Server-0.1.2-macos-universal.dmg**；无需自行构建或安装 Docker。
安装包包含 Apple Silicon 和 Intel 两种架构，构建目标为 macOS 12+，静态打包 OPUS。
升级前用“活动监视器”退出本应用的 `stackchan-server` 进程，再把新应用拖入“应用程序”替换。
设置与对话历史保留在原来的 Application Support 目录；不要同时运行两个副本。
旧版 0.1.1 与 0.1.0 安装包继续保留，便于回退。

Download the universal DMG from Assets. No user build or Docker installation is needed.
Apple Silicon and Intel executables target macOS 12+, with OPUS included statically.
Stop the old app's `stackchan-server` processes in Activity Monitor before replacing
the app in Applications. Existing settings/history are retained; do not run two copies.
The 0.1.1 and 0.1.0 downloads remain available. Verify the matching checksum first:

```bash
shasum -a 256 -c StackChan-AI-Server-0.1.2-macos-universal.dmg.sha256
```

## 许可与源码 / License and corresponding source

这是本应用的第一个 **AGPL-3.0-only** 构建。Release 附带与该 tag 完全一致的源码包
`stackchan-ai-server-macos-v0.1.2-source.tar.gz` 及其校验文件；设置页面内也带有源码链接。
已发布的 `macos-v0.1.1`、`macos-v0.1.0` 与 HA `v2.8.0-beta.3` 保留原有 MIT 许可，未被重建或覆盖。

This is the first **AGPL-3.0-only** build of this app. The Release carries the
corresponding-source archive built from this exact tag
(`stackchan-ai-server-macos-v0.1.2-source.tar.gz`) with its checksum, and the settings
page carries a source link. Previously published artifacts — `macos-v0.1.1`,
`macos-v0.1.0` and HA `v2.8.0-beta.3` — keep their original MIT license and were
neither rebuilt nor overwritten. Go module and OPUS sources are not vendored in the
archive; their versions are pinned in `go.mod`/`go.sum` and `macos/prepare-opus.sh`,
which downloads a checksum-pinned OPUS release.

## 限制与验证 / Limitations and verification

仍是临时签名预览版，尚无 Developer ID 签名或 Apple 公证，也没有菜单栏控制和自动更新。
确认来源后，无法验证开发者的提示可参考
[Apple 安装说明](https://support.apple.com/en-us/102445)；不要全局关闭 Gatekeeper 或绕过恶意软件警告。
服务端静音判定只有单元测试覆盖，没有在 `auto` 模式的真机上验证过；欢迎回报日志。
本次不改变 HA 插件版本，插件侧对应的是 `v2.8.0-beta.4`。

Still an ad-hoc signed preview, not Developer ID signed or Apple notarized; no
menu-bar controls or automatic updates. Follow the Apple guidance above for
unidentified-developer warnings; never disable Gatekeeper globally or bypass malware
warnings. The server-side turn detection is covered by unit tests only — no physical
device running factory firmware in `auto` mode was available, so logs from real
hardware are welcome. This release does not change the HA add-on version; the matching
add-on release is `v2.8.0-beta.4`.
