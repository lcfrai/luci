// External diagnostic only. No product state, layout, network or mutation API.
// Passed to page.evaluate once, outside all original primary action spans.
export function browserReadinessInit(config) {
 'use strict';
 if (window.__luciAutonomousReadiness) throw Error('Readiness observer already installed');
 const limits=Object.freeze({stages:200,samplesPerStage:180,stageMs:3000,totalSamples:36000,nodesPerSample:512,controlsPerSample:32,ancestors:32,rows:1000,bytes:524288});
 if (!config || config.projects?.length!==4 || config.rounds!==100 || !Number.isSafeInteger(config.authorityGeneration)) throw Error('Invalid fixed readiness cohort');
 let projects=config.projects.map((p,index)=>{
  if(typeof p.id!=='string'||!p.id||p.id.length>256||typeof p.name!=='string'||!p.name||p.name.length>256||typeof p.value!=='string'||p.value.length>131072)throw Error('Invalid private readiness context');
  return {id:p.id,name:p.name,value:p.value,ordinal:index};
 });
 if(new Set(projects.map(p=>p.id)).size!==4||new Set(projects.map(p=>p.name)).size!==4)throw Error('Ambiguous private readiness identity');
 const authorityGeneration=config.authorityGeneration,timeOrigin=performance.timeOrigin;config=null;
 const seenEditors=new WeakSet();
 let active=null,raf=0,timer=0,closed=false,generation=0,round=0,totalSamples=0,stages=0,sequence=0,bytes=0,overflow=false,lateCallbacks=0,rows=[],previousEditor=null,homeObserved=false;
 const fixedReasons=new Set(['cohort-end','action-failed','authority-changed','source-changed','document-leaving','driver-cancelled']);
 const now=()=>performance.now();
 function emit(row){
  const result={sequence:sequence+1,at:now(),authorityGeneration,...row};
  const size=JSON.stringify(result).length;
  if(rows.length>=limits.rows||bytes+size>limits.bytes){overflow=true;closed=true;active=null;generation++;clear();detach();projects=[];previousEditor=null;return;}
  sequence++;rows.push(result);bytes+=size;
 }
 function clear(){if(raf)cancelAnimationFrame(raf);if(timer)clearTimeout(timer);raf=timer=0;}
 function finish(status,reason,at=now()){
  if(!active)return;const a=active;a.maxGap=Math.max(a.maxGap,at-(a.lastSampleAt??a.clickAt));active=null;clear();
  emit({generation:a.generation,round:a.round,projectOrdinal:a.project.ordinal,stage:a.stage,status,reason,clickAt:a.clickAt,observedAt:status==='ready'?at:null,samples:a.samples,firstSampleAt:a.firstSampleAt,lastSampleAt:a.lastSampleAt,maxSampleGapMs:a.maxGap,maxNodesVisited:a.maxNodes,maxControlsVisited:a.maxControls});
  if(a.stage==='home')homeObserved=status==='ready';
 }
 function invalidate(reason){
  if(!fixedReasons.has(reason))throw Error('Unknown readiness invalidation');
  if(closed){clear();detach();projects=[];previousEditor=null;active=null;return;}finish('cancelled',reason);closed=true;generation++;clear();detach();projects=[];previousEditor=null;
  emit({generation,round,projectOrdinal:null,stage:null,status:'closed',reason});
 }
 // Every traversal is bounded, including ancestor and root discovery walks.
 function readDOM(a){
  if(a.shell&&!a.shell.isConnected||a.lens&&!a.lens.isConnected||a.definition&&!a.definition.isConnected||a.editor&&!a.editor.isConnected)return {reason:'observed-disconnection',terminal:true,visited:0,controls:0};
  let visited=0,controls=0;
  const count=node=>{if(visited>=limits.nodesPerSample)throw 'node-cap';visited++;if(node instanceof HTMLButtonElement||node instanceof HTMLTextAreaElement){if(controls>=limits.controlsPerSample)throw 'control-cap';controls++;}};
  const scan=(root,predicate,prune=()=>false)=>{
   const result=[];let node=root.firstElementChild;
   while(node){count(node);if(predicate(node))result.push(node);
    if(!prune(node)&&node.firstElementChild){node=node.firstElementChild;continue;}
    while(node&&node!==root&&!node.nextElementSibling)node=node.parentElement;
    if(!node||node===root)break;node=node.nextElementSibling;
   }return result;
  };
  const direct=(root,predicate)=>{const result=[];for(let node=root.firstElementChild;node;node=node.nextElementSibling){count(node);if(predicate(node))result.push(node);}return result;};
  const one=items=>items.length===1?items[0]:null;
  const usable=node=>{
   if(!node?.isConnected)return false;
   let p=node;for(let n=0;p;n++,p=p.parentElement){if(n>=limits.ancestors)throw 'ancestor-cap';if(p.hasAttribute('hidden')||p.hasAttribute('inert')||p.getAttribute('aria-hidden')==='true')return false;}
   return !node.matches(':disabled') && node.getAttribute('aria-disabled')!=='true';
  };
  try{
   const shells=scan(document.body,n=>n.classList.contains('shell'),n=>n.classList.contains('shell'));
   if(shells.length!==1)return {reason:'ambiguous-root',terminal:true,visited,controls};const shell=shells[0];
   const main=one(direct(shell,n=>n.matches('main.shell-main')));
   const workspace=main&&one(direct(main,n=>n.classList.contains('shell-workspace')));
   const lens=workspace&&one(direct(workspace,n=>n.classList.contains('shell-lens')));
   const rail=one(direct(shell,n=>n.matches('nav#project-navigation')));
   if(!lens||!rail)return {reason:'missing-root',terminal:!!a.lens,visited,controls};
   if(a.clickedShell!==shell||a.shell&& (a.shell!==shell||a.lens!==lens))return {reason:'root-replaced',terminal:true,visited,controls};
   a.shell=shell;a.lens=lens;
   const selected=scan(rail,n=>n.matches('[role="treeitem"][data-kind="project"][aria-selected="true"]'));
   const nodes=scan(lens,n=>n.matches('section.definition-workspace,section.definition-workspace > header h1,.overview-heading h2,nav.overview-projects[aria-label="Open a project"] button.overview-project-destination,.definition-specification > label.luci-field > textarea'));
   const definitions=nodes.filter(n=>n.matches('section.definition-workspace'));
   const fields=nodes.filter(n=>n instanceof HTMLTextAreaElement);
   if(a.stage==='home'){
    if(previousEditor===null){previousEditor=fields.find(field=>field.labels?.length===1&&Array.from(field.labels[0].childNodes).filter(n=>n.nodeType===Node.TEXT_NODE).map(n=>n.textContent).join('').trim()==='The idea')??null;if(previousEditor)seenEditors.add(previousEditor);}
    const headings=nodes.filter(n=>n.matches('.overview-heading h2')&&n.textContent.trim()==='Across your projects');
    const buttons=nodes.filter(n=>n instanceof HTMLButtonElement&&n.getAttribute('aria-label')==='Open project '+a.project.name);
    return {ready:selected.length===0&&definitions.length===0&&fields.length===0&&headings.length===1&&buttons.length===1&&usable(headings[0])&&usable(buttons[0]),visited,controls};
   }
   if(selected.length!==1||selected[0].getAttribute('data-id')!==a.project.id||selected[0].getAttribute('aria-label')!==a.project.name)return {reason:'project-mismatch',visited,controls};
   const fieldsWithLabel=fields.filter(field=>field.labels?.length===1&&field.labels[0].control===field&&!field.hasAttribute('aria-label')&&!field.hasAttribute('aria-labelledby')&&Array.from(field.labels[0].childNodes).filter(n=>n.nodeType===Node.TEXT_NODE).map(n=>n.textContent).join('').trim()==='The idea');
   if(definitions.length!==1||fieldsWithLabel.length!==1)return {reason:'missing-editor',terminal:!!a.editor||!!a.definition,visited,controls};
   const field=fieldsWithLabel[0];
   const headings=nodes.filter(n=>n.matches('section.definition-workspace > header h1')&&n.textContent.trim()==='Define your project');
   if(!definitions[0].contains(field)||headings.length!==1||!definitions[0].contains(headings[0]))return {reason:'ambiguous-editor',terminal:true,visited,controls};
   if(field===previousEditor||seenEditors.has(field)&&field!==a.editor)return {reason:'retained-editor',terminal:true,visited,controls};
   if(a.editor&&a.editor!==field)return {reason:'editor-replaced',terminal:true,visited,controls};
   if(a.definition&&a.definition!==definitions[0])return {reason:'definition-replaced',terminal:true,visited,controls};
   a.definition=definitions[0];a.editor=field;seenEditors.add(field);
   return {ready:field.value===a.project.value&&!field.readOnly&&usable(field)&&usable(definitions[0]),visited,controls};
  }catch(reason){return {reason:typeof reason==='string'?reason:'read-error',terminal:true,visited,controls};}
 }
 function sample(expected,at=now()){
  if(closed||!active||active.generation!==expected){lateCallbacks++;return;}
  const a=active;
  if(at-a.clickAt>=limits.stageMs||a.samples>=limits.samplesPerStage||totalSamples>=limits.totalSamples){finish('censored','sample-or-time-cap',at);return;}
  a.samples++;totalSamples++;a.firstSampleAt??=at;a.maxGap=Math.max(a.maxGap,at-(a.lastSampleAt??a.clickAt));a.lastSampleAt=at;
  const value=readDOM(a);a.maxNodes=Math.max(a.maxNodes,value.visited);a.maxControls=Math.max(a.maxControls,value.controls);
  if(value.terminal){finish('censored',value.reason,at);return;}
  if(value.ready){finish('ready','predicate-observed',at);return;}
  raf=requestAnimationFrame(()=>sample(expected));
 }
 function arm(stage,project,clickedShell,clickAt){
  if(closed)return;finish('cancelled','superseded');if(closed)return;
  if(stages>=limits.stages){invalidate('driver-cancelled');return;}
  stages++;active={stage,project,clickAt,generation:++generation,round,samples:0,firstSampleAt:null,lastSampleAt:null,maxGap:0,maxNodes:0,maxControls:0,clickedShell,shell:null,lens:null,editor:null,definition:null};
  const expected=generation;sample(expected);if(active?.generation!==expected)return;timer=setTimeout(()=>{if(active?.generation===expected)finish('censored','wall-time-cap');},limits.stageMs);
 }
 function clicked(event){
  if(closed||!event.isTrusted)return;const clickAt=now();
  let node=event.target,button=null,clickedShell=null,projectRow=null;
  for(let n=0;node&&n<limits.ancestors;n++,node=node.parentElement){
   if(node instanceof HTMLButtonElement&&!button)button=node;
   if(node instanceof Element&&node.matches('[role="treeitem"][data-kind="project"]'))projectRow=node;
   if(node instanceof Element&&node.classList.contains('shell')){clickedShell=node;break;}
  }
  if(projectRow){invalidate('source-changed');return;}
  if(!button||!clickedShell)return;
  if(button.matches('.rail-brand > button.rail-home[aria-label="All projects"]')){
   if(round>=100){invalidate('driver-cancelled');return;}
   if(active)finish('cancelled','superseded');homeObserved=false;
   previousEditor=null;arm('home',projects[round%4],clickedShell,clickAt);return;
  }
  if(button.matches('nav.overview-projects[aria-label="Open a project"] button.overview-project-destination')){
   if(round>=100){invalidate('driver-cancelled');return;}
   const project=projects[round%4];
   if(button.getAttribute('aria-label')!=='Open project '+project.name){invalidate('driver-cancelled');return;}
   // An actual trusted Open click can be earlier than the next rAF. Capture the
   // Home predicate here before the product handler, never through a driver RPC.
   if(active?.stage==='home'){if(raf)cancelAnimationFrame(raf);raf=0;sample(active.generation);}
   if(!homeObserved){finish('censored','home-not-observed');invalidate('driver-cancelled');return;}
   arm('define',project,clickedShell,clickAt);round++;
  }
 }
 function leaving(){invalidate('document-leaving');}
 function detach(){document.removeEventListener('click',clicked,true);window.removeEventListener('pagehide',leaving);}
 document.addEventListener('click',clicked,true);window.addEventListener('pagehide',leaving);
 const api=Object.freeze({
  snapshot(){return {version:1,timeOrigin,authorityGeneration,limits,closed,round,stages,totalSamples,sequence,bytes,overflow,lateCallbacks,activeGeneration:active?.generation??null,rows:rows.splice(0)};},
  invalidate,
  close(){invalidate('cohort-end');return this.snapshot();}
 });
 Object.defineProperty(window,'__luciAutonomousReadiness',{value:api,configurable:false});
 return {version:1,timeOrigin,authorityGeneration,limits};
}

export async function beginReadiness(page,owners){
 return page.evaluate(browserReadinessInit,{projects:owners.map(owner=>({id:owner.owner.id,name:owner.owner.name,value:owner.goal})),rounds:100,authorityGeneration:1});
}
export async function drainReadiness(page,reason){
 return page.evaluate(reason=>{const observer=window.__luciAutonomousReadiness;if(!observer)return null;if(['failure','before-document-navigation','before-active-service-restart'].includes(reason))observer.invalidate(reason==='failure'?'action-failed':'source-changed');return observer.snapshot();},reason);
}
export async function endReadiness(page){return page.evaluate(()=>window.__luciAutonomousReadiness?.close()??null);}
