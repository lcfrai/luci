import assert from 'node:assert/strict';
import test from 'node:test';
import {readFileSync} from 'node:fs';
import {runInNewContext} from 'node:vm';
import {resolve} from 'node:path';
const root=resolve(process.env.C071_SOURCE_ROOT??new URL('.',import.meta.url).pathname),variant=process.env.C071_VARIANT??'warm',source=readFileSync(root+'/'+variant+'/launch.mjs','utf8');
const start=source.indexOf("page.on('response',"),end=source.indexOf(';receipt.browserVersion=browser.version()',start),drainStart=source.indexOf('  acceptRestartResponses=false;'),drainEnd=source.indexOf('  closing=true;',drainStart);
assert(start>=0&&end>start&&drainStart>=0&&drainEnd>drainStart);
test('actual drain seals callback acquisition before awaiting an already accepted observation',async()=>{
 const request={},browserPending=new Map([[request,'browser-exact']]),pendingMutations=new Map([['browser-exact',{method:'POST',path:'/v1/sessions/s-1234abcd/restart'}]]),browserResponseRequests=new WeakSet();
 let release,callback,reads=0;const existing=new Promise(r=>{release=r;}),browserResponseObservations=new Set([existing]),failures=[];
 const sandbox={assert,browserPending,pendingMutations,browserResponseRequests,browserResponseObservations,acceptRestartResponses:true,closing:false,page:{on:(event,fn)=>{assert.equal(event,'response');callback=fn;}},boundedAttempt:async(fn,ms)=>{assert.equal(ms,5500);return fn();},receipt:{cleanup:[]},safeError:()=>({code:'controlled'}),fail:e=>failures.push(e)};
 runInNewContext(source.slice(start,end),sandbox);
 const draining=runInNewContext('(async()=>{'+source.slice(drainStart,drainEnd)+'closing=true;})()',sandbox);
 assert.equal(sandbox.acceptRestartResponses,false);assert.equal(sandbox.closing,false);
 callback({request:()=>request,body:async()=>{reads++;return Buffer.from('{}');}});
 assert.equal(reads,0);assert.equal(failures.length,0);assert.equal(browserResponseRequests.has(request),false);assert.equal(browserResponseObservations.size,1);assert.equal(pendingMutations.size,1);
 release();await draining;assert.equal(sandbox.closing,true);assert.equal(sandbox.receipt.cleanup.length,0);assert.equal(pendingMutations.size,1);
});
