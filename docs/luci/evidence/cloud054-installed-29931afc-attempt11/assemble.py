#!/usr/bin/env python3
"""Assemble existing sanitized C054 attempt11 evidence; no runtime actions."""
from pathlib import Path
from datetime import datetime, timezone
import hashlib, json, os, re, subprocess

B = Path('/workspace/.luci-tools/cloud054-terminal-close')
S = B / 'actual-29931afc/evidence/attempt11'
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
replacement = '[PRIVATE_FIXTURE_ATTEMPT_11]'
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
assert sha(original(S / 'independent-closure.json')) == '0b37cbc99eead1c620aaeb04920bbf9b2da3148fe47a1bc1b3caa47758f904c0'
assert r['sourceCommit'] == r['helperSourceCommit'] == C
assert r['passed'] is False and w['passed'] is False and w['launcherExitCode'] == 1
assert r['failure'] == {'name': 'Error', 'code': 'ETIMEDOUT', 'message': 'spawnSync docker ETIMEDOUT'}
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
    assert sha(original(S / 'driver/accepted-harness' / name)) == r[key] == sha(original(Path('/workspace/.luci-tools/cloud069-terminal-cleanup-v11/warm') / name))
for row in r['helperBindings']:
    data = subprocess.check_output(['git', '-C', '/workspace/luci', 'show', C + ':' + row['path']], timeout=30)
    assert meta(data) == {k: row[k] for k in ['sha256', 'bytes']}
    assert original(S / 'driver/accepted-harness' / row['path'].removeprefix('scripts/acceptance/')) == data
copy(B / 'outer-v2/run.py', 'sources/outer-run.py')
copy(S / 'independent-closure.json', 'closure/independent-closure.json')
package = read(S / 'driver/package-provenance.json')
assert r['packageIntegrity']['verifiedMembers'] == 962 and r['packageIntegrity']['allMembersMatch'] is True
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
assert row['markers']['first']['sequence'] == 70 and row['exit'][0]['sequence'] == 71
assert next(e['sequence'] for e in row['events'] if e['kind'] == 'close') == 72
assert typed['ui']['footer'].endswith('ended') and typed['ui']['overlay'] is None
failure = next(p for p in r['phases'] if p['kind'] == 'scenario-failure')
assert failure['step'] == 'initial-natural-settlement'
assert failure['error']['messageSHA256'] == sha(r['failure']['message'].encode())
assert failure['error']['messageBytes'] == len(r['failure']['message'].encode())
lifecycles = [p for p in r['phases'] if p['kind'] == 'initial-settlement-lifecycle']
assert [[x['status'] for x in p['statuses']] for p in lifecycles] == [['exited'], ['removing'], []]
assert [p['observedAt'] for p in lifecycles] == ['2026-10-09T15:24:41.798Z', '2026-10-09T15:25:02.166Z', '2026-10-09T15:25:05.378Z']
assert all(p['heldLeaseCount'] == 1 and p['ledgerSHA256'] == closure['ledger']['sha256'] for p in lifecycles)
settlement = [p for p in r['phases'] if p['kind'] == 'initial-settlement-observation']
assert len(settlement) == 3
assert not [p for p in r['phases'] if p['kind'] == 'initial-settlement-confirmed']
assert not [x for x in r['apiActions'] if x['path'].endswith('/restart')]
assert not [x for x in r['browserMutations'] if x['path'].endswith('/restart')]
released = next(p for p in r['phases'] if p['kind'] == 'poll-released')
ready = next(p for p in r['phases'] if p.get('step') == 'initial-natural-settlement')
assert released['released'] == '2026-10-09T15:24:40.588Z'
assert ready['observedAt'] == '2026-10-09T15:24:41.572Z'
def millis(value): return datetime.fromisoformat(value.replace('Z', '+00:00')).timestamp() * 1000
release_to_ready = round(millis(ready['observedAt']) - millis(released['released']))
exit_to_ready = round(millis(ready['observedAt']) - millis(row['exit'][0]['wall']))
assert release_to_ready == 984 and exit_to_ready == 3878
close_delta = row['closed']['monotonicMs'] - row['exit'][0]['monotonicMs']
stop = next(x for x in r['cleanupPhases'] if x['name'] == 'supported-session-stop')
assert stop['status'] == 'completed' and stop['result']['containers'] == []
ledger = stop['result']['ledger']
assert len(ledger['leases']) == 1 and ledger['sha256'] == closure['ledger']['sha256']
lease = ledger['leases'][0]
assert lease == {'id': 'manual-4c302090', 'sessionId': 's-0cf36a75', 'sandboxId': 's-0cf36a75', 'projectId': 'p-eccf393f', 'state': 'backend-settled', 'resources': {'CPUMillis': 1000, 'MemoryMiB': 1024}}
assert not any(x['method'] == 'DELETE' for x in r['apiActions'])
assert 'final' not in r and r['pendingMutations'] == []
assert next(x for x in r['cleanupPhases'] if x['name'] == 'final-source-and-inventory')['status'] == 'failed-or-unsettled'
assert closure['knownLiveProcessCount'] == 0 and closure['knownZombieCount'] == 2
assert closure['containers'] == [] and closure['volumes'] == [] and closure['ledger']['counts'] == {'leases': 1, 'checks': 0, 'fences': 0, 'checkFences': 0}
assert r['privateTransportErrors']['rows'] == 0 and r['privateTransportErrors']['passed'] is True
assert len(r['controllerLog']['relayRows']) == 1 and r['controllerLog']['relayRows'][0]['flags']['websocketStatus'] == 1000
last = next(p for p in r['phases'] if p['kind'] == 'scenario-failure-observation')
assert last['ui']['restartCount'] == 1 and len(last['wire']['browser']['rows']) == 1
private_inputs = read(Path(__file__).parent / 'private-hash-inputs.json')
assert {x['kind'] for x in private_inputs} == {'execution-ledger', 'session-registry', 'shell-history'}
private_meta = []
for item in private_inputs:
    path = Path(item['source'])
    assert str(path).startswith(private + '/')
    assert meta(original(path)) == {k: item[k] for k in ['sha256', 'bytes']}
    displayed = item['relativePath'] if item['kind'] != 'shell-history' else 'state/conversations/h/[OWNED_SHELL_HOME]/.bash_history'
    private_meta.append({'kind': item['kind'], 'relativePathRepresentation': displayed, 'relativePathSHA256': sha(item['relativePath'].encode()), 'original': {k: item[k] for k in ['sha256', 'bytes']}, 'contentExported': False})
