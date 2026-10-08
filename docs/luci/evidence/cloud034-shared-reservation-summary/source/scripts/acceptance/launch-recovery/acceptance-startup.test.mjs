// Controlled production orchestration: no subprocess, Docker, browser, or socket.
import test from 'node:test';
import assert from 'node:assert/strict';
import {mkdtempSync,mkdirSync,readFileSync,realpathSync,rmSync} from 'node:fs';
import vm from 'node:vm';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {createWorkflowDeadline,runBoundedCleanup,writeFailureCheckpoint} from './acceptance-lifecycle.mjs';
import {createStartupScope,startAcceptanceServices,startAcceptanceBrowser} from './acceptance-startup.mjs';

const deferred=()=>{let resolve,reject;const promise=new Promise((yes,no)=>{resolve=yes;reject=no;});return{promise,resolve,reject};};
const cancelled=()=>Object.assign(Error('controlled cancellation'),{code:'controlled-cancel'});
function fixture(t) {
 const control=createWorkflowDeadline({timeoutMs:2000});t.after(()=>control.close());
 const calls=[],failures=[];
 const scope=createStartupScope({assertActive:()=>control.assertActive(),onFailure:error=>failures.push(error.code),lateCloseTimeoutMs:15});
 const options={scope,assertActive:()=>control.assertActive(),startProvider:async()=>{calls.push('provider-start');return{kind:'provider'};},adoptProvider:()=>calls.push('provider-adopt'),closeProvider:async()=>calls.push('provider-close'),freePort:async()=>{calls.push('port');return 1234;},startProxy:async()=>{calls.push('proxy-start');return{kind:'proxy'};},adoptProxy:()=>calls.push('proxy-adopt'),closeProxy:async()=>{calls.push('proxy-close');return{drained:true,pendingRequests:0,activeHandlers:0};},configure:()=>calls.push('configure'),startService:async()=>calls.push('spawn')};
 return{control,scope,calls,failures,options};
}

test('actual startup cannot spawn after controller cleanup; late proxy cannot repair a timed-out drain phase',async t=>{
 const f=fixture(t),factory=deferred(),entered=deferred(),heldCleanup=deferred(),cleanupEntered=deferred();
 f.options.startProxy=()=>{f.calls.push('proxy-start');entered.resolve();return factory.promise;};
 const operation=startAcceptanceServices(f.options);const raced=f.control.run(()=>operation);
 await entered.promise;f.control.fail(cancelled());await assert.rejects(raced,{code:'controlled-cancel'});
 const dir=mkdtempSync(join(tmpdir(),'late-startup-'));t.after(()=>rmSync(dir,{recursive:true,force:true}));const path=join(dir,'failed.json');
 const cleanup=runBoundedCleanup({checkpoint:()=>writeFailureCheckpoint(path,{passed:false,startupResources:f.scope.snapshot(),dockerRequestsDrained:false}),phases:[
  {name:'controllers',timeoutMs:50,run:()=>{assert(!f.calls.includes('spawn'));f.calls.push('controllers-complete');return true;}},
  {name:'proxy',timeoutMs:50,run:()=>f.scope.close('proxy',5)},
  {name:'browser',timeoutMs:1000,run:()=>{cleanupEntered.resolve();return heldCleanup.promise;}},
 ]});
 await cleanupEntered.promise;
 assert.equal(JSON.parse(readFileSync(path)).startupResources.find(row=>row.name==='proxy').state,'starting');
 assert.equal(f.scope.snapshot().find(row=>row.name==='proxy').closeTimedOut,true);
 factory.resolve({kind:'proxy'});await assert.rejects(operation,{code:'controlled-cancel'});
 assert.equal(f.calls.filter(x=>x==='proxy-close').length,1);assert(!f.calls.includes('spawn'));assert(!f.calls.includes('configure'));
 heldCleanup.resolve();const results=await cleanup;
 assert.equal(results[0].status,'completed');assert.equal(results[1].status,'failed-or-unsettled');
 assert.equal(results.some(row=>row.name==='proxy'&&row.status==='completed'&&row.result?.drained),false);
 assert.equal(f.scope.snapshot().find(row=>row.name==='proxy').state,'closed');
 assert.equal(JSON.parse(readFileSync(path)).dockerRequestsDrained,false);
});

