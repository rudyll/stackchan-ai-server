# StackChan AI Server for macOS 0.1.0 — Preview

## 下载即用 / Ready to run

下载 Assets 中的 **StackChan-AI-Server-0.1.0-macos-universal.dmg**，
将应用拖入“应用程序”后打开。**不需要自行构建，也不需要 Docker、Go、Homebrew 或音频库。**
安装包包含 Apple Silicon 和 Intel 两种架构，构建目标为 macOS 12+。
同时提供匹配的 `.sha256` 校验文件。

Download the universal DMG from Assets, drag the app into Applications, and launch.
No user build or extra runtime installation is needed. Both Apple Silicon and
Intel executables are included, targeting macOS 12+. A matching checksum is attached.

## 更新内容 / Included

- 可直接下载的通用 DMG，静态打包 OPUS，并附开源许可和校验文件。
- 新的 3D StackChan 应用图标；网页页头、登录页和浏览器图标使用同一形象。
- 当前 standalone 设置界面、可选 HA 实体控制桥接、本地文字历史、静默待机和常驻 NVS 注入指引。
- 中英文安装及升级说明；首次启动显示设置页登录 Token。

- Ready-made universal DMG with statically included OPUS, license notices and SHA-256 checksum.
- A 3D StackChan app icon, shared with the settings header, login and browser icon.
- Current standalone provider settings, optional HA bridge, text history, idle standby and NVS setup guide.
- Bilingual installation/upgrade instructions and a first-run settings-token dialog.

## 安装与限制 / Installation and limitations

这是**临时签名的预览版，尚无 Developer ID 签名和 Apple 公证**。
确认下载来自本项目后，如遇无法验证开发者的提示，可按
[Apple 官方说明](https://support.apple.com/en-us/102445)在“系统设置 → 隐私与安全性”中选择“仍要打开”。
不要关闭整个系统的 Gatekeeper，也不要绕过恶意软件警告。

预览版尚无菜单栏控制或自动升级。升级前在“活动监视器”中停止本应用的
`stackchan-server` 进程，再替换应用；不要同时运行多个副本。
数据保留在 `~/Library/Application Support/StackChan AI Server`，删除应用不会清除数据。
设备仍需按 NVS 指引配置；注入会覆盖原 NVS，可能需要重新配网。
设置页只监听本机；如需设备接入，请在 macOS 防火墙提示时允许本应用的局域网传入连接。

This preview is ad-hoc signed, **not Developer ID signed or Apple notarized**.
After verifying the source, use Privacy & Security → Open Anyway for an
unidentified-developer warning. Do not disable Gatekeeper globally or bypass malware warnings.
There are no menu-bar controls or automatic updates yet. Stop this app's
`stackchan-server` processes in Activity Monitor before replacing it; do not run
multiple copies. Settings/history remain in the per-user Application Support
directory. Configure the device via the NVS guide; injection replaces existing NVS.
The settings page is local-only; permit incoming LAN connections if prompted and needed.

此 Release 只发布 macOS standalone 安装包，不修改 HA add-on 版本。
This is a standalone macOS release, not a new HA add-on release.

## 验证范围 / Verification scope

已在 Apple Silicon / macOS 26.6.2 上验证候选 DMG 的校验、挂载、签名完整性、
应用启动、设置页与设备 OTA 接口；两种架构的静态 OPUS 编解码测试均通过，
Intel 路径使用 Rosetta 执行。桌面和手机尺寸网页的图标加载及鉴权回归通过。
尚未在 macOS 12 或真实 Intel Mac 上实机测试，也未验证真实设备语音及外部 AI / HA 服务。

Candidate DMG integrity, mount, signature sealing, app startup, settings and OTA
endpoints were checked on Apple Silicon / macOS 26.6.2. Static OPUS codec tests
passed for both architectures, with Intel execution under Rosetta. Desktop/mobile
GUI image loading and authentication regressions passed. macOS 12, physical Intel
hardware, real device audio and external AI/HA sessions have not been tested.
