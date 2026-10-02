$ErrorActionPreference = 'Stop'
$luciExe = Join-Path $PSScriptRoot 'luci.exe'
$luciState = Join-Path $env:LOCALAPPDATA 'Luci/state'
New-Item -ItemType Directory -Force -Path $luciState | Out-Null
$luciOwned = $null
try {
    try {$luciHealth = Invoke-RestMethod -Uri 'http://127.0.0.1:8730/v1/healthz' -TimeoutSec 2} catch {$luciHealth = $null}
    if ($null -eq $luciHealth) {$luciOwned = Start-Process -FilePath $luciExe -WorkingDirectory $PSScriptRoot -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $luciState 'service.out.log') -RedirectStandardError (Join-Path $luciState 'service.err.log')}
    $luciReady = $false
    for ($luciAttempt = 0; $luciAttempt -lt 30; $luciAttempt++) {try { $luciStatus = Invoke-RestMethod -Uri 'http://127.0.0.1:8730/v1/setup/status' -TimeoutSec 2; if ($luciStatus.product -eq 'Luci') {$luciReady = $true;break} } catch {}; Start-Sleep -Seconds 1}
    if (-not $luciReady) {throw ('Luci could not start. Read ' + (Join-Path $luciState 'service.err.log'))}
    Start-Process 'http://127.0.0.1:8730'
    Write-Host 'Luci is running. Press Enter here to stop its local service.'
    Read-Host | Out-Null
} finally {if ($null -ne $luciOwned -and -not $luciOwned.HasExited) {Stop-Process -Id $luciOwned.Id}}
