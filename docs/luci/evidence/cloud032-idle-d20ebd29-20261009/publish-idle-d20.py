from pathlib import Path
import gzip,hashlib,json,subprocess

ROOT=Path('/workspace/luci')
BASE=ROOT/'artifacts/quality-swarm/idle-d20ebd29-counter-attempt1'
RUN=BASE/'driver'
PREP=Path('/workspace/.luci-tools/cloud032-idle-67a5b7c7')
ANALYSIS=Path('/workspace/.luci-tools/cloud032-idle-d20ebd29-analysis')
PUBLIC=Path('/workspace/.luci-tools/publish-idle-d20/docs/luci/evidence')
OUT=PUBLIC/'cloud032-idle-d20ebd29-20261009'
PRODUCT='d20ebd292f3f1de04d6726bab0998c210987bcb5'
HELPER='c625ef0b7842748f4ce74cd3ffc08979d571bddf'
assert not OUT.exists(), 'Refusing to overwrite frozen capsule'
OUT.mkdir(parents=True)
sha=lambda b:hashlib.sha256(b).hexdigest()
def meta(p):
    b=p.read_bytes()
    return {'sha256':sha(b),'bytes':len(b)}
copied=[]
def put(name,data):
    p=OUT/name;p.parent.mkdir(parents=True,exist_ok=True);p.write_bytes(data)
def js(name,value):put(name,(json.dumps(value,indent=2)+'\n').encode())
def cp(source,name):
    data=source.read_bytes();put(name,data)
    copied.append({'member':name,'originalPath':str(source),'original':{'sha256':sha(data),'bytes':len(data)},'transformation':'none'})
def git(path,commit):return subprocess.check_output(['git','show',commit+':'+path],cwd=ROOT)

raw=(RUN/'result.json').read_bytes();d=json.loads(raw)
inv=json.loads((BASE/'invocation.json').read_text())
pkg=json.loads((RUN/'package-provenance.json').read_text())
sb=json.loads((RUN/'accepted-harness/external-source-binding.json').read_text())
wrapper=json.loads((BASE/'wrapper-result.json').read_text())
a=json.loads((ANALYSIS/'analysis.json').read_text())
projected=json.loads(gzip.decompress((ANALYSIS/'timings.projection.json.gz').read_bytes()))
assert sha(raw)=='da1cd7eb46948775af3d5641389940776a4c75f34c1bc71b42bef9ce42c79a77'
assert d['sourceCommit']==inv['productSource']==pkg['sourceCommit']==PRODUCT
assert inv['helperSource']==d['harnessCheckout']['commit']==HELPER
assert pkg['sourceDirty'] is False and d['harnessCheckout']['dirty'] is False
assert d['externalSourceBinding']==sb and wrapper['rawResultSHA256']==sha(raw)
assert a['originalResultSHA256']==projected['originalResultSHA256']==sha(raw)
assert len(projected['interactions'])==432 and len(projected['browserLatencyRows'])==3056
assert d['diagnosticOnly'] is True and d['fullSustainedAcceptance'] is False
assert d['providerCalls']==d['actualTaskCount']==d['externalModelCalls']==0
assert all(row['success'] is True for row in projected['interactions'])
assert d['targetComparison']['project-switch']['withinReference'] is False and d['targetComparison']['input']['withinReference'] is True

original_paths={'result':RUN/'result.json','trace':RUN/'action-only-trace.zip','invocation':BASE/'invocation.json','wrapper':BASE/'wrapper-result.json','driverIdentity':BASE/'driver-identity.json','journal':RUN/'diagnostic-journal.jsonl','image':RUN/'image-provenance.json','package':RUN/'package-provenance.json','binding':RUN/'accepted-harness/external-source-binding.json','analysis':ANALYSIS/'analysis.json','closure':ANALYSIS/'independent-closure.json','traceProjection':ANALYSIS/'trace.projection.json','timingProjection':ANALYSIS/'timings.projection.json.gz'}
originals={key:meta(path) for key,path in original_paths.items()}
for name in ['analysis.json','trace.projection.json','timings.projection.json.gz','independent-closure.json','analyze.py','closure.py']:
    cp(ANALYSIS/name,name)
