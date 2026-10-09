// Controlled extraction of the actual old/new provider callbacks. No sockets,
// child processes, provider/model requests or native workload are started.
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFileSync} from 'node:fs';
import {createHash} from 'node:crypto';
import {projectWorkload} from '../../../../scripts/acceptance/sustained-four-project/workloads.mjs';
import {consumeMixedToolResult,encodeMixedObservation} from '../../../../scripts/acceptance/sustained-four-project/receipt-transport.mjs';
const hash=b=>createHash('sha256').update(b).digest('hex');
const read=path=>readFileSync(new URL(path,import.meta.url),'utf8');
const prefix='../../../../scripts/acceptance/sustained-four-project/';
const original=JSON.parse(read(prefix+'fixtures/native-full-completion.json'));
const initial=read(prefix+'fixtures/native-initial-running.txt');
const truncated=read(prefix+'fixtures/native-truncated-completion.txt');
const stdout=read(prefix+'fixtures/native-full-command-stdout.txt');
const tests=read('./after/receipt-transport.test.mjs');
const harness=tests.slice(tests.indexOf('function providerHarness('),tests.indexOf("test('actual provider callback polls"));
assert(harness.startsWith('function providerHarness('));
async function replay(source,final){
 const ctx={assert,Buffer,JSON,Object,vm,projectWorkload,consumeMixedToolResult,hash,original,task:original.task,operation:original.operation,driver:source};
 vm.runInNewContext(harness+'\nglobalThis.make=providerHarness;',ctx,{timeout:1000});
 const f=ctx.make();await f.send('initial',initial);assert.equal(f.failures.length,0);
 assert.equal(f.responses[0].name,'write_stdin');assert.equal(JSON.parse(f.responses[0].arguments).session_id,39517);
 await f.send(f.responses[0].call_id,final);
 return {accepted:f.observations.size,dispatches:f.state.codingDispatches,failures:f.failures.map(e=>e.message),settlements:f.receipt.taskReceiptSettlements??[]};
}
const before=read('./before/sustained.mjs'),after=read('./after/sustained.mjs');
const originalFailure=await replay(before,truncated);
assert.equal(originalFailure.accepted,0);assert.equal(originalFailure.dispatches,1);
assert.equal(originalFailure.failures.length,1);assert.match(originalFailure.failures[0],/Actual tool output must supply completion receipt or live command identity/);
const stillRefused=await replay(after,truncated);
assert.equal(stillRefused.accepted,0);assert.equal(stillRefused.dispatches,1);assert.match(stillRefused.failures[0],/lacks a complete bound receipt/);
const wire=encodeMixedObservation(original);
const rewritten=stdout.replace(/LUCI_MIXED_OBSERVATION=[A-Za-z0-9+/=]+/,wire);
assert.notEqual(stdout,rewritten);
const visible=rewritten.slice(0,5000)+'\n[controlled omitted middle]\n'+rewritten.slice(-5000);
const corrected=await replay(after,'Chunk ID: controlled\nWall time: 0.1 seconds\nProcess exited with code 0\nOutput:\n'+visible);
assert.equal(corrected.accepted,1);assert.equal(corrected.dispatches,1);assert.equal(corrected.failures.length,0);
console.log(JSON.stringify({issue:'CLOUD-041',scope:'Pure callback replay; corrected output uses actual generated receipt transport and observed edge envelope, not a native/installed rerun',inputs:{oldDriverSHA256:hash(before),newDriverSHA256:hash(after),harnessTestSHA256:hash(tests),initialSHA256:hash(initial),truncatedSHA256:hash(truncated),fullStdoutSHA256:hash(stdout)},observedTailCharacters:5000,actualReceiptBytes:Buffer.byteLength(JSON.stringify(original)),newWireBytes:Buffer.byteLength(wire),newWireAndNewlineBytes:Buffer.byteLength(wire)+1,originalFailure,originalStillConservativelyRefused:stillRefused,controlledCorrected:corrected,nativeSessionExitClaim:false},null,2));
