// Executes the actual installed openRecovery function with deferred check history.
// Page/locator transport is controlled; no browser, server or recovery mutation runs.
import test from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFileSync} from 'node:fs';

const source=readFileSync(new URL('./installed-recovery.mjs',import.meta.url),'utf8');
const start=source.indexOf('  async function openRecovery('),end=source.indexOf('  async function screenshot(',start);
assert(start>=0&&end>start);
const body=source.slice(start,end);
function deferred(){let resolve,reject;const promise=new Promise((ok,no)=>{resolve=ok;reject=no;});return {promise,resolve,reject};}
function fixture(overrides={}){
 const history=deferred(),events=[],project={id:'p-target'},id='check-target';
 const review={projectId:project.id,checkId:id,expectedRevision:3,allocationId:'alloc-target'};
 let visible=false,predicate,respond,count=overrides.count??1;
 const buttons={
  async waitFor(options){assert.equal(options?.state,'visible');events.push('wait-history');await history.promise;visible=true;},
  async count(){return visible?count:0;},
  async click(){assert(visible,'Recovery control clicked before history was ready');assert.equal(count,1);events.push('click-review');
   const response={url:()=> 'http://fixture'+(overrides.responsePath??'/projects/p-target/checks/check-target/recovery'),
    request:()=>({method:()=>overrides.method??'GET'}),status:()=>overrides.status??200,json:async()=>overrides.responseReview??review};
   assert(predicate(response),'Recovery response must match the exact GET identity');respond(response);
  },
 };
 const dialogObject={
  getByText(text,options){
   if(typeof text==='string'){assert.equal(text,id);assert.equal(options.exact,true);return {async waitFor(){assert.equal(overrides.dialogID??id,id,'Dialog check identity changed');events.push('dialog-check');}};}
   assert.equal(text.source,'A small permanent Docker recovery marker');return {async waitFor(){events.push('dialog-marker');}};
  },
  getByRole(role,options){assert.equal(role,'button');assert.equal(options.name,'Recover this exact check');assert.equal(options.exact,true);return {async isDisabled(){events.push('recovery-disabled');return overrides.disabled??true;}};},
 };
 const page={
  waitForResponse(match){assert(!predicate);predicate=match;events.push('wait-response');return new Promise(resolve=>{respond=resolve;});},
  locator(selector){assert.equal(selector,'.project-checks .check-runs > li');return {getByRole(role,options){assert.equal(role,'button');assert.equal(options.name,'Review check recovery');assert.equal(options.exact,true);return buttons;}};},
 };
 const openRecovery=vm.runInNewContext(body+'\nopenRecovery',{
  assert,URL,page,route:p=>'/projects/'+p.id,dialog:()=>dialogObject,
  openProject:async p=>{assert.equal(p,project);events.push('project-heading-ready');},
 },{timeout:1000});
 return {open:()=>openRecovery(project,id,review),events,dialogObject,
  release:()=>{visible=true;history.resolve();},refuse:()=>history.reject(Error('Recovery control visibility timed out'))};
}

test('Actual recovery opening waits for delayed check history before exact single-control click',async()=>{
 const f=fixture();let settled=false;
 const outcome=f.open().then(value=>{settled=true;return {value};},error=>{settled=true;return {error};});
 await new Promise(setImmediate);
 if(settled)throw (await outcome).error??Error('Recovery opened before check history was released');
 assert(!f.events.includes('click-review'));
 f.release();const result=await outcome;if(result.error)throw result.error;
 assert.equal(result.value,f.dialogObject);
 assert.deepEqual(f.events,['project-heading-ready','wait-response','wait-history','click-review','dialog-check','dialog-marker','recovery-disabled']);
});

test('Duplicate recovery controls remain an error after history readiness, without clicking',async()=>{
 const f=fixture({count:2});f.release();await assert.rejects(f.open(),error=>error.code==='ERR_ASSERTION');assert(!f.events.includes('click-review'));
});

test('Unavailable recovery control fails the wait without clicking or recovery mutation',async()=>{
 const f=fixture(),outcome=assert.rejects(f.open(),/visibility timed out/);await new Promise(setImmediate);f.refuse();await outcome;assert(!f.events.includes('click-review'));
});

test('Ready controls still require the exact recovery GET and unmodified review',async()=>{
 for(const overrides of [{responsePath:'/projects/p-other/checks/check-target/recovery'},
  {responsePath:'/projects/p-target/checks/check-other/recovery'},{method:'POST'},{status:409},
  {responseReview:{projectId:'p-target',checkId:'check-target',expectedRevision:4,allocationId:'alloc-target'}}]){
  const f=fixture(overrides);f.release();await assert.rejects(f.open(),error=>error.code==='ERR_ASSERTION');assert(!f.events.includes('dialog-check'));
 }
});

test('Recovery dialog retains exact check identity and requires unchecked confirmation',async()=>{
 for(const overrides of [{dialogID:'check-other'},{disabled:false}]){
  const f=fixture(overrides);f.release();await assert.rejects(f.open(),error=>error.code==='ERR_ASSERTION');
  assert.equal(f.events.filter(event=>event==='click-review').length,1);
 }
});
