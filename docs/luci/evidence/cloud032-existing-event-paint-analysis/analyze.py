#!/usr/bin/env python3
"""Offline only. Verify pinned published inputs; preserve every original action/click.

No app/browser/network/profiler is launched. No Node/browser clock subtraction.
Output directory must be fresh. Optional private original is read, never copied.
"""
import argparse
import collections
import hashlib
import json
import math
from pathlib import Path

PINS = {
    'attempt2/all-browser-timing.json': '34a83cceb71e5823b3d5910b464bf1569a43e7dec9a6d1c88473fd9b3d551560',
    'attempt2/all-interactions.json': 'd378b4fdfcb3ab7fd132633319de00b7b1d1db1842fcf8dfb668c023ddd042cf',
    'attempt2/result-allowlisted.json': 'bcc24748cbafbbca9ccb94dfa53996539f9313e6ea3e0a243f6fbc19c43286ce',
    'attempt2/analysis/all16-stage-joins.json': '94b50258c6753abc7db2cfac73f058229bdda0fd32e6716f724bf9122a925e06',
    'attempt2/layout-trace-capture.json': '6f26ad42ea68117e7f70f7e38471684f7ddfcd6da4aab8fa07765224f9d12a5f',
    'accepted-preparation-v2/actual/interaction-metrics.mjs': '1bec6579a3fc27310c88bfbb378321bf3fc764de62900f9be068fffe3626a2cc',
    'accepted-preparation-v2/actual/latency-diagnostics.mjs': '4d1c0f75f15c01444c22133f360f10700af58c6dfebd3906700e67dc9407313f',
    'accepted-preparation-v2/actual/autonomous-readiness.mjs': '7f534431a37a0bd599c2de48870d3c9e8badfd1e33b2d48e47078782aa74467b',
    'accepted-preparation-v2/runtime-binding.json': '4afa510e062fbeee777eba04b1f57a1ba8137347cf61ee4ed1e3c09b19f880da',
}
RAW_SHA = 'fead13b770f2351282fc4c9a1bd9d0ab34248b087311e9b4ccdb651ed424fe26'
PRODUCT = 'b94ad37e53181e360ac8910ee980570de622728c'
EVENT_KEYS = ('sequence', 'name', 'startTime', 'duration', 'processingStart', 'processingEnd', 'interactionId')
WITNESS_KEYS = ('sequence', 'at', 'authorityGeneration', 'generation', 'round', 'projectOrdinal', 'stage', 'status', 'reason', 'clickAt', 'observedAt', 'samples', 'firstSampleAt', 'lastSampleAt', 'maxSampleGapMs', 'maxNodesVisited', 'maxControlsVisited')
NODE_REGIONS = {
    'allProjectsClick': ('action-start', 'all-projects-click-end'),
    'overviewHeadingWait': ('all-projects-click-end', 'overview-heading-ready'),
    'openProjectClick': ('open-project-click-start', 'open-project-click-end'),
    'ideaFieldWait': ('verify-start', 'idea-field-wait-end'),
    'contextEvaluate': ('idea-field-wait-end', 'idea-context-evaluate-end'),
    'inputValueAssertion': ('idea-context-evaluate-end', 'verify-end'),
    'finalFrames': ('frames-start', 'frames-end'),
}


def digest(data):
    return hashlib.sha256(data).hexdigest()


def bounded_read(path, limit):
    assert path.is_file() and path.stat().st_size <= limit, 'Absent or oversized declared input'
    with path.open('rb') as stream:
        value = stream.read(limit + 1)
    assert len(value) <= limit, 'Input grew beyond declared byte bound'
    return value


def statistics(values):
    values = sorted(v for v in values if v is not None)
    assert all(isinstance(v, (int, float)) and math.isfinite(v) for v in values)
    def rank(q):
        return values[max(0, math.ceil(len(values) * q) - 1)] if values else None
    return {'observedCount': len(values), 'minMs': values[0] if values else None,
            'p50Ms': rank(.5), 'p95Ms': rank(.95), 'p99Ms': rank(.99),
            'maxMs': values[-1] if values else None, 'quantile': 'nearest rank; missing observations excluded only from this statistic and retained as rows/counts'}


