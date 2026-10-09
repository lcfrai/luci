#!/usr/bin/env python3
"""Assemble existing sanitized C054 attempt9 evidence; no runtime actions."""
from pathlib import Path
from datetime import datetime, timezone
import hashlib, json, os, re, subprocess

B = Path('/workspace/.luci-tools/cloud054-terminal-close')
S = B / 'actual-29931afc/evidence/attempt9'
O = Path(__file__).parent / 'capsule'
C = '29931afc5b31ae9b9bd7b8973a7e3fdb840609a3'
IMAGE = 'sha256:76043c5d0f30207c05343e1dbc0f450658eb52726590c8c47c714c67371acec8'
PACKAGE = Path('/workspace/luci/artifacts/quality-swarm/candidate-29931afc/package')

def sha(data): return hashlib.sha256(data).hexdigest()
def meta(data): return {'sha256': sha(data), 'bytes': len(data)}
def enc(value): return (json.dumps(value, indent=2) + '\n').encode()
def read(path): return json.loads(path.read_bytes())
def put(path, data):
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    assert not path.exists(), path
    path.write_bytes(data)
    path.chmod(0o600)
def original(path):
    before = path.stat()
    assert path.is_file() and not path.is_symlink() and before.st_size <= 4 * 1024 * 1024
    data = path.read_bytes()
    after = path.stat()
    assert (before.st_ino, before.st_size, before.st_mtime_ns) == (after.st_ino, after.st_size, after.st_mtime_ns)
    return data

if not __debug__:
    raise RuntimeError('Optimized Python would disable binding checks and is unsupported.')
os.umask(0o077)
assert not O.exists()
private = (S / 'driver/private-fixture-path').read_text().strip()
assert private.startswith(str(B / 'actual-29931afc/fixtures') + '/')
replacement = '[PRIVATE_FIXTURE_ATTEMPT_9]'
orig = []
def copy(path, name):
    raw = original(path)
    public = raw.replace(private.encode(), replacement.encode())
    assert path.suffix != '.png' or public == raw
    put(O / name, public)
    orig.append({'source': str(path), 'publishedPath': name, 'original': meta(raw),
                 'published': meta(public), 'representation': 'exact-original' if raw == public else 'exact-private-root-substitution',
                 'substitutions': raw.count(private.encode())})

r = read(S / 'driver/result.json')
w = read(S / 'wrapper-result.json')
closure = read(S / 'independent-closure.json')
assert sha(original(S / 'independent-closure.json')) == 'a09eb5a493d835b39980fd4ec7b32897b172cd847bc241fe6fa5285122891437'
assert r['sourceCommit'] == r['helperSourceCommit'] == C
assert r['passed'] is False and w['passed'] is False and w['launcherExitCode'] == 1
assert r['failure'] == {'name': 'AssertionError', 'code': 'ERR_ASSERTION', 'message': 'Six GiB fixture root floor'}
assert w['driverReport']['sha256'] == closure['originalResultSHA256'] == sha(original(S / 'driver/result.json'))
assert closure['originalWrapperSHA256'] == sha(original(S / 'wrapper-result.json'))
assert w['sourceBefore'] == w['sourceAfter']
for path, identity in w['sourceBefore'].items():
    assert meta(original(Path(path))) == identity

O.mkdir(mode=0o700)
put(O / '.gitattributes', b'# Preserve captured evidence bytes across platforms.\n** -text\n*.stdout -whitespace\n*.stderr -whitespace\n')
put(O / '.gitignore', b'# Captured logs are intentional publication evidence.\n!*.log\n')
for name in ['before-launch.json', 'identities.jsonl', 'launcher.stderr', 'launcher.stdout', 'wrapper-result.json']:
    copy(S / name, 'run/' + name)
phases = sorted((S / 'driver').glob('phase-*.json'))
assert len(phases) == 35 and len(r['phases']) == 35
for path in phases:
    copy(path, 'run/driver/' + path.name)
for name in ['result.json', 'result.json.incomplete.json', 'package-provenance.json', 'image-provenance.json', 'failure.png', 'first-ended-poll-held.png']:
    copy(S / 'driver' / name, 'run/driver/' + name)
