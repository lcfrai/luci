// PREPARED ONLY. Invocation is an explicit future Docker/API mutation.
// node preview-smoke.mjs /absolute/path/to/reviewed-run-config.json
import assert from 'node:assert/strict';
import {spawn, spawnSync} from 'node:child_process';
import {createHash, randomUUID} from 'node:crypto';
import {appendFileSync, chmodSync, copyFileSync, existsSync, lstatSync, mkdirSync, readFileSync, realpathSync, readdirSync, rmSync, statSync, statfsSync, writeFileSync} from 'node:fs';
import {createServer} from 'node:net';
import {dirname, isAbsolute, join, resolve, sep} from 'node:path';
import {fileURLToPath} from 'node:url';
import {setTimeout as delay} from 'node:timers/promises';

assert.equal(process.platform,'linux');
assert.equal(Number(process.versions.node.split('.')[0]),24,'Use the locked Node 24 toolchain');
process.umask(0o077);
const helperDir=dirname(fileURLToPath(import.meta.url));
const cfg=JSON.parse(readFileSync(process.argv[2],'utf8'));
for(const key of ['repository','binary','evidenceParent','go','provenanceFile']) assert.ok(isAbsolute(cfg[key]),`${key} must be absolute`);
for(const key of ['binarySHA256','provenanceSHA256']) assert.match(cfg[key],/^[a-f0-9]{64}$/);
assert.match(cfg.sourceCommit,/^[a-f0-9]{40}$/);
assert.match(cfg.repositoryCommit,/^[a-f0-9]{40}$/);
assert.ok(cfg.engineID&&cfg.engineDriver);
assert.match(cfg.packageIdentity,/^[a-f0-9]{64}$/);
assert.ok(cfg.mode==='scratch'||cfg.mode==='warm','mode: scratch or warm');
if(cfg.mode==='warm') assert.match(cfg.baseImageID,/^sha256:[a-f0-9]{64}$/);
assert.equal(cfg.dockerHost,'unix:///var/run/docker.sock','Only this explicitly reviewed local daemon endpoint is supported');
assert.equal(cfg.acknowledgeFixtureMutation,true,'The run config must acknowledge this exact fixture run');
assert.equal(realpathSync(cfg.evidenceParent),cfg.evidenceParent,'Use a canonical existing evidence parent');
const run=join(cfg.evidenceParent,`preview-${Date.now()}-${randomUUID()}`);
mkdirSync(run,{mode:0o700});
const result={passed:false,scope:'hand-written fixture preview transport only; no live model, project check pass, installer or full product acceptance',startedAt:new Date().toISOString(),run,mode:cfg.mode,checks:[],cleanup:{},fixtureSourceSHA256:null};
const write=(name,value)=>writeFileSync(join(run,name),typeof value==='string'?value:JSON.stringify(value,null,2)+'\n',{mode:0o600});
const hash=(p)=>createHash('sha256').update(readFileSync(p)).digest('hex');
const record=(name,details={})=>{ const row={at:new Date().toISOString(),name,...details}; result.checks.push(row); appendFileSync(join(run,'events.jsonl'),JSON.stringify(row)+'\n',{mode:0o600}); };
const root=join(run,'private');
const state=join(root,'state'), projects=join(root,'projects'), scratchTmp=join(root,'tmp');
for(const p of [root,state,projects,scratchTmp,join(root,'docker-config')]) mkdirSync(p,{mode:0o700});
// No inherited model/cloud credentials, Docker context, Git identity/hooks/config,
// controller state or LUCI overrides enter the new controller.
const baseEnv={PATH:process.env.PATH,LANG:'C.UTF-8',TZ:'UTC',TMPDIR:scratchTmp,DOCKER_HOST:cfg.dockerHost,DOCKER_CONFIG:join(root,'docker-config'),GIT_CONFIG_NOSYSTEM:'1',GIT_CONFIG_GLOBAL:join(root,'empty-git-config'),GIT_TERMINAL_PROMPT:'0',AWS_EC2_METADATA_DISABLED:'true'};
writeFileSync(baseEnv.GIT_CONFIG_GLOBAL,'',{mode:0o600});
const lifecycleDeadline=Date.now()+8*60_000;
const deadline=lifecycleDeadline-45_000;
const abort=new AbortController();
for(const signal of ['SIGINT','SIGTERM'])process.on(signal,()=>{result.cancelSignal=signal;abort.abort(new Error(`Driver received ${signal}`));});
const outerTimer=setTimeout(()=>{
 result.passed=false;result.error='Eight-minute outer lifecycle deadline expired; any unknown Docker work remains retained';
 result.finishedAt=new Date().toISOString();result.cleanup.outerDeadline=true;
 try{terminateController('SIGKILL');}catch(e){result.cleanup.deadlineKillError=e.message;}
 write('result.json',result);process.exit(124);
},8*60_000);
outerTimer.unref();
function fence(){abort.signal.throwIfAborted();assert.ok(Date.now()<deadline,'Fixture work deadline expired');}