assert next(x for x in private_meta if x['kind'] == 'execution-ledger')['original'] == {k: closure['ledger'][k] for k in ['sha256', 'bytes']}
put(O / 'private-state-source-bindings.json', enc({'capturedAt': datetime.now(timezone.utc).isoformat(), 'fixtureRootSHA256': sha(private.encode()), 'sources': private_meta, 'scope': 'Three explicitly scoped files hashed after closure, without exporting contents. Ledger hash equals independent closure; no separate output file or unrecorded historical hash time is claimed.'}))
put(O / 'closure/retained-lease-projection.json', enc({'closureObservedAt': closure['at'], 'independentClosureSHA256': sha(original(S / 'independent-closure.json')), 'originalPrivateLedger': {k: closure['ledger'][k] for k in ['sha256', 'bytes']}, 'lease': lease, 'counts': closure['ledger']['counts'], 'containerAndVolumeInventoryEmpty': True, 'source': 'Safe row projection from supported-session-stop phase; exact original ledger hash agrees with independent closure and publication-time bounded hash.', 'released': False, 'originalStateMutated': False}))
put(O / 'output-observation-bindings.json', enc({'originalResult': meta(original(S / 'driver/result.json')), 'typedPhaseObservedAt': typed['observedAt'], 'wire': {'stdoutFrames': row['stdoutFrames'], 'stdoutBytes': row['stdoutBytes'], 'markers': row['markers'], 'exit': row['exit'], 'closed': row['closed']}, 'finalPassiveUI': last['ui'], 'screenshots': [{'publishedPath': 'run/driver/' + name, 'original': meta(original(S / 'driver' / name))} for name in ['first-ended-poll-held.png', 'failure.png']], 'scope': 'Bindings to existing output observations/markers/wire and screenshots only; no separate output file claimed. Private shell-history/session contents excluded.'}))
source_path = 'control-plane/internal/api/execution_leases.go'
source_bytes = subprocess.check_output(['git', '-C', '/workspace/luci', 'show', C + ':' + source_path], timeout=30)
assert meta(source_bytes) == {'sha256': '539dff31c1c9c06f4f0c6250d392ac5f72a0e52d34bad69bb5e73221ef9f7a3d', 'bytes': 28162}
source_lines = source_bytes.decode().splitlines(keepends=True)
put(O / 'source-contract.json', enc({'productSource': C, 'path': source_path, 'originalBlob': meta(source_bytes),
    'excerpts': [{'firstLine': start, 'lastLine': end, 'text': ''.join(source_lines[start-1:end])} for start, end in [(238, 286), (308, 323)]],
    'contract': 'A later eligible worker admission invokes bounded reconciliation that can remove a same-scope, settled, exited worker lease when the required absence proof is available.',
    'scope': 'Immutable source contract only. A charge held at closure alone does not establish a permanent leak; subsequent admission and release were not observed or performed.'}))
