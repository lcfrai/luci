#!/usr/bin/env node
// CLOUD-013 installed same-project DAG witness. Real Docker/native commands; scripted provider.
// Approved context/run seeding is labelled API fixture setup; actual task commands finish naturally before human integration.
import assert from 'node:assert/strict';
import {createAcceptanceJournal,diagnosticError,journalProcessIdentity} from '../launch-recovery/durable-acceptance-journal.mjs';
import {createWorkflowDeadline,boundedAttempt,runBoundedCleanup,writeFailureCheckpoint,settleOwnedBrowser} from '../launch-recovery/acceptance-lifecycle.mjs';
import {createStartupScope,startAcceptanceServices,startAcceptanceBrowser} from '../launch-recovery/acceptance-startup.mjs';
import { spawn, execFileSync } from 'node:child_process';
import { createHash, randomUUID } from 'node:crypto';
import { createServer as httpServer } from 'node:http';
import { createServer } from 'node:net';
import { existsSync, statfsSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, realpathSync, rmSync, lstatSync, statSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve, sep } from 'node:path';
import { fileURLToPath } from 'node:url';
import { setTimeout as delay } from 'node:timers/promises';
import {verifyTaskContextDelivery} from '../task-context-delivery.mjs';
import {recordDependencyProbe,assertDependencyRefusal} from './dependency-probe.mjs';
import {probeStaleReview} from './stale-review-probe.mjs';
import {observeSourceReadCohort,sourceReadUI} from './source-read-cohort.mjs';
import {selectAuthorizedTask,taskFailureDiagnostic,assertExactTaskFailure} from './queued-failure.mjs';
import {checkCommand,validateCheckCreate,assertRecordedCheck,assertCheckCapacity,assertJoinedCheck,assertOwnedCheckContainer} from './project-check-guard.mjs';
import {taskContracts,variants,seedFiles as initialFiles,variantFor,changedPaths,taskCommand,taskHandoff,expectedResult} from './dag-fixture.mjs';
import {contextFilesV1,contextFileNames,dependencyFile,validateContextManifest} from './context-contract.mjs';

const here=dirname(fileURLToPath(import.meta.url)),repo=resolve(process.env.LUCI_ACCEPTANCE_REPO??resolve(here,'../../..')),args=process.argv.slice(2);
assert.equal(realpathSync(here),realpathSync(join(repo,'scripts/acceptance/task-dependencies')),'Executed DAG helper must be inside the declared helper checkout');
const {chromium}=await import(join(repo,'desktop/node_modules/playwright/index.mjs'));
const {validateWorkerCreate}=await import(join(repo,'scripts/acceptance/launch-recovery/worker-create-guard.mjs'));
const {startFencedDockerProxy}=await import(join(repo,'scripts/acceptance/launch-recovery/fenced-docker-proxy.mjs'));
const {createNativeTerminalProbe}=await import(join(repo,'scripts/acceptance/four-project-queue/native-terminal-diagnostics.mjs'));
const option = (name, fallback) => { const i = args.indexOf(name); if (i < 0) return fallback; assert(args[i + 1] && !args[i + 1].startsWith('--'), 'Missing value for ' + name); return args[i + 1]; };
if(args.includes('--help')){console.log('PREPARED ONLY: installed same-project DAG with four exact serial workers and reviewed conflict recovery. Required --binary --web --provenance --expected-source --image --expected-image-id --image-provenance --report FRESH_JSON --fixture-root SHORT_PATH. Root exclusive Docker window and verified clean package required; no model quality or Windows claim.');process.exit(0);}
const binary = resolve(option('--binary', 'artifacts/luci')), web = resolve(option('--web', 'desktop/dist'));
const image = option('--image'), releaseImage = image;
const expectedImage = option('--expected-image-id'); assert.match(expectedImage ?? '', /^sha256:[a-f0-9]{64}$/);
const imageEvidencePath=option('--image-provenance');assert(imageEvidencePath,'Verified candidate image provenance is required');
const imageEvidence=JSON.parse(readFileSync(resolve(imageEvidencePath),'utf8'));assert.equal(imageEvidence.candidateImage,expectedImage);assert.equal(imageEvidence.status,'export-verified-runtime-pending');assert.equal(imageEvidence.exportVerification?.status,'passed');assert.equal(imageEvidence.exportVerification?.candidateImage,expectedImage);assert.deepEqual(imageEvidence.sourcesAfter,imageEvidence.sourcesBefore);
assert(image, 'Supply an existing trusted --image'); assert.equal(process.platform, 'linux'); assert.notEqual(process.getuid(), 0);
assert(existsSync(binary)); assert(existsSync(join(web, 'index.html')));
const sha = bytes => createHash('sha256').update(bytes).digest('hex');
const provenancePath = option('--provenance'); assert(provenancePath, 'Matching clean package provenance is required');
const provenanceBytes = readFileSync(resolve(provenancePath)), provenance = JSON.parse(provenanceBytes);
assert.equal(provenance.sourceDirty, false); assert.equal(provenance.sourceCommit, option('--expected-source'));
assert.match(provenance.sourceCommit, /^[a-f0-9]{40}$/);
const expectedHelperSource=option('--expected-helper-source',provenance.sourceCommit);assert.match(expectedHelperSource,/^[a-f0-9]{40}$/);
const helperCommit=execFileSync('git',['rev-parse','HEAD'],{cwd:repo,encoding:'utf8'}).trim();const helperDirty=!!execFileSync('git',['status','--porcelain'],{cwd:repo,encoding:'utf8'}).trim();assert.equal(helperCommit,expectedHelperSource,'Acceptance helper checkout must match its separately declared source');assert.equal(helperDirty,false,'Acceptance helper source must be clean');
for(const [path,expected] of Object.entries(imageEvidence.sourcesBefore??{})){
 assert(/^(sandboxd\/|images\/sandbox\/)/.test(path)&&!path.split('/').includes('..'));
 assert.equal(sha(execFileSync('git',['show',provenance.sourceCommit+':'+path],{cwd:repo,maxBuffer:32*1024**2})),expected,'Sandbox source differs from packaged source commit: '+path);
}
assert(Object.keys(imageEvidence.sourcesBefore??{}).length>10,'Sandbox source fence missing'); assert.equal(sha(readFileSync(binary)), provenance.binarySHA256); assert.equal(sha(readFileSync(join(web, 'index.html'))), provenance.webIndexSHA256);
const packageIntegrity = JSON.parse(execFileSync('python3', ['-c', `import hashlib,json,pathlib,sys,tarfile
archive=pathlib.Path(sys.argv[1]); root=pathlib.Path(sys.argv[3]).resolve()
digest=hashlib.sha256()
with archive.open('rb') as stream:
 while block:=stream.read(1024*1024): digest.update(block)
if digest.hexdigest()!=sys.argv[2]: raise ValueError('Package archive hash changed')
count=0
with tarfile.open(archive,'r:gz') as source:
 for row in source:
  p=pathlib.PurePosixPath(row.name)
  if p.is_absolute() or '..' in p.parts: raise ValueError('Unsafe package member')
  if row.isdir(): continue
  if not row.isfile(): raise ValueError('Package member is not an ordinary file')
  installed=root.joinpath(*p.parts)
  if installed.is_symlink() or not installed.is_file(): raise ValueError('Installed package member missing or linked')
  a=hashlib.sha256(); b=hashlib.sha256()
  with source.extractfile(row) as stream:
   while block:=stream.read(1024*1024): a.update(block)
  with installed.open('rb') as stream:
   while block:=stream.read(1024*1024): b.update(block)
  if a.digest()!=b.digest(): raise ValueError('Installed package member differs: '+str(p))
  count+=1
print(json.dumps({'archiveSHA256':digest.hexdigest(),'verifiedMembers':count,'allMembersMatch':True}))`, provenance.archive, provenance.archiveSHA256, dirname(binary)], { encoding: 'utf8', timeout: 60000, maxBuffer: 1024 * 1024 }));
assert.equal(realpathSync(web), realpathSync(join(dirname(binary), 'web')));
const rootFree = () => { const volume = statfsSync('/'); return Number(volume.bavail) * Number(volume.bsize); };
const initialRootFree = rootFree(); assert(initialRootFree >= 6 * 1024 ** 3, 'This fixture needs six GiB root free, including a conservative one GiB working reserve');
let cleanupDeadline = Infinity;
const cleanupBudget = maximum => {const remaining = cleanupDeadline - Date.now(); if (remaining <= 0) throw Object.assign(Error('Owned cleanup deadline exceeded'),{code:'cleanup-deadline-exceeded'}); return Math.min(maximum,remaining);};
const docker = (...values) => execFileSync('docker', values, { encoding: 'utf8', timeout: Math.ceil(cleanupBudget(30_000)) }).trim();
assert.equal(docker('ps', '-aq'), '', 'Exclusive runtime admission requires zero existing containers');
assert.equal(docker('volume', 'ls', '-q'), '', 'Exclusive runtime admission requires zero existing volumes');
const imageInfo = JSON.parse(docker('image', 'inspect', image))[0], releaseImageInfo = JSON.parse(docker('image', 'inspect', releaseImage))[0];
assert.match(imageInfo.Id, /^sha256:[a-f0-9]{64}$/); assert.match(releaseImageInfo.Id, /^sha256:[a-f0-9]{64}$/);
assert.equal(imageInfo.Id, expectedImage);
assert.equal(releaseImageInfo.Id, expectedImage);
const bridge = docker('network', 'inspect', 'bridge', '--format', '{{(index .IPAM.Config 0).Gateway}}');
assert.match(bridge, /^(10\.|172\.(1[6-9]|2\d|3[01])\.|192\.168\.)/);
const report = resolve(option('--report', 'artifacts/quality-swarm/dag-installed/attempt-' + Date.now() + '/result.json')), evidence = dirname(report);
if (existsSync(evidence)) assert.equal(readdirSync(evidence).length, 0, 'Use a fresh evidence directory; prior failures must remain');
mkdirSync(evidence, { recursive: true, mode: 0o700 });
const journal=createAcceptanceJournal(join(evidence,'diagnostic-journal.jsonl'));
journal.record('driver-start',{...journalProcessIdentity(),deadlineMs:900000});
process.on('uncaughtExceptionMonitor',(error,origin)=>{try{journal.record('driver-fatal',{...diagnosticError(error),origin});}catch{}});
process.on('exit',exitCode=>{try{journal.record('driver-exit',{exitCode});journal.close();}catch{}});
writeFileSync(join(evidence, 'package-provenance.json'), provenanceBytes, { mode: 0o600, flag: 'wx' });
writeFileSync(join(evidence,'image-provenance.json'),readFileSync(resolve(imageEvidencePath)),{mode:0o600,flag:'wx'});
const driverNames=['dag.mjs','dag-fixture.mjs','context-contract.mjs','dependency-probe.mjs','stale-review-probe.mjs','source-read-cohort.mjs','queued-failure.mjs','project-check-guard.mjs','run-with-observer.py'];
const reusedHelpers=['task-context-delivery.mjs','four-project-queue/native-terminal-diagnostics.mjs','four-project-queue/sample-linux-owned.py','launch-recovery/fenced-docker-proxy.mjs','launch-recovery/worker-create-guard.mjs','launch-recovery/durable-acceptance-journal.mjs','launch-recovery/acceptance-lifecycle.mjs','launch-recovery/acceptance-startup.mjs'];
mkdirSync(join(evidence, 'accepted-harness'), { mode: 0o700 });
for (const name of driverNames) writeFileSync(join(evidence, 'accepted-harness', name), readFileSync(new URL(name, import.meta.url)), { mode: 0o600, flag: 'wx' });
for (const name of reusedHelpers) writeFileSync(join(evidence,'accepted-harness',name.replaceAll('/','_')),readFileSync(join(repo,'scripts/acceptance',name)),{mode:0o600,flag:'wx'});
const fixtureParent = resolve(option('--fixture-root', 'artifacts/quality-swarm/private-fixtures')); mkdirSync(fixtureParent, {recursive:true,mode:0o700});
const fixture = mkdtempSync(join(fixtureParent,'luci-dag-'));
for (const folder of ['home', 'projects', 'state', 'docker']) mkdirSync(join(fixture, folder), { mode: 0o700 });
const key = 'fixture-only-' + randomUUID(), lifecycle = randomUUID() + randomUUID(), password = randomUUID() + randomUUID();
const sensitive = [key, lifecycle, password], scrub = value => sensitive.reduce((text, secret) => text.replaceAll(secret, '[redacted]'), String(value));
const receipt = {
  passed: false, mode:'dag-installed-scripted', startedAt: new Date().toISOString(), sourceCommit: provenance.sourceCommit, sourceDirty: provenance.sourceDirty,
  harnessCheckout: {commit:helperCommit,dirty:helperDirty,expectedSource:expectedHelperSource,productSource:provenance.sourceCommit,separateHelperRevision:helperCommit!==provenance.sourceCommit},
  packageIntegrity, initialRootFreeBytes: initialRootFree,
  resourceScope: 'Exactly four real workers for one DAG (A,B1,B2,join), plus one separately consented whole-build Docker check after integration, all serial; B1 deliberately conflicts, B2 is separately authorized. Zero existing containers and six GiB root admission before every Create, no recovery fences allowed; hard one GiB running reserve. No image pulls, release or cloud allocation.',
  binarySHA256: sha(readFileSync(binary)), webIndexSHA256: sha(readFileSync(join(web, 'index.html'))), driverSHA256: sha(readFileSync(fileURLToPath(import.meta.url))),
  helperSHA256: Object.fromEntries(driverNames.filter(name=>name!=='dag.mjs').map(name => [name, sha(readFileSync(new URL(name, import.meta.url)))])),
  reusedHelperSHA256:Object.fromEntries(reusedHelpers.map(name=>['scripts/acceptance/'+name,sha(readFileSync(join(repo,'scripts/acceptance',name)))])),
  image: { reference: image, id: imageInfo.Id }, releaseImage: { reference: releaseImage, id: releaseImageInfo.Id },
  scope: 'Installed dependency gates, stale-review refusal, real Git conflict, retained attempt, fresh reviewed retry, natural task exits and combined output checks. Scripted provider; no live-model or Windows claim.',
  limitations: ['Existing trusted Docker images; installation is not exercised.', 'Provider output is deliberately scripted; this proves actual CLI, Docker, task coordination and review mechanics. It does not measure model reasoning, production application quality, cloud deployment or Windows behavior.'],
  externalModelCalls: 0, cloudResourcesCreated: 0, setupScaffolding: ['API-seeded personal owner and one deterministic source/check/context fixture','API-configured one-worker policy; browser queue/review/retirement consent','Explicit stale-review and blocked-dependency API probes; no fixture repair of private product ledgers','Local scripted provider, no external model calls'], checks: [], browserMutations: [], browserErrors: [], cleanup: [],
};

