import argparse,subprocess,json,hashlib,os
from pathlib import Path
p=argparse.ArgumentParser();p.add_argument('--node',required=True);p.add_argument('--output',required=True);a=p.parse_args();r=Path(__file__).resolve().parent;o=Path(a.output).resolve();o.mkdir(mode=0o700,exist_ok=False)
commands=[('route-original','installed-v5/route-settlement.test.mjs',{'C059_ORIGINAL':'1'},1),('route-fixed','installed-v5/route-settlement.test.mjs',{},0),('budget','installed-v5/gate-budget.test.mjs',{},0),('payload-original','installed-v6/scenario.test.mjs',{'C059_PAYLOAD_ORIGINAL':'1'},1),('payload-fixed','installed-v6/scenario.test.mjs',{},0)]
rows=[]
for label,file,extra,want in commands:
 env={k:v for k,v in os.environ.items() if k in ['PATH','LANG','LC_ALL','TZ']};env.update(extra)
 command=[a.node,'--test','--test-concurrency=1',str(r/file)];v=subprocess.run(command,env=env,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=30)
 for stream in ['stdout','stderr']:(o/(label+'.'+stream)).write_bytes(getattr(v,stream))
 row={'label':label,'returncode':v.returncode,'expectedReturncode':want,'command':command};rows.append(row);(o/'result.json').write_text(json.dumps(rows,indent=2)+'\n');assert v.returncode==want,row
print(json.dumps({'inertInvocations':len(rows),'appStarted':False}))
