# Release Checklist

## Before release

1. Run `go test ./...`.
2. Run `go vet ./...`.
3. Run `npm --prefix frontend run build`.
4. Run `D:/go/bin/wails3.exe build`.
5. Confirm `bin/ai-cage.exe` exists.
6. Confirm `frontend/bindings` is current.
7. Confirm Markdown links and Skill references.
8. Confirm `LICENSE` is present.

## GitHub metadata

Recommended repository description:

> Windows proxy workbench with Clash Verge import, an independent mihomo sidecar, TUN application routing, and a sandbox host window.

Recommended topics:

- `windows`
- `go`
- `wails`
- `react`
- `typescript`
- `mihomo`
- `clash`
- `proxy`
- `tun`
- `desktop`

## Release notes template

```text
AI Cage v0.1.0

Initial Windows release.

- Clash Verge snapshot import
- Independent mihomo sidecar
- AI-CAGE proxy group
- PROCESS-PATH application-cage rules
- Dedicated sandbox host window
- System theme and DPI-aware UI
- Loopback-only controller and core IPC
- GPL-3.0-or-later license
```

## Publish

1. Commit and push `main`.
2. Tag `v0.1.0`.
3. Create a GitHub Release.
4. Upload `bin/ai-cage.exe`.
5. Verify the release page and download link.
