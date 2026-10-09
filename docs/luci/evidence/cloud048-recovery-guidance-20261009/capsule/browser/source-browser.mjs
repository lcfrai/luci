#!/usr/bin/env node
// Bounded actual-component browser check. Controlled API transport; no app runtime.
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {execFileSync} from 'node:child_process';
import {existsSync,mkdirSync,mkdtempSync,readFileSync,readdirSync,rmSync,statSync,writeFileSync} from 'node:fs';
import {dirname,join,resolve} from 'node:path';
import {fileURLToPath,pathToFileURL} from 'node:url';
assert.equal(process.env.LUCI_CLOUD048_SOURCE_APPLIED,'yes','Wait for source application confirmation');
assert.equal(process.env.GOMAXPROCS,'2','Bound esbuild parallelism');
const here=dirname(fileURLToPath(import.meta.url)),repo='/workspace/luci',out=resolve(process.argv[2]??'');
assert.equal(dirname(out),here);assert(!existsSync(out),'Evidence directory must be fresh');
mkdirSync(out,{mode:0o700});
const hash=body=>createHash('sha256').update(body).digest('hex'),fileHash=path=>hash(readFileSync(path));
const fixture=join(here,'fixture.tsx'),runner=fileURLToPath(import.meta.url),sources={};
for(const file of ['desktop/src/components/journey/ProjectSwarm.tsx','desktop/src/components/journey/SwarmRunQueue.tsx','desktop/src/components/journey/ProjectSwarmQueue.test.tsx','desktop/src/components/journey/project-swarm.css','desktop/src/components/journey/journey.css','desktop/src/styles/app.css'])sources[join(repo,file)]=fileHash(join(repo,file));
const receipt={passed:false,scope:'Actual styled ProjectSwarm and SwarmRunQueue in Chromium with controlled HTTP transport and actual API validation',limitations:['No installed backend, Docker, native agent, provider or model execution.','Fixture mounts the two real components in an explicit scroll container; it is not AppShell or full-product acceptance.','Initial failure and subsequent retirement/queue receipts are controlled state, not live admission evidence.'],startedAt:new Date().toISOString(),checkout:execFileSync('git',['rev-parse','HEAD'],{cwd:repo,encoding:'utf8'}).trim(),sourceBefore:sources,fixtureBefore:fileHash(fixture),runnerBefore:fileHash(runner),checks:[],errors:[],cleanup:[],requests:[]};
writeFileSync(join(out,'fixture.tsx'),readFileSync(fixture),{mode:0o600,flag:'wx'});writeFileSync(join(out,'source-browser.mjs'),readFileSync(runner),{mode:0o600,flag:'wx'});
const save=()=>writeFileSync(join(out,'result.json'),JSON.stringify(receipt,null,2)+'\n',{mode:0o600});
const log=(kind,value)=>{const row={at:new Date().toISOString(),kind,...value};writeFileSync(join(out,'raw.jsonl'),JSON.stringify(row)+'\n',{flag:'a',mode:0o600});};
const check=(label,detail={})=>{receipt.checks.push({label,passed:true,...detail});log('check',{label,...detail});save();};
const timed=async(promise,ms,label)=>{let timer;try{return await Promise.race([promise,new Promise((_,reject)=>{timer=setTimeout(()=>reject(Error(label+' timed out')),ms);})]);}finally{clearTimeout(timer);}};
let server,browser,page,profile,base,closed=false,mode,current,items,scenarioRequests;
const health=()=>assert(!closed,'Browser workflow already closed');
const now='2026-10-09T08:00:00Z';
const failure={stage:'admission',reasonCode:'cpu_pressure',message:'Measured CPU pressure prevented final admission. No sandbox allocation was attempted. Inspect this retained attempt before explicitly retrying.'};
function makeRun(projectId='p-1',runId='run-1',failed=true){return {id:runId,projectId,worktreeId:'main',revision:failed?1:2,state:failed?'blocked':'ready',reason:'Controlled reviewed inputs',createdAt:now,updatedAt:now,input:{implementation:{projectId,worktreeId:'main',briefRevision:1,planRevision:1,specificationRevision:1,designPlanRevision:1,taskIds:['a'],taskTitles:['Build a'],requirementIds:['r1'],briefDigest:'brief',planDigest:'plan',specificationDigest:'spec',designPlanDigest:'design',sourceCommit:'a'.repeat(40),sourceDigest:'source',sourceDirty:false},approvals:{specificationApprovalId:'as',specificationRevision:1,specificationDigest:'spec',designApprovalId:'ad',designRevision:1,designDigest:'design',briefDigest:'brief',sketchDigest:''},outcome:'preview',requirements:[{id:'r1',title:'App',acceptance:'Works'}],route:{harness:'codex',model:'recorded-model',permissionMode:'bypass',connection:{id:'saved',kind:'compatible-api',revision:3}}},operations:failed?[{id:'op-refused',kind:'session',taskId:'a',childId:'child-refused',worktreeId:'isolated-refused',attempt:1,status:'failed',allocation:'ready',requestDigest:'digest',createdAt:now,updatedAt:now,startFailure:failure}]:[],checks:[],releases:[],allowedActions:[],contextFreshness:{status:'current',reason:'Current'},swarm:{state:'ready',maxParallel:1,tasks:[{id:'a',title:'Build a',dependsOn:[],writePaths:['src/a.ts'],acceptance:'Verify the reviewed behavior',...(failed?{operationId:'op-refused'}:{})}]}};}
const row=(id,taskId,state='queued')=>({id,revision:2,projectId:'p-1',runId:'run-1',taskId,taskTitle:taskId==='a'?'Build a':'Existing saved work '+taskId,state,priority:'normal',queuedAt:now,updatedAt:now});
function reset(value){mode=value;scenarioRequests=[];current=makeRun();items=[{...row('q-failed','a','failed'),operationId:'op-refused',childId:'child-refused',reasonCode:'cpu_pressure',reason:failure.message},...Array.from({length:24},(_,index)=>({...row('q-history-'+index,'history-'+index,'paused'),reason:'Existing reviewed work remains paused; inspect its priority and controls before continuing.'}))];for(const item of items.slice(1)){current.swarm.tasks.push({id:item.taskId,title:item.taskTitle,dependsOn:[],writePaths:['src/'+item.taskId+'.ts'],acceptance:'Verify this separately reviewed saved task'});current.input.implementation.taskIds.push(item.taskId);current.input.implementation.taskTitles.push(item.taskTitle);}if(mode==='queued')items.push(row('q-already-reviewed','a'));if(mode!=='enabled'){current.revision=2;current.operations[0].status='cancelled';}}
function queue(projectId='p-1'){const selected=projectId==='p-1'?structuredClone(items):[];return {revision:70,observedAt:now,dispatchPaused:false,hasMore:false,items:selected,counts:Object.fromEntries(['queued','paused','reserved','running','blocked','unknown','completed','failed','cancelled'].map(state=>[state,selected.filter(item=>item.state===state).length]))};}
const mutations=()=>scenarioRequests.filter(request=>request.method!=='GET');
async function transport(req,res){
 const path=new URL(req.url,'http://controlled').pathname;let body;
 try{
  health();if(req.method!=='GET'){let raw='';for await(const part of req){raw+=part;assert(Buffer.byteLength(raw)<4096);}body=JSON.parse(raw);}
  const request={method:req.method,path,...(body===undefined?{}:{body})};scenarioRequests.push(request);receipt.requests.push({mode,...request});log('api',{mode,...request});
  let value;if(path==='/controlled/bootstrap'){assert.equal(req.method,'GET');value={mode,run:current,nextRun:makeRun('p-next','run-next',false)};}
  else if(req.method==='GET'&&/^\/v1\/product\/projects\/(p-1|p-next)\/swarm-queue$/.test(path))value=queue(path.includes('p-next')?'p-next':'p-1');
  else if(req.method==='GET'&&path==='/v1/product/projects/p-1/runs/run-1')value={run:current};
  else if(req.method==='GET'&&path==='/v1/product/projects/p-next/runs/run-next')value={run:makeRun('p-next','run-next',false)};
  else if(req.method==='POST'&&path==='/v1/product/projects/p-1/runs/run-1/swarm/tasks/a/abandon'){
   assert.equal(mode,'enabled');assert.equal(current.revision,1);assert.deepEqual(body,{expectedRevision:1});current=structuredClone(current);current.revision=2;current.operations[0].status='cancelled';value={run:current};
  }else if(req.method==='POST'&&path==='/v1/product/projects/p-1/runs/run-1/swarm/queue'){
   assert.equal(mode,'enabled');assert.equal(current.revision,2);assert.equal(body.expectedRevision,2);assert.deepEqual(body.taskIds,['a']);assert.equal(body.priority,'normal');assert.match(body.idempotencyKey,/^[a-f0-9-]{36}$/);assert(!items.some(item=>item.id==='q-new'));const added=row('q-new','a');items.push(added);value={items:[added],queue:queue()};
  }else throw Error('Unexpected API request '+req.method+' '+path);
  res.statusCode=200;res.setHeader('Content-Type','application/json');res.end(JSON.stringify(value));
 }catch(error){receipt.errors.push('Transport: '+error.message);log('transport-error',{message:error.message});res.statusCode=500;res.setHeader('Content-Type','application/json');res.end(JSON.stringify({message:'Controlled API refused unexpected request'}));}
}
async function geometry(locator){return locator.evaluate(element=>{const r=element.getBoundingClientRect();return {tag:element.tagName,focused:document.activeElement===element,x:r.x,y:r.y,width:r.width,height:r.height,bottom:r.bottom,right:r.right,viewportWidth:innerWidth,viewportHeight:innerHeight};});}
async function frames(){await page.evaluate(()=>new Promise(done=>requestAnimationFrame(()=>requestAnimationFrame(done))));}
function ownedBrowserPids(){if(!profile)return [];return readdirSync('/proc').filter(name=>/^\d+$/.test(name)).flatMap(name=>{try{const path='/proc/'+name;if(statSync(path).uid!==process.getuid())return [];const values=readFileSync(path+'/environ','utf8').split('\0');return values.includes('LUCI_CLOUD048_BROWSER_PROFILE='+profile)?[Number(name)]:[];}catch{return [];}});}
async function workflow(){
 const {createServer}=await import(pathToFileURL(join(repo,'desktop/node_modules/vite/dist/node/index.js')).href);
 const {default:react}=await import(pathToFileURL(join(repo,'desktop/node_modules/@vitejs/plugin-react/dist/index.js')).href);
 const {default:tailwind}=await import(pathToFileURL(join(repo,'desktop/node_modules/@tailwindcss/vite/dist/index.mjs')).href);
 const {chromium}=await import(pathToFileURL(join(repo,'desktop/node_modules/playwright/index.mjs')).href);
 server=await createServer({configFile:false,root:join(repo,'desktop'),cacheDir:join(out,'vite-cache'),clearScreen:false,resolve:{dedupe:['react','react-dom']},plugins:[{name:'cloud048-source-binding',enforce:'pre',transform(_code,id){const path=id.split('?')[0];if(path.startsWith(repo+'/desktop/src/')&&existsSync(path)){const digest=fileHash(path);if(sources[path])assert.equal(sources[path],digest,'Source changed during serving: '+path);else sources[path]=digest;}},configureServer(vite){vite.middlewares.use((req,res,next)=>req.url?.startsWith('/v1/')||req.url?.startsWith('/controlled/')?void transport(req,res):next());vite.middlewares.use('/recovery-fixture',async(_req,res)=>{try{res.setHeader('Content-Type','text/html');res.end(await vite.transformIndexHtml('/recovery-fixture','<!doctype html><html lang="en"><head><meta name="viewport" content="width=device-width,initial-scale=1"><link rel="icon" href="data:,"><title>CLOUD048 controlled recovery</title></head><body><div id="root"></div><script type="module" src="/@fs'+fixture+'"></script></body></html>'));}catch(error){receipt.errors.push(error.stack);res.statusCode=500;res.end('Fixture failed');}});}},react(),tailwind()],server:{host:'127.0.0.1',port:15148,strictPort:true,watch:null,fs:{allow:[repo,here]}}});
 await server.listen();base='http://127.0.0.1:15148';profile=mkdtempSync(join(here,'profile-'));
 // Chromium's Unix socket pathname must stay short. The exact owned user-data
 // profile and XDG paths remain here; Chromium manages its socket under /tmp.
 browser=await chromium.launchPersistentContext(profile,{headless:true,chromiumSandbox:true,serviceWorkers:'block',executablePath:'/usr/bin/chromium',timeout:30000,env:{PATH:process.env.PATH,TMPDIR:'/tmp',XDG_CONFIG_HOME:join(profile,'config'),XDG_CACHE_HOME:join(profile,'cache'),LUCI_CLOUD048_BROWSER_PROFILE:profile},args:['--no-proxy-server']});
 receipt.browserVersion=browser.browser().version();const context=browser;await context.route('**/*',route=>{if(new URL(route.request().url()).origin===base)return route.continue();receipt.errors.push('External request blocked: '+route.request().url());return route.abort();});
 page=await context.newPage();page.setDefaultTimeout(10000);page.on('pageerror',error=>{receipt.errors.push(error.message);log('pageerror',{message:error.stack});});page.on('console',message=>{log('console',{type:message.type(),text:message.text()});if(message.type()==='error')receipt.errors.push('Console: '+message.text());});page.on('response',response=>{if(response.status()>=400)log('http-error',{status:response.status(),url:response.url()});});
 for(const width of [1440,390])for(const state of ['enabled','queued','execute-revoked']){
  health();reset(state);await page.setViewportSize({width,height:width===390?844:1080});await page.goto(base+'/recovery-fixture');await page.getByRole('heading',{name:'Agent team',exact:true}).waitFor();
  const queueRegion=page.getByRole('region',{name:'Background queue review',exact:true});const review=page.getByRole('button',{name:/^Queue reviewed tasks/});
  await page.getByText(/^Queue status and controls/).click();await page.getByRole('button',{name:'Resume queued task',exact:true}).first().waitFor();
  if(state==='enabled'){
   assert.equal(await page.getByRole('button',{name:'Go to queue review',exact:true}).count(),0);
   await page.getByText('More task actions',{exact:true}).click();await page.getByRole('button',{name:'Retain changes and retry',exact:true}).click();
   await page.getByRole('button',{name:'Retain output and allow retry',exact:true}).click();
   await page.getByRole('button',{name:'Go to queue review',exact:true}).waitFor();await page.waitForFunction(()=>window.__cloud048.busy==='');assert.equal(mutations().length,1);assert(mutations()[0].path.endsWith('/abandon'));
  }
  const next=page.getByRole('button',{name:'Go to queue review',exact:true});await next.waitFor();await page.getByText('Previous start: '+failure.message,{exact:true}).waitFor();
  assert.equal(await review.isDisabled(),state!=='enabled');const initialQueue=await geometry(queueRegion);assert(initialQueue.height>2*(width===390?844:1080),'Expanded existing queue must be taller than two viewports');
  await next.scrollIntoViewIfNeeded();await frames();const before=structuredClone(mutations());await next.click();await frames();
  assert.equal(await page.getByRole('dialog').count(),0);assert.deepEqual(mutations(),before);assert.equal(await page.evaluate(()=>window.__cloud048.unexpectedSessionOpens),0);
  const target=state==='enabled'?review:queueRegion,position=await geometry(target);assert.equal(position.focused,true);assert(position.y>=-1&&position.y<position.viewportHeight,'Focused target top must be visible');
  if(state==='enabled'){assert.equal(position.tag,'BUTTON');assert(position.bottom<=position.viewportHeight+1&&position.x>=-1&&position.right<=position.viewportWidth+1,'Native review button must fit in viewport');}
  await page.screenshot({path:join(out,state+'-navigation-'+width+'.png')});check('navigation-'+state+'-'+width,{position,expandedQueueHeight:initialQueue.height,mutationsBefore:before.length,mutationsAfter:mutations().length,consentOpened:false});
  if(state==='enabled'){
   await page.keyboard.press('Enter');const dialog=page.getByRole('dialog',{name:'Queue reviewed work',exact:true});await dialog.waitFor();assert.deepEqual(mutations(),before);await dialog.getByText('Build a',{exact:true}).waitFor();
   await page.screenshot({path:join(out,'separate-consent-'+width+'.png')});await dialog.getByRole('button',{name:'Authorize queued tasks',exact:true}).click();await page.getByText('1 tasks queued.',{exact:false}).waitFor();
   assert.equal(mutations().length,before.length+1);assert(mutations().at(-1).path.endsWith('/swarm/queue'));assert.equal(mutations().filter(value=>value.path.endsWith('/swarm/queue')).length,1);assert.equal(await page.evaluate(()=>window.__cloud048.unexpectedSessionOpens),0);check('separate-single-authorization-'+width,{mutations:structuredClone(mutations())});
   await page.evaluate(()=>window.__cloud048.switchProject());await page.waitForFunction(()=>window.__cloud048.run.projectId==='p-next');assert.equal(await page.getByRole('button',{name:'Go to queue review',exact:true}).count(),0);assert.equal(await page.getByRole('dialog').count(),0);assert.equal(mutations().length,before.length+1);check('project-switch-clears-guidance-'+width);
  }
 }
 assert.deepEqual(receipt.errors,[]);receipt.passed=true;
}
const hardStop=setTimeout(()=>{receipt.errors.push('Hard 178 second deadline');receipt.passed=false;for(const pid of ownedBrowserPids())try{process.kill(pid,'SIGKILL');}catch{}save();process.exit(1);},178000);
try{await timed(workflow(),148000,'Controlled browser workflow');}catch(error){receipt.errors.push(error.stack??String(error));process.exitCode=1;if(page)try{await timed(page.screenshot({path:join(out,'failure.png')}),3000,'Failure screenshot');}catch{}}
finally{
 closed=true;
 if(browser)try{await timed(browser.close(),10000,'Owned browser close');receipt.cleanup.push('Owned Chromium closed');}catch(error){receipt.errors.push(String(error));}
 if(server)try{await timed(server.close(),10000,'Owned Vite close');receipt.cleanup.push('Owned Vite closed');}catch(error){receipt.errors.push(String(error));}
 const remaining=ownedBrowserPids();if(remaining.length){receipt.errors.push('Owned Chromium processes required forced cleanup');for(const pid of remaining)try{process.kill(pid,'SIGKILL');}catch{}await new Promise(done=>setTimeout(done,250));}
 receipt.remainingOwnedBrowserPids=ownedBrowserPids();if(receipt.remainingOwnedBrowserPids.length)receipt.errors.push('Owned browser processes remain');
 if(profile&&!receipt.remainingOwnedBrowserPids.length){assert.equal(dirname(profile),here);assert(profile.startsWith(join(here,'profile-')));rmSync(profile,{recursive:true});receipt.cleanup.push('Owned profile removed');}
 receipt.sourceAfter=Object.fromEntries(Object.keys(sources).map(path=>[path,fileHash(path)]));receipt.fixtureAfter=fileHash(fixture);receipt.runnerAfter=fileHash(runner);
 receipt.sourceUnchanged=JSON.stringify(receipt.sourceBefore)===JSON.stringify(receipt.sourceAfter)&&receipt.fixtureBefore===receipt.fixtureAfter&&receipt.runnerBefore===receipt.runnerAfter;
 if(!receipt.sourceUnchanged)receipt.errors.push('Source, fixture or runner changed');receipt.finishedAt=new Date().toISOString();if(receipt.errors.length){receipt.passed=false;process.exitCode=1;}save();clearTimeout(hardStop);console.log(JSON.stringify({passed:receipt.passed,report:join(out,'result.json'),checks:receipt.checks.length,errors:receipt.errors,sourceUnchanged:receipt.sourceUnchanged,cleanup:receipt.cleanup}));
}
