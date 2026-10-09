import assert from 'node:assert/strict';
import test from 'node:test';
import {readFileSync,mkdtempSync,writeFileSync,statSync,rmSync,openSync,closeSync,fsyncSync,writeSync,constants} from 'node:fs';
import {tmpdir} from 'node:os';
import {join,dirname,resolve} from 'node:path';
import {pathToFileURL} from 'node:url';
import {createHash} from 'node:crypto';
import {runInNewContext} from 'node:vm';
import {boundedAttempt as actualBoundedAttempt} from './support/acceptance-lifecycle.mjs';
const root=resolve(process.env.C073_SOURCE_ROOT??new URL('.',import.meta.url).pathname),variant=process.env.C073_VARIANT??'warm';
const source=readFileSync(join(root,variant,'launch.mjs'),'utf8');
const module=await import(pathToFileURL(join(root,variant,'launch.mjs')));
const isFixed=typeof module.createFinalFailureObservation==='function';
const tailStart=source.indexOf(' }catch(e){failure');
const tailEnd=source.indexOf('\n return receipt;',tailStart);
const failStart=source.indexOf(isFixed?" let failurePhase='workflow';":' const fail=e=>{failure??=e;');
const failEnd=source.indexOf('\n const health=',failStart);
const healthStart=failEnd+1,healthEnd=source.indexOf('\n const stageTo=',healthStart);
const routeStart=source.indexOf("installRoutes:context=>context.route('**/*',")+"installRoutes:context=>context.route('**/*',".length;
const routeEnd=source.indexOf('),newPage:',routeStart);
assert(tailStart>0&&tailEnd>tailStart&&failStart>0&&failEnd>failStart&&routeEnd>routeStart);
// VM Script binds module metadata explicitly; cleanup phase bodies remain inert.
const tail=source.slice(tailStart,tailEnd).replaceAll('import.meta.url','capturedImportMetaURL'),failCode=source.slice(failStart,failEnd),healthCode=source.slice(healthStart,healthEnd),routeCode=source.slice(routeStart,routeEnd);
async function flow(hooks={}){
 const receipt={scenario:{passed:true},cleanup:[],phases:[],browserMutations:[]},writes=[],calls={cleanup:0,close:0,abort:0,continued:0},pendingMutations=new Map(hooks.pending?[['one',{path:'/v1/sessions/owned/restart'}]]:[]);
 const durable=(path,value)=>{if(path.endsWith('private-first-failure.json')&&hooks.retainError)throw Error('controlled private writer failure');writes.push({path,value:structuredClone(value)});};
 const sandbox={assert,Buffer,URL,performance,structuredClone,createFinalFailureObservation:module.createFinalFailureObservation,closingReadRoute:module.closingReadRoute,actualBoundedAttempt,receipt,hooks,writes,calls,pendingMutations,durable,join,Date,console:{log(){}},process:{off(){},exitCode:0},clearInterval(){},safeError:()=>({name:'Error',code:'controlled',message:'controlled main-catch fixture'}),checked:x=>x,record:x=>receipt.phases.push(x),evidence:'/controlled',report:'/controlled/result.json',product:'controlled-immutable-source',page:null,monitor:null,controllerLog:null,privateTransportErrors:null,browserResponseObservations:new Set(),acceptRestartResponses:true,containers:new Map(),signals:[],timer:null,serviceIdentity:null,service:null,session:{id:'owned'},engine:null,instance:null,project:null,worktree:null,sourceBindings:[],browserPending:new Map(),WORKFLOW_MS:123,base:'http://127.0.0.1:1234'};
 const prefix=`let stage='product-settlement',failure,closing=false,cleanupDeadline=Infinity; const workflow={fail(){},close(){calls.close++},assertActive(){}};${failCode}${healthCode}
 const routeCallback=${routeCode};
 const invokeRoute=()=>routeCallback({request:()=>({url:()=>hooks.url??base+'/v1/sessions',method:()=>hooks.method??'GET'}),abort:async()=>{calls.abort++;if(hooks.abort)return hooks.abort();},continue:async()=>{calls.continued++}});
 const context=()=>({fail,health,invokeRoute,receipt,closing,stage});
 const boundedAttempt=async(fn,ms,code)=>{if(code==='closing-read-drain-timeout'&&hooks.finalDrain)await hooks.finalDrain(context());if(ms===2000)return actualBoundedAttempt(fn,ms,code);if(ms===5500&&hooks.drain)await hooks.drain(context());return fn();};
 const runBoundedCleanup=async config=>{calls.cleanup++;if(hooks.cleanup)await hooks.cleanup({...context(),onFailure:config.onFailure});return config.phases.map(p=>({name:p.name,status:'completed'}));};
 try{if(hooks.main)await hooks.main(context());`; 
 await runInNewContext('(async()=>{'+prefix+tail+'})()',sandbox);
 return {receipt,writes,calls,pendingMutations,exitCode:sandbox.process.exitCode};
}
const privateWrite=x=>x.writes.find(w=>w.path.endsWith('private-first-failure.json'));
test('exact local session poll aborts during actual finally without false failure',async()=>{
 const x=await flow({cleanup:c=>c.invokeRoute()});
 assert.equal(x.receipt.passed,true);assert.equal(x.exitCode,0);assert.equal(x.calls.abort,1);assert.equal(x.calls.continued,0);assert.equal(x.calls.cleanup,1);assert.equal(x.receipt.failureObservation.first,null);assert.equal(privateWrite(x),undefined);
 const rows=x.receipt.closingReadObservations;assert.equal(rows.length,1);assert.equal(rows[0].method,'GET');assert.equal(rows[0].routeClass,'session-list');assert.equal(rows[0].abort,'acknowledged');assert(Date.parse(rows[0].startedAt)<=Date.parse(rows[0].settledAt));
});
test('owned session read has separate bounded route class',async()=>{const x=await flow({url:'http://127.0.0.1:1234/v1/sessions/owned',cleanup:c=>c.invokeRoute()});assert.equal(x.receipt.passed,true);assert.equal(x.receipt.closingReadObservations[0].routeClass,'owned-session');});
test('active exact poll is forwarded through unchanged health guard',async()=>{const x=await flow({main:c=>c.invokeRoute()});assert.equal(x.receipt.passed,true);assert.equal(x.calls.continued,1);assert.equal(x.calls.abort,0);assert.equal(x.receipt.closingReadObservations.length,0);});
for(const [label,config] of [
 ['foreign-origin',{url:'http://127.0.0.1:4321/v1/sessions'}],['unknown-route',{url:'http://127.0.0.1:1234/v1/projects'}],['different-session',{url:'http://127.0.0.1:1234/v1/sessions/foreign'}],['query',{url:'http://127.0.0.1:1234/v1/sessions?worktreeId=owned'}],['fragment',{url:'http://127.0.0.1:1234/v1/sessions#fragment'}],['userinfo',{url:'http://controlled:controlled@127.0.0.1:1234/v1/sessions'}],['nested-action',{url:'http://127.0.0.1:1234/v1/sessions/owned/restart'}],['create-mutation',{method:'POST'}],['restart-mutation',{method:'POST',url:'http://127.0.0.1:1234/v1/sessions/owned/restart'}],['delete-mutation',{method:'DELETE',url:'http://127.0.0.1:1234/v1/sessions/owned'}],['head-method',{method:'HEAD'}],['options-method',{method:'OPTIONS'}]
])test('closing '+label+' remains a strict failed workflow with no forwarding or settlement',async()=>{const x=await flow({...config,pending:true,cleanup:c=>c.invokeRoute()});assert.equal(x.receipt.passed,false);assert.equal(x.exitCode,1);assert.equal(x.calls.continued,0);assert.equal(x.receipt.failureObservation.first.sourceLabel,'workflow-closing');assert.equal(x.receipt.closingReadObservations.length,0);assert.equal(x.pendingMutations.size,1);assert.equal(x.receipt.pendingMutations.length,1);});
test('allowed closing poll never clears an earlier failure or pending mutation',async()=>{const x=await flow({pending:true,main:c=>c.fail(Error('controlled earlier failure')),cleanup:c=>c.invokeRoute()});assert.equal(x.receipt.passed,false);assert.equal(x.receipt.failureObservation.first.phase,'workflow');assert.equal(x.receipt.closingReadObservations[0].abort,'acknowledged');assert.equal(x.pendingMutations.size,1);});
test('abort rejection remains explicit and conservatively fails',async()=>{const x=await flow({abort:()=>{throw Error('controlled abort failure')},cleanup:c=>c.invokeRoute()});assert.equal(x.receipt.passed,false);assert.equal(x.receipt.closingReadObservations[0].abort,'unconfirmed');assert.equal(x.receipt.failureObservation.first.phase,'cleanup');assert.equal(x.calls.continued,0);});
test('actual2000ms abort timeout retains unconfirmed outcome and records failure while its callback remains unsettled',async()=>{const x=await flow({abort:()=>new Promise(()=>{}),cleanup:async c=>{const promise=c.invokeRoute();await new Promise(resolve=>setTimeout(resolve,2100));void promise;}});assert.equal(x.receipt.passed,false);assert.equal(x.receipt.closingReadObservations[0].abort,'unconfirmed');assert.equal(x.receipt.failureObservation.first.phase,'cleanup');assert.equal(x.calls.cleanup,1);assert.equal(x.calls.continued,0);});
test('bounded closing poll overflow retains strict failure',async()=>{const x=await flow({cleanup:async c=>{for(let i=0;i<17;i++)await c.invokeRoute();}});assert.equal(x.receipt.passed,false);assert.equal(x.receipt.closingReadObservations.length,16);assert.equal(x.calls.continued,0);assert.equal(x.receipt.failureObservation.first.phase,'cleanup');});
test('independent disposal error is not erased by accepted read teardown',async()=>{const x=await flow({cleanup:async c=>{await c.invokeRoute();c.onFailure(Error('transport-observer-dispose-timeout'));}});assert.equal(x.receipt.passed,false);assert.equal(x.receipt.cleanup.length,1);assert.equal(x.receipt.failureObservation.first,null);});
test('no mutation guard, pending state, response handler or final condition changed',()=>{const before=readFileSync(join(new URL('.',import.meta.url).pathname,'before',variant,'launch.mjs'),'utf8');const expression="receipt.passed=!failure&&receipt.scenario?.passed===true&&receipt.cleanup.length===0&&receipt.cleanupPhases.every(x=>x.status==='completed');";assert(source.includes(expression));assert(before.includes(expression));const handler=s=>s.slice(s.indexOf("page.on('response',"),s.indexOf(';receipt.browserVersion=browser.version()',s.indexOf("page.on('response',")));assert.equal(handler(source),handler(before));const mutation=s=>s.slice(s.indexOf("if(!['GET','HEAD','OPTIONS'].includes(req.method()))"),s.indexOf("}),newPage:",s.indexOf("if(!['GET','HEAD','OPTIONS'].includes(req.method()))")));assert.equal(mutation(source),mutation(before));});

