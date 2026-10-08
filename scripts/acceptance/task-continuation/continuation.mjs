#!/usr/bin/env node
// CLOUD-012 installed retained-source continuation witness. Real Docker/native commands; scripted provider.
// Approved context/run seeding is labelled API fixture setup; actual task commands finish naturally before human integration.
import assert from 'node:assert/strict';
import { spawn, execFileSync } from 'node:child_process';
import { createHash, randomUUID } from 'node:crypto';
import { createServer as httpServer } from 'node:http';
import { createServer } from 'node:net';
import { existsSync, statfsSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, realpathSync, rmSync, lstatSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve, sep } from 'node:path';
import { fileURLToPath } from 'node:url';
import { setTimeout as delay } from 'node:timers/promises';
import {verifyTaskContextDelivery} from '../task-context-delivery.mjs';
import {taskContracts,variants,seedFiles as initialFiles,variantFor,changedPaths,taskCommand,taskHandoff,answer,question,partialFiles,finalFiles} from './continuation-fixture.mjs';
import {contextFilesV1,contextFileNames,dependencyFile,validateContinuationContext} from './continuation-context.mjs';
import {validateCheckpointReview,validatePreviousAttempt,validateSeedOperation} from './checkpoint-contract.mjs';
import {reconcileManualStart} from './manual-start.mjs';
import {automaticCompletion,assertReadOnlyCompletionRequests,inPlaceCompletion} from './automatic-completion.mjs';