test('late provider is closed once and no port, proxy, or service starts',async t=>{
 const f=fixture(t),factory=deferred(),entered=deferred();f.options.startProvider=()=>{entered.resolve();return factory.promise;};
 const operation=startAcceptanceServices(f.options);await entered.promise;f.control.fail(cancelled());
 const cleanup=f.scope.close('provider',100);factory.resolve({kind:'provider'});
 await assert.rejects(operation,{code:'controlled-cancel'});await cleanup;
 assert.deepEqual(f.calls,['provider-adopt','provider-close']);assert.equal(f.scope.has('proxy'),false);
});

for(const heldPort of [1,2])test('cancellation while port '+heldPort+' is pending prevents later startup effects',async t=>{
 const f=fixture(t),factory=deferred(),entered=deferred();let ports=0;
 f.options.freePort=()=>{ports++;if(ports===heldPort){entered.resolve();return factory.promise;}return 1234;};
 const operation=startAcceptanceServices(f.options);await entered.promise;f.control.fail(cancelled());factory.resolve(4321);
 await assert.rejects(operation,{code:'controlled-cancel'});assert.equal(ports,heldPort);assert(!f.calls.includes('proxy-start'));assert(!f.calls.includes('spawn'));
 await f.scope.close('provider');
});

test('unreturned proxy factory remains explicitly pending after bounded close',async t=>{
 const f=fixture(t),entered=deferred();f.options.startProxy=()=>{entered.resolve();return new Promise(()=>{});};
 const operation=startAcceptanceServices(f.options);const raced=f.control.run(()=>operation);await entered.promise;f.control.fail(cancelled());await assert.rejects(raced);
 await assert.rejects(f.scope.close('proxy',10),{code:'startup-proxy-close-deadline-exceeded'});
 assert.deepEqual(f.scope.snapshot().find(row=>row.name==='proxy'),{name:'proxy',state:'starting',late:false,closeTimedOut:true});assert(!f.calls.includes('spawn'));
});

test('late never-resolving proxy close is bounded and retains uncertainty',async t=>{
 const f=fixture(t),factory=deferred(),entered=deferred();f.options.startProxy=()=>{entered.resolve();return factory.promise;};f.options.closeProxy=()=>{f.calls.push('proxy-close');return new Promise(()=>{});};
 const operation=startAcceptanceServices(f.options);await entered.promise;f.control.fail(cancelled());factory.resolve({});
 const began=Date.now();await assert.rejects(operation,{code:'controlled-cancel'});assert(Date.now()-began<500);
 assert.deepEqual(f.failures,['startup-proxy-close-deadline-exceeded']);assert(!f.calls.includes('spawn'));
 assert.deepEqual(f.scope.snapshot().find(row=>row.name==='proxy'),{name:'proxy',state:'closing',late:true,closeTimedOut:true});
});

for(const held of ['browser','browser-context','browser-page'])test('late '+held+' handle is closed and cannot advance browser startup',async t=>{
 const f=fixture(t),factory=deferred(),entered=deferred(),calls=[];
 const handle=name=>({close:async()=>calls.push(name+'-close')});
 const make=name=>{calls.push(name+'-start');if(held===name){entered.resolve();return factory.promise;}return handle(name);};
 const operation=startAcceptanceBrowser({scope:f.scope,assertActive:()=>f.control.assertActive(),launch:()=>make('browser'),adoptBrowser:()=>{},closeBrowser:value=>value.close(),newContext:()=>make('browser-context'),installRoutes:async()=>calls.push('routes'),newPage:()=>make('browser-page'),adoptPage:()=>{}});
 await entered.promise;f.control.fail(cancelled());factory.resolve(handle(held));await assert.rejects(operation,{code:'controlled-cancel'});
 assert.equal(calls.filter(x=>x===held+'-close').length,1);
 if(held==='browser')assert(!calls.includes('browser-context-start'));
 if(held==='browser-context')assert(!calls.includes('routes'));
 assert.equal(f.scope.snapshot().find(row=>row.name===held).state,'closed');
});