test('unawaited accepted closing read is drained before final success',async()=>{let entered,release;const started=new Promise(r=>entered=r),hold=new Promise(r=>release=r);let finished=false;const promise=flow({abort:()=>hold,cleanup:c=>{void c.invokeRoute();entered();}}).then(x=>{finished=true;return x;});await started;await new Promise(resolve=>setImmediate(resolve));assert.equal(finished,false);release();const x=await promise;assert.equal(x.receipt.passed,true);assert.equal(x.receipt.closingReadObservations[0].abort,'acknowledged');assert.equal(x.writes.at(-1).path,'/controlled/result.json');});
test('unawaited closing abort rejection is captured before final failure snapshot',async()=>{let entered,reject;const started=new Promise(r=>entered=r),hold=new Promise((_,r)=>reject=r);const promise=flow({abort:()=>hold,cleanup:c=>{void c.invokeRoute();entered();}});await started;reject(Error('controlled queued abort rejection'));const x=await promise;assert.equal(x.receipt.passed,false);assert.equal(x.receipt.failureObservation.failurePresent,true);assert.equal(x.receipt.closingReadObservations[0].abort,'unconfirmed');assert(x.receipt.cleanup.some(e=>e.code==='closing-read-abort-unsettled'));});
test('closing-read final drain seals new observations and preserves strict failure',async()=>{const x=await flow({finalDrain:c=>c.invokeRoute()});assert.equal(x.receipt.passed,false);assert.equal(x.receipt.closingReadObservations.length,0);assert.equal(x.calls.continued,0);assert.equal(x.receipt.failureObservation.first.phase,'cleanup');});
