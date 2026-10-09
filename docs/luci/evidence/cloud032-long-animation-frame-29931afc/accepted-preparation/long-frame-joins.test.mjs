import test from 'node:test';
import assert from 'node:assert/strict';
import {browserLimits,createLongFrameLedger} from './long-frame-observer.mjs';
import {joinLongFrames} from './long-frame-joins.mjs';
const frame=(sequence,startTime,duration)=>({sequence,startTime,duration,blockingDuration:0,renderStart:0,styleAndLayoutStart:0,firstUIEventTimestamp:0,scripts:[]});
function capture(){
  const receipt={},ledger=createLongFrameLedger(receipt);
  ledger.append({version:1,timeOrigin:1000,limits:browserLimits,supported:true,status:'observing',failureCode:null,wired:true,closed:false,observed:3,retained:3,scripts:0,bytes:900,dropped:0,lateCallbacks:0,callbacks:1,callbackMs:0.2,rows:[frame(1,10,60),frame(2,105,80),frame(3,500,60)],snapshotAt:600,visibility:'visible'},1000);
  return receipt.longAnimationFrameCapture;
}
const click=(stage,start,ready)=>({switchIndex:0,round:0,projectOrdinal:0,stage,timeOrigin:1000,status:'matched',candidateCount:1,trustedCaptureListenerAt:start+3,observedReadyAt:ready,event:{timeOrigin:1000,name:'click',startTime:start,duration:40,processingStart:start+2,processingEnd:start+8}});
test('joins exact browser-clock click/ready/pre-Open intervals and retains unrelated frames',()=>{
  const output=joinLongFrames(capture(),{rows:[click('home',20,60),click('define',150,190)]},2);
  assert.equal(output.allFrames.length,3);assert.deepEqual(output.joins[0].intervals.reportedClick.frameIDs,['0:1']);assert.deepEqual(output.joins[1].intervals.reportedClick.frameIDs,['0:2']);
  assert.deepEqual(output.joins[1].intervals.homeObservedReadyToOpenClick.frameIDs,['0:1','0:2']);assert(!JSON.stringify(output).includes('nodeClickCommandMs'));
});
test('never joins another document and marks incomplete snapshot coverage explicitly',()=>{
  const row=click('home',20,60);row.timeOrigin=2000;row.event.timeOrigin=2000;
  const result=joinLongFrames(capture(),{rows:[row]},1);assert.equal(result.joins[0].intervals.reportedClick.coverage,'unobserved-document');assert.deepEqual(result.joins[0].intervals.reportedClick.frameIDs,[]);
  const late=joinLongFrames(capture(),{rows:[click('home',700,750)]},1);assert.equal(late.joins[0].intervals.reportedClick.coverage,'after-final-snapshot');
});
test('missing original join is retained and ambiguous or wrong EventTiming identity refuses attribution',()=>{
  const missing={...click('home',20,60),status:'unavailable',candidateCount:0};assert.equal(joinLongFrames(capture(),{rows:[missing]},1).joins[0].status,'unavailable');
  for(const changed of [{candidateCount:2},{event:{...missing.event,timeOrigin:999}},{trustedCaptureListenerAt:1000}])assert.throws(()=>joinLongFrames(capture(),{rows:[{...click('home',20,60),...changed}]},1));
});
test('invalid capture, incomplete cohort, duplicate identity and reversed ready interval refuse derivation',()=>{
  assert.throws(()=>joinLongFrames({...capture(),valid:false},{rows:[]},0));
  assert.throws(()=>joinLongFrames(capture(),{rows:[click('home',20,60)]}));
  assert.throws(()=>joinLongFrames(capture(),{rows:[click('home',20,60),click('home',20,60)]},2));
  assert.throws(()=>joinLongFrames(capture(),{rows:[click('home',20,10)]},1));
});
