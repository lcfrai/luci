#!/usr/bin/env python3
"""Generate a hash-bound idle-only diagnostic from the preserved attempt3 driver.

Does not run any app, browser, compiler, test or Docker command. Exact replacements
refuse a changed source template. The generated driver is diagnostic-only and is
not a replacement for the complete sustained acceptance script.
"""
from pathlib import Path
import difflib
import hashlib
import json
import shutil
import sys

HERE=Path(__file__).resolve().parent
REPO=Path('/workspace/luci')
ORIGINAL=HERE/'original'
OUT=HERE/'actual'
OUT.mkdir(exist_ok=True,mode=0o700)
sha=lambda data:hashlib.sha256(data).hexdigest()
source=(ORIGINAL/'sustained.mjs').read_text()
changes=[]
def replace(old,new,label):
    global source
    assert source.count(old)==1,(label,source.count(old))
    source=source.replace(old,new)
    changes.append({'label':label,'oldSHA256':sha(old.encode()),'newSHA256':sha(new.encode())})

replace("const PREPARATION_MS=120000,WORKFLOW_MS=1800000,CLEANUP_MS=180000,", "const PREPARATION_MS=60000,WORKFLOW_MS=210000,CLEANUP_MS=120000,",'bounded diagnostic deadline')
replace("throw Error('Thirty minute acceptance deadline expired')", "throw Error('Idle diagnostic deadline expired')",'accurate deadline text')
replace("validateHelperLocation(realpathSync(here),realpathSync(join(repo,'scripts/acceptance/sustained-four-project')));", """// The source template belongs to the preserved installed attempt, while this
// generated external diagnostic has its own exact source binding and scope.
const externalBinding=JSON.parse(readFileSync(join(here,'external-source-binding.json'),'utf8'));
assert.equal(realpathSync(here),externalBinding.executionDirectory);
for(const [name,expected] of Object.entries(externalBinding.localSources)) assert.equal(sha(readFileSync(join(here,name))),expected,'External diagnostic source changed: '+name);
for(const [name,expected] of Object.entries(externalBinding.repositoryDependencies)) assert.equal(sha(readFileSync(join(repo,name))),expected,'Captured repository dependency changed: '+name);""",'external executed-source identity; retain product/clean dependency checkout binding')
replace("const check = (name, details = {}) =>", """Object.assign(receipt,{
 mode:'installed-four-project-idle-diagnostic',diagnosticOnly:true,fullSustainedAcceptance:false,
 scope:'Fresh installed package, exact original four saved contexts and paused queues, original draft/warm setup and idle100 switches/200 inputs. No task execution or sustained completion claim.',
 resourceScope:'One owned browser and installed app. Docker readiness/image inventory reads are required. Explicit guards reject every Create/Start before forwarding. No full Linux resource sampler; browser diagnostic collection is identical to the static control.',
 limitations:[...receipt.limitations,'Diagnostic subset only; original sustained failure remains unchanged. No resource-cost subtraction.','No independent Linux observer in either prepared control arm; this is not a matched replay of the old observer load.'],
 externalSourceBinding:externalBinding,dependencyCheckout:receipt.harnessCheckout,
 diagnosticPreparedFrom:{product:'4df94ab9dad0e9fedd28a873bbadc56720e1402e',helper:'5d306b6dae3cc9d0a4d9cbf95f8142c37958964f'},
});
const check = (name, details = {}) =>""",'declare diagnostic scope before startup')
replace("async function setPaused(paused){", "async function setPaused(paused){\n assert.equal(paused,true,'Idle diagnostic cannot enable queue dispatch');",'never unpause queue')
replace("function identifyCreate(bytes,meta){", "function identifyCreate(bytes,meta){\n throw Object.assign(Error('Idle diagnostic refuses all Docker Create requests'),{code:'idle-diagnostic-create-forbidden'});",'hard zero Create identity guard')
replace("beforeCreate:admission,beforeStart:assertWorkflowActive,", "beforeCreate:()=>{throw Error('Idle diagnostic refuses Create');},beforeStart:()=>{throw Error('Idle diagnostic refuses Start');},",'hard zero Create and Start forwarding guards')
replace("maxCreates:receipt.bounds.maxCreates,maxCreatesByKind:{worker:5,fence:1},maxStarts:receipt.bounds.maxStarts,", "maxCreates:receipt.bounds.maxCreates,maxCreatesByKind:{worker:1,fence:1},maxStarts:receipt.bounds.maxStarts,",'match declared positive structural proxy budgets; hard deny remains authority')
proxy_phase=next(line for line in source.splitlines(keepends=True) if line.startswith("  {name:'proxy',timeoutMs:"))
provider_phase=next(line for line in source.splitlines(keepends=True) if line.startswith("  {name:'provider',timeoutMs:"))
assert source.count(proxy_phase)==1 and source.count(provider_phase)==1
source=source.replace(proxy_phase,'').replace(provider_phase,provider_phase+proxy_phase)
changes.append({'label':'Diagnostic cleanup controllers→browser→provider→proxy; preserve original proxy close bounds, reserve browser fallback before a slow read drain','movedPhaseSHA256':sha(proxy_phase.encode())})
replace("const driverNames=['sustained.mjs',", "const driverNames=['sustained.mjs','external-source-binding.json',",'capture external binding beside all executed source')
replace("receipt.bounds={preparationDeadlineMs:PREPARATION_MS,scenarioDeadlineMs:1800000,cleanupDeadlineMs:CLEANUP_MS,totalDeadlineMs:DRIVER_TOTAL_MS,maxMeasuredActions:6000,actualFuzzSecondsPerInitialWorker:150,upstreamRequestMs:120000,maxCreates:5,maxStarts:5,preCreateFreeBytes:6*1024**3,runningReserveBytes:1024**3,workerMemoryBytes:1024**3,workerNanoCPUs:1000000000};", "receipt.bounds={preparationDeadlineMs:PREPARATION_MS,scenarioDeadlineMs:WORKFLOW_MS,cleanupDeadlineMs:CLEANUP_MS,totalDeadlineMs:DRIVER_TOTAL_MS,maxMeasuredActions:432,primaryMeasuredActions:400,optionalTraceMeasuredActions:32,actualFuzzSeconds:0,allowedCreates:0,allowedStarts:0,upstreamRequestMs:120000,maxCreates:1,maxStarts:1,preCreateFreeBytes:6*1024**3,runningReserveBytes:1024**3,proxyStructuralBudgetNote:'Existing proxy requires positive numeric maxima; unconditional guards refuse Create and Start before forwarding.'};",'diagnostic budgets; proxy positive structural bounds cannot grant mutation')
start=" receipt.predeclaredTimingReason='Before installed execution, standalone offline cold smoke took584s"
line=next(line for line in source.splitlines() if line.startswith(start))
replace(line," receipt.predeclaredTimingReason='Prepared idle subset:60s quiet,100 switches/200 inputs/100 resets;210s workflow,60s preparation and120s cleanup. No extension on failure.';",'accurate workload timing rationale')
start=" setStage('sustained-work');const loaded=phaseStart('loaded');"
end="\n });\n}catch(error){"
assert source.count(start)==1 and source.count(end)==1
begin=source.index(start);finish=source.index(end,begin)
removed=source[begin:finish]
replacement=""" receipt.interactionSummary=summarizeInteractions(receipt.interactions);
 receipt.latencyTargets={measurement:'Original All projects→open saved Define context→two frames; input→expected field→two frames; no physical-input claim',projectSwitchP95Ms:300,inputP95Ms:100,p99StrictlyBelowMs:1000};
 receipt.targetComparison=Object.fromEntries(['project-switch','input'].map(kind=>{const value=receipt.interactionSummary['idle-interactive-calibration:'+kind];assert.equal(value.count,kind==='input'?200:100);assert.equal(value.failed,0);return[kind,{...value,withinReference:value.p95Ms<=(kind==='input'?100:300)&&value.p99Ms<1000}];}));
 assert.equal(receipt.interactions.length,400);
 const traceAfter=option('--trace-after','no');assert(['yes','no'].includes(traceAfter));
 if(traceAfter==='yes'){
  const tracePhase=phaseStart('idle-actionability-trace-followup');receipt.traceScope='Eight extra rounds after the primary100; snapshots/screenshots/sources disabled; instrumentation differs and does not replace primary metrics.';
  await page.context().tracing.start({screenshots:false,snapshots:false,sources:false});
  try{for(let index=0;index<8;index++)await interactions.round(index);await passiveTiming('trace-followup-complete');phaseEnd(tracePhase);}
  finally{await boundedAttempt(()=>page.context().tracing.stop({path:join(evidence,'action-only-trace.zip')}),5000,'trace-stop-timeout');}
 }
 assert.deepEqual(effects(),{create:0,start:0,destroy:0});assert.equal(proxy.observations.failures.length,0);assert.equal(receipt.browserErrors.length,0);assert.equal(providerCalls,0);assert.equal(auxiliaryCalls,0);assert.equal(workerContainers().length,0);
 receipt.idleDiagnosticCompleted=true;receipt.passed=true;
 check('Fresh installed idle diagnostic completed; no full sustained or latency-target success implied');"""
