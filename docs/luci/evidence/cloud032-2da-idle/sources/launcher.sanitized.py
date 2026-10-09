from pathlib import Path
import subprocess,os,json,datetime,time,hashlib
root=Path(__file__).resolve().parent;repo=Path('/workspace/luci');product='2da1a821b8456c20a858fb86ef87649ea917d9de'
assert subprocess.check_output(['git','rev-parse','HEAD'],cwd=repo,text=True).strip()==product
assert not subprocess.check_output(['git','status','--porcelain'],cwd=repo,text=True)
assert not subprocess.check_output(['docker','ps','-aq'],text=True).strip()
assert not subprocess.check_output(['docker','volume','ls','-q'],text=True).strip()
prior=Path('<PRIOR_DIAGNOSTIC_DIRECTORY>');old=json.loads((prior/'invocation.json').read_text());cmd=[x.replace(str(prior),str(root)).replace('d348f0127a920aab05b54a4192baa8ed3800fe54',product).replace('package-0.5.0-d348f012','package-0.5.0-2da1a821').replace('candidate-d348f012','candidate-2da1a821').replace('actual-idle-after-1','actual-idle-1') for x in old['command']]
provenance=json.loads(Path(cmd[cmd.index('--provenance')+1]).read_text());assert provenance['sourceCommit']==product and provenance['sourceDirty'] is False
sha=lambda p:hashlib.sha256(p.read_bytes()).hexdigest()
inv={**old,'preparedAt':datetime.datetime.now(datetime.timezone.utc).isoformat(),'status':'launching','command':cmd,'productSource':product,'helperSource':product,'driverSHA256':sha(root/'actual/sustained.mjs'),'bindingSHA256':sha(root/'actual/external-source-binding.json'),'launcherSHA256':sha(Path(__file__)),'comparisonBaseline':'da8 317.287164/66.501146 ms switch/input p95; unchanged targets300/100; source change does not isolate all environmental variation'}
(root/'invocation.json').write_text(json.dumps(inv,indent=2)+'\n')
env=dict(os.environ,LUCI_ACCEPTANCE_REPO=str(repo));t=time.monotonic();start=datetime.datetime.now(datetime.timezone.utc).isoformat()
with (root/'driver.stdout.log').open('w') as out,(root/'driver.stderr.log').open('w') as err:
 child=subprocess.Popen(cmd,cwd=repo,env=env,stdout=out,stderr=err)
 stat=Path('/proc',str(child.pid),'stat').read_text().rsplit(') ',1)[1].split();identity={'pid':child.pid,'startTicks':int(stat[19])}
 (root/'driver-identity.json').write_text(json.dumps(identity)+'\n')
 try:code=child.wait(timeout=450);timeout=False
 except subprocess.TimeoutExpired:code=None;timeout=True
r={'startedAt':start,'finishedAt':datetime.datetime.now(datetime.timezone.utc).isoformat(),'elapsedSeconds':time.monotonic()-t,'exitCode':code,'externalMonitorTimeout':timeout,'driverIdentity':identity,'ownedCleanupRequiresFinalReceipt':True}
(root/'wrapper-result.json').write_text(json.dumps(r,indent=2)+'\n');print(json.dumps(r),flush=True)
raise SystemExit(124 if timeout else code)
