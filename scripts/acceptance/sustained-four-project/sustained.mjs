#!/usr/bin/env node
// Installed sustained mixed-work witness. Real Docker/native commands; explicitly scripted provider.
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
import { mixedTaskCommand } from './task-command.mjs';
import { projectWorkload } from './workloads.mjs';
import { installBrowserTiming, timingSnapshot, interactionRunner, summarizeInteractions } from './interaction-metrics.mjs';

const here=dirname(fileURLToPath(import.meta.url)),repo=resolve(process.env.LUCI_ACCEPTANCE_REPO??resolve(here,'../../..')),args=process.argv.slice(2);
const {chromium}=await import(join(repo,'desktop/node_modules/playwright/index.mjs'));
const {validateWorkerCreate}=await import(join(repo,'scripts/acceptance/launch-recovery/worker-create-guard.mjs'));
const {startFencedDockerProxy}=await import(join(repo,'scripts/acceptance/launch-recovery/fenced-docker-proxy.mjs'));
const {createNativeTerminalProbe}=await import(join(repo,'scripts/acceptance/four-project-queue/native-terminal-diagnostics.mjs'));
const option = (name, fallback) => { const i = args.indexOf(name); if (i < 0) return fallback; assert(args[i + 1] && !args[i + 1].startsWith('--'), 'Missing value for ' + name); return args[i + 1]; };
if(args.includes('--help')){console.log('PREPARED ONLY: installed scripted five-worker/four-project mixed build/test acceptance. Required --binary --web --provenance --expected-source --image --expected-image-id --image-provenance --report FRESH_JSON --fixture-root SHORT_PATH. Root exclusive Docker window and verified clean package required; no model quality or Windows claim.');process.exit(0);}
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
const report = resolve(option('--report', 'artifacts/quality-swarm/sustained-mixed/attempt-' + Date.now() + '/result.json')), evidence = dirname(report);
if (existsSync(evidence)) assert.equal(readdirSync(evidence).length, 0, 'Use a fresh evidence directory; prior failures must remain');
mkdirSync(evidence, { recursive: true, mode: 0o700 });
writeFileSync(join(evidence, 'package-provenance.json'), provenanceBytes, { mode: 0o600, flag: 'wx' });
writeFileSync(join(evidence,'image-provenance.json'),readFileSync(resolve(imageEvidencePath)),{mode:0o600,flag:'wx'});
const driverNames=['sustained.mjs','task-command.mjs','workloads.mjs','interaction-metrics.mjs','run-with-observer.py'];
const reusedHelpers=['four-project-queue/native-terminal-diagnostics.mjs','four-project-queue/sample-linux-owned.py','launch-recovery/fenced-docker-proxy.mjs','launch-recovery/worker-create-guard.mjs'];
mkdirSync(join(evidence, 'accepted-harness'), { mode: 0o700 });
for (const name of driverNames) writeFileSync(join(evidence, 'accepted-harness', name), readFileSync(new URL(name, import.meta.url)), { mode: 0o600, flag: 'wx' });
for (const name of reusedHelpers) writeFileSync(join(evidence,'accepted-harness',name.replaceAll('/','_')),readFileSync(join(repo,'scripts/acceptance',name)),{mode:0o600,flag:'wx'});
const fixtureParent = resolve(option('--fixture-root', 'artifacts/quality-swarm/private-fixtures')); mkdirSync(fixtureParent, {recursive:true,mode:0o700});
const fixture = mkdtempSync(join(fixtureParent,'luci-sustained-'));
for (const folder of ['home', 'projects', 'state', 'docker']) mkdirSync(join(fixture, folder), { mode: 0o700 });
const key = 'fixture-only-' + randomUUID(), lifecycle = randomUUID() + randomUUID(), password = randomUUID() + randomUUID();
const sensitive = [key, lifecycle, password], scrub = value => sensitive.reduce((text, secret) => text.replaceAll(secret, '[redacted]'), String(value));
const receipt = {
  passed: false, mode:'sustained-installed-scripted-mixed', startedAt: new Date().toISOString(), sourceCommit: provenance.sourceCommit, sourceDirty: provenance.sourceDirty,
  harnessCheckout: { commit: execFileSync('git', ['rev-parse', 'HEAD'], { cwd: repo, encoding: 'utf8' }).trim(), dirty: !!execFileSync('git', ['status', '--porcelain'], { cwd: repo, encoding: 'utf8' }).trim() },
  packageIntegrity, initialRootFreeBytes: initialRootFree,
  resourceScope: 'At most five real workers across four projects, one at a time; four initial150s fuzz phases and one short reviewed amendment. Zero existing containers and six GiB root admission before every Create, no recovery fences allowed; hard one GiB running reserve. No image pulls, release or cloud allocation.',
  binarySHA256: sha(readFileSync(binary)), webIndexSHA256: sha(readFileSync(join(web, 'index.html'))), driverSHA256: sha(readFileSync(fileURLToPath(import.meta.url))),
  helperSHA256: Object.fromEntries(driverNames.filter(name=>name!=='sustained.mjs').map(name => [name, sha(readFileSync(new URL(name, import.meta.url)))])),
  reusedHelperSHA256:Object.fromEntries(reusedHelpers.map(name=>['scripts/acceptance/'+name,sha(readFileSync(join(repo,'scripts/acceptance',name)))])),
  image: { reference: image, id: imageInfo.Id }, releaseImage: { reference: releaseImage, id: releaseImageInfo.Id },
  scope: 'Installed mixed build/test commands, explicit idle/settle, action latency, cancellation, active-command restart and reviewed amendment. Scripted provider; no live-model or Windows claim.',
  limitations: ['Existing trusted Docker images; installation is not exercised.', 'Provider output is deliberately scripted; this proves actual CLI, Docker, task coordination and review mechanics. It does not measure model reasoning, production application quality, cloud deployment or Windows behavior.'],
  externalModelCalls: 0, cloudResourcesCreated: 0, setupScaffolding: ['API-seeded personal owner and four deterministic source/test/context fixtures','API-configured one-worker policy; browser queue/cancel/review consent','Reviewed amendment context/test fixture seeded via API/private owned file setup; no claim that an LLM authored the fixtures','Local scripted provider, no external model calls'], checks: [], browserMutations: [], browserErrors: [], cleanup: [],
};

