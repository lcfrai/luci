#!/usr/bin/env python3
"""Prepare public retained evidence only; never launch or modify the original run."""
from pathlib import Path
import hashlib, json, os, re, subprocess

B = Path(__file__).resolve().parent
S = Path('/workspace/.luci-tools/cloud077-installed-972d6864/evidence/warm-attempt1')
O = B/'capsule'
C = '972d6864a30b742b12545c8e12f4ca3ae2faa884'
CL = Path('/workspace/.luci-tools/cloud077-installed-972d6864-analysis/independent-closure.json')
sha = lambda raw: hashlib.sha256(raw).hexdigest()
meta = lambda raw: {'bytes': len(raw), 'sha256': sha(raw)}
enc = lambda value: (json.dumps(value, indent=2)+'\n').encode()
os.umask(0o077)
assert not O.exists()

def read(path):
    path = Path(path)
    assert path.name not in ['private-controller.stderr', 'private-transport-errors.jsonl', 'launcher.stdout', 'launcher.stderr']
    before = path.stat()
    assert path.is_file() and not path.is_symlink() and before.st_size < 4*1024*1024
    raw = path.read_bytes()
    after = path.stat()
    assert (before.st_ino, before.st_size, before.st_mtime_ns) == (after.st_ino, after.st_size, after.st_mtime_ns)
    return raw

def all_hashes():
    result = {}
    for path in sorted(S.rglob('*')):
        if path.is_file():
            assert not path.is_symlink()
            with path.open('rb') as stream:
                result[str(path.relative_to(S))] = {'bytes': path.stat().st_size, 'sha256': hashlib.file_digest(stream, 'sha256').hexdigest()}
    return result

before = all_hashes()
private = read(S/'driver/private-fixture-path').decode().strip()
assert private.startswith('/workspace/.w77-972d6864-a1/luci-c054-')
replacement = '[PRIVATE_FIXTURE_WARM_972D_ATTEMPT_1]'
originals = []

def put(name, raw):
    path = O/name
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open('xb') as stream:
        stream.write(raw)

def copy(source, name):
    source = Path(source)
    raw = read(source)
    public = raw.replace(private.encode(), replacement.encode())
    put(name, public)
    originals.append({'source': str(source), 'publishedPath': name, 'original': meta(raw), 'published': meta(public),
        'representation': 'exact-original' if raw == public else 'exact-private-root-substitution', 'substitutions': raw.count(private.encode())})

r = json.loads(read(S/'driver/result.json'))
w = json.loads(read(S/'wrapper-result.json'))
assert sha(read(S/'driver/result.json')) == '0d01c6ba4b5b2ed21adcfb91088f84785c980ad9cb6932953466ec923ba5e4e5'
assert r['sourceCommit'] == r['helperSourceCommit'] == C
assert r['passed'] is False and w['passed'] is False and w['launcherExitCode'] == 1 and w['failure'] is None
assert r['scenario']['passed'] is False and r['scenario']['warmRestartPassed'] is True
assert r['scenario']['attempts'] == {'initial': 1, 'replacement': 1, 'total': 2}
assert r['scenario']['successfulGenerationStreams'] == 2
ordering = r['scenario']['oldReadOrdering']
assert ordering['exercised'] is False and ordering['releasedMonotonicMs'] < ordering['acknowledgedMonotonicMs']
assert ordering['originalGateBudgetMs'] == 8000 and ordering['clientReadBudgetMs'] == 10000
assert r['scenario']['requiredAcceptanceIncomplete'] == ['installed-old-response-after-restart-acknowledgement']
assert r['cleanup'] == r['pendingMutations'] == []
assert all(row['status'] == 'completed' for row in r['cleanupPhases'])
assert r['failureObservation']['failurePresent'] is False
assert w['sourceBefore'] == w['sourceAfter'] and w['driverReport']['sha256'] == before['driver/result.json']['sha256']
assert r['launcherSHA256'] == '0ebd521c6160a19848e6e549b890478af1ed83d7e2230399862879c2d000a66f'
assert r['scenarioSHA256'] == '31981be982085541d1f79c1af3c52a6e480b939efc9d4020abb577905e98b2a6'
completed = next(row for row in r['phases'] if row['kind'] == 'scenario-complete')
streams = completed['wire']['browser']['rows']
assert len(streams) == 2
for row, code in zip(streams, [0, 7]):
    assert row['exit'][0]['reason'] == {'kind': 'exit-status', 'code': code}
    assert row['closed']['code'] == 1000 and row['closed']['wasClean'] is True