for source,name in [(BASE/'wrapper-result.json','wrapper-result.original.json'),(BASE/'driver-identity.json','driver-identity.original.json'),(RUN/'diagnostic-journal.jsonl','diagnostic-journal.original.jsonl'),(BASE/'driver.stdout.log','driver.stdout.original.log'),(BASE/'driver.stderr.log','driver.stderr.original.log')]:cp(source,name)

helpers={};templates={};git_bindings=[]
for name,expected in {**sb['localSources'],**sb['repositoryDependencies']}.items():
    filename=name.removeprefix('scripts/acceptance/').replace('/','_') if name.startswith('scripts/') else name
    source=RUN/'accepted-harness'/filename
    data=source.read_bytes();assert sha(data)==expected and data==(PREP/'actual'/filename).read_bytes()
    member='sources/accepted/'+filename+'.gz';put(member,gzip.compress(data,mtime=0))
    helpers[name]={'sha256':sha(data),'bytes':len(data),'member':member}
    if name.startswith('scripts/'):
        assert git(name,HELPER)==data
        git_bindings.append({'path':name,'commit':HELPER,'sha256':sha(data),'bytes':len(data),'scope':'Imported repository dependency, identical to accepted flattened copy'})
for path,row in inv['sourceBinding']['sources'].items():
    filename=path.removeprefix('scripts/acceptance/sustained-four-project/') if path.startswith('scripts/acceptance/sustained-four-project/') else path.removeprefix('scripts/acceptance/').replace('/','_')
    source=PREP/'original'/filename;data=source.read_bytes()
    assert meta(source)==row and git(path,PRODUCT)==data and git(path,HELPER)==data
    member='sources/template/'+filename+'.gz';put(member,gzip.compress(data,mtime=0))
    templates[path]={'sha256':sha(data),'bytes':len(data),'member':member,'productCommit':PRODUCT,'helperCommit':HELPER}
assert len(helpers)==len(templates)==17 and len(sb['repositoryDependencies'])==7
for name in ['prepare-idle.py','launch.py','idle-extraction.patch','idle-extraction.json','template-capture.json','current-template-vs-990.patch','preparation-check-attempt1.json','preparation-check-initial.py','preparation-verification.json','verify-preparation.py','README.md']:
    cp(PREP/name,'sources/preparation/'+name)
cp(Path('/workspace/.luci-tools/run-idle-d20-with-counter.py'),'sources/run-idle-d20-with-counter.py')
cp(Path(__file__),'publish-idle-d20.py')

replacements={str(PREP):'<EXTERNAL_DIAGNOSTIC_DIRECTORY>',str(BASE):'<RUN_DIRECTORY>',str(pkg['packageDir']):'<EXTRACTED_PACKAGE>',pkg['archive']:'<PACKAGE_ARCHIVE>',str(BASE.parent/'candidate-d20ebd29/package/package-provenance.json'):'<PACKAGE_PROVENANCE_JSON>',inv['command'][inv['command'].index('--fixture-root')+1]:'<PRIVATE_FRESH_FIXTURE_PARENT>'}
def sanitize(value):
    if isinstance(value,dict):return {k:sanitize(v) for k,v in value.items()}
    if isinstance(value,list):return [sanitize(v) for v in value]
    if isinstance(value,str):
        for old,new in sorted(replacements.items(),key=lambda x:-len(x[0])):value=value.replace(old,new)
    return value
