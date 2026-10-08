#requires -Version 5.1
[CmdletBinding(DefaultParameterSetName = 'Manifest')]
param(
    [Parameter(Mandatory = $true, ParameterSetName = 'Manifest')][string]$ManifestPath,
    [Parameter(Mandatory = $true, ParameterSetName = 'Roots')][int[]]$RootProcessId,
    [Parameter(Mandatory = $true, ParameterSetName = 'Roots')][string[]]$RootCreationFileTimeUtc,
    [Parameter(Mandatory = $true, ParameterSetName = 'Roots')][string[]]$RootRole,
    [Parameter(Mandatory = $true, ParameterSetName = 'Roots')][string[]]$DiskPath,
    [Parameter(ParameterSetName = 'Roots')][string]$DockerInstance = '',
    [Parameter(ParameterSetName = 'Roots')][string]$DockerImageId = '',
    [Parameter(ParameterSetName = 'Roots')][string]$DockerPipeName = 'docker_engine',
    [Parameter(Mandatory = $true)][string]$OutputDirectory,
    [ValidateRange(1, 3600)][int]$DurationSeconds = 1800,
    [ValidateRange(1, 60)][int]$IntervalSeconds = 1,
    [ValidateRange(100, 2000)][int]$DockerBudgetMilliseconds = 600
)

function Get-LuciField($Value, [string]$Name) {
    if ($null -eq $Value) { return $null }
    if ($Value -is [System.Collections.IDictionary]) { return $Value[$Name] }
    $property = $Value.PSObject.Properties[$Name]
    if ($null -ne $property) { return $property.Value }
    return $null
}

function Assert-LuciLocalPath([string]$Path) {
    if ($Path -notmatch '^[A-Za-z]:[\\/]' -or $Path.Contains([char]0)) { throw 'Use an absolute path on a local Windows drive.' }
    return [IO.Path]::GetFullPath($Path)
}

function ConvertTo-LuciResourceScope($Value) {
    if ((Get-LuciField $Value 'version') -ne 1) { throw 'Resource manifest version must be 1.' }
    $roots = @(Get-LuciField $Value 'roots')
    $disks = @(Get-LuciField $Value 'disks')
    if ($roots.Count -lt 1 -or $roots.Count -gt 8 -or $disks.Count -lt 1 -or $disks.Count -gt 4) { throw 'Specify 1-8 exact process roots and 1-4 local disk paths.' }
    $known = @{}; $normalizedRoots = @(); $normalizedDisks = @()
    foreach ($root in $roots) {
        [int]$processNumber = 0; [UInt64]$creation = 0
        $role = [string](Get-LuciField $root 'role')
        if (-not [int]::TryParse([string](Get-LuciField $root 'processId'), [ref]$processNumber) -or $processNumber -le 0 -or
            -not [UInt64]::TryParse([string](Get-LuciField $root 'creationFileTimeUtc'), [ref]$creation) -or $creation -eq 0 -or
            $role -notin @('control-plane', 'browser', 'desktop', 'test-driver', 'other-owned')) { throw 'Each root needs a positive PID, exact creation FILETIME and a supported role.' }
        if ($known.ContainsKey($processNumber)) { throw 'A root process can be specified only once.' }
        $known[$processNumber] = $true
        $normalizedRoots += [pscustomobject]@{processId = $processNumber; creationFileTimeUtc = $creation.ToString([Globalization.CultureInfo]::InvariantCulture); role = $role}
    }
    $roles = @{}
    foreach ($disk in $disks) {
        $role = [string](Get-LuciField $disk 'role')
        if ($role -notmatch '^[a-z][a-z0-9-]{0,31}$' -or $roles.ContainsKey($role)) { throw 'Disk roles must be unique short names.' }
        $roles[$role] = $true
        $normalizedDisks += [pscustomobject]@{role = $role; path = (Assert-LuciLocalPath ([string](Get-LuciField $disk 'path')))}
    }
    $docker = Get-LuciField $Value 'docker'; $normalizedDocker = $null
    if ($null -ne $docker) {
        $instance = [string](Get-LuciField $docker 'instance'); $image = [string](Get-LuciField $docker 'imageId'); $pipe = [string](Get-LuciField $docker 'pipeName')
        if ($instance -notmatch '^[A-Za-z0-9_-]{1,128}$' -or $image -notmatch '^sha256:[0-9a-f]{64}$' -or $pipe -notmatch '^[A-Za-z0-9_.-]{1,80}$') { throw 'Docker observation needs one exact instance, image digest and local pipe name.' }
        $normalizedDocker = [pscustomobject]@{instance = $instance; imageId = $image; pipeName = $pipe}
    }
    return [pscustomobject]@{roots = $normalizedRoots; disks = $normalizedDisks; docker = $normalizedDocker}
}

