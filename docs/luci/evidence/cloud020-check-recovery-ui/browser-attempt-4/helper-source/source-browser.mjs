// Derived from the retained task-review-polish and develop-overview-ui witnesses.
// Actual application components; controlled check APIs. No execution backend.
import assert from 'node:assert/strict';
import {readFileSync,writeFileSync,mkdirSync,mkdtempSync,rmSync,existsSync} from 'node:fs';
import {execFileSync} from 'node:child_process';
import {createHash} from 'node:crypto';
import {join,dirname,resolve} from 'node:path';
import {homedir} from 'node:os';
import {fileURLToPath,pathToFileURL} from 'node:url';
const here=dirname(fileURLToPath(import.meta.url));
const repo=resolve(process.env.LUCI_CHECK_UI_REPO||'/workspace/luci');
const manifestPath=resolve(process.env.LUCI_CLOUD020_MANIFEST||join(here,'../staging-manifest.json'));
assert.equal(process.env.LUCI_CLOUD020_BROWSER_RELEASE,'1','Run only in the root-coordinated browser slot.');
assert(process.argv[2],'Pass a fresh output directory; no default evidence location is reused.');
const out=resolve(process.argv[2]);
assert(!existsSync(out),'Choose a fresh output directory to preserve prior evidence.');
assert.equal(process.platform,'linux','This fixture witness uses Linux Chromium, not native Windows.');
assert.notEqual(process.getuid(),0,'Use an ordinary user with Chromium sandbox enabled.');
const sha=value=>createHash('sha256').update(value).digest('hex');
const manifest=JSON.parse(readFileSync(manifestPath,'utf8'));
for(const [path,digest] of Object.entries(manifest.staged)) assert.equal(sha(readFileSync(join(repo,path))),digest,'Frozen staged source differs: '+path);
const fixturePath=join(repo,'scripts/acceptance/cloud020-check-ui/fixture.tsx');
const fixtureSource=readFileSync(join(here,'fixture.tsx'),'utf8');
const {createServer}=await import(pathToFileURL(join(repo,'desktop/node_modules/vite/dist/node/index.js')).href);
const {chromium}=await import(pathToFileURL(join(repo,'desktop/node_modules/playwright/index.mjs')).href);
const paths=[...new Set(execFileSync('git',['ls-files','--cached','--others','--exclude-standard','desktop'],{cwd:repo,encoding:'utf8'}).trim().split('\n'))].filter(p=>p&&!p.startsWith('desktop/src-tauri/')).sort();
const fingerprint=()=>Object.fromEntries(paths.map(p=>[p,sha(readFileSync(join(repo,p)))]));
mkdirSync(out,{recursive:true,mode:0o700});
const receipt={passed:false,startedAt:new Date().toISOString(),scope:'Actual AppShell/Develop/ProjectChecks/CheckRecoveryDialog/WorkflowDialog/shadcn source at 1440 and 390 pixels; controlled check API fixture.',limitations:['No installed/native Windows, real HTTP backend, Docker, cloud, agent or model evidence.','Fixture responses and check-recovery POST callbacks are controlled; no resources are created.','ProjectPlan is the only omitted Develop leaf. Providers supply controlled account/team/setup values; check drafts, access hook, shell and dialogs are actual source.','Visibility events are controlled DOM observations; this does not prove OS background-window behavior.','Axe and keyboard/geometry checks are scoped, not full accessibility certification.','Settled focus restoration is measured after Radix cleanup; this does not settle native Windows immediate-Escape timing.'],head:execFileSync('git',['rev-parse','HEAD'],{cwd:repo,encoding:'utf8'}).trim(),sourceBefore:fingerprint(),frozenStagedFiles:manifest.staged,sharedGoldenSHA256:manifest.sharedGoldenSHA256,helperSHA256:sha(readFileSync(fileURLToPath(import.meta.url))),controlledFixtureSHA256:sha(fixtureSource),checks:[],errors:[]};
let browser,server,page,profile;
const record=value=>{receipt.checks.push(value);console.log(JSON.stringify({checkpoint:value.label}));};
const snapshot=()=>page.evaluate(()=>window.__checkFixture.snapshot());
const region=()=>page.locator('.project-checks');
const dialog=()=>page.getByRole('dialog',{name:'Review check recovery',exact:true});
const openRecovery=()=>region().getByRole('button',{name:'Review check recovery',exact:true}).first();
async function go(base,mode,width){
 await page.setViewportSize({width,height:width===390?844:1080});
 await page.goto(base+'/check-fixture?mode='+mode);
 await region().getByRole('heading',{name:'Verify the implementation',exact:true}).waitFor({timeout:60000});
 await region().locator('.check-runs > li').first().waitFor();
 await region().scrollIntoViewIfNeeded();
}
async function keyboardTrap(panel){
 await panel.getByRole('button',{name:'Keep inspecting',exact:true}).focus();
 await page.keyboard.press('Shift+Tab');assert(await panel.evaluate(el=>el.contains(document.activeElement)));
 await page.keyboard.press('Tab');assert(await panel.evaluate(el=>el.contains(document.activeElement)));
}
async function geometry(label){
 await page.addScriptTag({path:join(repo,'desktop/node_modules/axe-core/axe.min.js')});
 const violations=await page.evaluate(async()=> (await window.axe.run(document,{runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21aa']}})).violations.map(v=>({id:v.id,nodes:v.nodes.map(n=>n.target)})));
 const values=await page.evaluate(()=>({viewport:document.documentElement.clientWidth,document:document.documentElement.scrollWidth,dialogs:[...document.querySelectorAll('[role=dialog]')].map(el=>({width:el.clientWidth,scrollWidth:el.scrollWidth,left:el.getBoundingClientRect().left,right:el.getBoundingClientRect().right})),checks:[...document.querySelectorAll('.project-checks')].map(el=>({width:el.clientWidth,scrollWidth:el.scrollWidth}))}));
 record({label,geometry:values,axeViolations:violations});
 assert.deepEqual(violations,[],'Axe violations at '+label);assert(values.document<=values.viewport+1,'Document overflow at '+label);
 for(const d of values.dialogs){assert(d.scrollWidth<=d.width+1,'Dialog overflow at '+label);assert(d.left>=-1&&d.right<=values.viewport+1,'Dialog outside viewport at '+label);}
 for(const c of values.checks)assert(c.scrollWidth<=c.width+1,'Check region overflow at '+label);
 await page.screenshot({path:join(out,label+'.png')});
}
async function consentAndSubmit(){
 const panel=dialog(),submit=panel.getByRole('button',{name:'Recover this exact check',exact:true});
 await panel.getByRole('checkbox',{name:/I authorize recovery of this exact check command/}).waitFor();
 assert(await submit.isDisabled());assert.equal((await snapshot()).writes.length,0);
 await panel.getByRole('checkbox',{name:/I authorize recovery of this exact check command/}).check();
 await submit.dblclick();
}
async function assertExactWrite(label){
 const calls=await snapshot();assert.equal(calls.starts,0);assert.equal(calls.writes.length,1);
 assert.equal(calls.writes[0].kind,'recover');assert.equal(calls.writes[0].projectId,'alpha');assert.equal(calls.writes[0].checkId,'check-alpha');
 assert.deepEqual({revision:calls.writes[0].review.revision,recoveryDigest:calls.writes[0].review.recoveryDigest,method:calls.writes[0].review.method,commandIndex:calls.writes[0].review.commandIndex},{revision:4,recoveryDigest:'d'.repeat(64),method:'remove-exact',commandIndex:0});
 record({label,...calls});
}
try{
 server=await createServer({configFile:join(repo,'desktop/vite.config.ts'),root:join(repo,'desktop'),server:{host:'127.0.0.1',port:0,strictPort:false,watch:null,fs:{allow:[repo]}},plugins:[{name:'controlled-cloud020-check-fixture',enforce:'pre',resolveId(id,importer){
   if(id===fixturePath||id==='/@fs'+fixturePath)return fixturePath;
   if(id.endsWith('/AuthProvider'))return '\0check-auth';
   if(id.endsWith('/TeamWorkspaceProvider'))return '\0check-team';
   if(id.endsWith('/SetupProvider'))return '\0check-setup';
   if(importer?.endsWith('/ProjectWork.tsx')&&id==='./ProjectPlan')return '\0check-plan';
  },load(id){
   if(id===fixturePath)return fixtureSource;
   if(id==='\0check-plan')return 'export const ProjectPlan=()=>null;';
   if(id==='\0check-auth')return 'export const useAuth=()=>({profile:{user:{id:"controlled-operator",username:"fixture",displayName:"Controlled operator",role:"member"},permissions:{}},openAccount(){},openAdmin(){}});';
   if(id==='\0check-team')return 'export const useTeamWorkspace=()=>null;export const useTeamProjectPermissions=()=>({loading:false,error:"",permissions:{read:true,edit:true,execute:true,secrets:true}});';
   if(id==='\0check-setup')return 'export const useSetup=()=>({workspace:{mode:"local"},status:{access:{user:{username:"fixture"}},config:{projectsDir:"/controlled"}},openSettings(){},openAgents(){}});';
  },configureServer(vite){vite.middlewares.use('/check-fixture',async(_req,res)=>{res.setHeader('Content-Type','text/html');res.end(await vite.transformIndexHtml('/check-fixture','<!doctype html><html lang="en"><head><meta name="viewport" content="width=device-width,initial-scale=1"><title>Controlled CLOUD-020 check recovery fixture</title></head><body><div id="root"></div><script type="module" src="/@fs'+fixturePath+'"></script></body></html>'));});}}]});
 await server.listen();const base='http://127.0.0.1:'+server.httpServer.address().port;
 const profileRoot=process.env.LUCI_BROWSER_STATE_ROOT||join(homedir(),'.cache','luci-ui-acceptance');mkdirSync(profileRoot,{recursive:true});profile=mkdtempSync(join(profileRoot,'cloud020-check-'));
 browser=await chromium.launch({headless:true,chromiumSandbox:true,executablePath:process.env.LUCI_CHROMIUM_EXECUTABLE||'/usr/bin/chromium',env:{PATH:process.env.PATH,TMPDIR:profile,XDG_CONFIG_HOME:join(profile,'config'),XDG_CACHE_HOME:join(profile,'cache')},args:['--no-proxy-server']});receipt.browserVersion=browser.version();
 const context=await browser.newContext({serviceWorkers:'block'});
 await context.route('**/*',route=>{const url=new URL(route.request().url());if(url.origin!==base){receipt.errors.push('External request blocked: '+url.origin);return route.abort();}return route.continue();});
 page=await context.newPage();page.setDefaultTimeout(15000);await page.addInitScript(()=>localStorage.clear());page.on('pageerror',error=>receipt.errors.push(error.message));
 for(const width of [1440,390]){
  await go(base,'overview',width);
  const unknown=region().locator('.check-runs > li').filter({hasText:'Alpha cleanup is unconfirmed; inspect its original allocation.'});
  const failed=region().locator('.check-runs > li').filter({hasText:'Alpha retained failed check.'});
  await unknown.locator('details > summary').last().click();await failed.locator('details > summary').last().click();
  assert(await unknown.getByText('Alpha retained check output: 7 tests passed; cleanup remains unconfirmed.',{exact:true}).isVisible());
  assert(await failed.getByText('Alpha retained failure output: expected 200, received 500.',{exact:true}).isVisible());
  assert(await unknown.getByText(/A passed command below does not establish/).isVisible());
  assert.equal(await region().getByRole('button',{name:'Use these results for this build'}).count(),0);
  await unknown.scrollIntoViewIfNeeded();await geometry('retained-output-'+width);
  await failed.scrollIntoViewIfNeeded();await page.screenshot({path:join(out,'retained-failure-output-'+width+'.png')});
  await openRecovery().focus();await page.keyboard.press('Enter');
  await dialog().getByRole('checkbox',{name:/I authorize recovery/}).waitFor();
  assert(await dialog().getByText(/Revision 4/).isVisible());assert(await dialog().locator('code').filter({hasText:'check-alpha'}).isVisible());
  assert(await dialog().getByRole('button',{name:'Recover this exact check',exact:true}).isDisabled());
  await keyboardTrap(dialog());await geometry('exact-recovery-review-'+width);
  await page.keyboard.press('Escape');await dialog().waitFor({state:'detached'});
  await page.waitForFunction(()=>document.activeElement?.textContent?.trim()==='Review check recovery');
  assert(await openRecovery().evaluate(el=>el===document.activeElement));
  const noWrites=await snapshot();assert.equal(noWrites.writes.length,0);assert.equal(noWrites.starts,0);record({label:'review-only-keyboard-'+width,exactOpenerFocusRestored:true,...noWrites});

  await go(base,'recover',width);await openRecovery().click();await consentAndSubmit();
  await dialog().getByText(/Recovery is recorded and this check no longer holds capacity/).waitFor();
  await dialog().getByText(/Recovery is complete/).waitFor();await assertExactWrite('one-explicit-recovery-'+width);await geometry('recovery-recorded-'+width);

  await go(base,'lost-response',width);await openRecovery().click();await consentAndSubmit();
  await dialog().getByText(/No recovery request or command was repeated/).waitFor();await dialog().getByText(/Recovery is complete/).waitFor();
  assert.equal(await dialog().getByRole('checkbox').count(),0);await assertExactWrite('lost-response-no-replay-'+width);await geometry('lost-response-readback-'+width);

  await go(base,'pending-read',width);await openRecovery().focus();await page.keyboard.press('Enter');
  await dialog().getByText(/Reading recovery/).waitFor();await page.keyboard.press('Escape');await dialog().waitFor({state:'detached'});
  await page.waitForFunction(()=>document.activeElement?.textContent?.trim()==='Review check recovery');
  await page.evaluate(()=>window.__checkFixture.releaseRead());
  let calls=await snapshot();assert(calls.reads.some(r=>r.kind==='check'&&r.aborted));assert(!calls.reads.some(r=>r.kind==='recovery'));assert.equal(calls.writes.length,0);
  record({label:'pending-read-close-abort-'+width,exactOpenerFocusRestored:true,...calls});

  await go(base,'pending-write',width);await openRecovery().click();await consentAndSubmit();
  await dialog().getByText(/Recovering this check/).waitFor();assert(await dialog().getByRole('button',{name:'Keep inspecting',exact:true}).isDisabled());
  await page.keyboard.press('Escape');assert(await dialog().isVisible());
  await page.keyboard.press('Tab');
  const details=dialog().getByRole('region',{name:'Check recovery details',exact:true});
  await page.waitForFunction(()=>document.activeElement?.getAttribute('aria-label')==='Check recovery details');
  assert(await details.evaluate(el=>el===document.activeElement));
  const pendingFocus=await details.evaluate(el=>({tabIndex:el.tabIndex,outlineWidth:getComputedStyle(el).outlineWidth,outlineStyle:getComputedStyle(el).outlineStyle}));
  assert.equal(pendingFocus.tabIndex,0);assert.notEqual(pendingFocus.outlineStyle,'none');assert(parseFloat(pendingFocus.outlineWidth)>0);
  let pendingScroll=null;
  if(width===390){
   await page.keyboard.press('Home');await page.waitForTimeout(150);
   const before=await dialog().evaluate(el=>el.scrollTop);
   await page.keyboard.press('ArrowDown');await page.keyboard.press('ArrowDown');
   await page.waitForFunction(value=>document.querySelector('[role=dialog]')?.scrollTop>value,before);
   pendingScroll={before,after:await dialog().evaluate(el=>el.scrollTop)};
   assert(await details.evaluate(el=>el===document.activeElement));
  }
  record({label:'pending-details-keyboard-focus-and-scroll-'+width,pendingFocus,pendingScroll});
  await geometry('pending-recovery-'+width);
  await page.evaluate(()=>window.__checkFixture.releaseWrite());await dialog().getByText(/Recovery is complete/).waitFor();await assertExactWrite('pending-write-no-dismissal-'+width);

  await go(base,'read-denied',width);await region().getByRole('textbox',{name:'Executable',exact:true}).fill('unfinished-command');
  await openRecovery().click();const refresh=region().getByRole('button',{name:'Refresh check access',exact:true});await refresh.waitFor();
  await page.waitForFunction(()=>document.activeElement?.textContent?.trim()==='Refresh check access');
  assert.equal(await dialog().count(),0);assert.equal(await region().locator('.check-runs,.requirements-evidence,input').count(),0);
  assert.equal(await region().getByText(/Alpha retained/).count(),0);await geometry('read-access-denied-'+width);
  const deniedCalls=await snapshot();await page.evaluate(()=>window.__checkFixture.allowReads());await refresh.click();
  await region().getByText('Alpha cleanup is unconfirmed; inspect its original allocation.',{exact:true}).waitFor();
  assert.equal(await region().getByRole('textbox',{name:'Executable',exact:true}).inputValue(),'unfinished-command');
  assert.equal(await region().getByRole('checkbox',{name:/I trust this project/}).isChecked(),false);
  calls=await snapshot();assert.equal(calls.reads.filter(r=>r.kind==='history').length,deniedCalls.reads.filter(r=>r.kind==='history').length+1);assert.equal(calls.writes.length,0);assert.equal(calls.starts,0);
  record({label:'read-denial-clears-private-data-and-explicitly-restores-'+width,draftPreserved:true,exactRefreshFocus:true,...calls});
  // End this isolated fixture case through its real draft control before navigation.
  await region().getByRole('button',{name:'Discard verification setup',exact:true}).click();

  await go(base,'switch-denial',width);await openRecovery().click();await dialog().getByText(/Reading recovery/).waitFor();
  await page.evaluate(()=>window.__checkFixture.switchProject('beta'));
  await region().getByText('Beta cleanup is unconfirmed; inspect its original allocation.',{exact:true}).waitFor();
  await page.evaluate(()=>window.__checkFixture.denyRead());
  await page.waitForFunction(()=>window.__checkFixture.snapshot().pendingReadCount===0);
  assert.equal(await region().getByText(/Alpha retained|Alpha cleanup/).count(),0);assert.equal(await region().getByRole('button',{name:'Refresh check access'}).count(),0);assert.equal(await dialog().count(),0);
  assert.equal(await page.getByRole('treeitem',{name:'Beta',exact:true,includeHidden:true}).getAttribute('aria-selected'),'true');
  await geometry('project-switch-no-private-leak-'+width);record({label:'old-project-denial-fenced-'+width,...await snapshot()});
  if(width===390)await page.getByRole('button',{name:'Projects',exact:true}).click();
  await page.getByRole('treeitem',{name:'Alpha',exact:true}).click();
  await region().getByText('Alpha cleanup is unconfirmed; inspect its original allocation.',{exact:true}).waitFor();
  record({label:'actual-rail-navigation-'+width,selected:(await snapshot()).selected});
 }
 // Actual five-second UI timer; visibility is a controlled DOM event, not an OS claim.
 await go(base,'polling',1440);await page.evaluate(()=>window.__checkFixture.setSlowReads(true));
 await page.waitForFunction(()=>window.__checkFixture.snapshot().reads.filter(r=>r.kind==='check').length===4,{},{timeout:8000});
 await page.waitForTimeout(5500);let polled=await snapshot();assert.equal(polled.reads.filter(r=>r.kind==='check').length,4);assert(polled.maxInflight<=4);
 await page.evaluate(()=>window.__checkFixture.setVisibility('hidden'));
 assert((await snapshot()).reads.filter(r=>r.kind==='check').every(r=>r.aborted));
 await page.evaluate(()=>window.__checkFixture.releaseRead());
 await page.waitForFunction(()=>window.__checkFixture.snapshot().inflight===0);
 const hidden=(await snapshot()).reads.filter(r=>r.kind==='check').length;await page.waitForTimeout(5500);assert.equal((await snapshot()).reads.filter(r=>r.kind==='check').length,hidden);
 await page.evaluate(()=>{window.__checkFixture.setSlowReads(false);window.__checkFixture.setVisibility('visible');});
 await page.waitForFunction(()=>window.__checkFixture.snapshot().reads.filter(r=>r.kind==='check').length>=8,{},{timeout:8000});
 polled=await snapshot();assert.equal(polled.reads.filter(r=>r.kind==='check').length,8);assert.equal(new Set(polled.reads.filter(r=>r.kind==='check').map(r=>r.checkId)).size,6);
 await page.evaluate(()=>window.__checkFixture.settlePolls());
 await page.waitForFunction(()=>document.querySelectorAll('.check-runs [data-check-heading]').length===6&&[...document.querySelectorAll('.check-runs [data-check-heading]')].every(el=>el.textContent==='Commands passed'),{},{timeout:8000});
 const settled=(await snapshot()).reads.filter(r=>r.kind==='check').length;await page.waitForTimeout(5500);assert.equal((await snapshot()).reads.filter(r=>r.kind==='check').length,settled);
 record({label:'bounded-visible-polling-no-overlap-hidden-abort-fairness-and-stop',hiddenReadCount:hidden,settledReadCount:settled,...await snapshot()});
 await geometry('settled-check-history-1440');
 receipt.passed=true;
}catch(error){receipt.error=String(error.stack||error);if(page){receipt.focusAtFailure=await page.evaluate(()=>({tag:document.activeElement?.tagName,text:document.activeElement?.textContent?.trim().slice(0,200)})).catch(()=>null);receipt.fixtureAtFailure=await snapshot().catch(()=>null);await page.screenshot({path:join(out,'failed-stage.png')}).catch(()=>{});}console.error(receipt.error);}
finally{
 const cleanup=[];
 if(browser){try{await browser.close();cleanup.push('owned Chromium closed');}catch(error){receipt.errors.push('Chromium cleanup: '+String(error));}}
 if(server){try{await server.close();cleanup.push('owned Vite server closed');}catch(error){receipt.errors.push('Vite cleanup: '+String(error));}}
 if(profile){rmSync(profile,{recursive:true,force:true});cleanup.push('private temporary profile removed');}
 receipt.sourceAfter=fingerprint();receipt.sourceUnchanged=JSON.stringify(receipt.sourceBefore)===JSON.stringify(receipt.sourceAfter);
 receipt.helperUnchanged=receipt.helperSHA256===sha(readFileSync(fileURLToPath(import.meta.url)))&&receipt.controlledFixtureSHA256===sha(readFileSync(join(here,'fixture.tsx')));
 receipt.passed=receipt.passed&&receipt.sourceUnchanged&&receipt.helperUnchanged&&receipt.errors.length===0;receipt.finishedAt=new Date().toISOString();receipt.cleanup=cleanup;
 writeFileSync(join(out,'result.json'),JSON.stringify(receipt,null,2)+'\n');console.log(JSON.stringify({passed:receipt.passed,error:receipt.error,checks:receipt.checks.length,sourceUnchanged:receipt.sourceUnchanged}));if(!receipt.passed)process.exitCode=1;
}
