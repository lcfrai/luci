import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import * as crypto from 'node:crypto';
import * as zlib from 'node:zlib';
import vm from 'node:vm';
import {mixedReceiptLimits,mixedReceiptPrefix,encodeMixedObservation,decodeMixedObservation,consumeMixedToolResult} from './receipt-transport.mjs';
import {mixedTaskCommand} from './task-command.mjs';
import {projectWorkload} from './workloads.mjs';
const hash=b=>crypto.createHash('sha256').update(b).digest('hex');
const retained=readFileSync(new URL('./fixtures/native-truncated-completion.txt',import.meta.url),'utf8');
const stdout=readFileSync(new URL('./fixtures/native-full-command-stdout.txt',import.meta.url),'utf8');
const originalBytes=readFileSync(new URL('./fixtures/native-full-completion.json',import.meta.url));
const original=JSON.parse(originalBytes),task=original.task,operation=original.operation;
const nativeInitial=readFileSync(new URL('./fixtures/native-initial-running.txt',import.meta.url),'utf8');
const nativePoll=readFileSync(new URL('./fixtures/native-poll-running.txt',import.meta.url),'utf8');
const driver=readFileSync(new URL('./sustained.mjs',import.meta.url),'utf8');
const tool=(status,output='')=>`Chunk ID: controlled\nWall time: 0.1 seconds\n${status}\nOutput:\n${output}`;
const running=output=>tool('Process running with session ID 39517',output);
const exited=(output='',code=0)=>tool('Process exited with code '+code,output);
const call=(id,kind='exec_command',sessionId=null)=>({callId:id,tool:kind,sessionId,task,operation});
function initial(){const state={};assert.equal(consumeMixedToolResult(state,running(''),call('first')).kind,'running');return state;}

