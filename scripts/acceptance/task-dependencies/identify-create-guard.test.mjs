// Offline source-contract and exact driver-seam tests. No Docker/browser/API.
import assert from 'node:assert/strict';
import {test} from 'node:test';
import {createHash} from 'node:crypto';
import {readFileSync,mkdtempSync,rmSync} from 'node:fs';
import {join,resolve} from 'node:path';
import {tmpdir} from 'node:os';
import {assertBootstrapOwnerID,diagnoseCreateGuard} from './identify-create-guard.mjs';
import {assertCheckLease,checkCommandDigest} from './project-check-guard.mjs';
import {writeFailureCheckpoint} from '../launch-recovery/acceptance-lifecycle.mjs';

// Supplied from the retained fixture's trusted ownership fact. The original
// Docker request body and identify assertion stack were NOT retained.
const observedOwner='user-zrRnlGWRa3y_G022kbVzWv';
test('the exact former assertion rejects the observed management owner; actual opaqueID shape accepts it',()=>{
 const original=new Function('fixtureOwnerID','assert',"assert.match(fixtureOwnerID??'',/^user-[a-f0-9]{24}$/);");
 assert.throws(()=>original(observedOwner,assert),{code:'ERR_ASSERTION'});
 assertBootstrapOwnerID(observedOwner);
 for(const value of ['user-'+'a'.repeat(22),'user-'+'A_-9'.repeat(5)+'Z_'])assertBootstrapOwnerID(value);
 for(const value of [undefined,'','user-'+'a'.repeat(24),'user-'+'a'.repeat(21),'team-'+'a'.repeat(22),'user-'+'a'.repeat(21)+'+','user-'+'a'.repeat(21)+'/','user-'+'a'.repeat(21)+'='])assert.throws(()=>assertBootstrapOwnerID(value));
});

function leaseFixture(ownerID=observedOwner){
 const current={id:'run-fixture',projectId:'p-fixture',input:{stable:true}};
 const check={id:'check-'+'c'.repeat(24),projectId:current.projectId,resourcesHeld:true,commands:[{allocationId:'checkexec-'+'9'.repeat(24),status:'running'}]};
 const fixture='/private-fixture',primary=fixture+'/projects/main',scopeDigest=createHash('sha256').update(resolve(join(fixture,'state/conversations'))).digest('hex');
 const options={check,primary,sourceDigest:'e'.repeat(64),uid:process.getuid(),gid:process.getgid(),scopeDigest,actor:{userId:ownerID,projectId:current.projectId},engineID:'fixture-engine',instance:'11111111-1111-4111-8111-111111111111',imageVirtualSize:1024,storageDriver:'vfs',now:Date.now()};
 const identity={protocol:1,engineId:options.engineID,instanceId:options.instance,allocationId:check.commands[0].allocationId,checkId:check.id,commandIndex:0,scopeDigest,projectId:current.projectId,commandDigest:''};
 identity.commandDigest=checkCommandDigest({...options,identity});
 const ledger={version:1,leases:[],checks:[{id:identity.allocationId,scope:scopeDigest,identity,actor:structuredClone(options.actor),resources:{CPUMillis:1000,MemoryMiB:1024},state:'creating',createDiskBytes:2048,createdAt:new Date(options.now-100).toISOString(),updatedAt:new Date(options.now-100).toISOString()}]};
 return {current,check,fixture,primary,options,identity,ledger};
}

const dag=readFileSync(new URL('./dag.mjs',import.meta.url),'utf8');
const ownedSource=dag.slice(dag.indexOf('function identifyOwnedCreate('),dag.indexOf('\nasync function checkAdmission('));
assert(ownedSource.startsWith('function identifyOwnedCreate('));
function driverGuard(f,{ownerID=observedOwner}={}){
 const owner={owner:{id:f.current.projectId,path:f.primary},run:f.current};
 const checkConsent={run:f.current,commit:'d'.repeat(40),sourceDigest:f.options.sourceDigest};
 const imageInfo={Id:'sha256:'+'f'.repeat(64),Size:f.options.imageVirtualSize};
 const input={Labels:{'luci.check':'true','luci.check-id':f.check.id}};
 const names=['assert','health','checkConsent','stage','privateJSON','fixture','owner','privateRun','gitAt','docker','engineID','fixtureOwnerID','assertBootstrapOwnerID','validateCheckCreate','imageInfo','privateCheckLedger','sha','resolve','join','instanceID','process'];
 const values=[assert,()=>{},checkConsent,'product-check-verification',()=>[f.check],f.fixture,owner,()=>f.current,(_path,command)=>command==='rev-parse'?checkConsent.commit:'',()=>JSON.stringify({ID:f.options.engineID,Driver:f.options.storageDriver}),f.options.engineID,ownerID,assertBootstrapOwnerID,
  (_input,options)=>{assert.deepEqual(options.actor,{userId:ownerID,projectId:f.current.projectId});assertCheckLease(f.ledger,options);return f.identity;},imageInfo,()=>f.ledger,value=>createHash('sha256').update(value).digest('hex'),resolve,join,()=>f.options.instance,process];
 const identify=new Function(...names,'let checkIdentity; return ('+ownedSource+');')(...values);
 return ()=>identify(Buffer.from(JSON.stringify(input)),{name:'luci-check-'+f.identity.allocationId});
}
test('exact current DAG check branch binds the observed bootstrap owner to the private lease',()=>{
 const f=leaseFixture();assert.equal(driverGuard(f)(),'check');
});
for(const change of [
 f=>f.ledger.checks[0].actor.userId='user-'+'b'.repeat(22),
 f=>f.ledger.checks[0].actor.projectId='p-other',
 f=>f.ledger.checks[0].actor.teamId='team-'+'x'.repeat(22),
 f=>f.ledger.checks[0].identity.projectId='p-other',
 f=>f.ledger.checks[0].state='finished',
 f=>f.ledger.checks.push(structuredClone(f.ledger.checks[0])),
])test('valid owner syntax never authorizes a changed actor/project/lease: '+change.toString(),()=>{
 const f=leaseFixture();change(f);assert.throws(driverGuard(f));
});
test('a different valid bootstrap owner does not match the retained owner lease',()=>{
 const f=leaseFixture();assert.throws(driverGuard(f,{ownerID:'user-'+'B'.repeat(22)}));
});

