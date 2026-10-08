// Controlled manifest/parser cases only, not installed context delivery.
import test from 'node:test';import assert from 'node:assert/strict';import {createHash} from 'node:crypto';
import {contextFileNames,verifyTaskContext} from './task-context.mjs';
const sha=b=>createHash('sha256').update(b).digest('hex');
function fixture(version){
 const run={id:'run-123',projectId:'project-123'},operation={id:'op-123',childId:'s-12345678',taskId:'task-123'};
 const implementation={projectId:run.projectId,worktreeId:'work-123',sourceCommit:'a'.repeat(40),sourceDigest:'b'.repeat(64),sourceDirty:false};
 const packet={version:1,projectId:run.projectId,runId:run.id,taskId:operation.taskId,operationId:operation.id,dependencies:[]};
 if(version===2){implementation.dependencyContextDigest=sha(JSON.stringify(packet));operation.dependencyContext={digest:implementation.dependencyContextDigest,count:0};}
 operation.implementation=implementation;
 const files=Object.fromEntries(contextFileNames(version).map(name=>[name,Buffer.from(name==='.luci/task-handoffs.json'?JSON.stringify(packet,null,2)+'\n':'owned '+name)]));
 const manifest={version,binding:{...implementation,sourceCommit:'',sourceDigest:'',sourceDirty:false},files:contextFileNames(version).map(path=>({path,bytes:files[path].length,digest:sha(files[path])}))};
 files['manifest.json']=Buffer.from(JSON.stringify(manifest)+'\n');
 return {version,run,operation,files,session:{id:operation.childId,projectId:run.projectId,projectRunId:run.id,projectRunOperationId:operation.id,implementationContext:implementation,reviewedContext:{version,digest:sha(files['manifest.json'])}}};
}
for(const version of [1,2])test('exactV'+version+' file/binding fixture accepted',()=>verifyTaskContext(fixture(version)));
test('trustedV1 refuses V2 context rather than guessing upgrade',()=>{const f=fixture(2);f.version=1;assert.throws(()=>verifyTaskContext(f));});
for(const [name,change] of [
 ['missing packet',f=>delete f.files['.luci/task-handoffs.json']],
 ['wrong attempt',f=>f.operation.id='op-other'],
 ['wrong child',f=>f.session.id='s-87654321'],
 ['changed frozen bytes',f=>f.files['LUCI.md']=Buffer.from('different')],
 ['manifest authority changed',f=>f.session.reviewedContext.digest='f'.repeat(64)],
 ['typed packet digest differs',f=>f.operation.dependencyContext.digest='c'.repeat(64)],
 ['unexpected dependency count',f=>f.operation.dependencyContext.count=1],
])test('V2 refuses '+name,()=>{const f=fixture(2);change(f);assert.throws(()=>verifyTaskContext(f));});