const here=dirname(fileURLToPath(import.meta.url)),repo=resolve(process.env.LUCI_ACCEPTANCE_REPO??resolve(here,'../../..')),args=process.argv.slice(2);
const {chromium}=await import(join(repo,'desktop/node_modules/playwright/index.mjs'));
const {validateWorkerCreate}=await import(join(repo,'scripts/acceptance/launch-recovery/worker-create-guard.mjs'));
const {startFencedDockerProxy}=await import(join(repo,'scripts/acceptance/launch-recovery/fenced-docker-proxy.mjs'));
const {createNativeTerminalProbe}=await import(join(repo,'scripts/acceptance/four-project-queue/native-terminal-diagnostics.mjs'));
const option = (name, fallback) => { const i = args.indexOf(name); if (i < 0) return fallback; assert(args[i + 1] && !args[i + 1].startsWith('--'), 'Missing value for ' + name); return args[i + 1]; };
if(args.includes('--help')){console.log('PREPARED ONLY: installed retained-source continuation with exactly two serial workers, reviewed checkpoint and controller restart. Required --binary --web --provenance --expected-source --image --expected-image-id --image-provenance --report FRESH_JSON --fixture-root SHORT_PATH. Root exclusive Docker window and verified clean package required; no model quality or Windows claim.');process.exit(0);}
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
const docker = (...values) => execFileSync('docker', values, { encoding: 'utf8', timeout: 30_000 }).trim();
assert.equal(docker('ps', '-aq'), '', 'Exclusive runtime admission requires zero existing containers');
assert.equal(docker('volume', 'ls', '-q'), '', 'Exclusive runtime admission requires zero existing volumes');
const imageInfo = JSON.parse(docker('image', 'inspect', image))[0], releaseImageInfo = JSON.parse(docker('image', 'inspect', releaseImage))[0];
assert.match(imageInfo.Id, /^sha256:[a-f0-9]{64}$/); assert.match(releaseImageInfo.Id, /^sha256:[a-f0-9]{64}$/);
assert.equal(imageInfo.Id, expectedImage);
assert.equal(releaseImageInfo.Id, expectedImage);
const bridge = docker('network', 'inspect', 'bridge', '--format', '{{(index .IPAM.Config 0).Gateway}}');
assert.match(bridge, /^(10\.|172\.(1[6-9]|2\d|3[01])\.|192\.168\.)/);
const report = resolve(option('--report', 'artifacts/quality-swarm/continuation-installed/attempt-' + Date.now() + '/result.json')), evidence = dirname(report);
if (existsSync(evidence)) assert.equal(readdirSync(evidence).length, 0, 'Use a fresh evidence directory; prior failures must remain');
mkdirSync(evidence, { recursive: true, mode: 0o700 });
writeFileSync(join(evidence, 'package-provenance.json'), provenanceBytes, { mode: 0o600, flag: 'wx' });
writeFileSync(join(evidence,'image-provenance.json'),readFileSync(resolve(imageEvidencePath)),{mode:0o600,flag:'wx'});
const driverNames=['continuation.mjs','continuation-fixture.mjs','continuation-context.mjs','checkpoint-contract.mjs','manual-start.mjs','automatic-completion.mjs','run-with-observer.py'];
const reusedHelpers=['task-context-delivery.mjs','four-project-queue/native-terminal-diagnostics.mjs','four-project-queue/sample-linux-owned.py','launch-recovery/fenced-docker-proxy.mjs','launch-recovery/worker-create-guard.mjs'];
mkdirSync(join(evidence, 'accepted-harness'), { mode: 0o700 });
for (const name of driverNames) writeFileSync(join(evidence, 'accepted-harness', name), readFileSync(new URL(name, import.meta.url)), { mode: 0o600, flag: 'wx' });
for (const name of reusedHelpers) writeFileSync(join(evidence,'accepted-harness',name.replaceAll('/','_')),readFileSync(join(repo,'scripts/acceptance',name)),{mode:0o600,flag:'wx'});
const fixtureParent = resolve(option('--fixture-root', 'artifacts/quality-swarm/private-fixtures')); mkdirSync(fixtureParent, {recursive:true,mode:0o700});
const fixture = mkdtempSync(join(fixtureParent,'luci-continuation-'));
for (const folder of ['home', 'projects', 'state', 'docker']) mkdirSync(join(fixture, folder), { mode: 0o700 });
const key = 'fixture-only-' + randomUUID(), lifecycle = randomUUID() + randomUUID(), password = randomUUID() + randomUUID();
const sensitive = [key, lifecycle, password], scrub = value => sensitive.reduce((text, secret) => text.replaceAll(secret, '[redacted]'), String(value));
const receipt = {
  passed: false, mode:'continuation-installed-scripted', startedAt: new Date().toISOString(), sourceCommit: provenance.sourceCommit, sourceDirty: provenance.sourceDirty,
  harnessCheckout: {commit:helperCommit,dirty:helperDirty,expectedSource:expectedHelperSource,productSource:provenance.sourceCommit,separateHelperRevision:helperCommit!==provenance.sourceCommit},
  packageIntegrity, initialRootFreeBytes: initialRootFree,
  resourceScope: 'Exactly two real workers for one continuation (partial,finish), one at a time; a dependent task is never authorized to run. Zero existing containers and six GiB root admission before every Create, no recovery fences allowed; hard one GiB running reserve. No image pulls, release or cloud allocation.',
  binarySHA256: sha(readFileSync(binary)), webIndexSHA256: sha(readFileSync(join(web, 'index.html'))), driverSHA256: sha(readFileSync(fileURLToPath(import.meta.url))),
  helperSHA256: Object.fromEntries(driverNames.filter(name=>name!=='continuation.mjs').map(name => [name, sha(readFileSync(new URL(name, import.meta.url)))])),
  reusedHelperSHA256:Object.fromEntries(reusedHelpers.map(name=>['scripts/acceptance/'+name,sha(readFileSync(join(repo,'scripts/acceptance',name)))])),
  image: { reference: image, id: imageInfo.Id }, releaseImage: { reference: releaseImage, id: releaseImageInfo.Id },
  scope: 'Installed automatic start/completion/report/Home observation, complete checkpoint review, stale/unauthenticated refusal, genuine lost HTTP reservation reply, exact idempotent readback, reserved-seed controller restart, attributed previous-attempt context, natural exits and reviewed integration. Scripted provider; no live-model or Windows claim.',
  limitations: ['Existing trusted Docker images; installation is not exercised.', 'Provider output is deliberately scripted; this proves actual CLI, Docker, task coordination and review mechanics. It does not measure model reasoning, production application quality, cloud deployment or Windows behavior.'],
  externalModelCalls: 0, cloudResourcesCreated: 0, setupScaffolding: ['API-seeded personal owner and one deterministic source/check/context fixture','API-configured one-worker policy; browser starts, report answer, checkpoint and integration consent','Explicit stale-review and blocked-dependency API probes; no fixture repair of private product ledgers','Local scripted provider, no external model calls'], checks: [], browserMutations: [], browserErrors: [], cleanup: [],
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
let previousAttempt,checkpointReservation;
let stage = 'startup', fixtureError, reserveTimer, deadlineTimer, closing = false, signalFailure;
let providerCalls = 0, auxiliaryCalls = 0; const observations=new Map(), taskData=new Map(), calls=new Map(); let browser, page, run;
let providerBase; const initialBundles=new Map(), projectStates=[], titles=new Map();
const started = Date.now(), deadline = started + 15 * 60_000;
const remaining = () => { if (Date.now() >= deadline) throw Error('Fifteen minute acceptance deadline expired'); return deadline - Date.now(); };
const remainingBudget = (ms = 120_000) => Math.max(1, Math.min(ms, remaining()));
const redactError = error => scrub(error?.stack ?? error).replaceAll(fixture, '[private-fixture]').slice(0, 8000);
function stopOnFailure(error) { if (!fixtureError) fixtureError = error; for (const socket of sockets) socket.close(); if (service?.exitCode === null) service.kill('SIGTERM'); }
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
async function api(method, path, body, expected = 200, timeout = 30_000) {
  const response = await fetch(base + path, {method,redirect:'error',signal:AbortSignal.timeout(closing ? timeout : remainingBudget(timeout)),headers:{Origin:base,...(token?{Authorization:'Bearer '+token}:{}),...(body===undefined?{}:{'Content-Type':'application/json'})},body:body===undefined?undefined:JSON.stringify(body)});
  (receipt.apiActions ??= []).push({stage,method,path,status:response.status,at:new Date().toISOString()});
  const raw = await boundedResponse(response,2*1024**2); let value; try { value = raw ? JSON.parse(raw) : {}; } catch { throw Error('Invalid JSON response: '+method+' '+path+' '+response.status); }
  assert.equal(response.status, expected, method+' '+path+': '+response.status+' '+scrub(value.message??value.error??''));
  return value;
}
async function until(label, fn, ms = 120_000) {
  const end = Date.now() + remainingBudget(ms);
  while (Date.now() < end) { if (fixtureError) throw fixtureError; if (service && service.exitCode !== null) throw Error('Owned service exited unexpectedly'); const value=await fn(); if(value)return value; await delay(150); }
  throw Error(label+' did not complete within its bounded prerequisite window');
}
async function freePort() { const server=createServer(); await new Promise((done,reject)=>server.once('error',reject).listen(0,'127.0.0.1',done));const port=server.address().port;await new Promise(done=>server.close(done));return port; }
async function startService() {
  service=spawn(binary,[],{cwd:fixture,env,detached:true,stdio:['ignore','ignore','ignore']});service.on('error',stopOnFailure);controllerIdentities.push(processIdentity(service.pid));
  const end=Date.now()+remainingBudget(45_000);let ready;
  while(Date.now()<end) { if(fixtureError)throw fixtureError;assert.equal(service.exitCode,null);try{const res=await fetch(base+'/v1/control-plane/status',{signal:AbortSignal.timeout(1500)});if(res.ok){ready=await res.json();break;}}catch{}await delay(150); }
  assert.equal(ready?.product,'Luci');assert.equal(ready?.version,'0.5.0');
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
 const queueCompletedObservedAt=null;observeTaskLease(worker);
 const current=privateRun(owner);
 const task=current.swarm.tasks.find(value=>value.id===worker.taskId);assert.notEqual(task.integration?.state,'integrated');
 assert(!receipt.apiActions.some(value=>value.method==='DELETE'&&value.path===workspace+'/v1/sessions/'+worker.session.id),'Success workflow must not issue a worker stop');
 (receipt.naturalCompletions??=[]).push({task:worker.task,sessionID:worker.session.id,sessionStatus:stopped.status,recordedExitReason:stopped.exitReason,cleanNativeExitFrame:true,exitFrameObservedAt:probe.snapshot().exitObservedAt,stoppedAndAbsentObservedAt,queueCompletedObservedAt,leaseObservation:worker.leaseObservation,queueState:'not-queued',codingDispatches:worker.codingDispatches,finalResponseRequests:worker.finalResponseRequests,finalResponseAborts:worker.finalResponseAborts??0,finalResponseRetries:(worker.finalResponseRequests??1)-1,integratedBeforeExit:false});
 return stopped;
}

function streamResponse(res,item,sequence){
  const response={id:'resp_swarm_'+sequence,object:'response',created_at:1,status:'completed',model:'swarm-mechanics-fixture',output:[item],usage:{input_tokens:1,output_tokens:1,total_tokens:2,input_tokens_details:{cached_tokens:0},output_tokens_details:{reasoning_tokens:0}}};
  const events=[{type:'response.created',response:{...response,status:'in_progress',output:[]}},{type:'response.output_item.added',output_index:0,item:{...item,status:'in_progress',...(item.type==='function_call'?{arguments:''}:{content:[]})}}];
  if(item.type==='function_call')events.push({type:'response.function_call_arguments.delta',item_id:item.id,output_index:0,delta:item.arguments},{type:'response.function_call_arguments.done',item_id:item.id,output_index:0,arguments:item.arguments});
  else events.push({type:'response.content_part.added',item_id:item.id,output_index:0,content_index:0,part:{type:'output_text',text:'',annotations:[]}},{type:'response.output_text.delta',item_id:item.id,output_index:0,content_index:0,delta:item.content[0].text},{type:'response.output_text.done',item_id:item.id,output_index:0,content_index:0,text:item.content[0].text});
  events.push({type:'response.output_item.done',output_index:0,item},{type:'response.completed',response});res.writeHead(200,{'content-type':'text/event-stream'});res.end(events.map((e,i)=>'event: '+e.type+'\ndata: '+JSON.stringify({...e,sequence_number:i})+'\n\n').join(''));
}
async function attach(id){
  const path=workspace+'/v1/sessions/'+id+'/attach';const {ticket}=await api('POST','/v1/auth/socket-ticket',{path});sensitive.push(ticket);
  const socket=new WebSocket(base.replace('http:','ws:')+path+'?ticket='+encodeURIComponent(ticket));socket.binaryType='arraybuffer';sockets.add(socket);socketSessions.set(socket,id);
  assert(terminalProbes.length<8);const probe=createNativeTerminalProbe();terminalProbes.push({id,attachedStage:stage,probe});
  socket.addEventListener('message',event=>probe.frame(Buffer.from(event.data)));
  socket.addEventListener('close',event=>{probe.close(event.code,event.wasClean);sockets.delete(socket);socketSessions.delete(socket);});socket.addEventListener('error',()=>probe.error());
  await new Promise((done,reject)=>{const timeout=setTimeout(()=>reject(Error('Authenticated attach timed out')),remainingBudget(10_000));socket.addEventListener('open',()=>{clearTimeout(timeout);socket.send(Uint8Array.from([2,0,120,0,40]));done();},{once:true});socket.addEventListener('error',()=>{clearTimeout(timeout);reject(Error('Authenticated attach failed'));},{once:true});});
  return socket;
}

function outputs(input,result=[]) {if(!input||typeof input!=='object')return result;if(input.type==='function_call_output')result.push(input);for(const v of Array.isArray(input)?input:Object.values(input))outputs(v,result);assert(result.length<=100);return result;}
function message(text){return {id:'msg_swarm_'+providerCalls,type:'message',status:'completed',role:'assistant',content:[{type:'output_text',text,annotations:[]}]};}
function commandFor(key){const value=taskData.get(key);assert(value);return taskCommand({variant:key,operation:value.operation.id,projectId:value.owner.id,runId:owner.run.id,baseCommit:value.operation.checkpoint?.seedCommit??value.operation.baseCommit,contextHashes:hashes(initialBundles.get(value.session.id)),dependencyDigest:value.operation.dependencyContext.digest,previousAttempt:value.task==='finish'?previousAttempt:undefined,expectedUid:process.getuid()});}
function nextTool(state,name,args){if(name==='exec_command'){state.codingDispatches=(state.codingDispatches??0)+1;assert.equal(state.codingDispatches,1,'No duplicate task command');}assert(++state.count<=48,'Bounded task tool request cap');const callID='call_swarm_'+state.task+'_'+state.operation.id+'_'+state.count;calls.set(callID,{state,consumed:false});return {id:'fc_'+callID,type:'function_call',status:'completed',name,call_id:callID,arguments:JSON.stringify(args)};}
function startProvider(){
 provider=httpServer(async(req,res)=>{try{
  assert.equal(req.method,'POST');assert.equal(req.headers.authorization,'Bearer '+key);if(closing){res.writeHead(503);res.end();return;}
  assert.equal(req.url,'/v1/responses');const parts=[];let bytes=0;for await(const part of req){bytes+=part.length;assert(bytes<4*1024**2);parts.push(part);}const body=JSON.parse(Buffer.concat(parts));assert.equal(body.model,'swarm-mechanics-fixture');assert(++providerCalls<=80);
  const resultItems=outputs(body.input),pending=resultItems.filter(v=>calls.has(v.call_id)&&!calls.get(v.call_id).consumed);assert(pending.length<=1);let response;
  if(pending.length){const output=pending[0],entry=calls.get(output.call_id),state=entry.state;entry.consumed=true;const raw=typeof output.output==='string'?output.output:JSON.stringify(output.output);assert(!/Process exited with code (?:[1-9]\d*|-\d+)|"exit_code"\s*:\s*(?:[1-9]\d*|-\d+)|Command failed/.test(raw));
   const matches=[...raw.matchAll(/LUCI_CONTINUATION_OBSERVATION=([A-Za-z0-9+/=]+)/g)];
   if(matches.length){assert.equal(matches.length,1);assert(!observations.has(state.task));const actual=JSON.parse(Buffer.from(matches[0][1],'base64'));assert.equal(actual.variant,state.task);assert.equal(actual.taskId,state.taskId);assert.equal(actual.operation,state.operation.id);assert.equal(actual.uid,process.getuid());assert.equal(actual.cwd,'/home/dev/work');assert.equal(actual.readOnly,true);assert.deepEqual(actual.reported,taskHandoff(state.task));assert.deepEqual(hashes(actual.files),hashes(initialBundles.get(state.session.id)));assert.deepEqual(actual.changed,changedPaths(state.task));assert.equal(actual.artifact.baseCommit,state.operation.checkpoint?.seedCommit??state.operation.baseCommit);assert.equal(actual.artifact.check.code,0);assert.equal(actual.artifact.check.stdout,'CONTINUATION_CHECK_OK '+state.task+'\n');assert.match(actual.commit,/^[a-f0-9]{40,64}$/);observations.set(state.task,actual);entry.finalTask=state.task;entry.finalOutputSHA256=sha(raw);state.finalResponseRequests=1;response=message('Controlled continuation '+state.task+' committed. Human integration and build verification remain separate.');}
   else{const running=raw.match(/Process running with session ID (\d+)/)||raw.match(/"session_id"\s*:\s*(\d+)/);assert(running,'Actual command output must identify live command or completion');response=nextTool(state,'write_stdin',{session_id:Number(running[1]),chars:'',yield_time_ms:1000,max_output_tokens:12000});}
  }else if(resultItems.some(value=>calls.get(value.call_id)?.finalTask)){
   const finals=resultItems.filter(value=>calls.get(value.call_id)?.finalTask);assert.equal(finals.length,1);const output=finals[0],entry=calls.get(output.call_id),state=entry.state,raw=typeof output.output==='string'?output.output:JSON.stringify(output.output);assert.equal(sha(raw),entry.finalOutputSHA256);assert(observations.has(state.task));assert.equal(state.codingDispatches,1);assert(state.finalResponseRequests<3);state.finalResponseRequests++;response=message('Controlled continuation '+state.task+' committed. Human integration remains separate.');
  }else if((body.tools??[]).some(t=>(t.name??t.function?.name)==='exec_command')){
   const text=JSON.stringify(body.input);assert(text.includes('Reviewed context directory: '+authorityRoot));const matched=[...titles].filter(([id,title])=>text.includes(id+': '+title));assert.equal(matched.length,1);const taskId=matched[0][0];
   const state=await until('Verified exact task bundle before native command',()=>[...taskData.values()].find(value=>value.taskId===taskId&&value.count===0&&value.attached===true&&initialBundles.has(value.session.id)),120000);response=nextTool(state,'exec_command',commandFor(state.task));
  }else{assert(++auxiliaryCalls<=4);response=message('Controlled continuation fixture acknowledgement.');}
  (receipt.providerRequests??=[]).push({number:providerCalls,toolOutputCount:resultItems.length,issuedTool:response.type==='function_call'?response.name:null});streamResponse(res,response,providerCalls);
 }catch(error){stopOnFailure(error);if(!res.headersSent)res.writeHead(500);res.end('Controlled continuation fixture contract failed');}});
}
async function action(path,click,expected=200){const pending=page.waitForResponse(r=>new URL(r.url()).pathname===path&&r.request().method()==='POST',{timeout:remainingBudget(150000)});await click();const response=await pending;assert.equal(response.status(),expected,path+': '+await response.text());return response.json();}
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
 const contract=variants[key];assert(contract);const operation=await until('Exact explicitly started child '+key,async()=>{const candidate=(await api('GET',owner.route+'/runs/'+owner.run.id)).run;owner.run=candidate;run=candidate;return candidate.operations.find(op=>op.taskId===contract.taskId&&op.attempt===contract.attempt&&op.status==='running');},150000);
 const session=await api('GET','/v1/sessions/'+operation.childId);ownedSessions.add(session.id);const worktrees=(await api('GET','/v1/projects/'+owner.owner.id+'/worktrees')).worktrees,checkout=worktrees.find(w=>w.id===operation.worktreeId);assert(checkout);
 const state={task:key,taskId:contract.taskId,owner:owner.owner,operation,session,checkout:checkout.path,count:0,requestedLimits:{memoryMiB:1024,cpuMillis:1000}};taskData.set(key,state);
 const actual=live(session.id),bundle=bundleSnapshot(session.id);const contextDelivery=verifyTaskContextDelivery({session,stored:row(session.id),run:owner.run,operation,manifestBytes:readFileSync(join(bundles.get(session.id),'manifest.json')),version:2});assert.equal(contextDelivery.manifestDigest,bundle['manifest.json'].sha256);initialBundles.set(session.id,bundle);state.container=actual;
 const manifest=privateJSON(join(bundles.get(session.id),'manifest.json')),packet=privateJSON(join(bundles.get(session.id),dependencyFile),8*1024**2);validateContinuationContext(manifest,bundle,{projectId:owner.owner.id,runId:run.id,taskId:contract.taskId,operationId:operation.id,dependencyDigest:operation.dependencyContext?.digest,previousAttempt:key==='finish'?previousAttempt:undefined},packet);
 (receipt.dependencyPackets??=[]).push({variant:key,version:manifest.version,packet,manifestDigest:bundle['manifest.json'].sha256,dependencyContextDigest:manifest.binding.dependencyContextDigest});
 for(const name of canonicalNames.filter(name=>name!=='LUCI.md'))assert.equal(bundle[name].sha256,owner.canonicalBefore[name]);const luci=readFileSync(join(bundles.get(session.id),'LUCI.md'));assert(luci.subarray(0,owner.canonicalLuci.length).equals(owner.canonicalLuci));assert.deepEqual(row(session.id).implementationContext.taskIds,[contract.taskId]);assert.equal(workerContainers().length,1);
 (receipt.contextCaptures??=[]).push({variant:key,taskId:contract.taskId,operationID:operation.id,childID:session.id,baseCommit:operation.baseCommit,seedCommit:operation.checkpoint?.seedCommit,reviewedContextDelivery:contextDelivery.delivery,reviewedContextReference:contextDelivery.reference,hashes:hashes(bundle)});await attach(session.id);state.attached=true;return state;
}
async function setPaused(paused){
 const settings=await api('GET','/v1/product/swarm-scheduler');
 return api('PUT','/v1/product/swarm-scheduler',{expectedRevision:settings.revision,policy:{maxActive:1,maxPerProject:1,headroomPercent:25,minHeadroomMiB:2048,diskReserveMiB:1024},dispatchPaused:paused});
}

