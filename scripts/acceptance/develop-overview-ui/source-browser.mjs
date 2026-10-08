import assert from 'node:assert/strict';
import {readFileSync,writeFileSync,mkdirSync,mkdtempSync,rmSync} from 'node:fs';
import {execFileSync} from 'node:child_process';
import {createHash} from 'node:crypto';
import {join,dirname,resolve} from 'node:path';
import {homedir} from 'node:os';
import {fileURLToPath,pathToFileURL} from 'node:url';
const here=dirname(fileURLToPath(import.meta.url)),repo=resolve(here,'../../..');
const out=resolve(process.argv[2]||join(repo,'artifacts/quality-swarm/local010/browser'));
const {createServer}=await import(pathToFileURL(join(repo,'desktop/node_modules/vite/dist/node/index.js')).href);
const {chromium}=await import(pathToFileURL(join(repo,'desktop/node_modules/playwright/index.mjs')).href);
assert.equal(process.platform,'linux','This witness uses Linux Chromium; it is not a native Windows test.');
assert.notEqual(process.getuid(),0,'Run Chromium as an ordinary user with its sandbox enabled.');
mkdirSync(out,{recursive:true});
try {readFileSync(join(out,'result.json'));throw Error('Output already contains a receipt; choose a fresh directory to preserve it.');}catch(error){if(error.code!=='ENOENT')throw error;}

