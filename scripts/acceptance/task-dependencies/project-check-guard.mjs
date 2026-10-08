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

export function assertCheckLease(ledger,{check,primary,sourceDigest,uid,gid,scopeDigest,actor,engineID,instance,imageVirtualSize,storageDriver,now=Date.now()}){
 assert.equal(ledger.version,1);assert(Array.isArray(ledger.leases));assert.equal(ledger.leases.length,0,'Every worker reservation must already be released');
 assert.match(scopeDigest,/^[a-f0-9]{64}$/);assert.match(instance,/^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$/);
 assert.equal(typeof engineID,'string');assert(engineID.length>0&&engineID.length<=200&&!/[\x00\r\n]/.test(engineID));
 assert.match(check.commands[0].allocationId,/^checkexec-[a-f0-9]{24}$/);
 const identity={protocol:1,engineId:engineID,instanceId:instance,allocationId:check.commands[0].allocationId,checkId:check.id,commandIndex:0,scopeDigest,projectId:check.projectId,commandDigest:''};
 identity.commandDigest=checkCommandDigest({identity,primary,sourceDigest,uid,gid});
 const active=(ledger.checks??[]).filter(row=>row.state!=='finished');assert.equal(active.length,1,'Only this exact check may hold fixture capacity');
 const lease=active[0];assert.equal(lease.id,identity.allocationId);assert.equal(lease.scope,scopeDigest);assert.deepEqual(lease.identity,identity);assert.deepEqual(lease.actor,actor);assert.equal(actor.projectId,check.projectId);
 assert.deepEqual(lease.resources,{CPUMillis:1000,MemoryMiB:1024});assert.equal(lease.state,'creating');assert.equal(lease.ref??'','');assert.equal(lease.resultDigest??'','');
 assert(['vfs','overlay2','overlayfs'].includes(storageDriver));assert.equal(lease.createDiskBytes,positive(imageVirtualSize)*(storageDriver==='vfs'?2:1));
 const created=Date.parse(lease.createdAt),updated=Date.parse(lease.updatedAt);assert(Number.isFinite(created)&&Number.isFinite(updated)&&created<=updated&&updated<=now);
 assert.equal(check.resourcesHeld,true);assert.equal(check.commands[0].status,'running');
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
export function assertRecordedCheck(row,{run,commit,sourceDigest},terminal=false){
 assert.match(row.id,/^check-[a-f0-9]{24}$/);assert.equal(row.projectId,run.projectId);
 assert.equal(row.execution,'docker');assert.equal(row.memoryMiB,1024);assert.equal(row.cpuMillis,1000);
 assert.equal(row.commit,commit);assert.equal(row.sourceDigest,sourceDigest);assert.match(sourceDigest,/^[a-f0-9]{64}$/);
 assert.equal(row.dirty,false);assert.equal(row.sourceChanged,false);
 assert.deepEqual(row.requirementIds,['AC-dag']);assert.deepEqual(row.requirementSnapshots,run.input.requirements);
 assert.deepEqual(row.approvalSelection,run.input.approvals);
 assert.equal(row.specificationRevision,run.input.approvals.specificationRevision);assert.equal(row.specificationDigest,run.input.approvals.specificationDigest);
 assert.equal(row.designPlanRevision,run.input.approvals.designRevision);assert.equal(row.designPlanDigest,run.input.approvals.designDigest);
 assert.equal(row.commands.length,1);const result=row.commands[0];
 assert(Number.isSafeInteger(row.revision)&&row.revision>=1);assert.match(result.allocationId,/^checkexec-[a-f0-9]{24}$/);
 assert.deepEqual({label:result.label,executable:result.executable,args:result.args},checkCommand);
 assert.equal(run.swarm.tasks.length,3);assert(run.swarm.tasks.every(task=>task.integration?.state==='integrated'));
 if(terminal){
  assert.equal(row.status,'passed');assert.equal(row.evidenceStatus,'linked-test-evidence');assert.deepEqual(row.staleReasons,[]);
  assert.equal(result.status,'passed');assert.equal(result.exitCode,0);assert.equal(result.truncated,false);
  assert.equal(result.output,'DAG_CHECK_OK final\n');
  assert.equal(result.cleanupConfirmed,true);assert.equal(row.resourcesHeld,false);
 }else assert(['queued','running'].includes(row.status));
}
export function validateCheckCreate(input,options){
 const {name,fixture,primary,check,run,commit,sourceDigest,imageID,uid,gid}=options;
 assertRecordedCheck(check,{run,commit,sourceDigest});
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
 ordinaryDescendant(fixture,primary);assert(primary.startsWith(join(fixture,'projects')+sep));
 assert.equal(h.Mounts?.length,1);const m=h.Mounts[0];
 assert.equal(m.Type,'bind');assert.equal(m.Source,primary);assert.equal(m.Target,'/workspace');assert.equal(m.ReadOnly??false,false);
 assert.equal(m.Consistency??'','');assert.equal(Object.keys(m.VolumeOptions??{}).length,0);assert.equal(Object.keys(m.TmpfsOptions??{}).length,0);
 assert(['','rprivate'].includes(m.BindOptions?.Propagation??''));
 assert(Object.keys(m.BindOptions??{}).every(key=>key==='Propagation'),'Unexpected writable mount behavior');
 return {checkId:check.id,allocationId:identity.allocationId,name,projectId:run.projectId,runId:run.id,commit,sourceDigest,labels,engineId:identity.engineId,instanceId:identity.instanceId,scopeDigest:identity.scopeDigest,commandDigest:identity.commandDigest,createDiskBytes:lease.createDiskBytes};
}

export function assertCheckCapacity(view,{now,rootFreeBytes,engineID,engineInfo,daemonOnRootFilesystem,containerCount,imageVirtualSize,ledger,checkIdentity=null}){
 assert.equal(containerCount,0);assert.equal(daemonOnRootFilesystem,true);assert.equal(engineInfo.ID,engineID);
 assert.equal(engineInfo.MemoryLimit,true);assert.equal(engineInfo.SwapLimit,true);assert.equal(engineInfo.CpuCfsPeriod,true);assert.equal(engineInfo.CpuCfsQuota,true);assert.equal(engineInfo.PidsLimit,true);
 assert.equal(view.policy.maxActive,1);assert.equal(view.policy.maxPerProject,1);
 assert.equal(view.policy.headroomPercent,25);assert.equal(view.policy.minHeadroomMiB,2048);assert.equal(view.policy.diskReserveMiB,1024);
 const {host,execution,reservations}=view.resources;for(const row of [host,execution]){assert(['ready','partial'].includes(row.status));fresh(row.observedAt,now);}
 assert.equal(ledger.version,1);assert(Array.isArray(ledger.leases));assert.equal(ledger.leases.length,0,'No pending worker may be omitted from capacity accounting');
 const active=(ledger.checks??[]).filter(row=>row.state!=='finished');const expectedCount=checkIdentity?1:0;assert.equal(active.length,expectedCount,'Unexpected check reservations must not be ignored');
 if(checkIdentity){const lease=active[0];assert.equal(lease.id,checkIdentity.allocationId);assert.equal(lease.state,'creating');assert.equal(lease.ref??'','');assert.equal(lease.identity.engineId,engineID);assert.equal(lease.identity.scopeDigest,checkIdentity.scopeDigest);assert.equal(lease.identity.commandDigest,checkIdentity.commandDigest);assert.equal(lease.createDiskBytes,checkIdentity.createDiskBytes);assert.deepEqual(lease.resources,{CPUMillis:1000,MemoryMiB:1024});}
 assert(reservations);assert.equal(reservations.count,expectedCount);assert.equal(reservations.memoryMiB,expectedCount*1024);assert.equal(reservations.cpuMillis,expectedCount*1000);
 if(checkIdentity){fresh(reservations.observedAt,now);assert(Date.parse(reservations.observedAt)>=Date.parse(active[0].updatedAt),'Reservation summary predates this exact create claim');}
 const total=positive(host.memory.effectiveTotalBytes),available=positive(host.memory.effectiveAvailableBytes),daemon=positive(execution.totalMemoryBytes);
 assert(available>=Math.max(2048*1024**2,total*.25)+1024**3);
 assert(daemon>=Math.max(2048*1024**2,daemon*.25)+1024**3);
 assert(Math.min(positive(host.cpu.effectiveMillis),positive(execution.cpuMillis))*.75>=1000);
 if(host.cpu.utilizationPercent!==null&&host.cpu.utilizationPercent!==undefined)assert(host.cpu.utilizationPercent<=75);
 assert(positive(rootFreeBytes)>=6*1024**3);assert(positive(imageVirtualSize)*2+1024**3<=rootFreeBytes,'Reserve the conservative VFS two-copy demand separately from the running floor');
 assert(['vfs','overlay2','overlayfs'].includes(engineInfo.Driver),'Unknown storage driver is not an accepted check estimate');
 return {at:new Date(now).toISOString(),hostObservedAt:host.observedAt,executionObservedAt:execution.observedAt,hostAvailableBytes:available,hostTotalBytes:total,executionTotalBytes:daemon,rootFreeBytes,imageVirtualSizeBytes:imageVirtualSize,estimatedCreateDiskBytes:imageVirtualSize*2,storageDriver:engineInfo.Driver,containerCount,reservations:expectedCount,selfReservationCountedOnce:!!checkIdentity,scope:'Fresh acceptance guard cross-checks the exact production creating lease; installed mixed-account load and responsiveness remain separate gates'};
}

export function assertJoinedCheck(run,check){
 assert.equal(run.state,'awaiting-release');assert.equal(run.checks.length,1);
 const evidence=run.checks[0];assert.equal(evidence.checkId,check.id);assert.equal(evidence.projectId,run.projectId);
 assert.equal(evidence.commit,check.commit);assert.equal(evidence.sourceDigest,check.sourceDigest);assert.equal(evidence.execution,'docker');
 assert.deepEqual(evidence.requirements,run.input.requirements);assert.deepEqual(evidence.approvals,run.input.approvals);
 assert.deepEqual(evidence.commands,[{...checkCommand,status:'passed',exitCode:0}]);assert.equal(evidence.originalAvailable,true);
}

// Only an acknowledged fixture check may be inspected/stopped/removed on failure.
// The caller must already have stopped its controllers and drained proxy requests.
export function assertOwnedCheckContainer(value,{creation,identity,fixture,primary,imageID}){
 assert.equal(creation.kind,'check');assert.equal(creation.status,201);assert.match(creation.id,/^[a-f0-9]{64}$/);
 assert.equal(creation.name,identity.name);assert.deepEqual(creation.labels,identity.labels);
 assert.equal(value.Id,creation.id);assert.equal(value.Name,'/'+identity.name);assert.equal(value.Image,imageID);
 assert.deepEqual(value.Config.Labels,identity.labels);assert.equal(value.Config.Image,imageID);
 assert.deepEqual(value.Config.Entrypoint,['node']);assert.deepEqual(value.Config.Cmd,['verify.mjs','final']);assert.equal(value.Config.WorkingDir,'/workspace');
 ordinaryDescendant(fixture,primary);assert.equal(value.Mounts.length,1);const mount=value.Mounts[0];
 assert.equal(mount.Type,'bind');assert.equal(mount.Source,primary);assert.equal(mount.Destination,'/workspace');assert.equal(mount.RW,true);
 assert.equal(value.HostConfig.Memory,1024**3);assert.equal(value.HostConfig.NanoCpus,1e9);assert.equal(value.HostConfig.PidsLimit,256);
 assert.equal(value.HostConfig.MemorySwap,value.HostConfig.Memory);assert.equal(value.HostConfig.AutoRemove,false);assert.equal(value.HostConfig.ReadonlyRootfs,true);assert.equal(value.HostConfig.Privileged??false,false);
 assert.deepEqual(value.HostConfig.CapDrop,['ALL']);assert.equal(value.HostConfig.CapAdd?.length??0,0);assert.deepEqual(value.HostConfig.SecurityOpt,['no-new-privileges:true']);assert.deepEqual(value.Config.Healthcheck?.Test,['NONE']);
 assert.deepEqual(value.HostConfig.Tmpfs,{'/tmp':'rw,nosuid,nodev,size=1g'});
}
