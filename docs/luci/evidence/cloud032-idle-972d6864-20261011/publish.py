#!/usr/bin/env python3
"""Publish retained idle evidence only; no runtime operations or package/archive reads."""
from pathlib import Path
import argparse, datetime, gzip, hashlib, json, re, subprocess

p = argparse.ArgumentParser()
for name in ['original-run', 'timing', 'closure', 'preparation', 'prelaunch', 'helper-review', 'output']:
    p.add_argument('--' + name, required=True, type=Path)
a = p.parse_args()
BASE, TIMING, CLOSURE, PREP, OUT = a.original_run, a.timing, a.closure, a.preparation, a.output
R = Path('/workspace/luci')
PRODUCT = '972d6864a30b742b12545c8e12f4ca3ae2faa884'
RAW_HASH = '5f25447b1f6dc7eca50e220f358a4a029bd311e7d368e15ec85d693198569a00'
assert not OUT.exists(), 'Refuse to overwrite a frozen capsule'
OUT.mkdir(parents=True)
sha = lambda b: hashlib.sha256(b).hexdigest()
meta = lambda path: {'sha256': sha(path.read_bytes()), 'bytes': path.stat().st_size}
copies, projections = [], []

def put(name, contents):
    path = OUT / name; path.parent.mkdir(parents=True, exist_ok=True); path.write_bytes(contents)

def js(name, value):
    put(name, (json.dumps(value, indent=2) + '\n').encode())

def cp(path, name, origin):
    put(name, path.read_bytes())
    copies.append({'member': name, 'originLabel': origin, 'original': meta(path), 'transformation': 'none'})

def git(path, commit):
    return subprocess.check_output(['git', 'show', commit + ':' + path], cwd=R, timeout=20)

raw = (BASE / 'driver/result.json').read_bytes(); assert sha(raw) == RAW_HASH
d = json.loads(raw)
inv = json.loads((BASE / 'invocation.json').read_text())
wrapper = json.loads((BASE / 'wrapper-result.json').read_text())
pkg = json.loads((BASE / 'driver/package-provenance.json').read_text())
binding = json.loads((BASE / 'driver/accepted-harness/external-source-binding.json').read_text())
closure = json.loads((CLOSURE / 'independent-closure.json').read_text())
image = json.loads((BASE / 'driver/image-provenance.json').read_text())
helper_commit = inv['helperSource']
assert d['sourceCommit'] == inv['productSource'] == pkg['sourceCommit'] == PRODUCT
assert d['sourceDirty'] is False and pkg['sourceDirty'] is False
assert d['harnessCheckout']['commit'] == helper_commit and d['harnessCheckout']['dirty'] is False
assert d['externalSourceBinding'] == binding and wrapper['rawResultSHA256'] == RAW_HASH
assert wrapper['exitCode'] == 0 and not wrapper['externalMonitorTimeout'] and not wrapper['forcedExactDriver']
assert d['passed'] and d['idleDiagnosticCompleted'] and d['originalReceiptComplete']
assert d['diagnosticOnly'] and not d['fullSustainedAcceptance']
assert d['providerCalls'] == d['auxiliaryCalls'] == d['externalModelCalls'] == d['actualTaskCount'] == 0
assert not d['targetComparison']['project-switch']['withinReference'] and d['targetComparison']['input']['withinReference']
assert d['binarySHA256'] == pkg['binarySHA256'] and d['webIndexSHA256'] == pkg['webIndexSHA256']
assert closure['originalResultSHA256'] == RAW_HASH and closure['allOriginalArtifactsStable']
assert closure['originalArtifactsBefore'] == closure['originalArtifactsAfter']

# Keep the already-frozen timing capsule byte-identical; no reanalysis.
timing_names = (TIMING / 'ARTIFACTS.txt').read_text().splitlines()
assert len(timing_names) == 13
assert sorted(timing_names) == sorted(path.name for path in TIMING.iterdir() if path.is_file())
for line in (TIMING / 'SHA256SUMS').read_text().splitlines():
    expected, name = line.split('  ', 1); assert sha((TIMING / name).read_bytes()) == expected
for name in timing_names:
    cp(TIMING / name, 'timing/' + name, 'Frozen timing capsule/' + name)
timings = json.loads(gzip.decompress((TIMING / 'timings.all432.projection.json.gz').read_bytes()))
assert timings['originalResultSHA256'] == RAW_HASH and len(timings['interactions']) == 432
assert len(timings['browserLatencyRows']) == 3054 and len(timings['browserEventTimingRows']) == 19524

