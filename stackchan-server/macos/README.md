# macOS native package (development preview)

The macOS package runs the standalone Go server directly, without Docker.
The build currently targets the host architecture (`arm64` on Apple Silicon;
set `STACKCHAN_MACOS_ARCH=amd64` for Intel Macs) and bundles `libopus` into the
application.

## Build a local DMG

Install the native build prerequisites:

```bash
brew install go pkg-config opus
./stackchan-server/macos/build-dmg.sh
```

The unsigned development image is created at
`stackchan-server/dist/StackChan AI Server.dmg`. Open the app from the mounted
image and allow it in **System Settings → Privacy & Security** if Gatekeeper
blocks the first launch. A signed and notarized release will be added after the
Apple Developer account is available.

On first launch the app automatically detects the default LAN IPv4 address,
chooses free WebSocket and settings ports (normally `12800` and `8099`),
persists those choices under `~/Library/Application Support/StackChan AI
Server`, and opens the local settings page. The first-run dialog shows the
settings token required by the browser UI.

If Home Assistant or another server already uses the default port, the app
keeps the selected alternative port in its runtime state and prints the device
OTA address in the server log. Configure that address in the device NVS; the
firmware does not discover standalone servers automatically.
