// Controlled clocks and API/locator observations; no browser, daemon or service.
import test from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFileSync} from 'node:fs';
import {waitSubmittedCheckGate,waitCheckPreflight} from './installed-recovery.mjs';
import {assessCheckPreflight,summarizeCheckLedger} from './fixture-guards.mjs';
import {command} from './recovery-scenario.mjs';
const clone=structuredClone,GiB=1024**3;
const source=readFileSync(new URL('./installed-recovery.mjs',import.meta.url),'utf8');
function gateFixture(){
 let clock=10000,reads=0,acks=0;const records=[];
 const check={id:'check-target',projectId:'p-target',status:'queued',revision:1};
 const gate={id:'target-create-ack-lost',phase:'after-response',state:'armed'};
 const request={kind:'check',method:'POST',path:'/v1.51/containers/create',status:201,id:'a'.repeat(64),labels:{'luci.check-project':'p-target','luci.check-id':'check-target'}};
 const options={proxy:{observations:{gates:[gate]},async waitForGate(id,{timeoutMs}){assert.equal(id,gate.id);assert(timeoutMs>0&&timeoutMs<=2000);acks++;return clone(gate);}},
  projectId:check.projectId,checkId:check.id,gateId:gate.id,phase:gate.phase,deadline:12000,now:()=>clock,
  wait:async ms=>{clock+=ms;},readCheck:async timeout=>{assert(timeout>0&&timeout<=2000);reads++;return clone(check);},recordTerminal:async row=>records.push(clone(row))};
 return {check,gate,request,records,options,capture:()=>Object.assign(gate,{state:'lost',request:clone(request)}),reads:()=>reads,acks:()=>acks,clock:()=>clock};
}
test('queued check becoming blocked is recorded promptly without acknowledgement or mutation',async()=>{
 const f=gateFixture(),read=f.options.readCheck;f.options.readCheck=async timeout=>{const row=await read(timeout);if(f.reads()===2)Object.assign(row,{revision:5,status:'blocked',stage:'admission',reasonCode:'cpu_pressure',resourcesHeld:false});return row;};
 await assert.rejects(waitSubmittedCheckGate(f.options),error=>error.code==='check-terminal-before-create-gate');
 assert.equal(f.clock(),10250);assert.equal(f.acks(),0);assert.equal(f.records.length,1);assert.equal(f.records[0].check.reasonCode,'cpu_pressure');assert.equal(f.records[0].check.revision,5);assert(f.records[0].checkRequestedAt);assert(f.records[0].checkObservedAt);
});
test('real exact acknowledgement captured during GET wins a legitimate terminal unknown race',async()=>{
 const f=gateFixture();f.options.readCheck=async()=>{f.capture();return {...f.check,status:'unknown',reasonCode:'check_create_unconfirmed'};};
 const got=await waitSubmittedCheckGate(f.options);assert.equal(got.request.id,f.request.id);assert.equal(f.acks(),1);assert.equal(f.records.length,0);
});
test('captured gate still requires exact check/project and real daemon 201 identity',async()=>{
 for(const mutate of [f=>f.request.labels['luci.check-id']='other',f=>f.request.labels['luci.check-project']='other',f=>f.request.status=500,f=>f.request.id='invented']){
  const f=gateFixture();mutate(f);f.capture();await assert.rejects(waitSubmittedCheckGate(f.options));assert.equal(f.acks(),0);
 }
});
test('queued checks remain bounded by the original supplied deadline',async()=>{
 const f=gateFixture();await assert.rejects(waitSubmittedCheckGate(f.options),error=>error.code==='check-create-readiness-deadline');assert.equal(f.clock(),12000);assert.equal(f.acks(),0);assert.equal(f.reads(),8);
});
test('workflow health is rechecked after the real gate acknowledgement',async()=>{
 const f=gateFixture();f.capture();let failed=false;f.options.proxy.waitForGate=async()=>{failed=true;return clone(f.gate);};f.options.health=()=>{if(failed)throw Object.assign(Error('workflow'),{code:'workflow-deadline-exceeded'});};
 await assert.rejects(waitSubmittedCheckGate(f.options),error=>error.code==='workflow-deadline-exceeded');
});
test('actual adapter records the terminal check even when current resource observation is unavailable',async()=>{
 const f=gateFixture(),records=[],receipt={};Object.assign(f.check,{status:'blocked',revision:5,stage:'admission',reasonCode:'cpu_pressure',resourcesHeld:false});
 const start=source.indexOf('    waitForCheckGate('),end=source.indexOf('    async startCheckInBrowser(',start);assert(start>=0&&end>start);
 const fakeDate=class extends Date{constructor(value){super(value??10000);}static now(){return 10000;}};
 const fn=vm.runInNewContext('({'+source.slice(start,end)+'}).waitForCheckGate',{
  waitSubmittedCheckGate:options=>waitSubmittedCheckGate({...options,now:()=>10000}),proxy:f.options.proxy,health:()=>{},workflowDeadline:12000,bounds:{requestMs:120000},Date:fakeDate,
  route:p=>'/projects/'+p.id,ownerToken:'controlled',receipt,record:row=>records.push(JSON.parse(JSON.stringify(row))),safeError:error=>({code:error.code}),readinessError:code=>Object.assign(Error(code),{code}),
  api:async(method,path)=>{assert.equal(method,'GET');if(path==='/projects/p-target/checks/check-target')return clone(f.check);assert.equal(path,'/v1/product/swarm-scheduler');throw Object.assign(Error('controlled unavailable'),{code:'resource-read-unavailable'});},
 },{timeout:1000});
 await assert.rejects(fn({id:'p-target'},'check-target','target-create-ack-lost','after-response'),error=>error.code==='check-terminal-before-create-gate');
 assert.equal(records.length,1);assert.deepEqual(records[0].check,f.check);assert.equal(records[0].resourceObservationError.code,'resource-read-unavailable');assert(records[0].resourceRequestedAt);assert(records[0].resourceObservedAt);assert.equal(f.acks(),0);
});
function sample(now,{cpu=20,stale=false,unavailable=false,headroom=25}={}){
 const observedAt=new Date(now-(stale?6000:0)).toISOString();
 return {view:{policy:{headroomPercent:headroom,minHeadroomMiB:2048,diskReserveMiB:1024},resources:{
  host:{status:unavailable?'unavailable':'ready',observedAt,memory:{effectiveTotalBytes:16*GiB,effectiveAvailableBytes:12*GiB},cpu:{effectiveMillis:4000,utilizationPercent:cpu}},
  execution:{status:'ready',observedAt,totalMemoryBytes:16*GiB,cpuMillis:4000}}},
  options:{ledger:{version:1,leases:[],checks:[]},engine:{Driver:'vfs',MemoryLimit:true,SwapLimit:true,CpuCfsPeriod:true,CpuCfsQuota:true,PidsLimit:true},rootFreeBytes:16*GiB,imageSize:GiB}};
}
test('prospective admission preserves policy, thresholds and full-ledger demand without mutation',()=>{
 const f=sample(10000,{cpu:75}),before=clone(f);const ready=assessCheckPreflight(f.view,{...f.options,now:10000});assert.equal(ready.ready,true);assert.equal(ready.cpuCeilingPercent,100-f.view.policy.headroomPercent);assert.deepEqual(ready.prospective,{memoryMiB:1024,cpuMillis:1000});assert.deepEqual(f,before);
 for(const changes of [{cpu:75.01},{cpu:null},{stale:true},{unavailable:true},{headroom:10}]){const f=sample(10000,changes);assert.equal(assessCheckPreflight(f.view,{...f.options,now:10000}).ready,false);}
 const low=sample(10000);low.options.rootFreeBytes=5*GiB;assert.equal(assessCheckPreflight(low.view,{...low.options,now:10000}).ready,false);
 const charged=sample(10000);charged.options.ledger.checks=[{state:'creating',resources:{CPUMillis:1000,MemoryMiB:1024},createDiskBytes:2*GiB}];assert.deepEqual(assessCheckPreflight(charged.view,{...charged.options,now:10000}).prospective,{memoryMiB:2048,cpuMillis:2000});
});
function browserStartFixture({samples=[{cpu:99},{cpu:20}],expireDuringRead=false}={}){
 let clock=10000,reads=0,posts=0,count=0;const records=[],receipt={},ledger={version:1,leases:[],checks:[],privateSentinel:'must-never-be-published-ledger-field'};
 const plain=x=>JSON.parse(JSON.stringify(x)),vmAssert=Object.assign((v,m)=>assert(v,m),assert,{deepEqual:(a,b,m)=>assert.deepEqual(plain(a),plain(b),m)});
 const region={locator:selector=>selector==='.check-commands > li'?{count:async()=>count}:{evaluate:async()=>true,filter:()=>({evaluate:async()=>true})},getByLabel:()=>({fill:async()=>{}}),getByRole:(_role,{name})=>({check:async()=>{},click:async()=>{if(name==='Add check command')count++;else assert.equal(name,'Run selected requirement checks');}})};
 const coverage={source:{available:true,dirty:false,commit:'a'.repeat(40),sourceDigest:'b'.repeat(64)},specification:{status:'approved',revision:2},design:{status:'approved',revision:2}};
 const start=source.indexOf('    async startCheckInBrowser('),end=source.indexOf('    async waitCheckTerminal(',start);assert(start>=0&&end>start);
 const fakeDate=class extends Date{constructor(value){super(value??clock);}static now(){return clock;}};
 const context={assert:vmAssert,setStage:()=>{},openProject:async()=>{},authorizedChecks:new Map([['history',{}]]),page:{locator:()=>region},route:p=>'/projects/'+p.id,
  api:async(method,path)=>{assert.equal(method,'GET');if(path.endsWith('/checks'))return {coverage};assert.equal(path,'/v1/product/swarm-scheduler');const got=sample(clock,samples[Math.min(reads++,samples.length-1)]);if(expireDuringRead)clock=13000;return got.view;},
  pendingConsent:null,randomUUID:()=> 'controlled-consent',Date:fakeDate,workflowDeadline:13000,health:()=>{},record:r=>records.push(plain(r)),ledger:()=>ledger,engine:sample(clock).options.engine,free:()=>16*GiB,image:{Size:GiB},ownerToken:'controlled',receipt,
  waitCheckPreflight:options=>waitCheckPreflight({...options,now:()=>clock,wait:async ms=>{assert.equal(posts,0);clock+=ms;}}),
  action:async(path,click,status)=>{assert.equal(path,'/projects/p-target/checks');assert.equal(status,202);await click();posts++;return {row:{id:'check-target',projectId:'p-target',status:'blocked',reasonCode:'cpu_pressure'},request:{commands:[command],execution:'docker',acknowledgeHostExecution:true,requirementIds:['AC-recovery'],expectedSpecificationRevision:2,expectedDesignPlanRevision:2}};},
 };
 const fn=vm.runInNewContext('({'+source.slice(start,end)+'}).startCheckInBrowser',context,{timeout:1000});
 return {run:()=>fn({id:'p-target'},command),records,receipt,posts:()=>posts,reads:()=>reads,clock:()=>clock};
}
test('actual browser method waits for fresh CPU capacity then submits one POST and never replays a blocked response',async()=>{
 const f=browserStartFixture();const row=await f.run();assert.equal(row.status,'blocked');assert.equal(f.posts(),1);assert.equal(f.reads(),2);assert.equal(f.clock(),11000);assert.equal(f.receipt.checkConsents.length,1);
 const ready=f.records.find(row=>row.phase==='check-preflight-ready');assert.equal(ready.attempts,2);assert.equal(ready.resources.host.cpu.utilizationPercent,20);assert.equal(ready.policy.headroomPercent,25);assert.equal(ready.ledgerSummary.activeCheckCount,0);assert.equal(ready.ledgerSummary.workerLeaseCount,0);assert.match(ready.ledgerSummary.sha256,/^[a-f0-9]{64}$/);assert.equal(ready.ledger,undefined);assert(!JSON.stringify(f.records).includes('must-never-be-published-ledger-field'));
});
test('actual browser method cannot submit on stale/unavailable reads or an expired shared budget',async()=>{
 for(const options of [{samples:[{stale:true}]},{samples:[{unavailable:true}]},{samples:[{cpu:99}]},{samples:[{cpu:20}],expireDuringRead:true}]){
  const f=browserStartFixture(options);await assert.rejects(f.run(),error=>error.code==='check-preflight-deadline');assert.equal(f.posts(),0);assert.equal(f.clock(),13000);assert.equal(f.records.at(-1).phase,'check-preflight-refused');
 }
});
test('shared workflow refusal during preflight preserves the last actual observation',async()=>{
 const records=[];let failed=false;await assert.rejects(waitCheckPreflight({deadline:13000,now:()=>10000,read:async()=>{failed=true;return sample(10000);},record:row=>records.push(row),health:()=>{if(failed)throw Object.assign(Error('workflow'),{code:'workflow-deadline-exceeded'});}}),error=>error.code==='workflow-deadline-exceeded');
 assert.equal(records.length,1);assert.equal(records[0].last.resources.host.cpu.utilizationPercent,20);
});
test('ledger diagnostic hashes canonical parsed values and excludes private records',()=>{
 const first={version:1,leases:[],checks:[{state:'creating',actor:{userId:'private-ledger-actor'}}]};
 const second={checks:[{actor:{userId:'private-ledger-actor'},state:'creating'}],leases:[],version:1};
 const summary=summarizeCheckLedger(first);assert.deepEqual(summary,summarizeCheckLedger(second));assert.equal(summary.activeCheckCount,1);assert(!JSON.stringify(summary).includes('private-ledger-actor'));assert.match(summary.digestEncoding,/not raw file bytes/);
});