function Read-LuciProcessNative([int]$ProcessNumber) { return [LuciWindowsResourceNative]::ReadProcess([uint32]$ProcessNumber) }
function Read-LuciTopologyNative { return [LuciWindowsResourceNative]::ReadTopology() }
function Read-LuciHostNative { return [LuciWindowsResourceNative]::ReadHost() }
function Read-LuciDiskNative([string]$Path) { return [LuciWindowsResourceNative]::ReadDisk($Path) }
function Invoke-LuciDockerGet([string]$PipeName, [string]$Path, [int]$TimeoutMilliseconds) {
    return ([LuciWindowsResourceNative]::DockerGet($PipeName, $Path, $TimeoutMilliseconds) | ConvertFrom-Json)
}

function Get-LuciOwnedProcesses([hashtable]$Tracked, [hashtable]$Previous, [double]$ElapsedSeconds) {
    $topology = Read-LuciTopologyNative
    $present = @{}; $unavailable = 0; $reused = 0; $limited = $false
    $topologyIds = @{}; foreach ($entry in $topology.Rows) { $topologyIds[[int]$entry.ProcessID] = $true }
    foreach ($key in @($Tracked.Keys)) {
        $current = Read-LuciProcessNative ([int]$key)
        if ($null -eq $current) { if (-not $topology.Complete -or $topologyIds.ContainsKey([int]$key)) { $unavailable++ }; continue }
        if ([string]$current.CreationFileTime -ne $Tracked[$key].creationFileTimeUtc) { $reused++; continue }
        if ($current.Alive) { $present[[int]$key] = $current }
    }
    $changed = $true
    while ($changed) {
        $changed = $false
        foreach ($candidate in $topology.Rows) {
            $child = [int]$candidate.ProcessID; $parent = [int]$candidate.ParentID
            if ($child -le 0 -or $present.ContainsKey($child) -or -not $present.ContainsKey($parent)) { continue }
            if ($present.Count -ge 1024) { $limited = $true; break }
            $current = Read-LuciProcessNative $child
            if ($null -eq $current) { $unavailable++; continue }
            # Windows retains historical parent PIDs. Creation ordering rejects a
            # child of an earlier, exited process whose PID was later reused.
            if (-not $current.Alive -or $current.CreationFileTime -lt $present[$parent].CreationFileTime) { continue }
            $Tracked[$child] = [pscustomobject]@{creationFileTimeUtc = [string]$current.CreationFileTime; role = $Tracked[$parent].role}
            $present[$child] = $current; $changed = $true
        }
        if ($limited) { break }
    }
    $rows = @(); $rss = @{}; $privateBytes = @{}
    foreach ($key in @($present.Keys | Sort-Object)) {
        $current = $present[$key]; $role = $Tracked[$key].role
        $identity = [string]$key + ':' + [string]$current.CreationFileTime
        $cpu = $null; $ioDelta = $null
        if ($Previous.ContainsKey($identity)) {
            $old = $Previous[$identity]; $span = $ElapsedSeconds - $old.elapsed
            if ($span -gt 0 -and $current.CPU100ns -ge $old.cpu) { $cpu = ([double]$current.CPU100ns - [double]$old.cpu) / 10000000 / $span }
            if ($null -ne $current.ReadBytes -and $null -ne $current.WriteBytes -and $null -ne $old.read -and $null -ne $old.write -and $current.ReadBytes -ge $old.read -and $current.WriteBytes -ge $old.write) {
                $ioDelta = [ordered]@{readBytes = $current.ReadBytes - $old.read; writeBytes = $current.WriteBytes - $old.write}
            }
        }
        $Previous[$identity] = @{elapsed = $ElapsedSeconds; cpu = $current.CPU100ns; read = $current.ReadBytes; write = $current.WriteBytes}
        $rows += [ordered]@{processId = [int]$key; creationFileTimeUtc = [string]$current.CreationFileTime; role = $role; workingSetBytes = $current.WorkingSetBytes; privateCommitBytes = $current.PrivateCommitBytes; cpuSeconds = ([double]$current.CPU100ns / 10000000); cpuCoreEquivalent = $cpu; ioReadBytes = $current.ReadBytes; ioWriteBytes = $current.WriteBytes; ioOtherBytes = $current.OtherBytes; ioDelta = $ioDelta}
        if ($null -ne $current.WorkingSetBytes) { $rss[$role] = [double]$rss[$role] + [double]$current.WorkingSetBytes }
        if ($null -ne $current.PrivateCommitBytes) { $privateBytes[$role] = [double]$privateBytes[$role] + [double]$current.PrivateCommitBytes }
    }
    $workingSetComplete = $topology.Complete -and -not $limited -and $unavailable -eq 0 -and @($rows | Where-Object { $null -eq $_.workingSetBytes }).Count -eq 0
    return [ordered]@{rows = $rows; workingSetComplete = $workingSetComplete; summedWorkingSetBytesByRole = $rss; summedPrivateCommitBytesByRole = $privateBytes; topologyComplete = [bool]$topology.Complete; processLimitReached = $limited; unavailableProcessReads = $unavailable; rejectedReusedPIDs = $reused}
}

