param(
  [Parameter(Mandatory = $true)][string]$Installer,
  [string]$PreviousInstaller = '',
  [string]$PreviousVersion = '',
  [string]$Evidence = 'artifacts/native-installed-eval'
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$script:stage = 'admission'
$script:runtimeVersion = 'unknown'
$script:completedChecks = 0
function Write-InstalledFailure($Failure) {
  # Only classifications of our own literals are public. Never forward the
  # exception message, request URI/body, process output or fixture to Node.
  $reason = switch -Regex ($Failure.Exception.Message) {
    '^Pinned tauri-driver installation failed$' { 'tauri-driver-install-failed'; break }
    '^No installed WebView2 runtime was found\.$' { 'webview-runtime-missing'; break }
    '^Mismatched Edge driver:' { 'edge-driver-mismatch'; break }
    '^Pinned gsudo archive checksum differs$' { 'gsudo-checksum-mismatch'; break }
    '^Could not grant the runner user access' { 'directory-access-failed'; break }
    '^The WebDriver process must use a verified Medium-integrity token' { 'medium-integrity-failed'; break }
    '^Installer exceeded its two-minute bound:' { 'installer-timeout'; break }
    '^Installer returned ' { 'installer-exit-failed'; break }
    '^Installed resource is missing:' { 'installed-resource-missing'; break }
    '^Installed WebView journey failed:' { 'native-journey-failed'; break }
    '^Installer removed user data:' { 'preserved-data-missing'; break }
    '^Installer changed user data:' { 'preserved-data-changed'; break }
    '^Expected one private runtime configuration' { 'private-configuration-count'; break }
    '^Expected one installed NSIS uninstaller' { 'uninstaller-count'; break }
    '^Uninstall retained app resources after 30 seconds:' { 'uninstall-incomplete'; break }
    '^The isolated NSIS test install path must not contain spaces' { 'invalid-install-path'; break }
    '^(Installed-app acceptance is restricted|LUCI_API would bypass|PreviousVersion requires|Runner is not fresh)' { 'admission-refused'; break }
    default { 'unexpected-failure' }
  }
  $status = 0
  if ($Failure.Exception.PSObject.Properties['Response'] -and $Failure.Exception.Response -and $Failure.Exception.Response.PSObject.Properties['StatusCode']) {
    $status = [int]$Failure.Exception.Response.StatusCode
  }
  try {
    $exceptionType = $Failure.Exception.GetType().Name
    & node (Join-Path $PSScriptRoot 'native-eval-report.mjs') --installer $script:stage $reason $script:completedChecks $Failure.InvocationInfo.ScriptLineNumber $script:runtimeVersion $status $exceptionType
    if ($LASTEXITCODE -ne 0) { Write-Output '::error::Installed Windows acceptance failed; safe diagnostic reporter could not run. The original failure remains fatal.' }
  } catch {
    Write-Output '::error::Installed Windows acceptance failed; safe diagnostic reporter could not run. The original failure remains fatal.'
  }
}
try {
if ($env:GITHUB_ACTIONS -ne 'true' -or $env:RUNNER_OS -ne 'Windows') {
  throw 'Installed-app acceptance is restricted to a disposable Windows GitHub runner.'
}
if ($env:LUCI_API) { throw 'LUCI_API would bypass the installed app-owned sidecar.' }
$candidate = (Resolve-Path -LiteralPath $Installer).Path
$prior = if ($PreviousInstaller) { (Resolve-Path -LiteralPath $PreviousInstaller).Path } else { '' }
if ($PreviousVersion -and (-not $prior -or $PreviousVersion -notmatch '^\d+\.\d+\.\d+$')) { throw 'PreviousVersion requires a previous installer and an exact semantic version.' }
$evidencePath = [IO.Path]::GetFullPath($Evidence)
$taskRoot = Join-Path $env:RUNNER_TEMP ('luci-installed-' + [guid]::NewGuid().ToString('N'))
$installDir = Join-Path $taskRoot 'app'
$toolsDir = Join-Path $taskRoot 'tools'
$stateDir = Join-Path $env:APPDATA 'ai.lcfr.luci'
if (Test-Path -LiteralPath $stateDir) { throw "Runner is not fresh: Luci state already exists at $stateDir" }
$legacyStateDir = Join-Path $env:LOCALAPPDATA 'Luci\state'
if (Test-Path -LiteralPath $legacyStateDir) { throw "Runner is not fresh: legacy Luci state already exists at $legacyStateDir" }
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
} catch {
  Write-InstalledFailure $_
  throw
}

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
function Run-UI([string]$Mode, [string]$PassName, [string]$ExpectedVersion = '') {
  $script:stage = $PassName
  & node (Join-Path $PSScriptRoot 'native-installed-eval.mjs') (Join-Path $installDir 'luci.exe') $evidencePath $Mode $ExpectedVersion
  if ($LASTEXITCODE -ne 0) { throw "Installed WebView journey failed: $PassName" }
  Copy-Item -LiteralPath (Join-Path $evidencePath "$Mode-result.json") -Destination (Join-Path $evidencePath "$PassName-result.json")
  $receipt.checks += $PassName
  $script:completedChecks = $receipt.checks.Count
}
function Preserved-Hashes {
  $script:stage = 'preserve-user-data'
  $fixture = Get-Content -LiteralPath (Join-Path $evidencePath 'private-fixture.json') -Raw | ConvertFrom-Json
  $selectedStateDir = $fixture.stateDir
  $project = Join-Path $fixture.projectRoot $fixture.projectName
  $hashes = [ordered]@{}
  foreach ($name in @('LUCI.md','.luci\state.json','.git\HEAD')) {
    $path = Join-Path $project $name
    $hashes[$path] = (Get-FileHash -Algorithm SHA256 -LiteralPath $path).Hash
  }
  # 0.3 stores its canonical design in state.json; later versions also export it.
  $designExport = Join-Path $project '.luci\design.json'
  if (Test-Path -LiteralPath $designExport) { $hashes[$designExport] = (Get-FileHash -Algorithm SHA256 -LiteralPath $designExport).Hash }
  if ($fixture.PSObject.Properties['hasPlan'] -and $fixture.hasPlan) {
    $plan = Join-Path $project '.luci\plan.json'
    $hashes[$plan] = (Get-FileHash -Algorithm SHA256 -LiteralPath $plan).Hash
  }
  if ($fixture.PSObject.Properties['hasConfiguration'] -and $fixture.hasConfiguration) {
    $privateConfig = @(Get-ChildItem -LiteralPath (Join-Path $selectedStateDir 'sessions') -File -Filter 'application-config-*.json')
    if ($privateConfig.Count -ne 1) { throw 'Expected one private runtime configuration outside the project' }
    $hashes[$privateConfig[0].FullName] = (Get-FileHash -Algorithm SHA256 -LiteralPath $privateConfig[0].FullName).Hash
  }
  $identity = Join-Path $selectedStateDir 'control-plane\identity.json'
  if (Test-Path -LiteralPath $identity) { $hashes[$identity] = (Get-FileHash -Algorithm SHA256 -LiteralPath $identity).Hash }
  return $hashes
}
function Assert-Preserved($Before) {
  $script:stage = 'preserve-user-data'
  foreach ($path in $Before.Keys) {
    if (-not (Test-Path -LiteralPath $path)) { throw "Installer removed user data: $path" }
    if ((Get-FileHash -Algorithm SHA256 -LiteralPath $path).Hash -ne $Before[$path]) { throw "Installer changed user data: $path" }
  }
}

try {
  # Official external driver. No debug plugin or native app modification.
  $script:stage = 'install-tauri-driver'
  & cargo install tauri-driver --version 2.1.0 --locked --root $toolsDir
  if ($LASTEXITCODE -ne 0) { throw 'Pinned tauri-driver installation failed' }
  $script:stage = 'find-webview-runtime'
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
  $script:stage = 'download-edge-driver'
  Invoke-WebRequest -Uri "https://msedgedriver.microsoft.com/$runtimeVersion/edgedriver_win64.zip" -OutFile $driverZip
  Expand-Archive -LiteralPath $driverZip -DestinationPath $toolsDir
  $nativeDriver = Join-Path $toolsDir 'msedgedriver.exe'
  $script:stage = 'verify-edge-driver'
  $driverVersion = (& $nativeDriver --version | Out-String).Trim()
  if ($driverVersion -notmatch [regex]::Escape($runtimeVersion)) { throw "Mismatched Edge driver: $driverVersion / $runtimeVersion" }
  $receipt.tauriDriver = '2.1.0'
  $receipt.webviewVersion = $runtimeVersion
  $receipt.edgeDriverVersion = $driverVersion
  $receipt.edgeDriverSHA256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $nativeDriver).Hash.ToLowerInvariant()
  # Runtime >=150 deliberately rejects WebDriver's environment arguments in an
  # elevated host. Exercise the normal user token, not an HKLM policy override or
  # an older browser. See MicrosoftEdge/WebView2Feedback#5645 and tauri-apps/wry#1782.
  $gsudoZip = Join-Path $toolsDir 'gsudo.zip'
  $script:stage = 'download-gsudo'
  Invoke-WebRequest -Uri 'https://github.com/gerardog/gsudo/releases/download/v2.6.1/gsudo.portable.zip' -OutFile $gsudoZip
  $script:stage = 'verify-gsudo'
  if ((Get-FileHash -Algorithm SHA256 -LiteralPath $gsudoZip).Hash.ToLowerInvariant() -ne '21130bf178d7b9891207f00bff56f05b6b363ce9cec7b2d084e45fb12ee51f44') { throw 'Pinned gsudo archive checksum differs' }
  Expand-Archive -LiteralPath $gsudoZip -DestinationPath (Join-Path $toolsDir 'gsudo')
  $gsudo = Join-Path $toolsDir 'gsudo\x64\gsudo.exe'
  $script:stage = 'verify-medium-integrity'
  $runnerSID = [Security.Principal.WindowsIdentity]::GetCurrent().User.Value
  & icacls.exe $taskRoot /grant "*$($runnerSID):(OI)(CI)M" | Out-Null
  if ($LASTEXITCODE -ne 0) { throw 'Could not grant the runner user access to its test directory' }
  $groups = (& $gsudo --integrity Medium --direct whoami.exe /groups | Out-String)
  if ($LASTEXITCODE -ne 0 -or $groups -notmatch 'S-1-16-8192') { throw 'The WebDriver process must use a verified Medium-integrity token' }
  $receipt.driverIntegrity = 'Medium (S-1-16-8192)'
  $receipt.gsudo = '2.6.1; MIT; CI-only; archive SHA256 verified'
  $script:stage = 'start-driver'
  $driver = Start-Process -FilePath $gsudo -ArgumentList @('--integrity','Medium','--direct',(Join-Path $toolsDir 'bin\tauri-driver.exe'),'--native-driver',$nativeDriver) -PassThru -WindowStyle Hidden -RedirectStandardOutput (Join-Path $evidencePath 'driver-out.log') -RedirectStandardError (Join-Path $evidencePath 'driver-error.log')
  if ($prior) {
    $receipt.previousInstaller = [IO.Path]::GetFileName($prior)
    $receipt.previousInstallerSHA256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $prior).Hash.ToLowerInvariant()
    $receipt.previousInstallerVersion = $PreviousVersion
    $script:stage = 'install-previous'
    Install-Candidate $prior
    Run-UI 'seed' 'previous-version-seed' $PreviousVersion
    $before = Preserved-Hashes
    $script:stage = 'install-candidate'
    Install-Candidate $candidate
    Assert-Preserved $before
    Run-UI 'verify' 'version-upgrade'
  } else {
    $script:stage = 'install-candidate'
    Install-Candidate $candidate
    Run-UI 'seed' 'fresh-install'
  }
  Run-UI 'verify' 'clean-restart'
  $before = Preserved-Hashes
  $script:stage = 'reinstall-candidate'
  Install-Candidate $candidate
  Assert-Preserved $before
  Run-UI 'verify' 'same-version-reinstall'
  $before = Preserved-Hashes
  $script:stage = 'uninstall'
  $uninstallers = @(Get-ChildItem -LiteralPath $installDir -File -Filter '*uninstall*.exe')
  if ($uninstallers.Count -ne 1) { throw 'Expected one installed NSIS uninstaller.' }
  Run-Installer $uninstallers[0].FullName @('/S')
  # The NSIS launcher may return before its copied uninstaller completes. The
  # executable is removed before thousands of bundled resource files; wait for
  # every owned resource root, not just the first deleted executable.
  $script:stage = 'verify-uninstall'
  $uninstallTargets = @('luci.exe','luci-api.exe','runtime','licenses','cloud','docs')
  $deadline = (Get-Date).AddSeconds(30)
  do {
    $remaining = @($uninstallTargets | Where-Object { Test-Path -LiteralPath (Join-Path $installDir $_) })
    if ($remaining.Count -eq 0) { break }
    Start-Sleep -Milliseconds 200
  } while ((Get-Date) -lt $deadline)
  Assert-Preserved $before
  $script:stage = 'verify-uninstall'
  if ($remaining.Count -gt 0) {
    $receipt.remainingInstalledEntries = @(Get-ChildItem -LiteralPath $installDir -Recurse -Force | Select-Object -First 200 | ForEach-Object {
      @{ path=$_.FullName.Substring($installDir.Length + 1); directory=$_.PSIsContainer; bytes=if ($_.PSIsContainer) { 0 } else { $_.Length } }
    })
    throw "Uninstall retained app resources after 30 seconds: $($remaining -join ', ')"
  }
  $receipt.checks += 'Uninstall removed installed app/resources and preserved project, design, Git and identity data'
  $receipt.passed = $true
} catch {
  $receipt.error = $_.Exception.Message
  $receipt.failedStage = $script:stage
  Write-InstalledFailure $_
  throw
} finally {
  # The exact per-run tools path owns every process below; no generic process
  # names or other apps are terminated, including after the wrapper has exited.
  Get-CimInstance Win32_Process | Where-Object { $_.ExecutablePath -and $_.ExecutablePath.StartsWith($toolsDir + '\', [StringComparison]::OrdinalIgnoreCase) } | ForEach-Object { Stop-Process -Id $_.ProcessId -ErrorAction SilentlyContinue }
  [IO.File]::WriteAllText((Join-Path $evidencePath 'installer-receipt.json'), ($receipt | ConvertTo-Json -Depth 10), [Text.UTF8Encoding]::new($false))
  # The ephemeral test password is never uploaded as evidence.
  $privateFixture = Join-Path $evidencePath 'private-fixture.json'
  if (Test-Path -LiteralPath $privateFixture) { Remove-Item -LiteralPath $privateFixture }
}
