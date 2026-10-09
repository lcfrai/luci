"""Prepare an allowlisted, deterministic publication from retained private inputs.
No application/runtime calls. Existing destination files are verified, never replaced.
"""
from pathlib import Path
import collections, hashlib, json, re, shutil

OUT = Path(__file__).resolve().parent
TOOLS = Path('/workspace/.luci-tools')
RUNS = Path('/workspace/luci/artifacts/quality-swarm')
sha = lambda p: hashlib.sha256(p.read_bytes()).hexdigest()
def write(p, data):
    p = OUT / p
    p.parent.mkdir(parents=True, exist_ok=True)
    data = data.encode() if isinstance(data, str) else data
    if p.exists():
        assert p.read_bytes() == data, f'Existing publication differs: {p.relative_to(OUT)}'
    else:
        p.write_bytes(data)
def emit(p, obj):
    write(p, json.dumps(obj, indent=2) + '\n')
def copy(source, dest):
    assert source.is_file() and not source.is_symlink()
    write(dest, source.read_bytes())

expected = {
    1: ('cce8e1fa7195525e2bf4976cee45844876cf62cf2de5d5960ee88394cb99645f', '45463172fffdaa9503bfb6bddb78e8dde13446305c60327362e7e6151974d763'),
    2: ('fead13b770f2351282fc4c9a1bd9d0ab34248b087311e9b4ccdb651ed424fe26', 'f55eb46a4fc0d6ece3a2393135fb3636aaf5b5548f7db41eea3931b50659aa3c'),
}
frozen = {}
for version, source_name in [(1, 'cloud032-layout-b94'), (2, 'cloud032-layout-b94-v2')]:
    source = TOOLS / source_name
    sources = []
    for p in sorted(source.rglob('*')):
        if not p.is_file():
            continue
        assert not p.is_symlink()
        rel = p.relative_to(source)
        copy(p, Path(f'accepted-preparation-v{version}') / rel)
        sources.append({'path': str(rel), 'bytes': p.stat().st_size, 'sha256': sha(p)})
    # Verify the old frozen manifest without modifying its bytes.
    checked = 0
    for line in (source / 'SHA256SUMS').read_text().splitlines():
        digest, rel = line.split('  ', 1)
        assert sha(source / rel) == digest
        checked += 1
    frozen[f'v{version}'] = {'source': str(source), 'manifestSHA256': sha(source/'manifest.json'), 'sumsSHA256': sha(source/'SHA256SUMS'), 'frozenEntriesVerified': checked, 'files': sources}

