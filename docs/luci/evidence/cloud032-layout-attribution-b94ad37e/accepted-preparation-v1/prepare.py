"""External source preparation only. No app, browser, Docker, compiler or package mutation."""
from pathlib import Path
import json,hashlib,shutil,subprocess,difflib
R=Path(__file__).resolve().parent;T=Path('/workspace/.luci-tools/cloud032-long-frame-29931afc');REPO=Path('/workspace/luci')
OLD='29931afc5b31ae9b9bd7b8973a7e3fdb840609a3';SOURCE='b94ad37e53181e360ac8910ee980570de622728c'
sha=lambda p:hashlib.sha256(p.read_bytes()).hexdigest()
def put(p,v):p.write_text(json.dumps(v,indent=2)+'\n')
def once(t,a,b):assert t.count(a)==1,a;return t.replace(a,b)
assert not (R/'actual').exists(),'Fresh preparation required'
assert sha(T/'actual/sustained.mjs')=='82eacd50a9dea2c8c9ff1499a33f7d23f5799295d7fe0fa07874515304aae055'
shutil.copytree(T/'actual',R/'actual')
capture=json.loads((T/'template-capture.json').read_text())
for name,entry in capture['sources'].items():
 b=subprocess.check_output(['git','show',SOURCE+':'+name],cwd=REPO,timeout=10)
 assert hashlib.sha256(b).hexdigest()==entry['sha256'] and len(b)==entry['bytes'] and (REPO/name).read_bytes()==b,name
