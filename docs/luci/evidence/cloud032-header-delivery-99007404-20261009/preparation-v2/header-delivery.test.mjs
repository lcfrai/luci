import assert from 'node:assert/strict';
import {test} from 'node:test';
import {EventEmitter} from 'node:events';
import {createHeaderDelivery} from './header-delivery.mjs';
const tick=()=>new Promise(done=>setImmediate(done));
const base='http://127.0.0.1:15157',path='/v1/product/projects/a/overview';
class Session extends EventEmitter {
 calls=[]; rows=new Map(); cancelled=new Set(); bad=new Set();
 async send(name,args={}){
  this.calls.push({name,args});assert(!name.includes('getResponseBody'),'No body capture in headers-only mechanism');
  if(name==='Fetch.continueResponse'){
   if(this.bad.has(args.requestId))throw Error('Unexplained protocol failure');
   if(this.cancelled.has(args.requestId))throw Error('Protocol error: Invalid InterceptionId.');
   this.emit('Network.loadingFinished',{requestId:this.rows.get(args.requestId),timestamp:2});
  }
  return {};
 }
 async detach(){this.calls.push({name:'detach'});}
 request(id,{pathname=path,method='GET',origin=base}={}){
  const request={method,url:origin+pathname};this.rows.set('fetch-'+id,id);
  this.emit('Network.requestWillBeSent',{requestId:id,request,timestamp:1});return request;
 }
 response(id,request,status=200){this.emit('Fetch.requestPaused',{requestId:'fetch-'+id,networkId:id,request,responseStatusCode:status,responseHeaders:[{name:'content-type',value:'application/json'}]});}
 abort(id){this.cancelled.add('fetch-'+id);this.emit('Network.loadingFailed',{requestId:id,timestamp:2,canceled:true,errorText:'net::ERR_ABORTED'});}
}
async function fixture(extra={}){
 const session=new Session(),receipt={},failures=[];
 const manager=await createHeaderDelivery({base,projectIDs:['a','b','c','d'],receipt,onFailure:error=>failures.push(error.message),session,limits:{commandMs:50,cancelWitnessMs:5,drainMs:30,maxPauseMs:100,...extra}});
 return {session,receipt,manager,failures};
}
test('matched baseline resumes original headers; defer waits for round release without reading body',async()=>{
 const f=await fixture();
 for(const [round,condition]of [[0,'baseline'],[1,'defer']]){
  f.manager.enterRound({round,condition});const request=f.session.request(String(round));f.session.response(String(round),request);await tick();
  assert.equal(f.manager.stats().pending,condition==='defer'?1:0);
  await f.manager.endRound();assert.equal(f.manager.stats().pending,0);
 }
 assert.equal(f.receipt.headerDelivery.responses[0].releaseReason,'baseline-immediate');assert.equal(f.receipt.headerDelivery.responses[1].held,true);
 assert(f.session.calls.filter(row=>row.name==='Fetch.continueResponse').every(row=>Object.keys(row.args).join()==='requestId'));
 assert.deepEqual(f.failures,[]);assert(await f.manager.close());assert.doesNotThrow(()=>JSON.stringify(f.receipt));
});
test('403, POST, wrong path, unknown project and foreign origin are never held',async()=>{
 const f=await fixture();f.manager.enterRound({round:0,condition:'defer'});
 const cases=[{status:403},{method:'POST'},{pathname:path+'-extra'},{pathname:'/v1/product/projects/unknown/overview'},{origin:'http://foreign.invalid'}];
 for(const [i,value]of cases.entries()){const request=f.session.request(String(i),value);f.session.response(String(i),request,value.status??200);}
 await tick();await f.manager.endRound();assert(f.receipt.headerDelivery.responses.every(row=>!row.held&&row.releaseReason==='non-target-immediate'));assert.deepEqual(f.failures,[]);assert(await f.manager.close());
});
test('prebody natural cancellation resolves exact terminal ID without body capture, replay or disable during pending hold',async()=>{
 const f=await fixture();f.manager.enterRound({round:0,condition:'defer'});const request=f.session.request('abort');f.session.response('abort',request);await tick();
 f.session.abort('abort');await f.manager.endRound();assert.equal(f.receipt.headerDelivery.responses[0].outcome,'original-network-cancelled-terminal-ID');assert.equal(f.receipt.headerDelivery.requests[0].outcome,'natural-browser-abort');assert.equal(f.session.calls.filter(row=>row.name==='Fetch.continueResponse').length,1);assert(!f.session.calls.some(row=>row.name==='Fetch.disable'));assert(await f.manager.close());assert.deepEqual(f.failures,[]);
});
test('unexplained invalid/protocol failure is not relabelled natural cancellation or successful drain',async()=>{
 const f=await fixture();f.manager.enterRound({round:0,condition:'defer'});const request=f.session.request('bad');f.session.response('bad',request);await tick();f.session.bad.add('fetch-bad');
 await assert.rejects(f.manager.endRound());assert.equal(await f.manager.close(),false);assert(f.failures.length);assert.equal(f.manager.stats().pending,1);assert(!f.session.calls.some(row=>row.name==='Fetch.disable'));
});
test('a response initiated before the active round cannot inherit its intervention condition',async()=>{
 const f=await fixture();const request=f.session.request('old');f.manager.enterRound({round:0,condition:'defer'});f.session.response('old',request);await tick();await f.manager.endRound();assert.equal(f.receipt.headerDelivery.responses[0].releaseReason,'request-from-another-round');assert.equal(f.receipt.headerDelivery.responses[0].requestRound,null);assert(await f.manager.close());
});
test('hold deadline invalidates the diagnostic and continues original response without cancellation or replacement',async()=>{
 const f=await fixture({maxPauseMs:5});f.manager.enterRound({round:0,condition:'defer'});const request=f.session.request('late');f.session.response('late',request);await new Promise(done=>setTimeout(done,15));await assert.rejects(f.manager.endRound());assert.equal(f.receipt.headerDelivery.responses[0].releaseReason,'hold-timeout');assert(f.failures.length);assert(await f.manager.close());assert(!f.session.calls.some(row=>row.name==='Fetch.failRequest'||row.name==='Fetch.fulfillRequest'));
});
test('an oversized header is not retained as an eligible hold and invalidates the experiment',async()=>{
 const f=await fixture({maxHeaderBytes:4});f.manager.enterRound({round:0,condition:'defer'});const request=f.session.request('header');f.session.response('header',request);await tick();await assert.rejects(f.manager.endRound());assert.equal(f.receipt.headerDelivery.responses[0].releaseReason,'header-cap-refusal');assert(await f.manager.close());
});
test('long-task observations preserve sequence and refuse dropped rows',async()=>{
 const f=await fixture();const page={evaluate:async()=>({version:1,timeOrigin:1,rows:[{sequence:1,startTime:10,duration:60}],observed:1,dropped:0,unsupported:false})};await f.manager.longTasks(page,'first');assert.equal(f.receipt.headerDelivery.longTaskWindows.length,1);await assert.rejects(f.manager.longTasks({evaluate:async()=>({version:1,timeOrigin:1,rows:[],observed:1,dropped:1,unsupported:false})},'overflow'));assert(await f.manager.close());
});
test('out-of-order responses retain an exact request-sequence join for cancellation evidence',async()=>{
 const f=await fixture();f.manager.enterRound({round:0,condition:'defer'});
 const first=f.session.request('first'),second=f.session.request('second');
 f.session.response('second',second);f.session.response('first',first);await tick();f.session.abort('first');await f.manager.endRound();
 const responses=f.receipt.headerDelivery.responses;assert.deepEqual(responses.map(row=>row.requestSequence),[2,1]);
 assert.equal(responses.find(row=>row.requestSequence===1).outcome,'original-network-cancelled-terminal-ID');
 assert.equal(f.receipt.headerDelivery.requests.find(row=>row.sequence===1).outcome,'natural-browser-abort');assert(await f.manager.close());
});
