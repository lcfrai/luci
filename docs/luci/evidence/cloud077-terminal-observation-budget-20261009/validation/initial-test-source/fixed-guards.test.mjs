// Fixed-only behavioral guards; preparation does not imply these have run.
import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync,statSync} from 'node:fs';
import {loadVariant,VARIANT,controlledMonitor,privateCapture,assertHealthy,hasObservationFailure} from './controlled-fixture.mjs';

const {scenario,launch}=await loadVariant();
const fixed={skip:VARIANT!=='fixed'};

test('C077 fixed: launcher and scenario publish the same finite two-generation budgets',fixed,()=>{
  const limits=scenario.terminalObservationLimits();
  assert.deepEqual(launch.terminalObservationLimits(),limits);
  assert.deepEqual(scenario.TERMINAL_OBSERVATION_LIMITS,limits);
  assert.deepEqual(launch.TERMINAL_OBSERVATION_LIMITS,limits);
  assert.equal(limits.attemptsPerGeneration,34);
  assert.equal(limits.lastEarliestAttemptMs,61750);
  assert.equal(limits.generations,2);
  assert.equal(limits.maxSockets,68);
  assert.equal(limits.maxBrowserMutations,70);
});

async function endStream(fixture,code){
  const socket=fixture.create();fixture.open(socket);
  fixture.message(socket,3,'exit status '+code);socket.close(1000);
  const observed=await fixture.snapshot();assertHealthy(observed);
  assert(observed.browser.rows.at(-1).closed,'Native close observation must settle before the declared Restart boundary');
  return socket;
}

test('C077 fixed: exactly 34 attempts in each of two declared ladders remain observable',fixed,async()=>{
  const fixture=await controlledMonitor(scenario,launch);
  try{
    for(let i=0;i<33;i++)fixture.refuse(fixture.create());
    await endStream(fixture,0);
    const boundary=await fixture.monitor.restartBoundary();assert.equal(boundary.attemptCount,34);
    for(let i=0;i<33;i++)fixture.refuse(fixture.create());
    await endStream(fixture,7);
    const observed=await fixture.snapshot();assertHealthy(observed);
    assert.equal(observed.browser.rows.length,68);
    assert.equal(observed.protocol.rows.length,68);assert.equal(observed.sent.length,68);
    const streams=scenario.terminalGenerationStreams(observed);
    assert.deepEqual(streams.map(row=>row.ordinal),[34,68]);
    assert.deepEqual(streams.map(row=>row.exit.map(exit=>exit.reason.code)),[[0],[7]]);
    assert.equal(fixture.privateErrors.capture.close().passed,true);
  }finally{await fixture.dispose();}
});

test('C077 fixed: actual scenario selector rejects stdin attributed to a refused handshake',fixed,async()=>{
  const fixture=await controlledMonitor(scenario,launch);
  try{
    fixture.refuse(fixture.create());
    const observed=await fixture.snapshot();assertHealthy(observed);
    assert.deepEqual(scenario.terminalGenerationStreams(observed),[]);
    const invalid=structuredClone(observed);
    invalid.sent.find(row=>row.ordinal===invalid.browser.rows[0].ordinal).stdinFrames=1;
    assert.throws(()=>scenario.terminalGenerationStreams(invalid),/Refused handshake carried stdin/);
  }finally{await fixture.dispose();}
});

test('C077 fixed: an attempt after typed Exit and before admitted Restart fails',fixed,async()=>{
  const fixture=await controlledMonitor(scenario,launch);
  try{
    await endStream(fixture,0);
    fixture.refuse(fixture.create());
    const observed=await fixture.snapshot();
    assert(hasObservationFailure(observed));
    assert(observed.browser.failures.includes('attach-after-typed-exit'));
  }finally{await fixture.dispose();}
});

test('C077 fixed: an extra third successful stream fails independently of total-attempt capacity',fixed,async()=>{
  const fixture=await controlledMonitor(scenario,launch);
  try{
    await endStream(fixture,0);await fixture.monitor.restartBoundary();
    const second=fixture.create();fixture.open(second);
    const third=fixture.create();fixture.open(third);
    const observed=await fixture.snapshot();
    assert.equal(observed.browser.rows.length,3);
    assert(hasObservationFailure(observed));
    assert(observed.browser.failures.includes('extra-opened-stream'));
  }finally{await fixture.dispose();}
});

