#requires -Version 5.1
param(
    [ValidateRange(1024, 65535)][int]$Port = 8730,
    [ValidateRange(1024, 65535)][int]$PreviewPort = 8731,
    [ValidatePattern('^[a-z0-9][a-z0-9_-]*$')][string]$ProjectName = 'luci-docker',
    [switch]$NoBrowser
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
if ($Port -eq $PreviewPort) { throw 'The app and preview ports must differ.' }
if (-not (Get-Command docker -ErrorAction SilentlyContinue)) { throw 'Install and open Docker Desktop from https://docs.docker.com/desktop/, then run this launcher again.' }
& docker compose version | Out-Null
if ($LASTEXITCODE -ne 0) { throw 'Docker Compose is required. Update Docker Desktop, then retry.' }
$luciEngineOS = & docker info --format '{{.OSType}}' 2>$null
if ($LASTEXITCODE -ne 0 -or $luciEngineOS -ne 'linux') { throw 'Open Docker Desktop, select Linux containers and wait for its engine to start, then retry.' }
$luciSavedEnvironment = @{}
foreach ($luciKey in @('LUCI_PORT', 'LUCI_PREVIEW_PORT', 'LUCI_COMPOSE_PROJECT')) {
    $luciSavedEnvironment[$luciKey] = [Environment]::GetEnvironmentVariable($luciKey, 'Process')
}
try {
    $env:LUCI_PORT = [string]$Port
    $env:LUCI_PREVIEW_PORT = [string]$PreviewPort
    $env:LUCI_COMPOSE_PROJECT = $ProjectName
    $luciCompose = @('compose', '-f', (Join-Path $PSScriptRoot 'compose.yaml'))
    Write-Host 'Preparing Luci. The first launch downloads its pinned tools and private engine; subsequent launches reuse them.'
    & docker @luciCompose up --build --detach --wait --wait-timeout 240
    if ($LASTEXITCODE -ne 0) { throw 'Luci did not become ready. Check the engine and controller service logs, then retry.' }
    $luciBootstrap = (& docker @luciCompose exec -T controller sh -c 'if [ -r /luci-workspace/state/control-plane/bootstrap-token ]; then cat /luci-workspace/state/control-plane/bootstrap-token; fi') -join "`n"
    if ($LASTEXITCODE -ne 0) { throw 'Luci could not open its first-account flow. Check the controller service and retry.' }
    $luciBootstrap = $luciBootstrap.Trim()
    $luciUrl = 'http://127.0.0.1:' + $Port + '/'
    if ($luciBootstrap) {
        if ($luciBootstrap -notmatch '^[A-Za-z0-9_-]{43}$') { throw 'The first-account token has an unexpected format. Check the controller logs.' }
        $luciUrl += '#bootstrap=' + [Uri]::EscapeDataString($luciBootstrap)
    }
    if (-not $NoBrowser) { Start-Process -FilePath $luciUrl }
    $luciBootstrap = $null
    $luciUrl = $null
    Write-Host ('Luci is running at http://127.0.0.1:' + $Port + '/')
    Write-Host 'Your account, projects and private engine data are retained when the app restarts.'
} finally {
    foreach ($luciKey in $luciSavedEnvironment.Keys) {
        [Environment]::SetEnvironmentVariable($luciKey, $luciSavedEnvironment[$luciKey], 'Process')
    }
}
