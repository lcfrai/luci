import assert from 'node:assert/strict';
import {test} from 'node:test';
import {validateCheckpointReview,validatePreviousAttempt,validateSeedOperation} from './checkpoint-contract.mjs';
import {partialFiles,seedFiles,answer,taskHandoff} from './continuation-fixture.mjs';
function fixture(){
 const source={id:'old-op',childId:'s-aaaaaaaa',worktreeId:'w-old',taskId:'task-work',attempt:1,status:'review-required',allocation:'ready',baseCommit:'a'.repeat(40),requestDigest:'b'.repeat(64)};
 const review={projectId:'project',runId:'run',operationId:source.id,childId:source.childId,taskId:source.taskId,expectedRevision:9,sourceCommit:'c'.repeat(40),targetCommit:source.baseCommit,reviewDigest:'d'.repeat(64),canContinue:true,files:Object.entries(partialFiles).map(([path,after])=>({path,after,before:seedFiles[path]??'',beforeMode:path==='work.mjs'?'100644':'',afterMode:'100644',status:path==='work.mjs'?'modified':'added'}))};review.totalBytes=review.files.reduce((n,f)=>n+Buffer.byteLength(f.before)+Buffer.byteLength(f.after),0);
 const expected={projectId:'project',runId:'run',operation:source,revision:9,targetCommit:source.baseCommit,sourceCommit:review.sourceCommit};
 const next={...source,id:'next-op',childId:'s-bbbbbbbb',worktreeId:'w-new',attempt:2,status:'reserved',checkpoint:{sourceOperationId:source.id,sourceChildId:source.childId,sourceCommit:review.sourceCommit,sourceBaseCommit:source.baseCommit,targetCommit:source.baseCommit,seedCommit:'e'.repeat(40),reviewDigest:review.reviewDigest,contextDigest:'f'.repeat(64)},implementation:{sourceCommit:'e'.repeat(40)}};
 const reservation={operation:next,run:{id:'run',operations:[source,next],swarm:{tasks:[{id:'task-work',operationId:next.id}]}}};return {source,review,expected,reservation};
}
test('complete text review includes committed, dirty and untracked contents with exact byte total',()=>{const f=fixture();validateCheckpointReview(f.review,f.expected);});
test('truncated text, missing file, wrong mode, scope, child, revision or byte count cannot be consented',()=>{
 for(const mutate of [f=>{f.review.files.pop()},f=>{f.review.files[0].after='...'},f=>{f.review.files[0].afterMode='100755'},f=>{f.review.projectId='other'},f=>{f.review.childId='s-ffffffff'},f=>{f.review.expectedRevision++},f=>{f.review.totalBytes--},f=>{f.review.canContinue=false}]){const f=fixture();mutate(f);assert.throws(()=>validateCheckpointReview(f.review,f.expected));}
});
test('successor must be distinct reserved ready and exactly bound to copied source, seed and base',()=>{const f=fixture();validateSeedOperation(f.reservation,{runId:'run',source:f.source,review:f.review,targetCommit:f.source.baseCommit});
 for(const mutate of [f=>{f.reservation.operation.id=f.source.id},f=>{f.reservation.operation.childId=f.source.childId},f=>{f.reservation.operation.status='running'},f=>{f.reservation.operation.allocation='unknown'},f=>{f.reservation.operation.checkpoint.seedCommit=f.source.baseCommit},f=>{f.reservation.operation.implementation.sourceCommit=f.source.baseCommit},f=>{f.reservation.run.operations[0]={...f.source,status:'cancelled'}}]){const f=fixture();mutate(f);assert.throws(()=>validateSeedOperation(f.reservation,{runId:'run',source:f.source,review:f.review,targetCommit:f.source.baseCommit}));}
});
test('previous attempt receipt binds exact child and context and keeps editor answer separate',()=>{
 const f=fixture(),view={projectId:'project',runId:'run',taskId:'task-work',operationId:f.source.id,status:'captured',digest:'1'.repeat(64),integration:null,recordedChecks:[],questionIds:['q'],answers:[{questionId:'q',answer,attribution:'Editor',answeredAt:'2026-10-08T10:00:00Z'}],receipt:{projectId:'project',runId:'run',taskId:'task-work',operationId:f.source.id,childId:f.source.childId,worktreeId:f.source.worktreeId,requestDigest:f.source.requestDigest,implementationDigest:'2'.repeat(64),contextDigest:'3'.repeat(64),sourceCommit:f.review.sourceCommit,sourceDirty:true,reported:taskHandoff('partial')}};
 const expected={projectId:'project',runId:'run',operation:f.source,contextDigest:'3'.repeat(64),sourceCommit:f.review.sourceCommit};validatePreviousAttempt(view,expected);
 for(const mutate of [v=>{v.receipt.childId='s-ffffffff'},v=>{v.receipt.contextDigest='4'.repeat(64)},v=>{v.receipt.sourceDirty=false},v=>{v.answers[0].answer='43'},v=>{v.answers[0].attribution=''},v=>{v.recordedChecks=[{status:'passed'}]}]){const bad=structuredClone(view);mutate(bad);assert.throws(()=>validatePreviousAttempt(bad,expected));}
});
