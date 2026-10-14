# Builds dist/fleet-rdp-launcher.exe (the template, GUI subsystem),
# dist/fleet-rdp.exe (the CLI) and dist/fleet-rdp-maker.exe (the maker, GUI),
# all windows/amd64, and prints their SHA-256. The launcher is copied into
# cmd/maker/template/ and embedded in the maker, so the maker always carries
# the launcher from the same run.
# -trimpath keeps the output independent of the checkout path.
# GOOS, GOARCH and CGO_ENABLED are set for the build only and restored after.
#
# -Version vX.Y.Z goes into the executables' file properties (default v0.0.0).
# The default icon (tools/mkicon) and the .syso resource files are generated
# here on every build and are not committed.
param([string]$Version = 'v0.0.0')
$ErrorActionPreference = 'Stop'
Set-Location -LiteralPath $PSScriptRoot

if ($Version -notmatch '^v\d+\.\d+\.\d+$') { throw "Version '$Version' is not vX.Y.Z" }

# Runs on the host (GOOS/GOARCH are not set yet), so the tools run here.
$fv = ($Version.TrimStart('v')) + '.0'
go run ./tools/mkicon cmd/launcher/winres/icon.ico
if ($LASTEXITCODE -ne 0) { throw 'drawing the default icon failed' }
go run ./tools/mkicon -host fleet-rdp-maker cmd/maker/winres/icon.ico
if ($LASTEXITCODE -ne 0) { throw 'drawing the maker icon failed' }
foreach ($cmd in 'launcher', 'fleet-rdp', 'maker') {
    go run github.com/tc-hib/go-winres@v0.3.3 make --in "cmd/$cmd/winres/winres.json" --out "cmd/$cmd/rsrc" --arch amd64 --file-version $fv --product-version $fv
    if ($LASTEXITCODE -ne 0) { throw "making the $cmd resources failed" }
}

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

    Copy-Item -LiteralPath dist/fleet-rdp-launcher.exe -Destination cmd/maker/template/fleet-rdp-launcher.exe -Force

    go build -trimpath -ldflags '-H windowsgui -s -w' -o dist/fleet-rdp-maker.exe ./cmd/maker
    if ($LASTEXITCODE -ne 0) { throw 'building the maker failed' }

    go build -trimpath -ldflags '-s -w' -o dist/fleet-rdp.exe ./cmd/fleet-rdp
    if ($LASTEXITCODE -ne 0) { throw 'building the CLI failed' }

    foreach ($f in 'dist/fleet-rdp-launcher.exe', 'dist/fleet-rdp.exe', 'dist/fleet-rdp-maker.exe') {
        '{0}  {1}' -f (Get-FileHash -LiteralPath $f -Algorithm SHA256).Hash.ToLower(), (Split-Path -Leaf $f)
    }
}
finally {
    foreach ($n in $saved.Keys) {
        [Environment]::SetEnvironmentVariable($n, $saved[$n], 'Process')
    }
}
