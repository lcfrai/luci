// Inert tests of exact extracted guards: no helper import, process, HTTP or Docker.
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {join} from 'node:path';
import {test} from 'node:test';
const source=readFileSync(new URL('./cancel-preview.mjs',import.meta.url),'utf8');
function extracted(name,bindings={}){
 const start=source.indexOf('function '+name+'(');assert.ok(start>=0);const end=source.indexOf('\n}',start);assert.ok(end>start);
 const context={assert,Date,Object,Number,join,...bindings};
 return Function(...Object.keys(context),'return ('+source.slice(start,end+2)+')')(...Object.values(context));
}
const nonce='22222222-2222-4222-8222-222222222222',project={id:'p-12345678'},release={id:'rel-controlled'},cfg={engineID:'controlled-engine'},instanceID='11111111-1111-4111-8111-111111111111',commit='c'.repeat(40),scopeDigest='d'.repeat(64),sourceDigest='e'.repeat(64),containerID='a'.repeat(64);
const buildLease={id:'preview-build-controlled'};
const bindings={nonce,project,release,cfg,instanceID,commit,scopeDigest,sourceDigest,containerID,buildLease,result:{previewPostStartedUnixMs:Date.now()-5000}};
const container=()=>({Id:containerID,Created:new Date(Date.now()-2000).toISOString(),Image:'sha256:'+'b'.repeat(64),Config:{Labels:{'org.lcfr.fixture.cancel':nonce,'org.lcfr.fixture.project':project.id,'org.lcfr.fixture.stage':'cancel-run-v1','luci.preview':'true','luci.instance':instanceID,'luci.preview-protocol':'1','luci.preview-engine':cfg.engineID,'luci.preview-allocation':buildLease.id,'luci.preview-release':release.id,'luci.preview-project':project.id,'luci.preview-scope':scopeDigest,'luci.preview-source':sourceDigest,'luci.preview-commit':commit},Cmd:['node','/cancel/hold.mjs']},State:{Running:true,Pid:123,StartedAt:new Date(Date.now()-1000).toISOString()},HostConfig:{Memory:2**30,MemorySwap:2**30,CpuQuota:100000,CpuPeriod:100000,Privileged:false},Mounts:[]});
test('accepts exact fresh stage fixture with all product ownership labels',()=>{
 const value=extracted('verifyStageContainer',bindings)(container());assert.equal(value.id,containerID);assert.match(value.scope,/not cleanup authority/);
});
for(const [name,mutate] of Object.entries({missingProductLabel:x=>delete x.Config.Labels['luci.instance'],foreignNonce:x=>x.Config.Labels['org.lcfr.fixture.cancel']='foreign',foreignProject:x=>x.Config.Labels['org.lcfr.fixture.project']='p-foreign',wrongImmutableID:x=>x.Id='b'.repeat(64),stopped:x=>x.State.Running=false,wrongCommand:x=>x.Config.Cmd=['node','/elsewhere.mjs'],unlimitedMemory:x=>x.HostConfig.Memory=0,enabledSwap:x=>x.HostConfig.MemorySwap=2**31,wrongCPU:x=>x.HostConfig.CpuQuota=0,mount:x=>x.Mounts=[{}],foreignPresentProductLabel:x=>x.Config.Labels['luci.instance']='foreign',oldContainer:x=>x.Created='2000-01-01T00:00:00Z'})){
 test('stage guard rejects '+name,()=>{const row=container();mutate(row);assert.throws(()=>extracted('verifyStageContainer',bindings)(row));});
}
function proof(){return {schema:1,nonce,projectId:project.id,sourceSHA256:'f'.repeat(64),holdMs:45_000,startedUnixMs:Date.now(),platform:'linux',node:'v22.0.0',uid:0,cgroup:{'/sys/fs/cgroup/memory.max':{status:'read',text:'1073741824'},'/sys/fs/cgroup/memory.swap.max':{status:'read',text:'0'},'/sys/fs/cgroup/cpu.max':{status:'read',text:'100000 100000'}}};}
const proofBindings={nonce,project,helperDir:'/inert',hash:()=>'f'.repeat(64)};
test('source-bound RUN proof with actual declared limits passes',()=>{assert.equal(extracted('verifyProof',proofBindings)(proof()).holdMs,45_000);});
for(const [name,mutate] of Object.entries({stale:x=>x.startedUnixMs=Date.now()-11_000,foreignNonce:x=>x.nonce='foreign',differentSource:x=>x.sourceSHA256='0'.repeat(64),unboundedHold:x=>x.holdMs=0,missingCPU:x=>delete x.cgroup['/sys/fs/cgroup/cpu.max'],unlimitedMemory:x=>x.cgroup['/sys/fs/cgroup/memory.max'].text='max'})){
 test('RUN proof rejects '+name,()=>{const row=proof();mutate(row);assert.throws(()=>extracted('verifyProof',proofBindings)(row));});
}
test('one exact release POST and one exact cancel are permitted; replay is refused',()=>{
 const guard=extracted('allowRequest',{project,release,mutationCounts:new Map()});const path='/v1/product/projects/'+project.id+'/releases';guard('POST',path);guard('POST',path+'/'+release.id+'/cancel');assert.throws(()=>guard('POST',path));assert.throws(()=>guard('POST',path+'/'+release.id+'/cancel'));
});
for(const [name,method,path] of [['foreign-project','POST','/v1/product/projects/p-foreign/releases'],['Stop-assist','POST','/v1/product/projects/'+project.id+'/releases/'+release.id+'/stop'],['replacement-restart','POST','/v1/sessions/owned/restart'],['query','GET','/v1/product/projects/'+project.id+'/releases?foreign=true']]){
 test('request guard rejects '+name,()=>assert.throws(()=>extracted('allowRequest',{project,release,mutationCounts:new Map()})(method,path)));
}
