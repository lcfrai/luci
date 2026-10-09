import datetime,hashlib,json,os,subprocess,sys,time
from pathlib import Path
root=Path(__file__).resolve().parents[1];name=sys.argv[1];command=sys.argv[2:]
out=root/'validation'/name;out.mkdir()
paths=['header-delivery.mjs','header-delivery.test.mjs','prepare-v2.py','launch.py','actual/sustained.mjs','actual/header-delivery.mjs','actual/external-source-binding.json','actual/schedule.json','validation/verify-preflight.py']
def hashes():return {p:hashlib.sha256((root/p).read_bytes()).hexdigest() for p in paths}
record={'command':command,'cwd':str(root),'environmentOverrides':{'GOMAXPROCS':'2','PYTHONDONTWRITEBYTECODE':'1'},'timeoutSeconds':30,'startedAt':datetime.datetime.now(datetime.timezone.utc).isoformat(),'inputsBefore':hashes()}
(out/'command.json').write_text(json.dumps(record,indent=2)+'\n')
begin=time.monotonic()
with (out/'stdout.log').open('wb') as stdout,(out/'stderr.log').open('wb') as stderr:
 try:result=subprocess.run(command,cwd=root,env={**os.environ,'GOMAXPROCS':'2','PYTHONDONTWRITEBYTECODE':'1'},stdout=stdout,stderr=stderr,timeout=30);code=result.returncode
 except subprocess.TimeoutExpired:code=124
record.update(exitCode=code,elapsedSeconds=time.monotonic()-begin,inputsAfter=hashes());(out/'result.json').write_text(json.dumps(record,indent=2)+'\n');print(json.dumps({'name':name,'exitCode':code,'unchangedInputs':record['inputsBefore']==record['inputsAfter']}));sys.exit(code)