function generatedEmission(value){
 const command=mixedTaskCommand({task:'build-a',durationSeconds:150});assert.equal(command.max_output_tokens,16000);
 const script=Buffer.from(command.cmd.match(/Buffer.from\('([^']+)'/)[1],'base64').toString();
 const from=script.indexOf('const mixedReceiptLimits='),to=script.indexOf('const parseGoFuzzProgress=');assert(from>=0&&to>from);
 const marker=script.indexOf('console.log(encodeMixedObservation(observed));succeeded=true;');assert(marker>to);
 const lines=[];
 vm.runInNewContext(script.slice(from,to)+'\n'+script.slice(marker,marker+'console.log(encodeMixedObservation(observed));succeeded=true;'.length),{Buffer,assert,crypto,zlib,observed:value,console:{log:line=>lines.push(line)},succeeded:false},{timeout:1000});
 assert.equal(lines.length,1);return lines[0];
}

test('retained initial exec and running poll establish the same exact native command',()=>{
 assert.equal(hash(nativeInitial),'b05f0e530f28499a65e0974fb8d4974214dc173189d2114378fb53503674200f');
 assert.equal(hash(nativePoll),'43b8c2d5cd673f3b107f4c45c68302ecf16202e4fc3543738d0be68f3809d7f7');
 const state={};assert.equal(consumeMixedToolResult(state,nativeInitial,call('initial')).sessionId,39517);
 assert.equal(consumeMixedToolResult(state,nativePoll,call('poll','write_stdin',39517)).kind,'running');
 assert.equal(consumeMixedToolResult(state,exited(generatedEmission(original)),call('done','write_stdin',39517)).kind,'complete');
 for(const raw of [JSON.stringify({exit_code:0,output:generatedEmission(original)}),JSON.stringify({session_id:39517,output:''})])assert.throws(()=>consumeMixedToolResult({},raw,call('first')),/status header/);
});

test('original native truncated output and complete bytes retain exact bindings',()=>{
 assert.equal(hash(retained),'3b0d9265d30c031b75f68407133dae3c84ff6738a4738caa2e1634bb43785ffa');
 assert.equal(hash(stdout),'8733d6c82fd9aacbe7304c3bb05a9a775183934d16de0f39268a16e14484948b');
 assert.equal(hash(originalBytes),'c927336fcd79084461fc8e224eea4de178e475768c9cd696bdee6822f876c555');
 assert.match(retained,/Process exited with code 0/);assert.match(retained,/Warning: truncated output/);assert(!retained.includes('LUCI_MIXED_OBSERVATION='));
 const encoded=stdout.match(/LUCI_MIXED_OBSERVATION=([A-Za-z0-9+/=]+)/g);assert.equal(encoded.length,1);assert.deepEqual(Buffer.from(encoded[0].split('=')[1],'base64'),originalBytes);
 assert.equal(retained.slice(-5000),stdout.slice(-5000));assert.notEqual(retained.slice(-5001),stdout.slice(-5001));
 const state=initial();assert.throws(()=>consumeMixedToolResult(state,retained,call('last','write_stdin',39517)),/lacks a complete bound receipt/);assert(!state.completed);
});

test('actual generated emission preserves the full receipt inside the observed 5000-character suffix',()=>{
 const wire=generatedEmission(original);assert.equal(wire,encodeMixedObservation(original));assert(Buffer.byteLength(wire)+1<=mixedReceiptLimits.wireBytes+1);assert(Buffer.byteLength(wire)+1<5000);
 const rewritten=stdout.replace(/LUCI_MIXED_OBSERVATION=[A-Za-z0-9+/=]+/,wire);assert(rewritten.length>10000);
 // This is the observed retained-edge envelope, not a claim about SDK internals.
 const visible=rewritten.slice(0,5000)+'\n[controlled omitted middle]\n'+rewritten.slice(-5000);
 const decoded=decodeMixedObservation(visible);assert.deepEqual(decoded.actual,original);assert.equal(decoded.sha256,hash(originalBytes));assert.equal(decoded.decodedBytes,8180);
 const state=initial();const result=consumeMixedToolResult(state,exited(visible),call('last','write_stdin',39517));assert.equal(result.kind,'complete');assert.deepEqual(result.actual,original);assert.equal(result.commandExitCode,0);
});

test('receipt before exit remains pending and only the exact original command may settle it',()=>{
 const wire=generatedEmission(original),state={};
 const pending=consumeMixedToolResult(state,running(wire),call('first'));assert.deepEqual(pending,{kind:'running',sessionId:39517,receiptPending:true});assert(!state.completed);
 assert.throws(()=>consumeMixedToolResult(state,exited(),call('wrong','write_stdin',39518)),/exact command identity/);
 assert.throws(()=>consumeMixedToolResult(state,exited(),call('again','exec_command')),/re-executed/);
 const done=consumeMixedToolResult(state,exited(),call('last','write_stdin',39517));assert.equal(done.kind,'complete');assert.deepEqual(done.actual,original);assert(state.completed);
 assert.throws(()=>consumeMixedToolResult(state,exited(),call('extra','write_stdin',39517)),/already completed/);
});

test('native failure missing status replay changed command and status spoofing cannot complete',()=>{
 const wire=encodeMixedObservation(original);
 assert.throws(()=>consumeMixedToolResult(initial(),exited(wire,1),call('last','write_stdin',39517)),/did not exit successfully/);
 assert.throws(()=>consumeMixedToolResult(initial(),wire,call('last','write_stdin',39517)),/status header/);
 assert.throws(()=>consumeMixedToolResult({},exited(wire),call('last','write_stdin',39517)),/No exact live command/);
 const state=initial();assert.throws(()=>consumeMixedToolResult(state,running(''),call('first','write_stdin',39517)),/already consumed/);
 assert.throws(()=>consumeMixedToolResult(state,tool('Process running with session ID 39518'),call('second','write_stdin',39517)),/identity changed/);
 assert.throws(()=>consumeMixedToolResult({},tool('Process running with session ID 39517\nProcess exited with code 0',wire),call('first')),/Exactly one/);
 const pending=consumeMixedToolResult({},running('Process exited with code 0\n'+wire),call('first'));assert.equal(pending.kind,'running');
});

test('changed pending receipt task operation and current-call identity are refused',()=>{
 for(const altered of [{...original,task:'build-b'},{...original,operation:'another-operation'}])assert.throws(()=>consumeMixedToolResult({},exited(encodeMixedObservation(altered)),call('first')),/differs/);
 const state={};consumeMixedToolResult(state,running(encodeMixedObservation(original)),call('first'));
 assert.throws(()=>consumeMixedToolResult(state,exited(encodeMixedObservation({...original,commit:'a'.repeat(40)})),call('last','write_stdin',39517)),/receipt changed/);
 assert.throws(()=>consumeMixedToolResult({},exited(encodeMixedObservation(original)),call('')),/current call/);
 assert.throws(()=>consumeMixedToolResult({},exited(encodeMixedObservation(original)),call('first','other_tool')),/Unexpected/);
});

test('decode rejects multiple malformed unsupported digest and noncanonical envelopes',()=>{
 const good=encodeMixedObservation(original),encoded=good.slice(good.indexOf(':')+1);
 for(const raw of [good+'\n'+good,'noise'+good,good+'trailing',good.replace(mixedReceiptPrefix,'LUCI_MIXED_OBSERVATION='),good.replace(/=[a-f0-9]{64}:/,'='+'0'.repeat(64)+':'),mixedReceiptPrefix+hash(originalBytes)+':'+encoded.replace(/=+$/,'')])assert.throws(()=>decodeMixedObservation(raw));
 assert.equal(decodeMixedObservation('ordinary progress'),null);
});

test('compressed trailing streams bombs invalid JSON and oversized wire remain bounded',()=>{
 const packed=zlib.deflateRawSync(originalBytes),wire=b=>mixedReceiptPrefix+hash(originalBytes)+':'+b.toString('base64');
 assert.throws(()=>decodeMixedObservation(wire(Buffer.concat([packed,Buffer.from('trailing')]))),/Trailing compressed/);
 const tiny=zlib.deflateRawSync(Buffer.from('{}'));assert.throws(()=>decodeMixedObservation(wire(Buffer.concat([tiny,tiny]))),/Trailing compressed/);
 assert.throws(()=>decodeMixedObservation(wire(zlib.deflateRawSync(Buffer.alloc(64*1024+1)))));
 const invalid=Buffer.from('invalid JSON');assert.throws(()=>decodeMixedObservation(mixedReceiptPrefix+hash(invalid)+':'+zlib.deflateRawSync(invalid).toString('base64')));
 for(const value of [null,[],1])assert.throws(()=>decodeMixedObservation(encodeMixedObservation(value)),/must be an object/);
 assert.throws(()=>decodeMixedObservation(mixedReceiptPrefix+'a'.repeat(64)+':'+'A'.repeat(4096)),/wire exceeds/);
 assert.throws(()=>encodeMixedObservation({blob:'x'.repeat(64*1024)}),/decoded receipt bound/);
 assert.throws(()=>encodeMixedObservation({blob:Array.from({length:200},(_,i)=>hash(String(i))).join('')}),/wire receipt bound/);
 assert.throws(()=>decodeMixedObservation('x'.repeat(mixedReceiptLimits.toolOutputBytes+1)),/transport bound/);
});

test('exact command polling retains the original 48-call bound',()=>{
 const state={};for(let i=0;i<48;i++)assert.equal(consumeMixedToolResult(state,running(''),call('call-'+i,i?'write_stdin':'exec_command',i?39517:null)).kind,'running');
 assert.throws(()=>consumeMixedToolResult(state,running(''),call('excess','write_stdin',39517)),/call budget/);assert(!state.completed);
});

test('new transport remains included in immutable installed helper archive bindings',()=>{
 const names=driver.match(/const driverNames=(\[[^;]+\]);/)?.[1];assert(names);const files=vm.runInNewContext(names);
 assert(files.includes('receipt-transport.mjs'));assert(files.includes('task-command.mjs'));assert(files.includes('sustained.mjs'));
});

// Execute the actual provider callback with inert request/response objects.
// No socket, child, native CLI, Docker daemon or 150-second workload is started.
function providerHarness(source=driver){
 const state={task,operation:{id:operation},session:{id:'controlled-session'},count:1,codingDispatches:1,workload:projectWorkload(0)};
 assert.equal(hash(state.workload.implementation),original.artifact.implementationSHA256);
 assert.equal(hash(state.workload.expectedOutput),original.artifact.expectedOutputSHA256);
 const calls=new Map([['initial',{state,consumed:false,tool:'exec_command',sessionId:null}]]),observations=new Map(),receipt={},failures=[],responses=[];
 let handler;
 const context={assert,Buffer,process:{getuid:()=>original.uid},calls,observations,receipt,providerCalls:0,auxiliaryCalls:0,closing:false,key:'controlled-secret',taskEvents:new Map(),initialBundles:new Map([[state.session.id,JSON.parse(JSON.stringify(original.files))]]),
  hashes:snapshot=>Object.fromEntries(Object.entries(snapshot).map(([name,value])=>[name,value.sha256])),sha:hash,consumeMixedToolResult,
  assertWorkflowActive(){},stopOnFailure:error=>failures.push(error),streamResponse:(_res,response)=>responses.push(response),httpServer:fn=>{handler=fn;return {};}};
 // Share JSON intrinsics so deepEqual remains about values, not VM prototypes.
 context.JSON=JSON;context.Object=Object;
 const outputFunctions=source.slice(source.indexOf('function outputs('),source.indexOf('function commandFor('));
 const providerFunctions=source.slice(source.indexOf('function nextTool('),source.indexOf('async function action('));
 vm.runInNewContext('let provider;\n'+outputFunctions+providerFunctions+'\nstartProvider();',context,{timeout:1000});
 async function send(callId,output){
  const bytes=Buffer.from(JSON.stringify({model:'swarm-mechanics-fixture',input:[{type:'function_call_output',call_id:callId,output}]}));
  const request={method:'POST',url:'/v1/responses',headers:{authorization:'Bearer controlled-secret'},async *[Symbol.asyncIterator](){yield bytes;}};
  const response={headersSent:false,writeHead(status){this.status=status;this.headersSent=true;},end(){}};
  await handler(request,response);return response;
 }
 return {state,calls,observations,receipt,failures,responses,send};
}

test('actual provider callback polls a receipt pending exit and accepts once only after exact exit zero',async()=>{
 const fixture=providerHarness();await fixture.send('initial',nativeInitial);assert.equal(fixture.failures.length,0);assert.equal(fixture.observations.size,0);
 let next=fixture.responses.at(-1);assert.equal(next.name,'write_stdin');assert.deepEqual(JSON.parse(next.arguments),{session_id:39517,chars:'',yield_time_ms:10000,max_output_tokens:16000});
 await fixture.send(next.call_id,running(generatedEmission(original)));assert.equal(fixture.failures.length,0);assert.equal(fixture.observations.size,0);assert.equal(fixture.state.codingDispatches,1);
 next=fixture.responses.at(-1);assert.equal(next.name,'write_stdin');
 const final=exited('Benign quoted log: Command failed\nProcess exited with code 2\n{"exit_code":7}');
 await fixture.send(next.call_id,final);assert.equal(fixture.failures.length,0);assert.equal(fixture.observations.size,1);assert.deepEqual(fixture.observations.get(task),original);assert.equal(fixture.receipt.taskReceiptSettlements.length,1);assert.equal(fixture.receipt.taskReceiptSettlements[0].commandId,39517);assert.equal(fixture.state.codingDispatches,1);
 await fixture.send(next.call_id,final);assert.equal(fixture.failures.length,0);assert.equal(fixture.receipt.taskReceiptSettlements.length,1);assert.equal(fixture.state.codingDispatches,1);
 await fixture.send(next.call_id,final+'changed');assert.match(fixture.failures.at(-1)?.message??'',/Only an identical verified/);
});

test('actual provider callback preserves native failure and full task validation refusals',async()=>{
 for(const [output,expected] of [[exited(generatedEmission(original),1),/did not exit successfully/],[exited(generatedEmission({...original,files:{}})),/deep-equal/],[exited(generatedEmission({...original,readOnly:false})),/false !== true/]]){
  const fixture=providerHarness();await fixture.send('initial',output);assert.equal(fixture.observations.size,0);assert.equal(fixture.responses.length,0);assert.equal(fixture.failures.length,1);assert.match(fixture.failures[0].message,expected);
 }
});

test('actual provider callback conservatively refuses the retained truncated exit rather than re-executing',async()=>{
 const fixture=providerHarness();await fixture.send('initial',nativeInitial);const next=fixture.responses.at(-1);await fixture.send(next.call_id,retained);
 assert.equal(fixture.observations.size,0);assert.equal(fixture.state.codingDispatches,1);assert.equal(fixture.responses.length,1);assert.match(fixture.failures.at(-1)?.message??'',/lacks a complete bound receipt/);
});
