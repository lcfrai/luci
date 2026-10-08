// Controlled HTTP/VM fixtures only; no Docker, installed app or providers.
import test from 'node:test';
import assert from 'node:assert/strict';
import http from 'node:http';
import { mkdtemp, rm, readFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import vm from 'node:vm';
import { startFencedDockerProxy } from './fenced-docker-proxy.mjs';

const source=await readFile(new URL('./recovery.mjs',import.meta.url),'utf8');
const controls=source.slice(source.indexOf('async function waitForFixtureGate('),source.indexOf('function admission('));
assert(controls.includes('function guardedIdentifyCreate('));
function fixtureControls(proxy,identifyCreate){
 let publish;
 const fixtureFailureSignal=new Promise(resolve=>{publish=resolve;});
 const context={proxy,identifyCreate,fixtureFailureSignal,receipt:{},stopOnFailure:publish};
 const api=vm.runInNewContext(controls+';({waitForFixtureGate,guardedIdentifyCreate})',context);
 return {api,context};
}
const send=(socketPath,body)=>new Promise((resolve,reject)=>{
 const request=http.request({socketPath,path:'/v1.51/containers/create?name=luci-fixture',method:'POST',agent:false},response=>{
  response.resume();response.once('end',()=>resolve(response.statusCode));response.once('error',reject);
 });request.once('error',reject);request.end(JSON.stringify(body));
});
test('fatal create guard interrupts gate wait before timeout and still drains the real proxy',async t=>{
 const directory=await mkdtemp(join(tmpdir(),'lfg-'));
 let forwarded=0;const upstream=http.createServer((req,res)=>{forwarded++;req.resume();res.writeHead(500);res.end();});
 await new Promise(resolve=>upstream.listen(join(directory,'up.sock'),resolve));
 let proxy;const controls=fixtureControls(null,()=>{throw Error('PRIVATE_FIXTURE_TOKEN=must-not-appear');});
 t.after(async()=>{if(proxy)await proxy.close();await new Promise(resolve=>upstream.close(resolve));await rm(directory,{recursive:true,force:true});});
 proxy=await startFencedDockerProxy({socketPath:join(directory,'proxy.sock'),upstreamSocketPath:join(directory,'up.sock'),identifyCreate:controls.api.guardedIdentifyCreate,beforeCreate:()=>{},upstreamTimeoutMs:1000});
 controls.context.proxy=proxy;
 proxy.armGate({id:'original',phase:'before-forward',method:'POST'});
 const waiting=controls.api.waitForFixtureGate('original',{timeoutMs:60000});
 const rejected=assert.rejects(waiting,/Fixture Create identity\/config guard refused/);
 assert.equal(await send(join(directory,'proxy.sock'),{Image:'synthetic',Env:['PRIVATE_FIXTURE_TOKEN=must-not-appear'],Labels:{'luci.managed':'true'}}),503);
 let deadline;
 try{await Promise.race([rejected,new Promise((_,reject)=>{deadline=setTimeout(()=>reject(Error('fatal fixture did not interrupt gate wait')),2000);})]);}finally{clearTimeout(deadline);}
 assert.equal(forwarded,0);assert.equal(controls.context.receipt.createGuardFailures.length,1);
 assert.equal(JSON.stringify(controls.context.receipt).includes('PRIVATE_FIXTURE_TOKEN'),false);
 const cleanup=await proxy.close();assert.equal(cleanup.drained,true);assert.equal(cleanup.pendingRequests,0);assert.equal(cleanup.activeHandlers,0);assert.equal(cleanup.uncertainMutation,false);
 assert.equal(proxy.observations.requests[0].forwardedAt,undefined);
});
test('gate success and timeout remain authoritative when no fixture failure exists',async()=>{
 const success=fixtureControls({waitForGate:async()=>({status:'holding'})},()=>{});
 assert.equal((await success.api.waitForFixtureGate('original',{})).status,'holding');
 const timeout=fixtureControls({waitForGate:async()=>{throw Error('gate-wait-timeout');}},()=>{});
 await assert.rejects(timeout.api.waitForFixtureGate('original',{}),/gate-wait-timeout/);
});
test('fence validator accepts retained actual daemon shape with synthetic ownership labels',async()=>{
 // The shape retains Docker28.4/API1.51 scratch-probe defaults. Only its
 // generated identity/ownership labels are substituted; no actual call runs.
 const inspect=JSON.parse(await readFile(new URL('./fence-inspect-shape.json',import.meta.url),'utf8'));
 const code=source.slice(source.indexOf('function validateFence('),source.indexOf('function identifyCreate('));
 const validate=vm.runInThisContext('(function(assert,original,instanceID){'+code+';return validateFence;})')(assert,{session:{id:'s-12345678'}},()=> 'fixture-instance');
 assert.equal(validate(inspect,true).intent,'d'.repeat(32));
 const wire={...structuredClone(inspect.Config),HostConfig:structuredClone(inspect.HostConfig)};
 wire.HostConfig.CgroupnsMode='';wire.HostConfig.Runtime='';wire.HostConfig.MemorySwap=0;
 assert.equal(validate(wire,false).intent,'d'.repeat(32));
 const bad=structuredClone(wire);bad.HostConfig.CapAdd=['CAP_SYS_ADMIN'];assert.throws(()=>validate(bad,false));
});
