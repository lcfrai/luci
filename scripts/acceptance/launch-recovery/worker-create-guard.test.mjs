import assert from 'node:assert/strict';
import { test } from 'node:test';
import { mkdtempSync,mkdirSync,rmSync,symlinkSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { validateWorkerCreate } from './worker-create-guard.mjs';
function setup(t,version=1){
 const fixture=mkdtempSync(join(tmpdir(),'luci-mount-'));t.after(()=>rmSync(fixture,{recursive:true,force:true}));
 const scope={fixture,projectID:'p-1',runID:'run-1',operationID:'op-1',session:{id:'s-12345678',projectId:'p-1',harness:'codex',worktreeId:'w-1',conversationId:'c-1'},images:['verified'],instance:'fixture-instance',uid:1000,gid:1000};
 const paths=[join(fixture,'projects/.luci-swarms/p-1/run-1/op-1'),join(fixture,'state/conversations/local/.reviewed-context/s-12345678'),join(fixture,'state/conversations/native/local/w-1/codex/c-1')];for(const path of paths)mkdirSync(path,{recursive:true});mkdirSync(join(paths[0],'.git/worktrees'),{recursive:true});
 const input={Image:'verified',Labels:{'luci.managed':'true','luci.instance':scope.instance,'luci.session-id':scope.session.id,'luci.sandbox-id':scope.session.id},User:'0:0',WorkingDir:'/home/dev/work',Env:['LUCI_HOST_UID=1000','LUCI_HOST_GID=1000'],HostConfig:{Binds:[paths[0]+':/home/dev/work',paths[1]+':/home/dev/.luci-reviewed-context:ro',paths[2]+':/home/dev/.luci-agent',join(paths[0],'.git/worktrees')+':/home/dev/work/.git/worktrees:ro'],Memory:1024**3,NanoCpus:1000000000,PidsLimit:256,RestartPolicy:{Name:'no'},CapDrop:['ALL'],CapAdd:['CAP_CHOWN','CAP_DAC_OVERRIDE','CAP_SETGID','CAP_SETPCAP','CAP_SETUID'],SecurityOpt:['no-new-privileges:true'],ExtraHosts:['host.docker.internal:host-gateway'],PortBindings:{'7777/tcp':[{HostIp:'127.0.0.1',HostPort:''}]}}};if(version===2){scope.taskContextVersion=2;const output=join(fixture,'state/conversations/local/.task-output',scope.session.id);mkdirSync(output,{recursive:true});input.HostConfig.Binds.push(output+':/home/dev/.luci-task-output');}return {fixture,scope,input,paths};
}
test('only exact owned child mounts and loopback ephemeral binding pass',t=>{const {scope,input}=setup(t);assert.equal(validateWorkerCreate(input,scope).length,4);});
for(const version of [1,2])for(const [name,change] of [
 ['widened capabilities',s=>{s.input.HostConfig.CapAdd.push('CAP_SYS_ADMIN');}],
 ['missing dropped capabilities',s=>{s.input.HostConfig.CapDrop=[];}],
 ['unnormalized source-only capabilities',s=>{s.input.HostConfig.CapAdd=['CHOWN','DAC_OVERRIDE','SETUID','SETGID','SETPCAP'];}],
 ['duplicate wire capability',s=>{s.input.HostConfig.CapAdd.push('CAP_CHOWN');}],
 ['writable Git administration',s=>{s.input.HostConfig.Binds[3]=s.input.HostConfig.Binds[3].slice(0,-3);}],
 ['missing Git administration protection',s=>{s.input.HostConfig.Binds.splice(3,1);}],
 ['another operation Git administration',s=>{const other=join(s.fixture,'projects/.luci-swarms/p-1/run-1/op-2/.git/worktrees');mkdirSync(other,{recursive:true});s.input.HostConfig.Binds[3]=other+':/home/dev/work/.git/worktrees:ro';}],
 ['linked Git administration',s=>{const admin=join(s.paths[0],'.git/worktrees'),other=join(s.fixture,'other-admin');rmSync(admin,{recursive:true});mkdirSync(other);symlinkSync(other,admin);}],
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
 ['worker CPU budget expansion',s=>{s.input.HostConfig.NanoCpus=2000000000;}],
 ['worker process budget expansion',s=>{s.input.HostConfig.PidsLimit=512;}],
 ['duplicate host identity environment',s=>{s.input.Env.push('LUCI_HOST_UID=0');}],
 ['dynamic loader environment',s=>{s.input.Env.push('LD_PRELOAD=/host/payload');}],
])test('V'+version+' '+name+' is rejected before forwarding',t=>{const state=setup(t,version);change(state);assert.throws(()=>validateWorkerCreate(state.input,state.scope));});

// Source-faithful Linux Create after SDK serialization: null/zero optional
// fields are not daemon Inspect defaults. Synthetic environment values only.
test('complete serialized production worker shape retains strict admission',t=>{
 const {scope,input}=setup(t);
 Object.assign(input,{Hostname:'',Domainname:'',AttachStdin:false,AttachStdout:false,AttachStderr:false,Tty:false,OpenStdin:false,StdinOnce:false,Cmd:null,Entrypoint:null,Volumes:null,NetworkDisabled:false,ExposedPorts:{'7777/tcp':{}}});
 input.Env.unshift('LUCI_COMMAND=codex','LUCI_ARGS=["exec"]','LUCI_ATTACH_TOKEN=synthetic-fixture-token');
 Object.assign(input.HostConfig,{ContainerIDFile:'',LogConfig:{Type:'',Config:null},NetworkMode:'',AutoRemove:false,VolumeDriver:'',VolumesFrom:null,ConsoleSize:[0,0],CgroupnsMode:'',Dns:null,DnsOptions:null,DnsSearch:null,GroupAdd:null,IpcMode:'',Cgroup:'',Links:null,OomScoreAdj:0,PidMode:'',Privileged:false,PublishAllPorts:false,ReadonlyRootfs:false,UTSMode:'',UsernsMode:'',ShmSize:0,Runtime:'',Isolation:'',CpuShares:0,CgroupParent:'',BlkioWeight:0,CpuPeriod:0,CpuQuota:0,CpuRealtimePeriod:0,CpuRealtimeRuntime:0,CpusetCpus:'',CpusetMems:'',Devices:null,DeviceCgroupRules:null,DeviceRequests:null,MemoryReservation:0,MemorySwap:0,MemorySwappiness:null,OomKillDisable:null,Ulimits:null,Mounts:null});
 assert.equal(validateWorkerCreate(JSON.parse(JSON.stringify(input)),scope).length,4);
});

for(const version of [1,2])test('explicit V'+version+' optional Claude mount still has exact ownership',t=>{const {fixture,scope,input}=setup(t,version);const path=join(fixture,'state/conversations/local',scope.session.worktreeId);mkdirSync(path,{recursive:true});input.HostConfig.Binds.push(path+':/home/dev/.claude/projects');assert.equal(validateWorkerCreate(input,scope).length,version===2?6:5);});
test('V2 requires exactly the child-owned writable report output',t=>{const {scope,input}=setup(t,2);assert.equal(validateWorkerCreate(input,scope).length,5);});
for(const [name,change] of [
 ['missing output mount',s=>{s.input.HostConfig.Binds.pop();}],
 ['read-only report output',s=>{s.input.HostConfig.Binds[4]+=':ro';}],
 ['different child output',s=>{const path=join(s.fixture,'state/conversations/local/.task-output/s-87654321');mkdirSync(path);s.input.HostConfig.Binds[4]=path+':/home/dev/.luci-task-output';}],
 ['other conversation owner',s=>{const path=join(s.fixture,'state/conversations/other/.task-output',s.scope.session.id);mkdirSync(path,{recursive:true});s.input.HostConfig.Binds[4]=path+':/home/dev/.luci-task-output';}],
 ['symlink output ancestor',s=>{const parent=join(s.fixture,'state/conversations/local/.task-output'),outside=join(s.fixture,'outside-output');rmSync(parent,{recursive:true});mkdirSync(join(outside,s.scope.session.id),{recursive:true});symlinkSync(outside,parent);}],
 ['undeclared extra output',s=>{const path=join(s.fixture,'extra-output');mkdirSync(path);s.input.HostConfig.Binds.push(path+':/home/dev/extra');}],
])test('V2 '+name+' is refused before forwarding',t=>{const s=setup(t,2);change(s);assert.throws(()=>validateWorkerCreate(s.input,s.scope));});
test('V1 preserves its verified shape and refuses a V2 output mount',t=>{const s=setup(t,2);delete s.scope.taskContextVersion;assert.throws(()=>validateWorkerCreate(s.input,s.scope));});
for(const version of [0,3,'2',null])test('unrecognized taskContextVersion '+JSON.stringify(version)+' is refused',t=>{const s=setup(t);s.scope.taskContextVersion=version;assert.throws(()=>validateWorkerCreate(s.input,s.scope));});
