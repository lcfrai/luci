#!/usr/bin/env node
// Installed browser + actual Docker/Codex swarm mechanics; controlled provider, no model reasoning claim.
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
import { startDockerCreateProxy } from './docker-create-proxy.mjs';
import { chromium } from '../../../desktop/node_modules/playwright/index.mjs';
import { collectWorkerDiagnostics } from './worker-diagnostics.mjs';
import { createNativeTerminalProbe } from './native-terminal-diagnostics.mjs';

const repo = resolve(dirname(fileURLToPath(import.meta.url)), '../../..'), args = process.argv.slice(2);
const option = (name, fallback) => { const i = args.indexOf(name); if (i < 0) return fallback; assert(args[i + 1] && !args[i + 1].startsWith('--'), 'Missing value for ' + name); return args[i + 1]; };
if (args.includes('--help')) {
  console.log('PREPARED, NOT YET EXECUTED: installed four-project single-worker queue witness. Required --binary PATH --web PATH --provenance PATH --expected-source SHA --image TRUSTED_TAG --expected-image-id sha256:... --image-provenance VERIFIED_RECEIPT --report FRESH_PATH. Four serial real Docker/Codex tasks; explicit browser queue/review controls; deterministic provider, no model-quality claim. Six GiB before each create; one GiB hard running floor; no image pulls or cloud allocation.');
  process.exit(0);
}
assert(!args.includes('--capacity-only'), 'This driver requires all four serial project results');
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
const report = resolve(option('--report', 'artifacts/quality-swarm/installed-execution/attempt-' + Date.now() + '/result.json')), evidence = dirname(report);
if (existsSync(evidence)) assert.equal(readdirSync(evidence).length, 0, 'Use a fresh evidence directory; prior failures must remain');
mkdirSync(evidence, { recursive: true, mode: 0o700 });
writeFileSync(join(evidence, 'package-provenance.json'), provenanceBytes, { mode: 0o600, flag: 'wx' });
writeFileSync(join(evidence,'image-provenance.json'),readFileSync(resolve(imageEvidencePath)),{mode:0o600,flag:'wx'});
const driverNames = ['four-project-queue.mjs', 'docker-create-proxy.mjs', 'worker-diagnostics.mjs', 'native-terminal-diagnostics.mjs'];
mkdirSync(join(evidence, 'accepted-harness'), { mode: 0o700 });
for (const name of driverNames) writeFileSync(join(evidence, 'accepted-harness', name), readFileSync(new URL(name, import.meta.url)), { mode: 0o600, flag: 'wx' });
const fixtureParent = resolve(option('--fixture-root', 'artifacts/quality-swarm/private-fixtures')); mkdirSync(fixtureParent, {recursive:true,mode:0o700});
const fixture = mkdtempSync(join(fixtureParent,'luci-swarm-execution-'));
for (const folder of ['home', 'projects', 'state', 'docker']) mkdirSync(join(fixture, folder), { mode: 0o700 });
const key = 'fixture-only-' + randomUUID(), lifecycle = randomUUID() + randomUUID(), password = randomUUID() + randomUUID();
const sensitive = [key, lifecycle, password], scrub = value => sensitive.reduce((text, secret) => text.replaceAll(secret, '[redacted]'), String(value));
const receipt = {
  passed: false, mode:'four-project-single-worker-queue', startedAt: new Date().toISOString(), sourceCommit: provenance.sourceCommit, sourceDirty: provenance.sourceDirty,
  harnessCheckout: { commit: execFileSync('git', ['rev-parse', 'HEAD'], { cwd: repo, encoding: 'utf8' }).trim(), dirty: !!execFileSync('git', ['status', '--porcelain'], { cwd: repo, encoding: 'utf8' }).trim() },
  packageIntegrity, initialRootFreeBytes: initialRootFree,
  resourceScope: 'At most four owned real sandbox Create/Start pairs, serial across four projects. Zero existing containers and six GiB root admission before every create; hard one GiB running reserve. No image pulls, release or cloud allocation.',
  binarySHA256: sha(readFileSync(binary)), webIndexSHA256: sha(readFileSync(join(web, 'index.html'))), driverSHA256: sha(readFileSync(fileURLToPath(import.meta.url))),
  helperSHA256: Object.fromEntries(['docker-create-proxy.mjs', 'worker-diagnostics.mjs', 'native-terminal-diagnostics.mjs'].map(name => [name, sha(readFileSync(new URL(name, import.meta.url)))])),
  image: { reference: image, id: imageInfo.Id }, releaseImage: { reference: releaseImage, id: releaseImageInfo.Id },
  scope: 'Installed browser queue consent and overview across four projects, single-slot real Docker/Codex controlled command execution, isolated frozen context/output, cancellation before allocation and service restart without replay. No parallel speed claim.',
  limitations: ['Existing trusted Docker images; installation is not exercised.', 'Provider output is deliberately scripted; this proves actual CLI, Docker, task coordination and review mechanics. It does not measure model reasoning, production application quality, cloud deployment or Windows behavior.'],
  externalModelCalls: 0, cloudResourcesCreated: 0, setupScaffolding: ['API-seeded account/team, four approved contexts and unstarted reviewed runs', 'API-configured global one-worker policy and paused/resumed dispatch; these settings controls are fixture actions', 'Named local deterministic provider route', 'API-seeded explicit Bypass tasks; held scripted final replies exercise live recovery, followed by natural CLI exit; only failure cleanup may stop sessions'], checks: [], browserMutations: [], browserErrors: [], cleanup: [],
};

