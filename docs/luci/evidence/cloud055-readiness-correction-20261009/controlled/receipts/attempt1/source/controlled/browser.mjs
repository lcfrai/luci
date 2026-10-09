import assert from 'node:assert/strict';
import {readFileSync,writeFileSync,mkdirSync,mkdtempSync,rmSync,existsSync} from 'node:fs';
import {createHash} from 'node:crypto';
import {dirname,join,resolve} from 'node:path';
import {fileURLToPath,pathToFileURL} from 'node:url';
import {setTimeout as delay} from 'node:timers/promises';
import {browserReadinessInit as originalInit} from './original/autonomous-readiness.mjs';
import {browserReadinessInit as candidateInit} from '../observer/autonomous-readiness.mjs';
import {fixturePlugins} from './vite-fixture-plugin.mjs';
const here=dirname(fileURLToPath(import.meta.url)),root=dirname(here),desktop=join(here,'source/desktop'),deps=join(desktop,'node_modules'),out=resolve(process.argv[2]);
assert(!existsSync(out),'All attempts must remain immutable');mkdirSync(out,{recursive:true});
const load=p=>import(pathToFileURL(join(deps,p)).href);
const {createServer}=await load('vite/dist/node/index.js'),{chromium}=await load('playwright/index.mjs');
const {default:react}=await load('@vitejs/plugin-react/dist/index.js'),{default:tailwind}=await load('@tailwindcss/vite/dist/index.mjs');
const hash=value=>createHash('sha256').update(value).digest('hex');
const binding=JSON.parse(readFileSync(join(here,'source-binding.json')));
const sourceSnapshot=()=>Object.fromEntries(Object.keys(binding.files).map(p=>[p,hash(readFileSync(join(here,'source',p)))]));
const report={passed:false,scope:'Controlled actual d20 AppShell, real styled Home/Rail/Definition, synthetic deferred API responses. No installed app, performance, backend, authorization or paint claim.',sourceCommit:binding.commit,sourceBefore:sourceSnapshot(),checks:[],cases:[],errors:[],requests:[]};
let server,browser,context,page,base,deadline,runPromise;const profile=mkdtempSync('/tmp/c55-ready-');
writeFileSync(join(out,'owned-resources.json'),JSON.stringify({nodePid:process.pid,profile},null,2)+'\n');
const record=(label,data={})=>report.checks.push({label,passed:true,...data});
async function fresh(){
 if(context)await context.close();context=await browser.newContext({serviceWorkers:'block',viewport:{width:1440,height:1080}});
 await context.route('**/*',route=>new URL(route.request().url()).origin===base?route.continue():route.abort());
 page=await context.newPage();page.setDefaultTimeout(12000);page.on('pageerror',e=>report.errors.push(e.message));page.on('request',r=>report.requests.push({method:r.method(),origin:new URL(r.url()).origin}));
 await page.goto(base+'/readiness-fixture');await page.getByRole('heading',{name:'Across your projects',exact:true}).waitFor();
 await page.getByRole('button',{name:'Open project delta',exact:true}).click();await page.getByRole('textbox',{name:'The idea',exact:true}).waitFor();
 await page.waitForFunction(()=>document.querySelector('.definition-specification textarea')?.value==='Synthetic delta goal.');
}
async function drain(c){const s=await page.evaluate(()=>window.__luciAutonomousReadiness.snapshot());c.snapshots.push(s);c.rows.push(...s.rows);return s;}
async function stage(c,name){for(let i=0;i<180;i++){await drain(c);const row=c.rows.find(r=>r.stage===name);if(row)return row;await delay(25);}throw Error('Bounded stage did not settle: '+name);}
async function settleHome(c,init){
 await fresh();const config=await page.evaluate(()=>window.__homeFixture.config());if(c.kind==='wrong-value')config.projects[0].value='Different synthetic expected goal.';
 c.installation=await page.evaluate(init,config);assert.equal(c.installation.limits.controlsPerSample,32);
 await page.getByRole('button',{name:'All projects',exact:true}).click();const early=await stage(c,'home');assert.equal(early.status,'ready');
 c.earlyHome=await page.evaluate(()=>({selected:document.querySelector('[role="treeitem"][data-kind="project"][aria-selected="true"]')?.getAttribute('data-id')??null,controls:document.querySelectorAll('#project-navigation button,#project-navigation textarea,.shell-lens button,.shell-lens textarea').length,summaryGoals:document.querySelectorAll('.overview-goal').length}));
 assert.equal(c.earlyHome.summaryGoals,0);
 c.released=await page.evaluate(()=>Object.fromEntries([...window.__homeFixture.ids,'capacity'].map(key=>[key,window.__homeFixture.release(key)])));
 await page.waitForFunction(()=>document.querySelectorAll('.overview-goal').length===4&&document.querySelector('.overview-capacity-brief'));
 c.settledHome=await page.evaluate(()=>({selected:document.querySelector('[role="treeitem"][data-kind="project"][aria-selected="true"]')?.getAttribute('data-id')??null,controls:Array.from(document.querySelectorAll('#project-navigation button,#project-navigation textarea,.shell-lens button,.shell-lens textarea'),n=>({tag:n.tagName,component:n.closest('.overview-card')?'actual-project-card':n.closest('#project-navigation')?'actual-rail':n.closest('.overview-capacity')?'actual-capacity':'actual-home',label:n.getAttribute('aria-label')??n.textContent.trim()})),summaryGoals:document.querySelectorAll('.overview-goal').length}));
 assert(c.settledHome.controls.length>32,'Real settled Home must exceed the unchanged scan cap');assert.equal(c.settledHome.selected,null);
 await page.evaluate(()=>{document.addEventListener('click',event=>{if(event.isTrusted&&event.target.closest?.('button.overview-project-destination'))window.__trustedOpenCheckpoint={trusted:event.isTrusted,at:performance.now(),timeOrigin:performance.timeOrigin,selected:document.querySelector('[role="treeitem"][data-kind="project"][aria-selected="true"]')?.getAttribute('data-id')??null,homeHeading:document.querySelector('.overview-heading h2')?.textContent,controls:document.querySelectorAll('#project-navigation button,#project-navigation textarea,.shell-lens button,.shell-lens textarea').length};},{capture:true});});
}
async function one(kind,init){
 const c={kind,rows:[],snapshots:[]};report.cases.push(c);await settleHome(c,init);
 if(kind==='missing')await page.evaluate(()=>window.__homeFixture.deny(true));
 if(kind==='disabled')await page.evaluate(()=>window.__homeFixture.access(false));
 if(kind==='wrong-selection')await page.evaluate(()=>{window.__homeFixture.delay(150);document.addEventListener('click',event=>{if(event.isTrusted&&event.target.closest?.('button.overview-project-destination'))queueMicrotask(()=>window.__homeFixture.select('beta'));},{once:true});});
 if(kind==='fixed')await page.screenshot({path:join(out,'fixed-settled-home.png'),fullPage:true});
 await page.getByRole('button',{name:'Open project alpha',exact:true}).click();c.trustedOpen=await page.evaluate(()=>window.__trustedOpenCheckpoint);
 assert(c.trustedOpen.trusted&&c.trustedOpen.selected===null&&c.trustedOpen.controls>32&&c.trustedOpen.homeHeading==='Across your projects');
 const row=await stage(c,'define');c.terminal=row;
 if(kind==='original')assert(row.status==='censored'&&row.reason==='control-cap'&&row.samples===1&&row.firstSampleAt>=row.clickAt);
 else if(kind==='fixed'){
  assert.equal(row.status,'ready');assert(row.samples>=2);assert(row.maxControlsVisited<=32);
  c.rendered=await page.evaluate(()=>({selected:document.querySelector('[role="treeitem"][data-kind="project"][aria-selected="true"]')?.getAttribute('data-id'),value:document.querySelector('.definition-specification textarea')?.value,disabled:document.querySelector('.definition-specification textarea')?.matches(':disabled')}));
  assert.deepEqual(c.rendered,{selected:'alpha',value:'Identical synthetic saved goal.',disabled:false});await page.screenshot({path:join(out,'fixed-ready-define.png'),fullPage:true});
 }else{assert.equal(row.status,'censored');assert(['sample-or-time-cap','wall-time-cap','missing-editor'].includes(row.reason));assert(!c.rows.some(r=>r.stage==='define'&&r.status==='ready'));}
 c.closed=await page.evaluate(()=>window.__luciAutonomousReadiness.close());assert(c.closed.closed&&c.closed.activeGeneration===null);c.rows.push(...c.closed.rows);
 record(kind,{terminal:row.status,reason:row.reason,earlyHomeControls:c.earlyHome.controls,settledHomeControls:c.settledHome.controls.length,defineSamples:row.samples});
}
async function main(){
 assert.notEqual(process.getuid(),0);
 server=await createServer({configFile:false,root:desktop,cacheDir:join(out,'vite-cache'),clearScreen:false,optimizeDeps:{exclude:['@excalidraw/excalidraw']},plugins:[react(),tailwind(),...fixturePlugins(here)],server:{host:'127.0.0.1',port:0,watch:null,fs:{allow:[root,'/workspace/luci/desktop/node_modules']}}});await server.listen();base='http://127.0.0.1:'+server.httpServer.address().port;
 browser=await chromium.launch({headless:true,executablePath:'/usr/bin/chromium',env:{PATH:process.env.PATH,GOMAXPROCS:'2',TMPDIR:profile,XDG_CONFIG_HOME:join(profile,'config'),XDG_CACHE_HOME:join(profile,'cache')},args:['--no-proxy-server']});
 for(const kind of ['original','fixed','wrong-selection','missing','wrong-value','disabled'])await one(kind,kind==='original'?originalInit:candidateInit);
 assert.deepEqual(report.errors,[]);assert(report.requests.every(r=>r.method==='GET'&&r.origin===base));record('same-origin-read-only-controlled-transport',{requests:report.requests.length});
 const source=readFileSync(join(root,'observer/autonomous-readiness.mjs'),'utf8');assert(!/getBoundingClientRect|getClientRects|getComputedStyle|offsetWidth|offsetHeight|scrollIntoView|MutationObserver|\.focus\(|\.click\(|fetch\(|XMLHttpRequest|\.querySelector/.test(source));record('observer-retains-no-layout-write-network-APIs');
 report.passed=true;
}
try{
 runPromise=main();await Promise.race([runPromise,new Promise((_,reject)=>{deadline=setTimeout(()=>reject(Error('90-second controlled batch deadline')),90000);process.once('SIGTERM',()=>reject(Error('Owned runner terminated')));})]);
}catch(error){report.failure=String(error.stack||error);}finally{
 clearTimeout(deadline);report.cleanup={};
 for(const [name,resource] of [['context',context],['browser',browser],['vite',server]]){try{if(resource)await resource.close();report.cleanup[name+'Closed']=true;}catch(error){report.cleanup[name+'Error']=String(error);report.passed=false;}}
 if(runPromise)await Promise.race([runPromise.catch(()=>{}),delay(1000)]);
 rmSync(profile,{recursive:true,force:true});report.cleanup.profileRemoved=!existsSync(profile);
 report.sourceAfter=sourceSnapshot();report.sourceUnchanged=JSON.stringify(report.sourceBefore)===JSON.stringify(report.sourceAfter)&&JSON.stringify(report.sourceBefore)===JSON.stringify(binding.files);report.passed&&=report.sourceUnchanged;
 writeFileSync(join(out,'report.json'),JSON.stringify(report,null,2)+'\n');console.log(JSON.stringify({passed:report.passed,checks:report.checks,failure:report.failure,cleanup:report.cleanup,sourceUnchanged:report.sourceUnchanged}));process.exitCode=report.passed?0:1;
}
