import assert from 'node:assert/strict';
export function createReadinessLedger(receipt){
 let origin=null,sequence=0,windows=0,bytes=0;
 receipt.autonomousReadiness={version:1,diagnosticOnly:true,status:'not-started',windows:[],rows:0,bytes:0,limitations:['Structural DOM/native-enabled predicate only; rendered selection is not server provenance or permission authority.','First observed witness may be late; no actual first-ready, physical paint, frontend cause, overhead subtraction or acceptance claim.','Fixed four-project context/authority cohort only. Unobservable transitions require explicit harness invalidation.','Sampling and transport add overhead; original full action metrics and thresholds remain unchanged.']};
 return {
  begin(value){assert.equal(origin,null);assert.equal(value.version,1);assert(Number.isFinite(value.timeOrigin));origin=value.timeOrigin;receipt.autonomousReadiness.status='active';receipt.autonomousReadiness.setup=value;},
  append(value){
   if(!value){receipt.autonomousReadiness.status='observer-unavailable';throw Error('Installed readiness observer unavailable');}assert.equal(value.timeOrigin,origin,'Observer document changed');assert.equal(value.version,1);assert(Array.isArray(value.rows)&&value.rows.length<=1000);assert(++windows<=105,'Observer window bound');
   const size=Buffer.byteLength(JSON.stringify(value));assert(bytes+size<=1024**2,'Observer ledger byte bound');bytes+=size;
   for(const row of value.rows){assert.equal(row.sequence,++sequence,'Observer sequence gap');assert(Number.isFinite(row.at));}
   assert.equal(value.sequence,sequence,'Observer rows lost');
   receipt.autonomousReadiness.windows.push(value);receipt.autonomousReadiness.rows=sequence;receipt.autonomousReadiness.bytes=bytes;
   if(value.closed)receipt.autonomousReadiness.status=value.overflow?'overflow':'closed';
  },
  unavailable(){receipt.autonomousReadiness.status='cleanup-unavailable';}
 };
}