const check = (name, details = {}) => { receipt.checks.push({ name, passed: true, ...details }); console.log('PASS ' + name); };
const names = ['.luci/brief.json', '.luci/design-handoff.json', '.luci/design-handoff.md', '.luci/design.json', '.luci/plan.json', '.luci/specification.json', '.luci/specification.md', 'LUCI.md'];
// The private brief is generated from saved state; only seven members are checkout exports.
const canonicalNames = names.filter(name => name !== '.luci/brief.json');
const authorityRoot = '/home/dev/.luci-reviewed-context';
const sockets = new Set(), socketSessions = new Map(), ownedSessions = new Set(), bundles = new Map(), nativeHomes = new Map();
const terminalProbes = [];
let service, proxy, provider, base, env, token, team, workspace, project, projectRoute, active;
let stage = 'startup', fixtureError, reserveTimer, deadlineTimer, closing = false, signalFailure;
let providerCalls = 0, auxiliaryCalls = 0; const observations=new Map(), taskData=new Map(), calls=new Map(); let browser, page, run;
let providerBase; const initialBundles=new Map(), projectStates=[], titles=new Map();
const started = Date.now(), deadline = started + 20 * 60_000;
const remaining = () => { if (Date.now() >= deadline) throw Error('Twenty minute acceptance deadline expired'); return deadline - Date.now(); };
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
  const raw = await response.text(); let value; try { value = raw ? JSON.parse(raw) : {}; } catch { throw Error('Invalid JSON response: '+method+' '+path+' '+response.status); }
  assert.equal(response.status, expected, method+' '+path+': '+response.status+' '+scrub(value.message??value.error??''));
  return value;
}
async function until(label, fn, ms = 120_000) {
  const end = Date.now() + remainingBudget(ms);
  while (Date.now() < end) { if (fixtureError) throw fixtureError; if (service?.exitCode !== null) throw Error('Owned service exited unexpectedly'); const value=await fn(); if(value)return value; await delay(150); }
  throw Error(label+' did not complete within its bounded prerequisite window');
}
async function freePort() { const server=createServer(); await new Promise((done,reject)=>server.once('error',reject).listen(0,'127.0.0.1',done));const port=server.address().port;await new Promise(done=>server.close(done));return port; }
async function startService() {
  service=spawn(binary,[],{cwd:fixture,env,detached:true,stdio:['ignore','ignore','ignore']});service.on('error',stopOnFailure);
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
function registrySessions(){return JSON.parse(readFileSync(join(fixture,'state/control-plane/teams',team.id,'runtime/state/sessions/sessions.json'),'utf8')).sessions;}
function row(id){const rows=registrySessions().filter(value=>value.id===id);assert.equal(rows.length,1);return rows[0];}
function safeRow(value){return Object.fromEntries(['id','projectId','worktreeId','status','sandboxId','sandboxRef','conversationId','reviewedContext','implementationContext'].map(name=>[name,value[name]]));}
function containerIDs(id){assert.match(id,/^s-[a-f0-9]{8}$/);return docker('ps','-aq','--filter','label=luci.session-id='+id).split(/\s+/).filter(Boolean);}
function inspectOwned(id,sessionID,requireBundle=true){
  assert.match(id,/^[a-f0-9]{12,64}$/);assert(ownedSessions.has(sessionID));
  const value=JSON.parse(docker('inspect',id))[0];
  assert.equal(value.Config.Labels['luci.session-id'],sessionID);assert.equal(value.Config.Labels['luci.sandbox-id'],sessionID);assert.equal(value.Config.Labels['luci.managed'],'true');assert.equal(value.Image,imageInfo.Id);
  const instance=readFileSync(join(fixture,'state/control-plane/teams',team.id,'runtime/state/instance/instance-id'),'utf8').trim();assert.equal(value.Config.Labels['luci.instance'],instance);
  const work=value.Mounts.filter(m=>m.Destination==='/home/dev/work');assert.equal(work.length,1);assert.equal(work[0].RW,true);const expected=taskData.get(row(sessionID).implementationContext.taskIds[0]);assert(expected);assert.equal(realpathSync(work[0].Source),realpathSync(expected.checkout));assert.notEqual(realpathSync(work[0].Source),realpathSync(expected.owner.path));assert(lstatSync(join(work[0].Source,'.git')).isDirectory());assert(!existsSync(join(work[0].Source,'.git/commondir')));assert(!existsSync(join(work[0].Source,'.git/objects/info/alternates')));assert(!value.Mounts.some(m=>m.Source.includes('docker.sock')));
  assert(expected.requestedLimits.memoryMiB>0&&expected.requestedLimits.cpuMillis>0);assert.equal(value.HostConfig.Memory,expected.requestedLimits.memoryMiB*1024**2);assert.equal(value.HostConfig.NanoCpus,expected.requestedLimits.cpuMillis*1_000_000);assert.equal(value.HostConfig.PidsLimit,256);assert.equal(value.HostConfig.Privileged,false);assert(value.HostConfig.SecurityOpt.includes('no-new-privileges:true'));
  const proof={resourceLimits:{memoryBytes:value.HostConfig.Memory,nanoCPUs:value.HostConfig.NanoCpus,pidsLimit:value.HostConfig.PidsLimit,privileged:value.HostConfig.Privileged,securityOpt:value.HostConfig.SecurityOpt},worktreeID:row(sessionID).worktreeId,id:value.Id,image:value.Image,pid:value.State.Pid,running:value.State.Running,startedAt:value.State.StartedAt,created:value.Created,readOnly:true,sessionID};
  if(!requireBundle)return proof; // Cleanup authority uses labels/image/worktree, not the feature assertion under test.
  const native=value.Mounts.filter(m=>m.Destination==='/home/dev/.luci-agent');assert.equal(native.length,1);assert.equal(native[0].RW,true);const nativeRoot=realpathSync(native[0].Source);assert.equal(nativeRoot,resolve(native[0].Source));assert(nativeRoot.startsWith(join(fixture,'state/control-plane/teams',team.id)+sep));if(nativeHomes.has(sessionID))assert.equal(nativeHomes.get(sessionID),nativeRoot);nativeHomes.set(sessionID,nativeRoot);
  const mounts=value.Mounts.filter(m=>m.Destination===authorityRoot);assert.equal(mounts.length,1);const mount=mounts[0];assert.equal(mount.Type,'bind');assert.equal(mount.RW,false);
  const root=realpathSync(mount.Source);assert.equal(root,resolve(mount.Source));assert(root.startsWith(join(fixture,'state/control-plane/teams',team.id)+sep));assert(root.endsWith('/.reviewed-context/'+sessionID));assert(!root.startsWith(realpathSync(expected.owner.path)+sep));
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

function admission({ordinal,body}) {
 try {
  assert(ordinal>=1&&ordinal<=4);assert.equal(docker('ps','-aq'),'','Single-slot fixture must stop and remove the prior container before each create');
  const free=rootFree();assert(free>=6*1024**3,'Six GiB preallocation guard remains mandatory');
  const input=JSON.parse(body);assert([image,imageInfo.Id].includes(input.Image),'Only the verified candidate image may be allocated');
  receipt.resourceObservations??=[];receipt.resourceObservations.push({stage,ordinal,rootFreeBytes:free,requiredBytes:6*1024**3,at:new Date().toISOString()});
 } catch(error) {stopOnFailure(error);throw error;}
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
  assert.equal(req.url,'/v1/responses');const parts=[];let bytes=0;for await(const part of req){bytes+=part.length;assert(bytes<4*1024**2);parts.push(part);}const body=JSON.parse(Buffer.concat(parts));assert.equal(body.model,'swarm-mechanics-fixture');assert(++providerCalls<=100);
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
   const text=JSON.stringify(body.input);assert(text.includes('Reviewed context directory: '+authorityRoot));const matched=[...titles].filter(([task,title])=>text.includes(task+': '+title)).map(([task])=>task);assert.equal(matched.length,1,'Native opening prompt must identify exactly one assigned fixture task');const task=matched[0];await until('Controller verifies actual task bundle',()=>taskData.get(task)?.session&&initialBundles.has(taskData.get(task).session.id));const state=taskData.get(task);assert.equal(state.count,0,'No duplicate coding dispatch');response=nextTool(state,'exec_command',commandFor(task));
  }else{assert(++auxiliaryCalls<=20);response=message('Controlled swarm fixture acknowledgement.');}
  (receipt.providerRequests??=[]).push({number:providerCalls,toolOutputCount:resultItems.length,issuedTool:response.type==='function_call'?response.name:null});streamResponse(res,response,providerCalls);
 }catch(error){stopOnFailure(error);if(!res.headersSent)res.writeHead(500);res.end('Controlled swarm fixture contract failed');}});
}
async function action(path,click,expected=200){const pending=page.waitForResponse(r=>new URL(r.url()).pathname===path&&r.request().method()==='POST',{timeout:remainingBudget(150000)});await click();const response=await pending;assert.equal(response.status(),expected,path+': '+await response.text());return response.json();}
async function openTeam(state) {
 await page.goto(base);
 const username=page.getByLabel('Username',{exact:true}),enter=page.getByRole('button',{name:'Continue to projects',exact:true}),picker=page.getByLabel('Workspace',{exact:true});
 // goto completes before React has resolved the account state. Wait for an
 // actual rendered entry state before deciding whether sign-in is necessary.
 await until('Rendered account entry',async()=>await username.isVisible()||await enter.isVisible()||await picker.isVisible());
 if(await username.isVisible()){
  await username.fill('swarmowner');await page.getByLabel('Password',{exact:true}).fill(password);
  await action('/v1/auth/login',()=>page.getByRole('button',{name:'Sign in',exact:true}).click());
 }
 await until('Workspace entry',async()=>await enter.isVisible()||await picker.isVisible());if(await enter.isVisible())await enter.click();await picker.selectOption({label:team.name});
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
 assert.equal(effects().create,ordinal);assert.equal(effects().start,ordinal);assert.equal(docker('ps','-aq').split(/\s+/).filter(Boolean).length,1);
 return state;
}
async function acquireQueuedProject(ordinal,completed){
 const owner=await until('A distinct queued project receives the single slot',async()=>{
  for(const state of projectStates){const q=await api('GET',state.route+'/swarm-queue');const item=q.items.find(value=>value.state==='running'&&value.childId&&!completed.has(value.taskId));if(item){assert.equal(item.taskId,state.task,'Canceled extra task must never be allocated');return state;}}
  return null;
 },150000);
 const worker=await queuedChild(owner,ordinal);await setPaused(true);
 return {owner,worker};
}
async function verifyRenderedOverview(expectedRunning){
 const views=[];
 for(const owner of projectStates){
  const value=await api('GET',owner.route+'/overview');assert.equal(value.projectId,owner.owner.id);assert.equal(value.goal.text,owner.goal);
  const card=page.locator('li.overview-card').filter({has:page.getByRole('button',{name:'Open project '+owner.owner.name,exact:true})});
  await until('Rendered own-project goal and queue counts',async()=>{
   if(await card.locator('.overview-goal').innerText()!==owner.goal)return false;
   for(const [key,label] of [['running','Running'],['queued','Queued'],['blocked','Blocked']]){
    const pair=card.locator('dl.overview-counts > div').filter({has:page.locator('dt').filter({hasText:new RegExp('^'+label+'$')})});
    if(await pair.locator('dd').innerText()!==String(value.execution[key]))return false;
   }
   return true;
  });views.push({projectId:owner.owner.id,goal:value.goal.text,execution:value.execution});
 }
 assert.equal(views.reduce((total,view)=>total+view.execution.running,0),expectedRunning);
 (receipt.renderedOverviewChecks??=[]).push({stage,at:new Date().toISOString(),views});
}
async function setPaused(paused){
 const settings=await api('GET','/v1/product/swarm-scheduler');
 return api('PUT','/v1/product/swarm-scheduler',{expectedRevision:settings.revision,policy:{maxActive:1,maxPerProject:1,headroomPercent:25,minHeadroomMiB:2048,diskReserveMiB:1024},dispatchPaused:paused});
}

