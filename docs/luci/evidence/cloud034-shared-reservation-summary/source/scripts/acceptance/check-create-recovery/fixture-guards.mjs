import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
export const sha = bytes => createHash('sha256').update(bytes).digest('hex');
export function identityLabels(i) {
  return {'luci.check':'true','luci.check-protocol':String(i.protocol),'luci.instance':i.instanceId,
    'luci.check-engine':i.engineId,'luci.check-allocation':i.allocationId,'luci.check-id':i.checkId,
    'luci.check-command-index':String(i.commandIndex),'luci.check-scope':i.scopeDigest,
    'luci.check-project':i.projectId,'luci.check-command-digest':i.commandDigest};
}
export function validateFence(input,{record,name,inspected=false}) {
  assert(['intent','proved','retired'].includes(record.state));
  const cfg=inspected?input.Config:input,h=input.HostConfig;
  const binding=sha(JSON.stringify({Identity:record.identity,IntentID:record.intentId,ExpectedOriginalRef:''}));
  const labels={...identityLabels(record.identity),'luci.launch-fence':'1','luci.fence-intent':record.intentId,'luci.fence-binding':binding};
  assert.equal(name,'luci-check-'+record.identity.allocationId);assert.deepEqual(cfg.Labels,labels);
  assert.equal(cfg.Image??'','');assert.deepEqual(cfg.Cmd,['/']);assert.equal(cfg.NetworkDisabled,true);
  for(const k of ['Env','Entrypoint'])assert.equal(cfg[k]?.length??0,0);
  for(const k of ['WorkingDir','User'])assert.equal(cfg[k]??'','');
  for(const k of ['ExposedPorts','Volumes'])assert.equal(Object.keys(cfg[k]??{}).length,0);
  assert(!cfg.Healthcheck);assert.equal(cfg.Tty??false,false);assert.equal(cfg.OpenStdin??false,false);
  assert.equal(h.ReadonlyRootfs,true);assert.equal(h.NetworkMode,'none');assert.equal(h.IpcMode,'none');
  assert.equal(h.Privileged??false,false);assert.equal(h.AutoRemove??false,false);
  assert.equal(h.RestartPolicy?.Name,'no');assert.equal(h.RestartPolicy?.MaximumRetryCount??0,0);
  for(const k of ['Binds','Mounts','VolumesFrom','Devices','DeviceRequests','Links','ExtraHosts','CapAdd'])assert.equal(h[k]?.length??0,0);
  for(const k of ['PidMode','UTSMode'])assert.equal(h[k]??'','');assert.equal(Object.keys(h.PortBindings??{}).length,0);
  assert.deepEqual(h.CapDrop,['ALL']);assert.deepEqual(h.SecurityOpt,['no-new-privileges:true']);
  assert.equal(h.LogConfig?.Type,'none');assert.equal(h.Memory,6*1024**2);assert.equal(h.NanoCpus,100000000);assert.equal(h.PidsLimit,1);
  if(inspected){
    assert.equal(input.Id,record.proof.fenceRef);assert.equal(input.Name,'/'+name);assert.equal(input.Image,'');assert.deepEqual(input.Mounts,[]);
    assert.equal(input.State.Status,'created');assert.equal(input.State.Running,false);assert.equal(input.State.Pid,0);
    for(const k of ['Restarting','Paused','Dead'])assert.equal(input.State[k]??false,false);
    assert.equal(input.Path,'/');assert.equal(input.Args?.length??0,0);
  }
  return labels;
}
export function assertComputeEnvelope(view,{memoryMiB,cpuMillis}) {
  const {host,execution}=view.resources;
  const total=host.memory.effectiveTotalBytes,available=host.memory.effectiveAvailableBytes;
  assert(Number.isSafeInteger(total)&&total>0&&Number.isSafeInteger(available)&&available>=0);
  assert(Number.isSafeInteger(memoryMiB)&&memoryMiB>=0&&Number.isSafeInteger(cpuMillis)&&cpuMillis>=0);
  assert(available>=Math.max(2048*1024**2,total*.25)+memoryMiB*1024**2);
  assert(execution.totalMemoryBytes>=Math.max(2048*1024**2,execution.totalMemoryBytes*.25)+memoryMiB*1024**2);
  assert(Math.min(host.cpu.effectiveMillis,execution.cpuMillis)*.75>=cpuMillis);
}
export function assertOriginalInspect(input,{id,name,imageID,guardedCreate,imageEnv=[]}) {
  const cfg=input.Config,h=input.HostConfig,expected=guardedCreate.HostConfig;
  assert.equal(input.Id,id);assert.equal(input.Name,'/'+name);assert.equal(input.Image,imageID);assert.equal(cfg.Image,imageID);
  assert.deepEqual(cfg.Labels,guardedCreate.Labels);assert.equal(cfg.User,guardedCreate.User);assert.equal(cfg.WorkingDir,'/workspace');
  assert.deepEqual(cfg.Entrypoint,['node']);assert.deepEqual(cfg.Cmd,['verify.mjs','final']);
  const env=values=>{const result=new Map();for(const value of values){const offset=value.indexOf('=');assert(offset>0);const key=value.slice(0,offset);assert(!result.has(key));result.set(key,value.slice(offset+1));}return Object.fromEntries(result);};
  assert.deepEqual(env(cfg.Env),{...env(imageEnv),...env(guardedCreate.Env)});
  for(const key of ['Tty','OpenStdin','StdinOnce','AttachStdin','NetworkDisabled'])assert.equal(cfg[key]??false,false);
  assert.equal(cfg.AttachStdout,true);assert.equal(cfg.AttachStderr,true);assert.deepEqual(cfg.Healthcheck?.Test,['NONE']);
  for(const key of ['ExposedPorts','Volumes'])assert.equal(Object.keys(cfg[key]??{}).length,0);
  for(const key of ['AutoRemove','Init','ReadonlyRootfs','Memory','MemorySwap','NanoCpus','PidsLimit','CapDrop','SecurityOpt','Tmpfs'])assert.deepEqual(h[key],expected[key]);
  assert.equal(h.Privileged??false,false);assert.equal(h.PublishAllPorts??false,false);assert.equal(h.RestartPolicy?.Name,'no');assert.equal(h.RestartPolicy?.MaximumRetryCount??0,0);
  for(const key of ['Binds','VolumesFrom','Devices','DeviceRequests','DeviceCgroupRules','Links','GroupAdd','ExtraHosts','Ulimits','Dns','DnsOptions','DnsSearch','CapAdd'])assert.equal(h[key]?.length??0,0);
  for(const key of ['PidMode','UTSMode','UsernsMode','CgroupParent','CpusetCpus','CpusetMems'])assert.equal(h[key]??'','');
  for(const key of ['PortBindings','StorageOpt','Sysctls'])assert.equal(Object.keys(h[key]??{}).length,0);
  for(const key of ['CpuShares','CpuPeriod','CpuQuota','CpuRealtimePeriod','CpuRealtimeRuntime','MemoryReservation','BlkioWeight'])assert.equal(h[key]??0,0);
  assert(['default','bridge',''].includes(h.NetworkMode??''));assert(['private',''].includes(h.IpcMode??''));
  assert.deepEqual(h.Mounts,expected.Mounts);assert.equal(input.Mounts.length,1);const mount=input.Mounts[0];
  assert.equal(mount.Type,'bind');assert.equal(mount.Source,expected.Mounts[0].Source);assert.equal(mount.Destination,'/workspace');assert.equal(mount.RW,true);assert.equal(mount.Propagation,'rprivate');
  assert.equal(input.State.Status,'created');assert.equal(input.State.Running,false);assert.equal(input.State.Pid,0);
  for(const key of ['Restarting','Paused','Dead'])assert.equal(input.State[key]??false,false);
  assert.equal(input.Path,'node');assert.deepEqual(input.Args,['verify.mjs','final']);
}
export function assertCapacity(view,{ledger,engine,rootFreeBytes,imageSize,now=Date.now(),requireCreate=false,requireReservationMatch=true}) {
  assert.equal(ledger.version,1);assert.deepEqual(ledger.leases,[]);
  const active=(ledger.checks??[]).filter(r=>r.state!=='finished');assert(active.length<=2);
  for(const lease of active){
    assert.deepEqual(lease.resources,{CPUMillis:1000,MemoryMiB:1024});
    assert(Number.isSafeInteger(lease.createDiskBytes)&&lease.createDiskBytes>0);
    assert.equal(lease.createDiskBytes,imageSize*(engine.Driver==='vfs'?2:1));
  }
  const expected={count:active.length,memoryMiB:active.reduce((n,r)=>n+r.resources.MemoryMiB,0),cpuMillis:active.reduce((n,r)=>n+r.resources.CPUMillis,0)};
  const {host,execution,reservations}=view.resources;
  for(const row of [host,execution]){assert(['ready','partial'].includes(row.status));const age=now-Date.parse(row.observedAt);assert(age>=0&&age<=5000);}
  if(requireReservationMatch){
    for(const [k,v] of Object.entries(expected))assert.equal(reservations[k],v);
    const observed=Date.parse(reservations.observedAt),age=now-observed;
    // Reservation observedAt is this API process's last ledger mutation, not
    // this GET's sample time. Another team process can leave this cache stale.
    assert(Number.isFinite(age)&&age>=0);
    for(const lease of active)assert(observed>=Date.parse(lease.updatedAt),'Reservation observation predates the exact active claim');
  }
  assert.equal(view.policy.headroomPercent,25);assert.equal(view.policy.minHeadroomMiB,2048);assert.equal(view.policy.diskReserveMiB,1024);
  for(const k of ['MemoryLimit','SwapLimit','CpuCfsPeriod','CpuCfsQuota','PidsLimit'])assert.equal(engine[k],true);
  assert(rootFreeBytes>=1024**3);
  if(requireCreate){
    assertComputeEnvelope(view,expected);
    assert(['vfs','overlay2','overlayfs'].includes(engine.Driver));
    const pendingDisk=active.reduce((n,r)=>n+r.createDiskBytes,0);
    assert(rootFreeBytes>=6*1024**3);assert(rootFreeBytes>=pendingDisk+1024**3);
    assert(imageSize>0);
  }
  return expected;
}

// Explicit current-product visibility gate. This observes public totals only;
// exact charge ownership/release still comes from the complete private ledger.
export function assertPublicReservationTotals(reservations,expected,{now=Date.now(),maxAgeMs=5000}={}) {
  assert.equal(reservations?.status,'ready','Public reservation totals are unavailable or lack the reviewed freshness contract');
  assert(Number.isSafeInteger(maxAgeMs)&&maxAgeMs>0&&maxAgeMs<=5000);
  const age=now-Date.parse(reservations.observedAt);assert(Number.isFinite(age)&&age>=0&&age<=maxAgeMs,'Public reservation observation is stale or invalid');
  for(const key of ['count','memoryMiB','cpuMillis']){
    assert(Number.isSafeInteger(expected[key])&&expected[key]>=0);
    assert(Number.isSafeInteger(reservations[key])&&reservations[key]>=0,'Public reservation numeric total is invalid');
    assert.equal(reservations[key],expected[key],'Public reservation total disagrees with exact ledger charge');
  }
  return {status:'ready',observedAt:reservations.observedAt,count:reservations.count,memoryMiB:reservations.memoryMiB,cpuMillis:reservations.cpuMillis};
}
