import test from 'node:test';
import assert from 'node:assert/strict';
import http from 'node:http';
import { mkdtempSync, rmSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { startDockerCreateProxy } from './docker-create-proxy.mjs';

test('guard refusal and overlapping create never reach a real upstream request', async () => {
 const dir=mkdtempSync(join(tmpdir(),'luci-proxy-guard-'));
 const upstreamPath=join(dir,'upstream.sock'),proxyPath=join(dir,'proxy.sock');
 let forwarded=0,release,entered;
 const held=new Promise(done=>release=done),ready=new Promise(done=>entered=done);
 const server=http.createServer((req,res)=>{forwarded++;req.resume();req.on('end',()=>{res.writeHead(201,{'content-type':'application/json'});res.end(JSON.stringify({Id:'a'.repeat(64)}));});});
 await new Promise(done=>server.listen(upstreamPath,done));
 let proxy;
 const request=()=>new Promise((resolve,reject)=>{const req=http.request({socketPath:proxyPath,path:'/v1.47/containers/create',method:'POST'},res=>{res.resume();res.on('end',()=>resolve(res.statusCode));});req.on('error',reject);req.end(JSON.stringify({Image:'controlled-image'}));});
 try{
  proxy=await startDockerCreateProxy({socketPath:proxyPath,upstreamSocketPath:upstreamPath,maxCreates:3,beforeCreate:async({ordinal})=>{if(ordinal===1){entered();await held;}else throw Error('explicit capacity refusal');}});proxy.releaseHeld();
  const first=request();await ready;assert.equal(forwarded,0);
  assert.equal(await request(),503);assert.equal(forwarded,0);
  release();assert.equal(await first,201);assert.equal(forwarded,1);
  assert.equal(await request(),503);assert.equal(forwarded,1);
  assert.equal(proxy.observations.requests.filter(row=>row.allocationGuardRejected).length,2);
 }finally{
  release();if(proxy)await proxy.close();await new Promise(done=>server.close(done));rmSync(dir,{recursive:true});
 }
});

// The private socket adds a directory and basename to the public socket parent.
// Reject this before creating any directory, listener, or upstream request.
test('overlong private socket path is refused before listener allocation', async () => {
 await assert.rejects(startDockerCreateProxy({socketPath:'/workspace/'+ 'x'.repeat(75)+'/proxy.sock',upstreamSocketPath:'/var/run/docker.sock'}), /socket path is too long/);
});
