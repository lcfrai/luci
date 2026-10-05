#requires -Version 7.0
param([string]$Version = '0.4.16', [switch]$Native)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
if ($Version -notmatch '^\d+\.\d+\.\d+(?:-[a-zA-Z0-9.-]+)?$') { throw 'Version must be a semantic version.' }
$luciRoot = [IO.Path]::GetFullPath((Split-Path -Parent $PSScriptRoot))
$luciOutput = Join-Path $luciRoot ('artifacts/releases/' + $Version)
$luciStage = Join-Path $luciOutput ('.stage-' + [guid]::NewGuid().ToString('N'))
$luciNativeRoot = Join-Path $luciRoot 'desktop/src-tauri'
$luciSavedEnvironment = @{}
foreach ($luciKey in @('PATH','GOOS','GOARCH','CGO_ENABLED','GOFLAGS','CARGO_TARGET_DIR')) { $luciSavedEnvironment[$luciKey] = [Environment]::GetEnvironmentVariable($luciKey, 'Process') }

function Confirm-Exit([string]$Operation) {
    if ($LASTEXITCODE -ne 0) { throw ($Operation + ' failed (exit ' + $LASTEXITCODE + ').') }
}
function Copy-Tree([string]$Source, [string]$Destination) {
    if (-not (Test-Path -LiteralPath $Source -PathType Container)) { throw ('Missing source directory: ' + $Source) }
    $luciInputs = @((Get-Item -LiteralPath $Source -Force)) + @(Get-ChildItem -LiteralPath $Source -Recurse -Force)
    foreach ($luciInput in $luciInputs) { if ($luciInput.Attributes -band [IO.FileAttributes]::ReparsePoint) { throw ('Linked release input refused: ' + $luciInput.FullName) } }
    New-Item -ItemType Directory -Force -Path $Destination | Out-Null
    foreach ($luciFile in $luciInputs | Where-Object { -not $_.PSIsContainer }) {
        $luciRelative = [IO.Path]::GetRelativePath($Source, $luciFile.FullName)
        if ($luciRelative -match '(^|[\\/])(\.git|node_modules|target|artifacts)([\\/]|$)' -or $luciFile.Name -match '^\.env(?:\.|$)' -or $luciFile.Extension -eq '.exe') { throw ('Unexpected file in allowlisted source tree: ' + $luciRelative) }
        $luciDestinationFile = Join-Path $Destination $luciRelative
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $luciDestinationFile) | Out-Null
        Copy-Item -LiteralPath $luciFile.FullName -Destination $luciDestinationFile -Force
    }
}
function Replace-GeneratedTree([string]$Source, [string]$Destination) {
    $luciAbsolute = [IO.Path]::GetFullPath($Destination)
    $luciAllowed = @('runtime','licenses','cloud') | ForEach-Object { [IO.Path]::GetFullPath((Join-Path $luciNativeRoot $_)) }
    $luciAllowed += @('luci-windows-amd64','luci-linux-amd64','luci-darwin-arm64') | ForEach-Object { [IO.Path]::GetFullPath((Join-Path $luciOutput $_)) }
    if ($luciAllowed -notcontains $luciAbsolute -or -not $luciAbsolute.StartsWith($luciRoot + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) { throw ('Refusing to replace an unrecognised generated directory: ' + $Destination) }
    $luciAncestor = $luciAbsolute
    while ($luciAncestor.Length -gt $luciRoot.Length) {
        if (Test-Path -LiteralPath $luciAncestor) {
            if ((Get-Item -LiteralPath $luciAncestor -Force).Attributes -band [IO.FileAttributes]::ReparsePoint) { throw ('Refusing a linked generated path: ' + $luciAncestor) }
        }
        $luciAncestor = Split-Path -Parent $luciAncestor
    }
    if (Test-Path -LiteralPath $luciAbsolute) { Remove-Item -LiteralPath $luciAbsolute -Recurse -Force }
    Copy-Item -LiteralPath $Source -Destination $luciAbsolute -Recurse -Force
}
function Confirm-Licenses([string]$Directory) {
    $luciManifest = Get-Content -Raw -LiteralPath (Join-Path $Directory 'manifest.json') | ConvertFrom-Json
    if (@($luciManifest.missingLicenses).Count -ne 0 -or (Get-Item -LiteralPath (Join-Path $Directory 'MISSING-LICENSES.txt')).Length -ne 0) { throw 'Incomplete dependency licence evidence.' }
    return @($luciManifest.components).Count
}
function Get-SourceMetadata([string]$Directory) {
    # An exported source tree has no Git history. Do not accidentally report an
    # enclosing checkout's commit, and do not label unknown provenance as clean.
    if (-not (Test-Path -LiteralPath (Join-Path $Directory '.git'))) {
        return @{ sourceCommit=$null; sourceDirty=$null }
    }
    Get-Command git -ErrorAction Stop | Out-Null
    $luciCommit = (& git -C $Directory rev-parse --verify HEAD)
    Confirm-Exit 'Source commit lookup'
    if ($luciCommit -notmatch '^[0-9a-f]{40}(?:[0-9a-f]{24})?$') { throw 'Git did not return a full source commit.' }
    $luciChanges = @(& git -C $Directory status --porcelain=v1 --untracked-files=all)
    Confirm-Exit 'Source working-tree status'
    return @{ sourceCommit=[string]$luciCommit; sourceDirty=($luciChanges.Count -gt 0) }
}

try {
    # Capture before generated files are staged; source metadata describes the
    # inputs at build start, not a later repository state.
    $luciSource = Get-SourceMetadata $luciRoot
    $luciPackageVersion = (Get-Content -Raw -LiteralPath (Join-Path $luciRoot 'desktop/package.json') | ConvertFrom-Json).version
    $luciTauriVersion = (Get-Content -Raw -LiteralPath (Join-Path $luciNativeRoot 'tauri.conf.json') | ConvertFrom-Json).version
    $luciCargoVersion = [regex]::Match((Get-Content -Raw -LiteralPath (Join-Path $luciNativeRoot 'Cargo.toml')), '(?m)^version\s*=\s*"([^"]+)"').Groups[1].Value
    $luciApiVersion = [regex]::Match((Get-Content -Raw -LiteralPath (Join-Path $luciRoot 'control-plane/internal/product/server.go')), 'const Version = "([^"]+)"').Groups[1].Value
    foreach ($luciDeclared in @($luciPackageVersion,$luciTauriVersion,$luciCargoVersion,$luciApiVersion)) { if ($luciDeclared -ne $Version) { throw ('Version mismatch: requested ' + $Version + ', source declares ' + $luciDeclared) } }
    $luciCargoBin = Join-Path $env:USERPROFILE '.cargo/bin'
    if (Test-Path -LiteralPath $luciCargoBin) { $env:PATH = $luciCargoBin + [IO.Path]::PathSeparator + $env:PATH }
    foreach ($luciTool in @('go','python','npm')) { Get-Command $luciTool -ErrorAction Stop | Out-Null }
    # npm.ps1 reparses MyInvocation.Statement and can corrupt arguments inside a
    # script block. Use Node's command shim explicitly on Windows.
    $luciNpm = if ($IsWindows) { (Get-Command npm.cmd -ErrorAction Stop).Source } else { (Get-Command npm -ErrorAction Stop).Source }
    if ($Native) { Get-Command cargo -ErrorAction Stop | Out-Null }
    New-Item -ItemType Directory -Force -Path $luciStage | Out-Null
    Push-Location (Join-Path $luciRoot 'desktop')
    try { & $luciNpm run build; Confirm-Exit 'Desktop production build' } finally { Pop-Location }

    # Runtime allowlist: only the sandbox build context, never the whole repository or user state.
    $luciRuntime = Join-Path $luciStage 'runtime'
    foreach ($luciTree in @('sandboxd/cmd','sandboxd/internal','sandboxd/vendor')) { Copy-Tree (Join-Path $luciRoot $luciTree) (Join-Path $luciRuntime $luciTree) }
    foreach ($luciFile in @('sandboxd/go.mod','sandboxd/go.sum','images/sandbox/Dockerfile','images/sandbox/entrypoint.sh')) {
        $luciDestination = Join-Path $luciRuntime $luciFile
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $luciDestination) | Out-Null
        Copy-Item -LiteralPath (Join-Path $luciRoot $luciFile) -Destination $luciDestination
    }
    $luciTargets = @(@{os='windows';arch='amd64';suffix='.exe'},@{os='linux';arch='amd64';suffix=''},@{os='darwin';arch='arm64';suffix=''})
    $luciPackages = @()
    $env:CGO_ENABLED = '0'; $env:GOFLAGS = '-mod=readonly'
    foreach ($luciTarget in $luciTargets) {
        $luciName = 'luci-' + $luciTarget.os + '-' + $luciTarget.arch
        $luciPackage = Join-Path $luciStage $luciName
        New-Item -ItemType Directory -Path $luciPackage | Out-Null
        $env:GOOS = $luciTarget.os; $env:GOARCH = $luciTarget.arch
        Push-Location $luciRoot
        try { & go build -trimpath -ldflags '-s -w' -o (Join-Path $luciPackage ('luci' + $luciTarget.suffix)) ./control-plane/cmd/api; Confirm-Exit ($luciName + ' service build') } finally { Pop-Location }
        Copy-Tree (Join-Path $luciRoot 'desktop/dist') (Join-Path $luciPackage 'web')
        Copy-Tree $luciRuntime (Join-Path $luciPackage 'runtime')
        foreach ($luciDoc in @('LICENSE','NOTICE','README.md','DESIGN.md')) { Copy-Item -LiteralPath (Join-Path $luciRoot $luciDoc) -Destination $luciPackage }
        Copy-Tree (Join-Path $luciRoot 'docs/luci') (Join-Path $luciPackage 'docs/luci')
        foreach ($luciDoc in @('UAT.md','RUNBOOK.md')) { Copy-Item -LiteralPath (Join-Path $luciRoot ('docs/' + $luciDoc)) -Destination (Join-Path $luciPackage ('docs/' + $luciDoc)) }
        $luciLicenses = Join-Path $luciPackage 'licenses'
        & python (Join-Path $PSScriptRoot 'collect-licenses.py') $luciRoot $luciLicenses
        Confirm-Exit ($luciName + ' strict licence collection')
        $luciCount = Confirm-Licenses $luciLicenses
        if ($luciTarget.os -eq 'windows') {
            Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'launch-windows.ps1') -Destination (Join-Path $luciPackage 'Start-Luci.ps1')
            Set-Content -LiteralPath (Join-Path $luciPackage 'Start-Luci.cmd') -Value '@powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0Start-Luci.ps1"' -Encoding ascii
        } else { Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'launch-unix.sh') -Destination (Join-Path $luciPackage 'start-luci.sh') }
        $luciPackages += @{ name=$luciName; os=$luciTarget.os; path=$luciPackage; licenses=$luciCount }
    }

    # Archive the Linux core BEFORE inserting cloud/ into portable packages. It cannot contain itself.
    $luciCloudRoot = Join-Path $luciStage 'luci-cloud-linux-amd64'
    $luciLinux = ($luciPackages | Where-Object { $_.os -eq 'linux' }).path
    Copy-Item -LiteralPath $luciLinux -Destination $luciCloudRoot -Recurse
    & python (Join-Path $PSScriptRoot 'package.py') $luciCloudRoot linux
    Confirm-Exit 'Linux cloud runtime archive'
    $luciCloudArchive = $luciCloudRoot + '.tar.gz'
    $luciArtifacts = @($luciCloudArchive)
    foreach ($luciPackage in $luciPackages) {
        $luciCloudDestination = Join-Path $luciPackage.path 'cloud'
        New-Item -ItemType Directory -Path $luciCloudDestination | Out-Null
        Copy-Item -LiteralPath $luciCloudArchive -Destination $luciCloudDestination
        & python (Join-Path $PSScriptRoot 'package.py') $luciPackage.path $luciPackage.os
        Confirm-Exit ($luciPackage.name + ' archive')
        $luciArtifacts += $luciPackage.path + $(if ($luciPackage.os -eq 'windows') { '.zip' } else { '.tar.gz' })
    }

    $luciNativeCount = 0
    if ($Native) {
        $env:GOOS = 'windows'; $env:GOARCH = 'amd64'
        $luciNativeLicenses = Join-Path $luciStage 'native-licenses'
        & python (Join-Path $PSScriptRoot 'collect-licenses.py') $luciRoot $luciNativeLicenses --native
        Confirm-Exit 'Native strict licence collection'
        $luciNativeCount = Confirm-Licenses $luciNativeLicenses
        Replace-GeneratedTree $luciRuntime (Join-Path $luciNativeRoot 'runtime')
        Replace-GeneratedTree $luciNativeLicenses (Join-Path $luciNativeRoot 'licenses')
        Replace-GeneratedTree (Join-Path (($luciPackages | Where-Object { $_.os -eq 'windows' }).path) 'cloud') (Join-Path $luciNativeRoot 'cloud')
        $luciSidecars = Join-Path $luciNativeRoot 'binaries'
        New-Item -ItemType Directory -Force -Path $luciSidecars | Out-Null
        Copy-Item -LiteralPath (Join-Path (($luciPackages | Where-Object { $_.os -eq 'windows' }).path) 'luci.exe') -Destination (Join-Path $luciSidecars 'luci-api-x86_64-pc-windows-msvc.exe') -Force
        $env:CARGO_TARGET_DIR = Join-Path $luciNativeRoot 'target'
        $luciTauriOverride = Join-Path $luciStage 'tauri.release.json'
        Set-Content -LiteralPath $luciTauriOverride -Value '{"build":{"beforeBuildCommand":""}}' -Encoding utf8NoBOM
        Push-Location (Join-Path $luciRoot 'desktop')
        try { & $luciNpm run tauri -- build --config $luciTauriOverride --bundles nsis -- --locked; Confirm-Exit 'Native NSIS build' } finally { Pop-Location }
        $luciInstaller = Join-Path $env:CARGO_TARGET_DIR ('release/bundle/nsis/Luci_' + $Version + '_x64-setup.exe')
        if (-not (Test-Path -LiteralPath $luciInstaller -PathType Leaf)) { throw ('Missing expected installer: ' + $luciInstaller) }
        $luciArtifacts += $luciInstaller
    }
    $luciArtifactRecords = @()
    foreach ($luciArtifact in $luciArtifacts) {
        $luciDestination = Join-Path $luciOutput (Split-Path -Leaf $luciArtifact)
        Copy-Item -LiteralPath $luciArtifact -Destination $luciDestination -Force
        $luciArtifactRecords += @{name=(Split-Path -Leaf $luciArtifact); bytes=(Get-Item -LiteralPath $luciDestination).Length; sha256=(Get-FileHash -Algorithm SHA256 -LiteralPath $luciDestination).Hash.ToLowerInvariant()}
    }
    foreach ($luciPackage in $luciPackages) { Replace-GeneratedTree $luciPackage.path (Join-Path $luciOutput $luciPackage.name) }
    $luciHashes = $luciArtifactRecords | Sort-Object name | ForEach-Object { $_.sha256 + '  ' + $_.name }
    [IO.File]::WriteAllLines((Join-Path $luciOutput 'SHA256SUMS'), [string[]]$luciHashes, [Text.UTF8Encoding]::new($false))
    $luciRelease = @{version=$Version;sourceCommit=$luciSource.sourceCommit;sourceDirty=$luciSource.sourceDirty;builtAt=[DateTime]::UtcNow.ToString('o');artifacts=$luciArtifactRecords;portableLicenseComponents=@($luciPackages | ForEach-Object {@{target=$_.name;count=$_.licenses;missing=0}});nativeLicenseComponents=$luciNativeCount;nativeRequested=[bool]$Native;stage=$luciStage}
    $luciRelease | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath (Join-Path $luciOutput 'release-manifest.json') -Encoding utf8NoBOM
    Write-Host ('Reviewable Luci packages: ' + $luciOutput)
} finally {
    foreach ($luciKey in $luciSavedEnvironment.Keys) { [Environment]::SetEnvironmentVariable($luciKey, $luciSavedEnvironment[$luciKey], 'Process') }
}
