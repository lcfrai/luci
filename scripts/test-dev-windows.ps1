#requires -Version 5.1
param([string]$OutputDirectory = (Join-Path $PSScriptRoot '../artifacts/windows-source-launcher'))
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$luciOutput = [IO.Path]::GetFullPath($OutputDirectory)
New-Item -ItemType Directory -Force -Path $luciOutput | Out-Null
$luciFixture = Join-Path $luciOutput ('fixture-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Force -Path (Join-Path $luciFixture 'scripts'),(Join-Path $luciFixture 'desktop'),(Join-Path $luciFixture 'caller') | Out-Null
Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'dev.ps1') -Destination (Join-Path $luciFixture 'scripts/dev.ps1')
$luciBeforeLocation = Get-Location
$luciSavedEnvironment = @{}
$luciKeys = @('LUCI_RUNTIME_DIR','LUCI_ADDR','VITE_LUCI_API','LUCI_MANAGED_SHUTDOWN_TOKEN','LUCI_WORKER','LUCI_STATE_DIR','LUCI_ALLOWED_ORIGINS')
foreach ($luciKey in $luciKeys) { $luciSavedEnvironment[$luciKey] = [Environment]::GetEnvironmentVariable($luciKey, 'Process') }
$luciResults = [Collections.Generic.List[object]]::new()
if (Get-Variable -Name luciDevTest -Scope Global -ErrorAction SilentlyContinue) { throw 'An earlier launcher test is still active.' }
$global:luciDevTest = @{}

# These tests execute the actual source launcher with controlled external
# commands. No build, Docker operation, model request or user service occurs.
# The port-collision guard uses an actual exclusive loopback listener.
function go { $global:luciDevTest.goCalls++; $global:LASTEXITCODE = $(if ($global:luciDevTest.scenario -eq 'build failure') { 1 } else { 0 }) }
function docker { $global:luciDevTest.dockerCalls++; $global:LASTEXITCODE = 1 }
function npm.cmd {
    $global:luciDevTest.npmCalls.Add(($args -join ' '))
    if ($args[0] -eq 'ci' -and $global:luciDevTest.scenario -eq 'dependency failure') { $global:LASTEXITCODE = 7 }
    elseif ($args[0] -eq 'run' -and $global:luciDevTest.scenario -eq 'frontend failure') { $global:LASTEXITCODE = 8 }
    else { $global:LASTEXITCODE = 0 }
}
function Start-Process {
    param($FilePath,$WorkingDirectory,$WindowStyle,[switch]$PassThru,$RedirectStandardOutput,$RedirectStandardError)
    $global:luciDevTest.started++
    if ($WorkingDirectory -ne $luciFixture) { throw 'Incorrect service working directory.' }
    if ($env:LUCI_ADDR -ne ('127.0.0.1:' + $global:luciDevTest.port) -or $env:VITE_LUCI_API -ne ('http://127.0.0.1:' + $global:luciDevTest.port)) { throw 'API and frontend addresses differ.' }
    if ($env:LUCI_WORKER -or $env:LUCI_RUNTIME_DIR -ne $luciFixture -or $env:LUCI_STATE_DIR -ne 'preserved-private-state') { throw 'Source launch changed the wrong runtime state.' }
    if ($env:LUCI_MANAGED_SHUTDOWN_TOKEN -notmatch '^[0-9a-f]{64}$') { throw 'Missing owned shutdown capability.' }
    if ($global:luciDevTest.scenario -eq 'isolated frontend') {
        if ($env:LUCI_ALLOWED_ORIGINS -ne 'http://localhost:14200,http://127.0.0.1:14200') { throw 'Custom frontend origin was not scoped exactly.' }
        if ($FilePath -ne (Join-Path $luciFixture 'isolated/luci-api.exe') -or $RedirectStandardOutput -ne (Join-Path $luciFixture 'isolated/api.out.log')) { throw 'Acceptance reused another launch binary or logs.' }
    }
    $global:luciDevTest.ownedToken = $env:LUCI_MANAGED_SHUTDOWN_TOKEN
    $global:luciDevTest.service = [pscustomobject]@{Id=32003;HasExited=($global:luciDevTest.scenario -eq 'service exit')}
    $global:luciDevTest.service | Add-Member -MemberType ScriptMethod -Name Refresh -Value { }
    $global:luciDevTest.service | Add-Member -MemberType ScriptMethod -Name WaitForExit -Value { param($Timeout) if ($global:luciDevTest.scenario -ne 'forced owned cleanup') { $this.HasExited = $true }; return $this.HasExited }
    return $global:luciDevTest.service
}
function Stop-Process {
    param($Id)
    if ($Id -ne 32003) { throw 'Cleanup targeted an unowned process.' }
    $global:luciDevTest.stopped++
    $global:luciDevTest.service.HasExited = $true
}
function Invoke-RestMethod {
    param($Uri,$TimeoutSec,$Method,$Headers)
    if ($Method -eq 'Post') {
        if ($Uri -ne ('http://127.0.0.1:' + $global:luciDevTest.port + '/v1/control-plane/shutdown') -or $Headers['X-Luci-Lifecycle-Token'] -ne $global:luciDevTest.ownedToken) { throw 'Shutdown did not carry the owned capability.' }
        $global:luciDevTest.shutdown++
        return
    }
    $global:luciDevTest.probes++
    if ($global:luciDevTest.scenario -eq 'readiness timeout' -or ($global:luciDevTest.scenario -eq 'delayed readiness' -and $global:luciDevTest.probes -lt 3)) { throw 'Not ready yet.' }
    if ($global:luciDevTest.scenario -eq 'different service') { return @{product='Other'} }
    return @{product='Luci'}
}
function Start-Sleep { param($Seconds) $global:luciDevTest.waits++ }

