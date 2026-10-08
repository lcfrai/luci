#!/usr/bin/env node
// Independent installed CLOUD-009 recovery witness. Real Docker/process death; controlled provider.
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
import { validateWorkerCreate, workerBinds } from './worker-create-guard.mjs';
import { startFencedDockerProxy } from './fenced-docker-proxy.mjs';
import { chromium } from '../../../desktop/node_modules/playwright/index.mjs';
import { createNativeTerminalProbe } from '../four-project-queue/native-terminal-diagnostics.mjs';

const repo = resolve(dirname(fileURLToPath(import.meta.url)), '../../..'), args = process.argv.slice(2);
const option = (name, fallback) => { const i = args.indexOf(name); if (i < 0) return fallback; assert(args[i + 1] && !args[i + 1].startsWith('--'), 'Missing value for ' + name); return args[i + 1]; };
if (args.includes('--help')) {
 console.log('PREPARED: --scenario delayed-create|delayed-start --binary PATH --web PATH --provenance PATH --expected-source SHA --image TAG --expected-image-id sha256:ID --image-provenance RECEIPT --report FRESH_JSON --fixture-root SHORT_ABSOLUTE_PARENT [--chromium PATH]. Actual owned root SIGKILL/restart, explicit browser recovery, delayed Docker requests and one successor result. No live-model quality claim. Requires an exclusive authorized Docker window.');
 process.exit(0);
}
const scenario=option('--scenario');assert(['delayed-create','delayed-start'].includes(scenario),'Choose one explicitly bounded scenario');
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
const report = resolve(option('--report', 'artifacts/quality-swarm/launch-recovery/attempt-' + Date.now() + '/result.json')), evidence = dirname(report);
if (existsSync(evidence)) assert.equal(readdirSync(evidence).length, 0, 'Use a fresh evidence directory; prior failures must remain');
mkdirSync(evidence, { recursive: true, mode: 0o700 });
writeFileSync(join(evidence, 'package-provenance.json'), provenanceBytes, { mode: 0o600, flag: 'wx' });
writeFileSync(join(evidence,'image-provenance.json'),readFileSync(resolve(imageEvidencePath)),{mode:0o600,flag:'wx'});
const driverNames = ['recovery.mjs', 'fenced-docker-proxy.mjs', 'worker-create-guard.mjs', 'run-with-observer.py'];
const reusedHelpers = ['native-terminal-diagnostics.mjs','sample-linux-owned.py'];
mkdirSync(join(evidence, 'accepted-harness'), { mode: 0o700 });
for (const name of driverNames) writeFileSync(join(evidence, 'accepted-harness', name), readFileSync(new URL(name, import.meta.url)), { mode: 0o600, flag: 'wx' });
for (const name of reusedHelpers) writeFileSync(join(evidence,'accepted-harness',name),readFileSync(new URL('../four-project-queue/'+name,import.meta.url)),{mode:0o600,flag:'wx'});
const fixtureParent = resolve(option('--fixture-root', 'artifacts/quality-swarm/private-fixtures')); mkdirSync(fixtureParent, {recursive:true,mode:0o700});
const fixture = mkdtempSync(join(fixtureParent,'luci-launch-recovery-'));
for (const folder of ['home', 'projects', 'state', 'docker']) mkdirSync(join(fixture, folder), { mode: 0o700 });
const key = 'fixture-only-' + randomUUID(), lifecycle = randomUUID() + randomUUID(), password = randomUUID() + randomUUID();
const sensitive = [key, lifecycle, password], scrub = value => sensitive.reduce((text, secret) => text.replaceAll(secret, '[redacted]'), String(value));
const receipt = {
  passed: false, mode:'launch-recovery-'+scenario, startedAt: new Date().toISOString(), sourceCommit: provenance.sourceCommit, sourceDirty: provenance.sourceDirty,
  harnessCheckout: { commit: execFileSync('git', ['rev-parse', 'HEAD'], { cwd: repo, encoding: 'utf8' }).trim(), dirty: !!execFileSync('git', ['status', '--porcelain'], { cwd: repo, encoding: 'utf8' }).trim() },
  packageIntegrity, initialRootFreeBytes: initialRootFree,
  resourceScope: 'Bounded uncertain launch, scratch recovery fences and one successor task; exact per-shape lifecycle caps. Zero existing containers and six GiB root admission before every Create, including scratch fences; hard one GiB running reserve. No image pulls, release or cloud allocation.',
  binarySHA256: sha(readFileSync(binary)), webIndexSHA256: sha(readFileSync(join(web, 'index.html'))), driverSHA256: sha(readFileSync(fileURLToPath(import.meta.url))),
  helperSHA256: Object.fromEntries(driverNames.filter(name=>name!=='recovery.mjs').map(name => [name, sha(readFileSync(new URL(name, import.meta.url)))])),
  reusedHelperSHA256:Object.fromEntries(reusedHelpers.map(name=>['scripts/acceptance/four-project-queue/'+name,sha(readFileSync(new URL('../four-project-queue/'+name,import.meta.url)))])),
  image: { reference: image, id: imageInfo.Id }, releaseImage: { reference: releaseImage, id: releaseImageInfo.Id },
  scope: 'Installed explicit recovery after owned controller SIGKILL, real late Docker request denial, durable restart and one independent successor task. No model or performance claim.',
  limitations: ['Existing trusted Docker images; installation is not exercised.', 'Provider output is deliberately scripted; this proves actual CLI, Docker, task coordination and review mechanics. It does not measure model reasoning, production application quality, cloud deployment or Windows behavior.'],
  externalModelCalls: 0, cloudResourcesCreated: 0, setupScaffolding: ['API-seeded personal owner, two approved contexts and unstarted reviewed runs','API-configured one-worker policy; explicit browser task/recovery consent','Local scripted provider, no external model calls'], checks: [], browserMutations: [], browserErrors: [], cleanup: [],
};