test('C077 fixed: socket 35 in one never-opened ladder fails before total capacity',fixed,async()=>{
  const fixture=await controlledMonitor(scenario,launch);
  try{
    for(let i=0;i<35;i++)fixture.refuse(fixture.create());
    const observed=await fixture.snapshot();
    assert(hasObservationFailure(observed));
    assert(observed.browser.failures.includes('generation-attempt-limit'));
  }finally{await fixture.dispose();}
});

test('C077 fixed: socket 69 fails and all three retained attempt arrays stay bounded',fixed,async()=>{
  const fixture=await controlledMonitor(scenario,launch);
  try{
    for(let i=0;i<33;i++)fixture.refuse(fixture.create());
    await endStream(fixture,0);await fixture.monitor.restartBoundary();
    for(let i=0;i<34;i++)fixture.refuse(fixture.create());
    assertHealthy(await fixture.snapshot());
    fixture.refuse(fixture.create());
    const observed=await fixture.snapshot();
    assert(hasObservationFailure(observed));
    assert.equal(observed.browser.rows.length,68);
    assert.equal(observed.sent.length,68);assert.equal(observed.protocol.rows.length,68);
    assert(observed.browser.failures.includes('socket-limit'));
    assert(observed.protocol.failures.includes('cdp-socket-limit'));
  }finally{await fixture.dispose();}
});

test('C077 fixed: Restart declaration requires a settled first typed exit',fixed,async()=>{
  const fixture=await controlledMonitor(scenario,launch);
  try{
    const first=fixture.create();fixture.open(first);
    await assert.rejects(fixture.monitor.restartBoundary(),/typed exit|settlement/i);
    fixture.message(first,3,'exit status 0');first.close(1000);
    await fixture.snapshot();
    await fixture.monitor.restartBoundary();
    await assert.rejects(fixture.monitor.restartBoundary(),/Only one/);
  }finally{await fixture.dispose();}
});

for(const sample of [
  {name:'abnormal close',code:1006,wasClean:false,exit:0,error:false},
  {name:'unclean normal-code close',code:1000,wasClean:false,exit:0,error:false},
  {name:'nonzero first typed Exit',code:1000,wasClean:true,exit:7,error:false},
  {name:'first-stream error observation',code:1000,wasClean:true,exit:0,error:true},
])test('C077 fixed: Restart boundary rejects '+sample.name,fixed,async()=>{
  const fixture=await controlledMonitor(scenario,launch);
  try{
    const first=fixture.create();fixture.open(first);
    if(sample.error)first.emit('error');
    fixture.message(first,3,'exit status '+sample.exit);
    first.readyState=3;first.emit('close',{code:sample.code,wasClean:sample.wasClean});
    fixture.cdp.emit('Network.webSocketClosed',{requestId:first.requestId});
    await fixture.snapshot();
    await assert.rejects(fixture.monitor.restartBoundary(),/typed exit|settlement/i);
  }finally{await fixture.dispose();}
});

for(const kind of [0,3])test('C077 fixed: an unopened refused socket cannot contribute '+(kind===0?'stdout':'typed Exit'),fixed,async()=>{
  const fixture=await controlledMonitor(scenario,launch);
  try{
    const socket=fixture.create();
    fixture.message(socket,kind,kind===0?'unopened fixture bytes':'exit status 0');
    fixture.refuse(socket);
    const observed=await fixture.snapshot();
    assert(hasObservationFailure(observed));
    assert(observed.browser.failures.includes('message-before-open'));
    assert.equal(observed.browser.rows[0].stdoutFrames,0);
    assert.equal(observed.browser.rows[0].exit.length,0);
  }finally{await fixture.dispose();}
});

test('C077 fixed: installed CDP and Playwright observers each stop at eight errors per socket',fixed,async()=>{
  const fixture=await controlledMonitor(scenario,launch);
  try{
    const socket=fixture.create();
    for(let i=0;i<9;i++)fixture.errors(socket,'controlled observer error '+i);
    socket.close(1006);
    const observed=await fixture.snapshot();
    assert(hasObservationFailure(observed));
    assert.equal(observed.protocol.rows[0].errors.length,8);
    assert.equal(observed.sent[0].socketErrors.length,8);
    assert(observed.protocol.failures.includes('cdp-error-limit'));
    assert.equal(observed.observerFailure,'socket-error-limit');
    assert.equal(fixture.privateErrors.capture.snapshot().rows,16);
  }finally{await fixture.dispose();}
});