for path in sorted((S / 'driver/accepted-harness').rglob('*')):
    if path.is_file():
        copy(path, 'sources/accepted-harness/' + str(path.relative_to(S / 'driver/accepted-harness')))
assert len([p for p in (S / 'driver/accepted-harness').rglob('*') if p.is_file()]) == 6
for name, key in [('launch.mjs', 'launcherSHA256'), ('scenario.mjs', 'scenarioSHA256')]:
    assert sha(original(S / 'driver/accepted-harness' / name)) == r[key] == sha(original(B / 'installed-v8' / name))
for row in r['helperBindings']:
    data = subprocess.check_output(['git', '-C', '/workspace/luci', 'show', C + ':' + row['path']], timeout=30)
    assert meta(data) == {k: row[k] for k in ['sha256', 'bytes']}
    assert original(S / 'driver/accepted-harness' / row['path'].removeprefix('scripts/acceptance/')) == data
copy(B / 'outer-v2/run.py', 'sources/outer-run.py')
copy(S / 'independent-closure.json', 'closure/independent-closure.json')
copy(Path('/workspace/.luci-tools/cloud067-original-attempt9-journal.json'), 'cloud067-original-journal.json')
for name in ['prepared.json', 'preparation-refusal.json', 'RUN-attempt9.sh']:
    copy(B / 'actual-29931afc' / name, 'preflight/' + name)
for name in ['build-invocation.json', 'build-result.json', 'package-integrity.json', 'package-provenance.json', 'previous-release-preservation.json']:
    copy(PACKAGE / name, 'package/' + name)
assert read(PACKAGE / 'build-result.json')['exitCode'] == 0
assert read(PACKAGE / 'package-integrity.json')['verifiedExtractedFiles'] == r['packageIntegrity']['verifiedMembers'] == 962
package = read(S / 'driver/package-provenance.json')
assert package == read(PACKAGE / 'package-provenance.json')
assert package['sourceCommit'] == C and package['sourceDirty'] is False
for key in ['binarySHA256', 'webIndexSHA256', 'archiveSHA256']:
    assert r['package'][key] == package[key]
image = read(S / 'driver/image-provenance.json')
assert image['candidateImage'] == r['daemon']['imageID'] == closure['expectedImageID'] == IMAGE
assert image['sourceCommit'] == '5f4a966f6d70264c92ee2176dde912e1b43fc3c7'
assert image['sourcesBefore'] == image['sourcesAfter'] and len(image['sourcesAfter']) == 106
runtime = []
for path, digest in image['sourcesAfter'].items():
    data = subprocess.check_output(['git', '-C', '/workspace/luci', 'show', C + ':' + path], timeout=30)
    assert sha(data) == digest
    runtime.append({'path': path, 'sha256': digest, 'bytes': len(data)})
put(O / 'runtime-source-bindings.json', enc({'selectedProductSource': C, 'imageBuildSource': image['sourceCommit'], 'imageID': IMAGE, 'count': 106, 'inputs': runtime,
    'scope': 'All 106 captured runtime inputs match immutable selected product source. Image was carried from its recorded build source, not rebuilt or redownloaded for this publication.'}))

