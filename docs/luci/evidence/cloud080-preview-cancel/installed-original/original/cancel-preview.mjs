// PREPARED ONLY. Running this file explicitly starts an installed controller
// and one real Docker build through Product APIs. No runtime occurred in prep.
import assert from 'node:assert/strict';
import {spawn,spawnSync} from 'node:child_process';
import {createHash,randomUUID} from 'node:crypto';
import {appendFileSync,copyFileSync,existsSync,lstatSync,mkdirSync,readFileSync,readdirSync,realpathSync,statfsSync,writeFileSync} from 'node:fs';
import {createServer} from 'node:net';
import {dirname,isAbsolute,join,sep} from 'node:path';
import {fileURLToPath} from 'node:url';
import {performance} from 'node:perf_hooks';
import {setTimeout as delay} from 'node:timers/promises';

assert.equal(process.platform,'linux');assert.equal(Number(process.versions.node.split('.')[0]),24);
process.umask(0o077);
const helperDir=dirname(fileURLToPath(import.meta.url));
const cfg=JSON.parse(readFileSync(process.argv[2],'utf8'));
for(const key of ['repository','binary','provenanceFile','webDir','evidenceParent'])assert.ok(isAbsolute(cfg[key]));
for(const key of ['binarySHA256','provenanceSHA256','packageIdentity'])assert.match(cfg[key],/^[a-f0-9]{64}$/);
for(const key of ['sourceCommit','repositoryCommit'])assert.match(cfg[key],/^[a-f0-9]{40}$/);
assert.equal(cfg.mode,'live-preview-cancel');assert.equal(cfg.dockerHost,'unix:///var/run/docker.sock');
assert.equal(cfg.acknowledgeFixtureMutation,true);assert.equal(cfg.acknowledgeWarmBaseOnly,true);assert.equal(cfg.acknowledgeUnknownLeaseRetention,true);
assert.match(cfg.baseImageID,/^sha256:[a-f0-9]{64}$/);assert.ok(cfg.engineID&&cfg.engineDriver);
assert.equal(realpathSync(cfg.evidenceParent),cfg.evidenceParent);
const run=join(cfg.evidenceParent,`cancel-${Date.now()}-${randomUUID()}`);mkdirSync(run,{mode:0o700});
const write=(name,value)=>writeFileSync(join(run,name),typeof value==='string'||Buffer.isBuffer(value)?value:JSON.stringify(value,null,2)+'\n',{mode:0o600});
const sha=bytes=>createHash('sha256').update(bytes).digest('hex'),hash=path=>sha(readFileSync(path));
const result={passed:false,scenarioPassed:false,scope:'One handwritten real warm-base Docker RUN cancellation through installed HTTP APIs; no browser, model, cold pull, serving, Windows or full-product proof',startedAt:new Date().toISOString(),run,checks:[],cleanup:{manualDockerMutation:false,manualLedgerMutation:false,stopAssist:false},pendingMutations:[]};
let stage='preflight',firstFailure=null;
function safeWrite(name,value){try{write(name,value);return true;}catch(error){fail('evidence-retention',error);return false;}}
function record(name,details={}){const row={at:new Date().toISOString(),monotonicMs:performance.now(),stage,name,...details};assert.ok(result.checks.length<256);result.checks.push(row);appendFileSync(join(run,'events.jsonl'),JSON.stringify(row)+'\n',{mode:0o600});}
function fail(where,error){
 result.passed=false;process.exitCode=1;
 const raw=Buffer.from(String(error?.message??error)+'\n'+String(error?.stack??''));
 const projection={at:new Date().toISOString(),stage:where,messageStackBytes:raw.length,sha256:sha(raw),privateRetention:'not-attempted'};
 if(!firstFailure){
  firstFailure=projection;const bounded=raw.subarray(0,16<<10);projection.retainedBytes=bounded.length;projection.truncated=raw.length>bounded.length;projection.retainedSHA256=sha(bounded);
  try{writeFileSync(join(run,'first-failure.private.txt'),bounded,{mode:0o600,flag:'wx'});projection.privateRetention='retained';}catch{projection.privateRetention='failed';}
 }
 if(!result.cleanup.failures)result.cleanup.failures=[];
 if(result.cleanup.failures.length<16)result.cleanup.failures.push(projection);
}
const root=join(run,'private'),state=join(root,'state'),projects=join(root,'projects'),scratchTmp=join(root,'tmp');
for(const p of [root,state,projects,scratchTmp,join(root,'docker-config')])mkdirSync(p,{mode:0o700});
const baseEnv={PATH:process.env.PATH,LANG:'C.UTF-8',TZ:'UTC',TMPDIR:scratchTmp,DOCKER_HOST:cfg.dockerHost,DOCKER_CONFIG:join(root,'docker-config'),GIT_CONFIG_NOSYSTEM:'1',GIT_CONFIG_GLOBAL:join(root,'empty-git-config'),GIT_TERMINAL_PROMPT:'0',AWS_EC2_METADATA_DISABLED:'true'};
writeFileSync(baseEnv.GIT_CONFIG_GLOBAL,'',{mode:0o600});
const started=Date.now(),lifecycleDeadline=started+480_000,workDeadline=lifecycleDeadline-60_000;
const abort=new AbortController();
for(const signal of ['SIGINT','SIGTERM'])process.on(signal,()=>{result.signal=signal;abort.abort(new Error('Driver interrupted'));});
let child,childClosed=false,childError,processWatch,processScanError,logsBytes=0,logsTruncated=false;
let origin,token,project,release,instanceID,nonce,commit,sourceDigest,scopeDigest,buildLease,containerID;
let startTask,startOutcome,startAcknowledged=false,cancelSent=false,requestNumber=0,stageProof,policyBefore,finalOwnRows;
let observationsSealed=false;
const ownedProcesses=new Map(),pending=new Map();let commandNumber=0;const nonceInventorySamples=[];
const ledgerDir=cfg.sharedExecutionDir??join(state,'control-plane','execution');assert.ok(isAbsolute(ledgerDir));assert.ok(!ledgerDir.startsWith(projects+sep));
const ledgerFile=join(ledgerDir,'leases.json');
function fence(cleanup=false){if(!cleanup)abort.signal.throwIfAborted();assert.ok(Date.now()<(cleanup?lifecycleDeadline-3000:workDeadline),'Driver deadline reached');}
function timeout(budget,cleanup=false){fence(cleanup);return Math.max(1,Math.min(budget,(cleanup?lifecycleDeadline-3000:workDeadline)-Date.now()));}
function processIdentity(pid){try{const text=readFileSync(`/proc/${pid}/stat`,'utf8');const row=text.slice(text.lastIndexOf(')')+2).trim().split(/\s+/);return {pid:Number(pid),parentPID:Number(row[1]),startTicks:row[19],state:row[0]};}catch(e){if(e.code==='ENOENT'||e.code==='ESRCH')return null;throw e;}}
function captureProcesses(){
 if(!child?.pid)return;
 const names=readdirSync('/proc').filter(x=>/^\d+$/.test(x));assert.ok(names.length<10000);
 const rows=new Map(names.map(x=>[Number(x),processIdentity(x)]).filter(([,x])=>x));
 const selected=new Set(),actual=rows.get(child.pid),known=ownedProcesses.get(child.pid);
 if(actual&&(!known||known.startTicks===actual.startTicks)){selected.add(child.pid);ownedProcesses.set(child.pid,actual);}
 for(const [pid,old] of ownedProcesses)if(rows.get(pid)?.startTicks===old.startTicks)selected.add(pid);
 let changed=true;while(changed){changed=false;for(const [pid,row] of rows)if(!selected.has(pid)&&selected.has(row.parentPID)){selected.add(pid);ownedProcesses.set(pid,row);changed=true;}}
}
function signalController(signal){if(!child||childClosed)return;const old=ownedProcesses.get(child.pid),now=processIdentity(child.pid);if(!now)return;assert.ok(old&&old.startTicks===now.startTicks,'Controller process identity changed');child.kill(signal);}
const outerTimer=setTimeout(()=>{
 fail('outer-deadline',new Error('Bounded helper lifecycle expired; unknown Docker work remains retained'));
 try{signalController('SIGKILL');}catch(e){fail('outer-controller-kill',e);}
 result.firstFailure=firstFailure;result.pendingMutations=[...pending.values()];result.cleanup.outerDeadline=true;result.finishedAt=new Date().toISOString();
 try{write('result.json',result);}catch{}process.exit(124);
},480_000);outerTimer.unref();
function command(executable,args,budget=10_000,cleanup=false){
 const r=spawnSync(executable,args,{cwd:cfg.repository,env:baseEnv,encoding:'utf8',timeout:timeout(budget,cleanup),maxBuffer:2<<20,killSignal:'SIGKILL'});
 if(r.error||r.status!==0){
  const details={executable,args,status:r.status,signal:r.signal,error:r.error?.message??null,stdout:String(r.stdout??'').slice(0,4096),stderr:String(r.stderr??'').slice(0,16384)};
  try{write('command-failure-'+String(++commandNumber)+'.private.json',details);}catch{}
  throw Object.assign(new Error('Bounded read-only command failed; private command failure journal retained if writable'),{cause:r.error,commandStatus:r.status});
 }return r.stdout;
}
function docker(args,budget=5000,cleanup=false){
 const allowed=args[0]==='info'||args[0]==='inspect'||args[0]==='ps'||args[0]==='logs'||args[0]==='image'&&['inspect','ls'].includes(args[1])||args[0]==='volume'&&args[1]==='ls';
 assert.ok(allowed,'No Docker mutation is permitted in this helper');return command('docker',['--host',cfg.dockerHost,...args],budget,cleanup);
}
const dockerJSON=(args,budget,cleanup)=>JSON.parse(docker(args,budget,cleanup));
function boundedJSON(path,limit=2<<20){const stat=lstatSync(path);assert.ok(stat.isFile()&&!stat.isSymbolicLink()&&stat.size<=limit);const raw=readFileSync(path);assert.ok(raw.length<=limit);return JSON.parse(raw);}
function ownLedger(label){
 if(!existsSync(ledgerFile)){write(label+'-ledger.json',{exists:false});return [];}
 const raw=boundedJSON(ledgerFile),rows=(raw.previews??[]).filter(x=>x.identity?.projectId===project?.id);
 write(label+'-ledger.json',{rawSHA256:hash(ledgerFile),version:raw.version,ownPreviews:rows,counts:{workers:raw.leases?.length??0,checks:raw.checks?.length??0,runtimeImages:raw.runtimeImages?.length??0,previews:raw.previews?.length??0}});return rows;
}
function ownReceipt(label){
 const rows=boundedJSON(join(state,'sessions','releases',project.id+'.json'),8<<20);assert.equal(rows.length,1);const row=rows[0];assert.equal(row.id,release.id);assert.equal(row.projectId,project.id);assert.ok(row.runtime);write(label+'-release.json',row);return row;
}
const mutationCounts=new Map();
function allowRequest(method,path){
 assert.match(path,/^\/v1\/[a-zA-Z0-9/_-]+$/);assert.ok(!path.includes('//'));
 const global=new Map([['GET /v1/control-plane/status',true],['POST /v1/auth/bootstrap',true],['POST /v1/setup/git/identity',true],['POST /v1/product/projects',true],['GET /v1/product/swarm-scheduler',true]]);
 let allowed=global.has(method+' '+path);
 if(project){const base='/v1/product/projects/'+project.id;const reads=['/specification','/design-plan','/git','/releases'];const writes=['/brief','/specification','/specification/approve','/design-plan','/design-plan/approve','/git/commit','/releases'];allowed ||= method==='GET'&&reads.some(x=>path===base+x)||method==='POST'&&writes.some(x=>path===base+x);if(release)allowed ||= method==='POST'&&path===base+'/releases/'+release.id+'/cancel';}
 assert.ok(allowed,'Request is outside the exact owned workflow');
 if(method==='POST'){
  const key=method+' '+path,count=(mutationCounts.get(key)??0)+1;assert.equal(count,1,'No mutation replay is allowed');mutationCounts.set(key,count);
 }
}
async function readBounded(response,limit){
 const reader=response.body?.getReader();if(!reader)return Buffer.alloc(0);let bytes=0;const chunks=[];
 try{for(;;){const {done,value}=await reader.read();if(done)break;bytes+=value.length;if(bytes>limit){await reader.cancel();throw new Error('Response exceeds materialization bound');}chunks.push(value);}return Buffer.concat(chunks,bytes);}finally{reader.releaseLock();}
}
async function api(method,path,body,{expect=[200],budget=30_000,sensitive=false,cleanup=false}={}){
 fence(cleanup);allowRequest(method,path);assert.ok(++requestNumber<=400,'HTTP request journal bound exceeded');
 const id=String(requestNumber).padStart(3,'0'),requestAt=performance.now();const observation={id,method,path,startedAt:new Date().toISOString(),startedMonotonicMs:requestAt};
 if(method==='POST')pending.set(id,observation);
 write(id+'-request.json',{...observation,body:sensitive?'[bootstrap credentials omitted]':body??null});
 const response=await fetch(origin+path,{method,redirect:'error',signal:cleanup?AbortSignal.timeout(timeout(budget,true)):AbortSignal.any([abort.signal,AbortSignal.timeout(timeout(budget))]),headers:{...(body===undefined?{}:{'Content-Type':'application/json'}),...(token?{Authorization:'Bearer '+token}:{})},body:body===undefined?undefined:JSON.stringify(body)});
 assert.equal(observationsSealed,false,'HTTP observation arrived after final receipt seal');
 const bytes=await readBounded(response,2<<20),value=bytes.length?JSON.parse(bytes.toString('utf8')):null;assert.equal(observationsSealed,false,'HTTP body arrived after final receipt seal');
 const receipt={id,status:response.status,completedAt:new Date().toISOString(),completedMonotonicMs:performance.now(),elapsedMs:performance.now()-requestAt,bodyBytes:bytes.length,bodySHA256:sha(bytes),value:sensitive?{user:value?.user,tokenOmitted:true}:value};
 write(id+'-response.json',receipt);pending.delete(id);
 assert.ok(expect.includes(response.status),'HTTP status does not match the declared workflow contract');return {value,receipt};
}
async function until(label,fn,budget,cleanup=false){const end=Date.now()+timeout(budget,cleanup);let lastError;
 while(Date.now()<end){fence(cleanup);if(childError)throw childError;if(childClosed&&!cleanup)throw new Error('Controller closed during workflow');try{const value=await fn();if(value)return value;}catch(e){if(label!=='Controller readiness')throw e;lastError=e;}await delay(500);}
 throw new Error(label+' observation deadline expired',{cause:lastError});
}
async function availablePort(){const server=createServer();await new Promise((resolve,reject)=>server.once('error',reject).listen(0,'127.0.0.1',resolve));const port=server.address().port;await new Promise(resolve=>server.close(resolve));return port;}
function reservationIdentity(row){
 assert.equal(row.kind,'preview-build');assert.equal(row.state,'building');assert.equal(row.identity.projectId,project.id);assert.equal(row.identity.releaseId,release.id);assert.equal(row.identity.commit,commit);assert.equal(row.identity.engineId,cfg.engineID);assert.equal(row.identity.instanceId,instanceID);assert.equal(row.scope,row.identity.scopeDigest);assert.equal(row.id,row.identity.allocationId);assert.match(row.identity.sourceDigest,/^[a-f0-9]{64}$/);assert.equal(row.resources.MemoryMiB??row.resources.memoryMiB,1024);assert.equal(row.resources.CPUMillis??row.resources.cpuMillis,1000);return row;
}
function nonceContainers(cleanup=false){const text=docker(['ps','-a','--no-trunc','--filter','label=org.lcfr.fixture.cancel='+nonce,'--format','{{.ID}}'],5000,cleanup).trim();const ids=text?text.split('\n'):[];assert.ok(nonceInventorySamples.length<512,'Nonce observation journal bound exceeded');nonceInventorySamples.push({at:new Date().toISOString(),ids});write('nonce-container-observations.json',{scope:'Read-only fresh fixture nonce inventory, never cleanup authority',samples:nonceInventorySamples});assert.ok(ids.length<=1,'Fresh fixture nonce matched more than one container');for(const id of ids)assert.match(id,/^[a-f0-9]{64}$/);return ids;}
function verifyStageContainer(info){
 assert.match(info.Id,/^[a-f0-9]{64}$/);if(containerID)assert.equal(info.Id,containerID);const labels=info.Config.Labels;
 assert.equal(labels['org.lcfr.fixture.cancel'],nonce);assert.equal(labels['org.lcfr.fixture.project'],project.id);assert.equal(labels['org.lcfr.fixture.stage'],'cancel-run-v1');
 assert.deepEqual(info.Config.Cmd,['node','/cancel/hold.mjs']);assert.equal(info.State.Running,true);assert.equal(info.State.Pid>0,true);assert.equal(info.HostConfig.Memory,2**30);assert.equal(info.HostConfig.MemorySwap,2**30);assert.equal(info.HostConfig.CpuQuota,100000);assert.equal(info.HostConfig.CpuPeriod,100000);
 assert.equal(info.HostConfig.Privileged,false);assert.equal(info.Mounts.length,0);assert.ok(Date.parse(info.Created)>=result.previewPostStartedUnixMs-1000);
 const expected={'luci.preview':'true','luci.instance':instanceID,'luci.preview-protocol':'1','luci.preview-engine':cfg.engineID,'luci.preview-allocation':buildLease.id,'luci.preview-release':release.id,'luci.preview-project':project.id,'luci.preview-scope':scopeDigest,'luci.preview-source':sourceDigest,'luci.preview-commit':commit};
 for(const [key,value] of Object.entries(expected))if(Object.hasOwn(labels,key))assert.equal(labels[key],value,'Present product label differs from the exact lease');
 return {id:info.Id,createdAt:info.Created,startedAt:info.State.StartedAt,image:info.Image,fixtureLabels:{nonce:labels['org.lcfr.fixture.cancel'],project:labels['org.lcfr.fixture.project'],stage:labels['org.lcfr.fixture.stage']},presentProductLabelKeys:Object.keys(expected).filter(key=>Object.hasOwn(labels,key)),limits:{memory:info.HostConfig.Memory,memorySwap:info.HostConfig.MemorySwap,cpuQuota:info.HostConfig.CpuQuota,cpuPeriod:info.HostConfig.CpuPeriod},scope:'Read-only source-bound fixture-stage attribution; incomplete classic intermediate labels are not cleanup authority'};
}
function verifyProof(proof){
 assert.equal(proof.schema,1);assert.equal(proof.nonce,nonce);assert.equal(proof.projectId,project.id);assert.equal(proof.sourceSHA256,hash(join(helperDir,'fixture','hold.mjs')));assert.equal(proof.holdMs,45_000);assert.equal(proof.platform,'linux');assert.match(proof.node,/^v22\./);assert.equal(proof.uid,0);assert.ok(Date.now()-proof.startedUnixMs>=0&&Date.now()-proof.startedUnixMs<10_000,'Stage proof is too old to separate cancellation from natural expiry');
 const read=path=>proof.cgroup[path]?.status==='read'?proof.cgroup[path].text:null;
 assert.equal(read('/sys/fs/cgroup/memory.max')??read('/sys/fs/cgroup/memory/memory.limit_in_bytes'),'1073741824');
 const cpu=read('/sys/fs/cgroup/cpu.max');const [quota,period]=cpu?cpu.split(/\s+/):[read('/sys/fs/cgroup/cpu/cpu.cfs_quota_us'),read('/sys/fs/cgroup/cpu/cpu.cfs_period_us')];assert.equal(quota,'100000');assert.equal(period,'100000');
 if(read('/sys/fs/cgroup/memory.swap.max')!==null)assert.equal(read('/sys/fs/cgroup/memory.swap.max'),'0');else assert.equal(read('/sys/fs/cgroup/memory/memory.memsw.limit_in_bytes'),'1073741824');
 return proof;
}

