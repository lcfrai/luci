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
