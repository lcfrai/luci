// Actual two-person browser collaboration against an isolated compiled service.
// No provider calls, cloud resources or external email. Credentials stay in memory.
import assert from 'node:assert/strict';
import {spawn} from 'node:child_process';
import {randomUUID,createHash} from 'node:crypto';
import {mkdir,readFile,writeFile} from 'node:fs/promises';
import {createServer} from 'node:net';
import path from 'node:path';
import {setTimeout as delay} from 'node:timers/promises';
import {chromium} from '../desktop/node_modules/playwright/index.mjs';
import axe from '../desktop/node_modules/axe-core/axe.js';
const repo=path.resolve(import.meta.dirname,'..');
const binary=path.resolve(process.argv[2]),web=path.resolve(process.argv[3]);
const fixture=path.resolve(process.argv[4]||path.join(repo,'artifacts/uat',`team-${randomUUID()}`));
assert.match(fixture.replaceAll('\\','/'),/\/artifacts\/uat\/[^/]+$/);
await mkdir(fixture,{mode:0o700});
const evidence=path.join(fixture,'evidence');await mkdir(evidence,{mode:0o700});
const listener=createServer();await new Promise(done=>listener.listen(0,'127.0.0.1',done));const address=`127.0.0.1:${listener.address().port}`;await new Promise(done=>listener.close(done));
const base=`http://${address}`,lifecycle=randomUUID()+randomUUID();
const environment={...Object.fromEntries(Object.entries(process.env).filter(([key])=>!key.startsWith('LUCI_'))),LUCI_ADDR:address,LUCI_STATE_DIR:path.join(fixture,'state'),LUCI_PROJECTS_DIR:path.join(fixture,'projects'),LUCI_WEB_DIR:web,LUCI_RUNTIME_DIR:repo,LUCI_MANAGED_SHUTDOWN_TOKEN:lifecycle,LUCI_DOCKER_HOST:'tcp://127.0.0.1:1',DOCKER_HOST:'tcp://127.0.0.1:1',OPENAI_API_KEY:'',ANTHROPIC_API_KEY:'',GIT_CONFIG_NOSYSTEM:'1',GIT_CONFIG_GLOBAL:path.join(fixture,'no-git-config')};
const binarySha256=createHash('sha256').update(await readFile(binary)).digest('hex');
const webIndexSha256=createHash('sha256').update(await readFile(path.join(web,'index.html'))).digest('hex');
const receipt={passed:false,binary,binarySha256,web,webIndexSha256,checks:[],startedAt:new Date().toISOString(),externalModels:false,cloudResources:false};
let service,browser,alice,bob;const errors=[];
const check=label=>receipt.checks.push(label);
async function api(method,route,body,token,expected=200){const response=await fetch(base+route,{method,headers:{'Content-Type':'application/json',...(token?{Authorization:`Bearer ${token}`}:{})},body:body===undefined?undefined:JSON.stringify(body)});assert.equal(response.status,expected,`${method} ${route}`);return response.status===204?null:response.json();}
async function start(){service=spawn(binary,[],{cwd:repo,env:environment,stdio:'ignore'});for(let i=0;i<200;i++){try{const s=await api('GET','/v1/control-plane/status');receipt.version=s.version;return;}catch{await delay(100);}}throw new Error('Service not ready');}
async function stop(){if(service?.exitCode===null){await fetch(base+'/v1/control-plane/shutdown',{method:'POST',headers:{'X-Luci-Lifecycle-Token':lifecycle}});for(let i=0;i<150&&service.exitCode===null;i++)await delay(100);assert.equal(service.exitCode,0);}}
async function login(page,username,password){await page.goto(base);await page.getByLabel('Username',{exact:true}).fill(username);await page.getByLabel('Password',{exact:true}).fill(password);await page.getByRole('button',{name:'Sign in',exact:true}).click();await page.getByRole('button',{name:'Manage teams',exact:true}).waitFor();}
async function contextPage(){const context=await browser.newContext({viewport:{width:1440,height:1000}});const page=await context.newPage();page.setDefaultTimeout(20000);page.on('pageerror',error=>errors.push(error.message));return page;}
async function saveDefinition(page){const response=page.waitForResponse(r=>r.request().method()==='POST'&&r.url().endsWith('/specification'));await page.getByRole('button',{name:'Save definition',exact:true}).click();const result=await response;assert.equal(result.status(),200);return result.json();}
try {
 await start();
 const password=randomUUID()+randomUUID();
 const bootstrap=(await readFile(path.join(fixture,'state/control-plane/bootstrap-token'),'utf8')).trim();
 const owner=await api('POST','/v1/auth/bootstrap',{bootstrapToken:bootstrap,username:'alice',displayName:'Alice Owner',password},undefined,201);
 const colleague=await api('POST','/v1/admin/users',{username:'bob',displayName:'Bob Editor',password},owner.token,201);
 // Existing personal projects bypass first-visit host preparation; all team operations below use actual UI.
 await api('POST','/v1/product/projects',{name:'alice-personal'},owner.token,201);
 const bobLogin=await api('POST','/v1/auth/login',{username:'bob',password});
 await api('POST','/v1/product/projects',{name:'bob-personal'},bobLogin.token,201);
 browser=await chromium.launch({headless:true,...(process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE?{executablePath:process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE}:{})});receipt.browserVersion=browser.version();alice=await contextPage();bob=await contextPage();
 await login(alice,'alice',password);await login(bob,'bob',password);
 await alice.getByRole('button',{name:'Manage teams',exact:true}).click();await alice.getByLabel('Team name',{exact:true}).fill('Luci collaboration');await alice.getByRole('button',{name:'Create team',exact:true}).click();
 await alice.getByRole('button',{name:'Team access',exact:true}).waitFor();
 const team=(await api('GET','/v1/teams',undefined,owner.token)).teams[0].team;
 assert.equal(await alice.getByLabel('Workspace',{exact:true}).inputValue(),team.id);
 assert.equal(await alice.getByRole('treeitem',{name:/alice-personal/}).count(),0);check('Created and selected a team through UI; personal project did not leak into its catalogue');
 await alice.getByRole('button',{name:'New project',exact:true}).click();const create=alice.getByRole('dialog',{name:'Create a project',exact:true});await create.getByLabel('Project name',{exact:true}).fill('shared-atlas');await create.getByRole('button',{name:'Create project',exact:true}).click();
 await alice.getByRole('heading',{name:'Define your project',exact:true}).waitFor();
 const workspace=`/v1/teams/${team.id}/workspace`;
 const project=(await api('GET',workspace+'/v1/projects',undefined,owner.token)).projects[0];
 await alice.getByRole('button',{name:'Team access',exact:true}).click();
 await alice.getByText('Add an existing member',{exact:true}).click();const members=alice.getByRole('group',{name:'Add an existing Luci account',exact:true});await members.getByLabel('Username',{exact:true}).fill('bob');await members.getByLabel('Role',{exact:true}).selectOption('editor');await members.getByRole('button',{name:'Add member',exact:true}).click();await alice.getByLabel('Team access for Bob Editor',{exact:true}).waitFor();await alice.getByRole('tab',{name:'Project sharing',exact:true}).click();
 const share=alice.getByRole('group',{name:'Grant project access',exact:true});await share.getByLabel('Team member',{exact:true}).selectOption(colleague.id);await share.getByLabel('Project role',{exact:true}).selectOption('editor');await share.getByRole('button',{name:'Share project',exact:true}).click();await alice.getByLabel('Project access for Bob Editor',{exact:true}).waitFor();await alice.getByRole('button',{name:'Close',exact:true}).click();check('Added an editor and explicitly shared one project through real membership UI');
 await bob.reload();await bob.getByLabel('Workspace',{exact:true}).selectOption(team.id);await bob.getByRole('treeitem',{name:/shared-atlas$/}).click();await bob.getByRole('button',{name:'Define',exact:true}).click();
 const answers=[['What should this solution do?','A shared atlas of project decisions.'],['Who will use it?','Two collaborating developers.'],['What does it need to respect?','Retain edits and attribute reviewed changes.'],['What should we leave out?','Paid execution in this test']];
 for(const [label,value] of answers){await bob.getByRole('textbox',{name:label,exact:true}).fill(value);await bob.getByRole('button',{name:'Next question',exact:true}).click();}
 const outcome={title:'Shared notes survive restart',acceptance:'Save a shared note, restart the app and confirm both collaborators can read it.'};
 await bob.getByRole('button',{name:'Add an outcome',exact:true}).click();await bob.getByRole('textbox',{name:'Outcome 1',exact:true}).fill(outcome.title);await bob.getByRole('textbox',{name:'How to check outcome 1',exact:true}).fill(outcome.acceptance);await bob.getByRole('button',{name:'Review specification',exact:true}).click();
 const first=await saveDefinition(bob);assert.equal(first.revision,1);assert.deepEqual(first.requirements.map(({title,acceptance})=>({title,acceptance})),[outcome]);check('A separate browser identity saved shared definition with independent editing permission');
 await alice.reload();await alice.getByRole('button',{name:'Define',exact:true}).click();await alice.getByRole('tab',{name:'Specification',exact:true}).click();await alice.getByRole('textbox',{name:'The idea',exact:true}).waitFor();assert.equal(await alice.getByRole('textbox',{name:'The idea',exact:true}).inputValue(),answers[0][1]);
 await bob.getByRole('textbox',{name:'The idea',exact:true}).fill('Bob retained edit during concurrent collaboration.');
 await alice.getByRole('textbox',{name:'The idea',exact:true}).fill('Alice saved the newer shared definition.');await saveDefinition(alice);
 const conflict=bob.waitForResponse(r=>r.request().method()==='POST'&&r.url().endsWith('/specification'));await bob.getByRole('button',{name:'Save definition',exact:true}).click();assert.equal((await conflict).status(),409);assert.equal(await bob.getByRole('textbox',{name:'The idea',exact:true}).inputValue(),'Bob retained edit during concurrent collaboration.');
 await bob.getByRole('button',{name:'Reload saved definition',exact:true}).click();await bob.getByRole('region',{name:'Definition draft recovery',exact:true}).waitFor();
 await bob.screenshot({path:path.join(evidence,'conflict-recovery.png'),fullPage:true});check('Concurrent save returned 409 and retained the editor draft for explicit recovery');
 // Reopen the app with unsaved work: actual scoped browser draft is offered, never silently replaces newer context.
 await bob.reload();await bob.getByRole('button',{name:'Define',exact:true}).click();await bob.getByRole('region',{name:'Definition draft recovery',exact:true}).waitFor();check('Reload preserved the team-scoped definition draft and offered recovery against newer context');
 await alice.getByRole('button',{name:'Team access',exact:true}).click();await alice.getByLabel('Team access for Bob Editor',{exact:true}).click();const bobMember=alice.getByRole('group',{name:'Bob Editor',exact:true}).first();await bobMember.getByLabel('Role',{exact:true}).selectOption('viewer');await bobMember.getByRole('button',{name:'Save access',exact:true}).click();await alice.getByRole('button',{name:'Close',exact:true}).click();
 await bob.reload();await bob.getByRole('button',{name:'Define',exact:true}).click();await bob.locator('.definition-workspace').getByText(/Read-only access/).waitFor();assert.equal(await bob.getByRole('button',{name:'Approve definition',exact:true}).isEnabled(),false);check('Downgraded collaborator sees read-only controls and cannot approve or run agents');
 const deploymentTraffic=[];
 const recordDeploymentRequest=request=>{const url=new URL(request.url());if(url.origin===base&&url.pathname.startsWith(workspace+'/'))deploymentTraffic.push({method:request.method(),path:url.pathname});};
 bob.on('request',recordDeploymentRequest);
 try {
  await bob.getByRole('button',{name:'Deploy',exact:true}).click();await bob.getByRole('tab',{name:'Previews and publishing',exact:true}).click();
  await bob.getByText('Deployment access is required to build, stop, check health or roll back releases. Ask a team owner for editing and deployment access.',{exact:true}).waitFor();
  assert.equal(await bob.getByRole('button',{name:'Build local preview',exact:true}).isEnabled(),false);
  const refresh=bob.getByRole('button',{name:'Refresh',exact:true});assert.equal(await refresh.isEnabled(),true);
  const historyResponse=bob.waitForResponse(response=>response.request().method()==='GET'&&new URL(response.url()).pathname===`${workspace}/v1/product/projects/${project.id}/releases`);
  await refresh.click();const refreshed=await historyResponse;assert.equal(refreshed.status(),200);assert.deepEqual((await refreshed.json()).releases,[]);
  // Observe beyond the deployment-health polling interval without creating a
  // release or contacting Docker. Pending-release polling is covered separately.
  await delay(5500);
  assert.deepEqual(deploymentTraffic.filter(request=>!['GET','HEAD','OPTIONS'].includes(request.method)),[],'Viewer navigation or history refresh attempted a workspace mutation');
  await bob.screenshot({path:path.join(evidence,'viewer-deployment-permissions.png'),fullPage:true});
  receipt.viewerDeployment={history:'empty',observationMs:5500,requests:deploymentTraffic};
  check('Viewer sees disabled local preview with an owner-grant remedy, can GET empty release history, and sends no deployment mutation or health POST during navigation and refresh');
 } finally {bob.off('request',recordDeploymentRequest);}
 await alice.getByRole('button',{name:'Define',exact:true}).click();await alice.getByRole('button',{name:'Approve definition',exact:true}).click();
 const history=await api('GET',`${workspace}/v1/product/projects/${project.id}/context-history`,undefined,owner.token);assert.ok(history.events.some(row=>row.actor.id===colleague.id));assert.ok(history.events.some(row=>row.actor.id===owner.user.id&&row.action==='approve'));check('Private saved history attributes Bob’s change and Alice’s approval to their real authenticated identities');
 await alice.getByRole('button',{name:'Team access',exact:true}).click();await alice.waitForFunction(()=>getComputedStyle(document.querySelector('[role="dialog"]')).opacity==='1');await alice.evaluate(axe.source);const audit=await alice.evaluate(async()=>window.axe.run(document.querySelector('[role="dialog"]'),{runOnly:['wcag2a','wcag2aa','wcag21aa']}));await writeFile(path.join(evidence,'team-accessibility.json'),JSON.stringify(audit.violations,null,2));assert.deepEqual(audit.violations.map(v=>v.id),[]);await alice.screenshot({path:path.join(evidence,'team-access.png'),fullPage:true});check('Team access dialog passed bounded WCAG A/AA accessibility check');
 await alice.getByLabel('Team access for Bob Editor',{exact:true}).click();const removing=alice.getByRole('group',{name:'Bob Editor',exact:true}).first();await removing.getByRole('button',{name:'Remove access',exact:true}).click();await removing.getByRole('button',{name:'Confirm removal',exact:true}).click();await alice.getByRole('button',{name:'Close',exact:true}).click();
 await bob.reload();await bob.getByRole('heading',{name:'Workspace unavailable',exact:true}).waitFor();await api('GET',`${workspace}/v1/product/projects/${project.id}/brief`,undefined,bobLogin.token,403);check('Removed member receives explicit unavailable-team recovery and server immediately denies access');
 await stop();await start();await alice.reload();await alice.getByRole('button',{name:'Define',exact:true}).click();await alice.getByText(/Approved · Revision/).waitFor();check('Main service and team worker restart preserve membership, selected workspace and reviewed context');
 assert.deepEqual(errors,[]);receipt.passed=true;
} catch(error){receipt.error=String(error);for(const [name,page] of [['alice',alice],['bob',bob]])if(page){await page.screenshot({path:path.join(evidence,name+'-failure.png'),fullPage:true}).catch(()=>{});await writeFile(path.join(evidence,name+'-failure-dom.txt'),await page.locator('body').innerText()).catch(()=>{});}throw error;}
finally{await browser?.close();try{await stop();}catch(error){receipt.cleanupError=String(error);service?.kill();}await writeFile(path.join(evidence,'result.json'),JSON.stringify(receipt,null,2));console.log(JSON.stringify(receipt));}
