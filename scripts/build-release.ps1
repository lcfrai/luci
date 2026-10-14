#requires -Version 7.0
param([string]$Version = '0.5.0', [switch]$Native, [switch]$RequireSigned, [string]$CertificateThumbprint = '', [string]$SignToolPath = '', [string]$TimestampUrl = 'https://timestamp.digicert.com', [Parameter(Mandatory=$true)][string]$RuntimePayloadDir, [ValidateSet('v2','advanced-v1')][string]$RuntimeFormat = 'v2', [ValidateSet('all','windows-amd64','linux-amd64','darwin-arm64')][string]$Target = 'all', [string]$E2BSource = '')
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
if ($RequireSigned -and (-not $Native -or -not $CertificateThumbprint)) { throw 'RequireSigned requires -Native and an explicit CertificateThumbprint; unsigned fallback is refused.' }
if ($CertificateThumbprint -and -not $Native) { throw 'CertificateThumbprint requires -Native.' }
if ($Version -notmatch '^\d+\.\d+\.\d+(?:-[a-zA-Z0-9.-]+)?$') { throw 'Version must be a semantic version.' }
$luciRoot = [IO.Path]::GetFullPath((Split-Path -Parent $PSScriptRoot))
$luciOutput = Join-Path $luciRoot ('artifacts/releases/' + $Version)
$luciStage = Join-Path $luciOutput ('.stage-' + [guid]::NewGuid().ToString('N'))
$luciNativeRoot = Join-Path $luciRoot 'desktop/src-tauri'
$luciSavedEnvironment = @{}
$luciPayloadTool = Join-Path $PSScriptRoot $(if ($RuntimeFormat -eq 'v2') { 'runtime-oci/pull_payload.py' } else { 'runtime-oci/payload.py' })
$luciBuildSucceeded = $false
. (Join-Path $PSScriptRoot 'owned-build-stage.ps1')
. (Join-Path $PSScriptRoot 'windows-signing.ps1')
$luciSigningContext = $null
$luciSigningFiles = [Collections.Generic.List[object]]::new()
$luciNativeSigning = @{requested=$false;verified=$false;status='unsigned';required=[bool]$RequireSigned;files=@()}
# Refuse missing signer/tool before any build, payload staging or package writes.
if ($CertificateThumbprint) {
    $luciSigningContext = Get-LuciWindowsSigningContext $CertificateThumbprint $TimestampUrl $SignToolPath
    $luciNativeSigning = @{requested=$true;verified=$false;status='pending';required=[bool]$RequireSigned;certificateThumbprint=$luciSigningContext.thumbprint;signerSubject=$luciSigningContext.subject;timestampUrl=$TimestampUrl;files=@()}
}
foreach ($luciKey in @('PATH','GOOS','GOARCH','CGO_ENABLED','GOFLAGS','CARGO_TARGET_DIR','GOMAXPROCS','GOMEMLIMIT','NODE_OPTIONS','VITE_LUCI_API')) { $luciSavedEnvironment[$luciKey] = [Environment]::GetEnvironmentVariable($luciKey, 'Process') }

