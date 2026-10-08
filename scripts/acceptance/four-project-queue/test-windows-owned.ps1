#requires -Version 5.1
param([string]$ReportPath = '')
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$source = Join-Path $PSScriptRoot 'sample-windows-owned.ps1'
$tokens = $null; $parseErrors = $null
$tree = [Management.Automation.Language.Parser]::ParseFile($source, [ref]$tokens, [ref]$parseErrors)
if ($parseErrors.Count) { throw 'Sampler syntax did not parse.' }
# Load only function definitions, not the script's entry point or platform guard.
# All OS/Docker read functions below are replaced with controlled fixtures.
foreach ($node in $tree.EndBlock.Statements) {
    if ($node -is [Management.Automation.Language.FunctionDefinitionAst]) { . ([scriptblock]::Create($node.Extent.Text)) }
}
Add-Type -Path (Join-Path $PSScriptRoot 'windows-resource-native.cs')
$checks = [Collections.Generic.List[object]]::new()
function Assert-True($Value, [string]$Message) { if (-not $Value) { throw $Message } }
function Check([string]$Name, [scriptblock]$Body) { & $Body; $checks.Add(@{name = $Name; passed = $true}) }
function Process-Row([int]$Number, [UInt64]$Created, [bool]$Alive = $true) {
    return [pscustomobject]@{ProcessID = $Number; CreationFileTime = $Created; Alive = $Alive; CPU100ns = [UInt64]10000000; WorkingSetBytes = [UInt64]512; PrivateCommitBytes = [UInt64]1024; ReadBytes = [UInt64]10; WriteBytes = [UInt64]20; OtherBytes = [UInt64]5}
}
$script:fixture = @{}
function Read-LuciProcessNative([int]$ProcessNumber) { return $script:fixture.processes[$ProcessNumber] }
function Read-LuciTopologyNative { return [pscustomobject]@{Rows = $script:fixture.topology; Complete = $true} }
function Read-LuciHostNative { return $script:fixture.host }
function Read-LuciDiskNative([string]$Path) { return $script:fixture.disk }
function Invoke-LuciDockerGet([string]$PipeName, [string]$Path, [int]$TimeoutMilliseconds) {
    $script:fixture.dockerCalls++
    Assert-True ($TimeoutMilliseconds -gt 0 -and $TimeoutMilliseconds -le 2000) 'Docker deadline was lost.'
    if ($script:fixture.ContainsKey('delay')) { Start-Sleep -Milliseconds $script:fixture.delay }
    if (-not $script:fixture.responses.ContainsKey($Path)) { throw 'Unexpected Docker read path.' }
    return $script:fixture.responses[$Path]
}
Check 'Reject ambiguous process roots and unsafe Docker/path scope' {
    $valid = @{version = 1; roots = @(@{processId = 100; creationFileTimeUtc = '1000'; role = 'control-plane'}); disks = @(@{role = 'projects'; path = 'C:\acceptance'}); docker = $null}
    $scope = ConvertTo-LuciResourceScope $valid
    Assert-True ($scope.roots[0].creationFileTimeUtc -ceq '1000') 'Creation identity changed.'
    foreach ($case in @('duplicate', 'negative', 'unknown-role', 'zero-identity', 'unc', 'partial-docker')) {
        $value = $valid | ConvertTo-Json -Depth 8 | ConvertFrom-Json
        switch ($case) {
            'duplicate' { $value.roots = @($value.roots[0], $value.roots[0]) }
            'negative' { $value.roots[0].processId = -1 }
            'unknown-role' { $value.roots[0].role = 'private-browser-name' }
            'zero-identity' { $value.roots[0].creationFileTimeUtc = '0' }
            'unc' { $value.disks[0].path = '\\remote\share' }
            'partial-docker' { $value.docker = @{instance = 'own'} }
        }
        $failed = $false; try { ConvertTo-LuciResourceScope $value | Out-Null } catch { $failed = $true }
        Assert-True $failed ('Accepted invalid scope: ' + $case)
    }
}
Check 'Exact identity tree excludes foreign and historical-parent children' {
    $script:fixture = @{processes = @{100 = (Process-Row 100 1000); 200 = (Process-Row 200 1100); 300 = (Process-Row 300 1200); 400 = (Process-Row 400 900)}; topology = @(@{ProcessID = 200; ParentID = 100}, @{ProcessID = 300; ParentID = 99}, @{ProcessID = 400; ParentID = 100})}
    $tracked = @{100 = [pscustomobject]@{creationFileTimeUtc = '1000'; role = 'control-plane'}}
    $result = Get-LuciOwnedProcesses $tracked @{} 1
    Assert-True (($result.rows.processId -join ',') -eq '100,200') 'Foreign or historical child was attributed.'
    Assert-True ($result.rows[1].role -eq 'control-plane') 'Child lost inherited owner role.'
    $script:fixture.processes[100] = Process-Row 100 5000
    $tracked = @{100 = [pscustomobject]@{creationFileTimeUtc = '1000'; role = 'control-plane'}}
    $result = Get-LuciOwnedProcesses $tracked @{} 2
    Assert-True ($result.rows.Count -eq 0 -and $result.rejectedReusedPIDs -eq 1) 'Reused root acquired another process tree.'
}
Check 'Known children survive parent exit; reused child does not inherit CPU deltas' {
    $script:fixture = @{processes = @{100 = (Process-Row 100 1000 $false); 200 = (Process-Row 200 1100); 210 = (Process-Row 210 1200)}; topology = @(@{ProcessID = 200; ParentID = 100}, @{ProcessID = 210; ParentID = 200})}
    $tracked = @{100 = [pscustomobject]@{creationFileTimeUtc = '1000'; role = 'control-plane'}; 200 = [pscustomobject]@{creationFileTimeUtc = '1100'; role = 'control-plane'}}
    $result = Get-LuciOwnedProcesses $tracked @{} 1
    Assert-True (($result.rows.processId -join ',') -eq '200,210') 'Reparented known child or its new descendant was lost.'
    $script:fixture.processes[100] = Process-Row 100 1000; $script:fixture.processes[200] = Process-Row 200 5000
    $previous = @{'200:1100' = @{elapsed = 1; cpu = 900000000; read = 999; write = 999}}
    $result = Get-LuciOwnedProcesses $tracked $previous 2
    $child = @($result.rows | Where-Object { $_.processId -eq 200 })[0]
    Assert-True ($child.creationFileTimeUtc -eq '5000' -and $null -eq $child.cpuCoreEquivalent) 'New proven descendant inherited a reused PID counter.'
}
Check 'Missing process memory and I/O remain null; deltas require same identity' {
    $row = Process-Row 100 1000; $row.WorkingSetBytes = $null; $row.PrivateCommitBytes = $null; $row.ReadBytes = $null
    $script:fixture = @{processes = @{100 = $row}; topology = @()}
    $tracked = @{100 = [pscustomobject]@{creationFileTimeUtc = '1000'; role = 'browser'}}; $previous = @{}
    $first = Get-LuciOwnedProcesses $tracked $previous 1
    Assert-True ($null -eq $first.rows[0].workingSetBytes -and $null -eq $first.rows[0].ioReadBytes -and $first.summedWorkingSetBytesByRole.Count -eq 0) 'Unavailable metrics became zero.'
    Assert-True (-not $first.workingSetComplete) 'Incomplete working-set sum reported complete.'
    $row.CPU100ns = [UInt64]30000000
    $next = Get-LuciOwnedProcesses $tracked $previous 3
    Assert-True ($next.rows[0].cpuCoreEquivalent -eq 1 -and $null -eq $next.rows[0].ioDelta) 'CPU or missing I/O delta is incorrect.'
}
Check 'Root mismatch fails before creating output or reading Docker' {
    $script:fixture = @{processes = @{100 = (Process-Row 100 2000)}}
    $scope = [pscustomobject]@{roots = @([pscustomobject]@{processId = 100; creationFileTimeUtc = '1000'; role = 'browser'})}
    $failed = $false; try { Invoke-LuciWindowsSampler $scope 'C:\must-not-be-created' 1 1 100 } catch { $failed = $_.Exception.Message -like 'A root process*' }
    Assert-True $failed 'Mismatched root proceeded into native output setup.'
}
Check 'Host CPU uses deltas; unavailable disk and memory are not invented' {
    $script:fixture = @{host = [pscustomobject]@{CPUTotalTicks = 1000; CPUIdleTicks = 300; PhysicalTotalBytes = $null; PhysicalAvailableBytes = $null; CommitLimitBytes = 8000; CommitAvailableBytes = 4000; LogicalProcessors = 8}; disk = $null}
    $scope = [pscustomobject]@{disks = @([pscustomobject]@{role = 'projects'; path = 'C:\private-path-never-exported'})}; $previous = @{}
    $first = Get-LuciHostSample $scope $previous
    Assert-True ($null -eq $first.cpuUtilizationPercent -and $null -eq $first.physicalAvailableBytes -and $null -eq $first.disks[0].availableBytes) 'Missing host data became a guess.'
    $script:fixture.host.CPUTotalTicks = 2000; $script:fixture.host.CPUIdleTicks = 700
    $next = Get-LuciHostSample $scope $previous
    Assert-True ($next.cpuUtilizationPercent -eq 60 -and ($next | ConvertTo-Json -Depth 8) -notmatch 'private-path') 'Host scope or CPU calculation is incorrect.'
}
Check 'Docker reads only exact owned image and exports selected metrics' {
    $id = 'a' * 64; $foreign = 'b' * 64; $scope = @{instance = 'exact-own'; imageId = ('sha256:' + ('c' * 64)); pipeName = 'docker_engine'}
    $script:fixture = @{dockerCalls = 0; responses = @{ '/info' = @{OSType = 'linux'; MemTotal = 8000; NCPU = 4}; '/containers/json?all=1' = @(@{Id = $id; ImageID = $scope.imageId; State = 'running'; Labels = @{'luci.instance' = 'exact-own'; 'luci.managed' = 'true'}; Command = 'PRIVATE_COMMAND_MARKER'}, @{Id = $foreign; Labels = @{'luci.instance' = 'foreign'; 'luci.managed' = 'true'}}) }}
    $inspect = @{Config = @{Labels = @{'luci.instance' = 'exact-own'; 'luci.managed' = 'true'}; Env = @('SECRET_MARKER=must-not-export')}; Image = $scope.imageId; State = @{Pid = 33; Running = $true}; HostConfig = @{Memory = 1000; NanoCpus = 1000000000}; Mounts = @('private-mount-marker')}
    # Full inspection is intentionally absent: any attempt to request it fails the fixture.
    $stats = @{read = [DateTimeOffset]::UtcNow.ToString('o'); memory_stats = @{usage = 500; stats = @{inactive_file = 100}}; cpu_stats = @{cpu_usage = @{total_usage = 200000000}}; blkio_stats = @{}}
    $script:fixture.responses['/containers/' + $id + '/stats?stream=false&one-shot=true'] = $stats
    $result = Get-LuciDockerSample $scope 600 @{}
    Assert-True ($script:fixture.dockerCalls -eq 3) 'Observer requested a full Docker inspection or another unneeded endpoint.'
    Assert-True ($result.status -eq 'observed' -and $result.containers.Count -eq 1 -and $result.containers[0].workingSetBytes -eq 400) 'Exact-owned Docker statistics were not recorded.'
    Assert-True ($null -eq $result.containers[0].blockReadBytes -and $null -eq $result.containers[0].cpuCoreEquivalent) 'Unknown first-delta Docker data became zero.'
    Assert-True (($result | ConvertTo-Json -Depth 15) -notmatch 'SECRET_MARKER|private-mount-marker|PRIVATE_COMMAND_MARKER|Env|Mounts') 'Private Docker data escaped the field allowlist.'
    $script:fixture.responses['/containers/json?all=1'][0].ImageID = 'sha256:' + ('d' * 64)
    $result = Get-LuciDockerSample $scope 600 @{}
    Assert-True ($result.containers.Count -eq 0 -and $result.issues -contains 'container-identity-changed-or-image-mismatch') 'An unexpected image was attributed to the fixture.'
    $script:fixture.responses['/containers/json?all=1'][0].ImageID = $scope.imageId; $stats.read = [DateTimeOffset]::UtcNow.AddMinutes(-1).ToString('o')
    $result = Get-LuciDockerSample $scope 600 @{}
    Assert-True ($null -eq $result.containers[0].memoryUsageBytes -and $result.issues -contains 'container-statistics-not-fresh') 'Stale Docker usage was reported as current.'
}
Check 'Missing Docker scope makes no call; budget exhaustion is explicit' {
    $script:fixture = @{dockerCalls = 0; responses = @{}}
    $result = Get-LuciDockerSample $null 100 @{}
    Assert-True ($script:fixture.dockerCalls -eq 0 -and $result.status -eq 'unavailable') 'Absent scope probed Docker.'
    $script:fixture = @{dockerCalls = 0; delay = 120; responses = @{'/info' = @{OSType = 'linux'; MemTotal = 1000; NCPU = 1}}}
    $result = Get-LuciDockerSample @{instance = 'own'; imageId = ('sha256:' + ('a' * 64)); pipeName = 'docker_engine'} 100 @{}
    Assert-True ($script:fixture.dockerCalls -eq 1 -and -not $result.inventoryComplete -and $result.status -eq 'partial') 'Docker deadline kept issuing calls.'
}
Check 'Native HTTP parser handles bounded JSON and rejects error/truncation' {
    $bytes = [Text.Encoding]::UTF8.GetBytes("HTTP/1.1 200 OK`r`nContent-Length: 7`r`n`r`n{`"a`":1}")
    Assert-True (([LuciWindowsResourceNative]::DecodeResponse($bytes) | ConvertFrom-Json).a -eq 1) 'Plain named-pipe HTTP response failed.'
    $bytes = [Text.Encoding]::UTF8.GetBytes("HTTP/1.1 200 OK`r`nTransfer-Encoding: chunked`r`n`r`n3`r`n{`"a`r`n4`r`n`":1}`r`n0`r`n`r`n")
    Assert-True (([LuciWindowsResourceNative]::DecodeResponse($bytes) | ConvertFrom-Json).a -eq 1) 'Chunked named-pipe HTTP response failed.'
    foreach ($text in @("HTTP/1.1 500 Error`r`n`r`nSECRET_BODY", "HTTP/1.1 200 OK`r`nTransfer-Encoding: chunked`r`n`r`nffff`r`nshort")) {
        $failed = $false; try { [LuciWindowsResourceNative]::DecodeResponse([Text.Encoding]::UTF8.GetBytes($text)) | Out-Null } catch { $failed = $true; Assert-True ($_.Exception.Message -notmatch 'SECRET_BODY') 'HTTP error exposed response contents.' }
        Assert-True $failed 'Malformed or failed HTTP response was accepted.'
    }
    foreach ($path in @('/containers/create', ('/containers/' + ('a' * 64) + '/json'))) {
        $failed = $false; try { [LuciWindowsResourceNative]::DockerGet('docker_engine', $path, 100) | Out-Null } catch { $failed = $true }
        Assert-True $failed 'Mutating/full-inspection Docker path was accepted.'
    }
}
$receipt = [ordered]@{passed = $true; platform = [Environment]::OSVersion.Platform.ToString(); powershell = $PSVersionTable.PSVersion.ToString(); scope = 'Actual PowerShell selection/serialization functions with controlled OS/Docker fixtures, plus compiled native HTTP parser; no native Windows probes, Docker connection or workload.'; nativeWindowsVerified = $false; helperSHA256 = (Get-FileHash -LiteralPath $source -Algorithm SHA256).Hash.ToLowerInvariant(); nativeProbeSHA256 = (Get-FileHash -LiteralPath (Join-Path $PSScriptRoot 'windows-resource-native.cs') -Algorithm SHA256).Hash.ToLowerInvariant(); checks = $checks.ToArray()}
if ($ReportPath) { Write-LuciNewJSON ([IO.Path]::GetFullPath($ReportPath)) $receipt }
Write-Host ('Windows owned-resource controlled checks passed: ' + $checks.Count)
