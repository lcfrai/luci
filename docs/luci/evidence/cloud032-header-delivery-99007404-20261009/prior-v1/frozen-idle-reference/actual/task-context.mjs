// Exact fixture-owned context contract; no process, API or Docker operations.
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
const sha=b=>createHash('sha256').update(b).digest('hex');
const original=['.luci/brief.json','.luci/design-handoff.json','.luci/design-handoff.md','.luci/design.json','.luci/plan.json','.luci/specification.json','.luci/specification.md','LUCI.md'];
export function contextFileNames(version){assert(version===1||version===2);return (version===1?[...original]:[...original,'.luci/task-handoffs.json']).sort();}
export function verifyTaskContext({version,session,operation,run,files}){
 const names=contextFileNames(version);assert.deepEqual(Object.keys(files).sort(),[...names,'manifest.json'].sort());
 const manifest=JSON.parse(files['manifest.json']);assert.equal(manifest.version,version);assert.equal(session.reviewedContext.version,version);assert.equal(sha(files['manifest.json']),session.reviewedContext.digest);
 assert.deepEqual(manifest.files,names.map(path=>({path,bytes:files[path].length,digest:sha(files[path])})));
 assert.deepEqual(manifest.binding,{...session.implementationContext,sourceCommit:'',sourceDigest:'',sourceDirty:false});
 assert.equal(session.projectId,run.projectId);assert.equal(session.projectRunId,run.id);assert.equal(session.projectRunOperationId,operation.id);assert.equal(session.id,operation.childId);assert.deepEqual(session.implementationContext,operation.implementation);
 if(version===1){assert.equal(session.implementationContext.dependencyContextDigest,undefined);return;}
 const packet=JSON.parse(files['.luci/task-handoffs.json']);
 // All five approved workloads are independent; neither a prerequisite nor a
 // checkpoint may be substituted into this specific controlled experiment.
 const expected={version:1,projectId:run.projectId,runId:run.id,taskId:operation.taskId,operationId:operation.id,dependencies:[]};
 assert.deepEqual(packet,expected);assert.deepEqual(operation.dependencyContext,{digest:session.implementationContext.dependencyContextDigest,count:0});
 // This empty packet contains ASCII identity fields only, so the typed Go
 // canonical encoding equals this explicitly ordered JSON (not file bytes).
 for(const id of [run.projectId,run.id,operation.taskId,operation.id])assert.match(id,/^[a-zA-Z0-9_-]{1,200}$/);
 assert.equal(sha(Buffer.from(JSON.stringify(expected))),operation.dependencyContext.digest);
}