originals = {}
for key, rel in {'result': 'driver/result.json', 'invocation': 'invocation.json', 'wrapper': 'wrapper-result.json', 'driverIdentity': 'driver-identity.json', 'journal': 'driver/diagnostic-journal.jsonl', 'imageProvenance': 'driver/image-provenance.json', 'packageProvenance': 'driver/package-provenance.json', 'binding': 'driver/accepted-harness/external-source-binding.json', 'stdout': 'driver.stdout.log', 'stderr': 'driver.stderr.log'}.items():
    originals[key] = {'relativeOriginalMember': rel, **meta(BASE / rel)}
    assert {k: originals[key][k] for k in ['sha256', 'bytes']} == closure['originalArtifactsBefore'][rel]
originals['privateTrace'] = {'relativeOriginalMember': 'driver/action-only-trace.zip', **closure['originalArtifactsBefore']['driver/action-only-trace.zip'], 'hashOrigin': 'Independent closure receipt, not rehashed during publication.'}
originals['closure'] = meta(CLOSURE / 'independent-closure.json')
originals['prelaunchProcessSurvey'] = meta(a.prelaunch)
assert originals['prelaunchProcessSurvey']['sha256'] == closure['processSurvey']['prelaunchSHA256']

# All 17 accepted source files, including flattened imported dependencies.
accepted, imported, templates = {}, [], {}
for name, expected in {**binding['localSources'], **binding['repositoryDependencies']}.items():
    filename = name.removeprefix('scripts/acceptance/').replace('/', '_') if name.startswith('scripts/') else name
    source = BASE / 'driver/accepted-harness' / filename; contents = source.read_bytes()
    assert sha(contents) == expected
    member = 'sources/accepted/' + filename + '.gz'
    put(member, gzip.compress(contents, mtime=0))
    accepted[name] = {'sha256': expected, 'bytes': len(contents), 'member': member, 'acceptedOriginalMember': 'driver/accepted-harness/' + filename}
    if name.startswith('scripts/'):
        assert git(name, PRODUCT) == git(name, helper_commit) == contents
        imported.append({'path': name, 'sha256': expected, 'bytes': len(contents), 'productCommit': PRODUCT, 'helperCommit': helper_commit})
assert len(accepted) == 17 and len(imported) == 7
for path, expected in inv['sourceBinding']['sources'].items():
    contents = git(path, PRODUCT)
    assert contents == git(path, helper_commit)
    assert {'sha256': sha(contents), 'bytes': len(contents)} == expected
    filename = path.removeprefix('scripts/acceptance/sustained-four-project/') if path.startswith('scripts/acceptance/sustained-four-project/') else path.removeprefix('scripts/acceptance/').replace('/', '_')
    member = 'sources/template/' + filename + '.gz'; put(member, gzip.compress(contents, mtime=0))
    templates[path] = {**expected, 'member': member, 'productCommit': PRODUCT, 'helperCommit': helper_commit}
assert len(templates) == 17
assert meta(PREP / 'launch.py')['sha256'] == inv['launcherSHA256']
assert json.loads((PREP / 'template-capture.json').read_text()) == inv['sourceBinding']
for name in ['launch.py', 'prepare-idle.py', 'template-capture.json', 'idle-extraction.patch', 'idle-extraction.json']:
    cp(PREP / name, 'sources/preparation/' + name, 'Recorded idle preparation/' + name)
cp(CLOSURE / 'closure.py', 'closure/closure.py', 'Independent read-only closure helper')
assert meta(CLOSURE / 'closure.py')['sha256'] == closure['closureScriptSHA256']
cp(a.prelaunch, 'closure/prelaunch-process-survey.original.json', 'Recorded prelaunch selected process-name survey')
cp(a.helper_review, 'sources/accepted-helper-review.json', 'Independent accepted helper and Git-object review')
cp(Path(__file__), 'publish.py', 'This offline publication helper')

def arg_value(flag):
    return inv['command'][inv['command'].index(flag) + 1]

replacements = {str(BASE): '<RUN_DIRECTORY>', str(pkg['packageDir']): '<EXTRACTED_PACKAGE>', pkg['archive']: '<PACKAGE_ARCHIVE>', binding['executionDirectory']: '<ACCEPTED_HELPER_EXECUTION_DIRECTORY>', arg_value('--fixture-root'): '<FRESH_FIXTURE_PARENT>', arg_value('--provenance'): '<PACKAGE_PROVENANCE_JSON>', arg_value('--image-provenance'): '<IMAGE_PROVENANCE_JSON>', str(a.prelaunch): '<PRELAUNCH_PROCESS_SURVEY>', closure['adaptedFrom']['path']: '<PRIOR_CLOSURE_HELPER>'}

