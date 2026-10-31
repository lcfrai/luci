'use strict';
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const crypto = require('node:crypto');
const { EventEmitter } = require('node:events');
const { PassThrough } = require('node:stream');
const adapter = require('./codex-task-result.cjs');
const native = require('./codex-native-task.cjs');
const hash = b => crypto.createHash('sha256').update(b).digest('hex');
const report = () => ({version:1,summary:'Implemented the saved acceptance path; independent checks remain separate.',changedFiles:['src/save.js'],decisions:[],reportedChecks:[{command:'npm test',outcome:'not-run',detail:'Not run in this attempt.'}],questions:[],limitations:[],addressedMessages:[]});

test('strict final JSON rejects duplicate keys, unknowns, malformed values and oversized output', () => {
  assert.equal(adapter.reportValid(adapter.strictJSON(Buffer.from(JSON.stringify(report())))),true);
  for (const bytes of [Buffer.from('{"version":1,"version":1}'),Buffer.from('{"nested":{"a":1,"a":2}}'),Buffer.from('{} trailing'),Buffer.from([0xff]),Buffer.alloc(65537,32),Buffer.from('{"x":NaN}'),Buffer.from('{"x":1e3,"x":2}')]) assert.throws(() => adapter.strictJSON(bytes));
  for (const change of [{extra:true},{version:2},{summary:''},{questions:null},{changedFiles:['../secret']},{addressedMessages:[{kind:'unknown',recipientTaskId:'task-a',causalMessageId:'',summary:'x'}]}]) assert.equal(adapter.reportValid({...report(),...change}),false);
  const coordinated=report(); coordinated.addressedMessages=[{kind:'review-finding',recipientTaskId:'task-a',causalMessageId:'',summary:'The saved edge case needs independent review.'}];
  assert.equal(adapter.reportValid(coordinated),true);
});

test('fixed output flags preserve opaque prompts and native helper policy', () => {
  const original=[...native.PREFIX,'exec','--json','--color','never','--dangerously-bypass-approvals-and-sandbox','--','opaque --output-schema is not an argument'];
  const args=adapter.invocation('/usr/local/bin/node',['/usr/local/lib/luci/codex-native-task.cjs',...original]);
  assert.equal(native.validArguments(args.slice(1)),true);
  const wrong=[...args.slice(1)];wrong[wrong.indexOf('--output-schema')+1]='/tmp/other';
  assert.equal(native.validArguments(wrong),false);
  assert.throws(()=>adapter.invocation('/usr/local/bin/codex',['exec','--output-schema','/tmp/other','--','prompt']));
  assert.doesNotThrow(()=>adapter.invocation('/usr/local/bin/codex',['exec','--','--output-schema']));
  assert.equal(adapter.commandDigest(['a','bc'])===adapter.commandDigest(['ab','c']),false);
  assert.equal(adapter.commandDigest(['<é>','x']),adapter.commandDigest(['<é>','x']));
});

function channels() {
  const child=new EventEmitter();child.stdout=new PassThrough();child.stderr=new PassThrough();child.kill=()=>true;
  const proc=new EventEmitter(),stdout=new PassThrough(),stderr=new PassThrough();stdout.resume();stderr.resume();
  return {child,proc,io:{spawn:()=>child,process:proc,stdout,stderr}};
}
test('original close alone does not publish before both stream EOFs',async()=>{
  const {child,proc,io}=channels();let finished=false;
  const result=adapter.settled('/usr/local/bin/codex',[],io).then(v=>{finished=true;return v;});
  child.emit('close',0,null);await Promise.resolve();assert.equal(finished,false);
  child.stdout.end();child.stderr.end();
  assert.equal(await result,true);assert.equal(proc.listenerCount('SIGTERM'),0);assert.equal(proc.listenerCount('SIGINT'),0);
});
test('nonzero, unknown exit, interrupted streams and cancellation are never terminal success',async()=>{
  for(const state of ['nonzero','unknown','stream-error','cancel']) {
    const {child,proc,io}=channels();const result=adapter.settled('/usr/local/bin/codex',[],io);
    if(state==='cancel')proc.emit('SIGINT');
    if(state==='stream-error')child.stdout.emit('error',Error('opaque'));
    child.stdout.end();child.stderr.end();child.emit('close',state==='nonzero'?1:state==='unknown'?null:0,state==='unknown'?'SIGTERM':null);
    assert.equal(await result,false,state);
  }
});