const paths=[...new Set(execFileSync('git',['ls-files','--cached','--others','--exclude-standard','desktop','scripts/acceptance/develop-overview-ui'],{cwd:repo,encoding:'utf8'}).trim().split('\n'))].filter(p=>!p.startsWith('desktop/src-tauri/')).sort();
const sha=value=>createHash('sha256').update(value).digest('hex'), fingerprint=()=>Object.fromEntries(paths.map(p=>[p,sha(readFileSync(join(repo,p)))]));
const receipt={passed:false,startedAt:new Date().toISOString(),scope:'Actual AppShell/Develop/ProjectRun/Swarm/Queue/Understanding source at1440/390 with controlled APIs; no installed/backend/model acceptance.',limitations:['No installed-package, native Windows, real backend, Docker, agent or model evidence.','Read-only report/review responses and four project states are controlled fixture callbacks. No mutation is requested. ProjectPlan and ProjectChecks leaves below the run are omitted; AppShell and the run/queue/understanding UI are actual source.','Automated axe and keyboard/geometry checks do not establish full accessibility certification.'],sourceBefore:fingerprint(),checks:[],errors:[]};
receipt.head=execFileSync('git',['rev-parse','HEAD'],{cwd:repo,encoding:'utf8'}).trim();receipt.helperSHA256=sha(readFileSync(fileURLToPath(import.meta.url)));receipt.controlledFixtureSHA256=sha(readFileSync(join(here,'fixture.tsx')));receipt.limitations.push('Focus is awaited after Radix deferred restoration; this Linux source check does not resolve the original Windows LOCAL-008 immediate Escape timing report.');
let browser,server,page;const profileRoot=process.env.LUCI_BROWSER_STATE_ROOT||join(homedir(),'.cache','luci-ui-acceptance');mkdirSync(profileRoot,{recursive:true});const profile=mkdtempSync(join(profileRoot,'task-review-'));
async function checkGeometry(label){
 await page.addScriptTag({path:join(repo,'desktop/node_modules/axe-core/axe.min.js')});const violations=await page.evaluate(async()=> (await axe.run(document,{runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21aa']}})).violations.map(v=>({id:v.id,nodes:v.nodes.map(n=>n.target)})));assert.deepEqual(violations,[]);
 const geometry=await page.evaluate(()=>({documentWidth:document.documentElement.clientWidth,documentScrollWidth:document.documentElement.scrollWidth,dialogs:[...document.querySelectorAll('[role=dialog]')].map(el=>({width:el.clientWidth,scrollWidth:el.scrollWidth,left:el.getBoundingClientRect().left,right:el.getBoundingClientRect().right}))}));assert(geometry.documentScrollWidth<=geometry.documentWidth+1);for(const d of geometry.dialogs){assert(d.scrollWidth<=d.width+1);assert(d.left>=-1&&d.right<=geometry.documentWidth+1);}
 receipt.checks.push({label,geometry,axeViolations:violations});
}
async function keyboardTrap(dialog){await dialog.getByRole('button').last().focus();await page.keyboard.press('Tab');assert(await dialog.evaluate(el=>el.contains(document.activeElement)));await page.keyboard.press('Shift+Tab');assert(await dialog.evaluate(el=>el.contains(document.activeElement)));}
try{
 server=await createServer({configFile:join(repo,'desktop/vite.config.ts'),root:join(repo,'desktop'),server:{host:'127.0.0.1',port:0,strictPort:false,watch:null,fs:{allow:[repo]}},plugins:[{name:'controlled-handoff-fixture',enforce:'pre',resolveId(id,importer){if(id.endsWith('/AuthProvider'))return '\0handoff-auth';if(id.endsWith('/TeamWorkspaceProvider'))return '\0handoff-team';if(id.endsWith('/useProjectAccess'))return '\0handoff-access';if(id.endsWith('/SetupProvider'))return '\0setup';if(importer?.endsWith('/ProjectWork.tsx')&&(id==='./ProjectPlan'||id==='./ProjectChecks'))return '\0planning-'+id;},load(id){if(id==='\0setup')return 'export const useSetup=()=>({workspace:{mode:"local"},status:{access:{user:{username:"fixture"}},config:{projectsDir:"/fixture"}}});';if(id.startsWith('\0planning-'))return 'export const ProjectPlan=()=>null;export const ProjectChecks=()=>null;';if(id==='\0handoff-auth')return 'export const useAuth=()=>({profile:{user:{id:"fixture",username:"fixture",displayName:"Controlled operator",role:"member"},permissions:{}}});';if(id==='\0handoff-team')return 'export const useTeamWorkspace=()=>null; export const useTeamProjectPermissions=()=>({loading:false,error:"",permissions:{read:true,edit:true,execute:true}});';if(id==='\0handoff-scope')return 'export const useProjectDraftScope=id=>"controlled-account:"+id;';if(id==='\0handoff-access')return 'export const useProjectAccess=()=>({canRead:true,canEdit:true,canExecute:true,loading:false,error:"",message:""});';if(id==='\0handoff-queue')return 'export const SwarmRunQueue=()=>null;';},configureServer(vite){vite.middlewares.use('/handoff-fixture',async(req,res)=>{res.setHeader('Content-Type','text/html');res.end(await vite.transformIndexHtml('/handoff-fixture','<!doctype html><html lang="en"><head><meta name="viewport" content="width=device-width,initial-scale=1"><title>Controlled Develop overview fixture</title><style>body{overflow:auto!important;height:auto!important}</style></head><body><div id="root"></div><script type="module" src="/@fs'+join(here,'fixture.tsx')+'"></script></body></html>'));});}}]});await server.listen();const base='http://127.0.0.1:'+server.httpServer.address().port;
 browser=await chromium.launch({headless:true,executablePath:process.env.LUCI_CHROMIUM_EXECUTABLE||'/usr/bin/chromium',env:{PATH:process.env.PATH,TMPDIR:profile,XDG_CONFIG_HOME:join(profile,'config'),XDG_CACHE_HOME:join(profile,'cache')},args:['--no-proxy-server']});receipt.browserVersion=browser.version();
 const context=await browser.newContext({serviceWorkers:'block'});await context.route('**/*',route=>{if(new URL(route.request().url()).origin!==base){receipt.errors.push('External request blocked: '+new URL(route.request().url()).origin);return route.abort();}return route.continue();});
 page=await context.newPage();page.setDefaultTimeout(15000);page.on('pageerror',error=>receipt.errors.push(error.message));
 for(const width of [1440,390]){
  await page.setViewportSize({width,height:width===390?844:1080});await page.goto(base+'/handoff-fixture');
  await page.getByRole('heading',{name:'alpha implementation',exact:true}).waitFor();
  if(width===390)await page.getByRole('button',{name:'Show project overview',exact:true}).click();
  await checkGeometry('full-shell-'+width);
  const card=page.locator('.project-swarm-tasks > li').first();
  const early=page.getByRole('button',{name:'Review tasks',exact:true});
  receipt.checks.push({label:'initial-task-position-'+width,geometry:await card.evaluate(el=>({top:el.getBoundingClientRect().top,height:el.getBoundingClientRect().height})),reviewAction:await early.boundingBox(),overviewOpen:await page.getByRole('button',{name:'Hide project overview',exact:true}).isVisible()});
  await page.screenshot({path:join(out,'full-develop-'+width+'.png')});
  await early.focus();await page.keyboard.press('Enter');
  await page.waitForFunction(()=>document.activeElement?.textContent==='alpha review and follow-up');
  assert.equal(await page.getByRole('dialog').count(),0);
  receipt.checks.push({label:'next-task-visible-'+width,focus:await page.evaluate(()=>({text:document.activeElement?.textContent,top:document.activeElement?.getBoundingClientRect().top,bottom:document.activeElement?.getBoundingClientRect().bottom}))});
  await page.screenshot({path:join(out,'review-task-focused-'+width+'.png')});
  const integrated=page.locator('.project-swarm-tasks > li').first();
  assert.equal(await integrated.getByText(/The agent process ended/).isVisible(),false);
  const scope=integrated.getByText('Task scope and handoff',{exact:true});await scope.focus();await page.keyboard.press('Enter');
  assert(await integrated.getByText(/Worker observation before integration/).isVisible());await scope.click();
  await page.getByRole('button',{name:'Background queue',exact:true}).click();
  const queueSummary=page.getByText(/^Queue status and controls/);assert.equal(await queueSummary.evaluate(el=>el.closest('details').open),false);
  assert.equal(await page.getByRole('button',{name:'Refresh queue',exact:true}).isVisible(),false);
  await queueSummary.focus();await page.keyboard.press('Enter');assert(await page.getByRole('button',{name:'Refresh queue',exact:true}).isVisible());await queueSummary.click();
  receipt.checks.push({label:'keyboard-review-history-and-queue-'+width,mutations:await page.evaluate(()=>window.__developFixture.writes.length)});
  async function select(id){
    if(width===390)await page.getByRole('button',{name:'Projects',exact:true}).click();
    await page.getByRole('treeitem',{name:id,exact:true}).click();await page.getByRole('heading',{name:id+' implementation',exact:true}).waitFor();
    assert.equal(await page.evaluate(()=>window.__developFixture.selected()),id);
  }
  await select('beta');const beta=page.locator('.project-swarm-tasks > li').first();
  assert(await beta.getByText('Awaiting review',{exact:true}).isVisible());assert(await beta.getByText(/1 agent-reported question needs an answer/).isVisible());
  const report=beta.getByRole('button',{name:'Task result and questions',exact:true});await report.click();
  let dialog=page.getByRole('dialog',{name:'Task result and questions',exact:true});await dialog.getByRole('heading',{name:'Questions for the project'}).waitFor();
  await keyboardTrap(dialog);await checkGeometry('task-result-'+width);await page.screenshot({path:join(out,'task-result-'+width+'.png')});
  await page.keyboard.press('Escape');await dialog.waitFor({state:'detached'});
  const immediate=await report.evaluate(el=>document.activeElement===el);await page.waitForFunction(()=>document.activeElement?.textContent?.trim()==='Task result and questions');await page.keyboard.press('Tab');
  receipt.checks.push({label:'deferred-dialog-focus-'+width,immediateAfterDialogDetached:immediate,settled:true,nextKey:await page.evaluate(()=>document.activeElement?.textContent?.trim())});
  const review=beta.getByRole('button',{name:'Review task changes',exact:true});await review.click();dialog=page.getByRole('dialog',{name:'Review task changes',exact:true});
  await dialog.getByRole('checkbox').waitFor();assert(await dialog.getByRole('button',{name:'Integrate reviewed changes'}).isDisabled());
  await keyboardTrap(dialog);await checkGeometry('integration-review-'+width);await page.screenshot({path:join(out,'integration-review-'+width+'.png')});
  await dialog.getByRole('button',{name:'Keep reviewing'}).click();await dialog.waitFor({state:'detached'});await page.waitForFunction(()=>document.activeElement?.textContent?.trim()==='Review task changes');
  await select('gamma');assert(await page.getByRole('button',{name:/^Start ready tasks/}).isEnabled());assert(await page.getByText(/Commands run without interactive approvals/).isVisible());
  await checkGeometry('ready-project-'+width);await page.screenshot({path:join(out,'ready-project-'+width+'.png')});
  await select('delta');assert(await page.locator('.project-swarm-tasks > li').first().getByText(/The runtime outcome is unconfirmed/).isVisible());
  await page.evaluate(()=>window.__developFixture.blockDeltaStart());await page.getByRole('button',{name:'Refresh history',exact:true}).click();await page.getByText('Start blocked',{exact:true}).waitFor();
  const delta=page.locator('.project-swarm-tasks > li').first();assert.equal(await delta.getByRole('button',{name:'Review task changes',exact:true}).count(),0);assert.equal(await delta.getByRole('button',{name:'Task result and questions',exact:true}).count(),0);
  await checkGeometry('blocked-start-'+width);await page.screenshot({path:join(out,'blocked-start-'+width+'.png')});
  await select('alpha');await page.getByRole('button',{name:'Review tasks',exact:true}).waitFor();
  const calls=await page.evaluate(()=>({reads:window.__developFixture.reads,writes:window.__developFixture.writes,starts:window.__developFixture.starts}));assert.equal(calls.starts,0);assert.deepEqual(calls.writes,[]);
  assert(calls.reads.some(r=>r.kind==='review'&&r.projectId==='beta'&&r.runId==='run-beta'&&r.taskId==='a'));assert(calls.reads.some(r=>r.kind==='report'&&r.projectId==='beta'&&r.runId==='run-beta'&&r.operationId==='op-a'));
  receipt.checks.push({label:'four-project-scoped-read-only-navigation-'+width,...calls});

 }
 receipt.passed=true;
}catch(error){receipt.error=String(error.stack||error);if(page)receipt.focusAtFailure=await page.evaluate(()=>({tag:document.activeElement?.tagName,text:document.activeElement?.textContent?.trim().slice(0,200)})).catch(()=>null);if(page)await page.screenshot({path:join(out,'failed-stage.png')}).catch(()=>{});throw error;}
finally{if(browser)await browser.close();if(server)await server.close();rmSync(profile,{recursive:true,force:true});receipt.sourceAfter=fingerprint();receipt.sourceUnchanged=JSON.stringify(receipt.sourceBefore)===JSON.stringify(receipt.sourceAfter);receipt.passed=receipt.passed&&receipt.sourceUnchanged&&receipt.errors.length===0;receipt.finishedAt=new Date().toISOString();receipt.cleanup=['owned Chromium closed','owned Vite server closed','private profile removed'];writeFileSync(join(out,'result.json'),JSON.stringify(receipt,null,2)+'\n');console.log(JSON.stringify({passed:receipt.passed,error:receipt.error,checks:receipt.checks.length,sourceUnchanged:receipt.sourceUnchanged}));if(!receipt.passed)process.exitCode=1;}
