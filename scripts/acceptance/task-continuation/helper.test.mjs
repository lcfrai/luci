// Pure source/oracle validation. Private scratch only; never invokes Git, Docker, a browser or Luci.
import assert from 'node:assert/strict';
import {test} from 'node:test';
import {mkdtempSync,writeFileSync,rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {spawnSync} from 'node:child_process';
import {Script} from 'node:vm';
import {taskContracts,variants,variantFor,changedPaths,seedFiles,taskCommand,taskHandoff,partialFiles,finalFiles,question} from './continuation-fixture.mjs';
function scratch(t){const dir=mkdtempSync(join(tmpdir(),'continuation-helper-'));t.after(()=>rmSync(dir,{recursive:true,force:true}));for(const [name,text] of Object.entries(seedFiles))writeFileSync(join(dir,name),text);return dir;}
function apply(dir,files){for(const [name,text] of Object.entries(files))writeFileSync(join(dir,name),text);}
function verify(dir,mode){return spawnSync(process.execPath,['verify.mjs',mode],{cwd:dir,env:{PATH:process.env.PATH},encoding:'utf8',timeout:10000,maxBuffer:1024*1024});}
test('only two exact worker attempts allowed; dependent task is outside authorization',()=>{
 assert.deepEqual(taskContracts.map(t=>t.dependsOn),[[],['task-work']]);assert.deepEqual(Object.entries(variants).map(([key,v])=>variantFor(v.taskId,v.attempt)),['partial','finish']);
 for(const [taskId,attempt] of [['task-work',0],['task-work',3],['task-join',1],['task-join',3],['foreign',1]])assert.throws(()=>variantFor(taskId,attempt));
 for(const key of Object.keys(variants))for(const file of changedPaths(key))assert(taskContracts[0].writePaths.includes(file));
});
test('generated actual native commands parse with source/context/seed/answer checks',()=>{
 for(const variant of Object.keys(variants)){
  const command=taskCommand({variant,operation:'op-fixture',projectId:'p-fixture',runId:'run-fixture',baseCommit:'a'.repeat(40),contextHashes:{'manifest.json':'b'.repeat(64)},dependencyDigest:'c'.repeat(64),previousAttempt:variant==='finish'?{exact:'fixture-view'}:undefined,expectedUid:1000});
  assert.equal(command.workdir,'/home/dev/work');const source=command.cmd.split("node <<'LUCI_CONTINUATION_NODE'\n")[1].split('\nLUCI_CONTINUATION_NODE')[0];new Script(source);
  for(const value of ['c.baseCommit','manifest.version,2','packet.previousAttempt','c.answer','c.partialFiles','EROFS','LUCI_CONTINUATION_OBSERVATION','/home/dev/.luci-task-output',"git('ls-files'",'verify.mjs'])assert(source.includes(value));
 }
 for(const override of [{variant:'extra'},{operation:'../escape'},{expectedUid:0},{baseCommit:'HEAD'},{dependencyDigest:'bad'}])assert.throws(()=>taskCommand({variant:'partial',operation:'op',projectId:'p',runId:'r',baseCommit:'a'.repeat(40),contextHashes:{},dependencyDigest:'c'.repeat(64),expectedUid:1000,...override}));
});
test('first oracle fails before work then passes only exact partial result',t=>{
 const dir=scratch(t);assert.notEqual(verify(dir,'partial').status,0);apply(dir,partialFiles);const r=verify(dir,'partial');assert.equal(r.status,0,r.stderr);assert.equal(r.stdout,'CONTINUATION_CHECK_OK partial\n');assert.notEqual(verify(dir,'finish').status,0);
 writeFileSync(join(dir,'committed.mjs'),'export const partial = 18;\n');assert.notEqual(verify(dir,'partial').status,0);
});
test('successor and final oracle require preserved note, correct phase and exact result bytes',t=>{
 const dir=scratch(t);apply(dir,finalFiles);for(const mode of ['finish','final']){const r=verify(dir,mode);assert.equal(r.status,0,r.stderr);assert.equal(r.stdout,'CONTINUATION_CHECK_OK '+mode+'\n');}
 for(const [file,bad] of [['note.txt','lost note\n'],['work.mjs',partialFiles['work.mjs']],['result.json','{"partial":17,"remaining":25,"sum":43}\n'],['result.json','{"partial":17,"remaining":25,"sum":42}']]){apply(dir,finalFiles);writeFileSync(join(dir,file),bad);assert.notEqual(verify(dir,'final').status,0);}
 assert.notEqual(verify(dir,'unknown').status,0);
});
test('reports distinguish unresolved agent question and later claims, not recorded verification',()=>{
 const a=taskHandoff('partial'),b=taskHandoff('finish');assert.equal(a.questions.length,1);assert.equal(a.questions[0].question,question);assert.equal(a.questions[0].blocking,true);assert.deepEqual(b.questions,[]);
 for(const [key,r] of [['partial',a],['finish',b]]){assert.deepEqual(r.changedFiles,changedPaths(key));assert.equal(r.reportedChecks[0].command,'node verify.mjs '+key);assert.equal(r.recordedChecks,undefined);assert.equal(r.integration,undefined);}
});
