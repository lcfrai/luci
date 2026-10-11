// Pattern: storage-refresh-ui/source-browser.mjs; actual styled ProjectDeploy and
// product client, controlled loopback HTTP responses, no execution backend.
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { readFileSync, writeFileSync, mkdirSync, mkdtempSync, existsSync, readdirSync, rmSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
const here=dirname(fileURLToPath(import.meta.url)), repo='/workspace/luci';
const out=resolve(process.argv[2]);assert(!existsSync(out));mkdirSync(out,{mode:0o700});
assert.equal(process.platform,'linux');assert.notEqual(process.getuid(),0);
const sha=b=>createHash('sha256').update(b).digest('hex');
const paths=['desktop/src/components/lens/DeployLens.tsx','desktop/src/api/product.ts','desktop/src/hooks/useProjectAccess.ts','desktop/src/components/fluid/components/button.tsx','desktop/src/styles/app.css','desktop/src/components/journey/journey.css','desktop/package-lock.json','desktop/vite.config.ts'];
const bind=()=>Object.fromEntries(paths.map(p=>[p,sha(readFileSync(join(repo,p)))]));
const report={passed:false,scope:'Linux Chromium actual styled ProjectDeploy and product client over controlled loopback HTTP; no Luci backend, Docker, model, Windows or performance claim',startedAt:new Date().toISOString(),helperSHA256:sha(readFileSync(fileURLToPath(import.meta.url))),fixtureSHA256:sha(readFileSync(join(here,'fixture.tsx'))),sourceBefore:bind(),calls:[],checks:[],errors:[],cleanup:{}};
const save=()=>writeFileSync(join(out,'result.json'),JSON.stringify(report,null,2)+'\n');
for(const name of ['fixture.tsx','source-browser.mjs'])writeFileSync(join(out,name),readFileSync(join(here,name)));
const profile=mkdtempSync('/workspace/.t/preview-cancel-browser-');
let browser,server,page,context,releaseStart,releaseCancel,base,serverPort;
const known=new Map();
function identity(pid){try{const text=readFileSync(`/proc/${pid}/stat`,'utf8'),parts=text.slice(text.lastIndexOf(')')+2).trim().split(/\s+/);return{pid:Number(pid),ppid:Number(parts[1]),start:parts[19],state:parts[0]};}catch(e){if(['ENOENT','ESRCH'].includes(e.code))return null;throw e;}}
function capture(){const rows=readdirSync('/proc').filter(p=>/^\d+$/.test(p)).map(identity).filter(Boolean);let changed=true;const selected=new Set([process.pid]);while(changed){changed=false;for(const row of rows){const prior=known.get(row.pid);if((selected.has(row.ppid)||prior?.start===row.start)&&!selected.has(row.pid)){selected.add(row.pid);known.set(row.pid,row);changed=true;}}}}
const monitor=setInterval(capture,500);monitor.unref();
async function bounded(promise,ms,label){let timer;try{return await Promise.race([promise,new Promise((_,reject)=>{timer=setTimeout(()=>reject(new Error(label+' exceeded '+ms+' ms')),ms);})]);}finally{clearTimeout(timer);}}
const hard=setTimeout(()=>{report.passed=false;report.error='180 second hard deadline';capture();report.cleanup.known=[...known.values()];save();process.exit(124);},180000);hard.unref();
let rows=[];
const active={id:'preview-alpha',projectId:'alpha',briefRevision:1,revision:4,canCancel:true,canStop:false,cancelRequested:false,commit:'a'.repeat(40),target:'local',status:'building',url:'',message:'Alpha reviewed source is building',createdAt:'2026-10-11T13:30:00Z'};
const requested={...active,revision:5,status:'cancelling',canCancel:false,cancelRequested:true,message:'Cancellation intent recorded; backend outcome still pending'};
const unknown={...requested,revision:6,status:'unknown',message:'Alpha build acknowledgement is unknown; capacity remains reserved'};
const send=(res,status,value)=>{if(!res.destroyed){res.statusCode=status;res.setHeader('Content-Type','application/json');res.end(JSON.stringify(value));}};
try{
 const {createServer}=await import(pathToFileURL(join(repo,'desktop/node_modules/vite/dist/node/index.js')).href);
 const {chromium}=await import(pathToFileURL(join(repo,'desktop/node_modules/playwright/index.mjs')).href);
 server=await createServer({configFile:join(repo,'desktop/vite.config.ts'),root:join(repo,'desktop'),server:{host:'127.0.0.1',port:0,strictPort:false,watch:null,fs:{allow:[repo,here]}},plugins:[{name:'controlled-preview-cancel',configureServer(vite){
  vite.middlewares.use('/preview-cancel-fixture',async(req,res)=>{res.setHeader('Content-Type','text/html');res.end(await vite.transformIndexHtml('/preview-cancel-fixture','<!doctype html><html lang="en"><head><meta name="viewport" content="width=device-width,initial-scale=1"><title>Controlled preview cancellation</title></head><body><div id="root"></div><script>window.__LUCI_API__=location.origin;</script><script type="module" src="/@fs'+join(here,'fixture.tsx')+'"></script></body></html>'));});
  vite.middlewares.use('/v1',async(req,res)=>{let text='';for await(const chunk of req){text+=chunk;assert(text.length<4096);}const path='/v1'+req.url,body=text?JSON.parse(text):null;report.calls.push({path,method:req.method,body,at:new Date().toISOString()});
   if(req.method==='GET'&&path==='/v1/product/projects/alpha/releases')return send(res,200,{releases:rows});
   if(req.method==='GET'&&path==='/v1/product/projects/beta/releases')return send(res,200,{releases:[]});
   if(req.method==='POST'&&path==='/v1/product/projects/alpha/releases'){assert.deepEqual(body,{target:'local',port:3000,dockerfile:'Dockerfile'});rows=[active];releaseStart=()=>send(res,422,{...unknown,error:'preview_build_unconfirmed'});return;}
   if(req.method==='POST'&&path==='/v1/product/projects/alpha/releases/preview-alpha/cancel'){assert.deepEqual(body,{expectedRevision:4});releaseCancel=()=>{rows=[requested];send(res,202,requested);};return;}
   report.errors.push('Unexpected fixture HTTP '+req.method+' '+path);send(res,500,{error:'unexpected_fixture_request'});
  });
 }}]});await server.listen();serverPort=server.httpServer.address().port;base='http://127.0.0.1:'+serverPort;
 browser=await chromium.launch({headless:true,chromiumSandbox:true,executablePath:'/usr/bin/chromium',env:{PATH:process.env.PATH,TMPDIR:profile,HOME:profile},args:['--no-proxy-server']});report.browserVersion=browser.version();capture();
 context=await browser.newContext({serviceWorkers:'block'});await context.route('**/*',route=>{if(new URL(route.request().url()).origin!==base){report.errors.push('External request refused');return route.abort();}return route.continue();});page=await context.newPage();page.setDefaultTimeout(12000);page.on('pageerror',e=>report.errors.push(e.message));
 for(const width of [1440,390]){
  rows=[];releaseStart=releaseCancel=undefined;const startIndex=report.calls.length;
  await page.setViewportSize({width,height:width===390?844:1080});await page.goto(base+'/preview-cancel-fixture');await page.getByText('No releases yet. Your first preview will appear here.',{exact:true}).waitFor();
  await page.getByRole('button',{name:'Build local preview',exact:true}).click();
  const cancel=page.getByRole('button',{name:'Cancel build',exact:true});await cancel.waitFor({timeout:15000});assert(await cancel.isEnabled());assert(releaseStart,'Start response must remain held');
  await cancel.scrollIntoViewIfNeeded();await page.screenshot({path:join(out,`held-start-before-cancel-${width}.png`)});
  await cancel.focus();await page.keyboard.press('Enter');await page.getByText(/This does not yet confirm that the build stopped/).waitFor();
  await page.waitForFunction(()=>document.querySelector('button')!==null);assert(releaseCancel,'Cancel request must reach controlled HTTP');
  await page.screenshot({path:join(out,`cancel-request-pending-${width}.png`)});
  releaseCancel();releaseCancel=undefined;await page.getByText(/Waiting for the build outcome and confirmed cleanup/).waitFor();
  await page.getByRole('button',{name:'Project Beta',exact:true}).click();await page.getByRole('heading',{name:'Beta project',exact:true}).waitFor();
  rows=[unknown];releaseStart();releaseStart=undefined;
  await page.getByText('No releases yet. Your first preview will appear here.',{exact:true}).waitFor();assert.equal(await page.getByText('Alpha build acknowledgement is unknown; capacity remains reserved',{exact:true}).count(),0);assert.equal(await page.getByRole('alert').count(),0);
  await page.getByRole('button',{name:'Project Alpha',exact:true}).click();await page.getByText(/build outcome or cleanup is unconfirmed/).waitFor();
  const refresh=page.getByRole('button',{name:'Refresh',exact:true});const readsBefore=report.calls.filter(c=>c.path==='/v1/product/projects/alpha/releases'&&c.method==='GET').length;
  await refresh.focus();await page.keyboard.press('Enter');await page.getByText('Refreshing release history…',{exact:true}).waitFor({state:'hidden'});
  assert.equal(report.calls.filter(c=>c.path==='/v1/product/projects/alpha/releases'&&c.method==='GET').length,readsBefore+1);
  assert(await page.getByRole('button',{name:'Build local preview',exact:true}).isDisabled());assert.equal(await page.getByRole('button',{name:'Cancel build',exact:true}).count(),0);
  const geometry=await page.evaluate(()=>({viewport:innerWidth,width:document.documentElement.scrollWidth,buttons:[...document.querySelectorAll('button')].filter(b=>b.textContent==='Refresh').map(b=>({background:getComputedStyle(b).backgroundColor,border:getComputedStyle(b).borderRadius}))}));assert(geometry.width<=width+1,'No horizontal overflow');
  await page.getByText(/build outcome or cleanup is unconfirmed/).scrollIntoViewIfNeeded();await page.screenshot({path:join(out,`unknown-after-refresh-${width}.png`)});
  const calls=report.calls.slice(startIndex),posts=calls.filter(c=>c.method==='POST');assert.equal(posts.length,2);assert.equal(posts.filter(c=>c.path.endsWith('/cancel')).length,1);
  report.checks.push({width,heldStartCancelEnabled:true,keyboardCancel:true,requestIntentNotCleanup:true,lateStartProjectIsolated:true,unknownRetainedAfterExplicitRefresh:true,noReplay:true,geometry,calls});save();
 }
 assert.deepEqual(report.errors,[]);report.sourceAfter=bind();assert.deepEqual(report.sourceAfter,report.sourceBefore);report.passed=true;
}catch(error){report.error=String(error.stack||error);process.exitCode=1;}
finally{
 // Persist failure and exact known ownership before any potentially blocking close.
 capture();report.cleanup.before=[...known.values()];save();
 try{releaseCancel?.();releaseStart?.();}catch(e){report.cleanup.releaseError=e.message;report.passed=false;}
 try{await bounded(browser?.close()??Promise.resolve(),10000,'browser close');report.cleanup.browserClose=true;}catch(e){report.cleanup.browserError=e.message;report.passed=false;}
 try{await bounded(server?.close()??Promise.resolve(),5000,'Vite close');report.cleanup.serverClose=true;}catch(e){report.cleanup.serverError=e.message;report.passed=false;}
 clearInterval(monitor);clearTimeout(hard);
 report.cleanup.processes=[...known.values()].map(row=>{const now=identity(row.pid);return{...row,closure:!now||now.start!==row.start?'absent':now.state==='Z'?'zombie':'still-live',current:now};});
 if(report.cleanup.processes.some(x=>x.closure==='still-live')){report.passed=false;report.cleanup.liveOwnedProcesses=true;}
 report.cleanup.serverPort=serverPort;report.cleanup.profile=profile;
 if(report.cleanup.browserClose&&!report.cleanup.liveOwnedProcesses){rmSync(profile,{recursive:true});report.cleanup.profileRemoved=true;}
 report.finishedAt=new Date().toISOString();save();console.log(JSON.stringify({passed:report.passed,report:join(out,'result.json'),error:report.error??null}));if(!report.passed)process.exitCode=1;
}
