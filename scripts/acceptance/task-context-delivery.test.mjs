import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {test} from 'node:test';
import {verifyTaskContextDelivery} from './task-context-delivery.mjs';

function fixture(version=2){
 const implementation={projectId:'project',worktreeId:'work',taskIds:['task'],sourceCommit:'a'.repeat(40),sourceDigest:'b'.repeat(64),sourceDirty:false,...(version===2?{dependencyContextDigest:'c'.repeat(64)}:{})};
 const operation={id:'op',childId:'s-12345678',taskId:'task',worktreeId:'work',implementation};
 const run={projectId:'project',id:'run',swarm:{tasks:[{id:'task',operationId:'op'}]}};
 const manifestBytes=Buffer.from(JSON.stringify({version,binding:{...implementation,sourceCommit:'',sourceDigest:'',sourceDirty:false},files:[]})+'\n');
 const digest=createHash('sha256').update(manifestBytes).digest('hex');
 const common={id:operation.childId,projectId:'project',projectRunId:'run',projectRunOperationId:'op',worktreeId:'work',implementationContext:structuredClone(implementation)};
 return {version,run,operation,manifestBytes,session:{...structuredClone(common),reviewedContextDelivery:{status:'captured',version,digest}},stored:{...structuredClone(common),reviewedContext:{version,digest}}};
}
test('actual public delivery DTO and private reference both bind V1/V2 captured bytes',()=>{
 for(const version of [1,2]){const f=fixture(version),v=verifyTaskContextDelivery(f);assert.equal(v.manifestDigest,f.stored.reviewedContext.digest);assert.equal(f.session.reviewedContext,undefined);assert.equal(f.stored.reviewedContext.status,undefined);}
});
test('private-shape wire receipt or missing/legacy delivery does not assert captured context',()=>{
 for(const change of [f=>{f.session.reviewedContext=f.stored.reviewedContext;delete f.session.reviewedContextDelivery},f=>delete f.session.reviewedContextDelivery,f=>f.session.reviewedContextDelivery={status:'legacy-live'},f=>f.session.reviewedContextDelivery.status='captured-but-unverified']){const f=fixture();change(f);assert.throws(()=>verifyTaskContextDelivery(f));}
});
test('changed manifest bytes, version, public or private digest are refused',()=>{
 for(const change of [f=>f.manifestBytes=Buffer.concat([f.manifestBytes,Buffer.from(' ')]),f=>f.version=1,f=>f.session.reviewedContextDelivery.digest='0'.repeat(64),f=>f.stored.reviewedContext.digest='0'.repeat(64),f=>f.stored.reviewedContext.version=1]){const f=fixture();change(f);assert.throws(()=>verifyTaskContextDelivery(f));}
});
test('foreign session, attempt, source and worktree cannot substitute for the expected task',()=>{
 for(const field of ['id','projectId','projectRunId','projectRunOperationId','worktreeId'])for(const name of ['session','stored']){const f=fixture();f[name][field]='other';assert.throws(()=>verifyTaskContextDelivery(f));}
 for(const change of [f=>f.run.swarm.tasks[0].operationId='other',f=>f.session.implementationContext.sourceCommit='d'.repeat(40),f=>f.stored.implementationContext.taskIds=['other'],f=>f.operation.implementation.sourceDigest='d'.repeat(64)]){const f=fixture();change(f);assert.throws(()=>verifyTaskContextDelivery(f));}
});
test('matching manifest hashes still refuse substituted approved-context binding',()=>{
 const f=fixture(),manifest=JSON.parse(f.manifestBytes);manifest.binding.taskIds=['other'];f.manifestBytes=Buffer.from(JSON.stringify(manifest));const digest=createHash('sha256').update(f.manifestBytes).digest('hex');f.session.reviewedContextDelivery.digest=digest;f.stored.reviewedContext.digest=digest;assert.throws(()=>verifyTaskContextDelivery(f));
});