put(O / 'settlement-observations.json', enc({'originalResult': meta(original(S / 'driver/result.json')), 'lifecycles': lifecycles, 'observations': settlement,
    'failureObservedAt': failure['observedAt'], 'failure': r['failure'], 'scenarioErrorProjection': failure['error'],
    'budgetMs': 35000, 'settlementConfirmed': False, 'restartForwarded': False,
    'scope': 'Exact recorded lifecycle and charge operands, followed by the actual Docker subprocess timeout. Root-free-byte values belong only to their respective observation timestamps; no atomic failure-time free-byte operand was retained.'}))
analysis = {'productSource': C, 'helper': 'CLOUD-069 external v11 warm', 'imageID': IMAGE, 'overallPassed': False,
    'originalResult': meta(original(S / 'driver/result.json')), 'originalWrapper': meta(original(S / 'wrapper-result.json')),
    'failure': {'driver': r['failure'], 'scenarioProjection': failure['error'], 'step': failure['step'], 'observedLifecycleStates': ['exited', 'removing', 'absent'], 'actualDockerTimeoutRecorded': True, 'permanentLeakInferred': False, 'productDefectClassified': False},
    'counts': {'phaseRecords': 35, 'apiRestart': 0, 'browserForwardedRestart': 0, 'recordedSessionDELETE': 0, 'firstSocketTypedExit': 1, 'replacementAllocationObserved': False, 'secondExitObserved': False, 'providerInvoked': False},
    'firstTerminal': {'uiWhilePollHeld': typed['ui'], 'exit': row['exit'][0], 'close': row['closed'], 'socketError': row['error'], 'outputExitCloseSequence': [70, 71, 72], 'lastPassiveUI': last['ui']},
    'statusTiming': {'pollRelease': released['released'], 'nextMilestone': ready['observedAt'], 'nextMilestoneKind': 'initial-natural-settlement', 'restartPresenceConfirmedByMilestone': True, 'afterPollReleaseUpperBoundMs': release_to_ready, 'afterTypedExitReceiptUpperBoundMs': exit_to_ready, 'typedExitReceiptToCloseMonotonicMs': close_delta, 'exactRestartAppearanceTimestampRetained': False, 'firstFreshStatusResponseTimestampRetained': False, 'derivation': 'Accepted scenario awaits Restart count 1 after first gate release/dispose and before initial-natural-settlement milestone; not exact appearance latency.'},
    'cleanup': {'errors': r['cleanup'], 'driverFinalPresent': False, 'retainedLease': lease, 'manualReleasePerformed': False},
    'closure': {'knownLive': closure['knownLiveProcessCount'], 'knownZombies': closure['knownZombieCount'], 'zombieIdentities': [p for p in closure['identities'] if p['now'].get('state') == 'Z'], 'ledgerCounts': closure['ledger']['counts'], 'originalSHA256': sha(original(S / 'independent-closure.json'))},
    'sourceChecks': {'utilityGitHelpers': 4, 'acceptedExternalHelpers': 2, 'currentRuntimeInputs': 106, 'sourceBeforeAfterEqual': True},
    'limits': ['Failed before Restart: no warm/cold recovery acceptance.', 'Observed exited/removing/absent states precede actual Docker timeout; no permanent-leak or product-defect inference.', 'Container absence does not release retained capacity.', 'Private state/history only hashed; no contents exported or hand release.', 'Known identities and selected inventory only; no whole-machine/model/Windows/performance acceptance.']}