try {
    foreach ($luciCase in @('delayed readiness','service exit','readiness timeout','different service','dependency failure','frontend failure','build failure','runtime failure','occupied port','forced owned cleanup','isolated frontend')) {
        $global:luciDevTest.scenario = $luciCase
        $global:luciDevTest.goCalls=0; $global:luciDevTest.dockerCalls=0; $global:luciDevTest.started=0; $global:luciDevTest.stopped=0; $global:luciDevTest.shutdown=0; $global:luciDevTest.probes=0; $global:luciDevTest.waits=0
        $global:luciDevTest.npmCalls = [Collections.Generic.List[string]]::new()
        $global:luciDevTest.ownedToken = ''; $global:luciDevTest.service = $null
        foreach ($luciKey in $luciKeys) { [Environment]::SetEnvironmentVariable($luciKey, ('before-' + $luciKey), 'Process') }
        $env:LUCI_STATE_DIR = 'preserved-private-state'
        $luciDependencies = Join-Path $luciFixture 'desktop/node_modules'
        if (Test-Path -LiteralPath $luciDependencies) { Remove-Item -LiteralPath $luciDependencies -Recurse -Force }
        if ($global:luciDevTest.scenario -ne 'dependency failure') { New-Item -ItemType Directory -Path $luciDependencies | Out-Null }
        $luciListener = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback, 0)
        $luciListener.ExclusiveAddressUse = $true
        $luciListener.Start()
        $global:luciDevTest.port = $luciListener.LocalEndpoint.Port
        if ($global:luciDevTest.scenario -ne 'occupied port') { $luciListener.Stop() }
        $luciCaller = Join-Path $luciFixture 'caller'
        Set-Location -LiteralPath $luciCaller
        $luciFailure = ''
        try {
            $luciExtra = @{}
            if ($global:luciDevTest.scenario -eq 'isolated frontend') { $luciExtra.FrontendPort = 14200; $luciExtra.DevDirectory = Join-Path $luciFixture 'isolated' }
            & (Join-Path $luciFixture 'scripts/dev.ps1') -Port $global:luciDevTest.port -InstallRuntime:($global:luciDevTest.scenario -eq 'runtime failure') @luciExtra
        }
        catch { $luciFailure = $_.Exception.Message }
        finally { $luciListener.Stop() }
        if ((Get-Location).Path -ne $luciCaller) { throw ($global:luciDevTest.scenario + ': caller location was not restored.') }
        foreach ($luciKey in $luciKeys) {
            $luciExpected = if ($luciKey -eq 'LUCI_STATE_DIR') { 'preserved-private-state' } else { 'before-' + $luciKey }
            if ([Environment]::GetEnvironmentVariable($luciKey, 'Process') -ne $luciExpected) { throw ($global:luciDevTest.scenario + ': environment was not restored.') }
        }
        $luciExpectedFailure = switch ($global:luciDevTest.scenario) {
            'service exit' { 'Luci could not start' }
            'readiness timeout' { 'Luci did not become ready' }
            'different service' { 'Another application answered' }
            'dependency failure' { 'Dependency install failed' }
            'frontend failure' { 'Desktop development server failed' }
            'build failure' { 'Go build failed' }
            'runtime failure' { 'Runtime install failed' }
            'occupied port' { 'is already in use' }
            default { '' }
        }
        if ($luciExpectedFailure -and $luciFailure -notmatch [regex]::Escape($luciExpectedFailure)) { throw ($global:luciDevTest.scenario + ': unexpected failure: ' + $luciFailure) }
        if (-not $luciExpectedFailure -and $luciFailure) { throw ($global:luciDevTest.scenario + ': unexpected failure: ' + $luciFailure) }
        if ($global:luciDevTest.scenario -eq 'delayed readiness' -and ($global:luciDevTest.probes -ne 3 -or $global:luciDevTest.waits -ne 2)) { throw 'Frontend started before readiness.' }
        if ($global:luciDevTest.scenario -eq 'readiness timeout' -and $global:luciDevTest.probes -ne 30) { throw 'Readiness was not bounded.' }
        if ($global:luciDevTest.scenario -in @('service exit','readiness timeout','different service','build failure','runtime failure','occupied port') -and $global:luciDevTest.npmCalls.Count -ne 0) { throw 'Frontend started after admission failure.' }
        if ($global:luciDevTest.scenario -eq 'occupied port' -and ($global:luciDevTest.started -ne 0 -or $global:luciDevTest.goCalls -ne 0 -or $global:luciDevTest.stopped -ne 0)) { throw 'Occupied port changed an existing service.' }
        if ($global:luciDevTest.scenario -eq 'dependency failure' -and ($global:luciDevTest.npmCalls.Count -ne 1 -or $global:luciDevTest.npmCalls[0] -ne 'ci')) { throw 'Frontend started after dependency failure.' }
        if ($global:luciDevTest.started -eq 1 -and $global:luciDevTest.scenario -ne 'service exit' -and $global:luciDevTest.shutdown -ne 1) { throw 'Owned service was not asked to shut down.' }
        if ($global:luciDevTest.scenario -eq 'forced owned cleanup' -and $global:luciDevTest.stopped -ne 1) { throw 'Unresponsive owned service was retained.' }
        if ($global:luciDevTest.scenario -eq 'isolated frontend' -and $global:luciDevTest.npmCalls[0] -ne 'run dev -- --port 14200') { throw ('Selected frontend port was not passed to Vite: ' + $global:luciDevTest.npmCalls[0]) }
        $luciResults.Add(@{name=$global:luciDevTest.scenario;passed=$true})
    }
    @{passed=$true;platform=[Environment]::OSVersion.Platform.ToString();powershell=$PSVersionTable.PSVersion.ToString();launcherSHA256=(Get-FileHash -Algorithm SHA256 -LiteralPath (Join-Path $luciFixture 'scripts/dev.ps1')).Hash.ToLowerInvariant();scope='Real launcher with controlled external commands and real socket admission; not native Windows/service/model acceptance.';results=$luciResults.ToArray()} | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath (Join-Path $luciOutput 'results.json') -Encoding UTF8
    Write-Host ('Windows source launcher checks passed: ' + $luciResults.Count)
} finally {
    foreach ($luciKey in $luciKeys) { [Environment]::SetEnvironmentVariable($luciKey, $luciSavedEnvironment[$luciKey], 'Process') }
    Set-Location -LiteralPath $luciBeforeLocation
    Remove-Item -LiteralPath $luciFixture -Recurse -Force
    Remove-Variable -Name luciDevTest -Scope Global
}
