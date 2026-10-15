# fleet-rdp

One-click RDP launcher. Spec and plan live in the FleetView repo (`rmdi`):
`docs/superpowers/specs/2026-10-12-rdp-launcher-design.md`.

## Rules

- Prefer the gopls MCP tools (references, definitions, diagnostics) when they are available.
- Never modify `credstore/`. It is finished. The launcher only calls `credstore.Write`.
- The password never appears in an error, a log line, an argument or a test name.
- The payload format is frozen per `v`. `testdata/payload-v1.*` is copied into FleetView's dashboard; change it only together with a new `v` and a matching change there.
- invader/ and peicon/ are ports of FleetView's dashboard/src/utils/invader.ts and peIcon.ts; testdata/invader-v1.json is copied into FleetView like the payload vectors.
- Dependencies: stdlib, `golang.org/x/sys`, `golang.org/x/term`, `github.com/danieljoos/wincred` (inside `credstore` only).

Findings left for later live in `docs/deferred.md`; read it before working in a package it names.

## Checks

`.githooks/pre-commit` runs gofmt, vet, tests and the linux and windows builds. Enable it once per clone: `git config core.hooksPath .githooks`.

The maker window is checked by hand with `tools/uidrive/` (screenshot, set/click/read by control ID, Save As to an absolute path); each script's header shows its call. PowerShell variables ignore case, so a `$t` path clobbers a `$T` alias. The maker's WM_ACTIVATE handling (collapse More on deactivate, focus back on the Server field on activate) was checked by script only: synthetic WM_ACTIVATE inactive then active, `GetGUIThreadInfo` showed focus on the Server field; a real Alt-Tab needs an interactive desktop.