function command(executable,args,timeout=15_000,env=baseEnv,cleanup=false) {
 if(!cleanup)fence();
 timeout=Math.max(1,Math.min(timeout,(cleanup?lifecycleDeadline-10_000:deadline)-Date.now()));
 const r=spawnSync(executable,args,{cwd:cfg.repository,env,encoding:'utf8',timeout,maxBuffer:4<<20,killSignal:'SIGKILL'});
 if(r.error||r.status!==0) throw Object.assign(new Error(`${executable} ${args[0]} failed: ${r.error?.message??r.stderr?.slice(0,2000)??r.status}`),{commandResult:r});
 return r.stdout;
}
function docker(args,cleanup=false){return command('docker',['--host',cfg.dockerHost,...args],15_000,baseEnv,cleanup);}
const dockerJSON=(args)=>JSON.parse(docker(args));
async function availablePort(){const s=createServer();await new Promise((res,rej)=>s.once('error',rej).listen(0,'127.0.0.1',res));const p=s.address().port;await new Promise(r=>s.close(r));return p;}
let child,childClosed=false,childError,token,project,release,attemptedRelease=false,stopAttempted=false,origin,instanceID;
let requestNumber=0,logsBytes=0,logsTruncated=false,processWatch,processScanError;
const ownedProcesses=new Map();
function processIdentity(pid){
 try{const raw=readFileSync(`/proc/${pid}/stat`,'utf8');const rest=raw.slice(raw.lastIndexOf(')')+2).trim().split(/\s+/);return {pid:Number(pid),parentPID:Number(rest[1]),startTicks:rest[19],state:rest[0]};}
 catch(e){if(e.code==='ENOENT'||e.code==='ESRCH')return null;throw e;}
}
function captureProcesses(){
 if(!child?.pid)return;
 const names=readdirSync('/proc').filter(x=>/^\d+$/.test(x));assert.ok(names.length<10000,'Process inventory exceeds bound');
 const rows=new Map(names.map(x=>[Number(x),processIdentity(x)]).filter(([,x])=>x));
 const selected=new Set();const actualRoot=rows.get(child.pid),knownRoot=ownedProcesses.get(child.pid);
 if(actualRoot&&(!knownRoot||knownRoot.startTicks===actualRoot.startTicks)){selected.add(child.pid);ownedProcesses.set(child.pid,actualRoot);}
 // Include descendants of a previously acknowledged child even after reparenting.
 for(const [pid,known] of ownedProcesses){if(rows.get(pid)?.startTicks===known.startTicks)selected.add(pid);}
 let changed=true;while(changed){changed=false;for(const [pid,row] of rows)if(!selected.has(pid)&&selected.has(row.parentPID)){selected.add(pid);ownedProcesses.set(pid,row);changed=true;}}
}
function terminateController(signal){
 if(!child||childClosed)return;
 const known=ownedProcesses.get(child.pid),now=processIdentity(child.pid);
 if(!now)return;
 assert.ok(known&&now.startTicks===known.startTicks,'Refusing to signal an unverified/reused controller PID');
 child.kill(signal);
}

