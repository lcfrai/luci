// Explicit V1 legacy / V2 task bundle contract; no I/O on import.
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';

export const contextFilesV1=['.luci/brief.json','.luci/design-handoff.json','.luci/design-handoff.md','.luci/design.json','.luci/plan.json','.luci/specification.json','.luci/specification.md','LUCI.md'];
export const dependencyFile='.luci/task-handoffs.json';
export function contextFileNames(version){assert(version===1||version===2,'Unknown reviewed context version');return [...contextFilesV1,...(version===2?[dependencyFile]:[])].sort();}
const sha=value=>createHash('sha256').update(value).digest('hex');
// Go's typed packet is serialized in declaration order, preserved by its JSON
// document. Match encoding/json HTML and line-separator escaping explicitly.
export const typedPacketDigest=packet=>sha(JSON.stringify(packet).replace(/[<>&\u2028\u2029]/g,c=>'\\u'+c.charCodeAt(0).toString(16).padStart(4,'0')));
export function validateContextManifest(manifest,snapshot,{expectedVersion,projectId,runId,taskId,operationId,dependencyDigest,dependencies=[]},packet){
 assert.deepEqual(Object.keys(manifest).sort(),['binding','files','version']);
 assert.equal(manifest.version,expectedVersion);const names=contextFileNames(manifest.version);
 assert.deepEqual(Object.keys(snapshot).sort(),[...names,'manifest.json'].sort());
 assert.deepEqual(manifest.files.map(row=>row.path).sort(),names);assert.equal(new Set(manifest.files.map(row=>row.path)).size,names.length);
 for(const file of manifest.files){assert.deepEqual(Object.keys(file).sort(),['bytes','digest','path']);assert.equal(file.digest,snapshot[file.path].sha256);assert.equal(file.bytes,snapshot[file.path].bytes);assert.match(file.digest,/^[a-f0-9]{64}$/);assert(Number.isSafeInteger(file.bytes)&&file.bytes>=0&&file.bytes<=8*1024**2);}
 assert.equal(manifest.binding.projectId,projectId);assert.deepEqual(manifest.binding.taskIds,[taskId]);
 if(manifest.version===1){assert.equal(packet,undefined);assert.equal(manifest.binding.dependencyContextDigest,undefined);assert.equal(dependencyDigest,undefined);return;}
 assert.match(dependencyDigest??'',/^[a-f0-9]{64}$/);assert.equal(manifest.binding.dependencyContextDigest,dependencyDigest);
 assert.deepEqual(Object.keys(packet).sort(),['dependencies','operationId','projectId','runId','taskId','version']);
 assert.equal(packet.version,1);assert.equal(packet.projectId,projectId);assert.equal(packet.runId,runId);assert.equal(packet.taskId,taskId);assert.equal(packet.operationId,operationId);assert.equal(typedPacketDigest(packet),dependencyDigest);
 assert.deepEqual(packet.dependencies.map(dep=>dep.taskId),dependencies.map(dep=>dep.taskId));
 for(let i=0;i<dependencies.length;i++){
  const actual=packet.dependencies[i],expected=dependencies[i];assert.match(expected.reportDigest??'',/^[a-f0-9]{64}$/);assert.match(expected.contextDigest??'',/^[a-f0-9]{64}$/);assert.equal(actual.projectId,projectId);assert.equal(actual.runId,runId);assert.equal(actual.operationId,expected.operationId);assert.equal(actual.integration.state,'integrated');assert.equal(actual.integration.commit,expected.commit);assert.equal(actual.integration.sourceCommit,expected.sourceCommit);
  assert.equal(actual.status,'captured');assert.equal(actual.digest,expected.reportDigest);assert.equal(actual.receipt.projectId,projectId);assert.equal(actual.receipt.runId,runId);assert.equal(actual.receipt.taskId,expected.taskId);assert.equal(actual.receipt.status,'captured');assert.equal(actual.receipt.contextDigest,expected.contextDigest);assert.equal(actual.receipt.operationId,expected.operationId);assert.equal(actual.receipt.childId,expected.childId);assert.equal(actual.receipt.sourceCommit,expected.sourceCommit);assert.equal(actual.receipt.sourceDirty,false);assert.deepEqual(actual.receipt.reported,expected.report);assert.deepEqual(actual.answers,[]);assert.deepEqual(actual.questionIds,[]);assert.deepEqual(actual.recordedChecks,[],'Worker-reported checks must not become recorded project check evidence');
 }
}
