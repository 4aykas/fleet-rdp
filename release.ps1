# Cuts a release: checks, builds, tags, publishes to GitHub and verifies the
# published launcher byte for byte.
#
#   pwsh ./release.ps1 -Version v0.1.2 -Notes 'what changed'
#   pwsh ./release.ps1 -Version v0.1.2 -Notes 'x' -DryRun -ProbeHost H -ProbePort P
#
# Steps (each stops the release on failure):
#   a. -Version is vX.Y.Z and the tag exists neither locally nor on origin.
#   b. The working tree is clean (untracked files count) and HEAD equals
#      origin/main after a fetch, so what is released is what is pushed.
#   c. ./build.ps1, then SHA-256 of both executables.
#   d. Optional live smoke test: dist/fleet-rdp.exe probe <ProbeHost> <ProbePort>.
#   e. Annotated tag, push of the tag, gh release create with both assets. The
#      notes are -Notes, a blank line and the two "sha256  name" lines.
#   f. The launcher is downloaded from the release and its SHA-256 compared
#      with the local one.
#   g. Prints the pin command for the FleetView repo and the template hash.
#
# -DryRun does a-d and prints what e-g would do.
# -SkipPushCheck skips the "everything is pushed" half of step b. Testing only:
#   a real release must never use it.
param(
    [Parameter(Mandatory)][string]$Version,
    [Parameter(Mandatory)][string]$Notes,
    [switch]$DryRun,
    [string]$ProbeHost,
    [int]$ProbePort,
    [switch]$SkipPushCheck
)
$ErrorActionPreference = 'Stop'
Set-Location -LiteralPath $PSScriptRoot

$repo = '4aykas/fleet-rdp'

function Invoke-Native {
    # Runs a native command, returns its stdout lines, throws on a non-zero exit.
    param([Parameter(Mandatory)][string]$What, [Parameter(Mandatory)][scriptblock]$Command)
    $out = & $Command
    if ($LASTEXITCODE -ne 0) { throw "$What failed (exit $LASTEXITCODE)" }
    $out
}

function Get-Sha256([string]$Path) {
    (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLower()
}

# a. version and tag
if ($Version -notmatch '^v\d+\.\d+\.\d+$') { throw "Version '$Version' is not vX.Y.Z" }
if (Invoke-Native 'git tag' { git tag --list $Version }) { throw "tag $Version already exists locally" }
Invoke-Native 'git fetch' { git fetch origin --tags --quiet } | Out-Null
if (Invoke-Native 'git ls-remote' { git ls-remote --tags origin "refs/tags/$Version" }) { throw "tag $Version already exists on origin" }
if ($SkipPushCheck -and -not $DryRun) { throw '-SkipPushCheck is for -DryRun only: a release is always built from a pushed commit' }
if ($ProbeHost -and $ProbePort -lt 1) { throw '-ProbeHost needs -ProbePort' }
if ($ProbePort -and -not $ProbeHost) { throw '-ProbePort needs -ProbeHost' }

# b. clean and pushed
if (Invoke-Native 'git status' { git status --porcelain }) { throw 'the working tree is not clean (untracked files count)' }
if ($SkipPushCheck) {
    Write-Warning '-SkipPushCheck: not checking that HEAD is pushed (testing only)'
}
else {
    $head = Invoke-Native 'git rev-parse HEAD' { git rev-parse HEAD }
    $remote = Invoke-Native 'git rev-parse origin/main' { git rev-parse origin/main }
    if ($head -ne $remote) { throw "HEAD ($head) is not origin/main ($remote): push first" }
}

# c. build and hash
& ./build.ps1 | Out-Host
$launcher = 'dist/fleet-rdp-launcher.exe'
$cli = 'dist/fleet-rdp.exe'
$launcherSha = Get-Sha256 $launcher
$cliSha = Get-Sha256 $cli
$hashLines = @("$launcherSha  fleet-rdp-launcher.exe", "$cliSha  fleet-rdp.exe")

# d. live smoke test
if ($ProbeHost) {
    $line = Invoke-Native 'fleet-rdp probe' { & $cli probe $ProbeHost $ProbePort }
    "probe ${ProbeHost}:${ProbePort}  $line"
}

$notesText = (@($Notes, '') + $hashLines) -join "`n"

if ($DryRun) {
    "DRY RUN: would tag $Version, push it, and run:"
    "  gh release create $Version --repo $repo --title $Version --verify-tag $launcher $cli --notes <below>"
    $notesText -split "`n" | ForEach-Object { "    | $_" }
    "DRY RUN: would download https://github.com/$repo/releases/download/$Version/fleet-rdp-launcher.exe and expect $launcherSha"
    "pin: node scripts/pin-launcher.mjs $Version"
    "template sha256: $launcherSha"
    return
}

# e. tag and release
Invoke-Native 'git tag' { git tag -a $Version -m $Version } | Out-Null
Invoke-Native 'git push tag' { git push origin $Version } | Out-Null
Invoke-Native 'gh release create' {
    gh release create $Version --repo $repo --title $Version --verify-tag --notes $notesText $launcher $cli
} | Out-Host

# f. verify the published launcher
$tmp = Join-Path ([IO.Path]::GetTempPath()) ("fleet-rdp-release-" + [guid]::NewGuid() + '.exe')
try {
    Invoke-WebRequest -Uri "https://github.com/$repo/releases/download/$Version/fleet-rdp-launcher.exe" -OutFile $tmp
    $published = Get-Sha256 $tmp
}
finally {
    Remove-Item -LiteralPath $tmp -Force -ErrorAction SilentlyContinue
}
if ($published -ne $launcherSha) { throw "the published launcher ($published) differs from the local one ($launcherSha)" }
"published launcher matches the local build"

# g. last: what to do next
"pin: node scripts/pin-launcher.mjs $Version"
"template sha256: $launcherSha"
