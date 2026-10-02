param(
  [Parameter(Mandatory = $true)][string]$Installer,
  [string]$PreviousInstaller = '',
  [string]$Evidence = 'artifacts/native-installed-eval'
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
if ($env:GITHUB_ACTIONS -ne 'true' -or $env:RUNNER_OS -ne 'Windows') {
  throw 'Installed-app acceptance is restricted to a disposable Windows GitHub runner.'
}
if ($env:LUCI_API) { throw 'LUCI_API would bypass the installed app-owned sidecar.' }
$candidate = (Resolve-Path -LiteralPath $Installer).Path
$prior = if ($PreviousInstaller) { (Resolve-Path -LiteralPath $PreviousInstaller).Path } else { '' }
$evidencePath = [IO.Path]::GetFullPath($Evidence)
$taskRoot = Join-Path $env:RUNNER_TEMP ('luci-installed-' + [guid]::NewGuid().ToString('N'))
$installDir = Join-Path $taskRoot 'app'
$toolsDir = Join-Path $taskRoot 'tools'
$stateDir = Join-Path $env:APPDATA 'ai.lcfr.luci'
if (Test-Path -LiteralPath $stateDir) { throw "Runner is not fresh: Luci state already exists at $stateDir" }
New-Item -ItemType Directory -Path $taskRoot,$toolsDir,$evidencePath -Force | Out-Null
$receipt = [ordered]@{
  passed = $false; installer = [IO.Path]::GetFileName($candidate)
  installerSHA256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $candidate).Hash.ToLowerInvariant()
  sourceCommit = $env:GITHUB_SHA; checks = @(); limitations = @(
    'Disposable Windows runner; existing WebView2 runtime. No model/cloud calls.',
    'Silent NSIS lifecycle is exercised; graphical installer wizard and code signing are not covered.'
  )
}
$driver = $null

function Run-Installer([string]$Path, [string[]]$Arguments) {
  $process = Start-Process -FilePath $Path -ArgumentList $Arguments -PassThru -WindowStyle Hidden
  if (-not $process.WaitForExit(120000)) {
    Stop-Process -Id $process.Id -ErrorAction SilentlyContinue
    throw "Installer exceeded its two-minute bound: $([IO.Path]::GetFileName($Path))"
  }
  if ($process.ExitCode -ne 0) { throw "Installer returned $($process.ExitCode)" }
}
function Install-Candidate([string]$Path) {
  # NSIS /D must be the final argument. RUNNER_TEMP/app has no spaces on the
  # hosted image; fail rather than silently testing a different install path.
  if ($installDir -match '\s') { throw 'The isolated NSIS test install path must not contain spaces.' }
  Run-Installer $Path @('/S', "/D=$installDir")
  foreach ($name in @('luci.exe','luci-api.exe','runtime\images\sandbox\Dockerfile','licenses\manifest.json','cloud\luci-cloud-linux-amd64.tar.gz')) {
    if (-not (Test-Path -LiteralPath (Join-Path $installDir $name))) { throw "Installed resource is missing: $name" }
  }
}
function Run-UI([string]$Mode, [string]$PassName) {
  & node (Join-Path $PSScriptRoot 'native-installed-eval.mjs') (Join-Path $installDir 'luci.exe') $evidencePath $Mode
  if ($LASTEXITCODE -ne 0) { throw "Installed WebView journey failed: $PassName" }
  Copy-Item -LiteralPath (Join-Path $evidencePath "$Mode-result.json") -Destination (Join-Path $evidencePath "$PassName-result.json")
  $receipt.checks += $PassName
}
function Preserved-Hashes {
  $fixture = Get-Content -LiteralPath (Join-Path $evidencePath 'private-fixture.json') -Raw | ConvertFrom-Json
  $project = Join-Path $fixture.projectRoot $fixture.projectName
  $hashes = [ordered]@{}
  foreach ($name in @('LUCI.md','.luci\state.json','.luci\design.json','.git\HEAD')) {
    $path = Join-Path $project $name
    $hashes[$path] = (Get-FileHash -Algorithm SHA256 -LiteralPath $path).Hash
  }
  $identity = Join-Path $stateDir 'control-plane\identity.json'
  if (Test-Path -LiteralPath $identity) { $hashes[$identity] = (Get-FileHash -Algorithm SHA256 -LiteralPath $identity).Hash }
  return $hashes
}
function Assert-Preserved($Before) {
  foreach ($path in $Before.Keys) {
    if (-not (Test-Path -LiteralPath $path)) { throw "Installer removed user data: $path" }
    if ((Get-FileHash -Algorithm SHA256 -LiteralPath $path).Hash -ne $Before[$path]) { throw "Installer changed user data: $path" }
  }
}

