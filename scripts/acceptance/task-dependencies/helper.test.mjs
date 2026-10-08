// Pure helper/oracle checks with private scratch only; no Docker, Git or app.
import assert from 'node:assert/strict';
import {test} from 'node:test';
import {mkdtempSync,writeFileSync,rmSync,readFileSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {spawnSync} from 'node:child_process';
import {Script} from 'node:vm';
import {taskContracts,variants,variantFor,changedPaths,seedFiles,taskCommand,taskHandoff,expectedResult} from './dag-fixture.mjs';

test('exact four run-global attempts and dependency edges, with overlap explicitly serialized',()=>{
 assert.deepEqual(taskContracts.map(t=>t.dependsOn),[[],[],['task-a','task-b']]);
 assert(taskContracts[0].writePaths.includes('shared.txt')&&taskContracts[1].writePaths.includes('shared.txt'));
 assert.deepEqual(Object.entries(variants).map(([key,v])=>variantFor(v.taskId,v.attempt)),['a','b1','b2','join']);
 for(const [taskId,attempt] of [['task-b',1],['task-a',2],['task-join',3],['task-b',4],['foreign',1]])assert.throws(()=>variantFor(taskId,attempt));
 for(const [key,v] of Object.entries(variants))for(const name of changedPaths(key))assert(taskContracts.find(t=>t.id===v.taskId).writePaths.includes(name));
});
test('generated native command parses and binds exact source, task and frozen context',()=>{
 for(const variant of Object.keys(variants)){
  const command=taskCommand({variant,operation:'op-fixture',projectId:'p-fixture',runId:'run-fixture',dependencyDigest:'c'.repeat(64),baseCommit:'a'.repeat(40),contextHashes:{'manifest.json':'b'.repeat(64)},expectedUid:1000});
  assert.equal(command.workdir,'/home/dev/work');assert.equal(command.yield_time_ms,1000);
  const source=command.cmd.split("node <<'LUCI_DAG_NODE'\n")[1].split('\nLUCI_DAG_NODE')[0];new Script(source);
  assert(source.includes('manifest.binding.taskIds'));assert(source.includes('c.baseCommit'));assert(source.includes('EROFS'));assert(source.includes('LUCI_DAG_OBSERVATION='));assert(source.includes('manifest.version,2'));assert(source.includes('/home/dev/.luci-task-output'));assert(source.includes('handoff.json'));assert.deepEqual(taskHandoff(variant).changedFiles,changedPaths(variant));
 }
 assert.throws(()=>taskCommand({variant:'extra',operation:'op',projectId:'p',runId:'run-fixture',dependencyDigest:'c'.repeat(64),baseCommit:'a'.repeat(40),contextHashes:{},expectedUid:1000}));
 assert.throws(()=>taskCommand({variant:'a',operation:'../escape',projectId:'p',runId:'run-fixture',dependencyDigest:'c'.repeat(64),baseCommit:'a'.repeat(40),contextHashes:{},expectedUid:1000}));
});
function scratch(t){const dir=mkdtempSync(join(tmpdir(),'dag-helper-'));t.after(()=>rmSync(dir,{recursive:true,force:true}));for(const [name,text] of Object.entries(seedFiles))writeFileSync(join(dir,name),text);return dir;}
function apply(dir,key){for(const [name,text] of Object.entries(variants[key].files))writeFileSync(join(dir,name),text);}
function verify(dir,mode){return spawnSync(process.execPath,['verify.mjs',mode],{cwd:dir,env:{PATH:process.env.PATH},encoding:'utf8',timeout:10000,maxBuffer:1024*1024});}
test('alpha and initial beta independently pass distinct exact output oracles',t=>{
 for(const variant of ['a','b1']){const dir=scratch(t);apply(dir,variant);const r=verify(dir,variant);assert.equal(r.status,0,r.stderr);assert.equal(r.stdout,'DAG_CHECK_OK '+variant+'\n');}
});
test('replacement preserves alpha and final join verifies both values and exact output',t=>{
 const dir=scratch(t);apply(dir,'a');apply(dir,'b2');assert.equal(verify(dir,'b2').status,0);apply(dir,'join');assert.equal(verify(dir,'join').status,0);assert.equal(readFileSync(join(dir,'result.json'),'utf8'),expectedResult);
 for(const name of ['receipt-a.json','receipt-b2.json','receipt-join.json'])writeFileSync(join(dir,name),JSON.stringify({check:{code:0},baseCommit:'a'.repeat(40)}));assert.equal(verify(dir,'final').status,0);
 writeFileSync(join(dir,'result.json'),'{"alpha":17,"beta":25,"sum":43}\n');assert.notEqual(verify(dir,'final').status,0);apply(dir,'join');writeFileSync(join(dir,'receipt-b1.json'),'{}');assert.notEqual(verify(dir,'final').status,0);
});
test('join rejects a missing dependency or beta overwriting reviewed alpha',t=>{
 const dir=scratch(t);apply(dir,'b1');apply(dir,'join');assert.notEqual(verify(dir,'join').status,0);apply(dir,'a');apply(dir,'b1');assert.notEqual(verify(dir,'join').status,0);
});
