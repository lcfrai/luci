// Exercise the driver's actual phase wiring with controlled promises and files.
// No child process, browser, Docker client, provider server or compiler starts.
import test from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {mkdtempSync,readFileSync,rmSync,statSync} from 'node:fs';
import {dirname,join,resolve} from 'node:path';
import {tmpdir} from 'node:os';
import {fileURLToPath} from 'node:url';
const here=dirname(fileURLToPath(import.meta.url));
const repo=resolve(process.env.LUCI_ACCEPTANCE_REPO??resolve(here,'../../..'));
const {runBoundedCleanup,settleOwnedBrowser,writeFailureCheckpoint,createWorkflowDeadline,boundedAttempt}=await import(join(repo,'scripts/acceptance/launch-recovery/acceptance-lifecycle.mjs'));
const {createStartupScope,startAcceptanceServices,startAcceptanceBrowser}=await import(process.env.LUCI_ACCEPTANCE_STARTUP_TEST_MODULE??join(repo,'scripts/acceptance/launch-recovery/acceptance-startup.mjs'));
const source=readFileSync(new URL('./sustained.mjs',import.meta.url),'utf8');
const wiring=source.split('// BEGIN SUSTAINED CLEANUP WIRING: exercised with controlled resources only.\n')[1]?.split('// END SUSTAINED CLEANUP WIRING')[0];
assert(wiring,'Missing actual driver cleanup wiring');
const never=()=>new Promise(()=>{});
function context(t,overrides={}){
 const directory=mkdtempSync(join(tmpdir(),'sustained-lifecycle-'));t.after(()=>rmSync(directory,{recursive:true,force:true}));
 const checkpoint=join(directory,'incomplete.json'),failures=[],events=[],killed=[],receipt={};
 const owned={pid:41,start:'42'},reused={pid:51,start:'52'};
 const ctx={assert,receipt,proxy:{observations:{dropped:0,failures:[]},close:async()=>{events.push('proxy');return {drained:true,pendingRequests:0,activeHandlers:0};}},browser:undefined,
  controllerIdentities:[{pid:1,start:'2'}],stillOwned:identity=>identity===owned,
  stopService:async()=>{events.push('controllers');return true;},ownedProcesses:()=>[],
  provider:{listening:true,closeAllConnections(){events.push('provider-connections');},close(done){events.push('provider');done();}},
  process:{kill:pid=>killed.push(pid)},cleanupBudget:maximum=>Math.min(maximum,5),settleOwnedBrowser,
  cleanupFailure:error=>{failures.push(error.code??error.message);receipt.passed=false;},
  persistFailureCheckpoint:()=>writeFailureCheckpoint(checkpoint,{passed:false,status:'cleanup-incomplete',stage:'sustained-work',driver:{pid:7,startTicks:8},controllers:[{pid:1,start:2}],dockerRequestsDrained:false}),
  journal:{record(_event,row){events.push(row.stage);}},
  runBoundedCleanup:options=>runBoundedCleanup({...options,phases:options.phases.map(phase=>({...phase,timeoutMs:200,run:()=>{assert.equal(JSON.parse(readFileSync(checkpoint)).passed,false);return phase.run();}}))}),
  ...overrides};
 if(!ctx.startupScope)ctx.startupScope={has:name=>!!ctx[name],close:(name,timeoutMs)=>boundedAttempt(async()=>{const handle=ctx[name];if(name==='provider'){handle.closeAllConnections();if(handle.listening)await new Promise(resolve=>handle.close(resolve));return;}return handle.close();},timeoutMs,'startup-'+name+'-close-deadline-exceeded')};
 vm.runInNewContext(wiring+'\nglobalThis.run=settleSustainedResources;',ctx);
 return {ctx,checkpoint,receipt,failures,events,killed,owned,reused};
}

test('stuck browser close cannot skip controller/proxy/provider cleanup or exact owned fallback',async t=>{
 const fixture=context(t,{browser:{close:never}});
 fixture.ctx.ownedProcesses=()=>[fixture.owned,fixture.reused];
 const phases=await fixture.ctx.run();
 assert.deepEqual(phases.map(row=>row.name),['controllers','proxy','browser','provider']);
 assert(fixture.events.includes('controllers')&&fixture.events.includes('proxy')&&fixture.events.includes('provider'));
 assert(fixture.failures.includes('startup-browser-close-deadline-exceeded'));
 assert.deepEqual(fixture.killed,[fixture.owned.pid]);
 assert.equal(fixture.receipt.controllersStopped,true);assert.equal(fixture.receipt.dockerRequestsDrained,true);
 assert.equal(statSync(fixture.checkpoint).mode&0o777,0o600);
});