typed = next(p for p in r['phases'] if p['kind'] == 'typed-exit-before-poll')
rows = typed['wire']['browser']['rows']
assert len(rows) == 1
row = rows[0]
assert len(row['exit']) == 1 and row['exit'][0]['reason'] == {'kind': 'exit-status', 'code': 0}
assert row['exit'][0]['payload']['sha256'] == sha(b'exit status 0')
assert row['closed']['code'] == 1000 and row['closed']['wasClean'] is True and row['error'] is False
assert row['markers']['first']['sequence'] == 72 and row['exit'][0]['sequence'] == 73
assert next(e['sequence'] for e in row['events'] if e['kind'] == 'close') == 74
assert typed['ui']['footer'].endswith('ended') and typed['ui']['overlay'] is None
failure = next(p for p in r['phases'] if p['kind'] == 'scenario-failure')
assert failure['step'] == 'single-ui-restart'
assert not [x for x in r['apiActions'] if x['path'].endswith('/restart')]
assert not [x for x in r['browserMutations'] if x['path'].endswith('/restart')]
released = next(p for p in r['phases'] if p['kind'] == 'poll-released')
ready = next(p for p in r['phases'] if p.get('step') == 'second-old-poll-hold')
assert released['released'] == '2026-10-09T14:50:52.776Z'
assert ready['observedAt'] == '2026-10-09T14:50:53.887Z'
def millis(value): return datetime.fromisoformat(value.replace('Z', '+00:00')).timestamp() * 1000
release_to_ready = round(millis(ready['observedAt']) - millis(released['released']))
exit_to_ready = round(millis(ready['observedAt']) - millis(row['exit'][0]['wall']))
assert release_to_ready == 1111 and exit_to_ready == 3861
close_delta = row['closed']['monotonicMs'] - row['exit'][0]['monotonicMs']
stop = next(x for x in r['cleanupPhases'] if x['name'] == 'supported-session-stop')
assert stop['status'] == 'completed' and stop['result']['ledger']['leases'] == []
assert len(stop['result']['containers']) == 1 and stop['result']['containers'][0]['status'] == 'exited'
assert any(x['method'] == 'DELETE' and x['status'] == 200 for x in r['apiActions'])
assert r['final']['ledger']['leases'] == [] and r['final']['containers'] == [] and r['final']['volumes'] == '' and r['pendingMutations'] == []
assert closure['knownLiveProcessCount'] == 0 and closure['knownZombieCount'] == 2
assert closure['containers'] == [] and closure['volumes'] == [] and all(v == 0 for v in closure['ledger']['counts'].values())
assert r['privateTransportErrors']['rows'] == 0 and r['privateTransportErrors']['passed'] is True
assert len(r['controllerLog']['relayRows']) == 1 and r['controllerLog']['relayRows'][0]['flags']['websocketStatus'] == 1000
analysis = {'productSource': C, 'helperVersion': 8, 'imageID': IMAGE, 'overallPassed': False,
    'originalResult': meta(original(S / 'driver/result.json')), 'originalWrapper': meta(original(S / 'wrapper-result.json')),
    'failure': {'driver': r['failure'], 'scenarioProjection': failure['error'], 'step': failure['step'], 'freeBytesAtFailingAssertionRecorded': False},
    'counts': {'phaseRecords': 35, 'apiRestart': 0, 'browserForwardedRestart': 0, 'firstSocketTypedExit': 1, 'replacementAllocationObserved': False, 'secondExitObserved': False, 'providerInvoked': False},
    'firstTerminal': {'uiWhilePollHeld': typed['ui'], 'exit': row['exit'][0], 'close': row['closed'], 'socketError': row['error'], 'outputExitCloseSequence': [72, 73, 74]},
    'statusTiming': {'pollRelease': released['released'], 'nextMilestone': ready['observedAt'], 'nextMilestoneKind': 'second-old-poll-hold', 'restartPresenceConfirmedByMilestone': True,
        'afterPollReleaseUpperBoundMs': release_to_ready, 'afterTypedExitReceiptUpperBoundMs': exit_to_ready, 'typedExitReceiptToCloseMonotonicMs': close_delta,
        'exactRestartAppearanceTimestampRetained': False, 'firstFreshStatusResponseTimestampRetained': False,
        'derivation': 'Accepted scenario.mjs awaits Restart count 1 after gate release/dispose and before recording second-old-poll-hold; the timestamp is an upper bound, not exact appearance latency.'},
    'cleanup': r['cleanup'], 'closure': {'knownLive': closure['knownLiveProcessCount'], 'knownZombies': closure['knownZombieCount'], 'ledgerCounts': closure['ledger']['counts'], 'originalSHA256': sha(original(S / 'independent-closure.json'))},
    'preflight': {'missingParentFailure': 'outer argument validation; no owned child', 'separateFromExecutedAttempt': True},
    'sourceChecks': {'utilityGitHelpers': 4, 'acceptedExternalHelpers': 2, 'currentRuntimeInputs': 106, 'sourceBeforeAfterEqual': True},
    'limits': ['Overall failed. No product Restart response/replacement socket/second exit7.', 'No failure-time atomic free-byte operand retained.', 'No runtime action or private fixture census performed by publication.', 'Known identities and selected inventory only; no whole-machine closure, model, Windows or performance acceptance.']}
