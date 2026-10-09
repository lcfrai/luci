import assert from 'node:assert/strict';
import {setTimeout as delay} from 'node:timers/promises';
// Every route promise is retained before yielding. Sealing changes transport
// policy only; completion events are the only way to settle forwarded writes.
export function createRouteBoundary({origin,allowed,health,record,fail,isClosing}) {
 const callbacks=new Set(),mutations=new Map();let sealed=false,closingReads=0;
 const readMethod=method=>['GET','HEAD','OPTIONS'].includes(method);
 const track=task=>{callbacks.add(task);void task.then(()=>callbacks.delete(task),error=>{callbacks.delete(task);fail(error);});return task;};
 const finished=request=>{const pending=mutations.get(request);if(pending){mutations.delete(request);try{record({kind:'browser-mutation-terminal',...pending,failed:!!request.failure()});}catch(error){fail(error);}}};
 const snapshot=()=>({sealed,pendingCallbacks:callbacks.size,pendingMutations:[...mutations.values()],closingReads});
 return {
  track,finished,snapshot,
  seal(){sealed=true;record({kind:'browser-routes-sealed',...snapshot()});},
  handle(route){
   const request=route.request(),method=request.method(),url=request.url();
   if(!readMethod(method))mutations.set(request,{method,path:new URL(url).pathname});
   return track((async()=>{
    try{
     const admissible=allowed({method,url,origin});
     if((sealed||isClosing())&&readMethod(method)&&admissible){closingReads++;record({kind:'owned-teardown-read-cancelled',method,path:new URL(url).pathname});await route.abort('aborted');return;}
     health();assert(!sealed,'Browser route admission is sealed');assert(admissible,'Unexpected browser request');await route.continue();
    }catch(error){fail(error);try{await route.abort('blockedbyclient');}catch(abortError){fail(abortError);}}
   })());
  },
  async drain(timeoutMs=2500){
   assert(sealed,'Seal route admission before draining');const deadline=Date.now()+timeoutMs;
   while(callbacks.size||mutations.size){if(Date.now()>=deadline)throw Object.assign(Error('Browser route callbacks or mutations remain unresolved'),{code:'browser-route-drain-deadline'});await delay(20);}
   // Let already-queued completion callbacks enter before taking the sealed snapshot.
   await delay(0);assert.equal(callbacks.size,0);assert.equal(mutations.size,0);record({kind:'browser-routes-drained',...snapshot()});return snapshot();
  },
 };
}
