#!/usr/bin/env python3
"""Offline strict allowlist projection. No application execution or modified inputs."""
from pathlib import Path
import argparse, collections, datetime, gzip, hashlib, json, math

EXPECTED = 'ea7a1722ba0e284ed9e123893b792305b8494579c834b06f842f31e924da4b99'
PRIMARY = 'idle-interactive-calibration'
FOLLOWUP = 'idle-actionability-trace-followup'
OUT = Path(__file__).resolve().parent
parser = argparse.ArgumentParser()
parser.add_argument('--result', required=True)
args = parser.parse_args()
input_path = Path(args.result)
raw = input_path.read_bytes()
sha = lambda data: hashlib.sha256(data).hexdigest()
assert sha(raw) == EXPECTED
data = json.loads(raw)
assert data['sourceCommit'] == '577f7a0fdb039b7f875901a6a567d62fdfee88c8'
assert data['sourceDirty'] is False

def save(name, value):
    (OUT / name).write_text(json.dumps(value, indent=2) + '\n')

def exact_fields(row, allowed):
    assert not set(row) - set(allowed), sorted(set(row) - set(allowed))
    return {key: row[key] for key in allowed if key in row}

def numeric(value):
    assert isinstance(value, (int, float)) and not isinstance(value, bool) and math.isfinite(value)
    return value

def quantiles(values):
    if not values:
        return {'count': 0}
    values = sorted(values)
    return {'count': len(values), **{key: values[math.ceil(len(values) * p) - 1] for key, p in [('p50Ms', .5), ('p95Ms', .95), ('p99Ms', .99)]}, 'maxMs': values[-1]}

def ref(value, expected=False):
    if value is None:
        return None
    projected = exact_fields(value, ['sequence', 'at', 'actionKind'] if expected else ['sequence', 'at'])
    numeric(projected['sequence']); numeric(projected['at'])
    if expected:
        assert projected['actionKind'] in ['input', 'project-switch']
    return projected

stage_names = {'action-start', 'action-end', 'all-projects-click-end', 'overview-heading-ready', 'open-project-click-end', 'verify-start', 'verify-end', 'frames-start', 'frames-end'}
rows = []
for index, original in enumerate(data['interactions']):
    assert set(original) == {'phase', 'kind', 'projectID', 'at', 'success', 'diagnostics', 'elapsedMs'}
    assert original['phase'] in [PRIMARY, FOLLOWUP]
    assert original['kind'] in ['project-switch', 'input', 'draft-reset']
    assert original['success'] is True
    datetime.datetime.fromisoformat(original['at'].replace('Z', '+00:00'))
    diagnostic = original['diagnostics']
    assert set(diagnostic) == {'version', 'nodeStages', 'nodeClockReads', 'browserFrames', 'limitations'}
    stages = []
    for stage in diagnostic['nodeStages']:
        stage = exact_fields(stage, ['stage', 'offsetMs'])
        assert stage['stage'] in stage_names
        numeric(stage['offsetMs'])
        stages.append(stage)
    frame = exact_fields(diagnostic['browserFrames'], ['requestedAt', 'firstFrameAt', 'secondFrameAt', 'references'])
    for key in ['requestedAt', 'firstFrameAt', 'secondFrameAt']:
        numeric(frame[key])
    references = exact_fields(frame['references'], ['timeOrigin', 'input', 'pointer', 'expectedDOM'])
    numeric(references['timeOrigin'])
    frame['references'] = {'timeOrigin': references['timeOrigin'], **{key: ref(references[key], key == 'expectedDOM') for key in ['input', 'pointer', 'expectedDOM']}}
    rows.append({'index': index, 'phase': original['phase'], 'kind': original['kind'], 'at': original['at'], 'success': True, 'elapsedMs': numeric(original['elapsedMs']), 'diagnostics': {'version': numeric(diagnostic['version']), 'nodeStages': stages, 'nodeClockReads': numeric(diagnostic['nodeClockReads']), 'browserFrames': frame}})
assert len(rows) == 400
assert collections.Counter(row['phase'] for row in rows) == {PRIMARY: 400}