async function integrate(task){stage='ui-review-integrate-'+task;const reviewResponse=page.waitForResponse(r=>new URL(r.url()).pathname===projectRoute+'/runs/'+run.id+'/swarm/tasks/'+task+'/review'&&r.request().method()==='GET');await taskRow(task).getByRole('button',{name:'Review task changes',exact:true}).click();const response=await reviewResponse;assert.equal(response.status(),200);const {review}=await response.json();assert.equal(review.canIntegrate,true);assert.equal(review.sourceCommit,observations.get(task).commit);assert.deepEqual(review.paths,[task+'-result.txt']);const dialog=page.getByRole('dialog',{name:'Review task changes',exact:true});await dialog.getByLabel('Reviewed task diff').waitFor();assert((await dialog.getByLabel('Reviewed task diff').innerText()).includes(task+'-result.txt'));await dialog.getByRole('checkbox').check();const result=await action(projectRoute+'/runs/'+run.id+'/swarm/tasks/'+task+'/integrate',()=>dialog.getByRole('button',{name:'Integrate reviewed changes',exact:true}).click());run=result.run;assert.equal(run.swarm.tasks.find(t=>t.id===task).integration.state,'integrated');assert(existsSync(join(project.path,task+'-result.txt')));(receipt.integrations??=[]).push({task,review,commit:run.swarm.tasks.find(t=>t.id===task).integration.commit});}