const check = (name, details = {}) => { receipt.checks.push({ name, passed: true, ...details }); console.log('PASS ' + name); };
const names = contextFilesV1;
// The private brief is generated from saved state; only seven members are checkout exports.
const canonicalNames = names.filter(name => name !== '.luci/brief.json');
const authorityRoot = '/home/dev/.luci-reviewed-context';
const sockets = new Set(), socketSessions = new Map(), ownedSessions = new Set(), bundles = new Map(), nativeHomes = new Map();
const terminalProbes = [];
let service, proxy, provider, base, env, token, project, projectRoute;const workspace='';
const controllerIdentities=[];let owner;
let stage = 'startup', fixtureError, reserveTimer, deadlineTimer, closing = false, signalFailure, workflowControl, cleanupHardStop;
function setStage(value){health();stage=value;journal.record('driver-stage',{stage});}
let providerCalls = 0, auxiliaryCalls = 0; const observations=new Map(), taskData=new Map(), calls=new Map(); let browser, page, run;
let checkConsent=null,checkIdentity=null,fixtureOwnerID;
let providerBase; const initialBundles=new Map(), projectStates=[], titles=new Map();
const started = Date.now(), deadline = started + 15 * 60_000;
const remaining = () => { if (Date.now() >= deadline) throw Error('Fifteen minute acceptance deadline expired'); return deadline - Date.now(); };
const remainingBudget = (ms = 120_000) => Math.max(1, Math.min(ms, remaining()));
const redactError = error => scrub(error?.stack ?? error).replaceAll(fixture, '[private-fixture]').slice(0, 8000);
let sourceReadCohort;
function stopOnFailure(error) { sourceReadCohort?.cancel(error); if (!fixtureError) {fixtureError = error;try{journal.record('driver-background-failure',{stage,...diagnosticError(error)});}catch{}} workflowControl?.fail(error); for (const socket of sockets) socket.close(); if (service?.exitCode === null) service.kill('SIGTERM'); }
function processIdentity(pid) {
  try { const data = readFileSync(`/proc/${pid}/stat`, 'utf8'); const fields = data.slice(data.lastIndexOf(')') + 2).trim().split(/\s+/); return {pid:Number(pid),start:fields[19],group:Number(fields[2]),status:fields[0]}; }
  catch (error) { if (['ENOENT','ESRCH'].includes(error.code)) return null; throw error; }
}
function stillOwned(identity) { const now = processIdentity(identity.pid); return now && now.start === identity.start && now.status !== 'Z'; }
function ownedProcesses(family='service') {
  const state = join(fixture, 'state'), rows = [];
  for (const pid of readdirSync('/proc').filter(value => /^\d+$/.test(value))) {
    const identity = processIdentity(pid); if (!identity || identity.status === 'Z') continue;
    try {
      const values = Object.fromEntries(readFileSync(`/proc/${pid}/environ`, 'utf8').split('\0').filter(Boolean).map(value => {const i=value.indexOf('=');return [value.slice(0,i),value.slice(i+1)];}));
      if ((family==='browser'?values.LUCI_SWARM_BROWSER_FIXTURE===fixture:(values.LUCI_STATE_DIR === state || values.LUCI_STATE_DIR?.startsWith(state + '/control-plane/'))) && processIdentity(pid)?.start === identity.start) rows.push(identity);
    } catch (error) { if (!['ENOENT','ESRCH','EACCES','EPERM'].includes(error.code)) throw error; }
  }
  return rows;
}
async function api(method, path, body, expected = 200, timeout = 30_000, observeResponse) {
  if (!closing) health(); else if (!['GET','HEAD'].includes(method)) throw Error('Workflow mutation refused during cleanup');
  const response = await fetch(base + path, {method,redirect:'error',signal:AbortSignal.timeout(closing ? timeout : remainingBudget(timeout)),headers:{Origin:base,...(token?{Authorization:'Bearer '+token}:{}),...(body===undefined?{}:{'Content-Type':'application/json'})},body:body===undefined?undefined:JSON.stringify(body)});
  (receipt.apiActions ??= []).push({stage,method,path,status:response.status,at:new Date().toISOString()});
  const raw = await boundedResponse(response,2*1024**2); let value; try { value = raw ? JSON.parse(raw) : {}; } catch { throw Error('Invalid JSON response: '+method+' '+path+' '+response.status); }
  observeResponse?.(value,response.status);
  assert.equal(response.status, expected, method+' '+path+': '+response.status+' '+scrub(value.message??value.error??''));
  health();return value;
}
async function until(label, fn, ms = 120_000) {
  const end = Date.now() + remainingBudget(ms);
  while (Date.now() < end) { if (fixtureError) throw fixtureError; if (service && service.exitCode !== null) throw Error('Owned service exited unexpectedly'); const value=await fn(); if(value)return value; await delay(150); }
  throw Error(label+' did not complete within its bounded prerequisite window');
}
async function freePort() { health();const server=createServer();await new Promise((done,reject)=>server.once('error',reject).listen(0,'127.0.0.1',done));let port;try{health();port=server.address().port;}finally{await new Promise(done=>server.close(done));}health();return port; }
async function startService() {
  health(); // Fence the actual spawn even if a startup factory resolved during cleanup.
  service=spawn(binary,[],{cwd:fixture,env,detached:true,stdio:['ignore','ignore','ignore']});service.on('error',stopOnFailure);controllerIdentities.push(processIdentity(service.pid));journal.record('controller-start',journalProcessIdentity(service.pid));service.on('exit',(exitCode,signal)=>{try{journal.record('controller-exit',{pid:service.pid,exitCode,signal});}catch{}});
  const end=Date.now()+remainingBudget(45_000);let ready;
  while(Date.now()<end) { health();assert.equal(service.exitCode,null);try{const res=await fetch(base+'/v1/control-plane/status',{signal:AbortSignal.timeout(1500)});if(res.ok){ready=await res.json();health();break;}}catch{}health();await delay(150); }
  health();assert.equal(ready?.product,'Luci');assert.equal(ready?.version,'0.5.0');
  (receipt.lifecycle??=[]).push({action:'start',pid:service.pid,at:new Date().toISOString()});
}
async function stopService(strict=true) {
  if(!service?.pid)return true;
  const identities=ownedProcesses();
  if(service.exitCode===null){assert(identities.some(row=>row.pid===service.pid),'Owned service identity missing');service.kill('SIGTERM');}
  const end=Date.now()+20_000;
  while(Date.now()<end&&(identities.some(stillOwned)||ownedProcesses().length))await delay(100);
  const remainingRows=[...identities.filter(stillOwned),...ownedProcesses()];
  if(remainingRows.length){
    receipt.cleanup.push('Owned service required force termination; acceptance fails');receipt.passed=false;process.exitCode=1;
    for(const row of remainingRows)if(stillOwned(row)){try{process.kill(row.group===row.pid?-row.pid:row.pid,'SIGKILL');}catch(error){if(error.code!=='ESRCH')throw error;}}
    const final=Date.now()+5000;while(Date.now()<final&&remainingRows.some(stillOwned))await delay(100);
  }
  const stopped=!identities.some(stillOwned)&&ownedProcesses().length===0;
  (receipt.lifecycle??=[]).push({action:'stop',at:new Date().toISOString(),stopped,exitCode:service.exitCode,forced:remainingRows.length>0});
  if(strict){assert(stopped);assert.equal(remainingRows.length,0);assert.equal(service.exitCode,0);}
  return stopped;
}
function registrySessions(){return JSON.parse(readFileSync(join(fixture,'state/sessions/sessions.json'),'utf8')).sessions;}
function row(id){const rows=registrySessions().filter(value=>value.id===id);assert.equal(rows.length,1);return rows[0];}
function safeRow(value){return Object.fromEntries(['id','projectId','worktreeId','status','sandboxId','sandboxRef','conversationId','reviewedContext','implementationContext'].map(name=>[name,value[name]]));}
function containerIDs(id){assert.match(id,/^s-[a-f0-9]{8}$/);return docker('ps','-aq','--filter','label=luci.session-id='+id).split(/\s+/).filter(Boolean);}
function inspectOwned(id,sessionID,requireBundle=true){
  assert.match(id,/^[a-f0-9]{12,64}$/);assert(ownedSessions.has(sessionID));
  const value=JSON.parse(docker('inspect',id))[0];
  assert.equal(value.Config.Labels['luci.session-id'],sessionID);assert.equal(value.Config.Labels['luci.sandbox-id'],sessionID);assert.equal(value.Config.Labels['luci.managed'],'true');assert.equal(value.Image,imageInfo.Id);
  const instance=readFileSync(join(fixture,'state/instance/instance-id'),'utf8').trim();assert.equal(value.Config.Labels['luci.instance'],instance);
  const work=value.Mounts.filter(m=>m.Destination==='/home/dev/work');assert.equal(work.length,1);assert.equal(work[0].RW,true);const expected=[...taskData.values()].find(value=>value.session.id===sessionID);assert(expected);assert.equal(realpathSync(work[0].Source),realpathSync(expected.checkout));assert.notEqual(realpathSync(work[0].Source),realpathSync(expected.owner.path));assert(lstatSync(join(work[0].Source,'.git')).isDirectory());assert(!existsSync(join(work[0].Source,'.git/commondir')));assert(!existsSync(join(work[0].Source,'.git/objects/info/alternates')));assert(!value.Mounts.some(m=>m.Source.includes('docker.sock')));
  assert(expected.requestedLimits.memoryMiB>0&&expected.requestedLimits.cpuMillis>0);assert.equal(value.HostConfig.Memory,expected.requestedLimits.memoryMiB*1024**2);assert.equal(value.HostConfig.NanoCpus,expected.requestedLimits.cpuMillis*1_000_000);assert.equal(value.HostConfig.PidsLimit,256);assert.equal(value.HostConfig.Privileged,false);assert(value.HostConfig.SecurityOpt.includes('no-new-privileges:true'));
  const proof={resourceLimits:{memoryBytes:value.HostConfig.Memory,nanoCPUs:value.HostConfig.NanoCpus,pidsLimit:value.HostConfig.PidsLimit,privileged:value.HostConfig.Privileged,securityOpt:value.HostConfig.SecurityOpt},worktreeID:row(sessionID).worktreeId,id:value.Id,image:value.Image,pid:value.State.Pid,running:value.State.Running,startedAt:value.State.StartedAt,created:value.Created,readOnly:true,sessionID};
  if(!requireBundle)return proof; // Cleanup authority uses labels/image/worktree, not the feature assertion under test.
  const output=value.Mounts.filter(m=>m.Destination==='/home/dev/.luci-task-output');assert.equal(output.length,1);assert.equal(output[0].RW,true);assert.equal(realpathSync(output[0].Source),join(fixture,'state/conversations/local/.task-output',sessionID));
  const native=value.Mounts.filter(m=>m.Destination==='/home/dev/.luci-agent');assert.equal(native.length,1);assert.equal(native[0].RW,true);const nativeRoot=realpathSync(native[0].Source);assert.equal(nativeRoot,resolve(native[0].Source));assert(nativeRoot.startsWith(join(fixture,'state')+sep));if(nativeHomes.has(sessionID))assert.equal(nativeHomes.get(sessionID),nativeRoot);nativeHomes.set(sessionID,nativeRoot);
  const mounts=value.Mounts.filter(m=>m.Destination===authorityRoot);assert.equal(mounts.length,1);const mount=mounts[0];assert.equal(mount.Type,'bind');assert.equal(mount.RW,false);
  const root=realpathSync(mount.Source);assert.equal(root,resolve(mount.Source));assert(root.startsWith(join(fixture,'state')+sep));assert(root.endsWith('/.reviewed-context/'+sessionID));assert(!root.startsWith(realpathSync(expected.owner.path)+sep));
  bundles.set(sessionID,root);
  return proof;
}
function live(id){const ids=containerIDs(id);assert.equal(ids.length,1);const proof=inspectOwned(ids[0],id);assert(proof.running&&proof.pid>0);return proof;}
function bundleSnapshot(id){
  const root=bundles.get(id);assert(root);assert.equal(realpathSync(root),root);const files={};
  const manifest=privateJSON(join(root,'manifest.json'));const capturedNames=contextFileNames(manifest.version);
  for(const name of [...capturedNames,'manifest.json']){const path=join(root,name);const info=lstatSync(path);assert(info.isFile()&&!info.isSymbolicLink()&&info.nlink===1);assert(info.size<=32*1024**2);files[name]={sha256:sha(readFileSync(path)),bytes:info.size,inode:String(info.ino),device:String(info.dev)};}
  assert.deepEqual(readdirSync(root).sort(),['.luci','LUCI.md','manifest.json'].sort());assert.deepEqual(readdirSync(join(root,'.luci')).sort(),capturedNames.filter(n=>n.startsWith('.luci/')).map(n=>n.slice(6)).sort());
  return files;
}
function hashes(snapshot){return Object.fromEntries(Object.entries(snapshot).map(([name,value])=>[name,value.sha256]));}
function canonical(){return Object.fromEntries(canonicalNames.map(name=>[name,sha(readFileSync(join(project.path,name)))]));}
function effects(){assert.equal(proxy.observations.dropped,0,'Lost transport observations invalidate zero-side-effect claims');return {create:proxy.observations.counts.create,start:proxy.observations.counts.start,destroy:proxy.observations.requests.filter(r=>r.method==='DELETE'&&/^\/v\d+\.\d+\/containers\/[^/]+$/.test(r.path)).length};}
// Read-only, allowlisted measurement from this fixture's own durable ledger.
// Timestamps bound publication between samples; they are not an exact release
// instant. Missing/changed schema or an unreadable file stays unavailable.
function observeTaskLease(worker){
 const observation=worker.leaseObservation??={scope:'Exact task session and sandbox in the fixture-owned private execution ledger',firstPresentAt:null,lastPresentAt:null,firstAbsentAt:null,samples:0,unavailableSamples:0};
 if(observation.firstAbsentAt)return observation;
 const at=new Date().toISOString();observation.samples++;
 try{
  const path=join(fixture,'state/control-plane/execution/leases.json'),info=lstatSync(path);
  assert(info.isFile()&&!info.isSymbolicLink()&&info.nlink===1&&info.size<=2*1024**2);
  const bytes=readFileSync(path);assert(bytes.length<=2*1024**2);
  const ledger=JSON.parse(bytes);assert.equal(ledger.version,1);assert(Array.isArray(ledger.leases)&&ledger.leases.length<=4096);
  const rows=ledger.leases.filter(value=>value.sessionId===worker.session.id);
  assert(rows.length<=1);if(rows.length){assert.equal(rows[0].sandboxId,worker.session.id);assert.equal(rows[0].projectId,worker.session.projectId);}
  if(rows.length){observation.firstPresentAt??=at;observation.lastPresentAt=at;}
  else observation.firstAbsentAt??=at;
 }catch{observation.unavailableSamples++;}
 return observation;
}
async function completeNaturally(worker,owner){
 assert(observations.has(worker.task));assert.equal(worker.codingDispatches,1);
 observeTaskLease(worker);
 worker.finalReleased=true;for(const finish of [...(worker.finalWaiters??[])])finish(true);
 const stopped=await until('Native one-shot task exits without a stop request',async()=>{const value=await api('GET',workspace+'/v1/sessions/'+worker.session.id);observeTaskLease(worker);return value.status==='exited'&&containerIDs(worker.session.id).length===0?value:null;},90_000);
 const stoppedAndAbsentObservedAt=new Date().toISOString();
 const probe=[...terminalProbes].reverse().find(value=>value.id===worker.session.id)?.probe;
 assert(probe,'Authenticated terminal observation must exist');
 await until('Clean native exit frame',()=>probe.snapshot().exitFrames>0,10_000);
 assert.deepEqual(probe.snapshot().exit,{classification:'exit-status',code:0},'Native CLI must report explicit exit status 0, not be killed or fail');assert.equal(probe.snapshot().trustEnterSent,false,'One-shot execution must not depend on fixture keystrokes');
 const completed=await until('Queue records natural task completion',async()=>{const value=await api('GET',owner.route+'/swarm-queue');return value.items.find(item=>item.childId===worker.session.id&&item.state==='completed');},30_000);
 const queueCompletedObservedAt=new Date().toISOString();observeTaskLease(worker);
 // The next queued clone can be allocating now. Inspect only this fixture's
 // retained receipt; the authenticated queue above remains the progress API.
 const current=privateRun(owner);
 const task=current.swarm.tasks.find(value=>value.id===worker.taskId);assert.notEqual(task.integration?.state,'integrated');
 assert(!receipt.apiActions.some(value=>value.method==='DELETE'&&value.path===workspace+'/v1/sessions/'+worker.session.id),'Success workflow must not issue a worker stop');
 (receipt.naturalCompletions??=[]).push({task:worker.task,sessionID:worker.session.id,sessionStatus:stopped.status,recordedExitReason:stopped.exitReason,cleanNativeExitFrame:true,exitFrameObservedAt:probe.snapshot().exitObservedAt,stoppedAndAbsentObservedAt,queueCompletedObservedAt,leaseObservation:worker.leaseObservation,queueState:completed.state,codingDispatches:worker.codingDispatches,finalResponseRequests:worker.finalResponseRequests,finalResponseAborts:worker.finalResponseAborts??0,finalResponseRetries:(worker.finalResponseRequests??1)-1,integratedBeforeExit:false});
 return completed;
}

