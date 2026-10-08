#!/usr/bin/env node
// Controlled actual-component regression only. Run after the root releases its exclusive witness.
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {execFileSync} from 'node:child_process';
import {existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync} from 'node:fs';
import {dirname, join, resolve} from 'node:path';
import {fileURLToPath, pathToFileURL} from 'node:url';

assert.equal(process.env.LUCI_CLOUD028_RELEASE, 'controlled-browser', 'Root must release the installed exclusive run before execution');
const here=dirname(fileURLToPath(import.meta.url)), prepared=resolve(here,'..');
const repo=resolve(process.env.LUCI_ACCEPTANCE_REPO||'/workspace/luci');
const out=resolve(process.env.LUCI_CLOUD028_OUT||join(prepared,'attempt-'+Date.now()));
assert(!existsSync(out), 'Use fresh evidence; never overwrite the original failure');
mkdirSync(out,{recursive:true,mode:0o700});
const sha=value=>createHash('sha256').update(value).digest('hex');
const manifest=JSON.parse(readFileSync(join(prepared,'preparation.json'),'utf8'));
const oldSource=readFileSync(join(prepared,'original/sustained.mjs'),'utf8');
const newSource=readFileSync(join(prepared,'staged/scripts/acceptance/sustained-four-project/sustained.mjs'),'utf8');
const helperPath=join(prepared,'staged/scripts/acceptance/sustained-four-project/queue-controls.mjs');
const fixturePath=join(here,'fixture.tsx');
assert.equal(sha(oldSource),manifest.originalDriverSHA256);
assert.equal(sha(newSource),manifest.stagedDriverSHA256);
assert.equal(sha(readFileSync(helperPath)),manifest.stagedHelperSHA256);
for(const [path,row] of Object.entries(manifest.productSources)) assert.equal(sha(readFileSync(join(repo,path))),row.sha256,path+' source changed');
const {openQueueControls}=await import(pathToFileURL(helperPath).href);
const {createServer}=await import(pathToFileURL(join(repo,'desktop/node_modules/vite/dist/node/index.js')).href);
const {default:react}=await import(pathToFileURL(join(repo,'desktop/node_modules/@vitejs/plugin-react/dist/index.js')).href);
const {default:tailwind}=await import(pathToFileURL(join(repo,'desktop/node_modules/@tailwindcss/vite/dist/index.mjs')).href);
const {chromium}=await import(pathToFileURL(join(repo,'desktop/node_modules/playwright/index.mjs')).href);
const receipt={passed:false,scope:'CLOUD028 actual complete SwarmRunQueue component and actual swarmQueueApi validation with controlled HTTP transport; exact original/corrected queueControls source extraction',limitations:['No installed app, backend, Docker, provider, model or sustained performance execution.','Only the actual queue component is mounted; project navigation remounts this component fixture, not AppShell.','Source-driver private run/effect reads are controlled fixtures; zero Docker effects are not an installed allocation witness.'],startedAt:new Date().toISOString(),productCommit:manifest.productCommit,helperCheckout:execFileSync('git',['rev-parse','HEAD'],{cwd:repo,encoding:'utf8'}).trim(),originalDriverSHA256:sha(oldSource),correctedDriverSHA256:sha(newSource),helperSHA256:sha(readFileSync(helperPath)),fixtureSHA256:sha(readFileSync(fixturePath)),runnerSHA256:sha(readFileSync(fileURLToPath(import.meta.url))),checks:[],errors:[],cleanup:[]};
writeFileSync(join(out,'preparation.json'),JSON.stringify(manifest,null,2)+'\n',{mode:0o600,flag:'wx'});
for(const [name,value] of [['original-sustained.mjs',oldSource],['corrected-sustained.mjs',newSource],['queue-controls.mjs',readFileSync(helperPath)],['fixture.tsx',readFileSync(fixturePath)],['source-browser.mjs',readFileSync(fileURLToPath(import.meta.url))]])writeFileSync(join(out,name),value,{mode:0o600,flag:'wx'});
const persist=()=>writeFileSync(join(out,'result.json'),JSON.stringify(receipt,null,2)+'\n',{mode:0o600});
const record=row=>{receipt.checks.push(row);persist();console.log(JSON.stringify(row));};
let server,browser,page,base,profile,closed=false;
const health=()=>assert(!closed,'Controlled workflow closed');
const timed=async(promise,ms,label)=>{let timer;try{return await Promise.race([promise,new Promise((_,reject)=>{timer=setTimeout(()=>reject(Error(label+' deadline')),ms);})]);}finally{clearTimeout(timer);}};
const watchdog=setTimeout(()=>{receipt.errors.push('Controlled browser hard deadline exceeded');receipt.passed=false;persist();process.exit(1);},180000);
const timestamp='2026-10-08T00:00:00Z';
const row=(projectId,runId,taskId,title,state='queued')=>({id:'q-'+taskId,revision:2,projectId,runId,taskId,taskTitle:title,state,priority:'normal',queuedAt:timestamp,updatedAt:timestamp});
let state,requests;
function reset(){state={'p-a':[row('p-a','run-a','build-a','Build a','running'),row('p-a','run-a','cancel-a','Cancel this extra queued task'),row('p-a','other-run','other-a','Other run a')],'p-b':[row('p-b','run-b','build-b','Build b'),row('p-b','other-run','other-b','Other run b')]};requests=[];}
function queue(projectId){const items=structuredClone(state[projectId]);assert(items);return {revision:70+requests.filter(r=>r.method==='POST').length,observedAt:timestamp,dispatchPaused:false,items,hasMore:false,counts:Object.fromEntries(['queued','paused','reserved','running','blocked','unknown','completed','failed','cancelled'].map(kind=>[kind,items.filter(item=>item.state===kind).length]))};}
const mutations=()=>requests.filter(row=>row.method==='POST');
async function transport(req,res){
 const pathname=new URL(req.url,'http://controlled').pathname;
 try{
  health();const match=pathname.match(/^\/v1\/product\/projects\/(p-a|p-b)\/swarm-queue(?:\/(q-cancel-a|q-build-b)\/(cancel|priority))?$/);assert(match,'Unexpected controlled API path');
  const [,projectId,itemId,action]=match;
  let value;
  if(req.method==='GET'){assert(!itemId);requests.push({method:'GET',pathname});value=queue(projectId);}
  else{
   assert.equal(req.method,'POST');assert(itemId&&action);
   let raw='';for await(const chunk of req){raw+=chunk;assert(Buffer.byteLength(raw)<=1024,'Controlled request body bound');}
   const body=JSON.parse(raw),item=state[projectId].find(row=>row.id===itemId);assert(item);assert.equal(item.revision,2);assert.equal(item.state,'queued');
   if(action==='cancel'){assert.equal(projectId,'p-a');assert.equal(itemId,'q-cancel-a');assert.deepEqual(body,{expectedRevision:2});item.state='cancelled';}
   else{assert.equal(projectId,'p-b');assert.equal(itemId,'q-build-b');assert.deepEqual(body,{expectedRevision:2,priority:'high'});item.priority='high';}
   item.revision++;requests.push({method:'POST',pathname,body});value={item:structuredClone(item),queue:queue(projectId)};
  }
  res.statusCode=200;res.setHeader('Content-Type','application/json');res.end(JSON.stringify(value));
 }catch(error){receipt.errors.push('Controlled transport: '+error.message);res.statusCode=500;res.setHeader('Content-Type','application/json');res.end(JSON.stringify({error:'internal',message:'Controlled queue transport refused unexpected request'}));}
}
function extractedFunction(source,start,end){const from=source.indexOf(start),to=source.indexOf(end,from);assert(from>=0&&to>from);return source.slice(from,to);}
const originalAction=extractedFunction(oldSource,'async function action(path,click,expected=200){','\nasync function openProject');
assert.equal(extractedFunction(newSource,'async function action(path,click,expected=200){','\nasync function openProject'),originalAction);
function makeDriver(source,context){const body=originalAction+'\n'+extractedFunction(source,'async function queueControls(first){','\nasync function activeRestart');return new Function(...Object.keys(context),body+'\nreturn queueControls;')(...Object.values(context));}
function projectStates(){return [{owner:{id:'p-a'},route:'/v1/product/projects/p-a',task:'build-a',queued:[{id:'q-cancel-a',taskId:'cancel-a'}]},{owner:{id:'p-b'},route:'/v1/product/projects/p-b',task:'build-b',queued:[{id:'q-build-b',taskId:'build-b'}]}];}
const region=()=>page.getByRole('region',{name:'Background task queue',exact:true});
async function visit(projectId){health();await page.goto(base+'/queue-fixture?project='+projectId);health();await page.waitForFunction(id=>window.__queueFixture?.projectId===id&&window.__queueFixture.queueChanges>0&&window.__queueFixture.queue!==null,projectId);health();assert.equal(await region().locator('details.swarm-queue-controls').evaluate(el=>el.open),false);assert.equal(await region().getByRole('button',{name:'Cancel queued task',exact:true}).count(),0);assert.equal(await region().locator('li').filter({hasText:/Other run/}).count(),0);}
try{
 reset();
 server=await createServer({configFile:false,root:join(repo,'desktop'),cacheDir:join(out,'vite-cache'),clearScreen:false,resolve:{dedupe:['react','react-dom']},plugins:[react(),tailwind(),{name:'controlled-cloud028-queue-fixture',configureServer(vite){vite.middlewares.use((req,res,next)=>req.url?.startsWith('/v1/product/projects/')?void transport(req,res):next());vite.middlewares.use('/queue-fixture',async(_req,res)=>{try{res.setHeader('Content-Type','text/html');res.end(await vite.transformIndexHtml('/queue-fixture','<!doctype html><html lang="en"><head><meta name="viewport" content="width=device-width,initial-scale=1"><title>Controlled CLOUD028 queue component</title></head><body><div id="root"></div><script type="module" src="/@fs'+fixturePath+'"></script></body></html>'));}catch(error){receipt.errors.push(String(error));res.statusCode=500;res.end('Controlled fixture failed');}});}}],server:{host:'127.0.0.1',port:0,strictPort:false,watch:null,fs:{allow:[repo,prepared]}}});
 await server.listen();base='http://127.0.0.1:'+server.httpServer.address().port;
 mkdirSync('/tmp/lc28',{recursive:true,mode:0o700});profile=mkdtempSync('/tmp/lc28/p-');
 browser=await chromium.launch({headless:true,chromiumSandbox:true,timeout:30000,executablePath:process.env.LUCI_CHROMIUM_EXECUTABLE||'/usr/bin/chromium',env:{PATH:process.env.PATH,TMPDIR:profile,XDG_CONFIG_HOME:join(profile,'config'),XDG_CACHE_HOME:join(profile,'cache')},args:['--no-proxy-server']});
 receipt.browserVersion=browser.version();const context=await browser.newContext({serviceWorkers:'block'});
 await context.route('**/*',route=>{if(new URL(route.request().url()).origin!==base){receipt.errors.push('External request blocked');return route.abort();}return route.continue();});
 page=await context.newPage();page.on('pageerror',error=>receipt.errors.push(error.message));page.setDefaultTimeout(10000);
 for(const width of [1440,390]){
  await page.setViewportSize({width,height:width===390?844:1080});
  for(const corrected of [false,true]){
   reset();const projects=projectStates(),mounts=[];page.setDefaultTimeout(10000);
   const openProject=async state=>{if(corrected&&mounts.length===1){await page.getByText('Future starts cancelled for this queue entry. Existing sessions and saved output are retained.',{exact:true}).waitFor();await page.screenshot({path:join(out,'corrected-cancel-'+width+'.png'),fullPage:true});}await visit(state.owner.id);page.setDefaultTimeout(corrected?10000:750);mounts.push({projectId:state.owner.id,initiallyClosed:true});};
   const bindings={assert,page,health,remainingBudget:()=>corrected?10000:750,setStage:()=>health(),openProject,openQueueControls,projectStates:projects,titles:new Map([['build-a','Build a'],['build-b','Build b']]),effects:()=>({creates:0,starts:0}),privateRun:()=>({operations:[]}),check:()=>{},api:async(method,path)=>{assert.equal(method,'GET');const response=await fetch(base+path);assert.equal(response.status,200);return response.json();}};
   const driver=makeDriver(corrected?newSource:oldSource,bindings);
   if(!corrected){
    let failure;try{await driver(projects[0]);}catch(error){failure=error;}
    assert(failure,'Original helper must fail on the closed disclosure');assert.equal(failure.name,'TimeoutError');assert.match(failure.message,/Cancel queued task/);assert.equal(mutations().length,0);assert.equal(await region().locator('details').evaluate(el=>el.open),false);
    const diagnostic={width,name:failure.name,message:failure.message,mutations:mutations(),mounts};writeFileSync(join(out,'original-failure-'+width+'.json'),JSON.stringify(diagnostic,null,2)+'\n',{mode:0o600,flag:'wx'});
    await page.screenshot({path:join(out,'original-closed-'+width+'.png'),fullPage:true});record({label:'original-exact-driver-refuses-hidden-cancel-'+width,passed:true,expectedFailure:'TimeoutError',writes:0,mounts});
   }else{
    await driver(projects[0]);assert.deepEqual(mutations(),[{method:'POST',pathname:'/v1/product/projects/p-a/swarm-queue/q-cancel-a/cancel',body:{expectedRevision:2}},{method:'POST',pathname:'/v1/product/projects/p-b/swarm-queue/q-build-b/priority',body:{expectedRevision:2,priority:'high'}}]);
    assert.deepEqual(mounts,[{projectId:'p-a',initiallyClosed:true},{projectId:'p-b',initiallyClosed:true}]);assert.equal(await region().getByRole('combobox',{name:'Priority for Build b',exact:true}).inputValue(),'high');assert.equal(await region().getByRole('button',{name:'Cancel queued task',exact:true}).count(),1);
    await page.screenshot({path:join(out,'corrected-priority-'+width+'.png'),fullPage:true});record({label:'corrected-exact-driver-cancel-and-priority-'+width,passed:true,mutations:structuredClone(mutations()),mounts});
    const before=structuredClone(mutations());await openQueueControls(page,health);await openQueueControls(page,health);assert.equal(await region().locator('details').evaluate(el=>el.open),true);assert.deepEqual(mutations(),before);
    const oldLabelCount=await page.getByLabel('Priority for Build b',{exact:true}).count();record({label:'open-disclosure-idempotent-and-priority-selector-'+width,passed:true,oldExactLabelCount:oldLabelCount,exactAccessibleComboboxCount:await region().getByRole('combobox',{name:'Priority for Build b',exact:true}).count(),additionalWrites:0});
   }
  }
 }
 assert.deepEqual(receipt.errors,[]);receipt.passed=true;
}catch(error){receipt.errors.push(error.stack||String(error));process.exitCode=1;if(page)try{await timed(page.screenshot({path:join(out,'unexpected-failure.png'),fullPage:true}),5000,'Failure screenshot');}catch{}}
finally{
 closed=true;
 if(browser)try{await timed(browser.close(),15000,'Owned Chromium close');receipt.cleanup.push('owned Chromium closed');}catch(error){receipt.errors.push(String(error));}
 if(server)try{await timed(server.close(),15000,'Owned Vite close');receipt.cleanup.push('owned Vite closed');}catch(error){receipt.errors.push(String(error));}
 if(profile&&receipt.cleanup.includes('owned Chromium closed'))rmSync(profile,{recursive:true,force:true});
 for(const [path,row]of Object.entries(manifest.productSources))try{assert.equal(sha(readFileSync(join(repo,path))),row.sha256);}catch(error){receipt.errors.push(String(error));}
 receipt.finishedAt=new Date().toISOString();if(receipt.errors.length){receipt.passed=false;process.exitCode=1;}persist();clearTimeout(watchdog);
}
