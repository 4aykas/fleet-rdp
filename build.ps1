# Builds dist/fleet-rdp-launcher.exe (the template, GUI subsystem) and
# dist/fleet-rdp.exe (the CLI), both windows/amd64, and prints their SHA-256.
# -trimpath keeps the output independent of the checkout path.
# GOOS, GOARCH and CGO_ENABLED are set for the build only and restored after.
$ErrorActionPreference = 'Stop'
Set-Location -LiteralPath $PSScriptRoot

$saved = @{}
foreach ($n in 'GOOS', 'GOARCH', 'CGO_ENABLED') {
    $saved[$n] = [Environment]::GetEnvironmentVariable($n, 'Process')
}
try {
    $env:GOOS = 'windows'
    $env:GOARCH = 'amd64'
    $env:CGO_ENABLED = '0'

    New-Item -ItemType Directory -Force -Path dist | Out-Null

    go build -trimpath -ldflags '-H windowsgui -s -w' -o dist/fleet-rdp-launcher.exe ./cmd/launcher
    if ($LASTEXITCODE -ne 0) { throw 'building the launcher failed' }

    go build -trimpath -ldflags '-s -w' -o dist/fleet-rdp.exe ./cmd/fleet-rdp
    if ($LASTEXITCODE -ne 0) { throw 'building the CLI failed' }

    foreach ($f in 'dist/fleet-rdp-launcher.exe', 'dist/fleet-rdp.exe') {
        '{0}  {1}' -f (Get-FileHash -LiteralPath $f -Algorithm SHA256).Hash.ToLower(), (Split-Path -Leaf $f)
    }
}
finally {
    foreach ($n in $saved.Keys) {
        [Environment]::SetEnvironmentVariable($n, $saved[$n], 'Process')
    }
}
