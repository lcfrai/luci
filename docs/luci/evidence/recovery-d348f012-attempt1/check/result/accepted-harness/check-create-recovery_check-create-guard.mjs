// Scoped extraction/generalization from helper81f2 project-check-guard.mjs. No DAG or fabricated filtered ledger.
// Dedicated one-check acceptance guard, not a production admission replacement.
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {lstatSync,realpathSync} from 'node:fs';
import {join,resolve,sep} from 'node:path';

export const checkCommand={label:'node',executable:'node',args:['verify.mjs','final']};
const checkEnv={HOME:'/tmp',GOCACHE:'/tmp/go-cache',GOMODCACHE:'/tmp/go-mod',npm_config_cache:'/tmp/npm-cache'};
const gib=1024**3;
const hash=value=>createHash('sha256').update(value).digest('hex');
const positive=value=>{assert(Number.isSafeInteger(value)&&value>0);return value;};
const fresh=(value,now)=>{const age=now-Date.parse(value);assert(Number.isFinite(age)&&age>=0&&age<=5000,'Fresh independent capacity observations are required');};

// Go CheckSpec has exported field names, while CheckIdentity has JSON tags.
// Match encoding/json HTML escaping before independently checking its digest.
export function checkCommandDigest({identity,primary,sourceDigest,uid,gid}){
 const Identity={protocol:1,engineId:'',instanceId:'',allocationId:identity.allocationId,checkId:identity.checkId,commandIndex:0,scopeDigest:identity.scopeDigest,projectId:identity.projectId,commandDigest:''};
 const spec={Identity,Executable:checkCommand.executable,Args:checkCommand.args,SourceDirectory:primary,SourceDigest:sourceDigest,Resources:{CPUMillis:1000,MemoryMiB:1024},UserID:uid,GroupID:gid};
 const encoded=JSON.stringify(spec).replace(/[<>&\u2028\u2029]/g,value=>'\\u'+value.charCodeAt(0).toString(16).padStart(4,'0'));
 return hash(encoded);
}

