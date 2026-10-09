#!/usr/bin/env python3
"""Inert source assembly for an explicit separate settled-DOM control."""
from pathlib import Path
import hashlib,json,shutil,subprocess,difflib
HERE=Path(__file__).resolve().parent
BASE=Path('/workspace/.luci-tools/cloud032-await-boundary-edd0-runtime-v1/capsule')
PRODUCT='edd0cb121ff6847f681ab6bf41509a497f043d41';HELPER='4d6595072888bfa06fed0070d9c63b4cc1e6d1a9';REPO=Path('/workspace/luci')
sha=lambda p:hashlib.sha256(p.read_bytes()).hexdigest()
def put(name,value):(HERE/name).write_text(json.dumps(value,indent=2)+'\n')
def once(text,a,b):assert text.count(a)==1,a;return text.replace(a,b)
def main():
 assert sha(BASE/'manifest.json')=='5c1dc89cff6e18b799d87e7e5458683a378dd222cd5cb26cc901336083bb3fab'
 shutil.copytree(BASE/'actual',HERE/'actual')
 for name in ['settled-control.mjs','dependency-binding.py']:shutil.copyfile(HERE/name,HERE/'actual'/name)
 shutil.copyfile(HERE/'dependencies/manifest.json',HERE/'actual/dependency-manifest.json')
 capture=json.loads((BASE/'template-capture.json').read_text())
 for path,row in capture['sources'].items():
  data=subprocess.check_output(['git','show',HELPER+':'+path],cwd=REPO,timeout=10);assert hashlib.sha256(data).hexdigest()==row['sha256'] and len(data)==row['bytes']
 capture.update(capturedHEAD=HELPER,scope='Exact 17 original helper inputs matched committed4d6595 bytes; no inference that current product equals installed edd0.');put('template-capture.json',capture)
 original=(BASE/'actual/sustained.mjs').read_text();driver=original
 driver=once(driver,"import {createReadinessLedger} from './readiness-ledger.mjs';", "import {createReadinessLedger} from './readiness-ledger.mjs';\nimport {runSettledControl} from './settled-control.mjs';")
 driver=once(driver,"const packageIntegrity = JSON.parse(execFileSync", "const verifyPackageIntegrity = () => JSON.parse(execFileSync")
 driver=once(driver,"assert.equal(realpathSync(web),", "const packageIntegrity=verifyPackageIntegrity();\nassert.equal(realpathSync(web),")
 driver=once(driver,"const driverNames=['sustained.mjs',", "const driverNames=['settled-control.mjs','dependency-binding.py','dependency-manifest.json','sustained.mjs',")
 driver=once(driver,"mode:'installed-four-project-await-boundary-edd0-diagnostic',", "mode:'installed-four-project-settled-evaluate-control',")
 driver=once(driver,"scope:'Fresh installed package, exact original four saved contexts and paused queues, original draft/warm setup and idle100 switches/200 inputs. No task execution or sustained completion claim.',", "scope:'Fresh installed edd0 package, four exact saved contexts and paused queues;32 settled-DOM control evaluations after original draft/warm setup and60s quiet. Original400+32 latency cohort is not executed or replaced.',")
 driver=once(driver,"resourceScope:'One owned browser and installed app. Docker readiness/image inventory reads are required. Explicit guards reject every Create/Start before forwarding. No full Linux resource sampler; browser diagnostic collection is identical to the static control.',", "resourceScope:'One owned browser and installed app; same real DOM in both settled arms. Explicit Docker endpoint and unchanged unconditional Create/Start refusals. No static page, worker, provider request or performance-acceptance claim.',")
 driver=once(driver,"maxMeasuredActions:432,primaryMeasuredActions:400,optionalTraceMeasuredActions:32,", "maxMeasuredActions:0,primaryMeasuredActions:0,optionalTraceMeasuredActions:0,settledControlEvaluations:32,originalReferenceActionCount:432,")
 driver=once(driver,"receipt.predeclaredTimingReason='Prepared idle subset:60s quiet,100 switches/200 inputs/100 resets;210s workflow,60s preparation and120s cleanup. No extension on failure.';", "receipt.predeclaredTimingReason='Separate settled real-DOM control:60s quiet then32 balanced locator/retained-handle evaluations,60s control phase/five-second call caps; original210s workflow/60s preparation/120s cleanup remain. No extension, retry or acceptance replacement.';")
 start=driver.index(" setStage('idle-baseline');")
 end=driver.index(" assert.deepEqual(effects(),",start)
 removed=driver[start:end]
 replacement=""" setStage('settled-control-baseline');await quietPhase('quiet-app-open-idle');
 receipt.originalActionCohortExecuted=false;receipt.acceptanceReplacement=false;
 receipt.originalLatencyTargetReference={projectSwitchP95Ms:300,inputP95Ms:100,p99StrictlyBelowMs:1000,measuredHere:false};
 setStage('settled-evaluate-control');const controlPhase=phaseStart('settled-real-dom-control');
 await runSettledControl({page,owners:projectStates,receipt,assertHealthy:health});phaseEnd(controlPhase);
 await passiveTiming('settled-control-end');assert.equal(receipt.interactions.length,0);assert.equal(receipt.settledEvaluateControl.rows.length,32);
"""
 driver=driver[:start]+replacement+driver[end:]
 driver=once(driver,"receipt.idleDiagnosticCompleted=true;receipt.passed=true;", "receipt.settledControlCompleted=true;receipt.passed=true;")
 driver=once(driver,"check('Fresh installed idle diagnostic completed; no full sustained or latency-target success implied');", "check('Separate settled-DOM control completed; original latency cohort not executed and no acceptance replacement');")
 needle="try{assert(startupScope.snapshot().every(row=>row.state==='closed'&&!row.closeTimedOut),'Startup resource remains unsettled');"
 insert="""try{receipt.packageIntegrityAfter=verifyPackageIntegrity();assert.deepEqual(receipt.packageIntegrityAfter,packageIntegrity);
  const dependencyAfter=join(evidence,'dependency-after.json');execFileSync('python3',[join(here,'dependency-binding.py'),'verify',join(here,'dependency-manifest.json'),dependencyAfter],{timeout:Math.ceil(commandBudget(30000)),maxBuffer:1024*1024});receipt.dependencyAfter=JSON.parse(readFileSync(dependencyAfter));assert.equal(receipt.dependencyAfter.passed,true);
  assert(startupScope.snapshot().every(row=>row.state==='closed'&&!row.closeTimedOut),'Startup resource remains unsettled');"""
 driver=once(driver,needle,insert)
 driver=once(driver,'assertWorkflowActive();receipt.browserVersion=browser.version();page.setDefaultTimeout(15000);',"assertWorkflowActive();receipt.browserVersion=browser.version();receipt.browserProcessIdentitiesAtSetup=ownedProcesses('browser');page.setDefaultTimeout(15000);")
 (HERE/'actual/sustained.mjs').write_text(driver)
 binding=json.loads((BASE/'actual/external-source-binding.json').read_text());binding.update(executionDirectory=str(HERE/'actual'),prospectiveProduct=PRODUCT,prospectiveHelper=HELPER,scope='Separate32-call settled real-DOM control. Original432 action acceptance cohort unexecuted; no adjustment/replacement. Same callback/element identity, balanced orders, explicit endpoint, package/dependency before/after. Original remaining guards/lifecycle retained.',controlRevision=1,controlSourceSHA256=sha(HERE/'actual/settled-control.mjs'),dependencyManifestSHA256=sha(HERE/'actual/dependency-manifest.json'),dependencyCheckerSHA256=sha(HERE/'actual/dependency-binding.py'),originalFullActionCohortExecuted=False)
 for name in ['settled-control.mjs','dependency-binding.py','dependency-manifest.json']:binding['localSources'][name]=sha(HERE/'actual'/name)
 for name in binding['localSources']:binding['localSources'][name]=sha(HERE/'actual'/name)
 put('actual/external-source-binding.json',binding)
 old=(BASE/'launch.py').read_text();launcher=old.replace('8c7bf16aa035b27dbea185b565052495eb60777f',HELPER).replace('8c7 helper','4d6595 helper')
 launcher=once(launcher,"    parser.add_argument('--trace-after',choices=['yes','no'],default='yes')\n",'')
 launcher=once(launcher,"    assert args.trace_after=='yes','Original complete 400 + 32 action counts required'\n",'')
 launcher=once(launcher,",'--trace-after',args.trace_after",'')
 launcher=once(launcher,"'primarySwitches':100,'primaryInputs':200,'primaryResets':100,'extraTraceRounds':8 if args.trace_after=='yes' else 0,", "'primarySwitches':0,'primaryInputs':0,'primaryResets':0,'extraTraceRounds':0,'originalActionCohortExecuted':False,'settledControlCalls':32,'settledControlPhaseSeconds':60,'settledControlCallSeconds':5,")
 launcher=once(launcher,"    result_dir=output/'driver';result=result_dir/'result.json'", """    dependency_before=output/'dependency-before.json'
    subprocess.run(['python3',str(HERE/'actual/dependency-binding.py'),'verify',str(HERE/'actual/dependency-manifest.json'),str(dependency_before)],check=True,timeout=30,capture_output=True)
    assert json.loads(dependency_before.read_text())['passed'] is True
    protected={name:sha(path) for name,path in {'launcher':Path(__file__),'binding':HERE/'actual/external-source-binding.json','capture':HERE/'template-capture.json','provenance':args.provenance,'image-provenance':args.image_provenance,**{'source/'+name:HERE/'actual'/name for name in binding['localSources']}}.items()}
    result_dir=output/'driver';result=result_dir/'result.json'""")
 launcher=once(launcher,"'provenanceSHA256':sha(args.provenance),", "'provenanceSHA256':sha(args.provenance),'dependencyBeforeSHA256':sha(dependency_before),'protectedInputSHA256':protected,")
 launcher=once(launcher,"    receipt={'startedAt':started,", """    dependency_after=output/'dependency-after-wrapper.json';dependency_check=subprocess.run(['python3',str(HERE/'actual/dependency-binding.py'),'verify',str(HERE/'actual/dependency-manifest.json'),str(dependency_after)],timeout=30,capture_output=True)
    after={name:sha(path) for name,path in {'launcher':Path(__file__),'binding':HERE/'actual/external-source-binding.json','capture':HERE/'template-capture.json','provenance':args.provenance,'image-provenance':args.image_provenance,**{'source/'+name:HERE/'actual'/name for name in binding['localSources']}}.items()}
    inputs_stable=after==protected and dependency_check.returncode==0
    receipt={'protectedInputsUnchanged':after==protected,'dependencyAfterPassed':dependency_check.returncode==0,'dependencyAfterSHA256':sha(dependency_after) if dependency_after.is_file() else None,'startedAt':started,""")
 launcher=once(launcher,"return 124 if timeout else child.returncode if child.returncode is not None else 1", "return 124 if timeout else 1 if not inputs_stable else child.returncode if child.returncode is not None else 1")
 launcher=once(launcher,"    dependency_after=output/'dependency-after-wrapper.json';dependency_check=subprocess.run(['python3',str(HERE/'actual/dependency-binding.py'),'verify',str(HERE/'actual/dependency-manifest.json'),str(dependency_after)],timeout=30,capture_output=True)\n    after={name:sha(path) for name,path in {'launcher':Path(__file__),'binding':HERE/'actual/external-source-binding.json','capture':HERE/'template-capture.json','provenance':args.provenance,'image-provenance':args.image_provenance,**{'source/'+name:HERE/'actual'/name for name in binding['localSources']}}.items()}\n    inputs_stable=after==protected and dependency_check.returncode==0\n    receipt={'protectedInputsUnchanged':after==protected,'dependencyAfterPassed':dependency_check.returncode==0,'dependencyAfterSHA256':sha(dependency_after) if dependency_after.is_file() else None,'startedAt':started,","    dependency_after=output/'dependency-after-wrapper.json';postcheck_error=None;dependency_passed=False;protected_unchanged=False\n    try:\n        dependency_check=subprocess.run(['python3',str(HERE/'actual/dependency-binding.py'),'verify',str(HERE/'actual/dependency-manifest.json'),str(dependency_after)],timeout=30,capture_output=True)\n        dependency_passed=dependency_check.returncode==0\n        after={name:sha(path) for name,path in {'launcher':Path(__file__),'binding':HERE/'actual/external-source-binding.json','capture':HERE/'template-capture.json','provenance':args.provenance,'image-provenance':args.image_provenance,**{'source/'+name:HERE/'actual'/name for name in binding['localSources']}}.items()}\n        protected_unchanged=after==protected\n    except (OSError,subprocess.TimeoutExpired,ValueError) as error:\n        postcheck_error=type(error).__name__\n    inputs_stable=protected_unchanged and dependency_passed\n    receipt={'protectedInputsUnchanged':protected_unchanged,'dependencyAfterPassed':dependency_passed,'postcheckErrorType':postcheck_error,'dependencyAfterSHA256':sha(dependency_after) if dependency_after.is_file() else None,'startedAt':started,")
 launcher=once(launcher,"'outerKillWaitSeconds':10,","'outerKillWaitSeconds':10,'postDriverDependencyCheckSeconds':30,")
 (HERE/'launch.py').write_text(launcher)
 patches=[]
 for name,before,after in [('sustained.mjs',original,driver),('launch.py',old,launcher)]:patches.extend(difflib.unified_diff(before.splitlines(keepends=True),after.splitlines(keepends=True),fromfile='await-boundary/'+name,tofile='settled-control/'+name))
 (HERE/'integration.patch').write_text(''.join(patches));(HERE/'removed-original-cohort.txt').write_text(removed)
 command=json.loads((BASE/'launch-command.json').read_text());argv=command['command'];argv[1]=str(HERE/'launch.py');argv[argv.index('--helper-source')+1]=HELPER;argv[argv.index('--output')+1]='/workspace/luci/artifacts/quality-swarm/settled-evaluate-edd0cb12-attempt1';i=argv.index('--trace-after');del argv[i:i+2];command['scope']='Prepared only. Root must approve/co-ordinate exclusive window; no original400+32 cohort or acceptance replacement. Productedd0; helper4d6595.';put('launch-command.json',command)
 put('preparation.json',{'productSource':PRODUCT,'helperSource':HELPER,'same17CommittedHelperInputs':True,'originalProductSourceUnchanged':True,'newControlCalls':32,'originalFullActionCohortExecuted':False,'removedOriginalCohortSHA256':hashlib.sha256(removed.encode()).hexdigest(),'originalGuardsAndLifecycleOtherwiseRetained':True,'installedRun':False,'dependencyManifestSHA256':sha(HERE/'dependencies/manifest.json'),'driverSHA256':sha(HERE/'actual/sustained.mjs'),'launcherSHA256':sha(HERE/'launch.py'),'bindingSHA256':sha(HERE/'actual/external-source-binding.json')})
 print(json.dumps({'prepared':True,'product':PRODUCT,'helper':HELPER,'calls':32,'installedRun':False}))
if __name__=='__main__':main()