const check = (name, details = {}) => { receipt.checks.push({ name, passed: true, ...details }); console.log('PASS ' + name); };
const names = ['.luci/brief.json', '.luci/design-handoff.json', '.luci/design-handoff.md', '.luci/design.json', '.luci/plan.json', '.luci/specification.json', '.luci/specification.md', 'LUCI.md'];
// The private brief is generated from saved state; only seven members are checkout exports.
const canonicalNames = names.filter(name => name !== '.luci/brief.json');
const authorityRoot = '/home/dev/.luci-reviewed-context';
const sockets = new Set(), socketSessions = new Map(), ownedSessions = new Set(), bundles = new Map(), nativeHomes = new Map();
const terminalProbes = [];
let service, proxy, provider, base, env, token, project, projectRoute;const workspace='';
const controllerIdentities=[];let measuredPhase='setup',interactions,interactionRound=0;const taskEvents=new Map();
let stage = 'startup', fixtureError, reserveTimer, deadlineTimer, closing = false, signalFailure;
let providerCalls = 0, auxiliaryCalls = 0; const observations=new Map(), taskData=new Map(), calls=new Map(); let browser, page, run;
let providerBase; const initialBundles=new Map(), projectStates=[], titles=new Map();
receipt.phases=[];receipt.interactions=[];const started = Date.now(), deadline = started + 30 * 60_000;
const remaining = () => { if (Date.now() >= deadline) throw Error('Thirty minute acceptance deadline expired'); return deadline - Date.now(); };
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
function commandFor(task){const value=taskData.get(task);assert(value);return mixedTaskCommand({task,operation:value.operation.id,projectId:value.owner.id,implementation:value.workload.implementation,expectedOutput:value.workload.expectedOutput,authorityRoot,contextNames:names,durationSeconds:task==='amend-a'?0:150});}
function nextTool(state,name,args){if(name==='exec_command'){state.codingDispatches=(state.codingDispatches??0)+1;assert.equal(state.codingDispatches,1,'No duplicate task command');}assert(++state.count<=48,'Bounded task tool request cap');const callID='call_swarm_'+state.task+'_'+state.operation.id+'_'+state.count;calls.set(callID,{state,consumed:false});return {id:'fc_'+callID,type:'function_call',status:'completed',name,call_id:callID,arguments:JSON.stringify(args)};}
function startProvider(){
 provider=httpServer(async(req,res)=>{try{
  assert.equal(req.method,'POST');assert.equal(req.headers.authorization,'Bearer '+key);if(closing){res.writeHead(503);res.end();return;}
  assert.equal(req.url,'/v1/responses');const parts=[];let bytes=0;for await(const part of req){bytes+=part.length;assert(bytes<4*1024**2);parts.push(part);}const body=JSON.parse(Buffer.concat(parts));assert.equal(body.model,'swarm-mechanics-fixture');assert(++providerCalls<=256);
  const resultItems=outputs(body.input),pending=resultItems.filter(v=>calls.has(v.call_id)&&!calls.get(v.call_id).consumed);assert(pending.length<=1,'Ambiguous unconsumed task output');let response;
  if(pending.length){const output=pending[0],entry=calls.get(output.call_id),state=entry.state;entry.consumed=true;const raw=typeof output.output==='string'?output.output:JSON.stringify(output.output);for(const match of raw.matchAll(/LUCI_MIXED_EVENT=([A-Za-z0-9+/=]+)/g)){const event=JSON.parse(Buffer.from(match[1],'base64'));assert.equal(event.task,state.task);assert.equal(event.operation,state.operation.id);const rows=taskEvents.get(state.task)??[];assert(rows.length<80);rows.push({...event,observedAt:new Date().toISOString()});taskEvents.set(state.task,rows);}assert(!/Process exited with code (?:[1-9]\d*|-\d+)|"exit_code"\s*:\s*(?:[1-9]\d*|-\d+)|Command failed/.test(raw));
   const match=[...raw.matchAll(/LUCI_MIXED_OBSERVATION=([A-Za-z0-9+/=]+)/g)];
   if(match.length){assert.equal(match.length,1);assert(!observations.has(state.task));const actual=JSON.parse(Buffer.from(match[0][1],'base64'));assert.equal(actual.task,state.task);assert.equal(actual.operation,state.operation.id);assert.equal(actual.uid,process.getuid());assert.equal(actual.cwd,'/home/dev/work');assert.equal(actual.readOnly,true);assert.deepEqual(hashes(actual.files),hashes(initialBundles.get(state.session.id)));assert.deepEqual([...actual.changed].sort(),['engine.go',state.task+'-result.json'].sort());assert.equal(actual.artifact.implementationSHA256,sha(state.workload.implementation));assert.equal(actual.artifact.expectedOutputSHA256,sha(state.workload.expectedOutput));assert.equal(actual.artifact.commands.find(row=>row.label==='expected-red')?.code,1);assert(actual.artifact.commands.filter(row=>row.label!=='expected-red').every(row=>row.code===0&&!row.signal));if(state.task!=='amend-a')assert(actual.artifact.commands.find(row=>row.label==='fuzz')?.elapsedMs>=150000);assert.match(actual.commit,/^[a-f0-9]{40,64}$/);observations.set(state.task,actual);entry.finalTask=state.task;entry.finalOutputSHA256=sha(raw);state.finalResponseRequests=(state.finalResponseRequests??0)+1;response=message('Controlled task '+state.task+' committed. Human integration and complete build verification remain separate.');}
   else {const running=raw.match(/Process running with session ID (\d+)/)||raw.match(/"session_id"\s*:\s*(\d+)/);assert(running,'Actual tool output must supply completion receipt or live command identity');response=nextTool(state,'write_stdin',{session_id:Number(running[1]),chars:'',yield_time_ms:10000,max_output_tokens:16000});}
  }else if(resultItems.some(value=>calls.get(value.call_id)?.finalTask)){
   const finals=resultItems.filter(value=>calls.get(value.call_id)?.finalTask);assert.equal(finals.length,1);
   const output=finals[0],entry=calls.get(output.call_id),state=entry.state,raw=typeof output.output==='string'?output.output:JSON.stringify(output.output);
   assert.equal(sha(raw),entry.finalOutputSHA256,'Only an identical verified final-output request may retry');assert(observations.has(state.task));assert.equal(state.codingDispatches,1);assert((state.finalResponseRequests??0)<3);state.finalResponseRequests=(state.finalResponseRequests??0)+1;
   response=message('Controlled task '+state.task+' committed. Human integration and complete build verification remain separate.');
  }else if((body.tools??[]).some(t=>(t.name??t.function?.name)==='exec_command')){
   const text=JSON.stringify(body.input);assert(text.includes('Reviewed context directory: '+authorityRoot));const matched=[...titles].filter(([task,title])=>text.includes(task+': '+title)).map(([task])=>task);assert.equal(matched.length,1,'Native opening prompt must identify exactly one assigned fixture task');const task=matched[0];await until('Controller verifies actual task bundle',()=>taskData.get(task)?.session&&initialBundles.has(taskData.get(task).session.id));const state=taskData.get(task);assert.equal(state.count,0,'No duplicate coding dispatch');response=nextTool(state,'exec_command',commandFor(task));
  }else{assert(++auxiliaryCalls<=4);response=message('Controlled swarm fixture acknowledgement.');}
  (receipt.providerRequests??=[]).push({number:providerCalls,toolOutputCount:resultItems.length,issuedTool:response.type==='function_call'?response.name:null});streamResponse(res,response,providerCalls);
 }catch(error){stopOnFailure(error);if(!res.headersSent)res.writeHead(500);res.end('Controlled swarm fixture contract failed');}});
}
async function action(path,click,expected=200){const pending=page.waitForResponse(r=>new URL(r.url()).pathname===path&&r.request().method()==='POST',{timeout:remainingBudget(150000)});await click();const response=await pending;assert.equal(response.status(),expected,path+': '+await response.text());return response.json();}
async function openProject(state) {
 await passiveTiming('before-document-navigation');await page.goto(base);
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
 const state={task:owner.task,owner:owner.owner,workload:owner.workload,operation,session,checkout:checkout.path,count:0,requestedLimits:{memoryMiB:queued.memoryMiB,cpuMillis:queued.cpuMillis}};taskData.set(owner.task,state);
 const actual=live(session.id),bundle=bundleSnapshot(session.id);initialBundles.set(session.id,bundle);state.container=actual;
 for(const name of canonicalNames.filter(name=>name!=='LUCI.md'))assert.equal(bundle[name].sha256,owner.canonicalBefore[name],'Captured canonical export differs from its own project: '+name);
 const capturedRoot=bundles.get(session.id),capturedLuci=readFileSync(join(capturedRoot,'LUCI.md'));
 assert(capturedLuci.subarray(0,owner.canonicalLuci.length).equals(owner.canonicalLuci),'Task brief must retain exact own-project LUCI prefix');
 for(const name of ['.luci/brief.json','.luci/specification.json','.luci/design.json','.luci/plan.json'])assert.equal(JSON.parse(readFileSync(join(capturedRoot,name),'utf8')).projectId,owner.owner.id,'Mixed-project context: '+name);
 assert.equal(row(session.id).implementationContext.taskIds.length,1);assert.equal(row(session.id).projectId,owner.owner.id);
 assert.equal(workerContainers().length,1,'One worker ceiling is enforced');
 return state;
}
async function setPaused(paused){
 const settings=await api('GET','/v1/product/swarm-scheduler');
 return api('PUT','/v1/product/swarm-scheduler',{expectedRevision:settings.revision,policy:{maxActive:1,maxPerProject:1,headroomPercent:25,minHeadroomMiB:2048,diskReserveMiB:1024},dispatchPaused:paused});
}

