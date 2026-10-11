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
import { consumeMixedToolResult } from './receipt-transport.mjs';
import { openQueueControls } from './queue-controls.mjs';
import { projectWorkload } from './workloads.mjs';
import {contextFileNames, verifyTaskContext} from './task-context.mjs';
import {parseSourceBindingOptions,validateSourceBinding,validateHelperLocation} from './source-binding.mjs';
import { installBrowserTiming, timingSnapshot, interactionRunner, summarizeInteractions, createTimingLedger } from './interaction-metrics.mjs';

const here=dirname(fileURLToPath(import.meta.url)),repo=resolve(process.env.LUCI_ACCEPTANCE_REPO??'/workspace/luci'),args=process.argv.slice(2);
const helperRoot=resolve(here,'../frozen-helpers');
const option = (name, fallback) => { const i = args.indexOf(name); if (i < 0) return fallback; assert(args[i + 1] && !args[i + 1].startsWith('--'), 'Missing value for ' + name); return args[i + 1]; };
if(args.includes('--help')){console.log('CLOUD-032 diagnostic only: frozen installed prefix through idle100 switches/200 inputs, zero Create/Start hard guards. Original package/image/source/provenance arguments remain mandatory. No full sustained pass. Explicit controlled runtime window required.');process.exit(0);}
const PREPARATION_MS=60000,WORKFLOW_MS=210000,CLEANUP_MS=120000,DRIVER_TOTAL_MS=PREPARATION_MS+WORKFLOW_MS+CLEANUP_MS;
const driverStarted=Date.now(),preparationDeadline=driverStarted+PREPARATION_MS;
let commandDeadline=preparationDeadline,preparationActive=true;
const commandBudget=maximum=>{const left=commandDeadline-Date.now();if(left<=0)throw Object.assign(Error('Acceptance command deadline exceeded'),{code:preparationActive?'preparation-deadline-exceeded':'acceptance-command-deadline-exceeded'});return Math.min(maximum,left);};
const report = resolve(option('--report', 'artifacts/quality-swarm/sustained-mixed/attempt-' + Date.now() + '/result.json')), evidence = dirname(report);
if (existsSync(evidence)) assert.equal(readdirSync(evidence).length, 0, 'Use a fresh evidence directory; prior failures must remain');
mkdirSync(evidence, { recursive: true, mode: 0o700 });
const {createAcceptanceJournal,diagnosticError,journalProcessIdentity}=await import(join(helperRoot,'scripts/acceptance/launch-recovery/durable-acceptance-journal.mjs'));
const {createWorkflowDeadline,boundedAttempt,runBoundedCleanup,writeFailureCheckpoint,settleOwnedBrowser}=await import(join(helperRoot,'scripts/acceptance/launch-recovery/acceptance-lifecycle.mjs'));
const {createStartupScope,startAcceptanceServices,startAcceptanceBrowser}=await import(join(helperRoot,'scripts/acceptance/launch-recovery/acceptance-startup.mjs'));

const journal=createAcceptanceJournal(join(evidence,'diagnostic-journal.jsonl'));
journal.record('driver-start',{...journalProcessIdentity(),deadlineMs:DRIVER_TOTAL_MS,stage:'preparation'});
const preparationCheckpoint=error=>({version:1,passed:false,status:'preparation-failed',stage:'preparation',at:new Date().toISOString(),preparationDeadlineMs:PREPARATION_MS,scenarioDeadlineMs:WORKFLOW_MS,cleanupDeadlineMs:CLEANUP_MS,totalDeadlineMs:DRIVER_TOTAL_MS,driver:journalProcessIdentity(),failure:diagnosticError(error),externalResourcesStarted:false,originalReceiptComplete:false});
process.on('uncaughtExceptionMonitor',(error,origin)=>{try{journal.record('driver-fatal',{...diagnosticError(error),origin,stage:preparationActive?'preparation':'driver'});if(preparationActive)writeFailureCheckpoint(report,preparationCheckpoint(error));}catch{}});
process.on('exit',exitCode=>{try{journal.record('driver-exit',{exitCode});journal.close();}catch{}});
const preparationHardStop=setTimeout(()=>{const error=Object.assign(Error('Preparation deadline exceeded'),{code:'preparation-deadline-exceeded'});try{journal.record('driver-caught-failure',{stage:'preparation',...diagnosticError(error)});writeFailureCheckpoint(report,preparationCheckpoint(error));}catch{}process.exit(1);},Math.max(1,preparationDeadline-Date.now()));
for(const signal of ['SIGTERM','SIGINT'])process.on(signal,()=>{if(!preparationActive)return;const error=Object.assign(Error('Preparation interrupted'),{code:'preparation-interrupted'});try{journal.record('driver-signal',{stage:'preparation',signal});writeFailureCheckpoint(report,{...preparationCheckpoint(error),signal});}catch{}process.exit(1);});
const {chromium}=await import(join(repo,'desktop/node_modules/playwright/index.mjs'));
const {validateWorkerCreate}=await import(join(helperRoot,'scripts/acceptance/launch-recovery/worker-create-guard.mjs'));
const {startFencedDockerProxy}=await import(join(helperRoot,'scripts/acceptance/launch-recovery/fenced-docker-proxy.mjs'));
const {createNativeTerminalProbe}=await import(join(helperRoot,'scripts/acceptance/four-project-queue/native-terminal-diagnostics.mjs'));