assert len([row for row in r['browserMutations'] if row['path'].endswith('/restart')]) == 1
assert r['packageIntegrity'] == {'verifiedMembers': 965, 'allMembersMatch': True}
cl = json.loads(read(CL))
assert sha(read(CL)) == 'c33b6bc241bb802234b35633dada6ab8749ece78a5578b9c64462be1ddf69beb'
assert cl['sameEngine'] and cl['sameImage'] and cl['containers'] == cl['volumes'] == []
assert cl['ledger']['counts'] == {'leases': 0, 'checks': 0, 'fences': 0, 'checkFences': 0}

O.mkdir()
put('.gitattributes', b'** -text\n')
for name in ['before-launch.json', 'identities.jsonl', 'wrapper-result.json']:
    copy(S/name, 'run/'+name)
phases = sorted((S/'driver').glob('phase-*.json'))
assert len(phases) == len(r['phases']) == 55
for path in phases:
    copy(path, 'run/driver/'+path.name)
for name in ['result.json', 'result.json.incomplete.json', 'package-provenance.json', 'image-provenance.json']:
    copy(S/'driver'/name, 'run/driver/'+name)
accepted = sorted(path for path in (S/'driver/accepted-harness').rglob('*') if path.is_file())
assert len(accepted) == 6
for path in accepted:
    copy(path, 'sources/accepted-harness/'+str(path.relative_to(S/'driver/accepted-harness')))
for name, key in [('launch.mjs', 'launcherSHA256'), ('scenario.mjs', 'scenarioSHA256')]:
    assert sha(read(S/'driver/accepted-harness'/name)) == r[key]
repo_bindings = {}
for row in r['helperBindings']:
    raw = subprocess.check_output(['git', '-C', '/workspace/luci', 'show', C+':'+row['path']], timeout=20)
    assert meta(raw) == {key: row[key] for key in ['sha256', 'bytes']}
    assert read(S/'driver/accepted-harness'/row['path'].removeprefix('scripts/acceptance/')) == raw
    repo_bindings[row['path']] = meta(raw)
outer = Path('/workspace/.luci-tools/cloud077-terminal-observation-budget/fixed/outer-run.py')
assert meta(read(outer)) == w['sourceBefore'][str(outer)]
copy(outer, 'sources/outer-run.py')
copy(CL, 'closure/independent-closure.json')
copy(CL.with_name('independent-warm-closure.py'), 'closure/independent-warm-closure.py')
package = json.loads(read(S/'driver/package-provenance.json'))
image = json.loads(read(S/'driver/image-provenance.json'))
assert package['sourceCommit'] == C and package['sourceDirty'] is False
for key in ['binarySHA256', 'webIndexSHA256', 'archiveSHA256']:
    assert package[key] == r['package'][key]
assert image['candidateImage'] == r['daemon']['imageID'] == 'sha256:76043c5d0f30207c05343e1dbc0f450658eb52726590c8c47c714c67371acec8'
assert image['sourceCommit'] == '5f4a966f6d70264c92ee2176dde912e1b43fc3c7'
assert image['sourcesBefore'] == image['sourcesAfter'] and len(image['sourcesBefore']) == 106
for name, expected in image['sourcesBefore'].items():
    assert sha(subprocess.check_output(['git', '-C', '/workspace/luci', 'show', C+':'+name], timeout=20)) == expected
put('runtime-source-bindings.json', enc({'productSource': C, 'imageOriginalSource': image['sourceCommit'],
    'imageID': image['candidateImage'], 'imageProvenanceSHA256': before['driver/image-provenance.json']['sha256'],
    'matchingImageInputCount': 106, 'imageInputs': image['sourcesBefore'], 'helperBindings': repo_bindings,
    'passed': True, 'scope': 'Exact committed input parity; original image760 provenance remains unchanged. No image rebuild or new runtime executed by assembler.'}))
