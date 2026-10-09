// External control only. No navigation/edit/injection into product DOM in timed arms.
import assert from 'node:assert/strict';
export const CONTROL_LIMITS=Object.freeze({destinations:4,blocksPerDestination:2,callsPerBlock:4,totalCalls:32,callMs:5000,phaseMs:60000,blockMs:15000,rows:32,bytes:131072});
export const ARM_ORDERS=Object.freeze([Object.freeze(['locator','handle','handle','locator']),Object.freeze(['handle','locator','locator','handle'])]);

// Setup-only call on the exact retained handle. Registry is diagnostic state,
// never a DOM attribute, product store, listener to product events or API write.
export function installSettledBlock(el,config){
 'use strict';
 if(window.__luciSettledEvaluateControl)throw Error('Control registry already exists');
 if(!config||!Number.isSafeInteger(config.generation)||config.generation<1||config.generation>8||config.authorityGeneration!==1||typeof config.projectID!=='string'||config.projectID.length>256||typeof config.projectName!=='string'||config.projectName.length>256||typeof config.expected!=='string'||config.expected.length>131072)throw Error('Invalid control setup');
 const scan=(root,predicate,prune=()=>false)=>{const out=[];let count=0,node=root.firstElementChild;while(node){if(++count>512)throw Error('Control node cap');if(predicate(node))out.push(node);if(!prune(node)&&node.firstElementChild){node=node.firstElementChild;continue;}while(node&&node!==root&&!node.nextElementSibling)node=node.parentElement;if(!node||node===root)break;node=node.nextElementSibling;}return out;};
 // React's root contains the shell in the actual installed DOM. Root discovery
 // is setup-only and bounded; unrelated matches cannot silently win.
 const allShells=scan(document.body,n=>n.classList.contains('shell'),n=>n.classList.contains('shell'));
 if(allShells.length!==1)throw Error('Ambiguous control shell');const shell=allShells[0];
 const rail=Array.from(shell.children).find(n=>n.matches('nav#project-navigation'));
 const main=Array.from(shell.children).find(n=>n.matches('main.shell-main'));
 const workspace=main&&Array.from(main.children).find(n=>n.classList.contains('shell-workspace'));
 const lens=workspace&&Array.from(workspace.children).find(n=>n.classList.contains('shell-lens'));
 if(!rail||!lens)throw Error('Control root unavailable');
 const definitions=scan(lens,n=>n.matches('section.definition-workspace'));
 const selected=scan(rail,n=>n.matches('[role="treeitem"][data-kind="project"][aria-selected="true"]'));
 if(definitions.length!==1||selected.length!==1)throw Error('Ambiguous control identity');const definition=definitions[0],project=selected[0];
 const entries=scan(definition,n=>n.matches('.definition-specification > label.luci-field > textarea'));
 if(entries.length!==1||entries[0]!==el||!(el instanceof HTMLTextAreaElement))throw Error('Control editor identity unavailable');
 if(el.labels?.length!==1||el.labels[0].control!==el||el.hasAttribute('aria-label')||el.hasAttribute('aria-labelledby')||Array.from(el.labels[0].childNodes).filter(n=>n.nodeType===Node.TEXT_NODE).map(n=>n.textContent).join('').trim()!=='The idea')throw Error('Control editor label unavailable');
 let closed=false;const origin=performance.timeOrigin,started=performance.now(),generation=config.generation;
 const usable=node=>{let n=node;for(let i=0;n;i++,n=n.parentElement){if(i>=32)return false;if(n.hasAttribute('hidden')||n.hasAttribute('inert')||n.getAttribute('aria-hidden')==='true')return false;}return !node.matches(':disabled')&&node.getAttribute('aria-disabled')!=='true';};
 const state={element:el,generation,authorityGeneration:1,timeOrigin:origin,calls:0,closed:false,
  check(target,requested){
   if(closed||state.closed||requested!==generation)return 'inactive-generation';
   if(document.visibilityState!=='visible')return 'document-not-visible';
   if(performance.timeOrigin!==origin||performance.now()-started>15000)return 'block-clock-or-deadline';
   if(target!==el)return 'replaced-editor';
   if(!el.isConnected||!shell.isConnected||!rail.isConnected||!lens.isConnected||!definition.isConnected||!project.isConnected)return 'disconnected-identity';
   if(!definition.contains(el)||!lens.contains(definition)||!shell.contains(lens)||!shell.contains(rail))return 'changed-root';
   if(project.getAttribute('data-id')!==config.projectID||project.getAttribute('aria-label')!==config.projectName||project.getAttribute('aria-selected')!=='true')return 'changed-selection';
   if(el.value!==config.expected)return 'changed-context';
   if(el.readOnly||!usable(el)||!usable(definition))return 'lost-editability';
   return null;
  },
  close(){closed=true;state.closed=true;window.removeEventListener('pagehide',leaving);state.element=null;},
 };
 function leaving(){state.close();}
 const problem=state.check(el,generation);if(problem)throw Error('Invalid settled control identity: '+problem);
 Object.defineProperty(window,'__luciSettledEvaluateControl',{value:state,configurable:true});window.addEventListener('pagehide',leaving,{once:true});
 return {generation,authorityGeneration:1,timeOrigin:origin,readyAt:performance.now(),sameElement:true,connected:true,nativeEditable:true};
}

