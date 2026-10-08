#requires -Version 5.1
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$AcceptanceRoot,
    [ValidateRange(1024, 65535)][int]$Port = 18730,
    [ValidateRange(1024, 65535)][int]$FrontendPort = 14200,
    [ValidateRange(8, 2048)][int]$MinimumFreeGiB = 40,
    [switch]$PrepareOnly
)

$ErrorActionPreference = 'Stop'
if ($env:OS -ne 'Windows_NT') { throw 'Run this launcher in native Windows PowerShell or PowerShell on Windows.' }
$repoRoot = Split-Path -Parent $PSScriptRoot
$root = [System.IO.Path]::GetFullPath($AcceptanceRoot)
if (-not [System.IO.Path]::IsPathRooted($AcceptanceRoot) -or $root.StartsWith('\\')) { throw 'Choose an absolute folder on a local roomy drive, for example D:\LuciAcceptance.' }
$repoBoundary = $repoRoot.TrimEnd('\', '/') + [System.IO.Path]::DirectorySeparatorChar
if ($root.Equals($repoRoot, [StringComparison]::OrdinalIgnoreCase) -or $root.StartsWith($repoBoundary, [StringComparison]::OrdinalIgnoreCase)) { throw 'Keep acceptance state outside the source checkout.' }
foreach ($tool in @('git.exe', 'go.exe', 'npm.cmd', 'docker.exe')) {
    if (-not (Get-Command $tool -ErrorAction SilentlyContinue)) { throw "Missing $tool. Install the prerequisites in docs/luci/LOCAL-WINDOWS-ACCEPTANCE.md first." }
}

# Test the actual selected port before creating state. Never attach to or stop
# an existing Luci service, and never repurpose its current state directory.
if ($Port -eq $FrontendPort) { throw 'Choose distinct API and frontend ports.' }
foreach ($candidatePort in @($Port, $FrontendPort)) {
    $listener = New-Object System.Net.Sockets.TcpListener([System.Net.IPAddress]::Loopback, $candidatePort)
    $listener.Server.ExclusiveAddressUse = $true
    try { $listener.Start() } catch { throw "Port $candidatePort is already occupied. Choose another -Port or -FrontendPort; the existing service has not been changed." } finally { $listener.Stop() }
}
$engine = & docker.exe info --format '{{.OSType}}'
if ($LASTEXITCODE -ne 0 -or ($engine | Out-String).Trim() -ne 'linux') { throw 'Start Docker Desktop with its Linux engine before acceptance.' }

if (-not ('LuciAcceptanceNative' -as [type])) {
    Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
public static class LuciAcceptanceNative {
    [StructLayout(LayoutKind.Sequential)] public struct MemoryStatus {
        public uint Length, MemoryLoad;
        public ulong TotalPhysical, AvailablePhysical, CommitLimit, AvailableCommit, TotalVirtual, AvailableVirtual, ExtendedVirtual;
    }
    [DllImport("kernel32.dll", SetLastError=true)] public static extern bool GlobalMemoryStatusEx(ref MemoryStatus value);
    [DllImport("kernel32.dll")] public static extern uint GetActiveProcessorCount(ushort group);
    [DllImport("kernel32.dll", CharSet=CharSet.Unicode, SetLastError=true)] public static extern bool GetDiskFreeSpaceEx(string path, out ulong available, out ulong total, out ulong free);
}
'@
}

New-Item -ItemType Directory -Path $root -Force | Out-Null
[UInt64]$available = 0; [UInt64]$total = 0; [UInt64]$free = 0
if (-not [LuciAcceptanceNative]::GetDiskFreeSpaceEx($root, [ref]$available, [ref]$total, [ref]$free)) { throw 'The selected volume capacity could not be measured.' }
if ($available -lt ([UInt64]$MinimumFreeGiB * 1GB)) { throw "The selected volume needs at least $MinimumFreeGiB GiB free for this acceptance run. Choose a roomier drive; no application state was created." }
$memory = New-Object LuciAcceptanceNative+MemoryStatus
$memory.Length = [System.Runtime.InteropServices.Marshal]::SizeOf($memory)
if (-not [LuciAcceptanceNative]::GlobalMemoryStatusEx([ref]$memory)) { throw 'Native physical-memory and commitment measurements are unavailable.' }

$runRoot = Join-Path $root ('luci-' + (Get-Date -Format 'yyyyMMdd-HHmmss') + '-' + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $runRoot | Out-Null
# Protect the new run from inherited broad grants. This changes only the new
# GUID-named folder, never an existing app/profile/data directory.
$sid = [System.Security.Principal.WindowsIdentity]::GetCurrent().User
$acl = New-Object System.Security.AccessControl.DirectorySecurity
$acl.SetOwner($sid)
$acl.SetAccessRuleProtection($true, $false)
$rule = New-Object System.Security.AccessControl.FileSystemAccessRule($sid, 'FullControl', 'ContainerInherit,ObjectInherit', 'None', 'Allow')
$acl.AddAccessRule($rule)
Set-Acl -LiteralPath $runRoot -AclObject $acl
$state = Join-Path $runRoot 'state'; $projects = Join-Path $runRoot 'projects'; $artifacts = Join-Path $runRoot 'artifacts'
foreach ($path in @($state, $projects, $artifacts)) { New-Item -ItemType Directory -Path $path | Out-Null }
$revision = (& git.exe -C $repoRoot rev-parse HEAD | Out-String).Trim()
if ($LASTEXITCODE -ne 0) { throw 'The checkout revision could not be recorded.' }
$manifest = [ordered]@{
    version = 1; observedAt = [DateTime]::UtcNow.ToString('o'); revision = $revision
    runRoot = $runRoot; stateDir = $state; projectsDir = $projects; artifactsDir = $artifacts
    apiAddress = "127.0.0.1:$Port"; frontendPort = $FrontendPort
    physicalTotalBytes = $memory.TotalPhysical; physicalAvailableBytes = $memory.AvailablePhysical
    logicalProcessorCount = [LuciAcceptanceNative]::GetActiveProcessorCount(0xffff)
    commitLimitBytes = $memory.CommitLimit; commitAvailableBytes = $memory.AvailableCommit
    selectedVolumeTotalBytes = $total; selectedVolumeAvailableBytes = $available
    scope = 'Native Windows baseline; no models dispatched; Docker VM allocation is separate from host physical RAM.'
    nativeSwarmAcceptance = 'unverified'
}
$manifest | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath (Join-Path $artifacts 'windows-baseline.json') -Encoding UTF8
Write-Host "Isolated run: $runRoot"
Write-Host "Revision: $revision"
Write-Host ('Host RAM: {0:N2} GiB total; {1:N2} GiB currently available.' -f ($memory.TotalPhysical / 1GB), ($memory.AvailablePhysical / 1GB))
Write-Host 'Docker VM memory is a separate allocation. Keep your ordinary browser/editor open during the capacity acceptance.'
Write-Host "Private baseline: $(Join-Path $artifacts 'windows-baseline.json')"
Write-Host 'Follow docs/luci/SWARM-CAPACITY-ACCEPTANCE.md for four-project work, recovery and whole-computer responsiveness evidence.'
if ($PrepareOnly) { Write-Host 'Prepared only. Re-run without -PrepareOnly to create and launch a separate fresh run.'; return }

$saved = @{}
# Remove inherited Luci service/worker/managed settings only in this process;
# old values stay in memory and are restored without being logged or persisted.
foreach ($entry in (Get-ChildItem Env:LUCI_*)) { $saved[$entry.Name] = $entry.Value }
try {
    foreach ($name in $saved.Keys) { [Environment]::SetEnvironmentVariable($name, $null, 'Process') }
    $env:LUCI_STATE_DIR = $state; $env:LUCI_PROJECTS_DIR = $projects; $env:LUCI_ADDR = "127.0.0.1:$Port"
    Write-Host 'Launching the source app. Close this run with Ctrl+C; all evidence and project data remain in its isolated folder.'
    & (Join-Path $PSScriptRoot 'dev.ps1') -Port $Port -FrontendPort $FrontendPort -DevDirectory (Join-Path $artifacts 'dev')
} finally {
    foreach ($entry in (Get-ChildItem Env:LUCI_*)) { [Environment]::SetEnvironmentVariable($entry.Name, $null, 'Process') }
    foreach ($name in $saved.Keys) { [Environment]::SetEnvironmentVariable($name, $saved[$name], 'Process') }
}
