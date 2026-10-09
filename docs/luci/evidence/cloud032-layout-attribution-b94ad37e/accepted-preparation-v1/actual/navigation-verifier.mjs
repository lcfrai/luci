// External diagnostic only. Serialized as the original navigation evaluate callback.
// No product state writes, extra RPC, observer or retry loop is introduced.
export function navigationContextWitness(el,value){
 return new Promise((done,reject)=>{
  const enteredAt=performance.now(),until=enteredAt+10000,timeOrigin=performance.timeOrigin;
  let firstSampleAt=null,samples=0,failedDisconnected=0,failedValueMismatch=0;
  const sample=()=>{
   const sampledAt=performance.now();if(firstSampleAt===null)firstSampleAt=sampledAt;samples++;
   const connected=el.isConnected;
   if(connected&&el.value===value){
    const firstMatchedAt=performance.now();
    window.__luciAcceptanceLatency.markExpectedDOM('project-switch');
    return done({version:1,status:'matched',timeOrigin,enteredAt,firstSampleAt,firstMatchedAt,resolveRequestedAt:performance.now(),samples,failedSamples:failedDisconnected+failedValueMismatch,failedDisconnected,failedValueMismatch});
   }
   if(connected)failedValueMismatch++;else failedDisconnected++;
   if(performance.now()>=until)return reject(Error('Expected project context did not render'));
   requestAnimationFrame(sample);
  };sample();
 });
}

// Diagnostic receipt storage has no RPC or asynchronous work. A completed action
// owns its value; a late evaluate continuation cannot mutate its closed record.
export function navigationWitnessSlot(){
 let closed=false,value;
 return {accept(next){if(!closed)value=next;},close(){closed=true;return value;}};
}