function streamResponse(res,item,sequence){
  health();
  const response={id:'resp_swarm_'+sequence,object:'response',created_at:1,status:'completed',model:'swarm-mechanics-fixture',output:[item],usage:{input_tokens:1,output_tokens:1,total_tokens:2,input_tokens_details:{cached_tokens:0},output_tokens_details:{reasoning_tokens:0}}};
  const events=[{type:'response.created',response:{...response,status:'in_progress',output:[]}},{type:'response.output_item.added',output_index:0,item:{...item,status:'in_progress',...(item.type==='function_call'?{arguments:''}:{content:[]})}}];
  if(item.type==='function_call')events.push({type:'response.function_call_arguments.delta',item_id:item.id,output_index:0,delta:item.arguments},{type:'response.function_call_arguments.done',item_id:item.id,output_index:0,arguments:item.arguments});
  else events.push({type:'response.content_part.added',item_id:item.id,output_index:0,content_index:0,part:{type:'output_text',text:'',annotations:[]}},{type:'response.output_text.delta',item_id:item.id,output_index:0,content_index:0,delta:item.content[0].text},{type:'response.output_text.done',item_id:item.id,output_index:0,content_index:0,text:item.content[0].text});
  events.push({type:'response.output_item.done',output_index:0,item},{type:'response.completed',response});res.writeHead(200,{'content-type':'text/event-stream'});res.end(events.map((e,i)=>'event: '+e.type+'\ndata: '+JSON.stringify({...e,sequence_number:i})+'\n\n').join(''));
}
async function attach(id){
  const path=workspace+'/v1/sessions/'+id+'/attach';const {ticket}=await api('POST','/v1/auth/socket-ticket',{path});sensitive.push(ticket);
  health();const socket=new WebSocket(base.replace('http:','ws:')+path+'?ticket='+encodeURIComponent(ticket));socket.binaryType='arraybuffer';sockets.add(socket);socketSessions.set(socket,id);
  assert(terminalProbes.length<8);const probe=createNativeTerminalProbe();terminalProbes.push({id,attachedStage:stage,probe});
  socket.addEventListener('message',event=>probe.frame(Buffer.from(event.data)));
  socket.addEventListener('close',event=>{probe.close(event.code,event.wasClean);sockets.delete(socket);socketSessions.delete(socket);});socket.addEventListener('error',()=>probe.error());
  await new Promise((done,reject)=>{const timeout=setTimeout(()=>reject(Error('Authenticated attach timed out')),remainingBudget(10_000));socket.addEventListener('open',()=>{clearTimeout(timeout);try{health();socket.send(Uint8Array.from([2,0,120,0,40]));done();}catch(error){socket.close();reject(error);}},{once:true});socket.addEventListener('error',()=>{clearTimeout(timeout);reject(Error('Authenticated attach failed'));},{once:true});});
  return socket;
}

