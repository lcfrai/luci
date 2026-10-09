#!/usr/bin/env python3
"""Package retained safe warm receipts only; no product execution or raw-log reads."""
from pathlib import Path
from datetime import datetime, timezone
import hashlib
import json
import os
import re
import subprocess

BASE = Path('/workspace/.luci-tools')
SOURCE = BASE / 'cloud070-installed-b94ad37e/evidence'
OUT = Path(__file__).resolve().parent
COMMIT = 'b94ad37e53181e360ac8910ee980570de622728c'
IMAGE = 'sha256:76043c5d0f30207c05343e1dbc0f450658eb52726590c8c47c714c67371acec8'

def sha(data): return hashlib.sha256(data).hexdigest()
def meta(data): return {'sha256': sha(data), 'bytes': len(data)}
def encode(value): return (json.dumps(value, indent=2) + '\n').encode()
def original(path):
    before = path.stat()
    assert path.is_file() and not path.is_symlink() and before.st_size <= 4 * 1024 * 1024
    data = path.read_bytes()
    after = path.stat()
    assert (before.st_ino, before.st_size, before.st_mtime_ns) == (after.st_ino, after.st_size, after.st_mtime_ns)
    return data
def read(path): return json.loads(original(path))
def put(path, data):
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    assert not path.exists(), str(path)
    with path.open('xb') as file: file.write(data)
    path.chmod(0o600)
def git_blob(path):
    return subprocess.check_output(['git', '-C', '/workspace/luci', 'show', COMMIT + ':' + path], timeout=30)

