// Executes the real installed stop wrapper against controlled process identities.
import test from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFileSync} from 'node:fs';
import * as adapter from './installed-recovery.mjs';
const source=readFileSync(new URL('./installed-recovery.mjs',import.meta.url),'utf8');
const body=source.slice(source.indexOf('  async function stopControllers(){'),source.indexOf('  async function until('));
function fixture(){
 const identity={pid:1234,start:'5678',group:1234,status:'S'},signals=[],receipt={cleanup:[]};let live={...identity};
 const service={pid:1234,exitCode:null,signalCode:null,kill(signal){signals.push([1234,signal]);live=null;service.signalCode=signal;return true;}};
 const list=()=>live&&live.start===identity.start&&live.status!=='Z'?[live]:[];
 const stop=vm.runInNewContext('('+body.trim().replace('async function stopControllers','async function')+')',{
  assert,service,serviceIdentity:identity,ownedProcesses:list,receipt,delay:async()=>{},Date,
  processIdentity:()=>live,stillOwned:id=>!!live&&live.start===id.start&&live.status!=='Z',
  process:{kill(pid,signal){signals.push([pid,signal]);live=null;}},
  stopControllerTree:adapter.stopControllerTree,
 });
 return{stop,signals,receipt,service,setLive:v=>{live=v;},identity};
}
test('repeated stop after signalled child exit does not signal again or fail',async()=>{
 const f=fixture();await f.stop();assert.equal(f.service.exitCode,null);await f.stop();assert.deepEqual(f.signals,[[1234,'SIGTERM']]);assert.deepEqual(f.receipt.cleanup,[]);
});
test('does not signal a reused controller PID',async()=>{
 const f=fixture();f.setLive({...f.identity,start:'different'});await f.stop();assert.deepEqual(f.signals,[]);
});
test('already absent controller is a safe idempotent stop',async()=>{
 const f=fixture();f.setLive(null);await f.stop();await f.stop();assert.deepEqual(f.signals,[]);
});

function direct(overrides={}){
 let time=0;const identity={pid:1234,start:'5678',group:1234,status:'S'};
 return {identity,readIdentity:()=>null,listOwned:()=>[],signal(){throw Error('unexpected signal');},wait:async()=>{time+=10;},now:()=>time,graceMs:20,forceMs:20,...overrides};
}
test('forced owned descendant cleanup remains a failed acceptance even after absence',async()=>{
 const id={pid:5678,start:'original',group:5678,status:'S'},signals=[];let live=true,forced=0;
 await assert.rejects(adapter.stopControllerTree(direct({identity:null,readIdentity:()=>live?id:null,listOwned:()=>live?[id]:[],signal:(pid,kind)=>{signals.push([pid,kind]);live=false;},onForce:()=>forced++})),/required forced/);
 assert.deepEqual(signals,[[-5678,'SIGKILL']]);assert.equal(forced,1);
});
test('unsettled owned descendant is never reported closed',async()=>{
 const id={pid:5678,start:'original',group:5678,status:'S'};
 await assert.rejects(adapter.stopControllerTree(direct({identity:null,readIdentity:()=>id,listOwned:()=>[id],signal(){}})),/did not settle/);
});
test('unknown process observation propagates without signaling',async()=>{
 let signals=0;await assert.rejects(adapter.stopControllerTree(direct({readIdentity(){throw Error('unreadable identity');},signal(){signals++;}})),/unreadable/);assert.equal(signals,0);
});
test('exit between identity read and signal requires confirmed identity absence',async()=>{
 let live=true;const f=direct();f.readIdentity=()=>live?f.identity:null;f.signal=()=>{live=false;throw Object.assign(Error('gone'),{code:'ESRCH'});};
 await adapter.stopControllerTree(f);
});
test('permission failure cannot turn into cleanup success',async()=>{
 const f=direct();f.readIdentity=()=>f.identity;f.signal=()=>{throw Object.assign(Error('denied'),{code:'EPERM'});};await assert.rejects(adapter.stopControllerTree(f),/denied/);
});
