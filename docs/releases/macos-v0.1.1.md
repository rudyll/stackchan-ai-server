# StackChan AI Server for macOS 0.1.1 — Crown & Wings

## 更新内容 / Changes

- 恢复选定的手绘皇冠＋天使翅膀形象，保留接近实机的方形机身、点眼和直线嘴。
- DMG 中的应用图标、网页页头小图标、登录页和浏览器图标统一使用这一形象，处理透明外缘。
- 中英文 README 加入图片，补齐 Home Assistant 商店的 icon.png / logo.png。
- 本次只更新外观，不修改语音处理、设备接入或已保存的配置。

- Hardware-faithful StackChan artwork with a simple hand-drawn crown and angel wings,
  retaining the original dot eyes and straight mouth.
- Matching transparent artwork in the macOS app's full ICNS size set and the
  embedded settings header, login page and browser icon.
- Images in both READMEs and matching Home Assistant store icon/logo resources,
  with asset consistency and transparency regression checks.
- Visual-only update: voice processing, device setup and saved configuration are unchanged.

## 下载与升级 / Download and upgrade

下载 **StackChan-AI-Server-0.1.1-macos-universal.dmg**；无需自行构建或安装 Docker。
安装包包含 Apple Silicon 和 Intel 两种架构，构建目标为 macOS 12+，静态打包 OPUS。
升级前用“活动监视器”退出本应用的 `stackchan-server` 进程，再把新应用拖入“应用程序”替换。
设置与对话历史保留在原来的 Application Support 目录；不要同时运行两个副本。
旧版 0.1.0 安装包继续保留，便于回退。

Download the universal DMG from Assets. No user build or Docker installation is needed.
Apple Silicon and Intel executables target macOS 12+, with OPUS included statically.
Stop the old app's `stackchan-server` processes in Activity Monitor before replacing
the app in Applications. Existing settings/history are retained; do not run two copies.
The 0.1.0 download remains available. Verify the matching checksum before installing:

```bash
shasum -a 256 -c StackChan-AI-Server-0.1.1-macos-universal.dmg.sha256
```

## 限制与验证 / Limitations and verification

仍是临时签名预览版，尚无 Developer ID 签名或 Apple 公证，也没有菜单栏控制和自动更新。
确认来源后，无法验证开发者的提示可参考
[Apple 安装说明](https://support.apple.com/en-us/102445)；不要全局关闭 Gatekeeper 或绕过恶意软件警告。
HA 这次只更新仓库内的商店图片，不发布新插件版本；已安装插件内部的网页图片要等更新或重建后才改变。
商店图片不替代 HA 导航侧栏的图标。

Still an ad-hoc signed preview, not Developer ID signed or Apple notarized; no
menu-bar controls or automatic updates. Follow the Apple guidance above for
unidentified-developer warnings; never disable Gatekeeper globally or bypass malware warnings.
This does not publish a new HA add-on version. Store image files are updated in
the repository; an already installed add-on retains its bundled GUI image until
updated/rebuilt. Store artwork does not replace HA's navigation sidebar icon.

候选 DMG 已验证：校验、挂载、签名、两种架构与最低系统目标、完整应用图标、独立启动、
受保护的设置接口、设备 OTA 接口，以及桌面/手机网页图片显示。静态 OPUS 和图标测试在
两种架构通过，Intel 使用 Rosetta。Go 测试、构建、Python 测试与 shell/YAML 检查通过。
未测试真实 HA 商店画面、物理 StackChan、外部 AI 服务、macOS 12 实机或真实 Intel Mac。
已有两处非本次引入的日志格式 vet 警告仍在 `internal/web_socket/web_socket.go`。

Candidate DMG integrity, mount, signatures, both architectures/minimum target,
complete ICNS set, isolated startup, protected settings, OTA and desktop/mobile
image rendering were verified. Static OPUS and icon tests passed for both
architectures (Intel via Rosetta). Go tests/build, Python tests and shell/YAML
checks passed. Actual HA store rendering, physical StackChan, external AI services,
macOS 12 and physical Intel hardware were not tested. Two pre-existing logging
format vet warnings remain in `internal/web_socket/web_socket.go`.