function Get-LuciHostSample($Scope, [hashtable]$Previous) {
    $native = Read-LuciHostNative; $cpu = $null
    if ($null -ne $native.CPUTotalTicks -and $null -ne $native.CPUIdleTicks) {
        if ($Previous.ContainsKey('total') -and $native.CPUTotalTicks -gt $Previous.total -and $native.CPUIdleTicks -ge $Previous.idle) {
            $total = [double]$native.CPUTotalTicks - [double]$Previous.total; $idle = [double]$native.CPUIdleTicks - [double]$Previous.idle
            if ($idle -le $total) { $cpu = 100 * ($total - $idle) / $total }
        }
        $Previous.total = $native.CPUTotalTicks; $Previous.idle = $native.CPUIdleTicks
    }
    $disks = @()
    foreach ($disk in $Scope.disks) {
        $value = Read-LuciDiskNative $disk.path
        $disks += [ordered]@{role = $disk.role; availableBytes = (Get-LuciField $value 'AvailableBytes'); totalBytes = (Get-LuciField $value 'TotalBytes'); status = $(if ($null -eq $value) { 'unavailable' } else { 'observed' })}
    }
    return [ordered]@{physicalTotalBytes = $native.PhysicalTotalBytes; physicalAvailableBytes = $native.PhysicalAvailableBytes; commitLimitBytes = $native.CommitLimitBytes; commitAvailableBytes = $native.CommitAvailableBytes; logicalProcessors = $native.LogicalProcessors; cpuUtilizationPercent = $cpu; cpuScope = 'GetSystemTimes processor group; not all groups on hosts above 64 logical CPUs'; disks = $disks; hardPagingRate = $null; hardPagingStatus = 'not collected; process faults are not a hard-paging rate'}
}

