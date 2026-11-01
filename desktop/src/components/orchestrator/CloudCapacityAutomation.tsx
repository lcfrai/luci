import { useEffect, useRef, useState } from 'react';
import { ProductApiError, errorText } from '../../api/product';
import { capacityInputValid, capacityMoney, capacityMoneyInput, capacityReviewValid, cloudCapacityApi, reviewMatchesInput, sameCapacityPolicy, type CapacityHarness, type CloudCapacityInput, type CloudCapacityReview, type CloudCapacityView } from '../../api/cloudCapacity';
import { useUnsavedProjectDraft } from '../../hooks/useUnsavedProjectDraft';
import { useTeamWorkspace } from '../teams/TeamWorkspaceProvider';
import { Button } from '../fluid/components/button';
import './cloud-capacity.css';

type Props={projectId:string;scope:string;actorId:string;canRead:boolean;canEnable:boolean;blocked?:boolean;onChanged?():void};
type Form={harness:CapacityHarness;maxHosts:string;hostMinutes:string;idleMinutes:string;budget:string;expires:string};
type Pending={version:1;input:CloudCapacityInput;review:CloudCapacityReview};
const localDate=(value:string)=>{const d=new Date(value);return new Date(d.getTime()-d.getTimezoneOffset()*60_000).toISOString().slice(0,16)};
const money=(value:number)=>'$'+capacityMoneyInput(value)+' USD';
const integer=(value:string)=>/^\d+$/.test(value)&&Number.isSafeInteger(Number(value))?Number(value):0;
const minutesText=(seconds:number)=>String(Number((seconds/60).toFixed(6)));
const secondsFromMinutes=(value:string)=>{if(!/^\d+(?:\.\d{1,6})?$/.test(value))return 0;const seconds=Number(value)*60,rounded=Math.round(seconds);return Number.isSafeInteger(rounded)&&Math.abs(seconds-rounded)<0.0001?rounded:0};
const blank:Form={harness:'codex',maxHosts:'1',hostMinutes:'',idleMinutes:'5',budget:'',expires:''};
function formFor(v:CloudCapacityView):Form{
  const p=v.policy,o=v.operator;
  return p?{harness:p.harness,maxHosts:String(p.maxHosts),hostMinutes:minutesText(p.hostSeconds),idleMinutes:minutesText(p.idleSeconds),budget:capacityMoneyInput(p.budgetMicros),expires:localDate(p.expiresAt)}:
    {...blank,hostMinutes:o?minutesText(o.minHostSeconds):'',idleMinutes:o?minutesText(Math.min(300,o.minHostSeconds)):'5',expires:localDate(new Date(Date.now()+4*3600_000).toISOString())};
}
function readPending(key:string,project:string,actor:string,teamId:string):{raw:string|null;value:Pending|null;error:string}{
  let raw:string|null=null;
  try{
    raw=localStorage.getItem(key);if(!raw)return{raw,value:null,error:''};
    if(raw.length>32_768)throw new Error('The retained capacity request is too large. Keep it for recovery before changing capacity.');
    const p=JSON.parse(raw) as Pending;
    if(p.version!==1||!capacityInputValid(p.input)||!capacityReviewValid(p.review,project,actor,teamId)||!reviewMatchesInput(p.review,p.input)||p.input.reviewId!==p.review.reviewId||p.input.reviewDigest!==p.review.digest||!p.input.acknowledgeCost)throw new Error('The retained capacity request cannot be read. Keep it for recovery; refresh the saved policy.');
    return{raw,value:p,error:''};
  }catch(cause){return{raw,value:null,error:errorText(cause)}}
}