function outputs(input,result=[]) {if(!input||typeof input!=='object')return result;if(input.type==='function_call_output')result.push(input);for(const v of Array.isArray(input)?input:Object.values(input))outputs(v,result);assert(result.length<=100);return result;}
function message(text){return {id:'msg_swarm_'+providerCalls,type:'message',status:'completed',role:'assistant',content:[{type:'output_text',text,annotations:[]}]};}
function commandFor(key){const value=taskData.get(key);assert(value);return taskCommand({variant:key,operation:value.operation.id,projectId:value.owner.id,runId:owner.run.id,baseCommit:value.operation.baseCommit,contextHashes:hashes(initialBundles.get(value.session.id)),dependencyDigest:value.operation.dependencyContext.digest,dependencies:value.dependencies,expectedUid:process.getuid()});}
function nextTool(state,name,args){health();if(name==='exec_command'){state.codingDispatches=(state.codingDispatches??0)+1;assert.equal(state.codingDispatches,1,'No duplicate task command');}assert(++state.count<=48,'Bounded task tool request cap');const callID='call_swarm_'+state.task+'_'+state.operation.id+'_'+state.count;calls.set(callID,{state,consumed:false});return {id:'fc_'+callID,type:'function_call',status:'completed',name,call_id:callID,arguments:JSON.stringify(args)};}
function startProvider(){
 health();provider=httpServer(async(req,res)=>{try{
  assert.equal(req.method,'POST');assert.equal(req.headers.authorization,'Bearer '+key);if(closing){res.writeHead(503);res.end();return;}
  assert.equal(req.url,'/v1/responses');const parts=[];let bytes=0;for await(const part of req){bytes+=part.length;assert(bytes<4*1024**2);parts.push(part);}health();const body=JSON.parse(Buffer.concat(parts));assert.equal(body.model,'swarm-mechanics-fixture');assert(++providerCalls<=80);
  const resultItems=outputs(body.input),pending=resultItems.filter(v=>calls.has(v.call_id)&&!calls.get(v.call_id).consumed);assert(pending.length<=1);let response;
  if(pending.length){const output=pending[0],entry=calls.get(output.call_id),state=entry.state;entry.consumed=true;const raw=typeof output.output==='string'?output.output:JSON.stringify(output.output);assert(!/Process exited with code (?:[1-9]\d*|-\d+)|"exit_code"\s*:\s*(?:[1-9]\d*|-\d+)|Command failed/.test(raw));
   const matches=[...raw.matchAll(/LUCI_DAG_OBSERVATION=([A-Za-z0-9+/=]+)/g)];
   if(matches.length){assert.equal(matches.length,1);assert(!observations.has(state.task));const actual=JSON.parse(Buffer.from(matches[0][1],'base64'));assert.equal(actual.variant,state.task);assert.equal(actual.taskId,state.taskId);assert.equal(actual.operation,state.operation.id);assert.equal(actual.uid,process.getuid());assert.equal(actual.cwd,'/home/dev/work');assert.equal(actual.readOnly,true);assert.deepEqual(actual.reported,taskHandoff(state.task));assert.deepEqual(hashes(actual.files),hashes(initialBundles.get(state.session.id)));assert.deepEqual(actual.changed,changedPaths(state.task));assert.equal(actual.artifact.baseCommit,state.operation.baseCommit);assert.equal(actual.artifact.check.code,0);assert.equal(actual.artifact.check.stdout,'DAG_CHECK_OK '+state.task+'\n');assert.match(actual.commit,/^[a-f0-9]{40,64}$/);observations.set(state.task,actual);entry.finalTask=state.task;entry.finalOutputSHA256=sha(raw);state.finalResponseRequests=1;response=message('Controlled DAG '+state.task+' committed. Human integration and build verification remain separate.');}
   else{const running=raw.match(/Process running with session ID (\d+)/)||raw.match(/"session_id"\s*:\s*(\d+)/);assert(running,'Actual command output must identify live command or completion');response=nextTool(state,'write_stdin',{session_id:Number(running[1]),chars:'',yield_time_ms:1000,max_output_tokens:12000});}
  }else if(resultItems.some(value=>calls.get(value.call_id)?.finalTask)){
   const finals=resultItems.filter(value=>calls.get(value.call_id)?.finalTask);assert.equal(finals.length,1);const output=finals[0],entry=calls.get(output.call_id),state=entry.state,raw=typeof output.output==='string'?output.output:JSON.stringify(output.output);assert.equal(sha(raw),entry.finalOutputSHA256);assert(observations.has(state.task));assert.equal(state.codingDispatches,1);assert(state.finalResponseRequests<3);state.finalResponseRequests++;response=message('Controlled DAG '+state.task+' committed. Human integration remains separate.');
  }else if((body.tools??[]).some(t=>(t.name??t.function?.name)==='exec_command')){
   const text=JSON.stringify(body.input);assert(text.includes('Reviewed context directory: '+authorityRoot));const matched=[...titles].filter(([id,title])=>text.includes(id+': '+title));assert.equal(matched.length,1);const taskId=matched[0][0];
   const state=await until('Verified exact task bundle before native command',()=>[...taskData.values()].find(value=>value.taskId===taskId&&value.count===0&&value.attached===true&&initialBundles.has(value.session.id)),120000);response=nextTool(state,'exec_command',commandFor(state.task));
  }else{assert(++auxiliaryCalls<=4);response=message('Controlled DAG fixture acknowledgement.');}
  (receipt.providerRequests??=[]).push({number:providerCalls,toolOutputCount:resultItems.length,issuedTool:response.type==='function_call'?response.name:null});streamResponse(res,response,providerCalls);
 }catch(error){stopOnFailure(error);if(!res.headersSent)res.writeHead(500);res.end('Controlled DAG fixture contract failed');}});
}
async function action(path,click,expected=200){health();const pending=page.waitForResponse(r=>new URL(r.url()).pathname===path&&r.request().method()==='POST',{timeout:remainingBudget(150000)});void pending.catch(()=>{});await click();health();const response=await pending;health();assert.equal(response.status(),expected,path+': '+await response.text());return response.json();}
async function openProject(state) {
 await page.goto(base);
 const username=page.getByLabel('Username',{exact:true}),enter=page.getByRole('button',{name:'Continue to projects',exact:true}),picker=page.getByLabel('Workspace',{exact:true});
 // goto completes before React has resolved the account state. Wait for an
 // actual rendered entry state before deciding whether sign-in is necessary.
 await until('Rendered account entry',async()=>await username.isVisible()||await enter.isVisible()||await picker.isVisible());
 if(await username.isVisible()){
  await username.fill('swarmowner');await page.getByLabel('Password',{exact:true}).fill(password);
  await action('/v1/auth/login',()=>page.getByRole('button',{name:'Sign in',exact:true}).click());
 }
 await until('Workspace entry',async()=>await enter.isVisible()||await picker.isVisible());if(await enter.isVisible())await enter.click();await picker.selectOption('');
 await page.getByRole('button',{name:'All projects',exact:true}).click();await page.getByRole('heading',{name:'Across your projects',exact:true}).waitFor();
 if(state){project=state.owner;projectRoute=state.route;run=state.run;await page.getByRole('button',{name:'Open project '+state.owner.name,exact:true}).click();await page.getByRole('button',{name:'Develop',exact:true}).click();await page.getByRole('region',{name:'Agent team',exact:true}).waitFor();}
}
function taskRow(task){return page.locator('.project-swarm-tasks > li').filter({has:page.getByRole('heading',{name:titles.get(task),exact:true})});}
// Current task-card secondary actions are intentionally disclosed per task.
async function openTaskActions(taskID){
 const summary=taskRow(taskID).locator('summary').filter({hasText:/^More task actions$/});assert.equal(await summary.count(),1);await summary.waitFor();
 if(!await summary.evaluate(node=>node.parentElement.tagName==='DETAILS'&&node.parentElement.open))await summary.click();
 assert.equal(await summary.evaluate(node=>node.parentElement.tagName==='DETAILS'&&node.parentElement.open),true);
}
async function queuedChild(key){
 const contract=variants[key];assert(contract);const authorization=owner.queueAuthorizations?.get(contract.taskId);assert(authorization,'The exact task needs a retained explicit queue authorization');
 const queued=await until('Exact authorized DAG child '+key,async()=>{
  const queue=await api('GET',owner.route+'/swarm-queue');
  const candidate=selectAuthorizedTask(queue.items,authorization,[...taskData.values()].map(value=>value.session.id));
  if(candidate?.state==='failed'){
   const input={variant:key,authorization,queueItem:candidate,expectedAttempt:contract.attempt};
   const diagnostic=taskFailureDiagnostic(input),path=join(evidence,'queued-failure-'+key+'.json');
   (receipt.queuedFailures??=[]).push(diagnostic);writeFileSync(path,JSON.stringify(diagnostic,null,2)+'\n',{mode:0o600,flag:'wx'});
   // Preserve queue evidence even if either subsequent read or identity check fails.
   const failedSession=await api('GET','/v1/sessions/'+candidate.childId);
   Object.assign(diagnostic,taskFailureDiagnostic({...input,session:failedSession}));
   writeFileSync(path,JSON.stringify(diagnostic,null,2)+'\n',{mode:0o600});
   const failedRun=(await api('GET',owner.route+'/runs/'+owner.run.id)).run;
   Object.assign(diagnostic,taskFailureDiagnostic({...input,session:failedSession,run:failedRun}));
   writeFileSync(path,JSON.stringify(diagnostic,null,2)+'\n',{mode:0o600});
   assertExactTaskFailure(diagnostic);diagnostic.classification='exact-authorized-task-failed';
   writeFileSync(path,JSON.stringify(diagnostic,null,2)+'\n',{mode:0o600});
   throw Error('Exact authorized DAG child '+key+' failed: '+diagnostic.queue.reasonCode+'; retained public queue/session/operation diagnostic. No retry was authorized.');
  }
  return candidate;
 },150000);
 const session=await api('GET','/v1/sessions/'+queued.childId);ownedSessions.add(session.id);owner.run=(await api('GET',owner.route+'/runs/'+owner.run.id)).run;run=owner.run;
 const operation=run.operations.find(op=>op.taskId===contract.taskId&&op.childId===session.id);assert(operation);assert.equal(operation.attempt,contract.attempt);const worktrees=(await api('GET','/v1/projects/'+owner.owner.id+'/worktrees')).worktrees,checkout=worktrees.find(w=>w.id===operation.worktreeId);assert(checkout);
 const state={task:key,taskId:contract.taskId,owner:owner.owner,operation,session,checkout:checkout.path,count:0,requestedLimits:{memoryMiB:queued.memoryMiB,cpuMillis:queued.cpuMillis}};taskData.set(key,state);
 const actual=live(session.id),bundle=bundleSnapshot(session.id);const contextDelivery=verifyTaskContextDelivery({session,stored:row(session.id),run:owner.run,operation,manifestBytes:readFileSync(join(bundles.get(session.id),'manifest.json')),version:2});assert.equal(contextDelivery.manifestDigest,bundle['manifest.json'].sha256);initialBundles.set(session.id,bundle);state.container=actual;
 state.dependencies=taskContracts.find(task=>task.id===contract.taskId).dependsOn.map(id=>{const task=run.swarm.tasks.find(task=>task.id===id),op=run.operations.find(op=>op.id===task.operationId),variant=id==='task-a'?'a':'b2';assert.equal(task.integration.state,'integrated');return {taskId:id,operationId:op.id,commit:task.integration.commit,sourceCommit:task.integration.sourceCommit,childId:op.childId,contextDigest:initialBundles.get(op.childId)['manifest.json'].sha256,reportDigest:op.handoff?.digest,report:taskHandoff(variant)};});
 const manifest=privateJSON(join(bundles.get(session.id),'manifest.json')),packet=privateJSON(join(bundles.get(session.id),dependencyFile),8*1024**2);validateContextManifest(manifest,bundle,{expectedVersion:2,projectId:owner.owner.id,runId:run.id,taskId:contract.taskId,operationId:operation.id,dependencyDigest:operation.dependencyContext?.digest,dependencies:state.dependencies},packet);
 (receipt.dependencyPackets??=[]).push({variant:key,version:manifest.version,packet,manifestDigest:bundle['manifest.json'].sha256,dependencyContextDigest:manifest.binding.dependencyContextDigest});
 for(const name of canonicalNames.filter(name=>name!=='LUCI.md'))assert.equal(bundle[name].sha256,owner.canonicalBefore[name]);const capturedRoot=bundles.get(session.id),luci=readFileSync(join(capturedRoot,'LUCI.md'));assert(luci.subarray(0,owner.canonicalLuci.length).equals(owner.canonicalLuci));for(const name of ['.luci/brief.json','.luci/specification.json','.luci/design.json','.luci/plan.json'])assert.equal(JSON.parse(readFileSync(join(capturedRoot,name),'utf8')).projectId,owner.owner.id);assert.deepEqual(row(session.id).implementationContext.taskIds,[contract.taskId]);assert.equal(workerContainers().length,1);
 (receipt.contextCaptures??=[]).push({variant:key,taskId:contract.taskId,operationID:operation.id,childID:session.id,baseCommit:operation.baseCommit,reviewedContextDelivery:contextDelivery.delivery,reviewedContextReference:contextDelivery.reference,hashes:hashes(bundle)});await attach(session.id);state.attached=true;return state;
}
async function setPaused(paused){
 const settings=await api('GET','/v1/product/swarm-scheduler');
 return api('PUT','/v1/product/swarm-scheduler',{expectedRevision:settings.revision,policy:{maxActive:1,maxPerProject:1,headroomPercent:25,minHeadroomMiB:2048,diskReserveMiB:1024},dispatchPaused:paused});
}