test('never-resolving or throwing proxy retains uncertainty and cannot gain authority from a late result',async t=>{
 for(const throwing of [false,true]){
  let release;
  const fixture=context(t,{proxy:{observations:{dropped:0,failures:[]},close:()=>throwing?Promise.reject(Error('controlled close failure')):new Promise(resolve=>{release=resolve;})}});
  const phases=await fixture.ctx.run();
  assert.equal(phases.find(row=>row.name==='proxy').status,'failed-or-unsettled');
  assert.equal(fixture.receipt.dockerRequestsDrained,false);assert.equal(fixture.receipt.dockerMutationStateCertain,false);
  assert(fixture.events.includes('provider'));
  release?.({drained:true,pendingRequests:0,activeHandlers:0});await new Promise(resolve=>setImmediate(resolve));
  assert.equal(fixture.receipt.dockerRequestsDrained,false);assert.equal(fixture.receipt.dockerMutationStateCertain,false);
 }
});

test('transport failures refuse destructive authority even after a completed socket drain',async t=>{
 const fixture=context(t,{proxy:{observations:{dropped:0,failures:[{uncertainMutation:true}]},close:async()=>({drained:true,pendingRequests:0,activeHandlers:0})}});
 await fixture.ctx.run();
 assert.equal(fixture.receipt.controllersStopped,true);assert.equal(fixture.receipt.dockerRequestsDrained,false);assert.equal(fixture.receipt.dockerMutationStateCertain,false);
 assert(fixture.events.includes('provider'));assert(fixture.failures.length>0);
});

test('throwing controller/browser/provider cleanup still attempts every independent phase',async t=>{
 const fixture=context(t,{stopService:async()=>{throw Error('controlled controller failure');},browser:{close:()=>{throw Error('controlled browser failure');}},provider:{listening:true,closeAllConnections(){throw Error('controlled provider failure');}}});
 const phases=await fixture.ctx.run();
 assert.deepEqual(phases.map(row=>row.name),['controllers','proxy','browser','provider']);
 assert.equal(fixture.receipt.controllersStopped,false);assert.equal(fixture.receipt.dockerRequestsDrained,true);
 assert.equal(phases.find(row=>row.name==='provider').status,'failed-or-unsettled');assert(fixture.failures.length>=3);
});

test('missing browser handle still applies exact profile/process identity before fallback',async t=>{
 const fixture=context(t);fixture.ctx.ownedProcesses=()=>[fixture.owned,fixture.reused];
 await fixture.ctx.run();assert.deepEqual(fixture.killed,[fixture.owned.pid]);
});

test('late workflow continuation cannot seed state after the deadline',async()=>{
 const active=source.slice(source.indexOf('function assertWorkflowActive()'),source.indexOf('async function timed('));
 const seed=source.slice(source.indexOf('function seedFiles('),source.indexOf('async function saveContext('));
 let release,writes=0;
 const control=createWorkflowDeadline({timeoutMs:10});
 const ctx={assert,closing:false,workflowControl:control,fixtureError:undefined,remaining:()=>100,service:undefined,join,dirname,mkdirSync:()=>{},existsSync:()=>false,lstatSync:()=>({isFile:()=>true}),writeFileSync:()=>writes++};
 vm.runInNewContext(active+seed+'\nglobalThis.seed=seedFiles;',ctx);
 const pending=control.run(async()=>{await new Promise(resolve=>{release=resolve;});ctx.seed({path:'/controlled'}, {'engine.go':'unreachable'});});
 await assert.rejects(pending,{code:'workflow-deadline-exceeded'});release();await new Promise(resolve=>setImmediate(resolve));assert.equal(writes,0);
});

test('intentional service restart is permitted while failure/cleanup still refuses a new child',async()=>{
 const active=source.slice(source.indexOf('function assertWorkflowActive()'),source.indexOf('async function timed('));
 const start=source.slice(source.indexOf('async function startService()'),source.indexOf('async function stopService('));
 let spawns=0;
 const ctx={assert,closing:false,workflowControl:{assertActive(){}},fixtureError:undefined,remaining:()=>1000,remainingBudget:()=>1000,service:{exitCode:0},binary:'fixture',fixture:'fixture',env:{},spawn:()=>{spawns++;return {pid:9,exitCode:null,on(){}};},stopOnFailure(){},controllerIdentities:[],processIdentity:pid=>({pid,start:'1'}),journalProcessIdentity:pid=>({pid,startTicks:1}),journal:{record(){}},fetch:async()=>({ok:true,json:async()=>({product:'Luci',version:'0.5.0'})}),base:'fixture',AbortSignal,delay:async()=>{},receipt:{}};
 vm.runInNewContext(active+start+'\nglobalThis.start=startService;',ctx);
 await ctx.start();assert.equal(spawns,1);assert.equal(ctx.receipt.lifecycle.length,1);
 ctx.spawn=()=>({pid:undefined,on(){}});await assert.rejects(ctx.start(),/process identity is unavailable/);assert.equal(ctx.controllerIdentities.length,1);
 ctx.closing=true;await assert.rejects(ctx.start(),/Workflow closed/);assert.equal(spawns,1);
});