async function integrate(task){stage='ui-review-integrate-'+task;const reviewResponse=page.waitForResponse(r=>new URL(r.url()).pathname===projectRoute+'/runs/'+run.id+'/swarm/tasks/'+task+'/review'&&r.request().method()==='GET');await taskRow(task).getByRole('button',{name:'Review task changes',exact:true}).click();const response=await reviewResponse;assert.equal(response.status(),200);const {review}=await response.json();assert.equal(review.canIntegrate,true);assert.equal(review.sourceCommit,observations.get(task).commit);assert.deepEqual([...review.paths].sort(),['engine.go',task+'-result.json'].sort());const dialog=page.getByRole('dialog',{name:'Review task changes',exact:true});await dialog.getByLabel('Reviewed task diff').waitFor();assert((await dialog.getByLabel('Reviewed task diff').innerText()).includes(task+'-result.json'));await dialog.getByRole('checkbox').check();const result=await action(projectRoute+'/runs/'+run.id+'/swarm/tasks/'+task+'/integrate',()=>dialog.getByRole('button',{name:'Integrate reviewed changes',exact:true}).click());run=result.run;assert.equal(run.swarm.tasks.find(t=>t.id===task).integration.state,'integrated');assert(existsSync(join(project.path,task+'-result.json')));(receipt.integrations??=[]).push({task,review,commit:run.swarm.tasks.find(t=>t.id===task).integration.commit});}

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
 const input=JSON.parse(bytes),sessionID=input.Labels?.['luci.session-id'];assert.match(sessionID??'',/^s-[a-f0-9]{8}$/);assert.equal(meta.name,'luci-'+sessionID);assert.equal(input.Labels?.['luci.managed'],'true');assert.equal(input.Labels?.['luci.launch-fence'],undefined);
 const owners=projectStates.filter(owner=>privateRun(owner).operations.some(op=>op.childId===sessionID));assert.equal(owners.length,1);const owner=owners[0],op=privateRun(owner).operations.find(value=>value.childId===sessionID);assert.equal(op.taskId,owner.task,'Only the five specified tasks can allocate');ownedSessions.add(sessionID);
 validateWorkerCreate(input,{fixture,projectID:owner.owner.id,runID:owner.run.id,operationID:op.id,session:row(sessionID),images:[image,imageInfo.Id],instance:instanceID(),uid:process.getuid(),gid:process.getgid()});return 'worker';
}
function admission({kind,ordinal,body,...meta}){
 try{assert.equal(kind,'worker');assert(ordinal>=1&&ordinal<=5);identifyCreate(body,meta);const free=rootFree();assert(free>=6*1024**3,'Six GiB pre-create guard remains mandatory');assert.equal(workerContainers().length,0,'No overlapping full worker permitted');(receipt.resourceObservations??=[]).push({at:new Date().toISOString(),stage,ordinal,rootFreeBytes:free});}catch(error){stopOnFailure(error);throw error;}
}
function health(){remaining();if(fixtureError)throw fixtureError;if(service&&service.exitCode!==null)throw Error('Owned service exited unexpectedly');}
async function timed(promise,ms,label){let timer;try{return await Promise.race([promise,new Promise((_,reject)=>{timer=setTimeout(()=>reject(Error(label+' deadline exceeded')),ms);})]);}finally{clearTimeout(timer);}}
function phaseStart(name){measuredPhase=name;const row={name,startedAt:new Date().toISOString(),monotonicStartMs:performance.now()};receipt.phases.push(row);return row;}
function phaseEnd(row){row.finishedAt=new Date().toISOString();row.elapsedMs=performance.now()-row.monotonicStartMs;delete row.monotonicStartMs;}
async function passiveTiming(reason){if(!page)return;const value=await timed(timingSnapshot(page),15000,'Passive browser timing snapshot');if(value){assert(!value.overflow,'Browser observation overflow invalidates event metrics');(receipt.browserEventWindows??=[]).push({reason,at:new Date().toISOString(),...value});}}
async function quietPhase(name){const record=phaseStart(name);assert.equal(workerContainers().length,0);const before=effects();for(let elapsed=0;elapsed<60;elapsed++){health();await delay(1000);}assert.equal(workerContainers().length,0);assert.deepEqual(effects(),before);phaseEnd(record);assert(record.elapsedMs>=60000);}
async function prepare(owner){await openProject(owner);await page.getByRole('button',{name:'Prepare agent team',exact:true}).click();const dialog=page.getByRole('dialog',{name:'Prepare agent team',exact:true});await dialog.getByRole('combobox').selectOption('1');assert.equal(await dialog.getByRole('combobox').inputValue(),'1');owner.run=(await action(owner.route+'/runs/'+owner.run.id+'/swarm',()=>dialog.getByRole('button',{name:'Prepare team',exact:true}).click(),201)).run;}
async function queueOwner(owner){await openProject(owner);await page.getByRole('button',{name:/^Queue reviewed tasks/}).click();const result=await action(owner.route+'/runs/'+owner.run.id+'/swarm/queue',()=>page.getByRole('dialog',{name:'Queue reviewed work',exact:true}).getByRole('button',{name:'Authorize queued tasks',exact:true}).click(),201);assert.equal(result.items.length,owner.task==='build-a'?2:1);owner.queued=result.items;}
function seedFiles(owner,files){for(const [name,text] of Object.entries(files)){assert(['go.mod','engine.go','engine_test.go','cmd/app/main.go','witness.txt'].includes(name));const file=join(owner.path,name);mkdirSync(dirname(file),{recursive:true});assert(!existsSync(file)||lstatSync(file).isFile());writeFileSync(file,text,{mode:0o600});}}
async function saveContext(owner,workload,task,goal,connectionID,amend=false){
 project=owner;projectRoute='/v1/product/projects/'+owner.id;
 const brief=amend?await api('GET',projectRoute+'/brief'):{revision:0};await api('POST',projectRoute+'/brief',{revision:brief.revision,purpose:goal,audience:'Project owners',constraints:'Offline controlled acceptance workload. No external model or package downloads.',criteria:[amend?'Render the exact visible item total after the list.':'Meet fixed requirements and varied-input invariants.']});
 let spec=await api('GET',projectRoute+'/specification');spec=await api('POST',projectRoute+'/specification',{expectedRevision:spec.revision,expectedBriefRevision:spec.briefRevision,specification:{...spec,purpose:goal,requirements:[{id:'AC-work',title:workload.label,priority:'must',acceptance:amend?'Escaped ordered output includes exact visible item total.':'The supplied fixed tests and independent varied-input invariant checks pass; compiled behavior matches the saved expected output.'}]}});spec=await api('POST',projectRoute+'/specification/approve',{revision:spec.revision,briefRevision:spec.briefRevision});
 let design=await api('GET',projectRoute+'/design-plan');design=await api('POST',projectRoute+'/design-plan',{expectedRevision:design.revision,expectedBriefRevision:spec.briefRevision,expectedSpecificationRevision:spec.revision,plan:{...design,screens:[{id:'output',name:'Verified output',route:'/',purpose:workload.label,states:['Empty','Verified','Error and retry'],widths:[390,1440],requirementIds:['AC-work']}]}});design=await api('POST',projectRoute+'/design-plan/approve',{revision:design.revision});
 const tasks=[{id:task,title:titles.get(task),status:'todo',requirementIds:['AC-work'],writePaths:['engine.go',task+'-result.json'],acceptance:amend?'Implement the amended visible total; run the new fixed checks and compiled artifact.':'Implement engine.go; verify real red/green fixed tests, compiled output and150seconds varied-input fuzz tests; commit the result receipt.'}];
 if(task==='build-a')tasks.push({id:'cancel-a',title:'Cancel this extra queued task',status:'todo',requirementIds:['AC-work'],writePaths:['cancel-a-result.json'],acceptance:'This entry is intentionally canceled before allocation.'});
 const oldPlan=amend?await api('GET',projectRoute+'/plan'):{revision:0};const plan=await api('POST',projectRoute+'/plan',{expectedRevision:oldPlan.revision,expectedSpecificationRevision:spec.revision,tasks});
 const gitReview=await api('GET',projectRoute+'/git');await api('POST',projectRoute+'/git/commit',{message:amend?'Fixture setup: reviewed visible-total amendment':'Fixture setup: reviewed mixed workload and independent tests',expectedReviewDigest:gitReview.reviewDigest});
 const branches=(await api('GET','/v1/projects/'+owner.id+'/branches')).branches,branch=branches.find(row=>row.name==='main')??branches[0];assert(branch);const worktrees=(await api('GET','/v1/projects/'+owner.id+'/worktrees?branchId='+encodeURIComponent(branch.id))).worktrees,worktree=worktrees.find(row=>row.isPrimary)??worktrees[0];assert(worktree);
 const selection={projectId:owner.id,briefRevision:spec.briefRevision,specificationRevision:spec.revision,designPlanRevision:design.revision,planRevision:plan.revision,taskIds:tasks.map(item=>item.id)};
 const saved=(await api('POST',projectRoute+'/runs',{worktreeId:worktree.id,harness:'codex',model:'swarm-mechanics-fixture',permissionMode:'bypass',agentConnectionId:connectionID,outcome:'preview',implementationContext:selection,idempotencyKey:'sustained-'+task},201)).run;assert.equal(saved.input.route.permissionMode,'bypass');
 return {owner,route:projectRoute,run:saved,task,workload,goal,initialHead:execFileSync('git',['rev-parse','HEAD'],{cwd:owner.path,encoding:'utf8'}).trim(),canonicalBefore:canonical(),canonicalLuci:readFileSync(join(owner.path,'LUCI.md'))};
}
async function functionalDrafts(){
 stage='unsent-draft-isolation';await page.getByRole('button',{name:'All projects',exact:true}).click();
 for(const owner of projectStates){await page.getByRole('button',{name:'Open project '+owner.owner.name,exact:true}).click();await page.getByRole('button',{name:'Define',exact:true}).click();await page.getByRole('textbox',{name:'The idea',exact:true}).fill(owner.goal+' UNSENT '+owner.owner.id);await page.getByRole('button',{name:'All projects',exact:true}).click();const dialog=page.getByRole('dialog',{name:'Keep your changes?',exact:true});await dialog.getByRole('button',{name:'Keep drafts and continue',exact:true}).click();}
 for(const owner of projectStates){await page.getByRole('button',{name:'Open project '+owner.owner.name,exact:true}).click();await page.getByRole('button',{name:'Define',exact:true}).click();await page.getByRole('button',{name:'Restore draft',exact:true}).click();assert.equal(await page.getByRole('textbox',{name:'The idea',exact:true}).inputValue(),owner.goal+' UNSENT '+owner.owner.id);await page.getByRole('textbox',{name:'The idea',exact:true}).fill(owner.goal);await page.getByRole('button',{name:'All projects',exact:true}).click();if(await page.getByRole('dialog',{name:'Keep your changes?',exact:true}).isVisible())await page.getByRole('button',{name:'Discard edits and continue',exact:true}).click();}
 for(const owner of projectStates){project=owner.owner;assert.deepEqual(canonical(),owner.canonicalBefore);}check('Four distinct unsent idea drafts restore to their own project without changing approved source context');
}
async function queueControls(first){
 stage='loaded-queue-controls';await openProject(first);const queueRegion=page.getByRole('region',{name:'Background task queue',exact:true});const cancelRow=queueRegion.locator('li').filter({hasText:'Cancel this extra queued task'}),entry=first.queued.find(value=>value.taskId==='cancel-a');assert(entry);const before=effects();await action(first.route+'/swarm-queue/'+entry.id+'/cancel',()=>cancelRow.getByRole('button',{name:'Cancel queued task',exact:true}).click());
 const queue=await api('GET',first.route+'/swarm-queue'),canceled=queue.items.find(value=>value.id===entry.id);assert.equal(canceled.state,'cancelled');assert(!canceled.childId&&!canceled.operationId);assert.deepEqual(effects(),before);assert(!privateRun(first).operations.some(value=>value.taskId==='cancel-a'));
 const next=projectStates[1];await openProject(next);await action(next.route+'/swarm-queue/'+next.queued[0].id+'/priority',()=>page.getByLabel('Priority for '+titles.get(next.task),{exact:true}).selectOption('high'));const latest=await api('GET',next.route+'/swarm-queue');assert.equal(latest.items.find(value=>value.id===next.queued[0].id).priority,'high');check('Browser cancellation prevents allocation and queued priority changes retain exact project ownership');
}
async function activeRestart(worker){
 stage='active-command-restart';const before=effects(),original=live(worker.session.id),beforeRows=taskEvents.get(worker.task)??[];assert(beforeRows.some(row=>row.type==='command-start'&&row.label==='fuzz'));assert(!beforeRows.some(row=>row.type==='command-end'&&row.label==='fuzz'));
 const began=new Date().toISOString();await passiveTiming('before-active-service-restart');await stopService(true);assert.equal(live(worker.session.id).pid,original.pid);await startService();token=(await api('POST','/v1/auth/login',{username:'swarmowner',password})).token;sensitive.push(token);assert.deepEqual(effects(),before);const after=live(worker.session.id);assert.equal(after.id,original.id);assert.equal(after.pid,original.pid);assert.deepEqual(bundleSnapshot(worker.session.id),initialBundles.get(worker.session.id));await attach(worker.session.id);await openProject();await interactions.warm();(receipt.activeRestarts??=[]).push({task:worker.task,startedAt:began,finishedAt:new Date().toISOString(),sameContainer:true,samePID:true,sameContext:true,oneCommandDispatch:worker.codingDispatches===1});check('Service restart preserves a real active fuzz command and exact worker/context without replay');
}
async function exercise(worker,{restart=false}={}){
 let restarted=false;const start=Date.now();while(!observations.has(worker.task)){health();assert(Date.now()-start<540000,'One actual workload exceeded9minutes');const events=taskEvents.get(worker.task)??[];
  if(restart&&!restarted&&events.some(row=>row.type==='command-start'&&row.label==='fuzz')){await activeRestart(worker);restarted=true;}
  await interactions.round(interactionRound++);await delay(2000);
 }
 if(restart)assert(restarted);assert.equal(execFileSync('git',['rev-parse','HEAD'],{cwd:worker.owner.path,encoding:'utf8'}).trim(),projectStates.find(value=>value.task===worker.task).initialHead,'Primary source is unchanged before reviewed integration');assert.deepEqual(bundleSnapshot(worker.session.id),initialBundles.get(worker.session.id));
}
async function reviewOwner(owner){await openProject(owner);run=(await action(owner.route+'/runs/'+owner.run.id+'/swarm/reconcile',()=>page.getByRole('button',{name:'Check team status',exact:true}).click())).run;await integrate(owner.task);owner.run=run;assert.equal(sha(readFileSync(join(owner.owner.path,owner.task+'-result.json'))),observations.get(owner.task).outputSHA256);assert.equal(readFileSync(join(owner.owner.path,'engine.go'),'utf8'),owner.workload.implementation);assert.deepEqual(canonical(),owner.canonicalBefore);}
let engineID;
try {
 deadlineTimer=setTimeout(()=>stopOnFailure(Error('Thirty minute acceptance deadline expired')),30*60_000);
 reserveTimer=setInterval(()=>{try{const free=rootFree();(receipt.diskSamples??=[]).push({stage,at:new Date().toISOString(),rootFreeBytes:free});if(free<1024**3)stopOnFailure(Error('Hard one GiB reserve breached'));for(const worker of taskData.values())if(worker.finalReleased&&worker.session)observeTaskLease(worker);}catch(error){stopOnFailure(error);}},1000);
 for(const name of ['SIGTERM','SIGINT'])process.on(name,()=>{signalFailure=name;stopOnFailure(Error('Acceptance interrupted by '+name));});
 receipt.bounds={scenarioDeadlineMs:1800000,maxMeasuredActions:6000,actualFuzzSecondsPerInitialWorker:150,upstreamRequestMs:120000,maxCreates:5,maxStarts:5,preCreateFreeBytes:6*1024**3,runningReserveBytes:1024**3,workerMemoryBytes:1024**3,workerNanoCPUs:1000000000};
 receipt.predeclaredTimingReason='Before installed execution, standalone offline cold smoke took584s for four2s-fuzz tasks; uniform-trimpath B recheck reduced one workload by about66s. Replacing four2s windows with150s estimates1110s initial command work before idle120s, amendment, UI and Docker/review overhead. Root approved30minutes before any installed run; no padding or on-run extension.';
 engineID=docker('info','--format','{{.ID}}');assert(engineID.length>0&&engineID.length<=256);receipt.engineIdentitySHA256=sha(engineID);
 startProvider();await new Promise((done,reject)=>provider.once('error',reject).listen(0,bridge,done));providerBase='http://host.docker.internal:'+provider.address().port+'/v1';
 const port=await freePort(),preview=await freePort();base='http://127.0.0.1:'+port;
 const upstream=process.env.DOCKER_HOST||'unix:///var/run/docker.sock';assert(upstream.startsWith('unix://'));
 proxy=await startFencedDockerProxy({socketPath:join(fixture,'docker-proxy.sock'),upstreamSocketPath:upstream.slice(7),identifyCreate,beforeCreate:admission,maxCreates:receipt.bounds.maxCreates,maxCreatesByKind:{worker:5,fence:1},maxStarts:receipt.bounds.maxStarts,maxObservations:20000,upstreamTimeoutMs:120000,holdTimeoutMs:600000});
 writeFileSync(join(fixture,'docker/config.json'),JSON.stringify({cliPluginsExtraDirs:process.env.DOCKER_CONFIG?[join(process.env.DOCKER_CONFIG,'cli-plugins')]:[]}),{mode:0o600});
 env=Object.fromEntries(['PATH','LANG','LC_ALL','TZ'].filter(name=>process.env[name]).map(name=>[name,process.env[name]]));
 Object.assign(env,{HOME:join(fixture,'home'),DOCKER_CONFIG:join(fixture,'docker'),DOCKER_HOST:'unix://'+join(fixture,'docker-proxy.sock'),LUCI_DOCKER_HOST:'unix://'+join(fixture,'docker-proxy.sock'),LUCI_ADDR:'127.0.0.1:'+port,LUCI_STATE_DIR:join(fixture,'state'),LUCI_PROJECTS_DIR:join(fixture,'projects'),LUCI_WEB_DIR:web,LUCI_RUNTIME_DIR:join(dirname(binary),'runtime'),LUCI_SANDBOX_IMAGE:image,LUCI_BACKEND:'docker',LUCI_SESSION_MEMORY_MIB:'1024',LUCI_SESSION_CPU_MILLIS:'1000',LUCI_MANAGED_SHUTDOWN_TOKEN:lifecycle,LUCI_LOG_LEVEL:'error',LUCI_BROWSER_PREVIEW_ADDR:'127.0.0.1:'+preview,LUCI_BROWSER_PREVIEW_PORT:String(preview),GIT_CONFIG_GLOBAL:join(fixture,'home/.gitconfig'),GIT_CONFIG_NOSYSTEM:'1'});
 await startService();
 const bootstrap=readFileSync(join(fixture,'state/control-plane/bootstrap-token'),'utf8').trim();sensitive.push(bootstrap);
 const owner=await api('POST','/v1/auth/bootstrap',{bootstrapToken:bootstrap,username:'swarmowner',displayName:'Mixed Workload Fixture Owner',password},201);token=owner.token;sensitive.push(token);
 await api('POST','/v1/setup/git/identity',{name:'Swarm Fixture',email:'swarm@example.invalid'});
 const readiness=await api('POST','/v1/setup/check',{});for(const id of ['docker','sandbox-image'])assert.equal(readiness.checks.find(row=>row.id===id)?.status,'ready');
 const connections=await api('POST','/v1/setup/agent-connections',{action:'save',expectedRevision:0,name:'Local controlled mixed workload fixture',kind:'compatible-api',enabled:true,endpoints:{responses:providerBase},models:{codex:'swarm-mechanics-fixture'},apiKey:key});assert.equal(connections.connections.length,1);const connectionID=connections.connections[0].id;
 await setPaused(true);
 for(let index=0;index<4;index++){
  const task='build-'+'abcd'[index],workload=projectWorkload(index);titles.set(task,workload.label);
  (receipt.workloadSources??=[]).push({task,label:workload.label,files:Object.fromEntries(Object.entries(workload.files).map(([name,text])=>[name,{sha256:sha(text),bytes:Buffer.byteLength(text)}])),implementationSHA256:sha(workload.implementation),expectedOutputSHA256:sha(workload.expectedOutput)});
  const owner=await api('POST','/v1/product/projects',{name:'mixed-project-'+'abcd'[index]},201);assert(realpathSync(owner.path).startsWith(fixture+sep));seedFiles(owner,workload.files);projectStates.push(await saveContext(owner,workload,task,'Build '+workload.label+' with explicit verified behavior.',connectionID));
 }
 const browserEnv=Object.fromEntries(['PATH','LANG','LC_ALL','TZ'].filter(n=>process.env[n]).map(n=>[n,process.env[n]]));browserEnv.LUCI_SWARM_BROWSER_FIXTURE=fixture;
 browser=await chromium.launch({headless:true,executablePath:option('--chromium','/usr/bin/chromium'),env:browserEnv,args:['--no-proxy-server']});receipt.browserVersion=browser.version();
 const context=await browser.newContext({viewport:{width:1440,height:1080},serviceWorkers:'block'});await installBrowserTiming(context);
 await context.route('**/*',async route=>{const request=route.request(),url=new URL(request.url());if(url.origin!==base){stopOnFailure(Error('Browser external request refused'));await route.abort('blockedbyclient');return;}if(!['GET','HEAD','OPTIONS'].includes(request.method()))receipt.browserMutations.push({stage,method:request.method(),path:url.pathname});await route.continue();});
 page=await context.newPage();page.setDefaultTimeout(15000);page.on('pageerror',error=>{receipt.browserErrors.push(scrub(error));stopOnFailure(Error('Browser script error'));});
 for(const owner of projectStates){stage='prepare-and-queue-'+owner.task;await prepare(owner);await queueOwner(owner);}
 assert.equal(effects().create,0);await functionalDrafts();interactions=interactionRunner({page,owners:projectStates,receipt,assertHealthy:health,phase:()=>measuredPhase});await interactions.warm();
 stage='idle-baseline';await quietPhase('quiet-app-open-idle');const idle=phaseStart('idle-interactive-calibration');await interactions.calibrate(100);phaseEnd(idle);await passiveTiming('end-idle-calibration');
 stage='sustained-work';const loaded=phaseStart('loaded');await setPaused(false);
 let worker=await queuedChild(projectStates[0],1);await attach(worker.session.id);await queueControls(projectStates[0]);await openProject();await interactions.warm();
 const completedOwners=[];
 for(let index=0;index<4;index++){
  const owner=projectStates[index];stage='actual-work-'+owner.task;assert.equal(worker.task,owner.task);await exercise(worker,{restart:index===0});await completeNaturally(worker,owner);completedOwners.push(owner);
  let successor;
  if(index<3){successor=await queuedChild(projectStates[index+1],index+2);await attach(successor.session.id);const beforeReview=(await api('GET',owner.route+'/runs/'+owner.run.id)).run;assert.notEqual(beforeReview.swarm.tasks.find(value=>value.id===owner.task).integration?.state,'integrated');(receipt.preIntegrationProgress??=[]).push({completedTask:owner.task,nextTask:successor.task,nextSessionID:successor.session.id,observedAt:new Date().toISOString()});}
  await reviewOwner(owner);await openProject();await interactions.warm();worker=successor;
 }
 phaseEnd(loaded);assert(interactionRound>=200,'At least200 loaded project switches and400 inputs must be actually recorded');assert.equal(observations.size,4);
 const activeSeconds=[...observations.values()].map(value=>value.artifact.commands.find(command=>command.label==='fuzz')?.activeFuzzSeconds??0);assert(activeSeconds.every(seconds=>seconds>=150));assert(activeSeconds.reduce((a,b)=>a+b,0)>=600);receipt.actualFuzzSeconds=activeSeconds;await passiveTiming('end-four-project-loaded-phase');
 stage='reviewed-amendment';const prior=projectStates[0],originalWorker=taskData.get('build-a'),oldBundle=initialBundles.get(originalWorker.session.id);assert.deepEqual(bundleSnapshot(originalWorker.session.id),oldBundle);
 const originalWitness=readFileSync(join(originalWorker.checkout,'witness.txt'),'utf8'),newWorkload={...prior.workload,label:'Escaped UI renderer with visible total',implementation:prior.workload.amendedImplementation,expectedOutput:prior.workload.amendedExpectedOutput};seedFiles(prior.owner,prior.workload.amendedFiles);titles.set('amend-a',newWorkload.label);
 const amended=await saveContext(prior.owner,newWorkload,'amend-a','Build escaped UI output with an exact visible item total.',connectionID,true);projectStates[0]=amended;interactions.expected(amended,amended.goal);assert.notDeepEqual(amended.canonicalBefore,prior.canonicalBefore);assert.deepEqual(bundleSnapshot(originalWorker.session.id),oldBundle);assert.equal(readFileSync(join(originalWorker.checkout,'witness.txt'),'utf8'),originalWitness);
 await prepare(amended);await queueOwner(amended);const amendedWorker=await queuedChild(amended,5);await attach(amendedWorker.session.id);assert.notDeepEqual(hashes(initialBundles.get(amendedWorker.session.id)),hashes(oldBundle));assert.equal(readFileSync(join(amendedWorker.checkout,'witness.txt'),'utf8'),prior.workload.amendedFiles['witness.txt']);await openProject();await interactions.warm();const amendmentPhase=phaseStart('amendment');await exercise(amendedWorker);await completeNaturally(amendedWorker,amended);await reviewOwner(amended);phaseEnd(amendmentPhase);assert.deepEqual(bundleSnapshot(originalWorker.session.id),oldBundle);check('Reviewed amendment passes new real tests while original task context and source snapshot remain exact');
 await openProject();await interactions.warm();stage='settle';await quietPhase('quiet-app-open-settle');const settled=phaseStart('settled-interactive-calibration');await interactions.calibrate(100);phaseEnd(settled);await passiveTiming('final-calibration');
 receipt.interactionSummary=summarizeInteractions(receipt.interactions);receipt.latencyTargets={measurement:'Full browser automation All projects→open saved Define context→two frames; input command→expected field→two frames; not physical display/input latency',projectSwitchP95Ms:300,inputP95Ms:100,p99Ms:1000,minimumSamples:{loadedSwitches:200,loadedInputs:400,idleSwitches:100,idleInputs:200,settledSwitches:100,settledInputs:200}};
 const latencyFailures=[];for(const name of ['idle-interactive-calibration','loaded','settled-interactive-calibration'])for(const kind of ['project-switch','input']){const value=receipt.interactionSummary[name+':'+kind];assert(value);if(value.failed||value.p95Ms>(kind==='input'?100:300)||value.p99Ms>=1000)latencyFailures.push({phase:name,kind,...value});}receipt.latencyTargetsMet=latencyFailures.length===0;receipt.latencyFailures=latencyFailures;
 await openProject(amended);await page.screenshot({path:join(evidence,'amended-project-desktop.png'),fullPage:true});await page.setViewportSize({width:390,height:844});await page.getByRole('button',{name:'All projects',exact:true}).click();await page.getByRole('heading',{name:'Across your projects',exact:true}).waitFor();for(const owner of projectStates)await page.getByRole('button',{name:'Open project '+owner.owner.name,exact:true}).waitFor();assert(await timed(page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),15000,'Phone document geometry'),'Phone overview must not horizontally overflow');await page.screenshot({path:join(evidence,'four-project-phone.png'),fullPage:true});
 assert.equal(proxy.observations.counts.create,5);assert.equal(proxy.observations.counts.start,5);assert.equal(proxy.observations.dropped,0);assert.equal(proxy.observations.failures.length,0);assert.equal(observations.size,5);assert.equal(receipt.browserErrors.length,0);assert.equal(receipt.naturalCompletions.length,5);assert.equal(receipt.preIntegrationProgress.length,3);assert.equal(workerContainers().length,0);assert.equal(docker('info','--format','{{.ID}}'),engineID);
 check('Five actual native task commands across four projects finish naturally, with600seconds real fuzz work and browser-reviewed amendment');receipt.functionalPassed=true;assert(receipt.latencyTargetsMet,'Measured UI latency exceeded the declared candidate targets; retain actual timings without relabeling functional success as full acceptance');receipt.passed=true;
}catch(error){receipt.failedStage=stage;receipt.failure=redactError(error);process.exitCode=1;console.error(receipt.failure);if(page){try{await passiveTiming('failure');await page.screenshot({path:join(evidence,'failed-stage.png'),fullPage:true});}catch{}}}
finally {
 closing=true;clearTimeout(deadlineTimer);clearInterval(reserveTimer);for(const socket of sockets)socket.close();receipt.actualTaskResults=Object.fromEntries(observations);receipt.taskEvents=Object.fromEntries(taskEvents);receipt.interactionSummary=summarizeInteractions(receipt.interactions);
 receipt.taskChildren=[...taskData.values()].map(value=>({task:value.task,projectID:value.owner.id,operationID:value.operation.id,sessionID:value.session.id,container:value.container,bundle:initialBundles.get(value.session.id)}));receipt.terminalObservations=terminalProbes.map(({id,attachedStage,probe})=>({sessionID:id,attachedStage,...probe.snapshot()}));
 const cleanupFailure=error=>{receipt.cleanup.push(redactError(error));receipt.passed=false;process.exitCode=1;};
 for(const value of taskData.values()){try{const file=join(value.checkout,'.luci-mixed-failure-'+value.task+'.json');if(existsSync(file)){const info=lstatSync(file);assert(info.isFile()&&!info.isSymbolicLink()&&info.size<=60*1024**2);const target=join(evidence,value.task+'-private-failure.json');writeFileSync(target,readFileSync(file),{mode:0o600,flag:'wx'});(receipt.failureDiagnostics??=[]).push({task:value.task,available:true,bytes:info.size,sha256:sha(readFileSync(target)),private:true});}else if(!observations.has(value.task)){(receipt.failureDiagnostics??=[]).push({task:value.task,available:false,reason:'No settled native failure bundle exists at cleanup. An active command may be interrupted by this failed fixture; its temporary corpus cannot be claimed retained.'});}}catch(error){cleanupFailure(error);}}
 if(browser){try{await browser.close();}catch(error){cleanupFailure(error);}const end=Date.now()+5000;while(Date.now()<end&&ownedProcesses('browser').length)await delay(100);for(const identity of ownedProcesses('browser')){cleanupFailure(Error('Owned browser required forced cleanup'));if(stillOwned(identity)){try{process.kill(identity.pid,'SIGKILL');}catch(error){if(error.code!=='ESRCH')cleanupFailure(error);}}}}
 let stopped=false;try{stopped=await stopService(false);assert(stopped);assert(controllerIdentities.filter(Boolean).every(identity=>!stillOwned(identity)));}catch(error){cleanupFailure(error);}receipt.controllersStopped=stopped&&controllerIdentities.filter(Boolean).every(identity=>!stillOwned(identity));
 if(proxy){try{const cleanup=await proxy.close();assert(cleanup.drained&&cleanup.pendingRequests===0&&cleanup.activeHandlers===0);receipt.dockerRequestsDrained=true;}catch(error){cleanupFailure(error);}receipt.dockerTransportObservations=structuredClone(proxy.observations);if(proxy.observations.dropped||proxy.observations.failures.length)cleanupFailure(Error('Docker transport recorded lost, refused or uncertain observations'));}else{receipt.dockerRequestsDrained=true;receipt.dockerTransportNotStarted=true;}
 if(receipt.controllersStopped&&receipt.dockerRequestsDrained&&proxy){const created=proxy.observations.requests.filter(value=>value.method==='POST'&&/\/containers\/create$/.test(value.path)&&value.status===201&&/^[a-f0-9]{64}$/.test(value.id??''));for(const creation of created){try{const ids=docker('ps','-aq','--no-trunc').split(/\s+/).filter(Boolean);if(!ids.includes(creation.id))continue;const value=JSON.parse(docker('inspect',creation.id))[0];assert.equal(value.Id,creation.id);assert.equal(value.Name,'/'+creation.name);assert.equal(value.Config.Labels['luci.instance'],instanceID());assert.equal(value.Config.Labels['luci.session-id'],creation.labels['luci.session-id']);assert(ownedSessions.has(value.Config.Labels['luci.session-id']));assert.equal(value.Config.Labels['luci.managed'],'true');assert.equal(value.Image,imageInfo.Id);assert(value.Mounts.some(mount=>mount.Destination==='/home/dev/work'&&projectStates.some(owner=>realpathSync(mount.Source).startsWith(join(dirname(owner.owner.path),'.luci-swarms')+sep))));cleanupFailure(Error('Unexpected full worker remained; exact failed-fixture cleanup required'));if(value.State.Running)docker('stop','--time','5',value.Id);const current=JSON.parse(docker('inspect',value.Id))[0];assert.equal(current.Id,value.Id);assert.equal(current.State.Running,false);docker('rm',value.Id);}catch(error){cleanupFailure(error);}}}
 if(provider){provider.closeAllConnections();if(provider.listening)await new Promise(done=>provider.close(done));}
 try{assert(receipt.controllersStopped);assert.equal(receipt.dockerRequestsDrained,true);assert.equal(docker('ps','-aq'),'');assert.equal(docker('volume','ls','-q'),'');assert.equal(ownedProcesses().length,0);assert.equal(ownedProcesses('browser').length,0);assert.equal(JSON.parse(docker('image','inspect',image))[0].Id,imageInfo.Id);assert.equal(sha(readFileSync(binary)),provenance.binarySHA256);assert.equal(sha(readFileSync(join(web,'index.html'))),provenance.webIndexSHA256);assert.equal(docker('info','--format','{{.ID}}'),engineID);for(const name of driverNames)assert.equal(sha(readFileSync(new URL(name,import.meta.url))),sha(readFileSync(join(evidence,'accepted-harness',name))));for(const name of reusedHelpers)assert.equal(sha(readFileSync(join(repo,'scripts/acceptance',name))),sha(readFileSync(join(evidence,'accepted-harness',name.replaceAll('/','_')))));
  receipt.runtimeCleanup={containers:0,volumes:0,ownedServiceProcesses:0,ownedBrowserProcesses:0,protectedImagePackageAndHelpersUnchanged:true,rootFreeBytes:rootFree()};assert.equal(realpathSync(fixture),fixture);assert.equal(dirname(fixture),fixtureParent);assert(fixture.split(sep).at(-1).startsWith('luci-sustained-'));if(receipt.passed&&!fixtureError){rmSync(fixture,{recursive:true});receipt.fixtureRemoved=true;}
 }catch(error){cleanupFailure(error);}
 if(!receipt.fixtureRemoved)receipt.fixtureRetained={path:fixture,reason:'Failed or partial acceptance; private fixture credentials/source/context and bounded diagnostic files remain. Do not reuse as normal workspace.'};
 if(fixtureError){receipt.passed=false;process.exitCode=1;receipt.backgroundFailure=redactError(fixtureError);}receipt.providerCalls=providerCalls;receipt.auxiliaryCalls=auxiliaryCalls;receipt.actualTaskCount=observations.size;receipt.elapsedMs=Date.now()-started;receipt.signal=signalFailure??null;receipt.finishedAt=new Date().toISOString();writeFileSync(report,JSON.stringify(receipt,null,2)+'\n',{mode:0o600,flag:'wx'});console.log((receipt.passed?'PASS ':'FAIL ')+report);
}
