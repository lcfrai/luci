import test from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFileSync} from 'node:fs';
import {createHash} from 'node:crypto';
import {setImmediate as turn} from 'node:timers/promises';
import {holdNextSessionList} from './scenario.mjs';
const source=process.env.C054_GATE_ORIGINAL==='1'?readFileSync(new URL('./scenario-before-absolute-budget.mjs',import.meta.url),'utf8').split('export async function holdNextSessionList(')[1].split('\nexport async function runTerminalScenario')[0]:null;
const code=source?'async function holdNextSessionList('+source:holdNextSessionList.toString();
async function fixture({fetchMs=4500,bodyMs=0}={}){
  let now=0,id=0,handler,fulfilled=0,aborted=0,continued=0,fetchTimeout;
  const timers=new Map(),events=[];
  const timer=(fn,ms)=>{timers.set(++id,{due:now+ms,fn});return id;};
  const delay=ms=>new Promise(resolve=>timer(resolve,ms));
  const fn=vm.runInNewContext('('+code+')',{URL,performance:{now:()=>now},Date,assert,createHash,Buffer,scenarioError:error=>({name:error.name}),setTimeout:timer,clearTimeout:id=>timers.delete(id),delay});
  const page={on:()=>{},off:()=>{},route:async(pattern,fn)=>{handler=fn;},unroute:async()=>{}};
  const body=Buffer.from(JSON.stringify({sessions:[{id:'s-12345678',status:'running',executionGeneration:'a'.repeat(64)}]}));
  const route={request:()=>({method:()=> 'GET',url:()=> 'http://127.0.0.1:18730/v1/sessions'}),fetch:async({timeout})=>{fetchTimeout=timeout;now+=fetchMs;return {status:()=>200,body:async()=>{now+=bodyMs;return body;}};},fulfill:async args=>{assert.strictEqual(args.body,body);fulfilled++;},abort:async()=>{aborted++;},continue:async()=>{continued++;}};
  const gate=await fn(page,{base:'http://127.0.0.1:18730',workspace:'',sessionId:'s-12345678',health:()=>{},record:event=>events.push(event)});
  const invocation=handler(route);invocation.catch(()=>{});
  const flush=async()=>{for(let i=0;i<8;i++)await turn();};
  const advance=async target=>{assert(target>=now);now=target;for(const [key,t]of [...timers])if(t.due<=now){timers.delete(key);t.fn();}await flush();};
  return {gate,invocation,timers,events,flush,advance,now:()=>now,counts:()=>({fulfilled,aborted,continued,fetchTimeout}),body};
}
test('delayed fetch shares one absolute8s deadline and cannot forward after it',async()=>{const f=await fixture();await f.gate.ready;assert.equal(f.now(),4500);const earliest=Math.min(...[...f.timers.values()].map(x=>x.due));assert.equal(earliest,8000,'fetch must consume the original gate budget, not precede it');await f.advance(8000);await assert.rejects(f.gate.release(),/budget expired/);await f.invocation;assert.equal(f.counts().fulfilled,0);assert.equal(f.counts().aborted,1);await assert.rejects(f.gate.dispose(),/budget expired/);});
test('body read consuming remaining deadline cannot publish a ready stale response',async()=>{const f=await fixture({fetchMs:4500,bodyMs:4000});await assert.rejects(f.gate.ready,/budget expired/);await f.invocation;assert.equal(f.counts().fulfilled,0);assert.equal(f.counts().aborted,1);await assert.rejects(f.gate.dispose(),/budget expired/);});
test('within-budget real response bytes fulfill once and retain exact body hash',async()=>{const f=await fixture();const held=await f.gate.ready;await f.advance(6000);await f.gate.release();await f.gate.dispose();await f.invocation;assert.equal(f.counts().fulfilled,1);assert.equal(f.counts().aborted,0);assert.equal(f.counts().fetchTimeout,5000);assert.equal(held.bodySHA256,createHash('sha256').update(f.body).digest('hex'));assert.equal(f.gate.snapshot().expired,false);});
