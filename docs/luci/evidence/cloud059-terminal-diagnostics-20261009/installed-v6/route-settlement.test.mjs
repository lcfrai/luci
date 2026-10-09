import test from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFileSync} from 'node:fs';
import {createHash} from 'node:crypto';
import {EventEmitter} from 'node:events';
import {setImmediate as turn} from 'node:timers/promises';
import {holdNextSessionList,scenarioError} from './scenario.mjs';
const original=process.env.C059_ORIGINAL==='1',source=readFileSync(new URL(original?'../installed-v4/scenario.mjs':'./scenario.mjs',import.meta.url),'utf8');
const gateCode=source.slice(source.indexOf('export async function holdNextSessionList(')+7,source.indexOf('\nexport async function runTerminalScenario'));
const scenarioCode=source.slice(source.indexOf('export async function runTerminalScenario(')+7);
async function routeFixture({unrouteInvalidates=false,rejectFulfill=false,recordAfterFulfill=false}={}){
 let now=0,timerID=0,handler,handled=false,fulfill=0,abort=0,unroute=0,continueCount=0;
 const timers=new Map(),events=[],order=[];const timer=(fn,ms)=>{timers.set(++timerID,{at:now+ms,fn});return timerID;};
 const delay=ms=>new Promise(resolve=>timer(resolve,ms));
 const fn=vm.runInNewContext('('+gateCode+')',{URL,performance:{now:()=>now},Date,Buffer,assert,createHash,scenarioError,setTimeout:timer,clearTimeout:id=>timers.delete(id),delay});
 const page=new EventEmitter();page.route=async(_,fn)=>{handler=fn;};page.unroute=async()=>{order.push('unroute');unroute++;if(unrouteInvalidates)handled=true;};
 const req={method:()=> 'GET',url:()=> 'http://127.0.0.1:18730/v1/sessions'};
 const body=Buffer.from(JSON.stringify({sessions:[{id:'s-12345678',status:'running',executionGeneration:'a'.repeat(64)}]}));
 const route={request:()=>req,fetch:async()=>({status:()=>200,body:async()=>body}),fulfill:async args=>{order.push('fulfill');fulfill++;assert.strictEqual(args.body,body);if(handled)throw Error('route.fulfill: Route is already handled!');handled=true;if(rejectFulfill)throw Error('Transport result unavailable');},abort:async()=>{order.push('abort');abort++;if(handled)throw Error('route.abort: Route is already handled!');handled=true;},continue:async()=>{continueCount++;}};
 const gate=await fn(page,{base:'http://127.0.0.1:18730',workspace:'',sessionId:'s-12345678',health:()=>{},record:event=>{events.push(event);if(recordAfterFulfill&&event.kind==='poll-released')throw Error('CHECKPOINT_FAILURE');}});
 const invocation=handler(route);invocation.catch(()=>{});await gate.ready;
 return {gate,invocation,events,order,cancel:()=>{handled=true;page.emit('requestfailed',req);},counts:()=>({fulfill,abort,unroute,continueCount}),listeners:()=>page.listenerCount('requestfailed')};
}
test('dispose settles owned handler before removing route interception',async()=>{const f=await routeFixture({unrouteInvalidates:true});await f.gate.dispose();await f.invocation;assert.deepEqual(f.order,['fulfill','unroute']);assert.equal(f.counts().abort,0);assert.equal(f.listeners(),0);});
test('checkpoint failure after acknowledged fulfill never attempts a second terminal operation',async()=>{const f=await routeFixture({recordAfterFulfill:true});await assert.rejects(f.gate.release(),/CHECKPOINT_FAILURE/);await assert.rejects(f.gate.dispose(),/CHECKPOINT_FAILURE/);assert.equal(f.counts().fulfill,1);assert.equal(f.counts().abort,0);assert.equal(f.listeners(),0);});
test('rejected fulfill remains unknown and is never followed by abort or replay',async()=>{const f=await routeFixture({rejectFulfill:true});await assert.rejects(f.gate.release(),/Transport result unavailable/);await assert.rejects(f.gate.dispose(),/Transport result unavailable/);assert.equal(f.counts().fulfill,1);assert.equal(f.counts().abort,0);});
test('actual owned-request cancellation is recorded and leaves ordering unexercised without handling it again',async()=>{const f=await routeFixture();f.cancel();await assert.rejects(f.gate.release(),/ordering unexercised/);await assert.rejects(f.gate.dispose(),/ordering unexercised/);assert.equal(f.counts().fulfill,0);assert.equal(f.counts().abort,0);assert.equal(f.events.filter(x=>x.kind==='poll-request-failed').length,1);});
test('successful release and repeated disposal retain exactly one fulfillment and removal',async()=>{const f=await routeFixture();await f.gate.release();await f.gate.dispose();await f.gate.dispose();assert.deepEqual(f.counts(),{fulfill:1,abort:0,unroute:1,continueCount:0});});
async function scenarioFixture(){
 const events=[],originalError=Error('PRIMARY_COMMAND_FAILURE'),secondaryError=Error('SECONDARY_ROUTE_FAILURE');let types=0,disposals=0;
 const gate={ready:Promise.resolve({session:{status:'running',executionGeneration:'a'.repeat(64)}}),dispose:async()=>{disposals++;throw secondaryError;},snapshot:()=>({expired:false}),release:async()=>{}};
 const wire={observerFailure:null,browser:{failures:[],rows:[{markers:{ready:true,body:true},exit:[]}]},sent:[{stdinFrames:2}]};
 const ui={label:'fixture — running',footer:'attached',overlay:null,termPresent:true,restartCount:0};
 const pane={waitFor:async()=>{},evaluate:async()=>ui,locator:()=>({evaluate:async()=>{},focus:async()=>{}})};
 const page={locator:()=>pane,keyboard:{type:async()=>{if(++types===3)throw originalError;},press:async()=>{}}};
 const crossAssert=Object.assign((...args)=>assert(...args),assert,{deepEqual:(a,b)=>assert.deepEqual(JSON.parse(JSON.stringify(a)),JSON.parse(JSON.stringify(b)))});
 const fn=vm.runInNewContext('('+scenarioCode+')',{assert:crossAssert,Buffer,Date,Promise,scenarioError,delay:()=>new Promise(()=>{}),holdNextSessionList:async()=>gate});
 let failure;try{await fn({page,base:'http://127.0.0.1:18730',workspace:'',sessionId:'s-12345678',projectId:'p-1234',request:async()=>({id:'s-12345678',projectId:'p-1234',harness:'shell',status:'running',executionGeneration:'a'.repeat(64)}),record:x=>events.push(x),health:()=>{},monitor:{snapshot:async()=>wire},markers:{nonce:'inert'},evidenceDir:'/unused'});}catch(error){failure=error;}
 return {failure,originalError,events,types,disposals};
}
test('real scenario primary failure survives later route disposal failure',async()=>{const f=await scenarioFixture();assert.strictEqual(f.failure,f.originalError);assert.equal(f.types,3);assert.equal(f.disposals,1);});
test('real scenario records primary step and passive failure observation before gate cleanup',async()=>{const f=await scenarioFixture();const kinds=f.events.map(x=>x.kind);assert(kinds.includes('scenario-failure'));assert(kinds.includes('scenario-failure-observation'));assert(kinds.includes('scenario-gate-cleanup-failure'));assert(kinds.indexOf('scenario-failure')<kinds.indexOf('scenario-gate-cleanup-failure'));assert.equal(f.events.find(x=>x.kind==='scenario-failure').step,'first-exit-command');assert(!JSON.stringify(f.events).includes('PRIMARY_COMMAND_FAILURE'));});
test('diagnostic projection hashes arbitrary errors without retaining raw private messages',()=>{const p=scenarioError(Error('PRIVATE_SENTINEL /private/path'));assert.equal(p.messageRetained,false);assert.equal(p.messageSHA256,createHash('sha256').update('PRIVATE_SENTINEL /private/path').digest('hex'));assert(!JSON.stringify(p).includes('PRIVATE_SENTINEL'));});

