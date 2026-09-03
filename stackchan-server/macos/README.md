# macOS standalone — ready-made download

<p align="center"><img src="../logo.png" alt="StackChan with a hand-drawn crown and angel wings" width="160" height="160"></p>

[Download universal DMG](https://github.com/rudyll/stackchan-ai-server/releases/download/macos-v0.1.1/StackChan-AI-Server-0.1.1-macos-universal.dmg) · [Release and checksum](https://github.com/rudyll/stackchan-ai-server/releases/tag/macos-v0.1.1)

**No build required. No Docker, Homebrew, Go or audio-library installation for users.**
The app contains Apple Silicon and Intel executables, targeting macOS 12+.
The macOS preview is versioned separately from HA releases.

## Install / 安装

1. Open the DMG and drag **StackChan AI Server.app** into **Applications**.
   打开 DMG，将应用拖进“应用程序”，再从那里打开；不要长期从挂载镜像运行。
2. This preview has an ad-hoc signature, **not Developer ID signing or Apple notarization**.
   Confirm the source, then use **System Settings → Privacy & Security → Open Anyway**
   for an unidentified-developer warning. See [Apple's guidance](https://support.apple.com/en-us/102445).
   尚未经过 Apple 公证。确认来自本项目后，可在“隐私与安全性”中选择“仍要打开”；
   不要全局关闭 Gatekeeper，也不要绕过恶意软件警告。
3. First launch opens the browser UI and displays the Settings Token.
   首次启动会打开设置页并显示登录 Token；之后可从下述 settings-token 文件找回。
4. Configure the AI provider, then use **设备接入 / NVS 注入** in the sidebar.
   此处显示设备需要的服务器地址和端口；NVS 注入在连接设备的电脑上完成，
   会覆盖已有 NVS 设置，可能需要重新配网。
5. If the macOS firewall asks, allow incoming connections if LAN devices need access.
   网页设置只监听本机；设备端口需要局域网可达，不要直接暴露到公网。

The DMG includes these instructions, an Applications shortcut, and license notices
inside the app. To check the download, put its matching .sha256 file alongside it:

```bash
shasum -a 256 -c StackChan-AI-Server-0.1.1-macos-universal.dmg.sha256
```

## Data, ports and upgrades / 数据与升级

Data is kept at `~/Library/Application Support/StackChan AI Server`:

- `settings-token`: private GUI login token; do not share it.
- `settings.json`: saved provider configuration (may contain secrets).
- `runtime.env`: selected LAN host, device port and settings port.
- `conversation-history/`: optional local text history.
- `logs/`: server logs.

The app selects free ports (normally device `12800`, settings `8099`) and persists
them. Use the address shown in the NVS guide; firmware does not discover it automatically.
If needed, quit the app, edit `STACKCHAN_LOCAL_HOST` / `STACKCHAN_WS_PORT` in
`runtime.env`, then relaunch. Do not edit the generated `config/config.yaml`.

This preview runs in the background without menu-bar controls or auto-updates.
To stop it, use Activity Monitor to quit this app's `stackchan-server` processes.
Stop the old instance before replacing the app, and do not run multiple copies.
Removing the app does not remove saved data.
预览版暂无菜单栏控制和自动升级。升级前先用“活动监视器”退出对应进程，
再替换应用；不要同时运行多个副本。删除应用不会自动删除设置及历史。

## Optional build — developers only

On a Mac with Xcode Command Line Tools installed:

```bash
brew install go cmake pkg-config
bash stackchan-server/macos/build-dmg.sh
```

The builder uses `macos/VERSION`, builds both architectures by default, and outputs
`stackchan-server/dist/macos-0.1.1-universal/`. Set `STACKCHAN_MACOS_ARCH=arm64`
or `amd64` for a single-architecture build. Set `OUTPUT_DIR` to a new directory
for another build; existing releases are never overwritten.

OPUS 1.6.1 is built from its [official, checksum-pinned source](https://opus-codec.org/release/stable/2026/01/14/libopus-1_6_1.html)
for macOS 12 and linked statically. Homebrew bottles are not bundled: their
minimum macOS version can be newer than the application's declared minimum.
Sources/build cache are under `dist/macos-deps`; override with
`STACKCHAN_MACOS_DEPS_DIR`. Native icon generation and DMG mounting may require
an unrestricted macOS build environment.

The source PNG in `server/internal/service/ai/assets/stackchan-icon.png` produces
the complete ICNS size set. The same artwork is embedded at 256px in the Go GUI.
See [design record](../../docs/brand-icon.md).

## Release procedure

1. Run relevant tests, build, shell/YAML checks and `git diff --check`; verify the
   candidate's Mach-O architectures/minimum OS, signatures, dependency links,
   mounted image and isolated startup. Record limitations in the release notes.
2. Commit and push final main; rebuild from that exact clean tracked revision.
3. Tag it `macos-v<VERSION>` and publish a GitHub pre-release with the DMG and
   matching checksum. Use [the 0.1.1 release notes](../../docs/releases/macos-v0.1.1.md).
4. Verify tag target, release state, download URLs, asset digest and downloaded
   checksum. This does not bump the HA add-on version or replace its release.
