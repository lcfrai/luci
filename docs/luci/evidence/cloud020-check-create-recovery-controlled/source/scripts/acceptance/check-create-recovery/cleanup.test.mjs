// Five focused controlled failure cases; no runtime, Docker, browser or process.
import test from 'node:test';
import assert from 'node:assert/strict';
import {createWorkflowDeadline,runBoundedCleanup} from '../launch-recovery/acceptance-lifecycle.mjs';
import {createStartupScope,startAcceptanceBrowser} from '../launch-recovery/acceptance-startup.mjs';
const deferred=()=>{let resolve,reject;const promise=new Promise((yes,no)=>{resolve=yes;reject=no;});return{promise,resolve,reject};};
test('outer cleanup timeout cannot be repaired by a later successful scoped drain',async()=>{
 const drain=deferred(),attempted=[];
 const scope=createStartupScope({assertActive(){}});
 await scope.acquire('proxy',{start:async()=>({}),close:()=>drain.promise});
 const results=await runBoundedCleanup({checkpoint(){},phases:[
  {name:'proxy',timeoutMs:15,run:()=>scope.close('proxy',500)},
  {name:'independent',timeoutMs:50,run:()=>{attempted.push('independent');return true;}},
 ]});
 assert.deepEqual(results[0],{name:'proxy',status:'failed-or-unsettled',code:'proxy-deadline-exceeded'});
 const original=structuredClone(results);
 drain.resolve({drained:true,pendingRequests:0});
 assert.deepEqual(await scope.close('proxy',100),{drained:true,pendingRequests:0});
 await new Promise(resolve=>setImmediate(resolve));
 assert.deepEqual(results,original);assert.deepEqual(attempted,['independent']);
 assert.equal(scope.snapshot()[0].state,'closed');assert.equal(scope.snapshot()[0].closeTimedOut,false);
 assert.equal(results.every(row=>row.status==='completed'),false,'A later handle state must not upgrade the original cleanup outcome');
});

test('concurrent and repeated failed close calls preserve one immutable rejection',async()=>{
 const gate=deferred(),cause=Object.assign(Error('controlled close failure'),{code:'controlled-close-failed'});let closes=0;
 const scope=createStartupScope({assertActive(){}});
 await scope.acquire('proxy',{start:async()=>({}),close:()=>{closes++;return gate.promise;}});
 const first=assert.rejects(scope.close('proxy',200),error=>error===cause);
 const second=assert.rejects(scope.close('proxy',200),error=>error===cause);
 gate.reject(cause);await Promise.all([first,second]);
 await assert.rejects(scope.close('proxy',200),error=>error===cause);
 assert.equal(closes,1);assert.equal(scope.snapshot()[0].state,'close-failed');
});

test('late close rejection preserves timeout evidence and completed independent cleanup',async()=>{
 const gate=deferred(),cause=Object.assign(Error('late failure'),{code:'late-close-failure'}),attempted=[];
 const scope=createStartupScope({assertActive(){}});
 await scope.acquire('proxy',{start:async()=>({}),close:()=>gate.promise});
 const results=await runBoundedCleanup({checkpoint(){},phases:[
  {name:'proxy',timeoutMs:100,run:()=>scope.close('proxy',10)},
  {name:'independent',timeoutMs:50,run:()=>{attempted.push('independent');return true;}},
 ]});
 const original=structuredClone(results);gate.reject(cause);
 await assert.rejects(scope.close('proxy',100),error=>error===cause);
 await new Promise(resolve=>setImmediate(resolve));
 assert.equal(results[0].code,'startup-proxy-close-deadline-exceeded');assert.deepEqual(results,original);
 assert.deepEqual(attempted,['independent']);assert.equal(results[1].status,'completed');
 assert.deepEqual(scope.snapshot()[0],{name:'proxy',state:'close-failed',late:false,closeTimedOut:true});
});

test('failed parent close cannot certify acquired child resources',async()=>{
 const cause=Object.assign(Error('controlled parent close failure'),{code:'parent-close-failed'}),scope=createStartupScope({assertActive(){}});
 await startAcceptanceBrowser({scope,assertActive(){},launch:async()=>({}),closeBrowser:async()=>{throw cause;},
  newContext:async()=>({close:async()=>true}),installRoutes:async()=>{},newPage:async()=>({close:async()=>true})});
 await assert.rejects(scope.close('browser',100),error=>error===cause);
 const rows=scope.snapshot();assert.equal(rows.find(row=>row.name==='browser').state,'close-failed');
 for(const name of ['browser-context','browser-page']){
  const row=rows.find(value=>value.name===name);assert.equal(row.state,'acquired');assert.equal(row.closedByParent,false);
 }
 await scope.close('browser-context',100);
 assert.equal(scope.snapshot().find(row=>row.name==='browser-page').closedByParent,true);
});

test('the first workflow failure remains authoritative at late mutation boundaries',async()=>{
 const first=Object.assign(Error('first failure'),{code:'first-failure'}),later=Object.assign(Error('later failure'),{code:'later-failure'});
 const control=createWorkflowDeadline({timeoutMs:1000}),gate=deferred();let mutations=0;
 try{
  const operation=control.run(async()=>{await gate.promise;control.assertActive();mutations++;});
  control.fail(first);control.fail(later);
  await assert.rejects(operation,error=>error===first);
  gate.resolve();await new Promise(resolve=>setImmediate(resolve));
  assert.throws(()=>control.assertActive(),error=>error===first);assert.equal(mutations,0);assert.equal(control.failed,true);
 }finally{control.close();}
});

