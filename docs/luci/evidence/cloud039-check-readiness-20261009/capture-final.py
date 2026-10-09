from pathlib import Path
import hashlib,json,subprocess,time,datetime,shutil,os,tempfile
repo=Path('/workspace/luci'); private=Path('/workspace/.luci-tools/cloud039-check-readiness')
out=repo/'docs/luci/evidence/cloud039-check-readiness-20261009'; out.mkdir(exist_ok=False)
def sha(data):return hashlib.sha256(data).hexdigest()
def write(name,obj): (out/name).write_text(json.dumps(obj,indent=2)+'\n')
paths=sorted([p.relative_to(repo).as_posix() for p in (repo/'scripts/acceptance/check-create-recovery').iterdir() if p.is_file()]+['scripts/acceptance/launch-recovery/'+n for n in ['acceptance-lifecycle.mjs','acceptance-startup.mjs','durable-acceptance-journal.mjs','fenced-docker-proxy.mjs']]+['scripts/acceptance/four-project-queue/sample-linux-owned.py'])
def snapshot():return [{'path':p,'bytes':len((repo/p).read_bytes()),'sha256':sha((repo/p).read_bytes())} for p in paths]
before=snapshot(); head=subprocess.check_output(['git','rev-parse','HEAD'],cwd=repo,text=True).strip()
for p in paths:
 target=out/'tested-sources'/p;target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes((repo/p).read_bytes())
for root,name in [(private/'before','original-aadc'),(private/'regression-before','historical-regression-instrumentation')]:
 for p in root.iterdir():
  target=out/name/p.name;target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(p.read_bytes())
for p in private.glob('*.log'):
 target=out/'historical-logs'/p.name;target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(p.read_bytes())
shutil.copyfile(private/'before.json',out/'original-bindings.json')
pin='81f2ba5b194a81a4f2ad167e466793f59162b387'; pinned=[]
for n in ['fenced-docker-proxy.mjs','fenced-docker-proxy.test.mjs','acceptance-lifecycle.mjs','durable-acceptance-journal.mjs']:
 p='scripts/acceptance/launch-recovery/'+n;b=subprocess.check_output(['git','show',pin+':'+p],cwd=repo);target=out/'pinned-proxy-inputs'/n;target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(b);pinned.append({'commit':pin,'path':p,'sha256':sha(b),'bytes':len(b)})
commands=[]
def run(label,args,cwd=repo,expected=0):
 start=datetime.datetime.now(datetime.timezone.utc).isoformat();t=time.monotonic()
 result=subprocess.run(args,cwd=cwd,capture_output=True,timeout=45)
 (out/(label+'.stdout.log')).write_bytes(result.stdout);(out/(label+'.stderr.log')).write_bytes(result.stderr)
 commands.append({'label':label,'command':args,'cwd':'repository root' if cwd==repo else 'isolated temporary copy; only recovery-scenario.mjs replaced with exact aadc original','startedAtUTC':start,'durationSeconds':time.monotonic()-t,'exitCode':result.returncode,'expectedExitCode':expected,'stdoutSHA256':sha(result.stdout),'stderrSHA256':sha(result.stderr)})
 assert result.returncode==expected,(label,result.returncode,result.stdout[-1000:],result.stderr[-1000:])
# A fresh source-bound counterexample changes only scenario production bytes.
with tempfile.TemporaryDirectory(prefix='cloud039-bound-before-') as temp:
 root=Path(temp)
 for p in paths:
  target=root/p;target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes((repo/p).read_bytes())
 target=root/'scripts/acceptance/check-create-recovery/recovery-scenario.mjs';target.write_bytes((private/'before/recovery-scenario.mjs').read_bytes())
 run('before-bound-regression',['node','--test','--test-reporter=spec','--test-timeout=30000','--test-name-pattern=actual scenario promptly reports submitted CPU refusal','scripts/acceptance/check-create-recovery/scenario-flow.test.mjs'],root,1)
run('final-controlled',['node','--test','--test-reporter=spec','--test-timeout=30000','--test-concurrency=2']+[str(p.relative_to(repo)) for p in sorted((repo/'scripts/acceptance/check-create-recovery').glob('*.test.mjs'))])
for name in ['fixture-guards.mjs','installed-recovery.mjs','recovery-scenario.mjs','scenario-flow.test.mjs','check-readiness.test.mjs']:
 run('syntax-'+name,['node','--check','scripts/acceptance/check-create-recovery/'+name])
after=snapshot();assert before==after
original=json.loads((private/'before.json').read_text())
for row in original['files']:
 b=subprocess.check_output(['git','show',head+':'+row['path']],cwd=repo);assert sha(b)==row['sha256'];assert b==(out/'original-aadc'/Path(row['path']).name).read_bytes()
write('source-bindings.json',{'baseCommit':head,'before':before,'after':after,'allSelectedInputsUnchanged':True,'scope':'All files in the check-create-recovery helper directory plus its four current shared launch modules and sampler; selected helper validation, not full repository/compiler provenance','pinnedProxyInputs':pinned,'originalSourcesMatchBaseCommit':True,'nodeVersion':subprocess.check_output(['node','--version'],text=True).strip()})
write('commands.json',commands)
(out/'candidate.patch').write_bytes(subprocess.check_output(['git','diff','--','scripts/acceptance/check-create-recovery'],cwd=repo))
shutil.copyfile(__file__,out/'capture-final.py')
print(json.dumps({'directory':str(out),'selectedSourceCount':len(paths),'commands':[{k:r[k] for k in ['label','exitCode']} for r in commands]}))
