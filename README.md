# fleet-rdp

One-click RDP launcher for Windows. A launcher is a small `.exe` that carries
one connection (host, port, login, password). A double-click saves the login in
Windows Credential Manager and opens `mstsc` full screen, with no prompts.

The `.exe` is as secret as the password it carries. Treat it that way.

## Build

    pwsh ./build.ps1

Produces `dist/fleet-rdp-launcher.exe` (the template; do not run it) and
`dist/fleet-rdp.exe` (the CLI), and prints their SHA-256. Both are
windows/amd64. On another OS, `go build ./cmd/fleet-rdp` builds the CLI.

## Make a launcher

    set FLEET_RDP_PASSWORD=...
    fleet-rdp build --host 10.0.0.5 --port 2157 --user vlad --name SGCLOUD -o SGCLOUD.exe

- The password comes from `FLEET_RDP_PASSWORD`, or from stdin (a hidden prompt
  on a terminal). Never from an argument.
- `--template` defaults to `fleet-rdp-launcher.exe` beside the CLI. A template
  that already carries a payload is refused.
- `--cert-sha256 <64 hex>` pins the server certificate. Without it the first
  run trusts the certificate the server shows (trust on first use).
- `fleet-rdp inspect SGCLOUD.exe` prints the payload with the password as `***`.

The launcher accepts `--single` and `--multi` to override the monitor
detection (`/multimon` is used when Windows reports more than one monitor).

## Payload format

See `payload/payload.go` and `testdata/payload-v1.*`.
