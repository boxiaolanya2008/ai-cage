# AI Cage

AI Cage is a Windows-first desktop workspace for managing a local proxy stack. It connects to an existing Clash Verge mihomo core or runs an independent mihomo sidecar, with optional TUN routing for ordinary desktop applications.

> Status: active MVP development. The repository contains the application architecture and tests; real Clash Verge, Wintun, DNS, and UDP behavior still requires validation on a Windows machine with the target mihomo build.

## Modes

- **Clash Verge**: query the existing External Controller, inspect traffic, switch proxy mode, and select nodes. AI Cage does not start, stop, or reload Verge's core.
- **Independent sidecar**: import the current Clash Verge configuration snapshot read-only, then run a separate official `mihomo.exe`. The built-in `AI-CAGE` group selects the exit node, and optional process rules route only AI Cage-launched applications through it.

The two modes are mutually exclusive. Do not run Verge TUN and AI Cage sidecar TUN at the same time.

## Architecture

- Wails v3 + React + TypeScript desktop UI and system tray
- Go services for settings, Verge snapshot import, subscriptions, applications, and mihomo API access
- Non-elevated UI; elevated core only when sidecar TUN is requested
- Loopback-only Controller and core IPC with Bearer authentication
- Job Object supervision for the sidecar process tree
- `AI-CAGE` proxy group and `PROCESS-PATH` rules for the application cage
- Dedicated sandbox host window: the selected app opens inside an AI Cage window instead of directly on the desktop
- Built-in onboarding tutorial for subscriptions, proxy groups, node selection, and the sandbox window
- Optional public exit-IP check through the selected node

See [architecture](docs/architecture.md), [mihomo API](docs/mihomo-api.md), and [Windows TUN](docs/windows-tun.md).

## Requirements

- Windows 10/11 x64
- Go 1.26 and Node.js 24 for development
- WebView2 Runtime for the desktop UI
- An official mihomo executable for independent sidecar mode
- Clash Verge with External Controller enabled for Verge mode (commonly `127.0.0.1:9097`; it may be disabled by default)

## Development

```bash
go test ./...
npm --prefix frontend install
npm --prefix frontend run dev
D:/go/bin/wails3.exe dev
```

Production build:

```bash
npm --prefix frontend run build
D:/go/bin/wails3.exe build
```

The executable is written to `bin/ai-cage.exe`.

## Configuration

Runtime data is stored under `%APPDATA%\ai-cage`. The Controller secret is generated for sidecar mode and is not passed on the command line. For Verge mode, enter the loopback Controller URL and secret in Settings.

## Safety boundaries

TUN captures network traffic; it does not remove an application's sandbox or integrity restrictions. The application launcher starts ordinary processes without elevation. AI Cage does not silently download mihomo, modify enterprise policy, or expose its Controller to the LAN.

## Documentation

- [Development guide](docs/development.md)
- [Testing guide](docs/testing.md)
- [Release guide](docs/release.md)
- [中文说明](README-ZH.md)

## License

GPL-3.0-or-later. See [LICENSE](LICENSE).
