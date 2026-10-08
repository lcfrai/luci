import assert from 'node:assert/strict';
import { test } from 'node:test';
import { mkdtempSync,mkdirSync,rmSync,symlinkSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { validateWorkerCreate } from './worker-create-guard.mjs';
function setup(t){
 const fixture=mkdtempSync(join(tmpdir(),'luci-mount-'));t.after(()=>rmSync(fixture,{recursive:true,force:true}));
 const scope={fixture,projectID:'p-1',runID:'run-1',operationID:'op-1',session:{id:'s-12345678',projectId:'p-1',harness:'codex',worktreeId:'w-1',conversationId:'c-1'},images:['verified'],instance:'fixture-instance',uid:1000,gid:1000};
 const paths=[join(fixture,'projects/.luci-swarms/p-1/run-1/op-1'),join(fixture,'state/conversations/local/.reviewed-context/s-12345678'),join(fixture,'state/conversations/native/local/w-1/codex/c-1')];for(const path of paths)mkdirSync(path,{recursive:true});
 const input={Image:'verified',Labels:{'luci.managed':'true','luci.instance':scope.instance,'luci.session-id':scope.session.id,'luci.sandbox-id':scope.session.id},User:'0:0',WorkingDir:'/home/dev/work',Env:['LUCI_HOST_UID=1000','LUCI_HOST_GID=1000'],HostConfig:{Binds:[paths[0]+':/home/dev/work',paths[1]+':/home/dev/.luci-reviewed-context:ro',paths[2]+':/home/dev/.luci-agent'],Memory:1024**3,NanoCpus:1000000000,PidsLimit:256,RestartPolicy:{Name:'no'},CapDrop:['ALL'],CapAdd:['CHOWN','DAC_OVERRIDE','SETUID','SETGID','SETPCAP'],SecurityOpt:['no-new-privileges:true'],ExtraHosts:['host.docker.internal:host-gateway'],PortBindings:{'7777/tcp':[{HostIp:'127.0.0.1',HostPort:''}]}}};return {fixture,scope,input,paths};
}
test('only exact owned child mounts and loopback ephemeral binding pass',t=>{const {scope,input}=setup(t);assert.equal(validateWorkerCreate(input,scope).length,3);});
for(const [name,change] of [
 ['host Docker socket',s=>s.input.HostConfig.Binds.push('/var/run/docker.sock:/var/run/docker.sock')],
 ['another project source',s=>{const path=join(s.fixture,'projects/other');mkdirSync(path);s.input.HostConfig.Binds[0]=path+':/home/dev/work';}],
 ['writable reviewed context',s=>{s.input.HostConfig.Binds[1]=s.input.HostConfig.Binds[1].slice(0,-3);}],
 ['alternative mount API',s=>{s.input.HostConfig.Mounts=[{Type:'bind',Source:'/',Target:'/host'}];}],
 ['host PID namespace',s=>{s.input.HostConfig.PidMode='host';}],
 ['device mapping',s=>{s.input.HostConfig.Devices=[{PathOnHost:'/dev/kvm'}];}],
 ['nonloopback binding',s=>{s.input.HostConfig.PortBindings['7777/tcp'][0].HostIp='0.0.0.0';}],
 ['duplicate target',s=>{s.input.HostConfig.Binds.push(s.input.HostConfig.Binds[0]);}],
 ['symlink mount root',s=>{rmSync(s.paths[0],{recursive:true});const outside=join(s.fixture,'outside');mkdirSync(outside);symlinkSync(outside,s.paths[0]);}],
 ['worker memory budget expansion',s=>{s.input.HostConfig.Memory=4*1024**3;}],
 ['duplicate host identity environment',s=>{s.input.Env.push('LUCI_HOST_UID=0');}],
 ['dynamic loader environment',s=>{s.input.Env.push('LD_PRELOAD=/host/payload');}],
])test(name+' is rejected before forwarding',t=>{const state=setup(t);change(state);assert.throws(()=>validateWorkerCreate(state.input,state.scope));});
