param([ValidateRange(1,65535)][int]$Port = 8730, [string]$StateDir = '', [switch]$NoBrowser)
$ErrorActionPreference = 'Stop'
$luciExe = Join-Path $PSScriptRoot 'luci.exe'
if (-not (Test-Path -LiteralPath $luciExe -PathType Leaf)) { throw 'Extract the complete Luci ZIP, then open Start-Luci.ps1 again.' }
if (-not $StateDir) { $StateDir = if ($env:LUCI_STATE_DIR) { $env:LUCI_STATE_DIR } else { Join-Path $env:LOCALAPPDATA 'Luci/state' } }
$luciState = [IO.Path]::GetFullPath($StateDir)
New-Item -ItemType Directory -Force -Path $luciState | Out-Null
$luciBase = 'http://127.0.0.1:' + $Port
$luciOwned = $null
$luciPriorEnvironment = @{}
foreach ($luciKey in @('LUCI_STATE_DIR','LUCI_ADDR','LUCI_MANAGED_SHUTDOWN_TOKEN','LUCI_LOCAL_ACCESS_TOKEN','LUCI_PORTABLE_OWNER_ACCESS','LUCI_ALLOW_PUBLIC_BIND','LUCI_WORKER','LUCI_WEB_DIR','LUCI_RUNTIME_DIR','LUCI_CLOUD_BUNDLE')) {
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
    $env:LUCI_ALLOW_PUBLIC_BIND = ''
    $env:LUCI_WORKER = ''
    $env:LUCI_WEB_DIR = Join-Path $PSScriptRoot 'web'
    $env:LUCI_RUNTIME_DIR = Join-Path $PSScriptRoot 'runtime'
    $env:LUCI_CLOUD_BUNDLE = Join-Path $PSScriptRoot 'cloud/luci-cloud-linux-amd64.tar.gz'
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