routes = {'project-orchestrator', 'project-overview', 'project-runs', 'project-specification', 'project-swarm-queue', 'swarm-scheduler'}
event_names = {'beforeinput', 'click', 'input', 'keydown', 'keypress', 'keyup', 'mousedown', 'mouseout', 'mouseover', 'mouseup', 'pointerdown', 'pointerenter', 'pointerleave', 'pointerout', 'pointerover', 'pointerup'}
latency_fields = {
    'input-dispatch': ['sequence', 'kind', 'at'],
    'pointer-dispatch': ['sequence', 'kind', 'at'],
    'input-frame-1': ['sequence', 'kind', 'at', 'inputSequence'],
    'input-frame-2': ['sequence', 'kind', 'at', 'inputSequence'],
    'expected-dom': ['sequence', 'kind', 'at', 'actionKind', 'input', 'pointer'],
    'resource': ['sequence', 'kind', 'route', 'at', 'duration', 'responseStatus', 'requestStart', 'responseStart', 'responseEnd', 'transferSize', 'encodedBodySize', 'decodedBodySize'],
}
browser_rows, events, windows = [], [], []
for window_index, window in enumerate(data['browserEventWindows']):
    origin = numeric(window['timeOrigin'])
    diagnostic = window['latency']
    assert origin == diagnostic['timeOrigin']
    assert window['visibility'] in ['visible', 'hidden'] and window['visibilityNow'] in ['visible', 'hidden']
    window_projection = {'index': window_index, 'timeOrigin': origin, **{key: window[key] for key in ['version', 'observedEvents', 'droppedEvents', 'overflow', 'unsupported', 'visibility', 'visibilityNow']}, 'latencyStatus': {key: diagnostic[key] for key in ['version', 'observed', 'dropped', 'overflow', 'resourceUnsupported', 'callbacks', 'callbackMs', 'visibility']}}
    windows.append(window_projection)
    for original in diagnostic['rows']:
        assert original['kind'] in latency_fields
        value = exact_fields(original, latency_fields[original['kind']])
        for key in value:
            if key in ['sequence', 'at', 'inputSequence', 'duration', 'responseStatus', 'requestStart', 'responseStart', 'responseEnd', 'transferSize', 'encodedBodySize', 'decodedBodySize']:
                if key != 'responseStatus' or value[key] is not None: numeric(value[key])
        if 'route' in value:
            assert value['route'] in routes
        if value['kind'] == 'expected-dom':
            assert value['actionKind'] in ['input', 'project-switch']
            value['input'] = ref(value['input']); value['pointer'] = ref(value['pointer'])
        browser_rows.append({'windowIndex': window_index, 'timeOrigin': origin, **value})
    for original in window['events']:
        value = exact_fields(original, ['sequence', 'name', 'startTime', 'duration', 'processingStart', 'processingEnd', 'interactionId'])
        assert value['name'] in event_names
        for key in value:
            if key != 'name': numeric(value[key])
        events.append({'windowIndex': window_index, 'timeOrigin': origin, **value})
assert len(browser_rows) == data['latencyDiagnostics']['rows']
assert len(events) == data['browserTimingLedger']['events']
lookup = {(row['timeOrigin'], row['sequence']): row for row in browser_rows}
assert len(lookup) == len(browser_rows)

cohorts = {}
for phase in [PRIMARY]:
    for kind in ['project-switch', 'input', 'draft-reset']:
        key = phase + ':' + kind
        group = [row for row in rows if row['phase'] == phase and row['kind'] == kind]
        record = {**quantiles([row['elapsedMs'] for row in group]), 'successful': len(group), 'failed': 0}
        assert all(data['interactionSummary'][key][key2] == value for key2, value in record.items())
        assert data['interactionSummary'][key]['timeouts'] == []
        cohorts[key] = record

labels = [('allProjectsCommand', 'action-start', 'all-projects-click-end'), ('overviewHeadingWait', 'all-projects-click-end', 'overview-heading-ready'), ('openProjectCommand', 'overview-heading-ready', 'open-project-click-end'), ('verifyExpectedContext', 'verify-start', 'verify-end'), ('twoFrameCommand', 'frames-start', 'frames-end')]