put(O / 'analysis.json', enc(analysis))
copy(Path(__file__).parent / 'README.prepared.md', 'README.md')
copy(Path(__file__), 'assemble.py')
excluded = [
    {'path': 'driver/private-fixture-path', 'reason': 'Read solely to substitute the exact fixture root; pointer and fixture-root bytes excluded.'},
    {'path': 'driver/private-controller.stderr', 'bytes': r['controllerLog']['retainedBytes'], 'sha256': r['controllerLog']['retainedPrefixSHA256'], 'hashScope': 'Writer-reported retained prefix; not reread by publisher.', 'reason': 'Raw potentially private error/log bytes excluded; recorded safe relay projection retained.'},
    {'path': 'driver/private-transport-errors.jsonl', 'bytes': r['privateTransportErrors']['bytes'], 'sha256': r['privateTransportErrors']['sha256'], 'hashScope': 'Writer-reported private sink; not reread by publisher.', 'reason': 'Private diagnostic sink excluded even when empty.'},
    {'path': 'launcher-home/ and original fixture contents', 'reason': 'No contents exported. Only three explicitly scoped ledger/session/history files were read for bounded hashes; credentials/configuration and other private files remain unread.'},
    {'path': 'binary package/image/archive', 'reason': 'Only existing provenance and verification receipts are copied; no independent binary reread or retrieval.'}]
put(O / 'original-bindings.json', enc({'originals': orig, 'excluded': excluded, 'redactionPolicy': {'replacement': replacement, 'rootSHA256': sha(private.encode()), 'rootBytes': len(private.encode()), 'onlyExactRootSubstitution': True}}))
for item in orig:
    assert meta(original(Path(item['source']))) == item['original']
for item in private_inputs:
    assert meta(original(Path(item['source']))) == {k: item[k] for k in ['sha256', 'bytes']}
put(O / 'original-immutability.json', enc({'checked': len(orig), 'allUnchanged': True, 'scope': 'Before/after selected publication originals only. Three scoped private file hashes were also unchanged; original fixture invariance is separately attested by existing closure. No full private fixture census.'}))
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
    'scope': 'Bounded common credential-signature scan, safe field review and visual screenshot review; not exhaustive detection. Private raw diagnostic sinks were not reread; three scoped state/history files were hashed without content export.'}))
members = {str(path.relative_to(O)): meta(path.read_bytes()) for path in sorted(O.rglob('*')) if path.is_file()}
put(O / 'publication.json', enc({'version': 1, 'productSource': C, 'overallPassed': False, 'scope': 'First exit and fresh Restart readiness positives; failed at initial-natural-settlement before any Restart forwarding. Actual removing then absent observed before Docker timeout; one manual reservation remains at closure.', 'files': members}))
put(O / 'SHA256SUMS', (''.join(value['sha256'] + '  ' + name + '\n' for name, value in members.items()) + sha((O / 'publication.json').read_bytes()) + '  publication.json\n').encode())
assert {str(path.relative_to(O)) for path in O.rglob('*') if path.is_file()} == set(members) | {'publication.json', 'SHA256SUMS'}
for name, identity in members.items(): assert meta((O / name).read_bytes()) == identity
print(json.dumps({'capsule': str(O), 'members': len(members), 'totalFiles': len(members) + 2, 'originals': len(orig), 'publicationSHA256': sha((O / 'publication.json').read_bytes()), 'independentClosureSHA256': sha((O / 'closure/independent-closure.json').read_bytes())}, indent=2))