function Get-LuciDockerSample($Scope, [int]$BudgetMilliseconds, [hashtable]$Previous) {
    $result = [ordered]@{scope = 'separate-docker-daemon'; status = 'unavailable'; enginePlatform = $null; totalMemoryBytes = $null; cpuCount = $null; inventoryComplete = $false; containers = @(); issues = @()}
    if ($null -eq $Scope) { $result.issues = @('exact-docker-scope-not-configured'); return $result }
    $watch = [Diagnostics.Stopwatch]::StartNew()
    $request = {
        param([string]$Path)
        $remaining = $BudgetMilliseconds - [int]$watch.ElapsedMilliseconds
        if ($remaining -lt 1) { throw 'Docker observation budget exhausted.' }
        Invoke-LuciDockerGet $Scope.pipeName $Path $remaining
    }
    try {
        $info = & $request '/info'; $result.enginePlatform = Get-LuciField $info 'OSType'; $result.totalMemoryBytes = Get-LuciField $info 'MemTotal'; $result.cpuCount = Get-LuciField $info 'NCPU'
        if ($result.enginePlatform -ne 'linux') { $result.issues = @('linux-container-engine-required'); return $result }
        $inventory = @(& $request '/containers/json?all=1')
        $candidates = @($inventory | Where-Object { (Get-LuciField (Get-LuciField $_ 'Labels') 'luci.instance') -ceq $Scope.instance -and (Get-LuciField (Get-LuciField $_ 'Labels') 'luci.managed') -ceq 'true' })
        if ($candidates.Count -gt 8) { $result.issues = @('owned-container-count-exceeds-eight'); return $result }
        $result.inventoryComplete = $true
        foreach ($candidate in $candidates) {
            $id = [string](Get-LuciField $candidate 'Id')
            if ($id -cnotmatch '^[0-9a-f]{64}$') { throw 'Invalid container identity.' }
            # List supplies immutable image/labels. Do not request full inspect,
            # which includes container environment and mounts. Guest PIDs and
            # configured CPU limits remain unavailable through this narrow API.
            if ((Get-LuciField $candidate 'ImageID') -cne $Scope.imageId) { $result.issues += 'container-identity-changed-or-image-mismatch'; continue }
            $row = [ordered]@{id = $id; guestPID = $null; running = ((Get-LuciField $candidate 'State') -ceq 'running'); memoryCgroupLimitBytes = $null; configuredCPULimit = $null; observedAt = $null; memoryUsageBytes = $null; workingSetBytes = $null; cpuCoreEquivalent = $null; blockReadBytes = $null; blockWriteBytes = $null; unavailable = @('guest-pid-and-configured-cpu-limit-not-read') }
            if ($row.running) {
                $stats = & $request ('/containers/' + $id + '/stats?stream=false&one-shot=true')
                $stamp = [DateTimeOffset]::MinValue
                if (-not [DateTimeOffset]::TryParse([string](Get-LuciField $stats 'read'), [ref]$stamp) -or ([DateTimeOffset]::UtcNow - $stamp).TotalSeconds -gt 5 -or ($stamp - [DateTimeOffset]::UtcNow).TotalSeconds -gt 1) { $result.issues += 'container-statistics-not-fresh' }
                else {
                    $statsID = Get-LuciField $stats 'id'
                    if ($null -ne $statsID -and $statsID -cne $id) { throw 'Statistics identity mismatch.' }
                    $row.observedAt = $stamp.ToString('o'); $memory = Get-LuciField $stats 'memory_stats'; $row.memoryCgroupLimitBytes = Get-LuciField $memory 'limit'; $row.memoryUsageBytes = Get-LuciField $memory 'usage'; $memoryStats = Get-LuciField $memory 'stats'
                    $inactive = Get-LuciField $memoryStats 'inactive_file'; if ($null -eq $inactive) { $inactive = Get-LuciField $memoryStats 'total_inactive_file' }
                    if ($null -ne $inactive -and $null -ne $row.memoryUsageBytes -and $inactive -le $row.memoryUsageBytes) { $row.workingSetBytes = $row.memoryUsageBytes - $inactive }
                    $cpu = Get-LuciField (Get-LuciField (Get-LuciField $stats 'cpu_stats') 'cpu_usage') 'total_usage'
                    if ($null -ne $cpu) {
                        if ($Previous.ContainsKey($id)) { $old = $Previous[$id]; $span = ($stamp - $old.stamp).TotalSeconds; if ($span -gt 0 -and $cpu -ge $old.cpu) { $row.cpuCoreEquivalent = ([double]$cpu - [double]$old.cpu) / 1000000000 / $span } }
                        $Previous[$id] = @{cpu = $cpu; stamp = $stamp}
                    }
                    $io = Get-LuciField (Get-LuciField $stats 'blkio_stats') 'io_service_bytes_recursive'
                    if ($null -ne $io) {
                        $row.blockReadBytes = 0; $row.blockWriteBytes = 0
                        foreach ($entry in $io) { if ((Get-LuciField $entry 'op') -eq 'Read') { $row.blockReadBytes += Get-LuciField $entry 'value' }; if ((Get-LuciField $entry 'op') -eq 'Write') { $row.blockWriteBytes += Get-LuciField $entry 'value' } }
                    }
                }
            }
            $result.containers += $row
        }
        $result.status = $(if ($result.issues.Count -gt 0) { 'partial' } else { 'observed' })
    } catch { $result.status = 'partial'; $result.inventoryComplete = $false; $result.issues += 'docker-observation-unavailable-or-budget-exceeded' }
    return $result
}

