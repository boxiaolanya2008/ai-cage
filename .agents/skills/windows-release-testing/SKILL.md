---
name: windows-release-testing
description: Use when changing AI Cage's Windows elevation, core supervisor, Job Object, TUN/Wintun, system tray, single-instance behavior, build manifest, packaging, or release validation.
---

# Windows Release Testing

## When to use

Use this skill for Windows privilege boundaries, UAC, process trees, TUN, DNS, routing, tray behavior, manifests, packaging, and release checks.

## Workflow

1. Read `AGENTS.md`, `docs/windows-tun.md`, and `docs/testing.md`.
2. Confirm the main executable manifest remains `asInvoker`.
3. Confirm only the core is elevated and only when sidecar TUN is requested.
4. Verify core owns the mihomo Job Object and handles stop timeouts.
5. Run automated tests and a production Wails build.
6. Execute the Windows manual matrix; record exact versions and failures.

## Project rules

- Never elevate the WebView UI or ordinary launched applications.
- Never run Verge TUN and sidecar TUN simultaneously.
- Never treat build success as proof of Wintun, DNS, UDP, or sleep recovery.
- Never claim Authenticode signing unless a real certificate and verification result exist.

## References

- `docs/windows-tun.md`
- `docs/testing.md`
- `docs/development.md`
- `build/windows/wails.exe.manifest`

Read the manifest before changing build or privilege behavior.

## Avoid

Do not add a Windows service, auto-update, or driver installer unless the approved scope requires it. Do not execute unsigned binaries from user-writable paths with elevation.

## Verification

Require `go test ./...`, `npm --prefix frontend run build`, and `D:/go/bin/wails3.exe build`. Real-machine checks remain mandatory for TUN and elevation.
