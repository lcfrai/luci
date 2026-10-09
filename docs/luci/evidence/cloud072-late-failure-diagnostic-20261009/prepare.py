from pathlib import Path
import hashlib,json,difflib
B=Path('/workspace/.luci-tools/cloud072-late-failure-diagnostic');S=Path('/workspace/.luci-tools/cloud071-browser-restart-diagnostic')
module=r'''// CLOUD-072: first failure only; raw message/stack stay in one exclusive private file.
export function createFinalFailureObservation({retain}){
 let first=null;
 const stageNames=new Set(['preflight','controller-start','shell-create','browser-start','terminal-scenario','product-settlement']);
 const phaseNames=new Set(['workflow','main-catch','response-drain','cleanup']);
 const errorNames=new Set(['Error','AssertionError','TypeError','RangeError','AbortError','TimeoutError']);
 const labels=new Map([['Workflow closing','workflow-closing'],['Owned controller exited','owned-controller-exited'],['Browser page error','browser-page-error']]);
 function observe(error,{origin,stage,phase}){
  if(first)return;
  first={origin:origin==='main-catch'?'main-catch':'callback',stage:stageNames.has(stage)?stage:'unknown',phase:phaseNames.has(phase)?phase:'unknown',observedAt:new Date().toISOString(),monotonicMs:performance.now(),reportingFailed:false,privateRecord:{file:'private-first-failure.json',retention:'not-attempted'}};
  try{
   const read=key=>{try{return error?.[key];}catch{return undefined;}};
   const messageValue=read('message'),stackValue=read('stack'),nameValue=read('name');
   const message=typeof messageValue==='string'?messageValue:typeof error==='string'?error:'';
   const stack=typeof stackValue==='string'?stackValue:'';
   const record={message:message.slice(0,2048),stack:stack.slice(0,8192)};
   const bytes=Buffer.from(JSON.stringify(record,null,2)+'\n');
   if(bytes.length>64*1024)throw Error('Private failure record bound');
   first.errorName=errorNames.has(nameValue)?nameValue:'unknown';
   first.sourceLabel=labels.get(message)??null;
   first.messageCodeUnits=message.length;first.stackCodeUnits=stack.length;
   first.privateRecord={file:'private-first-failure.json',bytes:bytes.length,sha256:createHash('sha256').update(bytes).digest('hex'),maxBytes:64*1024,truncated:message.length>record.message.length||stack.length>record.stack.length,retention:'unconfirmed'};
   retain(record);
   first.privateRecord.retention='durable';
  }catch{first.reportingFailed=true;}
 }
 return {observe,snapshot(failurePresent){return {version:1,afterCleanup:true,snapshotAt:new Date().toISOString(),failurePresent:!!failurePresent,first:first?structuredClone(first):null};}};
}

'''
bindings=[]
for v in ['warm','cold']:
 for n in ['launch.mjs','scenario.mjs','outer-run.py']:
  raw=(S/v/n).read_bytes();before=B/'before'/v/n;before.parent.mkdir(parents=True,exist_ok=True);before.write_bytes(raw)
  target=B/v/n;target.parent.mkdir(parents=True,exist_ok=True)
  if n!='launch.mjs':target.write_bytes(raw);continue
  s=raw.decode();assert s.count('export async function main(')==1
  s=s.replace('export async function main(',module+'export async function main(')
  old=' const fail=e=>{failure??=e;workflow?.fail(e);};'
  new=" let failurePhase='workflow';\n const finalFailure=createFinalFailureObservation({retain:value=>durable(join(evidence,'private-first-failure.json'),value)});\n const rememberFailure=(e,origin)=>{const first=failure==null;failure??=e;if(first)try{finalFailure.observe(e,{origin,stage,phase:failurePhase});}catch{}};\n const fail=e=>{rememberFailure(e,'callback');workflow?.fail(e);};"
  assert s.count(old)==1;s=s.replace(old,new)
  old=' }catch(e){failure??=e;receipt.failedStage=stage;'
  new=" }catch(e){failurePhase='main-catch';rememberFailure(e,'main-catch');receipt.failedStage=stage;"
  assert s.count(old)==1;s=s.replace(old,new)
  old=' finally{\n  acceptRestartResponses=false;'
  new=" finally{\n  failurePhase='response-drain';\n  acceptRestartResponses=false;"
  assert s.count(old)==1;s=s.replace(old,new)
  old='  closing=true;workflow?.close();';new="  failurePhase='cleanup';\n  closing=true;workflow?.close();"
  assert s.count(old)==1;s=s.replace(old,new)
  old='  ]});receipt.endedAt=new Date().toISOString();'
  new='  ]});receipt.failureObservation=finalFailure.snapshot(!!failure);receipt.endedAt=new Date().toISOString();'
  assert s.count(old)==1;s=s.replace(old,new)
  target.write_text(s)
  diff=''.join(difflib.unified_diff(raw.decode().splitlines(True),s.splitlines(True),fromfile='before/'+v+'/launch.mjs',tofile=v+'/launch.mjs'))
  (B/(v+'-launch.mjs.diff')).write_text(diff)
  bindings.append({'variant':v,'originalSource':str(S/v/n),'original':{'sha256':hashlib.sha256(raw).hexdigest(),'bytes':len(raw)},'fixed':{'sha256':hashlib.sha256(s.encode()).hexdigest(),'bytes':len(s.encode())}})
(B/'source-binding.json').write_text(json.dumps(bindings,indent=2)+'\n')
(B/'CLOUD-072.patch').write_text(''.join((B/(v+'-launch.mjs.diff')).read_text() for v in ['warm','cold']))
print(json.dumps(bindings,indent=2))