def sanitize(value):
    if isinstance(value, dict): return {k: sanitize(v) for k, v in value.items()}
    if isinstance(value, list): return [sanitize(v) for v in value]
    if isinstance(value, str):
        for old, new in sorted(replacements.items(), key=lambda pair: -len(pair[0])):
            value = value.replace(old, new)
    return value

def project(name, source_label, original, value, scope):
    js(name, {'original': original, 'representation': scope, 'value': sanitize(value)})
    projections.append({'member': name, 'originLabel': source_label, 'original': original, 'transformation': scope})

project('runtime/invocation.sanitized.json', 'Original invocation', originals['invocation'], inv, 'Task-specific execution/package/archive/report/fixture-parent paths replaced; original command, bounds and source identity otherwise retained.')
project('runtime/source-binding.sanitized.json', 'Accepted original source binding', originals['binding'], binding, 'External execution directory replaced; all declared hashes and extraction relationships retained.')
project('runtime/package-provenance.sanitized.json', 'Original package provenance', originals['packageProvenance'], pkg, 'Package/archive locations replaced; source, sizes and recorded hashes retained. No archive or binary rehash.')
project('closure/independent-closure.sanitized.json', 'Independent read-only closure', originals['closure'], closure, 'Prelaunch/prior-helper path labels replaced; all recorded PID/start observations, process survey, inventories, hashes and limitations retained. Closure was not rerun.')
for rel, member in [('wrapper-result.json', 'runtime/wrapper-result.original.json'), ('driver-identity.json', 'runtime/driver-identity.original.json'), ('driver/diagnostic-journal.jsonl', 'runtime/diagnostic-journal.original.jsonl'), ('driver.stderr.log', 'runtime/driver.stderr.original.log')]:
    cp(BASE / rel, member, 'Original run/' + rel)
stdout = (BASE / 'driver.stdout.log').read_text()
put('runtime/driver.stdout.sanitized.log', sanitize(stdout).encode())
projections.append({'member': 'runtime/driver.stdout.sanitized.log', 'originLabel': 'Original driver stdout', 'original': originals['stdout'], 'transformation': 'Final raw result path replaced; PASS messages and all other characters unchanged.'})

# Runtime-image source parity uses immutable Git objects, never image archives.
assert image['sourcesBefore'] == image['sourcesAfter'] and len(image['sourcesBefore']) == 106
for path, expected in image['sourcesBefore'].items(): assert sha(git(path, PRODUCT)) == expected
assert image['candidateImage'] == d['image']['id'] == closure['imageID']
js('runtime/image-source-bindings.json', {'original': originals['imageProvenance'], 'recordedImageBuildCommit': image['sourceCommit'], 'candidateImage': image['candidateImage'], 'matchedProductSourceCommit': PRODUCT, 'runtimeSourceInputs': 106, 'sourcesBefore': image['sourcesBefore'], 'sourcesAfter': image['sourcesAfter'], 'scope': 'All 106 retained runtime source hashes match immutable product Git objects; source reuse is verified. No image build/export or image/archive payload hashing during publication.'})

# Explicit result allowlist, with runtime/cleanup assertions labelled as assertions.
keys = ['passed', 'diagnosticOnly', 'fullSustainedAcceptance', 'sourceCommit', 'sourceDirty', 'harnessCheckout', 'finalHarnessCheckout', 'dependencyCheckout', 'packageIntegrity', 'binarySHA256', 'webIndexSHA256', 'driverSHA256', 'helperSHA256', 'reusedHelperSHA256', 'image', 'releaseImage', 'bounds', 'browserVersion', 'browserErrors', 'providerCalls', 'auxiliaryCalls', 'externalModelCalls', 'actualTaskCount', 'controllersStopped', 'dockerRequestsDrained', 'dockerMutationStateCertain', 'cleanup', 'cleanupPhases', 'startupResources', 'runtimeCleanup', 'fixtureRemoved', 'idleDiagnosticCompleted', 'originalReceiptComplete', 'elapsedMs', 'totalElapsedMs', 'signal', 'finishedAt', 'interactionSummary', 'targetComparison', 'latencyTargets', 'workflowOperationUnsettled', 'engineIdentitySHA256']
assert all(key in d for key in keys)
projection = {key: d[key] for key in keys}
projection['dockerTransportSummary'] = {key: d['dockerTransportObservations'][key] for key in ['counts', 'dropped', 'failures', 'cleanup']}
assert d['browserErrors'] == d['cleanup'] == d['dockerTransportObservations']['failures'] == []
project('result-summary.projection.json', 'Original private completed driver result', originals['result'], projection, 'Explicit allowlist of exact original outcome, per-cohort quantiles, target decisions, source/package/runtime identities and cleanup assertions. Dynamic fixture/project identifiers, request paths/bodies and raw fixture content omitted. A projection is not reconstruction of the entire original.')
js('source-bindings.json', {'productSourceCommit': PRODUCT, 'helperSourceCommit': helper_commit, 'originals': originals, 'acceptedHelpers': accepted, 'acceptedHelperCount': 17, 'importedDependencies': imported, 'templateInputs': templates, 'scope': '17 accepted members include seven repository imports; 17 template inputs overlap and are not 34 independent helpers. Exact immutable Git comparisons; no current dirty-tree claim. Original raw result and private trace remain distinct from published projections.'})
js('original-bindings.json', {'exactCopies': copies, 'projections': projections, 'scope': 'Only declared publication members are listed as copies/projections. Original private result/trace/package/image payloads are hash-bound or projected, never copied wholesale.'})