js('invocation.sanitized.json',{'original':originals['invocation'],'representation':'Filesystem locations replaced; source, commands, hashes and budgets retained.','invocation':sanitize(inv)})
js('source-binding.sanitized.json',{'original':originals['binding'],'representation':'External execution location replaced; hashes and extraction contracts unchanged.','binding':sanitize(sb)})
js('package-provenance.sanitized.json',{'original':originals['package'],'representation':'Archive/extracted paths replaced; recorded verification, hashes and sizes retained.','provenance':sanitize(pkg)})
image=json.loads((RUN/'image-provenance.json').read_text())
assert len(image['sourcesBefore'])==len(image['sourcesAfter'])==104 and image['sourcesBefore']==image['sourcesAfter']
for path,expected in image['sourcesBefore'].items():assert sha(git(path,PRODUCT))==expected
js('image-source-bindings.json',{'original':originals['image'],'candidateImage':image['candidateImage'],'sourcesBefore':image['sourcesBefore'],'sourcesAfter':image['sourcesAfter'],'matchedProductSourceCommit':PRODUCT,'scope':'Retained image receipt with 104 exact Git-source comparisons; no rebuild or direct image-byte hashing during publication.'})
keys=['passed','diagnosticOnly','fullSustainedAcceptance','sourceCommit','sourceDirty','harnessCheckout','finalHarnessCheckout','dependencyCheckout','packageIntegrity','binarySHA256','webIndexSHA256','driverSHA256','helperSHA256','reusedHelperSHA256','image','bounds','lifecycle','browserVersion','browserErrors','providerCalls','auxiliaryCalls','externalModelCalls','actualTaskCount','controllersStopped','dockerRequestsDrained','dockerMutationStateCertain','cleanup','cleanupPhases','startupResources','runtimeCleanup','fixtureRemoved','idleDiagnosticCompleted','originalReceiptComplete','elapsedMs','totalElapsedMs','signal','finishedAt','targetComparison','latencyTargets','workflowOperationUnsettled']
assert all(key in d for key in keys)
js('result-summary.projection.json',{'original':originals['result'],'scope':'Diagnostic completion is not latency-target or sustained acceptance. Omitted raw fields remain private.','result':{key:d[key] for key in keys}})
js('source-bindings.json',{'sourceCommit':PRODUCT,'helperCommit':HELPER,'originals':originals,'acceptedHelpers':helpers,'templates':templates,'importedRepositoryDependencies':git_bindings,'selectedImmutableGitSourceBytesMatch':True,'currentWholeCheckoutClaimed':False,'packageHashesAreRecordedVerificationNotNewArchiveAudit':True,'allOriginalNumericRowsRetained':432,'browserLatencyRowsRetained':3056,'originalRawTraceIncluded':False,'scope':'17 accepted members include seven imported repository dependencies; the 17 template inputs overlap, not 34 independent helpers. Exact repository source comparisons use immutable Git objects, not a concurrently edited checkout.'})
js('original-bindings.json',{'files':copied,'scope':'Exact published copies and source locations. Raw private result/trace/image/package originals are separately hash-bound or projected, not copied in full.'})