export function assertRecordedCheck(check,options){
 const {project,commit,sourceDigest}=options;
 assert.equal(check.projectId,project.id);assert.match(check.id,/^check-[a-f0-9]{24}$/);
 assert.equal(check.execution,'docker');assert.equal(check.commit,commit);assert.equal(check.sourceDigest,sourceDigest);
 assert.equal(check.dirty,false);assert.equal(check.sourceChanged,false);assert.equal(check.memoryMiB,1024);assert.equal(check.cpuMillis,1000);
 assert.equal(check.commands.length,1);assert.deepEqual({label:check.commands[0].label,executable:check.commands[0].executable,args:check.commands[0].args},checkCommand);
 assert(['queued','running'].includes(check.status));assert(Number.isSafeInteger(check.revision)&&check.revision>=1);
 assert.deepEqual(check.requirementIds,['AC-recovery']);assert.equal(check.specificationRevision,project.spec.revision);assert.equal(check.designPlanRevision,project.design.revision);
 assert.equal(check.specificationDigest,project.spec.digest);assert.equal(check.designPlanDigest,project.design.digest);
}
export function assertCheckLease(ledger,options){
 const {check,primary,sourceDigest,uid,gid,scopeDigest,actor,engineID,instance,imageVirtualSize,storageDriver,authorizedChecks,now=Date.now()}=options;
 assert.equal(ledger.version,1);assert.deepEqual(ledger.leases,[]);
 const active=(ledger.checks??[]).filter(row=>row.state!=='finished');assert(active.length>=1&&active.length<=2);
 for(const other of active){
  assert(authorizedChecks.has(other.identity.checkId),'Unconsented current allocation');assert.match(other.identity.checkId,/^check-[a-f0-9]{24}$/);
  assert.equal(other.identity.engineId,engineID);assert.equal(other.identity.instanceId,instance);
  assert.equal(other.identity.protocol,1);assert.equal(other.identity.commandIndex,0);
  assert.equal(other.scope,scopeDigest);assert.equal(other.identity.scopeDigest,scopeDigest);
  assert.equal(other.identity.allocationId,other.id);assert.match(other.id,/^checkexec-[a-f0-9]{24}$/);
  assert.deepEqual(other.actor,{...actor,projectId:other.identity.projectId});
  assert.deepEqual(other.resources,{CPUMillis:1000,MemoryMiB:1024});
  assert.equal(other.createDiskBytes,imageVirtualSize*(storageDriver==='vfs'?2:1));
 }
 const matches=active.filter(row=>row.id===check.commands[0].allocationId);assert.equal(matches.length,1);const lease=matches[0];
 const identity={protocol:1,engineId:engineID,instanceId:instance,allocationId:lease.id,checkId:check.id,commandIndex:0,scopeDigest,projectId:check.projectId,commandDigest:''};
 identity.commandDigest=checkCommandDigest({identity,primary,sourceDigest,uid,gid});
 assert.deepEqual(lease.identity,identity);assert.deepEqual(lease.actor,actor);assert.equal(lease.state,'creating');assert.equal(lease.ref??'','');assert.equal(check.resourcesHeld,true);
 assert.equal(lease.createDiskBytes,imageVirtualSize*(storageDriver==='vfs'?2:1));assert(['vfs','overlay2','overlayfs'].includes(storageDriver));
 assert(Date.parse(lease.createdAt)<=Date.parse(lease.updatedAt)&&Date.parse(lease.updatedAt)<=now);
 return lease;
}
function environment(values){
 assert(Array.isArray(values));const result={};
 for(const value of values){const i=value.indexOf('=');assert(i>0);const key=value.slice(0,i);assert(!(key in result));result[key]=value.slice(i+1);}
 return result;
}
function ordinaryDescendant(root,path){
 assert.equal(realpathSync(root),resolve(root));assert(path.startsWith(root+sep));assert.equal(realpathSync(path),path);
 let next=root;for(const part of path.slice(root.length+1).split(sep)){next=join(next,part);const s=lstatSync(next);assert(s.isDirectory()&&!s.isSymbolicLink());}
}
export function validateCheckCreate(input,options){
 const {name,fixture,primary,check,run,commit,sourceDigest,imageID,uid,gid}=options;
 assertRecordedCheck(check,options);
 const lease=assertCheckLease(options.ledger,options),identity=lease.identity;
 assert.equal(name,'luci-check-'+identity.allocationId);assert.match(imageID,/^sha256:[a-f0-9]{64}$/);assert.equal(input.Image,imageID);
 assert.equal(input.User,uid+':'+gid);assert.equal(input.WorkingDir,'/workspace');
 assert.deepEqual(input.Entrypoint,['node']);assert.deepEqual(input.Cmd,['verify.mjs','final']);
 assert.equal(input.Tty??false,false);assert.equal(input.OpenStdin??false,false);assert.equal(input.StdinOnce??false,false);
 assert.equal(input.AttachStdin??false,false);assert.equal(input.AttachStdout,true);assert.equal(input.AttachStderr,true);
 assert.equal(input.NetworkDisabled??false,false);
 const labels={'luci.check':'true','luci.check-protocol':'1','luci.instance':identity.instanceId,'luci.check-engine':identity.engineId,'luci.check-allocation':identity.allocationId,'luci.check-id':identity.checkId,'luci.check-command-index':'0','luci.check-scope':identity.scopeDigest,'luci.check-project':identity.projectId,'luci.check-command-digest':identity.commandDigest,'luci.check-source-digest':sourceDigest,'luci.check-image':imageID};
 assert.deepEqual(input.Labels,labels);
 assert.deepEqual(environment(input.Env),checkEnv);
 assert.equal(Object.keys(input.Volumes??{}).length,0);
 assert.deepEqual(input.ExposedPorts??{},{});
 const h=input.HostConfig;assert(h);assert.equal(h.AutoRemove,false);assert.equal(h.Init,true);assert.equal(h.ReadonlyRootfs,true);
 assert.equal(h.Privileged??false,false);assert.equal(h.Memory,1024**3);assert.equal(h.NanoCpus,1e9);assert.equal(h.PidsLimit,256);
 assert.equal(h.MemorySwap,h.Memory);
 assert.deepEqual(h.CapDrop,['ALL']);assert.equal(h.CapAdd?.length??0,0);
 assert.deepEqual(h.SecurityOpt,['no-new-privileges:true']);
 assert.deepEqual(h.Tmpfs,{'/tmp':'rw,nosuid,nodev,size=1g'});
 assert(['','no'].includes(h.RestartPolicy?.Name??''));assert.equal(h.RestartPolicy?.MaximumRetryCount??0,0);
 assert.equal(Object.keys(h.PortBindings??{}).length,0);assert.equal(h.PublishAllPorts??false,false);
 for(const key of ['Binds','VolumesFrom','Devices','DeviceRequests','DeviceCgroupRules','Links','GroupAdd','ExtraHosts','Ulimits','Dns','DnsOptions','DnsSearch'])assert.equal(h[key]?.length??0,0);
 for(const key of ['PidMode','UTSMode','UsernsMode','CgroupnsMode','CgroupParent','Runtime'])assert.equal(h[key]??'','');
 assert.equal(h.NetworkMode??'','');assert.equal(h.IpcMode??'','');
 for(const key of ['StorageOpt','Sysctls'])assert.equal(Object.keys(h[key]??{}).length,0);
 for(const key of ['CpuShares','CpuPeriod','CpuQuota','CpuRealtimePeriod','CpuRealtimeRuntime','MemoryReservation','BlkioWeight'])assert.equal(h[key]??0,0);
 assert.equal(h.LogConfig?.Type??'','');assert.equal(Object.keys(h.LogConfig?.Config??{}).length,0);
 assert.deepEqual(input.Healthcheck?.Test,['NONE']);for(const key of ['Interval','Timeout','StartPeriod','StartInterval','Retries'])assert.equal(input.Healthcheck?.[key]??0,0);assert.equal(input.Hostname??'','');assert.equal(input.Domainname??'','');
 assert.equal(h.CpusetCpus??'','');assert.equal(h.CpusetMems??'','');
 ordinaryDescendant(fixture,primary);assert(primary.startsWith(options.projectRoot+sep));
 assert.equal(h.Mounts?.length,1);const m=h.Mounts[0];
 assert.equal(m.Type,'bind');assert.equal(m.Source,primary);assert.equal(m.Target,'/workspace');assert.equal(m.ReadOnly??false,false);
 assert.equal(m.Consistency??'','');assert.equal(Object.keys(m.VolumeOptions??{}).length,0);assert.equal(Object.keys(m.TmpfsOptions??{}).length,0);
 assert(['','rprivate'].includes(m.BindOptions?.Propagation??''));
 assert(Object.keys(m.BindOptions??{}).every(key=>key==='Propagation'),'Unexpected writable mount behavior');
 return {checkId:check.id,allocationId:identity.allocationId,name,projectId:options.project.id,commit,sourceDigest,labels,engineId:identity.engineId,instanceId:identity.instanceId,scopeDigest:identity.scopeDigest,commandDigest:identity.commandDigest,createDiskBytes:lease.createDiskBytes};
}