def event_association(pointer, origin):
    matches = [event for event in events if event['timeOrigin'] == origin and event['name'] == 'pointerdown' and event['processingStart'] <= pointer['at'] <= event['processingEnd']]
    associated = []
    if len(matches) == 1 and matches[0]['interactionId']:
        associated = [event for event in events if event['timeOrigin'] == origin and event['interactionId'] == matches[0]['interactionId']]
    return {'method': 'Unique EventTiming pointerdown whose recorded processing interval contains the logger timestamp; same nonzero interactionId joins subsequent events.', 'matchingPointerdownCount': len(matches), 'pointerdownWitnesses': matches, 'sameInteractionEvents': associated, 'limits': 'Coverage is thresholded and timestamp precision can prevent a match. Event duration is not a Node command boundary or product function duration.'}

spans = []
for row in rows:
    if row['kind'] != 'project-switch': continue
    stages = {stage['stage']: stage['offsetMs'] for stage in row['diagnostics']['nodeStages']}
    frame = row['diagnostics']['browserFrames']; refs = frame['references']; origin = refs['timeOrigin']
    pointer = lookup[(origin, refs['pointer']['sequence'])]
    expected = lookup[(origin, refs['expectedDOM']['sequence'])]
    assert pointer['kind'] == 'pointer-dispatch' and pointer['at'] == refs['pointer']['at']
    assert expected['kind'] == 'expected-dom' and expected['actionKind'] == 'project-switch' and expected['at'] == refs['expectedDOM']['at']
    node = {name: stages[end] - stages[start] for name, start, end in labels}
    browser = {'lastPointerToDriverExpectedDOMWitnessMs': expected['at'] - pointer['at'], 'driverExpectedDOMWitnessToFrameRequestMs': frame['requestedAt'] - expected['at'], 'frameRequestToSecondCallbackMs': frame['secondFrameAt'] - frame['requestedAt']}
    assert all(value >= 0 for value in [*node.values(), *browser.values()])
    overlapping = [resource for resource in browser_rows if resource['timeOrigin'] == origin and resource['kind'] == 'resource' and resource['at'] <= expected['at'] and resource['responseEnd'] >= pointer['at']]
    bracketing = {'status': 'No immediately preceding recorded action with same browser clock.'}
    if row['index'] > 0:
        previous = rows[row['index'] - 1]['diagnostics']['browserFrames']
        if previous['references']['timeOrigin'] == origin:
            candidates = [value for value in browser_rows if value['timeOrigin'] == origin and value['kind'] == 'pointer-dispatch' and previous['secondFrameAt'] < value['at'] <= pointer['at']]
            bracketing = {'status': 'Sequential browser-clock bracket, not explicit target identity.', 'previousActionIndex': row['index'] - 1, 'previousSecondFrameAt': previous['secondFrameAt'], 'pointerCount': len(candidates), 'pointers': candidates}
            if len(candidates) == 2 and candidates[-1]['sequence'] == pointer['sequence']:
                bracketing.update({'sourceConsistentHomePointer': candidates[0], 'homePointerToOpenPointerMs': pointer['at'] - candidates[0]['at'], 'homePointerEventAssociation': event_association(candidates[0], origin)})
    spans.append({'index': row['index'], 'phase': row['phase'], 'elapsedMs': row['elapsedMs'], 'nodeSpans': node, 'largestRecordedNodeSpan': max(node, key=node.get), 'browserClockSpans': browser, 'browserReferences': {'pointer': pointer, 'expectedDOM': expected, 'frames': frame}, 'openPointerEventAssociation': event_association(pointer, origin), 'homePointerAssociation': bracketing, 'temporallyOverlappingResources': overlapping, 'commandDelayBeforeVersusAfterPointer': 'Unknown: no browser-clock command-start/end anchors; Node and browser clock values are not aligned or subtracted.'})