switch=a['cohorts']['idle-interactive-calibration:project-switch'];inp=a['cohorts']['idle-interactive-calibration:input'];reset=a['cohorts']['idle-interactive-calibration:draft-reset']
follow_switch=a['cohorts']['idle-actionability-trace-followup:project-switch'];follow_input=a['cohorts']['idle-actionability-trace-followup:input']
README=f'''# CLOUD-032: installed idle diagnostic at d20ebd29

**Project switching fails the primary p95 target; input passes. CLOUD-032 remains open.** Installed product/package was `{PRODUCT}` and the clean helper checkout was `{HELPER}`. The fresh diagnostic completed with wrapper exit 0 in {wrapper['elapsedSeconds']:.3f} seconds and all 400 primary actions successful. `passed=true` and `idleDiagnosticCompleted` mean bounded diagnostic capture completed, not that timing targets or the local product passed.

| Primary cohort | Count | Median | p95 | p99 | Target |
| --- | ---: | ---: | ---: | ---: | --- |
| Project switch | 100 | {switch['p50Ms']:.6f} ms | **{switch['p95Ms']:.6f} ms** | {switch['p99Ms']:.6f} ms | p95 ≤ 300 ms; failed |
| Input | 200 | {inp['p50Ms']:.6f} ms | **{inp['p95Ms']:.6f} ms** | {inp['p99Ms']:.6f} ms | p95 ≤ 100 ms; passed |
| Draft reset | 100 | {reset['p50Ms']:.6f} ms | {reset['p95Ms']:.6f} ms | {reset['p99Ms']:.6f} ms | No declared p95 target; separate cohort |

The unchanged switching/input p99 requirement is strictly below 1000 ms. The separately traced eight-round follow-up contains 32 successful actions; its switch/input p95 is {follow_switch['p95Ms']:.6f}/{follow_input['p95Ms']:.6f} ms. Those smaller, differently instrumented cohorts do not upgrade the primary result. All 432 actions and numerical spans are retained without filtering or instrumentation subtraction.

## What the evidence supports

Fifteen of the 100 primary switches exceeded 300 ms. `analysis.json` retains every slow action's original Node spans, browser pointer/driver-expected-DOM/frame references and temporally overlapping resource categories. Node spans include automation transport, actionability and scheduling; individual p95 values cannot be added. Expected-DOM is sampled by driver evaluation after a locator wait, so these are **not autonomous React commit, physical display or isolated backend timings**. Resource overlap does not identify an exact project's request or a causal function.

The separate follow-up trace records 40 click calls, each with one click attempt, one visible/enabled/stable witness and one completed click. It does not cover the original 15 slow primary switches or establish their cause. No CPU profile weighting or causal improvement from CLOUD-048/050/051/052, response cancellation, layout stability or any other change is claimed. Earlier response-header intervention diagnostics remain separate; this run used the unchanged idle action protocol.

This run allocated no workers: zero task results, provider calls and external-model calls, hard zero-Create/Start guards and fresh state. Docker readiness/inventory reads still occur. A separate [same-cgroup counter adjunct](../cloud032-cgroup-d20ebd29-20261009/README.md) ran around the launcher. Its own publication owns counter rows, observer/supervisor source, supervision receipts and phase analysis; none is duplicated here. It is not an owned-process memory sampler, atomic admission snapshot, causal per-action attribution, sustained execution, model-quality, Windows or whole-computer responsiveness result. No previous run's RAM/CPU figures or zombie counts are transplanted to this one.

## Source, transformations and reproduction

- `timings.projection.json.gz` retains all **432 original action rows** and **3056 original browser latency rows**. Only each interaction's project ID is removed; numerical diagnostics remain exact. The original private result is {originals['result']['bytes']:,} bytes, SHA256 `{originals['result']['sha256']}`. A projection is not a reconstruction of all original fields.
- Original analysis and trace projection are copied unchanged alongside their exact derivation helper. Trace projection omits selectors, parameters and body values; the full private trace remains hash-bound, not published.
- `source-bindings.json` binds **17 accepted helper members**, including **seven imported repository dependencies**, and **17 overlapping full-template inputs** against exact product/helper Git objects. Image receipt projections bind **104 unchanged runtime source inputs** to the product commit. The original image and binary/archive bytes remain publisher-held; publication did not rebuild or directly audit them.
- The executed idle driver is byte-identical to the preceding 990 idle driver. C049 changes only the full sustained phone tail, removed by the existing idle extraction. Captured input/driver equality preserves protocol continuity; it is not a controlled causal speed comparison between product revisions or hosts.
- `sources/preparation/` retains the exact generator, launcher, extraction patch, source relationship proof and preparation README. The archived 67a5 preparation clean-tree assertion initially failed on concurrent documentation writes; its original script and labelled failure summary are retained. No app was launched by that preparation failure, and unavailable original per-subprocess syntax logs are not invented. That historical preparation event is not a new d20 failure. The reused runtime launcher still required exact clean helper source. This is distinct from the earlier 990 closure helper error, which did **not** recur here.
- `sources/run-idle-d20-with-counter.py` is the exact outer wrapper for this run. It launches the unchanged idle preparation and manages the separate adjunct; the companion capsule binds its outer invocation/result and supervisor identity. Reproduction needs both source capsules and the exact package/image, with task-specific paths adapted to fresh locations.
- Standard provenance paths remain in copied helpers/receipts; projected metadata replaces external execution, extracted package, archive and private fixture-parent locations. Original hashes/lengths remain bound. No private application/credential/retained failed-fixture files were read by publication.

Use the archived preparation README to reproduce in a fresh **exclusive empty Linux Docker environment** with the exact source, package and image. Paths are task-specific; do not run this exclusive harness against the Windows tester's shared Docker daemon or over retained evidence. The product is retrievable from the source commit; archive bytes remain private cloud artifacts. This capsule does not change setup requirements or authorize starting Windows workloads below the local tester's existing memory gates. `analyze.py` derives projections from private originals; it was already run by root and was not rerun for publication.

## Closure and limits

The wrapper retains `cleanupUnconfirmedByWrapper=true`: wrapper exit alone cannot certify cleanup. The final driver receipt reports all five startup handles closed, controller/proxy settlement, empty cleanup errors and fixture removal. Root's independent **read-only** receipt confirms only the exact recorded driver and controller PID/start identities are absent, Docker containers/volumes empty on the same exclusive engine, the same image ID and unchanged original artifact hashes.

There was **no independent browser PID/start census**. Browser/zombie absence and whole-machine closure are not established; historical PID-1 zombies from the prior sustained run remain explicit in that run's evidence and are neither assigned to nor cleared by this diagnostic. Fixture removal is the driver's assertion, and its absent final private path prevents independent fixture-path verification. No old failed fixture or execution ledger was read or edited.

`publication.json`, `SHA256SUMS` and `original-bindings.json` bind the published files and exact original copies. The companion report is bound separately. All acceptance gates remain open at their existing scopes; this failed timing result is not a `LOCAL TEST HANDOFF READY` declaration.
'''
put('README.md',README.encode())
companion=PUBLIC/'2026-10-09-cloud032-idle-d20ebd29.md'
companion.write_text(f'''# CLOUD-032: matching d20 idle result remains below acceptance

Product `{PRODUCT}` / helper `{HELPER}` completed 400 primary actions and 32 separately traced follow-up actions with no action failure. **Switching p95 {switch['p95Ms']:.6f} ms fails the unchanged 300 ms target; input p95 {inp['p95Ms']:.6f} ms passes 100 ms.** Fifteen of 100 primary switches exceed 300 ms. Wrapper exit 0 is diagnostic completion, not performance or product acceptance.

[Exact evidence and reproduction](cloud032-idle-d20ebd29-20261009/README.md) preserve all timings, original private-result/trace hashes, source/package/image bindings, preparation failure, cleanup assertions and independent exact driver/controller closure. There were zero worker/model executions. The [separate same-cgroup counter adjunct](cloud032-cgroup-d20ebd29-20261009/README.md) is not an owned-process memory sampler or per-action causal explanation. No browser census, whole-machine closure, Windows or causal speed improvement is inferred. Older sustained-run PID-1 zombies and failed fixtures remain separate and unchanged.

Raw result SHA256: `{originals['result']['sha256']}`. Matching d20 source/package is a usable checkpoint; full local acceptance and CLOUD-032 remain open.
''')
for p in OUT.rglob('*'):
    if not p.is_file():continue
    data=gzip.decompress(p.read_bytes()) if p.suffix=='.gz' else p.read_bytes()
    assert (b'/workspace/' + b'.c32/' + b'luci-sustained-') not in data,p
