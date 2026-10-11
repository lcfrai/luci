#!/usr/bin/env python3
from pathlib import Path
import datetime, hashlib, json, os, shutil, signal, subprocess, sys, time
B=Path(__file__).resolve().parent
label=sys.argv[1]
assert label in ['before','after']
out=B/'runs'/label
out.mkdir(parents=True,exist_ok=False)
node=shutil.which('node')
version=subprocess.check_output([node,'--version'],text=True).strip()
assert version=='v24.19.0',version
D=B/'tree/desktop'
cmd=[node,str(D/'node_modules/vitest/vitest.mjs'),'run','src/seams.test.ts','--maxWorkers=1','--no-file-parallelism','--reporter=json','--outputFile='+str(out/'vitest.json')]
def snapshot():
 paths=json.loads((B/'frozen-inputs.json').read_text())['files']
 return {rel:hashlib.sha256((B/'tree'/rel).read_bytes()).hexdigest() for rel in paths}
before=snapshot()
(out/'source-before.json').write_text(json.dumps(before,indent=2)+'\n')
env={k:os.environ[k] for k in ['PATH','HOME','LANG','LC_ALL','TZ','TMPDIR','TEMP','TMP'] if k in os.environ};env.update(CI='1',NO_COLOR='1')
row={'label':label,'command':cmd,'cwd':str(D),'nodeVersion':version,'startedAt':datetime.datetime.now(datetime.timezone.utc).isoformat(),'timeoutSeconds':240,'maxWorkers':1,'scope':'Only src/seams.test.ts; unchanged Vitest configuration in frozen external copy; shared installed dependencies. No tsc, full-suite, runtime or Docker.'}
(out/'invocation.json').write_text(json.dumps(row,indent=2)+'\n')
start=time.monotonic()
with (out/'stdout.log').open('xb') as so,(out/'stderr.log').open('xb') as se:
 p=subprocess.Popen(cmd,cwd=D,env=env,stdout=so,stderr=se,start_new_session=True)
 try:p.wait(timeout=240)
 except subprocess.TimeoutExpired:
  row['failure']='timeout'
  os.killpg(p.pid,signal.SIGTERM)
  try:p.wait(timeout=10)
  except subprocess.TimeoutExpired:
   os.killpg(p.pid,signal.SIGKILL);p.wait(timeout=10)
after=snapshot()
(out/'source-after.json').write_text(json.dumps(after,indent=2)+'\n')
row.update(exitCode=p.returncode,elapsedSeconds=time.monotonic()-start,completedAt=datetime.datetime.now(datetime.timezone.utc).isoformat(),inputsStable=before==after)
if (out/'vitest.json').exists():
 report=json.loads((out/'vitest.json').read_text())
 row['reporter']={k:report[k] for k in ['numTotalTests','numPassedTests','numFailedTests','numPendingTests','numTodoTests','success']}
 row['failures']=[{'file':t['name'],'title':a['fullName'],'messages':a['failureMessages']} for t in report['testResults'] for a in t['assertionResults'] if a['status']=='failed']
row['artifacts']={p.name:{'bytes':p.stat().st_size,'sha256':hashlib.sha256(p.read_bytes()).hexdigest()} for p in out.iterdir() if p.is_file()}
(out/'result.json').write_text(json.dumps(row,indent=2)+'\n')
print(json.dumps(row,indent=2))
sys.exit(p.returncode if row['inputsStable'] else 2)