async function readReview(key){
 const worker=taskData.get(key);assert(worker);setStage('browser-review-'+key);await openProject(owner);const pending=page.waitForResponse(r=>new URL(r.url()).pathname===owner.route+'/runs/'+owner.run.id+'/swarm/tasks/'+worker.taskId+'/review'&&r.request().method()==='GET');void pending.catch(()=>{});await taskRow(worker.taskId).getByRole('button',{name:'Review task changes',exact:true}).click();const response=await pending;assert.equal(response.status(),200);const {review}=await response.json();assert.equal(review.canIntegrate,true);assert.equal(review.sourceCommit,observations.get(key).commit);assert.deepEqual([...review.paths].sort(),changedPaths(key));const dialog=page.getByRole('dialog',{name:'Review task changes',exact:true});await dialog.getByLabel('Reviewed task diff').waitFor();assert((await dialog.getByLabel('Reviewed task diff').innerText()).includes(variants[key].receipt));return {review,dialog};
}
async function integrate(key,expected='integrated'){
 const {review,dialog}=await readReview(key);await dialog.getByRole('checkbox').check();health();
 const path=owner.route+'/runs/'+owner.run.id+'/swarm/tasks/'+variants[key].taskId+'/integrate';
 // Develop mounts ProjectWork's five context readers and the always-mounted
 // OrchestratorPanel reader. DeploymentOverview is confined to Deploy.
 if(key==='a')sourceReadCohort=observeSourceReadCohort({page,origin:base,projectPath:owner.route,mutationPath:path,assertActive:health,timeoutMs:remainingBudget(150000),
  persist:value=>{receipt.alphaSourceReadCohort=value;writeFileSync(join(evidence,'alpha-source-read-cohort.json'),JSON.stringify(value,null,2)+'\n',{mode:0o600});}});
 try{
  const result=await action(path,()=>{health();return dialog.getByRole('button',{name:'Integrate reviewed changes',exact:true}).click();});health();owner.run=result.run;run=owner.run;
  const integration=run.swarm.tasks.find(t=>t.id===variants[key].taskId).integration;assert.equal(integration.state,expected);(receipt.integrations??=[]).push({variant:key,review,integration});
  if(sourceReadCohort)await sourceReadCohort.finish({commit:integration.commit,readUI:async assertCurrent=>{
   assertCurrent();const facts=page.locator('details.project-work');
   if(await facts.getAttribute('open')===null){assertCurrent();await facts.locator(':scope > summary').click();assertCurrent();}
   const show=page.getByRole('button',{name:'Show project overview',exact:true});
   if(await show.isVisible()){assertCurrent();await show.click();assertCurrent();}
   const ready=await page.waitForFunction(sourceReadUI,{commit:integration.commit},{timeout:remainingBudget(30000)});
   try{assertCurrent();return await ready.jsonValue();}finally{await ready.dispose();}
  }});
  health();return integration;
 }catch(error){sourceReadCohort?.cancel(error);throw error;}finally{sourceReadCohort=null;}
}
async function boundedResponse(response, limit) {
 const parts=[];let size=0;
 for await(const part of response.body??[]){size+=part.length;if(size>limit){await response.body?.cancel().catch(()=>{});throw Error('HTTP response bound exceeded');}parts.push(part);}
 return Buffer.concat(parts,size).toString('utf8');
}
function privateJSON(path,limit=2*1024**2){const info=lstatSync(path);assert(info.isFile()&&!info.isSymbolicLink()&&info.nlink===1&&info.size<=limit);const bytes=readFileSync(path);assert(bytes.length<=limit);return JSON.parse(bytes);}
function privateRun(owner){const record=privateJSON(join(fixture,'state/sessions/project-runs',owner.owner.id,owner.run.id+'.json'));assert.equal(record.run.id,owner.run.id);assert.equal(record.run.projectId,owner.owner.id);return record.run;}
function privateCheckLedger(){return privateJSON(join(fixture,'state/control-plane/execution/leases.json'));}
function instanceID(){const value=readFileSync(join(fixture,'state/instance/instance-id'),'utf8').trim();assert(value.length>0&&value.length<=200);return value;}
function workerContainers(){return docker('ps','-aq','--no-trunc','--filter','label=luci.managed=true').split(/\s+/).filter(Boolean);}
function identifyCreate(bytes,meta){
 health();
 const input=JSON.parse(bytes);
 if(input.Labels?.['luci.check']==='true'){
  assert(checkConsent&&stage==='product-check-verification','A check needs its separate explicit final-build consent');
  const checkID=input.Labels['luci.check-id'];assert.match(checkID??'',/^check-[a-f0-9]{24}$/);
  const history=privateJSON(join(fixture,'state/sessions/project-checks',owner.owner.id+'.json'),16*1024**2);
  const candidates=history.filter(row=>row.id===checkID);assert.equal(candidates.length,1);
  const recorded=candidates[0];const current=privateRun(owner);assert.equal(current.id,checkConsent.run.id);assert.deepEqual(current.input,checkConsent.run.input);
  assert.equal(gitAt(owner.owner.path,'rev-parse','HEAD'),checkConsent.commit);assert.equal(gitAt(owner.owner.path,'status','--porcelain'),'');
  const checkEngineInfo=JSON.parse(docker('info','--format','{{json .}}'));assert.equal(checkEngineInfo.ID,engineID);assert.match(fixtureOwnerID??'',/^user-[a-f0-9]{24}$/);
  const identity=validateCheckCreate(input,{name:meta.name,fixture,primary:owner.owner.path,check:recorded,run:current,commit:checkConsent.commit,sourceDigest:checkConsent.sourceDigest,imageID:imageInfo.Id,uid:process.getuid(),gid:process.getgid(),ledger:privateCheckLedger(),scopeDigest:sha(resolve(join(fixture,'state/conversations'))),actor:{userId:fixtureOwnerID,projectId:owner.owner.id},engineID,instance:instanceID(),imageVirtualSize:imageInfo.Size,storageDriver:checkEngineInfo.Driver});
  if(checkIdentity)assert.deepEqual(identity,checkIdentity);else checkIdentity=identity;return 'check';
 }
 const sessionID=input.Labels?.['luci.session-id'];assert.match(sessionID??'',/^s-[a-f0-9]{8}$/);assert.equal(meta.name,'luci-'+sessionID);assert.equal(input.Labels?.['luci.managed'],'true');assert.equal(input.Labels?.['luci.launch-fence'],undefined);assert(owner);
 const saved=privateRun(owner),op=saved.operations.find(value=>value.childId===sessionID);assert(op);variantFor(op.taskId,op.attempt);const current=saved.swarm.tasks.find(t=>t.id===op.taskId);assert.equal(current.operationId,op.id);if(op.taskId==='task-join')for(const id of ['task-a','task-b'])assert.equal(saved.swarm.tasks.find(t=>t.id===id).integration?.state,'integrated','Join Create requires both current reviewed integrations');ownedSessions.add(sessionID);
 validateWorkerCreate(input,{fixture,projectID:owner.owner.id,runID:owner.run.id,operationID:op.id,session:row(sessionID),images:[image,imageInfo.Id],instance:instanceID(),uid:process.getuid(),gid:process.getgid(),taskContextVersion:2});return 'worker';
}
async function checkAdmission(identity=null){
 const engineInfo=JSON.parse(docker('info','--format','{{json .}}'));
 const view=await api('GET','/v1/product/swarm-scheduler');
 const measurement=assertCheckCapacity(view,{now:Date.now(),rootFreeBytes:rootFree(),engineID,engineInfo,daemonOnRootFilesystem:statSync(engineInfo.DockerRootDir).dev===statSync('/').dev,containerCount:docker('ps','-aq','--no-trunc').split(/\s+/).filter(Boolean).length,imageVirtualSize:imageInfo.Size,ledger:privateCheckLedger(),checkIdentity:identity});
 (receipt.checkResourceAdmissions??=[]).push(measurement);return measurement;
}
async function admission({kind,ordinal,body,...meta}){
 try{if(kind==='check'){assert.equal(ordinal,5,'One separate check follows exactly four worker Creates');assert.equal(proxy.observations.counts.worker,4);assert.equal(proxy.observations.counts.check,1);identifyCreate(body,meta);await checkAdmission(checkIdentity);health();return;}assert.equal(kind,'worker');assert(ordinal>=1&&ordinal<=4);identifyCreate(body,meta);const free=rootFree();assert(free>=6*1024**3,'Six GiB pre-create guard remains mandatory');assert.equal(workerContainers().length,0,'No overlapping full worker permitted');(receipt.resourceObservations??=[]).push({at:new Date().toISOString(),stage,ordinal,rootFreeBytes:free});}catch(error){stopOnFailure(error);throw error;}
}
function health(){if(closing)throw Error('Workflow is closing');workflowControl?.assertActive();remaining();if(fixtureError)throw fixtureError;if(service&&service.exitCode!==null)throw Error('Owned service exited unexpectedly');}
async function timed(promise,ms,label){let timer;try{return await Promise.race([promise,new Promise((_,reject)=>{timer=setTimeout(()=>reject(Error(label+' deadline exceeded')),ms);})]);}finally{clearTimeout(timer);}}




