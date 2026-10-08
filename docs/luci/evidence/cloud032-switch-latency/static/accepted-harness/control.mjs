#!/usr/bin/env node
// Prepared diagnostic only. Exact measured action/verify/two-rAF implementation
// is imported byte-for-byte from the failed installed sustained attempt3.
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {createHash,randomUUID} from 'node:crypto';
import {readFileSync,readdirSync,mkdirSync,writeFileSync,existsSync} from 'node:fs';
import {dirname,join,resolve} from 'node:path';
import {fileURLToPath} from 'node:url';
import {setTimeout as delay} from 'node:timers/promises';
import {interactionRunner,installBrowserTiming,timingSnapshot,createTimingLedger,summarizeInteractions} from './interaction-metrics.mjs';
import {createStartupScope,startAcceptanceBrowser} from './acceptance-startup.mjs';
import {createWorkflowDeadline,boundedAttempt,settleOwnedBrowser,writeFailureCheckpoint} from './acceptance-lifecycle.mjs';

const here=dirname(fileURLToPath(import.meta.url)),args=process.argv.slice(2);
const option=(name,fallback)=>{const at=args.indexOf(name);if(at<0)return fallback;assert(args[at+1]&&!args[at+1].startsWith('--'));return args[at+1];};
if(args.includes('--help')){console.log('Diagnostic only: node control.mjs --report FRESH/result.json [--repo /workspace/luci] [--trace-after yes]. No Docker, Luci service or product pass claim. Fixed100 switches/200 inputs/100 draft resets; quiet60s; workflow180s+cleanup20s. Optional8-round action-only trace AFTER the primary measurements, separately labelled.');process.exit(0);}
const repo=resolve(option('--repo','/workspace/luci')),report=resolve(option('--report')),evidence=dirname(report);
assert(!existsSync(evidence),'A fresh evidence directory is required');mkdirSync(evidence,{recursive:true,mode:0o700});
const traceAfter=option('--trace-after','no');assert(['yes','no'].includes(traceAfter));
const sha=bytes=>createHash('sha256').update(bytes).digest('hex');
const names=['control.mjs','control.html','owners.json','interaction-metrics.mjs','latency-diagnostics.mjs','acceptance-startup.mjs','acceptance-lifecycle.mjs'];
const sources=Object.fromEntries(names.map(name=>[name,sha(readFileSync(join(here,name)))]));
mkdirSync(join(evidence,'accepted-harness'),{mode:0o700});
for(const name of names)writeFileSync(join(evidence,'accepted-harness',name),readFileSync(join(here,name)),{mode:0o600,flag:'wx'});
const owners=JSON.parse(readFileSync(join(here,'owners.json'),'utf8'));assert.equal(owners.length,4);
const html=readFileSync(join(here,'control.html'),'utf8').replace('__CLOUD032_OWNERS__',JSON.stringify(owners).replaceAll('<','\\u003c'));
assert(!html.includes('__CLOUD032_OWNERS__'));
const receipt={version:1,mode:'static-dom-automation-control',diagnosticOnly:true,productAcceptance:false,startedAt:new Date().toISOString(),sources,
 bounds:{workflowMs:180000,cleanupMs:20000,viewport:{width:1440,height:1080},switches:100,inputs:200,draftResets:100,traceAfterRounds:traceAfter==='yes'?8:0},
 metric:'Original Node automation action through expected DOM and final two animation frames. Includes transport and scheduling. No cost subtraction or physical-input claim.',
 targetReference:{switchP95Ms:300,inputP95Ms:100,p99StrictlyBelowMs:1000},
 limitations:['Minimal static DOM has no product rendering, network, persistence or Radix dialog. It estimates the automation floor only.','No Linux whole-process observer in this static control. Original browser EventTiming/resource/input collector is unchanged. A subsequent app comparison must state this observer difference.','Optional trace runs only after primary metrics and changes instrumentation. It cannot explain an unrecorded original action retroactively.'],
 completed:false,interactions:[],browserErrors:[],cleanup:[],lifecycle:[]};