test('C077 fixed: per-channel per-socket error limit remains eight',fixed,()=>{
  const fixture=privateCapture(launch);
  try{
    for(let i=0;i<8;i++)fixture.capture.record({channel:'cdp',ordinal:68,error:'controlled '+i});
    const before=fixture.capture.snapshot();
    assert.throws(()=>fixture.capture.record({channel:'cdp',ordinal:68,error:'ninth controlled error'}),/limit/i);
    const after=fixture.capture.snapshot();
    assert.equal(after.rows,8);assert.equal(after.bytes,before.bytes);
    assert(after.failures.includes('private_socket_error_limit'));
  }finally{fixture.dispose();}
});

test('C077 fixed: error ordinal 69 is rejected without private-file growth',fixed,()=>{
  const fixture=privateCapture(launch);
  try{
    assert.throws(()=>fixture.capture.record({channel:'cdp',ordinal:69,error:'controlled'}),/invalid/i);
    assert.equal(fixture.capture.snapshot().rows,0);assert.equal(statSync(fixture.path).size,0);
  }finally{fixture.dispose();}
});

test('C077 fixed: all 1088 permitted private error slots fit and an additional row fails',fixed,()=>{
  const fixture=privateCapture(launch);
  try{
    for(let ordinal=1;ordinal<=68;ordinal++)for(const channel of ['cdp','playwright'])for(let i=0;i<8;i++){
      fixture.capture.record({channel,ordinal,error:'controlled error'});
    }
    const before=fixture.capture.snapshot(),bytes=readFileSync(fixture.path);
    assert.equal(before.rows,1088);assert.equal(before.maxRows,1088);
    assert.equal(before.maxBytes,14483456);assert.equal(before.maxRowBytes,13312);
    assert.equal(before.bytes,bytes.length);assert(bytes.length<=before.maxBytes);
    const lines=bytes.toString('utf8').trimEnd().split('\n');assert.equal(lines.length,1088);
    assert(lines.every(line=>Buffer.byteLength(line+'\n')<=before.maxRowBytes));
    assert.throws(()=>fixture.capture.record({channel:'cdp',ordinal:68,error:'overflow'}),/limit/i);
    const after=fixture.capture.snapshot();assert.equal(after.rows,1088);assert.equal(after.bytes,before.bytes);
    assert.equal(statSync(fixture.path).size,bytes.length);
  }finally{fixture.dispose();}
});

test('C077 fixed: maximum escaped text stays within encoded-row/file limits and 2049 raw bytes fail',fixed,()=>{
  const fixture=privateCapture(launch),text='\u0000'.repeat(2048);
  try{
    const receipt=fixture.capture.record({channel:'playwright',ordinal:68,error:text});
    const before=fixture.capture.snapshot(),bytes=readFileSync(fixture.path);
    assert.equal(receipt.errorBytes,2048);assert.equal(JSON.parse(bytes.toString('utf8')).error,text);
    assert(bytes.length>6*2048);assert(bytes.length<=13312);assert.equal(before.maxTextBytes,2048);
    assert.equal(before.bytes,bytes.length);assert(before.bytes<=before.maxBytes);
    assert.throws(()=>fixture.capture.record({channel:'cdp',ordinal:68,error:'x'.repeat(2049)}),/limit/i);
    const after=fixture.capture.snapshot();assert(after.failures.includes('private_error_text_limit'));
    assert.equal(after.rows,1);assert.equal(after.bytes,before.bytes);assert.equal(statSync(fixture.path).size,bytes.length);
  }finally{fixture.dispose();}
});

test('C077 fixed: text limit counts UTF-8 bytes, not JavaScript characters',fixed,()=>{
  const fixture=privateCapture(launch);
  try{
    assert.equal(fixture.capture.record({channel:'cdp',ordinal:1,error:'é'.repeat(1024)}).errorBytes,2048);
    const before=fixture.capture.snapshot();
    assert.throws(()=>fixture.capture.record({channel:'cdp',ordinal:2,error:'é'.repeat(1025)}),/limit/i);
    assert.equal(fixture.capture.snapshot().bytes,before.bytes);
  }finally{fixture.dispose();}
});