async function prepare(owner){await openProject(owner);await page.getByRole('button',{name:'Prepare agent team',exact:true}).click();const dialog=page.getByRole('dialog',{name:'Prepare agent team',exact:true});await dialog.getByRole('combobox').selectOption('1');assert.equal(await dialog.getByRole('combobox').inputValue(),'1');owner.run=(await action(owner.route+'/runs/'+owner.run.id+'/swarm',()=>dialog.getByRole('button',{name:'Prepare team',exact:true}).click(),201)).run;}
async function queueOwner(expectedTaskIds){await openProject(owner);await page.getByRole('button',{name:/^Queue reviewed tasks/}).click();const result=await action(owner.route+'/runs/'+owner.run.id+'/swarm/queue',()=>page.getByRole('dialog',{name:'Queue reviewed work',exact:true}).getByRole('button',{name:'Authorize queued tasks',exact:true}).click(),201);assert.deepEqual(result.items.map(item=>item.taskId).sort(),[...expectedTaskIds].sort());(receipt.queueConsents??=[]).push({at:new Date().toISOString(),taskIds:expectedTaskIds,items:result.items});owner.queued=result.items;owner.queueAuthorizations??=new Map();for(const item of result.items){assert.equal(item.projectId,owner.owner.id);assert.equal(item.runId,owner.run.id);owner.queueAuthorizations.set(item.taskId,{id:item.id,projectId:item.projectId,runId:item.runId,taskId:item.taskId});}}
function seedSource(project){health();for(const [name,text] of Object.entries(initialFiles)){assert(['verify.mjs','README.md'].includes(name));const file=join(project.path,name);assert(!existsSync(file)||lstatSync(file).isFile());writeFileSync(file,text,{mode:0o600});}}
async function saveContext(projectOwner,connectionID){
 project=projectOwner;projectRoute='/v1/product/projects/'+project.id;const goal='Build alpha and beta in isolated serial attempts, review their shared-file integration conflict, then join only the current integrated results.';
 await api('POST',projectRoute+'/brief',{revision:0,purpose:goal,audience:'Project owners',constraints:'Controlled offline DAG fixture. A and B have no dependency edge but intentionally overlap shared.txt and cannot write concurrently. Four workers maximum, including one explicitly authorized B retry.',criteria:['alpha=17','beta=25','joined sum=42 after reviewed integration only']});
 let spec=await api('GET',projectRoute+'/specification');spec=await api('POST',projectRoute+'/specification',{expectedRevision:spec.revision,expectedBriefRevision:spec.briefRevision,specification:{...spec,purpose:goal,requirements:[{id:'AC-dag',title:'Reviewed dependency join',priority:'must',acceptance:'Only integrated current alpha and beta results release the join; preserve the conflicting prior beta workspace and require fresh integration review after target movement.'}]}});spec=await api('POST',projectRoute+'/specification/approve',{revision:spec.revision,briefRevision:spec.briefRevision});
 let design=await api('GET',projectRoute+'/design-plan');design=await api('POST',projectRoute+'/design-plan',{expectedRevision:design.revision,expectedBriefRevision:spec.briefRevision,expectedSpecificationRevision:spec.revision,plan:{...design,screens:[{id:'result',name:'Combined verified result',route:'/',purpose:'Show exact alpha17,beta25,sum42',states:['Waiting for integration','Conflict retained','Verified'],widths:[390,1440],requirementIds:['AC-dag']}]}});design=await api('POST',projectRoute+'/design-plan/approve',{revision:design.revision});
 const tasks=taskContracts.map(value=>({...value,status:'todo',requirementIds:['AC-dag']}));const plan=await api('POST',projectRoute+'/plan',{expectedRevision:0,expectedSpecificationRevision:spec.revision,tasks});const gitReview=await api('GET',projectRoute+'/git');await api('POST',projectRoute+'/git/commit',{message:'Fixture setup: reviewed DAG contracts and independent checks',expectedReviewDigest:gitReview.reviewDigest});
 const branches=(await api('GET','/v1/projects/'+project.id+'/branches')).branches,branch=branches.find(row=>row.name==='main')??branches[0];assert(branch);const worktrees=(await api('GET','/v1/projects/'+project.id+'/worktrees?branchId='+encodeURIComponent(branch.id))).worktrees,worktree=worktrees.find(row=>row.isPrimary)??worktrees[0];assert(worktree);
 const selection={projectId:project.id,briefRevision:spec.briefRevision,specificationRevision:spec.revision,designPlanRevision:design.revision,planRevision:plan.revision,taskIds:tasks.map(t=>t.id)};const saved=(await api('POST',projectRoute+'/runs',{worktreeId:worktree.id,harness:'codex',model:'swarm-mechanics-fixture',permissionMode:'bypass',agentConnectionId:connectionID,outcome:'preview',implementationContext:selection,idempotencyKey:'dag-fixture-reviewed-build'},201)).run;
 receipt.acceptedIntent={tasks,selection,publicRunInputSHA256:sha(JSON.stringify(saved.input)),inputHashBasis:'Helper-derived SHA256 of JSON.stringify(public ProjectRun.input), not the private controller inputDigest',approvals:saved.input.approvals};return {owner:project,route:projectRoute,run:saved,goal,initialHead:execFileSync('git',['rev-parse','HEAD'],{cwd:project.path,encoding:'utf8'}).trim(),canonicalBefore:canonical(),canonicalLuci:readFileSync(join(project.path,'LUCI.md'))};
}
function gitAt(directory,...args){assert(isFixturePath(directory));return execFileSync('git',['-c','core.hooksPath=/dev/null',...args],{cwd:directory,env:{PATH:process.env.PATH,HOME:join(fixture,'home'),GIT_CONFIG_NOSYSTEM:'1',GIT_CONFIG_GLOBAL:join(fixture,'home/.gitconfig')},encoding:'utf8',timeout:15000,maxBuffer:1024*1024}).trim();}
function isFixturePath(directory){return typeof directory==='string'&&directory.startsWith(fixture+sep)&&realpathSync(directory)===directory&&lstatSync(directory).isDirectory();}
async function refreshRun(){owner.run=(await api('GET',owner.route+'/runs/'+owner.run.id)).run;run=owner.run;return run;}
async function reconcile(){await openProject(owner);owner.run=(await action(owner.route+'/runs/'+owner.run.id+'/swarm/reconcile',()=>page.getByRole('button',{name:'Check team status',exact:true}).click())).run;run=owner.run;}
async function complete(key){setStage('native-command-'+key);const worker=await queuedChild(key);await until('Native exact result '+key,()=>observations.has(key),120000);assert.equal(gitAt(worker.checkout,'rev-parse','HEAD'),observations.get(key).commit);assert.deepEqual(bundleSnapshot(worker.session.id),initialBundles.get(worker.session.id));await completeNaturally(worker,owner);return worker;}
async function assertJoinBlocked(label){
 assert.match(label,/^[a-z-]{1,64}$/);
 await refreshRun();const saved=privateRun(owner);
 assert(!saved.operations.some(op=>op.taskId==='task-join'),'Join was allocated before both current integrations');
 const queue=await api('GET',owner.route+'/swarm-queue');const item=queue.items.find(v=>v.taskId==='task-join');
 assert(item&&!item.childId&&!item.operationId);
 const snapshot=()=>({effects:effects(),primaryCommit:gitAt(owner.owner.path,'rev-parse','HEAD'),joinOperationIds:privateRun(owner).operations.filter(op=>op.taskId==='task-join').map(op=>op.id)});
 const before=snapshot(),requestedRevision=owner.run.revision;let probe;
 await api('POST',owner.route+'/runs/'+owner.run.id+'/swarm/tasks/task-join/reserve',
  {expectedRevision:requestedRevision,idempotencyKey:'blocked-join-'+label},409,30_000,(response,status)=>{
   probe=recordDependencyProbe(receipt,{label,status,response:Object.fromEntries(['code','message','error'].filter(k=>typeof response[k]==='string').map(k=>[k,scrub(response[k])])),
    requestedRevision,publicRevisionBefore:owner.run.revision,privateRevisionBefore:saved.revision,privateRevisionAfter:null,before,after:null});
   // Keep the reason and revisions even when the following strict assertion fails.
   // This diagnostic is not a successful dependency gate and does not authorize a retry.
   writeFileSync(join(evidence,'dependency-probe-'+label+'.json'),JSON.stringify(probe,null,2)+'\n',{mode:0o600,flag:'wx'});
  });
 probe.privateRevisionAfter=privateRun(owner).revision;probe.after=snapshot();
 writeFileSync(join(evidence,'dependency-probe-'+label+'.json'),JSON.stringify(probe,null,2)+'\n',{mode:0o600});
 assertDependencyRefusal(probe);
 assert.deepEqual(probe.after,probe.before,'A refused dependency reservation must not allocate or change the primary checkout');
 (receipt.dependencyGates??=[]).push({label,at:new Date().toISOString(),status:409,reason:probe.response.message,joinUnallocated:true,effects:before.effects});
}

