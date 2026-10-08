import test from 'node:test';
import assert from 'node:assert/strict';
import {stopControllerTree} from './installed-recovery.mjs';
function fixture(){let time=0,live=true,reads=0;const signals=[],identity={pid:1234,start:'original',group:1234,status:'S'};
 const args={identity,readIdentity:()=>{reads++;return live?identity:null;},listOwned:()=>[],signal:(pid,kind)=>{signals.push([pid,kind]);if(kind==='SIGKILL')live=false;},wait:async()=>{time+=10;},now:()=>time,graceMs:20,forceMs:20};
 return{args,signals,identity,live:()=>live,reads:()=>reads,time:()=>time};
}
test('known controller omitted by environment scan cannot certify TERM settlement',async()=>{
 const f=fixture();await assert.rejects(stopControllerTree(f.args),/required forced/);
 assert.deepEqual(f.signals,[[1234,'SIGTERM'],[-1234,'SIGKILL']]);assert(f.reads()>1);assert(f.time()>=20);assert.equal(f.live(),false);
});
test('unreadable known identity after TERM refuses cleanup success',async()=>{
 const f=fixture();let sent=false;f.args.signal=()=>{sent=true;};f.args.readIdentity=()=>{if(sent)throw Object.assign(Error('identity observation unavailable'),{code:'EACCES'});return f.identity;};
 await assert.rejects(stopControllerTree(f.args),/identity observation unavailable/);
});
