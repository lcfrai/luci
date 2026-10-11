#!/usr/bin/env python3
"""One root-authorized targeted actual-working-tree frontend batch plus full tsc."""
from pathlib import Path
import importlib.util,json,os,shutil,signal,subprocess,sys
B=Path(__file__).resolve().parent;R=Path('/workspace/luci');D=R/'desktop'
os.umask(0o077);out=B/'frontend';out.mkdir(exist_ok=False)
spec=importlib.util.spec_from_file_location('binding',B/'binding-and-execution.py');binding=importlib.util.module_from_spec(spec);spec.loader.exec_module(binding)
node=shutil.which('node');assert node
allowed=['PATH','HOME','LANG','LC_ALL','TZ','TMPDIR','TEMP','TMP','SystemRoot'];env={k:os.environ[k] for k in allowed if k in os.environ};env.update(CI='1',NO_COLOR='1')
version=subprocess.check_output([node,'--version'],text=True,env=env,timeout=10).strip();assert version=='v24.19.0',version
selected=['src/components/AppShell.test.tsx','src/components/definition/DefinitionWorkspace.test.tsx','src/components/orchestrator/OrchestratorPanel.test.tsx','src/components/orchestrator/ProjectUnderstanding.test.tsx','src/components/journey/ProjectPlan.test.tsx','src/components/journey/ProjectWork.test.tsx','src/components/setup/SetupPanel.test.tsx','src/components/setup/WorkspaceInstaller.test.tsx','src/components/projects/ProjectEntry.test.tsx']
assert all((D/rel).is_file() for rel in selected)
record={'startedAt':binding.now(),'nodeVersion':version,'headBefore':binding.git('rev-parse','HEAD'),'worktreeStatusBefore':binding.git('status','--porcelain=v1','--untracked-files=all'),'effectiveSource':'Actual root working tree HEAD6f00501d plus frozen LOCAL-023 and project-name guidance changes; no overlays or source edits by runner.','cleanHEADClaim':False,'selectedVitestFiles':selected,'maxWorkers':2,'environmentNames':sorted(env),'explicitEnvironment':{'CI':'1','NO_COLOR':'1'},'runner':binding.filemeta(Path(__file__)),'bindingHelper':binding.filemeta(B/'binding-and-execution.py'),'commands':[]}
binding.write(out/'invocation.json',record)
def cancel(sig,frame):raise InterruptedError('Runner received signal '+str(sig))
signal.signal(signal.SIGTERM,cancel)
try:
 before=binding.inputs();deps_before=binding.deps(node);binding.write(out/'source-before.json',before);binding.write(out/'dependencies-before.json',deps_before)
 vitest=binding.execute('vitest',[node,str(D/'node_modules/vitest/vitest.mjs'),'run',*selected,'--maxWorkers=2','--reporter=json','--outputFile='+str(out/'vitest.json')],300,out,env);record['commands'].append(vitest)
 after_vitest=binding.inputs();deps_after_vitest=binding.deps(node);binding.write(out/'source-after-vitest.json',after_vitest);binding.write(out/'dependencies-after-vitest.json',deps_after_vitest)
 record['vitestSourceStable']=before==after_vitest;record['vitestDependenciesStable']=deps_before==deps_after_vitest
 if (out/'vitest.json').exists():
  raw=json.loads((out/'vitest.json').read_bytes());record['vitest']={k:raw.get(k) for k in ['numTotalTests','numPassedTests','numFailedTests','numPendingTests','numTodoTests','numTotalTestSuites','numPassedTestSuites','numFailedTestSuites','success']}
  record['vitestReporter']=binding.filemeta(out/'vitest.json');record['reportedTestFiles']=[x['name'] for x in raw.get('testResults',[])];record['selectedFilesMatch']=set(record['reportedTestFiles'])=={str(D/rel) for rel in selected}
  record['failedTests']=[{'file':x['name'],'title':a.get('fullName'),'status':a['status'],'failureMessages':a.get('failureMessages',[])} for x in raw.get('testResults',[]) for a in x.get('assertionResults',[]) if a.get('status')=='failed']
 print(json.dumps({'vitest':record.get('vitest'),'failedTests':record.get('failedTests'),'inputsStable':record['vitestSourceStable'],'dependenciesStable':record['vitestDependenciesStable']}),flush=True)
 binding.write(out/'intermediate.json',record)
 record['commands'].append(binding.execute('typecheck',[node,str(D/'node_modules/typescript/bin/tsc'),'--noEmit'],180,out,env))
 after=binding.inputs();deps_after=binding.deps(node);binding.write(out/'source-after.json',after);binding.write(out/'dependencies-after.json',deps_after)
 record.update(inputCount=before['count'],sourceStable=before==after,dependenciesStable=deps_before==deps_after,typecheckSourceStable=after_vitest==after,typecheckDependenciesStable=deps_after_vitest==deps_after,sourceChanges=[k for k in sorted(set(before['files'])|set(after['files'])) if before['files'].get(k)!=after['files'].get(k)])
 record['passed']=len(record['commands'])==2 and all(c['exitCode']==0 and not c.get('failure') for c in record['commands']) and record['sourceStable'] and record['dependenciesStable'] and record['vitestSourceStable'] and record['vitestDependenciesStable'] and record.get('vitest',{}).get('success') is True and record.get('selectedFilesMatch') is True
except BaseException as error:
 record['passed']=False;record['runnerFailure']={'type':type(error).__name__,'message':str(error)}
finally:
 proc=binding.ACTIVE
 if proc is not None and proc.poll() is None:
  signal.signal(signal.SIGTERM,signal.SIG_IGN)
  try:os.killpg(proc.pid,signal.SIGTERM)
  except ProcessLookupError:pass
  try:proc.wait(timeout=10)
  except subprocess.TimeoutExpired:
   try:os.killpg(proc.pid,signal.SIGKILL)
   except ProcessLookupError:pass
   proc.wait(timeout=10)
  record['interruptedChildCleanup']={'pid':proc.pid,'exitCode':proc.returncode,'scope':'Owned process group only;20second cleanup bound'}
 record.update(headAfter=binding.git('rev-parse','HEAD'),worktreeStatusAfter=binding.git('status','--porcelain=v1','--untracked-files=all'),completedAt=binding.now())
 binding.write(out/'result.json',record)
 print(json.dumps({k:record.get(k) for k in ['passed','inputCount','sourceStable','dependenciesStable','selectedFilesMatch','vitest','failedTests','sourceChanges','runnerFailure']},indent=2),flush=True)
sys.exit(0 if record.get('passed') else 1)
