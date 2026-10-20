param([ValidateRange(1,65535)][int]$Port = 8730, [string]$StateDir = '', [switch]$NoBrowser)
$ErrorActionPreference = 'Stop'
$luciExe = Join-Path $PSScriptRoot 'luci.exe'
if (-not (Test-Path -LiteralPath $luciExe -PathType Leaf)) { throw 'Extract the complete Luci ZIP, then open Start-Luci.ps1 again.' }
$luciPackageFile = Join-Path $PSScriptRoot 'package-identity'
if (-not (Test-Path -LiteralPath $luciPackageFile -PathType Leaf)) { throw 'This package is incomplete. Extract the complete verified Luci archive into a new folder, then open its launcher.' }
$luciPackageInfo = Get-Item -LiteralPath $luciPackageFile -Force
if (($luciPackageInfo.Attributes -band [IO.FileAttributes]::ReparsePoint) -or $luciPackageInfo.Length -gt 66) { throw 'This package has invalid identity metadata. Extract the complete verified Luci archive into a new folder.' }
$luciPackageId = (Get-Content -LiteralPath $luciPackageFile -Raw).Trim()
if ($luciPackageId -cnotmatch '^[0-9a-f]{64}$') { throw 'This package has invalid identity metadata. Extract the complete verified Luci archive into a new folder.' }
$luciDocumentPins = @('','')
$luciDocumentPinFile = Join-Path $PSScriptRoot 'document-runtime-pins'
if (Test-Path -LiteralPath $luciDocumentPinFile) {
    $luciDocumentPinInfo = Get-Item -LiteralPath $luciDocumentPinFile -Force
    if ($luciDocumentPinInfo.PSIsContainer -or ($luciDocumentPinInfo.Attributes -band [IO.FileAttributes]::ReparsePoint) -or $luciDocumentPinInfo.Length -ne 130) { throw 'Invalid packaged document proof configuration.' }
    $luciDocumentPinBytes = [IO.File]::ReadAllBytes($luciDocumentPinFile)
    $luciDocumentPinText = [Text.Encoding]::ASCII.GetString($luciDocumentPinBytes)
    if ($luciDocumentPinText -cnotmatch '\A[0-9a-f]{64}\n[0-9a-f]{64}\n\z') { throw 'Invalid packaged document proof configuration.' }
    $luciDocumentPins = $luciDocumentPinText.Split("`n")
}
$luciDocumentImagePin = ''
$luciDocumentImagePinFile = Join-Path $PSScriptRoot 'document-image-manifest-pin'
if (Test-Path -LiteralPath $luciDocumentImagePinFile) {
    $luciDocumentImagePinInfo = Get-Item -LiteralPath $luciDocumentImagePinFile -Force
    if ($luciDocumentImagePinInfo.PSIsContainer -or ($luciDocumentImagePinInfo.Attributes -band [IO.FileAttributes]::ReparsePoint) -or $luciDocumentImagePinInfo.Length -ne 65) { throw 'Invalid packaged document image configuration.' }
    $luciDocumentImagePinText = [Text.Encoding]::ASCII.GetString([IO.File]::ReadAllBytes($luciDocumentImagePinFile))
    if ($luciDocumentImagePinText -cnotmatch '\A[0-9a-f]{64}\n\z') { throw 'Invalid packaged document image configuration.' }
    $luciDocumentImagePin = $luciDocumentImagePinText.Substring(0,64)
}
if (-not $StateDir) { $StateDir = if ($env:LUCI_STATE_DIR) { $env:LUCI_STATE_DIR } else { Join-Path $env:LOCALAPPDATA 'Luci/state' } }
$luciState = [IO.Path]::GetFullPath($StateDir)
New-Item -ItemType Directory -Force -Path $luciState | Out-Null
$luciBase = 'http://127.0.0.1:' + $Port
$luciOwned = $null
$luciPriorEnvironment = @{}
foreach ($luciKey in @('LUCI_STATE_DIR','LUCI_ADDR','LUCI_MANAGED_SHUTDOWN_TOKEN','LUCI_LOCAL_ACCESS_TOKEN','LUCI_PORTABLE_OWNER_ACCESS','LUCI_PORTABLE_PACKAGE_ID','LUCI_ALLOW_PUBLIC_BIND','LUCI_WORKER','LUCI_WEB_DIR','LUCI_RUNTIME_DIR','LUCI_CLOUD_BUNDLE','LUCI_DOCUMENT_PROFILE_SHA256','LUCI_DOCUMENT_TOOL_DOMAIN_SHA256','LUCI_DOCUMENT_IMAGE_MANIFEST_SHA256')) {
    $luciPriorEnvironment[$luciKey] = [Environment]::GetEnvironmentVariable($luciKey, 'Process')
}
$luciLifecycle = [Guid]::NewGuid().ToString('N') + [Guid]::NewGuid().ToString('N')
function Read-LuciCode([string]$Path) {
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) { return $null }
    $luciCodeItem = Get-Item -LiteralPath $Path -Force
    if (($luciCodeItem.Attributes -band [IO.FileAttributes]::ReparsePoint) -or $luciCodeItem.Length -gt 128) { throw 'Luci could not verify its private account-access file.' }
    $luciPrivateCode = (Get-Content -LiteralPath $Path -Raw).Trim()
    if ($luciPrivateCode -notmatch '^[A-Za-z0-9_-]{43}$') { throw 'Luci could not verify its private account-access file.' }
    return $luciPrivateCode
}
try {
    $env:LUCI_STATE_DIR = $luciState
    $env:LUCI_ADDR = '127.0.0.1:' + $Port
    $env:LUCI_MANAGED_SHUTDOWN_TOKEN = $luciLifecycle
    $env:LUCI_LOCAL_ACCESS_TOKEN = ''
    $env:LUCI_PORTABLE_OWNER_ACCESS = '1'
    $env:LUCI_PORTABLE_PACKAGE_ID = $luciPackageId
    $env:LUCI_ALLOW_PUBLIC_BIND = ''
    $env:LUCI_WORKER = ''
    $env:LUCI_WEB_DIR = Join-Path $PSScriptRoot 'web'
    $env:LUCI_RUNTIME_DIR = Join-Path $PSScriptRoot 'runtime'
    $env:LUCI_CLOUD_BUNDLE = Join-Path $PSScriptRoot 'cloud/luci-cloud-linux-amd64.tar.gz'
    # These are selected release inputs, never inherited user environment or a
    # self-hash of the runtime descriptor. An omitted developer profile clears both.
    $env:LUCI_DOCUMENT_PROFILE_SHA256 = $luciDocumentPins[0]
    $env:LUCI_DOCUMENT_TOOL_DOMAIN_SHA256 = $luciDocumentPins[1]
    $env:LUCI_DOCUMENT_IMAGE_MANIFEST_SHA256 = $luciDocumentImagePin
    try { $luciStatus = Invoke-RestMethod -Uri ($luciBase + '/v1/control-plane/status') -TimeoutSec 2 } catch { $luciStatus = $null }
    if ($null -eq $luciStatus) {
        $luciOwned = Start-Process -FilePath $luciExe -WorkingDirectory $PSScriptRoot -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $luciState 'service.out.log') -RedirectStandardError (Join-Path $luciState 'service.err.log')
    } elseif ($luciStatus.product -ne 'Luci') {
        throw 'Another application is using Luci''s address. Close it, or start Luci with a different -Port.'
    }
    $luciReady = $false
    for ($luciAttempt = 0; $luciAttempt -lt 30; $luciAttempt++) {
        if ($null -ne $luciOwned -and $luciOwned.HasExited) { throw ('Luci could not start. Read ' + (Join-Path $luciState 'service.err.log')) }
        try {
            $luciStatus = Invoke-RestMethod -Uri ($luciBase + '/v1/control-plane/status') -TimeoutSec 2
            if ($luciStatus.product -eq 'Luci') { $luciReady = $true; break }
        } catch { }
        Start-Sleep -Seconds 1
    }
    if (-not $luciReady) { throw ('Luci could not start. Read ' + (Join-Path $luciState 'service.err.log')) }
    $luciOwnerCode = Read-LuciCode (Join-Path $luciState 'control-plane/owner-access-token')
    if (-not $luciOwnerCode) { throw 'An older or different Luci workspace is already running at this address. Close its launcher, then start this version again.' }
    try {
        Invoke-RestMethod -Uri ($luciBase + '/v1/auth/local-owner') -Headers @{'X-Luci-Local-Access-Token'=$luciOwnerCode} -TimeoutSec 5 | Out-Null
    } catch { throw 'The running workspace does not belong to this launcher. Close its launcher or choose a different -Port; your saved data has not been reset.' }
    try {
        $luciRunningPackage = Invoke-RestMethod -Uri ($luciBase + '/v1/auth/local-package') -Headers @{'X-Luci-Local-Access-Token'=$luciOwnerCode} -TimeoutSec 5
    } catch { $luciRunningPackage = '' }
    if (([string]$luciRunningPackage).TrimEnd([char[]]"`r`n") -cne $luciPackageId) { throw 'A different or older Luci package is already running for this workspace. Close its launcher before opening this package. The existing service and saved data have not been changed.' }
    # Fragments are not sent in HTTP requests and Luci removes them immediately.
    # No password or private proof is printed by this launcher.
    $luciFragment = 'owner=' + $luciOwnerCode
    if ($luciStatus.bootstrapRequired) {
        $luciBootstrapCode = Read-LuciCode (Join-Path $luciState 'control-plane/bootstrap-token')
        if ($luciBootstrapCode) { $luciFragment += '&bootstrap=' + $luciBootstrapCode }
    }
    if (-not $NoBrowser) { Start-Process ($luciBase + '/#' + $luciFragment) }
    Write-Host ('Luci is running at ' + $luciBase)
    if ($null -ne $luciOwned) { Write-Host 'Keep this launcher open. Press Enter here to stop Luci.' }
    else { Write-Host 'This workspace was already running. Press Enter to close this launcher.' }
    Read-Host | Out-Null
} finally {
    if ($null -ne $luciOwned -and -not $luciOwned.HasExited) {
        try {
            Invoke-RestMethod -Method Post -Uri ($luciBase + '/v1/control-plane/shutdown') -Headers @{'X-Luci-Lifecycle-Token'=$luciLifecycle} -TimeoutSec 5 | Out-Null
            $luciOwned.WaitForExit(5000) | Out-Null
        } catch { }
        if (-not $luciOwned.HasExited) { Stop-Process -Id $luciOwned.Id }
    }
    foreach ($luciKey in $luciPriorEnvironment.Keys) { [Environment]::SetEnvironmentVariable($luciKey, $luciPriorEnvironment[$luciKey], 'Process') }
}
