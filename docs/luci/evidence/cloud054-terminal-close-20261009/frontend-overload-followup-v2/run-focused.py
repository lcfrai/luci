import datetime,hashlib,json,os,pathlib,subprocess,sys,time
base=pathlib.Path(__file__).parent;tree=base/'tree/desktop';name=sys.argv[1];args=sys.argv[2:]
out=base/'evidence'/name;out.mkdir(parents=True,exist_ok=False)
def bind(p):
 b=p.read_bytes();return {'bytes':len(b),'sha256':hashlib.sha256(b).hexdigest()}
def sources():
 return {str(p.relative_to(tree)):bind(p) for p in sorted(tree.rglob('*')) if p.is_file() and 'node_modules' not in p.parts and '.cloud054-cache' not in p.parts}
pre=sources();(out/'inputs-before.json').write_text(json.dumps(pre,indent=2)+'\n')
command=['node','node_modules/vitest/vitest.mjs','run','--maxWorkers=1',*args,'--reporter=json','--outputFile='+str(out/'vitest.json')]
t=time.monotonic();started=datetime.datetime.now(datetime.timezone.utc).isoformat()
with (out/'stdout.log').open('wb') as stdout,(out/'stderr.log').open('wb') as stderr:
 p=subprocess.run(command,cwd=tree,stdout=stdout,stderr=stderr,timeout=180)
post=sources();(out/'inputs-after.json').write_text(json.dumps(post,indent=2)+'\n')
result={'command':command,'cwd':str(tree),'startedAt':started,'elapsedSeconds':time.monotonic()-t,'exitCode':p.returncode,'sourceStable':pre==post,'sourceCount':len(pre),'scope':'Controlled Vitest fake socket/jsdom tests; no product app/Docker/provider runtime','inputs':{'before':bind(out/'inputs-before.json'),'after':bind(out/'inputs-after.json')},'logs':{'stdout':bind(out/'stdout.log'),'stderr':bind(out/'stderr.log')}}
(out/'result.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result))
