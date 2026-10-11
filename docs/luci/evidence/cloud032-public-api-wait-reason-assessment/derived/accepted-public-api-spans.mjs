// Public Playwright Logger API only. No patching/tracing/RPC/browser mutation.
// Do not read args, hints, Error.message, selectors, results or arbitrary text.
const API_NAMES=new Set(['locator.click','locator.waitFor','locator.evaluate','locator.inputValue','locator.press','page.waitForFunction','page.evaluate']);
export const EXPECTED_CALLS=Object.freeze({
 'project-switch':['locator.click','locator.waitFor','locator.click','locator.waitFor','locator.evaluate','locator.inputValue','page.evaluate'],
 input:['locator.press','page.waitForFunction','page.evaluate'],
 'draft-reset':['locator.click','locator.click','locator.click','locator.inputValue','page.evaluate'],
});
export function createPublicAPISpans({clock=()=>performance.now(),maxEntries=8000,maxBytes=2*1024**2,maxActions=400}={}){
 const rows=[],actions=[];let windowActive=false,active=null,bytes=0,overflow=false,ignoredOutside=0;
 const problems=[];
 const problem=code=>{if(problems.length<100)problems.push(code);else overflow=true;};
 const append=row=>{
  if(overflow)return false;
  const size=Buffer.byteLength(JSON.stringify(row));
  if(rows.length>=maxEntries||bytes+size>maxBytes){overflow=true;problem('entry-or-byte-bound');return false;}
  bytes+=size;rows.push(row);return true;
 };
 const logger={
  isEnabled(name,severity){return windowActive&&active!==null&&!overflow&&name==='api'&&severity==='info';},
  log(name,severity,message,_args,_hints){
   if(!windowActive||active===null){ignoredOutside++;return;}
   if(name!=='api'||severity!=='info'){problem('unexpected-logger-channel');return;}
   if(typeof message!=='string'||message.length>80){problem('unrecognized-api-message');return;}
   const match=/^(=>|<=) ([A-Za-z]+\.[A-Za-z]+) (started|succeeded|failed)$/.exec(message);
   if(!match||match[0]!==message||!API_NAMES.has(match[2])||(match[1]==='=>')!==(match[3]==='started')){problem('unrecognized-api-message');return;}
   const now=clock();if(!Number.isFinite(now)||now<active.startedNodeMs){problem('invalid-node-clock');return;}
   if(active.lastNodeMs!==undefined&&now<active.lastNodeMs){problem('nonmonotonic-node-clock');return;}active.lastNodeMs=now;
   const api=match[2],state=match[3];
   if(state==='started'){
    if(active.pending!==null)problem('overlapping-public-call');
    active.pending={api,index:rows.length};
   }else{
    if(active.pending?.api!==api)problem('unmatched-public-call-end');
    active.pending=null;
   }
   append({ordinal:active.ordinal,api,state,nodeOffsetMs:now-active.startedNodeMs});
  },
 };
 return{
  logger,
  startWindow(){if(windowActive||actions.length||active!==null)throw Error('API diagnostic window may start only once');windowActive=true;},
  begin(ordinal,kind,startedNodeMs){
   if(!windowActive)throw Error('API diagnostic action outside window');
   if(active!==null||ordinal!==actions.length+1||ordinal>maxActions||!EXPECTED_CALLS[kind]||!Number.isFinite(startedNodeMs))throw Error('Invalid API diagnostic action identity');
   active={ordinal,kind,startedNodeMs,first:rows.length,pending:null};
  },
  end(ordinal){
   if(active===null||ordinal!==active.ordinal)throw Error('API diagnostic action identity changed');
   if(active.pending!==null)problem('action-ended-with-pending-call');
   const received=rows.slice(active.first).map(row=>row.api+':'+row.state);
   const expected=EXPECTED_CALLS[active.kind].flatMap(api=>[api+':started',api+':succeeded']);
   const complete=received.length===expected.length&&received.every((value,index)=>value===expected[index]);
   if(!complete)problem('incomplete-or-unexpected-public-call-sequence');
   actions.push({ordinal,kind:active.kind,startedNodeMs:active.startedNodeMs,firstEntry:active.first,entryCount:rows.length-active.first,complete});active=null;
  },
  stopWindow(){windowActive=false;if(active!==null)problem('window-stopped-mid-action');},
  snapshot(){return{version:1,scope:'Public Logger api-name lifecycle only; callback times in same Node clock as retained primary spans. No args/values/selectors/results are read. Public Logger is deprecated/incomplete: exact observed sequence is required and absence is not evidence of no internal retry.',limits:{maxEntries,maxBytes,maxActions},complete:!windowActive&&active===null&&!overflow&&problems.length===0&&actions.length===400&&actions.every(x=>x.complete),windowActive,activeOrdinal:active?.ordinal??null,overflow,bytes,ignoredOutside,problems:[...problems],actions:structuredClone(actions),entries:structuredClone(rows),limitations:['Instrumentation adds overhead; no acceptance comparison or subtraction','Logs delimit public calls, not internal retry iterations, transport phases or actual renderer-ready time','Success means exact observed public sequence, not completeness of all internal Playwright work','Node offsets are never subtracted from browser performance or trace timestamps']};},
 };
}