async function readReview(key){
 const worker=taskData.get(key);assert(worker);stage='browser-review-'+key;await openProject(owner);const pending=page.waitForResponse(r=>new URL(r.url()).pathname===owner.route+'/runs/'+owner.run.id+'/swarm/tasks/'+worker.taskId+'/review'&&r.request().method()==='GET');await taskRow(worker.taskId).getByRole('button',{name:'Review task changes',exact:true}).click();const response=await pending;assert.equal(response.status(),200);const {review}=await response.json();assert.equal(review.canIntegrate,true);assert.equal(review.sourceCommit,observations.get(key).commit);assert.deepEqual([...review.paths].sort(),changedPaths(key));const dialog=page.getByRole('dialog',{name:'Review task changes',exact:true});await dialog.getByLabel('Reviewed task diff').waitFor();assert((await dialog.getByLabel('Reviewed task diff').innerText()).includes('result.json'));return {review,dialog};
}
async function integrate(key,expected='integrated'){
 const {review,dialog}=await readReview(key);await dialog.getByRole('checkbox').check();const result=await action(owner.route+'/runs/'+owner.run.id+'/swarm/tasks/'+variants[key].taskId+'/integrate',()=>dialog.getByRole('button',{name:'Integrate reviewed changes',exact:true}).click());owner.run=result.run;run=owner.run;const integration=run.swarm.tasks.find(t=>t.id===variants[key].taskId).integration;assert.equal(integration.state,expected);(receipt.integrations??=[]).push({variant:key,review,integration});return integration;
}
async function boundedResponse(response, limit) {
 const parts=[];let size=0;
 for await(const part of response.body??[]){size+=part.length;if(size>limit){await response.body?.cancel().catch(()=>{});throw Error('HTTP response bound exceeded');}parts.push(part);}
 return Buffer.concat(parts,size).toString('utf8');
}
function privateJSON(path,limit=2*1024**2){const info=lstatSync(path);assert(info.isFile()&&!info.isSymbolicLink()&&info.nlink===1&&info.size<=limit);const bytes=readFileSync(path);assert(bytes.length<=limit);return JSON.parse(bytes);}
function privateRun(owner){const record=privateJSON(join(fixture,'state/sessions/project-runs',owner.owner.id,owner.run.id+'.json'));assert.equal(record.run.id,owner.run.id);assert.equal(record.run.projectId,owner.owner.id);return record.run;}
function instanceID(){const value=readFileSync(join(fixture,'state/instance/instance-id'),'utf8').trim();assert(value.length>0&&value.length<=200);return value;}
function workerContainers(){return docker('ps','-aq','--no-trunc','--filter','label=luci.managed=true').split(/\s+/).filter(Boolean);}
function identifyCreate(bytes,meta){
 const input=JSON.parse(bytes),sessionID=input.Labels?.['luci.session-id'];assert.match(sessionID??'',/^s-[a-f0-9]{8}$/);assert.equal(meta.name,'luci-'+sessionID);assert.equal(input.Labels?.['luci.managed'],'true');assert.equal(input.Labels?.['luci.launch-fence'],undefined);assert(owner);
 const saved=privateRun(owner),op=saved.operations.find(value=>value.childId===sessionID);assert(op);variantFor(op.taskId,op.attempt);const current=saved.swarm.tasks.find(t=>t.id===op.taskId);assert.equal(current.operationId,op.id);assert.equal(op.taskId,'task-work','Dependent task is never authorized by this fixture');ownedSessions.add(sessionID);
 validateWorkerCreate(input,{fixture,projectID:owner.owner.id,runID:owner.run.id,operationID:op.id,session:row(sessionID),images:[image,imageInfo.Id],instance:instanceID(),uid:process.getuid(),gid:process.getgid(),taskContextVersion:2});return 'worker';
}
function admission({kind,ordinal,body,...meta}){
 try{assert.equal(kind,'worker');assert(ordinal>=1&&ordinal<=2);identifyCreate(body,meta);const free=rootFree();assert(free>=6*1024**3,'Six GiB pre-create guard remains mandatory');assert.equal(workerContainers().length,0,'No overlapping full worker permitted');(receipt.resourceObservations??=[]).push({at:new Date().toISOString(),stage,ordinal,rootFreeBytes:free});}catch(error){stopOnFailure(error);throw error;}
}
function health(){remaining();if(fixtureError)throw fixtureError;if(service&&service.exitCode!==null)throw Error('Owned service exited unexpectedly');}
async function timed(promise,ms,label){let timer;try{return await Promise.race([promise,new Promise((_,reject)=>{timer=setTimeout(()=>reject(Error(label+' deadline exceeded')),ms);})]);}finally{clearTimeout(timer);}}