assert meta(RUN/'result.json')==originals['result'] and meta(RUN/'action-only-trace.zip')==originals['trace']
members=[{'path':str(p.relative_to(OUT)),'bytes':p.stat().st_size,'sha256':sha(p.read_bytes())} for p in sorted(OUT.rglob('*')) if p.is_file()]
js('publication.json',{'sourceCommit':PRODUCT,'helperCommit':HELPER,'scope':'Matching installed idle diagnostic completion with failed project-switch p95 and passing input p95, not sustained/native/live-model acceptance.','originalResult':originals['result'],'primaryRows':400,'separateFollowupRows':32,'switchP95Ms':switch['p95Ms'],'inputP95Ms':inp['p95Ms'],'acceptedHelperMembers':17,'importedDependenciesIncluded':7,'templateInputs':17,'imageSourceInputs':104,'companionReport':{'path':companion.name,**meta(companion)},'members':members,'manifestConvention':'Members exclude publication.json and SHA256SUMS; sums bind publication.json; companion bound separately.'})
files=[p for p in sorted(OUT.rglob('*')) if p.is_file()]
put('SHA256SUMS',''.join(f'{sha(p.read_bytes())}  {p.relative_to(OUT)}\n' for p in files).encode())
summary={'path':str(OUT),'files':len(files)+1,'manifestMembers':len(members),'sumEntries':len(files),'originalCopies':len(copied),'publicationSHA256':meta(OUT/'publication.json')['sha256'],'sumsSHA256':meta(OUT/'SHA256SUMS')['sha256'],'companionSHA256':meta(companion)['sha256'],'scope':'External publication only; no app/build/tests/metrics re-execution and no repository mutation'}
Path('/workspace/.luci-tools/publish-idle-d20-result.json').write_text(json.dumps(summary,indent=2)+'\n')
print(json.dumps(summary,indent=2))
