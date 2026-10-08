#requires -Version 5.1
param([switch]$InstallRuntime, [ValidateRange(1,65535)][int]$Port = 8730)
$ErrorActionPreference = 'Stop'
$luciRoot = Split-Path -Parent $PSScriptRoot
$luciPriorLocation = Get-Location
$luciPriorEnvironment = @{}
foreach ($luciKey in @('LUCI_RUNTIME_DIR','LUCI_ADDR','VITE_LUCI_API','LUCI_MANAGED_SHUTDOWN_TOKEN','LUCI_WORKER')) {
    $luciPriorEnvironment[$luciKey] = [Environment]::GetEnvironmentVariable($luciKey, 'Process')
}
$luciService = $null
$luciBase = 'http://127.0.0.1:' + $Port
$luciLifecycle = [Guid]::NewGuid().ToString('N') + [Guid]::NewGuid().ToString('N')
try {
    Set-Location -LiteralPath $luciRoot
    Get-Command go -ErrorAction Stop | Out-Null
    # Use Node's command shim: npm.ps1 can be blocked by script policy and
    # reparses arguments differently from the command-line executable.
    Get-Command npm.cmd -ErrorAction Stop | Out-Null
    # Never open this checkout's frontend against an older, unrelated service.
    $luciProbe = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback, $Port)
    $luciProbe.ExclusiveAddressUse = $true
    try { $luciProbe.Start() }
    catch { throw ('Luci API port ' + $Port + ' is already in use. Stop its launcher, or choose another -Port. No existing process was stopped.') }
    finally { $luciProbe.Stop() }
    $luciDev = Join-Path $luciRoot 'artifacts/dev'
    New-Item -ItemType Directory -Force -Path $luciDev | Out-Null
    & go build -o (Join-Path $luciDev 'luci-api.exe') ./control-plane/cmd/api
    if ($LASTEXITCODE -ne 0) { throw 'Go build failed' }
    $env:LUCI_RUNTIME_DIR = $luciRoot
    $env:LUCI_ADDR = '127.0.0.1:' + $Port
    $env:VITE_LUCI_API = $luciBase
    $env:LUCI_MANAGED_SHUTDOWN_TOKEN = $luciLifecycle
    $env:LUCI_WORKER = ''
    if ($InstallRuntime) {
        & docker build --file images/sandbox/Dockerfile --tag luci/sandbox:dev .
        if ($LASTEXITCODE -ne 0) { throw 'Runtime install failed. Start Docker and retry.' }
    }
    $luciService = Start-Process -FilePath (Join-Path $luciDev 'luci-api.exe') -WorkingDirectory $luciRoot -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $luciDev 'api.out.log') -RedirectStandardError (Join-Path $luciDev 'api.err.log')
    $luciReady = $false
    for ($luciAttempt = 0; $luciAttempt -lt 30; $luciAttempt++) {
        $luciService.Refresh()
        if ($luciService.HasExited) { throw ('Luci could not start. Read ' + (Join-Path $luciDev 'api.err.log')) }
        try { $luciStatus = Invoke-RestMethod -Uri ($luciBase + '/v1/control-plane/status') -TimeoutSec 2 }
        catch { $luciStatus = $null }
        if ($null -ne $luciStatus) {
            if ($luciStatus.product -ne 'Luci') { throw 'Another application answered at the Luci API address. Stop it or choose another -Port.' }
            $luciService.Refresh()
            if ($luciService.HasExited) { throw ('Luci could not start. Read ' + (Join-Path $luciDev 'api.err.log')) }
            $luciReady = $true
            break
        }
        Start-Sleep -Seconds 1
    }
    if (-not $luciReady) { throw ('Luci did not become ready. Read ' + (Join-Path $luciDev 'api.err.log')) }
    Set-Location -LiteralPath (Join-Path $luciRoot 'desktop')
    if (-not (Test-Path -LiteralPath node_modules -PathType Container)) { & npm.cmd ci; if ($LASTEXITCODE -ne 0) { throw 'Dependency install failed' } }
    Write-Host 'Luci: http://localhost:1420 — follow setup in the app. Press Ctrl+C to stop.'
    & npm.cmd run dev
    if ($LASTEXITCODE -ne 0) { throw ('Desktop development server failed (exit ' + $LASTEXITCODE + '). Check whether port 1420 is already in use.') }
} finally {
    try {
        if ($null -ne $luciService -and -not $luciService.HasExited) {
            try {
                Invoke-RestMethod -Method Post -Uri ($luciBase + '/v1/control-plane/shutdown') -Headers @{'X-Luci-Lifecycle-Token'=$luciLifecycle} -TimeoutSec 5 | Out-Null
                $luciService.WaitForExit(5000) | Out-Null
            } catch { }
            if (-not $luciService.HasExited) { Stop-Process -Id $luciService.Id }
        }
    } finally {
        foreach ($luciKey in $luciPriorEnvironment.Keys) { [Environment]::SetEnvironmentVariable($luciKey, $luciPriorEnvironment[$luciKey], 'Process') }
        Set-Location -LiteralPath $luciPriorLocation
    }
}