function fixture(t,behavior='success') {
  const dir=fs.mkdtempSync(path.join(os.tmpdir(),'luci-terminal-unit-'));t.after(()=>fs.rmSync(dir,{recursive:true,force:true}));
  const map=p=>typeof p==='string'&&p.startsWith('/')?path.join(dir,p.slice(1)):p;
  const ioFS={constants:fs.constants};
  for(const name of ['lstatSync','openSync','mkdirSync','existsSync'])ioFS[name]=(p,...args)=>fs[name](map(p),...args);
  for(const name of ['fstatSync','readSync','closeSync','writeFileSync','fsyncSync'])ioFS[name]=fs[name].bind(fs);
  const args=['exec','--color','never','--dangerously-bypass-approvals-and-sandbox','--','saved task'];
  const schema=fs.readFileSync(path.join(__dirname,'task-handoff.schema.json'));
  const contract={version:1,sessionId:'s-12345678',projectId:'p-a',runId:'run-a',operationId:'op-a',taskId:'task-a',worktreeId:'w-a',attempt:1,role:'coding',reviewOfTaskId:'',requestDigest:'a'.repeat(64),implementationDigest:'b'.repeat(64),contextDigest:'c'.repeat(64),sourceCommit:'d'.repeat(40),sourceDigest:'e'.repeat(64),sourceDirty:false,nativeDelegation:'',image:'luci/sandbox:dev',commandDigest:adapter.commandDigest(['/usr/local/bin/codex',...args]),schemaDigest:hash(schema)};
  for(const p of [adapter.CONTRACT,adapter.SCHEMA,adapter.OUTPUT+'/reserved'])fs.mkdirSync(path.dirname(map(p)),{recursive:true});
  const raw=Buffer.from(JSON.stringify(contract)+'\n');fs.writeFileSync(map(adapter.CONTRACT),raw);fs.writeFileSync(map(adapter.SCHEMA),schema);
  let starts=0;
  const proc=new EventEmitter(),stdout=new PassThrough(),stderr=new PassThrough();stdout.resume();stderr.resume();
  const io={fs:ioFS,process:proc,stdout,stderr,spawn:()=>{
    starts++;const child=new EventEmitter();child.stdout=new PassThrough();child.stderr=new PassThrough();child.kill=()=>true;
    queueMicrotask(()=>{
      if(behavior!=='missing')fs.writeFileSync(map(adapter.CANDIDATE),behavior==='malformed'?'{}':JSON.stringify(report()));
      child.stdout.end();child.stderr.end();child.emit('close',behavior==='failed'?1:0,null);
    });return child;
  }};
  return {io,argv:[hash(raw),'/usr/local/bin/codex',...args],map,starts:()=>starts};
}
test('valid final file publishes once and its receipt correlates exact raw bytes',async t=>{
  const f=fixture(t);assert.equal(await adapter.run(f.argv,f.io),0);
  const raw=fs.readFileSync(f.map(adapter.OUTPUT+'/handoff.json')),receipt=JSON.parse(fs.readFileSync(f.map(adapter.OUTPUT+'/terminal-receipt.json')));
  assert.equal(receipt.reportDigest,hash(raw));assert.equal(receipt.contractDigest,f.argv[0]);
  assert.equal(await adapter.run(f.argv,f.io),78);assert.equal(f.starts(),1);
});
test('failure, missing, malformed and existing output never overwrite or replay',async t=>{
  for(const state of ['failed','missing','malformed','existing']) {
    const f=fixture(t,state);
    if(state==='existing')fs.writeFileSync(f.map(adapter.OUTPUT+'/handoff.json'),'original');
    assert.equal(await adapter.run(f.argv,f.io),78,state);
    assert.equal(await adapter.run(f.argv,f.io),78,state);
    assert.equal(f.starts(),state==='existing'?0:1);
    if(state==='existing')assert.equal(fs.readFileSync(f.map(adapter.OUTPUT+'/handoff.json'),'utf8'),'original');
    else assert.equal(fs.existsSync(f.map(adapter.OUTPUT+'/handoff.json')),false);
  }
});
test('contract or original argument mismatch refuses before child creation',async t=>{
  const f=fixture(t);f.argv.at(-1);f.argv[f.argv.length-1]='another prompt';
  assert.equal(await adapter.run(f.argv,f.io),78);assert.equal(f.starts(),0);
});