function Confirm-Exit([string]$Operation) {
    if ($LASTEXITCODE -ne 0) { throw ($Operation + ' failed (exit ' + $LASTEXITCODE + ').') }
}
function Copy-Tree([string]$Source, [string]$Destination, [string[]]$ExcludePaths = @()) {
    if (-not (Test-Path -LiteralPath $Source -PathType Container)) { throw ('Missing source directory: ' + $Source) }
    $luciInputs = @((Get-Item -LiteralPath $Source -Force)) + @(Get-ChildItem -LiteralPath $Source -Recurse -Force)
    foreach ($luciInput in $luciInputs) { if ($luciInput.Attributes -band [IO.FileAttributes]::ReparsePoint) { throw ('Linked release input refused: ' + $luciInput.FullName) } }
    New-Item -ItemType Directory -Force -Path $Destination | Out-Null
    foreach ($luciFile in $luciInputs | Where-Object { -not $_.PSIsContainer }) {
        $luciRelative = [IO.Path]::GetRelativePath($Source, $luciFile.FullName)
        $luciPortableRelative = $luciRelative.Replace('\','/')
        if (@($ExcludePaths | Where-Object { $luciPortableRelative -eq $_ -or $luciPortableRelative.StartsWith($_ + '/', [StringComparison]::Ordinal) }).Count -gt 0) { continue }
        if ($luciRelative -match '(^|[\\/])(\.git|node_modules|target|artifacts)([\\/]|$)' -or $luciFile.Name -match '^\.env(?:\.|$)' -or $luciFile.Extension -eq '.exe') { throw ('Unexpected file in allowlisted source tree: ' + $luciRelative) }
        $luciDestinationFile = Join-Path $Destination $luciRelative
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $luciDestinationFile) | Out-Null
        Copy-Item -LiteralPath $luciFile.FullName -Destination $luciDestinationFile -Force
    }
}
function Replace-GeneratedTree([string]$Source, [string]$Destination) {
    $luciAbsolute = [IO.Path]::GetFullPath($Destination)
    $luciAllowed = @('runtime','licenses','cloud','guides') | ForEach-Object { [IO.Path]::GetFullPath((Join-Path $luciNativeRoot $_)) }
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
    if ($luciSource.sourceDirty -ne $false) { throw 'Verified runtime distribution requires the full clean source checkout.' }
    if (-not [IO.Path]::IsPathFullyQualified($E2BSource) -or -not (Test-Path -LiteralPath $E2BSource -PathType Container) -or (Get-Item -LiteralPath $E2BSource -Force).Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'Supply -E2BSource with an existing absolute, ordinary pinned E2B checkout; this is a maintainer build input.' }
    if ($Native -and $Target -notin @('all','windows-amd64')) { throw 'Native Windows packaging requires a Windows target.' }
    & python (Join-Path $PSScriptRoot 'release-version.py') check $Version
    Confirm-Exit 'Authoritative release version alignment'
    $luciPackageVersion = (Get-Content -Raw -LiteralPath (Join-Path $luciRoot 'desktop/package.json') | ConvertFrom-Json).version
    $luciTauriVersion = (Get-Content -Raw -LiteralPath (Join-Path $luciNativeRoot 'tauri.conf.json') | ConvertFrom-Json).version
    $luciCargoVersion = [regex]::Match((Get-Content -Raw -LiteralPath (Join-Path $luciNativeRoot 'Cargo.toml')), '(?m)^version\s*=\s*"([^"]+)"').Groups[1].Value
    $luciApiVersion = [regex]::Match((Get-Content -Raw -LiteralPath (Join-Path $luciRoot 'control-plane/internal/product/server.go')), 'const Version = "([^"]+)"').Groups[1].Value
    foreach ($luciDeclared in @($luciPackageVersion,$luciTauriVersion,$luciCargoVersion,$luciApiVersion)) { if ($luciDeclared -ne $Version) { throw ('Version mismatch: requested ' + $Version + ', source declares ' + $luciDeclared) } }
    $luciCargoBin = Join-Path $env:USERPROFILE '.cargo/bin'
    if (Test-Path -LiteralPath $luciCargoBin) { $env:PATH = $luciCargoBin + [IO.Path]::PathSeparator + $env:PATH }
    foreach ($luciTool in @('go','python','npm')) { Get-Command $luciTool -ErrorAction Stop | Out-Null }
    & python $luciPayloadTool verify --payload $RuntimePayloadDir
    Confirm-Exit 'Preverified source-bound runtime payload'
    # npm.ps1 reparses MyInvocation.Statement and can corrupt arguments inside a
    # script block. Use Node's command shim explicitly on Windows.
    $luciNpm = if ($IsWindows) { (Get-Command npm.cmd -ErrorAction Stop).Source } else { (Get-Command npm -ErrorAction Stop).Source }
    if ($Native) { Get-Command cargo -ErrorAction Stop | Out-Null }
    New-Item -ItemType Directory -Force -Path $luciStage | Out-Null
    $env:GOMAXPROCS = '1'; $env:GOMEMLIMIT = '512MiB'; $env:NODE_OPTIONS = '--max-old-space-size=1024'
    [Environment]::SetEnvironmentVariable('VITE_LUCI_API', $null, 'Process')
    $luciRuntime = Join-Path $luciStage 'runtime'
    New-Item -ItemType Directory -Force -Path $luciRuntime | Out-Null
    $luciAzureArchive = Join-Path $luciRuntime 'azure-setup.tar'
    $luciAzureHash = (& python (Join-Path $PSScriptRoot 'azure-setup/package.py') build --source $luciRoot --revision $luciSource.sourceCommit --e2b-source $E2BSource --output $luciAzureArchive | Select-Object -Last 1)
    Confirm-Exit 'Source-bound Azure Develop setup bundle'
    if ($luciAzureHash -notmatch '^[0-9a-f]{64}$') { throw 'Azure Develop setup bundle hash is invalid.' }
    & python (Join-Path $PSScriptRoot 'azure-setup/package.py') verify --archive $luciAzureArchive --sha256 $luciAzureHash | Out-Null
    Confirm-Exit 'Azure Develop setup bundle verification'
    [IO.File]::WriteAllText((Join-Path $luciRuntime 'azure-setup.sha256'), ($luciAzureHash + "`n"), [Text.UTF8Encoding]::new($false))
    Push-Location (Join-Path $luciRoot 'desktop')
    try { & $luciNpm run build; Confirm-Exit 'Desktop production build' } finally { Pop-Location }

    $luciGuides = Join-Path $luciStage 'package-guides'
    $luciDirtyArgument = if ($null -eq $luciSource.sourceDirty) { 'unknown' } else { ([string]$luciSource.sourceDirty).ToLowerInvariant() }
    & python (Join-Path $PSScriptRoot 'stage-release-docs.py') $luciRoot $luciGuides "--source-commit=$($luciSource.sourceCommit)" "--source-dirty=$luciDirtyArgument"
    Confirm-Exit 'Source-bound package documentation'

    # Runtime allowlist: only the sandbox build context, never the whole repository or user state.
    foreach ($luciTree in @('sandboxd/cmd','sandboxd/internal','sandboxd/vendor')) { Copy-Tree (Join-Path $luciRoot $luciTree) (Join-Path $luciRuntime $luciTree) }
    foreach ($luciFile in @('sandboxd/go.mod','sandboxd/go.sum','images/sandbox/Dockerfile','images/sandbox/entrypoint.sh')) {
        $luciDestination = Join-Path $luciRuntime $luciFile
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $luciDestination) | Out-Null
        Copy-Item -LiteralPath (Join-Path $luciRoot $luciFile) -Destination $luciDestination
    }
    $luciTargets = @(@{os='windows';arch='amd64';suffix='.exe'},@{os='linux';arch='amd64';suffix=''},@{os='darwin';arch='arm64';suffix=''})
    # Linux management core is still needed by the supported compatibility
    # transfer; do not build the unrelated Darwin target for a Windows review.
    if ($Target -ne 'all') { $luciTargets = @($luciTargets | Where-Object { ($_.os + '-' + $_.arch) -eq $Target -or $_.os -eq 'linux' }) }
    $luciPackages = @()
    $env:CGO_ENABLED = '0'; $env:GOFLAGS = '-mod=readonly -p=1'
    foreach ($luciTarget in $luciTargets) {
        $luciName = 'luci-' + $luciTarget.os + '-' + $luciTarget.arch
        $luciPackage = Join-Path $luciStage $luciName
        New-Item -ItemType Directory -Path $luciPackage | Out-Null
        $env:GOOS = $luciTarget.os; $env:GOARCH = $luciTarget.arch
        Push-Location $luciRoot
        try { & go build -trimpath -ldflags '-s -w' -o (Join-Path $luciPackage ('luci' + $luciTarget.suffix)) ./control-plane/cmd/api; Confirm-Exit ($luciName + ' service build') } finally { Pop-Location }
        if ($luciTarget.os -eq 'windows' -and $luciSigningContext) {
            # Sign BEFORE package identity/archive creation. This exact signed core
            # is later copied as the native app's executable sidecar.
            $luciSigningFiles.Add((Set-LuciWindowsSignature (Join-Path $luciPackage 'luci.exe') $luciSigningContext 'windows-portable-core'))
        }
        Copy-Tree (Join-Path $luciRoot 'desktop/dist') (Join-Path $luciPackage 'web')
        Copy-Tree $luciRuntime (Join-Path $luciPackage 'runtime')
        Copy-Tree $luciGuides $luciPackage
        $luciLicenses = Join-Path $luciPackage 'licenses'
        & python (Join-Path $PSScriptRoot 'collect-licenses.py') $luciRoot $luciLicenses
        Confirm-Exit ($luciName + ' strict licence collection')
        $luciCount = Confirm-Licenses $luciLicenses
        & python $luciPayloadTool stage --payload $RuntimePayloadDir --repo $luciRoot --revision $luciSource.sourceCommit --package $luciPackage
        Confirm-Exit ($luciName + ' verified runtime payload and source receipt')
        if ($luciTarget.os -eq 'windows') {
            Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'launch-windows.ps1') -Destination (Join-Path $luciPackage 'Start-Luci.ps1')
            Set-Content -LiteralPath (Join-Path $luciPackage 'Start-Luci.cmd') -Value '@powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0Start-Luci.ps1"' -Encoding ascii
        } else { Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'launch-unix.sh') -Destination (Join-Path $luciPackage 'start-luci.sh') }
        $luciPackages += @{ name=$luciName; os=$luciTarget.os; path=$luciPackage; licenses=$luciCount; selected=($Target -eq 'all' -or $luciName -eq ('luci-' + $Target)) }
    }

    # Archive the Linux core BEFORE inserting cloud/ into portable packages. It cannot contain itself.
    $luciCloudRoot = Join-Path $luciStage 'luci-cloud-linux-amd64'
    $luciLinux = ($luciPackages | Where-Object { $_.os -eq 'linux' }).path
    & python $luciPayloadTool core-from-package --package $luciLinux --output $luciCloudRoot
    Confirm-Exit 'Explicit management-core runtime companion contract'
    & python (Join-Path $PSScriptRoot 'package.py') $luciCloudRoot linux
    Confirm-Exit 'Linux cloud runtime archive'
    $luciCloudArchive = $luciCloudRoot + '.tar.gz'
    $luciArtifacts = @($luciCloudArchive)
    foreach ($luciPackage in $luciPackages) {
        if (-not $luciPackage.selected) { continue }
        $luciCloudDestination = Join-Path $luciPackage.path 'cloud'
        New-Item -ItemType Directory -Path $luciCloudDestination | Out-Null
        Copy-Item -LiteralPath $luciCloudArchive -Destination $luciCloudDestination
        & python (Join-Path $PSScriptRoot 'package.py') $luciPackage.path $luciPackage.os
        Confirm-Exit ($luciPackage.name + ' archive')
        $luciArtifacts += $luciPackage.path + $(if ($luciPackage.os -eq 'windows') { '.zip' } else { '.tar.gz' })
    }

    $luciNativeCount = 0
    if ($Native) {
        $luciWindowsPackage = ($luciPackages | Where-Object { $_.os -eq 'windows' }).path
        $env:GOOS = 'windows'; $env:GOARCH = 'amd64'
        $luciNativeLicenses = Join-Path $luciStage 'native-licenses'
        & python (Join-Path $PSScriptRoot 'collect-licenses.py') $luciRoot $luciNativeLicenses --native
        Confirm-Exit 'Native strict licence collection'
        $luciNativeCount = Confirm-Licenses $luciNativeLicenses
        Copy-Tree (Join-Path $luciWindowsPackage 'licenses/runtime-image') (Join-Path $luciNativeLicenses 'runtime-image')
        if ($RuntimeFormat -eq 'v2') { Copy-Tree (Join-Path $luciWindowsPackage 'licenses/runtime-bootstrap') (Join-Path $luciNativeLicenses 'runtime-bootstrap') }
        Replace-GeneratedTree $luciGuides (Join-Path $luciNativeRoot 'guides')
        Replace-GeneratedTree (Join-Path $luciWindowsPackage 'runtime') (Join-Path $luciNativeRoot 'runtime')
        Replace-GeneratedTree $luciNativeLicenses (Join-Path $luciNativeRoot 'licenses')
        Replace-GeneratedTree (Join-Path (($luciPackages | Where-Object { $_.os -eq 'windows' }).path) 'cloud') (Join-Path $luciNativeRoot 'cloud')
        $luciSidecars = Join-Path $luciNativeRoot 'binaries'
        New-Item -ItemType Directory -Force -Path $luciSidecars | Out-Null
        Copy-Item -LiteralPath (Join-Path (($luciPackages | Where-Object { $_.os -eq 'windows' }).path) 'luci.exe') -Destination (Join-Path $luciSidecars 'luci-api-x86_64-pc-windows-msvc.exe') -Force
        $env:CARGO_TARGET_DIR = Join-Path $luciNativeRoot 'target'
        $luciTauriOverride = Join-Path $luciStage 'tauri.release.json'
        $luciOverride = @{build=@{beforeBuildCommand=''}}
        if ($luciSigningContext) {
            $luciOverride.bundle = @{windows=@{certificateThumbprint=$luciSigningContext.thumbprint;digestAlgorithm='sha256';timestampUrl=$TimestampUrl;tsp=$true;signCommand=(Get-LuciTauriSigningCommand $luciSigningContext (Join-Path $PSHOME 'pwsh.exe') (Join-Path $PSScriptRoot 'sign-windows-file.ps1') $luciSigningFiles[0].sha256)}}
        }
        $luciOverride | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath $luciTauriOverride -Encoding utf8NoBOM
        Push-Location (Join-Path $luciRoot 'desktop')
        try { & $luciNpm run tauri -- build --config $luciTauriOverride --bundles nsis -- --locked; Confirm-Exit 'Native NSIS build' } finally { Pop-Location }
        $luciInstaller = Join-Path $env:CARGO_TARGET_DIR ('release/bundle/nsis/Luci_' + $Version + '_x64-setup.exe')
        if (-not (Test-Path -LiteralPath $luciInstaller -PathType Leaf)) { throw ('Missing expected installer: ' + $luciInstaller) }
        if ($luciSigningContext) {
            # NSIS consumes the externalBin source; tauri-build also makes a
            # runtime copy. Neither may diverge from the archived signed core.
            foreach ($luciCorePath in @((Join-Path $luciWindowsPackage 'luci.exe'), (Join-Path $luciSidecars 'luci-api-x86_64-pc-windows-msvc.exe'), (Join-Path $env:CARGO_TARGET_DIR 'release/luci-api.exe'))) {
                Confirm-LuciWindowsCoreIdentity $luciCorePath $luciSigningFiles[0].sha256
            }
            $luciSigningFiles.Add((Confirm-LuciWindowsSignature (Join-Path $env:CARGO_TARGET_DIR 'release/luci-api.exe') $luciSigningContext 'native-runtime-api-copy'))
            $luciSigningFiles.Add((Confirm-LuciWindowsSignature (Join-Path $luciSidecars 'luci-api-x86_64-pc-windows-msvc.exe') $luciSigningContext 'native-api-sidecar'))
            $luciSigningFiles.Add((Confirm-LuciWindowsSignature (Join-Path $env:CARGO_TARGET_DIR 'release/luci.exe') $luciSigningContext 'native-application'))
            $luciSigningFiles.Add((Confirm-LuciWindowsSignature $luciInstaller $luciSigningContext 'native-installer'))
            $luciNativeSigning.files = @($luciSigningFiles.ToArray())
            $luciNativeSigning.verified = $true; $luciNativeSigning.status = 'verified-authenticode'
        }
        $luciArtifacts += $luciInstaller
    }
    $luciArtifactRecords = @()
    foreach ($luciArtifact in $luciArtifacts) {
        $luciDestination = Join-Path $luciOutput (Split-Path -Leaf $luciArtifact)
        Copy-Item -LiteralPath $luciArtifact -Destination $luciDestination -Force
        $luciArtifactRecords += @{name=(Split-Path -Leaf $luciArtifact); bytes=(Get-Item -LiteralPath $luciDestination).Length; sha256=(Get-FileHash -Algorithm SHA256 -LiteralPath $luciDestination).Hash.ToLowerInvariant()}
    }
    $luciRuntimeRecord = (& python $luciPayloadTool describe-package --package $luciLinux)
    Confirm-Exit 'Runtime distribution release provenance'
    $luciRuntimeRecord = $luciRuntimeRecord | ConvertFrom-Json
    foreach ($luciPackage in $luciPackages) { if ($luciPackage.selected) { Replace-GeneratedTree $luciPackage.path (Join-Path $luciOutput $luciPackage.name) } }
    $luciHashes = $luciArtifactRecords | Sort-Object name | ForEach-Object { $_.sha256 + '  ' + $_.name }
    [IO.File]::WriteAllLines((Join-Path $luciOutput 'SHA256SUMS'), [string[]]$luciHashes, [Text.UTF8Encoding]::new($false))
    if ($luciSource.sourceDirty -eq $false) {
        $luciEndingSource = Get-SourceMetadata $luciRoot
        if ($luciEndingSource.sourceCommit -ne $luciSource.sourceCommit -or $luciEndingSource.sourceDirty -ne $false) { throw 'Source changed during packaging; no clean release manifest was written.' }
    }
    $luciRelease = @{version=$Version;sourceCommit=$luciSource.sourceCommit;sourceDirty=$luciSource.sourceDirty;builtAt=[DateTime]::UtcNow.ToString('o');artifacts=$luciArtifactRecords;portableLicenseComponents=@($luciPackages | ForEach-Object {@{target=$_.name;count=$_.licenses;missing=0}});nativeLicenseComponents=$luciNativeCount;nativeRequested=[bool]$Native;nativeSigning=$luciNativeSigning;stage=$luciStage}
    $luciRelease.runtimeDistribution = $luciRuntimeRecord
    $luciRelease | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath (Join-Path $luciOutput 'release-manifest.json') -Encoding utf8NoBOM
    $luciBuildSucceeded = $true
} finally {
    foreach ($luciKey in $luciSavedEnvironment.Keys) { [Environment]::SetEnvironmentVariable($luciKey, $luciSavedEnvironment[$luciKey], 'Process') }
    # Published packages and their checksums are complete. Keep failed stages
    # for diagnosis, but do not retain another full copy after every success.
    if ($luciBuildSucceeded) { Remove-LuciCompletedBuildStage -WorkspaceRoot $luciRoot -Version $Version -StagePath $luciStage | Out-Null }
}
Write-Host ('Reviewable Luci packages: ' + $luciOutput)
