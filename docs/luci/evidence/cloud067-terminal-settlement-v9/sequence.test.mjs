import assert from 'node:assert/strict';
import test from 'node:test';
import {readFileSync} from 'node:fs';
import {resolve} from 'node:path';
import {pathToFileURL} from 'node:url';

const scenarioPath=resolve(process.env.C067_SCENARIO??new URL('./scenario.mjs',import.meta.url).pathname);
const launcherPath=resolve(process.env.C067_LAUNCHER??new URL('./launch.mjs',import.meta.url).pathname);
const source=readFileSync(scenarioPath,'utf8'),launcher=readFileSync(launcherPath,'utf8');
const {waitForOwnedTerminalSettlement}=await import(pathToFileURL(scenarioPath));
const start=source.indexOf("    stepTo('first-poll-release');"),end=source.indexOf("const responsePromise=",start);
assert(start>0&&end>start);const sequence=source.slice(start,end);
const admissionStart=launcher.indexOf(' async function admission(label)'),admissionEnd=launcher.indexOf('\n const startup=',admissionStart);
assert(admissionStart>0&&admissionEnd>admissionStart);const admissionSource=launcher.slice(admissionStart,admissionEnd).trim();
const AsyncFunction=Object.getPrototypeOf(async()=>{}).constructor;
const id='a'.repeat(64),gen='b'.repeat(64),sessionId='s-1234abcd',projectId='p-5678abcd';
const allocation={id,sandboxId:sessionId,sessionId,instanceId:'fixture-instance',imageID:'sha256:'+'c'.repeat(64),name:'/luci-'+sessionId,status:'running',memoryBytes:1024**3,cpuNano:1_000_000_000};
const lease={id:'manual-owned',sessionId,projectId,sandboxId:sessionId,state:'backend-settled',resources:{CPUMillis:1000,MemoryMiB:1024}};
const ledger=leases=>({present:true,sha256:'d'.repeat(64),bytes:498,leases:structuredClone(leases),checks:[],fences:[],checkFences:[]});
function pending(){return {initial:{sessionId,executionGeneration:gen,containers:[allocation],ledger:ledger([lease])},current:{session:{id:sessionId,projectId,status:'exited',sandboxId:sessionId,sandboxRef:id,credentialsCleared:true},containers:[{...allocation,status:'exited'}],ledger:ledger([lease]),rootFreeBytes:5*1024**3}};}
function settled(){const v=pending();v.current.containers=[];v.current.ledger=ledger([]);v.current.rootFreeBytes=7*1024**3;return v;}
async function run(observations,{allowForward=true}={}){
 let clock=0,calls=0,last,forward=0,reads=0;const events=[],records=[],effects={mutations:0,destroys:0,stops:0};
 const observed=()=>{last=structuredClone(observations[Math.min(calls++,observations.length-1)]);return last;};
 const health=()=>{};
 const args={page:{},base:'http://fixture.invalid',workspace:'',sessionId,projectId,health,record:row=>records.push(row),gates:[],before:{executionGeneration:gen},observeSettlement:observed,delay:()=>new Promise(()=>{}),firstGate:{release:async()=>events.push('first-gate-released'),dispose:async()=>events.push('first-gate-disposed')},state:async()=>({restartCount:1}),stepTo:value=>events.push(value),until:async(label,fn,ms)=>{assert.equal(label,'Fresh exited state and Restart');assert.equal(ms,20000);assert(await fn());events.push('original20s-status-check-passed');},holdNextSessionList:async()=>{events.push('second-old-poll-opened');return {ready:Promise.resolve({})};},waitForOwnedTerminalSettlement:typeof waitForOwnedTerminalSettlement==='function'?options=>waitForOwnedTerminalSettlement({...options,now:()=>clock,sleep:async ms=>{clock+=ms;}}):undefined};
 const perform=new AsyncFunction(...Object.keys(args),sequence);
 const admission=new Function('health','free','request','record','readLedger','inventory','assert',`return (${admissionSource});`)(health,()=>last?.current.rootFreeBytes??observations[0].current.rootFreeBytes,async(method)=>{assert.equal(method,'GET');reads++;return {policy:{},resources:{}};},row=>records.push(row),()=>last?.current.ledger??observations[0].current.ledger,()=>last?.current.containers??observations[0].current.containers,assert);
 let error;
 try{await perform(...Object.values(args));await admission('ui-restart');if(allowForward)forward++;}catch(value){error=value;}
 assert.deepEqual(effects,{mutations:0,destroys:0,stops:0});
 return {error,events,records,forward,reads,clock,calls};
}

test('unchanged status check precedes exact natural cleanup, then old-poll gate and unchanged six-GiB admission',async()=>{
 const result=await run([pending(),settled()]);
 assert.ifError(result.error);assert.equal(result.forward,1);assert.equal(result.calls,2);
 assert(result.events.indexOf('original20s-status-check-passed')<result.events.indexOf('initial-natural-settlement'));
 assert(result.events.indexOf('initial-natural-settlement')<result.events.indexOf('second-old-poll-opened'));
 assert(result.records.some(row=>row.kind==='initial-settlement-confirmed'));assert.equal(result.clock,350);
});

test('the unchanged disk floor still refuses after fully proven cleanup if unrelated space remains low',async()=>{
 const empty=settled();empty.current.rootFreeBytes=5*1024**3;
 const result=await run([empty]);assert.match(result.error?.message??'',/Six GiB fixture root floor/);assert.equal(result.forward,0);
});

for(const [name,change,message] of [
 ['foreign immutable container',v=>v.current.containers[0].id='f'.repeat(64),/ownership changed/],
 ['changed image',v=>v.current.containers[0].imageID='sha256:'+'e'.repeat(64),/ownership changed/],
 ['changed instance',v=>v.current.containers[0].instanceId='foreign-instance',/ownership changed/],
 ['unknown daemon state',v=>v.current.containers[0].status='unknown',/state unconfirmed/],
 ['unavailable ledger',v=>v.current.ledger.present=false,/Ledger unavailable/],
 ['missing held-state arrays',v=>delete v.current.ledger.fences,/Incomplete held-resource/],
 ['foreign held lease',v=>v.current.ledger.leases[0].projectId='other-project',/Unrelated or changed charge/],
 ['unsettled launch fence',v=>v.current.ledger.fences=[{state:'pending'}],/Unsettled launch fence/],
 ['missing charge while container remains',v=>v.current.ledger.leases=[],/expected charge/],
 ['changed registry allocation',v=>v.current.session.sandboxRef='f'.repeat(64),/Expected values to be strictly equal/],
 ])test(name+' refuses before creating the stale-read gate or forwarding Restart',async()=>{
 const value=pending();value.current.rootFreeBytes=7*1024**3;change(value);
 const result=await run([value]);assert.match(result.error?.message??'',message);assert.equal(result.forward,0);assert(!result.events.includes('second-old-poll-opened'));
});

test('exact owned charge without container stays pending and bounded; never released or bypassed by helper',async()=>{
 const value=settled();value.current.ledger=ledger([lease]);
 const result=await run([value]);assert.match(result.error?.message??'',/cleanup remains unconfirmed/);assert.equal(result.forward,0);assert(result.clock<=35000);assert(!result.events.includes('second-old-poll-opened'));
});

test('exact stopped container and charge stay pending until bounded refusal without force cleanup',async()=>{
 const value=pending();value.current.rootFreeBytes=7*1024**3;
 const result=await run([value]);assert.match(result.error?.message??'',/cleanup remains unconfirmed/);assert.equal(result.forward,0);assert(result.clock<=35000);assert(!result.events.includes('second-old-poll-opened'));
});
