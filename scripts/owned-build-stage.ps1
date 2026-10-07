#requires -Version 7.0

function Remove-LuciCompletedBuildStage {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)][string]$WorkspaceRoot,
        [Parameter(Mandatory)][string]$Version,
        [Parameter(Mandatory)][string]$StagePath
    )

    if ($Version -notmatch '^\d+\.\d+\.\d+(?:-[a-zA-Z0-9.-]+)?$') { throw 'Invalid build-stage version.' }
    if (-not [IO.Path]::IsPathFullyQualified($WorkspaceRoot) -or -not [IO.Path]::IsPathFullyQualified($StagePath)) { throw 'Build-stage cleanup requires absolute paths.' }
    $luciComparison = if ($IsWindows) { [StringComparison]::OrdinalIgnoreCase } else { [StringComparison]::Ordinal }
    $luciWorkspace = [IO.Path]::TrimEndingDirectorySeparator([IO.Path]::GetFullPath($WorkspaceRoot))
    $luciExpectedParent = [IO.Path]::GetFullPath((Join-Path $luciWorkspace ('artifacts/releases/' + $Version)))
    $luciTarget = [IO.Path]::TrimEndingDirectorySeparator([IO.Path]::GetFullPath($StagePath))
    if (-not $luciTarget.StartsWith($luciWorkspace + [IO.Path]::DirectorySeparatorChar, $luciComparison) -or
        -not [string]::Equals((Split-Path -Parent $luciTarget), $luciExpectedParent, $luciComparison) -or
        (Split-Path -Leaf $luciTarget) -cnotmatch '^\.stage-[a-f0-9]{32}$') {
        throw 'Refusing cleanup outside the exact generated build stage.'
    }
    if (-not (Test-Path -LiteralPath $luciWorkspace -PathType Container)) { throw 'Build workspace is missing.' }
    # Check the complete ancestry before enumerating or removing anything. A
    # generated-looking path through a junction is not an owned build stage.
    $luciAncestor = $luciTarget
    while ($luciAncestor) {
        if (Test-Path -LiteralPath $luciAncestor) {
            $luciItem = Get-Item -LiteralPath $luciAncestor -Force -ErrorAction Stop
            if ($luciItem.Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'Refusing a linked build-stage path.' }
        }
        $luciParent = Split-Path -Parent $luciAncestor
        if ($luciParent -eq $luciAncestor) { break }
        $luciAncestor = $luciParent
    }
    if (-not (Test-Path -LiteralPath $luciTarget)) { return $false }
    if (-not (Test-Path -LiteralPath $luciTarget -PathType Container)) { throw 'Build stage is not a directory.' }
    foreach ($luciItem in Get-ChildItem -LiteralPath $luciTarget -Recurse -Force -ErrorAction Stop) {
        if ($luciItem.Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'Refusing a linked build-stage entry.' }
    }
    Remove-Item -LiteralPath $luciTarget -Recurse -Force -ErrorAction Stop
    return $true
}
