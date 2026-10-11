import os,sys,json,hashlib,subprocess,time,signal
from pathlib import Path
root=Path('/workspace/luci');base=Path('/workspace/.luci-tools/cloud080081-integrated');out=base/sys.argv[1];out.mkdir(parents=True,exist_ok=False)
paths=subprocess.check_output(['git','ls-files','--cached','--others','--exclude-standard','control-plane','sandboxd','go.work'],cwd=root,text=True).splitlines()
paths=sorted(set(p for p in paths if '/testdata/' not in p and '/docs/' not in p and (p.endswith(('.go','.mod','.sum','.html','.json','.work')))))
def hashes():
 rows=[]
 for p in paths:
  f=root/p
  if f.is_file():b=f.read_bytes();rows.append({'path':p,'sha256':hashlib.sha256(b).hexdigest(),'bytes':len(b)})
 for p in ['/workspace/.luci-tools/go.work','/workspace/.luci-tools/go.work.sum']:
  f=Path(p)
  if f.is_file():b=f.read_bytes();rows.append({'externalPath':p,'sha256':hashlib.sha256(b).hexdigest(),'bytes':len(b)})
 return rows
before=hashes();(out/'inputs-before.json').write_text(json.dumps(before,indent=2))
cmd=['go','test','-p','1','-count=1','-timeout=4m','-json']+sys.argv[2:];env=os.environ.copy();env.update(GOMAXPROCS='2',GOPROXY='off',GOSUMDB='off');start=time.time()
(out/'command.json').write_text(json.dumps({'argv':cmd,'cwd':str(root),'baseHead':subprocess.check_output(['git','rev-parse','HEAD'],cwd=root,text=True).strip(),'workingTree':'integrated candidate, not yet committed','start':start,'environment':{k:env.get(k) for k in ['GOMAXPROCS','GOCACHE','GOMODCACHE','GOTMPDIR','GOWORK','GOFLAGS']}},indent=2))
with open(out/'stdout.jsonl','w') as stdout,open(out/'stderr.log','w') as stderr:
 p=subprocess.Popen(cmd,cwd=root,env=env,stdout=stdout,stderr=stderr,start_new_session=True)
 try:code=p.wait(timeout=300)
 except subprocess.TimeoutExpired:
  os.killpg(p.pid,signal.SIGTERM)
  try:p.wait(timeout=5)
  except subprocess.TimeoutExpired:os.killpg(p.pid,signal.SIGKILL);p.wait()
  code=124
rows=[]
for line in (out/'stdout.jsonl').read_text().splitlines():
 try:rows.append(json.loads(line))
 except json.JSONDecodeError:pass
counts={x:sum(r.get('Action')==x and bool(r.get('Test')) for r in rows) for x in ['pass','fail','skip']}
counts['topPass']=sum(r.get('Action')=='pass' and bool(r.get('Test')) and '/' not in r['Test'] for r in rows)
after=hashes();(out/'inputs-after.json').write_text(json.dumps(after,indent=2));result={'exit':code,'elapsedSeconds':time.time()-start,'inputCount':len(before),'sourceStable':before==after,**counts};(out/'result.json').write_text(json.dumps(result,indent=2));print(json.dumps(result));print((out/'stderr.log').read_text()[-3000:]);print('\n'.join(r.get('Output','').strip() for r in rows if r.get('Action')=='build-output'));sys.exit(code)