const taskContextVersion=Number(option('--task-context-version','1')); assert(taskContextVersion===1||taskContextVersion===2,'Explicit trusted context version must be1or2');
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
// The source template belongs to the preserved installed attempt, while this
// generated external diagnostic has its own exact source binding and scope.
const externalBinding=JSON.parse(readFileSync(join(here,'external-source-binding.json'),'utf8'));
assert.equal(realpathSync(here),externalBinding.executionDirectory);
for(const [name,expected] of Object.entries(externalBinding.localSources)) assert.equal(sha(readFileSync(join(here,name))),expected,'External diagnostic source changed: '+name);
for(const [name,expected] of Object.entries(externalBinding.repositoryDependencies)) assert.equal(sha(readFileSync(join(helperRoot,name))),expected,'Captured repository dependency changed: '+name);
const sourceBindingOptions=parseSourceBindingOptions(args);
function verifySourceBinding(){
 assert.equal(sourceBindingOptions.expectedProduct,provenance.sourceCommit);
 assert.equal(sourceBindingOptions.expectedHelper,externalBinding.helperSourceCommit);
 assert.equal(provenance.sourceCommit,'577f7a0fdb039b7f875901a6a567d62fdfee88c8');assert.equal(provenance.sourceDirty,false);
 for(const [name,expected] of Object.entries(externalBinding.repositoryDependencies))assert.equal(sha(readFileSync(join(helperRoot,name))),expected);
 return {productSource:provenance.sourceCommit,frozenHelperSourceCommit:externalBinding.helperSourceCommit,frozenHelperBytesVerified:true,mutableCheckoutNotUsedForHelpers:true};
}
const harnessCheckout=verifySourceBinding();
for(const [path,expected] of Object.entries(imageEvidence.sourcesBefore??{})){
 assert(/^(sandboxd\/|images\/sandbox\/)/.test(path)&&!path.split('/').includes('..'));
 assert.equal(sha(execFileSync('git',['show',provenance.sourceCommit+':'+path],{cwd:repo,maxBuffer:32*1024**2,timeout:Math.ceil(commandBudget(30000))})),expected,'Sandbox source differs from packaged source commit: '+path);
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
print(json.dumps({'archiveSHA256':digest.hexdigest(),'verifiedMembers':count,'allMembersMatch':True}))`, provenance.archive, provenance.archiveSHA256, dirname(binary)], { encoding: 'utf8', timeout: Math.ceil(commandBudget(60000)), maxBuffer: 1024 * 1024 }));
assert.equal(realpathSync(web), realpathSync(join(dirname(binary), 'web')));
const rootFree = () => { const volume = statfsSync('/'); return Number(volume.bavail) * Number(volume.bsize); };
const initialRootFree = rootFree(); assert(initialRootFree >= 6 * 1024 ** 3, 'This fixture needs six GiB root free, including a conservative one GiB working reserve');
const docker = (...values) => execFileSync('docker', values, { encoding: 'utf8', timeout: Math.ceil(commandBudget(30_000)) }).trim();
assert.equal(docker('ps', '-aq'), '', 'Exclusive runtime admission requires zero existing containers');
assert.equal(docker('volume', 'ls', '-q'), '', 'Exclusive runtime admission requires zero existing volumes');
const imageInfo = JSON.parse(docker('image', 'inspect', image))[0], releaseImageInfo = JSON.parse(docker('image', 'inspect', releaseImage))[0];
assert.match(imageInfo.Id, /^sha256:[a-f0-9]{64}$/); assert.match(releaseImageInfo.Id, /^sha256:[a-f0-9]{64}$/);
assert.equal(imageInfo.Id, expectedImage);
assert.equal(releaseImageInfo.Id, expectedImage);
const bridge = docker('network', 'inspect', 'bridge', '--format', '{{(index .IPAM.Config 0).Gateway}}');
assert.match(bridge, /^(10\.|172\.(1[6-9]|2\d|3[01])\.|192\.168\.)/);
writeFileSync(join(evidence, 'package-provenance.json'), provenanceBytes, { mode: 0o600, flag: 'wx' });
writeFileSync(join(evidence,'image-provenance.json'),readFileSync(resolve(imageEvidencePath)),{mode:0o600,flag:'wx'});
const driverNames=['sustained.mjs','external-source-binding.json','task-command.mjs','receipt-transport.mjs','workloads.mjs','interaction-metrics.mjs','latency-diagnostics.mjs','task-context.mjs','source-binding.mjs','queue-controls.mjs','run-with-observer.py'];
const reusedHelpers=['four-project-queue/native-terminal-diagnostics.mjs','four-project-queue/sample-linux-owned.py','launch-recovery/fenced-docker-proxy.mjs','launch-recovery/worker-create-guard.mjs','launch-recovery/durable-acceptance-journal.mjs','launch-recovery/acceptance-lifecycle.mjs','launch-recovery/acceptance-startup.mjs'];
mkdirSync(join(evidence, 'accepted-harness'), { mode: 0o700 });
for (const name of driverNames) writeFileSync(join(evidence, 'accepted-harness', name), readFileSync(new URL(name, import.meta.url)), { mode: 0o600, flag: 'wx' });
for (const name of reusedHelpers) writeFileSync(join(evidence,'accepted-harness',name.replaceAll('/','_')),readFileSync(join(helperRoot,'scripts/acceptance',name)),{mode:0o600,flag:'wx'});
const fixtureParent = resolve(option('--fixture-root', 'artifacts/quality-swarm/private-fixtures')); mkdirSync(fixtureParent, {recursive:true,mode:0o700});
const fixture = mkdtempSync(join(fixtureParent,'luci-sustained-'));
for (const folder of ['home', 'projects', 'state', 'docker']) mkdirSync(join(fixture, folder), { mode: 0o700 });
const key = 'fixture-only-' + randomUUID(), lifecycle = randomUUID() + randomUUID(), password = randomUUID() + randomUUID();
const sensitive = [key, lifecycle, password], scrub = value => sensitive.reduce((text, secret) => text.replaceAll(secret, '[redacted]'), String(value));
const receipt = {
  passed: false, taskContextVersion, mode:'sustained-installed-scripted-mixed', startedAt: new Date().toISOString(), sourceCommit: provenance.sourceCommit, sourceDirty: provenance.sourceDirty,
  harnessCheckout,
  packageIntegrity, initialRootFreeBytes: initialRootFree,
  resourceScope: 'At most five real workers across four projects, one at a time; four initial150s fuzz phases and one short reviewed amendment. Worker handoff reports are intentionally not fabricated by this scripted workload; missing reports remain explicit and do not prove no questions. Zero existing containers and six GiB root admission before every Create, no recovery fences allowed; hard one GiB running reserve. No image pulls, release or cloud allocation.',
  binarySHA256: sha(readFileSync(binary)), webIndexSHA256: sha(readFileSync(join(web, 'index.html'))), driverSHA256: sha(readFileSync(fileURLToPath(import.meta.url))),
  helperSHA256: Object.fromEntries(driverNames.filter(name=>name!=='sustained.mjs').map(name => [name, sha(readFileSync(new URL(name, import.meta.url)))])),
  reusedHelperSHA256:Object.fromEntries(reusedHelpers.map(name=>['scripts/acceptance/'+name,sha(readFileSync(join(helperRoot,'scripts/acceptance',name)))])),
  image: { reference: image, id: imageInfo.Id }, releaseImage: { reference: releaseImage, id: releaseImageInfo.Id },
  scope: 'Installed mixed build/test commands, explicit idle/settle, action latency, cancellation, active-command restart and reviewed amendment. Scripted provider; no live-model or Windows claim.',
  limitations: ['Existing trusted Docker images; installation is not exercised.', 'Provider output is deliberately scripted; this proves actual CLI, Docker, task coordination and review mechanics. It does not measure model reasoning, production application quality, cloud deployment or Windows behavior.'],
  externalModelCalls: 0, cloudResourcesCreated: 0, setupScaffolding: ['API-seeded personal owner and four deterministic source/test/context fixtures','API-configured one-worker policy; browser queue/cancel/review consent','Reviewed amendment context/test fixture seeded via API/private owned file setup; no claim that an LLM authored the fixtures','Local scripted provider, no external model calls'], checks: [], browserMutations: [], browserErrors: [], cleanup: [],
};

Object.assign(receipt,{
 mode:'installed-four-project-idle-diagnostic',diagnosticOnly:true,fullSustainedAcceptance:false,
 scope:'Unmodified installed package binary AND packaged web, exact original four saved contexts and paused queues, original draft/warm setup and idle100 switches/200 inputs/100 resets. Full original idle sample only; no task execution or sustained completion claim.',
 resourceScope:'One owned browser and installed app. Docker readiness/image inventory reads are required. Explicit guards reject every Create/Start before forwarding. No full Linux resource sampler; browser diagnostic collection is identical to the static control.',
 limitations:[...receipt.limitations,'Diagnostic subset only; original sustained failure remains unchanged. No resource-cost subtraction.','No independent Linux observer in either prepared control arm; this is not a matched replay of the old observer load.'],
 externalSourceBinding:externalBinding,dependencyCheckout:receipt.harnessCheckout,
 diagnosticPreparedFrom:{kind:'current frozen repository template',sourceSHA256:externalBinding.templateSourceSHA256},
});
const check = (name, details = {}) => { receipt.checks.push({ name, passed: true, ...details }); console.log('PASS ' + name); };
const names=contextFileNames(taskContextVersion);
// The private brief is generated from saved state; only seven members are checkout exports.
const canonicalNames = names.filter(name => name !== '.luci/brief.json' && name !== '.luci/task-handoffs.json');
const authorityRoot = '/home/dev/.luci-reviewed-context';
const sockets = new Set(), socketSessions = new Map(), ownedSessions = new Set(), bundles = new Map(), nativeHomes = new Map();
const terminalProbes = [];
let service, proxy, provider, base, env, token, project, projectRoute;const workspace='';
const controllerIdentities=[];let measuredPhase='setup',interactions,interactionRound=0;const taskEvents=new Map();
let stage = 'startup', fixtureError, reserveTimer, closing = false, signalFailure,workflowControl,cleanupHardStop,cleanupDeadline=Infinity,checkpointWritten=false;
function setStage(value){health();stage=value;journal.record('driver-stage',{stage});}
let providerCalls = 0, auxiliaryCalls = 0; const observations=new Map(), taskData=new Map(), calls=new Map(); let browser, page, run;
const startupScope=createStartupScope({assertActive:assertWorkflowActive,onFailure:error=>{cleanupFailure(error);stopOnFailure(error);},onTransition:({name,state})=>journal.record('driver-stage',{stage:'startup-'+name+'-'+state})});
let providerBase; const initialBundles=new Map(), projectStates=[], titles=new Map();
commandBudget(1);preparationActive=false;clearTimeout(preparationHardStop);commandDeadline=Date.now()+WORKFLOW_MS;
journal.record('driver-stage',{stage:'workflow-start'});
receipt.phases=[];receipt.interactions=[];const timingLedger=createTimingLedger(receipt);const started = Date.now(), deadline = started + WORKFLOW_MS;
const remaining = () => { if (Date.now() >= deadline) throw Error('Idle diagnostic deadline expired'); return deadline - Date.now(); };
const remainingBudget = (ms = 120_000) => Math.max(1, Math.min(ms, remaining()));
const redactError = error => scrub(error?.stack ?? error).replaceAll(fixture, '[private-fixture]').slice(0, 8000);
function stopOnFailure(error) { if (!fixtureError) {fixtureError = error;try{journal.record('driver-background-failure',{stage,...diagnosticError(error)});}catch{}} workflowControl?.fail(error); for (const socket of sockets) {try{socket.close();}catch{}} if (service?.exitCode === null) service.kill('SIGTERM'); }
function processIdentity(pid) {
  try { const data = readFileSync(`/proc/${pid}/stat`, 'utf8'); const fields = data.slice(data.lastIndexOf(')') + 2).trim().split(/\s+/); return {pid:Number(pid),start:fields[19],group:Number(fields[2]),status:fields[0]}; }
  catch (error) { if (['ENOENT','ESRCH'].includes(error.code)) return null; throw error; }
}
function stillOwned(identity) { const now = processIdentity(identity.pid); return now && now.start === identity.start && now.status !== 'Z'; }
function captureOwnedBrowserIdentities(stage){
 const proof=receipt.browserProcessIdentities??={scope:'Observed fixture-owned browser identities at startup and immediately before cleanup only; not an exhaustive lifetime census. Zombie states are retained, not treated as absent.',snapshots:[],final:[]};
 proof.snapshots.push({stage,identities:ownedProcesses('browser')});
 writeFileSync(join(evidence,'owned-browser-identities.json'),JSON.stringify(proof,null,2)+'\n',{mode:0o600});
}
function finishOwnedBrowserIdentities(){
 const proof=receipt.browserProcessIdentities;if(!proof)return;
 const observed=new Map(proof.snapshots.flatMap(row=>row.identities).map(row=>[row.pid+':'+row.start,row]));
 proof.final=[...observed.values()].map(identity=>{const current=processIdentity(identity.pid);return {identity,current,classification:!current?'absent':current.start!==identity.start?'pid-reused':current.status==='Z'?'same-identity-zombie':'same-identity-present'};});
 proof.exhaustiveLifetimeCensus=false;
 writeFileSync(join(evidence,'owned-browser-identities.json'),JSON.stringify(proof,null,2)+'\n',{mode:0o600});
}

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
  const workflowRequest=!closing;
  if(workflowRequest)health();else if(!['GET','HEAD'].includes(method))throw Error('Workflow mutation refused during cleanup');
  const response = await fetch(base + path, {method,redirect:'error',signal:AbortSignal.timeout(closing ? timeout : remainingBudget(timeout)),headers:{Origin:base,...(token?{Authorization:'Bearer '+token}:{}),...(body===undefined?{}:{'Content-Type':'application/json'})},body:body===undefined?undefined:JSON.stringify(body)});
  (receipt.apiActions ??= []).push({stage,method,path,status:response.status,at:new Date().toISOString()});
  const raw = await boundedResponse(response,2*1024**2); let value; try { value = raw ? JSON.parse(raw) : {}; } catch { throw Error('Invalid JSON response: '+method+' '+path+' '+response.status); }
  assert.equal(response.status, expected, method+' '+path+': '+response.status+' '+scrub(value.message??value.error??''));
  if(workflowRequest)health();
  return value;
}
async function until(label, fn, ms = 120_000) {
  const end = Date.now() + remainingBudget(ms);
  while (Date.now() < end) { health();const value=await fn();health();if(value)return value;await delay(150); }
  throw Error(label+' did not complete within its bounded prerequisite window');
}
async function freePort() { const server=createServer(); await new Promise((done,reject)=>server.once('error',reject).listen(0,'127.0.0.1',done));const port=server.address().port;await new Promise(done=>server.close(done));return port; }
async function startService() {
  assertWorkflowActive();
  const child=spawn(binary,[],{cwd:fixture,env,detached:true,stdio:['ignore','ignore','ignore']});service=child;child.on('error',stopOnFailure);assert(Number.isSafeInteger(child.pid)&&child.pid>0,'Owned service process identity is unavailable');controllerIdentities.push(processIdentity(child.pid));journal.record('controller-start',journalProcessIdentity(child.pid));child.on('exit',(exitCode,signal)=>{try{journal.record('controller-exit',{pid:child.pid,exitCode,signal});}catch{}});
  const end=Date.now()+remainingBudget(45_000);let ready;
  while(Date.now()<end) { assertWorkflowActive();assert.equal(service.exitCode,null);try{const res=await fetch(base+'/v1/control-plane/status',{signal:AbortSignal.timeout(1500)});assertWorkflowActive();if(res.ok){ready=await res.json();assertWorkflowActive();break;}}catch{}assertWorkflowActive();await delay(150); }
  assertWorkflowActive();assert.equal(ready?.product,'Luci');assert.equal(ready?.version,'0.5.0');
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
  assertWorkflowActive();
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
  health();return socket;
}

function outputs(input,result=[]) {if(!input||typeof input!=='object')return result;if(input.type==='function_call_output')result.push(input);for(const v of Array.isArray(input)?input:Object.values(input))outputs(v,result);assert(result.length<=100);return result;}
function message(text){return {id:'msg_swarm_'+providerCalls,type:'message',status:'completed',role:'assistant',content:[{type:'output_text',text,annotations:[]}]};}
function commandFor(task){const value=taskData.get(task);assert(value);return mixedTaskCommand({task,operation:value.operation.id,projectId:value.owner.id,implementation:value.workload.implementation,expectedOutput:value.workload.expectedOutput,authorityRoot,contextNames:names,durationSeconds:task==='amend-a'?0:150});}
function nextTool(state,name,args){assertWorkflowActive();if(name==='exec_command'){state.codingDispatches=(state.codingDispatches??0)+1;assert.equal(state.codingDispatches,1,'No duplicate task command');}assert(++state.count<=48,'Bounded task tool request cap');const callID='call_swarm_'+state.task+'_'+state.operation.id+'_'+state.count;calls.set(callID,{state,consumed:false,tool:name,sessionId:name==='write_stdin'?args.session_id:null});return {id:'fc_'+callID,type:'function_call',status:'completed',name,call_id:callID,arguments:JSON.stringify(args)};}
function startProvider(){
 provider=httpServer(async(req,res)=>{try{
  assert.equal(req.method,'POST');assert.equal(req.headers.authorization,'Bearer '+key);if(closing){res.writeHead(503);res.end();return;}
  assert.equal(req.url,'/v1/responses');const parts=[];let bytes=0;for await(const part of req){bytes+=part.length;assert(bytes<4*1024**2);parts.push(part);}const body=JSON.parse(Buffer.concat(parts));assert.equal(body.model,'swarm-mechanics-fixture');assert(++providerCalls<=256);
  const resultItems=outputs(body.input),pending=resultItems.filter(v=>calls.has(v.call_id)&&!calls.get(v.call_id).consumed);assert(pending.length<=1,'Ambiguous unconsumed task output');let response;
  if(pending.length){const output=pending[0],entry=calls.get(output.call_id),state=entry.state;entry.consumed=true;const raw=typeof output.output==='string'?output.output:JSON.stringify(output.output);for(const match of raw.matchAll(/LUCI_MIXED_EVENT=([A-Za-z0-9+/=]+)/g)){const event=JSON.parse(Buffer.from(match[1],'base64'));assert.equal(event.task,state.task);assert.equal(event.operation,state.operation.id);const rows=taskEvents.get(state.task)??[];assert(rows.length<80);rows.push({...event,observedAt:new Date().toISOString()});taskEvents.set(state.task,rows);}
   const settlement=consumeMixedToolResult(state.receiptTransport??={},raw,{callId:output.call_id,tool:entry.tool,sessionId:entry.sessionId,task:state.task,operation:state.operation.id});
   if(settlement.kind==='complete'){assert(!observations.has(state.task));const actual=settlement.actual;assert.equal(actual.task,state.task);assert.equal(actual.operation,state.operation.id);assert.equal(actual.uid,process.getuid());assert.equal(actual.cwd,'/home/dev/work');assert.equal(actual.readOnly,true);assert.deepEqual(hashes(actual.files),hashes(initialBundles.get(state.session.id)));assert.deepEqual([...actual.changed].sort(),['engine.go',state.task+'-result.json'].sort());assert.equal(actual.artifact.implementationSHA256,sha(state.workload.implementation));assert.equal(actual.artifact.expectedOutputSHA256,sha(state.workload.expectedOutput));assert.equal(actual.artifact.commands.find(row=>row.label==='expected-red')?.code,1);assert(actual.artifact.commands.filter(row=>row.label!=='expected-red').every(row=>row.code===0&&!row.signal));if(state.task!=='amend-a')assert(actual.artifact.commands.find(row=>row.label==='fuzz')?.elapsedMs>=150000);assert.match(actual.commit,/^[a-f0-9]{40,64}$/);(receipt.taskReceiptSettlements??=[]).push({task:state.task,operation:state.operation.id,callId:output.call_id,commandId:state.receiptTransport.commandId??null,commandExitCode:settlement.commandExitCode,receiptSHA256:settlement.receiptSHA256,wireBytes:settlement.wireBytes,decodedBytes:settlement.decodedBytes});observations.set(state.task,actual);entry.finalTask=state.task;entry.finalOutputSHA256=sha(raw);state.finalResponseRequests=(state.finalResponseRequests??0)+1;response=message('Controlled task '+state.task+' committed. Human integration and complete build verification remain separate.');}
   else {response=nextTool(state,'write_stdin',{session_id:settlement.sessionId,chars:'',yield_time_ms:10000,max_output_tokens:16000});}
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
async function action(path,click,expected=200){health();const pending=page.waitForResponse(r=>new URL(r.url()).pathname===path&&r.request().method()==='POST',{timeout:remainingBudget(150000)});void pending.catch(()=>{});await click();health();const response=await pending;health();assert.equal(response.status(),expected,path+': '+await response.text());return response.json();}
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
// Cross-project fairness and admission may choose any remaining project;
// a priority change inside one project does not require it to run next.
// Select only a single running allocation from this explicit remaining set;
// no dispatch, cancellation or priority mutation is performed by observation.
async function failRequiredQueuedTarget(terminals,running){
 const {owner,queued}=terminals[0];
 // Only public identifiers, codes, timestamps and numeric observations belong
 // in this receipt. Never copy task titles, messages, contexts or raw errors.
 const fields=(value,keys)=>Object.fromEntries(keys.map(key=>[key,value?.[key]]).filter(([,value])=>value===null||typeof value==='boolean'||typeof value==='number'&&Number.isFinite(value)||typeof value==='string'&&/^[a-zA-Z0-9_.:+-]{1,128}$/.test(value)));
 const queueView=item=>fields(item,['id','projectId','runId','taskId','state','reasonCode','revision','priority','queuedAt','updatedAt','operationId','childId','memoryMiB','cpuMillis']);
 const startFailure=value=>fields(value,['stage','reasonCode']);
 const admissionEvidence=(value,failure)=>{
  if(!value||Array.isArray(value)||typeof value!=='object'||failure?.stage!=='admission')return null;
  // Go emits UTC RFC3339Nano. Preserve nanoseconds when checking the original
  // five-second decision window; today's time is unrelated to this history.
  const timestamp=value=>{
   if(typeof value!=='string')return null;
   const parts=/^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})(?:\.(\d{1,9}))?Z$/.exec(value);if(!parts)return null;
   const millis=Date.parse(parts[1]+'Z');if(!Number.isFinite(millis)||new Date(millis).toISOString().slice(0,19)!==parts[1])return null;
   const nanos=BigInt((parts[2]??'').padEnd(9,'0'));if(parts[1]==='0001-01-01T00:00:00'&&nanos===0n)return null;
   return BigInt(millis)*1000000n+nanos;
  };
  const observed=timestamp(value.observedAt),decided=timestamp(value.decidedAt);
  if(observed===null||decided===null||decided<observed||decided-observed>5000000000n)return null;
  if(!['observedPercent','maximumPercent'].every(key=>typeof value[key]==='number'&&Number.isFinite(value[key]))||!['observedBytes','requiredBytes'].every(key=>Number.isSafeInteger(value[key])&&value[key]>=0))return null;
  if(value.metric==='host_cpu_utilization_percent'){
   if(failure.reasonCode!=='cpu_pressure'||value.observedBytes!==0||value.requiredBytes!==0||value.maximumPercent<0||value.maximumPercent>100||value.observedPercent>100||value.observedPercent<=value.maximumPercent)return null;
  }else if(['host_available_memory','execution_total_memory'].includes(value.metric)){
   if(failure.reasonCode!=='memory_pressure'||value.observedPercent!==0||value.maximumPercent!==0||value.requiredBytes<=0||value.observedBytes>=value.requiredBytes)return null;
  }else return null;
  return fields(value,['metric','observedAt','decidedAt','observedPercent','maximumPercent','observedBytes','requiredBytes']);
 };
 const diagnostic={version:1,code:queued.state==='completed'?'required-queued-task-completed-before-capture':'required-queued-task-terminal',observedAt:new Date().toISOString(),observedDuringStage:stage,
  queue:queueView(queued),otherTerminalTargets:terminals.slice(1).map(value=>queueView(value.queued)),runningPeers:running.map(value=>queueView(value.queued)),
  resourceObservationBasis:'Later cached scheduler GET; non-atomic with queue, run, session and actual refusal. Exact admission operands are not retained by this scheduler snapshot.',
  admissionDecision:{status:'unavailable'},
  diagnosticReads:'pending',retryAttempted:false};
 receipt.queueRefusal=diagnostic;
 // The first durable record precedes every diagnostic await. The second is
 // additive, so interruption or a failed GET cannot erase the observed refusal.
 writeFailureCheckpoint(join(evidence,'required-queue-refusal.json'),diagnostic);
 setStage('required-queue-terminal-'+owner.task);
 const reads=[['run',owner.route+'/runs/'+owner.run.id],['scheduler','/v1/product/swarm-scheduler']];
 if(queued.childId)reads.push(['session',workspace+'/v1/sessions/'+encodeURIComponent(queued.childId)]);
 const observations=await Promise.allSettled(reads.map(([,path])=>api('GET',path,undefined,200,3000)));
 diagnostic.diagnosticReads={};
 for(let index=0;index<reads.length;index++){
  const [kind]=reads[index],observation=observations[index];
  if(observation.status!=='fulfilled'){diagnostic.diagnosticReads[kind]='unavailable';continue;}
  const value=observation.value;
  try{
   if(kind==='run'){
    const run=value.run;assert.equal(run.id,owner.run.id);assert.equal(run.projectId,owner.owner.id);
    diagnostic.run=fields(run,['id','projectId','revision','state','createdAt','updatedAt']);
    if(queued.operationId){
     assert(Array.isArray(run.operations));const matches=run.operations.filter(op=>op.id===queued.operationId);assert.equal(matches.length,1);
     const op=matches[0];assert.equal(op.taskId,owner.task);assert.equal(op.childId,queued.childId);
     diagnostic.operation={...fields(op,['id','taskId','childId','status','createdAt','updatedAt']),startFailure:startFailure(op.startFailure)};
    }
   }else if(kind==='session'){
    assert.equal(value.id,queued.childId);assert.equal(value.projectId,owner.owner.id);assert.equal(value.projectRunId,owner.run.id);assert.equal(value.projectRunOperationId,queued.operationId);
    diagnostic.session={...fields(value,['id','projectId','projectRunId','projectRunOperationId','status','createdAt','updatedAt']),startFailure:startFailure(value.startFailure)};
    const actual=value.status==='exited'?admissionEvidence(value.startAdmissionEvidence,value.startFailure):null;
    if(actual){diagnostic.session.startAdmissionEvidence=actual;diagnostic.admissionDecision={status:'retained',basis:'Actual final admission operands from the exact session; freshness is measured at decidedAt, not the later diagnostic GET.'};}
   }else{
    const resources=value.resources??{},host=resources.host??{},execution=resources.execution??{};
    diagnostic.scheduler={...fields(value,['revision','observedAt','dispatchPaused']),policy:fields(value.policy,['maxActive','maxPerProject','headroomPercent','minHeadroomMiB','diskReserveMiB'])};
    diagnostic.resources={host:{...fields(host,['observedAt','status']),cpu:fields(host.cpu,['logicalProcessors','effectiveMillis','utilizationPercent','pressureSomeAvg10']),memory:fields(host.memory,['effectiveTotalBytes','effectiveAvailableBytes']),disks:(Array.isArray(host.disks)?host.disks:[]).slice(0,16).map(disk=>fields(disk,['observedAt','role','totalBytes','availableBytes']))},execution:fields(execution,['observedAt','status','totalMemoryBytes','cpuMillis']),reservations:fields(resources.reservations,['observedAt','status','count','memoryMiB','cpuMillis'])};
   }
   diagnostic.diagnosticReads[kind]='observed';
  }catch{diagnostic.diagnosticReads[kind]='identity-or-shape-mismatch';}
 }
 writeFailureCheckpoint(join(evidence,'required-queue-refusal-details.json'),diagnostic);
 const outcome=queued.state==='completed'?'completed before receipt capture; no verified task result was accepted':'is '+queued.state+' ('+(diagnostic.queue.reasonCode??'reason-unavailable')+'); inspect the retained attempt before explicitly retrying';
 throw Object.assign(Error('Required queued task '+owner.task+' '+outcome),{code:diagnostic.code});
}
async function allocatedQueuedOwner(candidates){
 const owners=Array.isArray(candidates)?candidates:[candidates];
 assert(owners.length>=1&&owners.length<=4,'Bounded remaining project candidates required');
 assert.equal(new Set(owners.map(owner=>owner.owner.id)).size,owners.length,'Duplicate candidate project');
 assert.equal(new Set(owners.map(owner=>owner.task)).size,owners.length,'Duplicate candidate task');
 return until('Exactly one queued worker allocated',async()=>{
  const snapshots=await Promise.all(owners.map(async owner=>({owner,queue:await api('GET',owner.route+'/swarm-queue')})));
  const running=[],terminals=[];
  for(const {owner,queue} of snapshots){
   assert(Array.isArray(queue.items)&&queue.items.length<=8,'Bounded fixture queue observation required');
   for(const item of queue.items){
    assert.equal(item.projectId,owner.owner.id,'Observed queue belongs to another project');
    if(item.taskId==='cancel-a')assert(!item.childId&&!item.operationId,'Cancelled waiting fixture task must never allocate');
    if(item.taskId===owner.task&&['failed','cancelled','completed'].includes(item.state)){
     assert.equal(item.runId,owner.run.id,'Terminal queue belongs to another run');
     assert(owner.queued.some(expected=>expected.id===item.id&&expected.taskId===item.taskId),'Terminal queue has no exact fixture enqueue receipt');
     assert(!terminals.some(value=>value.owner===owner),'Duplicate terminal fixture task');
     terminals.push({owner,queued:item});
    }
    if(item.state!=='running')continue;
    assert(item.childId,'Running queue task requires exact child identity');
    assert.equal(item.taskId,owner.task,'Unexpected running fixture task');
    assert.equal(item.runId,owner.run.id,'Running queue belongs to another run');
    assert(owner.queued.some(expected=>expected.id===item.id&&expected.taskId===item.taskId),'Running queue has no exact fixture enqueue receipt');
    running.push({owner,queued:item});
   }
  }
  assert(running.length<=1,'One worker ceiling is enforced across candidate projects');
  if(terminals.length)await failRequiredQueuedTarget(terminals,running);
  return running[0]??null;
 },150000);
}
async function queuedChild(candidates,ordinal){
 const {owner,queued}=await allocatedQueuedOwner(candidates);
 const session=await api('GET',workspace+'/v1/sessions/'+queued.childId);ownedSessions.add(session.id);
 owner.run=(await api('GET',owner.route+'/runs/'+owner.run.id)).run;
 const operation=owner.run.operations.find(op=>op.taskId===owner.task&&op.childId===session.id);assert(operation);
 const worktrees=(await api('GET',workspace+'/v1/projects/'+owner.owner.id+'/worktrees')).worktrees,checkout=worktrees.find(w=>w.id===operation.worktreeId);assert(checkout);
 const state={task:owner.task,owner:owner.owner,workload:owner.workload,operation,session,checkout:checkout.path,count:0,requestedLimits:{memoryMiB:queued.memoryMiB,cpuMillis:queued.cpuMillis}};taskData.set(owner.task,state);
 const actual=live(session.id),bundle=bundleSnapshot(session.id);initialBundles.set(session.id,bundle);state.container=actual;
 verifyTaskContext({version:taskContextVersion,session:row(session.id),operation,run:owner.run,files:Object.fromEntries([...names,'manifest.json'].map(name=>[name,readFileSync(join(bundles.get(session.id),name))]))});
 if(taskContextVersion===2){const mount=JSON.parse(docker('inspect',actual.id))[0].Mounts.filter(value=>value.Destination==='/home/dev/.luci-task-output');assert.equal(mount.length,1);assert.equal(mount[0].RW,true);assert.equal(realpathSync(mount[0].Source),join(fixture,'state/conversations/local/.task-output',session.id));}
 for(const name of canonicalNames.filter(name=>name!=='LUCI.md'))assert.equal(bundle[name].sha256,owner.canonicalBefore[name],'Captured canonical export differs from its own project: '+name);
 const capturedRoot=bundles.get(session.id),capturedLuci=readFileSync(join(capturedRoot,'LUCI.md'));
 assert(capturedLuci.subarray(0,owner.canonicalLuci.length).equals(owner.canonicalLuci),'Task brief must retain exact own-project LUCI prefix');
 for(const name of ['.luci/brief.json','.luci/specification.json','.luci/design.json','.luci/plan.json'])assert.equal(JSON.parse(readFileSync(join(capturedRoot,name),'utf8')).projectId,owner.owner.id,'Mixed-project context: '+name);
 assert.equal(row(session.id).implementationContext.taskIds.length,1);assert.equal(row(session.id).projectId,owner.owner.id);
 assert.equal(workerContainers().length,1,'One worker ceiling is enforced');
 (receipt.allocationOrder??=[]).push({ordinal,task:owner.task,projectID:owner.owner.id,queueID:queued.id,sessionID:session.id,operationID:operation.id,observedAt:new Date().toISOString()});
 return state;
}
async function setPaused(paused){
 assert.equal(paused,true,'Idle diagnostic cannot enable queue dispatch');
 const settings=await api('GET','/v1/product/swarm-scheduler');
 return api('PUT','/v1/product/swarm-scheduler',{expectedRevision:settings.revision,policy:{maxActive:1,maxPerProject:1,headroomPercent:25,minHeadroomMiB:2048,diskReserveMiB:1024},dispatchPaused:paused});
}

async function integrate(task){setStage('ui-review-integrate-'+task);const reviewResponse=page.waitForResponse(r=>new URL(r.url()).pathname===projectRoute+'/runs/'+run.id+'/swarm/tasks/'+task+'/review'&&r.request().method()==='GET');void reviewResponse.catch(()=>{});await taskRow(task).getByRole('button',{name:'Review task changes',exact:true}).click();const response=await reviewResponse;assert.equal(response.status(),200);const {review}=await response.json();assert.equal(review.canIntegrate,true);assert.equal(review.sourceCommit,observations.get(task).commit);assert.deepEqual([...review.paths].sort(),['engine.go',task+'-result.json'].sort());const dialog=page.getByRole('dialog',{name:'Review task changes',exact:true});await dialog.getByLabel('Reviewed task diff').waitFor();assert((await dialog.getByLabel('Reviewed task diff').innerText()).includes(task+'-result.json'));await dialog.getByRole('checkbox').check();const result=await action(projectRoute+'/runs/'+run.id+'/swarm/tasks/'+task+'/integrate',()=>dialog.getByRole('button',{name:'Integrate reviewed changes',exact:true}).click());run=result.run;assert.equal(run.swarm.tasks.find(t=>t.id===task).integration.state,'integrated');assert(existsSync(join(project.path,task+'-result.json')));(receipt.integrations??=[]).push({task,review,commit:run.swarm.tasks.find(t=>t.id===task).integration.commit});}

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
 throw Object.assign(Error('Idle diagnostic refuses all Docker Create requests'),{code:'idle-diagnostic-create-forbidden'});
 health();
 const input=JSON.parse(bytes),sessionID=input.Labels?.['luci.session-id'];assert.match(sessionID??'',/^s-[a-f0-9]{8}$/);assert.equal(meta.name,'luci-'+sessionID);assert.equal(input.Labels?.['luci.managed'],'true');assert.equal(input.Labels?.['luci.launch-fence'],undefined);
 const owners=projectStates.filter(owner=>privateRun(owner).operations.some(op=>op.childId===sessionID));assert.equal(owners.length,1);const owner=owners[0],op=privateRun(owner).operations.find(value=>value.childId===sessionID);assert.equal(op.taskId,owner.task,'Only the five specified tasks can allocate');ownedSessions.add(sessionID);
 validateWorkerCreate(input,{fixture,projectID:owner.owner.id,runID:owner.run.id,operationID:op.id,session:row(sessionID),images:[image,imageInfo.Id],instance:instanceID(),uid:process.getuid(),gid:process.getgid(),taskContextVersion});return 'worker';
}
function admission({kind,ordinal,body,...meta}){
 try{health();assert.equal(kind,'worker');assert(ordinal>=1&&ordinal<=5);identifyCreate(body,meta);const free=rootFree();assert(free>=6*1024**3,'Six GiB pre-create guard remains mandatory');assert.equal(workerContainers().length,0,'No overlapping full worker permitted');(receipt.resourceObservations??=[]).push({at:new Date().toISOString(),stage,ordinal,rootFreeBytes:free});}catch(error){stopOnFailure(error);throw error;}
}
function assertWorkflowActive(){if(closing)throw Error('Workflow closed for cleanup');workflowControl?.assertActive();remaining();if(fixtureError)throw fixtureError;}
function health(){assertWorkflowActive();if(service&&service.exitCode!==null)throw Error('Owned service exited unexpectedly');}
async function timed(promise,ms,label){let timer;try{return await Promise.race([promise,new Promise((_,reject)=>{timer=setTimeout(()=>reject(Error(label+' deadline exceeded')),ms);})]);}finally{clearTimeout(timer);}}
function phaseStart(name){measuredPhase=name;const row={name,startedAt:new Date().toISOString(),monotonicStartMs:performance.now()};receipt.phases.push(row);return row;}
function phaseEnd(row){row.finishedAt=new Date().toISOString();row.elapsedMs=performance.now()-row.monotonicStartMs;delete row.monotonicStartMs;}
async function passiveTiming(reason){if(!page)return;const value=await timed(timingSnapshot(page),15000,'Passive browser timing snapshot');if(value)timingLedger.append(value,reason);}
async function quietPhase(name){const record=phaseStart(name);assert.equal(workerContainers().length,0);const before=effects();for(let elapsed=0;elapsed<60;elapsed++){health();await delay(1000);}assert.equal(workerContainers().length,0);assert.deepEqual(effects(),before);phaseEnd(record);assert(record.elapsedMs>=60000);}
async function prepare(owner){await openProject(owner);await page.getByRole('button',{name:'Prepare agent team',exact:true}).click();const dialog=page.getByRole('dialog',{name:'Prepare agent team',exact:true});await dialog.getByRole('combobox').selectOption('1');assert.equal(await dialog.getByRole('combobox').inputValue(),'1');owner.run=(await action(owner.route+'/runs/'+owner.run.id+'/swarm',()=>dialog.getByRole('button',{name:'Prepare team',exact:true}).click(),201)).run;}
async function queueOwner(owner){await openProject(owner);await page.getByRole('button',{name:/^Queue reviewed tasks/}).click();const result=await action(owner.route+'/runs/'+owner.run.id+'/swarm/queue',()=>page.getByRole('dialog',{name:'Queue reviewed work',exact:true}).getByRole('button',{name:'Authorize queued tasks',exact:true}).click(),201);assert.equal(result.items.length,owner.task==='build-a'?2:1);owner.queued=result.items;}
function seedFiles(owner,files){health();for(const [name,text] of Object.entries(files)){assert(['go.mod','engine.go','engine_test.go','cmd/app/main.go','witness.txt'].includes(name));const file=join(owner.path,name);mkdirSync(dirname(file),{recursive:true});assert(!existsSync(file)||lstatSync(file).isFile());writeFileSync(file,text,{mode:0o600});}}
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
 setStage('unsent-draft-isolation');await page.getByRole('button',{name:'All projects',exact:true}).click();
 for(const owner of projectStates){await page.getByRole('button',{name:'Open project '+owner.owner.name,exact:true}).click();await page.getByRole('button',{name:'Discover',exact:true}).click();await page.getByRole('textbox',{name:'The idea',exact:true}).fill(owner.goal+' UNSENT '+owner.owner.id);await page.getByRole('button',{name:'All projects',exact:true}).click();const dialog=page.getByRole('dialog',{name:'Keep your changes?',exact:true});await dialog.getByRole('button',{name:'Keep drafts and continue',exact:true}).click();}
 for(const owner of projectStates){await page.getByRole('button',{name:'Open project '+owner.owner.name,exact:true}).click();await page.getByRole('button',{name:'Discover',exact:true}).click();await page.getByRole('button',{name:'Restore draft',exact:true}).click();assert.equal(await page.getByRole('textbox',{name:'The idea',exact:true}).inputValue(),owner.goal+' UNSENT '+owner.owner.id);await page.getByRole('textbox',{name:'The idea',exact:true}).fill(owner.goal);await page.getByRole('button',{name:'All projects',exact:true}).click();if(await page.getByRole('dialog',{name:'Keep your changes?',exact:true}).isVisible())await page.getByRole('button',{name:'Discard edits and continue',exact:true}).click();}
 for(const owner of projectStates){project=owner.owner;assert.deepEqual(canonical(),owner.canonicalBefore);}check('Four distinct unsent idea drafts restore to their own project without changing approved source context');
}
function exactQueueControlEntry(owner,expected,queue){
 assert(expected,'Exact control enqueue receipt required');
 assert(Array.isArray(queue.items)&&queue.items.length<=8,'Bounded fixture queue observation required');
 for(const item of queue.items)assert.equal(item.projectId,owner.owner.id,'Queue controls require own-project entries');
 const matches=queue.items.filter(value=>value.id===expected.id);assert.equal(matches.length,1,'Exact unique control entry required');
 const current=matches[0];assert.equal(current.projectId,owner.owner.id);assert.equal(current.runId,owner.run.id);assert.equal(current.taskId,expected.taskId);
 assert(!current.childId&&!current.operationId,'Queue controls require an unallocated waiting entry');return current;
}
async function protectCancellation(){
 setStage('protect-exact-cancellation-entry');
 const owner=projectStates.find(value=>value.task==='build-a');assert(owner,'Exact cancellation project required');
 const entry=owner.queued.find(value=>value.taskId==='cancel-a');
 const pending=await api('GET',owner.route+'/swarm-queue');assert.equal(pending.dispatchPaused,true,'Protect cancellation before global dispatch resumes');
 assert.equal(exactQueueControlEntry(owner,entry,pending).state,'queued');
 await openProject(owner);const region=await openQueueControls(page,health),row=region.locator('li').filter({hasText:'Cancel this extra queued task'}),before=effects();
 await action(owner.route+'/swarm-queue/'+entry.id+'/pause',()=>row.getByRole('button',{name:'Pause queued task',exact:true}).click());
 const protectedQueue=await api('GET',owner.route+'/swarm-queue');assert.equal(protectedQueue.dispatchPaused,true);assert.equal(exactQueueControlEntry(owner,entry,protectedQueue).state,'paused');
 assert.deepEqual(effects(),before);assert(!privateRun(owner).operations.some(value=>value.taskId==='cancel-a'));
 receipt.queueCancellationGuard={projectID:owner.owner.id,runID:owner.run.id,queueID:entry.id,task:'cancel-a',state:'paused',observedAt:new Date().toISOString()};
}
async function queueControls(worker){
 setStage('loaded-queue-controls');
 const first=projectStates.find(owner=>owner.task==='build-a');assert(first,'Exact cancellation project required');
 const entry=first.queued.find(value=>value.taskId==='cancel-a');
 assert.equal(exactQueueControlEntry(first,entry,await api('GET',first.route+'/swarm-queue')).state,'paused');
 await openProject(first);const queueRegion=await openQueueControls(page,health);const cancelRow=queueRegion.locator('li').filter({hasText:'Cancel this extra queued task'});
 const before=effects();await action(first.route+'/swarm-queue/'+entry.id+'/cancel',()=>cancelRow.getByRole('button',{name:'Cancel queued task',exact:true}).click());
 assert.equal(exactQueueControlEntry(first,entry,await api('GET',first.route+'/swarm-queue')).state,'cancelled');
 assert.deepEqual(effects(),before);assert(!privateRun(first).operations.some(value=>value.taskId==='cancel-a'));
 // Observe a different waiting project for the UI priority action. The receipt
 // proves that exact mutation; it never predicts cross-project dispatch order.
 let next,nextEntry;
 for(const candidate of projectStates.filter(owner=>owner.task!==worker.task)){
  const expected=candidate.queued.find(value=>value.taskId===candidate.task),current=exactQueueControlEntry(candidate,expected,await api('GET',candidate.route+'/swarm-queue'));
  if(['queued','blocked'].includes(current.state)){next=candidate;nextEntry=expected;break;}
 }
 assert(next&&nextEntry,'A different waiting priority project is required');
 await openProject(next);const nextQueue=await openQueueControls(page,health);
 await action(next.route+'/swarm-queue/'+nextEntry.id+'/priority',()=>nextQueue.getByRole('combobox',{name:'Priority for '+titles.get(next.task),exact:true}).selectOption('high'));
 const prioritized=exactQueueControlEntry(next,nextEntry,await api('GET',next.route+'/swarm-queue'));assert(['queued','blocked'].includes(prioritized.state));assert.equal(prioritized.priority,'high');assert.deepEqual(effects(),before);
 receipt.queueControls={activeTask:worker.task,cancelled:{projectID:first.owner.id,runID:first.run.id,queueID:entry.id,task:'cancel-a'},prioritized:{projectID:next.owner.id,runID:next.run.id,queueID:nextEntry.id,task:next.task,priority:prioritized.priority},observedAt:new Date().toISOString()};
 check('Browser cancellation prevents allocation and queued priority changes retain exact project ownership');
}
async function activeRestart(worker){
 setStage('active-command-restart');const before=effects(),original=live(worker.session.id),beforeRows=taskEvents.get(worker.task)??[];assert(beforeRows.some(row=>row.type==='command-start'&&row.label==='fuzz'));assert(!beforeRows.some(row=>row.type==='command-end'&&row.label==='fuzz'));
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

function cleanupBudget(maximum){const remaining=cleanupDeadline-Date.now();if(remaining<=0)throw Object.assign(Error('Owned cleanup deadline exceeded'),{code:'cleanup-deadline-exceeded'});return Math.min(maximum,remaining);}
function cleanupFailure(error){receipt.cleanup.push(redactError(error));receipt.passed=false;process.exitCode=1;}
function failureCheckpoint(){return {version:1,passed:false,status:'cleanup-incomplete',stage,at:new Date().toISOString(),preparationDeadlineMs:PREPARATION_MS,scenarioDeadlineMs:WORKFLOW_MS,cleanupDeadlineMs:CLEANUP_MS,totalDeadlineMs:DRIVER_TOTAL_MS,driver:journalProcessIdentity(),controllers:controllerIdentities.filter(Boolean),signal:signalFailure??null,failure:diagnosticError(fixtureError),controllersStopped:false,dockerRequestsDrained:false,dockerMutationStateCertain:false,workflowOperationUnsettled:workflowControl?.failed===true,startupResources:startupScope.snapshot(),originalReceiptComplete:false};}
function persistFailureCheckpoint(){
 if(checkpointWritten)return;
 try{writeFailureCheckpoint(report+'.incomplete.json',failureCheckpoint());checkpointWritten=true;journal.record('driver-cleanup-start',{stage,deadlineMs:Math.max(0,cleanupDeadline-Date.now())});}catch(error){cleanupFailure(error);}
}
function beginCleanup(){
 if(closing)return;closing=true;workflowControl?.close();clearInterval(reserveTimer);
 cleanupDeadline=Math.min(Date.now()+CLEANUP_MS,driverStarted+DRIVER_TOTAL_MS);commandDeadline=cleanupDeadline;
 cleanupHardStop=setTimeout(()=>{try{journal.record('driver-background-failure',{stage,code:'cleanup-hard-stop'});}catch{}try{writeFailureCheckpoint(report,{...failureCheckpoint(),status:'cleanup-hard-stop',finishedAt:new Date().toISOString()});}catch{}process.exit(1);},Math.max(1,cleanupDeadline-Date.now()));
}
// BEGIN SUSTAINED STARTUP WIRING: exercised with controlled factories only.
async function startSustainedServices(){
 const upstream=process.env.DOCKER_HOST||'unix:///var/run/docker.sock';assert(upstream.startsWith('unix://'));
 return startAcceptanceServices({scope:startupScope,assertActive:assertWorkflowActive,
  startProvider:async()=>{startProvider();await new Promise((done,reject)=>provider.once('error',reject).listen(0,bridge,done));return provider;},
  adoptProvider:handle=>{provider=handle;},closeProvider:async handle=>{try{handle.closeAllConnections();}finally{if(handle.listening)await new Promise(done=>handle.close(done));}},
  freePort,startProxy:()=>startFencedDockerProxy({socketPath:join(fixture,'docker-proxy.sock'),upstreamSocketPath:upstream.slice(7),journal,identifyCreate,beforeCreate:()=>{throw Error('Idle diagnostic refuses Create');},beforeStart:()=>{throw Error('Idle diagnostic refuses Start');},maxCreates:receipt.bounds.maxCreates,maxCreatesByKind:{worker:1,fence:1},maxStarts:receipt.bounds.maxStarts,maxObservations:20000,upstreamTimeoutMs:120000,holdTimeoutMs:600000}),
  adoptProxy:handle=>{proxy=handle;},closeProxy:handle=>handle.close(),
  configure:({provider,port,preview})=>{assertWorkflowActive();providerBase='http://host.docker.internal:'+provider.address().port+'/v1';base='http://127.0.0.1:'+port;
 writeFileSync(join(fixture,'docker/config.json'),JSON.stringify({cliPluginsExtraDirs:process.env.DOCKER_CONFIG?[join(process.env.DOCKER_CONFIG,'cli-plugins')]:[]}),{mode:0o600});
 env=Object.fromEntries(['PATH','LANG','LC_ALL','TZ'].filter(name=>process.env[name]).map(name=>[name,process.env[name]]));
 Object.assign(env,{HOME:join(fixture,'home'),DOCKER_CONFIG:join(fixture,'docker'),DOCKER_HOST:'unix://'+join(fixture,'docker-proxy.sock'),LUCI_DOCKER_HOST:'unix://'+join(fixture,'docker-proxy.sock'),LUCI_ADDR:'127.0.0.1:'+port,LUCI_STATE_DIR:join(fixture,'state'),LUCI_PROJECTS_DIR:join(fixture,'projects'),LUCI_WEB_DIR:web,LUCI_RUNTIME_DIR:join(dirname(binary),'runtime'),LUCI_SANDBOX_IMAGE:image,LUCI_BACKEND:'docker',LUCI_SESSION_MEMORY_MIB:'1024',LUCI_SESSION_CPU_MILLIS:'1000',LUCI_MANAGED_SHUTDOWN_TOKEN:lifecycle,LUCI_LOG_LEVEL:'error',LUCI_BROWSER_PREVIEW_ADDR:'127.0.0.1:'+preview,LUCI_BROWSER_PREVIEW_PORT:String(preview),GIT_CONFIG_GLOBAL:join(fixture,'home/.gitconfig'),GIT_CONFIG_NOSYSTEM:'1'});
  },startService,
 });
}
// END SUSTAINED STARTUP WIRING
// BEGIN SUSTAINED CLEANUP WIRING: exercised with controlled resources only.
async function settleSustainedResources(){
 const phases=await runBoundedCleanup({checkpoint:persistFailureCheckpoint,onFailure:cleanupFailure,onPhase:(name,status)=>journal.record('driver-stage',{stage:'cleanup-'+name+'-'+status}),phases:[
  {name:'controllers',timeoutMs:30000,run:async()=>{const stopped=await stopService(false);assert(stopped);assert(controllerIdentities.filter(Boolean).every(identity=>!stillOwned(identity)));return {stopped:true};}},
  {name:'browser',timeoutMs:12000,run:async()=>{try{if(startupScope.has('browser'))await startupScope.close('browser',cleanupBudget(5000));}catch(error){cleanupFailure(error);}finally{await settleOwnedBrowser({browser:undefined,listOwned:()=>ownedProcesses('browser'),stillOwned,forceTerminate:identity=>{try{process.kill(identity.pid,'SIGKILL');}catch(error){if(error.code!=='ESRCH')throw error;}},onFailure:cleanupFailure,closeTimeoutMs:cleanupBudget(5000),settleTimeoutMs:cleanupBudget(5000)});}}},
  {name:'provider',timeoutMs:3000,run:()=>startupScope.has('provider')?startupScope.close('provider',cleanupBudget(2500)):undefined},
  {name:'proxy',timeoutMs:125000,run:async()=>{if(!startupScope.has('proxy'))return {drained:true,notStarted:true,mutationStateCertain:true};const result=await startupScope.close('proxy',cleanupBudget(124000));assert(proxy);assert(result.drained&&result.pendingRequests===0&&result.activeHandlers===0);assert.equal(proxy.observations.dropped,0);assert.equal(proxy.observations.failures.length,0,'Docker transport recorded lost, refused or uncertain observations');return {drained:true,mutationStateCertain:true};}},
 ]});
 // A late result from a timed-out phase cannot grant destructive authority.
 const completed=name=>phases.find(row=>row.name===name&&row.status==='completed')?.result;
 receipt.controllersStopped=completed('controllers')?.stopped===true;
 receipt.dockerRequestsDrained=completed('proxy')?.drained===true;
 receipt.dockerMutationStateCertain=completed('proxy')?.mutationStateCertain===true;
 receipt.dockerTransportNotStarted=completed('proxy')?.notStarted===true;
 return phases;
}
// END SUSTAINED CLEANUP WIRING
let engineID;
try {
 workflowControl=createWorkflowDeadline({timeoutMs:remaining(),onFailure:stopOnFailure});
 await workflowControl.run(async()=>{
 reserveTimer=setInterval(()=>{try{const free=rootFree();(receipt.diskSamples??=[]).push({stage,at:new Date().toISOString(),rootFreeBytes:free});if(free<1024**3)stopOnFailure(Error('Hard one GiB reserve breached'));for(const worker of taskData.values())if(worker.finalReleased&&worker.session)observeTaskLease(worker);}catch(error){stopOnFailure(error);}},1000);
 for(const name of ['SIGTERM','SIGINT'])process.on(name,()=>{signalFailure=name;try{journal.record('driver-signal',{stage,signal:name});}catch{}stopOnFailure(Error('Acceptance interrupted by '+name));});
 receipt.bounds={preparationDeadlineMs:PREPARATION_MS,scenarioDeadlineMs:WORKFLOW_MS,cleanupDeadlineMs:CLEANUP_MS,totalDeadlineMs:DRIVER_TOTAL_MS,maxMeasuredActions:432,primaryMeasuredActions:400,optionalTraceMeasuredActions:32,actualFuzzSeconds:0,allowedCreates:0,allowedStarts:0,upstreamRequestMs:120000,maxCreates:1,maxStarts:1,preCreateFreeBytes:6*1024**3,runningReserveBytes:1024**3,proxyStructuralBudgetNote:'Existing proxy requires positive numeric maxima; unconditional guards refuse Create and Start before forwarding.'};
 receipt.predeclaredTimingReason='Prepared idle subset:60s quiet,100 switches/200 inputs/100 resets;210s workflow,60s preparation and120s cleanup. No extension on failure.';
 engineID=docker('info','--format','{{.ID}}');assert(engineID.length>0&&engineID.length<=256);receipt.engineIdentitySHA256=sha(engineID);
 await startSustainedServices();
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
 // BEGIN SUSTAINED BROWSER WIRING: exercised with controlled factories only.
 await startAcceptanceBrowser({scope:startupScope,assertActive:assertWorkflowActive,
  launch:()=>chromium.launch({headless:true,executablePath:option('--chromium','/usr/bin/chromium'),env:browserEnv,args:['--no-proxy-server']}),adoptBrowser:handle=>{browser=handle;},closeBrowser:handle=>handle.close(),
  newContext:handle=>handle.newContext({viewport:{width:1440,height:1080},serviceWorkers:'block'}),
  installRoutes:async context=>{await installBrowserTiming(context);assertWorkflowActive();await context.route('**/*',async route=>{const request=route.request(),url=new URL(request.url());if(closing||fixtureError||workflowControl?.failed){await route.abort('blockedbyclient');return;}if(url.origin!==base){stopOnFailure(Error('Browser external request refused'));await route.abort('blockedbyclient');return;}if(!['GET','HEAD','OPTIONS'].includes(request.method()))receipt.browserMutations.push({stage,method:request.method(),path:url.pathname});await route.continue();});assertWorkflowActive();},
  newPage:context=>context.newPage(),adoptPage:handle=>{page=handle;},
 });
 assertWorkflowActive();captureOwnedBrowserIdentities('browser-ready');receipt.browserVersion=browser.version();page.setDefaultTimeout(15000);page.on('pageerror',error=>{receipt.browserErrors.push(scrub(error));stopOnFailure(Error('Browser script error'));});
 // END SUSTAINED BROWSER WIRING
 for(const owner of projectStates){setStage('prepare-and-queue-'+owner.task);await prepare(owner);await queueOwner(owner);}
 await protectCancellation();assert.equal(effects().create,0);await functionalDrafts();interactions=interactionRunner({page,owners:projectStates,receipt,assertHealthy:health,phase:()=>measuredPhase,observe:passiveTiming});await interactions.warm();
 setStage('idle-baseline');await quietPhase('quiet-app-open-idle');const idle=phaseStart('idle-interactive-calibration');await interactions.calibrate(100);phaseEnd(idle);await passiveTiming('end-idle-calibration');
 receipt.interactionSummary=summarizeInteractions(receipt.interactions);
 receipt.latencyTargets={measurement:'Original All projects→open saved Discover context→two frames; input→expected field→two frames; no physical-input claim',projectSwitchP95Ms:300,inputP95Ms:100,p99StrictlyBelowMs:1000};
 receipt.targetComparison=Object.fromEntries(['project-switch','input'].map(kind=>{const value=receipt.interactionSummary['idle-interactive-calibration:'+kind];assert.equal(value.count,kind==='input'?200:100);assert.equal(value.failed,0);return[kind,{...value,withinReference:value.p95Ms<=(kind==='input'?100:300)&&value.p99Ms<1000}];}));
 assert.equal(receipt.interactions.length,400);assert.equal(receipt.interactions.filter(row=>row.kind==='draft-reset').length,100);
 receipt.idlePerformanceTargetsMet=Object.values(receipt.targetComparison).every(row=>row.withinReference);receipt.performanceScope='Original packaged web; all100 switches and200 inputs, no warm outlier exclusion, no overhead subtraction. Exit and passed denote complete capture/cleanup; targetComparison and idlePerformanceTargetsMet govern idle performance only.';
 assert.equal(option('--trace-after','no'),'no','Full primary sample only; generic trace disabled');
 assert.deepEqual(effects(),{create:0,start:0,destroy:0});assert.equal(proxy.observations.failures.length,0);assert.equal(receipt.browserErrors.length,0);assert.equal(providerCalls,0);assert.equal(auxiliaryCalls,0);assert.equal(workerContainers().length,0);
 receipt.idleDiagnosticCompleted=true;receipt.passed=true;
 check('Fresh installed idle diagnostic completed; no full sustained or latency-target success implied');
 });
}catch(error){
 workflowControl?.fail(error);fixtureError??=error;
 try{journal.record('driver-caught-failure',{stage,...diagnosticError(error)});}catch{}
 receipt.failedStage=stage;receipt.failure=redactError(error);process.exitCode=1;console.error(receipt.failure);
 beginCleanup();persistFailureCheckpoint();
 if(page){try{await boundedAttempt(()=>passiveTiming('failure'),cleanupBudget(3000),'failure-timing-deadline-exceeded');}catch{}try{await boundedAttempt(()=>page.screenshot({path:join(evidence,'failed-stage.png'),fullPage:true}),cleanupBudget(3000),'failure-screenshot-deadline-exceeded');}catch{}}
}
finally {
 beginCleanup();persistFailureCheckpoint();
 try {
  for(const socket of sockets){try{socket.close();}catch(error){cleanupFailure(error);}}
  receipt.actualTaskResults=Object.fromEntries(observations);receipt.taskEvents=Object.fromEntries(taskEvents);receipt.interactionSummary=summarizeInteractions(receipt.interactions);
  receipt.taskChildren=[...taskData.values()].map(value=>({task:value.task,projectID:value.owner.id,operationID:value.operation.id,sessionID:value.session.id,container:value.container,bundle:initialBundles.get(value.session.id)}));receipt.terminalObservations=terminalProbes.map(({id,attachedStage,probe})=>({sessionID:id,attachedStage,...probe.snapshot()}));
 for(const value of taskData.values()){try{const file=join(value.checkout,'.luci-mixed-failure-'+value.task+'.json');if(existsSync(file)){const info=lstatSync(file);assert(info.isFile()&&!info.isSymbolicLink()&&info.size<=60*1024**2);const target=join(evidence,value.task+'-private-failure.json');writeFileSync(target,readFileSync(file),{mode:0o600,flag:'wx'});(receipt.failureDiagnostics??=[]).push({task:value.task,available:true,bytes:info.size,sha256:sha(readFileSync(target)),private:true});}else if(!observations.has(value.task)){(receipt.failureDiagnostics??=[]).push({task:value.task,available:false,reason:'No settled native failure bundle exists at cleanup. An active command may be interrupted by this failed fixture; its temporary corpus cannot be claimed retained.'});}}catch(error){cleanupFailure(error);}}

  captureOwnedBrowserIdentities('before-cleanup');
  receipt.cleanupPhases=await settleSustainedResources();receipt.startupResources=startupScope.snapshot();finishOwnedBrowserIdentities();
  if(proxy)receipt.dockerTransportObservations=structuredClone(proxy.observations);
 if(receipt.controllersStopped&&receipt.dockerRequestsDrained&&receipt.dockerMutationStateCertain&&proxy){const created=proxy.observations.requests.filter(value=>value.method==='POST'&&/\/containers\/create$/.test(value.path)&&value.status===201&&/^[a-f0-9]{64}$/.test(value.id??''));for(const creation of created){try{const ids=docker('ps','-aq','--no-trunc').split(/\s+/).filter(Boolean);if(!ids.includes(creation.id))continue;const value=JSON.parse(docker('inspect',creation.id))[0];assert.equal(value.Id,creation.id);assert.equal(value.Name,'/'+creation.name);assert.equal(value.Config.Labels['luci.instance'],instanceID());assert.equal(value.Config.Labels['luci.session-id'],creation.labels['luci.session-id']);assert(ownedSessions.has(value.Config.Labels['luci.session-id']));assert.equal(value.Config.Labels['luci.managed'],'true');assert.equal(value.Image,imageInfo.Id);assert(value.Mounts.some(mount=>mount.Destination==='/home/dev/work'&&projectStates.some(owner=>realpathSync(mount.Source).startsWith(join(dirname(owner.owner.path),'.luci-swarms')+sep))));cleanupFailure(Error('Unexpected full worker remained; exact failed-fixture cleanup required'));if(value.State.Running)docker('stop','--time','5',value.Id);const current=JSON.parse(docker('inspect',value.Id))[0];assert.equal(current.Id,value.Id);assert.equal(current.State.Running,false);docker('rm',value.Id);}catch(error){cleanupFailure(error);}}}
 try{assert(startupScope.snapshot().every(row=>row.state==='closed'&&!row.closeTimedOut),'Startup resource remains unsettled');assert(receipt.controllersStopped);assert.equal(receipt.dockerRequestsDrained,true);assert.equal(receipt.dockerMutationStateCertain,true);assert.equal(docker('ps','-aq'),'');assert.equal(docker('volume','ls','-q'),'');assert.equal(ownedProcesses().length,0);assert.equal(ownedProcesses('browser').length,0);assert.equal(JSON.parse(docker('image','inspect',image))[0].Id,imageInfo.Id);assert.equal(sha(readFileSync(binary)),provenance.binarySHA256);assert.equal(sha(readFileSync(join(web,'index.html'))),provenance.webIndexSHA256);assert.equal(docker('info','--format','{{.ID}}'),engineID);for(const name of driverNames)assert.equal(sha(readFileSync(new URL(name,import.meta.url))),sha(readFileSync(join(evidence,'accepted-harness',name))));for(const name of reusedHelpers)assert.equal(sha(readFileSync(join(helperRoot,'scripts/acceptance',name))),sha(readFileSync(join(evidence,'accepted-harness',name.replaceAll('/','_')))));
  receipt.finalHarnessCheckout=verifySourceBinding();assert.deepEqual(receipt.finalHarnessCheckout,receipt.harnessCheckout);
  receipt.runtimeCleanup={containers:0,volumes:0,ownedServiceProcesses:0,ownedBrowserProcesses:0,protectedImagePackageAndHelpersUnchanged:true,rootFreeBytes:rootFree()};assert.equal(realpathSync(fixture),fixture);assert.equal(dirname(fixture),fixtureParent);assert(fixture.split(sep).at(-1).startsWith('luci-sustained-'));if(receipt.passed&&!fixtureError){rmSync(fixture,{recursive:true});receipt.fixtureRemoved=true;}
 }catch(error){cleanupFailure(error);}
 if(!receipt.fixtureRemoved)receipt.fixtureRetained={path:fixture,reason:'Failed or partial acceptance; private fixture credentials/source/context and bounded diagnostic files remain. Do not reuse as normal workspace.'};

 }catch(error){cleanupFailure(error);}
 finally {
 if(fixtureError){receipt.passed=false;process.exitCode=1;receipt.backgroundFailure=redactError(fixtureError);}receipt.providerCalls=providerCalls;receipt.auxiliaryCalls=auxiliaryCalls;receipt.actualTaskCount=observations.size;receipt.elapsedMs=Date.now()-started;receipt.totalElapsedMs=Date.now()-driverStarted;receipt.workflowOperationUnsettled=workflowControl?.failed===true;receipt.startupResources=startupScope.snapshot();receipt.originalReceiptComplete=true;receipt.signal=signalFailure??null;receipt.finishedAt=new Date().toISOString();try{journal.record('driver-receipt-prepared',{stage,passed:receipt.passed});}catch(error){cleanupFailure(error);}writeFileSync(report,JSON.stringify(receipt,null,2)+'\n',{mode:0o600,flag:'wx'});console.log((receipt.passed?'PASS ':'FAIL ')+report);
  clearTimeout(cleanupHardStop);
  // Promise.race cannot cancel a losing operation. End only this driver after
  // the durable final receipt; unsettled external resources stay explicit.
  const finalExitCode=process.exitCode??(receipt.passed?0:1);setImmediate(()=>process.exit(finalExitCode));
 }
}
