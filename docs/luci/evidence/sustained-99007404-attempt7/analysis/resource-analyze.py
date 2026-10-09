"""Read-only, bounded-stream resource audit; never launches Luci or reads fixtures.

Adapted from the frozen sustained6 analyzer. Unlike that analyzer, this audit
does not read the workflow result or retain all raw sample objects in memory.
"""
import collections
import datetime
import hashlib
import json
import math
from pathlib import Path

BASE = Path('<PRIVATE_RUN_EVIDENCE_DIRECTORY>')
OUT = Path('<PRIVATE_TOOLING_DIRECTORY>/publish-sustained7-resource')
PRIOR = Path('<PRIVATE_TOOLING_DIRECTORY>/publish-sustained6/capsule/analysis/resource-analyze.py')
MAX_LINE_BYTES = 1024 * 1024
MAX_ROWS = 10000


def binding(path):
    digest = hashlib.sha256()
    size = 0
    with path.open('rb') as stream:
        while chunk := stream.read(1024 * 1024):
            size += len(chunk)
            digest.update(chunk)
    return {'bytes': size, 'sha256': digest.hexdigest()}


def number(value):
    return type(value) in (int, float) and math.isfinite(value)


def stats(values):
    values = sorted(v for v in values if number(v))
    if not values:
        return {'count': 0, 'min': None, 'p50': None, 'p95': None, 'max': None}
    return {'count': len(values), 'min': values[0],
            'p50': values[math.ceil(len(values) * .5) - 1],
            'p95': values[math.ceil(len(values) * .95) - 1], 'max': values[-1]}


def timestamp(value):
    return datetime.datetime.fromisoformat(value.replace('Z', '+00:00'))