export function CloudCapacityAutomation(props:Props){
  const team=useTeamWorkspace(),teamId=team?.teamId??'';
  const [activated,setActivated]=useState(false);
  return <details className="cloud-capacity" onToggle={e=>{if(e.currentTarget.open)setActivated(true)}}>
    <summary>Automatic cloud capacity</summary>
    <p>Let Luci add cloud hosts when approved work needs them, then retire them when idle.</p>
    {activated&&<CapacityBody key={JSON.stringify([props.scope,props.actorId,props.projectId,teamId,props.canRead,props.canEnable])} {...props} teamId={teamId}/>}
  </details>;
}
function CapacityBody({projectId,scope,actorId,canRead,canEnable,blocked=false,onChanged,teamId}:Props & {teamId:string}){
  const storageKey='luci.cloud-capacity-request.v1:'+JSON.stringify([scope,projectId,actorId,teamId]);
  const [initial]=useState(()=>readPending(storageKey,projectId,actorId,teamId));
  const [pending,setPending]=useState(initial.value),pendingRef=useRef(initial.value),rawRef=useRef(initial.raw);
  const [storageError,setStorageError]=useState(initial.error),[clearRecovery,setClearRecovery]=useState(false);
  const [view,setView]=useState<CloudCapacityView|null>(null),viewRef=useRef<CloudCapacityView|null>(null);
  const [form,setForm]=useState<Form>(blank),[dirty,setDirty]=useState(false),dirtyRef=useRef(false);
  const [review,setReview]=useState<CloudCapacityReview|null>(null),[reviewInput,setReviewInput]=useState<CloudCapacityInput|null>(null);
  const [costAccepted,setCostAccepted]=useState(false),[busy,setBusy]=useState(''),[error,setError]=useState(''),[notice,setNotice]=useState('');
  const lifecycle=useRef({mounted:true,epoch:0,controller:null as AbortController|null,working:false});
  useEffect(()=>{lifecycle.current.mounted=true;return()=>{lifecycle.current.mounted=false;lifecycle.current.epoch++;lifecycle.current.controller?.abort()}},[]);
  useUnsavedProjectDraft(projectId,dirty&&!pending,'Cloud capacity limits','develop',{retained:false,discard:()=>reset()});

  function persist(value:Pending|null){
    try{
      if(storageError||localStorage.getItem(storageKey)!==rawRef.current)throw new Error('Another window changed the retained capacity request. Reopen this project to read it.');
      const raw=value?JSON.stringify(value):null;if(raw&&raw.length>32_768)throw new Error('Capacity request exceeds browser recovery storage.');
      if(raw)localStorage.setItem(storageKey,raw);else localStorage.removeItem(storageKey);
      if(localStorage.getItem(storageKey)!==raw)throw new Error('The browser recovery change could not be confirmed. Reopen this project and read the saved capacity policy before continuing.');
      rawRef.current=raw;pendingRef.current=value;setPending(value);
    }catch(cause){setStorageError(errorText(cause));throw cause}
  }
  function clearBrokenRecovery(){
    try{if(!clearRecovery||!viewRef.current||localStorage.getItem(storageKey)!==rawRef.current)throw new Error('Read the current saved policy and confirm the local recovery reset first.');localStorage.removeItem(storageKey);rawRef.current=null;pendingRef.current=null;setPending(null);setStorageError('');setClearRecovery(false);setReview(null);setReviewInput(null);setCostAccepted(false);setNotice('The browser recovery copy was removed. The saved cloud policy and hosts are unchanged.')}catch(cause){setError(errorText(cause))}
  }
  function reset(){if(pendingRef.current)return;setForm(viewRef.current?formFor(viewRef.current):blank);dirtyRef.current=false;setDirty(false);setReview(null);setReviewInput(null);setCostAccepted(false)}
  function change(update:Partial<Form>){if(!canEnable||busy||pending||storageError)return;setForm(v=>({...v,...update}));dirtyRef.current=true;setDirty(true);setReview(null);setReviewInput(null);setCostAccepted(false)}
  function receive(v:CloudCapacityView){
    viewRef.current=v;setView(v);
    const original=pendingRef.current;
    if(original&&v.policy&&(sameCapacityPolicy(v.policy,original.review.policy)||v.policy.revision>original.review.policy.revision)){
      persist(null);dirtyRef.current=false;setDirty(false);setReview(null);setReviewInput(null);setCostAccepted(false);setNotice(v.policy.enabled?'Saved capacity policy read back.':'New provisioning is stopped. Existing owned hosts continue normal retirement.');onChanged?.();
    }
    if(!dirtyRef.current&&!pendingRef.current)setForm(formFor(v));
  }
  async function run(label:string,action:(signal:AbortSignal,current:()=>boolean)=>Promise<void>){
    if(!canRead||lifecycle.current.working)return;
    lifecycle.current.controller?.abort();const c=new AbortController(),epoch=++lifecycle.current.epoch;
    lifecycle.current.controller=c;lifecycle.current.working=true;setBusy(label);setError('');setNotice('');
    const current=()=>lifecycle.current.mounted&&lifecycle.current.epoch===epoch&&!c.signal.aborted;
    try{await action(c.signal,current)}catch(cause){if(current())setError(errorText(cause))}finally{lifecycle.current.working=false;if(current())setBusy('')}
  }
  const refresh=()=>run('Reading capacity…',async(signal,current)=>{const v=await cloudCapacityApi.read(projectId,actorId,signal,teamId);if(current())receive(v)});
  useEffect(()=>{if(canRead)void refresh()},[projectId,scope,actorId,teamId,canRead]);
  function inputFromForm():CloudCapacityInput{
    if(!view?.operator)throw new Error('Complete guided Azure setup before enabling automatic capacity.');
    const o=view.operator,maxHosts=integer(form.maxHosts),seconds=secondsFromMinutes(form.hostMinutes),idle=secondsFromMinutes(form.idleMinutes),budget=capacityMoney(form.budget),expires=Date.parse(form.expires);
    if(maxHosts<1||maxHosts>o.maxHosts||seconds<o.minHostSeconds||seconds>o.maxHostSeconds||idle<1||idle>seconds||budget===null||budget>o.budgetMicros||!Number.isFinite(expires)||expires<=Date.now()+seconds*1000)throw new Error('Choose valid host counts, lifetimes, allowance and an expiry beyond the full host lifetime, within the configured limits.');
    return{harness:form.harness,expectedRevision:view.policy?.revision??0,maxHosts,hostSeconds:seconds,idleSeconds:idle,budgetMicros:budget,expiresAt:new Date(expires).toISOString(),acknowledgeCost:false};
  }
  async function prepare(signal:AbortSignal,current:()=>boolean){
    if(!canEnable||blocked)return;const input=inputFromForm(),r=await cloudCapacityApi.review(projectId,actorId,input,signal,teamId);if(!current())return;
    if(r.policy.profileDigest!==viewRef.current?.operator?.profileDigest)throw new Error('The cloud host profile or estimate changed. Refresh capacity and review the current limits.');
    setReview(r);setReviewInput(input);setCostAccepted(false);setNotice('Review these limits before allowing Luci to add hosts. Preparing this review creates no cloud resource.');
  }
  async function enable(signal:AbortSignal,current:()=>boolean,retry=false){
    if(!canEnable||blocked)return;
    let original=pendingRef.current;
    if(!retry){if(!review||!reviewInput||!costAccepted)return;original={version:1,input:{...reviewInput,reviewDigest:review.digest,reviewId:review.reviewId,acknowledgeCost:true},review};persist(original)}
    if(!original)return;
    try{
      const result=await cloudCapacityApi.enable(projectId,actorId,original.input,original.review,signal,teamId);if(!current())return;
      persist(null);dirtyRef.current=false;setDirty(false);setReview(null);setReviewInput(null);setCostAccepted(false);
      const v={...(viewRef.current??{configured:true,hosts:[]}),policy:result.policy};receive(v);setNotice('Automatic capacity is enabled within your saved limits. Hosts are added only when approved work needs them.');onChanged?.();
    }catch(cause){
      if(current()&&cause instanceof ProductApiError&&String(cause.code)==='cloud_capacity_review_refused'){
        persist(null);setReview(null);setReviewInput(null);setCostAccepted(false);setNotice('The request was refused before a policy write. Refresh capacity, then review your limits again.');
      }
      throw cause;
    }
  }
  async function stop(signal:AbortSignal,current:()=>boolean){
    const fresh=await cloudCapacityApi.read(projectId,actorId,signal,teamId);if(!current())return;receive(fresh);
    if(!fresh.policy?.enabled){setNotice('New provisioning is already stopped.');return}
    await cloudCapacityApi.disable(projectId,fresh.policy.revision,fresh.policy.harness,signal);if(!current())return;
    const v=await cloudCapacityApi.read(projectId,actorId,signal,teamId);if(!current())return;receive(v);setReview(null);setReviewInput(null);setCostAccepted(false);
    setNotice(v.policy?.enabled?'The policy changed again. Refresh and review current capacity.':'New provisioning is stopped. Running tasks finish before their hosts retire.');onChanged?.();
  }
  const operator=view?.operator,active=view?.hosts.filter(h=>!h.retired)??[],retired=view?.hosts.filter(h=>h.retired)??[];
  const editable=canEnable&&!blocked&&!busy&&!pending&&!storageError;
  return <div>
    {!canRead?<p>Project control and private infrastructure access are required to read capacity.</p>:<>
      {error&&<p role="alert">{error}</p>}{storageError&&<div><p role="alert">{storageError}</p><details><summary>Browser recovery</summary><p>Read the saved policy first. Removing this browser copy does not stop automatic capacity or delete any host.</p><label className="setup-check"><input type="checkbox" checked={clearRecovery} onChange={e=>setClearRecovery(e.target.checked)}/>I have read the saved policy and want to remove this browser recovery copy.</label><Button type="button" variant="tertiary" disabled={!!busy||!view||!clearRecovery} onClick={clearBrokenRecovery}>Remove browser recovery copy</Button></details></div>}{notice&&<p role="status">{notice}</p>}
      <div className="journey-actions"><Button type="button" variant="tertiary" disabled={!!busy} onClick={()=>void refresh()}>Refresh capacity</Button>
        <Button type="button" variant="secondary" disabled={!!busy||!view?.policy?.enabled} onClick={()=>void run('Stopping new provisioning…',stop)}>Stop new provisioning</Button></div>
      {busy&&<p role="status">{busy}</p>}
      {view&&!view.configured&&<p>{view.reason||'Automatic capacity is not configured. In Setup → Bring your own cloud, enable elastic hosts while preparing the Azure workspace.'}</p>}
      {view?.policy&&<p>Automatic capacity: <strong>{view.policy.enabled?(Date.parse(view.policy.expiresAt)>Date.now()?'enabled':'expired'):'stopped'}</strong>. Up to {view.policy.maxHosts} extra hosts; expires {new Date(view.policy.expiresAt).toLocaleString()}.</p>}
      {view?.spend&&<div className="cloud-capacity-spend"><p>Project host allowance remaining: <strong>{money(view.spend.policyRemainingMicros)}</strong>. Workspace allowance remaining: <strong>{money(view.spend.operatorRemainingMicros)}</strong>.</p><details><summary>How allowances are counted</summary><p>Project hosts retain {money(view.spend.policyUsedMicros)} in estimates and reservations. Workspace tasks and hosts retain {money(view.spend.operatorUsedMicros)}. These totals accumulate; they are not provider invoices and do not reset monthly. {view.spend.unknown?'Some resource outcomes are unconfirmed, so their estimates remain reserved.':'Only confirmed zero-effect reservations are released.'}</p></details></div>}
      {pending&&<div role="status"><p>The enable outcome is unconfirmed. The original request is retained in this browser. Refresh to read its result or retry that exact request.</p>
        <Button type="button" variant="secondary" disabled={!!busy||!canEnable||blocked||!!storageError} onClick={()=>void run('Recovering original request…',(signal,current)=>enable(signal,current,true))}>Retry original enable</Button></div>}
      {operator&&<>
        <fieldset disabled={!editable}><legend>Capacity limits for this project</legend><div className="cloud-capacity-fields">
          <label>Coding engine<select value={form.harness} onChange={e=>change({harness:e.target.value as CapacityHarness})}><option value="codex">Codex</option><option value="claude-code">Claude Code</option></select></label>
          <label>Maximum extra hosts<input type="number" min={1} max={operator.maxHosts} step={1} value={form.maxHosts} onChange={e=>change({maxHosts:e.target.value})}/><small>Workspace maximum: {operator.maxHosts}.</small></label>
          <label>Host lifetime (minutes)<input type="number" min={minutesText(operator.minHostSeconds)} max={minutesText(operator.maxHostSeconds)} step="any" value={form.hostMinutes} onChange={e=>change({hostMinutes:e.target.value})}/></label>
          <label>Retire after idle (minutes)<input type="number" min={minutesText(1)} step="any" value={form.idleMinutes} onChange={e=>change({idleMinutes:e.target.value})}/></label>
          <label>Host spending allowance (USD)<input type="text" inputMode="decimal" value={form.budget} onChange={e=>change({budget:e.target.value})}/><small>Workspace allowance: {money(operator.budgetMicros)}.</small></label>
          <label>Allow new hosts until<input type="datetime-local" value={form.expires} onChange={e=>change({expires:e.target.value})}/><small>Your local time.</small></label>
        </div></fieldset>
        <p>Configured host estimate: {money(operator.hostCostMicrosPerHour)} per hour. The host lifetime includes up to {minutesText(operator.bootstrapSeconds)} minutes for startup. Task/model costs and Azure charges remain separate; this allowance does not cap the provider bill.</p>
        {!canEnable&&<p>Execution and cloud-spending permission are required to enable new capacity. You can still inspect retained hosts or stop new provisioning.</p>}
        <div className="journey-actions"><Button type="button" variant="secondary" disabled={!editable} onClick={()=>void run('Preparing capacity review…',prepare)}>Review capacity limits</Button>
          <Button type="button" variant="tertiary" disabled={!!busy||!!pending||!dirty} onClick={reset}>Discard limit edits</Button></div>
        {review&&reviewInput&&<div className="cloud-capacity-review"><h4>Enable automatic capacity</h4>
          <p>Allow up to <strong>{review.policy.maxHosts} extra hosts</strong>, each for up to {review.policy.hostSeconds/60} minutes, with a total retained estimate allowance of <strong>{money(review.policy.budgetMicros)}</strong>. New hosts are allowed until {new Date(review.policy.expiresAt).toLocaleString()}.</p>
          <label className="setup-check"><input type="checkbox" checked={costAccepted} disabled={!editable} onChange={e=>setCostAccepted(e.target.checked)}/>I approve these limits and the ongoing provider costs.</label>
          <Button type="button" variant="primary" disabled={!editable||!costAccepted} onClick={()=>void run('Enabling automatic capacity…',(signal,current)=>enable(signal,current))}>Enable automatic capacity</Button>
        </div>}
      </>}
      {!!active.length&&<section aria-label="Retained cloud hosts"><h4>Current hosts</h4><ul className="cloud-capacity-hosts">{active.map(host=><li key={host.id}><strong>{host.state.replaceAll('-',' ')}</strong><span>Retained estimate: {money(host.estimateMicros)}</span><span>Lifetime ends {new Date(host.expiresAt).toLocaleString()}</span><details><summary>Host reference</summary><code>{host.id}</code></details></li>)}</ul><p>Unconfirmed resources remain listed until their original ownership and retirement are resolved.</p></section>}
      {!!retired.length&&<details><summary>{retired.length} retired host{retired.length===1?'':'s'}</summary><ul>{retired.map(h=><li key={h.id}>{h.id} · retained estimate {money(h.estimateMicros)}</li>)}</ul></details>}
    </>}
  </div>;
}
