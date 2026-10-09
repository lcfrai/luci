#!/usr/bin/env node
// Dedicated installed adapter; no provider or worker scenario. Runtime requires
// explicit CLI/window admission. Imported for controlled tests without effects.
import assert from 'node:assert/strict';
import {spawn,execFileSync} from 'node:child_process';
import {randomUUID} from 'node:crypto';
import {createServer} from 'node:net';
import {readFileSync,writeFileSync,existsSync,mkdirSync,mkdtempSync,readdirSync,lstatSync,realpathSync,statSync,statfsSync} from 'node:fs';
import {resolve,join,dirname,sep} from 'node:path';
import {fileURLToPath} from 'node:url';
import {setTimeout as delay} from 'node:timers/promises';
import {createWorkflowDeadline,runBoundedCleanup,writeFailureCheckpoint,settleOwnedBrowser,boundedAttempt} from '../launch-recovery/acceptance-lifecycle.mjs';
import {createStartupScope,startAcceptanceBrowser} from '../launch-recovery/acceptance-startup.mjs';
import {createAcceptanceJournal,diagnosticError,journalProcessIdentity} from '../launch-recovery/durable-acceptance-journal.mjs';
import {startFencedDockerProxy} from '../launch-recovery/fenced-docker-proxy.mjs';
import {runInstalledRecoveryScenario,pins,bounds,command} from './recovery-scenario.mjs';
import {validateCheckCreate} from './check-create-guard.mjs';
import {sha,validateFence,identityLabels,assertCapacity,assertComputeEnvelope,assertOriginalInspect,assertPublicReservationTotals,assessCheckPreflight,summarizeCheckLedger} from './fixture-guards.mjs';
import {verifyPackage,verifyHelperSourceBinding} from './provenance.mjs';

