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
function New-LuciSDKFixture {
    $id = 'a' * 64; $workerID = 'b' * 64; $image = 'sha256:' + ('c' * 64)
    $scope = @{instance = 'exact-own'; imageId = $image; pipeName = 'docker_engine'; includeChecks = $true}
    $labels = @{'luci.check' = 'true'; 'luci.check-protocol' = '1'; 'luci.instance' = 'exact-own'; 'luci.check-engine' = 'fixture-engine'; 'luci.check-allocation' = ('checkexec-' + ('d' * 24)); 'luci.check-id' = ('check-' + ('e' * 24)); 'luci.check-command-index' = '0'; 'luci.check-scope' = ('1' * 64); 'luci.check-project' = 'p-owned'; 'luci.check-command-digest' = ('2' * 64); 'luci.check-source-digest' = ('3' * 64); 'luci.check-image' = $image}
    $check = @{Id = $id; ImageID = $image; State = 'running'; Names = @('/luci-check-' + $labels['luci.check-allocation']); Labels = $labels; Command = 'PRIVATE_COMMAND_MARKER'}
    $worker = @{Id = $workerID; ImageID = $image; State = 'running'; Labels = @{'luci.instance' = 'exact-own'; 'luci.managed' = 'true'}}
    $stats = @{id = $id; read = [DateTimeOffset]::UtcNow.ToString('o'); memory_stats = @{limit = 1000; usage = 500; stats = @{inactive_file = 100}}; cpu_stats = @{cpu_usage = @{total_usage = 1000000000}}; blkio_stats = @{}; privateField = 'SECRET_MARKER'}
    $workerStats = @{id = $workerID; read = $stats.read; memory_stats = @{usage = 250}; cpu_stats = @{cpu_usage = @{total_usage = 200000000}}; blkio_stats = @{}}
    $info = @{ID = 'fixture-engine'; OSType = 'linux'; MemTotal = 8000; NCPU = 4}
    $script:fixture = @{dockerCalls = 0; responses = @{'/info' = $info; '/containers/json?all=1' = @($worker, $check)}}
    $script:fixture.responses['/containers/' + $id + '/stats?stream=false&one-shot=true'] = $stats
    $script:fixture.responses['/containers/' + $workerID + '/stats?stream=false&one-shot=true'] = $workerStats
    return @{scope = $scope; check = $check; worker = $worker; stats = $stats; labels = $labels; info = $info; id = $id}
}
Check 'Version-one manifests and command switch explicitly opt into checks' {
    $manifest = @{version = 1; roots = @(@{processId = 100; creationFileTimeUtc = '1000'; role = 'control-plane'}); disks = @(@{role = 'projects'; path = 'C:\acceptance'}); docker = @{instance = 'exact-own'; imageId = ('sha256:' + ('c' * 64)); pipeName = 'docker_engine'}}
    $scope = ConvertTo-LuciResourceScope $manifest
    Assert-True (-not $scope.includeChecks -and -not $scope.docker.includeChecks) 'Old manifest enabled SDK checks.'
    $manifest.includeChecks = $true; $scope = ConvertTo-LuciResourceScope $manifest
    Assert-True ($scope.includeChecks -and $scope.docker.includeChecks) 'Manifest opt-in was lost.'
    $manifest.includeChecks = $false; $scope = ConvertTo-LuciResourceScope $manifest -IncludeChecks
    Assert-True $scope.docker.includeChecks 'Command-line opt-in was lost.'
    foreach ($value in @('true', 1, $null)) {
        $manifest.includeChecks = $value
        $failed = $false; try { ConvertTo-LuciResourceScope $manifest | Out-Null } catch { $failed = $true }
        Assert-True $failed 'Non-boolean includeChecks was accepted.'
    }
}
Check 'Default Docker observation remains worker-only with unchanged row shape' {
    $data = New-LuciSDKFixture; $data.scope.Remove('includeChecks')
    $result = Get-LuciDockerSample $data.scope 600 @{}
    Assert-True ($script:fixture.dockerCalls -eq 3 -and $result.status -eq 'observed' -and $result.containers.Count -eq 1 -and $result.containers[0].id -ceq $data.worker.Id) 'Default observation included SDK checks or lost workers.'
    Assert-True (-not (Test-LuciFieldPresent $result 'includeChecks') -and -not (Test-LuciFieldPresent $result.containers[0] 'kind') -and -not (Test-LuciFieldPresent $result.containers[0] 'memorySwapLimitBytes')) 'Default output schema changed.'
}
Check 'Opted-in exact SDK check and worker attribution preserves missing native facts' {
    $data = New-LuciSDKFixture; $previous = @{}
    $result = Get-LuciDockerSample $data.scope 600 $previous
    $row = @($result.containers | Where-Object { $_.kind -ceq 'check' })[0]
    Assert-True ($script:fixture.dockerCalls -eq 4 -and $result.status -eq 'observed' -and $result.inventoryComplete -and $result.checkCandidates -eq 1 -and $result.checkIdentityRows -eq 1 -and $result.containers.Count -eq 2) 'Opted-in check did not use the bounded allowlisted reads.'
    Assert-True ($row.checkId -ceq $data.labels['luci.check-id'] -and $row.allocationId -ceq $data.labels['luci.check-allocation'] -and $row.commandIndex -eq 0 -and $row.protocol -eq 1) 'SDK check attribution is incomplete.'
    Assert-True ($row.memoryUsageBytes -eq 500 -and $row.workingSetBytes -eq 400 -and $row.memoryCgroupLimitBytes -eq 1000 -and $null -eq $row.cpuCoreEquivalent) 'Check metrics or initial CPU baseline are incorrect.'
    Assert-True ($null -eq $row.memoryLimitBytes -and $null -eq $row.memorySwapLimitBytes -and $row.configuredMemoryLimitsStatus -ceq 'unavailable-no-inspect-endpoint' -and $null -eq $row.guestPID -and $null -eq $row.guestPIDStartTicks -and $null -eq $row.cgroupIdentitySHA256 -and $row.guestIdentityStatus -ceq 'unavailable-no-inspect-endpoint') 'Unsupported native/container facts were invented.'
    Assert-True (($result | ConvertTo-Json -Depth 18) -notmatch 'SECRET_MARKER|PRIVATE_COMMAND_MARKER|fixture-engine|luci.check-source-digest') 'Private or unallowlisted check fields escaped serialization.'
    $data.stats.read = ([DateTimeOffset]::Parse($data.stats.read)).AddMilliseconds(500).ToString('o'); $data.stats.cpu_stats.cpu_usage.total_usage = 2000000000
    $next = Get-LuciDockerSample $data.scope 600 $previous
    $nextRow = @($next.containers | Where-Object { $_.kind -ceq 'check' })[0]
    Assert-True ($nextRow.cpuCoreEquivalent -eq 2) 'Pinned immutable-ID CPU delta was lost.'
}
Check 'Foreign SDK checks are excluded and exact-instance image mismatch is explicit' {
    $data = New-LuciSDKFixture; $data.labels['luci.instance'] = 'foreign'
    $result = Get-LuciDockerSample $data.scope 600 @{}
    Assert-True ($script:fixture.dockerCalls -eq 3 -and $result.status -eq 'observed' -and $result.containers.Count -eq 1 -and $result.checkCandidates -eq 0) 'Foreign instance was sampled or attributed.'
    $data = New-LuciSDKFixture; $data.check.ImageID = 'sha256:' + ('f' * 64)
    $result = Get-LuciDockerSample $data.scope 600 @{}
    Assert-True ($script:fixture.dockerCalls -eq 3 -and $result.status -eq 'partial' -and -not $result.inventoryComplete -and $result.containers.Count -eq 1 -and $result.issues -contains 'check-instance-or-image-mismatch') 'Own mismatched-image check became zero usage or was read.'
}
Check 'Every required SDK label and current engine identity must validate before stats' {
    $keys = @('luci.check', 'luci.check-protocol', 'luci.check-engine', 'luci.check-allocation', 'luci.check-id', 'luci.check-command-index', 'luci.check-scope', 'luci.check-project', 'luci.check-command-digest', 'luci.check-source-digest', 'luci.check-image')
    foreach ($key in $keys) {
        $data = New-LuciSDKFixture; $data.labels.Remove($key)
        $result = Get-LuciDockerSample $data.scope 600 @{}
        Assert-True ($script:fixture.dockerCalls -eq 3 -and $result.status -eq 'partial' -and -not $result.inventoryComplete -and $result.checkCandidates -eq 1 -and $result.checkIdentityRows -eq 0 -and $result.issues -contains 'check-required-label-unavailable') ('Missing SDK label was attributed: ' + $key)
    }
    foreach ($case in @('protocol', 'engine', 'missing-engine', 'allocation', 'check-id', 'index', 'project', 'scope', 'command', 'source', 'label-image', 'name', 'worker-collision', 'null-worker-collision', 'null-marker', 'case-label')) {
        $data = New-LuciSDKFixture
        switch ($case) {
            'protocol' { $data.labels['luci.check-protocol'] = '2' }
            'engine' { $data.info.ID = 'another-engine' }
            'missing-engine' { $data.info.Remove('ID') }
            'allocation' { $data.labels['luci.check-allocation'] = 'checkexec-wrong' }
            'check-id' { $data.labels['luci.check-id'] = 'check-wrong' }
            'index' { $data.labels['luci.check-command-index'] = '8' }
            'project' { $data.labels['luci.check-project'] = 'PRIVATE/PROJECT' }
            'scope' { $data.labels['luci.check-scope'] = 'bad' }
            'command' { $data.labels['luci.check-command-digest'] = 'bad' }
            'source' { $data.labels['luci.check-source-digest'] = 'A' * 64 }
            'label-image' { $data.labels['luci.check-image'] = 'sha256:' + ('f' * 64) }
            'name' { $data.check.Names = @('/luci-check-another-allocation') }
            'worker-collision' { $data.labels['luci.managed'] = 'true' }
            'null-worker-collision' { $data.labels['luci.managed'] = $null }
            'null-marker' { $data.labels['luci.check'] = $null }
            'case-label' { $value = $data.labels['luci.check-protocol']; $data.labels.Remove('luci.check-protocol'); $data.labels['LUCI.CHECK-PROTOCOL'] = $value }
        }
        $result = Get-LuciDockerSample $data.scope 600 @{}
        Assert-True ($script:fixture.dockerCalls -eq 3 -and $result.status -eq 'partial' -and -not $result.inventoryComplete -and $result.checkCandidates -eq 1 -and $result.checkIdentityRows -eq 0 -and $result.issues.Count -gt 0) ('Malformed own SDK check was attributed: ' + $case)
    }
}
Check 'Stale or wrong-ID SDK statistics never become check usage' {
    $data = New-LuciSDKFixture; $data.stats.read = [DateTimeOffset]::UtcNow.AddMinutes(-1).ToString('o')
    $result = Get-LuciDockerSample $data.scope 600 @{}
    $row = @($result.containers | Where-Object { $_.kind -ceq 'check' })[0]
    Assert-True ($result.status -eq 'partial' -and $null -eq $row.observedAt -and $null -eq $row.memoryUsageBytes -and $null -eq $row.memoryCgroupLimitBytes -and $result.issues -contains 'container-statistics-not-fresh') 'Stale check statistics became current or zero usage.'
    foreach ($id in @($null, ('f' * 64))) {
        $data = New-LuciSDKFixture; $data.stats.id = $id
        $result = Get-LuciDockerSample $data.scope 600 @{}
        Assert-True ($result.status -eq 'partial' -and -not $result.inventoryComplete -and $result.containers.Count -eq 1 -and $result.containers[0].kind -ceq 'worker' -and $result.issues -contains 'check-statistics-identity-mismatch') 'Unidentified SDK statistics became attributed usage.'
    }
}
Check 'Missing or invalid SDK usage and CPU counters remain null and explicitly partial' {
    foreach ($field in @('usage', 'cpu')) {
        foreach ($value in @($null, -1, '500', $true, 0.5, [double]1e-40, [double]::NaN, [double]::PositiveInfinity, [double]1e40)) {
            $data = New-LuciSDKFixture
            if ($field -eq 'usage') { $data.stats.memory_stats.usage = $value } else { $data.stats.cpu_stats.cpu_usage.total_usage = $value }
            $previous = @{}; $result = Get-LuciDockerSample $data.scope 600 $previous
            $row = @($result.containers | Where-Object { $_.kind -ceq 'check' })[0]
            Assert-True ($result.status -eq 'partial' -and $result.issues -contains 'check-statistics-measurement-unavailable') ('Invalid check ' + $field + ' did not produce explicit missing measurement.')
            if ($field -eq 'usage') { Assert-True ($null -eq $row.memoryUsageBytes -and $null -eq $row.workingSetBytes) 'Invalid memory usage became an attributed value.' }
            else { Assert-True ($null -eq $row.cpuCoreEquivalent -and -not $previous.ContainsKey($data.id)) 'Invalid CPU usage became a counter baseline or delta.' }
        }
    }
    $data = New-LuciSDKFixture; $data.stats.memory_stats.usage = 0; $data.stats.memory_stats.stats.inactive_file = 0; $data.stats.cpu_stats.cpu_usage.total_usage = 0
    $result = Get-LuciDockerSample $data.scope 600 @{}
    $row = @($result.containers | Where-Object { $_.kind -ceq 'check' })[0]
    Assert-True ($result.status -eq 'observed' -and $row.memoryUsageBytes -eq 0 -and $row.workingSetBytes -eq 0) 'An actual measured zero was rejected.'
    $data = New-LuciSDKFixture; $data.stats.memory_stats.stats.inactive_file = -1
    $result = Get-LuciDockerSample $data.scope 600 @{}
    $row = @($result.containers | Where-Object { $_.kind -ceq 'check' })[0]
    Assert-True ($result.status -eq 'partial' -and $null -eq $row.workingSetBytes -and $result.issues -contains 'check-statistics-measurement-unavailable') 'Invalid inactive-file counter invented a working set.'
}
Check 'Opted-in checks retain the shared deadline and eight-candidate bound' {
    $data = New-LuciSDKFixture; $script:fixture.delay = 120
    $result = Get-LuciDockerSample $data.scope 100 @{}
    Assert-True ($script:fixture.dockerCalls -eq 1 -and $result.status -eq 'partial' -and -not $result.inventoryComplete) 'Check observation escaped the shared deadline.'
    $data = New-LuciSDKFixture; $script:fixture.responses['/containers/json?all=1'] = @($data.check) * 9
    $result = Get-LuciDockerSample $data.scope 600 @{}
    Assert-True ($script:fixture.dockerCalls -eq 2 -and -not $result.inventoryComplete -and $result.issues -contains 'owned-container-count-exceeds-eight') 'Check observation escaped the candidate bound.'
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
