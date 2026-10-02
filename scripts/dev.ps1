param([switch]$InstallRuntime)
$ErrorActionPreference = 'Stop'
$luciRoot = Split-Path -Parent $PSScriptRoot
Set-Location -LiteralPath $luciRoot
$luciDev = Join-Path $luciRoot 'artifacts/dev'
New-Item -ItemType Directory -Force -Path $luciDev | Out-Null
& go build -o (Join-Path $luciDev 'luci-api.exe') ./control-plane/cmd/api
if ($LASTEXITCODE -ne 0) { throw 'Go build failed' }
$env:LUCI_RUNTIME_DIR = $luciRoot
if ($InstallRuntime) {
    & docker build --file images/sandbox/Dockerfile --tag luci/sandbox:dev .
    if ($LASTEXITCODE -ne 0) { throw 'Runtime install failed. Start Docker and retry.' }
}
$luciService = Start-Process -FilePath (Join-Path $luciDev 'luci-api.exe') -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $luciDev 'api.out.log') -RedirectStandardError (Join-Path $luciDev 'api.err.log')
try {
    Set-Location -LiteralPath (Join-Path $luciRoot 'desktop')
    if (-not (Test-Path node_modules)) { & npm ci; if ($LASTEXITCODE -ne 0) { throw 'Dependency install failed' } }
    Write-Host 'Luci: http://localhost:1420 — follow setup in the app. Press Ctrl+C to stop.'
    & npm run dev
} finally {
    if (-not $luciService.HasExited) { Stop-Process -Id $luciService.Id }
    Set-Location -LiteralPath $luciRoot
}
