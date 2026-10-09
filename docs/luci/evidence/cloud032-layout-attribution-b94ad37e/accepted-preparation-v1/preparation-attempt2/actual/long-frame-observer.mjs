import assert from 'node:assert/strict';

export const browserLimits = Object.freeze({maxFrames:2048,maxScripts:8192,maxScriptsPerFrame:128,maxBytes:2*1024*1024});
export const ledgerLimits = Object.freeze({maxFrames:4096,maxScripts:16384,maxBytes:4*1024*1024,maxDocuments:16,maxWindows:2000});

// Runs once per document at setup. It modifies only the existing diagnostic
// snapshot object, never product state, storage, input, network or layout.
export function browserLongFrameInit(config) {
  'use strict';
  const limits=config.limits, timeOrigin=performance.timeOrigin;
  const state={version:1,timeOrigin,limits,supported:false,status:'pending',failureCode:null,wired:false,closed:false,observed:0,retained:0,scripts:0,bytes:0,dropped:0,lateCallbacks:0,callbacks:0,callbackMs:0,rows:[]};
  let observer=null;
  const finite=value=>typeof value==='number'&&Number.isFinite(value)&&value>=0;
  const fail=code=>{state.status='invalid';state.failureCode??=code;observer?.disconnect();};
  const invokers=['classic-script','module-script','event-listener','user-callback','resolve-promise','reject-promise'];
  const attributions=['self','descendant','ancestor','same-page','other'];
  const bundleMap=new Map(config.bundles.map(value=>[value.path,value.sha256]));
  const scriptProjection=entry=>{
    const result={};
    for(const key of ['startTime','duration','executionStart','forcedStyleAndLayoutDuration','pauseDuration']){
      if(!finite(entry[key]))throw Error('invalid-script-time');result[key]=entry[key];
    }
    result.invokerType=invokers.includes(entry.invokerType)?entry.invokerType:'unknown';
    result.windowAttribution=attributions.includes(entry.windowAttribution)?entry.windowAttribution:'unknown';
    result.sourceClass='unattributed';result.bundleSHA256=null;result.sourceCharPosition=null;
    // URL parsing is local classification only. No URL/function/selector string
    // or private source text is ever copied to the diagnostic result.
    try{
      const source=new URL(entry.sourceURL,location.href),hash=source.origin===location.origin&&!source.search&&!source.hash?bundleMap.get(source.pathname):null;
      if(hash){result.sourceClass='app-bundle';result.bundleSHA256=hash;result.sourceCharPosition=Number.isSafeInteger(entry.sourceCharPosition)&&entry.sourceCharPosition>=0?entry.sourceCharPosition:null;}
    }catch{}
    return result;
  };
  function ingest(entries){
    if(state.closed){state.lateCallbacks++;return;}
    if(state.status==='invalid'||state.status==='unsupported')return;
    for(const entry of entries){
      state.observed++;
      if(state.retained>=limits.maxFrames){state.dropped++;fail('frame-bound');break;}
      try{
        if(entry.entryType!=='long-animation-frame')throw Error('wrong-entry-type');
        const row={sequence:state.observed};
        for(const key of ['startTime','duration','blockingDuration','renderStart','styleAndLayoutStart','firstUIEventTimestamp']){
          if(!finite(entry[key]))throw Error('invalid-frame-time');row[key]=entry[key];
        }
        const scripts=entry.scripts;
        if(!Array.isArray(scripts))throw Error('invalid-script-list');
        if(scripts.length>limits.maxScriptsPerFrame||state.scripts+scripts.length>limits.maxScripts)throw Error('script-bound');
        row.scripts=scripts.map(scriptProjection);
        const bytes=JSON.stringify(row).length*2;
        if(state.bytes+bytes>limits.maxBytes)throw Error('byte-bound');
        state.rows.push(row);state.retained++;state.scripts+=scripts.length;state.bytes+=bytes;
      }catch(error){state.dropped++;fail(['wrong-entry-type','invalid-frame-time','invalid-script-time','invalid-script-list','script-bound','byte-bound'].includes(error.message)?error.message:'projection-failed');break;}
    }
  }
  function receive(entries){const start=performance.now();try{ingest(entries);}finally{state.callbacks++;state.callbackMs+=Math.max(0,performance.now()-start);}}
  function snapshot(){
    if(!state.closed&&observer&&state.status==='observing')receive(observer.takeRecords());
    return {...state,snapshotAt:performance.now(),visibility:document.visibilityState,rows:state.rows.splice(0)};
  }
  function wire(){
    if(state.wired)return;
    const latency=window.__luciAcceptanceLatency;
    if(!latency||typeof latency.snapshot!=='function'){fail('missing-latency-snapshot');return;}
    const original=latency.snapshot;
    latency.snapshot=function(...args){return {...Reflect.apply(original,this,args),longAnimationFrames:snapshot()};};
    state.wired=true;
  }
  try{
    state.supported=typeof PerformanceObserver!=='undefined'&&PerformanceObserver.supportedEntryTypes.includes('long-animation-frame');
    if(state.supported){observer=new PerformanceObserver(list=>receive(list.getEntries()));observer.observe({type:'long-animation-frame',buffered:true});state.status='observing';}
    else state.status='unsupported';
  }catch{state.status='unsupported';state.failureCode='observer-install-failed';observer?.disconnect();}
  if(window.__luciAcceptanceLatency)wire();
  else document.addEventListener('DOMContentLoaded',wire,{once:true});
  window.addEventListener('pagehide',()=>{if(!state.closed&&observer&&state.status==='observing')receive(observer.takeRecords());state.closed=true;observer?.disconnect();},{once:true});
}

