// Local Unix-socket fixtures only. No Docker engine, installed product, or worker.
import test from 'node:test';
import assert from 'node:assert/strict';
import http from 'node:http';
import vm from 'node:vm';
import {readFileSync} from 'node:fs';
import {mkdtemp,rm} from 'node:fs/promises';
import {join} from 'node:path';
import {tmpdir} from 'node:os';
import {startFencedDockerProxy} from '../launch-recovery/fenced-docker-proxy.mjs';
import {routes} from './worker-recovery-scenario.mjs';
const source=readFileSync(new URL('./installed-worker-recovery.mjs',import.meta.url),'utf8');
const start=source.indexOf(' async function stopWithUnavailableInspect('),end=source.indexOf('\n const ports=',start);
assert(start>0&&end>start);const exactFunction=source.slice(start,end);
const target={teamId:'team-fixture',projectId:'p-a',runId:'run-a',operationId:'op-a',childId:'s-12345678',taskId:'build-a',queueId:'queue-a'};
const send=(socketPath,path)=>new Promise(resolve=>{const request=http.request({socketPath,path,method:'GET',agent:false},response=>{response.resume();response.once('end',()=>resolve({status:response.statusCode}));response.once('error',error=>resolve({error:error.code}));});request.once('error',error=>resolve({error:error.code}));request.end();});
async function fixture(t){
 const root=await mkdtemp(join(tmpdir(),'l016-')),upstreamSocketPath=join(root,'up.sock'),socketPath=join(root,'p.sock'),seen=[];
 const server=http.createServer((request,response)=>{seen.push({method:request.method,path:request.url});response.writeHead(200,{'Content-Type':'application/json'});response.end(JSON.stringify({Id:'a'.repeat(64),State:{Running:true}}));});
 await new Promise(done=>server.listen(upstreamSocketPath,done));
 const proxy=await startFencedDockerProxy({socketPath,upstreamSocketPath,identifyCreate:()=>{throw Error('No fixture Create allowed');},beforeCreate:()=>{throw Error('No fixture Create allowed');},upstreamTimeoutMs:1000,holdTimeoutMs:1000});
 t.after(async()=>{await proxy.close();server.closeAllConnections();await new Promise(done=>server.close(done));await rm(root,{recursive:true,force:true});});
 return {proxy,seen,send:path=>send(socketPath,path)};
}
function choreography(f,request){const receipt={};const run=vm.runInNewContext(exactFunction+'\nstopWithUnavailableInspect',{assert,RegExp,proxy:f.proxy,request,routes,memberToken:'synthetic-token',receipt},{timeout:1000});return {run,receipt};}

test('Actual adapter inspection window covers background reads before Stop and ends afterward',async t=>{
 const f=await fixture(t),path='/v1.51/containers/luci-'+target.childId+'/json';
 const {run,receipt}=choreography(f,async(method,selected,body,actor)=>{
  assert.equal(method,'DELETE');assert.equal(selected,routes(target).stop);assert.equal(body,undefined);assert.equal(actor,'synthetic-token');
  for(let background=0;background<2;background++)assert((await f.send(path)).error);
  assert.equal((await f.send('/v1.51/containers/luci-s-ffffffff/json')).status,200,'Foreign read is outside the target window');
  const exact=await f.send(path);return {status:exact.error?503:200,body:{}};
 });
 assert.equal((await run(target)).status,503);assert.equal(receipt.negativeTransportWindows[0].lostExactInspectResponses,3);
 assert.equal(f.proxy.observations.gates.filter(gate=>gate.state==='lost').length,3);assert.equal(f.proxy.observations.gates.filter(gate=>gate.state==='cancelled').length,5);
 assert.equal((await f.send(path)).status,200,'Positive retry has a restored real upstream response');
 assert(f.seen.every(row=>row.method==='GET'));assert.deepEqual(f.proxy.observations.failures,[]);assert.equal(f.proxy.observations.counts.create,0);assert.equal(f.proxy.observations.counts.start,0);
});
test('Actual adapter refuses an exhausted inspection window even if caller returns503',async t=>{
 const f=await fixture(t),path='/v1.51/containers/luci-'+target.childId+'/json';
 const {run}=choreography(f,async()=>{for(let i=0;i<8;i++)assert((await f.send(path)).error);return {status:503,body:{}};});
 await assert.rejects(run(target),/must not exhaust/);assert(f.seen.every(row=>row.method==='GET'));
});