test('handler selected before async unroute cannot begin a late owned fetch',async()=>{
 let handler,removeResolve,fetches=0,aborts=0;const events=[],listeners=new EventEmitter();
 const fn=vm.runInNewContext('('+gateCode+')',{URL,performance,Date,Buffer,assert,createHash,scenarioError,setTimeout,clearTimeout,delay:()=>new Promise(()=>{})});
 const page={on:listeners.on.bind(listeners),off:listeners.off.bind(listeners),route:async(_,fn)=>{handler=fn;},unroute:()=>new Promise(resolve=>{removeResolve=resolve;})};
 const gate=await fn(page,{base:'http://127.0.0.1:18730',workspace:'',sessionId:'s-12345678',health:()=>{},record:x=>events.push(x)});
 const disposal=gate.dispose();disposal.catch(()=>{});for(let i=0;i<3;i++)await turn();
 const body=Buffer.from(JSON.stringify({sessions:[{id:'s-12345678',status:'running'}]}));
 const pending=handler({request:()=>({method:()=> 'GET',url:()=> 'http://127.0.0.1:18730/v1/sessions'}),fetch:async()=>{fetches++;return {status:()=>200,body:async()=>body};},fulfill:async()=>{},abort:async()=>{aborts++;},continue:async()=>{}});pending.catch(()=>{});
 removeResolve();await assert.rejects(disposal,/ordering unexercised/);await pending;assert.equal(fetches,0);assert.equal(aborts,1);assert.equal(listeners.listenerCount('requestfailed'),0);
});

test('requestfailed checkpoint error cannot escape listener or prevent gate settlement',async()=>{
 let handler;const page=new EventEmitter();page.route=async(_,fn)=>{handler=fn;};page.unroute=async()=>{};
 const fn=vm.runInNewContext('('+gateCode+')',{URL,performance,Date,Buffer,assert,createHash,scenarioError,setTimeout,clearTimeout,delay:()=>new Promise(()=>{})});
 const gate=await fn(page,{base:'http://127.0.0.1:18730',workspace:'',sessionId:'s-12345678',health:()=>{},record:x=>{if(x.kind==='poll-request-failed')throw Error('DIAGNOSTIC_WRITE_FAILED');}});
 let fulfilled=0,aborted=0;const req={method:()=> 'GET',url:()=> 'http://127.0.0.1:18730/v1/sessions'};const body=Buffer.from(JSON.stringify({sessions:[{id:'s-12345678',status:'running'}]}));
 const pending=handler({request:()=>req,fetch:async()=>({status:()=>200,body:async()=>body}),fulfill:async()=>{fulfilled++;},abort:async()=>{aborted++;},continue:async()=>{}});pending.catch(()=>{});await gate.ready;
 assert.doesNotThrow(()=>page.emit('requestfailed',req));await assert.rejects(gate.release(),/DIAGNOSTIC_WRITE_FAILED/);await assert.rejects(gate.dispose(),/DIAGNOSTIC_WRITE_FAILED/);await pending;assert.equal(fulfilled,0);assert.equal(aborted,0);
});