function scratch(t){const dir=mkdtempSync(join(tmpdir(),'luci-identify-guard-unit-'));t.after(()=>rmSync(dir,{recursive:true,force:true}));return join(dir,'diagnostic.json');}
function diagnosticInput(path,identify){return {bytes:Buffer.from('{"privateToken":"never-publish","Env":["SECRET=never-publish"]}'),stage:'product-check-verification',phase:'identify-create',requestSequence:965,ordinal:5,identify,persist:value=>writeFailureCheckpoint(path,value)};}
test('failure diagnostic is durable before the exact guard error escapes; forwarding is unreachable',t=>{
 const path=scratch(t),error=Object.assign(new assert.AssertionError({message:'never-publish'}),{stack:'AssertionError: never-publish\n at identifyOwnedCreate (/private-secret/dag.mjs:412:150)\n at guard (/private-secret/project-check-guard.mjs:34:2)'});
 let forwarded=0;const options=diagnosticInput(path,()=>{throw error;});
 assert.throws(()=>{diagnoseCreateGuard(options);forwarded++;},value=>{const row=JSON.parse(readFileSync(path));assert.equal(row.outcome,'refused');assert.equal(row.failure.code,'ERR_ASSERTION');assert.equal(row.forwarded,false);assert.deepEqual(row.failure.locations,[{file:'dag.mjs',line:412,column:150},{file:'project-check-guard.mjs',line:34,column:2}]);return value===error;});
 const bytes=readFileSync(path,'utf8'),row=JSON.parse(bytes);assert.equal(forwarded,0);assert.equal(row.bodySHA256,createHash('sha256').update(options.bytes).digest('hex'));assert.equal(row.bodyBytes,options.bytes.length);assert(bytes.length<2048);assert(!bytes.includes('never-publish'));assert(!bytes.includes('private-secret'));assert(!bytes.includes('SECRET='));assert.equal(row.body,undefined);
});
test('successful identification is persisted before simulated forward and is not a creation receipt',t=>{
 const path=scratch(t),events=[],options=diagnosticInput(path,()=>{events.push('guard');return 'check';});
 const persist=options.persist;options.persist=value=>{persist(value);events.push('durable');};
 assert.equal(diagnoseCreateGuard(options),'check');const row=JSON.parse(readFileSync(path));events.push('forward');
 assert.deepEqual(events,['guard','durable','forward']);assert.equal(row.outcome,'identified');assert.equal(row.forwarded,false);assert.equal(row.kind,'check');assert.equal(row.failure,undefined);
});
test('pre-forward revalidation retains the same body hash and separate phase',t=>{
 const path=scratch(t),options={...diagnosticInput(path,()=> 'worker'),phase:'before-create',ordinal:4};
 diagnoseCreateGuard(options);const row=JSON.parse(readFileSync(path));assert.equal(row.phase,'before-create');assert.equal(row.ordinal,4);
});
test('durable write failure refuses forwarding even if identification succeeded',()=>{
 let forwarded=0;assert.throws(()=>{diagnoseCreateGuard({...diagnosticInput('',()=> 'check'),persist:()=>{throw Error('disk failed');}});forwarded++;},/disk failed/);assert.equal(forwarded,0);
});
test('malformed body and arbitrary error text never enter bounded diagnostic',t=>{
 const path=scratch(t),options=diagnosticInput(path,()=>JSON.parse('{never-publish'));
 assert.throws(()=>diagnoseCreateGuard(options),SyntaxError);const row=JSON.parse(readFileSync(path));assert.equal(row.failure.name,'SyntaxError');assert.equal(row.failure.code,'identify-guard-failed');assert(!readFileSync(path,'utf8').includes('never-publish'));
});
test('exact DAG persists both identity phases and retains all existing ownership checks',()=>{
 assert(dag.includes('writeFailureCheckpoint(join(evidence,\'identify-create-'));
 assert(dag.includes("identifyCreate(body,{...meta,ordinal},'before-create')"));
 assert(ownedSource.includes('assertBootstrapOwnerID(fixtureOwnerID)'));
 assert(ownedSource.includes('actor:{userId:fixtureOwnerID,projectId:owner.owner.id}'));
 assert(dag.includes("'identify-create-guard.mjs'"));
 const proxy=readFileSync(new URL('../launch-recovery/fenced-docker-proxy.mjs',import.meta.url),'utf8');
 assert(proxy.indexOf('identifyCreate(body, cleanMeta(row))')<proxy.indexOf('beforeCreate({ ...cleanMeta(row), body })'));
 assert(proxy.indexOf('beforeCreate({ ...cleanMeta(row), body })')<proxy.indexOf('await forward(req, res, body, row)'));
});
