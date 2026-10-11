// Identical assertions and stimulus run against C077_VARIANT=before or fixed.
// Prepared only; no test, import, or syntax check has been executed at authoring.
import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync,statSync} from 'node:fs';
import {loadVariant,controlledMonitor,privateCapture,fakePage,assertHealthy,BASE,ORIGIN,ATTACH,MARKERS} from './controlled-fixture.mjs';

const {scenario,launch}=await loadVariant();

test('C077 same-body positive control: two normal Exit 0/7 streams with no refused attempts',async()=>{
  const fixture=await controlledMonitor(scenario,launch);
  try{
    const initial=fixture.create();fixture.open(initial);
    fixture.message(initial,3,'exit status 0');initial.close(1000);
    const first=await fixture.snapshot();assertHealthy(first);
    assert.equal(first.browser.rows.length,1);
    assert.deepEqual(first.browser.rows[0].exit.map(row=>row.reason),[{kind:'exit-status',code:0}]);
    assert.equal(first.browser.rows[0].closed.wasClean,true);
    await fixture.monitor.restartBoundary?.();
    const replacement=fixture.create();fixture.open(replacement);
    fixture.message(replacement,3,'exit status 7');replacement.close(1000);
    const observed=await fixture.snapshot();assertHealthy(observed);
    assert.equal(observed.browser.rows.length,2);
    assert.equal(observed.protocol.rows.length,2);assert.equal(observed.sent.length,2);
    assert(observed.browser.rows.every(row=>row.open&&row.closed.code===1000&&row.closed.wasClean&&!row.error));
    assert.deepEqual(observed.browser.rows.map(row=>row.exit.map(exit=>exit.reason.code)),[[0],[7]]);
    assert.equal(observed.protocol.rows.filter(row=>row.handshakes.some(handshake=>handshake.status===101)).length,2);
    assert.equal(fixture.privateErrors.capture.snapshot().rows,0);
    assert.equal(fixture.privateErrors.capture.close().passed,true);
  }finally{await fixture.dispose();}
});

test('C077 same-body: first Exit 0, one Restart boundary, ten refused handshakes, replacement Exit 7',async()=>{
  const fixture=await controlledMonitor(scenario,launch);
  try{
    const initial=fixture.create();fixture.open(initial);
    fixture.message(initial,0,MARKERS.first+'\n');
    fixture.message(initial,3,'exit status 0');initial.close(1000);
    const first=await fixture.snapshot();assertHealthy(first);
    assert.equal(first.browser.rows.length,1);
    assert.deepEqual(first.browser.rows[0].exit.map(row=>row.reason),[{kind:'exit-status',code:0}]);
    assert.equal(first.browser.rows[0].closed.wasClean,true);

    // The old export has no declaration API; all transport stimulus and outcome
    // assertions remain the same on both variants. The declaration is passive.
    await fixture.monitor.restartBoundary?.();
    for(let i=0;i<10;i++)fixture.refuse(fixture.create());
    const replacement=fixture.create();fixture.open(replacement);
    fixture.message(replacement,0,MARKERS.second+'\n');
    fixture.message(replacement,3,'exit status 7');replacement.close(1000);
    const observed=await fixture.snapshot();
    assertHealthy(observed);
    assert.equal(observed.browser.rows.length,12);
    assert.equal(observed.protocol.rows.length,12);
    assert.equal(observed.sent.length,12);
    const attached=observed.browser.rows.filter(row=>row.open);
    assert.deepEqual(attached.map(row=>row.ordinal),[1,12]);
    assert.deepEqual(attached.map(row=>row.exit.map(exit=>exit.reason.code)),[[0],[7]]);
    assert(attached.every(row=>row.closed.code===1000&&row.closed.wasClean));
    assert(observed.browser.rows.slice(1,11).every(row=>row.open===null&&row.exit.length===0&&row.stdoutFrames===0));
    assert.equal(observed.protocol.rows.filter(row=>row.handshakes.some(handshake=>handshake.status===101)).length,2);
    assert.equal(fixture.privateErrors.capture.snapshot().rows,20);
    assert.equal(fixture.privateErrors.capture.close().passed,true);
  }finally{await fixture.dispose();}
});

test('C077 same-body: exported native observer alone retains twelve bounded attempts',async()=>{
  const fixture=fakePage();
  await fixture.page.addInitScript(scenario.terminalMonitor,{origin:ORIGIN,attachPath:ATTACH,markers:MARKERS,limits:scenario.TERMINAL_OBSERVATION_LIMITS});
  for(let i=0;i<12;i++)fixture.refuse(fixture.create());
  await new Promise(resolve=>setImmediate(resolve));
  const observed=await fixture.page.evaluate(()=>globalThis.__luciC054Terminal());
  assert.deepEqual(observed.failures,[]);
  assert.equal(observed.rows.length,12);
  assert(observed.rows.every(row=>row.open===null&&row.exit.length===0));
});

test('C077 same-body: exported CDP observer independently retains twelve refused attempts',async()=>{
  const fixture=fakePage(),privateErrors=privateCapture(launch);
  const diagnostics=await scenario.installTransportDiagnostics(fixture.page,{origin:ORIGIN,attachPath:ATTACH,retainTransportError:row=>privateErrors.capture.record(row)});
  try{
    for(let ordinal=1;ordinal<=12;ordinal++){
      const requestId='direct-cdp-'+ordinal;
      fixture.cdp.emit('Network.webSocketCreated',{requestId,url:ORIGIN+ATTACH});
      fixture.cdp.emit('Network.webSocketHandshakeResponseReceived',{requestId,response:{status:409,headers:{}}});
      fixture.cdp.emit('Network.webSocketFrameError',{requestId,errorMessage:'net::ERR_CONNECTION_CLOSED controlled fixture'});
      fixture.cdp.emit('Network.webSocketClosed',{requestId});
    }
    const observed=diagnostics.snapshot();
    assert.equal(observed.setupComplete,true);assert.deepEqual(observed.failures,[]);
    assert.equal(observed.rows.length,12);
    assert(observed.rows.every(row=>row.handshakes[0].status===409&&row.closed));
    assert.equal(privateErrors.capture.close().passed,true);
  }finally{await diagnostics.dispose();privateErrors.dispose();}
});

test('C077 same-body: private error ordinal nine is retained without exposing text in its receipt',()=>{
  const fixture=privateCapture(launch),secret='controlled-only error sentinel';
  try{
    const receipt=fixture.capture.record({channel:'cdp',ordinal:9,error:secret});
    assert.equal(receipt.record,1);assert.equal(receipt.errorBytes,Buffer.byteLength(secret));
    assert.equal(receipt.retainedPrivately,true);
    assert(!JSON.stringify(receipt).includes(secret));
    assert.deepEqual(JSON.parse(readFileSync(fixture.path,'utf8')).error,secret);
    assert.equal(fixture.capture.close().passed,true);
    assert.equal(statSync(fixture.path).mode&0o777,0o600);
  }finally{fixture.dispose();}
});

test('C077 same-body: forty private errors fit while each channel/socket stays below eight',()=>{
  const fixture=privateCapture(launch);
  try{
    for(let i=0;i<40;i++)fixture.capture.record({channel:i%2?'playwright':'cdp',ordinal:1+Math.floor(i/2)%8,error:'controlled error '+i});
    const observed=fixture.capture.close();
    assert.equal(observed.rows,40);assert.equal(observed.passed,true);
    assert.equal(readFileSync(fixture.path,'utf8').trimEnd().split('\n').length,40);
  }finally{fixture.dispose();}
});
