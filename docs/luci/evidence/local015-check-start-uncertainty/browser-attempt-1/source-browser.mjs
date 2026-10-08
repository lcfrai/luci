// Focused LOCAL-015 source witness. Controlled providers and callbacks; no backend.
import assert from 'node:assert/strict';
import {readFileSync,writeFileSync,mkdirSync,mkdtempSync,rmSync,existsSync} from 'node:fs';
import {createHash} from 'node:crypto';
import {join,dirname,resolve} from 'node:path';
import {fileURLToPath,pathToFileURL} from 'node:url';
const here=dirname(fileURLToPath(import.meta.url));
const repo=resolve(process.env.LUCI_CHECK_UI_REPO);
assert.equal(process.env.LUCI_LOCAL015_BROWSER_RELEASE,'1','Use only the root-coordinated browser slot.');
assert.equal(process.platform,'linux','Linux controlled source witness only.');
assert.notEqual(process.getuid(),0,'Use Chromium sandbox as an ordinary user.');
assert(process.argv[2],'Pass a fresh evidence directory.');
const out=resolve(process.argv[2]);assert(!existsSync(out),'Preserve prior evidence.');mkdirSync(out,{recursive:true,mode:0o700});
const sha=value=>createHash('sha256').update(value).digest('hex');
const inputs=JSON.parse(readFileSync(process.env.LUCI_LOCAL015_INPUTS,'utf8'));
const fingerprint=()=>Object.fromEntries(Object.keys(inputs).map(path=>[path,sha(readFileSync(join(repo,path)))]));
assert.deepEqual(fingerprint(),inputs,'Source changed before browser launch.');
const fixture=readFileSync(join(here,'fixture.tsx'),'utf8');
const virtual=join(repo,'scripts/acceptance/local015-check-ui/fixture.tsx');
const {createServer}=await import(pathToFileURL(join(repo,'desktop/node_modules/vite/dist/node/index.js')).href);
const {chromium}=await import(pathToFileURL(join(repo,'desktop/node_modules/playwright/index.mjs')).href);
const receipt={passed:false,startedAt:new Date().toISOString(),scope:'Actual ProjectChecks, useProjectAccess, draft hooks and shadcn controls with controlled API/account/team providers at 1440 and 390 pixels.',limitations:['No installed/native Windows, real HTTP/backend, Docker, worker or model evidence.','Permission changes and API responses are callback fixtures.','No whole-app or physical-input performance claim; accessibility/overflow observations are scoped.'],sourceBefore:inputs,helperSHA256:sha(readFileSync(fileURLToPath(import.meta.url))),fixtureSHA256:sha(fixture),checks:[],errors:[],cleanup:[]};
const save=()=>writeFileSync(join(out,'result.json'),JSON.stringify(receipt,null,2)+'\n');
const record=value=>{receipt.checks.push(value);save();console.log(value.label);};
const bounded=async(label,action,timeout=7000)=>{let timer;try{return await Promise.race([action(),new Promise((_,reject)=>{timer=setTimeout(()=>reject(Error(label+' timed out')),timeout);})]);}finally{clearTimeout(timer);}};
let browser,server,page,profile;
const snapshot=()=>page.evaluate(()=>window.__local015.snapshot());
const consent=()=>page.getByRole('checkbox',{name:/I trust this project's code/});
const run=()=>page.getByRole('button',{name:'Run project checks',exact:true});
const refresh=()=>page.getByRole('button',{name:'Refresh check history',exact:true});
try{
 server=await createServer({configFile:join(repo,'desktop/vite.config.ts'),root:join(repo,'desktop'),server:{host:'127.0.0.1',port:0,strictPort:false,watch:null,fs:{allow:[repo]}},plugins:[{name:'controlled-local015-fixture',enforce:'pre',resolveId(id){
  if(id===virtual||id==='/@fs'+virtual)return virtual;
  if(id.endsWith('/AuthProvider'))return '\0local015-auth';
  if(id.endsWith('/SetupProvider'))return '\0local015-setup';
  if(id.endsWith('/TeamWorkspaceProvider'))return '\0local015-team';
 },load(id){
  if(id===virtual)return fixture;
  if(id==='\0local015-auth')return 'export const useAuth=()=>({profile:{user:{id:"controlled-local015-owner"}}});';
  if(id==='\0local015-setup')return 'export const useSetup=()=>({workspace:{mode:"local"},status:{config:{projectsDir:"/controlled-local015"},access:{user:{username:"controlled-local015-owner"}}}});';
  if(id==='\0local015-team')return 'import {useSyncExternalStore} from "react"; export const useTeamProjectPermissions=()=>({loading:false,error:"",permissions:useSyncExternalStore(window.__local015.subscribe,window.__local015.permissionSnapshot)});';
 },configureServer(vite){vite.middlewares.use('/local015',async(_req,res)=>{res.setHeader('Content-Type','text/html');res.end(await vite.transformIndexHtml('/local015','<!doctype html><html lang="en"><head><meta name="viewport" content="width=device-width,initial-scale=1"><title>LOCAL-015 controlled source witness</title></head><body><div id="root"></div><script type="module" src="/@fs'+virtual+'"></script></body></html>'));});}}]});
 await server.listen();const base='http://127.0.0.1:'+server.httpServer.address().port;
 profile=mkdtempSync('/tmp/l015-');
 browser=await chromium.launch({headless:true,chromiumSandbox:true,executablePath:process.env.LUCI_CHROMIUM_EXECUTABLE||'/usr/bin/chromium',env:{PATH:process.env.PATH,TMPDIR:profile,XDG_CONFIG_HOME:join(profile,'config'),XDG_CACHE_HOME:join(profile,'cache')},args:['--no-proxy-server']});receipt.browserVersion=browser.version();
 const context=await browser.newContext({serviceWorkers:'block'});await context.route('**/*',route=>{if(new URL(route.request().url()).origin!==base){receipt.errors.push('External request blocked');return route.abort();}return route.continue();});
 page=await context.newPage();page.setDefaultTimeout(12000);page.on('pageerror',error=>receipt.errors.push(error.message));await page.addInitScript(()=>localStorage.clear());
 const fresh=async width=>{await page.setViewportSize({width,height:width===390?844:1080});await page.goto(base+'/local015');await page.getByRole('button',{name:'Remove Controlled acceptance command'}).waitFor();await page.getByRole('button',{name:'Refresh requirement evidence',exact:true}).waitFor();await page.waitForFunction(()=>window.__local015?.snapshot().histories===1);};
 for(const width of [1440,390]){
  for(const outcome of ['rejection','success']){
   await fresh(width);await consent().check();await run().click();assert.equal((await snapshot()).starts,1);
   await page.evaluate(()=>window.__local015.setEdit(false));await page.waitForFunction(()=>window.__local015.snapshot().histories===2);
   await page.getByText(/Read-only access/).waitFor();
   await page.evaluate(()=>window.__local015.setEdit(true));await page.waitForFunction(()=>window.__local015.snapshot().histories===3);
   await page.getByText('No verification runs recorded yet.',{exact:true}).waitFor();
   await page.evaluate(value=>window.__local015.settle(value),outcome);
   await refresh().waitFor();assert(await run().isDisabled());assert(!await consent().isChecked());assert(await consent().isDisabled());
   assert.equal(await page.getByText(/Private old-authority/).count(),0);
   const before=await snapshot();assert.equal(before.starts,1);assert.equal(before.histories,3);
   await page.addScriptTag({path:join(repo,'desktop/node_modules/axe-core/axe.min.js')});
   const violations=await page.evaluate(async()=> (await window.axe.run(document,{runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21aa']}})).violations.map(v=>({id:v.id,nodes:v.nodes.map(n=>n.target)})));
   assert.deepEqual(violations,[]);const geometry=await page.evaluate(()=>({width:document.documentElement.clientWidth,scrollWidth:document.documentElement.scrollWidth}));assert(geometry.scrollWidth<=geometry.width+1);
   await refresh().scrollIntoViewIfNeeded();await page.screenshot({path:join(out,`${outcome}-blocked-${width}.png`)});
   await refresh().focus();await page.keyboard.press('Enter');await refresh().waitFor({state:'detached'});
   assert(await run().isDisabled());assert(!await consent().isChecked());assert.equal((await snapshot()).histories,4);
   await consent().focus();await page.keyboard.press('Space');assert(!await run().isDisabled());
   const after=await snapshot();assert.equal(after.starts,1);
   const settle=after.events.find(row=>row.kind==='settled'),lastHistory=after.events.filter(row=>row.kind==='history').at(-1);assert(lastHistory.number>settle.number);
   record({label:`edit-churn-${outcome}-${width}`,before,after,violations,geometry,freshExplicitReadAfterSettlement:true,freshKeyboardConsent:true,privateReceiptSuppressed:true});
  }
  await fresh(width);await consent().check();await run().click();await page.evaluate(()=>window.__local015.switchProject('beta'));
  await page.getByText('Selected project: beta',{exact:true}).waitFor();await page.getByRole('button',{name:'Remove Controlled acceptance command'}).waitFor();
  await page.evaluate(()=>window.__local015.settle('rejection'));assert.equal(await refresh().count(),0);assert.equal(await page.getByText(/Private old-authority/).count(),0);
  await consent().check();assert(!await run().isDisabled());assert.equal((await snapshot()).starts,1);
  record({label:`project-switch-privacy-${width}`,...await snapshot()});
 }
 assert.deepEqual(receipt.errors,[]);receipt.passed=true;
}catch(error){receipt.error=String(error.stack||error);if(page){receipt.failureSnapshot=await bounded('failure snapshot',snapshot).catch(()=>null);await bounded('failure screenshot',()=>page.screenshot({path:join(out,'failure.png')})).catch(()=>{});}console.error(receipt.error);}
finally{
 receipt.finalizationStartedAt=new Date().toISOString();save();
 for(const [label,action] of [['owned Chromium',()=>browser?.close()],['owned Vite',()=>server?.close()]]){try{await bounded(label,action);receipt.cleanup.push({label,closed:true});}catch(error){receipt.errors.push(String(error));receipt.cleanup.push({label,closed:false});}save();}
 if(profile&&!receipt.cleanup.some(row=>!row.closed)){rmSync(profile,{recursive:true,force:true});receipt.cleanup.push({label:'private profile',closed:true});}
 receipt.sourceAfter=fingerprint();receipt.sourceUnchanged=JSON.stringify(receipt.sourceBefore)===JSON.stringify(receipt.sourceAfter);receipt.helperUnchanged=receipt.helperSHA256===sha(readFileSync(fileURLToPath(import.meta.url)))&&receipt.fixtureSHA256===sha(readFileSync(join(here,'fixture.tsx')));
 receipt.passed=receipt.passed&&receipt.errors.length===0&&receipt.sourceUnchanged&&receipt.helperUnchanged;receipt.finishedAt=new Date().toISOString();save();console.log(JSON.stringify({passed:receipt.passed,checks:receipt.checks.length,error:receipt.error,cleanup:receipt.cleanup}));if(!receipt.passed)process.exitCode=1;
}
