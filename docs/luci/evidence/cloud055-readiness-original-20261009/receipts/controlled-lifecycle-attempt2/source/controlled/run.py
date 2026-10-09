from pathlib import Path
import subprocess,sys,shutil,hashlib,json,datetime
base=Path(__file__).resolve().parent.parent
out=base/'receipts'/sys.argv[1]
assert not out.exists();out.mkdir(mode=0o700)
script=sys.argv[2] if len(sys.argv)>2 else 'browser.mjs'
files=['observer/autonomous-readiness.mjs','observer/readiness-ledger.mjs','controlled/'+script,'controlled/browser.mjs','controlled/fixture.tsx','controlled/run.py']
binding={}
for name in files:
 source=base/name;target=out/'source'/name;target.parent.mkdir(parents=True,exist_ok=True);shutil.copyfile(source,target);binding[name]=hashlib.sha256(source.read_bytes()).hexdigest()
cmd=['node',str(base/'controlled'/script),str(out/'browser')]
receipt={'command':cmd,'sourceSHA256':binding,'startedAt':datetime.datetime.now(datetime.timezone.utc).isoformat()}
with (out/'stdout.txt').open('w') as stdout,(out/'stderr.txt').open('w') as stderr:
 try: result=subprocess.run(cmd,stdout=stdout,stderr=stderr,timeout=150);receipt['exitCode']=result.returncode
 except subprocess.TimeoutExpired: receipt['timedOut']=True
receipt['endedAt']=datetime.datetime.now(datetime.timezone.utc).isoformat()
receipt['unchanged']=all(hashlib.sha256((base/name).read_bytes()).hexdigest()==value for name,value in binding.items())
(out/'command.json').write_text(json.dumps(receipt,indent=2)+'\n')
print(json.dumps(receipt));sys.exit(0 if receipt.get('exitCode')==0 and receipt['unchanged'] else 1)
