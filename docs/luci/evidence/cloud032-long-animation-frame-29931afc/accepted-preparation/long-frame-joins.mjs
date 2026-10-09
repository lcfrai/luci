import assert from 'node:assert/strict';
import {readFileSync,writeFileSync,statSync} from 'node:fs';
import {createHash} from 'node:crypto';
import {pathToFileURL} from 'node:url';
import {createLongFrameLedger} from './long-frame-observer.mjs';

// Consume the existing exact EventTiming/click join, never nearest timestamps.
// Every retained frame remains in the output; intervals reference those rows.
export function joinLongFrames(capture,clickProjection,expectedClicks=200){
  assert(capture?.valid===true&&capture.failureCode===null&&capture.status==='observing','Valid complete protocol capture required');
  assert(Array.isArray(capture.windows)&&capture.windows.length<=2000,'Bounded capture windows required');
  assert(Array.isArray(clickProjection.rows)&&clickProjection.rows.length===expectedClicks,'Exact original click cohort required');
  const validated={},ledger=createLongFrameLedger(validated),docs=new Map(),frames=[];
  for(const window of capture.windows){
    ledger.append(window,window.timeOrigin);
    if(!docs.has(window.timeOrigin))docs.set(window.timeOrigin,{ordinal:docs.size,lastSnapshotAt:window.snapshotAt});
    const doc=docs.get(window.timeOrigin);doc.lastSnapshotAt=window.snapshotAt;
    for(const frame of window.rows)frames.push({id:`${doc.ordinal}:${frame.sequence}`,timeOrigin:window.timeOrigin,...frame});
  }
  const known=new Map(),joins=[];
  for(const row of clickProjection.rows){
    assert(Number.isSafeInteger(row.switchIndex)&&row.switchIndex>=0&&Number.isSafeInteger(row.round)&&row.round>=0&&Number.isSafeInteger(row.projectOrdinal)&&row.projectOrdinal>=0&&row.projectOrdinal<4,'Invalid original click identity');
    assert(['home','define'].includes(row.stage),'Unexpected original navigation stage');
    const key=JSON.stringify([row.timeOrigin,row.round,row.projectOrdinal,row.stage]);assert(!known.has(key),'Duplicate click identity');known.set(key,row);
  }
  for(const row of clickProjection.rows){
    const joined={switchIndex:row.switchIndex,round:row.round,projectOrdinal:row.projectOrdinal,stage:row.stage,timeOrigin:row.timeOrigin,status:row.status,intervals:{}};
    if(row.status!=='matched'){joined.reason='Original exact click join unavailable';joins.push(joined);continue;}
    const event=row.event;
    assert(row.candidateCount===1&&event?.timeOrigin===row.timeOrigin&&event.name==='click'&&event.processingStart<=row.trustedCaptureListenerAt&&row.trustedCaptureListenerAt<=event.processingEnd,'Exact original EventTiming join required');
    assert([row.timeOrigin,row.trustedCaptureListenerAt,row.observedReadyAt,event.startTime,event.duration,event.processingStart,event.processingEnd].every(value=>Number.isFinite(value)&&value>=0),'Finite browser clock fields required');
    const intervals=[['reportedClick',event.startTime,event.startTime+event.duration],['clickToObservedReady',row.trustedCaptureListenerAt,row.observedReadyAt]];
    if(row.stage==='define'){
      const home=known.get(JSON.stringify([row.timeOrigin,row.round,row.projectOrdinal,'home']));
      if(home?.status==='matched')intervals.push(['homeObservedReadyToOpenClick',home.observedReadyAt,row.trustedCaptureListenerAt]);
      else joined.preOpenInterval='unavailable-home-join';
    }
    for(const [name,start,end] of intervals){
      assert(Number.isFinite(start)&&Number.isFinite(end)&&end>=start,'Invalid same-clock interval');
      const doc=docs.get(row.timeOrigin);
      joined.intervals[name]={start,end,coverage:!doc?'unobserved-document':doc.lastSnapshotAt<end?'after-final-snapshot':'snapshot-after-interval',lastSnapshotAt:doc?.lastSnapshotAt??null,frameIDs:frames.filter(frame=>frame.timeOrigin===row.timeOrigin&&frame.startTime<end&&frame.startTime+frame.duration>start).map(frame=>frame.id)};
    }
    joins.push(joined);
  }
  return {version:1,status:'derived',clicks:joins.length,frames:frames.length,documents:docs.size,joins,allFrames:frames,limits:'All original exact click joins and all retained frames remain. API eligibility is selective: no frame overlap is not proof of an idle renderer or absence of subthreshold work. Snapshot coverage is not a continuous rendering watermark. Script/forced-layout durations are reported wall intervals, not measured CPU. No Node clock is converted, no threshold is altered and no original time is subtracted.'};
}

if(process.argv[1]&&import.meta.url===pathToFileURL(process.argv[1]).href){
  const args=process.argv.slice(2),option=name=>{const i=args.indexOf(name);assert(i>=0&&args[i+1],name+' required');return args[i+1];};
  const resultPath=option('--result'),clicksPath=option('--clicks'),output=option('--output');
  assert(statSync(resultPath).size<=64*1024*1024&&statSync(clicksPath).size<=2*1024*1024,'Bounded analysis inputs required');
  const resultBytes=readFileSync(resultPath),clickBytes=readFileSync(clicksPath),sha=data=>createHash('sha256').update(data).digest('hex');
  const binding={rawResultSHA256:sha(resultBytes),exactClickProjectionSHA256:sha(clickBytes)};
  try{const value=joinLongFrames(JSON.parse(resultBytes).longAnimationFrameCapture,JSON.parse(clickBytes));writeFileSync(output,JSON.stringify({...binding,...value},null,2)+'\n',{flag:'wx',mode:0o600});console.log(JSON.stringify({status:'derived',clicks:value.clicks,frames:value.frames}));}
  catch{writeFileSync(output,JSON.stringify({...binding,status:'invalid',reason:'Long-frame capture or exact original click join refused; no attribution produced'},null,2)+'\n',{flag:'wx',mode:0o600});process.exitCode=1;}
}