try {
  # Official external driver. No debug plugin or native app modification.
  & cargo install tauri-driver --version 2.1.0 --locked --root $toolsDir
  if ($LASTEXITCODE -ne 0) { throw 'Pinned tauri-driver installation failed' }
  $webviewRoots = @(
    (Join-Path ${env:ProgramFiles(x86)} 'Microsoft\EdgeWebView\Application'),
    (Join-Path $env:ProgramFiles 'Microsoft\EdgeWebView\Application')
  )
  $webview = @($webviewRoots | Where-Object { Test-Path -LiteralPath $_ } | ForEach-Object {
    Get-ChildItem -LiteralPath $_ -Directory | Where-Object { $_.Name -match '^\d+\.\d+\.\d+\.\d+$' }
  } | Sort-Object { [version]$_.Name } -Descending | Select-Object -First 1)
  if ($webview.Count -ne 1) { throw 'No installed WebView2 runtime was found.' }
  $runtimeVersion = $webview[0].Name
  $driverZip = Join-Path $toolsDir 'edgedriver.zip'
  Invoke-WebRequest -Uri "https://msedgedriver.microsoft.com/$runtimeVersion/edgedriver_win64.zip" -OutFile $driverZip
  Expand-Archive -LiteralPath $driverZip -DestinationPath $toolsDir
  $nativeDriver = Join-Path $toolsDir 'msedgedriver.exe'
  $driverVersion = (& $nativeDriver --version | Out-String).Trim()
  if ($driverVersion -notmatch [regex]::Escape($runtimeVersion)) { throw "Mismatched Edge driver: $driverVersion / $runtimeVersion" }
  $receipt.tauriDriver = '2.1.0'
  $receipt.webviewVersion = $runtimeVersion
  $receipt.edgeDriverVersion = $driverVersion
  $receipt.edgeDriverSHA256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $nativeDriver).Hash.ToLowerInvariant()
  $driver = Start-Process -FilePath (Join-Path $toolsDir 'bin\tauri-driver.exe') -ArgumentList @('--native-driver', $nativeDriver) -PassThru -WindowStyle Hidden -RedirectStandardOutput (Join-Path $evidencePath 'driver-out.log') -RedirectStandardError (Join-Path $evidencePath 'driver-error.log')
  if ($prior) {
    $receipt.previousInstaller = [IO.Path]::GetFileName($prior)
    $receipt.previousInstallerSHA256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $prior).Hash.ToLowerInvariant()
    Install-Candidate $prior
    Run-UI 'seed' 'previous-version-seed'
    $before = Preserved-Hashes
    Install-Candidate $candidate
    Assert-Preserved $before
    Run-UI 'verify' 'version-upgrade'
  } else {
    Install-Candidate $candidate
    Run-UI 'seed' 'fresh-install'
  }
  Run-UI 'verify' 'clean-restart'
  $before = Preserved-Hashes
  Install-Candidate $candidate
  Assert-Preserved $before
  Run-UI 'verify' 'same-version-reinstall'
  $before = Preserved-Hashes
  $uninstallers = @(Get-ChildItem -LiteralPath $installDir -File -Filter '*uninstall*.exe')
  if ($uninstallers.Count -ne 1) { throw 'Expected one installed NSIS uninstaller.' }
  Run-Installer $uninstallers[0].FullName @('/S')
  $deadline = (Get-Date).AddSeconds(15)
  while ((Test-Path -LiteralPath (Join-Path $installDir 'luci.exe')) -and (Get-Date) -lt $deadline) { Start-Sleep -Milliseconds 200 }
  foreach ($name in @('luci.exe','luci-api.exe','runtime','licenses','cloud')) {
    if (Test-Path -LiteralPath (Join-Path $installDir $name)) { throw "Uninstall retained app files: $name" }
  }
  Assert-Preserved $before
  $receipt.checks += 'Uninstall removed installed app/resources and preserved project, design, Git and identity data'
  $receipt.passed = $true
} catch {
  $receipt.error = $_.Exception.Message
  throw
} finally {
  if ($driver -and -not $driver.HasExited) {
    # tauri-driver owns the native driver process. Reap only that exact child.
    Get-CimInstance Win32_Process | Where-Object { $_.ParentProcessId -eq $driver.Id } | ForEach-Object { Stop-Process -Id $_.ProcessId -ErrorAction SilentlyContinue }
    Stop-Process -Id $driver.Id -ErrorAction SilentlyContinue
  }
  [IO.File]::WriteAllText((Join-Path $evidencePath 'installer-receipt.json'), ($receipt | ConvertTo-Json -Depth 10), [Text.UTF8Encoding]::new($false))
  # The ephemeral test password is never uploaded as evidence.
  $privateFixture = Join-Path $evidencePath 'private-fixture.json'
  if (Test-Path -LiteralPath $privateFixture) { Remove-Item -LiteralPath $privateFixture }
}
