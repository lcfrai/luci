#!/usr/bin/env python3
"""Prepare only: copies pinned helpers and adds diagnostics outside the checkout."""
from pathlib import Path
import hashlib,json,shutil,difflib

ROOT=Path(__file__).resolve().parent
SOURCE=Path('/workspace/.luci-tools/cloud032-idle-2da1a821/actual')
DEST=ROOT/'candidate'
sha=lambda b:hashlib.sha256(b).hexdigest()
manifest=json.loads((SOURCE/'external-source-binding.json').read_text())
assert not DEST.exists(), 'Use a fresh preparation directory; preserve previous candidates'
DEST.mkdir(mode=0o700)
changes=[]
for name,digest in manifest['localSources'].items():
 raw=(SOURCE/name).read_bytes();assert sha(raw)==digest,(name,'changed frozen source')
 (DEST/name).write_bytes(raw)

# Reuse byte-for-byte runtime helper captures, never an in-progress checkout
# helper. Their only local transitive import is startup -> acceptance-lifecycle.
accepted=SOURCE.parent/'runs/actual-idle-1/accepted-harness'
frozen=[]
for name,digest in manifest['repositoryDependencies'].items():
 capture=accepted/name.removeprefix('scripts/acceptance/').replace('/','_')
 raw=capture.read_bytes();assert sha(raw)==digest,(name,'accepted runtime helper changed')
 basename=Path(name).name;assert not (DEST/basename).exists()
 (DEST/basename).write_bytes(raw);frozen.append(basename)

def edit(name,old,new,label):
 path=DEST/name;text=path.read_text();assert text.count(old)==1,(name,label,text.count(old))
 path.write_text(text.replace(old,new));changes.append({'file':name,'label':label,'oldSHA256':sha(old.encode()),'newSHA256':sha(new.encode())})

edit('sustained.mjs',"import assert from 'node:assert/strict';","import assert from 'node:assert/strict';\nimport {createPrimaryProfiler} from './primary-profiler.mjs';",'external profiler only')
edit('sustained.mjs',"const driverNames=['sustained.mjs'","const driverNames=['primary-profiler.mjs','sustained.mjs'",'capture profiler source with accepted harness')
edit('sustained.mjs',"const driverNames=['primary-profiler.mjs'","const driverNames=["+','.join(repr(x) for x in frozen)+",'primary-profiler.mjs'",'capture exact frozen runtime dependencies')
for name in manifest['repositoryDependencies']:
 old="await import(join(repo,"+repr(name)+"))"
 if old in (DEST/'sustained.mjs').read_text():edit('sustained.mjs',old,"await import(join(here,"+repr(Path(name).name)+"))",'import frozen '+name)
old="for(const [name,expected] of Object.entries(externalBinding.repositoryDependencies)) assert.equal(sha(readFileSync(join(repo,name))),expected,'Captured repository dependency changed: '+name);"
new="for(const [name,expected] of Object.entries(externalBinding.frozenRuntimeDependencies)) assert.equal(sha(readFileSync(join(here,name.split('/').at(-1)))),expected,'Frozen runtime dependency changed: '+name);\nfor(const [name,expected] of Object.entries(externalBinding.repositoryDependencies)) assert.equal(sha(readFileSync(join(repo,name))),expected,'Recorded runtime tooling changed: '+name);"
edit('sustained.mjs',old,new,'verify frozen inputs plus separately recorded installed tooling; retain clean helper checkout guard')
path=DEST/'sustained.mjs';text=path.read_text();needle="readFileSync(join(repo,'scripts/acceptance',name))";assert text.count(needle)==3
text=text.replace(needle,"readFileSync(join(here,name.split('/').at(-1)))")
text=text.replace("['scripts/acceptance/'+name,sha(","['frozen-runtime-original-path/scripts/acceptance/'+name,sha(")
path.write_text(text);changes.append({'file':'sustained.mjs','label':'three capture/final hash reads use frozen runtime bytes; receipt labels original paths explicitly','replacementCount':3})
edit('sustained.mjs',"let engineID;","let engineID,primaryProfiler;",'retain profiler for independent failure cleanup')
old="setStage('idle-baseline');await quietPhase('quiet-app-open-idle');const idle=phaseStart('idle-interactive-calibration');await interactions.calibrate(100);phaseEnd(idle);await passiveTiming('end-idle-calibration');"
new="""setStage('idle-baseline');await quietPhase('quiet-app-open-idle');
 assert.equal(option('--trace-after','no'),'no','CDP-first diagnostic forbids Playwright throttling-changing trace arm');
 primaryProfiler=createPrimaryProfiler({page,directory:join(evidence,'private-primary-profile'),assertActive:assertWorkflowActive});
 await primaryProfiler.start();assertWorkflowActive();
 const idle=phaseStart('idle-interactive-calibration');await interactions.calibrate(100);phaseEnd(idle);await passiveTiming('end-idle-calibration');
 receipt.primaryProfiling=await boundedAttempt(()=>primaryProfiler.finish('primary-complete'),10000,'primary-profiling-stop-deadline');
 assert(receipt.primaryProfiling.complete,'Primary profiling incomplete; preserve original actions and diagnostics');"""
edit('sustained.mjs',old,new,'trace overlaps exact original primary400; phase and action timings remain unchanged')
edit('sustained.mjs',"finally {\n beginCleanup();persistFailureCheckpoint();","finally {\n beginCleanup();persistFailureCheckpoint();\n if(primaryProfiler){try{receipt.primaryProfiling=await boundedAttempt(()=>primaryProfiler.finish('workflow-finally'),cleanupBudget(10000),'profiling-finally-deadline');}catch(error){cleanupFailure(error);receipt.primaryProfiling=primaryProfiler.snapshot();}}",'profiler cannot indefinitely prevent existing independent owned-resource cleanup')

