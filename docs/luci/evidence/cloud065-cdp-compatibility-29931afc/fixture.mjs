import assert from 'node:assert/strict';
import fs from 'node:fs';
import http from 'node:http';
import {createHash} from 'node:crypto';
import {chromium} from '/workspace/luci/desktop/node_modules/playwright/index.mjs';
import {holdNextRestartResponse} from '/workspace/.luci-tools/cloud065-installed-cold-restart-29931afc-v1/scenario.mjs';
const dir=new URL('.',import.meta.url).pathname,marker='C065_CDP_'+Date.now(),started=Date.now(),events=[],known=new Map();
const report={started:new Date().toISOString(),scope:'Disposable loopback HTTP plus actual Chromium CDP compatibility only; no Luci, Docker, provider or product acceptance.',browser:'/usr/bin/chromium',scenarioSHA256:createHash('sha256').update(fs.readFileSync('/workspace/.luci-tools/cloud065-installed-cold-restart-29931afc-v1/scenario.mjs')).digest('hex'),checks:[],events,cleanup:{},passed:false};
let browser,server,context,gate,posts=0,forwarded=0,failed=false;
function bounded(p,ms,label){let t;return Promise.race([p,new Promise((_,reject)=>{t=setTimeout(()=>reject(Error(label)),ms);})]).finally(()=>clearTimeout(t));}
function identity(pid){try{const s=fs.readFileSync(`/proc/${pid}/stat`,'utf8'),fields=s.slice(s.lastIndexOf(')')+2).split(' ');return {pid:Number(pid),startTicks:fields[19],state:fields[0]};}catch{return null;}}
function capture(){for(const name of fs.readdirSync('/proc'))if(/^\d+$/.test(name)){try{if(fs.readFileSync(`/proc/${name}/environ`).includes(Buffer.from('LUCI_C065_CDP_FIXTURE='+marker+'\0'))){const row=identity(name);if(row)known.set(name,row);}}catch{}}}
const session={id:'s-1234abcd',projectId:'p-5678abcd',harness:'shell',status:'running',executionGeneration:'b'.repeat(64)},body=JSON.stringify(session),digest=createHash('sha256').update(body).digest('hex');
function check(name){report.checks.push({name,passed:true,at:new Date().toISOString()});}
const health=()=>assert(Date.now()-started<35000,'Fixture deadline');
async function main(){
 server=http.createServer((req,res)=>{if(req.method==='POST'&&req.url==='/v1/sessions/s-1234abcd/restart'){posts++;res.writeHead(200,{'content-type':'application/json'});res.end(body);}else if(req.url==='/v1/sessions'){res.writeHead(200,{'content-type':'application/json'});res.end(JSON.stringify({sessions:[session]}));}else if(req.url==='/'){res.writeHead(200,{'content-type':'text/html'});res.end('<!doctype html><title>C065 compatibility</title>');}else{res.writeHead(404);res.end();}});
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));const base='http://127.0.0.1:'+server.address().port;
 browser=await chromium.launch({executablePath:'/usr/bin/chromium',headless:true,args:['--no-proxy-server'],env:{...process.env,LUCI_C065_CDP_FIXTURE:marker}});capture();
 context=await browser.newContext();await context.route('**/*',async route=>{const r=route.request();assert.equal(new URL(r.url()).origin,base);if(r.method()==='POST'){assert.equal(new URL(r.url()).pathname,'/v1/sessions/s-1234abcd/restart');forwarded++;}await route.continue();});
 const page=await context.newPage();await page.goto(base);
 for(const mode of ['release','dispose-held']){
  gate=await holdNextRestartResponse(page,{base,workspace:'',sessionId:session.id,projectId:session.projectId,health,record:row=>events.push({mode,...row})});
  await page.evaluate(()=>{window.result=null;window.fetchError=null;void fetch('/v1/sessions/s-1234abcd/restart',{method:'POST'}).then(r=>r.json()).then(r=>{window.result=r;},e=>{window.fetchError=String(e);});});
  const observed=await bounded(gate.ready,6000,'Authentic response gate did not become ready');assert.equal(observed.sha256,digest);assert.equal(await page.evaluate(()=>window.result),null);assert.equal(posts,mode==='release'?1:2);assert.equal(forwarded,posts);check(mode+': authentic body held after one guarded forward');
  const live=await bounded(page.evaluate(()=>fetch('/v1/sessions').then(r=>r.json())),1500,'Other GET blocked by response gate');assert.equal(live.sessions[0].executionGeneration,session.executionGeneration);check(mode+': independent GET proceeds while POST response held');
  if(mode==='release')await gate.release();await gate.dispose();
  await page.waitForFunction(()=>window.result!==null,{},{timeout:2000});assert.deepEqual(await page.evaluate(()=>window.result),session);assert.equal(await page.evaluate(()=>window.fetchError),null);assert.equal(posts,mode==='release'?1:2);assert.equal(forwarded,posts);assert.equal(gate.snapshot().settlement.attempted,'Fetch.continueResponse');assert.equal(gate.snapshot().settlement.acknowledged,true);assert.equal(gate.snapshot().disposed,true);check(mode+': exact response delivered once and gate detached');gate=null;
 }
 capture();report.passed=true;
}
try{await bounded(main(),30000,'Overall fixture work deadline');}catch(error){failed=true;report.error={name:error.name,message:error.message,stack:error.stack};}finally{
 if(gate)try{await bounded(gate.dispose(),5000,'Final gate cleanup deadline');report.cleanup.gateDisposed=true;}catch(error){failed=true;report.cleanup.gateError=error.message;}
 capture();if(browser)try{await bounded(browser.close(),5000,'Browser close deadline');report.cleanup.browserCloseAcknowledged=true;}catch(error){failed=true;report.cleanup.browserError=error.message;}
 if(server)try{server.closeAllConnections();await bounded(new Promise((resolve,reject)=>server.close(error=>error?reject(error):resolve())),2000,'HTTP close deadline');report.cleanup.httpClosed=!server.listening;}catch(error){failed=true;report.cleanup.httpError=error.message;}
 capture();const remaining=[];for(const [pid,old]of known){const now=identity(pid);if(now&&now.startTicks===old.startTicks&&now.state!=='Z'){remaining.push(now);try{process.kill(Number(pid),'SIGTERM');}catch{}}}report.cleanup.forcedExactTerm=remaining;
 await new Promise(resolve=>setTimeout(resolve,100));report.cleanup.knownProcesses=[...known].map(([pid,old])=>{const now=identity(pid);return {...old,final:!now?'absent':now.startTicks!==old.startTicks?'reused':now.state==='Z'?'zombie':'live',finalState:now?.state??null};});
 report.cleanup.live=report.cleanup.knownProcesses.filter(r=>r.final==='live').length;report.cleanup.zombies=report.cleanup.knownProcesses.filter(r=>r.final==='zombie').length;report.posts=posts;report.guardedForwards=forwarded;report.finished=new Date().toISOString();report.elapsedMs=Date.now()-started;report.passed=report.passed&&!failed&&report.cleanup.live===0&&remaining.length===0;fs.writeFileSync(dir+'result.json',JSON.stringify(report,null,2)+'\n');console.log(JSON.stringify({passed:report.passed,checks:report.checks.length,posts,forwarded,elapsedMs:report.elapsedMs,cleanup:report.cleanup,error:report.error},null,2));process.exit(report.passed?0:1);
}
