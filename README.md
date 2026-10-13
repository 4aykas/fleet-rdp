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

    fleet-rdp build --host 10.0.0.5 --port 2157 --user vlad --name SGCLOUD -o SGCLOUD.exe

- The password is asked for at a hidden prompt; nothing is echoed and nothing
  lands in the shell history. This is the way to give it. It is never taken
  from an argument.
- `FLEET_RDP_PASSWORD` is for scripts that pull the password from a secret
  store and set it for one process:

      cmd:         set FLEET_RDP_PASSWORD=...
      PowerShell:  $env:FLEET_RDP_PASSWORD = "..."

  Do not type the password into an interactive shell line: it goes into the
  history (PSReadLine saves it to `ConsoleHost_history.txt`).
- The built `.exe` carries the password. Keep it in a folder only its owner can
  read, and send it only to the person it is for.
- `--template` defaults to `fleet-rdp-launcher.exe` beside the CLI. A template
  that already carries a payload is refused.
- `--cert-sha256 <64 hex>` pins the server certificate. Without it the first
  run trusts the certificate the server shows (trust on first use).
- `fleet-rdp inspect SGCLOUD.exe` prints the payload with the password as `***`.
- After the server's RDP certificate renews, `mstsc` shows its certificate
  dialog once (the stored pin is kept; "zero clicks" holds until the renewal).

- `fleet-rdp probe HOST PORT` connects the way an RDP client does and prints
  `<sha256 of the server certificate>  <elapsed>`. Use it to see what
  certificate a server shows, or to get the value for `--cert-sha256`.

The launcher accepts `--single` and `--multi` to override the monitor
detection (`/multimon` is used when Windows reports more than one monitor).

## Payload format

See `payload/payload.go` and `testdata/payload-v1.*`.