primary = [row for row in spans if row['phase'] == PRIMARY]
slow = [row for row in primary if row['elapsedMs'] > 300]
followup = [row for row in spans if row['phase'] == FOLLOWUP]
assert len(primary) == 100 and len(followup) == 0

groups = {'primaryAll100': primary, 'primaryOver300ms': slow, 'primaryAtMost300ms': [row for row in primary if row['elapsedMs'] <= 300]}
limits = [
    'Original end-to-end rows and thresholds are unchanged. No overhead subtraction, outlier exclusion, mixed-clock subtraction, additive percentile or causal attribution.',
    'Node command spans include Playwright transport, scheduling and actionability; they do not isolate product functions.',
    'The expected-DOM witness is driver-triggered after locator wait and value check; it is not autonomous React-commit or physical-paint timing.',
    'The last-pointer reference is exact. A source-consistent first/Home pointer requires exactly two pointer rows after the previous action frame; it lacks explicit target identity.',
    'No browser-clock command-start/end anchors were retained, so an All projects command cannot be divided into pre-pointer versus post-pointer delay.',
    'Resource association is same-clock overlap only; route classes contain no exact project identity and do not prove request-to-action causality.',
    'EventTiming association requires a unique interval-containing pointerdown and nonzero interactionId. Thresholded observations and clock precision limit coverage.',
    'This run contains the complete primary400 actions only, without generic tracing and with separate numeric Home instrumentation. The previous eight-round Home probe did not sample these tail actions.',
    'No new app, browser, tests, runtime measurement, CPU profile or private state mutation. No product-completion claim.',
]
decisive_rows = []
for row in slow:
    home = row['homePointerAssociation']
    click = [event for event in home.get('homePointerEventAssociation', {}).get('sameInteractionEvents', []) if event['name'] == 'click']
    specs = [resource for resource in row['temporallyOverlappingResources'] if resource['route'] == 'project-specification']
    decisive_rows.append({'originalActionIndex': row['index'], 'primarySwitchOrdinal': primary.index(row) + 1, 'elapsedMs': row['elapsedMs'], 'largestRecordedNodeSpan': row['largestRecordedNodeSpan'], 'allProjectsCommandMs': row['nodeSpans']['allProjectsCommand'], 'verifyExpectedContextMs': row['nodeSpans']['verifyExpectedContext'], 'sourceConsistentHomePointerAvailable': 'sourceConsistentHomePointer' in home, 'homePointerToDestinationPointerMs': home.get('homePointerToOpenPointerMs'), 'homeClickEventTiming': [{'sequence': event['sequence'], 'interactionId': event['interactionId'], 'durationMs': event['duration'], 'processingStart': event['processingStart'], 'processingEnd': event['processingEnd'], 'processingIntervalMs': event['processingEnd'] - event['processingStart']} for event in click], 'destinationPointerToDriverExpectedDOMWitnessMs': row['browserClockSpans']['lastPointerToDriverExpectedDOMWitnessMs'], 'associatedSpecificationResources': [{'sequence': resource['sequence'], 'durationMs': resource['duration'], 'responseStatus': resource['responseStatus'], 'responseEnd': resource['responseEnd']} for resource in specs], 'wholeCommandBeforeVersusAfterPointer': 'Unknown; no shared command-boundary anchors.'})
save('slow-decisive-breakdown.json', {'originalResultSHA256': EXPECTED, 'rows': decisive_rows, 'limits': limits, 'decision': 'See analysis.md for the evidence-supported decision; node regions alone do not isolate product functions.'})
analysis = {'originalResultSHA256': EXPECTED, 'sourceCommit': data['sourceCommit'], 'sourceDirty': False, 'quantileMethod': 'Nearest rank: sorted[ceil(N*p)-1], including every successful row in its declared cohort.', 'cohorts': cohorts, 'pointerAssociationCoverage': {'primarySourceConsistentHomePointers': sum('sourceConsistentHomePointer' in row['homePointerAssociation'] for row in primary), 'primarySlowSourceConsistentHomePointers': sum('sourceConsistentHomePointer' in row['homePointerAssociation'] for row in slow)}, 'switchGroups': {name: {'elapsed': quantiles([row['elapsedMs'] for row in group]), 'largestRecordedSpanCounts': dict(collections.Counter(row['largestRecordedNodeSpan'] for row in group)), 'nodeSpanQuantiles': {label: quantiles([row['nodeSpans'][label] for row in group]) for label, _, _ in labels}, 'browserSpanQuantiles': {label: quantiles([row['browserClockSpans'][label] for row in group]) for label in primary[0]['browserClockSpans']}} for name, group in groups.items()}, 'primarySlowSwitches': slow, 'limits': limits}
save('analysis.json', analysis)
save('switches.all100.json', spans)