const ledgerDir=cfg.sharedExecutionDir??join(state,'control-plane','execution');
assert.ok(isAbsolute(ledgerDir));
assert.ok(!ledgerDir.startsWith(projects+sep));
const ledgerFile=join(ledgerDir,'leases.json');
const releaseFile=()=>join(state,'sessions','releases',`${project.id}.json`);
function boundedJSON(p,limit=2<<20){const s=lstatSync(p);assert.ok(s.isFile()&&!s.isSymbolicLink()&&s.size<=limit,`Unsafe or oversized receipt ${p}`);return JSON.parse(readFileSync(p,'utf8'));}
function receipt(label){
 const rows=boundedJSON(releaseFile(),8<<20);
 const row=rows.find(x=>x.id===release?.id);
 assert.ok(row?.runtime,'Expected an exact retained typed preview receipt');
 assert.equal(row.projectId,project.id);
 write(`${label}-release.json`,row);
 return row;
}
function ledger(label){
 if(!existsSync(ledgerFile)){write(`${label}-ledger.json`,{exists:false});return null;}
 const value=boundedJSON(ledgerFile), own=(value.previews??[]).filter(x=>x.identity?.projectId===project?.id);
 write(`${label}-ledger.json`,{rawSHA256:hash(ledgerFile),version:value.version,ownPreviews:own,counts:{workers:value.leases?.length??0,checks:value.checks?.length??0,runtimeImages:value.runtimeImages?.length??0,allPreviews:value.previews?.length??0}});
 return own;
}
async function api(method,path,body,{expect=[200],timeout=30_000,sensitive=false,cleanup=false}={}){
 if(!cleanup)fence();
 const ms=Math.max(1,Math.min(timeout,(cleanup?lifecycleDeadline-10_000:deadline)-Date.now()));
 const num=String(++requestNumber).padStart(3,'0');
 write(`${num}-request.json`,{method,path,body:sensitive?'[bootstrap credentials omitted]':body??null,timeout:ms,at:new Date().toISOString()});
 const response=await fetch(origin+path,{method,redirect:'error',signal:cleanup?AbortSignal.timeout(ms):AbortSignal.any([abort.signal,AbortSignal.timeout(ms)]),headers:{...(body===undefined?{}:{'Content-Type':'application/json'}),...(token?{Authorization:`Bearer ${token}`}:{})},body:body===undefined?undefined:JSON.stringify(body)});
 const text=await response.text();assert.ok(Buffer.byteLength(text)<=4<<20,'API response exceeds receipt bound');
 let value;try{value=text?JSON.parse(text):null;}catch{value={raw:text};}
 write(`${num}-response.json`,{status:response.status,value:sensitive?{user:value?.user,tokenOmitted:true}:value});
 assert.ok(expect.includes(response.status),`${method} ${path}: HTTP ${response.status}: ${sensitive?'[sensitive response omitted]':text.slice(0,1000)}`);
 return value;
}
async function until(name,fn,ms=30_000){const end=Math.min(deadline,Date.now()+ms);let last;while(Date.now()<end){fence();if(childError)throw childError;if(childClosed)throw new Error('Controller exited; see bounded logs');try{const value=await fn();if(value)return value;}catch(e){last=e;}await delay(500);}throw new Error(`${name} timed out: ${last?.message??'not ready'}`);}
function assertOwnedContainer(info,r,scope){
 const x=r.runtime,labels=info.Config.Labels;
 assert.match(info.Id,/^[a-f0-9]{64}$/);assert.equal(info.Id,x.containerId);
 for(const [k,v] of Object.entries({'luci.preview':'true','luci.instance':x.instanceId,'luci.preview-protocol':'1','luci.preview-engine':x.engineId,'luci.preview-allocation':x.allocationId,'luci.preview-release':release.id,'luci.preview-project':project.id,'luci.preview-source':x.sourceDigest,'luci.preview-scope':scope,'luci.preview-commit':x.commit,'luci.preview-image':x.image})) assert.equal(labels[k],v,k);
 assert.equal(x.instanceId,instanceID);assert.equal(info.Image,x.image);
 assert.equal(info.HostConfig.Memory,1024*1024*1024);assert.equal(info.HostConfig.MemorySwap,info.HostConfig.Memory);assert.equal(info.HostConfig.NanoCpus,1e9);assert.equal(info.HostConfig.PidsLimit,256);
 assert.deepEqual(info.HostConfig.CapDrop,['ALL']);assert.deepEqual(info.HostConfig.SecurityOpt,['no-new-privileges:true']);assert.equal(info.HostConfig.Privileged,false);assert.equal(info.Mounts.length,0);
 assert.equal(info.Config.User,'65532:65532');
 const bindings=Object.values(info.NetworkSettings.Ports??{}).flatMap(x=>x??[]);assert.ok(bindings.length>0);assert.ok(bindings.every(x=>x.HostIp==='127.0.0.1'));
}
async function stopExact(){
 if(!release||stopAttempted)return;stopAttempted=true;
 const before=receipt('before-stop');
 if(!before.runtime.buildAllocationId&&!before.runtime.allocationId){result.cleanup.noAllocationReceipt=true;return;}
 const stopped=await api('POST',`/v1/product/projects/${project.id}/releases/${release.id}/stop`,{}, {timeout:35_000,cleanup:true});
 assert.equal(stopped.status,'stopped');
 const after=receipt('after-stop');assert.equal(after.runtime.phase,'finished');assert.equal(after.runtime.resourcesHeld,false);assert.equal(after.runtime.cleanupConfirmed,true);
 const ids=docker(['ps','-a','--no-trunc','--filter',`label=luci.instance=${instanceID}`,'--filter',`label=luci.preview-release=${release.id}`,'--format','{{.ID}}'],true).trim();assert.equal(ids,'','Exact owned preview containers survived Stop');
 assert.equal((ledger('after-stop')??[]).length,0,'Owned preview ledger lineage was not retired');
 result.cleanup.previewStopConfirmed=true;result.cleanup.retainedImage=after.runtime.image??null;
 // Stop settles compute. Output/cache images are intentionally retained. Never
 // prune or remove images globally, nor infer image ownership from a short ID.
 record('normal API Stop settled exact allocations; image/cache retained');
}
try {
 result.config={repository:cfg.repository,binary:cfg.binary,sourceCommit:cfg.sourceCommit,repositoryCommit:cfg.repositoryCommit,dockerHost:cfg.dockerHost,ledgerDir,ledgerScope:cfg.sharedExecutionDir?'explicit existing machine ledger':'fresh fixture host ledger; does not account pending work in another controller state root'};
 assert.equal(hash(cfg.binary),cfg.binarySHA256,'Binary identity changed');assert.equal(hash(cfg.provenanceFile),cfg.provenanceSHA256,'Package provenance identity changed');
 result.binarySHA256=cfg.binarySHA256;result.provenanceSHA256=cfg.provenanceSHA256;
 write('package-provenance.json',readFileSync(cfg.provenanceFile,'utf8'));
 assert.equal(command('git',['rev-parse','HEAD']).trim(),cfg.repositoryCommit);
 assert.equal(readFileSync(join(dirname(cfg.binary),'package-identity'),'utf8').trim(),cfg.packageIdentity);
 assert.equal(command('git',['diff','--name-only',cfg.sourceCommit,'--','control-plane','images','sandboxd','desktop/src']),'','Product source differs from packaged source');
 const beforeStatus=command('git',['status','--porcelain=v1','--untracked-files=normal']);write('source-status-before.txt',beforeStatus);assert.equal(beforeStatus,'','Use the published clean source checkpoint');
 const anchors=['control-plane/internal/backend/dockerbe/previews.go','control-plane/internal/backend/dockerbe/preview_context.go','control-plane/internal/api/preview_execution.go','control-plane/internal/api/execution_preview_leases.go','control-plane/internal/api/execution_capacity.go','control-plane/internal/product/releases.go','control-plane/internal/product/project_resolver.go','control-plane/internal/product/project_runs_evidence.go'];
 const binding=Object.fromEntries(anchors.map(p=>{const disk=hash(join(cfg.repository,p));const packaged=createHash('sha256').update(command('git',['show',`${cfg.sourceCommit}:${p}`])).digest('hex');assert.equal(disk,packaged,`Source differs from package revision: ${p}`);return[p,disk];}));write('source-bindings-before.json',binding);
 result.helperSHA256=hash(fileURLToPath(import.meta.url));result.fixtureSourceSHA256=hash(join(helperDir,'fixture.go'));
 const info=dockerJSON(['info','--format','{{json .}}']);assert.equal(info.ID,cfg.engineID);assert.equal(info.Driver,cfg.engineDriver);assert.equal(info.OSType,'linux');assert.ok(['overlay2','overlayfs','vfs'].includes(info.Driver));
 const arch=({x86_64:'amd64',amd64:'amd64',aarch64:'arm64',arm64:'arm64'})[info.Architecture];assert.ok(arch,'Unsupported engine architecture');
 write('engine-before.json',{ID:info.ID,Driver:info.Driver,OSType:info.OSType,Architecture:info.Architecture,DockerRootDir:info.DockerRootDir,MemTotal:info.MemTotal,NCPU:info.NCPU,ServerVersion:info.ServerVersion});
 if(cfg.mode==='warm') {const [base]=dockerJSON(['image','inspect',cfg.baseImageID]);assert.equal(base.Id,cfg.baseImageID);assert.equal(base.Os,'linux');assert.equal(base.Architecture,arch);assert.ok(base.Size>0);assert.equal((base.Config.OnBuild??[]).length,0);assert.equal(Object.keys(base.Config.Volumes??{}).length,0);write('base-image.json',base);result.baseImageID=base.Id;result.minimumBuildDiskBytes=3*2**30+2*base.Size;}
 else result.minimumBuildDiskBytes=3*2**30;
 const source=join(root,'fixture.go'),app=join(root,'fixture');copyFileSync(join(helperDir,'fixture.go'),source);
 const compileEnv={...baseEnv,CGO_ENABLED:'0',GOMAXPROCS:'2',GOOS:'linux',GOARCH:arch,GOTOOLCHAIN:'local',GOPROXY:'off',GOSUMDB:'off',GOWORK:'off',GOFLAGS:'',GO111MODULE:'off',GOCACHE:cfg.goCache??join(root,'go-cache'),GOTMPDIR:scratchTmp};
 const compileArgs=['build','-p=2','-trimpath','-ldflags=-buildid=','-o',app,source];
 write('compile-invocation.json',{command:[cfg.go,...compileArgs],environment:compileEnv,timeoutMs:120_000});
 const compiled=spawnSync(cfg.go,compileArgs,{env:compileEnv,cwd:root,encoding:'utf8',timeout:120_000,maxBuffer:4<<20,killSignal:'SIGKILL'});
 write('compile-stdout.log',compiled.stdout??'');write('compile-stderr.log',compiled.stderr??'');write('compile-result.json',{exitCode:compiled.status,signal:compiled.signal,error:compiled.error?.message??null});
 assert.equal(compiled.status,0,'Bounded offline static fixture compile failed');assert.ok(statSync(app).size<16<<20);result.fixtureBinarySHA256=hash(app);result.goVersion=command(cfg.go,['version']);
 const apiPort=await availablePort();fence();const previewPort=await availablePort();fence();assert.notEqual(apiPort,previewPort);
 const storage=statfsSync(root);result.preSpawnRootAvailableBytes=storage.bavail*storage.bsize;assert.ok(result.preSpawnRootAvailableBytes>result.minimumBuildDiskBytes+4*statSync(app).size,'Root storage cannot retain the unchanged preview estimate');
 assert.equal(hash(cfg.binary),cfg.binarySHA256);fence();origin=`http://127.0.0.1:${apiPort}`;
 const env={...baseEnv,LUCI_ADDR:`127.0.0.1:${apiPort}`,LUCI_BROWSER_PREVIEW_ADDR:`127.0.0.1:${previewPort}`,LUCI_BROWSER_PREVIEW_PORT:String(previewPort),LUCI_BACKEND:'docker',LUCI_DOCKER_HOST:cfg.dockerHost,LUCI_STATE_DIR:state,LUCI_PROJECTS_DIR:projects,LUCI_EXECUTION_STATE_DIR:ledgerDir,LUCI_RUNTIME_DIR:cfg.repository,LUCI_WEB_DIR:cfg.webDir??join(cfg.repository,'desktop','dist'),LUCI_SANDBOX_IMAGE:cfg.mode==='warm'?cfg.baseImageID:'luci/preview-fixture-no-worker:absent',LUCI_GIT_NAME:'Luci Preview Fixture',LUCI_GIT_EMAIL:'preview-fixture@example.invalid',LUCI_REAP_INTERVAL:'30s',LUCI_SESSION_MEMORY_MIB:'1024',LUCI_SESSION_CPU_MILLIS:'1000'};
 write('controller-invocation.json',{command:[cfg.binary],cwd:cfg.repository,environment:env});
 child=spawn(cfg.binary,[],{cwd:cfg.repository,env,stdio:['ignore','pipe','pipe']});result.controllerPID=child.pid;
 captureProcesses();processWatch=setInterval(()=>{try{captureProcesses();}catch(e){processScanError=e.message;}},1000);processWatch.unref();
 child.once('error',e=>{childError=e;});child.once('close',(code,signal)=>{childClosed=true;result.controllerExit={code,signal};});
 for(const [name,stream] of [['controller-stdout.log',child.stdout],['controller-stderr.log',child.stderr]])stream.on('data',chunk=>{if(logsBytes+chunk.length<=16<<20){appendFileSync(join(run,name),chunk,{mode:0o600});logsBytes+=chunk.length;}else logsTruncated=true;});
 await until('controller readiness',()=>api('GET','/v1/control-plane/status',undefined,{timeout:2_000}));
 const bootstrapToken=readFileSync(join(state,'control-plane','bootstrap-token'),'utf8').trim();
 const auth=await api('POST','/v1/auth/bootstrap',{bootstrapToken,username:'preview-owner',displayName:'Preview Fixture Owner',password:randomUUID()+randomUUID()},{sensitive:true,expect:[201]});token=auth.token;assert.ok(token);
 instanceID=readFileSync(join(state,'instance','instance-id'),'utf8').trim();result.instanceID=instanceID;
 await api('POST','/v1/setup/git/identity',{name:'Luci Preview Fixture',email:'preview-fixture@example.invalid'});
 project=await api('POST','/v1/product/projects',{name:`preview-${randomUUID().slice(0,8)}`},{expect:[200,201]});assert.ok(realpathSync(project.path).startsWith(realpathSync(projects)+sep));result.projectId=project.id;
 const route=`/v1/product/projects/${project.id}`;
 await api('POST',route+'/brief',{revision:0,purpose:'Verify a local preview calculator',audience:'Release verification',constraints:'Fixture only; no model, cloud calls or runtime installation',criteria:['Add two entered amounts','Bind the preview to loopback and settle owned execution on Stop']});
 const spec=await api('GET',route+'/specification');
 const saved=await api('POST',route+'/specification',{expectedRevision:spec.revision,expectedBriefRevision:spec.briefRevision,specification:{...spec,requirements:spec.requirements.map(x=>({...x,acceptance:x.title}))}});
 const approved=await api('POST',route+'/specification/approve',{revision:saved.revision,briefRevision:saved.briefRevision});assert.equal(approved.status,'approved');
 const design=await api('GET',route+'/design-plan');
 const designSaved=await api('POST',route+'/design-plan',{expectedRevision:design.revision,expectedBriefRevision:approved.briefRevision,expectedSpecificationRevision:approved.revision,plan:{...design,screens:[{id:'calculator',name:'Calculator',route:'/',purpose:'Add two amounts',states:['Ready','Invalid input'],widths:[390,1440],requirementIds:approved.requirements.map(x=>x.id)}]}});
 const handoff=await api('POST',route+'/design-plan/approve',{revision:designSaved.revision});assert.equal(handoff.status,'approved');
 copyFileSync(source,join(project.path,'fixture.go'));copyFileSync(app,join(project.path,'fixture'));chmodSync(join(project.path,'fixture'),0o755);
 writeFileSync(join(project.path,'Dockerfile'),`FROM ${cfg.mode==='scratch'?'scratch':cfg.baseImageID}\nCOPY fixture /fixture\nUSER 65532:65532\nEXPOSE 3000\nENTRYPOINT ["/fixture"]\n`,{mode:0o600});
 const gitBefore=await api('GET',route+'/git');await api('POST',route+'/git/commit',{message:'Reviewed preview transport fixture',expectedReviewDigest:gitBefore.reviewDigest});
 const git=await api('GET',route+'/git');assert.equal(git.clean,true);const commit=git.log[0].hash;assert.match(commit,/^[a-f0-9]{40}$/);result.fixtureCommit=commit;
 assert.deepEqual(await api('GET',route+'/specification'),approved);assert.deepEqual(await api('GET',route+'/design-plan'),handoff);
 const checks=await api('GET',route+'/checks');write('check-gate-observation.json',{scope:'Read-only retained checks; no passing project check or project-run acceptance fabricated',response:checks});
 const configuration=await api('POST',route+'/configuration',{expectedRevision:0,updates:[{name:'APP_GREETING',value:'Luci fixture preview OK'}]});assert.equal(configuration.revision,1);
 const metadata=await api('GET',route+'/configuration');assert.ok(!JSON.stringify(metadata).includes('Luci fixture preview OK'));
 const releasesBefore=await api('GET',route+'/releases');assert.equal(releasesBefore.releases.length,0);
 const rejected=await api('POST',route+'/releases',{target:'local',port:3000,dockerfile:'Dockerfile',expectedCommit:'0'.repeat(40)},{expect:[409]});assert.equal(rejected.error,'source_changed');
 const dirtyFile=join(project.path,'uncommitted-gate-probe.txt');writeFileSync(dirtyFile,'uncommitted fixture source\n',{mode:0o600});
 try{const dirty=await api('POST',route+'/releases',{target:'local',port:3000,dockerfile:'Dockerfile',expectedCommit:commit},{expect:[409]});assert.equal(dirty.error,'commit_required');}finally{rmSync(dirtyFile);}
 assert.equal((await api('GET',route+'/releases')).releases.length,0);assert.equal((await api('GET',route+'/git')).clean,true);
 const policyBefore=await api('GET','/v1/product/swarm-scheduler');assert.equal(policyBefore.policy.diskReserveMiB,2048,'Fixture must not lower/change the normal disk reserve');ledger('before-build');
 record('normal source/review/config API gates retained',{commit,configurationRevision:1,projectChecks:'observed only; not executed'});
 attemptedRelease=true;
 const created=await api('POST',route+'/releases',{target:'local',port:3000,dockerfile:'Dockerfile',expectedCommit:commit},{expect:[200,201,422],timeout:195_000});
 if(created.id){release=created;result.releaseId=created.id;}
 assert.ok(release,'No exact release receipt returned');
 const r=receipt('created');result.runtime=r.runtime;ledger('after-build');
 assert.equal(created.status,'ready',`Preview refused or unconfirmed: ${created.message}`);assert.equal(created.commit,commit);assert.equal(created.configurationRevision,1);
 assert.equal(r.runtime.phase,'running');assert.equal(r.runtime.resourcesHeld,true);assert.notEqual(r.runtime.buildAllocationId,r.runtime.allocationId);assert.equal(r.runtime.commit,commit);assert.equal(r.runtime.engineId,info.ID);
 const own=ledger('running');assert.equal(own.length,2);for(const v of own){assert.equal(v.scope,v.identity.scopeDigest);assert.equal(v.identity.projectId,project.id);assert.equal(v.identity.releaseId,release.id);assert.equal(v.identity.engineId,info.ID);assert.equal(v.identity.instanceId,instanceID);assert.equal(v.identity.commit,commit);assert.equal(v.identity.sourceDigest,r.runtime.sourceDigest);assert.equal(v.resources.MemoryMiB??v.resources.memoryMiB,1024);assert.equal(v.resources.CPUMillis??v.resources.cpuMillis,1000);}assert.ok(own.some(x=>x.id===r.runtime.buildAllocationId&&x.kind==='preview-build'&&x.state==='finished'));assert.ok(own.some(x=>x.id===r.runtime.allocationId&&x.kind==='preview'&&x.state==='running'));
 const runningPolicy=await api('GET','/v1/product/swarm-scheduler');assert.deepEqual(runningPolicy.policy,policyBefore.policy);const reserved=runningPolicy.resources.reservations;assert.equal(reserved.status,'ready');assert.ok(reserved.count>=1&&reserved.memoryMiB>=1024&&reserved.cpuMillis>=1000);
 const [container]=dockerJSON(['inspect',r.runtime.containerId]);assertOwnedContainer(container,r,own.find(x=>x.id===r.runtime.allocationId).scope);write('owned-container-running.json',container);
 const [built]=dockerJSON(['image','inspect',r.runtime.image]);assert.equal(built.Id,r.runtime.image);assert.equal(built.Config.Labels['luci.preview-allocation'],r.runtime.buildAllocationId);assert.equal(built.Config.Labels['luci.instance'],instanceID);assert.equal(built.Config.Labels['luci.preview-source'],r.runtime.sourceDigest);write('owned-output-image.json',built);
 const url=new URL(r.runtime.url);assert.equal(url.protocol,'http:');assert.equal(url.hostname,'127.0.0.1');assert.ok(Number(url.port)>0);
 const page=await fetch(url,{redirect:'error',signal:AbortSignal.timeout(5_000)});assert.equal(page.status,200);const html=await page.text();assert.match(html,/Preview calculator/);write('http-index.html',html);
 const response=await fetch(new URL('/sum?a=12.5&b=7.25',url),{redirect:'error',signal:AbortSignal.timeout(5_000)});assert.equal(response.status,200);const answer=await response.json();assert.deepEqual(answer,{greeting:'Luci fixture preview OK',modelKeyPresent:false,sum:19.75,uid:65532});write('http-answer.json',answer);
 const invalid=await fetch(new URL('/sum?a=invalid&b=1',url),{redirect:'error',signal:AbortSignal.timeout(5_000)});assert.equal(invalid.status,400);write('http-invalid.json',{status:invalid.status,body:await invalid.text()});
 record('actual image build, limited serving allocation and useful HTTP response verified',{containerId:container.Id,imageID:built.Id,buildAllocationId:r.runtime.buildAllocationId,allocationId:r.runtime.allocationId});
 await stopExact();const finalPolicy=await api('GET','/v1/product/swarm-scheduler');assert.deepEqual(finalPolicy.policy,policyBefore.policy);if(!cfg.sharedExecutionDir){assert.equal(finalPolicy.resources.reservations.count,0);assert.equal(finalPolicy.resources.reservations.memoryMiB,0);assert.equal(finalPolicy.resources.reservations.cpuMillis,0);}
 const afterBinding=Object.fromEntries(anchors.map(p=>[p,hash(join(cfg.repository,p))]));write('source-bindings-after.json',afterBinding);assert.deepEqual(afterBinding,binding);assert.equal(command('git',['rev-parse','HEAD']).trim(),cfg.repositoryCommit);const afterStatus=command('git',['status','--porcelain=v1','--untracked-files=normal']);write('source-status-after.txt',afterStatus);assert.equal(afterStatus,'');assert.equal(hash(cfg.binary),cfg.binarySHA256);
 assert.equal(logsTruncated,false,'Controller logs exceeded evidence bound');result.passed=true;
} catch(error){result.error=error.message;result.errorStack=error.stack;process.exitCode=1;}
finally {
 // A lost creation response is recovered only by read-only listing of this
 // fresh fixture project. Never replay release creation or infer a new ID.
 if(attemptedRelease&&!release&&project&&token&&!childClosed){try{const saved=await api('GET',`/v1/product/projects/${project.id}/releases`,undefined,{cleanup:true,timeout:5_000});if(saved.releases.length===1){release=saved.releases[0];result.releaseId=release.id;}else result.cleanup.releaseIdentityUnknown=true;}catch(e){result.cleanup.lookupError=e.message;}}
 if(release&&!stopAttempted&&!childClosed){try{await stopExact();}catch(e){result.cleanup.stopError=e.message;result.passed=false;process.exitCode=1;}}
 try{if(project)ledger('final');}catch(e){result.cleanup.ledgerObservationError=e.message;result.passed=false;process.exitCode=1;}
 try{captureProcesses();}catch(e){processScanError=e.message;}
 if(child&&!childClosed){try{terminateController('SIGTERM');}catch(e){result.cleanup.controllerSignalError=e.message;result.passed=false;process.exitCode=1;}const end=Math.min(lifecycleDeadline-3000,Date.now()+8_000);while(!childClosed&&Date.now()<end)await delay(100);if(!childClosed){result.cleanup.controllerKillRequired=true;result.passed=false;process.exitCode=1;try{terminateController('SIGKILL');}catch(e){result.cleanup.controllerKillError=e.message;}const hardEnd=Math.min(lifecycleDeadline-1000,Date.now()+2_000);while(!childClosed&&Date.now()<hardEnd)await delay(100);}}
 if(processWatch)clearInterval(processWatch);
 const processReceipts=[];
 for(const [pid,known] of ownedProcesses){try{const current=processIdentity(pid);processReceipts.push({...known,closure:!current||current.startTicks!==known.startTicks?'exact-process-gone':'still-present',current});}catch(e){processReceipts.push({...known,closure:'unreadable',error:e.message});}}
 write('owned-process-closure.json',{scanError:processScanError??null,processes:processReceipts,scope:'Controller and descendants observed while it was alive; this fixture creates one inline bootstrap owner and never asks for a managed worker. Short-lived children may exit between one-second scans.'});
 if(processScanError||processReceipts.some(x=>x.closure!=='exact-process-gone')){result.cleanup.processClosureUnconfirmed=true;result.passed=false;process.exitCode=1;}
 clearTimeout(outerTimer);
 result.cleanup.controllerExitObserved=child?childClosed:true;result.cleanup.privateStateRetained=true;result.cleanup.manualDockerRemoval=false;result.cleanup.globalPrune=false;result.logsTruncated=logsTruncated;
 if(child&&!childClosed){result.passed=false;process.exitCode=1;}
 if(abort.signal.aborted){result.passed=false;process.exitCode=1;}
 result.finishedAt=new Date().toISOString();write('result.json',result);
 console.log(JSON.stringify({passed:result.passed,result:join(run,'result.json'),error:result.error??null}));
}
