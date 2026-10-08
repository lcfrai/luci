// Fixture admission guard. Pure validation plus read-only filesystem identity;
// importing this file never connects to Docker or starts a process.
import assert from 'node:assert/strict';
import { lstatSync, realpathSync } from 'node:fs';
import { isAbsolute, join, resolve, sep } from 'node:path';

export function workerBinds(input) {
 assert(Array.isArray(input.HostConfig?.Binds));
 return input.HostConfig.Binds.map(value=>{
  assert.equal(typeof value,'string');const parts=value.split(':');assert(parts.length===2||parts.length===3);assert(parts.length===2||parts[2]==='ro');
  return {Type:'bind',Source:parts[0],Target:parts[1],ReadOnly:parts[2]==='ro'};
 });
}
export function validateWorkerCreate(input,{fixture,projectID,runID,operationID,session,images,instance,uid,gid,taskContextVersion=1}) {
 assert(taskContextVersion===1||taskContextVersion===2,'Unsupported trusted task context version');
 const host=input.HostConfig;assert(host);assert(images.includes(input.Image));assert.equal(input.Labels?.['luci.managed'],'true');assert.equal(input.Labels?.['luci.instance'],instance);assert.equal(input.Labels?.['luci.session-id'],session.id);assert.equal(input.Labels?.['luci.sandbox-id'],session.id);assert.equal(session.projectId,projectID);assert.equal(session.harness,'codex');
 for(const value of [projectID,runID,operationID,session.id,session.worktreeId,session.conversationId])assert.match(value??'',/^[a-zA-Z0-9_-]{1,200}$/);
 assert.equal(input.User,'0:0');assert.equal(input.WorkingDir,'/home/dev/work');assert.equal(input.Tty??false,false);assert.equal(input.OpenStdin??false,false);assert.equal(input.Entrypoint?.length??0,0);assert.equal(input.Cmd?.length??0,0);assert.equal(Object.keys(input.Volumes??{}).length,0);
 assert(Array.isArray(input.Env));assert.equal(new Set(input.Env.map(value=>value.split('=',1)[0])).size,input.Env.length);assert.equal(input.Env.filter(value=>value==='LUCI_HOST_UID='+uid).length,1);assert.equal(input.Env.filter(value=>value==='LUCI_HOST_GID='+gid).length,1);assert(!input.Env.some(value=>/^(LD_|GCONV_PATH=|GLIBC_TUNABLES=)/.test(value)));
 assert.equal(host.Privileged??false,false);assert.equal(host.AutoRemove??false,false);assert.equal(host.ReadonlyRootfs??false,false);assert.equal(host.RestartPolicy?.Name,'no');assert.equal(host.RestartPolicy?.MaximumRetryCount??0,0);assert.equal(host.PidsLimit,256);assert.equal(host.Memory,1024**3);assert.equal(host.NanoCpus,1000000000);
 // Docker SDK v28.3.3 normalizes and sorts capabilities before HTTP Create.
 // Match only that exact wire form, preserving the same five bootstrap rights.
 assert.deepEqual(host.CapDrop,['ALL']);assert.deepEqual(host.CapAdd,['CAP_CHOWN','CAP_DAC_OVERRIDE','CAP_SETGID','CAP_SETPCAP','CAP_SETUID']);assert.deepEqual(host.SecurityOpt,['no-new-privileges:true']);assert.deepEqual(host.ExtraHosts,['host.docker.internal:host-gateway']);
 for(const key of ['Mounts','VolumesFrom','Devices','DeviceRequests','DeviceCgroupRules','Links','GroupAdd'])assert.equal(host[key]?.length??0,0);
 for(const key of ['NetworkMode','IpcMode','PidMode','UTSMode','UsernsMode','CgroupnsMode','CgroupParent','Runtime'])assert.equal(host[key]??'','');
 for(const key of ['Tmpfs','StorageOpt','Sysctls'])assert.equal(Object.keys(host[key]??{}).length,0);
 const ports=Object.entries(host.PortBindings??{});assert.equal(ports.length,1);assert.match(ports[0][0],/^\d{1,5}\/tcp$/);assert.deepEqual(ports[0][1],[{HostIp:'127.0.0.1',HostPort:''}]);
 const root=realpathSync(fixture);assert.equal(root,resolve(fixture));
 const expected=new Map([
  ['/home/dev/work',{path:join(root,'projects','.luci-swarms',projectID,runID,operationID),readOnly:false,required:true}],
  ['/home/dev/work/.git/worktrees',{path:join(root,'projects','.luci-swarms',projectID,runID,operationID,'.git','worktrees'),readOnly:true,required:true}],
  ['/home/dev/.luci-reviewed-context',{path:join(root,'state','conversations','local','.reviewed-context',session.id),readOnly:true,required:true}],
  ['/home/dev/.luci-agent',{path:join(root,'state','conversations','native','local',session.worktreeId,'codex',session.conversationId),readOnly:false,required:true}],
  ['/home/dev/.claude/projects',{path:join(root,'state','conversations','local',session.worktreeId),readOnly:false,required:false}],
 ]);
 if(taskContextVersion===2)expected.set('/home/dev/.luci-task-output',{path:join(root,'state','conversations','local','.task-output',session.id),readOnly:false,required:true});
 const binds=workerBinds(input),seen=new Set(),required=taskContextVersion===2?5:4;assert(binds.length>=required&&binds.length<=required+1);
 for(const bind of binds){
  assert(!seen.has(bind.Target));seen.add(bind.Target);const allowed=expected.get(bind.Target);assert(allowed,'Unexpected fixture mount destination');assert.equal(bind.ReadOnly,allowed.readOnly);assert(isAbsolute(bind.Source));assert.equal(bind.Source,allowed.path);assert.equal(realpathSync(bind.Source),allowed.path);assert(lstatSync(bind.Source).isDirectory());
  // Every ancestor through the unique fixture root must be an ordinary directory;
  // same-target symlink aliases are refused as well as escaping symlinks.
  let ancestor=root;for(const part of bind.Source.slice(root.length+1).split(sep)){ancestor=join(ancestor,part);const entry=lstatSync(ancestor);assert(entry.isDirectory()&&!entry.isSymbolicLink());}
 }
 for(const [target,allowed] of expected)if(allowed.required)assert(seen.has(target),'Required fixture mount missing');
 return binds;
}
