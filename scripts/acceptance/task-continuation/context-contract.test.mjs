import assert from 'node:assert/strict';
import {test} from 'node:test';
import {createHash} from 'node:crypto';
import {contextFileNames,dependencyFile,typedPacketDigest,validateContinuationContext} from './continuation-context.mjs';
import {taskHandoff,answer} from './continuation-fixture.mjs';
const sha=value=>createHash('sha256').update(value).digest('hex');
export function prior(){return {projectId:'project',runId:'run',taskId:'task-work',operationId:'old-op',status:'captured',digest:'a'.repeat(64),receipt:{version:1,projectId:'project',runId:'run',taskId:'task-work',operationId:'old-op',childId:'s-aaaaaaaa',worktreeId:'w-old',requestDigest:'b'.repeat(64),implementationDigest:'c'.repeat(64),contextDigest:'d'.repeat(64),sourceCommit:'e'.repeat(40),sourceDigest:'f'.repeat(64),sourceDirty:true,status:'captured',capturedAt:'2026-10-08T10:00:00Z',reported:taskHandoff('partial')},answers:[{questionId:'q-one',answer,answeredAt:'2026-10-08T10:01:00Z',attribution:'Project editor'}],questionIds:['q-one'],recordedChecks:[],integration:null};}
function fixture(successor=false){
 const packet={version:1,projectId:'project',runId:'run',taskId:'task-work',operationId:successor?'next-op':'old-op',dependencies:[],...(successor?{previousAttempt:prior()}:{})};
 const expected={projectId:'project',runId:'run',taskId:'task-work',operationId:packet.operationId,dependencyDigest:typedPacketDigest(packet),previousAttempt:successor?structuredClone(packet.previousAttempt):undefined};
 const manifest={version:2,binding:{projectId:'project',taskIds:['task-work'],dependencyContextDigest:expected.dependencyDigest},files:[]},snapshot={};
 for(const name of contextFileNames(2)){const text=name===dependencyFile?JSON.stringify(packet,null,2)+'\n':'fixture '+name+'\n',row={path:name,bytes:Buffer.byteLength(text),digest:sha(text)};manifest.files.push(row);snapshot[name]={sha256:row.digest,bytes:row.bytes};}
 snapshot['manifest.json']={sha256:sha(JSON.stringify(manifest)),bytes:Buffer.byteLength(JSON.stringify(manifest))};return {manifest,snapshot,expected,packet};
}
const verify=f=>validateContinuationContext(f.manifest,f.snapshot,f.expected,f.packet);
test('initial V2 context has no invented previous attempt; successor has exact separate attribution',()=>{verify(fixture());verify(fixture(true));});
test('unknown versions, missing/duplicate/unlisted files and unknown packet fields fail',()=>{
 for(const mutate of [f=>{f.manifest.version=1},f=>{f.manifest.version=3},f=>{f.manifest.extra=1},f=>{f.manifest.files.push(f.manifest.files[0])},f=>{delete f.snapshot[dependencyFile]},f=>{f.snapshot['unexpected.txt']={sha256:'a'.repeat(64),bytes:0}},f=>{f.packet.extra=1}]){const f=fixture();mutate(f);assert.throws(()=>verify(f));}
});
test('member digest/size and typed dependency digest are separate exact checks',()=>{
 const f=fixture(true);assert.notEqual(f.expected.dependencyDigest,f.snapshot[dependencyFile].sha256);verify(f);
 for(const mutate of [f=>{f.manifest.files[0].bytes++},f=>{f.snapshot[dependencyFile].sha256='b'.repeat(64)},f=>{f.manifest.binding.dependencyContextDigest='c'.repeat(64)},f=>{f.packet.operationId='foreign'}]){const v=fixture(true);mutate(v);assert.throws(()=>verify(v));}
});
test('substituted child, old answer, report or fabricated verification cannot pass even with recomputed digest',()=>{
 for(const mutate of [f=>{f.packet.previousAttempt.receipt.childId='s-ffffffff'},f=>{f.packet.previousAttempt.answers[0].answer='Use 43'},f=>{f.packet.previousAttempt.receipt.reported.summary='other'},f=>{f.packet.previousAttempt.recordedChecks=[{status:'passed'}]},f=>{f.packet.previousAttempt.integration={state:'integrated'}},f=>{f.packet.dependencies=[f.packet.previousAttempt]}]){
  const f=fixture(true);mutate(f);f.expected.dependencyDigest=typedPacketDigest(f.packet);f.manifest.binding.dependencyContextDigest=f.expected.dependencyDigest;assert.throws(()=>verify(f));
 }
});
test('a prior operation cannot be silently absent from successor or injected into first context',()=>{for(const successor of [false,true]){const f=fixture(successor);if(successor)delete f.packet.previousAttempt;else f.packet.previousAttempt=prior();assert.throws(()=>verify(f));}});
test('typed digest preserves Go escaping and key order from actual typed export',()=>{assert.equal(typedPacketDigest({value:'<>&\u2028\u2029'}),sha('{"value":"\\u003c\\u003e\\u0026\\u2028\\u2029"}'));});
