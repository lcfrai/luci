import assert from 'node:assert/strict';
import test from 'node:test';
import {readFileSync} from 'node:fs';
import {resolve} from 'node:path';
import {createHash} from 'node:crypto';

const path=resolve(process.env.C068_SCENARIO??new URL('./scenario.mjs',import.meta.url).pathname),source=readFileSync(path,'utf8');
const AsyncFunction=Object.getPrototypeOf(async()=>{}).constructor;
class Clock {
 now=100;id=0;timers=new Map();
 set=(fn,ms)=>{const id=++this.id;this.timers.set(id,{at:this.now+ms,fn});return id;};
 clear=id=>this.timers.delete(id);
 delay=ms=>new Promise(resolve=>this.set(resolve,ms));
 async flush(){for(let i=0;i<30;i++)await Promise.resolve();}
 async next(){const next=[...this.timers].sort((a,b)=>a[1].at-b[1].at)[0];if(!next)return false;this.now=next[1].at;this.timers.delete(next[0]);next[1].fn();await this.flush();return true;}
}
function exported(name,next){const start=source.indexOf('export '+name);assert(start>=0);const end=source.indexOf(next,start);assert(end>start);return source.slice(start,end).replace(/^export /,'');}
const heldSource=exported('async function holdNextSessionList','// Wait for natural cleanup only.');
const releaseStart=source.indexOf('export function createBoundedSecondPollRelease');
const releaseSource=releaseStart<0?null:source.slice(releaseStart,source.indexOf('export async function runTerminalScenario',releaseStart)).replace(/^export /,'');
const start=source.indexOf("    stepTo('second-old-poll-hold');"),end=source.indexOf("    stepTo('second-exit-command');",start);assert(start>0&&end>start);const orderingSource=source.slice(start,end);
const sessionId='s-1234abcd',projectId='p-5678abcd',oldGeneration='a'.repeat(64),newGeneration='b'.repeat(64);
const pathPart='/v1/sessions/'+sessionId;

async function exercise(createMs){
 const clock=new Clock(),records=[],events=[],effects={restartPOST:0,readFetch:0,readFulfill:0,readAbort:0,stop:0,destroy:0};
 let routeHandler,resolveAck,complete=false,result,error;
 const ack=new Promise(resolve=>{resolveAck=resolve;});
 const request={url:()=> 'http://127.0.0.1:12345/v1/sessions',method:()=> 'GET'};
 const old={id:sessionId,projectId,status:'exited',executionGeneration:undefined};
 const body=Buffer.from(JSON.stringify({sessions:[old]}));
 const route={request:()=>request,continue:async()=>{},fetch:async()=>{effects.readFetch++;return {status:()=>200,body:async()=>body};},fulfill:async()=>{effects.readFulfill++;events.push({kind:'read-delivered',at:clock.now});},abort:async()=>{effects.readAbort++;}};
 const response={status:()=>200,json:async()=>({id:sessionId,status:'starting',executionGeneration:newGeneration})};
 const page={route:async(_,handler)=>{routeHandler=handler;queueMicrotask(()=>void routeHandler(route));},unroute:async()=>{},on:()=>{},off:()=>{},waitForResponse:()=>ack};
 const pane={getByRole:()=>({click:async()=>{effects.restartPOST++;events.push({kind:'restart-requested',at:clock.now});clock.set(()=>{events.push({kind:'restart-acknowledged',at:clock.now});resolveAck(response);},createMs);}}),locator:()=>({evaluate:async()=>true})};
 const state=async()=>({footer:'attached',restartCount:0,overlay:null});
 const snapshot=async()=>({browser:{rows:[{}, {open:{}}]}});
 const record=row=>records.push(row),health=()=>{};
 const common={assert,performance:{now:()=>clock.now},setTimeout:clock.set,clearTimeout:clock.clear,delay:clock.delay,createHash,scenarioError:error=>({message:String(error.message)})};
 const gateFactory=new Function(...Object.keys(common),`return (${heldSource});`)(...Object.values(common));
 const bounded=releaseSource?new Function(...Object.keys(common),`return (${releaseSource});`)(...Object.values(common)):undefined;
 const args={assert,stepTo:step=>events.push({kind:step,at:clock.now}),holdNextSessionList:gateFactory,page,base:'http://127.0.0.1:12345',workspace:'',sessionId,projectId,health,record,gates:[],delay:clock.delay,path:pathPart,pane,before:{executionGeneration:oldGeneration},secondRelease:undefined,createBoundedSecondPollRelease:bounded,until:async(_,fn)=>{const value=await fn();assert(value);return value;},snapshot,state,fresh:async()=>({id:sessionId,status:'running',executionGeneration:newGeneration})};
 const phase=new AsyncFunction(...Object.keys(args),orderingSource+'\nreturn typeof oldReadOrdering === "undefined" ? {exercised:true} : oldReadOrdering;');
 const work=phase(...Object.values(args)).then(value=>{result=value;complete=true;},value=>{error=value;complete=true;});
 await clock.flush();let spins=0;while(!complete&&spins++<30)assert(await clock.next(),'Controlled scenario stranded without a timer');await work;
 return {result,error,clock:clock.now,effects,events,records};
}

test('a36s synchronous Create completes warm sequencing while the old-after-ACK ordering remains explicitly open',async()=>{
 const result=await exercise(36000);
 assert.ifError(result.error);assert.equal(result.result.exercised,false);
 assert.equal(result.effects.restartPOST,1);assert.equal(result.effects.readFetch,1);assert.equal(result.effects.readFulfill,1);assert.equal(result.effects.readAbort,0);assert.equal(result.effects.stop,0);assert.equal(result.effects.destroy,0);
 const request=result.events.find(row=>row.kind==='restart-requested'),read=result.events.find(row=>row.kind==='read-delivered'),ack=result.events.find(row=>row.kind==='restart-acknowledged');
 assert(read.at-request.at<8000);assert(read.at<ack.at);assert.equal(ack.at-request.at,36000);
});

test('a fast acknowledged Restart still exercises the strict old-response-after-ACK ordering',async()=>{
 const result=await exercise(1000);assert.ifError(result.error);assert.equal(result.result.exercised,true);
 const read=result.events.find(row=>row.kind==='read-delivered'),ack=result.events.find(row=>row.kind==='restart-acknowledged');assert(read.at>=ack.at);assert.equal(result.effects.restartPOST,1);assert.equal(result.effects.readFulfill,1);assert.equal(result.effects.readAbort,0);
});

test('a145s Create inside the150s mutation budget does not extend the8s read gate',async()=>{
 const result=await exercise(145000);assert.ifError(result.error);assert.equal(result.result.exercised,false);
 const request=result.events.find(row=>row.kind==='restart-requested'),read=result.events.find(row=>row.kind==='read-delivered'),ack=result.events.find(row=>row.kind==='restart-acknowledged');assert(read.at-request.at<8000);assert.equal(ack.at-request.at,145000);assert.equal(result.effects.restartPOST,1);assert.equal(result.effects.readFulfill,1);
});

test('the full scenario cannot report blanket success while the ordering is open, and completes natural exit before returning',()=>{
 assert(source.includes("stepTo('second-exit-command')"));
 assert(source.includes("return {passed:oldReadOrdering.exercised,warmRestartPassed:true,oldReadOrdering,requiredAcceptanceIncomplete:oldReadOrdering.exercised?[]:['installed-old-response-after-restart-acknowledgement']"));
 assert(source.indexOf("stepTo('second-exit-command')")<source.indexOf('return {passed:oldReadOrdering.exercised'));
});
