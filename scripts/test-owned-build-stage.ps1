#requires -Version 7.0
param([string]$OutputDirectory = (Join-Path ([IO.Path]::GetTempPath()) 'luci-build-stage-tests'))
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
. (Join-Path $PSScriptRoot 'owned-build-stage.ps1')
$luciFixture = Join-Path ([IO.Path]::GetFullPath($OutputDirectory)) ('fixture-' + [guid]::NewGuid().ToString('N'))
$luciWorkspace = Join-Path $luciFixture 'workspace'
$luciVersion = '0.4.99'
$luciRelease = Join-Path $luciWorkspace ('artifacts/releases/' + $luciVersion)
$luciStageName = '.stage-' + [guid]::NewGuid().ToString('N')
$luciOutside = Join-Path $luciFixture 'preserved'
$luciResults = [Collections.Generic.List[object]]::new()
New-Item -ItemType Directory -Path $luciRelease,$luciOutside -Force | Out-Null
Set-Content -LiteralPath (Join-Path $luciOutside 'keep.txt') -Value 'must survive'

function Assert-Refused([string]$Name, [string]$Path) {
    $luciRefused = $false
    try { Remove-LuciCompletedBuildStage -WorkspaceRoot $luciWorkspace -Version $luciVersion -StagePath $Path | Out-Null }
    catch { $luciRefused = $true }
    if (-not $luciRefused) { throw ($Name + ' was incorrectly accepted.') }
    if (-not (Test-Path -LiteralPath (Join-Path $luciOutside 'keep.txt'))) { throw 'An unrelated fixture file was removed.' }
    $luciResults.Add(@{name=$Name;passed=$true})
}

# Real removal, with a nested payload, proves the successful cleanup path.
$luciOwned = Join-Path $luciRelease $luciStageName
New-Item -ItemType Directory -Path (Join-Path $luciOwned 'nested') -Force | Out-Null
Set-Content -LiteralPath (Join-Path $luciOwned 'nested/generated.bin') -Value 'generated'
if (-not (Remove-LuciCompletedBuildStage -WorkspaceRoot $luciWorkspace -Version $luciVersion -StagePath $luciOwned) -or (Test-Path -LiteralPath $luciOwned)) { throw 'Completed owned stage was retained.' }
$luciResults.Add(@{name='completed stage removed';passed=$true})
Assert-Refused 'outside workspace preserved' (Join-Path $luciOutside $luciStageName)
Assert-Refused 'release root preserved' $luciRelease
Assert-Refused 'other version refused' (Join-Path (Split-Path -Parent $luciRelease) ('0.4.98/' + $luciStageName))
Assert-Refused 'ordinary directory refused' (Join-Path $luciRelease 'luci-windows-amd64')
Assert-Refused 'nested stage refused' (Join-Path $luciRelease ('nested/' + $luciStageName))
Assert-Refused 'relative stage refused' $luciStageName

if ($IsWindows) {
    $luciLinkedStage = Join-Path $luciRelease ('.stage-' + [guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $luciLinkedStage | Out-Null
    $luciJunction = Join-Path $luciLinkedStage 'linked'
    New-Item -ItemType Junction -Path $luciJunction -Target $luciOutside | Out-Null
    Assert-Refused 'descendant junction preserved' $luciLinkedStage
    if (-not (Test-Path -LiteralPath $luciJunction)) { throw 'Refused stage was modified.' }
    # A second real junction covers a linked ancestor rather than a child.
    $luciAncestorWorkspace = Join-Path $luciFixture 'linked-workspace'
    New-Item -ItemType Junction -Path $luciAncestorWorkspace -Target $luciWorkspace | Out-Null
    $luciRefused = $false
    try { Remove-LuciCompletedBuildStage -WorkspaceRoot $luciAncestorWorkspace -Version $luciVersion -StagePath (Join-Path $luciAncestorWorkspace ('artifacts/releases/' + $luciVersion + '/' + $luciStageName)) | Out-Null }
    catch { $luciRefused = $true }
    if (-not $luciRefused -or -not (Test-Path -LiteralPath $luciAncestorWorkspace)) { throw 'Linked ancestor was accepted or modified.' }
    $luciResults.Add(@{name='ancestor junction preserved';passed=$true})
}
# Keep the small fixture and refusal evidence for inspection. No generic test
# teardown recursively removes the junction cases.
@{passed=$true;fixture=$luciFixture;results=@($luciResults)} | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath (Join-Path $luciFixture 'results.json') -Encoding utf8NoBOM
Write-Host ('Build stage boundary checks passed: ' + $luciResults.Count)
