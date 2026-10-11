// Prepared intercepted-transport fixture. No service or Docker call runs until invoked.
// node installer-disk-smoke.mjs /absolute/reviewed-config.json
import assert from 'node:assert/strict';
import {spawn,spawnSync} from 'node:child_process';
import {createHash,randomUUID} from 'node:crypto';
import {appendFileSync,chmodSync,existsSync,lstatSync,mkdirSync,mkdtempSync,readFileSync,readdirSync,realpathSync,statfsSync,writeFileSync} from 'node:fs';
import http from 'node:http';
import {createServer} from 'node:net';
import {dirname,isAbsolute,join} from 'node:path';
import {fileURLToPath} from 'node:url';
import {setTimeout as delay} from 'node:timers/promises';
assert.equal(process.platform,'linux');assert.equal(process.versions.node,'24.19.0');process.umask(0o077);
const cfg=JSON.parse(readFileSync(process.argv[2],'utf8'));
for(const k of ['repository','binary','runtimeDir','webDir','evidenceParent','socketParent','provenanceFile','runtimeInputsFile'])assert.ok(isAbsolute(cfg[k]),`${k} must be absolute`);
for(const k of ['binarySHA256','provenanceSHA256','runtimeInputsSHA256','packageIdentity'])assert.match(cfg[k],/^[a-f0-9]{64}$/);
assert.match(cfg.sourceCommit,/^[a-f0-9]{40}$/);
assert.equal(cfg.dockerHost,'unix:///var/run/docker.sock');assert.equal(cfg.acknowledgeFixtureMutation,true);
assert.equal(realpathSync(cfg.evidenceParent),cfg.evidenceParent);assert.equal(realpathSync(cfg.socketParent),cfg.socketParent);
const run=mkdtempSync(join(cfg.evidenceParent,'attempt-'));chmodSync(run,0o700);
const privateDir=join(run,'private'),state=join(privateDir,'state'),projects=join(privateDir,'projects');
for(const p of [privateDir,state,projects])mkdirSync(p,{mode:0o700});
const socketDir=mkdtempSync(join(cfg.socketParent,'ri-'));chmodSync(socketDir,0o700);
const socketPath=join(socketDir,'docker.sock');assert.ok(Buffer.byteLength(socketPath)<100);
const proxyHost=`unix://${socketPath}`;
const tmp=join(socketDir,'tmp');mkdirSync(tmp,{mode:0o700});
const write=(name,value)=>writeFileSync(join(run,name),typeof value==='string'||Buffer.isBuffer(value)?value:JSON.stringify(value,null,2)+'\n',{mode:0o600});
const hash=p=>createHash('sha256').update(readFileSync(p)).digest('hex');
const sha=bytes=>createHash('sha256').update(bytes).digest('hex');
function diagnosticHash(p){try{return hash(p);}catch(e){return {unavailable:e.code??e.message};}}
const started=Date.now(),overallDeadline=started+180_000,workDeadline=overallDeadline-35_000;
const abort=new AbortController();
const result={passed:false,scope:'installed C081 insufficient-disk admission through an explicit read-only Docker observation proxy; not unrestricted installed/runtime-build acceptance',startedAt:new Date().toISOString(),run,socketDir,expected:{firstHTTP:202,status:'blocked',phase:'blocked',reasonCode:'disk_pressure',resourcesHeld:false,mutationAttempts:0},proxy:{mutationAttempts:[],errors:[],requests:0},cleanup:{},reservationObservation:{status:'unknown',basis:'not yet observed'},job:null};
let child,childClosed=false,childError,origin,token,job,requestNumber=0,processWatch,proxyServer,proxyListening=false,logsBytes=0,logsTruncated=false,processScanError,primaryComplete=false;
const ownedProcesses=new Map(),proxySockets=new Set(),upstreams=new Set();
for(const sig of ['SIGINT','SIGTERM'])process.on(sig,()=>{result.cancelSignal=sig;abort.abort(new Error(`Driver received ${sig}`));});
function fence(){abort.signal.throwIfAborted();assert.ok(Date.now()<workDeadline,'Fixture work deadline expired');}
function remaining(cleanup=false,max=5000){return Math.max(1,Math.min(max,(cleanup?overallDeadline-4000:workDeadline)-Date.now()));}
function identity(pid){try{const raw=readFileSync(`/proc/${pid}/stat`,'utf8'),f=raw.slice(raw.lastIndexOf(')')+2).trim().split(/\s+/);return {pid:Number(pid),parentPID:Number(f[1]),startTicks:f[19],state:f[0]};}catch(e){if(['ENOENT','ESRCH'].includes(e.code))return null;throw e;}}
function captureProcesses(){
 if(!child?.pid)return;
 const names=readdirSync('/proc').filter(x=>/^\d+$/.test(x));assert.ok(names.length<10000,'Process inventory too large');
 const rows=new Map(names.map(x=>[Number(x),identity(x)]).filter(([,x])=>x)),selected=new Set();
 const root=rows.get(child.pid),known=ownedProcesses.get(child.pid);
 if(root&&(!known||known.startTicks===root.startTicks)){selected.add(child.pid);ownedProcesses.set(child.pid,root);}
 for(const [pid,before] of ownedProcesses)if(rows.get(pid)?.startTicks===before.startTicks)selected.add(pid);
 let changed=true;while(changed){changed=false;for(const [pid,row] of rows)if(!selected.has(pid)&&selected.has(row.parentPID)){selected.add(pid);ownedProcesses.set(pid,row);changed=true;}}
}
function signalOwned(signal){if(!child||childClosed)return;const actual=identity(child.pid),before=ownedProcesses.get(child.pid);if(!actual)return;assert.ok(before&&actual.startTicks===before.startTicks,'Controller PID ownership changed');child.kill(signal);}
const outer=setTimeout(()=>{result.passed=false;result.error='Three-minute outer deadline; original job/ledger may remain unresolved';result.cleanup.forcedDeadline=true;try{signalOwned('SIGKILL');}catch(e){result.cleanup.signalError=e.message;}result.finishedAt=new Date().toISOString();write('result.json',result);process.exit(124);},180_000);outer.unref();
const envBase={PATH:process.env.PATH,LANG:'C.UTF-8',TZ:'UTC',TMPDIR:tmp,GIT_CONFIG_NOSYSTEM:'1',GIT_OPTIONAL_LOCKS:'0',GIT_CONFIG_GLOBAL:join(privateDir,'empty-git-config'),GIT_TERMINAL_PROMPT:'0',AWS_EC2_METADATA_DISABLED:'true'};
writeFileSync(envBase.GIT_CONFIG_GLOBAL,'',{mode:0o600});
function git(args){fence();const r=spawnSync('git',args,{cwd:cfg.repository,env:envBase,timeout:remaining(false,10000),maxBuffer:2<<20});assert.equal(r.status,0,`git ${args[0]} failed`);return r.stdout;}
async function port(){const server=createServer();await new Promise((res,rej)=>server.once('error',rej).listen(0,'127.0.0.1',res));const p=server.address().port;await new Promise(r=>server.close(r));return p;}
function daemonRequest(method,path,{cleanup=false,timeout=5000}={}){
 assert.ok(method==='GET'||method==='HEAD');assert.ok(path.startsWith('/')&&!path.includes('\r')&&!path.includes('\n'));
 return new Promise((resolve,reject)=>{
  const req=http.request({socketPath:'/var/run/docker.sock',method,path,headers:{Host:'docker','User-Agent':'luci-c081-read-only-smoke'}},res=>{
   let bytes=0;const parts=[];res.on('data',b=>{bytes+=b.length;if(bytes>8<<20){req.destroy(new Error('Docker response exceeds 8 MiB bound'));return;}parts.push(b);});
   res.on('error',reject);res.on('end',()=>resolve({status:res.statusCode,headers:res.headers,body:Buffer.concat(parts)}));
  });
  upstreams.add(req);req.once('close',()=>upstreams.delete(req));req.on('error',reject);req.setTimeout(remaining(cleanup,timeout),()=>req.destroy(new Error('Docker read timed out')));req.end();
 });
}
const trace=row=>{result.proxy.requests++;assert.ok(result.proxy.requests<=4000,'Docker request evidence exceeded bound');appendFileSync(join(run,'docker-requests.jsonl'),JSON.stringify({at:new Date().toISOString(),...row})+'\n',{mode:0o600});};
async function startProxy(){
 proxyServer=http.createServer(async(req,res)=>{
  const record={method:req.method,path:(req.url??'').slice(0,4096)};
  try{
   if(!['GET','HEAD'].includes(req.method)){result.proxy.mutationAttempts.push(record);trace({...record,forwarded:false,status:503,refusal:'fixture forbids and records every mutation attempt'});req.resume();res.writeHead(503,{'Content-Type':'application/json'});res.end('{"message":"fixture forbids Docker mutations; this attempt fails acceptance"}');return;}
   assert.ok((req.url??'').length<=4096,'Docker URL exceeds bound');
   const observed=await daemonRequest(req.method,req.url,{cleanup:true});trace({...record,forwarded:true,status:observed.status,responseBytes:observed.body.length});res.writeHead(observed.status,observed.headers);res.end(observed.body);
  }catch(e){result.proxy.errors.push(e.message);if(!res.headersSent)res.writeHead(502,{'Content-Type':'application/json'});res.end('{"message":"Docker observation transport failed"}');}
 });
 proxyServer.maxConnections=32;proxyServer.requestTimeout=10_000;proxyServer.headersTimeout=5000;
 proxyServer.on('connection',s=>{proxySockets.add(s);s.once('close',()=>proxySockets.delete(s));});
 proxyServer.on('upgrade',(req,socket)=>{const record={method:req.method,path:(req.url??'').slice(0,4096),upgrade:true};result.proxy.mutationAttempts.push(record);trace({...record,forwarded:false,status:503});socket.end('HTTP/1.1 503 Service Unavailable\r\nConnection: close\r\nContent-Length: 0\r\n\r\n');});
 await new Promise((res,rej)=>proxyServer.once('error',rej).listen(socketPath,res));proxyListening=true;chmodSync(socketPath,0o600);
}
async function daemonJSON(path,label,cleanup=false){const r=await daemonRequest('GET',path,{cleanup});write(label+'.http.json',{method:'GET',path,status:r.status});write(label+'.body.json',r.body);assert.equal(r.status,200);return JSON.parse(r.body);}
async function inventory(label,cleanup=false){
 const images=await daemonJSON('/images/json?all=1',label+'-images',cleanup),containers=await daemonJSON('/containers/json?all=1',label+'-containers',cleanup);
 const normalized={images:images.map(x=>({id:x.Id,tags:[...(x.RepoTags??[])].sort(),digests:[...(x.RepoDigests??[])].sort(),size:x.Size,labels:x.Labels??{}})).sort((a,b)=>a.id.localeCompare(b.id)),containers:containers.map(x=>({id:x.Id,imageID:x.ImageID,image:x.Image,state:x.State,names:[...(x.Names??[])].sort(),labels:x.Labels??{}})).sort((a,b)=>a.id.localeCompare(b.id))};
 write(label+'-inventory.json',normalized);return normalized;
}
async function api(method,path,body,{expect=200,timeout=5000,sensitive=false,cleanup=false}={}){
 if(!cleanup)fence();const n=String(++requestNumber).padStart(3,'0'),requestBody=body===undefined?undefined:JSON.stringify(body);
 write(n+'-request.json',{method,path,body:sensitive?'[credentials omitted]':body??null,authenticated:Boolean(token),at:new Date().toISOString()});
 const response=await fetch(origin+path,{method,redirect:'error',signal:cleanup?AbortSignal.timeout(remaining(true,timeout)):AbortSignal.any([abort.signal,AbortSignal.timeout(remaining(false,timeout))]),headers:{...(token?{Authorization:`Bearer ${token}`}:{ }),...(body===undefined?{}:{'Content-Type':'application/json'})},body:requestBody});
 const raw=Buffer.from(await response.arrayBuffer());assert.ok(raw.length<=2<<20,'API response exceeds receipt bound');
 let value;try{value=raw.length?JSON.parse(raw):null;}catch{value=null;}
 write(n+'-response.http.json',{status:response.status,contentType:response.headers.get('content-type'),at:new Date().toISOString(),...(sensitive?{credentialsOmitted:true}:{bodySHA256:sha(raw)})});
 if(sensitive)write(n+'-response.safe.json',{user:value?.user,tokenOmitted:true});else write(n+'-response.body.json',raw);
 assert.ok((Array.isArray(expect)?expect:[expect]).includes(response.status),`${method} ${path}: unexpected HTTP ${response.status}${sensitive?' (sensitive response omitted)':''}`);
 assert.notEqual(value,null,`${method} ${path} response is not JSON`);return value;
}
function regularBytes(path,limit){const st=lstatSync(path);assert.ok(st.isFile()&&!st.isSymbolicLink()&&st.size<=limit,'Private receipt is unsafe/oversized');return readFileSync(path);}
function privateReceipts(label){
 const paths={job:join(state,'sessions','runtime-install.json'),ledger:join(state,'control-plane','execution','leases.json')};const out={};
 for(const [kind,path]of Object.entries(paths)){if(!existsSync(path)){out[kind]={exists:false};continue;}const raw=regularBytes(path,2<<20);write(`${label}-${kind}.original.json`,raw);out[kind]={exists:true,sha256:sha(raw),value:JSON.parse(raw)};}
 write(label+'-private-receipts.json',out);return out;
}
async function until(label,operation,millis=30_000){const end=Math.min(workDeadline,Date.now()+millis);let last;while(Date.now()<end){fence();if(childError)throw childError;if(childClosed)throw new Error('Controller exited before observation');try{const v=await operation();if(v)return v;}catch(e){last=e;}await delay(300);}throw new Error(`${label} timed out${last?': '+last.message:''}`);}
const anchors=['control-plane/cmd/api/main.go','control-plane/cmd/api/config.go','control-plane/cmd/api/execution_storage.go','control-plane/internal/management/runtime.go','control-plane/internal/backend/dockerbe/resource_disk_linux.go','control-plane/internal/backend/dockerbe/resource_observer.go','control-plane/internal/product/runtime_install.go','control-plane/internal/product/setup_tools.go','control-plane/internal/product/setup_storage.go','control-plane/internal/api/runtime_install_execution.go','control-plane/internal/api/execution_runtime_image_leases.go','control-plane/internal/api/execution_capacity.go','control-plane/internal/backend/runtime_image.go','control-plane/internal/backend/dockerbe/runtime_image.go','control-plane/internal/backend/dockerbe/runtime_image_context.go','control-plane/internal/runtimeinstall/runtime.go'];
let initialInventory,binding,runtimeManifest;
try{
 assert.equal(hash(cfg.binary),cfg.binarySHA256);assert.equal(hash(cfg.provenanceFile),cfg.provenanceSHA256);assert.equal(hash(cfg.runtimeInputsFile),cfg.runtimeInputsSHA256);assert.equal(readFileSync(join(dirname(cfg.binary),'package-identity'),'utf8').trim(),cfg.packageIdentity);
 const provenance=JSON.parse(readFileSync(cfg.provenanceFile));assert.equal(provenance.sourceCommit,cfg.sourceCommit);assert.equal(provenance.sourceDirty,false);assert.equal(provenance.binarySHA256,cfg.binarySHA256);assert.equal(provenance.packageIdentity,cfg.packageIdentity);assert.equal(provenance.allMembersMatch,true);
 result.binding={sourceCommit:cfg.sourceCommit,sourceBinding:'Git objects at exact packaged product revision; live checkout is diagnostic only',binarySHA256:cfg.binarySHA256,packageIdentity:cfg.packageIdentity,provenanceSHA256:cfg.provenanceSHA256,runtimeInputsSHA256:cfg.runtimeInputsSHA256,helperSHA256:hash(fileURLToPath(import.meta.url))};
 write('package-provenance.original.json',readFileSync(cfg.provenanceFile));write('runtime-inputs.original.json',readFileSync(cfg.runtimeInputsFile));
 result.observedCheckoutBefore=git(['rev-parse','HEAD']).toString().trim();const status=git(['status','--porcelain=v1','--untracked-files=normal']);write('checkout-status-before.txt',status);
 binding=Object.fromEntries(anchors.map(p=>[p,sha(git(['show',`${cfg.sourceCommit}:${p}`]))]));write('packaged-source-git-object-bindings.json',{sourceCommit:cfg.sourceCommit,paths:binding});
 write('live-checkout-selected-before.json',{scope:'Diagnostic snapshot only; these bytes are not consumed by the installed binary and may change during parallel work',head:result.observedCheckoutBefore,paths:Object.fromEntries(anchors.map(p=>[p,diagnosticHash(join(cfg.repository,p))]))});
 runtimeManifest=JSON.parse(readFileSync(cfg.runtimeInputsFile));assert.equal(runtimeManifest.length,106);for(const row of runtimeManifest){assert.ok(!row.path.startsWith('/')&&!row.path.split('/').includes('..'));assert.equal(hash(join(cfg.runtimeDir,row.path)),row.sha256,`Bundled source changed: ${row.path}`);}
 const info=await daemonJSON('/info','engine-before');assert.equal(info.ID,cfg.engineID);assert.equal(info.Driver,cfg.engineDriver);assert.equal(info.OSType,'linux');
 result.engine={id:info.ID,driver:info.Driver,root:info.DockerRootDir};const fs=statfsSync(info.DockerRootDir);result.diskObservation={at:new Date().toISOString(),path:info.DockerRootDir,availableBytes:fs.bavail*fs.bsize,basis:'read-only host statfs of exact reported local daemon root; not the admission-decision timestamp'};
 assert.ok(result.diskObservation.availableBytes<8*2**30,'This fixture requires current space below the unchanged 8 GiB reserve; no installation request sent');
 initialInventory=await inventory('before');
 const image=`luci/installer-disk-fixture:${randomUUID().replaceAll('-','')}`;result.image=image;const absent=await daemonRequest('GET',`/images/${encodeURIComponent(image)}/json`);write('unique-tag-before.http.json',{status:absent.status,image});assert.equal(absent.status,404,'Unique target tag must not exist');
 await startProxy();fence();
 const apiPort=await port();fence();const browserPort=await port();fence();assert.notEqual(apiPort,browserPort);origin=`http://127.0.0.1:${apiPort}`;
 const configDir=join(privateDir,'docker-config');mkdirSync(configDir,{mode:0o700});
 const env={...envBase,DOCKER_HOST:proxyHost,DOCKER_CONFIG:configDir,LUCI_DOCKER_HOST:proxyHost,LUCI_ADDR:`127.0.0.1:${apiPort}`,LUCI_BROWSER_PREVIEW_ADDR:`127.0.0.1:${browserPort}`,LUCI_BROWSER_PREVIEW_PORT:String(browserPort),LUCI_BACKEND:'docker',LUCI_STATE_DIR:state,LUCI_PROJECTS_DIR:projects,LUCI_EXECUTION_STATE_DIR:join(state,'control-plane','execution'),LUCI_RUNTIME_DIR:cfg.runtimeDir,LUCI_WEB_DIR:cfg.webDir,LUCI_SANDBOX_IMAGE:image,LUCI_REAP_INTERVAL:'30s',LUCI_SESSION_MEMORY_MIB:'1024',LUCI_SESSION_CPU_MILLIS:'1000'};
 assert.equal(hash(cfg.binary),cfg.binarySHA256);fence();write('controller-invocation.json',{command:[cfg.binary],cwd:dirname(cfg.binary),environment:env});
 child=spawn(cfg.binary,[],{cwd:dirname(cfg.binary),env,stdio:['ignore','pipe','pipe']});result.controllerPID=child.pid;
 child.once('error',e=>{childError=e;});child.once('close',(code,signal)=>{childClosed=true;result.controllerExit={code,signal};});
 captureProcesses();processWatch=setInterval(()=>{try{captureProcesses();}catch(e){processScanError=e.message;}},500);processWatch.unref();
 for(const [name,stream]of [['controller.stdout.log',child.stdout],['controller.stderr.log',child.stderr]])stream.on('data',chunk=>{if(logsBytes+chunk.length<=4<<20){appendFileSync(join(run,name),chunk,{mode:0o600});logsBytes+=chunk.length;}else logsTruncated=true;});
 await until('controller readiness',()=>api('GET','/v1/control-plane/status',undefined,{timeout:2000}));
 const bootstrapToken=readFileSync(join(state,'control-plane','bootstrap-token'),'utf8').trim();
 const auth=await api('POST','/v1/auth/bootstrap',{bootstrapToken,username:'installer-disk-owner',displayName:'Installer Disk Fixture',password:randomUUID()+randomUUID()},{expect:201,sensitive:true});token=auth.token;assert.ok(token);assert.equal(auth.user.role,'super-admin');
 const me=await api('GET','/v1/auth/me');assert.equal(me.user.id,auth.user.id);assert.equal(me.user.role,'super-admin');
 const initialJob=await api('GET','/v1/setup/runtime/install');write('initial-job.json',initialJob);assert.equal(initialJob.status,'idle');assert.equal(initialJob.image,image);
 const ready=await until('fresh resource observation',async()=>{const x=await api('GET','/v1/product/swarm-scheduler');const h=x.resources?.host,e=x.resources?.execution;return h?.memory?.effectiveAvailableBytes!=null&&e?.totalMemoryBytes!=null&&e?.cpuMillis>0&&Date.now()-Date.parse(h.observedAt)<5000&&Date.now()-Date.parse(e.observedAt)<5000&&x;});
 write('pre-admission-resource-diagnostics.json',ready);assert.equal(ready.policy.diskReserveMiB,2048,'Do not alter the normal policy for this fixture');privateReceipts('before-request');
 // Exactly one request. Neither a timeout nor a refusal authorizes replay.
 const accepted=await api('POST','/v1/setup/runtime/install',{}, {expect:202,timeout:10_000});write('first-accepted-job.json',accepted);result.acceptedJobID=accepted.id;assert.match(accepted.id,/^runtime-/);assert.equal(accepted.image,image);assert.equal(accepted.status,'installing');assert.equal(accepted.execution?.phase,'queued');
 job=await until('terminal installer outcome',async()=>{const x=await api('GET','/v1/setup/runtime/install');assert.equal(x.id,accepted.id);return !['installing','cancelling','reconciling'].includes(x.status)&&x;},30_000);
 result.job=job;write('original-terminal-job.json',job);
 const diagnostics=await api('GET','/v1/product/swarm-scheduler');write('original-terminal-resource-diagnostics.json',diagnostics);const receipts=privateReceipts('original-terminal');
 const after=await inventory('original-terminal');const target=await daemonRequest('GET',`/images/${encodeURIComponent(image)}/json`);write('unique-tag-original-terminal.http.json',{status:target.status,image});
 // Store all original observations BEFORE assertions and BEFORE controller close.
 write('pre-cleanup-observation.json',{job,diagnostics,proxy:{...result.proxy},targetStatus:target.status,inventoryUnchanged:JSON.stringify(after)===JSON.stringify(initialInventory),receipts:{job:receipts.job?.sha256,ledger:receipts.ledger?.sha256}});primaryComplete=true;
 assert.equal(job.status,'blocked');assert.equal(job.execution?.phase,'blocked');assert.equal(job.execution?.reasonCode,'disk_pressure');assert.equal(job.execution?.resourcesHeld,false);assert.equal(job.execution?.jobId,accepted.id);assert.equal(job.execution?.imageTag,image);assert.ok(!job.execution?.allocationId&&!job.execution?.imageId,'No durable allocation should be acquired before refusal');
 // CleanupConfirmed is not required here: a never-admitted job did not own work
 // to clean. Unknown/missing ledger evidence is never translated into zero.
 assert.ok(result.proxy.requests>0,'Installed service must use the actual read-only proxy');assert.equal(result.proxy.mutationAttempts.length,0);assert.equal(result.proxy.errors.length,0);assert.deepEqual(after,initialInventory);assert.equal(target.status,404);
 assert.ok(receipts.ledger?.exists,'Admission must leave a readable valid ledger');const l=receipts.ledger.value;assert.equal(l.version,1);for(const key of ['leases','checks','previews','runtimeImages'])assert.equal((l[key]??[]).length,0,`Unexpected retained ${key}`);
 assert.deepEqual(receipts.job.value,job);const reservations=diagnostics.resources?.reservations;assert.equal(reservations?.status,'ready');assert.equal(reservations.count,0);assert.equal(reservations.memoryMiB,0);assert.equal(reservations.cpuMillis,0);assert.deepEqual(diagnostics.policy,ready.policy);
 result.reservationObservation={status:'confirmed-zero',basis:'exact original terminal job, readable valid fresh fixture ledger, and ready shared reservation API observation agree',jobStatus:job.status,resourcesHeld:job.execution.resourcesHeld};
 assert.equal(logsTruncated,false);result.refusalProved=true;result.passed=true;
}catch(e){result.error=e.message;result.errorStack=e.stack;process.exitCode=1;}
finally{
 // Preserve original job/diagnostics while the service is still alive, including
 // any unexpected unknown state. No cancel/reconcile/retry can rewrite this test.
 if(token&&child&&!childClosed&&!primaryComplete){try{const current=await api('GET','/v1/setup/runtime/install',undefined,{cleanup:true});result.job=current;write('failure-before-cleanup-job.json',current);const diagnostics=await api('GET','/v1/product/swarm-scheduler',undefined,{cleanup:true});write('failure-before-cleanup-resource-diagnostics.json',diagnostics);}catch(e){result.cleanup.preCloseObservationError=e.message;}}
 try{privateReceipts('before-cleanup');}catch(e){result.cleanup.privateReceiptError=e.message;}
 try{captureProcesses();}catch(e){processScanError=e.message;}
 if(child&&!childClosed){try{signalOwned('SIGTERM');}catch(e){result.cleanup.signalError=e.message;result.passed=false;}const end=Math.min(overallDeadline-10000,Date.now()+8000);while(!childClosed&&Date.now()<end)await delay(100);if(!childClosed){result.cleanup.forcedKill=true;result.passed=false;try{signalOwned('SIGKILL');}catch(e){result.cleanup.killError=e.message;}const hard=Math.min(overallDeadline-5000,Date.now()+2000);while(!childClosed&&Date.now()<hard)await delay(100);}}
 if(processWatch)clearInterval(processWatch);
 const processes=[];for(const[pid,before]of ownedProcesses){try{const current=identity(pid);processes.push({...before,closure:!current||current.startTicks!==before.startTicks?'exact-process-gone':'still-present',current});}catch(e){processes.push({...before,closure:'unreadable',error:e.message});}}
 write('owned-process-closure.json',{processScanError:processScanError??null,processes,scope:'Fresh inline bootstrap owner; no managed worker requested. Descendants sampled every 500 ms; short-lived processes may finish between samples.'});
 if(processScanError||processes.some(x=>x.closure!=='exact-process-gone')||child&&!childClosed){result.cleanup.processClosureUnconfirmed=true;result.passed=false;}
 result.cleanup.controllerExitObserved=child?childClosed:true;
 if(child&&(result.controllerExit?.code!==0||result.controllerExit?.signal)){result.cleanup.nonGracefulControllerExit=true;result.passed=false;}
 // Keep proxy until controller closure so late lifecycle requests are recorded.
 for(const request of upstreams)request.destroy();for(const socket of proxySockets)socket.destroy();
 if(proxyListening){await new Promise(resolve=>proxyServer.close(resolve));result.cleanup.proxyClosed=true;}
 try{if(initialInventory){const final=await inventory('after-controller-close',true);assert.deepEqual(final,initialInventory);result.cleanup.inventoryUnchanged=true;}if(result.image){const image=await daemonRequest('GET',`/images/${encodeURIComponent(result.image)}/json`,{cleanup:true});write('unique-tag-after-close.http.json',{status:image.status,image:result.image});assert.equal(image.status,404);}}catch(e){result.cleanup.finalInventoryError=e.message;result.passed=false;}
 try{const afterCleanup=privateReceipts('after-cleanup');if(result.refusalProved){assert.deepEqual(afterCleanup.job?.value,job);assert.ok(afterCleanup.ledger?.exists);for(const key of ['leases','checks','previews','runtimeImages'])assert.equal((afterCleanup.ledger.value[key]??[]).length,0);}if(binding){const after=Object.fromEntries(anchors.map(p=>[p,diagnosticHash(join(cfg.repository,p))]));write('live-checkout-selected-after.json',{scope:'Diagnostic snapshot only; no parity with packaged source or concurrent working tree is asserted',paths:after});assert.equal(hash(cfg.binary),cfg.binarySHA256);for(const row of runtimeManifest??[])assert.equal(hash(join(cfg.runtimeDir,row.path)),row.sha256);}}catch(e){result.cleanup.sourceOrReceiptError=e.message;result.passed=false;}
 if(result.proxy.mutationAttempts.length||result.proxy.errors.length||logsTruncated||abort.signal.aborted){result.passed=false;}
 result.cleanup.privateStateRetained=true;result.cleanup.noCancelReconcileRetry=true;result.cleanup.noManualDockerMutation=true;result.logsTruncated=logsTruncated;result.finishedAt=new Date().toISOString();clearTimeout(outer);
 if(!result.passed)process.exitCode=1;write('result.json',result);console.log(JSON.stringify({passed:result.passed,result:join(run,'result.json'),status:result.job?.status,reasonCode:result.job?.execution?.reasonCode,error:result.error??null}));
}
