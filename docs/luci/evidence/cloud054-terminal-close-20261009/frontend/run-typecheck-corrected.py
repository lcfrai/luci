import datetime,hashlib,json,pathlib,subprocess,time
b=pathlib.Path(__file__).parent;tree=b/'tree/desktop';out=b/'evidence/typecheck-corrected';out.mkdir(parents=True,exist_ok=False)
def bind(p):
 raw=p.read_bytes();return {'bytes':len(raw),'sha256':hashlib.sha256(raw).hexdigest()}
def snapshot():return {str(p.relative_to(tree)):bind(p) for p in sorted(tree.rglob('*')) if p.is_file() and 'node_modules' not in p.parts and '.cloud054-cache' not in p.parts}
a=snapshot();(out/'inputs-before.json').write_text(json.dumps(a,indent=2)+'\n')
cmd=['node','node_modules/typescript/bin/tsc','--noEmit'];t=time.monotonic();started=datetime.datetime.now(datetime.timezone.utc).isoformat()
with (out/'stdout.log').open('wb') as so,(out/'stderr.log').open('wb') as se:r=subprocess.run(cmd,cwd=tree,stdout=so,stderr=se,timeout=120)
z=snapshot();(out/'inputs-after.json').write_text(json.dumps(z,indent=2)+'\n');result={'command':cmd,'startedAt':started,'elapsedSeconds':time.monotonic()-t,'exitCode':r.returncode,'inputCount':len(a),'inputsStable':a==z,'inputsBefore':bind(out/'inputs-before.json'),'inputsAfter':bind(out/'inputs-after.json'),'logs':{n:bind(out/n) for n in ['stdout.log','stderr.log']}}
(out/'result.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result))
