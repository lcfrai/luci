// Checkout exports and immutable worker capture have different contracts.
// journey_context.go exposes the former at GET /projects/{id}/context;
// reviewed_context.go adds brief.json only to the latter.
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';

export const projectExportPaths=Object.freeze([
 '.luci/design-handoff.json','.luci/design-handoff.md','.luci/design.json',
 '.luci/plan.json','.luci/specification.json','.luci/specification.md','LUCI.md',
]);
const sha=bytes=>createHash('sha256').update(bytes).digest('hex');

export function captureProjectContext({projectId,briefRevision,context,brief,read}){
 assert.equal(context.projectId,projectId,'Context must belong to the selected project');
 assert.equal(brief.projectId,projectId,'Brief must belong to the selected project');
 assert(Number.isSafeInteger(briefRevision)&&briefRevision>0);
 assert.equal(brief.revision,briefRevision,'Brief must match the selected run revision');
 assert(Array.isArray(context.exports));
 assert.deepEqual(context.exports.map(item=>item.path).sort(),[...projectExportPaths],
  'Public context exports must match the approved fixture contract');
 for(const item of context.exports)assert.equal(item.status,'current','Canonical export is not current: '+item.path);
 // Paths are fully validated before any callback reads from the project.
 const files=Object.fromEntries(context.exports.map(({path})=>[path,read(path)]));
 for(const [path,bytes] of Object.entries(files))assert(Buffer.isBuffer(bytes),'Expected export bytes: '+path);
 return {projectId,brief:structuredClone(brief),canonicalLuci:Buffer.from(files['LUCI.md']),
  canonicalBefore:Object.fromEntries(Object.entries(files).map(([path,bytes])=>[path,sha(bytes)]))};
}

export function verifyCapturedProjectContext({snapshot,read}){
 for(const path of projectExportPaths.filter(path=>path!=='LUCI.md'))
  assert.equal(sha(read(path)),snapshot.canonicalBefore[path],'Captured canonical export differs from its own project: '+path);
 const luci=read('LUCI.md');
 assert(luci.subarray(0,snapshot.canonicalLuci.length).equals(snapshot.canonicalLuci),
  'Task brief must retain exact own-project LUCI prefix');
 const brief=JSON.parse(read('.luci/brief.json'));
 assert.equal(brief.projectId,snapshot.projectId,'Captured brief must belong to the selected project');
 assert.deepEqual(brief,snapshot.brief,'Captured brief must equal the public approved project brief');
}