source=source[:begin]+replacement+source[finish:]
changes.append({'label':'Stop after exact original idle phase; remove dispatch/work/amendment/settled acceptance tail','removedBytes':len(removed.encode()),'oldSHA256':sha(removed.encode()),'newSHA256':sha(replacement.encode())})
replace("const here=dirname(fileURLToPath(import.meta.url)),repo=resolve(process.env.LUCI_ACCEPTANCE_REPO??resolve(here,'../../..'))", "const here=dirname(fileURLToPath(import.meta.url)),repo=resolve(process.env.LUCI_ACCEPTANCE_REPO??'/workspace/luci')",'explicit external diagnostic repository fallback')
original_help=next(line for line in source.splitlines() if line.startswith("if(args.includes('--help'))"))
replace(original_help,"if(args.includes('--help')){console.log('CLOUD-032 diagnostic only: frozen installed prefix through idle100 switches/200 inputs, zero Create/Start hard guards. Original package/image/source/provenance arguments remain mandatory. No full sustained pass. Explicit controlled runtime window required.');process.exit(0);}",'diagnostic CLI scope')
for path in ORIGINAL.iterdir():
    if path.is_file():shutil.copyfile(path,OUT/path.name)
(OUT/'sustained.mjs').write_text(source)
dependency_names=['four-project-queue/native-terminal-diagnostics.mjs','four-project-queue/sample-linux-owned.py','launch-recovery/fenced-docker-proxy.mjs','launch-recovery/worker-create-guard.mjs','launch-recovery/durable-acceptance-journal.mjs','launch-recovery/acceptance-lifecycle.mjs','launch-recovery/acceptance-startup.mjs']
dependencies={}
dependency_differences={}
for name in dependency_names:
    old=(ORIGINAL/name.replace('/','_')).read_bytes()
    live=(REPO/'scripts/acceptance'/name).read_bytes()
    if old!=live:
        assert name=='launch-recovery/fenced-docker-proxy.mjs' and '--allow-current-proxy' in sys.argv,('Explicit current-proxy binding required, or unexpected dependency changed',name)
        dependency_differences['scripts/acceptance/'+name]={'templateSHA256':sha(old),'currentSHA256':sha(live),'reason':'Root-authorized current opt-in proxy diagnostics; zero Create/Start forwarding remains mandatory.'}
        (HERE/'current-proxy.patch').write_text(''.join(difflib.unified_diff(old.decode().splitlines(keepends=True),live.decode().splitlines(keepends=True),fromfile='preserved-attempt3/'+name,tofile='current-dependency/'+name)))
    dependencies['scripts/acceptance/'+name]=sha(live)
    shutil.copyfile(REPO/'scripts/acceptance'/name,OUT/name.replace('/','_'))