def main():
    sources = ['observer/metadata.json', 'observer/summary.json',
               'observer-observer-self-cost.json', 'observer/accepted-sampler.py']
    bindings = {name: binding(BASE / name) for name in sources}
    metadata = json.loads((BASE / sources[0]).read_text())
    published = json.loads((BASE / sources[1]).read_text())
    own = json.loads((BASE / sources[2]).read_text())
    hz = own['clockTicksPerSecond']
    if not number(hz) or hz <= 0:
        raise ValueError('Recorded clock tick frequency unavailable')
    counts = collections.Counter()
    statuses = collections.Counter()
    obs_statuses = collections.Counter()
    phases = collections.defaultdict(lambda: {'samples': 0, 'rss': [], 'physical': [], 'effective': []})
    distributions = collections.defaultdict(list)
    issues = collections.Counter()
    cgroup_issues = collections.Counter()
    process_previous = {}
    process_records = {}
    containers = {}
    disk = collections.defaultdict(list)
    pressure = collections.Counter()
    roles_peak = collections.Counter()
    io_total = collections.Counter()
    role_cpu_ticks = collections.Counter()
    row_io_total = collections.Counter()
    node_statuses = collections.Counter()
    cgroup_ids = set()
    discrepancy = []
    formula_mismatches = []
    formula_unverifiable = []
    gaps = []
    first = last = previous_sample = previous_host_cpu = None
    cpu_ticks_total = 0
    physical_validated = collections.Counter()
    sample_digest = hashlib.sha256()
    sample_bytes = 0
    max_line = 0
    sampler_path = BASE / 'observer/samples.jsonl'
    with sampler_path.open('rb') as stream:
        while True:
            raw = stream.readline(MAX_LINE_BYTES + 1)
            if not raw:
                break
            if len(raw) > MAX_LINE_BYTES:
                raise ValueError('Sample line exceeds audit bound')
            sample_digest.update(raw)
            sample_bytes += len(raw)
            max_line = max(max_line, len(raw))
            sample = json.loads(raw)
            index = counts['samples']
            counts['samples'] += 1
            if counts['samples'] > MAX_ROWS:
                raise ValueError('Sample count exceeds audit bound')
            at = sample['observedAt']
            if first is None:
                first = at
            last = at
            elapsed = sample['elapsedSeconds']
            if previous_sample:
                gap = elapsed - previous_sample['elapsed']
                wall_gap = (timestamp(at) - timestamp(previous_sample['at'])).total_seconds()
                distributions['sampleIntervalSeconds'].append(gap)
                distributions['wallTimestampIntervalSeconds'].append(wall_gap)
                if gap > metadata['intervalSeconds'] * 1.5 or gap <= 0:
                    gaps.append({'afterSampleIndex': index - 1, 'before': previous_sample['at'],
                                 'after': at, 'monotonicGapSeconds': gap, 'wallGapSeconds': wall_gap})
            previous_sample = {'at': at, 'elapsed': elapsed}
            for name, value in [('wholeSamplerDurationSeconds', sample.get('sampleDurationSeconds')),
                                ('summedOwnedProcessRSSBytes', sample.get('summedProcessRSSBytes'))]:
                distributions[name].append(value)
            cgroup = sample.get('controlPlaneCgroupMemory', {})
            statuses[cgroup.get('status', 'missing')] += 1
            distributions['cgroupObservationDurationSeconds'].append(cgroup.get('durationSeconds'))
            distributions['cgroupReadCount'].append(cgroup.get('readCount'))
            distributions['cgroupReadBytes'].append(cgroup.get('readBytes'))
            cgroup_issues.update(cgroup.get('issues', []))
            host = sample.get('host', {})
            memory = host.get('memory') or {}
            physical_validated[str(memory.get('physicalCountersValidated'))] += 1
            for key in ['MemTotal', 'MemAvailable', 'SwapTotal', 'SwapFree', 'CommitLimit', 'Committed_AS']:
                distributions['host' + key + 'Bytes'].append(memory.get(key))
            for name, row in (host.get('disks') or {}).items():
                disk[name].append(row.get('availableBytes'))
            for name in ['cpu', 'memory', 'io']:
                value = (host.get('pressure') or {}).get(name)
                pressure[name + (':observed' if value is not None else ':unavailable')] += 1
            host_cpu = host.get('cpuTicks')
            if previous_host_cpu and isinstance(host_cpu, list) and len(host_cpu) == 8:
                deltas = [a - b for a, b in zip(host_cpu, previous_host_cpu)]
                total = sum(deltas)
                if total > 0 and all(v >= 0 for v in deltas):
                    distributions['hostCPUUtilizationPercentBetweenSamples'].append(
                        (total - deltas[3] - deltas[4]) * 100 / total)
                else:
                    counts['unusableHostCPUDeltas'] += 1
            previous_host_cpu = host_cpu
            phase = phases[sample.get('phase', 'missing')]
            phase['samples'] += 1
            phase.setdefault('firstObservedAt', at)
            phase['lastObservedAt'] = at
            phase['rss'].append(sample.get('summedProcessRSSBytes'))
            phase['physical'].append(memory.get('MemAvailable'))
            for observation in cgroup.get('observations', []):
                counts['cgroupObservationRows'] += 1
                obs_statuses[observation.get('status', 'missing')] += 1
                identity = (observation.get('pid'), observation.get('startTicks'))
                cgroup_ids.add(identity)
                cgroup_issues.update(observation.get('issues', []))
                if observation.get('status') != 'observed':
                    continue
                counts['validCgroupObservationRows'] += 1
                effective = observation.get('productComparableEffectiveAvailableBytes')
                distributions['effectiveAvailableBytes'].append(effective)
                distributions['effectiveTotalBytes'].append(observation.get('productComparableEffectiveTotalBytes'))
                phase['effective'].append(effective)
                nodes = observation.get('nodes', [])
                for node in nodes:
                    node_statuses[str(node.get('limitStatus')) + '/' + str(node.get('currentStatus'))] += 1
                finite = [n for n in nodes if n.get('limitStatus') == 'finite']
                identity_stable = observation.get('identityAndTopologyStable') is True
                if not identity_stable:
                    discrepancy.append({'sampleIndex': index, 'issue': 'observed cgroup identity/topology not stable'})
                if (any(n.get('currentStatus') != 'observed' or type(n.get('limitBytes')) is not int
                        or type(n.get('currentBytes')) is not int for n in finite)
                        or type(observation.get('physicalTotalBytes')) is not int
                        or type(observation.get('physicalAvailableBytes')) is not int):
                    formula_unverifiable.append({'sampleIndex': index, 'pid': identity[0], 'startTicks': identity[1]})
                else:
                    total = min([observation['physicalTotalBytes']] + [n['limitBytes'] for n in finite])
                    available = min([observation['physicalAvailableBytes']] +
                                    [max(0, n['limitBytes'] - n['currentBytes']) for n in finite])
                    if total != observation.get('productComparableEffectiveTotalBytes') or available != effective:
                        formula_mismatches.append({'sampleIndex': index, 'pid': identity[0],
                                                   'expectedTotalBytes': total, 'expectedAvailableBytes': available})
            issues.update(sample.get('containerIssues', []))
            counts['unreadableOrRacedProcessScanRows'] += sample.get('processStatUnreadableOrRacedCount', 0)
            if sample.get('driverAlive') is False:
                counts['driverAbsentSamples'] += 1
            groups = collections.Counter()
            core_sum = 0.0
            core_values = 0
            for proc in sample.get('processes', []):
                key = (proc['pid'], proc['startTicks'])
                role = proc['role']
                groups[role] += proc['residentBytes']
                rec = process_records.setdefault(key, {'pid': key[0], 'startTicks': key[1], 'role': role,
                    'firstObservedAt': at, 'sampleRows': 0, 'peakRSSBytes': 0,
                    'cpuTicksDelta': 0, 'ioCounterDeltas': collections.Counter()})
                rec['sampleRows'] += 1
                rec['lastObservedAt'] = at
                rec['finalObservedState'] = proc['state']
                rec['peakRSSBytes'] = max(rec['peakRSSBytes'], proc['residentBytes'])
                if key in process_previous:
                    old = process_previous[key]
                    ticks = proc['cpuTicks'] - old['cpuTicks']
                    if ticks < 0:
                        discrepancy.append({'sampleIndex': index, 'issue': 'owned process CPU counter decreased'})
                    ticks = max(0, ticks)
                    cpu_ticks_total += ticks
                    role_cpu_ticks[role] += ticks
                    rec['cpuTicksDelta'] += ticks
                    if proc.get('io') and old.get('io'):
                        expected = {k: max(0, proc['io'][k] - old['io'][k]) for k in
                                    ['read_bytes', 'write_bytes', 'rchar', 'wchar']}
                        io_total.update(expected)
                        rec['ioCounterDeltas'].update(expected)
                        if expected != proc.get('ioDelta'):
                            discrepancy.append({'sampleIndex': index, 'issue': 'reported process IO delta mismatch', 'pid': key[0]})
                process_previous[key] = {'cpuTicks': proc['cpuTicks'], 'io': proc.get('io')}
                row_io_total.update(proc.get('ioDelta', {}))
                if number(proc.get('cpuCoreEquivalent')):
                    core_sum += proc['cpuCoreEquivalent']
                    core_values += 1
            if core_values:
                distributions['summedOwnedProcessCPUCoreEquivalent'].append(core_sum)
            for role, value in groups.items():
                roles_peak[role] = max(roles_peak[role], value)
            if dict(groups) != sample.get('summedProcessRSSBytesByRole') or sum(groups.values()) != sample.get('summedProcessRSSBytes'):
                discrepancy.append({'sampleIndex': index, 'issue': 'summed process RSS mismatch'})
            for container in sample.get('containers', []):
                cid = container['id']
                rec = containers.setdefault(cid, {'containerId': cid, 'sandboxIds': set(), 'explicitKinds': set(),
                    'identityRows': 0, 'usageRows': 0, 'runningRows': 0, 'stoppedRows': 0,
                    'pidStartIdentities': set(), 'firstObservedAt': at, 'lastObservedAt': at,
                    'memoryUsageBytes': [], 'workingSetBytes': [], 'cpuCoreEquivalent': [],
                    'cpuUsageNanoseconds': [], 'readBytes': [], 'writeBytes': [], 'memoryLimitBytes': set(),
                    'nanoCPULimit': set()})
                rec['identityRows'] += 1
                rec['lastObservedAt'] = at
                if container.get('sandboxId'):
                    rec['sandboxIds'].add(container['sandboxId'])
                if container.get('kind'):
                    rec['explicitKinds'].add(container['kind'])
                if container.get('pid'):
                    rec['pidStartIdentities'].add((container['pid'], container.get('pidStartTicks')))
                rec['runningRows' if container.get('running') else 'stoppedRows'] += 1
                if number(container.get('memoryUsageBytes')):
                    rec['usageRows'] += 1
                for field in ['memoryUsageBytes', 'workingSetBytes', 'cpuCoreEquivalent', 'cpuUsageNanoseconds']:
                    rec[field].append(container.get(field))
                for field in ['memoryLimitBytes', 'nanoCPULimit']:
                    if number(container.get(field)):
                        rec[field].add(container[field])
                for field in ['read', 'write']:
                    rec[field + 'Bytes'].append((container.get('blockIOBytes') or {}).get(field))
    bindings['observer/samples.jsonl'] = {'bytes': sample_bytes, 'sha256': sample_digest.hexdigest()}
    for name in sources:
        if binding(BASE / name) != bindings[name]:
            raise ValueError('Original changed during analysis: ' + name)
    if binding(sampler_path) != bindings['observer/samples.jsonl']:
        raise ValueError('Original sample stream changed during analysis')
    summary_comparisons = []

    def compare(name, expected, actual):
        same = math.isclose(expected, actual, rel_tol=1e-12, abs_tol=1e-8) if number(expected) and number(actual) else expected == actual
        summary_comparisons.append({'field': name, 'published': expected, 'recomputed': actual, 'matches': same})
        if not same:
            discrepancy.append({'issue': 'published summary mismatch', 'field': name})

    compare('samples', published['samples'], counts['samples'])
    compare('helperSHA256', metadata['helperSHA256'], bindings['observer/accepted-sampler.py']['sha256'])
    compare('peakObserved.summedRSS_allOwnedProcesses', published['peakObserved']['summedRSS_allOwnedProcesses'],
            stats(distributions['summedOwnedProcessRSSBytes'])['max'])
    for role, peak in roles_peak.items():
        compare('peakObserved.summedRSS_' + role, published['peakObserved'].get('summedRSS_' + role), peak)
    compare('observedCounterDeltas.observedProcessCPUSeconds', published['observedCounterDeltas']['observedProcessCPUSeconds'], cpu_ticks_total / hz)
    for name, value in io_total.items():
        compare('observedCounterDeltas.observedProcessIO_' + name, published['observedCounterDeltas'].get('observedProcessIO_' + name), value)
    compare('containerIds', published['containerIds'], sorted(containers))
    compare('processIdentities', sorted((p['pid'], p['startTicks'], p['role']) for p in published['processIdentities']),
            sorted((p['pid'], p['startTicks'], p['role']) for p in process_records.values()))
    container_rows = []
    for rec in containers.values():
        for key in ['sandboxIds', 'explicitKinds', 'pidStartIdentities', 'memoryLimitBytes', 'nanoCPULimit']:
            rec[key] = sorted(rec[key])
        for key in ['memoryUsageBytes', 'workingSetBytes', 'cpuCoreEquivalent', 'cpuUsageNanoseconds', 'readBytes', 'writeBytes']:
            rec[key] = stats(rec[key])
        rec['coverage'] = 'cgroup-usage-observed' if rec['usageRows'] else 'identity-only'
        rec['kindBasis'] = 'Default managed-worker sampler mode; check opt-in absent. No independent check allocation attribution.'
        container_rows.append(rec)
    compare('peakObserved.singleContainerCgroupUsageBytes', published['peakObserved']['singleContainerCgroupUsageBytes'],
            max((r['memoryUsageBytes']['max'] or 0) for r in container_rows))
    phase_rows = []
    for name, rec in phases.items():
        for key in ['rss', 'physical', 'effective']:
            rec[key] = stats(rec[key])
        phase_rows.append({'name': name, **rec})
    observer_cpu = own['userCPUSeconds'] + own['systemCPUSeconds']
    compare('observerSelfCost.cpuSecondsPerWallSecond', own['cpuSecondsPerWallSecond'], observer_cpu / own['wallSeconds'])
    outcome = {
        'schemaVersion': 1, 'source': 'Read-only retained sustained attempt 7 observer stream; no new workload or atomic admission measurement',
        'analysisProducedAt': datetime.datetime.now(datetime.timezone.utc).isoformat(),
        'streamingBounds': {'maxLineBytes': MAX_LINE_BYTES, 'maxRows': MAX_ROWS, 'largestObservedLineBytes': max_line,
                            'memoryPolicy': 'One raw sample at a time; bounded numeric lists and exact identity aggregates only'},
        'bindings': bindings, 'priorAnalyzerBinding': binding(PRIOR), 'analyzerBinding': binding(Path(__file__)),
        'sampleCount': counts['samples'], 'firstObservedAt': first, 'lastObservedAt': last,
        'wallSpanBetweenFirstAndLastSamplesSeconds': (timestamp(last) - timestamp(first)).total_seconds(),
        'publishedObserverDurationSeconds': published['durationSeconds'],
        'counts': dict(counts), 'controlPlaneCgroupSampleStatusCounts': dict(statuses),
        'controlPlaneCgroupObservationStatusCounts': dict(obs_statuses), 'controlPlaneIdentities': sorted(cgroup_ids),
        'cgroupNodeStatusCounts': dict(node_statuses), 'cgroupIssueCounts': dict(cgroup_issues),
        'formulaRecomputation': {'validObservationCount': counts['validCgroupObservationRows'],
            'mismatches': formula_mismatches, 'unverifiable': formula_unverifiable,
            'method': 'Independent minima of physical and finite visible ancestor limits; available uses max(0, limit-current); no cache/reservation discount'},
        'distributions': {k: stats(v) for k, v in sorted(distributions.items())},
        'samplingGapsOverOnePointFiveNominalIntervals': gaps,
        'observerPhaseSummaries': phase_rows,
        'physicalCounterValidationStatusCounts': dict(physical_validated),
        'diskAvailableBytesByObservedPath': {k: stats(v) for k, v in disk.items()},
        'hostPressureObservationCounts': dict(pressure),
        'ownedProcessCPUSecondsFromExactIdentityCounterDeltas': cpu_ticks_total / hz,
        'ownedProcessCPUSecondsByRole': {k: v / hz for k, v in role_cpu_ticks.items()},
        'ownedProcessIOBytesFromExactIdentityCounterDeltas': dict(io_total),
        'ownedProcessIOBytesFromRecordedDeltas': dict(row_io_total),
        'sampledRSSByRolePeaksBytes': dict(roles_peak), 'ownedProcessIdentityCoverage': list(process_records.values()),
        'containerSampleCoverage': sorted(container_rows, key=lambda r: r['firstObservedAt']),
        'containerIssueCounts': dict(issues),
        'checkObservation': {'enabled': metadata.get('includeChecks', False),
            'explicitCheckIdentityRows': sum(r['identityRows'] for r in container_rows if 'check' in r['explicitKinds']),
            'explicitCheckUsageRows': sum(r['usageRows'] for r in container_rows if 'check' in r['explicitKinds']),
            'status': 'Not opted in; actual check execution, identities and resource use are unestablished by this stream'},
        'observerSelfCost': own,
        'observerSelfCostRecomputation': {'cpuSeconds': observer_cpu, 'cpuSecondsPerWallSecond': observer_cpu / own['wallSeconds']},
        'publishedSummaryComparisons': summary_comparisons, 'discrepancies': discrepancy,
        'limitations': [
            'Samples are sequential approximately 1 Hz observations; short-lived processes or peaks between samples may be absent. Gaps are retained, not filled.',
            'Only post-start-running and post-driver-exit-settle observer phases are available here. They are not the workload idle/loaded/settled phases; no workflow result or fixture state was read.',
            'Physical host counters, visible shared cgroup effective limits and Luci-owned RSS measure different scopes. Cgroup/physical values include unrelated work and observation work.',
            'Owned process RSS includes the test driver and browser and double-counts shared pages. Container cgroup usage is separate; no RSS/container/cgroup sum represents unique attributable RAM.',
            'Container memory and CPU statistics are sampled coverage, not full lifecycle peaks/cost. Container cumulative CPU/I/O first/max are not equivalent to total run deltas or exclusive ownership accounting.',
            'Observed process CPU/I/O deltas exclude activity before first or after final identity observation; Linux rchar/wchar are logical I/O, read_bytes/write_bytes physical counters, not guaranteed physical media traffic.',
            'Host CPU between samples subtracts idle and iowait from eight aggregate tick counters. It includes unrelated work, differs from product admission timing and establishes no refusal cause.',
            'Unavailable host pressure counters are not zero pressure. Disk paths can share filesystems and their free values must not be summed.',
            'Observer RUSAGE_SELF excludes spawned helper/Docker CLI and daemon cost. No overhead subtraction, causal speedup, Windows/whole-computer responsiveness or full acceptance pass is claimed.',
            'Known final sampled process states and summary driver-exit observation are not independent cleanup proof; root supplies separate closure evidence.',
            *metadata.get('controlPlaneCgroupMemory', {}).get('limitations', []),
        ],
    }
    (OUT / 'resource-analysis.json').write_text(json.dumps(outcome, indent=2) + '\n')
    print(json.dumps({'samples': outcome['sampleCount'], 'cgroupStatuses': dict(statuses),
        'validCgroupObservations': counts['validCgroupObservationRows'], 'formulaMismatches': len(formula_mismatches),
        'discrepancies': discrepancy, 'physical': outcome['distributions']['hostMemAvailableBytes'],
        'effective': outcome['distributions']['effectiveAvailableBytes'],
        'ownedRSS': outcome['distributions']['summedOwnedProcessRSSBytes'],
        'observerCPU': observer_cpu / own['wallSeconds'],
        'containerCoverage': [{k: r[k] for k in ['containerId','sandboxIds','identityRows','usageRows','memoryUsageBytes']} for r in outcome['containerSampleCoverage']]}, indent=2))


if __name__ == '__main__':
    main()
