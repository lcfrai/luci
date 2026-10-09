import assert from 'node:assert/strict';
import {readFileSync,writeFileSync,mkdtempSync,mkdirSync,rmSync,existsSync} from 'node:fs';
import {createHash} from 'node:crypto';
import {join,dirname,resolve} from 'node:path';
import {fileURLToPath,pathToFileURL} from 'node:url';
import {setTimeout as delay} from 'node:timers/promises';
import {browserReadinessInit} from '../observer/autonomous-readiness.mjs';
import {installBrowserTiming,interactionRunner} from '/workspace/.luci-tools/cloud032-idle-67a5b7c7/actual/interaction-metrics.mjs';
const here=dirname(fileURLToPath(import.meta.url)),repo='/workspace/luci',out=resolve(process.argv[2]);
assert(!existsSync(out),'Retain every prior result');mkdirSync(out,{recursive:true});
const {createServer}=await import(pathToFileURL(join(repo,'desktop/node_modules/vite/dist/node/index.js')).href);
const {chromium}=await import(pathToFileURL(join(repo,'desktop/node_modules/playwright/index.mjs')).href);
const sources=['desktop/src/components/AppShell.tsx','desktop/src/components/rail/Rail.tsx','desktop/src/components/rail/ProjectRow.tsx','desktop/src/components/projects/ProjectOverviewBoard.tsx','desktop/src/components/definition/DefinitionWorkspace.tsx','desktop/src/store/slices/uiSlice.ts'];
const hash=v=>createHash('sha256').update(v).digest('hex'),snapshot=()=>Object.fromEntries(sources.map(p=>[p,hash(readFileSync(join(repo,p)))]));
const report={passed:false,scope:'Controlled styled actual AppShell/Home/Rail/Definition source fixture with synthetic API values. No installed app, backend, worker, provider, acceptance performance or paint claim.',sourceBefore:snapshot(),checks:[],errors:[],cases:[]};
let server,browser;const profile=mkdtempSync('/tmp/c32-ready-');
let page,context,requests=[];
const record=(label,data={})=>report.checks.push({label,passed:true,...data});
async function fresh(){if(context)await context.close();context=await browser.newContext({serviceWorkers:'block',viewport:{width:1440,height:1080}});await installBrowserTiming(context);await context.route('**/*',route=>new URL(route.request().url()).origin===base?route.continue():route.abort());page=await context.newPage();page.setDefaultTimeout(10000);page.on('pageerror',e=>report.errors.push(e.message));page.on('request',r=>requests.push({method:r.method(),origin:new URL(r.url()).origin}));await page.goto(base+'/readiness-fixture');await page.getByRole('heading',{name:'Across your projects',exact:true}).waitFor();await page.getByRole('button',{name:'Open project delta',exact:true}).click();await page.getByRole('textbox',{name:'The idea',exact:true}).waitFor();}
const config=()=>page.evaluate(()=>window.__homeFixture.config());
const start=async()=>page.evaluate(browserReadinessInit,await config());
const snap=()=>page.evaluate(()=>window.__luciAutonomousReadiness.snapshot());
const close=()=>page.evaluate(()=>window.__luciAutonomousReadiness.close());
async function home(){await page.getByRole('button',{name:'All projects',exact:true}).click();await page.getByRole('heading',{name:'Across your projects',exact:true}).waitFor();}
async function open(name='alpha'){await page.getByRole('button',{name:'Open project '+name,exact:true}).click();}
async function waitSettled(){await page.waitForFunction(()=>window.__luciAutonomousReadiness.snapshot);await delay(100);}
let base;
try{
 assert.notEqual(process.getuid(),0);
 server=await createServer({configFile:join(repo,'desktop/vite.config.ts'),root:join(repo,'desktop'),cacheDir:join(out,'vite-cache'),server:{host:'127.0.0.1',port:0,watch:null,fs:{allow:[repo,here]}},plugins:[{name:'controlled-readiness',enforce:'pre',resolveId(id){if(id.endsWith('/AuthProvider'))return '\0auth';if(id.endsWith('/TeamWorkspaceProvider'))return '\0team';if(id.endsWith('/SetupProvider'))return '\0setup';if(id.endsWith('/useProjectAccess'))return '\0access';},load(id){if(id==='\0auth')return 'export const useAuth=()=>({profile:{user:{id:"fixture",username:"fixture",displayName:"Fixture owner",role:"super-admin"},permissions:{}}});';if(id==='\0team')return 'export const useTeamWorkspace=()=>null;export const useTeamProjectPermissions=()=>({loading:false,error:"",permissions:{read:true,edit:true,execute:true}});';if(id==='\0setup')return 'export const useSetup=()=>({workspace:{mode:"local"},status:{configured:true,access:{user:{username:"fixture"}},config:{projectsDir:"/fixture"}}});';if(id==='\0access')return 'import {useSyncExternalStore} from "react";export const useProjectAccess=()=>useSyncExternalStore(window.__readinessAccess.subscribe,window.__readinessAccess.get);';},configureServer(vite){vite.middlewares.use('/readiness-fixture',async(req,res)=>{res.setHeader('Content-Type','text/html');res.end(await vite.transformIndexHtml('/readiness-fixture','<!doctype html><html lang="en"><head><meta name="viewport" content="width=device-width,initial-scale=1"><title>Controlled autonomous readiness</title></head><body><div id="root"></div><script type="module" src="/@fs'+join(here,'fixture.tsx')+'"></script></body></html>'));});}}]});await server.listen();base='http://127.0.0.1:'+server.httpServer.address().port;
 browser=await chromium.launch({headless:true,executablePath:'/usr/bin/chromium',env:{PATH:process.env.PATH,TMPDIR:profile,XDG_CONFIG_HOME:join(profile,'config'),XDG_CACHE_HOME:join(profile,'cache')},args:['--no-proxy-server']});
 // Same unmodified primary action module in both arms. Controlled adapter adds
 // an explicit 360ms post-wait hold solely to expose the old observation gap.
 for(const enabled of [false,true]){
  await fresh();if(enabled)await start();const receipt={interactions:[]};const owners=await page.evaluate(()=>window.__homeFixture.owners());
  const heldPage=new Proxy(page,{get(target,key){if(key==='getByRole')return(role,options)=>{const locator=target.getByRole(role,options);if(role==='textbox'&&options?.name==='The idea')return new Proxy(locator,{get(l,k){if(k==='waitFor')return async(...args)=>{await l.waitFor(...args);await delay(360);};const v=l[k];return typeof v==='function'?v.bind(l):v;}});return locator;};const v=target[key];return typeof v==='function'?v.bind(target):v;}});
  const runner=interactionRunner({page:heldPage,owners,receipt,assertHealthy:()=>{},phase:()=>enabled?'controlled-with-witness':'controlled-late-only'});await runner.round(0);
  const latency=await page.evaluate(()=>window.__luciAcceptanceLatency.snapshot());const observation=enabled?await close():null;
  const primary=receipt.interactions.find(r=>r.kind==='project-switch');assert(primary.success&&primary.elapsedMs>300);assert.equal(receipt.interactions.length,4);
  const expected=latency.rows?.find?.(r=>r.type==='expected-dom');
  const row={enabled,interactions:receipt.interactions,latency,observation};report.cases.push(row);
  if(enabled){const witness=observation.rows.find(r=>r.stage==='define'&&r.status==='ready');assert(witness,'Correct saved editor must be autonomously observed');const mark=latency.rows.find(r=>r.kind==='expected-dom'&&r.actionKind==='project-switch');assert(mark.at-witness.observedAt>=300);record('autonomous-witness-precedes-controlled-late-driver',{gapMs:mark.at-witness.observedAt,unchangedFullSpanMs:primary.elapsedMs});}
  else record('original-late-only-limitation-retained',{autonomousWitnessAvailable:false,notAProductFailure:true,unchangedFullSpanMs:primary.elapsedMs});
 }
 // Disabled fieldset and delayed fresh data cannot become a positive witness.
 await fresh();await start();await home();await page.evaluate(()=>{window.__homeFixture.delay(250);window.__homeFixture.access(false);});await open();await delay(350);let value=await snap();assert(!value.rows.some(r=>r.stage==='define'&&r.status==='ready'));await page.evaluate(()=>window.__homeFixture.access(true));await delay(80);value=await close();assert(value.rows.some(r=>r.stage==='define'&&r.status==='ready'));record('delayed-read-and-native-disabled-fieldset',{rows:value.rows});
 // Equal values in alpha/beta cannot bypass selected project identity.
 await fresh();await start();await home();await page.evaluate(()=>window.__homeFixture.delay(200));await open();await page.evaluate(()=>window.__homeFixture.select('beta'));await delay(300);value=await snap();assert(!value.rows.some(r=>r.stage==='define'&&r.status==='ready'));await page.evaluate(()=>window.__luciAutonomousReadiness.invalidate('authority-changed'));value=await snap();assert(value.closed&&value.activeGeneration===null);record('same-value-wrong-project-and-authority-invalidation',{rows:value.rows});
 // Wrong expected original destination cancels, even if native and trusted.
 await fresh();await start();await home();await open('beta');value=await snap();assert(value.closed&&!value.rows.some(r=>r.stage==='define'&&r.status==='ready'));record('wrong-trusted-project-refused',{rows:value.rows});
 // Programmatic clicks never arm, even though the real app receives them.
 await fresh();await start();await page.evaluate(()=>document.querySelector('.rail-home').click());await delay(80);value=await close();assert.equal(value.stages,0);record('untrusted-click-cannot-arm');
 // Read loss + explicit authoritative lifecycle invalidation, not inferred grant.
 await fresh();await start();await home();await page.evaluate(()=>window.__homeFixture.delay(200));await open();await page.evaluate(()=>{window.__homeFixture.access(false,false);window.__luciAutonomousReadiness.invalidate('authority-changed');});await delay(280);value=await snap();assert(value.closed&&!value.rows.some(r=>r.stage==='define'&&r.status==='ready'));record('read-revoked-with-authoritative-invalidation',{rows:value.rows});
 // Observer itself can be delayed: report the sampling blind interval.
 await fresh();await start();await home();await page.evaluate(()=>{document.querySelector('[aria-label="Open project alpha"]').addEventListener('click',()=>{const end=performance.now()+180;while(performance.now()<end){};},{once:true});});await open();await delay(100);value=await close();assert(value.rows.some(r=>r.stage==='define'&&r.maxSampleGapMs>=150));record('observer-delay-exposes-gap',{rows:value.rows});
 // Unmount cannot reuse the previous node. Censor boundedly, never positive.
 await fresh();await start();await home();await page.evaluate(()=>window.__homeFixture.delay(250));await open();await page.evaluate(()=>window.__homeFixture.unmount());await delay(80);value=await close();assert(!value.rows.some(r=>r.stage==='define'&&r.status==='ready'));assert.equal(value.activeGeneration,null);record('unmount-settles-without-stale-witness',{rows:value.rows});
 // A permanently wrong value produces an actual declared wall/sample cap.
 await fresh();const wrong=await config();wrong.projects[0].value='A different synthetic expected value.';await page.evaluate(browserReadinessInit,wrong);await home();await open();await delay(3100);value=await close();assert(value.rows.some(r=>r.stage==='define'&&r.status==='censored'&&['wall-time-cap','sample-or-time-cap'].includes(r.reason)));record('wrong-value-bounded-censor',{rows:value.rows});
 assert.deepEqual(report.errors,[]);assert(requests.every(r=>r.method==='GET'&&r.origin===base));record('controlled-fixture-only-same-origin-GET',{requests:requests.length});
 const source=readFileSync(join(here,'../observer/autonomous-readiness.mjs'),'utf8');assert(!/getBoundingClientRect|getClientRects|getComputedStyle|offsetWidth|offsetHeight|scrollIntoView|MutationObserver|\.focus\(|\.click\(|fetch\(|XMLHttpRequest|\.querySelector/.test(source));record('observer-source-forbidden-layout-write-network-api-scan');
 report.passed=true;
}catch(error){report.failure=String(error.stack||error);}finally{if(context)await context.close();if(browser)await browser.close();if(server)await server.close();rmSync(profile,{recursive:true,force:true});report.sourceAfter=snapshot();report.sourceUnchanged=JSON.stringify(report.sourceBefore)===JSON.stringify(report.sourceAfter);report.passed&&=report.sourceUnchanged;writeFileSync(join(out,'report.json'),JSON.stringify(report,null,2)+'\n');console.log(JSON.stringify({passed:report.passed,checks:report.checks.length,failure:report.failure,sourceUnchanged:report.sourceUnchanged}));process.exitCode=report.passed?0:1;}
