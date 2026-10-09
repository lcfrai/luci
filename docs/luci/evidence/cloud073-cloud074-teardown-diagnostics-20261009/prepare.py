from pathlib import Path
import json,hashlib,difflib
B=Path(__file__).parent;S=Path('/workspace/.luci-tools/cloud072-late-failure-diagnostic')
module='''// CLOUD-073: only these exact local read polls may be aborted during declared teardown.
export function closingReadRoute(method,url,base,sessionId){
 if(method!=='GET'||url.origin!==base||url.username||url.password||url.search||url.hash)return null;
 if(url.pathname==='/v1/sessions')return 'session-list';
 if(typeof sessionId==='string'&&url.pathname==='/v1/sessions/'+encodeURIComponent(sessionId))return 'owned-session';
 return null;
}

'''
local=''' const closingReadObservations=[],closingReadSettlements=new Set();let acceptClosingReads=true;receipt.closingReadObservations=closingReadObservations;
 const abortClosingRead=(route,req,url)=>{
  const routeClass=closingReadRoute(req.method(),url,base,session?.id);if(!routeClass)return false;
  assert(acceptClosingReads,'Closing read observation sealed');assert(closingReadObservations.length<16,'Closing read observation limit');
  const row={method:'GET',routeClass,startedAt:new Date().toISOString(),abort:'pending'};closingReadObservations.push(row);
  const observation=(async()=>{
   try{await boundedAttempt(()=>route.abort('blockedbyclient'),2000,'closing-read-abort-timeout');row.abort='acknowledged';}
   catch(error){row.abort='unconfirmed';fail(error);throw error;}
   finally{row.settledAt=new Date().toISOString();}
  })();closingReadSettlements.add(observation);observation.then(()=>closingReadSettlements.delete(observation),()=>closingReadSettlements.delete(observation));return observation.then(()=>true);
 };
'''
bindings=[]
for v in ['warm','cold']:
 for n in ['launch.mjs','scenario.mjs','outer-run.py']:
  p=S/v/n;raw=p.read_bytes();old=B/'before'/v/n;old.parent.mkdir(parents=True,exist_ok=True);old.write_bytes(raw);new=B/v/n;new.parent.mkdir(parents=True,exist_ok=True)
  if n!='launch.mjs':
   if not new.exists():new.write_bytes(raw)
   continue
  s=raw.decode();assert s.count('export async function main(')==1;s=s.replace('export async function main(',module+'export async function main(')
  anchor=' const health=()=>';assert s.count(anchor)==1;s=s.replace(anchor,local+anchor)
  anchor="const req=route.request(),url=new URL(req.url());try{health();assert.equal(url.origin,base,'External browser request refused');"
  replacement="const req=route.request(),url=new URL(req.url());try{if(closing&&await abortClosingRead(route,req,url))return;health();assert.equal(url.origin,base,'External browser request refused');"
  assert s.count(anchor)==1;s=s.replace(anchor,replacement)
  anchor='  ]});receipt.failureObservation=finalFailure.snapshot(!!failure);'
  replacement="  ]});acceptClosingReads=false;try{await boundedAttempt(()=>Promise.allSettled([...closingReadSettlements]),2000,'closing-read-drain-timeout');}catch(error){fail(error);receipt.cleanup.push(safeError(error));}if(closingReadObservations.some(row=>row.abort!=='acknowledged'))receipt.cleanup.push({name:'Error',code:'closing-read-abort-unsettled',message:'closing-read-abort-unsettled'});receipt.closingReadObservations=structuredClone(closingReadObservations);receipt.failureObservation=finalFailure.snapshot(!!failure);"
  assert s.count(anchor)==1;s=s.replace(anchor,replacement);new.write_text(s)
  (B/(v+'-launch.mjs.diff')).write_text(''.join(difflib.unified_diff(raw.decode().splitlines(True),s.splitlines(True),fromfile='before/'+v+'/launch.mjs',tofile=v+'/launch.mjs')))
  bindings.append({'variant':v,'originalSource':str(p),'original':{'sha256':hashlib.sha256(raw).hexdigest(),'bytes':len(raw)},'fixed':{'sha256':hashlib.sha256(s.encode()).hexdigest(),'bytes':len(s.encode())}})
(B/'source-binding.json').write_text(json.dumps(bindings,indent=2)+'\n');(B/'CLOUD-073.patch').write_text(''.join((B/(v+'-launch.mjs.diff')).read_text() for v in ['warm','cold']))
print(json.dumps(bindings,indent=2))
