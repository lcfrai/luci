import assert from 'node:assert/strict';
import {readFileSync,writeFileSync,mkdirSync,mkdtempSync,rmSync} from 'node:fs';
import {createHash} from 'node:crypto';
import {join,dirname,resolve} from 'node:path';
import {fileURLToPath,pathToFileURL} from 'node:url';
const here=dirname(fileURLToPath(import.meta.url)),repo='/workspace/luci',out=resolve(process.argv[2]);
const {createServer}=await import(pathToFileURL(join(repo,'desktop/node_modules/vite/dist/node/index.js')).href);
const {chromium}=await import(pathToFileURL(join(repo,'desktop/node_modules/playwright/index.mjs')).href);
assert.notEqual(process.getuid(),0);mkdirSync(out,{recursive:true});
const files=['desktop/src/components/projects/ProjectOverviewBoard.tsx','desktop/src/components/projects/HostCapacity.tsx','desktop/src/components/projects/project-overview.css'];
const hash=v=>createHash('sha256').update(v).digest('hex'),snapshot=()=>Object.fromEntries(files.map(p=>[p,hash(readFileSync(join(repo,p)))]));
const receipt={passed:false,scope:'Actual AppShell/ProjectsHome/ProjectOverviewBoard/HostCapacity, controlled read-only callbacks, Chromium Linux; no backend, agents, models, resource/performance acceptance.',sourceBefore:snapshot(),helperSHA256:hash(readFileSync(fileURLToPath(import.meta.url))),fixtureSHA256:hash(readFileSync(join(here,'fixture.tsx'))),checks:[],errors:[]};
let server,browser;const profile=mkdtempSync(join(here,'profile-'));
try{
 server=await createServer({configFile:join(repo,'desktop/vite.config.ts'),root:join(repo,'desktop'),server:{host:'127.0.0.1',port:0,watch:null,fs:{allow:[repo,here]}},plugins:[{name:'controlled-home',enforce:'pre',resolveId(id){if(id.endsWith('/AuthProvider'))return '\0auth';if(id.endsWith('/TeamWorkspaceProvider'))return '\0team';if(id.endsWith('/SetupProvider'))return '\0setup';if(id.endsWith('/useProjectAccess'))return '\0access';},load(id){if(id==='\0auth')return 'export const useAuth=()=>({profile:{user:{id:"fixture",username:"fixture",displayName:"Fixture owner",role:"super-admin"},permissions:{}}});';if(id==='\0team')return 'export const useTeamWorkspace=()=>null;export const useTeamProjectPermissions=()=>({loading:false,error:"",permissions:{read:true,edit:true,execute:true}});';if(id==='\0setup')return 'export const useSetup=()=>({workspace:{mode:"local"},status:{configured:true,access:{user:{username:"fixture"}},config:{projectsDir:"/fixture"}}});';if(id==='\0access')return 'export const useProjectAccess=()=>({canRead:true,canEdit:true,canExecute:true,loading:false,error:"",message:""});';},configureServer(vite){vite.middlewares.use('/home-fixture',async(req,res)=>{res.setHeader('Content-Type','text/html');res.end(await vite.transformIndexHtml('/home-fixture','<!doctype html><html lang="en"><head><meta name="viewport" content="width=device-width,initial-scale=1"><title>Controlled Home stability</title></head><body><div id="root"></div><script type="module" src="/@fs'+join(here,'fixture.tsx')+'"></script></body></html>'));});}}]});await server.listen();const base='http://127.0.0.1:'+server.httpServer.address().port;
 browser=await chromium.launch({headless:true,executablePath:'/usr/bin/chromium',env:{PATH:process.env.PATH,TMPDIR:profile,XDG_CONFIG_HOME:join(profile,'config'),XDG_CACHE_HOME:join(profile,'cache')},args:['--no-proxy-server']});
 const context=await browser.newContext({serviceWorkers:'block'});await context.route('**/*',route=>new URL(route.request().url()).origin===base?route.continue():route.abort());const page=await context.newPage();page.setDefaultTimeout(12000);page.on('pageerror',e=>receipt.errors.push(e.message));
 for(const width of [1440,390]){
  await page.setViewportSize({width,height:width===390?844:1080});await page.goto(base+'/home-fixture');await page.getByRole('heading',{name:'Across your projects',exact:true}).waitFor();
  const geometry=async label=>{await page.evaluate(()=>new Promise(done=>requestAnimationFrame(()=>requestAnimationFrame(done))));const rows=await page.evaluate(()=>['alpha','beta','gamma','delta'].map(id=>{const el=document.querySelector('[aria-label="Open project '+id+'"]'),r=el.getBoundingClientRect();return{id,x:r.x,y:r.y,width:r.width,height:r.height};}));receipt.checks.push({label:label+'-'+width,rows});await page.screenshot({path:join(out,label+'-'+width+'.png')});return rows;};
  const initial=await geometry('initial');
  const projectButtons=page.getByRole('navigation',{name:'Open a project',exact:true}).getByRole('button');
  assert.equal(await projectButtons.count(),4);
  assert.deepEqual(await projectButtons.evaluateAll(nodes=>nodes.map(node=>node.dataset.slot)),['button','button','button','button']);
  await projectButtons.nth(0).focus();
  for(let i=0;i<4;i++){assert.equal(await projectButtons.nth(i).evaluate(node=>node===document.activeElement),true);if(i<3)await page.keyboard.press('Tab');}
  receipt.checks.push({label:'keyboard-order-'+width,passed:true});
  await page.screenshot({path:join(out,'keyboard-'+width+'.png')});
  await page.evaluate(()=>window.__homeFixture.release('capacity','denied'));await page.getByRole('button',{name:'Retry capacity',exact:true}).waitFor();await geometry('capacity-error');
  await page.getByRole('button',{name:'Retry capacity',exact:true}).click();await geometry('capacity-retry-loading');
  await page.evaluate(()=>window.__homeFixture.release('capacity'));const capacity=await geometry('capacity');
  for(const id of ['alpha','beta','gamma','delta']){await page.evaluate(id=>window.__homeFixture.release(id),id);await geometry('resolved-'+id);}
  await page.getByRole('button',{name:'Refresh overview',exact:true}).click();await page.evaluate(()=>['alpha','beta','gamma','delta'].forEach(id=>window.__homeFixture.release(id,'warning')));const warning=await geometry('warnings');assert.equal(await page.getByText('Queue activity unavailable; no zero counts inferred.',{exact:true}).count(),4);assert.equal(await page.getByText('Workspace dispatch is paused.',{exact:true}).count(),4);receipt.checks.push({label:'warnings-visible-'+width,passed:true});
  await page.getByRole('button',{name:'Refresh overview',exact:true}).click();await page.evaluate(()=>['alpha','beta','gamma','delta'].forEach(id=>window.__homeFixture.release(id,'busy')));await geometry('source-busy');assert.equal(await page.getByText(/Source is being prepared; showing last recorded context/).count(),4);assert.equal(await page.getByRole('button',{name:/^Inspect run-/}).count(),0);assert.equal(await page.getByText('Refresh project context before inspecting this recorded work.',{exact:true}).count(),4);receipt.checks.push({label:'stale-review-unavailable-'+width,passed:true});
  await page.getByRole('button',{name:'Refresh overview',exact:true}).click();await page.evaluate(()=>['alpha','beta','gamma','delta'].forEach(id=>window.__homeFixture.release(id,'denied')));const denied=await geometry('denied');
  const points=receipt.checks.filter(c=>c.rows&&c.label.endsWith('-'+width));const maxShift=Math.max(...points.flatMap(c=>c.rows.map((v,i)=>Math.abs(v.y-initial[i].y))));receipt.checks.push({label:'target-stability-'+width,maxShiftPx:maxShift,passed:maxShift<=1});
  assert.equal(await page.getByText(/Access revoked/).count(),4);assert.equal(await page.getByRole('button',{name:'Review retained task changes',exact:true}).count(),0);
  // Opening is available even when fresh summary reads have failed. AppShell's
  // actual selection/draft scopes choose the destination, never the fixture.
  const privateValues=await page.locator('.overview-goal').count();assert.equal(privateValues,0);receipt.checks.push({label:'denied-private-summary-removed-'+width,passed:true});
  assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true);
  await page.addScriptTag({content:readFileSync(join(repo,'desktop/node_modules/axe-core/axe.min.js'),'utf8')});
  const accessibility=await page.evaluate(()=>axe.run(document.querySelector('.project-overview-board'),{runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21aa']}}));
  writeFileSync(join(out,'accessibility-'+width+'.json'),JSON.stringify(accessibility,null,2)+'\n');assert.deepEqual(accessibility.violations,[]);receipt.checks.push({label:'accessibility-'+width,passed:true,passes:accessibility.passes.length,incomplete:accessibility.incomplete.length});
  await page.getByRole('button',{name:'Open project gamma',exact:true}).focus();await page.keyboard.press('Enter');await page.getByRole('textbox',{name:'The idea',exact:true}).waitFor();assert.equal(await page.evaluate(()=>window.__homeFixture.selected()),'gamma');assert.equal(await page.getByRole('textbox',{name:'The idea',exact:true}).inputValue(),'Build gamma with durable project context and reviewed task results.');receipt.checks.push({label:'keyboard-current-context-'+width,passed:true});
  if(width===390){
    await page.goto(base+'/home-fixture');await page.getByRole('heading',{name:'Across your projects',exact:true}).waitFor();
    const longName='longprojectname'.repeat(9);await page.evaluate(name=>window.__homeFixture.rename('gamma',name),longName);
    const target=page.getByRole('button',{name:'Open project '+longName,exact:true});await target.waitFor();const beforeLong=await target.boundingBox();
    await page.evaluate(()=>{window.__homeFixture.release('capacity');['alpha','beta','gamma','delta'].forEach(id=>window.__homeFixture.release(id));});
    await page.getByText('Build gamma with durable project context and reviewed task results.',{exact:true}).waitFor();
    const afterLong=await target.boundingBox();assert.equal(afterLong.y,beforeLong.y);assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true);
    const heading=page.getByRole('heading',{name:longName,exact:true});assert.equal(await heading.evaluate(node=>node.scrollWidth<=node.clientWidth),true);
    await page.screenshot({path:join(out,'long-name-390.png')});receipt.checks.push({label:'long-name-390',passed:true,beforeLong,afterLong});
  }

 }
 assert.deepEqual(receipt.errors,[]);receipt.passed=receipt.checks.filter(c=>c.label.startsWith('target-stability')).every(c=>c.passed);
}catch(error){receipt.failure=String(error.stack||error);}finally{if(browser)await browser.close();if(server)await server.close();rmSync(profile,{recursive:true,force:true});receipt.sourceAfter=snapshot();receipt.sourceUnchanged=JSON.stringify(receipt.sourceBefore)===JSON.stringify(receipt.sourceAfter);writeFileSync(join(out,'report.json'),JSON.stringify(receipt,null,2)+'\n');console.log(JSON.stringify({passed:receipt.passed,error:receipt.failure,stability:receipt.checks.filter(c=>c.label.startsWith('target-stability')),errors:receipt.errors,sourceUnchanged:receipt.sourceUnchanged}));process.exitCode=receipt.passed?0:1;}
