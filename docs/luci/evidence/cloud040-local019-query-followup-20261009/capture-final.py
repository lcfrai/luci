from pathlib import Path
import hashlib,json,subprocess,datetime,time,shutil,tempfile
repo=Path('/workspace/luci');prior=repo/'docs/luci/evidence/cloud040-sdk-check-stats-20261009';private=Path('/workspace/.luci-tools/cloud040-stats-followup');out=repo/'docs/luci/evidence/cloud040-local019-query-followup-20261009';out.mkdir(exist_ok=False)
sha=lambda b:hashlib.sha256(b).hexdigest()
paths=sorted([p.relative_to(repo).as_posix() for p in (repo/'scripts/acceptance/check-create-recovery').iterdir() if p.is_file()]+['scripts/acceptance/launch-recovery/'+n for n in ['acceptance-lifecycle.mjs','acceptance-startup.mjs','durable-acceptance-journal.mjs','fenced-docker-proxy.mjs']]+['scripts/acceptance/four-project-queue/sample-linux-owned.py'])
snapshot=lambda:[{'path':p,'bytes':len((repo/p).read_bytes()),'sha256':sha((repo/p).read_bytes())} for p in paths]
before=snapshot()
for p in paths:
 target=out/'tested-sources'/p;target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes((repo/p).read_bytes())
for p in private.iterdir():
 if p.is_file() and p.name!='capture-final.py':
  target=out/'retained-before'/p.name;target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(p.read_bytes())
commands=[]
def run(label,args,cwd=repo,expected=0):
 start=datetime.datetime.now(datetime.timezone.utc).isoformat();t=time.monotonic();r=subprocess.run(args,cwd=cwd,capture_output=True,timeout=45)
 (out/(label+'.stdout.log')).write_bytes(r.stdout);(out/(label+'.stderr.log')).write_bytes(r.stderr)
 commands.append({'label':label,'command':args,'cwd':'repository root' if cwd==repo else 'isolated final sources with only earlier C040 proxy restored','startedAtUTC':start,'durationSeconds':time.monotonic()-t,'exitCode':r.returncode,'expectedExitCode':expected,'stdoutSHA256':sha(r.stdout),'stderrSHA256':sha(r.stderr)})
 assert r.returncode==expected,(label,r.returncode,r.stdout[-1500:])
with tempfile.TemporaryDirectory(prefix='cloud040-query-before-') as temp:
 root=Path(temp)
 for p in paths:
  target=root/p;target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes((repo/p).read_bytes())
 (root/'scripts/acceptance/launch-recovery/fenced-docker-proxy.mjs').write_bytes((private/'before-fenced-docker-proxy.mjs').read_bytes())
 run('before-bound-followup',['node','--test','--test-timeout=30000','--test-reporter=spec','--test-name-pattern=versioned and unversioned|both owned stats routes|stats followup leaves','scripts/acceptance/check-create-recovery/proxy-recovery.test.mjs'],root,1)
run('final-controlled',['node','--test','--test-timeout=30000','--test-reporter=spec','--test-concurrency=2']+[p.relative_to(repo).as_posix() for p in sorted((repo/'scripts/acceptance/check-create-recovery').glob('*.test.mjs'))])
for p in ['scripts/acceptance/launch-recovery/fenced-docker-proxy.mjs','scripts/acceptance/check-create-recovery/proxy-recovery.test.mjs']:run('syntax-'+Path(p).name,['node','--check',p])
after=snapshot();assert before==after
for line in (prior/'SHA256SUMS').read_text().splitlines():
 digest,path=line.split('  ',1);assert sha((prior/path).read_bytes())==digest
(out/'source-bindings.json').write_text(json.dumps({'baseCommit':subprocess.check_output(['git','rev-parse','HEAD'],cwd=repo,text=True).strip(),'scope':'selected helper sources/configuration, not product/runtime validation','before':before,'after':after,'selectedInputsUnchanged':True,'previousCapsuleManifestSHA256':sha((prior/'publication-manifest.json').read_bytes()),'previousCapsuleEveryChecksumStillMatches':True,'previousProxySHA256':sha((private/'before-fenced-docker-proxy.mjs').read_bytes()),'nodeVersion':subprocess.check_output(['node','--version'],text=True).strip()},indent=2)+'\n')
(out/'commands.json').write_text(json.dumps(commands,indent=2)+'\n')
# Patch relative to the earlier controlled C040 candidate, not its base Git commit.
for n,p in [('proxy','scripts/acceptance/launch-recovery/fenced-docker-proxy.mjs'),('tests','scripts/acceptance/check-create-recovery/proxy-recovery.test.mjs')]:
 r=subprocess.run(['diff','-u',str(private/('before-'+Path(p).name)),str(repo/p)],capture_output=True);assert r.returncode==1;(out/(n+'-followup.patch')).write_bytes(r.stdout)
shutil.copyfile(__file__,out/'capture-final.py')
print(json.dumps({'directory':str(out),'sourceCount':len(paths),'commands':[{k:r[k] for k in ['label','exitCode']} for r in commands]}))
