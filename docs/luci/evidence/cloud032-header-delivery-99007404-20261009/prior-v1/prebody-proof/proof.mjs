#!/usr/bin/env node
// One affected proof: natural cancellation while getResponseBody is pending.
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {createHash} from 'node:crypto';
import {existsSync,mkdirSync,mkdtempSync,readFileSync,readdirSync,rmSync,statSync,writeFileSync} from 'node:fs';
import {dirname,join,resolve} from 'node:path';
import {fileURLToPath,pathToFileURL} from 'node:url';
assert.equal(process.env.GOMAXPROCS,'2');
const here=dirname(fileURLToPath(import.meta.url)),out=resolve(process.argv[2]??'');assert.equal(dirname(out),here);assert(!existsSync(out));mkdirSync(out,{mode:0o700});
const hash=value=>createHash('sha256').update(value).digest('hex'),runner=fileURLToPath(import.meta.url);
const original='/workspace/.luci-tools/cloud032-idle-integrated-candidate-20261009/actual/sustained.mjs',oldManifest='/workspace/.luci-tools/cloud032-summary-delivery-diagnostic-20261009/transport-proof/publication-manifest.json';
const source=readFileSync(original,'utf8'),sourceSHA=hash(source);assert.equal(sourceSHA,'18429d2a6497cfec582c8565b572b3ca567b756f2f9d403efd79b6b405719dc2');
const previousManifest=readFileSync(oldManifest),previousRoot=dirname(oldManifest),previous=JSON.parse(previousManifest);
const previousFiles=Object.fromEntries(previous.members.map(row=>{const actual=hash(readFileSync(join(previousRoot,row.path)));assert.equal(actual,row.sha256);return[row.path,actual];}));
const start=source.indexOf("await context.route('**/*',async route=>"),suffix='await route.continue();});',end=source.indexOf(suffix,start)+suffix.length;assert(start>=0&&end>start);
const registration=source.slice(start,end),handler=registration.slice("await context.route('**/*',".length,-2);
const receipt={passed:false,scope:'ONE additional pre-body natural-cancellation case; previous 18 checkpoints not repeated or counted here; main diagnostic NOT RUN',startedAt:new Date().toISOString(),sourceBinding:{original,sourceSHA256:sourceSHA,guardSHA256:hash(registration),runnerSHA256:hash(readFileSync(runner)),previousManifestSHA256:hash(previousManifest),previousMemberHashes:previousFiles},caps:{requests:4,declaredBodyBytes:65536,workflowMs:20000,hardStopMs:88000,captureJoinAfterAbortMs:2000},browserMutations:[],guard:[],server:[],checkpoints:[],networkFailures:[],errors:[],cleanup:[],protocolCommands:[]};
writeFileSync(join(out,'proof.mjs'),readFileSync(runner),{mode:0o600,flag:'wx'});writeFileSync(join(out,'frozen-route-guard.txt'),registration+'\n',{mode:0o600,flag:'wx'});
const save=()=>writeFileSync(join(out,'result.json'),JSON.stringify(receipt,null,2)+'\n',{mode:0o600});
const event=(kind,details={})=>writeFileSync(join(out,'raw.jsonl'),JSON.stringify({at:new Date().toISOString(),monotonicMs:performance.now(),kind,...details})+'\n',{mode:0o600,flag:'a'});
const checkpoint=(name,details={})=>{receipt.checkpoints.push({name,...details});event('checkpoint',{name,...details});save();};
const error=value=>{receipt.errors.push(value.stack??String(value));event('error',{message:String(value)});};
const timed=async(promise,ms,label)=>{let timer;try{return await Promise.race([promise,new Promise((_,reject)=>{timer=setTimeout(()=>reject(Error(label+' timeout')),ms);})]);}finally{clearTimeout(timer);}};
let server,context,page,cdp,base,profile,targetURL,targetResponse,bodyTimer,closing=false,fixtureError=null,workflowControl={failed:false},stage='prebody-transport-proof';
let capturePromise,captureSettled=false,captureResult,paused,pausedResolved=false,browserRead,browserClosed=false;
let resolveCaptureStarted;const captureStarted=new Promise(resolve=>{resolveCaptureStarted=resolve;});
let resolveNetworkFailure;const networkFailure=new Promise(resolve=>{resolveNetworkFailure=resolve;});
let resolveServerClosed;const serverTargetClosed=new Promise(resolve=>{resolveServerClosed=resolve;});
const send=async(method,params)=>{receipt.protocolCommands.push({method,requestId:params?.requestId,captureInFlight:!!capturePromise&&!captureSettled});event('protocol-command',{method,requestId:params?.requestId,captureInFlight:!!capturePromise&&!captureSettled});return cdp.send(method,params);};
function stopOnFailure(value){fixtureError??=value;error(value);}
function ownedPids(){if(!profile)return[];return readdirSync('/proc').filter(name=>/^\d+$/.test(name)).flatMap(name=>{try{const path='/proc/'+name;return statSync(path).uid===process.getuid()&&readFileSync(path+'/environ','utf8').split('\0').includes('LUCI_C032_PREBODY_PROFILE='+profile)?[Number(name)]:[];}catch{return [];}});}
async function closeOwnedBrowser(reason){if(!context||browserClosed)return;await timed(context.close(),8000,'Owned Chromium close');browserClosed=true;receipt.cleanup.push('Owned Chromium closed: '+reason);}
async function run(){
 const body=JSON.stringify({original:'This body is deliberately withheld before natural cancellation. ☃'});
 server=createServer((req,res)=>{
  const url=new URL(req.url,base).href,row={url,method:req.method,bodyBytesWritten:0,headersSent:false,responseClosed:false,responseFinished:false};receipt.server.push(row);event('server-request',{url,method:req.method});
  if(url===targetURL){
   targetResponse=res;row.plannedBodyBytes=Buffer.byteLength(body);assert(row.plannedBodyBytes<=65536);res.writeHead(200,{'Content-Type':'application/json; charset=utf-8','Content-Length':String(row.plannedBodyBytes),'Cache-Control':'no-store','X-Proof-Identity':'original-prebody','Connection':'close'});res.flushHeaders();row.headersSent=true;event('server-headers-flushed',{url,plannedBodyBytes:row.plannedBodyBytes,bodyBytesWritten:0});
   // Normal response completion is deliberately later than the cancellation case.
   // No server-triggered abort is allowed to stand in for browser cancellation.
   bodyTimer=setTimeout(()=>{if(!res.destroyed){row.bodyBytesWritten=Buffer.byteLength(body);res.end(body);event('server-delayed-body-written',{url,bytes:row.bodyBytesWritten});}},10000);
   res.on('finish',()=>{row.responseFinished=true;event('server-response-finished',{url});});res.on('close',()=>{row.responseClosed=true;row.bodyBytesWrittenAtClose=row.bodyBytesWritten;event('server-response-closed',{url,bodyBytesWritten:row.bodyBytesWritten,responseFinished:row.responseFinished});resolveServerClosed(row);});
  }else if(req.url==='/'){
   const html='<!doctype html><title>Pre-body transport proof</title><link rel="icon" href="data:,"><p>One controlled cancellation case</p>';res.writeHead(200,{'Content-Type':'text/html','Cache-Control':'no-store','Connection':'close'});row.headersSent=true;row.bodyBytesWritten=Buffer.byteLength(html);res.end(html);
  }else{res.writeHead(404);res.end();error(Error('Unexpected loopback path '+req.url));}
 });await new Promise((resolve,reject)=>{server.once('error',reject);server.listen(0,'127.0.0.1',resolve);});base='http://127.0.0.1:'+server.address().port;targetURL=base+'/v1/product/projects/p-proof/overview?case=prebody-abort';receipt.origin=base;receipt.targetURL=targetURL;
 const {chromium}=await import(pathToFileURL('/workspace/luci/desktop/node_modules/playwright/index.mjs').href);profile=mkdtempSync(join(here,'profile-'));
 context=await chromium.launchPersistentContext(profile,{headless:true,chromiumSandbox:true,serviceWorkers:'block',executablePath:'/usr/bin/chromium',timeout:20000,env:{PATH:process.env.PATH,TMPDIR:'/tmp',XDG_CONFIG_HOME:join(profile,'config'),XDG_CACHE_HOME:join(profile,'cache'),LUCI_C032_PREBODY_PROFILE:profile},args:['--no-proxy-server']});receipt.browserVersion=context.browser().version();
 const originalGuard=eval('('+handler+')');await context.route('**/*',async route=>{const req=route.request();receipt.guard.push({url:req.url(),method:req.method()});event('original-guard-invoked',{url:req.url(),method:req.method()});assert(receipt.guard.length<=4);await originalGuard(route);});
 page=await context.newPage();page.on('pageerror',error);page.setDefaultTimeout(3000);cdp=await context.newCDPSession(page);
 await send('Network.enable');cdp.on('Network.loadingFailed',value=>{receipt.networkFailures.push(value);event('network-loading-failed',value);if(paused?.networkId===value.requestId)resolveNetworkFailure(value);});
 cdp.on('Fetch.requestPaused',value=>{
  try{
   assert(!paused,'Exactly one response pause is expected');assert.equal(value.request.url,targetURL);assert.equal(value.request.method,'GET');assert.equal(value.responseStatusCode,200);assert(value.networkId);const header=value.responseHeaders.find(row=>row.name.toLowerCase()==='content-length');assert(header&&Number(header.value)<=65536);
   paused={id:value.requestId,networkId:value.networkId,url:value.request.url,status:value.responseStatusCode,headers:value.responseHeaders};receipt.pausedResponse=paused;event('response-headers-paused',paused);
   capturePromise=send('Fetch.getResponseBody',{requestId:paused.id}).then(value=>({status:'fulfilled',bytes:Buffer.byteLength(value.body,value.base64Encoded?'base64':'utf8'),sha256:hash(Buffer.from(value.body,value.base64Encoded?'base64':'utf8'))}),cause=>({status:'rejected',name:cause.name,message:cause.message})).then(value=>{captureResult=value;captureSettled=true;receipt.captureResult=value;event('body-capture-settled',value);return value;});
   resolveCaptureStarted();
  }catch(cause){error(cause);resolveCaptureStarted();}
 });
 await send('Fetch.enable',{patterns:[{urlPattern:targetURL,requestStage:'Response'}]});await page.goto(base+'/');
 browserRead=page.evaluate(async url=>{window.prebodyController=new AbortController();window.prebodyState={started:true,settled:false};try{const response=await fetch(url,{signal:window.prebodyController.signal,cache:'no-store'});const text=await response.text();return{status:response.status,bodyBytes:new TextEncoder().encode(text).byteLength};}catch(cause){return{errorName:cause.name,errorMessage:cause.message};}finally{window.prebodyState.settled=true;}},targetURL);
 await timed(captureStarted,3000,'Response capture start');assert(paused&&capturePromise);const inBrowser=await page.evaluate(()=>({started:window.prebodyState.started,settled:window.prebodyState.settled,aborted:window.prebodyController.signal.aborted}));
 assert.deepEqual(inBrowser,{started:true,settled:false,aborted:false});assert.equal(captureSettled,false);assert.equal(receipt.server.find(row=>row.url===targetURL).bodyBytesWritten,0);checkpoint('headers-paused-body-capture-pending-before-browser-abort',{browser:inBrowser,networkId:paused.networkId,bodyBytesWritten:0,captureSettled:false});
 await page.evaluate(()=>window.prebodyController.abort());event('browser-AbortSignal-triggered',{networkId:paused.networkId});
 const outcomes=await Promise.allSettled([timed(browserRead,1500,'Browser AbortError'),timed(capturePromise,2000,'getResponseBody cancellation join'),timed(networkFailure,2000,'Matching canceled network event')]);
 receipt.abortOutcomes=outcomes.map(row=>row.status==='fulfilled'?row:{status:row.status,message:row.reason.message});save();
 for(const outcome of outcomes)if(outcome.status==='rejected')throw outcome.reason;
 const [browserResult,bodyResult,networkResult]=outcomes.map(row=>row.value);receipt.browserResult=browserResult;
 assert.equal(browserResult.errorName,'AbortError');assert.equal(networkResult.requestId,paused.networkId);assert.equal(networkResult.canceled,true);assert.equal(bodyResult.status,'rejected');assert(captureSettled);pausedResolved=true;
 checkpoint('prebody-cancellation-correlated-and-capture-joined',{browserResult,bodyResult,networkResult,joinedBeforeAnyDisableOrRelease:true});
 const closed=await timed(serverTargetClosed,1500,'Original server response socket closes after cancellation');assert.equal(closed.bodyBytesWritten,0);assert.equal(closed.bodyBytesWrittenAtClose,0);assert.equal(closed.responseFinished,false);assert.equal(receipt.server.filter(row=>row.url===targetURL).length,1);assert.equal(receipt.guard.filter(row=>row.url===targetURL).length,1);assert.equal(receipt.browserMutations.length,0);
 assert(!receipt.protocolCommands.some(row=>['Fetch.continueResponse','Fetch.failRequest','Fetch.disable'].includes(row.method)),'No request modification or disable while joining capture');checkpoint('original-request-once-zero-body-bytes-and-server-closed',{serverCount:1,guardCount:1,serverResponseClosed:true,bodyBytesWritten:0,requestReplayed:false});assert.deepEqual(receipt.errors,[]);receipt.passed=true;
}
const hard=setTimeout(()=>{error(Error('88 second hard stop; owned browser terminated without claiming capture drain'));receipt.passed=false;receipt.hardStopCaptureInFlight=!!capturePromise&&!captureSettled;for(const pid of ownedPids())try{process.kill(pid,'SIGKILL');}catch{}save();process.exit(1);},88000);
try{await timed(run(),20000,'Single pre-body case');}catch(cause){error(cause);process.exitCode=1;}
finally{
 clearTimeout(bodyTimer);
 // Never modify/disable Fetch while getResponseBody is pending. A failed join
 // closes the owned browser, records failure, then joins its protocol promise.
 if(capturePromise&&!captureSettled){receipt.passed=false;receipt.captureJoinNeededBrowserClose=true;error(Error('Body capture did not join before cleanup; closing owned browser instead of modifying Fetch'));try{await closeOwnedBrowser('capture promise still pending');await timed(capturePromise,2000,'Capture promise join after browser close');}catch(cause){error(cause);}}
 receipt.captureJoinedAtCleanup=!capturePromise||captureSettled;
 if(cdp&&!browserClosed&&receipt.captureJoinedAtCleanup){
  if(paused&&!pausedResolved){receipt.passed=false;receipt.unresolvedPausedID=paused.id;error(Error('Paused ID not correlated as naturally cancelled; closing browser without claiming drain'));try{await closeOwnedBrowser('unresolved paused response');}catch(cause){error(cause);}}
  else try{await timed(send('Fetch.disable'),1000,'Disable after capture join');await cdp.detach();receipt.cleanup.push('CDP disabled and detached only after capture joined');}catch(cause){error(cause);}
 }
 if(context&&!browserClosed)try{await closeOwnedBrowser('normal cleanup');}catch(cause){error(cause);}
 if(server)try{server.closeAllConnections();await timed(new Promise((resolve,reject)=>server.close(cause=>cause?reject(cause):resolve())),3000,'Loopback server close');receipt.cleanup.push('Owned loopback server closed');}catch(cause){error(cause);}
 const remaining=ownedPids();if(remaining.length){receipt.passed=false;error(Error('Owned Chromium required forced cleanup'));for(const pid of remaining)try{process.kill(pid,'SIGKILL');}catch{}await new Promise(resolve=>setTimeout(resolve,150));}receipt.remainingOwnedBrowserPids=ownedPids();
 if(profile&&!receipt.remainingOwnedBrowserPids.length){assert.equal(dirname(profile),here);rmSync(profile,{recursive:true});receipt.cleanup.push('Owned profile removed');}
 receipt.pausedResolution=paused?pausedResolved?'natural-cancel-correlated':browserClosed?'browser-closed-after-failed-proof':'unresolved':'no-pause';receipt.activePausedIDs=paused&&!pausedResolved&&!browserClosed?[paused.id]:[];
 receipt.sourceUnchanged=hash(readFileSync(original))===sourceSHA;receipt.runnerUnchanged=hash(readFileSync(runner))===receipt.sourceBinding.runnerSHA256;receipt.previousEvidenceUnchanged=hash(readFileSync(oldManifest))===receipt.sourceBinding.previousManifestSHA256&&Object.entries(previousFiles).every(([path,digest])=>hash(readFileSync(join(previousRoot,path)))===digest);
 if(!receipt.sourceUnchanged||!receipt.runnerUnchanged||!receipt.previousEvidenceUnchanged)error(Error('Source or original evidence changed'));if(receipt.errors.length){receipt.passed=false;process.exitCode=1;}receipt.finishedAt=new Date().toISOString();save();clearTimeout(hard);console.log(JSON.stringify({passed:receipt.passed,affectedCases:1,checkpoints:receipt.checkpoints.length,report:join(out,'result.json'),captureJoinedAtCleanup:receipt.captureJoinedAtCleanup,pausedResolution:receipt.pausedResolution,activePausedIDs:receipt.activePausedIDs,errors:receipt.errors,cleanup:receipt.cleanup,remainingOwnedBrowserPids:receipt.remainingOwnedBrowserPids,previousEvidenceUnchanged:receipt.previousEvidenceUnchanged}));
}
