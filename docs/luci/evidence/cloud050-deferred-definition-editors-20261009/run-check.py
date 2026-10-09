import sys,subprocess,json,hashlib,time
from pathlib import Path
r=Path('/workspace/luci');out=Path('/workspace/.luci-tools/cloud050-context-scale');name=sys.argv[1];args=sys.argv[2:]
files=subprocess.check_output(['git','ls-files','desktop/src','desktop/package.json','desktop/package-lock.json','desktop/vitest.config.ts','desktop/vite.config.ts','desktop/tsconfig.json'],cwd=r,text=True).splitlines()
def snap():return {p:hashlib.sha256((r/p).read_bytes()).hexdigest() for p in files if (r/p).is_file()}
before=snap();(out/(name+'-inputs-before.json')).write_text(json.dumps(before,indent=2)+'\n')
start=time.time();(out/(name+'-command.json')).write_text(json.dumps({'command':args,'cwd':str(r/'desktop'),'startedAt':start,'sourceBindingScope':'All Git-tracked desktop/src inputs plus manifest/lock and test/build/TypeScript configs. Other concurrent source changes are reported explicitly; dependency installation bytes not independently frozen.'},indent=2)+'\n')
with (out/(name+'.stdout.log')).open('wb') as stdout,(out/(name+'.stderr.log')).open('wb') as stderr:run=subprocess.run(args,cwd=r/'desktop',stdout=stdout,stderr=stderr)
after=snap();(out/(name+'-inputs-after.json')).write_text(json.dumps(after,indent=2)+'\n');(out/(name+'-exit.json')).write_text(json.dumps({'exitCode':run.returncode,'wallSeconds':time.time()-start,'changedInputs':{p:{'before':before.get(p),'after':after.get(p)} for p in set(before)|set(after) if before.get(p)!=after.get(p)}},indent=2)+'\n');print(name,run.returncode);sys.exit(run.returncode)
