from pathlib import Path
import hashlib,json,subprocess,time,datetime,shutil,tempfile
repo=Path('/workspace/luci'); private=Path('/workspace/.luci-tools/cloud040-stats'); out=repo/'docs/luci/evidence/cloud040-sdk-check-stats-20261009';out.mkdir(exist_ok=False)
sha=lambda b:hashlib.sha256(b).hexdigest()
paths=sorted([p.relative_to(repo).as_posix() for p in (repo/'scripts/acceptance/check-create-recovery').iterdir() if p.is_file()]+['scripts/acceptance/launch-recovery/'+n for n in ['acceptance-lifecycle.mjs','acceptance-startup.mjs','durable-acceptance-journal.mjs','fenced-docker-proxy.mjs']]+['scripts/acceptance/four-project-queue/sample-linux-owned.py','control-plane/internal/backend/dockerbe/resource_observer.go','control-plane/go.mod','control-plane/go.sum','sandboxd/go.mod','sandboxd/go.sum','go.work','go.work.sum'])
def snapshot():return [{'path':p,'sha256':sha((repo/p).read_bytes()),'bytes':len((repo/p).read_bytes())} for p in paths]
before=snapshot();head=subprocess.check_output(['git','rev-parse','HEAD'],cwd=repo,text=True).strip()
for p in paths:
 target=out/'tested-sources'/p;target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes((repo/p).read_bytes())
for p in private.iterdir():
 if p.is_file() and p.name!='capture-final.py':
  target=out/'retained-inputs'/p.name;target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(p.read_bytes())
pinned=[]; pin='81f2ba5b194a81a4f2ad167e466793f59162b387'
for n in ['fenced-docker-proxy.mjs','fenced-docker-proxy.test.mjs','acceptance-lifecycle.mjs','durable-acceptance-journal.mjs']:
 p='scripts/acceptance/launch-recovery/'+n;b=subprocess.check_output(['git','show',pin+':'+p],cwd=repo);target=out/'pinned-inputs'/n;target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(b);pinned.append({'commit':pin,'path':p,'bytes':len(b),'sha256':sha(b)})
for p in ['scripts/acceptance/launch-recovery/fenced-docker-proxy.mjs','scripts/acceptance/check-create-recovery/proxy-recovery.test.mjs']:
 b=subprocess.check_output(['git','show',head+':'+p],cwd=repo);target=out/'original-da8'/Path(p).name;target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(b)
commands=[]
def run(label,args,cwd=repo,expected=0,timeout=45):
 started=datetime.datetime.now(datetime.timezone.utc).isoformat();t=time.monotonic();r=subprocess.run(args,cwd=cwd,capture_output=True,timeout=timeout)
 (out/(label+'.stdout.log')).write_bytes(r.stdout);(out/(label+'.stderr.log')).write_bytes(r.stderr)
 commands.append({'label':label,'command':args,'cwd':'repository root' if cwd==repo else 'isolated frozen source tree; only proxy replaced with exact original da8 bytes','startedAtUTC':started,'durationSeconds':time.monotonic()-t,'exitCode':r.returncode,'expectedExitCode':expected,'stdoutSHA256':sha(r.stdout),'stderrSHA256':sha(r.stderr)})
 assert r.returncode==expected,(label,r.returncode,r.stdout[-2000:],r.stderr[-1000:]);return r
with tempfile.TemporaryDirectory(prefix='cloud040-before-') as temp:
 root=Path(temp)
 for p in paths:
  target=root/p;target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes((repo/p).read_bytes())
 (root/'scripts/acceptance/launch-recovery/fenced-docker-proxy.mjs').write_bytes((out/'original-da8/fenced-docker-proxy.mjs').read_bytes())
 run('original-proxy-bound-regression',['node','--test','--test-timeout=30000','--test-reporter=spec','--test-name-pattern=SDK generated one-shot stats request forwards one bounded read','scripts/acceptance/check-create-recovery/proxy-recovery.test.mjs'],root,1)
run('corrected-controlled',['node','--test','--test-timeout=30000','--test-reporter=spec','--test-concurrency=2']+[p.relative_to(repo).as_posix() for p in sorted((repo/'scripts/acceptance/check-create-recovery').glob('*.test.mjs'))])
for p in ['scripts/acceptance/launch-recovery/fenced-docker-proxy.mjs','scripts/acceptance/check-create-recovery/proxy-recovery.test.mjs']:
 run('syntax-'+Path(p).name,['node','--check',p])
# Actual pinned SDK generation is a fake RoundTripper, never a socket/client to Docker.
r=run('sdk-wire-repeat',['go','run','-p=1',str(private/'sdk-stats-wire.go')],timeout=120)
assert r.stdout==(repo/'scripts/acceptance/check-create-recovery/sdk-stats-wire.json').read_bytes()
module=json.loads(subprocess.check_output(['go','list','-m','-json','github.com/docker/docker'],cwd=repo))
external=[]
for name in ['client/container_stats.go','LICENSE']:
 p=Path(module['Dir'])/name;b=p.read_bytes();target=out/'sdk-source'/name;target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(b);external.append({'module':module['Path'],'version':module['Version'],'path':name,'bytes':len(b),'sha256':sha(b)})
modulePublic={k:module[k] for k in ['Path','Version','Sum','GoModSum'] if k in module}
after=snapshot();assert before==after
(out/'source-bindings.json').write_text(json.dumps({'baseCommit':head,'scope':'selected helper/support and product SDK-call/module inputs; not complete compiled Go inputs or installed-runtime provenance','before':before,'after':after,'unchangedDuringValidation':True,'pinnedProxyInputs':pinned,'sdkModule':modulePublic,'externalSDKSelectedFiles':external,'nodeVersion':subprocess.check_output(['node','--version'],text=True).strip(),'goVersion':subprocess.check_output(['go','version'],text=True).strip()},indent=2)+'\n')
(out/'commands.json').write_text(json.dumps(commands,indent=2)+'\n')
(out/'candidate.patch').write_bytes(subprocess.check_output(['git','diff','--','scripts/acceptance/launch-recovery/fenced-docker-proxy.mjs','scripts/acceptance/check-create-recovery/proxy-recovery.test.mjs'],cwd=repo))
shutil.copyfile(__file__,out/'capture-final.py')
print(json.dumps({'directory':str(out),'selectedSources':len(paths),'commands':[{k:r[k] for k in ['label','exitCode']} for r in commands]}))