def assemble(attempt):
    src = SOURCE / f'warm-attempt{attempt}'
    closure_dir = BASE / f'cloud070-installed-b94ad37e-warm-attempt{attempt}-closure'
    dest = OUT / f'warm-attempt{attempt}'
    assert not dest.exists()
    pointer = original(src / 'driver/private-fixture-path')
    private = pointer.decode().strip()
    assert private.startswith(str(BASE / 'cloud070-installed-b94ad37e/fixtures') + '/')
    replacement = f'[PRIVATE_FIXTURE_WARM_ATTEMPT_{attempt}]'
    originals = []

    def copy(path, name):
        raw = original(path)
        public = raw.replace(private.encode(), replacement.encode())
        assert path.suffix != '.png' or raw == public
        put(dest / name, public)
        originals.append({'source': str(path), 'publishedPath': name, 'original': meta(raw), 'published': meta(public),
            'representation': 'exact-original' if raw == public else 'exact-private-root-substitution',
            'substitutions': raw.count(private.encode())})

    result = read(src / 'driver/result.json')
    wrapper = read(src / 'wrapper-result.json')
    closure = read(closure_dir / 'closure.json')
    failure = read(closure_dir / 'failure.json')
    closure_manifest = read(closure_dir / 'manifest.json')
    assert result['sourceCommit'] == result['helperSourceCommit'] == COMMIT
    assert result['passed'] is False and wrapper['passed'] is False and wrapper['launcherExitCode'] == 1
    assert failure['originalResultSHA256'] == wrapper['driverReport']['sha256'] == sha(original(src / 'driver/result.json'))
    assert failure['originalWrapperSHA256'] == sha(original(src / 'wrapper-result.json'))
    assert wrapper['sourceBefore'] == wrapper['sourceAfter']
    assert closure['sourceCommit'] == COMMIT and closure['imageID'] == IMAGE
    assert closure['daemonIDMatches'] is True and closure['containerIDs'] == closure['volumeNames'] == []
    assert closure['ledger']['counts'] == {'leases': 0, 'checks': 0, 'fences': 0, 'checkFences': 0}
    for row in closure_manifest['members']:
        assert meta(original(closure_dir / row['path'])) == {k: row[k] for k in ['sha256', 'bytes']}

    dest.mkdir(mode=0o700)
    put(dest / '.gitattributes', b'# Preserve original evidence bytes across platforms.\n** -text\n')
    for name in ['before-launch.json', 'identities.jsonl', 'wrapper-result.json']:
        copy(src / name, 'run/' + name)
    phases = sorted((src / 'driver').glob('phase-*.json'))
    assert len(phases) == len(result['phases']) == (6 if attempt == 1 else 46)
    for index, path in enumerate(phases):
        assert read(path) == result['phases'][index]
        copy(path, 'run/driver/' + path.name)
    for name in ['result.json', 'result.json.incomplete.json', 'package-provenance.json', 'image-provenance.json']:
        copy(src / 'driver' / name, 'run/driver/' + name)
    screenshots = []
    if attempt == 2:
        for name in ['failure.png', 'first-ended-poll-held.png']:
            copy(src / 'driver' / name, 'run/driver/' + name)
            screenshots.append('run/driver/' + name)
    harness = src / 'driver/accepted-harness'
    harness_files = [p for p in sorted(harness.rglob('*')) if p.is_file()]
    assert len(harness_files) == 6
    for path in harness_files:
        copy(path, 'sources/accepted-harness/' + str(path.relative_to(harness)))
    source_bindings = []
    for path, identity in wrapper['sourceBefore'].items():
        data = original(Path(path))
        assert meta(data) == identity
        source_bindings.append({'path': path, **identity, 'matchesOriginalBeforeAfterAndPublication': True})
        if Path(path).name == 'outer-run.py': copy(Path(path), 'sources/outer-run.py')
    for name, key in [('launch.mjs', 'launcherSHA256'), ('scenario.mjs', 'scenarioSHA256')]:
        assert sha(original(harness / name)) == result[key]
        assert meta(original(harness / name)) == wrapper['sourceBefore'][str(BASE / 'cloud069-terminal-cleanup-v11/warm' / name)]
    utilities = []
    for row in result['helperBindings']:
        assert row['commit'] == COMMIT
        data = git_blob(row['path'])
        assert meta(data) == {k: row[k] for k in ['sha256', 'bytes']}
        assert original(harness / row['path'].removeprefix('scripts/acceptance/')) == data
        utilities.append(row)
    package = read(src / 'driver/package-provenance.json')
    assert package['sourceCommit'] == COMMIT and package['sourceDirty'] is False
    assert result['packageIntegrity'] == {'verifiedMembers': 963, 'allMembersMatch': True}
    for key in ['binarySHA256', 'webIndexSHA256', 'archiveSHA256']:
        assert result['package'][key] == package[key]
    image = read(src / 'driver/image-provenance.json')
    assert image['candidateImage'] == result['daemon']['imageID'] == IMAGE
    assert image['sourcesBefore'] == image['sourcesAfter'] and len(image['sourcesAfter']) == 106
    runtime = []
    for path, digest in image['sourcesAfter'].items():
        data = git_blob(path)
        assert sha(data) == digest
        runtime.append({'path': path, **meta(data)})
    put(dest / 'source-bindings.json', encode({'selectedProductSource': COMMIT, 'acceptedExternalHelper': 'CLOUD-069 v11 warm (pre-C071 response diagnostic fix)',
        'externalSourceBindings': source_bindings, 'immutableGitUtilityBindings': utilities,
        'packageVerification': result['packageIntegrity'], 'packageSubjects': result['package'],
        'scope': 'Frozen accepted helper copies match original run hashes. External launcher/scenario/wrapper match recorded before/after hashes and publication-time bytes. Four utility helpers also match immutable selected Git blobs. Package receipts are bound; package binaries were not reread or rebuilt.'}))
    put(dest / 'runtime-source-bindings.json', encode({'selectedProductSource': COMMIT, 'imageBuildSource': image['sourceCommit'], 'imageID': IMAGE,
        'count': len(runtime), 'inputs': runtime,
        'scope': 'All 106 recorded runtime inputs match immutable selected product source. Carried image provenance is retained; no rebuild, image pull, or runtime verification was performed for publication.'}))
    for row in closure_manifest['members']:
        copy(closure_dir / row['path'], 'closure/' + row['path'])
    copy(closure_dir / 'manifest.json', 'closure/original-manifest.json')

    browser_restart = sum(x['method'] == 'POST' and x['path'].endswith('/restart') for x in result['browserMutations'])
    api_restart = sum(x['method'] == 'POST' and x['path'].endswith('/restart') for x in result['apiActions'])
    stop_count = sum(x['method'] == 'DELETE' or x['path'].endswith('/stop') for x in result['apiActions'] + result['browserMutations'])
    assert api_restart == stop_count == 0
    common_limits = ['Both original driver and wrapper remain failed.',
        'Independent later closure is a separate observation and does not rewrite original cleanup or settle an ambiguous mutation.',
        'Known process identities and selected daemon only; historical unreadable process observations preclude an exhaustive process census.',
        'No numeric refusal-time admission operands; later or advisory memory/disk values cannot establish the refusal-time cause.',
        'No permanent-leak, model/provider, native app/service crash, Windows, or performance acceptance claim.',
        'Raw controller, transport-error, authentication, stdout/stderr, and fixture state contents were not read or published by this assembler.']
    analysis = {'productSource': COMMIT, 'attempt': attempt, 'overallPassed': False,
        'originalResult': meta(original(src / 'driver/result.json')), 'originalWrapper': meta(original(src / 'wrapper-result.json')),
        'counts': {'phases': len(phases), 'browserRestart': browser_restart, 'apiRestart': api_restart, 'recordedStop': stop_count,
            'pendingMutations': len(result['pendingMutations'])},
        'failureStage': result['failedStage'], 'failure': result['failure'],
        'automaticCleanup': {'finalPresent': 'final' in result, 'phaseStatuses': [{'name': x['name'], 'status': x['status']} for x in result['cleanupPhases']]},
        'independentClosure': {'observedAt': closure['at'], 'counts': closure['counts'], 'sameDaemon': closure['daemonIDMatches'],
            'containerCount': len(closure['containerIDs']), 'volumeCount': len(closure['volumeNames']), 'ledgerCounts': closure['ledger']['counts'],
            'original': meta(original(closure_dir / 'closure.json'))}, 'limits': common_limits}
    if attempt == 1:
        assert result['failedStage'] == 'shell-create' and browser_restart == 0 and result['pendingMutations'] == []
        problem = next(p['problem'] for p in result['phases'] if p['kind'] == 'http-problem-diagnostic')
        assert problem['status'] == 409 and problem['expectedStatus'] == 201
        assert problem['details'] == {'reasonCode': 'memory_pressure'} and problem['originalBodyBytes'] == 153
        assert result['browserMutations'] == [] and failure['knownWorkerAllocations'] == 0
        analysis['refusal'] = problem
        summary = ('The installed shell-create request returned HTTP 409 before an acknowledged session, worker, or browser. '
            'The existing allowlisted problem receipt identifies conflict/memory_pressure; its original 153-byte body is hash-bound and not exported. '
            'No numeric admission operands were captured. There was no Restart or terminal recovery exercise. '
            'The later independent closure found two known identities absent, no live or zombie identities, and empty container, volume, and ledger inventories. '
            'This later absence does not erase the controller zombie observation in original cleanup.')
    else:
        assert result['failedStage'] == 'terminal-scenario' and browser_restart == 1 and len(result['pendingMutations']) == 1
        assert 'final' not in result and failure['restart']['observedStatus'] == 409 and failure['restart']['responseBodyRetained'] is False
        assert not [p for p in result['phases'] if p['kind'] in ['restart-http-response', 'replacement-allocation-observed', 'scenario-complete']]
        settlement = next(p for p in result['phases'] if p['kind'] == 'initial-settlement-confirmed')
        assert abs(settlement['elapsedMs'] - 16009.196028) < 0.001 and settlement['polls'] == 24
        assert settlement['last']['containers'] == [] and all(settlement['last']['ledger'][k] == [] for k in ['leases', 'checks', 'fences', 'checkFences'])
        typed = next(p for p in result['phases'] if p['kind'] == 'typed-exit-before-poll')
        row = typed['wire']['browser']['rows'][0]
        assert len(row['exit']) == 1 and row['exit'][0]['reason'] == {'kind': 'exit-status', 'code': 0}
        assert row['closed']['code'] == 1000 and row['closed']['wasClean'] is True
        sequence = [row['markers']['first']['sequence'], row['exit'][0]['sequence'], next(x['sequence'] for x in row['events'] if x['kind'] == 'close')]
        assert sequence == sorted(sequence) and len(set(sequence)) == 3
        analysis['firstTerminal'] = {'typedExit': row['exit'][0], 'close': row['closed'], 'uiWhileOldPollHeld': typed['ui'], 'outputExitCloseSequence': sequence}
        analysis['naturalSettlement'] = {'phase': 'run/driver/phase-033.json', 'polls': settlement['polls'], 'elapsedMs': settlement['elapsedMs'],
            'allocationAndLease': settlement['pin'], 'last': settlement['last']}
        analysis['restart'] = {'observedStatus': 409, 'bodyCaptured': False, 'originalBodyHashAvailable': False,
            'numericAdmissionOperandsAvailable': False, 'screenshotWitnessSeparateFromHTTPBody': True,
            'originalHelperAssertedStatusBeforeBodyRead': True, 'pendingMutationRetained': True}
        summary = ('The first real shell produced typed Exit 0 followed by a browser-reported clean Close 1000. '
            'The ended pane was observed while an old running poll was held. Natural absence of the exact original allocation and release of its manual lease were confirmed in 24 polls over 16.009196 seconds, before one UI Restart. '
            'Restart returned HTTP 409. The original C069 helper asserted status before reading the body; no Restart problem body or body hash was retained. '
            'The screenshot is separate UI evidence, not a recovered response body or numeric admission operands. '
            'The original result retains one pending mutation; exact-container-leftovers and final-source-and-inventory remain failed-or-unsettled. '
            'No successful Restart, replacement allocation, second Exit 7, or completed warm recovery was observed. '
            'Later independent closure found three known identities absent, two browser zombies, and empty container, volume, and ledger inventories. '
            'That later sample does not certify remote mutation settlement or retroactively pass automatic cleanup.')
    put(dest / 'analysis.json', encode(analysis))
    readme = ('# Installed b94 warm attempt ' + str(attempt) + '\n\n' + summary + '\n\n'
        'Product and Git utility source: `' + COMMIT + '`. The accepted external warm launcher and scenario are the original CLOUD-069 v11 helpers, before the CLOUD-071 diagnostic fix. '
        'The product package receipt reports 963 matching members. Frozen helpers, original wrapper before/after hashes, four immutable Git utility bindings, and 106 carried-image runtime input bindings are retained. '
        'Publication did not rerun the product or independently reverify package binaries.\n\n'
        'Copied evidence bytes are unchanged except exact substitution of the one private fixture root with `' + replacement + '`. '
        '`original-bindings.json` retains original and public hashes, byte counts, and substitution counts. Original raw log hashes, where listed, come only from existing safe receipts; those logs were not reread. '
        'All stdout/stderr and raw private diagnostic or fixture files are excluded. Screenshots were previously visually reviewed by the supervising agent and are copied unchanged.\n\n'
        '`run/` contains original run evidence; `closure/` contains separately timed independent closure and its original manifest. '
        'Original manifests refer to original bytes; `publication.json` and `SHA256SUMS` bind the public bytes. `analysis.json` contains derived, explicitly scoped summaries. '
        'Receipt fields describing historical inventories are not current live allocations. Neither a 409 refusal nor later empty inventory proves permanent leakage or remote mutation settlement.\n\n'
        'The bounded credential-pattern scan is not exhaustive. Scope is the retained local-shell/browser evidence, known process identities, and selected daemon; no whole-machine, provider/model, native service-crash, Windows, or latency/performance acceptance is claimed.\n')
    put(dest / 'README.md', readme.encode())
    copy(Path(__file__).resolve(), 'assemble-warm.py')
    retained = {row['path']: row for row in closure['retainedFiles']}
    excluded = []
    for name in ['launcher.stdout', 'launcher.stderr', 'driver/private-controller.stderr', 'driver/private-transport-errors.jsonl']:
        row = retained.get(name)
        excluded.append({'path': name, 'originalRecordedMetadata': {k: row[k] for k in ['sha256', 'bytes']} if row else None,
            'hashScope': 'Existing independent closure metadata only; raw file not reread.',
            'reason': 'Raw or unreviewed diagnostic output excluded.'})
    excluded += [{'path': 'driver/private-fixture-path', 'original': meta(pointer), 'reason': 'Read solely for exact root substitution; pointer bytes excluded.'},
        {'path': 'launcher-home/ and original fixture contents', 'reason': 'Not read or exported; no credential, auth, ledger, registry, history or output contents reread.'},
        {'path': 'binary package/image/archive', 'reason': 'Existing provenance receipts only; binaries not read, retrieved, rebuilt or executed.'}]
    put(dest / 'original-bindings.json', encode({'originals': originals, 'excluded': excluded,
        'redactionPolicy': {'replacement': replacement, 'rootSHA256': sha(private.encode()), 'rootBytes': len(private.encode()), 'onlyExactRootSubstitution': True}}))
    for row in originals: assert meta(original(Path(row['source']))) == row['original']
    assert original(src / 'driver/private-fixture-path') == pointer
    put(dest / 'original-immutability.json', encode({'checked': len(originals), 'allUnchanged': True,
        'scope': 'Selected published originals and private-root pointer were unchanged across assembly. Full original fixture invariance was not reobserved; existing independent closure attestation remains separately timed.'}))
    patterns = {'private-key': rb'-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----',
        'github-token': rb'(?:gh[pousr]_[A-Za-z0-9]{30,}|github_pat_[A-Za-z0-9_]{50,})',
        'openai-token': rb'\bsk-(?:proj-)?[A-Za-z0-9_-]{30,}',
        'bearer-value': rb'(?i)bearer\s+[A-Za-z0-9_.-]{24,}', 'url-userinfo': rb'https?://[^/@\s:]+:[^/@\s]+@'}
    findings = []
    for path in dest.rglob('*'):
        if not path.is_file(): continue
        data = path.read_bytes()
        assert private.encode() not in data
        if path.suffix == '.png': continue
        for label, pattern in patterns.items():
            if re.search(pattern, data): findings.append({'path': str(path.relative_to(dest)), 'pattern': label})
    assert not findings, findings
    put(dest / 'privacy-scan.json', encode({'boundedPatternFindings': findings, 'exactPrivateRootAbsent': True,
        'screenshotsPreviouslyReviewedBySupervisingAgent': screenshots,
        'scope': 'Exact-root absence and bounded common credential-signature scan. Safe structured receipt field review and previously reported visual review; not exhaustive secret detection. All raw logs/stdout/stderr and private fixture contents excluded without reading.'}))
    members = {str(path.relative_to(dest)): meta(path.read_bytes()) for path in sorted(dest.rglob('*')) if path.is_file()}
    put(dest / 'publication.json', encode({'version': 1, 'productSource': COMMIT, 'attempt': attempt, 'overallPassed': False,
        'scope': summary, 'files': members}))
    put(dest / 'SHA256SUMS', (''.join(value['sha256'] + '  ' + name + '\n' for name, value in members.items())
        + sha((dest / 'publication.json').read_bytes()) + '  publication.json\n').encode())
    assert {str(p.relative_to(dest)) for p in dest.rglob('*') if p.is_file()} == set(members) | {'publication.json', 'SHA256SUMS'}
    for name, identity in members.items(): assert meta((dest / name).read_bytes()) == identity
    return {'capsule': str(dest), 'files': len(members) + 2, 'selectedOriginals': len(originals),
        'publicationSHA256': sha((dest / 'publication.json').read_bytes()), 'checksumsSHA256': sha((dest / 'SHA256SUMS').read_bytes()),
        'originalResultSHA256': sha(original(src / 'driver/result.json')), 'originalClosureSHA256': sha(original(closure_dir / 'closure.json'))}

if __name__ == '__main__':
    if not __debug__: raise RuntimeError('Optimized Python disables required checks.')
    os.umask(0o077)
    print(json.dumps([assemble(1), assemble(2)], indent=2))
