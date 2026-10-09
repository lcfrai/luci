from pathlib import Path
import datetime,hashlib,json,subprocess,sys,time
ROOT=Path('/workspace/luci');BASE=Path('/workspace/.luci-tools/cloud032-discussion-mount/validation');BASE.mkdir(exist_ok=True);label=sys.argv[1];command=sys.argv[2:];OUT=BASE/label;OUT.mkdir();sha=lambda p:hashlib.sha256(p.read_bytes()).hexdigest()
def now():return datetime.datetime.now(datetime.timezone.utc).isoformat()
def save(n,j):(OUT/n).write_text(json.dumps(j,indent=2)+'\n')
def snapshot(n):
 paths=sorted(p for p in(ROOT/'desktop/src').rglob('*')if p.is_file());paths +=[ROOT/p for p in ['desktop/package.json','desktop/package-lock.json','desktop/tsconfig.json','desktop/tsconfig.node.json','desktop/vite.config.ts','desktop/vitest.config.ts']]
 files={str(p.relative_to(ROOT)):{'sha256':sha(p),'bytes':p.stat().st_size}for p in paths};j={'at':now(),'commit':subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT,text=True).strip(),'status':subprocess.check_output(['git','status','--porcelain=v1'],cwd=ROOT,text=True).splitlines(),'files':files};save(n,j);return j
before=snapshot('source-before.json');start=now();tic=time.monotonic();save('invocation.json',{'command':command,'cwd':str(ROOT/'desktop'),'activation':'source /workspace/.luci-tools/activate.sh','startedAt':start,'label':label,'scope':'Controlled frontend verification only; no browser/app/runtime. No full frontend suite.'})
with(OUT/'stdout.log').open('wb')as stdout,(OUT/'stderr.log').open('wb')as stderr:result=subprocess.run(command,cwd=ROOT/'desktop',stdout=stdout,stderr=stderr,check=False)
after=snapshot('source-after.json');changed=[p for p in sorted(set(before['files'])|set(after['files']))if before['files'].get(p)!=after['files'].get(p)]
receipt={'command':command,'cwd':str(ROOT/'desktop'),'startedAt':start,'finishedAt':now(),'durationSeconds':time.monotonic()-tic,'exitCode':result.returncode,'sourceInputsUnchanged':not changed,'changedPaths':changed,'stdout':{'bytes':(OUT/'stdout.log').stat().st_size,'sha256':sha(OUT/'stdout.log')},'stderr':{'bytes':(OUT/'stderr.log').stat().st_size,'sha256':sha(OUT/'stderr.log')}};save('result.json',receipt);print(json.dumps(receipt),flush=True);sys.exit(result.returncode)