async function prepare(owner){await openProject(owner);await page.getByRole('button',{name:'Prepare agent team',exact:true}).click();const dialog=page.getByRole('dialog',{name:'Prepare agent team',exact:true});await dialog.getByRole('combobox').selectOption('1');assert.equal(await dialog.getByRole('combobox').inputValue(),'1');owner.run=(await action(owner.route+'/runs/'+owner.run.id+'/swarm',()=>dialog.getByRole('button',{name:'Prepare team',exact:true}).click(),201)).run;}
function seedSource(project){for(const [name,text] of Object.entries(initialFiles)){assert(Object.keys(initialFiles).includes(name));const file=join(project.path,name);assert(!existsSync(file)||lstatSync(file).isFile());writeFileSync(file,text,{mode:0o600});}}
async function saveContext(projectOwner,connectionID){
 project=projectOwner;projectRoute='/v1/product/projects/'+project.id;const goal='Preserve unfinished committed, dirty and untracked changes through an explicitly reviewed continuation; resolve its recorded question before a distinct successor finishes.';
 await api('POST',projectRoute+'/brief',{revision:0,purpose:goal,audience:'Project owners',constraints:'Controlled offline continuation fixture. Exactly two worker starts. The dependent task is intentionally not authorized and must remain unallocated.',criteria:['partial=17 retained','committed and unfinished source preserved','sum42 after explicit answer and continuation']});
 let spec=await api('GET',projectRoute+'/specification');spec=await api('POST',projectRoute+'/specification',{expectedRevision:spec.revision,expectedBriefRevision:spec.briefRevision,specification:{...spec,purpose:goal,requirements:[{id:'AC-continuation',title:'Reviewed retained-source continuation',priority:'must',acceptance:'Keep the original partial source unchanged, carry only completely reviewed files into a distinct seed, and require separate start, integration and independent checks.'}]}});spec=await api('POST',projectRoute+'/specification/approve',{revision:spec.revision,briefRevision:spec.briefRevision});
 let design=await api('GET',projectRoute+'/design-plan');design=await api('POST',projectRoute+'/design-plan',{expectedRevision:design.revision,expectedBriefRevision:spec.briefRevision,expectedSpecificationRevision:spec.revision,plan:{...design,screens:[{id:'result',name:'Combined verified result',route:'/',purpose:'Show retained partial17 and final sum42',states:['Unfinished','Checkpoint reserved','Continued','Integrated'],widths:[390,1440],requirementIds:['AC-continuation']}]}});design=await api('POST',projectRoute+'/design-plan/approve',{revision:design.revision});
 const tasks=taskContracts.map(value=>({...value,status:'todo',requirementIds:['AC-continuation']}));const plan=await api('POST',projectRoute+'/plan',{expectedRevision:0,expectedSpecificationRevision:spec.revision,tasks});const gitReview=await api('GET',projectRoute+'/git');await api('POST',projectRoute+'/git/commit',{message:'Fixture setup: reviewed continuation contract and independent checks',expectedReviewDigest:gitReview.reviewDigest});
 const branches=(await api('GET','/v1/projects/'+project.id+'/branches')).branches,branch=branches.find(row=>row.name==='main')??branches[0];assert(branch);const worktrees=(await api('GET','/v1/projects/'+project.id+'/worktrees?branchId='+encodeURIComponent(branch.id))).worktrees,worktree=worktrees.find(row=>row.isPrimary)??worktrees[0];assert(worktree);
 const selection={projectId:project.id,briefRevision:spec.briefRevision,specificationRevision:spec.revision,designPlanRevision:design.revision,planRevision:plan.revision,taskIds:tasks.map(t=>t.id)};const saved=(await api('POST',projectRoute+'/runs',{worktreeId:worktree.id,harness:'codex',model:'swarm-mechanics-fixture',permissionMode:'bypass',agentConnectionId:connectionID,outcome:'preview',implementationContext:selection,idempotencyKey:'continuation-reviewed-build'},201)).run;
 receipt.acceptedIntent={tasks,selection,publicRunInputSHA256:sha(JSON.stringify(saved.input)),inputHashBasis:'Helper-derived SHA256 of JSON.stringify(public ProjectRun.input), not the private controller inputDigest',approvals:saved.input.approvals};return {owner:project,route:projectRoute,run:saved,goal,initialHead:execFileSync('git',['rev-parse','HEAD'],{cwd:project.path,encoding:'utf8'}).trim(),canonicalBefore:canonical(),canonicalLuci:readFileSync(join(project.path,'LUCI.md'))};
}
function gitAt(directory,...args){assert(isFixturePath(directory));return execFileSync('git',['-c','core.hooksPath=/dev/null',...args],{cwd:directory,env:{PATH:process.env.PATH,HOME:join(fixture,'home'),GIT_CONFIG_NOSYSTEM:'1',GIT_CONFIG_GLOBAL:join(fixture,'home/.gitconfig'),GIT_OPTIONAL_LOCKS:'0'},encoding:'utf8',timeout:15000,maxBuffer:1024*1024}).trim();}
function isFixturePath(directory){return typeof directory==='string'&&directory.startsWith(fixture+sep)&&realpathSync(directory)===directory&&lstatSync(directory).isDirectory();}
async function refreshRun(){owner.run=(await api('GET',owner.route+'/runs/'+owner.run.id)).run;run=owner.run;return run;}
async function observeAutomaticCompletion(key,worker){
 stage='automatic-completion-'+key;
 const startedAt=new Date().toISOString(),deadline=Date.now()+60000,before=effects(),apiStart=receipt.apiActions.length,browserStart=receipt.browserMutations.length;
 const readBudget=()=>{assert(Date.now()<deadline,'Automatic completion exceeded its 60-second observation deadline');return Math.max(1,Math.min(10000,deadline-Date.now()));};
 const expected={projectId:owner.owner.id,runId:owner.run.id,operation:worker.operation,sourceCommit:observations.get(key).commit,contextDigest:initialBundles.get(worker.session.id)['manifest.json'].sha256,report:taskHandoff(key),sourceDirty:key==='partial',attentionKind:key==='partial'?'task-question':'task-review'};
 const handoffPath=owner.route+'/runs/'+owner.run.id+'/swarm/operations/'+worker.operation.id+'/handoff';
 let samples=0;
 const observed=await until('Automatic terminal run, captured report and Home attention '+key,async()=>{
  samples++;const latest=(await api('GET',owner.route+'/runs/'+owner.run.id,undefined,200,readBudget())).run;owner.run=latest;run=latest;const op=latest.operations.find(value=>value.id===worker.operation.id);
  const handoff=op?.handoff?.status==='captured'?(await api('GET',handoffPath,undefined,200,readBudget())).handoff:null;
  const overview=await api('GET',owner.route+'/overview',undefined,200,readBudget());
  const value=automaticCompletion({run:latest,handoff,overview,expected});
  if(!value)await delay(850);return value;
 },60000);
 assert(Date.now()<=deadline,'Automatic completion was observed after its deadline');
 const backendObservedAt=new Date().toISOString();
 const inPlace=await until('Current task card automatically exposes the exact completed attempt '+key,async()=>{
  readBudget();const card=taskRow(worker.taskId),review=card.getByRole('button',{name:'Review task changes',exact:true}),report=card.getByRole('button',{name:'Task result and questions',exact:true});
  const reviewVisible=await review.isVisible(),reportVisible=await report.isVisible();
  return inPlaceCompletion({statusVisible:await card.getByText('Awaiting review',{exact:true}).isVisible(),reviewVisible,reviewEnabled:reviewVisible&&await review.isEnabled(),reportVisible,reportEnabled:reportVisible&&await report.isEnabled(),questionVisible:await card.getByText(/^1 agent-reported question needs an answer\./).isVisible(),scopeText:await card.locator('[data-task-scope]').textContent()},expected);
 },Math.max(1,deadline-Date.now()));
 assert(Date.now()<=deadline,'In-place task completion appeared after its deadline');
 inPlace.observedAt=new Date().toISOString();
 await page.screenshot({path:join(evidence,'automatic-task-card-'+key+'.png'),fullPage:true});
 assertReadOnlyCompletionRequests([...receipt.apiActions.slice(apiStart),...receipt.browserMutations.slice(browserStart)],owner.route);
 // Opening Home and its exact recorded build is read-only. Neither the fixture
 // nor the browser may post a reconcile/collect to manufacture completion.
 await page.getByRole('button',{name:'All projects',exact:true}).click();await page.getByRole('heading',{name:'Across your projects',exact:true}).waitFor();
 const card=page.locator('.overview-card').filter({has:page.getByRole('button',{name:'Open project '+owner.owner.name,exact:true})});assert.equal(await card.count(),1);
 const attention=card.getByRole('region',{name:'Recorded review work',exact:true});await attention.waitFor();
 const label=key==='partial'?'Review reported task questions':'Task changes to review';await attention.getByRole('button',{name:'Inspect '+owner.run.id+': '+label,exact:true}).waitFor();
 await page.screenshot({path:join(evidence,'automatic-home-'+key+'.png'),fullPage:true});
 assertReadOnlyCompletionRequests([...receipt.apiActions.slice(apiStart),...receipt.browserMutations.slice(browserStart)],owner.route);
 assert.deepEqual(effects(),before,'Automatic observation must not allocate, start or clean another worker');
 (receipt.automaticCompletions??=[]).push({variant:key,startedAt,backendObservedAt,homeObservedAt:new Date().toISOString(),samples,operationID:worker.operation.id,childID:worker.session.id,recordedStatus:observed.operation.status,handoffDigest:observed.handoff.digest,sourceCommit:observed.handoff.receipt.sourceCommit,sourceDirty:observed.handoff.receipt.sourceDirty,contextDigest:observed.handoff.receipt.contextDigest,inPlace,attention:observed.attention,homeVisible:true,manualReconcile:false,manualCollection:false,recordedChecks:observed.handoff.recordedChecks,integrated:false});
 check('Natural exit automatically captured and surfaced on Home',{variant:key,operationID:worker.operation.id,kind:observed.attention.kind,samples});
 await openProject(owner);await taskRow(worker.taskId).getByText('Awaiting review',{exact:true}).waitFor();return observed;
}
async function startManualTask(key,label){
 const variant=variants[key],path=owner.route+'/runs/'+owner.run.id+'/swarm/reconcile',priorChecks=receipt.browserMutations.filter(item=>item.method==='POST'&&item.path===path).length;
 // CLOUD-017 must perform the real post-start observation. This is armed
 // before the click; a manual status action would hide a regression here.
 const observed=page.waitForResponse(response=>new URL(response.url()).pathname===path&&response.request().method()==='POST',{timeout:remainingBudget(150000)});observed.catch(()=>{});
 const session=await action('/v1/sessions',()=>taskRow(variant.taskId).getByRole('button',{name:label,exact:true}).click(),201),before=effects();
 const confirmed=await reconcileManualStart({session,selection:{projectId:owner.owner.id,runId:owner.run.id,taskId:variant.taskId,attempt:variant.attempt},readRun:refreshRun,reconcileRun:async()=>{const response=await observed;assert.equal(response.status(),200,'Automatic post-start observation must succeed');const result=await response.json();owner.run=result.run;run=owner.run;return run;}});
 await taskRow(variant.taskId).getByText('Recorded as running',{exact:true}).waitFor();assert.equal(await taskRow(variant.taskId).getByText('Start outcome unconfirmed',{exact:true}).count(),0);
 assert.equal(receipt.browserMutations.filter(item=>item.method==='POST'&&item.path===path).length,priorChecks+1,'Exactly one automatic observation, no manual fallback or replay');
 assert.deepEqual(effects(),before,'Status observation must not allocate or start another worker');
 (receipt.manualStartObservations??=[]).push({variant:key,sessionID:session.id,operationID:confirmed.operation.id,beforeStatus:confirmed.beforeStatus,observedStatus:confirmed.operation.status,beforeRevision:confirmed.beforeRevision,observedRevision:confirmed.run.revision,automaticBrowserReconcile:true,manualFallback:false});check('Exact manual child automatically observed as running',{variant:key,operationID:confirmed.operation.id});return session;
}
async function complete(key){stage='native-command-'+key;const worker=await queuedChild(key);await until('Native exact result '+key,()=>observations.has(key),120000);assert.equal(gitAt(worker.checkout,'rev-parse','HEAD'),observations.get(key).commit);assert.deepEqual(bundleSnapshot(worker.session.id),initialBundles.get(worker.session.id));await completeNaturally(worker,owner);return worker;}
function sourceSnapshot(directory,files){
 const values={};for(const [name,expected] of Object.entries(files)){assert(!name.includes('/')&&!name.includes('..'));const path=join(directory,name),info=lstatSync(path);assert(info.isFile()&&!info.isSymbolicLink()&&info.nlink===1);const bytes=readFileSync(path);assert.equal(bytes.toString('utf8'),expected);values[name]={sha256:sha(bytes),bytes:bytes.length,mode:info.mode&0o777};}
 return {head:gitAt(directory,'rev-parse','HEAD'),status:gitAt(directory,'status','--porcelain'),index:gitAt(directory,'ls-files','--stage'),files:values};
}
function assertPrimary(expected){assert.deepEqual({head:gitAt(owner.owner.path,'rev-parse','HEAD'),status:gitAt(owner.owner.path,'status','--porcelain'),canonical:canonical()},expected);}
function ownedWorktreeNames(){const root=join(fixture,'projects/.luci-swarms',owner.owner.id,owner.run.id);return existsSync(root)?readdirSync(root).sort():[];}
async function assertJoinBlocked(label){
 await refreshRun();const before=privateRun(owner),beforeEffects=effects(),beforeTrees=ownedWorktreeNames();assert(!before.operations.some(op=>op.taskId==='task-join'));assert.notEqual(before.swarm.tasks.find(t=>t.id==='task-work').integration?.state,'integrated');
 await api('POST',owner.route+'/runs/'+owner.run.id+'/swarm/tasks/task-join/reserve',{expectedRevision:owner.run.revision,idempotencyKey:'join-refusal-'+label},409);assert.deepEqual(privateRun(owner),before);assert.deepEqual(effects(),beforeEffects);assert.deepEqual(ownedWorktreeNames(),beforeTrees);(receipt.dependencyRefusals??=[]).push({stage:label,http:409,revision:owner.run.revision,noOperation:true,noWorkspace:true,noCreateOrStart:true});
}
async function inspectCheckpoint(operation,screenshot){
 await openProject(owner);await openTaskActions(operation.taskId);const route=owner.route+'/runs/'+owner.run.id+'/swarm/operations/'+operation.id+'/checkpoint',pending=page.waitForResponse(r=>new URL(r.url()).pathname===route&&r.request().method()==='GET');await taskRow(operation.taskId).getByRole('button',{name:'Continue from retained changes',exact:true}).click();const response=await pending;assert.equal(response.status(),200);const review=validateCheckpointReview((await response.json()).review,{projectId:owner.owner.id,runId:owner.run.id,operation,revision:owner.run.revision,targetCommit:owner.initialHead,sourceCommit:observations.get('partial').commit});
 const dialog=page.getByRole('dialog',{name:'Continue from retained changes',exact:true}),section=dialog.locator('[aria-label="Complete retained changes"]');await section.waitFor();assert.equal(await section.locator('details').count(),review.files.length);
 for(const file of review.files){const details=section.locator('details').filter({has:page.locator('summary').getByText(file.path,{exact:true})});assert.equal(await details.count(),1);await details.locator('summary').click();assert.equal(await details.getByLabel('After '+file.path,{exact:true}).textContent(),file.after);if(file.beforeMode)assert.equal(await details.getByLabel('Before '+file.path,{exact:true}).textContent(),file.before);else assert((await details.innerText()).includes('File did not exist.'));}
 assert.equal(await dialog.getByRole('checkbox').isChecked(),false);assert.equal(await dialog.getByRole('button',{name:'Continue from these reviewed changes',exact:true}).isDisabled(),true);await page.screenshot({path:join(evidence,screenshot),fullPage:true});return {review,dialog};
}

