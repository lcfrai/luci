#!/usr/bin/env python3
"""Explicit safe C075 installed-evidence projection; no runtime or credential reads."""
import collections
import hashlib
import json
import re
import shutil
from pathlib import Path

BASE = Path('/workspace/.luci-tools')
PREP = BASE / 'cloud075-installed-preparation'
RUN = BASE / 'cloud075-private-runs/attempt-001'
CLOSURE = BASE / 'cloud075-installed-closure'
OUT = BASE / 'cloud075-installed-publication'
sha = lambda b: hashlib.sha256(b).hexdigest()

def read(path, limit=4*1024*1024):
    assert path.is_file() and not path.is_symlink()
    data = path.read_bytes()
    assert len(data) <= limit
    return data

def write(path, data):
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open('xb') as f:
        f.write(data)

def put(path, value):
    write(OUT / path, (json.dumps(value, indent=2) + '\n').encode())

assert not OUT.exists()
OUT.mkdir(mode=0o700)
manifest = read(PREP / 'SHA256SUMS')
assert sha(manifest) == '86791bcda8bda705c49241cf18b7bd8d6d38864e41489ecc3c70ac5bb060c9be'
for line in manifest.decode().splitlines():
    expected, name = line.split('  ', 1)
    assert not name.startswith('/') and '..' not in Path(name).parts
    data = read(PREP / name)
    assert sha(data) == expected
    write(OUT / 'driver' / name, data)
write(OUT / 'driver/SHA256SUMS', manifest)

raw_result = read(RUN / 'evidence/result.json')
raw_events = read(RUN / 'evidence/events.jsonl')
raw_closure = read(CLOSURE / 'attempt-001.json')
assert sha(raw_result) == 'cb2c704902d2cd9c1e0fe537601405ab6cc58dacdb2d60f99003b18dee0febdf'
assert sha(raw_events) == '3bb9675848a248ba80db62bc179c4a198318334848dc84daef1cf37c36b14347'
assert sha(raw_closure) == 'ed16f19579c61ac5e9cee657153ce6b5934268c1842058db663ee0a8d06b4ad6'
r = json.loads(raw_result)
events = [json.loads(line) for line in raw_events.splitlines()]
c = json.loads(raw_closure)
assert r['passed'] and r['executed'] and r['scenario']['passed']
assert len(events) == r['journal']['count'] == 103
assert r['journal']['sha256'] == sha(raw_events)

projects = list(dict.fromkeys(row['projectId'] for row in r['scenario']['permissionReads']))
assert len(projects) == 4
project_names = {value: 'project-' + str(i+1) for i, value in enumerate(projects)}
def public_path(path):
    assert isinstance(path, str) and path.startswith('/v1/') and '?' not in path and '#' not in path
    path = re.sub(r'/teams/[^/]+', '/teams/{team}', path)
    path = re.sub(r'/members/user-[^/]+', '/members/{member}', path)
    for key, value in project_names.items():
        path = path.replace('/projects/' + key, '/projects/' + value)
    return path

event_fields = {
    'stage': ['stage'],
    'startup': ['name', 'state', 'late', 'closeTimedOut', 'parent', 'closedByParent'],
    'controller-start': [],
    'operator-http': ['method', 'path', 'status'],
    'exact-managed-scopes-published': ['count'],
    'browser-mutation-or-probe': ['method', 'path'],
    'browser-mutation-terminal': ['method', 'path', 'failed'],
    'measurement-boundary': ['initialAccountReadinessExcluded', 'projectCount'],
    'project-permission-read': ['projectId', 'path', 'phase', 'status', 'delivery'],
    'explicit-read-transport-gate': ['phase', 'status', 'bodySHA256', 'delayMs', 'manual'],
    'project-permission-read-delivery': ['projectId', 'path', 'phase', 'status', 'delivery', 'bodySHA256'],
    'current-project-one-read': ['phase', 'projectId', 'count', 'scopedPurposeSHA256', 'transportDelayedMs'],
    'workspace-denial-before-grant-delivery': ['targetProjectId', 'privatePurposeVisible'],
    'no-stale-project-content-after-membership-only-recovery': ['projectCount', 'realMemberSpecificationStatus', 'heldResponseDelivery'],
    'browser-routes-sealed': ['sealed', 'pendingCallbacks', 'pendingMutations', 'closingReads'],
    'browser-routes-drained': ['sealed', 'pendingCallbacks', 'pendingMutations', 'closingReads'],
}
safe_events = []
omitted = collections.Counter()
for row in events:
    kind = row['kind']
    if kind == 'rail-navigation':
        omitted[kind] += 1
        continue
    assert kind in event_fields
    safe = {key: row[key] for key in ['sequence', 'at', 'kind'] + event_fields[kind] if key in row}
    if 'path' in safe:
        safe['path'] = public_path(safe['path'])
    for key in ['projectId', 'targetProjectId']:
        if key in safe:
            safe[key] = project_names[safe[key]]
    safe_events.append(safe)
assert len(safe_events) == 96
put('actual/events.safe.json', {
    'version': 1, 'projection': 'Explicit field allowlist; fixture project IDs aliased, authority paths normalized; seven rail DOM metadata rows omitted. Sequence numbers and timestamps retained.',
    'originalSHA256': sha(raw_events), 'originalBytes': len(raw_events), 'originalRows': len(events),
    'omittedByKind': dict(omitted), 'rows': safe_events,
})