def write(path, value):
    path.write_text(json.dumps(value, indent=2, allow_nan=False) + '\n')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--evidence-root', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--original-result', type=Path)
    args = parser.parse_args()
    assert not args.output.exists(), 'Fresh output directory required; never overwrite receipts'
    inputs, blobs = [], {}
    for relative, expected in PINS.items():
        data = bounded_read(args.evidence_root / relative, 2 * 1024**2)
        assert digest(data) == expected, 'Declared published input hash mismatch: ' + relative
        inputs.append({'path': relative, 'bytes': len(data), 'sha256': expected})
        blobs[relative] = data
    browser = json.loads(blobs['attempt2/all-browser-timing.json'])
    actions_document = json.loads(blobs['attempt2/all-interactions.json'])
    actions = actions_document['rows']
    prior = json.loads(blobs['attempt2/analysis/all16-stage-joins.json'])
    result = json.loads(blobs['attempt2/result-allowlisted.json'])
    capture = json.loads(blobs['attempt2/layout-trace-capture.json'])
    assert result['sourceCommit'] == PRODUCT and result['rawResultSHA256'] == RAW_SHA
    assert len(actions) == 32 and [a['index'] for a in actions] == list(range(32))
    expected_kinds = ['project-switch', 'input', 'input', 'draft-reset'] * 8
    assert [a['kind'] for a in actions] == expected_kinds
    assert actions_document['rawResultSHA256'] == RAW_SHA
    original_check = {'provided': False, 'expectedSHA256': RAW_SHA, 'copied': False}
    if args.original_result:
        raw = bounded_read(args.original_result, 16 * 1024**2)
        assert digest(raw) == RAW_SHA, 'Private original hash mismatch'
        original = json.loads(raw)
        assert all(original[k] == browser[k] for k in browser), 'Published timing differs from original'
        projects = list(dict.fromkeys(a['projectID'] for a in original['interactions']))
        assert len(projects) == 4
        projected = [{**{k: v for k, v in a.items() if k != 'projectID'}, 'index': i,
                      'projectOrdinal': projects.index(a['projectID'])}
                     for i, a in enumerate(original['interactions'])]
        assert projected == actions, 'Published all-action projection differs from original'
        original_check = {'provided': True, 'bytes': len(raw), 'sha256': digest(raw), 'copied': False,
                          'allFourPublishedTimingFieldsEqual': True, 'all32ActionProjectionEqual': True}

    # Sequence checks cover every document/window/event, not just the 16 matches.
    events, documents, window_rows, issues = [], {}, [], []
    windows = browser['browserEventWindows']
    assert len(windows) <= 2000
    for wi, window in enumerate(windows):
        origin = window['timeOrigin']
        previous = documents.get(origin, 0)
        for event in window['events']:
            if event['sequence'] != previous + 1:
                issues.append({'kind': 'event-sequence-gap-or-duplicate', 'windowIndex': wi, 'expected': previous + 1, 'actual': event['sequence']})
            previous = event['sequence']
            assert all(math.isfinite(event[k]) for k in ('startTime', 'duration', 'processingStart', 'processingEnd'))
            events.append({'windowIndex': wi, 'timeOrigin': origin, **{k: event[k] for k in EVENT_KEYS}})
        if previous != window['observedEvents']:
            issues.append({'kind': 'event-counter-mismatch', 'windowIndex': wi})
        documents[origin] = previous
        if window['overflow'] or window['droppedEvents']:
            issues.append({'kind': 'event-overflow', 'windowIndex': wi})
        window_rows.append({'windowIndex': wi, **{k: window[k] for k in ('timeOrigin', 'observedEvents', 'droppedEvents', 'overflow', 'unsupported', 'visibility', 'visibilityNow')},
                            'retainedEvents': len(window['events']), 'latencySnapshotVisibility': window['latency']['visibility']})
    assert len(events) <= 200000

    readiness = browser['autonomousReadiness']
    stage_rows, terminal_rows, readiness_windows = [], [], []
    previous = 0
    for wi, window in enumerate(readiness['windows']):
        for row in window['rows']:
            if row['sequence'] != previous + 1:
                issues.append({'kind': 'readiness-sequence-gap-or-duplicate', 'windowIndex': wi})
            previous = row['sequence']
            safe = {k: row.get(k) for k in WITNESS_KEYS}
            safe.update({'windowIndex': wi, 'timeOrigin': window['timeOrigin']})
            (stage_rows if row['stage'] in ('home', 'define') else terminal_rows).append(safe)
        if previous != window['sequence']:
            issues.append({'kind': 'readiness-counter-mismatch', 'windowIndex': wi})
        readiness_windows.append({'windowIndex': wi, **{k: window[k] for k in ('timeOrigin', 'round', 'sequence', 'overflow', 'lateCallbacks', 'closed', 'activeGeneration')}})
    assert len(stage_rows) <= 200
    old_by_key = {(r['round'], r['stage']): r for r in prior['rows']}
    rows = []
    for round_number in range(8):
        action = actions[round_number * 4]
        marks = action['diagnostics']['nodeStages']
        assert len(marks) == 12 and len({m['stage'] for m in marks}) == 12
        mark_map = {m['stage']: m['offsetMs'] for m in marks}
        assert list(mark_map.values()) == sorted(mark_map.values())
        for stage in ('home', 'define'):
            witnesses = [w for w in stage_rows if w['round'] == round_number and w['stage'] == stage]
            witness = witnesses[0] if len(witnesses) == 1 else None
            if witness:
                assert witness['projectOrdinal'] == action['projectOrdinal'] == round_number % 4
                assert witness['authorityGeneration'] == readiness['setup']['authorityGeneration'] == 1
                assert witness['generation'] == round_number * 2 + (1 if stage == 'home' else 2)
                assert witness['timeOrigin'] == readiness['setup']['timeOrigin']
            matches = [] if witness is None else [e for e in events if e['timeOrigin'] == witness['timeOrigin'] and e['name'] == 'click' and e['processingStart'] <= witness['clickAt'] <= e['processingEnd']]
            event = matches[0] if len(matches) == 1 else None
            join_status = ('witness-missing' if not witnesses else 'witness-ambiguous' if len(witnesses) != 1 else
                           'event-missing-censored-or-unknown' if not matches else 'event-ambiguous' if len(matches) != 1 else 'unique')
            metric = None
            siblings = []
            if event:
                siblings = [e for e in events if e['timeOrigin'] == event['timeOrigin'] and event['interactionId'] != 0 and e['interactionId'] == event['interactionId']]
                metric = {'reportedEventDurationMs': event['duration'],
                          'reportedInteractionGroupMaxDurationMs': max((s['duration'] for s in siblings), default=None),
                          'inputQueueMs': event['processingStart'] - event['startTime'],
                          'handlerDispatchMs': event['processingEnd'] - event['processingStart'],
                          'reportedPostDispatchRemainderMs': event['startTime'] + event['duration'] - event['processingEnd']}
            dom_ms = None
            if witness and witness['status'] == 'ready' and witness['observedAt'] is not None:
                dom_ms = witness['observedAt'] - witness['clickAt']
            prior_row = old_by_key.get((round_number, stage))
            prior_agrees = bool(event and witness and prior_row and event == prior_row['event'] and witness['clickAt'] == prior_row['browserClickAt'] and witness['observedAt'] == prior_row['browserObservedAt'])
            rows.append({'round': round_number, 'projectOrdinal': action['projectOrdinal'], 'stage': stage,
                         'witnessCount': len(witnesses), 'witnesses': witnesses, 'eventJoinStatus': join_status,
                         'eventCandidateCount': len(matches), 'eventCandidates': matches,
                         'browserEventMetrics': metric, 'trustedClickToDOMObservedMs': dom_ms,
                         'sameInteractionMembers': siblings, 'sameInteractionClickCount': sum(e['name'] == 'click' for e in siblings),
                         'priorIndependentJoinAgreement': prior_agrees,
                         'nodeClockSeparate': {'actionIndex': action['index'], 'fullActionMs': action['elapsedMs'],
                                               'success': action['success'], 'nodeStages': marks,
                                               'regionsMs': {k: mark_map[b] - mark_map[a] for k, (a, b) in NODE_REGIONS.items()}}})
    coverage = {'originalActions': len(actions), 'expectedTrustedStages': 16, 'retainedTrustedStageRows': len(stage_rows),
                'outputStageRows': len(rows), 'joinStatuses': dict(collections.Counter(r['eventJoinStatus'] for r in rows)),
                'witnessStatuses': dict(collections.Counter(w['status'] for w in stage_rows)),
                'priorIndependentJoinAgreements': sum(r['priorIndependentJoinAgreement'] for r in rows),
                'zeroInteractionIds': sum(e['interactionId'] == 0 for r in rows for e in r['eventCandidates']),
                'multipleClickInteractionGroups': sum(r['sameInteractionClickCount'] > 1 for r in rows),
                'uniqueJoinedClickIdentities': len({(e['timeOrigin'], e['sequence']) for r in rows if r['eventJoinStatus'] == 'unique' for e in r['eventCandidates']}),
                'uniqueNonzeroInteractionIds': len({(e['timeOrigin'], e['interactionId']) for r in rows for e in r['eventCandidates'] if e['interactionId'] != 0}),
                'browserWindows': len(windows), 'browserDocuments': len(documents), 'allEventTimingEntries': len(events),
                'unsupportedWindowCount': sum(w['unsupported'] for w in windows),
                'hiddenSnapshotCount': sum(w['visibility'] != 'visible' or w['visibilityNow'] != 'visible' or w['latency']['visibility'] != 'visible' for w in windows),
                'recordedLateReadinessCallbacks': max(w['lateCallbacks'] for w in readiness['windows']),
                'readinessOverflowWindows': sum(w['overflow'] for w in readiness['windows']), 'sequenceOrOverflowIssues': issues}
    groups = {}
    for stage in ('home', 'define'):
        selected = [r for r in rows if r['stage'] == stage]
        groups[stage] = {'rowCount': len(selected), 'eventJoined': sum(r['eventJoinStatus'] == 'unique' for r in selected),
                         'eventDuration': statistics([r['browserEventMetrics']['reportedEventDurationMs'] if r['browserEventMetrics'] else None for r in selected]),
                         'reportedInteractionGroupMaxDuration': statistics([r['browserEventMetrics']['reportedInteractionGroupMaxDurationMs'] if r['browserEventMetrics'] else None for r in selected]),
                         'handlerDispatch': statistics([r['browserEventMetrics']['handlerDispatchMs'] if r['browserEventMetrics'] else None for r in selected]),
                         'postDispatchReportedRemainder': statistics([r['browserEventMetrics']['reportedPostDispatchRemainderMs'] if r['browserEventMetrics'] else None for r in selected]),
                         'trustedClickToDOMObserved': statistics([r['trustedClickToDOMObservedMs'] for r in selected])}
    action_groups = {kind: statistics([a['elapsedMs'] for a in actions if a['kind'] == kind]) for kind in dict.fromkeys(expected_kinds)}
    summary = {'scope': 'Offline reanalysis of eight instrumented b94 rounds, not a new measurement or acceptance cohort.',
               'productSource': PRODUCT, 'browserVersion': result['browserVersion'], 'coverage': coverage,
               'browserStageGroups': groups, 'fullNodeActionGroups': action_groups,
               'originalTargetsUnchanged': result['latencyTargets'], 'originalInteractionSummary': result['interactionSummary'],
               'traceCaptureComplete': capture['complete'], 'traceDataLossOccurred': capture['dataLossOccurred'],
               'clockPolicy': 'Only same-document browser differences and same-action Node differences; no cross-clock subtraction, normalization, overhead subtraction or additive CPU interpretation.',
               'limitations': ['EventTiming records have no target/isTrusted/action identifier; trust and target identity come from the original capture listener, joined uniquely within its same-document processing interval.',
                              'Duration threshold 16 ms, browser quantization and unreported entries prevent treating missing events as zero or a pass.',
                              'Click-event duration is separate from the maximum duration of reported pointerdown/up/click entries with the same document and interactionId; that group maximum is not page INP.',
                              'Home projectOrdinal denotes the next scheduled destination, not the project being exited.',
                              'Reported event duration concerns a next paint opportunity; it does not identify target-context pixels, compositor presentation or physical display.',
                              'DOM timestamps record when the observer sampled a matching predicate, not first possible readiness or a paint.',
                              'Visibility is sampled at observer initialization/drain only; intervening visibility changes are unavailable.',
                              'The post-dispatch remainder is derived from quantized reported duration, not separately measured paint cost or application work.',
                              'Trace and observer overhead are uncalibrated. Full Node action timings remain unchanged and cannot be corrected by subtraction.',
                              'Current source and CLOUD-078 are not the measured b94 product; no product-cause or before/after improvement is inferred.']}
    args.output.mkdir(parents=True)
    write(args.output / 'input-bindings.json', {'evidenceRoot': 'docs/luci/evidence/cloud032-layout-attribution-b94ad37e', 'inputs': inputs, 'privateOriginalVerification': original_check, 'privateOriginalRepublished': False})
    (args.output / 'all32-original-interactions.json').write_bytes(blobs['attempt2/all-interactions.json'])
    write(args.output / 'all16-click-joins.json', {'joinRule': 'same timeOrigin, name=click, processingStart <= trusted clickAt <= processingEnd; exactly one candidate required', 'rows': rows})
    write(args.output / 'observation-coverage.json', {'coverage': coverage, 'eventWindows': window_rows, 'readinessWindows': readiness_windows, 'terminalReadinessRows': terminal_rows, 'unrepresentedStageRows': [w for w in stage_rows if w['round'] not in range(8)]})
    write(args.output / 'summary.json', summary)
    print(json.dumps({'coverage': coverage, 'browserStageGroups': groups, 'fullNodeActionGroups': action_groups}, allow_nan=False))


if __name__ == '__main__':
    main()