const here=dirname(fileURLToPath(import.meta.url));
export const fixtureFiles={
  'README.md':'Controlled installed check recovery fixture. No model or deployment acceptance.\n',
  'seed.txt':'installed-recovery-fixture-v1\n',
  'verify.mjs':"import assert from 'node:assert/strict';\nimport {readFileSync} from 'node:fs';\nassert.equal(process.argv[2],'final');\nassert.equal(readFileSync('seed.txt','utf8'),'installed-recovery-fixture-v1\\n');\nconsole.log('RECOVERY_CHECK_OK');\n",
};
export function processIdentity(pid){
  try{const raw=readFileSync(`/proc/${pid}/stat`,'utf8'),f=raw.slice(raw.lastIndexOf(')')+2).trim().split(/\s+/);return {pid:Number(pid),start:f[19],group:Number(f[2]),status:f[0]};}
  catch(e){if(['ENOENT','ESRCH'].includes(e.code))return null;throw e;}
}
export function stableJSON(path,limit=2*1024**2){
  const before=lstatSync(path);assert(before.isFile()&&!before.isSymbolicLink()&&before.nlink===1&&before.size<=limit);
  const bytes=readFileSync(path),after=lstatSync(path);
  assert.equal(after.ino,before.ino);assert.equal(after.size,before.size);assert.equal(after.mtimeMs,before.mtimeMs);assert.equal(bytes.length,before.size);
  return JSON.parse(bytes);
}
// Exact process identities make repeated scoped cleanup safe after signal exit.
export async function stopControllerTree({identity,readIdentity,listOwned,signal,wait=delay,now=Date.now,onForce=()=>{},graceMs=20000,forceMs=5000}){
  const matches=id=>{const current=readIdentity(id.pid);return !!current&&current.start===id.start&&current.status!=='Z';};
  const send=(id,pid,kind)=>{if(!matches(id))return;try{signal(pid,kind);}catch(error){if(error.code!=='ESRCH'||matches(id))throw error;}};
  if(identity)send(identity,identity.pid,'SIGTERM');
  // An environment scan may omit an unreadable process. The controller's
  // captured identity must independently reach absence, exit or replacement.
  const observedOwned=()=>{const rows=[...listOwned()];if(identity&&matches(identity)&&!rows.some(row=>row.pid===identity.pid&&row.start===identity.start))rows.push(identity);return rows;};
  const end=now()+graceMs;while(now()<end&&observedOwned().length)await wait(100);
  const left=observedOwned();
  if(left.length){onForce();for(const id of left)send(id,id.group===id.pid?-id.pid:id.pid,'SIGKILL');}
  const final=now()+forceMs;while(now()<final&&observedOwned().length)await wait(100);
  assert.equal(observedOwned().length,0,'Owned controller cleanup did not settle');
  assert.equal(left.length,0,'Owned controller required forced shutdown');return true;
}
const readinessError=code=>Object.assign(new Error(code),{code});
// Observe a submitted check without replaying it. Gate capture is checked both
// before and after GET: a real lost ACK may itself make that check terminal.
export async function waitSubmittedCheckGate({proxy,projectId,checkId,gateId,phase,readCheck,recordTerminal,deadline,health=()=>{},now=Date.now,wait=delay}){
  const remaining=()=>{health();const ms=deadline-now();if(ms<=0)throw readinessError('check-create-readiness-deadline');return ms;};
  const captured=()=>{
    const rows=proxy.observations.gates.filter(row=>row.id===gateId);assert.equal(rows.length,1,'Expected exact check gate');const gate=rows[0];
    assert.equal(gate.phase,phase);if(gate.state==='armed')return null;
    assert(['holding','lost'].includes(gate.state),'Expected check gate is no longer available');
    const request=gate.request;assert.equal(request.kind,'check');assert.equal(request.method,'POST');assert(/\/containers\/create$/.test(request.path));
    assert.equal(request.labels['luci.check-project'],projectId);assert.equal(request.labels['luci.check-id'],checkId);
    if(phase==='after-response'){assert.equal(request.status,201);assert.match(request.id,/^[a-f0-9]{64}$/);}
    return gate;
  };
  const acknowledge=async()=>{const gate=await proxy.waitForGate(gateId,{timeoutMs:remaining()});health();return gate;};
  for(;;){
    remaining();if(captured())return acknowledge();
    const requestedAt=new Date(now()).toISOString(),check=await readCheck(Math.min(5000,remaining()));health();
    assert.equal(check.id,checkId);assert.equal(check.projectId,projectId);
    if(captured())return acknowledge();
    if(!['queued','running'].includes(check.status)){
      await recordTerminal({gateId,projectId,checkId,checkRequestedAt:requestedAt,checkObservedAt:new Date(now()).toISOString(),check},Math.max(0,Math.min(5000,deadline-now())));
      throw readinessError('check-terminal-before-create-gate');
    }
    await wait(Math.min(250,remaining()));
  }
}
// Advisory only: the unchanged installed admission remains authoritative after
// the single browser POST. Unavailable/stale reads never become ready capacity.
export async function waitCheckPreflight({read,record,deadline,health=()=>{},now=Date.now,wait=delay}){
  let attempts=0,last;
  const active=()=>{try{health();}catch(error){record({phase:'check-preflight-refused',at:new Date(now()).toISOString(),attempts,last,reasonCode:error.code??'workflow-unavailable'});throw error;}};
  for(;;){
    active();if(now()>=deadline){record({phase:'check-preflight-refused',at:new Date(now()).toISOString(),attempts,last});throw readinessError('check-preflight-deadline');}
    const requestedAt=new Date(now()).toISOString();
    try{
      const sample=await read(Math.min(5000,deadline-now()));
      last={requestedAt,observedAt:new Date(now()).toISOString(),policy:sample.view?.policy,resources:sample.view?.resources,ledgerSummary:summarizeCheckLedger(sample.options.ledger),rootFreeBytes:sample.options.rootFreeBytes,
        assessment:assessCheckPreflight(sample.view,{...sample.options,now:now()})};
    }catch(error){last={requestedAt,observedAt:new Date(now()).toISOString(),assessment:{ready:false,reasonCode:'resource_observation_unavailable'},readError:{name:error.name,code:error.code??null}};}
    attempts++;active();
    if(now()>=deadline)continue;
    if(last.assessment.ready){record({phase:'check-preflight-ready',at:new Date(now()).toISOString(),attempts,...last});return last;}
    await wait(Math.min(1000,deadline-now()));
  }
}
export async function main(args=process.argv.slice(2)){
  const valueOptions=new Set(['--repo','--binary','--web','--provenance','--image-provenance','--report','--fixture-root','--chromium','--observer-instance-pointer','--expected-image-id','--helper-commit','--source-binding','--expected-source']);
  const flagOptions=new Set(['--help','--window-granted','--charged-witness','--require-public-reservations']),seenOptions=new Set();
  for(let i=0;i<args.length;i++){
    const key=args[i];assert(valueOptions.has(key)||flagOptions.has(key),'Unknown adapter argument');assert(!seenOptions.has(key),'Duplicate adapter argument');seenOptions.add(key);
    if(valueOptions.has(key)){assert(args[i+1]&&!args[i+1].startsWith('--'),'Missing adapter argument value');i++;}
  }
  const option=(name,fallback)=>{const n=args.indexOf(name);if(n<0)return fallback;assert(args[n+1]&&!args[n+1].startsWith('--'));return args[n+1];};
  if(args.includes('--help')){console.log('Installed recovery requires --window-granted --repo --binary --web --provenance --image-provenance --expected-image-id --helper-commit --source-binding --report FRESH_JSON --fixture-root SHORT_PRIVATE_PARENT. Optional --expected-source defaults to reviewed4df baseline; also --chromium, --observer-instance-pointer, capacity-gated --charged-witness, and --require-public-reservations for current shared-ledger visibility acceptance.');return;}
  assert(args.includes('--window-granted'),'An explicit root runtime window is required');
  assert.equal(option('--expected-image-id'),pins.image);
  const helperCommit=option('--helper-commit');assert.match(helperCommit,/^[a-f0-9]{40}$/);
  const expectedSource=option('--expected-source',pins.product);assert.match(expectedSource,/^[a-f0-9]{40}$/);
  const runtimePins={...pins,product:expectedSource};
  const requirePublicReservations=args.includes('--require-public-reservations');
  assert.equal(process.platform,'linux');assert.notEqual(process.getuid(),0);
  const repo=resolve(option('--repo','/workspace/luci')),binary=resolve(option('--binary')),
    web=resolve(option('--web')),report=resolve(option('--report')),evidence=dirname(report),fixtureParent=resolve(option('--fixture-root'));
  assert.equal(realpathSync(fixtureParent),fixtureParent);assert(lstatSync(fixtureParent).isDirectory());
  assert(!existsSync(evidence),'Fresh output directory required');
  const sourceRoot=resolve(here,'..');
  const sourceBinding=verifyHelperSourceBinding({repo,acceptanceRoot:sourceRoot,bindingPath:resolve(option('--source-binding')),helperCommit,pins});
  const provenance=verifyPackage({binary,web,provenancePath:resolve(option('--provenance')),imageProvenancePath:resolve(option('--image-provenance')),pins:runtimePins,repo});
  const docker=(...a)=>execFileSync('docker',a,{encoding:'utf8',timeout:30000,maxBuffer:8*1024**2}).trim();
  assert.equal(docker('ps','-aq'),'');assert.equal(docker('volume','ls','-q'),'');
  const engine=JSON.parse(docker('info','--format','{{json .}}')),image=JSON.parse(docker('image','inspect',pins.image))[0];
  assert.equal(image.Id,pins.image);assert.equal(statSync(engine.DockerRootDir).dev,statSync('/').dev);
  const free=()=>{const s=statfsSync('/');return Number(s.bavail)*Number(s.bsize);};assert(free()>=6*1024**3);
  mkdirSync(evidence,{recursive:true,mode:0o700});
  const fixture=mkdtempSync(join(fixtureParent,'luci-check-recovery-'));
  for(const name of ['home','projects','state','docker'])mkdirSync(join(fixture,name),{mode:0o700});
  const receipt={passed:false,mode:'installed-check-create-recovery',startedAt:new Date().toISOString(),sourceCommit:expectedSource,
    helperSourceCommit:helperCommit,helperBaseCommit:pins.helper,packageIntegrity:provenance.verified,image:pins.image,externalModelCalls:0,actualTaskCount:0,
    bounds,publicReservationVisibilityRequired:requirePublicReservations,apiActions:[],browserMutations:[],phases:[],cleanup:[],setupScaffolding:['Fresh team and two approved check projects through installed APIs',args.includes('--charged-witness')?'Current supported policy permits two 1GiB/1000mCPU checks':'Current supported policy permits one 1GiB/1000mCPU check','Real daemon Create ACK deliberately lost once'],
    scope:'Actual installed unknown-Create check recovery; no command replay, unrelated completed history preserved, permanent marker retained. '+(args.includes('--charged-witness')?'Optional concurrent charged-witness isolation required.':'Concurrent charged-witness isolation is unverified in this one-allocation scope.')+' No full-product/model/Windows/deployment claim.'};
  writeFileSync(join(evidence,'package-provenance.json'),provenance.provenanceBytes,{flag:'wx',mode:0o600});
  writeFileSync(join(evidence,'helper-source-binding.json'),JSON.stringify(sourceBinding,null,2)+'\n',{flag:'wx',mode:0o600});
  mkdirSync(join(evidence,'accepted-harness'),{mode:0o700});
  for(const row of sourceBinding.executedFiles)writeFileSync(join(evidence,'accepted-harness',row.path.replaceAll('/','_')),readFileSync(join(sourceRoot,row.path)),{flag:'wx',mode:0o600});
  const journal=createAcceptanceJournal(join(evidence,'diagnostic-journal.jsonl'));
  journal.record('driver-start',{...journalProcessIdentity(),deadlineMs:bounds.scenarioMs});
  let closing=false,stage='startup',failure,service,serviceIdentity,proxy,browser,page,base,env,ownerToken,ownerID,team,teamPrefix,teamRoot,editorToken,editorID,workflow,workflowDeadline,timer,pendingConsent;
  const password=randomUUID()+randomUUID(),editorPassword=randomUUID()+randomUUID(),lifecycle=randomUUID()+randomUUID();
  const secrets=[password,editorPassword,lifecycle];
  const projects=[],authorizedChecks=new Map(),createIdentities=new Map(),signalHandlers=[];
  const safeError=e=>({...diagnosticError(e),locations:[...String(e?.stack??'').matchAll(/(?:^|[/\\])(installed-recovery\.mjs|recovery-scenario\.mjs|fixture-guards\.mjs|check-create-guard\.mjs|provenance\.mjs):(\d+):(\d+)/g)].slice(0,8).map(([,file,line,column])=>({file,line:Number(line),column:Number(column)}))});
  const encodePublic=value=>{const text=JSON.stringify(value,null,2)+'\n';assert(Buffer.byteLength(text)<=8*1024**2,'Receipt byte limit exceeded');for(const secret of secrets)assert(!text.includes(secret),'Private fixture credential reached receipt');return text;};
  const record=row=>{assert(receipt.phases.length<64);const encoded=encodePublic(row);receipt.phases.push(row);journal.record('driver-stage',{stage:row.phase});writeFileSync(join(evidence,'phase-'+String(receipt.phases.length).padStart(2,'0')+'.json'),encoded,{flag:'wx',mode:0o600});};
  const health=()=>{if(closing)throw Error('workflow-closing');workflow?.assertActive();if(failure)throw failure;if(service&&(service.exitCode!==null||service.signalCode!==null))throw Error('owned-controller-exited');};
  const fail=e=>{failure??=e;workflow?.fail(e);};
  const setStage=s=>{health();stage=s;journal.record('driver-stage',{stage});};
  const stillOwned=id=>{const now=processIdentity(id.pid);return now&&now.start===id.start&&now.status!=='Z';};
  function ownedProcesses(kind='service'){
    const found=[];
    for(const entry of readdirSync('/proc').filter(x=>/^\d+$/.test(x))){const id=processIdentity(entry);if(!id||id.status==='Z')continue;
      try{const values=Object.fromEntries(readFileSync(`/proc/${entry}/environ`,'utf8').split('\0').filter(Boolean).map(v=>{const i=v.indexOf('=');return [v.slice(0,i),v.slice(i+1)];}));
        const owns=kind==='browser'?values.LUCI_SWARM_BROWSER_FIXTURE===fixture:(values.LUCI_STATE_DIR===join(fixture,'state')||values.LUCI_STATE_DIR?.startsWith(join(fixture,'state')+sep));
        if(owns&&stillOwned(id))found.push(id);
      }catch(e){if(!['ENOENT','ESRCH','EACCES','EPERM'].includes(e.code))throw e;}}
    return found;
  }
  async function stopControllers(){
    return stopControllerTree({identity:serviceIdentity,readIdentity:processIdentity,listOwned:ownedProcesses,
      signal:(pid,signal)=>process.kill(pid,signal),onForce:()=>receipt.cleanup.push('owned-controller-required-force')});
  }
  async function until(label,fn,ms=120000){const end=Date.now()+ms;while(Date.now()<end){health();const value=await fn();if(value)return value;await delay(150);}throw Error(label+'-deadline');}
  async function api(method,path,body,status=200,token=ownerToken,timeoutMs=30000){
    assert(Number.isSafeInteger(timeoutMs)&&timeoutMs>0&&timeoutMs<=30000);health();const response=await fetch(base+path,{method,redirect:'error',signal:AbortSignal.timeout(timeoutMs),headers:{Origin:base,...(token?{Authorization:'Bearer '+token}:{}),...(body===undefined?{}:{'Content-Type':'application/json'})},body:body===undefined?undefined:JSON.stringify(body)});
    receipt.apiActions.push({stage,method,path,status:response.status,at:new Date().toISOString()});
    const parts=[];let size=0;for await(const part of response.body??[]){size+=part.length;assert(size<=2*1024**2);parts.push(part);}
    assert.equal(response.status,status,'Unexpected bounded API status');health();return JSON.parse(Buffer.concat(parts).toString()||'{}');
  }
  const inner=path=>teamPrefix+path,route=p=>inner('/v1/product/projects/'+p.id);
  const ledger=()=>{const path=join(fixture,'state/control-plane/execution/leases.json');return existsSync(path)?stableJSON(path):{version:1,leases:[]};};
  const instance=()=>{const value=readFileSync(join(teamRoot,'state/instance/instance-id'),'utf8').trim();assert.match(value,/^[a-f0-9-]{36}$/);return value;};
  // Product4df uses this team-runtime path on Linux; compact gateway paths are Windows-only.
  const conversations=()=>join(teamRoot,'state/conversations');
  const checkHistory=p=>{const path=join(teamRoot,'state/sessions/project-checks',p.id+'.json');return existsSync(path)?stableJSON(path):[];};
  function exactPrivateCheck(id){const project=projects.find(p=>checkHistory(p).some(r=>r.id===id));assert(project);const rows=checkHistory(project).filter(r=>r.id===id);assert.equal(rows.length,1);return {project,check:rows[0]};}
  function checkOptions(project,check){const auth=authorizedChecks.get(check.id);assert(auth);return {name:'luci-check-'+check.commands[0].allocationId,fixture,primary:project.path,projectRoot:join(teamRoot,'projects'),project,check,
    commit:auth.commit,sourceDigest:auth.sourceDigest,imageID:pins.image,uid:process.getuid(),gid:process.getgid(),ledger:ledger(),scopeDigest:sha(resolve(conversations())),actor:{userId:ownerID,teamId:team.team.id,projectId:project.id},
    engineID:engine.ID,instance:instance(),imageVirtualSize:image.Size,storageDriver:engine.Driver,authorizedChecks:new Set(authorizedChecks.keys())};}
  function identify(bytes,meta){
    health();const input=JSON.parse(bytes);assert.equal(input.Labels?.['luci.instance'],instance());
    if(input.Labels['luci.launch-fence']==='1'){
      const matches=(ledger().checkFences??[]).filter(r=>r.identity.allocationId===input.Labels['luci.check-allocation']);assert.equal(matches.length,1);
      assert.equal(matches[0].state,'intent');validateFence(input,{record:matches[0],name:meta.name});return 'fence';
    }
    assert.equal(input.Labels['luci.check'],'true');const {project,check}=exactPrivateCheck(input.Labels['luci.check-id']);
    if(!authorizedChecks.has(check.id)){
      assert(pendingConsent&&pendingConsent.projectId===project.id);assert(Date.parse(check.createdAt)>=Date.parse(pendingConsent.at));
      assert(![...authorizedChecks.values()].some(value=>value.consentID===pendingConsent.consentID));authorizedChecks.set(check.id,{...pendingConsent});
    }
    const result=validateCheckCreate(input,{...checkOptions(project,check),name:meta.name});createIdentities.set(result.allocationId,{...result,guardedCreate:structuredClone(input)});return 'check';
  }
  async function admission(meta){
    const kind=identify(meta.body,meta);assert.equal(kind,meta.kind);assert(meta.ordinal<=4);
    const knownIDs=new Set(proxy.observations.requests.filter(row=>row.status===201&&row.createResponseId).map(row=>row.createResponseId));
    for(const id of docker('ps','-aq','--no-trunc').split(/\s+/).filter(Boolean))assert(knownIDs.has(id),'Foreign Docker allocation appeared during exclusive fixture');
    assert.equal(docker('volume','ls','-q'),'');
    const currentEngine=JSON.parse(docker('info','--format','{{json .}}'));assert.equal(currentEngine.ID,engine.ID);
    const view=await api('GET','/v1/product/swarm-scheduler');
    const charge=assertCapacity(view,{ledger:ledger(),engine:currentEngine,rootFreeBytes:free(),imageSize:image.Size,requireCreate:kind==='check',requireReservationMatch:false});
    record({phase:'fresh-create-admission',at:new Date().toISOString(),kind,ordinal:meta.ordinal,charge,chargeAuthority:'full-private-shared-ledger',publicReservationScope:'owner-public-read-non-authoritative-for-release',rootFreeBytes:free(),resources:view.resources});health();
  }
  async function freePort(){const server=createServer();await new Promise((done,reject)=>server.once('error',reject).listen(0,'127.0.0.1',done));const port=server.address().port;await new Promise(done=>server.close(done));health();return port;}
  const scope=createStartupScope({assertActive:health,onFailure:fail,onTransition:row=>journal.record('driver-stage',{stage:'startup-'+row.name+'-'+row.state})});
  async function action(path,click,expected=200){const pending=page.waitForResponse(r=>new URL(r.url()).pathname===path&&r.request().method()==='POST',{timeout:120000});void pending.catch(()=>{});await click();const response=await pending;assert.equal(response.status(),expected);return {row:await response.json(),request:response.request().postDataJSON(),httpStatus:response.status()};}
  async function openProject(p){
    health();await page.goto(base);const username=page.getByLabel('Username',{exact:true}),enter=page.getByRole('button',{name:'Continue to projects',exact:true}),picker=page.getByLabel('Workspace',{exact:true});
    await until('account-entry',async()=>await username.isVisible()||await enter.isVisible()||await picker.isVisible());
    if(await username.isVisible()){await username.fill('recoveryowner');await page.getByLabel('Password',{exact:true}).fill(password);await action('/v1/auth/login',()=>page.getByRole('button',{name:'Sign in',exact:true}).click());}
    await until('workspace-entry',async()=>await enter.isVisible()||await picker.isVisible());if(await enter.isVisible())await enter.click();await picker.selectOption(team.team.id);
    await page.getByRole('button',{name:'All projects',exact:true}).click();await page.getByRole('heading',{name:'Across your projects',exact:true}).waitFor();
    await page.getByRole('button',{name:'Open project '+p.name,exact:true}).click();await page.getByRole('button',{name:'Develop',exact:true}).click();
    const details=page.locator('details.project-work');await details.waitFor();if(!await details.evaluate(e=>e.open))await details.locator(':scope > summary').click();
    await page.locator('.project-checks').getByRole('heading',{name:'Verify the implementation',exact:true}).waitFor();
  }
  const dialog=()=>page.getByRole('dialog',{name:'Review check recovery',exact:true});
  async function openRecovery(p,id,review){
    await openProject(p);const pending=page.waitForResponse(r=>new URL(r.url()).pathname===route(p)+'/checks/'+id+'/recovery'&&r.request().method()==='GET');void pending.catch(()=>{});
    // Only the target is unresolved in this project; historical check is settled.
    const buttons=page.locator('.project-checks .check-runs > li').getByRole('button',{name:'Review check recovery',exact:true});await buttons.waitFor({state:'visible'});assert.equal(await buttons.count(),1);await buttons.click();
    const response=await pending;assert.equal(response.status(),200);assert.deepEqual(await response.json(),review);
    await dialog().getByText(id,{exact:true}).waitFor();await dialog().getByText(/A small permanent Docker recovery marker/).waitFor();
    assert(await dialog().getByRole('button',{name:'Recover this exact check',exact:true}).isDisabled());
    return dialog();
  }
  async function screenshot(name){await page.screenshot({path:join(evidence,name+'.png'),fullPage:true});}
  async function createProject(name){
    const p=await api('POST',inner('/v1/product/projects'),{name},201);assert.equal(realpathSync(p.path),p.path);assert(p.path.startsWith(join(teamRoot,'projects')+sep));
    for(const [name,value] of Object.entries(fixtureFiles))writeFileSync(join(p.path,name),value,{mode:0o600});
    await api('POST',route(p)+'/brief',{revision:0,purpose:'Review exact retained check recovery without replay',audience:'Fixture project owner',constraints:'Controlled Docker fixture; retain uncertain execution and current authority.',criteria:['Preserve output and exact ownership while retiring only the reviewed allocation.']});
    let spec=await api('GET',route(p)+'/specification');spec=await api('POST',route(p)+'/specification',{expectedRevision:spec.revision,expectedBriefRevision:spec.briefRevision,specification:{...spec,requirements:[{id:'AC-recovery',title:'Reviewed check recovery',priority:'must',acceptance:'Retain uncertain check output, verify exact recovery ownership and never replay its command.'}]}});
    p.spec=await api('POST',route(p)+'/specification/approve',{revision:spec.revision,briefRevision:spec.briefRevision});
    let design=await api('GET',route(p)+'/design-plan');design=await api('POST',route(p)+'/design-plan',{expectedRevision:design.revision,expectedBriefRevision:p.spec.briefRevision,expectedSpecificationRevision:p.spec.revision,plan:{...design,screens:[{id:'recovery',name:'Exact retained check',route:'/',purpose:'Review exact check ownership and retained output',states:['Unknown','Recovery complete'],widths:[390,1440],requirementIds:['AC-recovery']}]}});
    p.design=await api('POST',route(p)+'/design-plan/approve',{revision:design.revision});
    const review=await api('GET',route(p)+'/git');await api('POST',route(p)+'/git/commit',{message:'Controlled recovery fixture and approved criteria',expectedReviewDigest:review.reviewDigest});
    projects.push(p);return p;
  }
  async function setupAuthority(){
    const account=await api('POST','/v1/auth/bootstrap',{bootstrapToken:readFileSync(join(fixture,'state/control-plane/bootstrap-token'),'utf8').trim(),username:'recoveryowner',displayName:'Recovery Fixture Owner',password},201);
    ownerToken=account.token;ownerID=account.user.id;secrets.push(ownerToken);
    const policy=await api('GET','/v1/admin/policy');receipt.policyBefore=policy;
    receipt.policyApplied=await api('PATCH','/v1/admin/policy',{maxConcurrentChecksPerUser:args.includes('--charged-witness')?2:1,checkMemoryMiB:1024,checkCPUMillis:1000});
    team=await api('POST','/v1/teams',{name:'Controlled check recovery team'},201);teamPrefix='/v1/teams/'+team.team.id+'/workspace';teamRoot=join(fixture,'state/control-plane/teams',team.team.id,'runtime');
    await api('POST',inner('/v1/setup/git/identity'),{name:'Recovery Fixture',email:'recovery@example.invalid'});
    // Scheduler administration is machine-scoped; team runtime requests are forbidden.
    const settings=await api('GET','/v1/product/swarm-scheduler');await api('PUT','/v1/product/swarm-scheduler',{expectedRevision:settings.revision,policy:{maxActive:1,maxPerProject:1,headroomPercent:25,minHeadroomMiB:2048,diskReserveMiB:1024},dispatchPaused:true});
    await createProject('recovery-target');await createProject('recovery-unrelated');
    const member=await api('POST','/v1/admin/users',{username:'recoveryeditor',displayName:'Recovery Current Authority Witness',password:editorPassword,role:'member'},201);editorID=member.id;
    team=await api('POST','/v1/teams/'+team.team.id+'/members',{expectedRevision:team.team.revision,username:'recoveryeditor',role:'editor',grants:{execute:true}});
    team=await api('PUT','/v1/teams/'+team.team.id+'/projects/'+projects[0].id+'/members/'+editorID,{expectedRevision:team.team.revision,role:'editor',grants:{execute:true}});
    const signed=await api('POST','/v1/auth/login',{username:'recoveryeditor',password:editorPassword});editorToken=signed.token;secrets.push(editorToken);
    const pointer=option('--observer-instance-pointer');if(pointer)writeFileSync(resolve(pointer),join(teamRoot,'state/instance/instance-id')+'\n',{flag:'wx',mode:0o600});
  }
  const readCheck=(p,id)=>api('GET',route(p)+'/checks/'+id);
  async function capacityReceipt(phase,details){
    const requestedAt=new Date().toISOString(),view=await api('GET','/v1/product/swarm-scheduler');
    record({phase,requestedAt,completedAt:new Date().toISOString(),...details,resources:view.resources,rootFreeBytes:free()});
  }
  function sourceSnapshot(){
    return projects.map(p=>{
      const names=execFileSync('git',['-c','core.hooksPath=/dev/null','ls-files','--cached','--others','--exclude-standard','-z'],{cwd:p.path,encoding:'utf8',timeout:15000,maxBuffer:1024**2}).split('\0').filter(Boolean);assert(names.length<=500);
      return {projectId:p.id,files:Object.fromEntries([...new Set(names)].sort().map(name=>{assert(!name.split('/').includes('..'));const path=join(p.path,name),s=lstatSync(path);assert(s.isFile()&&!s.isSymbolicLink()&&s.size<=8*1024**2);return [name,sha(readFileSync(path))];}))};
    });
  }
  async function verifyFence(record){
    const name='luci-check-'+record.identity.allocationId,id=record.proof.fenceRef;
    for(const ref of [id,name])validateFence(JSON.parse(docker('inspect',ref))[0],{record,name,inspected:true});
    const ids=docker('ps','-aq','--no-trunc','--filter','label=luci.instance='+record.identity.instanceId,'--filter','label=luci.check-allocation='+record.identity.allocationId).split(/\s+/).filter(Boolean);
    assert.deepEqual(ids,[id]);
  }
  const ctx={pins:runtimePins,expectedSource,bounds,windowGranted:true,freshFixture:true,externalModelCalls:0,projects,chargedWitness:args.includes('--charged-witness'),
    expectedOutput:'RECOVERY_CHECK_OK\n',get proxy(){return proxy;},record,
    waitForCheckGate(p,id,gateId,phase){return waitSubmittedCheckGate({proxy,projectId:p.id,checkId:id,gateId,phase,health,
      deadline:Math.min(workflowDeadline,Date.now()+bounds.requestMs),readCheck:timeout=>api('GET',route(p)+'/checks/'+id,undefined,200,ownerToken,timeout),
      recordTerminal:async(details,timeout)=>{
        const outcome={phase:'check-terminal-before-create-gate',...details,resourceRequestedAt:new Date().toISOString()};receipt.submittedCheckTerminal=outcome;
        try{if(timeout<=0)throw readinessError('resource-observation-budget-exhausted');const view=await api('GET','/v1/product/swarm-scheduler',undefined,200,ownerToken,timeout);outcome.policy=view.policy;outcome.resources=view.resources;}
        catch(error){outcome.resourceObservationError=safeError(error);}
        outcome.resourceObservedAt=new Date().toISOString();record(outcome);
      }});},
    async startCheckInBrowser(p,cmd){
      setStage('consent-check-'+(authorizedChecks.size+1));await openProject(p);
      const history=authorizedChecks.size===0;
      if(history)proxy.armGate({id:'history-pre-start-capacity',phase:'after-response',method:'POST',pathPattern:/\/containers\/create$/,decision:'hold',match:row=>row.kind==='check'&&row.labels['luci.check-project']===p.id});
      const region=page.locator('.project-checks');let count=await region.locator('.check-commands > li').count();assert(count<=8);
      while(count){await region.locator('.check-commands > li').first().getByRole('button',{name:/^Remove /}).click();count=await region.locator('.check-commands > li').count();}
      const advanced=region.locator('summary').filter({hasText:/^Advanced check commands$/});if(!await advanced.evaluate(e=>e.parentElement.open))await advanced.click();
      await region.getByLabel('Executable',{exact:true}).fill(cmd.executable);await region.getByLabel('Arguments · JSON array',{exact:true}).fill(JSON.stringify(cmd.args));
      await region.getByRole('button',{name:'Add check command',exact:true}).click();assert.equal(await region.locator('.check-commands > li').count(),1);
      const requirements=region.locator('details.check-requirements');if(!await requirements.evaluate(e=>e.open))await requirements.locator(':scope > summary').click();
      await region.getByRole('checkbox',{name:'Check AC-recovery · Reviewed check recovery',exact:true}).check();
      await region.getByRole('checkbox',{name:"I trust this project's code and these Docker commands to write project files and use the network.",exact:true}).check();
      const coverage=(await api('GET',route(p)+'/checks')).coverage;assert.equal(coverage.source.available,true);assert.equal(coverage.source.dirty,false);
      assert.equal(coverage.specification.status,'approved');assert.equal(coverage.design.status,'approved');
      await waitCheckPreflight({deadline:Math.min(workflowDeadline,Date.now()+30000),health,record:row=>record({...row,projectId:p.id}),read:async timeout=>({
        view:await api('GET','/v1/product/swarm-scheduler',undefined,200,ownerToken,timeout),
        options:{ledger:ledger(),engine,rootFreeBytes:free(),imageSize:image.Size}})});
      health();
      pendingConsent={projectId:p.id,commit:coverage.source.commit,sourceDigest:coverage.source.sourceDigest,at:new Date().toISOString(),consentID:randomUUID()};
      const sent=await action(route(p)+'/checks',()=>region.getByRole('button',{name:'Run selected requirement checks',exact:true}).click(),202);
      assert.deepEqual(sent.request,{commands:[cmd],execution:'docker',acknowledgeHostExecution:true,requirementIds:['AC-recovery'],expectedSpecificationRevision:coverage.specification.revision,expectedDesignPlanRevision:coverage.design.revision});
      if(authorizedChecks.has(sent.row.id))assert.equal(authorizedChecks.get(sent.row.id).consentID,pendingConsent.consentID);
      else authorizedChecks.set(sent.row.id,{...pendingConsent});pendingConsent=null;
      receipt.checkConsents??=[];receipt.checkConsents.push({checkId:sent.row.id,projectId:p.id,request:sent.request});
      if(history){
        const gate=await ctx.waitForCheckGate(p,sent.row.id,'history-pre-start-capacity','after-response');
        await capacityReceipt('actual-create-ack-held-before-product-start',{checkId:sent.row.id,containerId:gate.request.id});
        await proxy.releaseGate('history-pre-start-capacity');
      }
      return sent.row;
    },
    async waitCheckTerminal(p,id){const row=await until('check-terminal',async()=>{const check=await readCheck(p,id);return ['queued','running'].includes(check.status)?null:check;});await capacityReceipt('terminal-check-capacity',{check:row});return row;},
    readCheck,readRecovery:(p,id)=>api('GET',route(p)+'/checks/'+id+'/recovery'),
    readLedger:async()=>structuredClone(ledger()),snapshotProjectFiles:async()=>sourceSnapshot(),
    async admitAdditionalChargedWitness(){
      const current=ledger(),active=(current.checks??[]).filter(r=>r.state!=='finished');assert.equal(active.length,1);
      const demand=image.Size*(engine.Driver==='vfs'?2:1),pending=active.reduce((sum,row)=>sum+row.createDiskBytes,0);
      assert(free()>=pending+demand+1024**3,'Real capacity cannot admit the optional charged witness');
      const view=await api('GET','/v1/product/swarm-scheduler');
      const charge=assertCapacity(view,{ledger:current,engine,rootFreeBytes:free(),imageSize:image.Size,requireCreate:true,requireReservationMatch:false});
      assertComputeEnvelope(view,{memoryMiB:charge.memoryMiB+1024,cpuMillis:charge.cpuMillis+1000});
    },
    async assertCurrentReservationTotals(expected){
      const checkpointDeadline=Date.now()+10000;
      await until('exact-reservation-summary',async()=>{
        const view=await api('GET','/v1/product/swarm-scheduler',undefined,200,ownerToken,Math.max(1,checkpointDeadline-Date.now()));
        const totals=assertCapacity(view,{ledger:ledger(),engine:JSON.parse(docker('info','--format','{{json .}}')),rootFreeBytes:free(),imageSize:image.Size,requireReservationMatch:false});
        assert.deepEqual(totals,expected);
        const publicReservations=view.resources.reservations??null;
        // Backpressure is unknown, never zero. Retry only this authorized GET,
        // bounded by the checkpoint deadline; no control command is replayed.
        if(requirePublicReservations&&publicReservations?.status==='unavailable'){
          receipt.publicReservationUnavailableReads=(receipt.publicReservationUnavailableReads??0)+1;
          receipt.lastUnavailablePublicReservation={at:new Date().toISOString(),expected,observation:publicReservations};return false;
        }
        const publicVisibility=requirePublicReservations?assertPublicReservationTotals(publicReservations,expected):null;
        record({phase:'exact-shared-ledger-charge',totals,chargeAuthority:'full-private-shared-ledger',publicReservationScope:'owner-public-read-non-authoritative-for-release',publicReservations,publicVisibilityRequired:requirePublicReservations,publicVisibilityVerified:!!publicVisibility,publicVisibility,observedByOwnerUserId:ownerID});return true;
      },10000);
    },
    async assertRealOriginalCreatedAndNeverStarted(created,lease){
      const inspected=JSON.parse(docker('inspect',created.id))[0],identity=createIdentities.get(lease.id);assert(identity);
      assertOriginalInspect(inspected,{id:created.id,name:created.name,imageID:pins.image,guardedCreate:identity.guardedCreate,imageEnv:image.Config.Env??[],imageExposedPorts:image.Config.ExposedPorts??{},imageVolumes:image.Config.Volumes??{}});
      const requests=proxy.observations.requests.filter(r=>r.ref===created.id&&/\/start$/.test(r.path));assert.equal(requests.length,0);
    },
    async openAndDismissReviewInBrowser(p,id,review){
      setStage('read-and-dismiss-exact-recovery');const panel=await openRecovery(p,id,review);await screenshot('exact-recovery-review');
      await panel.getByRole('button',{name:'Keep inspecting',exact:true}).click();await panel.waitFor({state:'hidden'});
    },
    async probeNoEditAuthority({targetProject:p,target,review,expectedStatus}){
      setStage('current-edit-revoked');const editorView=await api('GET',route(p)+'/checks/'+target.id+'/recovery',undefined,200,editorToken);assert.deepEqual(editorView,review);
      team=await api('PUT','/v1/teams/'+team.team.id+'/projects/'+p.id+'/members/'+editorID,{expectedRevision:team.team.revision,role:'viewer',grants:{}});
      const denied=await api('POST',route(p)+'/checks/'+target.id+'/recovery',{expectedRevision:editorView.revision,recoveryDigest:editorView.recoveryDigest,acknowledgeStop:true},expectedStatus,editorToken);
      assert.equal(denied.error,'team_permission_denied');record({phase:'current-edit-refused',status:expectedStatus,error:denied.error,projectId:p.id,checkId:target.id});
    },
    async probeStaleReview({targetProject:p,target,review,expectedStatus}){
      setStage('stale-recovery-review');const denied=await api('POST',route(p)+'/checks/'+target.id+'/recovery',{expectedRevision:review.revision+1,recoveryDigest:review.recoveryDigest,acknowledgeStop:true},expectedStatus);
      assert.equal(denied.error,'check_recovery_conflict');record({phase:'stale-review-refused',status:expectedStatus,error:denied.error});
    },
    async recoverInBrowser(p,id,review){
      setStage('explicit-current-owner-recovery');const panel=await openRecovery(p,id,review),path=route(p)+'/checks/'+id+'/recovery';
      const offset=receipt.browserMutations.length;await panel.getByRole('checkbox',{name:/I authorize recovery of this exact check command/}).check();
      const posted=await action(path,()=>panel.getByRole('button',{name:'Recover this exact check',exact:true}).click());
      await panel.getByText(/Recovery is complete/).waitFor();await screenshot('recovery-complete-marker-retained');
      const postCount=receipt.browserMutations.slice(offset).filter(r=>r.path===path&&r.method==='POST').length;
      await panel.getByRole('button',{name:'Keep inspecting',exact:true}).click();return {...posted,postCount};
    },
    async assertAcknowledgedPermanentFence({originalCreate,original,fence}){
      assert.deepEqual(fence.identity,original.identity);const rows=proxy.observations.requests;
      const originalDelete=rows.filter(r=>r.method==='DELETE'&&r.ref===originalCreate.id);assert.equal(originalDelete.length,1);assert.equal(originalDelete[0].status,204);
      assert(!docker('ps','-aq','--no-trunc').split(/\s+/).includes(originalCreate.id));
      const ack=rows.filter(r=>r.kind==='fence'&&r.method==='POST'&&/\/containers\/create$/.test(r.path));assert.equal(ack.length,1);
      assert.equal(ack[0].status,201);assert.equal(ack[0].id,fence.proof.fenceRef);assert(ack[0].deliveredAt&&!ack[0].discardedAt);
      await verifyFence(fence);record({phase:'positive-fence-proof',at:new Date().toISOString(),originalId:originalCreate.id,fenceId:fence.proof.fenceRef,proof:fence.proof,state:fence.state});
    },
    assertFenceStillPresent:verifyFence,
  };
  let scenario;
  try{
    workflowDeadline=Date.now()+bounds.scenarioMs;workflow=createWorkflowDeadline({timeoutMs:bounds.scenarioMs,onFailure:fail});
    for(const signal of ['SIGINT','SIGTERM']){const handler=()=>fail(Object.assign(Error('interrupted'),{code:'fixture-interrupted'}));process.on(signal,handler);signalHandlers.push([signal,handler]);}
    await workflow.run(async()=>{
      timer=setInterval(()=>{try{if(free()<1024**3)fail(Error('working-reserve-breached'));}catch(e){fail(e);}},1000);
      const upstream=process.env.DOCKER_HOST??'unix:///var/run/docker.sock';assert(upstream.startsWith('unix://'));
      proxy=await scope.acquire('proxy',{start:()=>startFencedDockerProxy({socketPath:join(fixture,'docker-proxy.sock'),upstreamSocketPath:upstream.slice(7),journal,identifyCreate:identify,beforeCreate:admission,
        beforeStart:row=>{health();assert.equal(row.kind,'check');assert(!proxy.observations.gates.some(g=>g.id==='target-create-ack-lost'&&g.request?.id===row.ref),'Unknown target Start is forbidden');},
        allowCheckTraffic:true,checkTrafficProtocol:'sdk-ledger-v1',allowCheckRecovery:true,checkCreateLimit:args.includes('--charged-witness')?3:2,maxCreates:args.includes('--charged-witness')?4:3,maxCreatesByKind:{worker:1,check:args.includes('--charged-witness')?3:2,fence:1},maxStarts:args.includes('--charged-witness')?2:1,upstreamTimeoutMs:120000,holdTimeoutMs:120000}),close:handle=>handle.close()});
      const port=await freePort(),preview=await freePort();base='http://127.0.0.1:'+port;
      env=Object.fromEntries(['PATH','LANG','LC_ALL','TZ'].filter(k=>process.env[k]).map(k=>[k,process.env[k]]));
      Object.assign(env,{HOME:join(fixture,'home'),DOCKER_CONFIG:join(fixture,'docker'),DOCKER_HOST:'unix://'+join(fixture,'docker-proxy.sock'),LUCI_DOCKER_HOST:'unix://'+join(fixture,'docker-proxy.sock'),
        LUCI_ADDR:'127.0.0.1:'+port,LUCI_STATE_DIR:join(fixture,'state'),LUCI_PROJECTS_DIR:join(fixture,'projects'),LUCI_WEB_DIR:web,LUCI_RUNTIME_DIR:join(dirname(binary),'runtime'),LUCI_SANDBOX_IMAGE:pins.image,LUCI_BACKEND:'docker',
        LUCI_MANAGED_SHUTDOWN_TOKEN:lifecycle,LUCI_LOG_LEVEL:'error',LUCI_BROWSER_PREVIEW_ADDR:'127.0.0.1:'+preview,LUCI_BROWSER_PREVIEW_PORT:String(preview),GIT_CONFIG_GLOBAL:join(fixture,'home/.gitconfig'),GIT_CONFIG_NOSYSTEM:'1'});
      await scope.acquire('controller',{start:async()=>{health();service=spawn(binary,[],{cwd:fixture,env,detached:true,stdio:['ignore','ignore','ignore']});service.on('error',fail);serviceIdentity=processIdentity(service.pid);assert(serviceIdentity);journal.record('controller-start',journalProcessIdentity(service.pid));
        await until('controller-ready',async()=>{try{const r=await fetch(base+'/v1/control-plane/status',{signal:AbortSignal.timeout(1500)});if(!r.ok)return false;const value=await r.json();assert.equal(value.product,'Luci');assert.equal(value.version,'0.5.0');return true;}catch{return false;}},45000);return service;},close:stopControllers});
      await setupAuthority();
      const {chromium}=await import(join(repo,'desktop/node_modules/playwright/index.mjs'));
      const browserEnv=Object.fromEntries(['PATH','LANG','LC_ALL','TZ'].filter(k=>process.env[k]).map(k=>[k,process.env[k]]));browserEnv.LUCI_SWARM_BROWSER_FIXTURE=fixture;
      await startAcceptanceBrowser({scope,assertActive:health,launch:()=>chromium.launch({headless:true,executablePath:option('--chromium','/usr/bin/chromium'),env:browserEnv,args:['--no-proxy-server']}),adoptBrowser:h=>{browser=h;},closeBrowser:h=>h.close(),
        newContext:h=>h.newContext({viewport:{width:1440,height:1080},serviceWorkers:'block'}),installRoutes:async context=>context.route('**/*',async request=>{const r=request.request(),url=new URL(r.url());if(closing||failure||url.origin!==base){await request.abort('blockedbyclient');if(url.origin!==base)fail(Error('external-browser-request'));return;}
          if(!['GET','HEAD','OPTIONS'].includes(r.method()))receipt.browserMutations.push({stage,method:r.method(),path:url.pathname});await request.continue();}),newPage:h=>h.newPage(),adoptPage:h=>{page=h;}});
      page.setDefaultTimeout(15000);page.on('pageerror',()=>fail(Error('browser-page-error')));receipt.browserVersion=browser.version();
      scenario=await runInstalledRecoveryScenario(ctx);receipt.scenario=scenario;await screenshot('settled-history-marker-retained');
    });
  }catch(error){failure??=error;receipt.failedStage=stage;receipt.failure=safeError(error);if(page)try{await boundedAttempt(()=>screenshot('failed-stage'),5000,'screenshot-timeout');}catch{}}
  finally{
    closing=true;clearInterval(timer);workflow?.close();for(const [signal,handler] of signalHandlers)process.off(signal,handler);
    receipt.cleanupPhases=await runBoundedCleanup({checkpoint:()=>writeFailureCheckpoint(report+'.incomplete.json',{version:1,passed:false,status:'cleanup-incomplete',stage,at:new Date().toISOString(),failure:receipt.failure??null,originalReceiptComplete:false}),
      onFailure:error=>{receipt.cleanup.push(safeError(error));},phases:[
        {name:'controllers',timeoutMs:35000,run:async()=>{await stopControllers();if(scope.has('controller'))await scope.close('controller',5000);receipt.controllersStopped=true;return true;}},
        {name:'proxy',timeoutMs:125000,run:async()=>{if(proxy){receipt.proxyClosure=await scope.close('proxy',125000);assert(receipt.proxyClosure.drained);receipt.dockerRequestsDrained=true;}return receipt.proxyClosure;}},
        {name:'browser',timeoutMs:15000,run:async()=>{if(scope.has('browser'))try{await scope.close('browser',5000);}catch(e){receipt.cleanup.push(safeError(e));}await settleOwnedBrowser({browser:null,listOwned:()=>ownedProcesses('browser'),stillOwned,forceTerminate:id=>{if(stillOwned(id))process.kill(id.pid,'SIGKILL');},onFailure:e=>receipt.cleanup.push(safeError(e))});}},
      ]});
    receipt.startupResources=scope.snapshot();receipt.dockerTransportObservations=proxy?.observations??null;
    try{
      assert.equal(ownedProcesses().length,0);assert.equal(ownedProcesses('browser').length,0);
      const ids=docker('ps','-aq','--no-trunc').split(/\s+/).filter(Boolean),volumes=docker('volume','ls','-q').split(/\s+/).filter(Boolean),saved=ledger();
      receipt.independentClosure={at:new Date().toISOString(),containers:ids,volumes,ledgerSHA256:sha(JSON.stringify(saved)),activeChecks:(saved.checks??[]).filter(r=>r.state!=='finished').map(r=>({id:r.id,state:r.state,resources:r.resources})),retainedFences:(saved.checkFences??[]).map(r=>({allocationId:r.identity.allocationId,state:r.state,fenceRef:r.proof?.fenceRef}))};
      if(scenario){assert.deepEqual(ids,[scenario.retainedMarker.id]);assert.deepEqual(volumes,[]);assert.equal(receipt.independentClosure.activeChecks.length,0);const marker=(saved.checkFences??[]).find(r=>r.proof?.fenceRef===scenario.retainedMarker.id);assert(marker);await verifyFence(marker);}
    }catch(error){receipt.cleanup.push(safeError(error));}
    receipt.passed=!!scenario&&!failure&&receipt.cleanup.length===0&&receipt.cleanupPhases.every(p=>p.status==='completed')&&receipt.startupResources.every(p=>p.state==='closed'&&!p.late&&!p.closeTimedOut)&&receipt.controllersStopped===true&&receipt.dockerRequestsDrained===true;
    receipt.finishedAt=new Date().toISOString();receipt.fixtureRetained=true;receipt.permanentMarkerRemoved=false;
    writeFileSync(report,encodePublic(receipt),{flag:'wx',mode:0o600});journal.record('driver-exit',{exitCode:receipt.passed?0:1});journal.close();
    process.exitCode=receipt.passed?0:1;console.log(JSON.stringify({passed:receipt.passed,report,failedStage:receipt.failedStage??null,permanentMarkerRetained:scenario?.retainedMarker.id??null}));
  }
}
if(process.argv[1]&&resolve(process.argv[1])===fileURLToPath(import.meta.url))await main();