projection = {'originalResultSHA256': EXPECTED, 'originalResultBytes': len(raw), 'representation': 'Strict explicit allowlist; all 400 original action timings and stages retained, project identity and unneeded free text omitted. No whole-original reconstruction claim.', 'interactions': rows, 'cohorts': cohorts, 'latencyTargets': {'projectSwitchP95Ms': data['latencyTargets']['projectSwitchP95Ms'], 'inputP95Ms': data['latencyTargets']['inputP95Ms'], 'p99StrictlyBelowMs': data['latencyTargets']['p99StrictlyBelowMs']}, 'browserWindows': windows, 'browserLatencyRows': browser_rows, 'browserEventTimingRows': events, 'limits': limits}
(OUT / 'timings.all400.projection.json.gz').write_bytes(gzip.compress((json.dumps(projection, indent=2) + '\n').encode(), mtime=0))

source_directory = Path(data['externalSourceBinding']['executionDirectory'])
sources = {}
for name, ranges in [('interaction-metrics.mjs', [(61, 78)]), ('latency-diagnostics.mjs', [(10, 45), (54, 75)])]:
    source_raw = (source_directory / name).read_bytes()
    assert sha(source_raw) == data['externalSourceBinding']['localSources'][name]
    lines = source_raw.decode().splitlines()
    sources[name] = {'sha256': sha(source_raw), 'bytes': len(source_raw), 'lineExcerpts': [{'firstLine': start, 'lastLine': min(end, len(lines)), 'text': '\n'.join(lines[start - 1:end])} for start, end in ranges]}
save('source-binding.json', {'originalResultSHA256': EXPECTED, 'sourceCommit': data['sourceCommit'], 'sourceDirty': False, 'driverSHA256': data['driverSHA256'], 'recordedProductBinarySHA256': data['binarySHA256'], 'recordedWebIndexSHA256': data['webIndexSHA256'], 'recordedPackageArchiveSHA256': data['packageIntegrity']['archiveSHA256'], 'recordedPackageIntegrityAllMembersMatch': data['packageIntegrity']['allMembersMatch'], 'externalHelperHashes': data['externalSourceBinding']['localSources'], 'repositoryDependencyHashes': data['externalSourceBinding']['repositoryDependencies'], 'verifiedMetricSourceExcerpts': sources, 'limits': 'Metric helper bytes independently matched recorded hashes. Product/package identity is reported from the completed receipt, not independently re-executed. Private execution directories are omitted.'})

assert input_path.read_bytes() == raw
save('execution.json', {'startedFrom': 'Completed private original result supplied by operator', 'originalResultSHA256Before': EXPECTED, 'originalResultSHA256After': sha(input_path.read_bytes()), 'originalUnchanged': True, 'analysisScriptSHA256': sha(Path(__file__).read_bytes()), 'completedAt': datetime.datetime.now(datetime.timezone.utc).isoformat(), 'actionsProjected': len(rows), 'primaryActions': 400, 'followupActions': 0, 'browserLatencyRows': len(browser_rows), 'browserEventTimingRows': len(events), 'offlineOnly': True})
print(json.dumps({'actions': len(rows), 'primarySwitches': len(primary), 'primaryOver300ms': len(slow), 'dominantSlowRegions': dict(collections.Counter(row['largestRecordedNodeSpan'] for row in slow)), 'browserLatencyRows': len(browser_rows), 'browserEventTimingRows': len(events), 'originalUnchanged': True}))
