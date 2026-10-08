import assert from 'node:assert/strict';
import {test} from 'node:test';
import {mkdtempSync,mkdirSync,rmSync,symlinkSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {checkCommand,validateCheckCreate,assertRecordedCheck,assertCheckCapacity,assertJoinedCheck,assertOwnedCheckContainer} from './project-check-guard.mjs';

function fixture(t){
 const root=mkdtempSync(join(tmpdir(),'dag-check-'));t.after(()=>rmSync(root,{recursive:true,force:true}));
 const primary=join(root,'projects','primary');mkdirSync(primary,{recursive:true});
 const approvals={specificationRevision:3,specificationDigest:'a'.repeat(64),designRevision:4,designDigest:'b'.repeat(64)};
 const requirements=[{id:'AC-dag',title:'Reviewed dependency join',acceptance:'Exact joined output'}];
 const run={id:'run-fixture',projectId:'p-fixture',input:{approvals,requirements},swarm:{tasks:[{integration:{state:'integrated'}},{integration:{state:'integrated'}},{integration:{state:'integrated'}}]}};
 const check={id:'check-'+'c'.repeat(24),projectId:run.projectId,execution:'docker',memoryMiB:1024,cpuMillis:1000,commit:'d'.repeat(40),sourceDigest:'e'.repeat(64),dirty:false,sourceChanged:false,requirementIds:['AC-dag'],requirementSnapshots:requirements,approvalSelection:approvals,specificationRevision:3,specificationDigest:approvals.specificationDigest,designPlanRevision:4,designPlanDigest:approvals.designDigest,status:'running',commands:[{...checkCommand,status:'running',output:'',truncated:false}]};
 const options={fixture:root,primary,run,check,name:'luci-check-'+check.id+'-0',commit:check.commit,sourceDigest:check.sourceDigest,image:'fixture/image:exact',imageID:'sha256:'+'f'.repeat(64),uid:1000,gid:1000};
 const input={Image:options.image,User:'1000:1000',WorkingDir:'/workspace',Entrypoint:['node'],Cmd:['verify.mjs','final'],AttachStdout:true,AttachStderr:true,Labels:{'luci.check':'true','luci.check-id':check.id},Env:['HOME=/tmp','GOCACHE=/tmp/go-cache','GOMODCACHE=/tmp/go-mod','npm_config_cache=/tmp/npm-cache'],ExposedPorts:{},HostConfig:{AutoRemove:true,Init:true,ReadonlyRootfs:true,Memory:1024**3,NanoCpus:1e9,PidsLimit:256,CapDrop:['ALL'],SecurityOpt:['no-new-privileges'],Tmpfs:{'/tmp':'rw,nosuid,nodev,size=1g'},RestartPolicy:{Name:'no'},NetworkMode:'default',Mounts:[{Type:'bind',Source:primary,Target:'/workspace'}]}};
 return {input,options};
}
test('admits exactly the source-defined CLI check request with independent check identity',t=>{
 const f=fixture(t);const admitted=validateCheckCreate(f.input,f.options);assert.equal(admitted.checkId,f.options.check.id);assert.equal(admitted.commit,f.options.commit);
});
for(const [name,change] of Object.entries({
 'foreign check':f=>f.input.Labels['luci.check-id']='check-'+'a'.repeat(24),
 'worker marker':f=>f.input.Labels['luci.managed']='true',
 'foreign name':f=>f.options.name='luci-check-check-other-0',
 'extra command':f=>f.input.Cmd.push('extra'),
 'shell entrypoint':f=>f.input.Entrypoint=['sh'],
 'extra environment':f=>f.input.Env.push('LD_PRELOAD=/tmp/injected'),
 'extra mount':f=>f.input.HostConfig.Mounts.push({Type:'bind',Source:'/var/run/docker.sock',Target:'/var/run/docker.sock'}),
 'foreign source':f=>f.input.HostConfig.Mounts[0].Source='/tmp',
 'writable root':f=>f.input.HostConfig.ReadonlyRootfs=false,
 'host network':f=>f.input.HostConfig.NetworkMode='host',
 'host process namespace':f=>f.input.HostConfig.PidMode='host',
 'capability':f=>f.input.HostConfig.CapAdd=['CAP_SYS_ADMIN'],
 'oversized RAM':f=>f.input.HostConfig.Memory=2*1024**3,
 'oversized CPU':f=>f.input.HostConfig.NanoCpus=2e9,
 'oversized process limit':f=>f.input.HostConfig.PidsLimit=512,
 'published port':f=>f.input.HostConfig.PortBindings={'8080/tcp':[{HostPort:'8080'}]},
 'retained container':f=>f.input.HostConfig.AutoRemove=false,
 'wrong source commit':f=>f.options.check.commit='a'.repeat(40),
 'incomplete integration':f=>f.options.run.swarm.tasks[1].integration.state='unknown',
 'wrong criteria':f=>f.options.check.requirementIds=['AC-other'],
 'different argv receipt':f=>f.options.check.commands=[{...checkCommand,args:['verify.mjs','a']}],
}))test('refuses '+name,t=>{const f=fixture(t);change(f);assert.throws(()=>validateCheckCreate(f.input,f.options));});
test('same-target symlink alias is refused before container allocation',t=>{
 const f=fixture(t),alias=join(f.options.fixture,'projects','alias');symlinkSync(f.options.primary,alias);f.options.primary=alias;f.input.HostConfig.Mounts[0].Source=alias;assert.throws(()=>validateCheckCreate(f.input,f.options));
});
function capacity(){const now=Date.now(),observedAt=new Date(now-100).toISOString();return {view:{policy:{maxActive:1,maxPerProject:1,headroomPercent:25,minHeadroomMiB:2048,diskReserveMiB:1024},resources:{host:{observedAt,status:'ready',memory:{effectiveTotalBytes:16*1024**3,effectiveAvailableBytes:8*1024**3},cpu:{effectiveMillis:4000,utilizationPercent:20}},execution:{observedAt,status:'ready',totalMemoryBytes:16*1024**3,cpuMillis:4000},reservations:{count:0,memoryMiB:0,cpuMillis:0}}},options:{now,rootFreeBytes:7*1024**3,engineID:'engine-exact',engineInfo:{ID:'engine-exact',Driver:'vfs'},daemonOnRootFilesystem:true,containerCount:0,imageVirtualSize:2.3*1024**3}};}
test('fresh independent check guard preserves RAM, CPU, disk and zero-active boundaries',()=>{
 const f=capacity();f.options.imageVirtualSize=Math.floor(f.options.imageVirtualSize);assertCheckCapacity(f.view,f.options);
 for(const mutate of [x=>x.view.resources.host.observedAt=new Date(x.options.now-5001).toISOString(),x=>x.view.resources.host.memory.effectiveAvailableBytes=4*1024**3,x=>x.view.resources.host.cpu.utilizationPercent=76,x=>x.options.containerCount=1,x=>x.options.rootFreeBytes=6*1024**3-1,x=>x.options.engineInfo.ID='foreign',x=>x.options.daemonOnRootFilesystem=false,x=>x.view.resources.reservations.count=1]){const x=structuredClone(f);mutate(x);assert.throws(()=>assertCheckCapacity(x.view,x.options));}
});
test('whole-build evidence needs exact successful Docker output, unchanged source and explicit join',t=>{
 const f=fixture(t),check=f.options.check,run=f.options.run;Object.assign(check,{status:'passed',evidenceStatus:'linked-test-evidence',staleReasons:[]});Object.assign(check.commands[0],{status:'passed',exitCode:0,output:'DAG_CHECK_OK final\n'});assertRecordedCheck(check,f.options,true);
 Object.assign(run,{state:'awaiting-release',checks:[{checkId:check.id,projectId:run.projectId,commit:check.commit,sourceDigest:check.sourceDigest,execution:'docker',requirements:run.input.requirements,approvals:run.input.approvals,commands:[{...checkCommand,status:'passed',exitCode:0}],originalAvailable:true}]});assertJoinedCheck(run,check);
 for(const mutate of [x=>x.sourceChanged=true,x=>x.commands[0].exitCode=1,x=>x.commands[0].output='DAG_CHECK_OK a\n',x=>x.commands[0].truncated=true,x=>x.requirementSnapshots=[]]){const x=structuredClone(check);mutate(x);assert.throws(()=>assertRecordedCheck(x,f.options,true));}
 const wrong=structuredClone(run);wrong.checks[0].checkId='check-other';assert.throws(()=>assertJoinedCheck(wrong,check));
});

test('failed-check cleanup refuses unknown, swapped or foreign containers before mutation',t=>{
 const {input,options}=fixture(t),identity={checkId:options.check.id,name:options.name};
 const creation={kind:'check',status:201,id:'1'.repeat(64),name:options.name,labels:input.Labels};
 const value={Id:creation.id,Name:'/'+creation.name,Image:options.imageID,Config:{Labels:input.Labels,Entrypoint:input.Entrypoint,Cmd:input.Cmd,WorkingDir:input.WorkingDir},Mounts:[{Type:'bind',Source:options.primary,Destination:'/workspace',RW:true}],HostConfig:input.HostConfig};
 const context={creation,identity,fixture:options.fixture,primary:options.primary,imageID:options.imageID};
 assertOwnedCheckContainer(value,context);
 for(const mutate of [x=>x.creation.status=0,x=>x.creation.kind='worker',x=>x.creation.id='2'.repeat(64),x=>x.identity.checkId='check-'+'9'.repeat(24)]){const c=structuredClone(context);mutate(c);assert.throws(()=>assertOwnedCheckContainer(value,c));}
 for(const mutate of [x=>x.Id='2'.repeat(64),x=>x.Name='/unrelated',x=>x.Config.Labels['luci.check-id']='check-other',x=>x.Config.Cmd=['other'],x=>x.Image='sha256:'+'0'.repeat(64),x=>x.Mounts[0].Source='/tmp',x=>x.Mounts.push({Type:'bind',Source:'/var/run/docker.sock',Destination:'/socket',RW:true}),x=>x.HostConfig.Privileged=true]){const v=structuredClone(value);mutate(v);assert.throws(()=>assertOwnedCheckContainer(v,context));}
});
