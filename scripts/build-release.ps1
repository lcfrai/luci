#requires -Version 7.0
param([string]$Version = '0.5.0', [switch]$Native, [string]$CertificateThumbprint = '', [string]$TimestampUrl = 'https://timestamp.digicert.com', [Parameter(Mandatory=$true)][string]$RuntimePayloadDir, [ValidateSet('all','windows-amd64','linux-amd64','darwin-arm64')][string]$Target = 'all')
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
if ($CertificateThumbprint -and -not $Native) { throw 'CertificateThumbprint requires -Native.' }
if ($Version -notmatch '^\d+\.\d+\.\d+(?:-[a-zA-Z0-9.-]+)?$') { throw 'Version must be a semantic version.' }
$luciRoot = [IO.Path]::GetFullPath((Split-Path -Parent $PSScriptRoot))
$luciOutput = Join-Path $luciRoot ('artifacts/releases/' + $Version)
$luciStage = Join-Path $luciOutput ('.stage-' + [guid]::NewGuid().ToString('N'))
$luciNativeRoot = Join-Path $luciRoot 'desktop/src-tauri'
$luciSavedEnvironment = @{}
$luciBuildSucceeded = $false
. (Join-Path $PSScriptRoot 'owned-build-stage.ps1')
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
    & python (Join-Path $PSScriptRoot 'runtime-oci/payload.py') verify --payload $RuntimePayloadDir
    Confirm-Exit 'Preverified source-bound runtime payload'
    # npm.ps1 reparses MyInvocation.Statement and can corrupt arguments inside a
    # script block. Use Node's command shim explicitly on Windows.
    $luciNpm = if ($IsWindows) { (Get-Command npm.cmd -ErrorAction Stop).Source } else { (Get-Command npm -ErrorAction Stop).Source }
    if ($Native) { Get-Command cargo -ErrorAction Stop | Out-Null }
    New-Item -ItemType Directory -Force -Path $luciStage | Out-Null
    $env:GOMAXPROCS = '1'; $env:GOMEMLIMIT = '512MiB'; $env:NODE_OPTIONS = '--max-old-space-size=1024'
    [Environment]::SetEnvironmentVariable('VITE_LUCI_API', $null, 'Process')
    Push-Location (Join-Path $luciRoot 'desktop')
    try { & $luciNpm run build; Confirm-Exit 'Desktop production build' } finally { Pop-Location }

    $luciGuides = Join-Path $luciStage 'package-guides'
    $luciDirtyArgument = if ($null -eq $luciSource.sourceDirty) { 'unknown' } else { ([string]$luciSource.sourceDirty).ToLowerInvariant() }
    & python (Join-Path $PSScriptRoot 'stage-release-docs.py') $luciRoot $luciGuides "--source-commit=$($luciSource.sourceCommit)" "--source-dirty=$luciDirtyArgument"
    Confirm-Exit 'Source-bound package documentation'

    # Runtime allowlist: only the sandbox build context, never the whole repository or user state.
    $luciRuntime = Join-Path $luciStage 'runtime'
    foreach ($luciTree in @('sandboxd/cmd','sandboxd/internal','sandboxd/vendor')) { Copy-Tree (Join-Path $luciRoot $luciTree) (Join-Path $luciRuntime $luciTree) }
    foreach ($luciFile in @('sandboxd/go.mod','sandboxd/go.sum','images/sandbox/Dockerfile','images/sandbox/entrypoint.sh')) {
        $luciDestination = Join-Path $luciRuntime $luciFile
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $luciDestination) | Out-Null
        Copy-Item -LiteralPath (Join-Path $luciRoot $luciFile) -Destination $luciDestination
    }
    if ($luciSource.sourceDirty -eq $false) {
        $luciAzureArchive = Join-Path $luciRuntime 'azure-setup.tar'
        $luciAzureHash = (& python (Join-Path $PSScriptRoot 'azure-setup/package.py') build --source $luciRoot --revision $luciSource.sourceCommit --output $luciAzureArchive | Select-Object -Last 1)
        Confirm-Exit 'Source-bound Azure Develop setup bundle'
        if ($luciAzureHash -notmatch '^[0-9a-f]{64}$') { throw 'Azure Develop setup bundle hash is invalid.' }
        & python (Join-Path $PSScriptRoot 'azure-setup/package.py') verify --archive $luciAzureArchive --sha256 $luciAzureHash | Out-Null
        Confirm-Exit 'Azure Develop setup bundle verification'
        [IO.File]::WriteAllText((Join-Path $luciRuntime 'azure-setup.sha256'), ($luciAzureHash + "`n"), [Text.UTF8Encoding]::new($false))
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
        Copy-Tree (Join-Path $luciRoot 'desktop/dist') (Join-Path $luciPackage 'web')
        Copy-Tree $luciRuntime (Join-Path $luciPackage 'runtime')
        Copy-Tree $luciGuides $luciPackage
        $luciLicenses = Join-Path $luciPackage 'licenses'
        & python (Join-Path $PSScriptRoot 'collect-licenses.py') $luciRoot $luciLicenses
        Confirm-Exit ($luciName + ' strict licence collection')
        $luciCount = Confirm-Licenses $luciLicenses
        & python (Join-Path $PSScriptRoot 'runtime-oci/payload.py') stage --payload $RuntimePayloadDir --repo $luciRoot --revision $luciSource.sourceCommit --package $luciPackage
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
    & python (Join-Path $PSScriptRoot 'runtime-oci/payload.py') core-from-package --package $luciLinux --output $luciCloudRoot
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

    $luciNativeSigning = @{requested=$false;verified=$false;status='unsigned'}
    $luciNativeCount = 0
    if ($Native) {
        $luciWindowsPackage = ($luciPackages | Where-Object { $_.os -eq 'windows' }).path
        $env:GOOS = 'windows'; $env:GOARCH = 'amd64'
        $luciNativeLicenses = Join-Path $luciStage 'native-licenses'
        & python (Join-Path $PSScriptRoot 'collect-licenses.py') $luciRoot $luciNativeLicenses --native
        Confirm-Exit 'Native strict licence collection'
        $luciNativeCount = Confirm-Licenses $luciNativeLicenses
        Copy-Tree (Join-Path $luciWindowsPackage 'licenses/runtime-image') (Join-Path $luciNativeLicenses 'runtime-image')
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
        if ($CertificateThumbprint) {
            if (-not $IsWindows -or $CertificateThumbprint -notmatch '^[0-9a-fA-F]{40}$') { throw 'Native signing requires a Windows certificate-store SHA1 thumbprint.' }
            $luciTimestamp = [Uri]$TimestampUrl
            if ($luciTimestamp.Scheme -ne 'https' -or -not $luciTimestamp.Host -or $luciTimestamp.UserInfo -or $luciTimestamp.Fragment) { throw 'Use an HTTPS RFC3161 timestamp endpoint without credentials.' }
            $luciCertificate = Get-Item -LiteralPath ('Cert:\CurrentUser\My\' + $CertificateThumbprint)
            $luciCodeSigningUsage = @($luciCertificate.Extensions | Where-Object { $_.Oid.Value -eq '2.5.29.37' } | ForEach-Object { $_.EnhancedKeyUsages } | ForEach-Object { $_.Value })
            if (-not $luciCertificate.HasPrivateKey -or $luciCertificate.NotAfter -le [DateTime]::Now -or $luciCertificate.NotBefore -gt [DateTime]::Now -or $luciCodeSigningUsage -notcontains '1.3.6.1.5.5.7.3.3') { throw 'The provisioned certificate must be current, contain a private key, and permit code signing.' }
            $luciOverride.bundle = @{windows=@{certificateThumbprint=$CertificateThumbprint;digestAlgorithm='sha256';timestampUrl=$TimestampUrl;tsp=$true}}
            $luciNativeSigning = @{requested=$true;verified=$false;status='pending';certificateThumbprint=$CertificateThumbprint.ToUpperInvariant();timestampUrl=$TimestampUrl}
        }
        $luciOverride | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $luciTauriOverride -Encoding utf8NoBOM
        Push-Location (Join-Path $luciRoot 'desktop')
        try { & $luciNpm run tauri -- build --config $luciTauriOverride --bundles nsis -- --locked; Confirm-Exit 'Native NSIS build' } finally { Pop-Location }
        $luciInstaller = Join-Path $env:CARGO_TARGET_DIR ('release/bundle/nsis/Luci_' + $Version + '_x64-setup.exe')
        if (-not (Test-Path -LiteralPath $luciInstaller -PathType Leaf)) { throw ('Missing expected installer: ' + $luciInstaller) }
        if ($CertificateThumbprint) {
            foreach ($luciSignedPath in @((Join-Path $env:CARGO_TARGET_DIR 'release/luci.exe'), $luciInstaller)) {
                $luciSignature = Get-AuthenticodeSignature -LiteralPath $luciSignedPath
                if ($luciSignature.Status -ne 'Valid' -or -not $luciSignature.SignerCertificate -or $luciSignature.SignerCertificate.Thumbprint -ne $CertificateThumbprint -or -not $luciSignature.TimeStamperCertificate) { throw ('Publisher signature/timestamp verification failed: ' + $luciSignedPath) }
            }
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
    $luciRuntimeRecord = (& python (Join-Path $PSScriptRoot 'runtime-oci/payload.py') describe-package --package $luciLinux)
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