capture.update(capturedHEAD=SOURCE,scope='Original17 selected helper inputs unchanged; eight-round external layout attribution driver independently bound. No primary benchmark replay.')
put(R/'template-capture.json',capture);put(R/'actual/template-capture.json',capture)
package=Path('/tmp/luci-retained-tools/package-0.5.0-b94ad37e');prov=REPO/'artifacts/quality-swarm/candidate-b94ad37e/package/package-provenance.json';provenance=json.loads(prov.read_text())
assert provenance['sourceCommit']==SOURCE and provenance['sourceDirty'] is False and Path(provenance['packageDir'])==package
bundleMap=json.loads((T/'actual/long-frame-bundles.json').read_text());assert len(bundleMap)==170
assert sorted(p.name for p in (package/'web/assets').glob('*.js'))==sorted(Path(x['path']).name for x in bundleMap)
for x in bundleMap:assert sha(package/'web'/x['path'].lstrip('/'))==x['sha256']
assert sha(package/'web/index.html')=='f99796b77f74be2fed35d0fa5b0dbf6729b6552905831e402f9a6515a231825b'
for name in ['layout-trace-capture.mjs','layout-trace-projector.mjs']:shutil.copyfile(R/name,R/'actual'/name)
before=(T/'actual/sustained.mjs').read_text();driver=before
imports="import {createTraceSink,createLayoutTraceCapture} from './layout-trace-capture.mjs';\nimport {projectLayoutTrace} from './layout-trace-projector.mjs';"
driver=once(driver,"import {installLongFrameTiming} from './long-frame-observer.mjs';","import {installLongFrameTiming} from './long-frame-observer.mjs';\n"+imports)
driver=once(driver,"const driverNames=['long-frame-observer.mjs'","const driverNames=['layout-trace-capture.mjs','layout-trace-projector.mjs','long-frame-observer.mjs'")
driver=once(driver,"mode:'installed-four-project-navigation-callback-5f4-diagnostic'","mode:'installed-eight-round-layout-attribution-b94-diagnostic'")
driver=once(driver,"scope:'Fresh installed package, exact original four saved contexts and paused queues, original draft/warm setup and idle100 switches/200 inputs. No task execution or sustained completion claim.'","scope:'Fresh installed b94 package; eight exact original interaction rounds (32 actions), original saved contexts/paused queues/draft warm setup. Layout caller attribution only, no primary latency acceptance or sustained claim.'")
driver=once(driver,"No full Linux resource sampler; browser diagnostic collection is identical to the static control.","No full Linux resource sampler. Original browser diagnostics plus a bounded renderer trace; added overhead is unquantified, no comparison/acceptance claim.")
driver=once(driver,"function assertWorkflowActive(){if(closing)","let layoutCapture,layoutFailure,layoutFinish;\nasync function finishLayoutTrace(reason){if(!layoutCapture)return;if(!layoutFinish){layoutFinish=(async()=>{const value=await layoutCapture.finish(reason);receipt.layoutTraceCapture=value;writeFileSync(join(evidence,'layout-trace-capture.json'),JSON.stringify(value,null,2)+'\\n',{mode:0o600,flag:'wx'});return value;})();void layoutFinish.catch(()=>{});}return layoutFinish;}\nfunction assertWorkflowActive(){if(layoutFailure)throw layoutFailure;if(closing)")
driver=once(driver,"const phases=await runBoundedCleanup({checkpoint:persistFailureCheckpoint,onFailure:cleanupFailure,onPhase:(name,status)=>journal.record('driver-stage',{stage:'cleanup-'+name+'-'+status}),phases:[", "const phases=await runBoundedCleanup({checkpoint:persistFailureCheckpoint,onFailure:cleanupFailure,onPhase:(name,status)=>journal.record('driver-stage',{stage:'cleanup-'+name+'-'+status}),phases:[\n  {name:'layout-trace',timeoutMs:30000,run:async()=>{if(!layoutCapture)return {notStarted:true};if(!layoutFinish)layoutCapture.cancel();return finishLayoutTrace('failed');}},")
driver=once(driver,'maxMeasuredActions:432,primaryMeasuredActions:400,optionalTraceMeasuredActions:32','maxMeasuredActions:32,primaryMeasuredActions:0,diagnosticTraceMeasuredActions:32')
driver=once(driver,"receipt.predeclaredTimingReason='Prepared idle subset:60s quiet,100 switches/200 inputs/100 resets;210s workflow,60s preparation and120s cleanup. No extension on failure.';","receipt.predeclaredTimingReason='Eight original rounds only (8 switches/16 inputs/8 resets), after original60s quiet; bounded30s trace,210s workflow,60s preparation and120s cleanup. No primary benchmark or latency acceptance; no extension on failure.';")
start=driver.index(" setStage('idle-baseline');await quietPhase('quiet-app-open-idle');")
end=driver.index(" assert.deepEqual(effects(),{create:0,start:0,destroy:0});",start)
replacement=""" setStage('layout-attribution');await quietPhase('quiet-app-open-idle');
 readinessAttempted=true;readinessLedger.attempt();readinessLedger.begin(await timed(beginReadiness(page,projectStates),3000,'Autonomous DOM witness setup'));readinessStarted=true;
 receipt.latencyTargets={projectSwitchP95Ms:300,inputP95Ms:100,p99StrictlyBelowMs:1000,assessed:false,reason:'Eight instrumented diagnostic rounds are not a primary acceptance cohort; previous failures remain unchanged.'};receipt.targetComparison={status:'not-assessed-diagnostic-short-cohort'};
 const cdp=await timed(startupScope.acquire('layout-trace-cdp',{start:()=>browser.newBrowserCDPSession(),close:handle=>handle.detach(),parent:'browser'}),5000,'Layout CDP setup');assertWorkflowActive();
 const session={send:cdp.send.bind(cdp),on:cdp.on.bind(cdp),off:cdp.off.bind(cdp),detach:()=>startupScope.close('layout-trace-cdp',2000)};
 layoutCapture=createLayoutTraceCapture({session,sink:createTraceSink(evidence),onFailure:error=>{layoutFailure??=error;}});
 const tracePhase=phaseStart('eight-round-layout-attribution');let capture;
 try{
  await layoutCapture.start();assertWorkflowActive();
  await layoutCapture.runRounds({assertHealthy:health,round:index=>interactions.round(index),mark:async marker=>{
   health();const value=await page.evaluate(({name,round,edge})=>{const entry=performance.mark(name);return {name,round,edge,timeOrigin:performance.timeOrigin,startTime:entry.startTime};},marker);health();return value;
  }});
 }finally{capture=await finishLayoutTrace(layoutFailure?'failed':'completed');phaseEnd(tracePhase);}
 assert(capture.complete,'Bounded Layout capture refused; exact raw stream/journal retained');
 try{
  receipt.layoutTraceProjection=projectLayoutTrace(readFileSync(join(evidence,'layout-trace.raw.json')),{appOrigin:base,bundles:JSON.parse(readFileSync(join(here,'long-frame-bundles.json'),'utf8')),markers:capture.markers,expectedRounds:8,capture});
  writeFileSync(join(evidence,'layout-trace-projection.json'),JSON.stringify(receipt.layoutTraceProjection,null,2)+'\\n',{mode:0o600,flag:'wx'});
 }catch(error){const code=/^[a-z][a-z0-9-]{1,80}$/.test(error?.code??'')?error.code:'layout-projection-refused';receipt.layoutTraceProjection={validCapture:false,status:'refused',code};writeFileSync(join(evidence,'layout-trace-projection.json'),JSON.stringify(receipt.layoutTraceProjection,null,2)+'\\n',{mode:0o600,flag:'wx'});throw Object.assign(Error(code),{code});}
 assert(receipt.layoutTraceProjection.attributionAvailable===true,'Trace has no attributable layout caller; raw evidence retained, no component or acceptance claim');
 await passiveTiming('end-layout-attribution');readinessLedger.append(await timed(endReadiness(page),3000,'Autonomous DOM witness close'));readinessClosed=true;
 assert.equal(receipt.interactions.length,32);assert(receipt.interactions.every(value=>value.success));receipt.interactionSummary=summarizeInteractions(receipt.interactions);
"""
driver=driver[:start]+replacement+driver[end:]
driver=once(driver,"check('Fresh installed idle diagnostic completed; no full sustained or latency-target success implied');","check('Eight-round Layout caller capture completed; no latency acceptance, component cause or sustained success implied');")
driver=once(driver,'CLOUD-032 diagnostic only: frozen installed prefix through idle100 switches/200 inputs','CLOUD-032 diagnostic only: eight original rounds for bounded Layout caller attribution')
(R/'actual/sustained.mjs').write_text(driver)
binding=json.loads((R/'actual/external-source-binding.json').read_text());binding.update(executionDirectory=str(R/'actual'),prospectiveProduct=SOURCE,prospectiveHelper=SOURCE)
binding.update(scope='Exact b94 eight-round Layout caller attribution only;32 original actions,no primary acceptance cohort. Historical extraction fields describe the retained template, not this runtime.',templateBindingSHA256=sha(T/'actual/external-source-binding.json'),historicalTemplateSourceSHA256=binding['templateSourceSHA256'],templateSourceSHA256=sha(T/'actual/sustained.mjs'))
binding['navigationVerificationDiagnostic']={'scope':'Original verification function bytes retained;8 diagnostic switches only,not400+32 primary benchmark.','runtimeAuthorized':False}
binding['longFrameDiagnostic']['integration']='Retained setup-only LoAF observer accompanies bounded renderer trace; eight instrumented rounds, no primary acceptance or matched-overhead claim.'
binding['layoutDiagnostic']={'templateSource':OLD,'templateDriverSHA256':sha(T/'actual/sustained.mjs'),'scope':'32 instrumented original actions only; no primary benchmark acceptance','frontendJSAndIndexMatch299':True,'runtimeLaunched':False}
for name in ['layout-trace-capture.mjs','layout-trace-projector.mjs']:binding['localSources'][name]=sha(R/'actual'/name)
for name in binding['localSources']:binding['localSources'][name]=sha(R/'actual'/name)
put(R/'actual/external-source-binding.json',binding)
launcher=(T/'launch.py').read_text().replace(OLD,SOURCE).replace('29931afc','b94ad37e').replace('matching299','matchingb94').replace(sha(T/'package-provenance.json'),sha(prov))
old="'quietSeconds':60,'primarySwitches':100,'primaryInputs':200,'primaryResets':100,'extraTraceRounds':8,'originalActionCohortExecuted':True,'navigationVerificationResponsesExpected':108"
new="'quietSeconds':60,'primarySwitches':0,'primaryInputs':0,'primaryResets':0,'diagnosticRounds':8,'diagnosticSwitches':8,'diagnosticInputs':16,'diagnosticResets':8,'originalActionModulesUnchanged':True,'primaryLatencyAssessed':False,'navigationVerificationResponsesExpected':8"
launcher=once(launcher,old,new);launcher=once(launcher,"parser.add_argument('--trace-after',choices=['yes'],required=True)","parser.add_argument('--layout-diagnostic',choices=['yes'],required=True)")
launcher=once(launcher,"'--trace-after',args.trace_after","'--layout-diagnostic',args.layout_diagnostic")
(R/'launch.py').write_text(launcher)
cmd=json.loads((T/'launch-command.json').read_text());argv=cmd['command'];argv[1]=str(R/'launch.py')
for flag,value in [('--source',SOURCE),('--helper-source',SOURCE),('--package',str(package)),('--provenance',str(prov)),('--output','/workspace/luci/artifacts/quality-swarm/layout-b94ad37e-attempt1')]:argv[argv.index(flag)+1]=value
argv[argv.index('--trace-after')]='--layout-diagnostic';cmd['scope']='Prepared eight-round caller attribution only; no primary latency acceptance. Root runtime grant required.';put(R/'launch-command.json',cmd)
image=Path(argv[argv.index('--image-provenance')+1]);image_value=json.loads(image.read_text());assert image_value['sourcesBefore']==image_value['sourcesAfter']
subprocess.run(['git','diff','--exit-code',OLD,SOURCE,'--',*image_value['sourcesBefore']],cwd=REPO,check=True,stdout=subprocess.DEVNULL,timeout=15)
unchanged=['interaction-metrics.mjs','navigation-verifier.mjs','latency-diagnostics.mjs','autonomous-readiness.mjs','readiness-ledger.mjs']
for name in unchanged:assert (R/'actual'/name).read_bytes()==(T/'actual'/name).read_bytes()
put(R/'runtime-binding.json',{'productSource':SOURCE,'helperSource':SOURCE,'templateSource':OLD,'templateDriverSHA256':sha(T/'actual/sustained.mjs'),'driverSHA256':sha(R/'actual/sustained.mjs'),'unchangedActionModules':{name:sha(R/'actual'/name) for name in unchanged},'frontendJSFilesVerifiedIdentical':len(bundleMap),'webIndexSHA256':provenance['webIndexSHA256'],'packageProvenanceSHA256':sha(prov),'archiveSHA256':provenance['archiveSHA256'],'binarySHA256':provenance['binarySHA256'],'image':image_value['candidateImage'],'imageSourceInputsUnchanged':len(image_value['sourcesBefore']),'runtimeLaunched':False})
shutil.copyfile(prov,R/'package-provenance.json');shutil.copyfile(image,R/'image-provenance.json')
patch=[]
for name,a,b in [('actual/sustained.mjs',before,driver),('launch.py',(T/'launch.py').read_text(),launcher)]:patch.extend(difflib.unified_diff(a.splitlines(True),b.splitlines(True),fromfile='frozen299/'+name,tofile='layout-b94/'+name))
(R/'external-integration.patch').write_text(''.join(patch));print(json.dumps({'prepared':str(R),'runtimeLaunched':False,'source':SOURCE,'unchangedActionModules':True,'diagnosticRounds':8}))
