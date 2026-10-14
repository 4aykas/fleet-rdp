# Deferred findings

Review findings judged too small to fix at the time. Each says where, what,
and why it was left. Fix one, delete its line in the same commit. When a plan's
working folder (`.superpowers/sdd/`) is deleted, its open minors move here first.

## From the launcher icons and monitors plan (2026-10-13)

- `payload/payload.go`: the CLI default writes `"monitors":"auto"` into every payload. Harmless (`auto` means the same as empty); the brief asked for it.
- `cmd/fleet-rdp` run test, "two" case: does not assert that stderr mentions monitors.
- `launch/mstsc.go` `TestChooseMulti`: the "unknown flags ignored" case was dropped from the table.
- `cmd/launcher/run.go` test: `r.calls[:5]` panics instead of failing when fewer calls were made.
- `build.ps1` / `release.ps1`: the version regex is case-insensitive, so `V1.2.3` passes.
- `tools/mkicon` icon test: does not assert `biPlanes == 1` / `biCompression == 0`.
- A stale `.syso` can be picked up by a hand `go build`; worth a README note.

## From maker v0.1.4 (2026-10-14)

- `cmd/maker/main_windows.go` WM_ACTIVATE: the fallback to the Server field when the saved control is hidden or disabled builds and vets, but was never driven by hand (`tools/uidrive`).
- v0.1.4 itself is on `main` but not released; nothing user-visible changed beyond the Save As message.
