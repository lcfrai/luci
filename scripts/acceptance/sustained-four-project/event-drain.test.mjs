// Controlled instrumentation fixtures only. No Luci service, model or Docker.
import test from 'node:test';
import assert from 'node:assert/strict';
import {createTimingLedger,installBrowserTiming,timingSnapshot} from './interaction-metrics.mjs';
import {join} from 'node:path';
const windowFor=(sequences,extras={})=>({version:2,timeOrigin:10,events:sequences.map(sequence=>({sequence,name:'click',startTime:sequence,duration:16,processingStart:sequence,processingEnd:sequence+1,interactionId:sequence})),observedEvents:sequences.at(-1)??0,droppedEvents:0,overflow:false,unsupported:false,...extras});
test('drained document sequences remain complete and independent across navigation',()=>{
 const receipt={},ledger=createTimingLedger(receipt);ledger.append(windowFor([1,2]),'first');ledger.append(windowFor([3]),'second');ledger.append(windowFor([],{observedEvents:3}),'empty');ledger.append(windowFor([1],{timeOrigin:20}),'new-document');
 assert.equal(receipt.browserTimingLedger.events,4);assert.equal(receipt.browserTimingLedger.documents,2);assert(receipt.browserTimingLedger.sequenceVerified);
});
for(const [name,sequences] of [['gap',[3]],['duplicate',[1]]])test(name+' is retained as explicit invalid evidence',()=>{
 const receipt={},ledger=createTimingLedger(receipt);ledger.append(windowFor([1]),'first');assert.throws(()=>ledger.append(windowFor(sequences),'bad'),/gap or duplicate/);assert.equal(receipt.browserEventWindows.length,2);assert.equal(receipt.browserTimingLedger.sequenceVerified,false);
});
test('buffer overflow is retained before refusal and never reset into valid evidence',()=>{
 const receipt={},ledger=createTimingLedger(receipt);assert.throws(()=>ledger.append(windowFor([1,2],{observedEvents:3,droppedEvents:1,overflow:true}),'overflow'),/overflow/);assert.equal(receipt.browserEventWindows[0].droppedEvents,1);assert.equal(receipt.browserTimingLedger.sequenceVerified,false);
});
for(const [name,limit] of [['event', {maxEvents:1}],['byte',{maxBytes:1}],['window',{maxWindows:0}],['document',{maxDocuments:0}]])test(name+' ledger bound refuses before retaining excessive payload',()=>{
 const receipt={},ledger=createTimingLedger(receipt,{maxEvents:200000,maxBytes:32*1024**2,maxWindows:2000,maxDocuments:128,...limit});assert.throws(()=>ledger.append(windowFor([1,2]),'bounded'),/declared bound/);assert.equal(receipt.browserEventWindows.length,0);assert.equal(receipt.browserTimingLedger.events,0);assert.equal(receipt.browserTimingLedger.sequenceVerified,false);
});
test('actual browser collector drains over10000 controlled observer events in bounded batches',async()=>{
 const repo=process.env.LUCI_ACCEPTANCE_REPO;assert(repo);const {chromium}=await import(join(repo,'desktop/node_modules/playwright/index.mjs'));
 const browser=await chromium.launch({executablePath:'/usr/bin/chromium',headless:true,args:['--no-proxy-server']});
 try{
  const context=await browser.newContext();
  // Mock only the browser API's delivery boundary; these synthetic entries do
  // not represent user input timing or a measured app workload.
  await context.addInitScript(()=>{window.PerformanceObserver=class {static supportedEntryTypes=['event'];constructor(callback){window.__fixtureDeliver=rows=>callback({getEntries:()=>rows});}observe(){}};});
  await installBrowserTiming(context);const page=await context.newPage();
  const receipt={},ledger=createTimingLedger(receipt);
  for(let batch=0;batch<6;batch++){
   await page.evaluate(()=>window.__fixtureDeliver(Array.from({length:2000},(_,i)=>({name:'click',startTime:i,duration:16,processingStart:i,processingEnd:i+1,interactionId:i+1}))));
   const drained=await timingSnapshot(page);assert.equal(drained.events.length,2000);assert.equal(drained.overflow,false);ledger.append(drained,'controlled-batch');assert.equal(await page.evaluate(()=>window.__luciAcceptanceTiming.events.length),0);
  }
  assert.equal(receipt.browserTimingLedger.events,12000);assert(receipt.browserTimingLedger.sequenceVerified);assert.equal(receipt.browserEventWindows.at(-1).events.at(-1).sequence,12000);
  await page.evaluate(()=>window.__fixtureDeliver(Array.from({length:10001},(_,i)=>({name:'keydown',startTime:i,duration:16,processingStart:i,processingEnd:i+1,interactionId:i+1}))));
  const overflow=await timingSnapshot(page);assert.equal(overflow.events.length,10000);assert.equal(overflow.droppedEvents,1);assert.equal(overflow.overflow,true);assert.throws(()=>ledger.append(overflow,'controlled-overflow'),/overflow/);const after=await timingSnapshot(page);assert.equal(after.events.length,0);assert.equal(after.droppedEvents,1);assert.equal(after.overflow,true);
 }finally{await browser.close();}
});
