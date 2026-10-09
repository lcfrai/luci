from pathlib import Path
import hashlib,json,difflib
B=Path(__file__).parent;S=Path('/workspace/.luci-tools/cloud073-teardown-route');C=Path('/workspace/.luci-tools/cloud073-installed-78665270/evidence/cold-attempt1/driver/result.json');r=json.loads(C.read_text())
assert r['sourceCommit']=='786652704fa8bebd1ba1af41cfa1d414d7def100'
assert hashlib.sha256((S/'cold/launch.mjs').read_bytes()).hexdigest()==r['launcherSHA256'];assert hashlib.sha256((S/'cold/scenario.mjs').read_bytes()).hexdigest()==r['scenarioSHA256']
old="""// CLOUD-073: only these exact local read polls may be aborted during declared teardown.
export function closingReadRoute(method,url,base,sessionId){
 if(method!=='GET'||url.origin!==base||url.username||url.password||url.search||url.hash)return null;
 if(url.pathname==='/v1/sessions')return 'session-list';
 if(typeof sessionId==='string'&&url.pathname==='/v1/sessions/'+encodeURIComponent(sessionId))return 'owned-session';
 return null;
}"""
new="""// CLOUD-073: cancel the reads already allowed by the normal same-origin route guard.
// Teardown never forwards them; path/query/credentials are not retained in public rows.
export function closingReadRoute(method,url,base,sessionId){
 if(!['GET','HEAD','OPTIONS'].includes(method)||url.origin!==base)return null;
 if(url.pathname==='/v1/sessions')return 'session-list';
 if(typeof sessionId==='string'&&url.pathname==='/v1/sessions/'+encodeURIComponent(sessionId))return 'owned-session';
 return 'other-local-read';
}"""
bindings=[]
for v in ['warm','cold']:
 for n in ['launch.mjs','scenario.mjs','outer-run.py']:
  raw=(S/v/n).read_bytes();p=B/'before'/v/n;p.parent.mkdir(parents=True,exist_ok=True);p.write_bytes(raw);p=B/v/n;p.parent.mkdir(parents=True,exist_ok=True)
  if n!='launch.mjs':p.write_bytes(raw);continue
  s=raw.decode();assert s.count(old)==1;s=s.replace(old,new)
  a="const row={method:'GET',routeClass,startedAt:new Date().toISOString(),abort:'pending'};";b="const row={method:req.method(),routeClass,pathnameSHA256:sha(url.pathname),startedAt:new Date().toISOString(),abort:'pending'};";assert s.count(a)==1;s=s.replace(a,b);p.write_text(s)
  (B/(v+'-launch.mjs.diff')).write_text(''.join(difflib.unified_diff(raw.decode().splitlines(True),s.splitlines(True),fromfile='before/'+v+'/launch.mjs',tofile=v+'/launch.mjs')))
  bindings.append({'variant':v,'originalSource':str(S/v/n),'original':{'bytes':len(raw),'sha256':hashlib.sha256(raw).hexdigest()},'fixed':{'bytes':len(s.encode()),'sha256':hashlib.sha256(s.encode()).hexdigest()}})
(B/'CLOUD-073-v2.patch').write_text(''.join((B/(v+'-launch.mjs.diff')).read_text() for v in ['warm','cold']));(B/'source-binding.json').write_text(json.dumps({'originalActualReceipt':{'path':str(C),'sha256':hashlib.sha256(C.read_bytes()).hexdigest()},'helpers':bindings},indent=2)+'\n')
print(json.dumps(bindings,indent=2))