test('native provider progress survives intentional controller downtime but remains fenced on failure',()=>{
 const active=source.slice(source.indexOf('function assertWorkflowActive()'),source.indexOf('async function timed('));
 const tool=source.slice(source.indexOf('function nextTool('),source.indexOf('function startProvider('));
 const ctx={assert,closing:false,fixtureError:undefined,workflowControl:{assertActive(){}},remaining:()=>1000,service:{exitCode:0},calls:new Map()};
 vm.runInNewContext(active+tool+'\nglobalThis.next=nextTool;',ctx);
 const state={task:'build-a',operation:{id:'operation'},count:0};
 assert.equal(ctx.next(state,'write_stdin',{chars:''}).name,'write_stdin');
 ctx.fixtureError=Error('controlled deadline');assert.throws(()=>ctx.next(state,'write_stdin',{chars:''}),/controlled deadline/);assert.equal(state.count,1);
});

test('a late socket ticket cannot create a connection after cleanup started',async()=>{
 const active=source.slice(source.indexOf('function assertWorkflowActive()'),source.indexOf('async function timed('));
 const attach=source.slice(source.indexOf('async function attach('),source.indexOf('function outputs('));
 let release,opened=0;
 const ctx={assert,closing:false,fixtureError:undefined,workflowControl:{assertActive(){}},remaining:()=>1000,service:undefined,workspace:'',api:()=>new Promise(resolve=>{release=resolve;}),sensitive:[],WebSocket:class{constructor(){opened++;}},base:'http://fixture'};
 vm.runInNewContext(active+attach+'\nglobalThis.attachFixture=attach;',ctx);
 const pending=ctx.attachFixture('session');ctx.closing=true;release({ticket:'private-fixture-ticket'});
 await assert.rejects(pending,/Workflow closed/);assert.equal(opened,0);
});

test('a late API response cannot resume the workflow after cleanup started',async()=>{
 const active=source.slice(source.indexOf('function assertWorkflowActive()'),source.indexOf('async function timed('));
 const api=source.slice(source.indexOf('async function api('),source.indexOf('async function until('));
 let release;
 const ctx={assert,closing:false,fixtureError:undefined,workflowControl:{assertActive(){}},remaining:()=>1000,remainingBudget:()=>1000,service:undefined,base:'http://fixture',token:undefined,AbortSignal,stage:'controlled-response',receipt:{},scrub:String,fetch:async()=>({status:200}),boundedResponse:()=>new Promise(resolve=>{release=resolve;})};
 vm.runInNewContext(active+api+'\nglobalThis.apiFixture=api;',ctx);
 const pending=ctx.apiFixture('POST','/controlled',{});await new Promise(resolve=>setImmediate(resolve));ctx.closing=true;release('{"accepted":true}');
 await assert.rejects(pending,/Workflow closed/);
});

test('a failed click observes its response promise even when the response rejects later',async()=>{
 const action=source.slice(source.indexOf('async function action('),source.indexOf('async function openProject('));
 let rejectResponse;
 const ctx={assert,health(){},page:{waitForResponse:()=>new Promise((_resolve,reject)=>{rejectResponse=reject;})},remainingBudget:()=>100};
 vm.runInNewContext(action+'\nglobalThis.actionFixture=action;',ctx);
 await assert.rejects(ctx.actionFixture('/controlled',async()=>{throw Error('controlled click failure');}),/controlled click failure/);
 rejectResponse(Error('controlled late browser close'));await new Promise(resolve=>setImmediate(resolve));
});

