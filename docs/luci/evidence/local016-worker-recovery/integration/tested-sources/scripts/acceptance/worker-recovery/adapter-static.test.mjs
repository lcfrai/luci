import test from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFileSync} from 'node:fs';
import {main} from './installed-worker-recovery.mjs';
import {contextFileNames} from '../sustained-four-project/task-context.mjs';
const source=readFileSync(new URL('./installed-worker-recovery.mjs',import.meta.url),'utf8');
test('Import is inert and runtime requires an explicit window before any setup',async()=>{
 await assert.rejects(main([]),/Exclusive root runtime window required/);
 await assert.rejects(main(['--unexpected-option']),/assert/);
});
test('Installed adapter help exposes explicit product and image pins without runtime',async()=>{
 const original=console.log,lines=[];console.log=value=>lines.push(value);
 try{await main(['--help']);}finally{console.log=original;}
 assert.equal(lines.length,1);assert.match(lines[0],/--expected-source EXACT_SHA/);assert.match(lines[0],/--expected-helper-source EXACT_SHA/);assert.match(lines[0],/--expected-image-id sha256:DIGEST/);
});
test('Both actual generated native task commands parse without executing their processes',()=>{
 const start=source.indexOf('function commandFor(task)'),end=source.indexOf('\nfunction nextTool(',start);assert(start>0&&end>start);
 for(const task of ['build-a','build-b']){
  const taskData=new Map([[task,{operation:{id:'op-'+task},owner:{id:'p-'+task}}]]);
  const command=vm.runInNewContext(source.slice(start,end)+'\ncommandFor('+JSON.stringify(task)+')',{assert,Buffer,taskData,names:contextFileNames(2),authorityRoot:'/home/dev/.luci-reviewed-context'},{timeout:1000});
  assert.equal(command.workdir,'/home/dev/work');assert.equal(command.login,false);assert.equal(command.yield_time_ms,1000);
  const match=command.cmd.match(/Buffer.from\('([^']+)'/);assert(match);const native=Buffer.from(match[1],'base64').toString();assert.doesNotThrow(()=>new vm.Script(native));
 }
});
