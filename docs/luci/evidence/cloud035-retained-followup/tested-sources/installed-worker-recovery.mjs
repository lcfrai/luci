#!/usr/bin/env node
// LOCAL-016 fresh installed worker settlement witness. No original fixture input.
import assert from 'node:assert/strict';
import {spawn,execFileSync} from 'node:child_process';
import {createHash,randomUUID} from 'node:crypto';
import {createServer as httpServer} from 'node:http';
import {createServer} from 'node:net';
import {existsSync,statfsSync,statSync,mkdirSync,mkdtempSync,readFileSync,readdirSync,realpathSync,lstatSync,writeFileSync} from 'node:fs';
import {dirname,join,resolve,sep} from 'node:path';
import {fileURLToPath} from 'node:url';
import {setTimeout as delay} from 'node:timers/promises';
import {startFencedDockerProxy} from '../launch-recovery/fenced-docker-proxy.mjs';
import {validateWorkerCreate} from '../launch-recovery/worker-create-guard.mjs';
import {createWorkflowDeadline,runBoundedCleanup,writeFailureCheckpoint,boundedAttempt} from '../launch-recovery/acceptance-lifecycle.mjs';
import {createStartupScope} from '../launch-recovery/acceptance-startup.mjs';
import {createAcceptanceJournal,diagnosticError,journalProcessIdentity} from '../launch-recovery/durable-acceptance-journal.mjs';
import {createNativeTerminalProbe} from '../four-project-queue/native-terminal-diagnostics.mjs';
import {contextFileNames,verifyTaskContext} from '../sustained-four-project/task-context.mjs';
import {bounds,routes,runWorkerRecoveryScenario} from './worker-recovery-scenario.mjs';
import {verifyPackage} from './provenance.mjs';
import {captureSourceBinding} from './source-binding.mjs';
import {captureProjectContext,verifyCapturedProjectContext,projectExportPaths} from './project-context.mjs';
const here=dirname(fileURLToPath(import.meta.url)),sha=bytes=>createHash('sha256').update(bytes).digest('hex');
export async function main(args=process.argv.slice(2)){
 const valueOptions=new Set(['--repo','--binary','--web','--provenance','--image-provenance','--report','--fixture-root','--observer-instance-pointer','--expected-source','--expected-helper-source','--expected-image-id']);
 const seen=new Set();for(let i=0;i<args.length;i++){const key=args[i];assert(!seen.has(key));seen.add(key);if(valueOptions.has(key)){assert(args[++i]&&!args[i].startsWith('--'));}else assert(['--help','--window-granted'].includes(key));}
 const option=(key,defaultValue)=>{const i=args.indexOf(key);return i<0?defaultValue:args[i+1];};
 if(args.includes('--help')){console.log('Prepared LOCAL-016 worker recovery adapter. Runtime requires --window-granted --expected-source EXACT_SHA --expected-helper-source EXACT_SHA --expected-image-id sha256:DIGEST --binary --web --provenance --image-provenance --report FRESH_PATH --fixture-root FRESH_SHORT_PARENT [--repo /workspace/luci] [--observer-instance-pointer PATH]. No browser, new image, compiler, or model calls.');return;}
 assert(args.includes('--window-granted'),'Exclusive root runtime window required');assert.equal(process.platform,'linux');assert.notEqual(process.getuid(),0);
 const repo=resolve(option('--repo','/workspace/luci')),binary=resolve(option('--binary')),web=resolve(option('--web')),report=resolve(option('--report')),evidence=dirname(report),fixtureParent=resolve(option('--fixture-root'));
 assert.equal(realpathSync(fixtureParent),fixtureParent);assert(lstatSync(fixtureParent).isDirectory());assert(!existsSync(evidence),'Fresh result directory required');
 const pins={product:option('--expected-source'),image:option('--expected-image-id')};assert.match(pins.product??'',/^[a-f0-9]{40}$/);assert.match(pins.image??'',/^sha256:[a-f0-9]{64}$/);
 const verified=verifyPackage({repo,binary,web,pins,provenancePath:resolve(option('--provenance')),imageProvenancePath:resolve(option('--image-provenance'))}),provenance=verified.package;
 const verifyHelper=()=>captureSourceBinding({repo,actualDirectory:here,provenance,expectedProduct:pins.product,expectedHelper:option('--expected-helper-source')});
 const binding=verifyHelper();
 const docker=(...args)=>execFileSync('docker',args,{encoding:'utf8',timeout:30000,maxBuffer:8*1024**2}).trim();
 assert.equal(docker('ps','-aq'),'');assert.equal(docker('volume','ls','-q'),'');const image=pins.image,imageInfo=JSON.parse(docker('image','inspect',image))[0];assert.equal(imageInfo.Id,image);
 const engine=JSON.parse(docker('info','--format','{{json .}}')),engineID=engine.ID;assert(engineID);assert.equal(statSync(engine.DockerRootDir).dev,statSync('/').dev);
 const bridge=docker('network','inspect','bridge','--format','{{(index .IPAM.Config 0).Gateway}}');assert.match(bridge,/^(10\.|172\.(1[6-9]|2\d|3[01])\.|192\.168\.)/);
 const rootFree=()=>{const stat=statfsSync('/');return Number(stat.bavail)*Number(stat.bsize);};assert(rootFree()>=bounds.minimumCreateFreeBytes);
 mkdirSync(evidence,{recursive:true,mode:0o700});const fixture=mkdtempSync(join(fixtureParent,'luci-worker-recovery-'));for(const name of ['home','projects','state','docker'])mkdirSync(join(fixture,name),{mode:0o700});
 const receipt={passed:false,mode:'installed-settled-worker-stop',startedAt:new Date().toISOString(),sourceCommit:pins.product,image:pins.image,packageIntegrity:verified.verified,helperCheckout:binding.checkout,bounds,externalModelCalls:0,browserRuns:0,compilerRuns:0,checks:[],apiActions:[],phases:[],cleanup:[],setupScaffolding:['Fresh team, two approved synthetic projects and queued tasks through installed API','One actual native Codex command per worker via scripted local provider','First native CLI final response held until supported Stop; second exits naturally','Controller restart while first worker and its recorded output remain live'],limitations:['No diagnosis or reuse of the original failed sustained fixture','API recovery mechanics only; no browser or model-quality claim','Stop has no request revision field; exact operation/revision negatives exercise supported task abandonment without retiring the live child']};
 mkdirSync(join(evidence,'accepted-harness'),{mode:0o700});for(const item of binding.files)writeFileSync(join(evidence,'accepted-harness',item.path.replaceAll('/','_')),readFileSync(join(repo,item.path)),{flag:'wx',mode:0o600});
 writeFileSync(join(evidence,'source-binding.json'),JSON.stringify(binding,null,2)+'\n',{flag:'wx',mode:0o600});writeFileSync(join(evidence,'package-provenance.json'),verified.provenanceBytes,{flag:'wx',mode:0o600});
 const journal=createAcceptanceJournal(join(evidence,'diagnostic-journal.jsonl'));
 const key='fixture-only-'+randomUUID(),password=randomUUID()+randomUUID(),memberPassword=randomUUID()+randomUUID(),lifecycle=randomUUID()+randomUUID(),sensitive=[key,password,memberPassword,lifecycle];
 const scrub=value=>sensitive.reduce((text,secret)=>text.replaceAll(secret,'[redacted]'),String(value)).replaceAll(fixture,'[private-fixture]');
 const safeError=error=>({name:error?.name??'Error',code:error?.code??null,message:scrub(error?.message??error).slice(0,1000),locations:String(error?.stack??'').split('\n').filter(line=>/^\s+at /.test(line)).slice(0,8).map(scrub)});
 const encode=value=>{const text=JSON.stringify(value,null,2)+'\n';assert(Buffer.byteLength(text)<=8*1024**2);for(const secret of sensitive)assert(!text.includes(secret));return text;};
 let service,proxy,provider,base,env,token,ownerToken,ownerID,memberToken,memberID,team,teamRoot,workspace,providerBase,workflow,fixtureError,closing=false,stage='startup',reserveTimer;
 let providerCalls=0,auxiliaryCalls=0;const controllerIdentities=[],ownedSessions=new Set(),sockets=new Set(),socketSessions=new Map(),terminalProbes=[],taskData=new Map(),calls=new Map(),observations=new Map(),bundles=new Map(),initialBundles=new Map(),nativeHomes=new Map(),projectStates=[],titles=new Map();
 const authorityRoot='/home/dev/.luci-reviewed-context',names=contextFileNames(2),deadline=Date.now()+bounds.scenarioMs;
 const remainingBudget=ms=>{health();return Math.max(1,Math.min(ms??120000,deadline-Date.now()));};
 const health=()=>{if(closing)throw Error('Workflow closed');workflow?.assertActive();if(fixtureError)throw fixtureError;if(Date.now()>=deadline)throw Error('Scenario deadline expired');};
 const stopOnFailure=error=>{fixtureError??=error;workflow?.fail(error);};
 const startupScope=createStartupScope({assertActive:health,onFailure:stopOnFailure,onTransition:({name,state})=>journal.record('driver-stage',{stage:'startup-'+name+'-'+state})});
 const check=(name,detail={})=>receipt.checks.push({name,...detail});
 const record=value=>{stage=value.phase;receipt.phases.push(value);journal.record('driver-stage',{stage});writeFileSync(join(evidence,'phase-'+String(receipt.phases.length).padStart(2,'0')+'.json'),encode(value),{flag:'wx',mode:0o600});};
 const privateJSON=file=>{const stat=lstatSync(file);assert(stat.isFile()&&!stat.isSymbolicLink()&&stat.nlink===1&&stat.size<=2*1024**2);return JSON.parse(readFileSync(file,'utf8'));};
 const ledger=()=>privateJSON(join(fixture,'state/control-plane/execution/leases.json'));
 const registrySessions=()=>privateJSON(join(teamRoot,'state/sessions/sessions.json')).sessions;
 const row=id=>{const rows=registrySessions().filter(value=>value.id===id);assert.equal(rows.length,1);return rows[0];};
 const privateRun=owner=>{const run=privateJSON(join(teamRoot,'state/sessions/project-runs',owner.owner.id,owner.run.id+'.json')).run;assert.equal(run.id,owner.run.id);assert.equal(run.projectId,owner.owner.id);return run;};
 const instanceID=()=>readFileSync(join(teamRoot,'state/instance/instance-id'),'utf8').trim();
 const containerIDs=id=>{assert.match(id,/^s-[a-f0-9]{8}$/);return docker('ps','-aq','--no-trunc','--filter','label=luci.session-id='+id).split(/\s+/).filter(Boolean);};
 const workerContainers=()=>docker('ps','-aq','--no-trunc','--filter','label=luci.managed=true').split(/\s+/).filter(Boolean);
 const effects=()=>{assert.equal(proxy.observations.dropped,0);return {create:proxy.observations.counts.create,start:proxy.observations.counts.start,destroy:proxy.observations.requests.filter(r=>r.method==='DELETE').length};};
 const mutations=()=>proxy.observations.requests.filter(r=>!['GET','HEAD'].includes(r.method)).map(r=>({method:r.method,path:r.path,requestSequence:r.requestSequence}));
 async function request(method,path,body,auth=ownerToken,timeout=30000){health();const response=await fetch(base+path,{method,redirect:'error',signal:AbortSignal.timeout(remainingBudget(timeout)),headers:{Origin:base,...(auth?{Authorization:'Bearer '+auth}:{}),...(body===undefined?{}:{'Content-Type':'application/json'})},body:body===undefined?undefined:JSON.stringify(body)});const raw=await boundedResponse(response,2*1024**2),value=raw?JSON.parse(raw):{};receipt.apiActions.push({stage,method,path,status:response.status,at:new Date().toISOString()});health();return {status:response.status,body:value};}
 async function api(method,path,body,expected=200,timeout=30000){const response=await request(method,path,body,token,timeout);assert.equal(response.status,expected,method+' '+path+' '+scrub(response.body.error??''));return response.body;}
 async function until(label,fn,timeout=120000){const end=Date.now()+remainingBudget(timeout);while(Date.now()<end){health();const value=await fn();health();if(value)return value;await delay(150);}throw Error(label+' timed out');}
 function observeTaskLease(worker){const observation=worker.leaseObservation??={samples:0};observation.samples++;const present=ledger().leases.some(value=>value.sessionId===worker.session.id);if(present)observation.lastPresentAt=new Date().toISOString();else observation.firstAbsentAt??=new Date().toISOString();return observation;}


function processIdentity(pid) {
  try { const data = readFileSync(`/proc/${pid}/stat`, 'utf8'); const fields = data.slice(data.lastIndexOf(')') + 2).trim().split(/\s+/); return {pid:Number(pid),start:fields[19],group:Number(fields[2]),status:fields[0]}; }
  catch (error) { if (['ENOENT','ESRCH'].includes(error.code)) return null; throw error; }
}
function stillOwned(identity) { const now = processIdentity(identity.pid); return now && now.start === identity.start && now.status !== 'Z'; }
function ownedProcesses(family='service') {
  const state = join(fixture, 'state'), rows = [];
  for (const pid of readdirSync('/proc').filter(value => /^\d+$/.test(value))) {
    const identity = processIdentity(pid); if (!identity || identity.status === 'Z') continue;
    try {
      const values = Object.fromEntries(readFileSync(`/proc/${pid}/environ`, 'utf8').split('\0').filter(Boolean).map(value => {const i=value.indexOf('=');return [value.slice(0,i),value.slice(i+1)];}));
      if ((family==='browser'?values.LUCI_SWARM_BROWSER_FIXTURE===fixture:(values.LUCI_STATE_DIR === state || values.LUCI_STATE_DIR?.startsWith(state + '/control-plane/'))) && processIdentity(pid)?.start === identity.start) rows.push(identity);
    } catch (error) { if (!['ENOENT','ESRCH','EACCES','EPERM'].includes(error.code)) throw error; }
  }
  return rows;
}

async function freePort() { const server=createServer(); await new Promise((done,reject)=>server.once('error',reject).listen(0,'127.0.0.1',done));const port=server.address().port;await new Promise(done=>server.close(done));return port; }
async function startService() {
  health();
  service=spawn(binary,[],{cwd:fixture,env,detached:true,stdio:['ignore','ignore','ignore']});service.on('error',stopOnFailure);controllerIdentities.push(processIdentity(service.pid));
  const end=Date.now()+remainingBudget(45_000);let ready;
  while(Date.now()<end) { health();if(fixtureError)throw fixtureError;assert.equal(service.exitCode,null);try{const res=await fetch(base+'/v1/control-plane/status',{signal:AbortSignal.timeout(1500)});if(res.ok){ready=await res.json();break;}}catch{}await delay(150); }
  assert.equal(ready?.product,'Luci');assert.equal(ready?.version,'0.5.0');
  (receipt.lifecycle??=[]).push({action:'start',pid:service.pid,at:new Date().toISOString()});
}
async function stopService(strict=true) {
  if(!service?.pid)return true;
  const identities=ownedProcesses();
  if(service.exitCode===null){assert(identities.some(row=>row.pid===service.pid),'Owned service identity missing');service.kill('SIGTERM');}
  const end=Date.now()+20_000;
  while(Date.now()<end&&(identities.some(stillOwned)||ownedProcesses().length))await delay(100);
  const remainingRows=[...identities.filter(stillOwned),...ownedProcesses()];
  if(remainingRows.length){
    receipt.cleanup.push('Owned service required force termination; acceptance fails');receipt.passed=false;process.exitCode=1;
    for(const row of remainingRows)if(stillOwned(row)){try{process.kill(row.group===row.pid?-row.pid:row.pid,'SIGKILL');}catch(error){if(error.code!=='ESRCH')throw error;}}
    const final=Date.now()+5000;while(Date.now()<final&&remainingRows.some(stillOwned))await delay(100);
  }
  const stopped=!identities.some(stillOwned)&&ownedProcesses().length===0;
  (receipt.lifecycle??=[]).push({action:'stop',at:new Date().toISOString(),stopped,exitCode:service.exitCode,forced:remainingRows.length>0});
  if(strict){assert(stopped);assert.equal(remainingRows.length,0);assert.equal(service.exitCode,0);}
  return stopped;
}

function inspectOwned(id,sessionID,requireBundle=true){
  assert.match(id,/^[a-f0-9]{12,64}$/);assert(ownedSessions.has(sessionID));
  const value=JSON.parse(docker('inspect',id))[0];
  assert.equal(value.Config.Labels['luci.session-id'],sessionID);assert.equal(value.Config.Labels['luci.sandbox-id'],sessionID);assert.equal(value.Config.Labels['luci.managed'],'true');assert.equal(value.Image,imageInfo.Id);
  const instance=readFileSync(join(teamRoot,'state/instance/instance-id'),'utf8').trim();assert.equal(value.Config.Labels['luci.instance'],instance);
  const work=value.Mounts.filter(m=>m.Destination==='/home/dev/work');assert.equal(work.length,1);assert.equal(work[0].RW,true);const expected=taskData.get(row(sessionID).implementationContext.taskIds[0]);assert(expected);assert.equal(realpathSync(work[0].Source),realpathSync(expected.checkout));assert.notEqual(realpathSync(work[0].Source),realpathSync(expected.owner.path));assert(lstatSync(join(work[0].Source,'.git')).isDirectory());assert(!existsSync(join(work[0].Source,'.git/commondir')));assert(!existsSync(join(work[0].Source,'.git/objects/info/alternates')));assert(!value.Mounts.some(m=>m.Source.includes('docker.sock')));
  assert(expected.requestedLimits.memoryMiB>0&&expected.requestedLimits.cpuMillis>0);assert.equal(value.HostConfig.Memory,expected.requestedLimits.memoryMiB*1024**2);assert.equal(value.HostConfig.NanoCpus,expected.requestedLimits.cpuMillis*1_000_000);assert.equal(value.HostConfig.PidsLimit,256);assert.equal(value.HostConfig.Privileged,false);assert(value.HostConfig.SecurityOpt.includes('no-new-privileges:true'));
  const proof={resourceLimits:{memoryBytes:value.HostConfig.Memory,nanoCPUs:value.HostConfig.NanoCpus,pidsLimit:value.HostConfig.PidsLimit,privileged:value.HostConfig.Privileged,securityOpt:value.HostConfig.SecurityOpt},worktreeID:row(sessionID).worktreeId,id:value.Id,image:value.Image,pid:value.State.Pid,running:value.State.Running,startedAt:value.State.StartedAt,created:value.Created,readOnly:true,sessionID};
  if(!requireBundle)return proof; // Cleanup authority uses labels/image/worktree, not the feature assertion under test.
  const native=value.Mounts.filter(m=>m.Destination==='/home/dev/.luci-agent');assert.equal(native.length,1);assert.equal(native[0].RW,true);const nativeRoot=realpathSync(native[0].Source);assert.equal(nativeRoot,resolve(native[0].Source));assert(nativeRoot.startsWith(join(fixture,'state')+sep));if(nativeHomes.has(sessionID))assert.equal(nativeHomes.get(sessionID),nativeRoot);nativeHomes.set(sessionID,nativeRoot);
  const mounts=value.Mounts.filter(m=>m.Destination===authorityRoot);assert.equal(mounts.length,1);const mount=mounts[0];assert.equal(mount.Type,'bind');assert.equal(mount.RW,false);
  const root=realpathSync(mount.Source);assert.equal(root,resolve(mount.Source));assert(root.startsWith(join(fixture,'state')+sep));assert(root.endsWith('/.reviewed-context/'+sessionID));assert(!root.startsWith(realpathSync(expected.owner.path)+sep));
  bundles.set(sessionID,root);
  return proof;
}
function live(id){const ids=containerIDs(id);assert.equal(ids.length,1);const proof=inspectOwned(ids[0],id);assert(proof.running&&proof.pid>0);return proof;}
function bundleSnapshot(id){
  const root=bundles.get(id);assert(root);assert.equal(realpathSync(root),root);const files={};
  for(const name of [...names,'manifest.json']){const path=join(root,name);const info=lstatSync(path);assert(info.isFile()&&!info.isSymbolicLink()&&info.nlink===1);assert(info.size<=32*1024**2);files[name]={sha256:sha(readFileSync(path)),bytes:info.size,inode:String(info.ino),device:String(info.dev)};}
  assert.deepEqual(readdirSync(root).sort(),['.luci','LUCI.md','manifest.json'].sort());assert.deepEqual(readdirSync(join(root,'.luci')).sort(),names.filter(n=>n.startsWith('.luci/')).map(n=>n.slice(6)).sort());
  return files;
}
function hashes(snapshot){return Object.fromEntries(Object.entries(snapshot).map(([name,value])=>[name,value.sha256]));}

function holdFinalResponse(state,res){
 state.finalResponseRequests=(state.finalResponseRequests??0)+1;
 assert(state.finalResponseRequests<=4,'At most three identical final-answer transport retries');
 state.finalWaiters??=new Set();
 if(state.finalReleased)return Promise.resolve(true);
 return new Promise(resolveWait=>{
  let settled=false;
  const complete=send=>{if(settled)return;settled=true;clearTimeout(timer);res.off('close',closed);state.finalWaiters.delete(complete);if(!send)state.finalResponseAborts=(state.finalResponseAborts??0)+1;resolveWait(send);};
  const closed=()=>complete(false);
  const timer=setTimeout(()=>{stopOnFailure(Error('Controlled final answer hold timed out'));complete(false);},remainingBudget(300_000));
  state.finalWaiters.add(complete);res.once('close',closed);
 });
}

async function completeNaturally(worker,owner){
 assert(observations.has(worker.task));assert.equal(worker.codingDispatches,1);
 observeTaskLease(worker);
 worker.finalReleased=true;for(const finish of [...(worker.finalWaiters??[])])finish(true);
 const stopped=await until('Native one-shot task exits without a stop request',async()=>{const value=await api('GET',workspace+'/v1/sessions/'+worker.session.id);observeTaskLease(worker);return value.status==='exited'&&containerIDs(worker.session.id).length===0?value:null;},90_000);
 const stoppedAndAbsentObservedAt=new Date().toISOString();
 const probe=[...terminalProbes].reverse().find(value=>value.id===worker.session.id)?.probe;
 assert(probe,'Authenticated terminal observation must exist');
 await until('Clean native exit frame',()=>probe.snapshot().exitFrames>0,10_000);
 assert.deepEqual(probe.snapshot().exit,{classification:'exit-status',code:0},'Native CLI must report explicit exit status 0, not be killed or fail');assert.equal(probe.snapshot().trustEnterSent,false,'One-shot execution must not depend on fixture keystrokes');
 const completed=await until('Queue records natural task completion',async()=>{const value=await api('GET',owner.route+'/swarm-queue');return value.items.find(item=>item.childId===worker.session.id&&item.state==='completed');},30_000);
 const queueCompletedObservedAt=new Date().toISOString();observeTaskLease(worker);
 const current=(await api('GET',owner.route+'/runs/'+owner.run.id)).run;
 const task=current.swarm.tasks.find(value=>value.id===owner.task);assert.notEqual(task.integration?.state,'integrated');
 assert(!receipt.apiActions.some(value=>value.method==='DELETE'&&value.path===workspace+'/v1/sessions/'+worker.session.id),'Success workflow must not issue a worker stop');
 (receipt.naturalCompletions??=[]).push({task:worker.task,sessionID:worker.session.id,sessionStatus:stopped.status,recordedExitReason:stopped.exitReason,cleanNativeExitFrame:true,exitFrameObservedAt:probe.snapshot().exitObservedAt,stoppedAndAbsentObservedAt,queueCompletedObservedAt,leaseObservation:worker.leaseObservation,queueState:completed.state,codingDispatches:worker.codingDispatches,finalResponseRequests:worker.finalResponseRequests,finalResponseAborts:worker.finalResponseAborts??0,finalResponseRetries:(worker.finalResponseRequests??1)-1,integratedBeforeExit:false});
 return completed;
}

function streamResponse(res,item,sequence){
  const response={id:'resp_swarm_'+sequence,object:'response',created_at:1,status:'completed',model:'swarm-mechanics-fixture',output:[item],usage:{input_tokens:1,output_tokens:1,total_tokens:2,input_tokens_details:{cached_tokens:0},output_tokens_details:{reasoning_tokens:0}}};
  const events=[{type:'response.created',response:{...response,status:'in_progress',output:[]}},{type:'response.output_item.added',output_index:0,item:{...item,status:'in_progress',...(item.type==='function_call'?{arguments:''}:{content:[]})}}];
  if(item.type==='function_call')events.push({type:'response.function_call_arguments.delta',item_id:item.id,output_index:0,delta:item.arguments},{type:'response.function_call_arguments.done',item_id:item.id,output_index:0,arguments:item.arguments});
  else events.push({type:'response.content_part.added',item_id:item.id,output_index:0,content_index:0,part:{type:'output_text',text:'',annotations:[]}},{type:'response.output_text.delta',item_id:item.id,output_index:0,content_index:0,delta:item.content[0].text},{type:'response.output_text.done',item_id:item.id,output_index:0,content_index:0,text:item.content[0].text});
  events.push({type:'response.output_item.done',output_index:0,item},{type:'response.completed',response});res.writeHead(200,{'content-type':'text/event-stream'});res.end(events.map((e,i)=>'event: '+e.type+'\ndata: '+JSON.stringify({...e,sequence_number:i})+'\n\n').join(''));
}
async function attach(id){
  const path=workspace+'/v1/sessions/'+id+'/attach';const {ticket}=await api('POST','/v1/auth/socket-ticket',{path});sensitive.push(ticket);
  const socket=new WebSocket(base.replace('http:','ws:')+path+'?ticket='+encodeURIComponent(ticket));socket.binaryType='arraybuffer';sockets.add(socket);socketSessions.set(socket,id);
  assert(terminalProbes.length<8);const probe=createNativeTerminalProbe();terminalProbes.push({id,attachedStage:stage,probe});
  socket.addEventListener('message',event=>probe.frame(Buffer.from(event.data)));
  socket.addEventListener('close',event=>{probe.close(event.code,event.wasClean);sockets.delete(socket);socketSessions.delete(socket);});socket.addEventListener('error',()=>probe.error());
  await new Promise((done,reject)=>{const timeout=setTimeout(()=>reject(Error('Authenticated attach timed out')),remainingBudget(10_000));socket.addEventListener('open',()=>{clearTimeout(timeout);socket.send(Uint8Array.from([2,0,120,0,40]));done();},{once:true});socket.addEventListener('error',()=>{clearTimeout(timeout);reject(Error('Authenticated attach failed'));},{once:true});});
  return socket;
}

function outputs(input,result=[]) {if(!input||typeof input!=='object')return result;if(input.type==='function_call_output')result.push(input);for(const v of Array.isArray(input)?input:Object.values(input))outputs(v,result);assert(result.length<=100);return result;}
function message(text){return {id:'msg_swarm_'+providerCalls,type:'message',status:'completed',role:'assistant',content:[{type:'output_text',text,annotations:[]}]};}
function commandFor(task) {
 const value=taskData.get(task);assert(value);const op=value.operation.id;
 const script=String.raw`const fs=require('node:fs'),path=require('node:path'),crypto=require('node:crypto'),cp=require('node:child_process'),assert=require('node:assert/strict');
 (async()=>{const root=${JSON.stringify(authorityRoot)},task=${JSON.stringify(task)},operation=${JSON.stringify(op)},names=${JSON.stringify(names)};
 const files=Object.fromEntries(names.concat('manifest.json').map(name=>{const b=fs.readFileSync(path.join(root,name));return[name,{sha256:crypto.createHash('sha256').update(b).digest('hex'),bytes:b.length}]}));
 const text=fs.readFileSync(path.join(root,'LUCI.md'),'utf8'),section=text.slice(text.indexOf('## Assigned task contract'));
 const contract=JSON.parse(section.match(/\x60\x60\x60json\n([\s\S]*?)\n\x60\x60\x60/)[1]);assert.equal(contract.selectedTasks.length,1);assert.equal(contract.selectedTasks[0].id,task);assert.deepEqual(contract.selectedTasks[0].writePaths,[task+'-result.txt']);assert.equal(JSON.parse(fs.readFileSync(path.join(root,'.luci/plan.json'))).projectId,${JSON.stringify(value.owner.id)});
 let readOnly=false;try{fs.writeFileSync(path.join(root,'.swarm-fixture-read-only-probe'),'must not be writable',{flag:'wx'});}catch(e){assert.equal(e.code,'EROFS');readOnly=true;}assert(readOnly);
 const git=(...a)=>cp.execFileSync('git',a,{encoding:'utf8'}).trim();assert.equal(git('status','--porcelain'),'');const base=git('rev-parse','HEAD');

 const content='Controlled '+task+' result\n';fs.writeFileSync(task+'-result.txt',content,{flag:'wx'});assert.equal(fs.readFileSync(task+'-result.txt','utf8'),content);
 git('add','--',task+'-result.txt');git('-c','user.name=Luci swarm fixture','-c','user.email=swarm@example.invalid','commit','-m','Controlled task '+task);assert.equal(git('status','--porcelain'),'');
 const result={task,operation,uid:process.getuid(),cwd:process.cwd(),files,readOnly,base,commit:git('rev-parse','HEAD'),changed:git('diff','--name-only',base,'HEAD').split('\n'),outputSHA256:crypto.createHash('sha256').update(fs.readFileSync(task+'-result.txt')).digest('hex')};console.log('LUCI_SWARM_OBSERVATION='+Buffer.from(JSON.stringify(result)).toString('base64'));})().catch(e=>{console.error(e);process.exitCode=1;});`;
 return {cmd:'node -e "eval(Buffer.from(\''+Buffer.from(script).toString('base64')+'\',\'base64\').toString())"',workdir:'/home/dev/work',login:false,yield_time_ms:1000,max_output_tokens:4000};
}
function nextTool(state,name,args){health();if(name==='exec_command'){state.codingDispatches=(state.codingDispatches??0)+1;assert.equal(state.codingDispatches,1,'No duplicate task command');}assert(++state.count<=30,'Bounded task tool request cap');const callID='call_swarm_'+state.task+'_'+state.operation.id+'_'+state.count;calls.set(callID,{state,consumed:false});return {id:'fc_'+callID,type:'function_call',status:'completed',name,call_id:callID,arguments:JSON.stringify(args)};}
function startProvider(){
 provider=httpServer(async(req,res)=>{try{
  assert.equal(req.method,'POST');assert.equal(req.headers.authorization,'Bearer '+key);if(closing){res.writeHead(503);res.end();return;}
  assert.equal(req.url,'/v1/responses');const parts=[];let bytes=0;for await(const part of req){bytes+=part.length;assert(bytes<4*1024**2);parts.push(part);}const body=JSON.parse(Buffer.concat(parts));assert.equal(body.model,'swarm-mechanics-fixture');assert(++providerCalls<=16);
  const resultItems=outputs(body.input),pending=resultItems.filter(v=>calls.has(v.call_id)&&!calls.get(v.call_id).consumed);assert(pending.length<=1,'Ambiguous unconsumed task output');let response;
  if(pending.length){const output=pending[0],entry=calls.get(output.call_id),state=entry.state;entry.consumed=true;const raw=typeof output.output==='string'?output.output:JSON.stringify(output.output);assert(!/Process exited with code (?:[1-9]\d*|-\d+)|"exit_code"\s*:\s*(?:[1-9]\d*|-\d+)|Command failed/.test(raw));
   const match=[...raw.matchAll(/LUCI_SWARM_OBSERVATION=([A-Za-z0-9+/=]+)/g)];
   if(match.length){assert.equal(match.length,1);assert(!observations.has(state.task));const actual=JSON.parse(Buffer.from(match[0][1],'base64'));assert.equal(actual.task,state.task);assert.equal(actual.operation,state.operation.id);assert.equal(actual.uid,process.getuid());assert.equal(actual.cwd,'/home/dev/work');assert.equal(actual.readOnly,true);assert.deepEqual(hashes(actual.files),hashes(initialBundles.get(state.session.id)));assert.deepEqual(actual.changed,[state.task+'-result.txt']);assert.match(actual.commit,/^[a-f0-9]{40,64}$/);observations.set(state.task,actual);entry.finalTask=state.task;entry.finalOutputSHA256=sha(raw);if(!await holdFinalResponse(state,res))return;response=message('Controlled task '+state.task+' committed. Human integration and complete build verification remain separate.');}
   else {const running=raw.match(/Process running with session ID (\d+)/)||raw.match(/"session_id"\s*:\s*(\d+)/);assert(running,'Actual tool output must supply completion receipt or live command identity');response=nextTool(state,'write_stdin',{session_id:Number(running[1]),chars:'',yield_time_ms:1000,max_output_tokens:4000});}
  }else if(resultItems.some(value=>calls.get(value.call_id)?.finalTask)){
   const finals=resultItems.filter(value=>calls.get(value.call_id)?.finalTask);assert.equal(finals.length,1);
   const output=finals[0],entry=calls.get(output.call_id),state=entry.state,raw=typeof output.output==='string'?output.output:JSON.stringify(output.output);
   assert.equal(sha(raw),entry.finalOutputSHA256,'Only an identical verified final-output request may retry');assert(observations.has(state.task));assert.equal(state.codingDispatches,1);
   if(!await holdFinalResponse(state,res))return;response=message('Controlled task '+state.task+' committed. Human integration and complete build verification remain separate.');
  }else if((body.tools??[]).some(t=>(t.name??t.function?.name)==='exec_command')){
   const text=JSON.stringify(body.input);assert(text.includes('Reviewed context directory: '+authorityRoot));const matched=[...titles].filter(([task,title])=>text.includes(task+': '+title)).map(([task])=>task);assert.equal(matched.length,1,'Native opening prompt must identify exactly one assigned fixture task');const task=matched[0];assert(['build-a','build-b'].includes(task),'Only declared fresh workers may reach provider');await until('Controller verifies actual task bundle',()=>taskData.get(task)?.session&&initialBundles.has(taskData.get(task).session.id));const state=taskData.get(task);assert.equal(state.count,0,'No duplicate coding dispatch');response=nextTool(state,'exec_command',commandFor(task));
  }else{assert(++auxiliaryCalls<=4);response=message('Controlled swarm fixture acknowledgement.');}
  (receipt.providerRequests??=[]).push({number:providerCalls,toolOutputCount:resultItems.length,issuedTool:response.type==='function_call'?response.name:null});streamResponse(res,response,providerCalls);
 }catch(error){stopOnFailure(error);if(!res.headersSent)res.writeHead(500);res.end('Controlled swarm fixture contract failed');}});
}

async function queuedChild(owner,ordinal){
 const queued=await until('Exactly one queued worker allocated',async()=>{
  const queue=await api('GET',owner.route+'/swarm-queue');return queue.items.find(item=>item.taskId===owner.task&&item.childId&&item.state==='running');
 },150000);
 const session=await api('GET',workspace+'/v1/sessions/'+queued.childId);ownedSessions.add(session.id);
 owner.run=(await api('GET',owner.route+'/runs/'+owner.run.id)).run;
 const operation=owner.run.operations.find(op=>op.taskId===owner.task&&op.childId===session.id);assert(operation);
 const worktrees=(await api('GET',workspace+'/v1/projects/'+owner.owner.id+'/worktrees')).worktrees,checkout=worktrees.find(w=>w.id===operation.worktreeId);assert(checkout);
 const state={task:owner.task,owner:owner.owner,operation,session,checkout:checkout.path,count:0,requestedLimits:{memoryMiB:queued.memoryMiB,cpuMillis:queued.cpuMillis}};taskData.set(owner.task,state);
 const actual=live(session.id),bundle=bundleSnapshot(session.id);initialBundles.set(session.id,bundle);state.container=actual;
 const capturedRoot=bundles.get(session.id);
 verifyCapturedProjectContext({snapshot:owner.contextSnapshot,read:name=>readFileSync(join(capturedRoot,name))});
 for(const name of ['.luci/brief.json','.luci/specification.json','.luci/design.json','.luci/plan.json'])assert.equal(JSON.parse(readFileSync(join(capturedRoot,name),'utf8')).projectId,owner.owner.id,'Mixed-project context: '+name);
 assert.equal(row(session.id).implementationContext.taskIds.length,1);assert.equal(row(session.id).projectId,owner.owner.id);
 assert.equal(workerContainers().length,1,'Only the successful successor may be active');
 return state;
}

async function boundedResponse(response, limit) {
 const parts=[];let size=0;
 for await(const part of response.body??[]){size+=part.length;if(size>limit){await response.body?.cancel().catch(()=>{});throw Error('HTTP response bound exceeded');}parts.push(part);}
 return Buffer.concat(parts,size).toString('utf8');
}

async function seedProjects(connectionID){let project,projectRoute,run;
  for(let index=0;index<2;index++){
   const letter='abcd'[index],task='build-'+letter;titles.set(task,'Build project '+letter.toUpperCase()+' result');
  project=await api('POST',workspace+'/v1/product/projects',{name:'recovery-project-'+letter},201);assert(realpathSync(project.path).startsWith(fixture+sep));projectRoute=workspace+'/v1/product/projects/'+project.id;
  await api('POST',projectRoute+'/brief',{revision:0,purpose:'Prove '+task+' uses its own project context and reviewed integration under a shared worker budget',audience:'Project owners',constraints:'Local labelled fixture; no cloud or model calls',criteria:['Only this project assigned file is changed and reviewed before integration']});
  let spec=await api('GET',projectRoute+'/specification');spec=await api('POST',projectRoute+'/specification',{expectedRevision:spec.revision,expectedBriefRevision:spec.briefRevision,specification:{...spec,requirements:[{id:'AC-notes',title:'Coordinate isolated task work with reviewed integration',priority:'must',acceptance:'Each task changes only its assigned file using its own captured project context; canceled work never allocates a worker.'}]}});
  spec=await api('POST',projectRoute+'/specification/approve',{revision:spec.revision,briefRevision:spec.briefRevision});
  let design=await api('GET',projectRoute+'/design-plan');design=await api('POST',projectRoute+'/design-plan',{expectedRevision:design.revision,expectedBriefRevision:spec.briefRevision,expectedSpecificationRevision:spec.revision,plan:{...design,screens:[{id:'notes',name:'Saved notes',route:'/',purpose:'Save and restore a note',states:['Empty','Saved','Error and retry'],widths:[390,1440],requirementIds:['AC-notes']}]}});
  design=await api('POST',projectRoute+'/design-plan/approve',{revision:design.revision});
  const tasks=[{id:task,title:titles.get(task),status:'todo',requirementIds:['AC-notes'],writePaths:[task+'-result.txt'],acceptance:'Read only this project frozen context, commit its exact controlled task output, and leave primary source unchanged until review.'}];
  const plan=await api('POST',projectRoute+'/plan',{expectedRevision:0,expectedSpecificationRevision:spec.revision,tasks});
  const gitReview=await api('GET',projectRoute+'/git');await api('POST',projectRoute+'/git/commit',{message:'Fixture seed: approved swarm task graph',expectedReviewDigest:gitReview.reviewDigest});
  const branches=(await api('GET',workspace+'/v1/projects/'+project.id+'/branches')).branches;const branch=branches.find(row=>row.name==='main')??branches[0];assert(branch);
  const worktrees=(await api('GET',workspace+'/v1/projects/'+project.id+'/worktrees?branchId='+encodeURIComponent(branch.id))).worktrees;const worktree=worktrees.find(row=>row.isPrimary)??worktrees[0];assert(worktree);
  const selection={projectId:project.id,briefRevision:spec.briefRevision,specificationRevision:spec.revision,designPlanRevision:design.revision,planRevision:plan.revision,taskIds:tasks.map(item=>item.id)};
  const runInput={worktreeId:worktree.id,harness:'codex',model:'swarm-mechanics-fixture',permissionMode:'bypass',agentConnectionId:connectionID,outcome:'preview',implementationContext:selection};
  run=(await api('POST',projectRoute+'/runs',{...runInput,idempotencyKey:'create-installed-swarm-fixture'},201)).run;assert.equal(run.input.route.permissionMode,'bypass');
  const contextSnapshot=captureProjectContext({projectId:project.id,briefRevision:selection.briefRevision,context:await api('GET',projectRoute+'/context'),brief:await api('GET',projectRoute+'/brief'),read:name=>readFileSync(join(project.path,name))});
  const initialHead=execFileSync('git',['rev-parse','HEAD'],{cwd:project.path,encoding:'utf8'}).trim();
   projectStates.push({owner:project,route:projectRoute,run,task,index,initialHead,contextSnapshot,goal:'Prove '+task+' uses its own project context and reviewed integration under a shared worker budget'});
  }

}

 async function pause(paused){const settings=await api('GET','/v1/product/swarm-scheduler');const value=await api('PUT','/v1/product/swarm-scheduler',{expectedRevision:settings.revision,policy:{maxActive:1,maxPerProject:1,headroomPercent:25,minHeadroomMiB:2048,diskReserveMiB:1024},dispatchPaused:paused});assert.equal(value.dispatchPaused,paused);for(const owner of projectStates){const queue=await api('GET',owner.route+'/swarm-queue');assert.equal(queue.dispatchPaused,paused);}return value;}
 function identifyCreate(bytes,meta){health();const input=JSON.parse(bytes),id=input.Labels?.['luci.session-id'];assert.match(id??'',/^s-[a-f0-9]{8}$/);assert.equal(meta.name,'luci-'+id);assert.equal(input.Labels?.['luci.launch-fence'],undefined);const owners=projectStates.filter(owner=>privateRun(owner).operations.some(op=>op.childId===id));assert.equal(owners.length,1);const owner=owners[0],op=privateRun(owner).operations.find(op=>op.childId===id);assert.equal(op.taskId,owner.task);assert.equal(proxy?.observations.requests.filter(item=>item.method==='POST'&&/\/containers\/create$/.test(item.path)&&item.labels?.['luci.session-id']===id&&item.requestSequence!==meta.requestSequence).length??0,0,'No worker Create replay');ownedSessions.add(id);validateWorkerCreate(input,{fixture:teamRoot,projectID:owner.owner.id,runID:owner.run.id,operationID:op.id,session:row(id),images:[image],instance:instanceID(),uid:process.getuid(),gid:process.getgid(),taskContextVersion:2});return 'worker';}
 function admission({kind,ordinal,body, ...meta}){health();assert.equal(kind,'worker');assert(ordinal>=1&&ordinal<=2);assert(rootFree()>=bounds.minimumCreateFreeBytes);assert.equal(workerContainers().length,0);const input=JSON.parse(body),id=input.Labels['luci.session-id'],owner=projectStates.find(owner=>privateRun(owner).operations.some(op=>op.childId===id)),op=privateRun(owner).operations.find(op=>op.childId===id);validateWorkerCreate(input,{fixture:teamRoot,projectID:owner.owner.id,runID:owner.run.id,operationID:op.id,session:row(id),images:[image],instance:instanceID(),uid:process.getuid(),gid:process.getgid(),taskContextVersion:2});receipt.checks.push({name:'Fresh worker admission',ordinal,rootFreeBytes:rootFree()});}
 let serviceStarts=0;
 async function startOwnedService(){assert(serviceStarts<2);const name=['controller-first','controller-second'][serviceStarts++];await startupScope.acquire(name,{start:async()=>{await startService();return service;},close:()=>stopService(false)});}
 async function prepareFresh(){
  await startupScope.acquire('provider',{start:async()=>{startProvider();await new Promise((done,reject)=>provider.once('error',reject).listen(0,bridge,done));return provider;},close:async handle=>{handle.closeAllConnections();if(handle.listening)await new Promise(done=>handle.close(done));}});providerBase='http://host.docker.internal:'+provider.address().port+'/v1';
  const upstream=process.env.DOCKER_HOST??'unix:///var/run/docker.sock';assert(upstream.startsWith('unix://'));
  proxy=await startupScope.acquire('proxy',{start:()=>startFencedDockerProxy({socketPath:join(fixture,'docker-proxy.sock'),upstreamSocketPath:upstream.slice(7),journal,identifyCreate,beforeCreate:admission,beforeStart:()=>health(),maxCreates:2,maxCreatesByKind:{worker:2,fence:1},maxStarts:2,upstreamTimeoutMs:120000,holdTimeoutMs:15000,maxObservations:20000}),close:handle=>handle.close()});
  const port=await freePort(),preview=await freePort();base='http://127.0.0.1:'+port;
  writeFileSync(join(fixture,'docker/config.json'),JSON.stringify({cliPluginsExtraDirs:process.env.DOCKER_CONFIG?[join(process.env.DOCKER_CONFIG,'cli-plugins')]:[]}),{mode:0o600});
  env=Object.fromEntries(['PATH','LANG','LC_ALL','TZ'].filter(name=>process.env[name]).map(name=>[name,process.env[name]]));
  Object.assign(env,{HOME:join(fixture,'home'),DOCKER_CONFIG:join(fixture,'docker'),DOCKER_HOST:'unix://'+join(fixture,'docker-proxy.sock'),LUCI_DOCKER_HOST:'unix://'+join(fixture,'docker-proxy.sock'),LUCI_ADDR:'127.0.0.1:'+port,LUCI_STATE_DIR:join(fixture,'state'),LUCI_PROJECTS_DIR:join(fixture,'projects'),LUCI_WEB_DIR:web,LUCI_RUNTIME_DIR:join(dirname(binary),'runtime'),LUCI_SANDBOX_IMAGE:image,LUCI_BACKEND:'docker',LUCI_SESSION_MEMORY_MIB:'1024',LUCI_SESSION_CPU_MILLIS:'1000',LUCI_MANAGED_SHUTDOWN_TOKEN:lifecycle,LUCI_LOG_LEVEL:'error',LUCI_BROWSER_PREVIEW_ADDR:'127.0.0.1:'+preview,LUCI_BROWSER_PREVIEW_PORT:String(preview),GIT_CONFIG_GLOBAL:join(fixture,'home/.gitconfig'),GIT_CONFIG_NOSYSTEM:'1'});
  await startOwnedService();const bootstrap=readFileSync(join(fixture,'state/control-plane/bootstrap-token'),'utf8').trim();sensitive.push(bootstrap);
  const account=await api('POST','/v1/auth/bootstrap',{bootstrapToken:bootstrap,username:'swarmowner',displayName:'Worker Recovery Fixture Owner',password},201);token=ownerToken=account.token;ownerID=account.user.id;sensitive.push(token);
  await api('PATCH','/v1/admin/policy',{sessionMemoryMiB:1024,sessionCPUMillis:1000,maxSessionsPerUser:1});
  team=await api('POST','/v1/teams',{name:'LOCAL016 fresh worker recovery'},201);workspace='/v1/teams/'+team.team.id+'/workspace';teamRoot=join(fixture,'state/control-plane/teams',team.team.id,'runtime');
  await api('POST',workspace+'/v1/setup/git/identity',{name:'Worker Recovery Fixture',email:'worker-recovery@example.invalid'});
  const ready=await api('POST',workspace+'/v1/setup/check',{});for(const id of ['docker','sandbox-image'])assert.equal(ready.checks.find(row=>row.id===id)?.status,'ready');
  const connections=await api('POST',workspace+'/v1/setup/agent-connections',{action:'save',expectedRevision:0,name:'Local scripted worker recovery',kind:'compatible-api',enabled:true,endpoints:{responses:providerBase},models:{codex:'swarm-mechanics-fixture'},apiKey:key});assert.equal(connections.connections.length,1);
  await pause(true);await seedProjects(connections.connections[0].id);
  const member=await api('POST','/v1/admin/users',{username:'workereditor',displayName:'Current Worker Control Authority',password:memberPassword,role:'member'},201);memberID=member.id;
  team=await api('POST','/v1/teams/'+team.team.id+'/members',{expectedRevision:team.team.revision,username:'workereditor',role:'editor',grants:{execute:true}});
  await setMemberGrant({role:'editor',execute:true});const login=await api('POST','/v1/auth/login',{username:'workereditor',password:memberPassword});memberToken=login.token;sensitive.push(memberToken);
  const pointer=option('--observer-instance-pointer');if(pointer)writeFileSync(resolve(pointer),join(teamRoot,'state/instance/instance-id')+'\n',{flag:'wx',mode:0o600});
 }
 async function setMemberGrant({role,execute}){team=await api('PUT','/v1/teams/'+team.team.id+'/projects/'+projectStates[0].owner.id+'/members/'+memberID,{expectedRevision:team.team.revision,role,grants:{execute}});(receipt.grantChanges??=[]).push({actorId:memberID,teamId:team.team.id,projectId:projectStates[0].owner.id,teamRevision:team.team.revision,role,execute,at:new Date().toISOString()});}
 async function queueBoth(){for(const owner of projectStates){owner.run=(await api('POST',owner.route+'/runs/'+owner.run.id+'/swarm',{expectedRevision:owner.run.revision,maxParallel:1},201)).run;const result=await api('POST',owner.route+'/runs/'+owner.run.id+'/swarm/queue',{expectedRevision:owner.run.revision,idempotencyKey:'local016-'+owner.task,taskIds:[owner.task],priority:owner.task==='build-a'?'high':'normal'},201);assert.equal(result.items.length,1);owner.queued=result.items[0];}assert.equal(effects().create,0);}
 let targetWorker,targetIdentity,retainedBaseline;const transcriptPrefixes=new Map();
 async function waitTargetActive(){targetWorker=await queuedChild(projectStates[0],1);const session=row(targetWorker.session.id),op=privateRun(projectStates[0]).operations.find(op=>op.id===targetWorker.operation.id);verifyTaskContext({version:2,session,operation:op,run:privateRun(projectStates[0]),files:Object.fromEntries([...names,'manifest.json'].map(name=>[name,readFileSync(join(bundles.get(session.id),name))]))});await attach(session.id);targetIdentity={teamId:team.team.id,projectId:session.projectId,runId:projectStates[0].run.id,operationId:op.id,childId:session.id,taskId:op.taskId,queueId:projectStates[0].queued.id,scope:sha(resolve(join(teamRoot,'state/conversations'))),engineId:engineID,instanceId:instanceID(),containerId:targetWorker.container.id,actor:{userId:ownerID,teamId:team.team.id,projectId:session.projectId},successorProjectId:projectStates[1].owner.id};return targetIdentity;}
 function transcriptFiles(root){const result=[];function walk(dir,depth=0){assert(depth<=6);for(const name of readdirSync(dir)){const file=join(dir,name),info=lstatSync(file);assert(!info.isSymbolicLink());if(info.isDirectory())walk(file,depth+1);else if(name.endsWith('.jsonl')){assert(info.isFile()&&info.size<=4*1024**2);result.push(file);assert(result.length<=16);}}}walk(root);return result;}
 function retained(){const worker=targetWorker,owner=projectStates[0],git=(cwd,...a)=>execFileSync('git',a,{cwd,encoding:'utf8',timeout:15000,maxBuffer:1024**2}).trim();
  if(!retainedBaseline){const root=join(nativeHomes.get(worker.session.id),'sessions');assert(existsSync(root),'Native transcript directory must exist');for(const file of transcriptFiles(root))transcriptPrefixes.set(file,readFileSync(file));assert(transcriptPrefixes.size>0,'Actual native transcript must be retained');}
  const transcript=[];for(const [file,prefix] of transcriptPrefixes){const info=lstatSync(file);assert(info.isFile()&&!info.isSymbolicLink()&&info.size>=prefix.length&&info.size<=4*1024**2);const bytes=readFileSync(file);assert(bytes.subarray(0,prefix.length).equals(prefix),'Recorded transcript prefix changed');transcript.push({relative:file.slice(nativeHomes.get(worker.session.id).length+1),retainedPrefixBytes:prefix.length,retainedPrefixSHA256:sha(prefix)});}
  const value={context:hashes(bundleSnapshot(worker.session.id)),checkoutHead:git(worker.checkout,'rev-parse','HEAD'),checkoutStatus:git(worker.checkout,'status','--porcelain'),outputSHA256:sha(readFileSync(join(worker.checkout,worker.task+'-result.txt'))),primaryHead:git(owner.owner.path,'rev-parse','HEAD'),primaryStatus:git(owner.owner.path,'status','--porcelain'),canonical:Object.fromEntries(projectExportPaths.map(name=>[name,sha(readFileSync(join(owner.owner.path,name)))])),observationSHA256:sha(JSON.stringify(observations.get(worker.task))),transcript};retainedBaseline??=value;return value;}
 async function snapshot(target){const owner=projectStates[0],successor=projectStates[1],run=(await api('GET',routes(target).run)).run,session=await api('GET',routes(target).stop),queue=await api('GET',routes(target).queue),qB=(await api('GET',successor.route+'/swarm-queue')).items.find(item=>item.id===successor.queued.id);assert(qB);const ids=containerIDs(target.childId);assert(ids.length<=1);let inspected;if(ids.length){assert.equal(ids[0],target.containerId);inspected=inspectOwned(ids[0],target.childId);}const observationsNow=proxy.observations.requests;
  const absent=ids.length===0;let exactBackendAbsence={confirmed:false};if(absent){try{docker('inspect',target.containerId);assert.fail('Removed exact container unexpectedly exists');}catch(error){if(error.code==='ERR_ASSERTION')throw error;const stderr=String(error.stderr??'');assert(error.status===1&&stderr.includes('No such object'),'Docker failure is not exact absence');exactBackendAbsence={confirmed:true,childId:target.childId,immutableId:target.containerId,observedAt:new Date().toISOString()};}}
  return {run,session,queue,ledger:ledger(),successor:qB,retained:retained(),containerId:inspected?.id??target.containerId,containerPID:inspected?.pid??0,exactBackendAbsence,mutations:mutations(),dispatches:Object.fromEntries([...taskData.values()].map(worker=>[worker.session.id,worker.codingDispatches??0])),createsForTarget:observationsNow.filter(r=>r.method==='POST'&&/\/containers\/create$/.test(r.path)&&r.labels?.['luci.session-id']===target.childId).length,startsForTarget:observationsNow.filter(r=>r.method==='POST'&&/\/start$/.test(r.path)&&r.labels?.['luci.session-id']===target.childId).length,fenceCreates:proxy.observations.counts.fence??0};}
 async function restartControllers(){const before=live(targetWorker.session.id);await stopService(true);await startupScope.close('controller-first',5000);assert.equal(live(targetWorker.session.id).id,before.id);assert.equal(live(targetWorker.session.id).pid,before.pid);await startOwnedService();token=ownerToken=(await api('POST','/v1/auth/login',{username:'swarmowner',password})).token;sensitive.push(token);const scheduler=await api('GET','/v1/product/swarm-scheduler');assert.equal(scheduler.dispatchPaused,true);memberToken=(await api('POST','/v1/auth/login',{username:'workereditor',password:memberPassword})).token;sensitive.push(memberToken);await attach(targetWorker.session.id);}
 async function stopWithUnavailableInspect(target){
  const ids=Array.from({length:8},(_,index)=>'unknown-exact-inspect-'+index),pathPattern=new RegExp('^/v[0-9]+\\.[0-9]+/containers/luci-'+target.childId+'/json$');
  // Cover every exact named Inspect during the request, including background
  // observation. A single gate could be consumed before Stop reached Docker.
  for(const id of ids)proxy.armGate({id,phase:'after-response',method:'GET',pathPattern,decision:'lose-response'});
  let response;try{response=await request('DELETE',routes(target).stop,undefined,memberToken,10000);}
  finally{for(const gate of proxy.observations.gates.filter(gate=>ids.includes(gate.id)&&gate.state==='armed'))await proxy.cancelGate(gate.id);}
  const lost=proxy.observations.gates.filter(gate=>ids.includes(gate.id)&&gate.state==='lost');assert(lost.length>0&&lost.length<8,'Exact Inspect failure window must not exhaust its gate budget');
  const requests=proxy.observations.requests.filter(row=>ids.includes(row.gateId));assert.equal(requests.length,lost.length);
  for(const row of requests){assert.equal(row.method,'GET');assert(pathPattern.test(row.path));assert.equal(row.status,200);assert(row.forwardedAt&&row.completedAt&&row.discardedAt);assert(!row.deliveredAt);}
  (receipt.negativeTransportWindows??=[]).push({childId:target.childId,gateBudget:8,lostExactInspectResponses:lost.length,stopStatus:response?.status,budgetExhausted:false});return response;
 }

 const ports={record,prepareFresh,pause,queueBoth,waitTargetActive,setMemberGrant,snapshot,restartControllers,stopWithUnavailableInspect,
  publicIdentity:target=>Object.fromEntries(['teamId','projectId','runId','operationId','childId','taskId','queueId','containerId','successorProjectId','actor'].map(key=>[key,target[key]])),
  waitRecordedOutput:async()=>{await until('Actual native command retained its output',()=>observations.has('build-a'),60000);assert.equal(targetWorker.codingDispatches,1);retained();},
  preview:target=>api('GET',routes(target).preview),readAsMember:target=>request('GET',routes(target).stop,undefined,memberToken),stopAsMember:target=>request('DELETE',routes(target).stop,undefined,memberToken),
  foreignProjectProbe:()=>request('GET',projectStates[1].route+'/runs/'+projectStates[1].run.id,undefined,memberToken),abandon:(target,body)=>request('POST',routes(target).abandon,body,ownerToken),
  waitReleased:target=>until('Exact stopped task lease and queue release',async()=>{const current=await snapshot(target),item=current.queue.items.find(row=>row.id===target.queueId);return current.session.status==='exited'&&!current.ledger.leases.some(row=>row.sessionId===target.childId)&&(item.memoryMiB??0)===0&&(item.cpuMillis??0)===0?current:null;},30000),
  completeSuccessor:async()=>{const owner=projectStates[1],worker=await queuedChild(owner,2);await attach(worker.session.id);await until('Second native task output',()=>observations.has(worker.task),60000);const completed=await completeNaturally(worker,owner);await until('Second exact lease released',()=>!ledger().leases.some(row=>row.sessionId===worker.session.id),30000);return {projectId:owner.owner.id,childId:worker.session.id,nativeExitCode:0,commandDispatches:worker.codingDispatches,queueState:completed.state,leaseAbsent:true,outputSHA256:observations.get(worker.task).outputSHA256};},
 };
 let scenario;const signals=[];
 try{workflow=createWorkflowDeadline({timeoutMs:bounds.scenarioMs,onFailure:stopOnFailure});for(const signal of ['SIGINT','SIGTERM']){const handler=()=>stopOnFailure(Error('Interrupted '+signal));process.on(signal,handler);signals.push([signal,handler]);}reserveTimer=setInterval(()=>{try{if(rootFree()<bounds.runningReserveBytes)stopOnFailure(Error('Running reserve breached'));}catch(error){stopOnFailure(error);}},1000);scenario=await workflow.run(()=>runWorkerRecoveryScenario(ports));receipt.scenario=scenario;}
 catch(error){fixtureError??=error;receipt.failedStage=stage;receipt.failure=safeError(error);}
 finally{closing=true;clearInterval(reserveTimer);workflow?.close();for(const [signal,handler] of signals)process.off(signal,handler);for(const socket of sockets)socket.close();
  receipt.cleanupPhases=await runBoundedCleanup({checkpoint:()=>writeFailureCheckpoint(report+'.incomplete.json',{version:1,passed:false,status:'cleanup-incomplete',stage,at:new Date().toISOString(),originalReceiptComplete:false}),onFailure:error=>receipt.cleanup.push(safeError(error)),phases:[
   {name:'controllers',timeoutMs:35000,run:async()=>{receipt.controllersStopped=await stopService(false);assert(receipt.controllersStopped);for(const name of ['controller-first','controller-second'])if(startupScope.has(name))await startupScope.close(name,5000);}},
   {name:'proxy',timeoutMs:125000,run:async()=>{if(startupScope.has('proxy')){const result=await startupScope.close('proxy',120000);assert(result.drained);receipt.dockerRequestsDrained=true;return result;}receipt.dockerRequestsDrained=true;}},
   {name:'provider',timeoutMs:5000,run:async()=>{if(startupScope.has('provider'))await startupScope.close('provider',5000);}},
  ]});
  try{assert(receipt.controllersStopped&&receipt.dockerRequestsDrained);assert.equal(ownedProcesses().length,0);const created=proxy?.observations.requests.filter(row=>row.method==='POST'&&/\/containers\/create$/.test(row.path)&&row.status===201&&row.id)??[];
   for(const creation of created){const current=docker('ps','-aq','--no-trunc').split(/\s+/).filter(Boolean);if(!current.includes(creation.id))continue;receipt.cleanup.push({message:'Unexpected retained worker requires exact failed-fixture cleanup'});const value=JSON.parse(docker('inspect',creation.id))[0];assert.equal(value.Id,creation.id);assert.equal(value.Name,'/'+creation.name);assert.equal(value.Config.Labels['luci.instance'],instanceID());assert.equal(value.Config.Labels['luci.session-id'],creation.labels['luci.session-id']);assert(ownedSessions.has(value.Config.Labels['luci.session-id']));assert.equal(value.Config.Labels['luci.managed'],'true');assert.equal(value.Image,image);if(value.State.Running)docker('stop','--time','5',creation.id);const stopped=JSON.parse(docker('inspect',creation.id))[0];assert.equal(stopped.State.Running,false);assert.equal(stopped.Id,creation.id);docker('rm',creation.id);}
   assert.equal(docker('ps','-aq'),'');assert.equal(docker('volume','ls','-q'),'');if(scenario){assert.equal(proxy.observations.counts.create,2);assert.equal(proxy.observations.counts.worker,2);assert.equal(proxy.observations.counts.start,2);assert.equal(proxy.observations.counts.fence,0);assert.equal(proxy.observations.dropped,0);assert.deepEqual(proxy.observations.failures,[]);}assert.equal(sha(readFileSync(binary)),provenance.binarySHA256);assert.equal(sha(readFileSync(join(web,'index.html'))),provenance.webIndexSHA256);assert.equal(JSON.parse(docker('image','inspect',image))[0].Id,image);assert.equal(docker('info','--format','{{.ID}}'),engineID);assert.deepEqual(verifyHelper(),binding,'Helper checkout or executed source changed during the attempt');receipt.runtimeCleanup={containers:0,volumes:0,ownedControllers:0};
  }catch(error){receipt.cleanup.push(safeError(error));}
  receipt.startupResources=startupScope.snapshot();receipt.dockerTransportObservations=proxy?.observations??null;receipt.actualTaskResults=Object.fromEntries(observations);receipt.terminalObservations=terminalProbes.map(({id,probe})=>({childId:id,...probe.snapshot()}));receipt.fixtureRetained={path:fixture,reason:'Fresh diagnostic state retained; contains fixture credentials. Original sustained fixture was not accessed.'};receipt.finishedAt=new Date().toISOString();receipt.passed=!!scenario&&!fixtureError&&!receipt.cleanup.length&&receipt.cleanupPhases.every(phase=>phase.status==='completed')&&receipt.startupResources.every(row=>row.state==='closed'&&!row.late&&!row.closeTimedOut);
  writeFileSync(report,encode(receipt),{flag:'wx',mode:0o600});journal.close();process.exitCode=receipt.passed?0:1;console.log(JSON.stringify({passed:receipt.passed,report,failedStage:receipt.failedStage??null}));
 }
}
if(process.argv[1]&&resolve(process.argv[1])===fileURLToPath(import.meta.url))await main();
