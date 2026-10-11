import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFileSync} from 'node:fs';
import {createHash} from 'node:crypto';
import {createRequire} from 'node:module';
const require=createRequire('/workspace/luci/desktop/package.json');const ts=require('typescript');
const root='/workspace/.luci-tools/cloud032-home-phase-577-full100';
const input='/workspace/.luci-tools/cloud032-home-phase-9536e417/overlay/desktop/src/diagnostics/homePhaseTiming.ts';
const source=readFileSync(input,'utf8'),hash=value=>createHash('sha256').update(value).digest('hex');
const binding=JSON.parse(readFileSync(root+'/overlay-binding.json','utf8'));assert.equal(hash(source),binding.files.find(row=>row.path.endsWith('/homePhaseTiming.ts')).overlaySHA256);
const result=ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2020},reportDiagnostics:true});assert.equal(result.diagnostics.length,0);
const driver=readFileSync(root+'/actual/sustained.mjs','utf8');const begin=driver.indexOf('function validateHomeCapture(value){'),end=driver.indexOf('\nconst check =',begin);assert(begin>=0&&end>begin);const validator=vm.runInNewContext('('+driver.slice(begin,end)+')',{assert});
function fresh(){let ticks=0;const ctx={exports:{},window:{__luciHomePhaseRequested:1},performance:{timeOrigin:123456,now:()=>++ticks/10}};vm.runInNewContext(result.outputText,ctx);ctx.window.__luciHomePhase.arm();return ctx;}
function home(ctx,commit=true){const e=ctx.exports,id=e.homeHandlerBegin();e.homePhase(3,()=>e.homePhase(4,()=>{e.homePhase(5,()=>{});e.homePhase(6,()=>{e.homeCount(10,12);e.homeCount(11,1)});e.homePhase(7,()=>{})}));e.homePhase(9,()=>{});e.homePhase(8,()=>{});e.homeHandlerEnd(id);if(commit)e.homeCommit(e.homeRenderBegin());return ctx.window.__luciHomePhase.snapshot();}
let c=fresh(),windows=[];for(let n=1;n<=100;n++){const snap=home(c);assert.equal(snap.homes,n);assert.equal(snap.dropped,Math.max(0,n-16));assert.equal(snap.pending,0);assert.equal(snap.active,0);assert.equal(snap.rows.length,12);if(n===16)assert.equal(snap.dropped,0);if(n===17)assert.equal(snap.dropped,1);windows.push({homePhase:snap});}
const receipt={browserEventWindows:windows};validator(receipt);assert.equal(receipt.homePhaseCapture.rows,1200);assert.equal(receipt.homePhaseCapture.completedStartEntriesRetired,84);
console.log('PASS exact original probe:17th completed Home retires one start entry with12 phase rows retained; full100 preserves1200 contiguous phase rows and84 retirements');
const missing=structuredClone(receipt);missing.browserEventWindows[49].homePhase.rows.pop();assert.throws(()=>validator(missing));console.log('PASS missing phase/sequence still refuses complete capture');
const extra=structuredClone(receipt);extra.browserEventWindows.at(-1).homePhase.dropped++;assert.throws(()=>validator(extra));console.log('PASS extra dropped counter still refuses complete capture');
c=fresh();c.exports.homeHandlerBegin();c.exports.homeHandlerBegin();const overlap=c.window.__luciHomePhase.snapshot();assert.equal(overlap.homes,2);assert.equal(overlap.dropped,1);assert.notEqual(overlap.dropped,Math.max(0,overlap.homes-16));console.log('PASS actual overlapping unfinished Home exceeds permitted retirement count');
console.log(JSON.stringify({tests:4,passed:4,probeSHA256:hash(source),driverSHA256:hash(driver),transpileOnly:true,browser:false,webBuild:false}));
