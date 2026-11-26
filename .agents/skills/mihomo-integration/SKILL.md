---
name: mihomo-integration
description: Use when changing AI Cage's Clash Verge connection, subscriptions, mihomo configuration merge, External Controller client, proxy selection, traffic, logs, or sidecar lifecycle.
---

# mihomo Integration

## When to use

Use this skill for any change involving Clash Verge, mihomo, External Controller, subscriptions, proxy groups, TUN runtime configuration, or kernel process supervision.

## Workflow

1. Read `AGENTS.md`, `docs/architecture.md`, and `docs/mihomo-api.md`.
2. Identify whether the change affects Verge mode, sidecar mode, or both.
3. Read the target package and its tests before editing.
4. Preserve subscription proxies, groups, rules, and providers when merging runtime configuration.
5. Keep Controller and core IPC on loopback with Bearer authentication.
6. Add or update focused tests, then run `go test ./...`.

## Project rules

- Verge mode never starts, stops, or reloads the Verge core.
- The two modes are mutually exclusive; TUN must not be active in both.
- Sidecar uses a fixed application data directory and a pinned mihomo SHA-256.
- Controller Secret and subscription URLs never enter logs or command-line arguments.
- A failed start must not leave an untracked child process, route, or adapter.

## References

- `docs/mihomo-api.md`
- `docs/windows-tun.md`
- `docs/testing.md`

Read all three before changing lifecycle or TUN behavior.

## Avoid

Do not add silent kernel downloads, arbitrary configuration reload paths, LAN Controller binding, or assumptions that API behavior is identical across all mihomo versions.

## Verification

Run `go test ./...` and the relevant Windows manual scenario from `docs/testing.md`. Report unexecuted real-kernel checks explicitly.
