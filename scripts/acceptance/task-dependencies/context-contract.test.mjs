import assert from 'node:assert/strict';
import {test} from 'node:test';
import {createHash} from 'node:crypto';
import {contextFileNames,dependencyFile,typedPacketDigest,validateContextManifest} from './context-contract.mjs';
import {taskHandoff} from './dag-fixture.mjs';
const sha=value=>createHash('sha256').update(value).digest('hex');
function fixture(version=2,joining=false){
 const deps=joining?['a','b2'].map((key,i)=>({taskId:i?'task-b':'task-a',operationId:i?'op-b2':'op-a',commit:String(i+1).repeat(40),sourceCommit:String(i+3).repeat(40),childId:i?'s-bbbbbbbb':'s-aaaaaaaa',contextDigest:'c'.repeat(64),reportDigest:'d'.repeat(64),report:taskHandoff(key)})):[];
 const packet={version:1,projectId:'project',runId:'run',taskId:joining?'task-join':'task-a',operationId:'op-current',dependencies:deps.map(dep=>({projectId:'project',runId:'run',taskId:dep.taskId,operationId:dep.operationId,status:'captured',digest:dep.reportDigest,receipt:{projectId:'project',runId:'run',taskId:dep.taskId,status:'captured',contextDigest:dep.contextDigest,operationId:dep.operationId,childId:dep.childId,sourceCommit:dep.sourceCommit,sourceDirty:false,reported:dep.report},answers:[],questionIds:[],recordedChecks:[],integration:{state:'integrated',commit:dep.commit,sourceCommit:dep.sourceCommit}}))};
 const expected={expectedVersion:version,projectId:'project',runId:'run',taskId:packet.taskId,operationId:'op-current',dependencies:deps,...(version===2?{dependencyDigest:typedPacketDigest(packet)}:{})};
 const manifest={version,binding:{projectId:'project',taskIds:[packet.taskId],...(version===2?{dependencyContextDigest:expected.dependencyDigest}:{})},files:[]},snapshot={};
 for(const name of contextFileNames(version)){const body=name===dependencyFile?JSON.stringify(packet,null,2)+'\n':'controlled '+name+'\n';const row={path:name,bytes:Buffer.byteLength(body),digest:sha(body)};manifest.files.push(row);snapshot[name]={sha256:row.digest,bytes:row.bytes};}
 snapshot['manifest.json']={sha256:sha(JSON.stringify(manifest)),bytes:Buffer.byteLength(JSON.stringify(manifest))};return {manifest,snapshot,expected,packet:version===2?packet:undefined};
}
const verify=f=>validateContextManifest(f.manifest,f.snapshot,f.expected,f.packet);
test('explicit legacy V1 and new empty-dependency V2 contracts validate independently',()=>{verify(fixture(1));verify(fixture(2));});
test('V2 join binds current A and B2 integrated operations and their attributed reports',()=>{verify(fixture(2,true));});
test('unknown version, manifest property, duplicate member and unlisted files fail closed',()=>{
 for(const mutate of [f=>{f.manifest.version=3},f=>{f.manifest.other=true},f=>{f.manifest.files.push(f.manifest.files[0])},f=>{f.snapshot['.luci/unrecognized.json']={sha256:'a'.repeat(64),bytes:2}},f=>{delete f.snapshot[dependencyFile]}]){const f=fixture();mutate(f);assert.throws(()=>verify(f));}
});
test('member bytes and dependency semantic digest are separately checked',()=>{
 const f=fixture();assert.notEqual(f.expected.dependencyDigest,f.snapshot[dependencyFile].sha256);verify(f);
 for(const mutate of [f=>{f.manifest.files[0].bytes++},f=>{f.snapshot[dependencyFile].sha256='b'.repeat(64)},f=>{f.manifest.binding.dependencyContextDigest='c'.repeat(64)},f=>{f.packet.operationId='op-foreign'}]){const f=fixture();mutate(f);assert.throws(()=>verify(f));}
});
test('retired B1, unintegrated commits and fabricated recorded checks cannot satisfy join',()=>{
 for(const mutate of [f=>{f.packet.dependencies[1].operationId='op-b1'},f=>{f.packet.dependencies[1].integration.state='unknown'},f=>{f.packet.dependencies[1].integration.commit='d'.repeat(40)},f=>{f.packet.dependencies[1].recordedChecks=[{status:'passed'}]},f=>{f.packet.dependencies[1].receipt.reported=taskHandoff('b1')}]){
  const f=fixture(2,true);mutate(f);f.expected.dependencyDigest=typedPacketDigest(f.packet);f.manifest.binding.dependencyContextDigest=f.expected.dependencyDigest;assert.throws(()=>verify(f));
 }
});
test('typed digest keeps Go HTML/line-separator escaping distinct from raw export bytes',()=>{assert.equal(typedPacketDigest({value:'<>&\u2028\u2029'}),sha('{"value":"\\u003c\\u003e\\u0026\\u2028\\u2029"}'));});
