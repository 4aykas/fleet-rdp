# Builds dist/fleet-rdp-launcher.exe (the template, GUI subsystem) and
# dist/fleet-rdp.exe (the CLI), both windows/amd64, and prints their SHA-256.
# -trimpath keeps the output independent of the checkout path.
$ErrorActionPreference = 'Stop'
Set-Location -LiteralPath $PSScriptRoot

$env:GOOS = 'windows'
$env:GOARCH = 'amd64'
$env:CGO_ENABLED = '0'

New-Item -ItemType Directory -Force -Path dist | Out-Null

go build -trimpath -ldflags '-H windowsgui -s -w' -o dist/fleet-rdp-launcher.exe ./cmd/launcher
if ($LASTEXITCODE -ne 0) { throw 'building the launcher failed' }

go build -trimpath -ldflags '-s -w' -o dist/fleet-rdp.exe ./cmd/fleet-rdp
if ($LASTEXITCODE -ne 0) { throw 'building the CLI failed' }

Get-ChildItem dist/*.exe | ForEach-Object {
    '{0}  {1}' -f (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLower(), $_.Name
}