private_values = set()
private_identity_values = set()
raw_bindings = []
for attempt in [1, 2]:
    root = RUNS / f'layout-b94ad37e-attempt{attempt}'
    driver = root / 'driver'
    result_bytes = (driver/'result.json').read_bytes()
    trace_bytes = (driver/'layout-trace.raw.json').read_bytes()
    assert (hashlib.sha256(result_bytes).hexdigest(), hashlib.sha256(trace_bytes).hexdigest()) == expected[attempt]
    result = json.loads(result_bytes)
    for raw in [result_bytes, trace_bytes]:
        # Exact private fixture identifiers plus credential-looking values, never emitted.
        private_identity_values.update(x.decode() for x in re.findall(rb'\b(?:p-[a-f0-9]{8}|run-[a-f0-9]{24}|queue-[a-f0-9]{24}|user-[A-Za-z0-9_-]{22})\b', raw))
        private_values.update(x.decode() for x in re.findall(rb'Bearer ([A-Za-z0-9_./+~-]{16,})', raw))
    def inspect(value):
        if isinstance(value, dict):
            for key, item in value.items():
                if re.fullmatch(r'(?:access_?token|refresh_?token|password|authorization|cookie|set-cookie|secret)', key, re.I) and isinstance(item, str) and len(item) >= 8:
                    private_values.add(item)
                inspect(item)
        elif isinstance(value, list):
            for item in value: inspect(item)
    inspect(result)
    inspect(json.loads(trace_bytes))
    prefix = Path(f'attempt{attempt}')
    for name in ['wrapper-result.json', 'dependency-before.json', 'dependency-after-wrapper.json']:
        copy(root/name, prefix/name)
    copy(driver/'layout-trace-capture.json', prefix/'layout-trace-capture.json')
    analysis = TOOLS/f'cloud032-layout-b94-attempt{attempt}-analysis'
    for p in sorted(analysis.iterdir()):
        if p.is_file(): copy(p, prefix/'analysis'/p.name)
    if attempt == 2:
        copy(driver/'layout-trace-projection.json', prefix/'layout-trace-projection.json')
    ids = list(dict.fromkeys(x['projectID'] for x in result['interactions']))
    interactions = [{**{k:v for k,v in x.items() if k != 'projectID'}, 'index':i, 'projectOrdinal':ids.index(x['projectID'])} for i,x in enumerate(result['interactions'])]
    emit(prefix/'all-interactions.json', {'rawResultSHA256':expected[attempt][0], 'projectIdentityMapping':'First-appearance ordinals; actual fixture identities excluded', 'rows':interactions})
    # Only reviewed, non-content evidence groups; all raw result/trace bytes remain private.
    selected = ['passed','diagnosticOnly','fullSustainedAcceptance','startedAt','finishedAt','sourceCommit','sourceDirty','harnessCheckout','externalSourceBinding','packageIntegrity','packageIntegrityAfter','binarySHA256','webIndexSHA256','driverSHA256','helperSHA256','reusedHelperSHA256','browserVersion','checks','externalModelCalls','cloudResourcesCreated','providerCalls','auxiliaryCalls','actualTaskCount','controllersStopped','dockerRequestsDrained','dockerMutationStateCertain','dockerTransportNotStarted','runtimeCleanup','fixtureRemoved','fixtureRetained','workflowOperationUnsettled','originalReceiptComplete','signal','interactionSummary','latencyTargets','targetComparison','elapsedMs','totalElapsedMs']
    safe = {k:result[k] for k in selected if k in result}
    safe['rawResultSHA256'] = expected[attempt][0]
    safe['rawResultPrivate'] = True
    safe['privateTraceSHA256'] = expected[attempt][1]
    safe['dockerTransportCounts'] = result['dockerTransportObservations']['counts']
    safe['failedStage'] = result.get('failedStage')
    safe['failureCode'] = (result['layoutTraceCapture'].get('failure') or {}).get('code')
    emit(prefix/'result-allowlisted.json', safe)
    emit(prefix/'all-browser-timing.json', {k:result[k] for k in ['latencyDiagnostics','browserEventWindows','browserTimingLedger','autonomousReadiness']})
    raw_bindings.append({'attempt':attempt,'privateResult':str(driver/'result.json'),'privateResultSHA256':expected[attempt][0],'privateResultBytes':len(result_bytes),'privateTrace':str(driver/'layout-trace.raw.json'),'privateTraceSHA256':expected[attempt][1],'privateTraceBytes':len(trace_bytes),'publicRawTrace':False,'publicRawResult':False})

emit('source-bindings.json', {'frozenPreparations':frozen,'privateInputs':raw_bindings,'scope':'Preparations are immutable historical snapshots. Their readiness runtimeLaunched fields predate each actual run; live results are separate.'})
# Exact known values scanned without publishing them. Allowlisting, rather than this
# finite scan, is the primary privacy boundary. Source-code credential identifiers
# are expected and are not captured credential values.
scan_files = [p for p in OUT.rglob('*') if p.is_file() and p.name not in ['privacy-review.json','manifest.json','SHA256SUMS']]
matches = []
for p in scan_files:
    raw = p.read_bytes()
    for kind, values in [('captured-credential', private_values), ('private-fixture-identity', private_identity_values)]:
        if any(v.encode() in raw for v in values): matches.append({'path':str(p.relative_to(OUT)),'kind':kind})
assert not matches, matches
emit('privacy-review.json', {'scope':'Allowlisted metadata/timings/source positions; raw trace/result excluded. Known captured value scan is finite, not proof of universal secret absence. No headers/cookies/account/private response content intentionally exported. Source code contains credential handling, not captured payloads.','privateInputCount':4,'credentialValuesChecked':len(private_values),'fixtureIdentityValuesChecked':len(private_identity_values),'publicFilesScanned':len(scan_files),'exactKnownValueMatches':matches,'rawPrivateInputs':raw_bindings})
print(json.dumps({'publicFilesBeforeFinalManifest':len(scan_files)+1,'rawInputsHashBound':len(raw_bindings),'exactKnownValueMatches':len(matches)}))
