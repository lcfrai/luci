$ErrorActionPreference = 'Stop'
$luciExe = Join-Path $PSScriptRoot 'luci.exe'
$luciState = Join-Path $env:LOCALAPPDATA 'Luci/state'
New-Item -ItemType Directory -Force -Path $luciState | Out-Null
$luciOwned = $null
$luciPriorState = $env:LUCI_STATE_DIR
$luciPriorLifecycle = $env:LUCI_MANAGED_SHUTDOWN_TOKEN
$luciLifecycle = [Guid]::NewGuid().ToString('N') + [Guid]::NewGuid().ToString('N')
try {
    $env:LUCI_STATE_DIR = $luciState
    $env:LUCI_MANAGED_SHUTDOWN_TOKEN = $luciLifecycle
    try {$luciHealth = Invoke-RestMethod -Uri 'http://127.0.0.1:8730/v1/control-plane/status' -TimeoutSec 2; if ($luciHealth.product -ne 'Luci') {throw 'Different service'}} catch {$luciHealth = $null}
    if ($null -eq $luciHealth) {$luciOwned = Start-Process -FilePath $luciExe -WorkingDirectory $PSScriptRoot -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $luciState 'service.out.log') -RedirectStandardError (Join-Path $luciState 'service.err.log')}
    $luciReady = $false
    for ($luciAttempt = 0; $luciAttempt -lt 30; $luciAttempt++) {try { $luciStatus = Invoke-RestMethod -Uri 'http://127.0.0.1:8730/v1/control-plane/status' -TimeoutSec 2; if ($luciStatus.product -eq 'Luci') {$luciReady = $true;break} } catch {}; Start-Sleep -Seconds 1}
    if (-not $luciReady) {throw ('Luci could not start. Read ' + (Join-Path $luciState 'service.err.log'))}
    $luciUrl = 'http://127.0.0.1:8730'
    $luciCodeFile = Join-Path $luciState 'control-plane/bootstrap-token'
    if ($null -ne $luciOwned -and $luciStatus.bootstrapRequired -and (Test-Path -LiteralPath $luciCodeFile)) {
        $luciCode = (Get-Content -LiteralPath $luciCodeFile -Raw).Trim()
        if ($luciCode -match '^[A-Za-z0-9_-]{43}$') {$luciUrl += '/#bootstrap=' + $luciCode}
    }
    Start-Process $luciUrl
    Write-Host 'Luci is running. Press Enter here to stop its local service.'
    Read-Host | Out-Null
} finally {
    if ($null -ne $luciOwned -and -not $luciOwned.HasExited) {
        try {Invoke-RestMethod -Method Post -Uri 'http://127.0.0.1:8730/v1/control-plane/shutdown' -Headers @{'X-Luci-Lifecycle-Token'=$luciLifecycle} -TimeoutSec 5 | Out-Null; $luciOwned.WaitForExit(5000) | Out-Null} catch {}
        if (-not $luciOwned.HasExited) {Stop-Process -Id $luciOwned.Id}
    }
    $env:LUCI_STATE_DIR = $luciPriorState
    $env:LUCI_MANAGED_SHUTDOWN_TOKEN = $luciPriorLifecycle
}
