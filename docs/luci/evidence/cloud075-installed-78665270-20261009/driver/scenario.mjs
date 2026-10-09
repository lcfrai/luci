// Real installed API/UI state. Only permission-read transport is delayed;
// neither authoritative bodies nor DOM/store/auth state are fabricated.
import assert from 'node:assert/strict';
import {setTimeout as delay} from 'node:timers/promises';
import {emptyGrants,assertEmptySessions,budgets} from './guards.mjs';
import {sha} from './capture.mjs';
import {clickRailControl} from './navigation.mjs';
import {purposeControl} from './purpose.mjs';
export async function runScenario({page,context,base,request,owner,member,team,projects,record,health,fail,trackRoute,onCleanup,onPhase}){
 const workspace=`/v1/teams/${team.id}/workspace`,permissionPath=id=>`/v1/teams/${team.id}/projects/${id}/members`;
 const reads=[],mutations=[],activeRoutes=new Set(),requestFailures=new Map();let arm=null,held=null,gateTimer=null;
 const until=async(label,fn,ms=12000)=>{const end=Date.now()+ms;while(Date.now()<end){health();if(await fn())return;await delay(50);}throw Error(label);};
 const finishGate=()=>{clearTimeout(gateTimer);held?.release();};
 onCleanup(()=>{finishGate();});
 const onRequestFailed=request=>{if(projects.some(project=>permissionPath(project.id)===new URL(request.url()).pathname))requestFailures.set(request,request.failure()?.errorText??'unknown');};
 page.on('requestfailed',onRequestFailed);
 const routeHandler=async route=>{
  const task=(async()=>{
   const req=route.request(),path=new URL(req.url()).pathname;
   if(req.method()!=='GET'||!projects.some(project=>permissionPath(project.id)===path))return route.fallback();
   health();const project=projects.find(project=>permissionPath(project.id)===path),row={projectId:project.id,path,at:Date.now(),phase:arm?.name??'unarmed',status:null,delivery:null};reads.push(row);assert(reads.length<=80);record({kind:'project-permission-read',...row});
   if(!arm||arm.projectId!==project.id)return route.fallback();
   const selected=arm;const response=await route.fetch({timeout:budgets.httpMs}),body=await response.body();health();assert(body.length<=budgets.bodyBytes);row.status=response.status();row.bodySHA256=sha(body);
   if(response.ok()){const value=JSON.parse(body);assert.equal(value.team.id,team.id);assert.equal(value.projectId,project.id);assert.equal(value.permissions.read,true);}
   if(selected.manual){
    assert.equal(held,null,'Only one shared permission read should reach the manual gate');let release;const ready=new Promise(done=>{release=done;});held={row,release,expiresAt:Date.now()+budgets.gateMs,released:false};gateTimer=setTimeout(()=>{fail(Error('Manual authority response gate expired'));release();},budgets.gateMs);record({kind:'explicit-read-transport-gate',phase:selected.name,status:row.status,bodySHA256:row.bodySHA256,manual:true});await ready;clearTimeout(gateTimer);held.released=true;
   }else{record({kind:'explicit-read-transport-gate',phase:selected.name,status:row.status,bodySHA256:row.bodySHA256,delayMs:300});await delay(300);}
   health();try{await route.fulfill({response,body});row.delivery='fulfilled';}catch(error){row.delivery='unconfirmed';if(!selected.manual||!held?.released)throw error;await until('Failed held delivery lacked an actual request-cancellation event',()=>requestFailures.has(req),1000);assert.equal(requestFailures.get(req),'net::ERR_ABORTED','Held delivery failure was not an observed exact request cancellation');row.delivery='cancelled-after-authority-invalidation';}
   record({kind:'project-permission-read-delivery',...row});
  })();activeRoutes.add(task);trackRoute(task);try{await task;}catch(error){fail(error);await route.abort('failed').catch(()=>{});}finally{activeRoutes.delete(task);}
 };
 const observeMutation=req=>{if(!['GET','HEAD','OPTIONS'].includes(req.method())){const row={method:req.method(),path:new URL(req.url()).pathname};mutations.push(row);record({kind:'browser-mutation-or-probe',...row});}};
 page.on('request',observeMutation);await context.route(base+`/v1/teams/${team.id}/projects/*/members`,routeHandler);
 const home=()=>clickRailControl({page,teamId:team.id,name:'All projects',health,record});
 const visiblePurpose=project=>purposeControl(page,team.id,project.id);
 const checkScope=async project=>{assert.equal(await visiblePurpose(project).inputValue(),project.purpose);const visible=await page.locator('body').innerText();for(const other of projects)if(other.id!==project.id)assert(!visible.includes(other.purpose),'Another project purpose was shown in the selected workspace');};
 const oneRead=async(project,name,action)=>{
  await until('Previous permission read still active',()=>activeRoutes.size===0);const start=reads.length;arm={projectId:project.id,name,manual:false};
  await action();await visiblePurpose(project).waitFor();await checkScope(project);await until('Permission transport did not settle',()=>activeRoutes.size===0);
  const phase=reads.slice(start);assert.equal(phase.length,1,'Current Define entry should share one overlapping permission GET');assert.equal(phase[0].projectId,project.id);assert.equal(phase[0].status,200);record({kind:'current-project-one-read',phase:name,projectId:project.id,count:phase.length,scopedPurposeSHA256:sha(project.purpose),transportDelayedMs:300});arm=null;
 };
 const teamChange=async restore=>{const current=await request('GET','/v1/teams/'+team.id,undefined,200,owner.token);return restore?request('POST',`/v1/teams/${team.id}/members`,{expectedRevision:current.team.revision,username:member.username,role:'editor',grants:emptyGrants},200,owner.token):request('DELETE',`/v1/teams/${team.id}/members/${member.id}`,{expectedRevision:current.team.revision},200,owner.token);};
 try{
  onPhase('real-ui-member-sign-in');await page.goto(base,{waitUntil:'domcontentloaded'});await page.getByLabel('Username',{exact:true}).fill(member.username);await page.getByLabel('Password',{exact:true}).fill(member.password);await page.getByRole('button',{name:'Sign in',exact:true}).click();
  const entry=page.getByRole('button',{name:'Continue to projects',exact:true});await until('Initial account entry did not settle',async()=>await entry.isVisible()||await page.getByLabel('Workspace',{exact:true}).isVisible());if(await entry.isVisible())await entry.click();
  await page.getByLabel('Workspace',{exact:true}).selectOption(team.id);await home();await page.getByRole('button',{name:'Open project '+projects[0].name,exact:true}).waitFor();
  assertEmptySessions(await request('GET',workspace+'/v1/sessions',undefined,200,owner.token));record({kind:'measurement-boundary',initialAccountReadinessExcluded:true,projectCount:4});
  onPhase('four-current-project-permission-reads');for(const project of projects){await home();await oneRead(project,'select-'+project.name,()=>page.getByRole('button',{name:'Open project '+project.name,exact:true}).click());}
  onPhase('same-project-browser-refresh');const selected=projects[3];await oneRead(selected,'same-project-browser-reload',()=>page.reload({waitUntil:'domcontentloaded'}));
  // This real transport race is deliberately coarser than the component test's
  // promise-resolution microtask ordering. It must not be labelled its replay.
  onPhase('unpublished-real-grant-held');await home();await until('Previous permission request pending',()=>activeRoutes.size===0);const target=projects[0];held=null;arm={projectId:target.id,name:'pre-denial-held-grant',manual:true};
  await page.getByRole('button',{name:'Open project '+target.name,exact:true}).click();await until('No real grant reached response gate',()=>held!==null);assert.equal(held.row.status,200);assert.equal(await visiblePurpose(target).isVisible(),false);
  onPhase('real-team-authority-revoked');await teamChange(false);await page.getByRole('heading',{name:'Workspace unavailable',exact:true}).waitFor({timeout:12000});assert(!((await page.locator('body').innerText()).includes(target.purpose)));record({kind:'workspace-denial-before-grant-delivery',targetProjectId:target.id,privatePurposeVisible:false});
  finishGate();await until('Stale response route did not settle',()=>activeRoutes.size===0);arm=null;
  onPhase('membership-restored-without-project-grants');await teamChange(true);await page.getByRole('button',{name:'Retry connection',exact:true}).click();await until('Workspace membership recovery did not settle',async()=>!(await page.getByRole('heading',{name:'Workspace unavailable',exact:true}).isVisible()));
  const body=await page.locator('body').innerText();for(const project of projects)assert(!body.includes(project.purpose),'Old project purpose became visible without a current grant');
  await request('GET',workspace+`/v1/product/projects/${target.id}/specification`,undefined,403,member.token);record({kind:'no-stale-project-content-after-membership-only-recovery',projectCount:4,realMemberSpecificationStatus:403,heldResponseDelivery:held.row.delivery});
  onPhase('fresh-project-grant-restored');const current=await request('GET','/v1/teams/'+team.id,undefined,200,owner.token);await request('PUT',`/v1/teams/${team.id}/projects/${target.id}/members/${member.id}`,{expectedRevision:current.team.revision,role:'editor',grants:emptyGrants},200,owner.token);
  await page.reload({waitUntil:'domcontentloaded'});await home();await page.getByRole('button',{name:'Open project '+target.name,exact:true}).waitFor();await oneRead(target,'restored-current-project',()=>page.getByRole('button',{name:'Open project '+target.name,exact:true}).click());
  assertEmptySessions(await request('GET',workspace+'/v1/sessions',undefined,200,owner.token));
  const permitted=mutations.every(row=>row.method==='POST'&&(row.path==='/v1/auth/login'||row.path.endsWith('/v1/setup/check')||row.path==='/v1/setup/dependencies/check'));assert(permitted,'Unexpected browser write');
  return {passed:true,scope:'Installed Linux web/API; four isolated project selections; later same-project full browser refresh; real delayed permission body across membership removal/recovery; exact unpublished microtask race remains source-test-only',permissionReads:reads,allowedBrowserMutationsAndReadinessProbes:mutations,agentSessionsBefore:0,agentSessionsAfter:0,agentOrCheckDispatches:0,providerCalls:0,dockerEndpoint:'private-absent',transportGating:{normalReadDelayMs:300,manualPreDenial:true},latencyAcceptance:false,nativeDesktopAcceptance:false};
 }finally{finishGate();page.off('request',observeMutation);await context.unroute(base+`/v1/teams/${team.id}/projects/*/members`,routeHandler);await Promise.allSettled([...activeRoutes]);page.off('requestfailed',onRequestFailed);onCleanup(null);}
}