survey = closure['processSurvey']
primary = d['interactionSummary']['idle-interactive-calibration:project-switch']
inp = d['interactionSummary']['idle-interactive-calibration:input']
reset = d['interactionSummary']['idle-interactive-calibration:draft-reset']
readme = f'''# CLOUD-032: installed idle diagnostic at 972d6864

**Project switching fails the unchanged primary p95 target; input passes. CLOUD-032 remains open.** Product/package and clean helper source were `{PRODUCT}`. The original wrapper exited 0 in {wrapper['elapsedSeconds']:.3f} seconds. All 400 primary and 32 separate follow-up actions succeeded. `passed=true` and `idleDiagnosticCompleted=true` describe successful bounded diagnostic capture, not performance or product acceptance.

| Primary cohort | Count | p50 | p95 | p99 | Target decision |
|---|---:|---:|---:|---:|---|
| Project switch | 100 | {primary['p50Ms']:.6f} ms | **{primary['p95Ms']:.6f} ms** | {primary['p99Ms']:.6f} ms | p95 ≤ 300 ms: failed |
| Input | 200 | {inp['p50Ms']:.6f} ms | **{inp['p95Ms']:.6f} ms** | {inp['p99Ms']:.6f} ms | p95 ≤ 100 ms: passed |
| Draft reset | 100 | {reset['p50Ms']:.6f} ms | {reset['p95Ms']:.6f} ms | {reset['p99Ms']:.6f} ms | Separate cohort; no declared p95 target |

Switch/input p99 are strictly below 1000 ms. The separate eight-switch follow-up has p95/maximum 251.833108 ms and cannot upgrade the original primary result. [Frozen timing review](timing/README.md) retains all 432 actions, 3,054 browser latency rows and 19,524 EventTiming rows. Its 13 files were copied exactly with their original checksums; no timing was recomputed for publication.

Nine primary switches exceed 300 ms. Their largest recorded Node span is the All projects command in six and context verification in three. Browser associations support post-dispatch observations but cannot divide the whole Node command before/after the pointer or attribute delay to a product function. Associated specification resources in the three verification-tail cases take 14–28 ms; slow permission/network reads are not established. No causal improvement, overhead subtraction, excluded outlier, physical-paint or React-commit claim is made.

This plain idle attempt created/started zero workers and made zero provider, auxiliary or external-model calls. Docker readiness/inventory reads occurred. No counter adjunct or owned-process resource sampler ran. It is not sustained workload, model-quality, Windows, native-product acceptance or whole-computer responsiveness evidence.

The original private result is {len(raw):,} bytes, SHA-256 `{RAW_HASH}`. `result-summary.projection.json` retains exact allowlisted original outcome/target/runtime fields; the compressed timing projection retains all action measurements. These public projection hashes differ from the original and do not reconstruct omitted private fields. The raw trace remains hash-bound through the independent closure receipt, unpublished and unused for explaining primary failures.

`source-bindings.json` binds 17 accepted helpers, including seven imported repository dependencies, and 17 overlapping full-template inputs to immutable product/helper Git objects. Accepted helper bytes are frozen under `sources/accepted/`; source-template bytes and exact generator, launcher, extraction patch and capture metadata are included. Invocation, wrapper, driver identity, diagnostic journal and stdout/stderr are included; only explicitly labelled paths are sanitized. Source/script provenance locations are ordinary metadata, not private fixture instances. `runtime/image-source-bindings.json` verifies all 106 unchanged runtime source inputs against the product commit; the image build originally used its separately recorded revision. Package/image archive hashes are retained verification identities, not new payload audits. No giant archive was read.

The wrapper explicitly retains `cleanupUnconfirmedByWrapper=true`. The driver asserts controller/proxy settlement, five closed startup handles, empty cleanup errors and fixture removal. The independent read-only closure observed the exact recorded driver/controller PID/start identities absent, empty containers/volumes on the same exclusive engine and unchanged image identity. Its selected comm-name survey changed from {survey['prelaunchCount']} to {survey['currentCount']}, with {len(survey['newZombies'])} new Chromium zombie identities. Those survey differences are **unattributed**: no owned-browser PID/start census was captured. Browser absence and whole-machine closure are not established. Fixture removal remains a driver assertion. Prelaunch survey and closure helper/receipt are included without rerunning them or touching any failed fixture or ledger.

Reproduction requires the exact source/package/image and an exclusive empty Linux Docker environment, with archived preparation paths adapted to fresh locations. The launcher records fixed bounds and zero-Create/Start guards. This publication does not authorize execution against shared Windows Docker or change existing tester resource gates. The offline `publish.py` accepts original paths as arguments and verifies copies, projections and accepted source hashes; it performs no app/test/runtime action. All publication files are declared in `ARTIFACTS.txt`, `publication.json` and `SHA256SUMS`.
'''
put('README.md', readme.encode())