// EXACT same callback bytes in both arms. An already-settled context is a
// precondition: a false predicate is retained as refusal, never polled/retried.
// This is a diagnostic predicate, not the original acceptance polling callback.
export function evaluateSettledContext(el,generation){
 'use strict';
 const enteredAt=performance.now(),origin=performance.timeOrigin,state=window.__luciSettledEvaluateControl;
 const firstSampleAt=performance.now(),samples=1;
 if(!state)return {status:'refused',reason:'missing-registry',timeOrigin:origin,enteredAt,firstSampleAt,finishedAt:performance.now(),samples};
 const reason=state.check(el,generation);
 if(reason)return {status:'refused',reason,timeOrigin:origin,enteredAt,firstSampleAt,finishedAt:performance.now(),samples,generation};
 if(++state.calls>4)return {status:'refused',reason:'call-cap',timeOrigin:origin,enteredAt,firstSampleAt,finishedAt:performance.now(),samples,generation};
 return {status:'observed',reason:null,timeOrigin:origin,enteredAt,firstSampleAt,finishedAt:performance.now(),samples,generation,sameElement:true,connected:true,valueMatches:true,nativeEditable:true,call:state.calls};
}
export function closeSettledBlock(generation){
 const state=window.__luciSettledEvaluateControl;if(!state)return {closed:false,reason:'missing-registry'};
 if(state.generation!==generation)throw Error('Control generation changed before close');
 const reason=state.check(state.element,generation),calls=state.calls;state.close();delete window.__luciSettledEvaluateControl;
 return {closed:true,reason,calls,generation,timeOrigin:performance.timeOrigin};
}
function limited(promise,ms){let timer;return Promise.race([promise,new Promise((_,reject)=>{timer=setTimeout(()=>reject(Error('Control call deadline')),ms);})]).finally(()=>clearTimeout(timer));}
const allowedReasons=new Set(['inactive-generation','document-not-visible','block-clock-or-deadline','replaced-editor','disconnected-identity','changed-root','changed-selection','changed-context','lost-editability','missing-registry','call-cap']);
function safeCallback(value){
 assert(value&&['observed','refused'].includes(value.status));assert(value.reason===null||allowedReasons.has(value.reason));
 const out={status:value.status,reason:value.reason};for(const name of ['timeOrigin','enteredAt','firstSampleAt','finishedAt','samples','generation','call'])if(name in value){assert(Number.isFinite(value[name])&&value[name]>=0);out[name]=value[name];}
 for(const name of ['sameElement','connected','valueMatches','nativeEditable'])if(name in value){assert(typeof value[name]==='boolean');out[name]=value[name];}
 assert(out.samples===1&&out.enteredAt<=out.firstSampleAt&&out.firstSampleAt<=out.finishedAt);return out;
}
export async function runSettledControl({page,owners,receipt,assertHealthy}){
 assert.equal(owners.length,4);assert(!receipt.settledEvaluateControl);
 const out=receipt.settledEvaluateControl={version:1,diagnosticOnly:true,status:'running',limits:CONTROL_LIMITS,orders:ARM_ORDERS,rows:[],blocks:[],failures:[],lateCallbacks:0,originalFullActionCohortExecuted:false,acceptanceReplacement:false};
 const phaseStarted=performance.now();let bytes=0;
 const healthy=()=>{assertHealthy();assert(performance.now()-phaseStarted<CONTROL_LIMITS.phaseMs,'Control phase deadline');};
 const bound=promise=>limited(promise,Math.min(CONTROL_LIMITS.callMs,Math.max(1,CONTROL_LIMITS.phaseMs-(performance.now()-phaseStarted))));
 try{
  for(let destination=0;destination<4;destination++){
   healthy();const owner=owners[destination];
   await bound(page.getByRole('button',{name:'All projects',exact:true}).click());await bound(page.getByRole('heading',{name:'Across your projects',exact:true}).waitFor());await bound(page.getByRole('button',{name:'Open project '+owner.owner.name,exact:true}).click());
   const field=page.getByRole('textbox',{name:'The idea',exact:true});await bound(field.waitFor());
   // Fresh setup-only saved-value wait; measured arms never navigate/edit/wait
   // for a context change. Each installed block rechecks exact identity.
   await bound(page.waitForFunction(expected=>{const nodes=document.querySelectorAll('.definition-specification > label.luci-field > textarea');return nodes.length===1&&nodes[0].isConnected&&nodes[0].value===expected&&!nodes[0].readOnly&&!nodes[0].matches(':disabled');},owner.goal,{timeout:5000}));
   for(let block=0;block<2;block++){
    healthy();const generation=destination*2+block+1;let handle,installAttempted=false,primaryError;
    const record={destination,block,generation,status:'setup',setup:null,close:null};out.blocks.push(record);
    try{
     handle=await bound(field.elementHandle({timeout:5000}));assert(handle,'Missing exact editor handle');
     installAttempted=true;record.setup=await bound(handle.evaluate(installSettledBlock,{generation,authorityGeneration:1,projectID:owner.owner.id,projectName:owner.owner.name,expected:owner.goal}));record.status='active';
     for(const [position,arm] of ARM_ORDERS[block].entries()){
      healthy();assert(out.rows.length<32);const row={index:out.rows.length,destination,block,generation,position,arm,success:false,nodeStart:performance.now(),nodeStages:[],callback:null};let stage='waitFor',active=true;
      const requireActive=()=>{if(!active){out.lateCallbacks++;throw Error('Control continuation after deadline');}};
      const mark=name=>{requireActive();return row.nodeStages.push({stage:name,offsetMs:performance.now()-row.nodeStart});};
      try{
       await bound((async()=>{mark('wait-start');await field.waitFor();mark('wait-end');stage='evaluate';const callback=await (arm==='locator'?field:handle).evaluate(evaluateSettledContext,generation);requireActive();row.callback=safeCallback(callback);mark('evaluate-end');assert(row.callback.status==='observed','Control predicate refused');assert.equal(row.callback.timeOrigin,record.setup.timeOrigin);assert.equal(row.callback.generation,generation);assert.equal(row.callback.call,position+1);stage='inputValue';const value=await field.inputValue();requireActive();assert(value===owner.goal,'Saved context changed after evaluation');mark('input-value-end');})());row.success=true;
      }catch(error){row.failureStage=stage;row.failureReason=row.callback?.reason??'command-or-deadline';throw error;}
      finally{active=false;row.elapsedMs=performance.now()-row.nodeStart;delete row.nodeStart;out.rows.push(row);bytes+=Buffer.byteLength(JSON.stringify(row));assert(bytes<=CONTROL_LIMITS.bytes,'Control receipt byte cap');}
     }
     record.status='completed';
    }catch(error){primaryError=error;record.status='failed';throw error;}
    finally{
     let cleanupError;
     if(installAttempted){try{record.close=await limited(page.evaluate(closeSettledBlock,generation),2000);assert(record.close.closed&&record.close.reason===null,'Control identity changed before close');assert.equal(record.close.calls,4);}catch(error){out.failures.push({generation,kind:'block-close-failed'});cleanupError??=error;}}
     if(handle){try{await limited(handle.dispose(),2000);}catch(error){out.failures.push({generation,kind:'handle-dispose-failed'});cleanupError??=error;}}
     if(!primaryError&&cleanupError)throw cleanupError;
    }
   }
  }
  assert.equal(out.rows.length,32);assert(out.rows.every(r=>r.success));assert.equal(out.failures.length,0);out.status='completed';
 }catch(error){out.status='failed';throw error;}
 finally{out.elapsedMs=performance.now()-phaseStarted;out.bytes=bytes;}
 return out;
}