let engineID;
try {
 deadlineTimer=setTimeout(()=>stopOnFailure(Error('Fifteen minute acceptance deadline expired')),15*60_000);
 reserveTimer=setInterval(()=>{try{const free=rootFree();(receipt.diskSamples??=[]).push({stage,at:new Date().toISOString(),rootFreeBytes:free});if(free<1024**3)stopOnFailure(Error('Hard one GiB reserve breached'));for(const worker of taskData.values())if(worker.finalReleased&&worker.session)observeTaskLease(worker);}catch(error){stopOnFailure(error);}},1000);
 for(const name of ['SIGTERM','SIGINT'])process.on(name,()=>{signalFailure=name;stopOnFailure(Error('Acceptance interrupted by '+name));});
 receipt.bounds={scenarioDeadlineMs:900000,upstreamRequestMs:120000,maxCreates:2,maxStarts:2,preCreateFreeBytes:6*1024**3,runningReserveBytes:1024**3,workerMemoryBytes:1024**3,workerNanoCPUs:1000000000,pidsLimit:256};
 engineID=docker('info','--format','{{.ID}}');assert(engineID.length>0&&engineID.length<=256);receipt.engineIdentitySHA256=sha(engineID);
 startProvider();await new Promise((done,reject)=>provider.once('error',reject).listen(0,bridge,done));providerBase='http://host.docker.internal:'+provider.address().port+'/v1';
 const port=await freePort(),preview=await freePort();base='http://127.0.0.1:'+port;
 const upstream=process.env.DOCKER_HOST||'unix:///var/run/docker.sock';assert(upstream.startsWith('unix://'));
 proxy=await startFencedDockerProxy({socketPath:join(fixture,'docker-proxy.sock'),upstreamSocketPath:upstream.slice(7),identifyCreate,beforeCreate:admission,maxCreates:receipt.bounds.maxCreates,maxCreatesByKind:{worker:2,fence:1},maxStarts:receipt.bounds.maxStarts,maxObservations:20000,upstreamTimeoutMs:120000,holdTimeoutMs:600000});
 writeFileSync(join(fixture,'docker/config.json'),JSON.stringify({cliPluginsExtraDirs:process.env.DOCKER_CONFIG?[join(process.env.DOCKER_CONFIG,'cli-plugins')]:[]}),{mode:0o600});
 env=Object.fromEntries(['PATH','LANG','LC_ALL','TZ'].filter(name=>process.env[name]).map(name=>[name,process.env[name]]));
 Object.assign(env,{HOME:join(fixture,'home'),DOCKER_CONFIG:join(fixture,'docker'),DOCKER_HOST:'unix://'+join(fixture,'docker-proxy.sock'),LUCI_DOCKER_HOST:'unix://'+join(fixture,'docker-proxy.sock'),LUCI_ADDR:'127.0.0.1:'+port,LUCI_STATE_DIR:join(fixture,'state'),LUCI_PROJECTS_DIR:join(fixture,'projects'),LUCI_WEB_DIR:web,LUCI_RUNTIME_DIR:join(dirname(binary),'runtime'),LUCI_SANDBOX_IMAGE:image,LUCI_BACKEND:'docker',LUCI_SESSION_MEMORY_MIB:'1024',LUCI_SESSION_CPU_MILLIS:'1000',LUCI_MANAGED_SHUTDOWN_TOKEN:lifecycle,LUCI_LOG_LEVEL:'error',LUCI_BROWSER_PREVIEW_ADDR:'127.0.0.1:'+preview,LUCI_BROWSER_PREVIEW_PORT:String(preview),GIT_CONFIG_GLOBAL:join(fixture,'home/.gitconfig'),GIT_CONFIG_NOSYSTEM:'1'});
 await startService();
 const bootstrap=readFileSync(join(fixture,'state/control-plane/bootstrap-token'),'utf8').trim();sensitive.push(bootstrap);
 const account=await api('POST','/v1/auth/bootstrap',{bootstrapToken:bootstrap,username:'swarmowner',displayName:'Continuation Fixture Owner',password},201);token=account.token;sensitive.push(token);
 await api('POST','/v1/setup/git/identity',{name:'Swarm Fixture',email:'swarm@example.invalid'});
 const readiness=await api('POST','/v1/setup/check',{});for(const id of ['docker','sandbox-image'])assert.equal(readiness.checks.find(row=>row.id===id)?.status,'ready');
 const connections=await api('POST','/v1/setup/agent-connections',{action:'save',expectedRevision:0,name:'Local controlled continuation fixture',kind:'compatible-api',enabled:true,endpoints:{responses:providerBase},models:{codex:'swarm-mechanics-fixture'},apiKey:key});assert.equal(connections.connections.length,1);const connectionID=connections.connections[0].id;
 await setPaused(false);
 const projectOwner=await api('POST','/v1/product/projects',{name:'reviewed-continuation-fixture'},201);assert(isFixturePath(projectOwner.path));seedSource(projectOwner);for(const task of taskContracts)titles.set(task.id,task.title);owner=await saveContext(projectOwner,connectionID);projectStates.push(owner);receipt.acceptedIntent.canonicalHashes=owner.canonicalBefore;receipt.acceptedIntent.initialCommit=owner.initialHead;receipt.fixtureSourceHashes=Object.fromEntries(Object.entries(initialFiles).map(([name,value])=>[name,sha(value)]));
 const browserEnv=Object.fromEntries(['PATH','LANG','LC_ALL','TZ'].filter(n=>process.env[n]).map(n=>[n,process.env[n]]));browserEnv.LUCI_SWARM_BROWSER_FIXTURE=fixture;
 browser=await chromium.launch({headless:true,executablePath:option('--chromium','/usr/bin/chromium'),env:browserEnv,args:['--no-proxy-server']});receipt.browserVersion=browser.version();
 const context=await browser.newContext({viewport:{width:1440,height:1080},serviceWorkers:'block'});
 await context.route('**/*',async route=>{const request=route.request(),url=new URL(request.url());if(url.origin!==base){stopOnFailure(Error('Browser external request refused'));await route.abort('blockedbyclient');return;}if(!['GET','HEAD','OPTIONS'].includes(request.method()))receipt.browserMutations.push({stage,method:request.method(),path:url.pathname});await route.continue();});
 page=await context.newPage();page.setDefaultTimeout(15000);page.on('pageerror',error=>{receipt.browserErrors.push(scrub(error));stopOnFailure(Error('Browser script error'));});

 await prepare(owner);run=owner.run;
 const pristinePrimary={head:gitAt(owner.owner.path,'rev-parse','HEAD'),status:gitAt(owner.owner.path,'status','--porcelain'),canonical:canonical()};assert.equal(pristinePrimary.head,owner.initialHead);assert.equal(pristinePrimary.status,'');
 await assertJoinBlocked('before-first-start');
 stage='browser-start-partial';await openProject(owner);await startManualTask('partial','Start task');
 const partial=await complete('partial');await observeAutomaticCompletion('partial',partial);
 const original=owner.run.operations.find(op=>op.id===partial.operation.id);assert.equal(original.status,'review-required');assert.equal(original.handoff?.status,'captured');partial.operation=original;
 const originalSnapshot=sourceSnapshot(partial.checkout,partialFiles);assert.equal(originalSnapshot.head,observations.get('partial').commit);assert.notEqual(originalSnapshot.head,owner.initialHead);assert.deepEqual(originalSnapshot.status.split('\n').map(v=>v.trim()).sort(),['?? note.txt','M work.mjs']);assert.equal(gitAt(partial.checkout,'show','HEAD:committed.mjs'),partialFiles['committed.mjs'].trim());assert.equal(gitAt(partial.checkout,'show','HEAD:work.mjs'),initialFiles['work.mjs'].trim());
 assertPrimary(pristinePrimary);await assertJoinBlocked('after-partial-exit');
 const oldReview=await inspectCheckpoint(original,'checkpoint-before-answer.png');await oldReview.dialog.getByRole('button',{name:'Keep inspecting',exact:true}).click();
 stage='browser-answer-retained-question';
 const handoffPath=owner.route+'/runs/'+owner.run.id+'/swarm/operations/'+original.id+'/handoff';
 const readHandoff=page.waitForResponse(r=>new URL(r.url()).pathname===handoffPath&&r.request().method()==='GET');await taskRow('task-work').getByRole('button',{name:'Task result and questions',exact:true}).click();const reportedResponse=await readHandoff;assert.equal(reportedResponse.status(),200);const reported=(await reportedResponse.json()).handoff;assert.equal(reported.status,'captured');assert.deepEqual(reported.receipt.reported,taskHandoff('partial'));assert.equal(reported.answers.length,0);assert.equal(reported.questionIds.length,1);
 const resultDialog=page.getByRole('dialog',{name:'Task result and questions',exact:true});await resultDialog.getByRole('heading',{name:question,exact:true}).waitFor();
 assert((await resultDialog.locator('[aria-label="Recorded verification evidence"]').innerText()).includes('No matching verification evidence is recorded here.'));
 await resultDialog.getByRole('button',{name:'Answer question',exact:true}).click();await resultDialog.getByLabel('Your answer',{exact:true}).fill(answer);
 const answered=await action(handoffPath+'/questions/'+reported.questionIds[0]+'/answer',()=>resultDialog.getByRole('button',{name:'Save answer',exact:true}).click());owner.run=answered.run;run=owner.run;
 previousAttempt=validatePreviousAttempt(answered.handoff,{projectId:owner.owner.id,runId:run.id,operation:original,contextDigest:partial.session.reviewedContextDelivery.digest,sourceCommit:originalSnapshot.head});
 await resultDialog.getByText(answer,{exact:true}).waitFor();await page.screenshot({path:join(evidence,'attributed-answer.png'),fullPage:true});await resultDialog.getByRole('button',{name:'Close task result',exact:true}).click();
 const preservedOperation=structuredClone(owner.run.operations.find(op=>op.id===original.id));
 assert.equal(preservedOperation.handoff.unanswered,0);assert.deepEqual(sourceSnapshot(partial.checkout,partialFiles),originalSnapshot);assertPrimary(pristinePrimary);assert.deepEqual(bundleSnapshot(partial.session.id),initialBundles.get(partial.session.id));
 check('Agent report and editor answer remain separately attributed; no integration or new start',{handoff:previousAttempt,originalSnapshot,effects:effects()});
 stage='checkpoint-refusal-probes';const checkpointPath=owner.route+'/runs/'+run.id+'/swarm/operations/'+original.id+'/checkpoint';
 const refusalBefore=privateRun(owner),refusalEffects=effects(),refusalTree=ownedWorktreeNames();
 const invalidBody={expectedRevision:run.revision,idempotencyKey:'continuation-unauthenticated',reviewDigest:oldReview.review.reviewDigest,childId:original.childId};
 const unauthorized=await fetch(base+checkpointPath,{method:'POST',redirect:'error',headers:{Origin:base,'Content-Type':'application/json'},body:JSON.stringify(invalidBody),signal:AbortSignal.timeout(remainingBudget(30000))});assert.equal(unauthorized.status,401);await boundedResponse(unauthorized,1024*1024);
 await api('POST',checkpointPath,{...invalidBody,idempotencyKey:'continuation-stale-review'},409);
 assert.deepEqual(privateRun(owner),refusalBefore);assert.deepEqual(effects(),refusalEffects);assert.deepEqual(ownedWorktreeNames(),refusalTree);assert.deepEqual(sourceSnapshot(partial.checkout,partialFiles),originalSnapshot);assertPrimary(pristinePrimary);
 check('Unauthenticated and stale-review requests refuse before new allocation',{unauthenticatedHTTP:401,staleReviewHTTP:409,freshRevisionUsed:run.revision,oldReviewRevision:oldReview.review.expectedRevision,unchangedPrivateRun:true,unchangedWorktrees:true,effects:effects(),permissionScope:'Unauthenticated refusal only; managed grant revocation is covered separately by product/API tests.'});
 stage='browser-complete-checkpoint-lost-reply';const reviewed=await inspectCheckpoint(preservedOperation,'checkpoint-complete-review.png');assert.notEqual(reviewed.review.reviewDigest,oldReview.review.reviewDigest);
 let submitted;let lostRequests=0;let signalAcknowledged,signalFailed;const acknowledged=new Promise((resolve,reject)=>{signalAcknowledged=resolve;signalFailed=reject;});
 const checkpointURL=base+checkpointPath;
 const loseOneReply=async route=>{try{if(route.request().method()!=='POST'){await route.fallback();return;}assert.equal(++lostRequests,1,'No automatic checkpoint POST replay');const body=route.request().postDataJSON();assert.equal(body.expectedRevision,reviewed.review.expectedRevision);assert.equal(body.reviewDigest,reviewed.review.reviewDigest);assert.equal(body.childId,original.childId);assert.match(body.idempotencyKey,/^[A-Za-z0-9-]{1,100}$/);receipt.browserMutations.push({stage,method:'POST',path:checkpointPath,replyFault:'after-real-upstream-201'});const upstream=await route.fetch({maxRedirects:0,timeout:remainingBudget(120000)});assert.equal(upstream.status(),201);const bytes=await upstream.body();assert(bytes.length<=2*1024**2);checkpointReservation=JSON.parse(bytes.toString('utf8'));submitted=body;await upstream.dispose();await route.abort('failed');signalAcknowledged({status:201,bodySHA256:sha(bytes)});}catch(error){signalFailed(error);stopOnFailure(error);try{await route.abort('failed');}catch{}}};
 await page.route(checkpointURL,loseOneReply);await reviewed.dialog.getByRole('checkbox').check();await reviewed.dialog.getByRole('button',{name:'Continue from these reviewed changes',exact:true}).click();const replyFault=await timed(acknowledged,remainingBudget(150000),'Real upstream checkpoint reservation');
 await taskRow('task-work').getByRole('button',{name:'Continue reserved task',exact:true}).waitFor();await page.unroute(checkpointURL,loseOneReply);assert.equal(lostRequests,1);
 const next=validateSeedOperation(checkpointReservation,{runId:run.id,source:preservedOperation,review:reviewed.review,targetCommit:owner.initialHead});owner.run=checkpointReservation.run;run=owner.run;
 assert.equal(effects().create,1);assert.equal(effects().start,1);await api('GET','/v1/sessions/'+next.childId,undefined,404);
 const trees=(await api('GET','/v1/projects/'+owner.owner.id+'/worktrees')).worktrees,seed=trees.find(w=>w.id===next.worktreeId);assert(seed);assert(isFixturePath(seed.path));const seedSnapshot=sourceSnapshot(seed.path,partialFiles);assert.equal(seedSnapshot.head,next.checkpoint.seedCommit);assert.equal(seedSnapshot.status,'');assert.deepEqual(gitAt(seed.path,'ls-files','--',...Object.keys(partialFiles)).split('\n').sort(),Object.keys(partialFiles).sort());
 const noReplayEffects=effects();const replay=await api('POST',checkpointPath,submitted,200);assert.equal(replay.operation.id,next.id);assert.equal(replay.operation.childId,next.childId);assert.equal(replay.operation.worktreeId,next.worktreeId);assert.deepEqual(replay.operation.checkpoint,next.checkpoint);assert.deepEqual(sourceSnapshot(seed.path,partialFiles),seedSnapshot);assert.deepEqual(effects(),noReplayEffects);owner.run=replay.run;run=owner.run;
 assert.deepEqual(sourceSnapshot(partial.checkout,partialFiles),originalSnapshot);assertPrimary(pristinePrimary);await assertJoinBlocked('after-checkpoint-reservation');
 receipt.checkpoint={review:reviewed.review,originalOperation:preservedOperation,nextOperation:next,seedSnapshot,originalSnapshot,previousAttempt,replyFault,explicitIdempotentReadbackHTTP:200};
 check('Complete reviewed seed is distinct, unstarted, and idempotent after a genuinely lost reply',{oldOperation:original.id,newOperation:next.id,oldChild:original.childId,newChild:next.childId,seedCommit:next.checkpoint.seedCommit,upstreamReply:replyFault.status,automaticPOSTReplays:lostRequests-1});
 stage='controller-restart-with-unstarted-seed';const runBeforeRestart=privateRun(owner);await page.close();page=undefined;for(const socket of sockets)socket.close();await stopService(true);await startService();
 assert.deepEqual(privateRun(owner),runBeforeRestart);assert.deepEqual(sourceSnapshot(seed.path,partialFiles),seedSnapshot);assert.deepEqual(sourceSnapshot(partial.checkout,partialFiles),originalSnapshot);assertPrimary(pristinePrimary);await api('GET','/v1/sessions/'+next.childId,undefined,404);assert.equal(effects().create,1);assert.equal(effects().start,1);
 page=await context.newPage();page.setDefaultTimeout(15000);page.on('pageerror',error=>{receipt.browserErrors.push(scrub(error));stopOnFailure(Error('Browser script error'));});await refreshRun();await openProject(owner);await taskRow('task-work').getByRole('button',{name:'Continue reserved task',exact:true}).waitFor();await page.screenshot({path:join(evidence,'reserved-seed-after-restart.png'),fullPage:true});await assertJoinBlocked('after-controller-restart');
 check('Controller restart preserves the reserved seed without allocation or replay',{seedCommit:seedSnapshot.head,privateRunUnchanged:true,unstartedChild:next.childId});
 stage='browser-start-successor';await startManualTask('finish','Continue reserved task');const finish=await complete('finish');assert.equal(finish.operation.id,next.id);await observeAutomaticCompletion('finish',finish);await assertJoinBlocked('after-successor-exit-before-integration');assert.deepEqual(sourceSnapshot(partial.checkout,partialFiles),originalSnapshot);assertPrimary(pristinePrimary);
 check('Successor reads the exact reviewed seed and separately attributed prior report and answer',{operationID:finish.operation.id,contextHashes:hashes(initialBundles.get(finish.session.id)),previousAttemptDigest:previousAttempt.digest,reportedChecksRemainClaims:true});
 stage='browser-review-final-integration';const accepted=await integrate('finish');assert.equal(accepted.sourceCommit,observations.get('finish').commit);assert.equal(gitAt(owner.owner.path,'rev-parse','HEAD'),accepted.commit);assert.equal(gitAt(owner.owner.path,'status','--porcelain'),'');for(const [name,text] of Object.entries(finalFiles))assert.equal(readFileSync(join(owner.owner.path,name),'utf8'),text);assert.deepEqual(canonical(),owner.canonicalBefore);
 const checkHead=gitAt(owner.owner.path,'rev-parse','HEAD'),checkStart=Date.now();const finalOutput=execFileSync(process.execPath,['verify.mjs','final'],{cwd:owner.owner.path,env:{PATH:process.env.PATH},encoding:'utf8',timeout:15000,maxBuffer:1024*1024});assert.equal(finalOutput,'CONTINUATION_CHECK_OK final\n');assert.equal(gitAt(owner.owner.path,'rev-parse','HEAD'),checkHead);assert.equal(gitAt(owner.owner.path,'status','--porcelain'),'');
 await refreshRun();assert.equal(run.operations.length,2);assert(!run.operations.some(op=>op.taskId==='task-join'));assert.equal(run.swarm.tasks.find(t=>t.id==='task-join').operationId??'','');assert.equal(run.swarm.tasks.find(t=>t.id==='task-work').integration.state,'integrated');assert.notEqual(run.state,'complete');assert.deepEqual(sourceSnapshot(partial.checkout,partialFiles),originalSnapshot);for(const w of taskData.values())assert.deepEqual(bundleSnapshot(w.session.id),initialBundles.get(w.session.id));assert.equal(effects().create,2);assert.equal(effects().start,2);assert.equal(observations.size,2);assert.equal(receipt.naturalCompletions.length,2);assert.equal(receipt.automaticCompletions.length,2);assert.equal(workerContainers().length,0);assert.equal(receipt.browserErrors.length,0);
 await openProject(owner);await page.screenshot({path:join(evidence,'continued-task-integrated.png'),fullPage:true});
 receipt.final={primaryCommit:checkHead,sourceHashes:Object.fromEntries(Object.keys(finalFiles).map(name=>[name,sha(readFileSync(join(owner.owner.path,name)))])),independentCheck:{command:'node verify.mjs final',exitCode:0,stdout:finalOutput,elapsedMs:Date.now()-checkStart,sourceCommit:checkHead,scope:'Acceptance-harness check, not a product-joined verification receipt'},runState:run.state,dependentTask:'Not authorized or started; now eligible only after the accepted integration.',retainedOriginalUnchanged:true};
 check('Reviewed integration and independent exact output checks pass without starting the dependent task',receipt.final);receipt.passed=true;

}catch(error){receipt.failedStage=stage;receipt.failure=redactError(error);process.exitCode=1;console.error(receipt.failure);if(page){try{await page.screenshot({path:join(evidence,'failed-stage.png'),fullPage:true});}catch{}}}
finally {
 closing=true;clearTimeout(deadlineTimer);clearInterval(reserveTimer);for(const socket of sockets)socket.close();receipt.actualTaskResults=Object.fromEntries(observations);
 receipt.taskChildren=[...taskData.values()].map(value=>({task:value.task,projectID:value.owner.id,operationID:value.operation.id,sessionID:value.session.id,container:value.container,bundle:initialBundles.get(value.session.id)}));receipt.terminalObservations=terminalProbes.map(({id,attachedStage,probe})=>({sessionID:id,attachedStage,...probe.snapshot()}));
 const cleanupFailure=error=>{receipt.cleanup.push(redactError(error));receipt.passed=false;process.exitCode=1;};
 if(browser){try{await browser.close();}catch(error){cleanupFailure(error);}const end=Date.now()+5000;while(Date.now()<end&&ownedProcesses('browser').length)await delay(100);for(const identity of ownedProcesses('browser')){cleanupFailure(Error('Owned browser required forced cleanup'));if(stillOwned(identity)){try{process.kill(identity.pid,'SIGKILL');}catch(error){if(error.code!=='ESRCH')cleanupFailure(error);}}}}
 let stopped=false;try{stopped=await stopService(false);assert(stopped);assert(controllerIdentities.filter(Boolean).every(identity=>!stillOwned(identity)));}catch(error){cleanupFailure(error);}receipt.controllersStopped=stopped&&controllerIdentities.filter(Boolean).every(identity=>!stillOwned(identity));
 if(proxy){try{const cleanup=await proxy.close();assert(cleanup.drained&&cleanup.pendingRequests===0&&cleanup.activeHandlers===0);receipt.dockerRequestsDrained=true;}catch(error){cleanupFailure(error);}receipt.dockerTransportObservations=structuredClone(proxy.observations);if(proxy.observations.dropped||proxy.observations.failures.length)cleanupFailure(Error('Docker transport recorded lost, refused or uncertain observations'));}else{receipt.dockerRequestsDrained=true;receipt.dockerTransportNotStarted=true;}
 if(receipt.controllersStopped&&receipt.dockerRequestsDrained&&proxy){const created=proxy.observations.requests.filter(value=>value.method==='POST'&&/\/containers\/create$/.test(value.path)&&value.status===201&&/^[a-f0-9]{64}$/.test(value.id??''));for(const creation of created){try{const ids=docker('ps','-aq','--no-trunc').split(/\s+/).filter(Boolean);if(!ids.includes(creation.id))continue;const value=JSON.parse(docker('inspect',creation.id))[0];assert.equal(value.Id,creation.id);assert.equal(value.Name,'/'+creation.name);assert.equal(value.Config.Labels['luci.instance'],instanceID());assert.equal(value.Config.Labels['luci.session-id'],creation.labels['luci.session-id']);assert(ownedSessions.has(value.Config.Labels['luci.session-id']));assert.equal(value.Config.Labels['luci.managed'],'true');assert.equal(value.Image,imageInfo.Id);assert(value.Mounts.some(mount=>mount.Destination==='/home/dev/work'&&projectStates.some(owner=>realpathSync(mount.Source).startsWith(join(dirname(owner.owner.path),'.luci-swarms')+sep))));cleanupFailure(Error('Unexpected full worker remained; exact failed-fixture cleanup required'));if(value.State.Running)docker('stop','--time','5',value.Id);const current=JSON.parse(docker('inspect',value.Id))[0];assert.equal(current.Id,value.Id);assert.equal(current.State.Running,false);docker('rm',value.Id);}catch(error){cleanupFailure(error);}}}
 if(provider){provider.closeAllConnections();if(provider.listening)await new Promise(done=>provider.close(done));}
 try{assert(receipt.controllersStopped);assert.equal(receipt.dockerRequestsDrained,true);assert.equal(docker('ps','-aq'),'');assert.equal(docker('volume','ls','-q'),'');assert.equal(ownedProcesses().length,0);assert.equal(ownedProcesses('browser').length,0);assert.equal(JSON.parse(docker('image','inspect',image))[0].Id,imageInfo.Id);assert.equal(sha(readFileSync(binary)),provenance.binarySHA256);assert.equal(sha(readFileSync(join(web,'index.html'))),provenance.webIndexSHA256);assert.equal(docker('info','--format','{{.ID}}'),engineID);for(const name of driverNames)assert.equal(sha(readFileSync(new URL(name,import.meta.url))),sha(readFileSync(join(evidence,'accepted-harness',name))));for(const name of reusedHelpers)assert.equal(sha(readFileSync(join(repo,'scripts/acceptance',name))),sha(readFileSync(join(evidence,'accepted-harness',name.replaceAll('/','_')))));
  receipt.runtimeCleanup={containers:0,volumes:0,ownedServiceProcesses:0,ownedBrowserProcesses:0,protectedImagePackageAndHelpersUnchanged:true,rootFreeBytes:rootFree()};assert.equal(realpathSync(fixture),fixture);assert.equal(dirname(fixture),fixtureParent);assert(fixture.split(sep).at(-1).startsWith('luci-continuation-'));receipt.fixturePreservedForInspection=true;
 }catch(error){cleanupFailure(error);}
 if(!receipt.fixtureRemoved)receipt.fixtureRetained={path:fixture,reason:'The exact original dirty task checkout, reserved continuation seed, captured reports/answer and frozen contexts are retained for inspection even after successful acceptance. This private fixture contains test credentials; do not publish or reuse it as a normal workspace.'};
 if(fixtureError){receipt.passed=false;process.exitCode=1;receipt.backgroundFailure=redactError(fixtureError);}receipt.providerCalls=providerCalls;receipt.auxiliaryCalls=auxiliaryCalls;receipt.actualTaskCount=observations.size;receipt.elapsedMs=Date.now()-started;receipt.signal=signalFailure??null;receipt.finishedAt=new Date().toISOString();writeFileSync(report,JSON.stringify(receipt,null,2)+'\n',{mode:0o600,flag:'wx'});console.log((receipt.passed?'PASS ':'FAIL ')+report);
}
