import { useEffect, useRef, useState } from 'react';
import { kubernetesDeploymentApi, type KubernetesReview, type KubernetesStatus, type KubernetesTarget, type KubernetesTargetInput } from '../../api/kubernetesDeployment';
import type { SavedDeploymentBlueprint } from '../../api/deploymentBlueprint';
import { projectRunsApi, type ProjectRun } from '../../api/projectRuns';
import { productApi, errorText } from '../../api/product';
import { useProjectAccess } from '../../hooks/useProjectAccess';
import { useProjectDraftScope } from '../../hooks/useProjectDraft';
import { useUnsavedProjectDraft } from '../../hooks/useUnsavedProjectDraft';
import { useTeamWorkspace } from '../teams/TeamWorkspaceProvider';
import { teamScopeGeneration } from '../../api/teamScope';
import { Button } from '../fluid/components/button';

const initialTarget = (): KubernetesTargetInput => ({ id:'', expectedRevision:0, name:'', server:'', namespace:'', credentials:{ca:'',token:''} });
export function KubernetesDeployment({projectId,active,saved,draftMatchesSaved,onTarget}: {
  projectId:string; active:boolean; saved:SavedDeploymentBlueprint|null; draftMatchesSaved:boolean; onTarget(target:KubernetesTarget):void;
}) {
  const scope=useProjectDraftScope(projectId), team=useTeamWorkspace(), access=useProjectAccess(projectId);
  const allowed=active && !!scope && access.canRead && access.canEdit && !!access.permissions?.deploy && !access.loading && !access.error && !team?.accessBlocked;
  const key=JSON.stringify([scope,projectId,team?.teamId,team?.revision,teamScopeGeneration(),allowed,!!access.permissions?.secrets]);
  return <KubernetesDeploymentEditor key={key} projectId={projectId} allowed={allowed} canSetCredentials={allowed && !!access.permissions?.secrets} saved={saved} draftMatchesSaved={draftMatchesSaved} onTarget={onTarget}/>;
}
function KubernetesDeploymentEditor({projectId,allowed,canSetCredentials,saved,draftMatchesSaved,onTarget}: {
  projectId:string; allowed:boolean; canSetCredentials:boolean; saved:SavedDeploymentBlueprint|null; draftMatchesSaved:boolean; onTarget(target:KubernetesTarget):void;
}) {
  const [targets,setTargets]=useState<KubernetesTarget[]>([]),[deployments,setDeployments]=useState<KubernetesStatus[]>([]),[loaded,setLoaded]=useState(false);
  const [target,setTarget]=useState(''),[runs,setRuns]=useState<ProjectRun[]>([]),[checks,setChecks]=useState<{id:string;commit:string;status:string}[]>([]),[run,setRun]=useState(''),[check,setCheck]=useState('');
  const [connection,setConnection]=useState(initialTarget),[credentialMode,setCredentialMode]=useState<'token'|'certificate'>('token');
  const [review,setReview]=useState<KubernetesReview|null>(null),[budget,setBudget]=useState(''),[ack,setAck]=useState(false),[busy,setBusy]=useState(false),[error,setError]=useState(''),[notice,setNotice]=useState(''),[uncertain,setUncertain]=useState(false);
  const controller=useRef<AbortController|null>(null),epoch=useRef(0),mounted=useRef(true);
  const binding=JSON.stringify([saved?.revision,saved?.digest,draftMatchesSaved]);
  const owner=useRef(binding); if (owner.current!==binding) { owner.current=binding; epoch.current++; controller.current?.abort(); }
  const retire=()=>{epoch.current++;controller.current?.abort();controller.current=null;setBusy(false);};
  const clearSecretDraft=()=>{retire();setConnection(initialTarget());setReview(null);setAck(false);};
  useUnsavedProjectDraft(projectId,!!connection.id || !!connection.name || !!connection.server || !!connection.namespace || !!connection.credentials.ca || !!connection.credentials.token || !!connection.credentials.certificate || !!connection.credentials.key,'Cluster connection setup','deploy',{retained:false,discard:clearSecretDraft});
  useEffect(()=>{mounted.current=true;return()=>{mounted.current=false;epoch.current++;controller.current?.abort();};},[]);
  useEffect(()=>{setReview(null);setAck(false);setBusy(false);},[binding]);
  function invalidate() { retire();setReview(null);setAck(false);setError(''); }
  async function execute(work:(signal:AbortSignal,current:()=>boolean)=>Promise<void>) {
    if (!allowed || busy) return;
    const request=new AbortController(),ticket=++epoch.current;controller.current?.abort();controller.current=request;setBusy(true);setError('');
    const current=()=>mounted.current && !request.signal.aborted && epoch.current===ticket;
    try {await work(request.signal,current);} catch(cause) {if(current())setError(errorText(cause));}
    finally {if(current()){controller.current=null;setBusy(false);}}
  }
  async function load() { await execute(async(signal,current)=>{
    const rows=await kubernetesDeploymentApi.targets(projectId,signal);if(!current())return;
    const history=await kubernetesDeploymentApi.list(projectId,signal);
    if(!current())return;setTargets(rows);setDeployments(history);setLoaded(true);setUncertain(false);setReview(null);setAck(false);setNotice('Read saved targets and retained deployments. No cluster operation was started.');
  }); }
  async function loadRuns() { await execute(async(signal,current)=>{
    const rows=await projectRunsApi.list(projectId,signal);if(!current())return;
    if(rows.some(row=>!Array.isArray(row.input.implementation.taskTitles) || row.input.implementation.taskTitles.length>200 || row.input.implementation.taskTitles.some(title=>typeof title!=='string' || title.length>2000) || !Number.isFinite(Date.parse(row.createdAt))))throw new Error('Recorded work could not be confirmed for this project.');
    const result=await productApi.checks(projectId,signal);
    if(!current())return;setRuns(rows);setChecks(result.runs.map(row=>({id:row.id,commit:row.commit,status:row.status})));setRun('');setCheck('');setReview(null);setAck(false);
  }); }
  async function saveConnection() { if(!canSetCredentials || uncertain)return;await execute(async(signal,current)=>{
    try {
      const value=await kubernetesDeploymentApi.saveTarget(projectId,connection,signal);
      if(!current())return;setConnection(initialTarget());setTargets(rows=>[...rows.filter(row=>row.id!==value.id),value]);setTarget(value.id);setReview(null);setAck(false);setNotice('Cluster connection saved after inspecting its existing namespace. Choose Use target in blueprint, then save the blueprint.');
    } catch(cause) {if(current())setUncertain(true);throw cause;}
  }); }
  const chooseTarget=(id:string)=>{invalidate();setTarget(id);};
  async function prepare() {if(!saved || !draftMatchesSaved || uncertain)return;await execute(async(signal,current)=>{
    const value=await kubernetesDeploymentApi.review(projectId,{targetId:target,blueprintRevision:saved.revision,blueprintDigest:saved.digest,runId:run,checkId:check,monthlyBudgetUsd:Number(budget)},signal);
    if(current()){setReview(value);setAck(false);setNotice('Review the exact resources below. Nothing has been applied.');}
  });}
  async function action(row:KubernetesReview,mode:'apply'|'observe') {
    if(mode==='apply' && (!ack || uncertain))return;
    await execute(async(signal,current)=>{
      // Withdraw Apply before sending. Lost responses can only be reconciled by
      // reading/observing the original; this component never repeats Apply.
      if(mode==='apply'){setUncertain(true);setReview(null);setAck(false);}
      const value=await kubernetesDeploymentApi.action(projectId,row,mode,mode==='apply' && ack,signal);
      if(current()){setDeployments(rows=>[...rows.filter(item=>item.review.id!==value.review.id),value]);setNotice('Original deployment status read. Replica readiness is separate from application health.');}
    });
  }
  const selected=targets.find(row=>row.id===target),selectedRun=runs.find(row=>row.id===run);
  const expiry=review ? Date.parse(review.expiresAt) : 0;
  const editConnection=(patch:Partial<KubernetesTargetInput>)=>{invalidate();setConnection(value=>({...value,...patch}));};
  return <section className="blueprint-result" aria-label="Deploy to an existing Kubernetes cluster">
    <h3>Deploy to your existing cluster</h3>
    <p>Connect an existing namespace, review a saved blueprint against a checked run, then apply its application and inference workloads. Images and model volumes must already exist.</p>
    <p className="journey-muted">This does not create a cluster, namespace, model, GPU capacity or analytics service. Image digests are supplied by you; Luci does not verify that they were built from the checked source.</p>
    {!allowed && <p role="status">Current project edit and Deploy permission are required. Saving connection credentials also requires Secrets permission.</p>}
    <Button type="button" variant="secondary" disabled={!allowed || busy} onClick={()=>void load()}>Read saved targets and deployments</Button>
    <details><summary>Set up or rotate a cluster connection</summary>
      <p>Credentials stay in private controller state and this unsaved window only. HTTPS with an explicit CA is required; kubeconfig plugins and ambient credentials are not used.</p>
      <fieldset disabled={!canSetCredentials || busy || uncertain}><legend>Explicit cluster access</legend>
        <label className="luci-field">Connection to update<select value={connection.id} onChange={event=>{
          const value=targets.find(row=>row.id===event.target.value);invalidate();setConnection(value?{...initialTarget(),id:value.id,expectedRevision:value.revision,name:value.name,server:value.server,namespace:value.namespace}:initialTarget());
        }}><option value="">New connection</option>{targets.map(row=><option key={row.id} value={row.id}>{row.name} · revision {row.revision}</option>)}</select></label>
        <label className="luci-field">Connection name<input value={connection.name} maxLength={120} onChange={event=>editConnection({name:event.target.value})}/></label>
        <label className="luci-field">Kubernetes HTTPS API URL<input value={connection.server} type="url" maxLength={2048} onChange={event=>editConnection({server:event.target.value})}/></label>
        <label className="luci-field">Existing namespace<input value={connection.namespace} maxLength={63} onChange={event=>editConnection({namespace:event.target.value})}/></label>
        <label className="luci-field">Cluster CA certificate PEM<textarea autoComplete="off" value={connection.credentials.ca} maxLength={65536} onChange={event=>editConnection({credentials:{...connection.credentials,ca:event.target.value}})}/></label>
        <label className="luci-field">Authentication<select value={credentialMode} onChange={event=>{setCredentialMode(event.target.value==='token'?'token':'certificate');editConnection({credentials:{ca:connection.credentials.ca}});}}><option value="token">Bearer token</option><option value="certificate">Client certificate and key</option></select></label>
        {credentialMode==='token'?<label className="luci-field">Cluster bearer token<input type="password" autoComplete="off" value={connection.credentials.token??''} maxLength={16384} onChange={event=>editConnection({credentials:{...connection.credentials,token:event.target.value}})}/></label>:<>
          <label className="luci-field">Client certificate PEM<textarea autoComplete="off" maxLength={65536} value={connection.credentials.certificate??''} onChange={event=>editConnection({credentials:{...connection.credentials,certificate:event.target.value}})}/></label>
          <label className="luci-field">Client private key PEM<textarea autoComplete="off" maxLength={65536} value={connection.credentials.key??''} onChange={event=>editConnection({credentials:{...connection.credentials,key:event.target.value}})}/></label>
        </>}
        <Button type="button" disabled={!connection.name || !connection.server || !connection.namespace || !connection.credentials.ca} onClick={()=>void saveConnection()}>Inspect namespace and save connection</Button>
        <Button type="button" variant="tertiary" onClick={clearSecretDraft}>Clear unsaved credentials</Button>
      </fieldset>
    </details>
    <fieldset disabled={!allowed || busy || uncertain}><legend>Choose a deployment</legend>
      <label className="luci-field">Saved cluster target<select value={target} onChange={event=>chooseTarget(event.target.value)}><option value="">Choose a target</option>{targets.map(row=><option key={row.id} value={row.id}>{row.name} · {row.namespace} · revision {row.revision}</option>)}</select></label>
      <Button type="button" variant="secondary" disabled={!selected} onClick={()=>{if(selected){invalidate();onTarget(selected);}}}>Use target in blueprint</Button>
      {!saved || !draftMatchesSaved?<p>Save the current blueprint before reviewing a deployment.</p>:<p>Use saved blueprint revision {saved.revision}. Its cluster reference and namespace must match this target.</p>}
      <Button type="button" variant="secondary" onClick={()=>void loadRuns()}>Load checked work</Button>
      <label className="luci-field">Project run<select value={run} onChange={event=>{invalidate();setRun(event.target.value);setCheck('');}}><option value="">Choose recorded work</option>{runs.map(row=><option key={row.id} value={row.id}>{row.input.implementation.taskTitles[0]||'Recorded work'} · {new Date(row.createdAt).toLocaleString()} · {row.id.slice(0,12)}</option>)}</select></label>
      <label className="luci-field">Passing check<select value={check} onChange={event=>{invalidate();setCheck(event.target.value);}}><option value="">Choose a passing source check</option>{checks.filter(row=>row.status==='passed').map(row=><option key={row.id} value={row.id}>{row.commit.slice(0,12)} · {row.id}</option>)}</select></label>
      <p className="journey-muted">The server verifies the check covers this run’s exact reviewed requirements and current clean source. Recorded labels alone are not approval.</p>
      <label className="luci-field">Reviewed monthly budget / authority ceiling (USD)<input type="number" min={1} max={1000000} step="0.01" value={budget} onChange={event=>{invalidate();setBudget(event.target.value);}}/></label>
      <Button type="button" disabled={!saved || !draftMatchesSaved || !selected || !selectedRun || !check || !Number.isFinite(Number(budget)) || Number(budget)<=0} onClick={()=>void prepare()}>Review cluster deployment</Button>
    </fieldset>
    {busy&&<p role="status">Reading or updating the selected deployment…</p>}{error&&<p role="alert">{error}</p>}{notice&&<p role="status">{notice}</p>}
    {uncertain&&<p role="status">An action was attempted or its response is unconfirmed. Read retained deployments, then Observe its original operation. Apply is not retried.</p>}
    {review&&<section aria-label="Reviewed cluster deployment"><h4>Review {review.target.name} / {review.target.namespace}</h4>
      <p>Saved blueprint {review.blueprintRevision} · checked source {review.request.blueprint.source.commit.slice(0,12)}. Review expires {new Date(review.expiresAt).toLocaleTimeString()}.</p>
      <ul>{review.request.blueprint.services.map(service=><li key={service.id}>{service.id}: {service.replicas} replicas; per replica request {service.resources.requestMilliCpu} millicores / {service.resources.requestMemoryMiB} MiB, limit {service.resources.limitMilliCpu} millicores / {service.resources.limitMemoryMiB} MiB{service.resources.gpuCount?`; ${service.resources.gpuCount} GPUs`:''}.</li>)}</ul>
      <p>Monetary cost is unknown. These workloads can trigger existing provider autoscaling; this connector does not obtain quotes, observe billing or enforce a spending cap.</p>
      <ul>{review.request.expected.map(row=><li key={`${row.kind}/${row.name}`}>{row.kind} {row.name} · {row.uid?`update observed UID ${row.uid}, version ${row.resourceVersion}`:'create if still absent'}</li>)}</ul>
      <label><input type="checkbox" checked={ack} disabled={busy} onChange={event=>setAck(event.target.checked)}/>I authorize these workload changes and their resource costs within my ${review.monthlyBudgetUsd} monthly plan. This is not a billing cap or quote. I supplied the image digests; source-to-image provenance is unverified.</label>
      <Button type="button" disabled={!allowed || busy || uncertain || !ack || Date.now()>=expiry} onClick={()=>void action(review,'apply')}>Apply reviewed workloads</Button>
    </section>}
    {loaded&&deployments.length===0&&<p>No retained cluster deployments are recorded for your account in this project.</p>}
    {deployments.length>0&&<section aria-label="Retained cluster deployments"><h4>Retained deployments</h4><p>Replica readiness is not application health. Unknown outcomes retain original identities and are never replayed. The most recent 50 completed observations and all unresolved operations remain listed.</p>
      <ul>{deployments.slice().reverse().map(row=><li key={row.review.id}><strong>{row.review.target.name} · {row.state==='ready'?'replicas ready; health unverified':row.state}</strong> · {new Date(row.review.createdAt).toLocaleString()}
        <Button type="button" variant="secondary" disabled={!allowed || busy || !row.review.applied} onClick={()=>void action(row.review,'observe')}>Observe original {row.review.id.slice(-8)}</Button>
        {row.operation&&<ul>{row.operation.resources.map(resource=><li key={`${resource.kind}/${resource.name}`}>{resource.kind} {resource.name}: {resource.ready?'ready':resource.attempted?'attempted; not ready or uncertain':'not attempted'}</li>)}</ul>}
      </li>)}</ul>
    </section>}
  </section>;
}
