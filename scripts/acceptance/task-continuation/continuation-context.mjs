import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
export const contextFilesV1=['.luci/brief.json','.luci/design-handoff.json','.luci/design-handoff.md','.luci/design.json','.luci/plan.json','.luci/specification.json','.luci/specification.md','LUCI.md'];
export const dependencyFile='.luci/task-handoffs.json';
export function contextFileNames(version){assert.equal(version,2,'This witness requires explicit task context V2');return [...contextFilesV1,dependencyFile].sort();}
export const typedPacketDigest=packet=>createHash('sha256').update(JSON.stringify(packet).replace(/[<>&\u2028\u2029]/g,c=>'\\u'+c.charCodeAt(0).toString(16).padStart(4,'0'))).digest('hex');
export function validateContinuationContext(manifest,snapshot,{projectId,runId,taskId,operationId,dependencyDigest,previousAttempt},packet){
 assert.deepEqual(Object.keys(manifest).sort(),['binding','files','version']);const names=contextFileNames(manifest.version);
 assert.deepEqual(Object.keys(snapshot).sort(),[...names,'manifest.json'].sort());assert.deepEqual(manifest.files.map(f=>f.path).sort(),names);
 assert.equal(new Set(manifest.files.map(f=>f.path)).size,names.length);
 for(const f of manifest.files){assert.deepEqual(Object.keys(f).sort(),['bytes','digest','path']);assert.equal(f.digest,snapshot[f.path].sha256);assert.equal(f.bytes,snapshot[f.path].bytes);assert.match(f.digest,/^[a-f0-9]{64}$/);assert(Number.isSafeInteger(f.bytes)&&f.bytes>=0&&f.bytes<=8*1024**2);}
 assert.equal(manifest.binding.projectId,projectId);assert.deepEqual(manifest.binding.taskIds,[taskId]);assert.match(dependencyDigest??'',/^[a-f0-9]{64}$/);assert.equal(manifest.binding.dependencyContextDigest,dependencyDigest);
 assert.deepEqual(Object.keys(packet).sort(),['version','projectId','runId','taskId','operationId','dependencies',...(previousAttempt?['previousAttempt']:[])].sort());
 assert.equal(packet.version,1);assert.equal(packet.projectId,projectId);assert.equal(packet.runId,runId);assert.equal(packet.taskId,taskId);assert.equal(packet.operationId,operationId);assert.equal(typedPacketDigest(packet),dependencyDigest);assert.deepEqual(packet.dependencies,[]);
 assert.deepEqual(packet.previousAttempt,previousAttempt);
 if(previousAttempt){assert.equal(previousAttempt.projectId,projectId);assert.equal(previousAttempt.runId,runId);assert.equal(previousAttempt.taskId,taskId);assert.notEqual(previousAttempt.operationId,operationId);assert.equal(previousAttempt.status,'captured');assert.equal(previousAttempt.integration,null);assert.deepEqual(previousAttempt.recordedChecks,[]);assert.equal(previousAttempt.receipt.sourceDirty,true);assert.equal(previousAttempt.answers.length,1);assert.equal(previousAttempt.questionIds.length,1);assert.equal(previousAttempt.answers[0].questionId,previousAttempt.questionIds[0]);assert(previousAttempt.answers[0].attribution);}
}