let closing=false,failure,browser,page,server,context,phase='setup',lastOwned=[];
const marker='cloud032-static-'+randomUUID();
const problem=error=>({code:typeof error?.code==='string'?error.code:'diagnostic-failed',name:error?.name??'Error',message:String(error?.message??error).slice(0,1200)});
const fail=error=>{failure??=error;workflow?.fail(error);};
const health=()=>{if(closing)throw Error('Diagnostic closed');workflow?.assertActive();if(failure)throw failure;};
const workflow=createWorkflowDeadline({timeoutMs:180000,onFailure:fail});
const scope=createStartupScope({assertActive:health,onFailure:fail,onTransition:row=>receipt.lifecycle.push({...row,at:new Date().toISOString()})});
function identity(pid){try{const raw=readFileSync('/proc/'+pid+'/stat','utf8'),fields=raw.slice(raw.lastIndexOf(')')+2).trim().split(/\s+/);return{pid:Number(pid),start:fields[19],state:fields[0]};}catch(error){if(['ENOENT','ESRCH'].includes(error.code))return null;throw error;}}
function stillOwned(row){const now=identity(row.pid);return now&&now.start===row.start&&now.state!=='Z';}
function owned(){const rows=[];for(const id of readdirSync('/proc').filter(value=>/^\d+$/.test(value))){const before=identity(id);if(!before||before.state==='Z')continue;try{if(readFileSync('/proc/'+id+'/environ','utf8').split('\0').includes('LUCI_CLOUD032_BROWSER='+marker)&&stillOwned(before))rows.push(before);}catch(error){if(!['ENOENT','ESRCH','EACCES','EPERM'].includes(error.code))throw error;}}lastOwned=rows;return rows;}
const signals=new Map(['SIGINT','SIGTERM'].map(name=>[name,()=>fail(Object.assign(Error('Diagnostic interrupted'),{code:name}))]));for(const [name,handler]of signals)process.on(name,handler);
const ledger=createTimingLedger(receipt);
const observe=async reason=>{health();ledger.append(await timingSnapshot(page),reason);};
try{
 await workflow.run(async()=>{
  const {chromium}=await import(join(repo,'desktop/node_modules/playwright/index.mjs'));health();
  server=await scope.acquire('static-server',{start:async()=>{const handle=createServer((req,res)=>{if(closing){res.writeHead(503).end();return;}if(req.method!=='GET'||req.url!=='/'){res.writeHead(404).end();return;}res.setHeader('Content-Type','text/html');res.end(html);});await new Promise((done,reject)=>handle.once('error',reject).listen(0,'127.0.0.1',done));return handle;},close:async handle=>{handle.closeAllConnections();await new Promise(done=>handle.close(done));}});
  const base='http://127.0.0.1:'+server.address().port;
  const browserEnv=Object.fromEntries(['PATH','LANG','LC_ALL','TZ'].filter(n=>process.env[n]).map(n=>[n,process.env[n]]));browserEnv.LUCI_CLOUD032_BROWSER=marker;
  await startAcceptanceBrowser({scope,assertActive:health,
   launch:()=>chromium.launch({headless:true,executablePath:'/usr/bin/chromium',env:browserEnv,args:['--no-proxy-server']}),adoptBrowser:value=>{browser=value;},closeBrowser:value=>value.close(),
   newContext:value=>value.newContext({viewport:{width:1440,height:1080},serviceWorkers:'block'}),
   installRoutes:async value=>{context=value;await installBrowserTiming(value);health();await value.route('**/*',async route=>{if(closing||failure||new URL(route.request().url()).origin!==base){await route.abort('blockedbyclient');return;}await route.continue();});health();},
   newPage:value=>value.newPage(),adoptPage:value=>{page=value;}});
  page.setDefaultTimeout(15000);page.on('pageerror',error=>{receipt.browserErrors.push(problem(error));fail(Error('Control script error'));});receipt.browserVersion=browser.version();
  await page.goto(base);health();
  const runner=interactionRunner({page,owners,receipt,assertHealthy:health,phase:()=>phase,observe});await runner.warm();
  receipt.quietStartedAt=new Date().toISOString();const quietStart=performance.now();for(let count=0;count<60;count++){health();await delay(1000);}health();receipt.quietElapsedMs=performance.now()-quietStart;assert(receipt.quietElapsedMs>=60000);
  phase='static-control-primary';receipt.primaryStartedAt=new Date().toISOString();await runner.calibrate(100);receipt.primaryFinishedAt=new Date().toISOString();await observe('primary-complete');
  receipt.primarySummary=summarizeInteractions(receipt.interactions);assert.equal(receipt.interactions.filter(row=>row.kind==='project-switch').length,100);assert.equal(receipt.interactions.filter(row=>row.kind==='input').length,200);assert(receipt.interactions.every(row=>row.success));
  receipt.targetComparison=Object.fromEntries(['project-switch','input'].map(kind=>{const row=receipt.primarySummary[phase+':'+kind];return[kind,{p95Ms:row.p95Ms,p99Ms:row.p99Ms,withinReference:row.p95Ms<=(kind==='input'?100:300)&&row.p99Ms<1000}];}));
  if(traceAfter==='yes'){
   phase='static-control-trace-followup';await context.tracing.start({screenshots:false,snapshots:false,sources:false});
   try{for(let i=0;i<8;i++)await runner.round(i);await observe('trace-followup-complete');}
   finally{await boundedAttempt(()=>context.tracing.stop({path:join(evidence,'action-only-trace.zip')}),5000,'trace-stop-timeout');}
  }
  assert.equal(receipt.browserErrors.length,0);receipt.completed=true;
 });
}catch(error){fail(error);receipt.failure=problem(error);}
finally{
 closing=true;workflow.close();const cleanupStarted=Date.now();
 const cleanupError=error=>{receipt.cleanup.push(problem(error));receipt.completed=false;};
 const cleanupWatchdog=setTimeout(()=>{
  receipt.completed=false;receipt.cleanupComplete=false;receipt.cleanupDeadlineExceeded=true;receipt.resources=scope.snapshot();receipt.finishedAt=new Date().toISOString();
  // Only previously observed marker+creation-time identities are eligible.
  // A watchdog exit cannot establish that cleanup finished; retain uncertainty.
  for(const row of lastOwned){try{if(stillOwned(row))process.kill(row.pid,'SIGKILL');}catch{}}
  try{writeFailureCheckpoint(join(evidence,'cleanup-timeout.json'),{version:1,completed:false,cleanupComplete:false,cleanupDeadlineExceeded:true,finishedAt:receipt.finishedAt,resources:receipt.resources,observedOwnedIdentities:lastOwned,measurementCount:receipt.interactions.length,measurementSidecar:'measurements-before-cleanup.json'});}catch{}
  process.exit(1);
 },20000);
 try{writeFileSync(join(evidence,'measurements-before-cleanup.json'),JSON.stringify(receipt,null,2)+'\n',{flag:'wx',mode:0o600});}catch(error){cleanupError(error);}
 try{writeFailureCheckpoint(join(evidence,'cleanup-checkpoint.json'),{version:1,completed:receipt.completed,failure:failure?problem(failure):null,cleanupComplete:false,at:new Date().toISOString(),resources:scope.snapshot()});}catch(error){cleanupError(error);}
 try{if(scope.has('browser'))await scope.close('browser',5000);}catch(error){cleanupError(error);}
 try{await settleOwnedBrowser({listOwned:owned,stillOwned,forceTerminate:row=>{if(stillOwned(row))process.kill(row.pid,'SIGKILL');},onFailure:cleanupError,settleTimeoutMs:5000});const end=Date.now()+3000;while(owned().length&&Date.now()<end)await delay(100);receipt.ownedBrowserProcessesRemaining=owned();if(receipt.ownedBrowserProcessesRemaining.length)cleanupError(Error('Owned browser remains'));}catch(error){cleanupError(error);}
 try{if(scope.has('static-server'))await scope.close('static-server',2000);}catch(error){cleanupError(error);}
 receipt.cleanupMs=Date.now()-cleanupStarted;receipt.resources=scope.snapshot();receipt.cleanupComplete=receipt.cleanup.length===0&&receipt.resources.every(row=>row.state==='closed'&&!row.closeTimedOut);
 receipt.completed=receipt.completed&&!failure&&receipt.cleanupComplete;
 for(const name of names)if(sha(readFileSync(join(here,name)))!==sources[name]){receipt.completed=false;receipt.sourceChanged=name;}
 receipt.interactionSummary=summarizeInteractions(receipt.interactions);receipt.finishedAt=new Date().toISOString();receipt.nodeOwnResourceUsage=process.resourceUsage();
 writeFileSync(report,JSON.stringify(receipt,null,2)+'\n',{flag:'wx',mode:0o600});
 clearTimeout(cleanupWatchdog);
 for(const [name,handler]of signals)process.off(name,handler);
 console.log(JSON.stringify({report,completed:receipt.completed,productAcceptance:false,primarySummary:receipt.primarySummary,cleanupComplete:receipt.cleanupComplete}));
 // A losing bounded promise is not cancellation. The receipt preserves any
 // unsettled scope state; the owned identity fallback has already been attempted.
 process.exitCode=receipt.completed?0:1;setImmediate(()=>process.exit(process.exitCode));
}
