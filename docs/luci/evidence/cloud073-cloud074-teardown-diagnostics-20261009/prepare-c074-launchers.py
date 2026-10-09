from pathlib import Path
import difflib,hashlib,json
B=Path(__file__).parent
module='''// CLOUD-074: disposal diagnostics only; preserve the existing outer wait and failure.
export function monitorDisposalSnapshot(monitor,onFailure){
 const at={wall:new Date().toISOString(),monotonicMs:performance.now()};
 try{return {at,available:true,observation:structuredClone(monitor.disposalSnapshot())};}
 catch{onFailure(Error('Monitor disposal snapshot unavailable'));return {at,available:false,observation:null};}
}
export async function observeMonitorDisposal(monitor,{boundedAttempt,onFailure}){
 const observation={version:1,entry:{wall:new Date().toISOString(),monotonicMs:performance.now()},operationEntered:null,wait:{state:'pending',at:null},before:monitorDisposalSnapshot(monitor,onFailure),after:null};
 try{await boundedAttempt(()=>{observation.operationEntered={wall:new Date().toISOString(),monotonicMs:performance.now()};return monitor.dispose();},2000,'transport-observer-dispose-timeout');observation.wait={state:'fulfilled',at:{wall:new Date().toISOString(),monotonicMs:performance.now()}};}
 catch(error){observation.wait={state:error?.code==='transport-observer-dispose-timeout'?'timeout':'rejected',at:{wall:new Date().toISOString(),monotonicMs:performance.now()}};onFailure(error);}
 finally{observation.after=monitorDisposalSnapshot(monitor,onFailure);}
 return structuredClone(observation);
}

'''
bindings=[]
for v in ['warm','cold']:
 p=B/v/'launch.mjs';raw=p.read_bytes();old=B/'before-c074'/v/'launch.mjs';old.parent.mkdir(parents=True,exist_ok=True);assert not old.exists();old.write_bytes(raw)
 s=raw.decode();assert s.count('export async function main(')==1;s=s.replace('export async function main(',module+'export async function main(')
 anchor="if(monitor)try{await boundedAttempt(()=>monitor.dispose(),2000,'transport-observer-dispose-timeout');}catch(e){receipt.cleanup.push(safeError(e));}"
 repl="if(monitor)receipt.monitorDisposal=await observeMonitorDisposal(monitor,{boundedAttempt,onFailure:error=>receipt.cleanup.push(safeError(error))});"
 assert s.count(anchor)==1;s=s.replace(anchor,repl)
 anchor='receipt.closingReadObservations=structuredClone(closingReadObservations);receipt.failureObservation='
 repl="receipt.closingReadObservations=structuredClone(closingReadObservations);if(monitor)receipt.monitorDisposalFinal=monitorDisposalSnapshot(monitor,error=>receipt.cleanup.push(safeError(error)));receipt.failureObservation="
 assert s.count(anchor)==1;s=s.replace(anchor,repl);p.write_text(s)
 (B/(v+'-c074-launch.mjs.diff')).write_text(''.join(difflib.unified_diff(raw.decode().splitlines(True),s.splitlines(True),fromfile='before-c074/'+v+'/launch.mjs',tofile=v+'/launch.mjs')))
 bindings.append({'variant':v,'beforeC074':{'sha256':hashlib.sha256(raw).hexdigest(),'bytes':len(raw)},'final':{'sha256':hashlib.sha256(s.encode()).hexdigest(),'bytes':len(s.encode())}})
(B/'c074-launcher-bindings.json').write_text(json.dumps(bindings,indent=2)+'\n')
print(json.dumps(bindings,indent=2))