try {
  deadlineTimer=setTimeout(()=>stopOnFailure(new Error('Twenty minute acceptance deadline expired')),20*60_000);
  reserveTimer=setInterval(()=>{try{const free=rootFree();(receipt.diskSamples??=[]).push({stage,at:new Date().toISOString(),rootFreeBytes:free});if(free<1024**3)stopOnFailure(new Error('Hard one GiB remaining reserve breached'));if(proxy&&(proxy.observations.counts.create>4||proxy.observations.counts.start>4))stopOnFailure(new Error('Four allocation request limit exceeded'));for(const worker of taskData.values())if(worker.finalReleased&&worker.session)observeTaskLease(worker);}catch(error){stopOnFailure(error);}},1000);
  for(const name of ['SIGTERM','SIGINT'])process.on(name,()=>{signalFailure=name;stopOnFailure(new Error('Acceptance interrupted by '+name));});
  startProvider();await new Promise((done,reject)=>provider.once('error',reject).listen(0,bridge,done));
  providerBase='http://host.docker.internal:'+provider.address().port+'/v1';
  const port=await freePort(),preview=await freePort();base='http://127.0.0.1:'+port;
  const upstream=process.env.DOCKER_HOST||'unix:///var/run/docker.sock';assert(upstream.startsWith('unix://'));
  proxy=await startDockerCreateProxy({socketPath:join(fixture,'docker-proxy.sock'),upstreamSocketPath:upstream.slice(7),upstreamTimeoutMs:120_000,maxObservations:20_000,maxCreates:4,maxStarts:4,beforeCreate:admission});proxy.releaseHeld();
  writeFileSync(join(fixture,'docker/config.json'),JSON.stringify({cliPluginsExtraDirs:process.env.DOCKER_CONFIG?[join(process.env.DOCKER_CONFIG,'cli-plugins')]:[]}),{mode:0o600});
  env=Object.fromEntries(['PATH','LANG','LC_ALL','TZ'].filter(name=>process.env[name]).map(name=>[name,process.env[name]]));
  Object.assign(env,{HOME:join(fixture,'home'),DOCKER_CONFIG:join(fixture,'docker'),DOCKER_HOST:'unix://'+join(fixture,'docker-proxy.sock'),LUCI_DOCKER_HOST:'unix://'+join(fixture,'docker-proxy.sock'),LUCI_ADDR:'127.0.0.1:'+port,LUCI_STATE_DIR:join(fixture,'state'),LUCI_PROJECTS_DIR:join(fixture,'projects'),LUCI_WEB_DIR:web,LUCI_RUNTIME_DIR:join(dirname(binary),'runtime'),LUCI_SANDBOX_IMAGE:image,LUCI_BACKEND:'docker',LUCI_MANAGED_SHUTDOWN_TOKEN:lifecycle,LUCI_LOG_LEVEL:'error',LUCI_BROWSER_PREVIEW_ADDR:'127.0.0.1:'+preview,LUCI_BROWSER_PREVIEW_PORT:String(preview),GIT_CONFIG_GLOBAL:join(fixture,'home/.gitconfig'),GIT_CONFIG_NOSYSTEM:'1'});
  await startService();
  const bootstrap=readFileSync(join(fixture,'state/control-plane/bootstrap-token'),'utf8').trim();sensitive.push(bootstrap);
  const owner=await api('POST','/v1/auth/bootstrap',{bootstrapToken:bootstrap,username:'swarmowner',displayName:'Swarm Fixture Owner',password},201);token=owner.token;sensitive.push(token);
  team=(await api('POST','/v1/teams',{name:'Swarm execution fixture'},201)).team;workspace='/v1/teams/'+team.id+'/workspace';
  await api('POST',workspace+'/v1/setup/git/identity',{name:'Swarm Fixture',email:'swarm@example.invalid'});
  const readiness=await api('POST',workspace+'/v1/setup/check',{});for(const id of ['docker','sandbox-image'])assert.equal(readiness.checks.find(row=>row.id===id)?.status,'ready');
  const connections=await api('POST',workspace+'/v1/setup/agent-connections',{action:'save',expectedRevision:0,name:'Local swarm execution fixture',kind:'compatible-api',enabled:true,endpoints:{responses:providerBase},models:{codex:'swarm-mechanics-fixture'},apiKey:key});assert.equal(connections.connections.length,1);const connectionID=connections.connections[0].id;
  await setPaused(true);
  for(let index=0;index<4;index++){
   const letter='abcd'[index],task='build-'+letter;titles.set(task,'Build project '+letter.toUpperCase()+' result');
  project=await api('POST',workspace+'/v1/product/projects',{name:'queue-project-'+letter},201);assert(realpathSync(project.path).startsWith(fixture+sep));projectRoute=workspace+'/v1/product/projects/'+project.id;
  await api('POST',projectRoute+'/brief',{revision:0,purpose:'Prove '+task+' uses its own project context and reviewed integration under a shared worker budget',audience:'Project owners',constraints:'Local labelled fixture; no cloud or model calls',criteria:['Only this project assigned file is changed and reviewed before integration']});
  let spec=await api('GET',projectRoute+'/specification');spec=await api('POST',projectRoute+'/specification',{expectedRevision:spec.revision,expectedBriefRevision:spec.briefRevision,specification:{...spec,requirements:[{id:'AC-notes',title:'Coordinate isolated task work with reviewed integration',priority:'must',acceptance:'Each task changes only its assigned file using its own captured project context; canceled work never allocates a worker.'}]}});
  spec=await api('POST',projectRoute+'/specification/approve',{revision:spec.revision,briefRevision:spec.briefRevision});
  let design=await api('GET',projectRoute+'/design-plan');design=await api('POST',projectRoute+'/design-plan',{expectedRevision:design.revision,expectedBriefRevision:spec.briefRevision,expectedSpecificationRevision:spec.revision,plan:{...design,screens:[{id:'notes',name:'Saved notes',route:'/',purpose:'Save and restore a note',states:['Empty','Saved','Error and retry'],widths:[390,1440],requirementIds:['AC-notes']}]}});
  design=await api('POST',projectRoute+'/design-plan/approve',{revision:design.revision});
  const tasks=[{id:task,title:titles.get(task),status:'todo',requirementIds:['AC-notes'],writePaths:[task+'-result.txt'],acceptance:'Read only this project frozen context, commit its exact controlled task output, and leave primary source unchanged until review.'}];
  if(index===0)tasks.push({id:'cancel-a',title:'Cancel this waiting work',status:'todo',requirementIds:['AC-notes'],writePaths:['cancel-a-result.txt'],acceptance:'Fixture cancellation must prevent this task from allocating a worker.'});
  const plan=await api('POST',projectRoute+'/plan',{expectedRevision:0,expectedSpecificationRevision:spec.revision,tasks});
  const gitReview=await api('GET',projectRoute+'/git');await api('POST',projectRoute+'/git/commit',{message:'Fixture seed: approved swarm task graph',expectedReviewDigest:gitReview.reviewDigest});
  const branches=(await api('GET',workspace+'/v1/projects/'+project.id+'/branches')).branches;const branch=branches.find(row=>row.name==='main')??branches[0];assert(branch);
  const worktrees=(await api('GET',workspace+'/v1/projects/'+project.id+'/worktrees?branchId='+encodeURIComponent(branch.id))).worktrees;const worktree=worktrees.find(row=>row.isPrimary)??worktrees[0];assert(worktree);
  const selection={projectId:project.id,briefRevision:spec.briefRevision,specificationRevision:spec.revision,designPlanRevision:design.revision,planRevision:plan.revision,taskIds:tasks.map(item=>item.id)};
  const runInput={worktreeId:worktree.id,harness:'codex',model:'swarm-mechanics-fixture',permissionMode:'bypass',agentConnectionId:connectionID,outcome:'preview',implementationContext:selection};
  if(index===0){
   const denied=(await api('POST',projectRoute+'/runs',{...runInput,permissionMode:'ask',idempotencyKey:'explicit-ask-refusal'},201)).run;
   const refusal=await api('POST',projectRoute+'/runs/'+denied.id+'/swarm',{expectedRevision:denied.revision,maxParallel:1},409);assert.equal(refusal.error,'task_permission_requires_interactive');
   const queueRefusal=await api('POST',projectRoute+'/runs/'+denied.id+'/swarm/tasks/'+task+'/queue',{expectedRevision:denied.revision,idempotencyKey:'unprepared-ask-queue-refusal',priority:'normal'},409);assert(queueRefusal.error);
   const retained=(await api('GET',projectRoute+'/runs/'+denied.id)).run;assert.equal(retained.input.route.permissionMode,'ask');assert(!retained.swarm);assert.equal(retained.operations.length,0);assert.equal(effects().create,0);assert.equal(effects().start,0);
   receipt.permissionRefusal={savedMode:'ask',prepareError:refusal.error,unpreparedQueueError:queueRefusal.error,noOperations:true,noBackendCreate:true,routeUnchanged:true};
   check('Ask permission is refused without preparation, allocation, or silent conversion to Bypass');
  }
  run=(await api('POST',projectRoute+'/runs',{...runInput,idempotencyKey:'create-installed-swarm-fixture'},201)).run;assert.equal(run.input.route.permissionMode,'bypass');
  const canonicalBefore=canonical(),initialHead=execFileSync('git',['rev-parse','HEAD'],{cwd:project.path,encoding:'utf8'}).trim();
   projectStates.push({owner:project,route:projectRoute,run,task,index,initialHead,canonicalBefore,canonicalLuci:readFileSync(join(project.path,'LUCI.md')),goal:'Prove '+task+' uses its own project context and reviewed integration under a shared worker budget'});
  }
  const browserEnv=Object.fromEntries(['PATH','LANG','LC_ALL','TZ'].filter(n=>process.env[n]).map(n=>[n,process.env[n]]));
  browserEnv.LUCI_SWARM_BROWSER_FIXTURE=fixture;
  browser=await chromium.launch({headless:true,executablePath:option('--chromium','/usr/bin/chromium'),env:browserEnv,args:['--no-proxy-server']});receipt.browserVersion=browser.version();
  const context=await browser.newContext({viewport:{width:1440,height:1080},serviceWorkers:'block'});
  await context.route('**/*',async route=>{const request=route.request(),url=new URL(request.url());if(url.origin!==base){stopOnFailure(Error('Browser external request refused'));await route.abort('blockedbyclient');return;}if(!['GET','HEAD','OPTIONS'].includes(request.method()))receipt.browserMutations.push({stage,method:request.method(),path:url.pathname});await route.continue();});
  page=await context.newPage();page.setDefaultTimeout(20000);page.on('pageerror',error=>{receipt.browserErrors.push(scrub(error));stopOnFailure(Error('Browser script error'));});await openTeam();

  for(const state of projectStates){
   stage='browser-prepare-'+state.task;await openTeam(state);
   await page.getByRole('button',{name:'Prepare agent team',exact:true}).click();
   const dialog=page.getByRole('dialog',{name:'Prepare agent team',exact:true});await dialog.getByRole('combobox').selectOption('1');assert.equal(await dialog.getByRole('combobox').inputValue(),'1');
   state.run=(await action(state.route+'/runs/'+state.run.id+'/swarm',()=>dialog.getByRole('button',{name:'Prepare team',exact:true}).click(),201)).run;run=state.run;
   await page.getByRole('button',{name:/^Queue reviewed tasks/}).click();
   const queued=await action(state.route+'/runs/'+state.run.id+'/swarm/queue',()=>page.getByRole('dialog',{name:'Queue reviewed work',exact:true}).getByRole('button',{name:'Authorize queued tasks',exact:true}).click(),201);
   state.queued=queued.items;assert.equal(queued.items.length,state.index===0?2:1);
  }
  assert.equal(effects().create,0);check('Browser explicitly authorizes five named tasks across four reviewed projects while workspace dispatch is paused');
  await openTeam();await page.getByRole('button',{name:'Refresh overview',exact:true}).click();
  for(const state of projectStates)await page.getByRole('button',{name:'Open project '+state.owner.name,exact:true}).waitFor();
  await verifyRenderedOverview(0);
  await page.screenshot({path:join(evidence,'four-projects-queued.png'),fullPage:true});
  const completed=new Set();let firstTask, cancelled, prefetched;
  for(let ordinal=1;ordinal<=4;ordinal++){
   stage='resume-single-slot-'+ordinal;
   let selected=prefetched;prefetched=undefined;
   if(!selected){await setPaused(false);selected=await acquireQueuedProject(ordinal,completed);}
   const {owner,worker}=selected;
   if(ordinal===1){firstTask=owner.task;assert.equal(owner.index,0,'First queued project expected to receive initial slot');}
   if(ordinal===2)assert.notEqual(owner.task,firstTask,'A second project must receive a turn before another ready task from the first');
   await attach(worker.session.id);await until('Actual Codex CLI committed '+owner.task,()=>observations.has(owner.task),150000);
   assert(!existsSync(join(owner.owner.path,owner.task+'-result.txt')));assert.equal(execFileSync('git',['rev-parse','HEAD'],{cwd:owner.owner.path,encoding:'utf8'}).trim(),owner.initialHead);
   assert.deepEqual(bundleSnapshot(worker.session.id),initialBundles.get(worker.session.id));
   for(const other of projectStates)if(other!==owner)assert(!existsSync(join(worker.checkout,other.task+'-result.txt')));
   check('Real controlled Codex task uses only its own frozen project context and isolated checkout',{task:owner.task,ordinal,containerID:worker.container.id});
   await openTeam();await page.getByRole('button',{name:'Refresh overview',exact:true}).click();
   const overview=await api('GET',owner.route+'/overview');assert.equal(overview.projectId,owner.owner.id);assert.equal(overview.execution.running,1);
   await verifyRenderedOverview(1);
   await page.screenshot({path:join(evidence,'running-project-'+ordinal+'.png'),fullPage:true});
   if(ordinal===1){
    stage='service-restart-owned-child';await delay(1000);const before=effects(),beforeExec=worker.codingDispatches;await stopService();assert.deepEqual(live(worker.session.id),worker.container);
    await startService();token=(await api('POST','/v1/auth/login',{username:'swarmowner',password})).token;sensitive.push(token);
    const recovered=await until('Queue recovers exact existing child',async()=>{const q=await api('GET',owner.route+'/swarm-queue');return q.items.find(item=>item.childId===worker.session.id&&item.state==='running');});
    assert.equal(recovered.operationId,worker.operation.id);assert.deepEqual(live(worker.session.id),worker.container);assert.deepEqual(bundleSnapshot(worker.session.id),initialBundles.get(worker.session.id));assert.deepEqual(effects(),before);assert.equal(worker.codingDispatches,beforeExec);await attach(worker.session.id);
    receipt.serviceRestart={sameChild:true,sameContainer:true,sameCapturedBytes:true,noNewAllocation:true,noDuplicateToolExecution:true,finalProviderTransportRetries:'Recorded per task after release; exact duplicate final-output payload only'};check('Actual service restart retains exact live child and context without another allocation or tool execution');
   }
   if(ordinal===2){
    stage='browser-cancel-waiting';await openTeam(projectStates[0]);
    const extra=page.locator('.swarm-run-queue-items > li').filter({has:page.getByText('Cancel this waiting work',{exact:true})});
    const item=(await api('GET',projectStates[0].route+'/swarm-queue')).items.find(item=>item.taskId==='cancel-a');assert.equal(item.state,'queued');
    await action(projectStates[0].route+'/swarm-queue/'+item.id+'/cancel',()=>extra.getByRole('button',{name:'Cancel queued task',exact:true}).click());
    cancelled=(await api('GET',projectStates[0].route+'/swarm-queue')).items.find(candidate=>candidate.id===item.id&&candidate.taskId==='cancel-a');assert.equal(cancelled.state,'cancelled');assert(!cancelled.childId&&!cancelled.operationId);
    const cancelledRun=(await api('GET',projectStates[0].route+'/runs/'+projectStates[0].run.id)).run;assert(!cancelledRun.operations.some(op=>op.taskId==='cancel-a'));assert.equal(effects().create,2);assert.equal(effects().start,2);
    check('Browser cancels extra waiting work before any recorded reservation or backend allocation');
   }
   stage='natural-completion-'+owner.task;await completeNaturally(worker,owner);
   if(ordinal<4){
    const unreviewed=(await api('GET',owner.route+'/runs/'+owner.run.id)).run;
    assert.notEqual(unreviewed.swarm.tasks.find(task=>task.id===owner.task).integration?.state,'integrated');assert(!existsSync(join(owner.owner.path,owner.task+'-result.txt')));
    await setPaused(false);prefetched=await acquireQueuedProject(ordinal+1,completed);assert.notEqual(prefetched.owner.owner.id,owner.owner.id);
    (receipt.automaticSlotRelease??=[]).push({completedTask:owner.task,completedSession:worker.session.id,nextTask:prefetched.owner.task,nextSession:prefetched.worker.session.id,priorTaskUnintegrated:true,noStopRequest:true});
    check('Natural task exit releases the shared slot before human integration',{completedTask:owner.task,nextTask:prefetched.owner.task});
   }
   stage='review-result-'+owner.task;await openTeam(owner);
   run=(await action(owner.route+'/runs/'+owner.run.id+'/swarm/reconcile',()=>page.getByRole('button',{name:'Check team status',exact:true}).click())).run;
   await integrate(owner.task);owner.run=run;completed.add(owner.task);
   assert.equal(sha(readFileSync(join(owner.owner.path,owner.task+'-result.txt'))),observations.get(owner.task).outputSHA256);
   assert.deepEqual(canonical(),owner.canonicalBefore);assert.equal(execFileSync('git',['status','--porcelain'],{cwd:owner.owner.path,encoding:'utf8'}).trim(),'');
   if(prefetched){assert.equal(docker('ps','-aq').split(/\s+/).filter(Boolean).length,1);assert(live(prefetched.worker.session.id));}else assert.equal(docker('ps','-aq'),'');
  }
  assert.equal(completed.size,4);assert.equal(receipt.naturalCompletions.length,4);assert.equal(receipt.automaticSlotRelease.length,3);assert(cancelled&&!cancelled.operationId);assert.equal(proxy.observations.counts.create,4);assert.equal(proxy.observations.counts.start,4);assert.equal(proxy.observations.dropped,0);assert.equal(receipt.browserErrors.length,0);
  await openTeam();await page.getByRole('button',{name:'Refresh overview',exact:true}).click();await page.screenshot({path:join(evidence,'four-projects-reviewed-desktop.png'),fullPage:true});
  await page.setViewportSize({width:390,height:844});await page.screenshot({path:join(evidence,'four-projects-reviewed-phone.png'),fullPage:true});
  receipt.actualTaskResults=Object.fromEntries(observations);receipt.taskChildren=[...taskData.values()].map(s=>({task:s.task,projectID:s.owner.id,operationID:s.operation.id,sessionID:s.session.id,container:s.container,bundle:initialBundles.get(s.session.id)}));receipt.cancelledQueueItem=cancelled;receipt.projectRuns=projectStates.map(s=>({projectID:s.owner.id,runID:s.run.id,state:s.run.state}));
  check('Four native one-shot tasks exit naturally and free shared capacity; each exact result is manually reviewed and integrated');receipt.passed=true;
}catch(error){receipt.failedStage=stage;receipt.failure=redactError(error);process.exitCode=1;console.error(receipt.failure);if(page){try{await page.screenshot({path:join(evidence,'failed-stage.png'),fullPage:true});}catch{}}}
finally{
  // Preserve partial work and context evidence even if a later stage fails.
  receipt.actualTaskResults=Object.fromEntries(observations);receipt.taskChildren=[...taskData.values()].map(s=>({task:s.task,projectID:s.owner.id,operationID:s.operation.id,sessionID:s.session.id,container:s.container,bundle:initialBundles.get(s.session.id)}));receipt.projectRuns=projectStates.map(s=>({projectID:s.owner.id,runID:s.run.id,state:s.run.state}));
  receipt.terminalObservations=terminalProbes.map(({id,attachedStage,probe})=>({sessionID:id,attachedStage,...probe.snapshot()}));
  closing=true;clearTimeout(deadlineTimer);clearInterval(reserveTimer);for(const socket of sockets)socket.close();if(browser){
   try{await browser.close();}catch(error){receipt.cleanup.push('Browser close failed: '+redactError(error));receipt.passed=false;process.exitCode=1;}
   const browserEnd=Date.now()+5000;while(Date.now()<browserEnd&&ownedProcesses('browser').length)await delay(100);
   const residual=ownedProcesses('browser');if(residual.length){receipt.cleanup.push('Owned browser processes required force termination; acceptance fails');receipt.passed=false;process.exitCode=1;for(const identity of residual)if(stillOwned(identity)){try{process.kill(identity.pid,'SIGKILL');}catch(error){if(error.code!=='ESRCH')receipt.cleanup.push(redactError(error));}}}
   receipt.browserCleanup={verifiedNoOwnedProcesses:ownedProcesses('browser').length===0,scope:'Separate inherited browser fixture marker and PID creation identity, distinct from service-family state marker'};
  }
  for(const id of ownedSessions)if(!receipt.passed&&service?.exitCode===null&&!fixtureError){try{await api('DELETE',workspace+'/v1/sessions/'+id,undefined,200);}catch(error){receipt.cleanup.push(redactError(error));receipt.passed=false;process.exitCode=1;}}
  let stopped=false;try{stopped=await stopService(false);}catch(error){receipt.cleanup.push(redactError(error));}
  if(!stopped){receipt.cleanup.push('Owned private writers remain; fixture retained');receipt.passed=false;process.exitCode=1;}
  const diagnostics=collectWorkerDiagnostics({fixture,teamID:team?.id,writersStopped:stopped});receipt.workerDiagnostics={...diagnostics};delete receipt.workerDiagnostics.document;
  if(diagnostics.status==='captured'){try{const bytes=JSON.stringify(diagnostics.document)+'\n';writeFileSync(join(evidence,'worker-diagnostics.json'),bytes,{mode:0o600,flag:'wx'});Object.assign(receipt.workerDiagnostics,{artifact:'worker-diagnostics.json',eventCount:diagnostics.document.events.length,sha256:sha(bytes)});}catch{receipt.workerDiagnostics={status:'refused',reason:'evidence-write-failed'};}}
  if(!proxy){receipt.dockerRequestsDrained=true;receipt.dockerTransportNotStarted='Proxy creation did not complete; no fixture request was forwarded';}
  if(proxy){try{await proxy.close();receipt.dockerRequestsDrained=true;}catch(error){receipt.cleanup.push(redactError(error));receipt.passed=false;process.exitCode=1;}receipt.dockerTransportObservations=structuredClone(proxy.observations);}
  if(stopped&&receipt.dockerRequestsDrained&&team&&project){
   try{for(const value of registrySessions())if(projectStates.some(state=>state.owner.id===value.projectId)){assert.match(value.id,/^s-[a-f0-9]{8}$/);ownedSessions.add(value.id);}}catch(error){if(error.code!=='ENOENT')receipt.cleanup.push(redactError(error));}
   for(const id of ownedSessions){let containers=[];try{containers=containerIDs(id);}catch(error){receipt.cleanup.push('Owned container enumeration failed: '+redactError(error));receipt.passed=false;process.exitCode=1;continue;}for(const container of containers){try{const value=JSON.parse(docker('inspect',container))[0];assert.equal(value.Config.Labels['luci.session-id'],id);assert.equal(value.Config.Labels['luci.managed'],'true');assert.equal(value.Image,imageInfo.Id);const instance=readFileSync(join(fixture,'state/control-plane/teams',team.id,'runtime/state/instance/instance-id'),'utf8').trim();assert.equal(value.Config.Labels['luci.instance'],instance);assert(value.Mounts.some(m=>m.Destination==='/home/dev/work'&&projectStates.some(state=>realpathSync(m.Source).startsWith(join(dirname(state.owner.path),'.luci-swarms')+sep))));if(receipt.passed){receipt.cleanup.push('Unexpected owned container required cleanup after claimed natural completion');receipt.passed=false;process.exitCode=1;}docker('rm','-f',value.Id);}catch(error){receipt.cleanup.push(redactError(error));receipt.passed=false;process.exitCode=1;}}}
  }
  if(provider){provider.closeAllConnections();if(provider.listening)await new Promise(done=>provider.close(done));}
  try{assert(stopped);assert.equal(receipt.dockerRequestsDrained,true);assert.equal(docker('ps','-aq'),'');assert.equal(docker('volume','ls','-q'),'');assert.equal(ownedProcesses().length,0);assert.equal(ownedProcesses('browser').length,0);assert.equal(JSON.parse(docker('image','inspect',image))[0].Id,imageInfo.Id);assert.equal(JSON.parse(docker('image','inspect',releaseImage))[0].Id,releaseImageInfo.Id);assert.equal(sha(readFileSync(binary)),provenance.binarySHA256);assert.equal(sha(readFileSync(join(web,'index.html'))),provenance.webIndexSHA256);
   receipt.runtimeCleanup={containers:0,volumes:0,ownedServiceProcesses:0,ownedBrowserProcesses:0,protectedImagesAndPackageUnchanged:true,rootFreeBytes:rootFree()};assert.equal(realpathSync(fixture),fixture);assert.equal(dirname(fixture),fixtureParent);assert(fixture.split(sep).at(-1).startsWith('luci-swarm-execution-'));if(receipt.passed&&!fixtureError){rmSync(fixture,{recursive:true});receipt.fixtureRemoved=true;}else{receipt.fixtureRetained={path:fixture,reason:'Failed acceptance: stopped private fixture retained for exact source/output/context diagnosis. Contains fixture credentials; do not publish raw state.'};}
  }catch(error){receipt.cleanup.push(redactError(error));receipt.passed=false;process.exitCode=1;}
  if(fixtureError){receipt.passed=false;process.exitCode=1;receipt.backgroundFailure=redactError(fixtureError);}receipt.providerCalls=providerCalls;receipt.auxiliaryCalls=auxiliaryCalls;receipt.actualTaskCount=observations.size;receipt.elapsedMs=Date.now()-started;receipt.signal=signalFailure??null;receipt.finishedAt=new Date().toISOString();writeFileSync(report,JSON.stringify(receipt,null,2)+'\n',{mode:0o600,flag:'wx'});console.log((receipt.passed?'PASS ':'FAIL ')+report);
}
