#!/usr/bin/env python3
from pathlib import Path
import importlib.util,json,os,shutil,signal,subprocess,sys
B=Path(__file__).resolve().parent;REAL=Path('/workspace/luci');out=B/'ui-validation'/sys.argv[1];out.mkdir(parents=True,exist_ok=False)
spec=importlib.util.spec_from_file_location('binding',B/'ui-binding.py');binding=importlib.util.module_from_spec(spec);spec.loader.exec_module(binding)
node=shutil.which('node');assert node
allowed=['PATH','HOME','LANG','LC_ALL','TZ','TMPDIR','TEMP','TMP','SystemRoot'];env={k:os.environ[k] for k in allowed if k in os.environ};env.update(CI='1',NO_COLOR='1')
version=subprocess.check_output([node,'--version'],text=True,env=env,timeout=10).strip();assert version=='v24.19.0',version
base=binding.inputs();frozen=out/'frozen';frozen.mkdir()
for relative in base['files']:
 dst=frozen/relative;dst.parent.mkdir(parents=True,exist_ok=True);shutil.copyfile(REAL/relative,dst)
overlays={}
for path in sorted((B/'tree/desktop').rglob('*')):
 if not path.is_file():continue
 relative=path.relative_to(B/'tree');target=frozen/relative;target.parent.mkdir(parents=True,exist_ok=True);shutil.copyfile(path,target);overlays[str(relative)]=binding.filemeta(path)
(frozen/'desktop/node_modules').symlink_to(REAL/'desktop/node_modules',target_is_directory=True)
binding.R=frozen;binding.D=frozen/'desktop';D=binding.D
selected=['src/components/setup/RuntimePreparation.test.tsx','src/components/setup/LocalTools.test.tsx','src/components/setup/RuntimeCancellation.test.tsx']
record={'startedAt':binding.now(),'sourceHEAD':subprocess.check_output(['git','-C',str(REAL),'rev-parse','HEAD'],text=True).strip(),'scope':'Frozen full UI source/config/raw-import inputs from root plus exact C081 frontend overlay. Base Go/raw inputs are captured as frontend support, not compiled or claimed C081 backend validation. Existing node_modules shared by symlink; bounded dependency fingerprints recorded. No repository source edits.','frozenRoot':str(frozen),'nodeVersion':version,'selectedVitestFiles':selected,'maxWorkers':1,'explicitEnvironment':{'CI':'1','NO_COLOR':'1'},'overlay':overlays,'commands':[]}
binding.write(out/'root-inputs-before-copy.json',base);binding.write(out/'invocation.json',record)
try:
 before=binding.inputs();deps=binding.deps(node);binding.write(out/'source-before.json',before);binding.write(out/'dependencies-before.json',deps)
 row=binding.execute('vitest',[node,str(D/'node_modules/vitest/vitest.mjs'),'run',*selected,'--maxWorkers=1','--reporter=json','--outputFile='+str(out/'vitest.json')],300,out,env);record['commands'].append(row)
 aftervitest=binding.inputs();afterdeps=binding.deps(node);binding.write(out/'source-after-vitest.json',aftervitest);binding.write(out/'dependencies-after-vitest.json',afterdeps)
 record['vitestSourceStable']=before==aftervitest;record['vitestDependenciesStable']=deps==afterdeps
 if (out/'vitest.json').exists():
  raw=json.loads((out/'vitest.json').read_bytes());record['vitest']={k:raw.get(k) for k in ['numTotalTests','numPassedTests','numFailedTests','numPendingTests','numTodoTests','numTotalTestSuites','numPassedTestSuites','numFailedTestSuites','success']};record['reportedTestFiles']=[x['name'] for x in raw.get('testResults',[])];record['selectedFilesMatch']=set(record['reportedTestFiles'])=={str(D/rel) for rel in selected};record['failures']=[{'file':x['name'],'title':a.get('fullName'),'failureMessages':a.get('failureMessages')} for x in raw.get('testResults',[]) for a in x.get('assertionResults',[]) if a.get('status')=='failed']
 binding.write(out/'intermediate.json',record);print(json.dumps({'vitest':record.get('vitest'),'failures':record.get('failures')}),flush=True)
 record['commands'].append(binding.execute('typecheck',[node,str(D/'node_modules/typescript/bin/tsc'),'--noEmit'],180,out,env))
 after=binding.inputs();depsafter=binding.deps(node);binding.write(out/'source-after.json',after);binding.write(out/'dependencies-after.json',depsafter)
 record.update(inputCount=before['count'],sourceStable=before==after,dependenciesStable=deps==depsafter)
 record['passed']=all(x['exitCode']==0 and not x.get('failure') for x in record['commands']) and record['sourceStable'] and record['dependenciesStable'] and record['vitestSourceStable'] and record['vitestDependenciesStable'] and record.get('selectedFilesMatch') is True and record.get('vitest',{}).get('success') is True
except BaseException as error:
 record['passed']=False;record['runnerFailure']={'type':type(error).__name__,'message':str(error)}
finally:
 proc=binding.ACTIVE
 if proc is not None and proc.poll() is None:
  try:os.killpg(proc.pid,signal.SIGTERM)
  except ProcessLookupError:pass
  try:proc.wait(timeout=10)
  except subprocess.TimeoutExpired:
   try:os.killpg(proc.pid,signal.SIGKILL)
   except ProcessLookupError:pass
   proc.wait(timeout=10)
 record['completedAt']=binding.now();binding.write(out/'result.json',record);print(json.dumps({k:record.get(k) for k in ['passed','inputCount','sourceStable','dependenciesStable','vitest','failures','runnerFailure']},indent=2),flush=True)
sys.exit(0 if record.get('passed') else 1)