test('normal production startup order and real close result are preserved',async t=>{
 const f=fixture(t);await startAcceptanceServices(f.options);
 assert.deepEqual(f.calls,['provider-start','provider-adopt','port','port','proxy-start','proxy-adopt','configure','spawn']);
 assert.deepEqual(await f.scope.close('proxy'),{drained:true,pendingRequests:0,activeHandlers:0});await f.scope.close('proxy');assert.equal(f.calls.filter(x=>x==='proxy-close').length,1);
 await f.scope.close('provider');
});

test('factory failure retains a started-but-failed resource and never permits service startup',async t=>{
 const f=fixture(t);f.options.startProxy=async()=>{throw Object.assign(Error('factory failed'),{code:'factory-failed'});};
 await assert.rejects(startAcceptanceServices(f.options),{code:'factory-failed'});
 assert.equal(f.scope.has('proxy'),true);assert.equal(f.scope.snapshot().find(row=>row.name==='proxy').state,'factory-failed');
 await assert.rejects(f.scope.close('proxy'),{code:'factory-failed'});assert(!f.calls.includes('spawn'));
});

test('a failed handle adoption still retains and closes the returned resource',async t=>{
 const f=fixture(t);f.options.adoptProxy=()=>{throw Object.assign(Error('adoption failed'),{code:'adoption-failed'});};
 await assert.rejects(startAcceptanceServices(f.options),{code:'adoption-failed'});
 assert.equal(f.calls.filter(row=>row==='proxy-close').length,1);assert(!f.calls.includes('spawn'));
 assert.equal(f.scope.snapshot().find(row=>row.name==='proxy').state,'closed');
});

test('confirmed browser close settles acquired descendants but never a pending child factory',async t=>{
 for(const pending of [false,true]) {
  const f=fixture(t),factory=deferred(),entered=deferred();let closes=0;
  const child={close:async()=>{closes++;}};
  const operation=startAcceptanceBrowser({scope:f.scope,assertActive:()=>f.control.assertActive(),launch:async()=>({close:async()=>{closes++;}}),closeBrowser:handle=>handle.close(),newContext:()=>{if(pending){entered.resolve();return factory.promise;}return child;},installRoutes:async()=>{},newPage:async()=>child});
  if(pending){await entered.promise;f.control.fail(cancelled());}else await operation;
  await f.scope.close('browser');
  const context=f.scope.snapshot().find(row=>row.name==='browser-context');
  assert.equal(context.state,pending?'starting':'closed');assert.equal(context.closedByParent,!pending);
  if(pending){factory.resolve(child);await assert.rejects(operation);assert.equal(closes,2);}else{assert.equal(closes,1);assert(f.scope.snapshot().every(row=>row.state==='closed'));}
 }
});

test('DAG invokes tested orchestration and guards the actual direct spawn boundary',async()=>{
 const source=readFileSync(new URL('../task-dependencies/dag.mjs',import.meta.url),'utf8');
 assert(source.includes('await startAcceptanceServices({scope:startupScope'));assert(source.includes('await startAcceptanceBrowser({scope:startupScope'));
 const body=source.slice(source.indexOf('async function startService()'),source.indexOf('async function stopService('));let spawns=0;
 const start=Function('health','spawn',body+';return startService;')(()=>{throw cancelled();},()=>spawns++);
 await assert.rejects(start(),{code:'controlled-cancel'});assert.equal(spawns,0);
});

test('actual DAG location assertion refuses an external driver pointed at a different clean checkout',t=>{
 const source=readFileSync(new URL('../task-dependencies/dag.mjs',import.meta.url),'utf8');
 const statement=source.split('\n').find(line=>line.startsWith('assert.equal(realpathSync(here),realpathSync(join(repo,'));assert(statement);
 assert(source.indexOf(statement)<source.indexOf('const {chromium}=await import'));
 const temp=mkdtempSync(join(tmpdir(),'startup-location-'));t.after(()=>rmSync(temp,{recursive:true,force:true}));
 const repo=join(temp,'repo'),here=join(repo,'scripts/acceptance/task-dependencies'),external=join(temp,'external');mkdirSync(here,{recursive:true});mkdirSync(external);
 vm.runInNewContext(statement,{assert,realpathSync,join,repo,here});
 assert.throws(()=>vm.runInNewContext(statement,{assert,realpathSync,join,repo,here:external}),/Executed DAG helper must be inside/);
});
