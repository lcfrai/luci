// Controlled promises/local files only: no subprocesses, Docker, or browser.
import test from 'node:test';
import assert from 'node:assert/strict';
import {mkdtempSync,readFileSync,rmSync,statSync} from 'node:fs';
import {join} from 'node:path';
import {tmpdir} from 'node:os';
import {createWorkflowDeadline,runBoundedCleanup,writeFailureCheckpoint,settleOwnedBrowser} from './acceptance-lifecycle.mjs';
import {createAcceptanceJournal,diagnosticError} from './durable-acceptance-journal.mjs';
const never=()=>new Promise(()=>{});
function directory(t){const path=mkdtempSync(join(tmpdir(),'lifecycle-test-'));t.after(()=>rmSync(path,{recursive:true,force:true}));return path;}

test('deadline rejects a stuck await and fences a late continuation',async()=>{
 let release,mutations=0,deadlineCalled=0;
 const work=new Promise(resolve=>{release=resolve;});
 const control=createWorkflowDeadline({timeoutMs:15,onFailure:()=>deadlineCalled++});
 const completion=control.run(async()=>{await work;control.assertActive();mutations++;});
 await assert.rejects(completion,{code:'workflow-deadline-exceeded'});
 release();await new Promise(resolve=>setImmediate(resolve));
 assert.equal(deadlineCalled,1);assert.equal(mutations,0);assert.equal(control.failed,true);
});

test('durable failure precedes never-resolving browser and transport; independent cleanup completes',async t=>{
 const path=join(directory(t),'failure.json'),attempts=[],failures=[];
 const began=Date.now();
 const results=await runBoundedCleanup({
  checkpoint:()=>writeFailureCheckpoint(path,{passed:false,status:'cleanup-incomplete',stage:'verification',controllers:[{pid:42,startTicks:43}],dockerRequestsDrained:false}),
  onFailure:error=>failures.push(error.code),
  phases:[
   {name:'browser',timeoutMs:15,run:()=>{assert.equal(JSON.parse(readFileSync(path)).passed,false);attempts.push('browser');return never();}},
   {name:'controllers',timeoutMs:15,run:()=>{attempts.push('controllers');return true;}},
   {name:'transport',timeoutMs:15,run:()=>{attempts.push('transport');return never();}},
   {name:'provider',timeoutMs:15,run:()=>{attempts.push('provider');return true;}},
  ],
 });
 assert.deepEqual(attempts,['browser','controllers','transport','provider']);
 assert.deepEqual(failures,['browser-deadline-exceeded','transport-deadline-exceeded']);
 assert.deepEqual(results.map(row=>row.status),['failed-or-unsettled','completed','failed-or-unsettled','completed']);
 assert(Date.now()-began<1000);assert.equal(JSON.parse(readFileSync(path)).dockerRequestsDrained,false);assert.equal(statSync(path).mode&0o777,0o600);
});

test('checkpoint errors still permit independent owned cleanup attempts',async()=>{
 const attempted=[],errors=[];
 await runBoundedCleanup({checkpoint:()=>{throw Error('disk unavailable');},onFailure:error=>errors.push(error),phases:[{name:'controller',timeoutMs:50,run:()=>attempted.push('controller')},{name:'proxy',timeoutMs:50,run:()=>attempted.push('proxy')}]});
 assert.equal(errors.length,1);assert.deepEqual(attempted,['controller','proxy']);
});

test('journal caps bytes/events, rejects reuse, and excludes secret-bearing fields',t=>{
 const path=join(directory(t),'journal.jsonl');
 const journal=createAcceptanceJournal(path,{maxEvents:1});
 journal.record('create-ack',{status:201,id:'a'.repeat(64),method:'POST',path:'/v1.51/containers/create?token=private',body:'private-body',headers:{Authorization:'private-auth'},env:['SECRET=private-env'],labels:{'luci.check':'true','secret':'private-label'}});
 assert.throws(()=>journal.record('driver-stage',{stage:'later'}),{code:'journal-budget-exceeded'});
 assert.throws(()=>journal.record('driver-stage',{stage:'later'}),{code:'journal-unavailable'});
 journal.close();const text=readFileSync(path,'utf8');assert(!text.includes('private'));assert.equal(JSON.parse(text).id,'a'.repeat(64));assert.equal(statSync(path).mode&0o777,0o600);
 assert.throws(()=>createAcceptanceJournal(path),{code:'EEXIST'});
});

test('failure locations preserve source line without exception message or private path',()=>{
 const error=Object.assign(new Error('password=private'),{code:'ERR_ASSERTION'});error.stack='Error: password=private\n at x (/private/fixture/scripts/acceptance/task-dependencies/dag.mjs:513:8)';
 const details=diagnosticError(error);assert.deepEqual(details.locations,[{file:'dag.mjs',line:513,column:8}]);assert(!JSON.stringify(details).includes('private'));
});

test('shared lifecycle and sustained diagnostics retain only approved basenames',t=>{
 const path=join(directory(t),'journal.jsonl');
 const error=new Error('credential=private');
 error.stack='Error: credential=private\n at run (/private/fixture/sustained.mjs:17:8)\n at timeout (/private/helpers/acceptance-lifecycle.mjs:29:5)\n at secret (/private/fixture/unknown.mjs:1:1)';
 const details=diagnosticError(error);
 assert.deepEqual(details.locations,[{file:'sustained.mjs',line:17,column:8},{file:'acceptance-lifecycle.mjs',line:29,column:5}]);
 const journal=createAcceptanceJournal(path);
 journal.record('driver-fatal',{...details,locations:[...details.locations,{file:'/private/sustained.mjs',line:2,column:1},{file:'unknown.mjs',line:1,column:1}]});
 journal.close();
 const text=readFileSync(path,'utf8'),row=JSON.parse(text);
 assert.deepEqual(row.locations,details.locations);
 assert(!text.includes('private'));assert(!text.includes('credential'));assert(!text.includes('unknown'));
});

// Ownership selection itself remains the fixture's exact profile/environment
// predicate. Here we prove a missing handle cannot bypass its identity gate.
test('missing browser handle or stuck close still reaches exact-owned fallback',async()=>{
 for(const browser of [undefined,{close:never}]){
 const owned={pid:123,start:'456',group:123},reused={pid:789,start:'10',group:789},killed=[],failures=[];let scans=0;
 await settleOwnedBrowser({browser,listOwned:()=>{scans++;return[owned,reused];},stillOwned:identity=>identity===owned,forceTerminate:identity=>killed.push(identity),onFailure:error=>failures.push(error.code),closeTimeoutMs:5,settleTimeoutMs:5,pollIntervalMs:1});
 assert(scans>0);assert.deepEqual(killed,[owned]);assert.equal(failures.length,browser?3:2);
 }
});