const nonnegative=value=>typeof value==='number'&&Number.isFinite(value)&&value>=0;
const count=value=>Number.isSafeInteger(value)&&value>=0;
const ownKeys=(value,keys)=>value&&typeof value==='object'&&!Array.isArray(value)&&Object.keys(value).every(key=>keys.includes(key));
const windowKeys=['version','timeOrigin','limits','supported','status','failureCode','wired','closed','observed','retained','scripts','bytes','dropped','lateCallbacks','callbacks','callbackMs','rows','snapshotAt','visibility'];
const frameKeys=['sequence','startTime','duration','blockingDuration','renderStart','styleAndLayoutStart','firstUIEventTimestamp','scripts'];
const scriptKeys=['startTime','duration','executionStart','forcedStyleAndLayoutDuration','pauseDuration','invokerType','windowAttribution','sourceClass','bundleSHA256','sourceCharPosition'];
export function createLongFrameLedger(receipt,limits=ledgerLimits){
  const docs=new Map();let frames=0,scripts=0,bytes=0;
  const result=receipt.longAnimationFrameCapture={version:1,status:'pending',valid:false,limits:{...limits},windows:[],frames:0,scripts:0,bytes:0,documents:0,failureCode:null,scope:'Page-owned Long Animation Frame observations only. Durations are browser wall-clock intervals, not CPU time or physical display. The API reports qualifying long frames and attributed scripts, not all renderer work. No original timing is changed or subtracted.'};
  const fail=code=>{result.status='invalid';result.valid=false;result.failureCode??=code;throw Object.assign(Error('Long-frame diagnostic refused: '+code),{code:'long-frame-'+code});};
  return {append(value,outerOrigin){
    if(result.status==='invalid')return fail(result.failureCode);
    if(!ownKeys(value,windowKeys)||value.version!==1||!nonnegative(value.timeOrigin)||value.timeOrigin!==outerOrigin||!nonnegative(value.snapshotAt)||!nonnegative(value.callbackMs)||!Array.isArray(value.rows))return fail('invalid-window');
    if(!['pending','observing','unsupported','invalid'].includes(value.status)||!['visible','hidden'].includes(value.visibility)||typeof value.supported!=='boolean'||typeof value.wired!=='boolean'||typeof value.closed!=='boolean')return fail('invalid-window-state');
    if(value.failureCode!==null&&!['frame-bound','wrong-entry-type','invalid-frame-time','invalid-script-time','invalid-script-list','script-bound','byte-bound','projection-failed','missing-latency-snapshot','observer-install-failed'].includes(value.failureCode))return fail('invalid-failure-code');
    if(!ownKeys(value.limits,Object.keys(browserLimits))||Object.keys(browserLimits).some(key=>value.limits[key]!==browserLimits[key]))return fail('browser-limits-mismatch');
    if(['observed','retained','scripts','bytes','dropped','lateCallbacks','callbacks'].some(key=>!count(value[key])))return fail('invalid-counters');
    if(value.rows.length>browserLimits.maxFrames||value.retained>browserLimits.maxFrames||value.scripts>browserLimits.maxScripts||value.bytes>browserLimits.maxBytes)return fail('browser-bound');
    const previous=docs.get(value.timeOrigin)??{sequence:0,scripts:0,bytes:0,observed:0,callbacks:0,callbackMs:0,snapshotAt:0,closed:false};
    if(previous.closed&&value.rows.length)return fail('closed-document-data');
    if(value.snapshotAt<previous.snapshotAt||value.callbacks<previous.callbacks||value.callbackMs<previous.callbackMs||value.observed<previous.observed)return fail('counter-regression');
    let sequence=previous.sequence,addedScripts=0;
    for(const row of value.rows){
      if(!ownKeys(row,frameKeys)||!count(row.sequence)||row.sequence!==++sequence||!Array.isArray(row.scripts)||row.scripts.length>browserLimits.maxScriptsPerFrame)return fail('invalid-frame-sequence');
      if(frameKeys.filter(key=>!['sequence','scripts'].includes(key)).some(key=>!nonnegative(row[key])))return fail('invalid-frame-time');
      for(const script of row.scripts){
        if(!ownKeys(script,scriptKeys)||['startTime','duration','executionStart','forcedStyleAndLayoutDuration','pauseDuration'].some(key=>!nonnegative(script[key])))return fail('invalid-script');
        if(!['classic-script','module-script','event-listener','user-callback','resolve-promise','reject-promise','unknown'].includes(script.invokerType)||!['self','descendant','ancestor','same-page','other','unknown'].includes(script.windowAttribution))return fail('invalid-script-enum');
        if(script.sourceClass==='app-bundle'){
          if(!/^[a-f0-9]{64}$/.test(script.bundleSHA256??'')||(script.sourceCharPosition!==null&&!count(script.sourceCharPosition)))return fail('invalid-source-binding');
        }else if(script.sourceClass!=='unattributed'||script.bundleSHA256!==null||script.sourceCharPosition!==null)return fail('invalid-source-binding');
      }
      addedScripts+=row.scripts.length;
    }
    const encoded=JSON.stringify(value),nextBytes=bytes+Buffer.byteLength(encoded);
    if(result.windows.length>=limits.maxWindows||frames+value.rows.length>limits.maxFrames||scripts+addedScripts>limits.maxScripts||nextBytes>limits.maxBytes||!docs.has(value.timeOrigin)&&docs.size>=limits.maxDocuments)return fail('ledger-bound');
    // Retain an in-budget failure window before refusal; it is never counted as
    // successful complete capture. Original action evidence remains separate.
    result.windows.push(value);frames+=value.rows.length;scripts+=addedScripts;bytes=nextBytes;
    Object.assign(result,{frames,scripts,bytes,documents:docs.size+(docs.has(value.timeOrigin)?0:1)});
    if(!value.supported||value.status==='unsupported')return fail('unsupported');
    if(!value.wired||value.status!=='observing'||value.dropped!==0)return fail(value.failureCode??'capture-unavailable');
    if(value.retained!==sequence||value.observed!==sequence||value.scripts!==previous.scripts+addedScripts||value.bytes<previous.bytes)return fail('capture-counter-mismatch');
    docs.set(value.timeOrigin,{sequence,scripts:value.scripts,bytes:value.bytes,observed:value.observed,callbacks:value.callbacks,callbackMs:value.callbackMs,snapshotAt:value.snapshotAt,closed:value.closed});
    result.status='observing';result.valid=true;
  }};
}

export async function installLongFrameTiming(context,{receipt,timingLedger,bundles}){
  assert(Array.isArray(bundles)&&bundles.length>0&&bundles.length<=512,'Bounded exact package bundle list required');
  const seen=new Set();for(const value of bundles){assert(ownKeys(value,['path','sha256'])&&/^\/assets\/[A-Za-z0-9._-]+\.js$/.test(value.path)&&/^[a-f0-9]{64}$/.test(value.sha256)&&!seen.has(value.path),'Invalid package bundle binding');seen.add(value.path);}
  assert.equal(receipt.longAnimationFrameCapture,undefined,'One long-frame observer installation only');
  const ledger=createLongFrameLedger(receipt),original=timingLedger.append.bind(timingLedger);
  timingLedger.append=(value,reason)=>{original(value,reason);ledger.append(value?.latency?.longAnimationFrames,value?.timeOrigin);};
  await context.addInitScript(browserLongFrameInit,{limits:browserLimits,bundles});
}
