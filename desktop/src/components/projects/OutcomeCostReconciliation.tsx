import { useEffect, useRef, useState } from 'react';
import { outcomeCostsApi, parseActualCost, formatActualCost, type OutcomeCostInput, type OutcomeCostReceipt, type OutcomeCosts } from '../../api/outcomeCosts';
import { projectRunsApi, type ProjectRun } from '../../api/projectRuns';
import { errorText } from '../../api/product';
import { teamScopeGeneration } from '../../api/teamScope';
import { useProjectAccess } from '../../hooks/useProjectAccess';
import { useProjectDraftScope } from '../../hooks/useProjectDraft';
import { useUnsavedProjectDraft } from '../../hooks/useUnsavedProjectDraft';
import { useTeamWorkspace } from '../teams/TeamWorkspaceProvider';
import { Button } from '../fluid/components/button';
import { useCockpit } from '../../store/store';

const initial=()=>({runId:'',operationId:'',category:'model-api' as OutcomeCostInput['category'],provider:'',currency:'USD',amount:'',billedOn:new Date().toISOString().slice(0,10),sourceFileName:'',sourceSha256:'',sourceRow:'1',providerReference:''});
export function OutcomeCostReconciliation({projectId,active,readOnly=false,onEdit}:{projectId:string;active:boolean;readOnly?:boolean;onEdit?():void}) {
 const access=useProjectAccess(projectId),scope=useProjectDraftScope(projectId),team=useTeamWorkspace();
 const selectedProject=useCockpit(state=>state.selection.projectId);
 const allowed=!!scope&&(readOnly||selectedProject===projectId)&&access.canRead&&access.canEdit&&!!access.permissions?.secrets&&!access.loading&&!access.error&&!team?.accessBlocked;
 const key=JSON.stringify([scope,projectId,allowed,readOnly,team?.teamId,team?.revision,teamScopeGeneration()]);
 return <CostEditor key={key} projectId={projectId} active={active} allowed={allowed} readOnly={readOnly} onEdit={onEdit}/>;
}
function CostEditor({projectId,active,allowed,readOnly,onEdit}:{projectId:string;active:boolean;allowed:boolean;readOnly:boolean;onEdit?():void}) {
 const [value,setValue]=useState<OutcomeCosts|null>(null),[runs,setRuns]=useState<ProjectRun[]>([]),[form,setForm]=useState(initial),[editing,setEditing]=useState('');
 const [dirty,setDirty]=useState(false),[ack,setAck]=useState(false),[baselineNeeded,setBaselineNeeded]=useState(false),[baselineAck,setBaselineAck]=useState(false),[uncertain,setUncertain]=useState(false);
 const [busy,setBusy]=useState(false),[fileBusy,setFileBusy]=useState(false),[error,setError]=useState(''),[notice,setNotice]=useState('');
 const epoch=useRef(0),controller=useRef<AbortController|null>(null),mounted=useRef(true),inFlight=useRef(false),fileInput=useRef<HTMLInputElement|null>(null);
 useEffect(()=>{mounted.current=true;return()=>{mounted.current=false;epoch.current++;controller.current?.abort();};},[]);
 function retire(){epoch.current++;controller.current?.abort();controller.current=null;inFlight.current=false;setBusy(false);setFileBusy(false);if(fileInput.current)fileInput.current.value='';}
 useEffect(()=>{if(!active)retire();},[active]);
 function clear(){retire();setForm(initial());setEditing('');setDirty(false);setAck(false);setBaselineNeeded(false);setBaselineAck(false);if(fileInput.current)fileInput.current.value='';}
 useUnsavedProjectDraft(projectId,dirty,'Bill reconciliation draft','develop',{retained:false,discard:clear});
 async function work(task:(signal:AbortSignal,current:()=>boolean)=>Promise<void>){
  if(!active||!allowed||inFlight.current||fileBusy)return;const request=new AbortController(),ticket=++epoch.current;controller.current?.abort();controller.current=request;inFlight.current=true;setBusy(true);setError('');const current=()=>mounted.current&&!request.signal.aborted&&epoch.current===ticket;
  try{await task(request.signal,current);}catch(cause){if(current())setError(errorText(cause));}finally{if(current()){inFlight.current=false;controller.current=null;setBusy(false);}}
 }
 function update<K extends keyof ReturnType<typeof initial>>(key:K,next:ReturnType<typeof initial>[K]){setForm(old=>({...old,[key]:next}));setDirty(true);setAck(false);}
 async function load(){await work(async(signal,current)=>{
  const ledger=await outcomeCostsApi.get(projectId,signal);if(!current())return;
  if(readOnly){setValue(ledger);return;}
  const history=await projectRunsApi.list(projectId,signal);if(!current())return;
  if(dirty&&value&&ledger.revision!==value.revision){setBaselineNeeded(true);setBaselineAck(false);}setValue(ledger);setRuns(history);setUncertain(false);setNotice('Read your retained bill reconciliations and original operations. Your unsaved draft was preserved.');
 });}
 async function chooseFile(file:File|undefined){
  if(!file||busy||!allowed||!active)return;
  if(file.size>128<<10||!file.name.toLowerCase().endsWith('.csv')||new TextEncoder().encode(file.name).length>128||/[\\/\x00-\x1f\x7f]/.test(file.name)){setError('Choose a CSV billing export of at most 128 KiB. Its contents stay in this window.');if(fileInput.current)fileInput.current.value='';return;}
  const ticket=++epoch.current;setFileBusy(true);setError('');
  try{const raw=await file.arrayBuffer(),digest=[...new Uint8Array(await crypto.subtle.digest('SHA-256',raw))].map(x=>x.toString(16).padStart(2,'0')).join('');if(mounted.current&&epoch.current===ticket){setForm(old=>({...old,sourceFileName:file.name,sourceSha256:digest}));setDirty(true);setAck(false);}}
  catch(cause){if(mounted.current&&epoch.current===ticket)setError(errorText(cause));}
  finally{if(mounted.current&&epoch.current===ticket)setFileBusy(false);}
 }
 function edit(row:OutcomeCostReceipt){
  if(dirty)return;clear();setEditing(row.id);setForm({...row,amount:formatActualCost('',row.amountMicros).trim().replace(/,/g,''),sourceRow:String(row.sourceRow)});setDirty(true);
 }
 async function save(){
  if(!value||!ack||uncertain||baselineNeeded&&!baselineAck)return;
  const amount=parseActualCost(form.amount);if(amount===null)return;
  const input:OutcomeCostInput={runId:form.runId,operationId:form.operationId,category:form.category,provider:form.provider,currency:form.currency,amountMicros:amount,billedOn:form.billedOn,sourceFileName:form.sourceFileName,sourceSha256:form.sourceSha256,sourceRow:Number(form.sourceRow),providerReference:form.providerReference};
  await work(async(signal,current)=>{setUncertain(true);const result=await outcomeCostsApi.save(projectId,{expectedRevision:value.revision,id:editing,remove:false,input,acknowledgeActualBill:true},signal);if(current()){setValue(result);setForm(initial());setEditing('');setDirty(false);setAck(false);setBaselineNeeded(false);setBaselineAck(false);setUncertain(false);if(fileInput.current)fileInput.current.value='';setNotice('Operator-reconciled bill row saved. No provider bill, balance or charge was changed.');}});
 }
 async function remove(row:OutcomeCostReceipt){if(!value||dirty||uncertain)return;await work(async(signal,current)=>{setUncertain(true);const result=await outcomeCostsApi.save(projectId,{expectedRevision:value.revision,id:row.id,remove:true,input:null,acknowledgeActualBill:false},signal);if(current()){setValue(result);setUncertain(false);setNotice('Removed this local reconciliation. The provider charge was not refunded or changed.');}});}
 const locked=!allowed||!active||busy||fileBusy,run=runs.find(row=>row.id===form.runId),operations=run?.operations.filter(row=>row.kind==='session')??[];
 const amount=parseActualCost(form.amount),valid=!!form.runId&&operations.some(row=>row.id===form.operationId)&&!!form.provider.trim()&&/^[A-Z]{3}$/.test(form.currency)&&amount!==null&&!!form.sourceSha256&&/^[1-9][0-9]{0,6}$/.test(form.sourceRow)&&Number(form.sourceRow)<=1e6&&!!form.billedOn;
 return <details className="overview-details"><summary>Your reconciled costs</summary>{!allowed?<p>Current Edit and Secrets access is required to read your private bill reconciliation.</p>:<>
  <p>Record itemized actual bill rows against original operations. Amounts are operator-reconciled; Luci has not verified them with the provider. Missing receipts remain unknown.</p>
  <Button type="button" variant="ghost" disabled={locked} onClick={()=>void load()}>{readOnly?'Read retained receipts':'Read retained receipts and operations'}</Button>
  {value&&<>
   {value.totals.length?<dl className="overview-counts">{value.totals.map(total=><div key={total.currency}><dt>{total.currency} retained receipts</dt><dd>{formatActualCost(total.currency,total.amountMicros)}</dd></div>)}</dl>:<p>No bill rows are reconciled. This is not zero spend.</p>}
   <p>{value.rows.length} retained rows (latest 256){value.omittedRows?`; ${value.omittedRows} earlier rows are outside this window`:''}. Currencies are kept separate. Harness token usage and provider billing verification are unavailable.</p>
   {value.rows.length>0&&<ul>{value.rows.slice().reverse().map(row=><li key={row.id}>{formatActualCost(row.currency,row.amountMicros)} · {row.provider} · {row.category==='model-api'?'model/API bill':'infrastructure bill'} · {row.billedOn}<br/>{row.sourceFileName}, row {row.sourceRow} · original {row.operationId.slice(-8)}{!readOnly&&<><Button type="button" variant="ghost" disabled={locked||dirty||uncertain} onClick={()=>edit(row)}>Edit row</Button><Button type="button" variant="ghost" disabled={locked||dirty||uncertain} onClick={()=>void remove(row)}>Remove reconciliation</Button></>}</li>)}</ul>}
   {readOnly?onEdit&&<Button type="button" variant="ghost" disabled={locked} onClick={onEdit}>Reconcile bills in Develop</Button>:<details><summary>{editing?'Edit the selected reconciliation':'Reconcile a bill row'}</summary><fieldset disabled={locked}>
    <label className="luci-field">Original run<select value={form.runId} onChange={event=>{update('runId',event.target.value);setForm(old=>({...old,runId:event.target.value,operationId:''}));}}><option value="">Choose a retained run</option>{runs.map(row=><option key={row.id} value={row.id}>{row.input.route.harness} · {row.state} · {new Date(row.createdAt).toLocaleString()} · {row.id.slice(-8)}</option>)}</select></label>
    <label className="luci-field">Original operation<select value={form.operationId} onChange={event=>update('operationId',event.target.value)}><option value="">Choose an original operation</option>{operations.map(row=><option key={row.id} value={row.id}>{row.taskId||'Coding session'} · {row.status} · {row.id.slice(-8)}</option>)}</select></label>
    <label className="luci-field">Bill category<select value={form.category} onChange={event=>update('category',event.target.value as OutcomeCostInput['category'])}><option value="model-api">Model / API</option><option value="infrastructure">Infrastructure</option></select></label>
    <label className="luci-field">Billing provider<input value={form.provider} maxLength={64} onChange={event=>update('provider',event.target.value)}/></label>
    <label className="luci-field">Actual amount<input inputMode="decimal" value={form.amount} onChange={event=>update('amount',event.target.value)} placeholder="0.012345"/></label>
    <label className="luci-field">Currency code<input value={form.currency} maxLength={3} onChange={event=>update('currency',event.target.value.toUpperCase())}/></label>
    <label className="luci-field">Bill date<input type="date" value={form.billedOn} onChange={event=>update('billedOn',event.target.value)}/></label>
    <label className="luci-field">CSV billing export<input ref={fileInput} type="file" accept=".csv,text/csv" onChange={event=>void chooseFile(event.target.files?.[0])}/><small>Only its filename, SHA256 and the row you identify are saved; customer-level contents are not uploaded.</small></label>
    {form.sourceFileName&&<p>{form.sourceFileName} · SHA256 <code>{form.sourceSha256}</code></p>}
    <label className="luci-field">Source row number<input inputMode="numeric" value={form.sourceRow} onChange={event=>update('sourceRow',event.target.value)}/></label>
    <label className="luci-field">Provider receipt or usage reference<input value={form.providerReference} maxLength={128} onChange={event=>update('providerReference',event.target.value)}/></label>
    {baselineNeeded&&<label><input type="checkbox" checked={baselineAck} onChange={event=>setBaselineAck(event.target.checked)}/>I reviewed retained revision {value.revision}; use it as the baseline for this draft.</label>}
    <label><input type="checkbox" checked={ack} onChange={event=>setAck(event.target.checked)}/>I matched this itemized actual bill row to the selected original operation. It is not an estimate, rate calculation or duplicate charge.</label>
    <Button type="button" disabled={!valid||!ack||uncertain||baselineNeeded&&!baselineAck} onClick={()=>void save()}>Save operator reconciliation</Button><Button type="button" variant="ghost" onClick={clear}>Clear bill draft</Button>
   </fieldset></details>}
  </>}
  {uncertain&&<p>The change response is unconfirmed. Read retained receipts before another change; this draft has been preserved.</p>}{(busy||fileBusy)&&<p role="status">{fileBusy?'Hashing the selected CSV in this window…':'Reading or saving retained bill rows…'}</p>}{error&&<p role="alert">{error}</p>}{notice&&<p role="status">{notice}</p>}
 </>}</details>;
}