edit('interaction-metrics.mjs','const frames=()=>page.evaluate(browserFrameWitness);','const frames=actionID=>page.evaluate(browserFrameWitness,actionID);','carry correlation ID in existing frame RPC; no added per-action RPC')
edit('interaction-metrics.mjs','diagnostic.frames(await frames());',"diagnostic.frames(await frames(receipt.interactions.length+1));",'correlation follows exact retained primary receipt order')

edit('latency-diagnostics.mjs'," const bound=2048;", """ const bound=2048;
 let profileMarks=0;
 Object.defineProperty(window,'__luciPrimaryProfileMark',{value:(name,at)=>{
  if(!window.__luciPrimaryProfileActive)return;
  if(++profileMarks>10000){window.__luciPrimaryProfileMarkerOverflow=true;return;}
  // Only fixed names, sequence numbers and same-document clock values.
  performance.mark(name,{detail:{observedAt:at,timeOrigin:performance.timeOrigin}});
  performance.clearMarks(name);
 },configurable:false});""",'bounded user-timing joins; no names, values, IDs, URLs or keys')
edit('latency-diagnostics.mjs','data.rows.push(value);return value;',"data.rows.push(value);if(['input-dispatch','pointer-dispatch','input-frame-1','input-frame-2','expected-dom'].includes(row.kind))window.__luciPrimaryProfileMark('luci-latency:'+row.kind+':'+sequence,row.at);return value;",'existing passive observer callbacks get exact sequence markers')
old="""export function browserFrameWitness() {
 const requestedAt=performance.now();
 return new Promise(resolve=>requestAnimationFrame(()=>{
  const firstFrameAt=performance.now();
  requestAnimationFrame(()=>resolve({requestedAt,firstFrameAt,secondFrameAt:performance.now(),references:window.__luciAcceptanceLatency?.references()??null}));
 }));
}"""
new="""export function browserFrameWitness(actionID) {
 const requestedAt=performance.now();
 const mark=(part,at)=>window.__luciPrimaryProfileMark?.('luci-primary:'+actionID+':'+part,at);
 mark('requested',requestedAt);
 return new Promise(resolve=>requestAnimationFrame(()=>{
  const firstFrameAt=performance.now();mark('frame-1',firstFrameAt);
  requestAnimationFrame(()=>{const secondFrameAt=performance.now();mark('frame-2',secondFrameAt);resolve({requestedAt,firstFrameAt,secondFrameAt,references:window.__luciAcceptanceLatency?.references()??null,primaryProfile:{actionID,markerOverflow:window.__luciPrimaryProfileMarkerOverflow===true}});});
 }));
}"""
edit('latency-diagnostics.mjs',old,new,'three marker witnesses inside existing two-frame RPC')

shutil.copyfile(ROOT/'primary-profiler.mjs',DEST/'primary-profiler.mjs')
tooling={}
repo=Path('/workspace/luci')
for package in ('playwright','playwright-core'):
 for path in sorted((repo/'desktop/node_modules'/package).rglob('*')):
  if path.is_file() and path.suffix in ('.js','.mjs','.cjs','.json'):
   tooling[str(path.relative_to(repo))]=sha(path.read_bytes())
new_manifest={**manifest,'executionDirectory':str(DEST),'scope':'Unexecuted external PRIMARY400 CDP-only diagnostic candidate; exact frozen primary workload, instrumented timings are not acceptance results','parentBindingSHA256':sha((SOURCE/'external-source-binding.json').read_bytes()),'localSources':{name:sha((DEST/name).read_bytes()) for name in [*manifest['localSources'],'primary-profiler.mjs',*frozen]},'frozenRuntimeDependencies':manifest['repositoryDependencies'],'repositoryDependencies':tooling,'runtimeToolingScope':'All installed playwright/playwright-core JS/MJS/CJS/JSON files, pinned at preparation; tooling is not the product commit','primaryProfilingChanges':changes}
(DEST/'external-source-binding.json').write_text(json.dumps(new_manifest,indent=2)+'\n')
patch=[]
for name in manifest['localSources']:
 before=(SOURCE/name).read_text();after=(DEST/name).read_text()
 if before!=after:patch.extend(difflib.unified_diff(before.splitlines(True),after.splitlines(True),fromfile='frozen/'+name,tofile='candidate/'+name))
(ROOT/'diagnostic.patch').write_text(''.join(patch))
(ROOT/'preparation.json').write_text(json.dumps({'status':'prepared-not-executed','sourceDirectory':str(SOURCE),'sourceBindingSHA256':sha((SOURCE/'external-source-binding.json').read_bytes()),'candidateBindingSHA256':sha((DEST/'external-source-binding.json').read_bytes()),'patchSHA256':sha((ROOT/'diagnostic.patch').read_bytes()),'changes':changes,'workflowUnchanged':['60s quiet','100 switches','200 inputs','100 draft resets','400 ordered primary spans','original Node start/end timing','300/100ms p95 limits','zero Create/Start/provider calls','private fixture and independent resource cleanup'],'limitations':['CDP sampling/markers add overhead; no acceptance comparison or overhead subtraction','No runtime executed','Clean dependency checkout and package identity still supplied independently at launch','Primary input/focus/setup actions unchanged; existing optional follow-up disabled with --trace-after no for this arm']},indent=2)+'\n')
print(json.dumps({'status':'prepared-not-executed','candidate':str(DEST),'bindingSHA256':sha((DEST/'external-source-binding.json').read_bytes())}))