# Verify copied and compressed members, all original bindings used, and privacy exclusions.
for row in copies: assert meta(OUT / row['member']) == row['original']
for row in accepted.values():
    contents = gzip.decompress((OUT / row['member']).read_bytes())
    assert sha(contents) == row['sha256'] and len(contents) == row['bytes']
assert (BASE / 'driver/result.json').read_bytes() == raw
private_ids = {row['projectID'] for row in d['interactions']}
for path in OUT.rglob('*'):
    if not path.is_file(): continue
    contents = gzip.decompress(path.read_bytes()) if path.suffix == '.gz' else path.read_bytes()
    assert not any(value.encode() in contents for value in private_ids), str(path.relative_to(OUT))
    assert not re.search(rb'/workspace/\.c32/luci-sustained-[A-Za-z0-9_-]+', contents), str(path.relative_to(OUT))
js('privacy-review.json', {'scope': 'Published metadata, exact source helpers, sanitized execution receipts and frozen strict-allowlist timing projection.', 'checks': ['Known original project IDs absent from every plain/decompressed member.', 'No concrete retained fresh-fixture instance path.', 'No request URL/body/header payloads or credential values copied.', 'Stdout path sanitization explicit; stderr empty.', 'All log and manifest members claimed as published are present.'], 'limits': 'Source code contains variable names and generic fixture path templates. No private fixture contents, failed ledger, archive or credential file was inspected.'})
paths = sorted(str(path.relative_to(OUT)) for path in OUT.rglob('*') if path.is_file())
all_paths = sorted(paths + ['ARTIFACTS.txt', 'publication.json', 'SHA256SUMS'])
put('ARTIFACTS.txt', ('\n'.join(all_paths) + '\n').encode())
js('publication.json', {'productSourceCommit': PRODUCT, 'helperSourceCommit': helper_commit, 'scope': 'Successful installed idle diagnostic capture, failed primary switching p95, passing input p95; no product acceptance or causal attribution.', 'originalResult': originals['result'], 'primaryActions': 400, 'separateFollowupActions': 32, 'acceptedHelperMembers': 17, 'importedRepositoryDependencies': 7, 'templateInputs': 17, 'runtimeImageSourceInputs': 106, 'timingRecomputed': False, 'giantArchiveRead': False, 'members': {name: meta(OUT / name) for name in sorted(paths + ['ARTIFACTS.txt'])}, 'manifestConvention': 'Members exclude publication.json and root SHA256SUMS; sums bind publication.json and every other declared member. Nested original manifests remain byte-identical.'})
put('SHA256SUMS', ''.join(f'{meta(OUT / name)["sha256"]}  {name}\n' for name in all_paths if name != 'SHA256SUMS').encode())
assert sorted(str(path.relative_to(OUT)) for path in OUT.rglob('*') if path.is_file()) == all_paths
for line in (OUT / 'SHA256SUMS').read_text().splitlines():
    expected, name = line.split('  ', 1); assert meta(OUT / name)['sha256'] == expected
print(json.dumps({'capsule': str(OUT), 'files': len(all_paths), 'bytes': sum((OUT / name).stat().st_size for name in all_paths), 'publicationSHA256': meta(OUT / 'publication.json')['sha256'], 'sumsSHA256': meta(OUT / 'SHA256SUMS')['sha256'], 'originalRawUnchanged': sha((BASE / 'driver/result.json').read_bytes()) == RAW_HASH, 'allCapsuleHashesVerified': True}, indent=2))