local_names=['sustained.mjs','task-command.mjs','workloads.mjs','interaction-metrics.mjs','latency-diagnostics.mjs','task-context.mjs','source-binding.mjs','queue-controls.mjs','run-with-observer.py']
binding={'version':1,'executionDirectory':str(OUT),'scope':'External generated diagnostic, not the clean helper checkout itself. Dependency checkout remains separately exact/clean.','templateSourceSHA256':sha((ORIGINAL/'sustained.mjs').read_bytes()),'localSources':{name:sha((OUT/name).read_bytes()) for name in local_names},'repositoryDependencies':dependencies,'dependencyDifferences':dependency_differences,'extractionChanges':changes}
(OUT/'external-source-binding.json').write_text(json.dumps(binding,indent=2)+'\n')
(HERE/'idle-extraction.patch').write_text(''.join(difflib.unified_diff((ORIGINAL/'sustained.mjs').read_text().splitlines(keepends=True),source.splitlines(keepends=True),fromfile='preserved-attempt3/sustained.mjs',tofile='external-idle-diagnostic/sustained.mjs')))
(HERE/'idle-extraction.json').write_text(json.dumps(binding,indent=2)+'\n')
print(json.dumps({'preparedOnly':True,'generated':str(OUT/'sustained.mjs'),'changes':len(changes),'sourceSHA256':sha(source.encode()),'runtimeExecuted':False}))
