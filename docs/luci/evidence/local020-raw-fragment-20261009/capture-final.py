from pathlib import Path
import json,hashlib,subprocess,datetime,time,shutil,tempfile
repo=Path('/workspace/luci');private=Path('/workspace/.luci-tools/local020-raw-fragment');out=repo/'docs/luci/evidence/local020-raw-fragment-20261009';out.mkdir(exist_ok=False)
sha=lambda b:hashlib.sha256(b).hexdigest()
paths=['scripts/acceptance/launch-recovery/fenced-docker-proxy.mjs','scripts/acceptance/check-create-recovery/proxy-recovery.test.mjs','scripts/acceptance/check-create-recovery/sdk-stats-wire.json','scripts/acceptance/check-create-recovery/recovery-scenario.mjs']
snapshot=lambda:[{'path':p,'bytes':len((repo/p).read_bytes()),'sha256':sha((repo/p).read_bytes())} for p in paths]
before=snapshot();head=subprocess.check_output(['git','rev-parse','HEAD'],cwd=repo,text=True).strip()
for p in paths:
 target=out/'tested-sources'/p;target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes((repo/p).read_bytes())
for p in private.iterdir():
 if p.is_file() and p.name!='capture-final.py':
  target=out/'retained-inputs'/p.name;target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(p.read_bytes())
pin='81f2ba5b194a81a4f2ad167e466793f59162b387';pinned=[]
for name in ['fenced-docker-proxy.mjs','fenced-docker-proxy.test.mjs','acceptance-lifecycle.mjs','durable-acceptance-journal.mjs']:
 path='scripts/acceptance/launch-recovery/'+name;b=subprocess.check_output(['git','show',pin+':'+path],cwd=repo);target=out/'pinned-inputs'/name;target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(b);pinned.append({'commit':pin,'path':path,'bytes':len(b),'sha256':sha(b)})
commands=[]
def run(label,args,cwd=repo,expected=0):
 started=datetime.datetime.now(datetime.timezone.utc).isoformat();t=time.monotonic();r=subprocess.run(args,cwd=cwd,capture_output=True,timeout=45)
 (out/(label+'.stdout.log')).write_bytes(r.stdout);(out/(label+'.stderr.log')).write_bytes(r.stderr)
 commands.append({'label':label,'command':args,'cwd':'repository root' if cwd==repo else 'isolated frozen sources, original 2da proxy only','startedAtUTC':started,'durationSeconds':time.monotonic()-t,'exitCode':r.returncode,'expectedExitCode':expected,'stdoutSHA256':sha(r.stdout),'stderrSHA256':sha(r.stderr)})
 assert r.returncode==expected,(label,r.stdout[-1600:],r.stderr)
with tempfile.TemporaryDirectory(prefix='local020-before-') as temp:
 root=Path(temp)
 for p in paths:
  target=root/p;target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes((repo/p).read_bytes())
 (root/paths[0]).write_bytes((private/'original-fenced-docker-proxy.mjs').read_bytes())
 run('original-bound-regression',['node','--test','--test-timeout=30000','--test-reporter=spec','--test-name-pattern=known SDK check query guard rejects raw fragments|raw fragment guard preserves generic','scripts/acceptance/check-create-recovery/proxy-recovery.test.mjs'],root,1)
run('final-proxy-affected',['node','--test','--test-timeout=30000','--test-reporter=spec','scripts/acceptance/check-create-recovery/proxy-recovery.test.mjs'])
for p in paths[:2]:run('syntax-'+Path(p).name,['node','--check',p])
after=snapshot();assert before==after
assert (private/'original-fenced-docker-proxy.mjs').read_bytes()==subprocess.check_output(['git','show',head+':'+paths[0]],cwd=repo)
assert (repo/paths[3]).read_bytes()==subprocess.check_output(['git','show',head+':'+paths[3]],cwd=repo)
(out/'source-bindings.json').write_text(json.dumps({'baseCommit':head,'scope':'four selected proxy/test/fixture/scenario repository inputs and four explicitly pinned Git inputs; controlled HTTP fixtures only, not installed/product/full-suite evidence','before':before,'after':after,'unchangedDuringValidation':True,'originalProxyMatchesBaseCommit':True,'scenarioAndFinalFailureAssertionUnchanged':True,'pinnedInputs':pinned,'nodeVersion':subprocess.check_output(['node','--version'],text=True).strip()},indent=2)+'\n')
(out/'commands.json').write_text(json.dumps(commands,indent=2)+'\n')
(out/'candidate.patch').write_bytes(subprocess.check_output(['git','diff','--',paths[0],paths[1]],cwd=repo))
shutil.copyfile(__file__,out/'capture-final.py')
print(json.dumps({'directory':str(out),'commands':[{k:r[k] for k in ['label','exitCode']} for r in commands]}))