put(O / 'analysis.json', enc(analysis))
copy(Path(__file__).parent / 'README.prepared.md', 'README.md')
copy(Path(__file__), 'assemble.py')
excluded = [
    {'path': 'driver/private-fixture-path', 'reason': 'Read solely to substitute the exact fixture root; pointer and fixture-root bytes excluded.'},
    {'path': 'driver/private-controller.stderr', 'bytes': r['controllerLog']['retainedBytes'], 'sha256': r['controllerLog']['retainedPrefixSHA256'], 'hashScope': 'Writer-reported retained prefix; not reread by publisher.', 'reason': 'Raw potentially private error/log bytes excluded; recorded safe relay projection retained.'},
    {'path': 'driver/private-transport-errors.jsonl', 'bytes': r['privateTransportErrors']['bytes'], 'sha256': r['privateTransportErrors']['sha256'], 'hashScope': 'Writer-reported private sink; not reread by publisher.', 'reason': 'Private diagnostic sink excluded even when empty.'},
    {'path': 'launcher-home/ and original fixture contents', 'reason': 'Private state/configuration/identity/account/session/credential files were not read or exported.'},
    {'path': 'binary package/image/archive', 'reason': 'Only existing provenance and verification receipts are copied; no independent binary reread or retrieval.'}]
put(O / 'original-bindings.json', enc({'originals': orig, 'excluded': excluded, 'redactionPolicy': {'replacement': replacement, 'rootSHA256': sha(private.encode()), 'rootBytes': len(private.encode()), 'onlyExactRootSubstitution': True}}))
for item in orig:
    assert meta(original(Path(item['source']))) == item['original']
put(O / 'original-immutability.json', enc({'checked': len(orig), 'allUnchanged': True, 'scope': 'Before/after selected publication originals only. Private-state invariance is separately attested by existing independent closure; no repeat private fixture census.'}))
patterns = {'private-key': rb'-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----',
    'github-token': rb'(?:gh[pousr]_[A-Za-z0-9]{30,}|github_pat_[A-Za-z0-9_]{50,})',
    'openai-token': rb'\bsk-(?:proj-)?[A-Za-z0-9_-]{30,}',
    'bearer-value': rb'(?i)bearer\s+[A-Za-z0-9_.-]{24,}',
    'url-userinfo': rb'https?://[^/@\s:]+:[^/@\s]+@'}
findings = []
for path in O.rglob('*'):
    if not path.is_file(): continue
    data = path.read_bytes()
    assert private.encode() not in data
    if path.suffix == '.png': continue
    for label, pattern in patterns.items():
        if re.search(pattern, data): findings.append({'path': str(path.relative_to(O)), 'pattern': label})
assert findings == [], findings
put(O / 'privacy-scan.json', enc({'boundedPatternFindings': findings, 'exactPrivateRootAbsent': True, 'screenshotsManuallyReviewed': ['run/driver/first-ended-poll-held.png', 'run/driver/failure.png'],
    'scope': 'Bounded common credential-signature scan, safe field review and visual screenshot review; not exhaustive detection. Private raw diagnostic sinks/state were not reread or exported.'}))
members = {str(path.relative_to(O)): meta(path.read_bytes()) for path in sorted(O.rglob('*')) if path.is_file()}
put(O / 'publication.json', enc({'version': 1, 'productSource': C, 'overallPassed': False, 'scope': 'Scoped first exit and fresh Restart readiness positives; failed before any Restart forwarding. Missing-parent preflight is separate.', 'files': members}))
put(O / 'SHA256SUMS', (''.join(value['sha256'] + '  ' + name + '\n' for name, value in members.items()) + sha((O / 'publication.json').read_bytes()) + '  publication.json\n').encode())
assert {str(path.relative_to(O)) for path in O.rglob('*') if path.is_file()} == set(members) | {'publication.json', 'SHA256SUMS'}
for name, identity in members.items(): assert meta((O / name).read_bytes()) == identity
print(json.dumps({'capsule': str(O), 'members': len(members), 'totalFiles': len(members) + 2, 'originals': len(orig), 'publicationSHA256': sha((O / 'publication.json').read_bytes()), 'independentClosureSHA256': sha((O / 'closure/independent-closure.json').read_bytes())}, indent=2))
