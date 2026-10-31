import {useEffect,useRef,useState} from 'react';
import {productApi,errorText,type CloudPlan,type CloudPlanRequest} from '../../api/product';
import type {DeploymentBlueprintPlan} from '../../api/deploymentBlueprint';
import {useProjectAccess} from '../../hooks/useProjectAccess';
import {useUnsavedProjectDraft} from '../../hooks/useUnsavedProjectDraft';
import {CloudManager} from '../setup/CloudManager';
import {Button} from '../fluid/components/button';

export function AzureTelemetryStorageHandoff({plan,active}:{plan:DeploymentBlueprintPlan;active:boolean}){
 const access=useProjectAccess(plan.blueprint.projectId),allowed=active&&access.canEdit&&access.permissions.deploy&&access.permissions.secrets;
 const store=plan.blueprint.services.find(s=>s.telemetryStorage?.azure),blob=store?.telemetryStorage?.azure;
 const [fields,setFields]=useState({subscriptionId:'',tenantId:'',name:'',region:'',budget:0}),[prepared,setPrepared]=useState<CloudPlan|null>(null),[busy,setBusy]=useState(false),[dirty,setDirty]=useState(false),[error,setError]=useState('');
 const pending=useRef<AbortController|null>(null),epoch=useRef(0),mounted=useRef(true),currentAccess=useRef(allowed);currentAccess.current=allowed;
 function invalidate(){epoch.current++;pending.current?.abort();pending.current=null;setPrepared(null);setBusy(false)}
 function discard(){invalidate();setFields({subscriptionId:'',tenantId:'',name:'',region:'',budget:0});setDirty(false);setError('')}
 useUnsavedProjectDraft(plan.blueprint.projectId,dirty,'Trace storage request','deploy',{retained:false,discard});
 useEffect(()=>{mounted.current=true;return()=>{mounted.current=false;epoch.current++;pending.current?.abort()}},[]);
 useEffect(()=>{invalidate()},[allowed,plan.digest]);
 function change(value:Partial<typeof fields>){invalidate();setFields(old=>({...old,...value}));setDirty(true);setError('')}
 async function review(){
  if(!allowed||!blob||!store?.telemetryStorage||busy)return;
  const controller=new AbortController(),ticket=++epoch.current;pending.current?.abort();pending.current=controller;setBusy(true);setError('');setPrepared(null);
  const current=()=>mounted.current&&currentAccess.current&&!controller.signal.aborted&&epoch.current===ticket;
  const request:CloudPlanRequest={provider:'azure',name:fields.name,region:fields.region,monthlyBudgetUsd:fields.budget,domain:'',size:'',sshPublicKey:'',foundation:{blueprint:plan.blueprint,expectedBlueprintDigest:plan.digest},telemetryStorage:{subscriptionId:fields.subscriptionId,tenantId:fields.tenantId,accountName:blob.accountName,containerName:blob.containerName,retentionDays:Math.ceil(store.telemetryStorage.retentionHours/24)}};
  try{const value=await productApi.cloudPlan(request,controller.signal);if(!current())return;const binding=value.foundation;
   if(value.kind!=='telemetry-storage'||value.provider!=='azure'||value.name!==request.name||value.region!==request.region||!binding?.telemetryStorage||binding.request.expectedBlueprintDigest!==plan.digest||binding.actor.projectId!==plan.blueprint.projectId||JSON.stringify(binding.telemetryStorage.spec)!==JSON.stringify(request.telemetryStorage)||binding.fit.tier!=='telemetry-blob'||!binding.fit.fits||binding.fit.capacityVerified!==false||value.estimate.monthly!==null||value.estimate.verified)throw new Error('The returned storage plan does not match this saved project topology. Read the saved blueprint before reviewing again.');
   setPrepared(value);setDirty(false)
  }catch(cause){if(current())setError(errorText(cause))}finally{if(current()){pending.current=null;setBusy(false)}}
 }
 if(!active||!blob)return null;
 return <details className="journey-details"><summary>Create missing Azure trace storage</summary><p>The saved Tempo configuration selects account {blob.accountName||'not chosen'} and container {blob.containerName||'not chosen'}. This separate review creates a dedicated StorageV2 account/private container with authenticated HTTPS and a retention rule, using the existing owned CloudJob path. It does not deploy workloads or install credentials.</p><p>Save this exact blueprint first. Then review storage, explicitly accept its unverified ongoing costs and create it. Resource preparation can privately import this original job’s account key into the selected Kubernetes Secret; no key is returned here. External storage and existing Secret references stay available.</p>
  <fieldset disabled={!allowed||busy}><legend>Existing Azure account and cost authority</legend>{(['subscriptionId','tenantId','name','region'] as const).map(field=><label className="luci-field" key={field}>{({subscriptionId:'Azure subscription ID',tenantId:'Azure tenant ID',name:'New dedicated resource-group name',region:'Azure region'})[field]}<input value={fields[field]} maxLength={field==='subscriptionId'||field==='tenantId'?36:40} onChange={e=>change({[field]:e.target.value})}/></label>)}<label className="luci-field">Monthly planning budget (USD)<input type="number" min={1} max={100000} step="0.01" value={fields.budget||''} onChange={e=>change({budget:Number(e.target.value)})}/></label><Button type="button" disabled={!allowed||busy||!blob.accountName||!blob.containerName} onClick={()=>void review()}>{busy?'Reviewing storage…':'Review dedicated trace storage'}</Button></fieldset>
  {!allowed&&<p>Current project Edit, Deploy and Secrets access is required.</p>}{error&&<p role="alert">{error}</p>}
  {allowed&&prepared?.foundation?.request.expectedBlueprintDigest===plan.digest&&<CloudManager key={prepared.id} client={productApi} plan={prepared} mode="managed" foundationOnly onlyPlanId={prepared.id}/>}
 </details>;
}