put('analysis.json', enc({'productSource': C, 'overallPassed': False, 'launcherExitCode': 1,
    'wrapperFailure': None, 'scenarioCompleted': True, 'warmRestartPassed': True, 'scenario': r['scenario'],
    'terminalStreams': [{'ordinal': row['ordinal'], 'exit': row['exit'], 'closed': row['closed']} for row in streams],
    'browserRestartCount': 1, 'pendingMutations': [], 'cleanupErrors': [],
    'cleanupPhaseOutcomes': [{'name': row['name'], 'status': row['status']} for row in r['cleanupPhases']],
    'oldResponseBeforeAcknowledgementMs': ordering['acknowledgedMonotonicMs']-ordering['releasedMonotonicMs'],
    'originalResult': before['driver/result.json'], 'originalWrapper': before['wrapper-result.json'],
    'independentClosureSHA256': sha(read(CL)),
    'scope': 'Terminal lifecycle succeeded; required installed old-response-after-ACK ordering remains unexercised. Overall acceptance remains false. No performance, Windows or live-model claim.'}))
copy(B/'README.prepared.md', 'README.md')
copy(Path(__file__), 'assemble.py')
published_paths = {str(Path(row['source']).relative_to(S)) for row in originals if Path(row['source']).is_relative_to(S)}
excluded = [{'path': name, **binding, 'scope': 'Hash-bound only; original bytes excluded from public capsule.'} for name, binding in before.items() if name not in published_paths]
put('original-bindings.json', enc({'originals': originals, 'excluded': excluded,
    'redactionPolicy': {'replacement': replacement, 'rootSHA256': sha(private.encode()), 'rootBytes': len(private.encode()), 'onlyExactRootSubstitution': True}}))
assert all_hashes() == before
for row in originals:
    assert meta(read(row['source'])) == row['original']
put('original-immutability.json', enc({'allOriginalArtifactCount': len(before), 'allOriginalArtifactsUnchanged': True,
    'originalArtifactsBefore': before, 'originalArtifactsAfter': all_hashes(), 'selectedPublicOriginalCount': len(originals),
    'scope': 'All original run files were hashed before/after; private log/error/stdout/auth contents were not exported. Independent closure separately binds unchanged private fixture files.'}))
patterns = {'private-key': rb'-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----',
    'github-token': rb'(?:gh[pousr]_[A-Za-z0-9]{30,}|github_pat_[A-Za-z0-9_]{50,})',
    'openai-token': rb'\bsk-(?:proj-)?[A-Za-z0-9_-]{30,}',
    'bearer-value': rb'(?i)bearer\s+[A-Za-z0-9_.-]{24,}',
    'url-userinfo': rb'https?://[^/@\s:]+:[^/@\s]+@'}
findings = []
for path in O.rglob('*'):
    if not path.is_file():
        continue
    raw = path.read_bytes()
    assert private.encode() not in raw
    for kind, pattern in patterns.items():
        if re.search(pattern, raw):
            findings.append({'path': str(path.relative_to(O)), 'pattern': kind})
assert not findings, findings
put('privacy-scan.json', enc({'exactPrivateRootAbsent': True, 'boundedPatternFindings': findings,
    'screenshotsPublished': False, 'excluded': 'Private logs/errors, launcher output, fixture pointer, screenshots and all fixture/auth/config contents.',
    'scope': 'Exact private-root substitution and common credential-pattern scan; not exhaustive. Screenshots are hash-bound but excluded; no visual-review claim.'}))
files = {str(path.relative_to(O)): meta(path.read_bytes()) for path in sorted(O.rglob('*')) if path.is_file()}
put('publication.json', enc({'version': 1, 'productSource': C, 'overallPassed': False, 'scenarioCompleted': True,
    'warmRestartPassed': True, 'requiredAcceptanceIncomplete': r['scenario']['requiredAcceptanceIncomplete'],
    'scope': 'Installed warm972d attempt1: two first-attempt streams, one UI Restart, natural Exit0/7 and closure; required old-response-after-ACK ordering absent. Original failure retained.', 'files': files}))
put('SHA256SUMS', (''.join(value['sha256']+'  '+name+'\n' for name, value in files.items())+sha((O/'publication.json').read_bytes())+'  publication.json\n').encode())
for line in (O/'SHA256SUMS').read_text().splitlines():
    digest, name = line.split('  ', 1)
    assert sha((O/name).read_bytes()) == digest
result = {'capsule': str(O), 'files': len(files)+2, 'publicationSHA256': sha((O/'publication.json').read_bytes()),
    'checksumsSHA256': sha((O/'SHA256SUMS').read_bytes()), 'bytes': sum(path.stat().st_size for path in O.rglob('*') if path.is_file())}
(B/'assembly-result.json').write_bytes(enc(result))
print(json.dumps(result, indent=2))
