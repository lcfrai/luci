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
import {startFencedDockerProxy} from './runtime/launch-recovery/fenced-docker-proxy.mjs';
import {validateWorkerCreate} from './runtime/launch-recovery/worker-create-guard.mjs';
import {createWorkflowDeadline,runBoundedCleanup,writeFailureCheckpoint,boundedAttempt} from './runtime/launch-recovery/acceptance-lifecycle.mjs';
import {createStartupScope} from './runtime/launch-recovery/acceptance-startup.mjs';
import {createAcceptanceJournal,diagnosticError,journalProcessIdentity} from './runtime/launch-recovery/durable-acceptance-journal.mjs';
import {createNativeTerminalProbe} from './runtime/four-project-queue/native-terminal-diagnostics.mjs';
import {contextFileNames,verifyTaskContext} from './runtime/sustained-four-project/task-context.mjs';
import {pins as referencePins,bounds,routes,runWorkerRecoveryScenario} from './worker-recovery-scenario.mjs';
import {verifyPackage} from './provenance.mjs';
const here=dirname(fileURLToPath(import.meta.url)),sha=bytes=>createHash('sha256').update(bytes).digest('hex');
export async function main(args=process.argv.slice(2)){
 const valueOptions=new Set(['--repo','--binary','--web','--provenance','--image-provenance','--report','--fixture-root','--observer-instance-pointer','--expected-source','--expected-image-id']);
 const seen=new Set();for(let i=0;i<args.length;i++){const key=args[i];assert(!seen.has(key));seen.add(key);if(valueOptions.has(key)){assert(args[++i]&&!args[i].startsWith('--'));}else assert(['--help','--window-granted'].includes(key));}
 const option=(key,defaultValue)=>{const i=args.indexOf(key);return i<0?defaultValue:args[i+1];};
 if(args.includes('--help')){console.log('Prepared LOCAL-016 worker recovery adapter. Runtime requires --window-granted --expected-source EXACT_SHA --expected-image-id sha256:DIGEST --binary --web --provenance --image-provenance --report FRESH_PATH --fixture-root FRESH_SHORT_PARENT [--repo /workspace/luci] [--observer-instance-pointer PATH]. No browser, new image, compiler, or model calls.');return;}
 assert(args.includes('--window-granted'),'Exclusive root runtime window required');assert.equal(process.platform,'linux');assert.notEqual(process.getuid(),0);
 const repo=resolve(option('--repo','/workspace/luci')),binary=resolve(option('--binary')),web=resolve(option('--web')),report=resolve(option('--report')),evidence=dirname(report),fixtureParent=resolve(option('--fixture-root'));
 assert.equal(realpathSync(fixtureParent),fixtureParent);assert(lstatSync(fixtureParent).isDirectory());assert(!existsSync(evidence),'Fresh result directory required');
 const pins={product:option('--expected-source'),image:option('--expected-image-id')};assert.match(pins.product??'',/^[a-f0-9]{40}$/);assert.match(pins.image??'',/^sha256:[a-f0-9]{64}$/);
 const verified=verifyPackage({repo,binary,web,pins,provenancePath:resolve(option('--provenance')),imageProvenancePath:resolve(option('--image-provenance'))}),provenance=verified.package;
 const binding=JSON.parse(readFileSync(join(here,'source-binding.json'),'utf8'));assert.deepEqual(binding.referencePins,referencePins);
 for(const row of binding.files){assert(/^[A-Za-z0-9_.\/-]+$/.test(row.path)&&!row.path.split('/').includes('..'));const file=join(here,row.path);assert.equal(realpathSync(file),file);assert.equal(sha(readFileSync(file)),row.sha256);}
 const docker=(...args)=>execFileSync('docker',args,{encoding:'utf8',timeout:30000,maxBuffer:8*1024**2}).trim();
 assert.equal(docker('ps','-aq'),'');assert.equal(docker('volume','ls','-q'),'');const image=pins.image,imageInfo=JSON.parse(docker('image','inspect',image))[0];assert.equal(imageInfo.Id,image);
 const engine=JSON.parse(docker('info','--format','{{json .}}')),engineID=engine.ID;assert(engineID);assert.equal(statSync(engine.DockerRootDir).dev,statSync('/').dev);
 const bridge=docker('network','inspect','bridge','--format','{{(index .IPAM.Config 0).Gateway}}');assert.match(bridge,/^(10\.|172\.(1[6-9]|2\d|3[01])\.|192\.168\.)/);
 const rootFree=()=>{const stat=statfsSync('/');return Number(stat.bavail)*Number(stat.bsize);};assert(rootFree()>=bounds.minimumCreateFreeBytes);
 mkdirSync(evidence,{recursive:true,mode:0o700});const fixture=mkdtempSync(join(fixtureParent,'luci-worker-recovery-'));for(const name of ['home','projects','state','docker'])mkdirSync(join(fixture,name),{mode:0o700});
 const receipt={passed:false,mode:'installed-settled-worker-stop',startedAt:new Date().toISOString(),sourceCommit:pins.product,image:pins.image,packageIntegrity:verified.verified,bounds,externalModelCalls:0,browserRuns:0,compilerRuns:0,checks:[],apiActions:[],phases:[],cleanup:[],setupScaffolding:['Fresh team, two approved synthetic projects and queued tasks through installed API','One actual native Codex command per worker via scripted local provider','First native CLI final response held until supported Stop; second exits naturally','Controller restart while first worker and its recorded output remain live'],limitations:['No diagnosis or reuse of the original failed sustained fixture','API recovery mechanics only; no browser or model-quality claim','Stop has no request revision field; exact operation/revision negatives exercise supported task abandonment without retiring the live child']};
 mkdirSync(join(evidence,'accepted-harness'),{mode:0o700});for(const item of binding.files)writeFileSync(join(evidence,'accepted-harness',item.path.replaceAll('/','_')),readFileSync(join(here,item.path)),{flag:'wx',mode:0o600});
 writeFileSync(join(evidence,'source-binding.json'),JSON.stringify(binding,null,2)+'\n',{flag:'wx',mode:0o600});writeFileSync(join(evidence,'package-provenance.json'),verified.provenanceBytes,{flag:'wx',mode:0o600});
 const journal=createAcceptanceJournal(join(evidence,'diagnostic-journal.jsonl'));
 const key='fixture-only-'+randomUUID(),password=randomUUID()+randomUUID(),memberPassword=randomUUID()+randomUUID(),lifecycle=randomUUID()+randomUUID(),sensitive=[key,password,memberPassword,lifecycle];
 const scrub=value=>sensitive.reduce((text,secret)=>text.replaceAll(secret,'[redacted]'),String(value)).replaceAll(fixture,'[private-fixture]');
 const safeError=error=>({name:error?.name??'Error',code:error?.code??null,message:scrub(error?.message??error).slice(0,1000),locations:String(error?.stack??'').split('\n').filter(line=>/^\s+at /.test(line)).slice(0,8).map(scrub)});
 const encode=value=>{const text=JSON.stringify(value,null,2)+'\n';assert(Buffer.byteLength(text)<=8*1024**2);for(const secret of sensitive)assert(!text.includes(secret));return text;};
 let service,proxy,provider,base,env,token,ownerToken,ownerID,memberToken,memberID,team,teamRoot,workspace,providerBase,workflow,fixtureError,closing=false,stage='startup',reserveTimer;
 let providerCalls=0,auxiliaryCalls=0;const controllerIdentities=[],ownedSessions=new Set(),sockets=new Set(),socketSessions=new Map(),terminalProbes=[],taskData=new Map(),calls=new Map(),observations=new Map(),bundles=new Map(),initialBundles=new Map(),nativeHomes=new Map(),projectStates=[],titles=new Map();
 const authorityRoot='/home/dev/.luci-reviewed-context',names=contextFileNames(2),canonicalNames=contextFileNames(1),deadline=Date.now()+bounds.scenarioMs;
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

