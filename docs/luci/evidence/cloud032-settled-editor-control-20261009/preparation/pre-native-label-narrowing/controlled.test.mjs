import assert from 'node:assert/strict';
import {test} from 'node:test';
import {createRequire} from 'node:module';
import {ARM_ORDERS,CONTROL_LIMITS,installSettledBlock,evaluateSettledContext,closeSettledBlock,runSettledControl} from './settled-control.mjs';
const require=createRequire('/workspace/luci/desktop/package.json');const {JSDOM}=require('jsdom');
const owners=Array.from({length:4},(_,i)=>({owner:{id:'p-synthetic-'+i,name:'Synthetic '+i},goal:'PRIVATE_SYNTHETIC_VALUE_'+i}));
function fixture(){
 const dom=new JSDOM('<!doctype html><body><div id="root"></div></body>',{runScripts:'outside-only',pretendToBeVisual:true});const w=dom.window;
 const render=owner=>{w.document.getElementById('root').innerHTML='<div class="shell"><nav id="project-navigation"><div role="treeitem" data-kind="project" aria-selected="true"></div></nav><main class="shell-main"><div class="shell-workspace"><div class="shell-lens"><section class="definition-workspace"><header><h1>Define your project</h1></header><div class="definition-specification"><label class="luci-field">The idea<textarea></textarea></label></div></section></div></div></main></div>';const row=w.document.querySelector('[role=treeitem]');row.setAttribute('data-id',owner.owner.id);row.setAttribute('aria-label',owner.owner.name);w.document.querySelector('textarea').value=owner.goal;};render(owners[0]);
 const execute=(fn,...args)=>{w.__controlTestArgs=args;try{return w.eval('('+fn.toString()+')(...window.__controlTestArgs)');}finally{delete w.__controlTestArgs;}};
 const el=()=>w.document.querySelector('textarea');
 return {w,dom,render,execute,el,config:{generation:1,authorityGeneration:1,projectID:owners[0].owner.id,projectName:owners[0].owner.name,expected:owners[0].goal}};
}
test('real DOM contract sets exact retained reference; callback returns only allowed metadata; close removes owned registry',()=>{
 const f=fixture();try{const setup=f.execute(installSettledBlock,f.el(),f.config);assert.equal(setup.generation,1);for(let i=1;i<=4;i++){const row=f.execute(evaluateSettledContext,f.el(),1);assert.equal(row.status,'observed');assert.equal(row.call,i);assert.equal(row.samples,1);assert.equal(row.sameElement,true);assert(!JSON.stringify(row).includes('PRIVATE_'));}const end=f.execute(closeSettledBlock,1);assert.equal(end.closed,true);assert.equal(end.calls,4);assert.equal(f.w.__luciSettledEvaluateControl,undefined);}finally{f.dom.window.close();}
});
for(const [label,change,reason] of [
 ['same-value replacement',f=>{const clone=f.el().cloneNode(true);clone.value=owners[0].goal;f.el().replaceWith(clone);},'replaced-editor'],
 ['wrong project',f=>f.w.document.querySelector('[role=treeitem]').setAttribute('aria-selected','false'),'changed-selection'],
 ['changed saved value',f=>{f.el().value='PRIVATE_DIFFERENT';},'changed-context'],
 ['lost native permission',f=>{f.el().disabled=true;},'lost-editability'],
 ['readonly editor',f=>{f.el().readOnly=true;},'lost-editability'],
 ['hidden ancestor',f=>f.w.document.querySelector('section').setAttribute('hidden',''),'lost-editability'],
])test('refuses '+label+' without polling/reacquire',()=>{const f=fixture();try{f.execute(installSettledBlock,f.el(),f.config);change(f);const row=f.execute(evaluateSettledContext,f.el(),1);assert.equal(row.status,'refused');assert.equal(row.reason,reason);assert.equal(row.samples,1);assert(!JSON.stringify(row).includes('PRIVATE_'));}finally{f.dom.window.close();}});
test('generation, per-block call cap and page-leaving refuse late use',()=>{const f=fixture();try{f.execute(installSettledBlock,f.el(),f.config);assert.equal(f.execute(evaluateSettledContext,f.el(),2).reason,'inactive-generation');for(let i=0;i<4;i++)f.execute(evaluateSettledContext,f.el(),1);assert.equal(f.execute(evaluateSettledContext,f.el(),1).reason,'call-cap');f.w.dispatchEvent(new f.w.Event('pagehide'));assert.equal(f.execute(evaluateSettledContext,f.el(),1).reason,'inactive-generation');}finally{f.dom.window.close();}});
function fakePage(options={}){
 const f=fixture(),calls=[];let fieldWaits=0,measured=0,disposed=0,resolveLate;
 const field={
  waitFor:async()=>{calls.push('wait');if(options.late&&++fieldWaits===2)await new Promise(r=>{resolveLate=r;});},
  elementHandle:async()=>{calls.push('acquire');const retained=f.el();return {evaluate:async(fn,arg)=>{if(fn===evaluateSettledContext){calls.push('handle-evaluate');measured++;}return f.execute(fn,retained,arg);},dispose:async()=>{calls.push('dispose');disposed++;}};},
  evaluate:async(fn,arg)=>{calls.push('locator-evaluate');measured++;if(options.replaceFirst&&measured===1){const clone=f.el().cloneNode(true);clone.value=owners[0].goal;f.el().replaceWith(clone);}return f.execute(fn,f.el(),arg);},
  inputValue:async()=>{calls.push('inputValue');return f.el().value;},
 };
 const page={getByRole:(role,query)=>role==='textbox'?field:role==='heading'?{waitFor:async()=>calls.push('heading')}:{click:async()=>{calls.push('click');if(query.name.startsWith('Open project '))f.render(owners.find(x=>'Open project '+x.owner.name===query.name));}},waitForFunction:async(fn,arg)=>{assert(f.execute(fn,arg));},evaluate:async(fn,arg)=>f.execute(fn,arg)};
 return {page,close:()=>f.dom.window.close(),calls,resolveLate:()=>resolveLate?.(),get measured(){return measured;},get disposed(){return disposed;}};
}
test('32 balanced exact same-callback comparisons preserve surrounding reads and release eight handles',async()=>{
 const f=fakePage(),receipt={};try{await runSettledControl({page:f.page,owners,receipt,assertHealthy:()=>{}});const out=receipt.settledEvaluateControl;assert.equal(out.status,'completed');assert.equal(out.rows.length,32);assert.equal(out.blocks.length,8);assert.equal(f.disposed,8);assert.equal(f.measured,32);assert.equal(f.calls.filter(x=>x==='wait').length,36);assert.equal(f.calls.filter(x=>x==='inputValue').length,32);assert.deepEqual(out.rows.map(x=>x.arm),Array.from({length:4},()=>ARM_ORDERS.flat()).flat());assert.equal(out.rows.filter(x=>x.arm==='locator').length,16);assert(out.rows.every(x=>x.success&&x.callback.samples===1&&x.callback.sameElement));assert(!JSON.stringify(out).includes('PRIVATE_'));assert.equal(out.lateCallbacks,0);assert.equal(CONTROL_LIMITS.phaseMs,60000);}finally{f.close();}
});
test('same-value replacement ends the phase on the failed first call; retains row and releases handle',async()=>{
 const f=fakePage({replaceFirst:true}),receipt={};try{await assert.rejects(runSettledControl({page:f.page,owners,receipt,assertHealthy:()=>{}}));const out=receipt.settledEvaluateControl;assert.equal(out.status,'failed');assert.equal(out.rows.length,1);assert.equal(out.rows[0].success,false);assert.equal(out.rows[0].failureReason,'replaced-editor');assert.equal(f.measured,1);assert.equal(f.disposed,1);}finally{f.close();}
});
test('timed-out wait cannot issue evaluation later; virtual deadline only, not a performance test',async()=>{
 const f=fakePage({late:true}),receipt={},original=setTimeout;globalThis.setTimeout=(fn,ms,...args)=>original(fn,ms>=1000?1:ms,...args);
 try{await assert.rejects(runSettledControl({page:f.page,owners,receipt,assertHealthy:()=>{}}));assert.equal(f.measured,0);f.resolveLate();await new Promise(r=>original(r,5));assert.equal(f.measured,0);assert.equal(receipt.settledEvaluateControl.rows.length,1);assert.equal(receipt.settledEvaluateControl.status,'failed');assert.equal(receipt.settledEvaluateControl.lateCallbacks,1);assert.equal(f.disposed,1);}finally{globalThis.setTimeout=original;f.close();}
});
