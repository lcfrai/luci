from pathlib import Path
import datetime, hashlib, json, os, subprocess, sys, time

root=Path('/workspace/luci')
adjunct=Path('/workspace/.luci-tools/cloud032-cgroup-counter-adjunct')
private=Path('/workspace/.luci-tools/idle-d20ebd29-counter-attempt1')
product='d20ebd292f3f1de04d6726bab0998c210987bcb5'
helper='c625ef0b7842748f4ce74cd3ffc08979d571bddf'
sha=lambda p:hashlib.sha256(p.read_bytes()).hexdigest()
now=lambda:datetime.datetime.now(datetime.timezone.utc).isoformat()
assert subprocess.check_output(['git','rev-parse','HEAD'],cwd=root,text=True).strip()==helper
assert not subprocess.check_output(['git','status','--porcelain'],cwd=root,text=True).strip()
for line in (adjunct/'SHA256SUMS').read_text().splitlines():
    digest,name=line.split(None,1);assert sha(adjunct/name.lstrip('* '))==digest,name
assert sha(adjunct/'observer.py')=='fc509776db0d1cbe8d474dfcd58e311d8588746ad9632fecab4d0ca7a2f0fea7'
assert sha(adjunct/'supervisor.py')=='4b555802ca17e558bb65e64477af8d28642a55d047b4a310b0b412a08ab25beb'
private.mkdir(mode=0o700,exist_ok=False)
sys.path.insert(0,str(adjunct))
from supervisor import ManagedObserver

command=[sys.executable,'/workspace/.luci-tools/cloud032-idle-67a5b7c7/launch.py',
 '--source',product,'--helper-source',helper,
 '--package','/tmp/luci-retained-tools/package-0.5.0-d20ebd29',
 '--provenance',str(root/'artifacts/quality-swarm/candidate-d20ebd29/package/package-provenance.json'),
 '--image','luci/sandbox:verified-candidate-20261008t140025z-a4a2e82b',
 '--expected-image-id','sha256:333effcf40151e2411fa036b4581113be2af7f8d441bf9566f1cce6e7f208e98',
 '--image-provenance','/workspace/.luci-tools/sandbox-candidate-20261008T140025Z-a4a2e82b/receipt.json',
 '--output',str(root/'artifacts/quality-swarm/idle-d20ebd29-counter-attempt1'),
 '--fixture-root','/workspace/.c32','--trace-after','yes']
record={'productSource':product,'helperSource':helper,'startedAt':now(),'command':command,
 'scope':'Fresh unchanged installed idle protocol plus separately bounded shared-cgroup counter observation. No per-action/process cause, threshold adjustment, Windows or full sustained acceptance claim.',
 'sources':{str(p):sha(p) for p in [Path(__file__),adjunct/'observer.py',adjunct/'supervisor.py',adjunct/'manifest.json',Path(command[1]),Path('/workspace/.luci-tools/cloud032-idle-67a5b7c7/actual/sustained.mjs')]}}
(private/'invocation.json').write_text(json.dumps(record,indent=2)+'\n')
managed=ManagedObserver(['--output',str(private/'cpu.jsonl'),'--cgroup-dir','/sys/fs/cgroup','--duration-seconds','399'],private/'supervision',wall_seconds=400,term_grace=1,kill_grace=1)
begin=time.monotonic();code=None;error=None
try:
    with managed:
        code=subprocess.run(command,cwd=root,check=False).returncode
except BaseException as exc:
    error={'type':type(exc).__name__,'message':str(exc)[:400]}
finally:
    result={**record,'finishedAt':now(),'elapsedSeconds':time.monotonic()-begin,'launcherExitCode':code,'error':error,'observer':managed.snapshot(),
            'cleanAfter':not subprocess.check_output(['git','status','--porcelain'],cwd=root,text=True).strip(),
            'sameHead':subprocess.check_output(['git','rev-parse','HEAD'],cwd=root,text=True).strip()==helper}
    (private/'result.json').write_text(json.dumps(result,indent=2)+'\n')
    print(json.dumps({'counterWrapperResult':str(private/'result.json'),'launcherExitCode':code,'observerProcessOutcome':result['observer'].get('processOutcome'),'observerSerializationComplete':result['observer'].get('observationComplete'),'error':error}),flush=True)
raise SystemExit(code if code is not None else 1)