try{
 result.config={...cfg};assert.equal(hash(cfg.binary),cfg.binarySHA256);assert.equal(hash(cfg.provenanceFile),cfg.provenanceSHA256);write('package-provenance.json',readFileSync(cfg.provenanceFile));
 assert.equal(readFileSync(join(dirname(cfg.binary),'package-identity'),'utf8').trim(),cfg.packageIdentity);
 assert.equal(command('git',['rev-parse','HEAD']).trim(),cfg.repositoryCommit);const beforeStatus=command('git',['status','--porcelain=v1','--untracked-files=normal']);write('source-status-before.txt',beforeStatus);assert.equal(beforeStatus,'');assert.equal(command('git',['diff','--name-only',cfg.sourceCommit,'--','control-plane','images','sandboxd','desktop/src']),'');
 const anchors=['control-plane/internal/api/preview_execution.go','control-plane/internal/api/execution_preview_leases.go','control-plane/internal/backend/dockerbe/previews.go','control-plane/internal/backend/dockerbe/preview_context.go','control-plane/internal/backend/dockerbe/preview_bases.go','control-plane/internal/product/releases.go','control-plane/internal/product/preview_cancel.go','control-plane/internal/product/preview_runtime.go','control-plane/internal/product/connections.go'];
 const binding=Object.fromEntries(anchors.map(path=>{const live=hash(join(cfg.repository,path));assert.equal(live,sha(command('git',['show',cfg.sourceCommit+':'+path])));return[path,live];}));write('source-bindings-before.json',binding);
 result.helperSHA256=hash(fileURLToPath(import.meta.url));result.fixtureSHA256={hold:hash(join(helperDir,'fixture','hold.mjs')),dockerfile:hash(join(helperDir,'fixture','Dockerfile.template'))};
 const engine=dockerJSON(['info','--format','{{json .}}']);assert.equal(engine.ID,cfg.engineID);assert.equal(engine.Driver,cfg.engineDriver);assert.equal(engine.OSType,'linux');const arch=({x86_64:'amd64',amd64:'amd64',aarch64:'arm64',arm64:'arm64'})[engine.Architecture];assert.ok(arch);write('engine.json',{ID:engine.ID,Driver:engine.Driver,OSType:engine.OSType,Architecture:engine.Architecture,MemTotal:engine.MemTotal,NCPU:engine.NCPU});
 const baseline={imageIDs:[...new Set(docker(['image','ls','-q','--no-trunc']).trim().split('\n').filter(Boolean))].sort(),containerIDs:docker(['ps','-a','-q','--no-trunc']).trim().split('\n').filter(Boolean),volumes:docker(['volume','ls','-q']).trim().split('\n').filter(Boolean)};write('daemon-baseline.json',baseline);assert.equal(baseline.imageIDs.length,9);assert.equal(baseline.containerIDs.length,0);assert.equal(baseline.volumes.length,0);assert.ok(baseline.imageIDs.includes(cfg.baseImageID));
 const [base]=dockerJSON(['image','inspect',cfg.baseImageID]);assert.equal(base.Id,cfg.baseImageID);assert.equal(base.Os,'linux');assert.equal(base.Architecture,arch);assert.ok(base.Size>0);assert.equal((base.Config.OnBuild??[]).length,0);assert.equal(Object.keys(base.Config.Volumes??{}).length,0);write('base-image-before.json',base);
 const free=statfsSync(root);result.preSpawnFreeBytes=free.bavail*free.bsize;result.ancestorPreSpawnFloorBytes=3*2**30+4*base.Size;assert.ok(result.preSpawnFreeBytes>result.ancestorPreSpawnFloorBytes,'Unchanged production preview estimate cannot fit; do not lower its reserve');
 const apiPort=await availablePort(),previewPort=await availablePort();assert.notEqual(apiPort,previewPort);origin='http://127.0.0.1:'+apiPort;
 const env={...baseEnv,LUCI_ADDR:'127.0.0.1:'+apiPort,LUCI_BROWSER_PREVIEW_ADDR:'127.0.0.1:'+previewPort,LUCI_BROWSER_PREVIEW_PORT:String(previewPort),LUCI_BACKEND:'docker',LUCI_DOCKER_HOST:cfg.dockerHost,LUCI_STATE_DIR:state,LUCI_PROJECTS_DIR:projects,LUCI_EXECUTION_STATE_DIR:ledgerDir,LUCI_RUNTIME_DIR:cfg.repository,LUCI_WEB_DIR:cfg.webDir,LUCI_SANDBOX_IMAGE:'luci/preview-fixture-no-worker:absent',LUCI_GIT_NAME:'Luci Cancellation Fixture',LUCI_GIT_EMAIL:'cancel-fixture@example.invalid',LUCI_REAP_INTERVAL:'30s',LUCI_SESSION_MEMORY_MIB:'1024',LUCI_SESSION_CPU_MILLIS:'1000'};
 write('controller-invocation.json',{command:[cfg.binary],cwd:cfg.repository,environment:env});stage='startup';child=spawn(cfg.binary,[],{cwd:cfg.repository,env,stdio:['ignore','pipe','pipe']});result.controllerPID=child.pid;captureProcesses();processWatch=setInterval(()=>{try{captureProcesses();}catch(e){processScanError=true;fail('process-observation',e);}},1000);processWatch.unref();
 child.once('error',error=>{childError=error;});child.once('close',(code,signal)=>{childClosed=true;result.controllerExit={code,signal};});
 for(const [name,stream] of [['controller-stdout.private.log',child.stdout],['controller-stderr.private.log',child.stderr]])stream.on('data',chunk=>{try{if(logsBytes+chunk.length<=16<<20){appendFileSync(join(run,name),chunk,{mode:0o600});logsBytes+=chunk.length;}else logsTruncated=true;}catch(e){fail('controller-log-retention',e);}});
 await until('Controller readiness',()=>api('GET','/v1/control-plane/status',undefined,{budget:2000}),30_000);
 const bootstrapToken=readFileSync(join(state,'control-plane','bootstrap-token'),'utf8').trim();const auth=await api('POST','/v1/auth/bootstrap',{bootstrapToken,username:'cancel-owner',displayName:'Cancellation Fixture Owner',password:randomUUID()+randomUUID()},{expect:[201],sensitive:true});token=auth.value.token;assert.ok(token);instanceID=readFileSync(join(state,'instance','instance-id'),'utf8').trim();result.instanceID=instanceID;
 stage='setup';await api('POST','/v1/setup/git/identity',{name:'Luci Cancellation Fixture',email:'cancel-fixture@example.invalid'});
 project=(await api('POST','/v1/product/projects',{name:'preview-cancel-'+randomUUID().slice(0,8)},{expect:[200,201]})).value;assert.match(project.id,/^p-[a-f0-9]{8,64}$/);assert.ok(realpathSync(project.path).startsWith(realpathSync(projects)+sep));result.projectId=project.id;const route='/v1/product/projects/'+project.id;
 await api('POST',route+'/brief',{revision:0,purpose:'Verify cancellation of an exact live preview build',audience:'Release verification',constraints:'One bounded fixture; no model, cloud, serving or replay',criteria:['Request cancellation while the exact RUN is live','Retain unknown work conservatively']});
 const spec=(await api('GET',route+'/specification')).value;const saved=(await api('POST',route+'/specification',{expectedRevision:spec.revision,expectedBriefRevision:spec.briefRevision,specification:{...spec,requirements:spec.requirements.map(x=>({...x,acceptance:x.title}))}})).value;const approved=(await api('POST',route+'/specification/approve',{revision:saved.revision,briefRevision:saved.briefRevision})).value;assert.equal(approved.status,'approved');
 const design=(await api('GET',route+'/design-plan')).value;const designSaved=(await api('POST',route+'/design-plan',{expectedRevision:design.revision,expectedBriefRevision:approved.briefRevision,expectedSpecificationRevision:approved.revision,plan:{...design,screens:[{id:'cancel-status',name:'Preview status',route:'/',purpose:'Explain pending cancellation and unknown outcome',states:['Building','Cancelling','Unknown'],widths:[390,1440],requirementIds:approved.requirements.map(x=>x.id)}]}})).value;const handoff=(await api('POST',route+'/design-plan/approve',{revision:designSaved.revision})).value;assert.equal(handoff.status,'approved');
 nonce=randomUUID();result.fixtureNonce=nonce;copyFileSync(join(helperDir,'fixture','hold.mjs'),join(project.path,'hold.mjs'));
 const manifest={nonce,projectId:project.id,holdMs:45_000,holdSourceSHA256:result.fixtureSHA256.hold};writeFileSync(join(project.path,'manifest.json'),JSON.stringify(manifest)+'\n',{mode:0o600});write('fixture-manifest.json',manifest);
 const dockerfile=readFileSync(join(helperDir,'fixture','Dockerfile.template'),'utf8').replaceAll('__BASE_IMAGE_ID__',cfg.baseImageID).replaceAll('__NONCE__',nonce).replaceAll('__PROJECT_ID__',project.id);writeFileSync(join(project.path,'Dockerfile'),dockerfile,{mode:0o600});write('reviewed-Dockerfile',dockerfile);assert.equal(nonceContainers().length,0);
 const gitBefore=(await api('GET',route+'/git')).value;await api('POST',route+'/git/commit',{message:'Reviewed bounded preview cancellation fixture',expectedReviewDigest:gitBefore.reviewDigest});const git=(await api('GET',route+'/git')).value;assert.equal(git.clean,true);commit=git.log[0].hash;assert.match(commit,/^[a-f0-9]{40}$/);result.fixtureCommit=commit;
 assert.equal((await api('GET',route+'/releases')).value.releases.length,0);policyBefore=(await api('GET','/v1/product/swarm-scheduler')).value;assert.equal(policyBefore.policy.diskReserveMiB,2048);assert.equal(ownLedger('before-start').length,0);result.ledgerScope=cfg.sharedExecutionDir?'explicit configured shared ledger':'fresh fixture host ledger; does not account pending reservations in another controller state root';
 stage='build-stage';result.previewPostStartedUnixMs=Date.now();result.previewPostStartedMonotonicMs=performance.now();
 startTask=api('POST',route+'/releases',{target:'local',port:3000,dockerfile:'Dockerfile',expectedCommit:commit},{expect:[422],budget:195_000}).then(value=>{startAcknowledged=true;startOutcome={ok:true,...value};return startOutcome;},error=>{startOutcome={ok:false,error};return startOutcome;});
 await until('Current live release',async()=>{if(startOutcome)throw new Error('Start completed before live cancellation stage');const rows=(await api('GET',route+'/releases',undefined,{budget:3000})).value.releases;if(!rows.length)return false;assert.equal(rows.length,1);const row=rows[0];assert.match(row.id,/^rel-[a-f0-9]{16,64}$/);assert.equal(row.commit,commit);if(row.status!=='building'||row.canCancel!==true)return false;assert.ok(Number.isSafeInteger(row.revision)&&row.revision>0);release=row;result.releaseId=row.id;const leases=ownLedger('live-build');if(leases.length!==1||leases[0].state!=='building')return false;buildLease=reservationIdentity(leases[0]);sourceDigest=buildLease.identity.sourceDigest;scopeDigest=buildLease.scope;return true;},120_000);
 await until('Exact nonce RUN',async()=>{
  if(startOutcome)throw new Error('Start completed before exact RUN proof');const ids=nonceContainers();if(!ids.length)return false;const [info]=dockerJSON(['inspect',ids[0]]);if(info.State.Status==='created'&&!info.State.Running)return false;write('stage-inspection.private.json',info);const projection=verifyStageContainer(info);
  const text=docker(['logs','--tail','5',ids[0]],3000);assert.ok(Buffer.byteLength(text)<=64<<10);const lines=text.trim().split('\n').filter(Boolean);if(!lines.length)return false;assert.equal(lines.length,1);assert.ok(lines[0].startsWith('LUCI_PREVIEW_CANCEL_STAGE '));stageProof=verifyProof(JSON.parse(lines[0].slice('LUCI_PREVIEW_CANCEL_STAGE '.length)));assert.equal(stageProof.architecture,arch==='amd64'?'x64':'arm64');containerID=ids[0];write('stage-proof.json',stageProof);write('stage-container.json',projection);result.stageContainerID=containerID;return true;
 },120_000);
 const current=(await api('GET',route+'/releases')).value.releases;assert.equal(current.length,1);release=current[0];assert.equal(release.canCancel,true);assert.equal(release.status,'building');assert.equal(release.id,result.releaseId);assert.equal(startOutcome,undefined);
 verifyProof(stageProof);verifyStageContainer(dockerJSON(['inspect',containerID],2000)[0]);record('exact RUN observed before cancellation',{containerID,releaseId:release.id,buildAllocationId:buildLease.id,proofAgeMs:Date.now()-stageProof.startedUnixMs});
 stage='cancel-intent';cancelSent=true;const cancelStarted=performance.now();const cancel=await api('POST',route+'/releases/'+release.id+'/cancel',{expectedRevision:release.revision},{expect:[202],budget:5000});
 result.cancelIntent={elapsedMs:performance.now()-cancelStarted,status:cancel.receipt.status,expectedRevision:release.revision,acknowledgedRevision:cancel.value.revision,startedMonotonicMs:cancelStarted,acknowledgedMonotonicMs:performance.now(),scope:'External HTTP request/body acknowledgement duration; separate from execution settlement, not a performance benchmark'};write('cancel-intent.json',result.cancelIntent);
 assert.equal(cancel.value.id,release.id);assert.equal(cancel.value.status,'cancelling');assert.equal(cancel.value.cancelRequested,true);assert.equal(cancel.value.canCancel??false,false);assert.equal(cancel.value.revision,release.revision+1);assert.ok(result.cancelIntent.elapsedMs<=2000,'Cancellation intent exceeded the declared two-second observation budget');
 stage='settlement';await until('Start return after cancellation',async()=>startOutcome,10_000);assert.equal(startOutcome.ok,true);assert.equal(startOutcome.value.id,release.id);assert.equal(startOutcome.value.status,'unknown');assert.equal(startOutcome.value.cancelRequested,true);assert.equal(startOutcome.value.canCancel??false,false);assert.ok(Date.now()-stageProof.startedUnixMs<45_000,'Natural RUN expiry could explain the Start return');
 result.startSettlement={status:startOutcome.receipt.status,elapsedSinceCancelMs:startOutcome.receipt.completedMonotonicMs-cancelStarted,naturalRunExpiryExcludedAtStartReturn:true,daemonSettlementConfirmed:false};
 const after=ownReceipt('after-cancel');assert.equal(after.runtime.phase,'unknown');assert.equal(after.runtime.resourcesHeld,true);assert.equal(after.runtime.cleanupConfirmed??false,false);assert.equal(after.runtime.buildAllocationId,buildLease.id);assert.equal(after.runtime.containerId??'','');assert.equal(after.cancelRequested,true);assert.equal(after.status,'unknown');
 const leases=ownLedger('after-cancel');assert.deepEqual(leases,[buildLease]);const publicRows=(await api('GET',route+'/releases')).value.releases;assert.equal(publicRows.length,1);assert.equal(publicRows[0].id,release.id);assert.equal(publicRows[0].status,'unknown');assert.equal(publicRows[0].canCancel??false,false);
 const policyAfter=(await api('GET','/v1/product/swarm-scheduler')).value;assert.deepEqual(policyAfter.policy,policyBefore.policy);assert.equal(policyAfter.resources.reservations.status,'ready');assert.ok(policyAfter.resources.reservations.count>=1&&policyAfter.resources.reservations.memoryMiB>=1024&&policyAfter.resources.reservations.cpuMillis>=1000);write('retained-reservation-view.json',policyAfter.resources.reservations);
 const daemonSamples=[];let daemonAbsent=false;const daemonEnd=Date.now()+timeout(65_000);
 while(Date.now()<daemonEnd){const ids=nonceContainers();const listed=docker(['ps','-a','--no-trunc','--filter','id='+containerID,'--format','{{.ID}}']).trim();assert.ok(!listed||listed===containerID);const sample={at:new Date().toISOString(),nonceIDs:ids,exactIDListed:listed!==''};daemonSamples.push(sample);assert.ok(daemonSamples.length<=100);if(ids.length===0&&listed===''){daemonAbsent=true;break;}await delay(750);}
 write('daemon-observation.json',{scope:'Read-only exact immutable ID plus fresh fixture nonce inventory; absence is not authoritative build-stream settlement and never releases capacity',containerID,daemonAbsent,samples:daemonSamples});result.startSettlement.daemonAbsenceObserved=daemonAbsent;
 assert.ok(daemonAbsent,'Bounded daemon closure observation did not establish exact RUN absence');assert.deepEqual(ownLedger('after-daemon-observation'),[buildLease]);const final=ownReceipt('final-observed');assert.equal(final.status,'unknown');assert.equal(final.runtime.resourcesHeld,true);
 const ownedServing=docker(['ps','-a','--no-trunc','--filter','label=luci.instance='+instanceID,'--filter','label=luci.preview-release='+release.id,'--format','{{.ID}}']).trim();assert.equal(ownedServing,'');
 const afterBase=dockerJSON(['image','inspect',cfg.baseImageID])[0];assert.equal(afterBase.Id,base.Id);assert.deepEqual(afterBase.RepoTags,base.RepoTags);assert.deepEqual(afterBase.RepoDigests,base.RepoDigests);assert.equal(afterBase.Size,base.Size);write('base-image-after.json',afterBase);
 const afterBinding=Object.fromEntries(anchors.map(path=>[path,hash(join(cfg.repository,path))]));write('source-bindings-after.json',afterBinding);assert.deepEqual(afterBinding,binding);assert.equal(command('git',['rev-parse','HEAD']).trim(),cfg.repositoryCommit);const afterStatus=command('git',['status','--porcelain=v1','--untracked-files=normal']);write('source-status-after.txt',afterStatus);assert.equal(afterStatus,'');assert.equal(hash(cfg.binary),cfg.binarySHA256);
 write('daemon-final.json',{images:[...new Set(docker(['image','ls','-q','--no-trunc']).trim().split('\n').filter(Boolean))].sort(),containerIDs:docker(['ps','-a','-q','--no-trunc']).trim().split('\n').filter(Boolean),volumeCount:docker(['volume','ls','-q']).trim().split('\n').filter(Boolean).length,scope:'Read-only final inventory; retained build image/cache is not pruned'});
 assert.equal(hash(fileURLToPath(import.meta.url)),result.helperSHA256);assert.equal(hash(join(helperDir,'fixture','hold.mjs')),result.fixtureSHA256.hold);assert.equal(hash(join(helperDir,'fixture','Dockerfile.template')),result.fixtureSHA256.dockerfile);
 result.scenarioPassed=true;record('cancel acknowledged; unknown charge retained; separate exact daemon absence observed',{releaseId:release.id,buildAllocationId:buildLease.id,containerID});
}catch(error){fail(stage,error);}
finally{
 stage='shutdown';
 if(project){try{ownLedger('shutdown-before');if(release)ownReceipt('shutdown-before');}catch(e){fail('shutdown-private-observation',e);}}
 try{captureProcesses();}catch(e){processScanError=true;fail('shutdown-process-observation',e);}
 if(child&&!childClosed){try{signalController('SIGTERM');}catch(e){fail('controller-sigterm',e);}const end=Math.min(lifecycleDeadline-8000,Date.now()+10_000);while(!childClosed&&Date.now()<end)await delay(100);if(!childClosed){result.cleanup.controllerKillRequired=true;fail('controller-shutdown',new Error('Graceful controller shutdown was not observed'));try{signalController('SIGKILL');}catch(e){fail('controller-sigkill',e);}const end=Math.min(lifecycleDeadline-3000,Date.now()+3000);while(!childClosed&&Date.now()<end)await delay(100);}}
 if(startTask&&!startOutcome){await Promise.race([startTask,delay(Math.max(1,Math.min(3000,lifecycleDeadline-Date.now()-2000)))]);}
 if(processWatch)clearInterval(processWatch);
 const rows=[];for(const [pid,known] of ownedProcesses){try{const current=processIdentity(pid);rows.push({...known,closure:!current||current.startTicks!==known.startTicks?'exact-process-gone':'still-present',current});}catch{rows.push({...known,closure:'unreadable'});}}
 safeWrite('owned-process-closure.json',{scanError:processScanError??false,processes:rows,scope:'Exact controller and descendants observed by PID/start identity; short-lived children may exit between samples. Zombies remain still-present.'});
 if(processScanError||rows.some(x=>x.closure!=='exact-process-gone'))fail('process-closure',new Error('Owned process closure remains unconfirmed'));
 if(project){try{const rows=ownLedger('shutdown-after');finalOwnRows=rows;if(result.scenarioPassed)assert.deepEqual(rows,[buildLease]);if(release)ownReceipt('shutdown-after');}catch(e){fail('shutdown-final-private-observation',e);}}
 observationsSealed=true;result.pendingMutations=[...pending.values()];result.requestCount=requestNumber;result.mutationCounts=Object.fromEntries(mutationCounts);result.cleanup.privateStateRetained=true;result.cleanup.unknownLeaseRetained=buildLease&&finalOwnRows?JSON.stringify(finalOwnRows)===JSON.stringify([buildLease]):null;result.cleanup.controllerExitObserved=child?childClosed:true;result.cleanup.startRequestAcknowledged=startAcknowledged;result.cleanup.cancelRequestSent=cancelSent;
 if(pending.size||child&&!childClosed||logsTruncated||abort.signal.aborted)fail('final-conservative-status',new Error('Pending mutation, live controller, truncated log or interruption remains'));
 result.firstFailure=firstFailure;result.logsTruncated=logsTruncated;result.passed=result.scenarioPassed&&!firstFailure;result.finishedAt=new Date().toISOString();clearTimeout(outerTimer);safeWrite('result.json',result);
 console.log(JSON.stringify({passed:result.passed,scenarioPassed:result.scenarioPassed,result:join(run,'result.json'),firstFailure:firstFailure?{stage:firstFailure.stage,sha256:firstFailure.sha256}:null}));
}
