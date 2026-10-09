import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import * as crypto from 'node:crypto';
import * as zlib from 'node:zlib';
import vm from 'node:vm';
import {mixedReceiptLimits,mixedReceiptPrefix,encodeMixedObservation,decodeMixedObservation,consumeMixedToolResult} from './receipt-transport.mjs';
import {mixedTaskCommand} from './task-command.mjs';
const hash=b=>crypto.createHash('sha256').update(b).digest('hex');
const retained=readFileSync(new URL('./fixtures/native-truncated-completion.txt',import.meta.url),'utf8');
const stdout=readFileSync(new URL('./fixtures/native-full-command-stdout.txt',import.meta.url),'utf8');
const originalBytes=readFileSync(new URL('./fixtures/native-full-completion.json',import.meta.url));
const original=JSON.parse(originalBytes),task=original.task,operation=original.operation;
const clone=structuredClone;
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
 assert.throws(()=>decodeMixedObservation(wire(Buffer.concat([packed,packed]))),/Trailing compressed/);
 assert.throws(()=>decodeMixedObservation(wire(zlib.deflateRawSync(Buffer.alloc(64*1024+1)))));
 for(const value of [null,[],1])assert.throws(()=>decodeMixedObservation(encodeMixedObservation(value)),/must be an object/);
 assert.throws(()=>decodeMixedObservation(mixedReceiptPrefix+'a'.repeat(64)+':'+'A'.repeat(4096)),/wire exceeds/);
 assert.throws(()=>encodeMixedObservation({blob:'x'.repeat(64*1024)}),/decoded receipt bound/);
 assert.throws(()=>encodeMixedObservation({blob:Array.from({length:200},(_,i)=>hash(String(i))).join('')}),/wire receipt bound/);
 assert.throws(()=>decodeMixedObservation('x'.repeat(mixedReceiptLimits.toolOutputBytes+1)),/transport bound/);
});