const check = (name, details = {}) => { receipt.checks.push({ name, passed: true, ...details }); console.log('PASS ' + name); };
const names = ['.luci/brief.json', '.luci/design-handoff.json', '.luci/design-handoff.md', '.luci/design.json', '.luci/plan.json', '.luci/specification.json', '.luci/specification.md', 'LUCI.md'];
// The private brief is generated from saved state; only seven members are checkout exports.
const canonicalNames = names.filter(name => name !== '.luci/brief.json');
const authorityRoot = '/home/dev/.luci-reviewed-context';
const sockets = new Set(), socketSessions = new Map(), ownedSessions = new Set(), bundles = new Map(), nativeHomes = new Map();
const terminalProbes = [];
let service, proxy, provider, base, env, token, project, projectRoute;const workspace='';
let original, originalCreate, engineID; const controllerIdentities=[];
let stage = 'startup', fixtureError, reserveTimer, deadlineTimer, closing = false, signalFailure;
let publishFixtureFailure;const fixtureFailureSignal=new Promise(resolveFailure=>{publishFixtureFailure=resolveFailure;});
let providerCalls = 0, auxiliaryCalls = 0; const observations=new Map(), taskData=new Map(), calls=new Map(); let browser, page, run;
let providerBase; const initialBundles=new Map(), projectStates=[], titles=new Map();
const started = Date.now(), deadline = started + 15 * 60_000;
const remaining = () => { if (Date.now() >= deadline) throw Error('Fifteen minute acceptance deadline expired'); return deadline - Date.now(); };
const remainingBudget = (ms = 120_000) => Math.max(1, Math.min(ms, remaining()));
const redactError = error => scrub(error?.stack ?? error).replaceAll(fixture, '[private-fixture]').slice(0, 8000);
function stopOnFailure(error) { if (!fixtureError) {fixtureError=error;publishFixtureFailure(error);} for (const socket of sockets) socket.close(); if (service?.exitCode === null) service.kill('SIGTERM'); }
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
  const work=value.Mounts.filter(m=>m.Destination==='/home/dev/work');assert.equal(work.length,1);assert.equal(work[0].RW,true);const expected=taskData.get(row(sessionID).implementationContext.taskIds[0]);assert(expected);assert.equal(realpathSync(work[0].Source),realpathSync(expected.checkout));assert.notEqual(realpathSync(work[0].Source),realpathSync(expected.owner.path));assert(lstatSync(join(work[0].Source,'.git')).isDirectory());assert(!existsSync(join(work[0].Source,'.git/commondir')));assert(!existsSync(join(work[0].Source,'.git/objects/info/alternates')));assert(!value.Mounts.some(m=>m.Source.includes('docker.sock')));
  assert(expected.requestedLimits.memoryMiB>0&&expected.requestedLimits.cpuMillis>0);assert.equal(value.HostConfig.Memory,expected.requestedLimits.memoryMiB*1024**2);assert.equal(value.HostConfig.NanoCpus,expected.requestedLimits.cpuMillis*1_000_000);assert.equal(value.HostConfig.PidsLimit,256);assert.equal(value.HostConfig.Privileged,false);assert(value.HostConfig.SecurityOpt.includes('no-new-privileges:true'));
  const proof={resourceLimits:{memoryBytes:value.HostConfig.Memory,nanoCPUs:value.HostConfig.NanoCpus,pidsLimit:value.HostConfig.PidsLimit,privileged:value.HostConfig.Privileged,securityOpt:value.HostConfig.SecurityOpt},worktreeID:row(sessionID).worktreeId,id:value.Id,image:value.Image,pid:value.State.Pid,running:value.State.Running,startedAt:value.State.StartedAt,created:value.Created,readOnly:true,sessionID};
  if(!requireBundle)return proof; // Cleanup authority uses labels/image/worktree, not the feature assertion under test.
  const native=value.Mounts.filter(m=>m.Destination==='/home/dev/.luci-agent');assert.equal(native.length,1);assert.equal(native[0].RW,true);const nativeRoot=realpathSync(native[0].Source);assert.equal(nativeRoot,resolve(native[0].Source));assert(nativeRoot.startsWith(join(fixture,'state')+sep));if(nativeHomes.has(sessionID))assert.equal(nativeHomes.get(sessionID),nativeRoot);nativeHomes.set(sessionID,nativeRoot);
  const mounts=value.Mounts.filter(m=>m.Destination===authorityRoot);assert.equal(mounts.length,1);const mount=mounts[0];assert.equal(mount.Type,'bind');assert.equal(mount.RW,false);
  const root=realpathSync(mount.Source);assert.equal(root,resolve(mount.Source));assert(root.startsWith(join(fixture,'state')+sep));assert(root.endsWith('/.reviewed-context/'+sessionID));assert(!root.startsWith(realpathSync(expected.owner.path)+sep));
  bundles.set(sessionID,root);
  return proof;
}
function live(id){const ids=containerIDs(id);assert.equal(ids.length,1);const proof=inspectOwned(ids[0],id);assert(proof.running&&proof.pid>0);return proof;}
function bundleSnapshot(id){
  const root=bundles.get(id);assert(root);assert.equal(realpathSync(root),root);const files={};
  for(const name of [...names,'manifest.json']){const path=join(root,name);const info=lstatSync(path);assert(info.isFile()&&!info.isSymbolicLink()&&info.nlink===1);assert(info.size<=32*1024**2);files[name]={sha256:sha(readFileSync(path)),bytes:info.size,inode:String(info.ino),device:String(info.dev)};}
  assert.deepEqual(readdirSync(root).sort(),['.luci','LUCI.md','manifest.json'].sort());assert.deepEqual(readdirSync(join(root,'.luci')).sort(),names.filter(n=>n.startsWith('.luci/')).map(n=>n.slice(6)).sort());
  return files;
}
function hashes(snapshot){return Object.fromEntries(Object.entries(snapshot).map(([name,value])=>[name,value.sha256]));}
function canonical(){return Object.fromEntries(canonicalNames.map(name=>[name,sha(readFileSync(join(project.path,name)))]));}
function effects(){assert.equal(proxy.observations.dropped,0,'Lost transport observations invalidate zero-side-effect claims');return {create:proxy.observations.counts.create,start:proxy.observations.counts.start,destroy:proxy.observations.requests.filter(r=>r.method==='DELETE'&&/^\/v\d+\.\d+\/containers\/[^/]+$/.test(r.path)).length};}
function holdFinalResponse(state,res){
 state.finalResponseRequests=(state.finalResponseRequests??0)+1;
 assert(state.finalResponseRequests<=4,'At most three identical final-answer transport retries');
 state.finalWaiters??=new Set();
 if(state.finalReleased)return Promise.resolve(true);
 return new Promise(resolveWait=>{
  let settled=false;
  const complete=send=>{if(settled)return;settled=true;clearTimeout(timer);res.off('close',closed);state.finalWaiters.delete(complete);if(!send)state.finalResponseAborts=(state.finalResponseAborts??0)+1;resolveWait(send);};
  const closed=()=>complete(false);
  const timer=setTimeout(()=>{stopOnFailure(Error('Controlled final answer hold timed out'));complete(false);},remainingBudget(150_000));
  state.finalWaiters.add(complete);res.once('close',closed);
 });
}
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
 const current=(await api('GET',owner.route+'/runs/'+owner.run.id)).run;
 const task=current.swarm.tasks.find(value=>value.id===owner.task);assert.notEqual(task.integration?.state,'integrated');
 assert(!receipt.apiActions.some(value=>value.method==='DELETE'&&value.path===workspace+'/v1/sessions/'+worker.session.id),'Success workflow must not issue a worker stop');
 (receipt.naturalCompletions??=[]).push({task:worker.task,sessionID:worker.session.id,sessionStatus:stopped.status,recordedExitReason:stopped.exitReason,cleanNativeExitFrame:true,exitFrameObservedAt:probe.snapshot().exitObservedAt,stoppedAndAbsentObservedAt,queueCompletedObservedAt,leaseObservation:worker.leaseObservation,queueState:completed.state,codingDispatches:worker.codingDispatches,finalResponseRequests:worker.finalResponseRequests,finalResponseAborts:worker.finalResponseAborts??0,finalResponseRetries:(worker.finalResponseRequests??1)-1,integratedBeforeExit:false});
 return completed;
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
function commandFor(task) {
 const value=taskData.get(task);assert(value);const op=value.operation.id;
 const script=String.raw`const fs=require('node:fs'),path=require('node:path'),crypto=require('node:crypto'),cp=require('node:child_process'),assert=require('node:assert/strict');
 (async()=>{const root=${JSON.stringify(authorityRoot)},task=${JSON.stringify(task)},operation=${JSON.stringify(op)},names=${JSON.stringify(names)};
 const files=Object.fromEntries(names.concat('manifest.json').map(name=>{const b=fs.readFileSync(path.join(root,name));return[name,{sha256:crypto.createHash('sha256').update(b).digest('hex'),bytes:b.length}]}));
 const text=fs.readFileSync(path.join(root,'LUCI.md'),'utf8'),section=text.slice(text.indexOf('## Assigned task contract'));
 const contract=JSON.parse(section.match(/\x60\x60\x60json\n([\s\S]*?)\n\x60\x60\x60/)[1]);assert.equal(contract.selectedTasks.length,1);assert.equal(contract.selectedTasks[0].id,task);assert.deepEqual(contract.selectedTasks[0].writePaths,[task+'-result.txt']);assert.equal(JSON.parse(fs.readFileSync(path.join(root,'.luci/plan.json'))).projectId,${JSON.stringify(value.owner.id)});
 let readOnly=false;try{fs.writeFileSync(path.join(root,'.swarm-fixture-read-only-probe'),'must not be writable',{flag:'wx'});}catch(e){assert.equal(e.code,'EROFS');readOnly=true;}assert(readOnly);
 const git=(...a)=>cp.execFileSync('git',a,{encoding:'utf8'}).trim();assert.equal(git('status','--porcelain'),'');const base=git('rev-parse','HEAD');

 const content='Controlled '+task+' result\n';fs.writeFileSync(task+'-result.txt',content,{flag:'wx'});assert.equal(fs.readFileSync(task+'-result.txt','utf8'),content);
 git('add','--',task+'-result.txt');git('-c','user.name=Luci swarm fixture','-c','user.email=swarm@example.invalid','commit','-m','Controlled task '+task);assert.equal(git('status','--porcelain'),'');
 const result={task,operation,uid:process.getuid(),cwd:process.cwd(),files,readOnly,base,commit:git('rev-parse','HEAD'),changed:git('diff','--name-only',base,'HEAD').split('\n'),outputSHA256:crypto.createHash('sha256').update(fs.readFileSync(task+'-result.txt')).digest('hex')};console.log('LUCI_SWARM_OBSERVATION='+Buffer.from(JSON.stringify(result)).toString('base64'));})().catch(e=>{console.error(e);process.exitCode=1;});`;
 return {cmd:'node -e "eval(Buffer.from(\''+Buffer.from(script).toString('base64')+'\',\'base64\').toString())"',workdir:'/home/dev/work',login:false,yield_time_ms:1000,max_output_tokens:4000};
}
function nextTool(state,name,args){if(name==='exec_command'){state.codingDispatches=(state.codingDispatches??0)+1;assert.equal(state.codingDispatches,1,'No duplicate task command');}assert(++state.count<=30,'Bounded task tool request cap');const callID='call_swarm_'+state.task+'_'+state.operation.id+'_'+state.count;calls.set(callID,{state,consumed:false});return {id:'fc_'+callID,type:'function_call',status:'completed',name,call_id:callID,arguments:JSON.stringify(args)};}
function startProvider(){
 provider=httpServer(async(req,res)=>{try{
  assert.equal(req.method,'POST');assert.equal(req.headers.authorization,'Bearer '+key);if(closing){res.writeHead(503);res.end();return;}
  assert.equal(req.url,'/v1/responses');const parts=[];let bytes=0;for await(const part of req){bytes+=part.length;assert(bytes<4*1024**2);parts.push(part);}const body=JSON.parse(Buffer.concat(parts));assert.equal(body.model,'swarm-mechanics-fixture');assert(++providerCalls<=16);
  const resultItems=outputs(body.input),pending=resultItems.filter(v=>calls.has(v.call_id)&&!calls.get(v.call_id).consumed);assert(pending.length<=1,'Ambiguous unconsumed task output');let response;
  if(pending.length){const output=pending[0],entry=calls.get(output.call_id),state=entry.state;entry.consumed=true;const raw=typeof output.output==='string'?output.output:JSON.stringify(output.output);assert(!/Process exited with code (?:[1-9]\d*|-\d+)|"exit_code"\s*:\s*(?:[1-9]\d*|-\d+)|Command failed/.test(raw));
   const match=[...raw.matchAll(/LUCI_SWARM_OBSERVATION=([A-Za-z0-9+/=]+)/g)];
   if(match.length){assert.equal(match.length,1);assert(!observations.has(state.task));const actual=JSON.parse(Buffer.from(match[0][1],'base64'));assert.equal(actual.task,state.task);assert.equal(actual.operation,state.operation.id);assert.equal(actual.uid,process.getuid());assert.equal(actual.cwd,'/home/dev/work');assert.equal(actual.readOnly,true);assert.deepEqual(hashes(actual.files),hashes(initialBundles.get(state.session.id)));assert.deepEqual(actual.changed,[state.task+'-result.txt']);assert.match(actual.commit,/^[a-f0-9]{40,64}$/);observations.set(state.task,actual);entry.finalTask=state.task;entry.finalOutputSHA256=sha(raw);if(!await holdFinalResponse(state,res))return;response=message('Controlled task '+state.task+' committed. Human integration and complete build verification remain separate.');}
   else {const running=raw.match(/Process running with session ID (\d+)/)||raw.match(/"session_id"\s*:\s*(\d+)/);assert(running,'Actual tool output must supply completion receipt or live command identity');response=nextTool(state,'write_stdin',{session_id:Number(running[1]),chars:'',yield_time_ms:1000,max_output_tokens:4000});}
  }else if(resultItems.some(value=>calls.get(value.call_id)?.finalTask)){
   const finals=resultItems.filter(value=>calls.get(value.call_id)?.finalTask);assert.equal(finals.length,1);
   const output=finals[0],entry=calls.get(output.call_id),state=entry.state,raw=typeof output.output==='string'?output.output:JSON.stringify(output.output);
   assert.equal(sha(raw),entry.finalOutputSHA256,'Only an identical verified final-output request may retry');assert(observations.has(state.task));assert.equal(state.codingDispatches,1);
   if(!await holdFinalResponse(state,res))return;response=message('Controlled task '+state.task+' committed. Human integration and complete build verification remain separate.');
  }else if((body.tools??[]).some(t=>(t.name??t.function?.name)==='exec_command')){
   const text=JSON.stringify(body.input);assert(text.includes('Reviewed context directory: '+authorityRoot));const matched=[...titles].filter(([task,title])=>text.includes(task+': '+title)).map(([task])=>task);assert.equal(matched.length,1,'Native opening prompt must identify exactly one assigned fixture task');const task=matched[0];assert.equal(task,'build-b','Retired original must never reach the provider');await until('Controller verifies actual task bundle',()=>taskData.get(task)?.session&&initialBundles.has(taskData.get(task).session.id));const state=taskData.get(task);assert.equal(state.count,0,'No duplicate coding dispatch');response=nextTool(state,'exec_command',commandFor(task));
  }else{assert(++auxiliaryCalls<=4);response=message('Controlled swarm fixture acknowledgement.');}
  (receipt.providerRequests??=[]).push({number:providerCalls,toolOutputCount:resultItems.length,issuedTool:response.type==='function_call'?response.name:null});streamResponse(res,response,providerCalls);
 }catch(error){stopOnFailure(error);if(!res.headersSent)res.writeHead(500);res.end('Controlled swarm fixture contract failed');}});
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
async function queuedChild(owner,ordinal){
 const queued=await until('Exactly one queued worker allocated',async()=>{
  const queue=await api('GET',owner.route+'/swarm-queue');return queue.items.find(item=>item.taskId===owner.task&&item.childId&&item.state==='running');
 },150000);
 const session=await api('GET',workspace+'/v1/sessions/'+queued.childId);ownedSessions.add(session.id);
 owner.run=(await api('GET',owner.route+'/runs/'+owner.run.id)).run;
 const operation=owner.run.operations.find(op=>op.taskId===owner.task&&op.childId===session.id);assert(operation);
 const worktrees=(await api('GET',workspace+'/v1/projects/'+owner.owner.id+'/worktrees')).worktrees,checkout=worktrees.find(w=>w.id===operation.worktreeId);assert(checkout);
 const state={task:owner.task,owner:owner.owner,operation,session,checkout:checkout.path,count:0,requestedLimits:{memoryMiB:queued.memoryMiB,cpuMillis:queued.cpuMillis}};taskData.set(owner.task,state);
 const actual=live(session.id),bundle=bundleSnapshot(session.id);initialBundles.set(session.id,bundle);state.container=actual;
 for(const name of canonicalNames.filter(name=>name!=='LUCI.md'))assert.equal(bundle[name].sha256,owner.canonicalBefore[name],'Captured canonical export differs from its own project: '+name);
 const capturedRoot=bundles.get(session.id),capturedLuci=readFileSync(join(capturedRoot,'LUCI.md'));
 assert(capturedLuci.subarray(0,owner.canonicalLuci.length).equals(owner.canonicalLuci),'Task brief must retain exact own-project LUCI prefix');
 for(const name of ['.luci/brief.json','.luci/specification.json','.luci/design.json','.luci/plan.json'])assert.equal(JSON.parse(readFileSync(join(capturedRoot,name),'utf8')).projectId,owner.owner.id,'Mixed-project context: '+name);
 assert.equal(row(session.id).implementationContext.taskIds.length,1);assert.equal(row(session.id).projectId,owner.owner.id);
 assert.equal(workerContainers().length,1,'Only the successful successor may be active');
 return state;
}
async function setPaused(paused){
 const settings=await api('GET','/v1/product/swarm-scheduler');
 return api('PUT','/v1/product/swarm-scheduler',{expectedRevision:settings.revision,policy:{maxActive:1,maxPerProject:1,headroomPercent:25,minHeadroomMiB:2048,diskReserveMiB:1024},dispatchPaused:paused});
}

async function integrate(task){stage='ui-review-integrate-'+task;const reviewResponse=page.waitForResponse(r=>new URL(r.url()).pathname===projectRoute+'/runs/'+run.id+'/swarm/tasks/'+task+'/review'&&r.request().method()==='GET');await taskRow(task).getByRole('button',{name:'Review task changes',exact:true}).click();const response=await reviewResponse;assert.equal(response.status(),200);const {review}=await response.json();assert.equal(review.canIntegrate,true);assert.equal(review.sourceCommit,observations.get(task).commit);assert.deepEqual(review.paths,[task+'-result.txt']);const dialog=page.getByRole('dialog',{name:'Review task changes',exact:true});await dialog.getByLabel('Reviewed task diff').waitFor();assert((await dialog.getByLabel('Reviewed task diff').innerText()).includes(task+'-result.txt'));await dialog.getByRole('checkbox').check();const result=await action(projectRoute+'/runs/'+run.id+'/swarm/tasks/'+task+'/integrate',()=>dialog.getByRole('button',{name:'Integrate reviewed changes',exact:true}).click());run=result.run;assert.equal(run.swarm.tasks.find(t=>t.id===task).integration.state,'integrated');assert(existsSync(join(project.path,task+'-result.txt')));(receipt.integrations??=[]).push({task,review,commit:run.swarm.tasks.find(t=>t.id===task).integration.commit});}

async function boundedResponse(response, limit) {
 const parts=[];let size=0;
 for await(const part of response.body??[]){size+=part.length;if(size>limit){await response.body?.cancel().catch(()=>{});throw Error('HTTP response bound exceeded');}parts.push(part);}
 return Buffer.concat(parts,size).toString('utf8');
}
function privateJSON(path,limit=2*1024**2){const info=lstatSync(path);assert(info.isFile()&&!info.isSymbolicLink()&&info.nlink===1&&info.size<=limit);const bytes=readFileSync(path);assert(bytes.length<=limit);return JSON.parse(bytes);}
function privateRun(owner){const record=privateJSON(join(fixture,'state/sessions/project-runs',owner.owner.id,owner.run.id+'.json'));assert.equal(record.run.id,owner.run.id);assert.equal(record.run.projectId,owner.owner.id);return record.run;}
function originalOperation(){const value=privateRun(projectStates[0]);const op=value.operations.find(op=>op.taskId==='build-a');assert(op&&op.childId);return {run:value,op};}
function ledger(){const value=privateJSON(join(fixture,'state/control-plane/execution/leases.json'));assert.equal(value.version,1);assert((value.leases??[]).length<=4096&&(value.fences??[]).length<=64);return value;}
function originalLease(){const matches=ledger().leases.filter(value=>value.sessionId===original.session.id);assert.equal(matches.length,1);return matches[0];}
function instanceID(){const value=readFileSync(join(fixture,'state/instance/instance-id'),'utf8').trim();assert(value.length>0&&value.length<=200);return value;}
function workerContainers(){return docker('ps','-aq','--filter','label=luci.managed=true').split(/\s+/).filter(Boolean);}
function validateFence(input, inspected=false){
 const cfg=inspected?input.Config:input,host=input.HostConfig,labels=cfg.Labels;assert(host&&labels&&original);
 assert.deepEqual(Object.keys(labels).sort(),['luci.launch-fence','luci.instance','luci.sandbox-id','luci.session-id','luci.fence-intent','luci.fence-binding'].sort());
 assert.equal(labels['luci.launch-fence'],'1');assert.equal(labels['luci.instance'],instanceID());assert.equal(labels['luci.session-id'],original.session.id);assert.equal(labels['luci.sandbox-id'],original.session.id);assert.match(labels['luci.fence-intent'],/^[a-f0-9]{32}$/);assert.match(labels['luci.fence-binding'],/^[a-f0-9]{64}$/);
 assert.equal(cfg.Image??'','');assert.deepEqual(cfg.Cmd,['/']);assert.equal(cfg.NetworkDisabled,true);assert.equal(cfg.Tty??false,false);assert.equal(cfg.OpenStdin??false,false);assert.equal(cfg.User??'','');assert.equal(cfg.WorkingDir??'','');
 for(const key of ['Entrypoint','Env'])assert.equal(cfg[key]?.length??0,0);for(const key of ['ExposedPorts','Volumes'])assert.equal(Object.keys(cfg[key]??{}).length,0);assert(!cfg.Healthcheck);
 assert.equal(host.ReadonlyRootfs,true);assert.equal(host.NetworkMode,'none');assert.equal(host.IpcMode,'none');assert.equal(host.Privileged??false,false);assert.equal(host.AutoRemove??false,false);assert.equal(host.RestartPolicy?.Name,'no');assert.equal(host.RestartPolicy?.MaximumRetryCount??0,0);
 for(const key of ['Binds','Mounts','VolumesFrom','Devices','DeviceRequests','Links','ExtraHosts','CapAdd'])assert.equal(host[key]?.length??0,0);assert.equal(Object.keys(host.PortBindings??{}).length,0);
 for(const key of ['PidMode','UTSMode'])assert.equal(host[key]??'','');assert.deepEqual(host.CapDrop,['ALL']);assert.deepEqual(host.SecurityOpt,['no-new-privileges:true']);assert.equal(host.LogConfig?.Type,'none');assert.equal(host.Memory,6*1024**2);assert.equal(host.NanoCpus,100000000);assert.equal(host.PidsLimit,1);
 if(inspected){assert.match(input.Id,/^[a-f0-9]{64}$/);assert.equal(input.Name,'/luci-'+original.session.id);assert.equal(input.Image,'');assert.deepEqual(input.Mounts,[]);assert.equal(input.State.Status,'created');assert.equal(input.State.Running,false);assert.equal(input.State.Pid,0);assert.equal(input.Path,'/');assert.equal(input.Args?.length??0,0);}
 return {intent:labels['luci.fence-intent'],binding:labels['luci.fence-binding']};
}
function identifyCreate(bytes,meta){
 const input=JSON.parse(bytes);assert.equal(input.Labels?.['luci.instance'],instanceID());assert.match(input.Labels?.['luci.session-id']??'',/^s-[a-f0-9]{8}$/);assert.equal(input.Labels?.['luci.sandbox-id'],input.Labels['luci.session-id']);assert.equal(meta.name,'luci-'+input.Labels['luci.session-id']);
 if(input.Labels['luci.launch-fence']==='1'){validateFence(input);return 'fence';}
 assert.equal(input.Labels['luci.managed'],'true');assert([image,imageInfo.Id].includes(input.Image));assert.equal(input.HostConfig.Privileged??false,false);assert.equal(input.HostConfig.AutoRemove??false,false);assert(input.HostConfig.Memory>0&&input.HostConfig.NanoCpus>0);assert.equal(input.HostConfig.PidsLimit,256);assert(input.HostConfig.SecurityOpt.includes('no-new-privileges:true'));
 const sessionID=input.Labels['luci.session-id'];const owners=projectStates.filter(owner=>privateRun(owner).operations.some(op=>op.childId===sessionID));assert.equal(owners.length,1);ownedSessions.add(sessionID);
 const owner=owners[0],op=privateRun(owner).operations.find(value=>value.childId===sessionID);validateWorkerCreate(input,{fixture,projectID:owner.owner.id,runID:owner.run.id,operationID:op.id,session:row(sessionID),images:[image,imageInfo.Id],instance:instanceID(),uid:process.getuid(),gid:process.getgid()});
 if(owners[0].task==='build-a'){assert(!originalCreate,'The original launch may not be replayed');originalCreate=input;}
 return 'worker';
}
async function waitForFixtureGate(id,options){
 // A fatal guard refusal cannot reach its gate. Fail promptly, then retain the
 // existing finally path: stop controllers, close/drain proxy, exact cleanup.
 const outcome=await Promise.race([proxy.waitForGate(id,options).then(value=>({value})),fixtureFailureSignal.then(error=>({error}))]);
 if(outcome.error)throw outcome.error;return outcome.value;
}
function guardedIdentifyCreate(bytes,meta){
 try{return identifyCreate(bytes,meta);}catch(error){
  // Keep only error type and local source locations: assertions can contain
  // private create/environment values, so never retain the raw message/body.
  const frames=String(error?.stack??'').split('\n').filter(line=>/^\s+at /.test(line)&&line.includes('/launch-recovery/')).map(line=>line.replace(/^.*\/launch-recovery\//,'launch-recovery/').slice(0,240)).slice(0,6);
  (receipt.createGuardFailures??=[]).push({at:new Date().toISOString(),name:error?.name==='AssertionError'?'AssertionError':'CreateGuardError',frames});
  stopOnFailure(Error('Fixture Create identity/config guard refused; inspect retained safe source locations'));
  throw error;
 }
}
function admission({kind,ordinal,body}){
 try{assert(ordinal>=1&&ordinal<=(scenario==='delayed-create'?3:4));const free=rootFree(),required=6*1024**3;assert(free>=required,'Fresh shape-specific disk admission failed');assert.equal(workerContainers().length,0,'Only one full-image allocation at a time');if(kind==='fence')validateFence(JSON.parse(body));else {const input=JSON.parse(body),session=row(input.Labels['luci.session-id']),owner=projectStates.find(value=>value.owner.id===session.projectId);assert(owner);const op=privateRun(owner).operations.find(value=>value.childId===session.id);assert(op);validateWorkerCreate(input,{fixture,projectID:owner.owner.id,runID:owner.run.id,operationID:op.id,session,images:[image,imageInfo.Id],instance:instanceID(),uid:process.getuid(),gid:process.getgid()});}receipt.resourceObservations??=[];receipt.resourceObservations.push({stage,kind,ordinal,rootFreeBytes:free,requiredBytes:required,at:new Date().toISOString()});}catch(error){stopOnFailure(error);throw error;}
}
function validateCapturedContext(owner,sessionID){
 const bundle=bundleSnapshot(sessionID);for(const name of canonicalNames.filter(name=>name!=='LUCI.md'))assert.equal(bundle[name].sha256,owner.canonicalBefore[name]);
 const root=bundles.get(sessionID),luci=readFileSync(join(root,'LUCI.md'));assert(luci.subarray(0,owner.canonicalLuci.length).equals(owner.canonicalLuci));
 for(const name of ['.luci/brief.json','.luci/specification.json','.luci/design.json','.luci/plan.json'])assert.equal(JSON.parse(readFileSync(join(root,name),'utf8')).projectId,owner.owner.id);
 assert.deepEqual(row(sessionID).implementationContext.taskIds,[owner.task]);assert.equal(row(sessionID).projectId,owner.owner.id);
}
function captureOriginal(){
 const owner=projectStates[0],{op}=originalOperation();assert.equal(op.allocation,'ready');assert(originalCreate);const session=row(op.childId);assert.equal(session.id,originalCreate.Labels['luci.session-id']);assert.equal(session.projectId,owner.owner.id);assert.deepEqual(session.implementationContext.taskIds,['build-a']);
 const mounts=workerBinds(originalCreate);const work=mounts.find(item=>item.Target==='/home/dev/work');assert(work&&work.Type==='bind');const checkout=realpathSync(work.Source);assert(checkout.startsWith(join(dirname(owner.owner.path),'.luci-swarms')+sep));
 const state={task:owner.task,owner:owner.owner,session,operation:op,checkout,count:0,requestedLimits:{memoryMiB:originalCreate.HostConfig.Memory/1024**2,cpuMillis:originalCreate.HostConfig.NanoCpus/1000000}};original=state;taskData.set(state.task,state);
 const captured=mounts.find(item=>item.Target===authorityRoot);assert(captured&&captured.ReadOnly);const root=realpathSync(captured.Source);assert(root.startsWith(join(fixture,'state')+sep));assert(root.endsWith('/.reviewed-context/'+session.id));bundles.set(session.id,root);initialBundles.set(session.id,bundleSnapshot(session.id));validateCapturedContext(owner,session.id);
 const current=originalLease();assert.equal(current.fenceProtocol,1);assert.equal(current.fenceEngineId,engineID);assert.equal(current.fenceInstanceId,instanceID());assert.equal(current.projectId,owner.owner.id);
 receipt.original={task:state.task,projectID:owner.owner.id,runID:owner.run.id,operationID:op.id,sessionID:session.id,leaseID:current.id,protocol:current.fenceProtocol,reviewedContext:session.reviewedContext,bundleHashes:hashes(initialBundles.get(session.id)),checkoutHead:execFileSync('git',['rev-parse','HEAD'],{cwd:checkout,encoding:'utf8'}).trim(),initialLeaseState:current.state};
 if(scenario==='delayed-start'){const ids=containerIDs(session.id);assert.equal(ids.length,1);state.container=inspectOwned(ids[0],session.id);assert.equal(state.container.running,false);assert.equal(state.container.pid,0);receipt.original.containerID=state.container.id;}
 return state;
}
async function crashController(label){
 const identity=controllerIdentities.at(-1);assert(identity&&service&&identity.pid===service.pid&&stillOwned(identity));const family=ownedProcesses();assert.deepEqual(family.map(row=>row.pid),[service.pid],'Personal owner scenario must be served by the exact root process');
 const exited=new Promise(resolveExit=>service.once('exit',(code,signal)=>resolveExit({code,signal})));
 assert.equal(service.kill('SIGKILL'),true);const result=await Promise.race([exited,delay(5000).then(()=>{throw Error('Killed root was not reaped');})]);assert.equal(result.signal,'SIGKILL');assert.equal(stillOwned(identity),null);assert.equal(ownedProcesses().length,0);
 (receipt.crashes??=[]).push({label,pid:identity.pid,startTicks:identity.start,signal:result.signal,reaped:true,at:new Date().toISOString()});
}
async function restartController(){await startService();token=(await api('POST','/v1/auth/login',{username:'swarmowner',password})).token;sensitive.push(token);await setPaused(true);assert.deepEqual(bundleSnapshot(original.session.id),initialBundles.get(original.session.id));}
async function queueOwner(owner){await openProject(owner);await page.getByRole('button',{name:/^Queue reviewed tasks/}).click();const result=await action(owner.route+'/runs/'+owner.run.id+'/swarm/queue',()=>page.getByRole('dialog',{name:'Queue reviewed work',exact:true}).getByRole('button',{name:'Authorize queued tasks',exact:true}).click(),201);assert.equal(result.items.length,1);owner.queued=result.items;}
function browserPostOutcome(path){
 return new Promise(resolveOutcome=>{
  let settled=false;
  const finish=value=>{if(settled)return;settled=true;clearTimeout(timer);page.off('response',received);page.off('requestfailed',failed);resolveOutcome(value);};
  const matches=request=>new URL(request.url()).pathname===path&&request.method()==='POST';
  const received=response=>{if(matches(response.request()))void response.json().then(body=>finish({status:response.status(),body}),()=>finish({transportError:'Unparseable recovery response'}));};
  const failed=request=>{if(matches(request))finish({transportError:'Expected or failed recovery transport interruption',errorCode:request.failure()?.errorText});};
  const timer=setTimeout(()=>finish({transportError:'Recovery response deadline expired'}),remainingBudget(120000));
  page.on('response',received);page.on('requestfailed',failed);
 });
}
async function beginRecovery(owner,mode='initial'){
 await openProject(owner);const initial=mode==='initial',title=initial?'Stop uncertain launch and retain output':'Continue launch recovery';
 const path=owner.route+'/runs/'+owner.run.id+'/swarm/tasks/'+owner.task+'/abandon';
 await taskRow(owner.task).getByRole('button',{name:title,exact:true}).click();const dialog=page.getByRole('dialog',{name:title,exact:true});
 const consent=dialog.getByRole('checkbox',{name:'I authorize recovery of this exact attempt, including stopping its original worker if needed and keeping a permanent Docker recovery marker.',exact:true});await consent.check();
 const submit=dialog.getByRole('button',{name:initial?title:'Continue reviewed recovery',exact:true});await until('Exact supported recovery is available',()=>submit.isEnabled(),20_000);
 const response=browserPostOutcome(path);
 await submit.click();return {response,path};
}
function durableRecovery(stageName){
 const {op}=originalOperation(),data=ledger();const fences=(data.fences??[]).filter(f=>f.childId===original.session.id);assert.equal(fences.length,1);const fence=fences[0];
 const result={stage:stageName,at:new Date().toISOString(),operationStatus:op.status,intentID:op.launchRecovery?.id,hasProof:!!op.launchRecoveryProof,proofRef:op.launchRecoveryProof?.fenceProof?.fenceRef,hasRetirement:!!op.absenceRetirement,finalizedAt:op.absenceRetirement?.finalizedAt??null,leaseRetained:data.leases.some(l=>l.sessionId===original.session.id),fenceState:fence.state,fenceIntentID:fence.intent.id,fenceRef:fence.proof?.fenceProof?.fenceRef,registryStatus:row(original.session.id).status};
 (receipt.recoveryCheckpoints??=[]).push(result);return {result,op,fence};
}
async function verifyFinalized(){
 await until('Exact retired operation and lease publication',()=>{const {op}=originalOperation();return op.absenceRetirement?.finalizedAt&&!ledger().leases.some(l=>l.sessionId===original.session.id);});
 const {result,op,fence}=durableRecovery('finalized');assert.equal(op.status,'cancelled');assert.equal(result.fenceState,'retired');assert.equal(result.registryStatus,'exited');assert.equal(result.leaseRetained,false);assert(op.launchRecoveryProof&&op.absenceRetirement);assert.equal(op.launchRecoveryProof.fenceProof.fenceRef,op.absenceRetirement.fenceProof.fenceRef);assert.equal(result.proofRef,result.fenceRef);
 const actual=JSON.parse(docker('inspect',result.proofRef))[0],binding=validateFence(actual,true);assert.equal(binding.intent,op.launchRecovery.id);assert.equal(actual.Id,result.proofRef);receipt.retainedFence={id:actual.Id,intent:binding.intent,binding:binding.binding,name:actual.Name,inert:true,observedAt:new Date().toISOString()};
 const publicRun=(await api('GET',projectStates[0].route+'/runs/'+projectStates[0].run.id)).run;const publicOp=publicRun.operations.find(v=>v.id===original.operation.id);assert.equal(publicOp.recovery?.state,'finalized');for(const name of ['launchRecovery','launchRecoveryProof','absenceRetirement'])assert(!Object.hasOwn(publicOp,name),'Private launch proof leaked through project API');
 assert.deepEqual(bundleSnapshot(original.session.id),initialBundles.get(original.session.id));assert.equal(execFileSync('git',['rev-parse','HEAD'],{cwd:original.checkout,encoding:'utf8'}).trim(),receipt.original.checkoutHead);assert.equal(execFileSync('git',['status','--porcelain'],{cwd:original.checkout,encoding:'utf8'}).trim(),'');assert.equal(observations.has('build-a'),false);
 const owner=projectStates[0];assert.deepEqual(Object.fromEntries(canonicalNames.map(name=>[name,sha(readFileSync(join(owner.owner.path,name)))])),owner.canonicalBefore);assert.equal(execFileSync('git',['rev-parse','HEAD'],{cwd:owner.owner.path,encoding:'utf8'}).trim(),owner.initialHead);assert.equal(execFileSync('git',['status','--porcelain'],{cwd:owner.owner.path,encoding:'utf8'}).trim(),'');assert(!existsSync(join(owner.owner.path,'build-a-result.txt')));
 return result;
}

try {
 deadlineTimer=setTimeout(()=>stopOnFailure(Error('Fifteen minute acceptance deadline expired')),15*60_000);
 reserveTimer=setInterval(()=>{try{const free=rootFree();(receipt.diskSamples??=[]).push({stage,at:new Date().toISOString(),rootFreeBytes:free});if(free<1024**3)stopOnFailure(Error('Hard one GiB reserve breached'));for(const worker of taskData.values())if(worker.finalReleased&&worker.session)observeTaskLease(worker);}catch(error){stopOnFailure(error);}},1000);
 for(const name of ['SIGTERM','SIGINT'])process.on(name,()=>{signalFailure=name;stopOnFailure(Error('Acceptance interrupted by '+name));});
 receipt.bounds={scenarioDeadlineMs:900000,originalHoldMs:600000,upstreamRequestMs:120000,maxCreates:scenario==='delayed-create'?3:4,maxStarts:scenario==='delayed-create'?1:2,preCreateFreeBytes:6*1024**3,runningReserveBytes:1024**3,workerMemoryBytes:1024**3,workerNanoCPUs:1000000000};
 engineID=docker('info','--format','{{.ID}}');assert(engineID.length>0&&engineID.length<=256);receipt.engineIdentitySHA256=sha(engineID);
 startProvider();await new Promise((done,reject)=>provider.once('error',reject).listen(0,bridge,done));providerBase='http://host.docker.internal:'+provider.address().port+'/v1';
 const port=await freePort(),preview=await freePort();base='http://127.0.0.1:'+port;
 const upstream=process.env.DOCKER_HOST||'unix:///var/run/docker.sock';assert(upstream.startsWith('unix://'));
 proxy=await startFencedDockerProxy({socketPath:join(fixture,'docker-proxy.sock'),upstreamSocketPath:upstream.slice(7),identifyCreate:guardedIdentifyCreate,beforeCreate:admission,maxCreates:receipt.bounds.maxCreates,maxCreatesByKind:{worker:2,fence:scenario==='delayed-create'?1:2},maxStarts:receipt.bounds.maxStarts,maxObservations:20000,upstreamTimeoutMs:120000,holdTimeoutMs:600000});
 proxy.armGate({id:'original',phase:'before-forward',method:'POST',pathPattern:scenario==='delayed-create'?/\/containers\/create$/:/\/containers\/[a-f0-9]{64}\/start$/,match:meta=>meta.kind==='worker'&&(scenario==='delayed-create'?meta.ordinal===1:meta.labels?.['luci.session-id']===originalCreate?.Labels?.['luci.session-id'])});
 writeFileSync(join(fixture,'docker/config.json'),JSON.stringify({cliPluginsExtraDirs:process.env.DOCKER_CONFIG?[join(process.env.DOCKER_CONFIG,'cli-plugins')]:[]}),{mode:0o600});
 env=Object.fromEntries(['PATH','LANG','LC_ALL','TZ'].filter(name=>process.env[name]).map(name=>[name,process.env[name]]));
 Object.assign(env,{HOME:join(fixture,'home'),DOCKER_CONFIG:join(fixture,'docker'),DOCKER_HOST:'unix://'+join(fixture,'docker-proxy.sock'),LUCI_DOCKER_HOST:'unix://'+join(fixture,'docker-proxy.sock'),LUCI_ADDR:'127.0.0.1:'+port,LUCI_STATE_DIR:join(fixture,'state'),LUCI_PROJECTS_DIR:join(fixture,'projects'),LUCI_WEB_DIR:web,LUCI_RUNTIME_DIR:join(dirname(binary),'runtime'),LUCI_SANDBOX_IMAGE:image,LUCI_BACKEND:'docker',LUCI_SESSION_MEMORY_MIB:'1024',LUCI_SESSION_CPU_MILLIS:'1000',LUCI_MANAGED_SHUTDOWN_TOKEN:lifecycle,LUCI_LOG_LEVEL:'error',LUCI_BROWSER_PREVIEW_ADDR:'127.0.0.1:'+preview,LUCI_BROWSER_PREVIEW_PORT:String(preview),GIT_CONFIG_GLOBAL:join(fixture,'home/.gitconfig'),GIT_CONFIG_NOSYSTEM:'1'});
 await startService();
 const bootstrap=readFileSync(join(fixture,'state/control-plane/bootstrap-token'),'utf8').trim();sensitive.push(bootstrap);
 const owner=await api('POST','/v1/auth/bootstrap',{bootstrapToken:bootstrap,username:'swarmowner',displayName:'Launch Recovery Fixture Owner',password},201);token=owner.token;sensitive.push(token);
 await api('POST','/v1/setup/git/identity',{name:'Swarm Fixture',email:'swarm@example.invalid'});
 const readiness=await api('POST','/v1/setup/check',{});for(const id of ['docker','sandbox-image'])assert.equal(readiness.checks.find(row=>row.id===id)?.status,'ready');
 const connections=await api('POST','/v1/setup/agent-connections',{action:'save',expectedRevision:0,name:'Local controlled recovery fixture',kind:'compatible-api',enabled:true,endpoints:{responses:providerBase},models:{codex:'swarm-mechanics-fixture'},apiKey:key});assert.equal(connections.connections.length,1);const connectionID=connections.connections[0].id;
 await setPaused(true);
  for(let index=0;index<2;index++){
   const letter='abcd'[index],task='build-'+letter;titles.set(task,'Build project '+letter.toUpperCase()+' result');
  project=await api('POST',workspace+'/v1/product/projects',{name:'recovery-project-'+letter},201);assert(realpathSync(project.path).startsWith(fixture+sep));projectRoute=workspace+'/v1/product/projects/'+project.id;
  await api('POST',projectRoute+'/brief',{revision:0,purpose:'Prove '+task+' uses its own project context and reviewed integration under a shared worker budget',audience:'Project owners',constraints:'Local labelled fixture; no cloud or model calls',criteria:['Only this project assigned file is changed and reviewed before integration']});
  let spec=await api('GET',projectRoute+'/specification');spec=await api('POST',projectRoute+'/specification',{expectedRevision:spec.revision,expectedBriefRevision:spec.briefRevision,specification:{...spec,requirements:[{id:'AC-notes',title:'Coordinate isolated task work with reviewed integration',priority:'must',acceptance:'Each task changes only its assigned file using its own captured project context; canceled work never allocates a worker.'}]}});
  spec=await api('POST',projectRoute+'/specification/approve',{revision:spec.revision,briefRevision:spec.briefRevision});
  let design=await api('GET',projectRoute+'/design-plan');design=await api('POST',projectRoute+'/design-plan',{expectedRevision:design.revision,expectedBriefRevision:spec.briefRevision,expectedSpecificationRevision:spec.revision,plan:{...design,screens:[{id:'notes',name:'Saved notes',route:'/',purpose:'Save and restore a note',states:['Empty','Saved','Error and retry'],widths:[390,1440],requirementIds:['AC-notes']}]}});
  design=await api('POST',projectRoute+'/design-plan/approve',{revision:design.revision});
  const tasks=[{id:task,title:titles.get(task),status:'todo',requirementIds:['AC-notes'],writePaths:[task+'-result.txt'],acceptance:'Read only this project frozen context, commit its exact controlled task output, and leave primary source unchanged until review.'}];
  const plan=await api('POST',projectRoute+'/plan',{expectedRevision:0,expectedSpecificationRevision:spec.revision,tasks});
  const gitReview=await api('GET',projectRoute+'/git');await api('POST',projectRoute+'/git/commit',{message:'Fixture seed: approved swarm task graph',expectedReviewDigest:gitReview.reviewDigest});
  const branches=(await api('GET',workspace+'/v1/projects/'+project.id+'/branches')).branches;const branch=branches.find(row=>row.name==='main')??branches[0];assert(branch);
  const worktrees=(await api('GET',workspace+'/v1/projects/'+project.id+'/worktrees?branchId='+encodeURIComponent(branch.id))).worktrees;const worktree=worktrees.find(row=>row.isPrimary)??worktrees[0];assert(worktree);
  const selection={projectId:project.id,briefRevision:spec.briefRevision,specificationRevision:spec.revision,designPlanRevision:design.revision,planRevision:plan.revision,taskIds:tasks.map(item=>item.id)};
  const runInput={worktreeId:worktree.id,harness:'codex',model:'swarm-mechanics-fixture',permissionMode:'bypass',agentConnectionId:connectionID,outcome:'preview',implementationContext:selection};
  run=(await api('POST',projectRoute+'/runs',{...runInput,idempotencyKey:'create-installed-swarm-fixture'},201)).run;assert.equal(run.input.route.permissionMode,'bypass');
  const canonicalBefore=canonical(),initialHead=execFileSync('git',['rev-parse','HEAD'],{cwd:project.path,encoding:'utf8'}).trim();
   projectStates.push({owner:project,route:projectRoute,run,task,index,initialHead,canonicalBefore,canonicalLuci:readFileSync(join(project.path,'LUCI.md')),goal:'Prove '+task+' uses its own project context and reviewed integration under a shared worker budget'});
  }

 const browserEnv=Object.fromEntries(['PATH','LANG','LC_ALL','TZ'].filter(n=>process.env[n]).map(n=>[n,process.env[n]]));browserEnv.LUCI_SWARM_BROWSER_FIXTURE=fixture;
 browser=await chromium.launch({headless:true,executablePath:option('--chromium','/usr/bin/chromium'),env:browserEnv,args:['--no-proxy-server']});receipt.browserVersion=browser.version();
 const context=await browser.newContext({viewport:{width:1440,height:1080},serviceWorkers:'block'});
 await context.route('**/*',async route=>{const request=route.request(),url=new URL(request.url());if(url.origin!==base){stopOnFailure(Error('Browser external request refused'));await route.abort('blockedbyclient');return;}if(!['GET','HEAD','OPTIONS'].includes(request.method()))receipt.browserMutations.push({stage,method:request.method(),path:url.pathname});await route.continue();});
 page=await context.newPage();page.setDefaultTimeout(20000);page.on('pageerror',error=>{receipt.browserErrors.push(scrub(error));stopOnFailure(Error('Browser script error'));});
 for(const owner of projectStates){
  stage='browser-prepare-'+owner.task;await openProject(owner);await page.getByRole('button',{name:'Prepare agent team',exact:true}).click();const dialog=page.getByRole('dialog',{name:'Prepare agent team',exact:true});await dialog.getByRole('combobox').selectOption('1');assert.equal(await dialog.getByRole('combobox').inputValue(),'1');owner.run=(await action(owner.route+'/runs/'+owner.run.id+'/swarm',()=>dialog.getByRole('button',{name:'Prepare team',exact:true}).click(),201)).run;run=owner.run;
 }
 const first=projectStates[0],successor=projectStates[1];assert.equal(effects().create,0);
 stage='original-launch';
 if(scenario==='delayed-create'){await queueOwner(first);await setPaused(false);}else{await openProject(first);await taskRow(first.task).getByRole('button',{name:'Start task',exact:true}).click();}
 const gate=await waitForFixtureGate('original',{timeoutMs:120000});captureOriginal();assert.equal(gate.request.labels['luci.session-id'],original.session.id);
 if(scenario==='delayed-start')assert.equal(gate.request.ref,original.container.id);
 stage='active-allocation-refusal';const before=effects(),snapshot=originalOperation();const busyStart=Date.now();const busy=await api('POST',first.route+'/runs/'+first.run.id+'/swarm/tasks/build-a/abandon',{expectedRevision:snapshot.run.revision,operationId:original.operation.id,childId:original.session.id,stopUncertainLaunch:true},409,5000);assert(Date.now()-busyStart<5000);assert(busy.error);assert(!originalOperation().op.launchRecovery);assert.equal((ledger().fences??[]).length,0);assert.deepEqual(effects(),before);check('Live allocation refuses recovery promptly without intent or effects',{error:busy.error,elapsedMs:Date.now()-busyStart});
 stage='kill-original-controller';await crashController('original-request-held');assert.equal(proxy.observations.gates.find(row=>row.id==='original').state,'holding');
 stage='restart-original-controller';await restartController();await queueOwner(successor);assert(originalLease());
 check('Root process death and restart preserve exact original child, context and retained capacity');
 if(scenario==='delayed-create'){
  stage='review-delayed-create-recovery';const pending=await beginRecovery(first);const result=await pending.response;assert.equal(result.status,200,JSON.stringify(result));await verifyFinalized();
 }else{
  proxy.armGate({id:'lost-fence-ack',phase:'after-response',method:'POST',pathPattern:/\/containers\/create$/,match:meta=>meta.kind==='fence'&&meta.status===201,decision:'lose-response'});
  stage='review-lost-ack-recovery';const pending=await beginRecovery(first);const lost=await waitForFixtureGate('lost-fence-ack',{timeoutMs:120000});assert.match(lost.request.id,/^[a-f0-9]{64}$/);const result=await pending.response;assert(result.status>=400&&result.status<600,'Lost fence acknowledgement must remain an explicit failed/pending action');
  const state=durableRecovery('lost-ack');assert(state.result.leaseRetained);assert.equal(state.result.hasProof,false);assert.equal(state.result.hasRetirement,false);assert.equal(state.result.fenceState,'intent');validateFence(JSON.parse(docker('inspect',lost.request.id))[0],true);
  receipt.lostAcknowledgement={containerID:lost.request.id,httpStatus:result.status,intentID:state.result.intentID};check('A real created fence without acknowledgement retains capacity and requires reviewed retry');
  proxy.armGate({id:'durable-proof',phase:'after-response',method:'GET',pathPattern:/\/containers\/[a-f0-9]{64}\/json$/,match:meta=>{const {op}=originalOperation();return meta.status===200&&op.launchRecoveryProof&&op.absenceRetirement&&!op.absenceRetirement.finalizedAt&&meta.ref===op.launchRecoveryProof.fenceProof.fenceRef;}});
  stage='review-retry-before-second-crash';const second=await beginRecovery(first,'retry');const boundary=await waitForFixtureGate('durable-proof',{timeoutMs:120000});const durable=durableRecovery('proof-persisted-before-finalize');assert(durable.result.leaseRetained&&durable.result.hasProof&&durable.result.hasRetirement);assert.equal(durable.result.finalizedAt,null);assert.notEqual(durable.result.proofRef,lost.request.id);assert.equal(durable.result.proofRef,boundary.request.ref);assert.equal(durable.result.intentID,receipt.lostAcknowledgement.intentID);
  assert(!docker('ps','-aq','--no-trunc').split(/\s+/).includes(lost.request.id));receipt.acknowledgedReplacement={id:durable.result.proofRef,originalLostID:lost.request.id,sameIntent:true};
  stage='kill-after-durable-proof';await crashController('proof-and-retirement-persisted-before-capacity-release');await proxy.releaseGate('durable-proof');await second.response;
  stage='restart-after-durable-proof';await restartController();assert(originalLease());const same=durableRecovery('restarted-with-proof');assert.equal(same.result.proofRef,durable.result.proofRef);assert.equal(same.result.intentID,durable.result.intentID);const beforeRetry=effects();
  stage='review-finalization-resume';const final=await beginRecovery(first,'retry');const finalResult=await final.response;assert.equal(finalResult.status,200,JSON.stringify(finalResult));await verifyFinalized();assert.equal(effects().create,beforeRetry.create,'Saved acknowledged proof must not allocate another fence');assert.equal(effects().start,beforeRetry.start);check('Reviewed retry resumes durable proof after a second root death without replacing the acknowledged fence');
 }
 stage='release-real-late-request';const late=await proxy.releaseGate('original');assert.equal(late.status,scenario==='delayed-create'?409:404);assert(late.forwardedAt&&late.completedAt);assert(!late.uncertainMutation);receipt.lateOriginal={method:late.method,path:late.path,status:late.status,forwardedAt:late.forwardedAt,completedAt:late.completedAt,realDaemonResponse:true};assert.equal(observations.has('build-a'),false);assert.equal(workerContainers().length,0);validateFence(JSON.parse(docker('inspect',receipt.retainedFence.id))[0],true);check('Original uploaded request reaches the real daemon after recovery and cannot allocate or start a worker',{status:late.status});
 stage='independent-project-advance';await setPaused(false);const worker=await queuedChild(successor,1);await setPaused(true);await attach(worker.session.id);await until('Actual successor Codex task commits its isolated result',()=>observations.has('build-b'),150000);assert(!existsSync(join(successor.owner.path,'build-b-result.txt')));assert.equal(observations.size,1);assert(!existsSync(join(worker.checkout,'build-a-result.txt')));assert.deepEqual(bundleSnapshot(worker.session.id),initialBundles.get(worker.session.id));
 await completeNaturally(worker,successor);await openProject(successor);run=(await action(successor.route+'/runs/'+successor.run.id+'/swarm/reconcile',()=>page.getByRole('button',{name:'Check team status',exact:true}).click())).run;await integrate('build-b');successor.run=run;assert.equal(sha(readFileSync(join(successor.owner.path,'build-b-result.txt'))),observations.get('build-b').outputSHA256);assert.deepEqual(canonical(),successor.canonicalBefore);
 await openProject(first);await page.screenshot({path:join(evidence,'recovered-original-desktop.png'),fullPage:true});await page.setViewportSize({width:390,height:844});await page.screenshot({path:join(evidence,'recovered-original-phone.png'),fullPage:true});
 assert.equal(proxy.observations.counts.create,receipt.bounds.maxCreates);assert.equal(proxy.observations.counts.start,receipt.bounds.maxStarts);assert.equal(proxy.observations.counts.worker,2);assert.equal(proxy.observations.counts.fence,scenario==='delayed-create'?1:2);assert.equal(proxy.observations.dropped,0);assert.equal(proxy.observations.failures.length,0);assert.equal(receipt.browserErrors.length,0);assert.equal(receipt.crashes.length,scenario==='delayed-create'?1:2);assert.equal(observations.size,1);assert.equal(workerContainers().length,0);assert.equal(docker('info','--format','{{.ID}}'),engineID);
 check('Another project completes a real isolated native task, frees capacity naturally and integrates only after browser review');receipt.passed=true;
}catch(error){receipt.failedStage=stage;receipt.failure=redactError(error);process.exitCode=1;console.error(receipt.failure);if(page){try{await page.screenshot({path:join(evidence,'failed-stage.png'),fullPage:true});}catch{}}}
finally {
 closing=true;clearTimeout(deadlineTimer);clearInterval(reserveTimer);for(const socket of sockets)socket.close();
 receipt.actualTaskResults=Object.fromEntries(observations);receipt.taskChildren=[...taskData.values()].map(value=>({task:value.task,projectID:value.owner.id,operationID:value.operation.id,sessionID:value.session.id,container:value.container,bundle:initialBundles.get(value.session.id)}));receipt.terminalObservations=terminalProbes.map(({id,attachedStage,probe})=>({sessionID:id,attachedStage,...probe.snapshot()}));
 const cleanupFailure=error=>{receipt.cleanup.push(redactError(error));receipt.passed=false;process.exitCode=1;};
 if(browser){try{await browser.close();}catch(error){cleanupFailure(error);}const end=Date.now()+5000;while(Date.now()<end&&ownedProcesses('browser').length)await delay(100);for(const identity of ownedProcesses('browser')){cleanupFailure(Error('Owned browser required forced cleanup'));if(stillOwned(identity)){try{process.kill(identity.pid,'SIGKILL');}catch(error){if(error.code!=='ESRCH')cleanupFailure(error);}}}}
 let stopped=false;try{stopped=await stopService(false);assert(stopped);assert(controllerIdentities.filter(Boolean).every(identity=>!stillOwned(identity)));}catch(error){cleanupFailure(error);}
 receipt.controllersStopped=stopped&&controllerIdentities.filter(Boolean).every(identity=>!stillOwned(identity));
 if(proxy){try{const cleanup=await proxy.close();assert(cleanup.drained&&cleanup.pendingRequests===0&&cleanup.activeHandlers===0);receipt.dockerRequestsDrained=true;}catch(error){cleanupFailure(error);}receipt.dockerTransportObservations=structuredClone(proxy.observations);if(proxy.observations.dropped||proxy.observations.failures.length)cleanupFailure(Error('Docker transport recorded a refusal, lost observation or uncertain request'));}
 else {receipt.dockerRequestsDrained=true;receipt.dockerTransportNotStarted=true;}
 // TEST-ONLY teardown is authorized solely after every owned controller is dead
 // and every captured original request is drained or cancelled before forwarding.
 // Production keeps these markers permanently. Never turn uncertainty into a
 // forced/name-based delete; retain all private state when this proof is absent.
 if(receipt.controllersStopped&&receipt.dockerRequestsDrained&&proxy){
  const created=proxy.observations.requests.filter(value=>value.method==='POST'&&/\/containers\/create$/.test(value.path)&&value.status===201&&/^[a-f0-9]{64}$/.test(value.id??''));
  for(const creation of created){try{
   const ids=docker('ps','-aq','--no-trunc').split(/\s+/).filter(Boolean);if(!ids.includes(creation.id))continue;
   const value=JSON.parse(docker('inspect',creation.id))[0];assert.equal(value.Id,creation.id);assert.equal(value.Name,'/'+creation.name);assert.equal(value.Config.Labels['luci.instance'],instanceID());assert.equal(value.Config.Labels['luci.session-id'],creation.labels['luci.session-id']);assert(ownedSessions.has(value.Config.Labels['luci.session-id']));
   if(creation.kind==='fence'){
    validateFence(value,true);for(const key of ['luci.fence-intent','luci.fence-binding'])assert.equal(value.Config.Labels[key],creation.labels[key]);
    docker('rm',value.Id);(receipt.testOnlyFenceTeardown??=[]).push({id:value.Id,immutableID:true,force:false,controllersDead:true,originalRequestsDrained:true,at:new Date().toISOString()});
   }else{
    assert.equal(value.Config.Labels['luci.managed'],'true');assert.equal(value.Image,imageInfo.Id);assert(value.Mounts.some(mount=>mount.Destination==='/home/dev/work'&&projectStates.some(owner=>realpathSync(mount.Source).startsWith(join(dirname(owner.owner.path),'.luci-swarms')+sep))));cleanupFailure(Error('Unexpected full worker remained after scenario; exact test cleanup required'));
    if(value.State.Running)docker('stop','--time','5',value.Id);const current=JSON.parse(docker('inspect',value.Id))[0];assert.equal(current.Id,value.Id);assert.equal(current.State.Running,false);docker('rm',value.Id);
   }
  }catch(error){cleanupFailure(error);}}
 }
 if(provider){provider.closeAllConnections();if(provider.listening)await new Promise(done=>provider.close(done));}
 try{assert(receipt.controllersStopped);assert.equal(receipt.dockerRequestsDrained,true);assert.equal(docker('ps','-aq'),'');assert.equal(docker('volume','ls','-q'),'');assert.equal(ownedProcesses().length,0);assert.equal(ownedProcesses('browser').length,0);assert.equal(JSON.parse(docker('image','inspect',image))[0].Id,imageInfo.Id);assert.equal(sha(readFileSync(binary)),provenance.binarySHA256);assert.equal(sha(readFileSync(join(web,'index.html'))),provenance.webIndexSHA256);assert.equal(docker('info','--format','{{.ID}}'),engineID);
  receipt.runtimeCleanup={containers:0,volumes:0,ownedServiceProcesses:0,ownedBrowserProcesses:0,protectedImageAndPackageUnchanged:true,rootFreeBytes:rootFree()};assert.equal(realpathSync(fixture),fixture);assert.equal(dirname(fixture),fixtureParent);assert(fixture.split(sep).at(-1).startsWith('luci-launch-recovery-'));if(receipt.passed&&!fixtureError){rmSync(fixture,{recursive:true});receipt.fixtureRemoved=true;}
 }catch(error){cleanupFailure(error);}
 if(!receipt.fixtureRemoved)receipt.fixtureRetained={path:fixture,reason:'Failed acceptance. Private fixture credentials and exact retained source/context remain. Test-only cleanup may have retired captured markers; do not resume this diagnostic fixture as a production workspace.'};
 if(fixtureError){receipt.passed=false;process.exitCode=1;receipt.backgroundFailure=redactError(fixtureError);}receipt.providerCalls=providerCalls;receipt.auxiliaryCalls=auxiliaryCalls;receipt.actualTaskCount=observations.size;receipt.elapsedMs=Date.now()-started;receipt.signal=signalFailure??null;receipt.finishedAt=new Date().toISOString();writeFileSync(report,JSON.stringify(receipt,null,2)+'\n',{mode:0o600,flag:'wx'});console.log((receipt.passed?'PASS ':'FAIL ')+report);
}
