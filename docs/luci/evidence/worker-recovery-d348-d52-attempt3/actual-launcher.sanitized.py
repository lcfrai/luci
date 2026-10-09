from pathlib import Path
import subprocess,json,datetime,os
repo=Path('/workspace/luci');head=subprocess.check_output(['git','rev-parse','HEAD'],cwd=repo,text=True).strip()
assert not subprocess.check_output(['git','status','--porcelain'],cwd=repo,text=True)
assert not subprocess.check_output(['docker','ps','-aq'],text=True).strip()
out=repo/'artifacts/quality-swarm/worker-recovery-d348f012-attempt3'
assert not out.exists()
cmd=['python3','-B','/workspace/.luci-tools/local016-worker-recovery-launch/run-installed.py','--window-granted','--repo',str(repo),'--expected-source','d348f0127a920aab05b54a4192baa8ed3800fe54','--expected-helper-source',head,'--expected-image-id','sha256:333effcf40151e2411fa036b4581113be2af7f8d441bf9566f1cce6e7f208e98','--binary','[installed-package]/luci','--web','[installed-package]/web','--provenance',str(repo/'artifacts/quality-swarm/candidate-d348f012/package/package-provenance.json'),'--image-provenance','[image-provenance-directory]/receipt.json','--fixture-base','/workspace','--output',str(out)]
start=datetime.datetime.now(datetime.timezone.utc).isoformat();receipt={'command':cmd,'cwd':str(repo),'startedAt':start,'helper':head,'product':'d348f0127a920aab05b54a4192baa8ed3800fe54'}
record=Path('/workspace/.luci-tools/local016-attempt3-invocation.json');assert not record.exists();record.write_text(json.dumps(receipt,indent=2)+'\n')
r=subprocess.run(cmd,cwd=repo)
receipt.update(exitCode=r.returncode,finishedAt=datetime.datetime.now(datetime.timezone.utc).isoformat());Path('/workspace/.luci-tools/local016-attempt3-exit.json').write_text(json.dumps(receipt,indent=2)+'\n');print(json.dumps({'output':str(out),'exitCode':r.returncode}));raise SystemExit(r.returncode)