async function verifyCombinedBuild(commit){
 setStage('product-check-verification');await openProject(owner);await refreshRun();assert.equal(owner.run.state,'awaiting-checks');assert.equal(workerContainers().length,0);
 const coverage=(await api('GET',owner.route+'/checks')).coverage;assert.equal(coverage.source.available,true);assert.equal(coverage.source.dirty,false);assert.equal(coverage.source.commit,commit);
 await page.getByRole('button',{name:'Verify this build',exact:true}).click();
 const region=page.getByRole('region',{name:'Build verification',exact:true});await region.waitFor();
 let count=await region.locator('.check-commands > li').count();assert(count<=8);
 while(count>0){await region.locator('.check-commands > li').first().getByRole('button',{name:/^Remove /}).click();count=await region.locator('.check-commands > li').count();}
 const advanced=region.locator('summary').filter({hasText:/^Advanced check commands$/});
 if(!await advanced.evaluate(node=>node.parentElement.open))await advanced.click();
 await region.getByLabel('Executable',{exact:true}).fill('node');await region.getByLabel('Arguments · JSON array',{exact:true}).fill('["verify.mjs","final"]');
 await region.getByRole('button',{name:'Add check command',exact:true}).click();assert.equal(await region.locator('.check-commands > li').count(),1);
 await region.getByRole('checkbox',{name:'Check AC-dag · Reviewed dependency join',exact:true}).check();
 await region.getByRole('checkbox',{name:"I trust this project's code and these Docker commands to write project files and use the network.",exact:true}).check();
 await checkAdmission();checkConsent={run:structuredClone(owner.run),commit,sourceDigest:coverage.source.sourceDigest,at:new Date().toISOString()};
 const expectedRequest={commands:[checkCommand],execution:'docker',acknowledgeHostExecution:true,requirementIds:['AC-dag'],expectedSpecificationRevision:coverage.specification.revision,expectedDesignPlanRevision:coverage.design.revision};
 const pending=page.waitForResponse(response=>new URL(response.url()).pathname===owner.route+'/checks'&&response.request().method()==='POST',{timeout:remainingBudget(120000)});void pending.catch(()=>{});
 await region.getByRole('button',{name:'Run selected requirement checks',exact:true}).click();const response=await pending;
 assert.deepEqual(response.request().postDataJSON(),expectedRequest);assert.equal(response.status(),202);const started=await response.json();
 assertRecordedCheck(started,checkConsent);(receipt.checkConsents??=[]).push({at:checkConsent.at,request:expectedRequest,checkId:started.id,commit,sourceDigest:checkConsent.sourceDigest});
 const checked=await until('Recorded whole-build Docker check',async()=>{const current=await api('GET',owner.route+'/checks/'+started.id);if(['queued','running'].includes(current.status))return null;return current;},120000);
 assertRecordedCheck(checked,checkConsent,true);assert.equal(gitAt(owner.owner.path,'rev-parse','HEAD'),commit);assert.equal(gitAt(owner.owner.path,'status','--porcelain'),'');
 const created=proxy.observations.requests.filter(row=>row.kind==='check'&&row.method==='POST'&&/\/containers\/create$/.test(row.path)&&row.status===201);assert.equal(created.length,1);assert.equal(created[0].name,'luci-check-'+started.commands[0].allocationId);assert.match(created[0].id,/^[a-f0-9]{64}$/);
 await until('Confirmed check container absence',()=>!docker('ps','-aq','--no-trunc').split(/\s+/).includes(created[0].id),10000);
 const joined=await action(owner.route+'/runs/'+owner.run.id+'/checks',()=>region.getByRole('button',{name:'Use these results for this build',exact:true}).click());owner.run=joined.run;run=owner.run;assertJoinedCheck(run,checked);
 receipt.productVerification={check:checked,joinedEvidence:run.checks[0],runState:run.state,container:{id:created[0].id,name:created[0].name,imageId:imageInfo.Id,createdAt:created[0].completedAt},sourceUnchanged:true,scope:'Actual Luci Docker whole-build check and explicit join; preview/release and live-model quality remain unverified.',resourceScope:'Fresh capacity guards cross-check the exact shared-ledger check reservation plus 1 GiB memory/combined swap, 1 CPU and 256 PIDs. The wrapper configures the independent observer with --include-checks for exact managed workers and protocol-1 checks. Its separate summary reports observed, identity-only or unobserved check coverage; this receipt alone does not establish a sampled check cgroup peak.'};
 assert.equal(gitAt(owner.owner.path,'rev-parse','HEAD'),commit);assert.equal(gitAt(owner.owner.path,'status','--porcelain'),'');checkConsent=null;
 await openProject(owner);await page.screenshot({path:join(evidence,'DAG-verified.png'),fullPage:true});
}
function retainedB(){
 const b=taskData.get('b1');assert(b);const integration=join(b.checkout+'-integration');assert(isFixturePath(b.checkout)&&isFixturePath(integration));const files=Object.fromEntries(changedPaths('b1').map(name=>{const file=join(b.checkout,name);const st=lstatSync(file);assert(st.isFile()&&!st.isSymbolicLink()&&st.nlink===1&&st.size<64*1024);return [name,sha(readFileSync(file))];}));const conflict=readFileSync(join(integration,'shared.txt'));assert(conflict.length<4096);return {sourceCommit:gitAt(b.checkout,'rev-parse','HEAD'),files,integrationHead:gitAt(integration,'rev-parse','HEAD'),unmerged:gitAt(integration,'ls-files','--unmerged'),conflictSHA256:sha(conflict)};
}
const startupScope=createStartupScope({assertActive:health,onFailure:stopOnFailure,onTransition:row=>journal.record('driver-stage',{stage:'startup-'+row.name+'-'+row.state})});
let engineID;
try {
 workflowControl=createWorkflowDeadline({timeoutMs:remaining(),onFailure:stopOnFailure});
 await workflowControl.run(async()=>{
 reserveTimer=setInterval(()=>{try{const free=rootFree();(receipt.diskSamples??=[]).push({stage,at:new Date().toISOString(),rootFreeBytes:free});if(free<1024**3)stopOnFailure(Error('Hard one GiB reserve breached'));for(const worker of taskData.values())if(worker.finalReleased&&worker.session)observeTaskLease(worker);}catch(error){stopOnFailure(error);}},1000);
 for(const name of ['SIGTERM','SIGINT'])process.on(name,()=>{signalFailure=name;try{journal.record('driver-signal',{stage,signal:name});}catch{}stopOnFailure(Error('Acceptance interrupted by '+name));});
 receipt.bounds={scenarioDeadlineMs:900000,cleanupDeadlineMs:180000,upstreamRequestMs:120000,maxCreates:5,maxStarts:5,maxWorkerCreates:4,maxCheckCreates:1,checkCompletionMs:120000,preCreateFreeBytes:6*1024**3,runningReserveBytes:1024**3,workerMemoryBytes:1024**3,workerNanoCPUs:1000000000,pidsLimit:256};
 engineID=docker('info','--format','{{.ID}}');assert(engineID.length>0&&engineID.length<=256);receipt.engineIdentitySHA256=sha(engineID);
 await startAcceptanceServices({scope:startupScope,assertActive:health,
  startProvider:async()=>{health();startProvider();const handle=provider;await new Promise((done,reject)=>handle.once('error',reject).listen(0,bridge,done));return handle;},
  adoptProvider:handle=>{provider=handle;},closeProvider:async handle=>{handle.closeAllConnections();if(handle.listening)await new Promise(done=>handle.close(done));},
  freePort,
  startProxy:()=>{health();const upstream=process.env.DOCKER_HOST||'unix:///var/run/docker.sock';assert(upstream.startsWith('unix://'));return startFencedDockerProxy({socketPath:join(fixture,'docker-proxy.sock'),upstreamSocketPath:upstream.slice(7),journal,identifyCreate,beforeCreate:admission,beforeStart:health,maxCreates:receipt.bounds.maxCreates,allowCheckTraffic:true,checkTrafficProtocol:'sdk-ledger-v1',maxCreatesByKind:{worker:4,fence:1,check:1},maxStarts:receipt.bounds.maxStarts,maxObservations:20000,upstreamTimeoutMs:120000,holdTimeoutMs:600000});},
  adoptProxy:handle=>{proxy=handle;},closeProxy:handle=>handle.close(),
  configure:({provider,port,preview})=>{health();providerBase='http://host.docker.internal:'+provider.address().port+'/v1';base='http://127.0.0.1:'+port;
 writeFileSync(join(fixture,'docker/config.json'),JSON.stringify({cliPluginsExtraDirs:process.env.DOCKER_CONFIG?[join(process.env.DOCKER_CONFIG,'cli-plugins')]:[]}),{mode:0o600});
 env=Object.fromEntries(['PATH','LANG','LC_ALL','TZ'].filter(name=>process.env[name]).map(name=>[name,process.env[name]]));
 Object.assign(env,{HOME:join(fixture,'home'),DOCKER_CONFIG:join(fixture,'docker'),DOCKER_HOST:'unix://'+join(fixture,'docker-proxy.sock'),LUCI_DOCKER_HOST:'unix://'+join(fixture,'docker-proxy.sock'),LUCI_ADDR:'127.0.0.1:'+port,LUCI_STATE_DIR:join(fixture,'state'),LUCI_PROJECTS_DIR:join(fixture,'projects'),LUCI_WEB_DIR:web,LUCI_RUNTIME_DIR:join(dirname(binary),'runtime'),LUCI_SANDBOX_IMAGE:image,LUCI_BACKEND:'docker',LUCI_SESSION_MEMORY_MIB:'1024',LUCI_SESSION_CPU_MILLIS:'1000',LUCI_MANAGED_SHUTDOWN_TOKEN:lifecycle,LUCI_LOG_LEVEL:'error',LUCI_BROWSER_PREVIEW_ADDR:'127.0.0.1:'+preview,LUCI_BROWSER_PREVIEW_PORT:String(preview),GIT_CONFIG_GLOBAL:join(fixture,'home/.gitconfig'),GIT_CONFIG_NOSYSTEM:'1'});
  },startService,
 });health();
 const bootstrap=readFileSync(join(fixture,'state/control-plane/bootstrap-token'),'utf8').trim();sensitive.push(bootstrap);
 const account=await api('POST','/v1/auth/bootstrap',{bootstrapToken:bootstrap,username:'swarmowner',displayName:'DAG Fixture Owner',password},201);token=account.token;sensitive.push(token);fixtureOwnerID=account.user.id;
 await api('POST','/v1/setup/git/identity',{name:'Swarm Fixture',email:'swarm@example.invalid'});
 const readiness=await api('POST','/v1/setup/check',{});for(const id of ['docker','sandbox-image'])assert.equal(readiness.checks.find(row=>row.id===id)?.status,'ready');
 const connections=await api('POST','/v1/setup/agent-connections',{action:'save',expectedRevision:0,name:'Local controlled DAG fixture',kind:'compatible-api',enabled:true,endpoints:{responses:providerBase},models:{codex:'swarm-mechanics-fixture'},apiKey:key});assert.equal(connections.connections.length,1);const connectionID=connections.connections[0].id;
 await setPaused(true);
 const projectOwner=await api('POST','/v1/product/projects',{name:'reviewed-dag-fixture'},201);assert(isFixturePath(projectOwner.path));seedSource(projectOwner);for(const task of taskContracts)titles.set(task.id,task.title);owner=await saveContext(projectOwner,connectionID);projectStates.push(owner);receipt.acceptedIntent.canonicalHashes=owner.canonicalBefore;receipt.acceptedIntent.initialCommit=owner.initialHead;receipt.fixtureSourceHashes=Object.fromEntries(Object.entries(initialFiles).map(([name,value])=>[name,sha(value)]));
 const browserEnv=Object.fromEntries(['PATH','LANG','LC_ALL','TZ'].filter(n=>process.env[n]).map(n=>[n,process.env[n]]));browserEnv.LUCI_SWARM_BROWSER_FIXTURE=fixture;
 await startAcceptanceBrowser({scope:startupScope,assertActive:health,
  launch:()=>{health();return chromium.launch({headless:true,executablePath:option('--chromium','/usr/bin/chromium'),env:browserEnv,args:['--no-proxy-server']});},adoptBrowser:handle=>{browser=handle;},closeBrowser:handle=>handle.close(),
  newContext:handle=>handle.newContext({viewport:{width:1440,height:1080},serviceWorkers:'block'}),
  installRoutes:async context=>{health();await context.route('**/*',async route=>{const request=route.request(),url=new URL(request.url());if(closing||fixtureError||workflowControl?.failed){await route.abort('blockedbyclient');return;}if(url.origin!==base){stopOnFailure(Error('Browser external request refused'));await route.abort('blockedbyclient');return;}if(!['GET','HEAD','OPTIONS'].includes(request.method()))receipt.browserMutations.push({stage,method:request.method(),path:url.pathname});await route.continue();});health();},newPage:context=>context.newPage(),adoptPage:handle=>{page=handle;},
 });health();receipt.browserVersion=browser.version();
 page.setDefaultTimeout(15000);page.on('pageerror',error=>{receipt.browserErrors.push(scrub(error));stopOnFailure(Error('Browser script error'));});
 setStage('prepare-and-authorize-DAG');await prepare(owner);await queueOwner(['task-a','task-b','task-join']);const initialQueue=await api('GET',owner.route+'/swarm-queue');const aEntry=initialQueue.items.find(value=>value.taskId==='task-a');assert(aEntry);await api('POST',owner.route+'/swarm-queue/'+aEntry.id+'/priority',{expectedRevision:aEntry.revision,priority:'high'});await assertJoinBlocked('before-starts');assert.equal(effects().create,0);
 await setPaused(false);const a=await complete('a');const b1=await complete('b1');assert.equal(a.operation.baseCommit,owner.initialHead);assert.equal(b1.operation.baseCommit,owner.initialHead);assert.equal(gitAt(owner.owner.path,'rev-parse','HEAD'),owner.initialHead);await reconcile();await assertJoinBlocked('both-processes-exited');
 const oldB=await readReview('b1');receipt.staleReview={captured:oldB.review};await oldB.dialog.getByRole('button',{name:'Keep reviewing',exact:true}).click();const integratedA=await integrate('a');assert.equal(gitAt(owner.owner.path,'rev-parse','HEAD'),integratedA.commit);await assertJoinBlocked('only-alpha-integrated');
 assert.equal(receipt.alphaSourceReadCohort?.phase,'ready');assert.equal(receipt.alphaSourceReadCohort.stableUIBoundary.commit,integratedA.commit);
 setStage('stale-review-refusal');await refreshRun();
 const staleRevision=owner.run.revision,stalePath=owner.route+'/runs/'+owner.run.id+'/swarm/tasks/task-b/integrate',staleActionOffset=receipt.apiActions?.length??0;
 const staleProbe=await probeStaleReview({receipt,requestedRevision:staleRevision,scrub,
  request:observe=>api('POST',stalePath,{expectedRevision:staleRevision,reviewDigest:oldB.review.reviewDigest},409,30_000,observe),
  responseStatus:()=>receipt.apiActions?.slice(staleActionOffset).find(row=>row.method==='POST'&&row.path===stalePath)?.status,
  read:{effects:()=>effects(),primaryCommit:()=>gitAt(owner.owner.path,'rev-parse','HEAD'),
   primaryStatusSHA256:()=>sha(gitAt(owner.owner.path,'status','--porcelain')),
   privateRevision:()=>privateRun(owner).revision,integrationCheckoutExists:()=>existsSync(b1.checkout+'-integration'),
   publicRevision:async phase=>{if(phase==='after'){owner.run=(await api('GET',owner.route+'/runs/'+owner.run.id,undefined,200,5_000)).run;run=owner.run;}return owner.run.revision;}},
  persist:(probe,first)=>writeFileSync(join(evidence,'stale-review-probe.json'),JSON.stringify(probe,null,2)+'\n',{mode:0o600,flag:first?'wx':'w'})});
 receipt.staleReview={...receipt.staleReview,rejectedAt:new Date().toISOString(),usedCurrentRevision:staleRevision,status:staleProbe.status,noIntegrationCheckout:true,mainUnchanged:true};
 setStage('renewed-review-real-conflict');const conflict=await integrate('b1','unknown');assert.equal(conflict.commit??'','');assert.equal(conflict.targetCommit,integratedA.commit);assert.notEqual(conflict.reviewDigest,oldB.review.reviewDigest);assert.equal(gitAt(owner.owner.path,'rev-parse','HEAD'),integratedA.commit);assert.equal(gitAt(owner.owner.path,'status','--porcelain'),'');const retained=retainedB();assert(retained.unmerged.includes('shared.txt'));assert.match(readFileSync(join(b1.checkout+'-integration','shared.txt'),'utf8'),/<<<<<<< HEAD[\s\S]*alpha[\s\S]*=======[\s\S]*beta[\s\S]*>>>>>>>/);receipt.conflict={integration:conflict,retained,mainCommit:integratedA.commit,at:new Date().toISOString()};await page.getByRole('dialog',{name:'Review task changes',exact:true}).waitFor({state:'hidden'});await until('Visible retained conflict notice',async()=>{const text=await page.getByRole('region',{name:'Agent team',exact:true}).locator(':scope > [role=status]').allTextContents();return text.length>0?text:null;});receipt.conflict.visibleStatus=await page.getByRole('region',{name:'Agent team',exact:true}).locator(':scope > [role=status]').allTextContents();await page.screenshot({path:join(evidence,'DAG-conflict-retained.png'),fullPage:true});assert(!receipt.conflict.visibleStatus.some(text=>/Reviewed changes integrated|Dependent tasks can now start/i.test(text)),'CLOUD-015: retained conflict must not claim integration or dependency release');assert(receipt.conflict.visibleStatus.some(text=>/not|unconfirmed|conflict|retained/i.test(text)),'Retained integration failure needs explicit status');writeFileSync(join(evidence,'retained-conflict-shared.txt'),readFileSync(join(b1.checkout+'-integration','shared.txt')),{mode:0o600,flag:'wx'});writeFileSync(join(evidence,'retained-b1.patch'),gitAt(b1.checkout,'diff','--binary',b1.operation.baseCommit,'HEAD','--')+'\n',{mode:0o600,flag:'wx'});await assertJoinBlocked('beta-conflict-retained');
 setStage('browser-retire-conflicted-attempt');await openProject(owner);await openTaskActions('task-b');await taskRow('task-b').getByRole('button',{name:'Retain changes and retry',exact:true}).click();const retireDialog=page.getByRole('dialog',{name:'Retain changes and retry',exact:true});owner.run=(await action(owner.route+'/runs/'+owner.run.id+'/swarm/tasks/task-b/abandon',()=>retireDialog.getByRole('button',{name:'Retain output and allow retry',exact:true}).click())).run;run=owner.run;const previous=run.operations.find(op=>op.id===b1.operation.id);assert.equal(previous.status,'cancelled');assert.equal(previous.integration.state,'unknown');assert.equal(run.swarm.tasks.find(t=>t.id==='task-b').integration,undefined);assert.deepEqual(retainedB(),retained);await assertJoinBlocked('conflict-retired-not-replaced');
 setStage('authorize-fresh-beta');await setPaused(true);await queueOwner(['task-b']);assert.equal(effects().create,2);await setPaused(false);const b2=await complete('b2');assert.notEqual(b2.operation.id,b1.operation.id);assert.notEqual(b2.checkout,b1.checkout);assert.equal(b2.operation.baseCommit,integratedA.commit);assert.deepEqual(retainedB(),retained);await reconcile();await assertJoinBlocked('fresh-beta-exited-not-integrated');const integratedB=await integrate('b2');assert.equal(gitAt(owner.owner.path,'rev-parse','HEAD'),integratedB.commit);assert.deepEqual(retainedB(),retained);
 setStage('dependency-release');const joined=await complete('join');assert.equal(joined.operation.baseCommit,integratedB.commit);const saved=privateRun(owner);for(const id of ['task-a','task-b'])assert.equal(saved.swarm.tasks.find(t=>t.id===id).integration.state,'integrated');await reconcile();const integratedJoin=await integrate('join');assert.equal(readFileSync(join(owner.owner.path,'result.json'),'utf8'),expectedResult);assert.equal(sha(readFileSync(join(owner.owner.path,'result.json'))),observations.get('join').artifact.outputs['result.json']);assert.deepEqual(retainedB(),retained);
 setStage('final-exact-source-check');const finalHead=gitAt(owner.owner.path,'rev-parse','HEAD');assert.equal(finalHead,integratedJoin.commit);assert.equal(gitAt(owner.owner.path,'status','--porcelain'),'');const checkedAt=new Date().toISOString(),output=execFileSync(process.execPath,['verify.mjs','final'],{cwd:owner.owner.path,env:{PATH:process.env.PATH,HOME:join(fixture,'home')},encoding:'utf8',timeout:15000,maxBuffer:1024*1024});assert.equal(output,'DAG_CHECK_OK final\n');assert.equal(gitAt(owner.owner.path,'rev-parse','HEAD'),finalHead);assert.equal(gitAt(owner.owner.path,'status','--porcelain'),'');assert.deepEqual(canonical(),owner.canonicalBefore);for(const worker of taskData.values())assert.deepEqual(bundleSnapshot(worker.session.id),initialBundles.get(worker.session.id));
 receipt.finalCheck={scope:'Independent fixture check on exact combined main commit, following native join checks; not a Luci product check receipt or deployment acceptance',command:['node','verify.mjs','final'],exitCode:0,stdout:output,commit:finalHead,at:checkedAt,resultSHA256:sha(readFileSync(join(owner.owner.path,'result.json'))),productRunState:owner.run.state};assert.equal(owner.run.state,'awaiting-checks');receipt.retainedBAtCompletion=retainedB();
 await openProject(owner);await page.screenshot({path:join(evidence,'DAG-integrated.png'),fullPage:true});await verifyCombinedBuild(finalHead);assert.equal(proxy.observations.counts.create,5);assert.equal(proxy.observations.counts.worker,4);assert.equal(proxy.observations.counts.check,1);assert.equal(proxy.observations.counts.start,5);assert.equal(proxy.observations.dropped,0);assert.equal(proxy.observations.failures.length,0);assert.equal(observations.size,4);assert.equal(receipt.naturalCompletions.length,4);assert.equal(receipt.browserErrors.length,0);assert.equal(workerContainers().length,0);assert.equal(docker('info','--format','{{.ID}}'),engineID);check('Four serial native workers exercise reviewed DAG dependencies, actual conflict, renewed review, retained retry and exact combined output plus one explicitly joined Docker whole-build check');receipt.passed=true;
 });
}catch(error){workflowControl?.fail(error);try{journal.record('driver-caught-failure',{stage,...diagnosticError(error)});}catch{}receipt.failedStage=stage;receipt.failure=redactError(error);process.exitCode=1;console.error(receipt.failure);if(page){try{await boundedAttempt(()=>page.screenshot({path:join(evidence,'failed-stage.png'),fullPage:true}),3000,'failure-screenshot-deadline-exceeded');}catch{}}}
finally {
 closing=true;workflowControl?.close();clearTimeout(deadlineTimer);clearInterval(reserveTimer);
 cleanupDeadline=Date.now()+180000;
 const cleanupFailure=error=>{receipt.cleanup.push(redactError(error));receipt.passed=false;process.exitCode=1;};
 const failureCheckpoint={version:1,passed:false,status:'cleanup-incomplete',stage,at:new Date().toISOString(),scenarioDeadlineMs:900000,cleanupDeadlineMs:180000,driver:journalProcessIdentity(),controllers:controllerIdentities.filter(Boolean),signal:signalFailure??null,failure:diagnosticError(fixtureError),dockerRequestsDrained:false,startupResources:startupScope.snapshot(),originalReceiptComplete:false};
 // The watchdog only terminates this driver. Unsettled external resources are
 // explicitly retained, never inferred absent or selected for forced cleanup.
 cleanupHardStop=setTimeout(()=>{
  try{journal.record('driver-background-failure',{stage,code:'cleanup-hard-stop'});}catch{}
  try{writeFailureCheckpoint(report,{...failureCheckpoint,status:'cleanup-hard-stop',finishedAt:new Date().toISOString()});}catch{}
  process.exit(1);
 },180000);
 try {
  receipt.cleanupPhases=await runBoundedCleanup({
   checkpoint:()=>{try{writeFailureCheckpoint(report+'.incomplete.json',failureCheckpoint);journal.record('driver-cleanup-start',{stage,deadlineMs:180000});}catch(error){cleanupFailure(error);}},
   onFailure:cleanupFailure,
   onPhase:(name,status)=>journal.record('driver-stage',{stage:'cleanup-'+name+'-'+status}),
   phases:[
    {name:'controllers',timeoutMs:30000,run:async()=>{const stopped=await stopService(false);assert(stopped);assert(controllerIdentities.filter(Boolean).every(identity=>!stillOwned(identity)));return true;}},
    {name:'proxy',timeoutMs:125000,run:async()=>{if(startupScope.has('proxy')){const cleanup=await startupScope.close('proxy',cleanupBudget(125000));assert(cleanup.drained&&cleanup.pendingRequests===0&&cleanup.activeHandlers===0);receipt.dockerTransportObservations=structuredClone(proxy.observations);if(proxy.observations.dropped||proxy.observations.failures.length)throw Error('Docker transport recorded lost, refused or uncertain observations');return {drained:true};}return {drained:true,notStarted:true};}},
    {name:'browser',timeoutMs:12000,run:async()=>{if(startupScope.has('browser')){try{await startupScope.close('browser',cleanupBudget(5000));}catch(error){cleanupFailure(error);}}await settleOwnedBrowser({browser:undefined,listOwned:()=>ownedProcesses('browser'),stillOwned,forceTerminate:identity=>{try{process.kill(identity.pid,'SIGKILL');}catch(error){if(error.code!=='ESRCH')throw error;}},onFailure:cleanupFailure,settleTimeoutMs:cleanupBudget(5000)});}},
    {name:'provider',timeoutMs:3000,run:async()=>{if(startupScope.has('provider'))await startupScope.close('provider',cleanupBudget(3000));}},
   ],
  });
  // Only a completed bounded phase grants cleanup authority. A late callback
  // cannot retroactively turn a timed-out phase into stopped/drained proof.
  receipt.controllersStopped=receipt.cleanupPhases.some(row=>row.name==='controllers'&&row.status==='completed'&&row.result===true);
  const proxyPhase=receipt.cleanupPhases.find(row=>row.name==='proxy'&&row.status==='completed');
  receipt.dockerRequestsDrained=proxyPhase?.result?.drained===true;receipt.dockerTransportNotStarted=proxyPhase?.result?.notStarted===true;
  for(const socket of sockets)socket.close();receipt.actualTaskResults=Object.fromEntries(observations);
  receipt.taskChildren=[...taskData.values()].map(value=>({task:value.task,projectID:value.owner.id,operationID:value.operation.id,sessionID:value.session.id,container:value.container,bundle:initialBundles.get(value.session.id)}));receipt.terminalObservations=terminalProbes.map(({id,attachedStage,probe})=>({sessionID:id,attachedStage,...probe.snapshot()}));
 if(receipt.controllersStopped&&receipt.dockerRequestsDrained&&proxy){const created=proxy.observations.requests.filter(value=>value.method==='POST'&&/\/containers\/create$/.test(value.path)&&value.status===201&&/^[a-f0-9]{64}$/.test(value.id??''));for(const creation of created){try{const ids=docker('ps','-aq','--no-trunc').split(/\s+/).filter(Boolean);if(!ids.includes(creation.id))continue;const value=JSON.parse(docker('inspect',creation.id))[0];assert.equal(value.Id,creation.id);assert.equal(value.Name,'/'+creation.name);if(creation.kind==='check'){
 assert(checkIdentity);assertOwnedCheckContainer(value,{creation,identity:checkIdentity,fixture,primary:owner.owner.path,imageID:imageInfo.Id});
 cleanupFailure(Error('Unexpected exact check container remained; failed-fixture immutable-ID cleanup required'));if(value.State.Running)docker('stop','--time','5',value.Id);const current=JSON.parse(docker('inspect',value.Id))[0];assert.equal(current.Id,value.Id);assert.equal(current.State.Running,false);docker('rm',value.Id);continue;
 }assert.equal(value.Config.Labels['luci.instance'],instanceID());assert.equal(value.Config.Labels['luci.session-id'],creation.labels['luci.session-id']);assert(ownedSessions.has(value.Config.Labels['luci.session-id']));assert.equal(value.Config.Labels['luci.managed'],'true');assert.equal(value.Image,imageInfo.Id);assert(value.Mounts.some(mount=>mount.Destination==='/home/dev/work'&&projectStates.some(owner=>realpathSync(mount.Source).startsWith(join(dirname(owner.owner.path),'.luci-swarms')+sep))));cleanupFailure(Error('Unexpected full worker remained; exact failed-fixture cleanup required'));if(value.State.Running)docker('stop','--time','5',value.Id);const current=JSON.parse(docker('inspect',value.Id))[0];assert.equal(current.Id,value.Id);assert.equal(current.State.Running,false);docker('rm',value.Id);}catch(error){cleanupFailure(error);}}}
 try{assert(receipt.controllersStopped);assert.equal(receipt.dockerRequestsDrained,true);assert(startupScope.snapshot().every(row=>row.state==='closed'&&!row.closeTimedOut),'Startup factory or handle remains unsettled');assert.equal(docker('ps','-aq'),'');assert.equal(docker('volume','ls','-q'),'');assert.equal(ownedProcesses().length,0);assert.equal(ownedProcesses('browser').length,0);assert.equal(JSON.parse(docker('image','inspect',image))[0].Id,imageInfo.Id);assert.equal(sha(readFileSync(binary)),provenance.binarySHA256);assert.equal(sha(readFileSync(join(web,'index.html'))),provenance.webIndexSHA256);assert.equal(docker('info','--format','{{.ID}}'),engineID);for(const name of driverNames)assert.equal(sha(readFileSync(new URL(name,import.meta.url))),sha(readFileSync(join(evidence,'accepted-harness',name))));for(const name of reusedHelpers)assert.equal(sha(readFileSync(join(repo,'scripts/acceptance',name))),sha(readFileSync(join(evidence,'accepted-harness',name.replaceAll('/','_')))));
  receipt.runtimeCleanup={containers:0,volumes:0,ownedServiceProcesses:0,ownedBrowserProcesses:0,protectedImagePackageAndHelpersUnchanged:true,rootFreeBytes:rootFree()};assert.equal(realpathSync(fixture),fixture);assert.equal(dirname(fixture),fixtureParent);assert(fixture.split(sep).at(-1).startsWith('luci-dag-'));receipt.fixturePreservedForInspection=true;
 }catch(error){cleanupFailure(error);}
 if(!receipt.fixtureRemoved)receipt.fixtureRetained={path:fixture,reason:'The exact original, conflicting and replacement task checkouts, frozen contexts and integration conflict are retained for inspection even after successful acceptance. This private fixture contains test credentials; do not publish or reuse it as a normal workspace.'};

 }catch(error){cleanupFailure(error);}
 finally {
 receipt.startupResources=startupScope.snapshot();if(fixtureError){receipt.passed=false;process.exitCode=1;receipt.backgroundFailure=redactError(fixtureError);}receipt.providerCalls=providerCalls;receipt.auxiliaryCalls=auxiliaryCalls;receipt.actualTaskCount=observations.size;receipt.elapsedMs=Date.now()-started;receipt.signal=signalFailure??null;receipt.finishedAt=new Date().toISOString();try{journal.record('driver-receipt-prepared',{stage,passed:receipt.passed});}catch(error){cleanupFailure(error);}writeFileSync(report,JSON.stringify(receipt,null,2)+'\n',{mode:0o600,flag:'wx'});console.log((receipt.passed?'PASS ':'FAIL ')+report);
  clearTimeout(cleanupHardStop);
  // A rejected race leaves its operation pending. Only this driver's lifetime
  // ends here; the failed receipt retains every unsettled resource and phase.
  const finalExitCode=process.exitCode??(receipt.passed?0:1);
  setImmediate(()=>process.exit(finalExitCode));
 }
}