test('actual sustained startup cannot configure or spawn after a pending proxy resolves following cleanup',async t=>{
 const startup=source.split('// BEGIN SUSTAINED STARTUP WIRING: exercised with controlled factories only.\n')[1]?.split('// END SUSTAINED STARTUP WIRING')[0];assert(startup);
 const active=source.slice(source.indexOf('function assertWorkflowActive()'),source.indexOf('async function timed('));
 let releaseProxy,enteredProxy,writes=0,spawns=0,proxyCloses=0;
 const entered=new Promise(resolve=>{enteredProxy=resolve;});
 const fixture=context(t,{proxy:undefined,provider:undefined,closing:false,fixtureError:undefined,service:undefined,remaining:()=>1000,
  process:{env:{}},join,dirname,fixture:'/controlled',web:'/controlled/web',binary:'/controlled/binary',image:'controlled-image',lifecycle:'private-controlled-token',
  receipt:{bounds:{maxCreates:5,maxStarts:5}},journal:{record(){}},identifyCreate(){},admission(){},bridge:'127.0.0.1',freePort:async()=>1234,
  startAcceptanceServices,writeFileSync:()=>writes++,startService:async()=>{spawns++;},
  startFencedDockerProxy:options=>{assert.equal(typeof options.beforeStart,'function');enteredProxy();return new Promise(resolve=>{releaseProxy=resolve;});},
 });
 const {ctx}=fixture;
 ctx.startProvider=()=>{ctx.provider={once(){return this;},listen(_port,_host,done){this.listening=true;done();return this;},address:()=>({port:1234}),closeAllConnections(){},close(done){this.listening=false;done();}};};
 const control=createWorkflowDeadline({timeoutMs:1000});ctx.workflowControl=control;
 vm.runInNewContext(active+'\nglobalThis.active=assertWorkflowActive;',ctx);
 ctx.startupScope=createStartupScope({assertActive:ctx.active,onFailure:ctx.cleanupFailure,lateCloseTimeoutMs:10});
 vm.runInNewContext(startup+'\nglobalThis.start=startSustainedServices;',ctx);
 const pending=control.run(()=>ctx.start());await entered;
 control.fail(Error('controlled cancellation'));await assert.rejects(pending,/controlled cancellation/);ctx.closing=true;
 await ctx.run();assert.equal(ctx.receipt.controllersStopped,true);assert.equal(ctx.receipt.dockerRequestsDrained,false);assert.equal(ctx.receipt.dockerMutationStateCertain,false);
 releaseProxy({observations:{dropped:0,failures:[]},close:async()=>{proxyCloses++;return {drained:true,pendingRequests:0,activeHandlers:0};}});
 await new Promise(resolve=>setImmediate(resolve));await new Promise(resolve=>setImmediate(resolve));
 assert.equal(writes,0);assert.equal(spawns,0);assert.equal(proxyCloses,1);
 assert.equal(ctx.receipt.dockerRequestsDrained,false);assert.equal(ctx.receipt.dockerMutationStateCertain,false);
 const state=ctx.startupScope.snapshot().find(row=>row.name==='proxy');assert.equal(state.state,'closed');assert.equal(state.late,true);assert.equal(state.closeTimedOut,true);
});

test('actual sustained browser wiring cannot install routes or open a page after canceled timing setup',async t=>{
 const browserWiring=source.split('// BEGIN SUSTAINED BROWSER WIRING: exercised with controlled factories only.\n')[1]?.split('// END SUSTAINED BROWSER WIRING')[0];assert(browserWiring);
 const active=source.slice(source.indexOf('function assertWorkflowActive()'),source.indexOf('async function timed('));
 let releaseTiming,enteredTiming,routes=0,pages=0,closes=0;
 const entered=new Promise(resolve=>{enteredTiming=resolve;});
 const browserContext={route:async()=>{routes++;},newPage:async()=>{pages++;},close:async()=>{}};
 const browserHandle={newContext:async()=>browserContext,close:async()=>{closes++;},version:()=> 'controlled'};
 const fixture=context(t,{proxy:undefined,provider:undefined,browser:undefined,closing:false,fixtureError:undefined,service:undefined,remaining:()=>1000,
  startAcceptanceBrowser,chromium:{launch:async()=>browserHandle},option:(_name,fallback)=>fallback,browserEnv:{},
  installBrowserTiming:()=>{enteredTiming();return new Promise(resolve=>{releaseTiming=resolve;});},scrub:String,stopOnFailure(){},});
 const {ctx}=fixture,control=createWorkflowDeadline({timeoutMs:1000});ctx.workflowControl=control;
 vm.runInNewContext(active+'\nglobalThis.active=assertWorkflowActive;',ctx);
 ctx.startupScope=createStartupScope({assertActive:ctx.active,onFailure:ctx.cleanupFailure,lateCloseTimeoutMs:10});
 vm.runInNewContext('globalThis.startBrowserFixture=async()=>{'+browserWiring+'};',ctx);
 const pending=control.run(()=>ctx.startBrowserFixture());await entered;control.fail(Error('controlled cancellation'));await assert.rejects(pending,/controlled cancellation/);ctx.closing=true;
 await ctx.run();releaseTiming();await new Promise(resolve=>setImmediate(resolve));
 assert.equal(closes,1);assert.equal(routes,0);assert.equal(pages,0);
 assert(ctx.startupScope.snapshot().every(row=>row.state==='closed'));
});