function Write-LuciNewJSON([string]$Path, $Value) {
    $file = [IO.File]::Open($Path, [IO.FileMode]::CreateNew, [IO.FileAccess]::Write, [IO.FileShare]::Read)
    try { $writer = New-Object IO.StreamWriter($file, (New-Object Text.UTF8Encoding($false))); try { $writer.WriteLine(($Value | ConvertTo-Json -Depth 18)); $writer.Flush() } finally { $writer.Dispose() } } finally { $file.Dispose() }
}

function Invoke-LuciWindowsSampler($Scope, [string]$Directory, [int]$Seconds, [int]$Interval, [int]$DockerBudget) {
    $tracked = @{}; $previous = @{}; $previousHost = @{}; $previousDocker = @{}
    foreach ($root in $Scope.roots) {
        $current = Read-LuciProcessNative $root.processId
        if ($null -eq $current -or -not $current.Alive -or [string]$current.CreationFileTime -cne $root.creationFileTimeUtc) { throw 'A root process is unavailable or its creation identity changed; nothing was sampled.' }
        $tracked[[int]$root.processId] = [pscustomobject]@{creationFileTimeUtc = $root.creationFileTimeUtc; role = $root.role}
    }
    $directory = Assert-LuciLocalPath $Directory
    $parent = Split-Path -Parent $directory
    if (-not (Test-Path -LiteralPath $parent -PathType Container)) { throw 'The chosen private output parent must already exist.' }
    for ($part = Get-Item -LiteralPath $parent; $null -ne $part; $part = $part.Parent) {
        if (($part.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) { throw 'Choose an output parent without directory links or reparse points.' }
    }
    [LuciWindowsResourceNative]::CreateFreshDirectory($directory)
    $sid = [Security.Principal.WindowsIdentity]::GetCurrent().User
    $acl = New-Object Security.AccessControl.DirectorySecurity
    $acl.SetOwner($sid); $acl.SetAccessRuleProtection($true, $false)
    $acl.AddAccessRule((New-Object Security.AccessControl.FileSystemAccessRule($sid, 'FullControl', 'ContainerInherit,ObjectInherit', 'None', 'Allow')))
    Set-Acl -LiteralPath $directory -AclObject $acl
    $limitations = @('Process working-set sums double-count shared pages; role peaks are not additive.', 'Docker cgroup usage is separate from host/VM/process memory; do not sum VM and guest measurements.', 'Only exact supplied roots and observed descendants are attributed; unrelated browser/editor/WSL/daemon/kernel use stays unassigned.', 'Transient processes and peaks can be missed. Missing values are not zero. Actual sample intervals are recorded.', 'No command lines, environment values, full Docker inspections, credentials or volume paths are written.', 'This records resource observations, not UI latency, model quality, realistic-build success or whole-computer responsiveness.', 'Native Windows execution was unverified when this helper was published; a new run supplies its own evidence.')
    $metadata = [ordered]@{version = 1; startedAt = [DateTime]::UtcNow.ToString('o'); platform = 'windows'; roots = $Scope.roots; requestedSeconds = $Seconds; requestedIntervalSeconds = $Interval; dockerObservationBudgetMilliseconds = $DockerBudget; diskRoles = @($Scope.disks | ForEach-Object { $_.role }); dockerConfigured = ($null -ne $Scope.docker); helperSHA256 = (Get-FileHash -LiteralPath $PSCommandPath -Algorithm SHA256).Hash.ToLowerInvariant(); nativeProbeSHA256 = (Get-FileHash -LiteralPath (Join-Path $PSScriptRoot 'windows-resource-native.cs') -Algorithm SHA256).Hash.ToLowerInvariant(); baseline = 'Operator must identify idle/load/settle phases; elapsed time alone does not identify them.'; limitations = $limitations}
    Write-LuciNewJSON (Join-Path $directory 'metadata.json') $metadata
    $stream = [IO.File]::Open((Join-Path $directory 'samples.jsonl'), [IO.FileMode]::CreateNew, [IO.FileAccess]::Write, [IO.FileShare]::Read)
    $writer = New-Object IO.StreamWriter($stream, (New-Object Text.UTF8Encoding($false)))
    $watch = [Diagnostics.Stopwatch]::StartNew(); $sampleCount = 0; $peakWorkingSet = $null; $complete = $false; $last = $null
    try {
        while ($watch.Elapsed.TotalSeconds -lt $Seconds) {
            $tick = $watch.Elapsed.TotalSeconds
            $processes = Get-LuciOwnedProcesses $tracked $previous $tick
            $hostSample = Get-LuciHostSample $Scope $previousHost
            $dockerSample = Get-LuciDockerSample $Scope.docker $DockerBudget $previousDocker
            $sum = $null
            if ($processes.workingSetComplete) {
                $sum = 0; foreach ($value in $processes.summedWorkingSetBytesByRole.Values) { $sum += $value }
                if ($null -eq $peakWorkingSet -or $sum -gt $peakWorkingSet) { $peakWorkingSet = $sum }
            }
            $last = [ordered]@{observedAt = [DateTime]::UtcNow.ToString('o'); elapsedSeconds = $watch.Elapsed.TotalSeconds; sampleDurationSeconds = ($watch.Elapsed.TotalSeconds - $tick); host = $hostSample; ownedProcesses = $processes; summedWorkingSetBytes = $sum; docker = $dockerSample}
            $writer.WriteLine(($last | ConvertTo-Json -Depth 18 -Compress)); $writer.Flush(); $sampleCount++
            $wait = [Math]::Min($Interval - ($watch.Elapsed.TotalSeconds - $tick), $Seconds - $watch.Elapsed.TotalSeconds)
            if ($wait -gt 0) { Start-Sleep -Milliseconds ([int]($wait * 1000)) }
        }
        $complete = $true
    } finally {
        $writer.Dispose(); $stream.Dispose()
        Write-LuciNewJSON (Join-Path $directory 'summary.json') ([ordered]@{finishedAt = [DateTime]::UtcNow.ToString('o'); completedRequestedWindow = $complete; samples = $sampleCount; observedSeconds = $watch.Elapsed.TotalSeconds; peakSummedWorkingSetBytes = $peakWorkingSet; finalObservedProcessCount = $(if ($null -ne $last) { @($last.ownedProcesses.rows).Count } else { $null }); limitations = $limitations})
    }
    Write-Host ('Recorded ' + $sampleCount + ' read-only resource samples. Application and Docker state were not changed.')
}

$ErrorActionPreference = 'Stop'
if ($env:OS -ne 'Windows_NT') { throw 'Run this observer natively on Windows; no workload or Docker resources were changed.' }
if ($PSCmdlet.ParameterSetName -eq 'Manifest') {
    $file = Get-Item -LiteralPath $ManifestPath
    if ($file.PSIsContainer -or $file.Length -gt 65536 -or ($file.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) { throw 'Use a small ordinary private JSON manifest.' }
    try { $inputScope = [IO.File]::ReadAllText($file.FullName) | ConvertFrom-Json } catch { throw 'The resource manifest is not valid JSON.' }
} else {
    if ($RootProcessId.Count -ne $RootCreationFileTimeUtc.Count -or $RootProcessId.Count -ne $RootRole.Count) { throw 'Provide one exact creation identity and role for each root PID.' }
    $roots = @(); for ($i = 0; $i -lt $RootProcessId.Count; $i++) { $roots += @{processId = $RootProcessId[$i]; creationFileTimeUtc = $RootCreationFileTimeUtc[$i]; role = $RootRole[$i]} }
    $disks = @(); for ($i = 0; $i -lt $DiskPath.Count; $i++) { $disks += @{role = ('disk-' + ($i + 1)); path = $DiskPath[$i]} }
    $docker = $null
    if ($DockerInstance -or $DockerImageId) { $docker = @{instance = $DockerInstance; imageId = $DockerImageId; pipeName = $DockerPipeName} }
    $inputScope = @{version = 1; roots = $roots; disks = $disks; docker = $docker}
}
$scope = ConvertTo-LuciResourceScope $inputScope
if (-not ('LuciWindowsResourceNative' -as [type])) { Add-Type -Path (Join-Path $PSScriptRoot 'windows-resource-native.cs') }
Invoke-LuciWindowsSampler $scope $OutputDirectory $DurationSeconds $IntervalSeconds $DockerBudgetMilliseconds
