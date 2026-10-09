#!/usr/bin/env node
// Tiny CDP/Playwright coexistence proof. No installed application or benchmark.
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {createHash} from 'node:crypto';
import {existsSync,mkdirSync,mkdtempSync,readFileSync,readdirSync,rmSync,statSync,writeFileSync} from 'node:fs';
import {dirname,join,resolve} from 'node:path';
import {fileURLToPath,pathToFileURL} from 'node:url';
const here=dirname(fileURLToPath(import.meta.url)),out=resolve(process.argv[2]??'');
assert.equal(dirname(out),here);assert(!existsSync(out));mkdirSync(out,{mode:0o700});
const sha=value=>createHash('sha256').update(value).digest('hex');
const original='/workspace/.luci-tools/cloud032-idle-integrated-candidate-20261009/actual/sustained.mjs';
const source=readFileSync(original,'utf8'),sourceSHA=sha(source);assert.equal(sourceSHA,'18429d2a6497cfec582c8565b572b3ca567b756f2f9d403efd79b6b405719dc2');
const from=source.indexOf("await context.route('**/*',async route=>"),suffix='await route.continue();});',to=source.indexOf(suffix,from)+suffix.length;assert(from>=0&&to>from);
const originalRegistration=source.slice(from,to),handler=originalRegistration.slice("await context.route('**/*',".length,-2);
assert(handler.startsWith('async route=>{')&&handler.endsWith('}'));
writeFileSync(join(out,'frozen-route-guard.txt'),originalRegistration+'\n',{mode:0o600,flag:'wx'});writeFileSync(join(out,'proof.mjs'),readFileSync(fileURLToPath(import.meta.url)),{mode:0o600,flag:'wx'});
const receipt={passed:false,scope:'Loopback HTTP/Chromium transport proof only; main C032 diagnostic NOT RUN',sourceBinding:{path:original,sha256:sourceSHA,guardSHA256:sha(originalRegistration),runnerSHA256:sha(readFileSync(fileURLToPath(import.meta.url)))},caps:{browserRequests:16,responseBodyBytes:65536,responsePauseMs:1000,hardStopMs:88000},browserMutations:[],server:[],guard:[],pauses:[],networkFailures:[],checks:[],errors:[],cleanup:[]};
const persist=()=>writeFileSync(join(out,'result.json'),JSON.stringify(receipt,(key,value)=>key==='timer'?undefined:value,2)+'\n',{mode:0o600});
const event=(kind,details={})=>{writeFileSync(join(out,'raw.jsonl'),JSON.stringify({at:new Date().toISOString(),monotonicMs:performance.now(),kind,...details})+'\n',{mode:0o600,flag:'a'});};
const check=(name,details={})=>{receipt.checks.push({name,...details});event('checkpoint',{name,...details});persist();};
const timed=async(promise,ms,label)=>{let timer;try{return await Promise.race([promise,new Promise((_,reject)=>{timer=setTimeout(()=>reject(Error(label+' timeout')),ms);})]);}finally{clearTimeout(timer);}};
let server,context,page,cdp,base,profile,closing=false,fixtureError=null,workflowControl={failed:false},stage='transport-proof',arm='baseline';
const pending=new Map(),handlers=new Set(),held=new Map(),expected=new Map();let pauseCount=0,requestCount=0,closed=false;
function stopOnFailure(error){fixtureError??=error;event('original-guard-stop',{message:error.message});}
function recordError(error){receipt.errors.push(error.stack??String(error));event('error',{message:String(error)});}
function ownedPids(){if(!profile)return [];return readdirSync('/proc').filter(name=>/^\d+$/.test(name)).flatMap(name=>{try{const path='/proc/'+name;return statSync(path).uid===process.getuid()&&readFileSync(path+'/environ','utf8').split('\0').includes('LUCI_C032_TRANSPORT_PROFILE='+profile)?[Number(name)]:[];}catch{return [];}});}
function target(request,status){const url=new URL(request.url);return request.method==='GET'&&status===200&&url.origin===base&&(/^\/v1\/product\/projects\/p-proof\/overview$/.test(url.pathname)||url.pathname==='/v1/product/swarm-scheduler');}
async function release(row,reason){
 if(!pending.has(row.id))return;clearTimeout(row.timer);row.releaseReason=reason;
 try{await cdp.send('Fetch.continueResponse',{requestId:row.id});row.releaseResult='continued-unchanged';}
 catch(error){row.releaseResult='protocol-error';row.releaseError=error.message;if(!row.browserAborted)throw error;row.releaseResult='browser-cancelled-before-release';}
 finally{row.elapsedMs=performance.now()-row.began;pending.delete(row.id);held.delete(row.url);event('response-release',{id:row.id,url:row.url,reason,result:row.releaseResult,elapsedMs:row.elapsedMs});}
}
async function refuse(row,reason){clearTimeout(row.timer);try{await cdp.send('Fetch.failRequest',{requestId:row.id,errorReason:'Aborted'});}catch(error){event('refusal-protocol-result',{id:row.id,message:error.message});}finally{pending.delete(row.id);held.delete(row.url);row.releaseResult='failed-closed';row.releaseReason=reason;event('response-failed-closed',{id:row.id,reason});}}
async function responsePaused(value){
 const row={id:value.requestId,networkId:value.networkId,url:value.request.url,method:value.request.method,status:value.responseStatusCode,arm,began:performance.now(),target:target(value.request,value.responseStatusCode)};
 receipt.pauses.push(row);pending.set(row.id,row);event('response-paused',{id:row.id,networkId:row.networkId,url:row.url,method:row.method,status:row.status,arm,rowTarget:row.target});
 if(++pauseCount>16){await refuse(row,'pause-count-cap');throw Error('Pause count cap exceeded');}
 row.timer=setTimeout(()=>{recordError(Error('Response pause time cap exceeded'));void refuse(row,'one-second-pause-cap');},1000);
 if(!row.target){await release(row,'non-target-immediate');return;}
 const length=value.responseHeaders.find(header=>header.name.toLowerCase()==='content-length')?.value;
 if(!length||!/^\d+$/.test(length)||Number(length)>65536){await refuse(row,'declared-byte-cap');throw Error('Response declared byte cap exceeded');}
 const result=await cdp.send('Fetch.getResponseBody',{requestId:row.id});const bytes=Buffer.from(result.body,result.base64Encoded?'base64':'utf8');
 if(bytes.length>65536){await refuse(row,'actual-byte-cap');throw Error('Response actual byte cap exceeded');}
 row.bodyBytes=bytes.length;row.bodySHA256=sha(bytes);row.originalHeaders=value.responseHeaders;event('body-captured',{id:row.id,bytes:bytes.length,sha256:row.bodySHA256});
 if(arm==='baseline'){await release(row,'baseline-immediate');return;}
 held.set(row.url,row);row.deliveryHeld=true;event('delivery-held',{id:row.id,url:row.url});
}
async function waitHeld(url){return timed(new Promise((resolve,reject)=>{const poll=()=>{if(held.has(url))return resolve(held.get(url));if(closed||receipt.errors.length)return reject(Error('Response hold unavailable'));setTimeout(poll,5);};poll();}),1500,'Target response hold');}
const browserFetch=(url,method='GET',abortable=false)=>page.evaluate(async({url,method,abortable})=>{
 const controller=new AbortController();if(abortable)window.proofAbort=controller;
 try{const response=await fetch(url,{method,cache:'no-store',signal:controller.signal,...(method==='POST'?{body:'original-post'}:{})});return {status:response.status,headers:[...response.headers],body:await response.text()};}
 catch(error){return {errorName:error.name,errorMessage:error.message};}
},{url,method,abortable});
function assertDelivery(value,url){const row=expected.get(url);assert(row);assert.equal(value.status,row.status);assert.equal(value.body,row.body);assert.deepEqual(Object.fromEntries(value.headers),row.headers);assert.equal(receipt.server.filter(item=>item.url===url).length,1,'Original request reaches server once');assert.equal(receipt.guard.filter(item=>item.url===url).length,1,'Original guard invoked once');}
async function run(){
 server=createServer((req,res)=>{
  const url=new URL(req.url,base).href;const status=new URL(url).searchParams.get('status')==='403'?403:200;
  const body=req.url==='/'?'<!doctype html><title>Transport proof</title><link rel="icon" href="data:,"><p>Controlled transport only</p>':JSON.stringify({url:new URL(url).pathname,query:new URL(url).search,method:req.method,original:'Exact original bytes: snowman ☃ and newline\n'});
  const headers={'cache-control':'no-store','connection':'close','content-length':String(Buffer.byteLength(body)),'content-type':req.url==='/'?'text/html; charset=utf-8':'application/json; charset=utf-8','date':'Wed, 01 Jan 2025 00:00:00 GMT','x-proof-identity':'exact-original-'+receipt.server.length};
  expected.set(url,{status,headers,body});receipt.server.push({url,method:req.method,status,bodyBytes:Buffer.byteLength(body),bodySHA256:sha(body)});event('server-original',{url,method:req.method,status});res.writeHead(status,headers);res.end(body);
 });await new Promise((resolve,reject)=>{server.once('error',reject);server.listen(0,'127.0.0.1',resolve);});base='http://127.0.0.1:'+server.address().port;receipt.origin=base;
 const {chromium}=await import(pathToFileURL('/workspace/luci/desktop/node_modules/playwright/index.mjs').href);profile=mkdtempSync(join(here,'profile-'));
 context=await chromium.launchPersistentContext(profile,{headless:true,chromiumSandbox:true,executablePath:'/usr/bin/chromium',serviceWorkers:'block',timeout:20000,env:{PATH:process.env.PATH,TMPDIR:'/tmp',XDG_CONFIG_HOME:join(profile,'config'),XDG_CACHE_HOME:join(profile,'cache'),LUCI_C032_TRANSPORT_PROFILE:profile},args:['--no-proxy-server']});receipt.browserVersion=context.browser().version();
 // Frozen handler is byte-identical. The outer wrapper only counts invocation;
 // every request is then handled by that unchanged original route guard.
 const originalGuard=eval('('+handler+')');
 await context.route('**/*',async route=>{const request=route.request();receipt.guard.push({url:request.url(),method:request.method(),closing,fixtureError:!!fixtureError,workflowFailed:workflowControl.failed});event('original-guard-invoked',{url:request.url(),method:request.method()});if(++requestCount>16){await route.abort('blockedbyclient');throw Error('Request count cap exceeded');}await originalGuard(route);});
 page=await context.newPage();page.setDefaultTimeout(3000);page.on('pageerror',recordError);
 cdp=await context.newCDPSession(page);await cdp.send('Network.enable');cdp.on('Network.loadingFailed',value=>{receipt.networkFailures.push(value);event('network-loading-failed',value);});
 cdp.on('Fetch.requestPaused',value=>{const work=responsePaused(value).catch(recordError);handlers.add(work);void work.finally(()=>handlers.delete(work));});
 await cdp.send('Fetch.enable',{patterns:[{urlPattern:base+'/v1/product/*',requestStage:'Response'}]});check('original-route-guard-and-separate-response-session-enabled');
 await page.goto(base+'/');check('document-delivered-through-original-guard',{guardRequests:receipt.guard.length});
 const overview=base+'/v1/product/projects/p-proof/overview',scheduler=base+'/v1/product/swarm-scheduler';
 const baseline=overview+'?case=baseline';const first=await browserFetch(baseline);assertDelivery(first,baseline);assert.equal(receipt.pauses.find(row=>row.url===baseline).releaseReason,'baseline-immediate');check('baseline-exact-body-status-headers-original-request-once');
 arm='defer';
 for(const [label,url]of [['overview',overview+'?case=defer'],['scheduler',scheduler+'?case=defer']]){
  let settled=false;const delivered=browserFetch(url).then(value=>{settled=true;return value;});const row=await waitHeld(url);assert.equal(settled,false);check(label+'-checkpoint-body-captured-browser-unsettled',{bodyBytes:row.bodyBytes,bodySHA256:row.bodySHA256,guardCount:receipt.guard.filter(item=>item.url===url).length,serverCount:receipt.server.filter(item=>item.url===url).length,pendingIDs:pending.size});
  await release(row,'explicit-checkpoint-release');const value=await delivered;assertDelivery(value,url);assert(row.elapsedMs<=1000);check(label+'-exact-unchanged-delivery-after-release',{elapsedMs:row.elapsedMs});
 }
 for(const [label,url,method]of [['forbidden',overview+'?status=403','GET'],['post',overview+'?case=post','POST'],['wrong-path',overview+'-extra?case=wrong-path','GET']]){
  const value=await browserFetch(url,method);assertDelivery(value,url);const row=receipt.pauses.find(row=>row.url===url);assert(row&&!row.target&&!row.deliveryHeld);assert.equal(row.releaseReason,'non-target-immediate');assert.equal(row.bodyBytes,undefined);check(label+'-never-held',{status:value.status,method});
 }
 assert.equal(receipt.browserMutations.length,1);assert.equal(receipt.browserMutations[0].method,'POST');assert.equal(receipt.browserMutations[0].path,'/v1/product/projects/p-proof/overview');check('original-guard-retains-mutation-log');
 const cancelURL=overview+'?case=natural-abort';const cancelled=browserFetch(cancelURL,'GET',true);const heldRow=await waitHeld(cancelURL);heldRow.browserAborted=true;await page.evaluate(()=>window.proofAbort.abort());const cancelValue=await timed(cancelled,500,'Natural AbortSignal rejection');assert.equal(cancelValue.errorName,'AbortError');check('natural-abort-rejects-before-held-response-release',{networkId:heldRow.networkId,pendingIDs:pending.size});await release(heldRow,'natural-abort-cleanup');assert.equal(receipt.server.filter(row=>row.url===cancelURL).length,1);assert.equal(receipt.guard.filter(row=>row.url===cancelURL).length,1);check('natural-abort-no-replay',{releaseResult:heldRow.releaseResult});
 const foreign='http://localhost:'+server.address().port+'/cross-origin';const foreignValue=await browserFetch(foreign);assert(foreignValue.errorName);assert(fixtureError);assert.equal(receipt.guard.filter(row=>row.url===foreign).length,1);assert.equal(receipt.server.filter(row=>row.url===foreign).length,0);check('original-guard-blocks-cross-origin');
 for(const flag of ['closing','fixtureError','workflowFailed']){
  closing=flag==='closing';fixtureError=flag==='fixtureError'?Error('Controlled background failure'):null;workflowControl.failed=flag==='workflowFailed';const url=overview+'?guard='+flag,value=await browserFetch(url);assert(value.errorName);assert.equal(receipt.guard.filter(row=>row.url===url).length,1);assert.equal(receipt.server.filter(row=>row.url===url).length,0);check('original-guard-blocks-'+flag);
 }
 closing=false;fixtureError=null;workflowControl.failed=false;
 await Promise.all([...handlers]);assert.equal(pending.size,0);assert(requestCount<=16);assert(receipt.pauses.every(row=>row.elapsedMs<=1000));assert(receipt.pauses.filter(row=>row.target).every(row=>row.bodyBytes<=65536));assert.deepEqual(receipt.errors,[]);check('caps-and-final-paused-ID-check',{browserRequests:requestCount,responsePauses:pauseCount,pausedIDs:[],maximumBodyBytes:Math.max(...receipt.pauses.map(row=>row.bodyBytes??0)),maximumPauseMs:Math.max(...receipt.pauses.map(row=>row.elapsedMs??0))});receipt.passed=true;
}
const hard=setTimeout(()=>{recordError(Error('88 second hard stop'));receipt.passed=false;for(const pid of ownedPids())try{process.kill(pid,'SIGKILL');}catch{}persist();process.exit(1);},88000);
try{await timed(run(),65000,'Transport proof');}catch(error){recordError(error);process.exitCode=1;}
finally{
 closed=true;
 for(const row of [...pending.values()])try{await timed(release(row,'finally-release'),500,'Final response release');}catch(error){recordError(error);await timed(refuse(row,'finally-refuse'),500,'Final response refusal').catch(recordError);}
 receipt.pausedIDsAtCleanup=[...pending.keys()];if(receipt.pausedIDsAtCleanup.length)recordError(Error('Paused response IDs remain'));
 if(cdp)try{await timed(cdp.send('Fetch.disable'),1000,'Disable proof response interception');await cdp.detach();receipt.cleanup.push('Separate CDP response session disabled and detached');}catch(error){recordError(error);}
 if(context)try{await timed(context.close(),8000,'Owned Chromium close');receipt.cleanup.push('Owned Chromium closed');}catch(error){recordError(error);}
 if(server)try{server.closeAllConnections();await timed(new Promise((resolve,reject)=>server.close(error=>error?reject(error):resolve())),3000,'Owned loopback server close');receipt.cleanup.push('Owned loopback server closed');}catch(error){recordError(error);}
 const remaining=ownedPids();if(remaining.length){recordError(Error('Owned browser required forced cleanup'));for(const pid of remaining)try{process.kill(pid,'SIGKILL');}catch{}await new Promise(resolve=>setTimeout(resolve,150));}receipt.remainingOwnedBrowserPids=ownedPids();
 if(profile&&!receipt.remainingOwnedBrowserPids.length){assert.equal(dirname(profile),here);rmSync(profile,{recursive:true});receipt.cleanup.push('Owned profile removed');}
 receipt.sourceUnchanged=sha(readFileSync(original))===sourceSHA;receipt.runnerUnchanged=sha(readFileSync(fileURLToPath(import.meta.url)))===receipt.sourceBinding.runnerSHA256;
 if(!receipt.sourceUnchanged||!receipt.runnerUnchanged)recordError(Error('Frozen guard source or proof runner changed'));if(receipt.errors.length){receipt.passed=false;process.exitCode=1;}receipt.finishedAt=new Date().toISOString();
 // Timer objects are implementation handles; retain only scalar pause evidence.
 for(const row of receipt.pauses)delete row.timer;
 persist();clearTimeout(hard);console.log(JSON.stringify({passed:receipt.passed,checks:receipt.checks.length,report:join(out,'result.json'),errors:receipt.errors,cleanup:receipt.cleanup,remainingOwnedBrowserPids:receipt.remainingOwnedBrowserPids,pausedIDsAtCleanup:receipt.pausedIDsAtCleanup}));
}
