---
name: frontend-ui
description: Use when changing AI Cage's React/TypeScript desktop UI, Wails bindings, page layout, styles, theme, toasts/modals, or the onboarding tutorial.
---

# Frontend UI

## When to use

Use this skill for any change under `frontend/`, Chinese interface copy, Wails binding updates, page interactions, theme variables, or tutorial content.

## Workflow

1. Read `AGENTS.md`, `docs/frontend.md`, and `docs/development.md`.
2. Read `frontend/src/App.tsx` for the page you are touching, plus `components.tsx` if you need a shared primitive.
3. Check `frontend/bindings/.../ai-cage/internal/app/appservice.ts` before calling a service method; if it is missing, the Go side changed and bindings must be regenerated.
4. Change the smallest component; reuse existing classes and primitives instead of adding a library.
5. Run `npm --prefix frontend run build`, then open the app to confirm the change.

## Project rules

- `frontend/bindings/` is generated. Never hand-edit it; change the Go service or model instead.
- `frontend/src/types.ts` only re-exports generated types. Do not duplicate model structs there.
- UI text is Chinese. Keep identifiers, API names, and log levels in English.
- Colors come from CSS variables in `styles.css`; never hard-code a hex value in a component.
- TUN changes require the confirmation modal, and Verge mode must show TUN as Verge-managed.
- Settings that need a kernel restart stay disabled while a connection is running.
- Polling stays at the existing 2-second interval with visibility and in-flight guards; do not add a second timer.
- The tutorial must not describe ban evasion or detection circumvention. Keep the compliance notice.

## References

- `docs/frontend.md`
- `docs/development.md`
- `frontend/src/App.tsx`
- `frontend/src/components.tsx`
- `frontend/src/tutorial.tsx`
- `frontend/src/styles.css`

Read `docs/frontend.md` and the target component before editing; read `tutorial.tsx` and the compliance rules before changing tutorial copy.

## Avoid

Do not add a router, state library, component library, remote asset loading, or client-side telemetry. Do not commit `frontend/dist` or `frontend/node_modules`.

## Verification

Run `npm --prefix frontend run build` (TypeScript plus Vite production build). Report the build result and whether the behavior was confirmed in the running Wails window.