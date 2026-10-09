from pathlib import Path
import subprocess,sys,shutil,hashlib,json,datetime,os,signal,time
base=Path(__file__).resolve().parent.parent
out=base/'controlled'/'receipts'/sys.argv[1]
assert not out.exists();out.mkdir(parents=True,mode=0o700)
files=['observer/autonomous-readiness.mjs','observer/stage-gate.patch','controlled/browser.mjs','controlled/vite-fixture-plugin.mjs','controlled/fixture.tsx','controlled/run.py','controlled/source-binding.json','controlled/original/autonomous-readiness.mjs','controlled/original/interaction-metrics.mjs','controlled/original/latency-diagnostics.mjs']
binding={}
for name in files:
 source=base/name;target=out/'source'/name;target.parent.mkdir(parents=True,exist_ok=True);shutil.copyfile(source,target);binding[name]=hashlib.sha256(source.read_bytes()).hexdigest()
cmd=['node',str(base/'controlled/browser.mjs'),str(out/'browser')]
receipt={'command':cmd,'environmentOverrides':{'GOMAXPROCS':'2'},'sourceSHA256':binding,'startedAt':datetime.datetime.now(datetime.timezone.utc).isoformat(),'stdout':'stdout.txt','stderr':'stderr.txt','deadlineSeconds':90,'outerDeadlineSeconds':110}
with (out/'stdout.txt').open('w') as stdout,(out/'stderr.txt').open('w') as stderr:
 process=subprocess.Popen(cmd,stdout=stdout,stderr=stderr,env={**os.environ,'GOMAXPROCS':'2'},start_new_session=True)
 receipt['nodePid']=process.pid
 try: receipt['exitCode']=process.wait(timeout=110)
 except subprocess.TimeoutExpired:
  receipt['timedOut']=True;os.killpg(process.pid,signal.SIGTERM)
  try: receipt['exitCode']=process.wait(timeout=10)
  except subprocess.TimeoutExpired:os.killpg(process.pid,signal.SIGKILL);receipt['exitCode']=process.wait(timeout=5)
receipt['endedAt']=datetime.datetime.now(datetime.timezone.utc).isoformat()
receipt['unchanged']=all(hashlib.sha256((base/name).read_bytes()).hexdigest()==value for name,value in binding.items())
owned=json.loads((out/'browser/owned-resources.json').read_text()) if (out/'browser/owned-resources.json').exists() else None
survivors=[]
if owned:
 for item in Path('/proc').iterdir():
  if not item.name.isdigit():continue
  try:
   args=(item/'cmdline').read_bytes()
   if str(owned['profile']).encode() in args:survivors.append(int(item.name))
  except (FileNotFoundError,PermissionError,ProcessLookupError):pass
receipt['cleanup']={'nodeExited':process.poll() is not None,'ownedProfileCommandSurvivors':survivors,'profileAbsent':not Path(owned['profile']).exists() if owned else None}
(out/'command.json').write_text(json.dumps(receipt,indent=2)+'\n')
print(json.dumps({'exitCode':receipt.get('exitCode'),'unchanged':receipt['unchanged'],'cleanup':receipt['cleanup'],'receipt':str(out/'command.json')}));sys.exit(0 if receipt.get('exitCode')==0 and receipt['unchanged'] and not survivors else 1)