permissions = []
for row in r['scenario']['permissionReads']:
    permissions.append({**{k: row[k] for k in ['phase', 'at', 'status', 'delivery', 'bodySHA256']}, 'project': project_names[row['projectId']]})
final = next(row['result'] for row in r['cleanupPhases'] if row['name'] == 'final-observations')
samples = final['unreadableProcessEnvironments']
put('actual/result.safe.json', {
    'version': 1, 'projection': 'Selected actual result fields; excludes private paths and unrelated process identities/environment diagnostics. Owned process proof is retained separately in closure.',
    'originalResultSHA256': sha(raw_result), 'originalResultBytes': len(raw_result),
    **{key: r[key] for key in ['passed', 'executed', 'product', 'packageProvenanceSHA256', 'packageIntegrity', 'binarySHA256', 'webIndexSHA256', 'startedAt', 'endedAt', 'browserVersion']},
    'scenario': {
        'passed': r['scenario']['passed'], 'projectCount': len(projects), 'permissionReads': permissions,
        'allowedBrowserMutationsAndReadinessProbes': r['scenario']['allowedBrowserMutationsAndReadinessProbes'],
        **{key: r['scenario'][key] for key in ['agentSessionsBefore', 'agentSessionsAfter', 'transportGating', 'latencyAcceptance', 'nativeDesktopAcceptance']},
        'zeroExecutionEvidence': 'Empty actual session lists before/after; browser/operator mutation allowlists reject execution and no provider is configured. The scenario providerCalls and agentOrCheckDispatches zero fields are declarations, not independent counters, and are intentionally not projected as measurements.',
        'scope': r['scenario']['scope'],
    },
    'operatorHTTP': {'count': sum(x['kind'] == 'operator-http' for x in events), 'methodStatusCounts': dict(collections.Counter(x['method'] + ' ' + str(x['status']) for x in events if x['kind'] == 'operator-http'))},
    'cleanup': {'errors': r['cleanup'], 'phaseStatuses': [{'name': x['name'], 'status': x['status']} for x in r['cleanupPhases']], 'startup': final['startup'], 'notWholeMachineClosure': final['notWholeMachineClosure'], 'unreadableEnvironmentDiagnosticSamples': {'retainedCount': len(samples), 'retentionCap': 128, 'codes': dict(collections.Counter(x['code'] for x in samples)), 'note': 'Retained capped scanner diagnostic samples, potentially repeated across scans; not a unique-process or total inventory. No PID or environment values published.'}},
    'routeBoundary': r['routeBoundary'], 'firstFailure': r.get('firstFailure'),
})

# Exact own-process proof is safe to retain; it contains no process environment,
# credentials, raw API response, session content, or unrelated process identities.
write(OUT / 'closure/receipt.json', raw_closure)
for name in ['inspect.py', 'README.md', 'SHA256SUMS']:
    write(OUT / 'closure' / ('source-' + name if name == 'SHA256SUMS' else name), read(CLOSURE / name))
for name in ['driver-start.json', 'driver-owned-identities.json', 'driver-managed-scopes.json']:
    data = read(RUN / name)
    expected = c['selectedReceiptsBefore'][name]
    assert sha(data) == expected['sha256'] and len(data) == expected['bytes']
    write(OUT / 'closure/ownership' / name, data)
put('closure/command-result.json', {'command': 'python3 inspect.py --parent /workspace/.luci-tools/cloud075-private-runs --run-root /workspace/.luci-tools/cloud075-private-runs/attempt-001 --output /workspace/.luci-tools/cloud075-installed-closure/attempt-001.json', 'exitCode': 2, 'reason': 'Inspection completed; two same-identity browser zombies prevent strict original-PID absence. All seven known originals are quiescent; five are absent. No cleanup mutation performed.'})
put('review.json', {'reviewer': '/root/performance_source_review/runtime_api_review', 'mode': 'Independent read-only selected result and frozen scenario review; no runtime or edits', 'supported': ['Four actual project selections, one permission GET per explicitly delayed overlap window, correct isolated purpose', 'Full-browser reload gives one fresh GET; restored grant entry gives one fresh GET', 'Held real 200 was fulfilled after workspace denial; membership-only recovery shows no private purposes and actual member spec GET is 403', 'Only browser login POST observed; actual session lists empty before/after'], 'limits': ['300 ms transport gates are request-count instrumentation, not latency acceptance', 'Exact unpublished microtask race remains source-test-only', 'Provider/dispatch zero fields are scenario declarations, not measured independent counters', 'No native desktop-window acceptance', 'Independent closure preserves two browser zombies and makes no whole-machine claim']})
put('publication.json', {'version': 1, 'intendedDestination': 'docs/luci/evidence/cloud075-installed-78665270-20261009', 'originalPreparationManifestSHA256': sha(manifest), 'originalResultSHA256': sha(raw_result), 'originalJournalSHA256': sha(raw_events), 'independentClosureSHA256': sha(raw_closure), 'carriedSourceCapsule': '../cloud075-project-authority-reads-20261009/REPORT.md', 'excluded': ['private run state', 'credentials and bootstrap tokens', 'raw stdout/stderr/private failures', 'screenshots', 'raw API bodies', 'raw full result/journal', 'unrelated process identities/environment samples', 'node_modules and package binary/web payload'], 'projectionSource': 'publication-extract.py'})
write(OUT / 'publication-extract.py', Path(__file__).read_bytes())
print(json.dumps({'prepared': str(OUT), 'projectedEvents': len(safe_events), 'originalEvents': len(events), 'permissionReads': len(permissions)}))
