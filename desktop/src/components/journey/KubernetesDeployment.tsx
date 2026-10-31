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
import { TelemetryReadings, telemetryDiscussion } from './TelemetryReadings';
import { OperationalView } from './OperationalView';

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
  const [publicRouteAck,setPublicRouteAck]=useState(false);
  const [telemetryAck,setTelemetryAck]=useState(false),[telemetryReading,setTelemetryReading]=useState(false);
  const discussionScope=useProjectDraftScope(projectId);
  const controller=useRef<AbortController|null>(null),epoch=useRef(0),mounted=useRef(true);
  const binding=JSON.stringify([saved?.revision,saved?.digest,draftMatchesSaved]);
  const owner=useRef(binding); if (owner.current!==binding) { owner.current=binding; epoch.current++; controller.current?.abort(); }
  const retire=()=>{epoch.current++;controller.current?.abort();controller.current=null;setBusy(false);setTelemetryReading(false);};
  useEffect(()=>{setPublicRouteAck(false);},[binding]);
  const clearSecretDraft=()=>{retire();setConnection(initialTarget());setReview(null);setAck(false);setTelemetryAck(false);};
  useUnsavedProjectDraft(projectId,!!connection.id || !!connection.name || !!connection.server || !!connection.namespace || !!connection.credentials.ca || !!connection.credentials.token || !!connection.credentials.certificate || !!connection.credentials.key,'Cluster connection setup','deploy',{retained:false,discard:clearSecretDraft});
  useEffect(()=>{mounted.current=true;return()=>{mounted.current=false;epoch.current++;controller.current?.abort();};},[]);
  useEffect(()=>{setReview(null);setAck(false);setTelemetryAck(false);setBusy(false);setTelemetryReading(false);},[binding]);
  function invalidate() { retire();setReview(null);setAck(false);setTelemetryAck(false);setError(''); }
  async function execute(work:(signal:AbortSignal,current:()=>boolean)=>Promise<void>) {
    if (!allowed || busy) return;
    const request=new AbortController(),ticket=++epoch.current;controller.current?.abort();controller.current=request;setBusy(true);setError('');
    const current=()=>mounted.current && !request.signal.aborted && epoch.current===ticket;
    try {await work(request.signal,current);} catch(cause) {if(current())setError(errorText(cause));}
    finally {if(current()){controller.current=null;setBusy(false);setTelemetryReading(false);}}
  }
  async function load() { await execute(async(signal,current)=>{
    const rows=await kubernetesDeploymentApi.targets(projectId,signal);if(!current())return;
    const history=await kubernetesDeploymentApi.list(projectId,signal);
    if(!current())return;setTargets(rows);setDeployments(history);setLoaded(true);setUncertain(false);setReview(null);setAck(false);setTelemetryAck(false);setNotice('Read saved targets and retained deployments. No cluster operation was started.');
  }); }
  async function loadRuns() { await execute(async(signal,current)=>{
    const rows=await projectRunsApi.list(projectId,signal);if(!current())return;
    if(rows.some(row=>!Array.isArray(row.input.implementation.taskTitles) || row.input.implementation.taskTitles.length>200 || row.input.implementation.taskTitles.some(title=>typeof title!=='string' || title.length>2000) || !Number.isFinite(Date.parse(row.createdAt))))throw new Error('Recorded work could not be confirmed for this project.');
    const result=await productApi.checks(projectId,signal);
    if(!current())return;setRuns(rows);setChecks(result.runs.map(row=>({id:row.id,commit:row.commit,status:row.status})));setRun('');setCheck('');setReview(null);setAck(false);setTelemetryAck(false);
  }); }
  async function saveConnection() { if(!canSetCredentials || uncertain)return;await execute(async(signal,current)=>{
    try {
      const value=await kubernetesDeploymentApi.saveTarget(projectId,connection,signal);
      if(!current())return;setConnection(initialTarget());setTargets(rows=>[...rows.filter(row=>row.id!==value.id),value]);setTarget(value.id);setReview(null);setAck(false);setTelemetryAck(false);setNotice('Cluster connection saved after inspecting its existing namespace. Choose Use target in blueprint, then save the blueprint.');
    } catch(cause) {if(current())setUncertain(true);throw cause;}
  }); }
  const chooseTarget=(id:string)=>{invalidate();setTarget(id);};
  async function prepare() {if(!saved || !draftMatchesSaved || uncertain)return;await execute(async(signal,current)=>{
    const value=await kubernetesDeploymentApi.review(projectId,{targetId:target,blueprintRevision:saved.revision,blueprintDigest:saved.digest,runId:run,checkId:check,monthlyBudgetUsd:Number(budget),...(saved.blueprint.publicRoute?{acknowledgePublicRoute:publicRouteAck}:{})},signal);
    if(current()){setReview(value);setAck(false);setTelemetryAck(false);setNotice('Review the exact resources below. Nothing has been applied.');}
  });}
  async function action(row:KubernetesReview,mode:'apply'|'observe') {
    if(mode==='apply' && (!ack || uncertain))return;
    if(mode==='apply'&&row.request.blueprint.publicRoute&&!publicRouteAck)return;
    if(mode==='apply'&&row.request.blueprint.services.some(service=>service.collector)&&!telemetryAck)return;
    await execute(async(signal,current)=>{
      // Withdraw Apply before sending. Lost responses can only be reconciled by
      // reading/observing the original; this component never repeats Apply.
      if(mode==='apply'){setUncertain(true);setReview(null);setAck(false);setTelemetryAck(false);}
      const value=await kubernetesDeploymentApi.action(projectId,row,mode,mode==='apply' && ack,signal,mode==='apply'&&telemetryAck);
      if(current()){setDeployments(rows=>[...rows.filter(item=>item.review.id!==value.review.id),value]);setNotice('Original deployment status read. Replica readiness is separate from application health.');}
    });
  }
  async function readTelemetry(row:KubernetesReview,stored=false) {
    if(!canSetCredentials||busy)return;setTelemetryReading(true);
    await execute(async(signal,current)=>{const value=await kubernetesDeploymentApi.telemetry(projectId,row,signal,stored);if(current()){setDeployments(rows=>[...rows.filter(item=>item.review.id!==value.review.id),value]);setNotice('An attributed product-event sample is retained. No workload was started or changed.');}});
  }
  function exportTelemetry(row:KubernetesStatus) {
    if(!row.operation?.telemetry)return;const url=URL.createObjectURL(new Blob([JSON.stringify({projectId,operationId:row.review.id,observation:row.operation.telemetry},null,2)],{type:'application/json'}));const link=document.createElement('a');link.href=url;link.download='luci-product-observation.json';link.click();URL.revokeObjectURL(url);
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
      {saved?.blueprint.publicRoute&&<label><input type="checkbox" checked={publicRouteAck} onChange={event=>setPublicRouteAck(event.target.checked)}/>I authorize the saved app route for {saved.blueprint.publicRoute.hostname}{saved.blueprint.publicRoute.path} through the operator-owned Traefik websecure controller and existing TLS Secret. DNS/controller setup remains my responsibility; no DNS or certificate creation is requested.</label>}
      <Button type="button" disabled={!saved || !draftMatchesSaved || !selected || !selectedRun || !check || !Number.isFinite(Number(budget)) || Number(budget)<=0 || !!saved?.blueprint.publicRoute&&!publicRouteAck} onClick={()=>void prepare()}>Review cluster deployment</Button>
    </fieldset>
    {busy&&<p role="status">{telemetryReading?'Reading original workload identities and reported event counters. This bounded read can take up to 90 seconds.':'Reading or updating the selected deployment…'}</p>}{busy&&telemetryReading&&<Button type="button" variant="secondary" onClick={()=>{retire();setNotice('This read was canceled. A sample saved before cancellation remains retained; read saved deployments to reconcile. No workload was changed.');}}>Cancel telemetry read</Button>}{error&&<p role="alert">{error}</p>}{notice&&<p role="status">{notice}</p>}
    {uncertain&&<p role="status">An action was attempted or its response is unconfirmed. Read retained deployments, then Observe its original operation. Apply is not retried.</p>}
    {review&&<section aria-label="Reviewed cluster deployment"><h4>Review {review.target.name} / {review.target.namespace}</h4>
      <p>Saved blueprint {review.blueprintRevision} · checked source {review.request.blueprint.source.commit.slice(0,12)}. Review expires {new Date(review.expiresAt).toLocaleTimeString()}.</p>
      <ul>{review.request.blueprint.services.map(service=><li key={service.id}>{service.id}: {service.replicas} replicas; per replica request {service.resources.requestMilliCpu} millicores / {service.resources.requestMemoryMiB} MiB, limit {service.resources.limitMilliCpu} millicores / {service.resources.limitMemoryMiB} MiB{service.resources.gpuCount?`; ${service.resources.gpuCount} GPUs`:''}.</li>)}</ul>
      {review.request.publicRouteBinding&&<details><summary>Reviewed public HTTPS route</summary><p>{review.request.blueprint.publicRoute?.hostname}{review.request.blueprint.publicRoute?.path} → {review.request.blueprint.publicRoute?.serviceId}. Existing class {review.request.publicRouteBinding.className} · UID {review.request.publicRouteBinding.classUid}; TLS Secret {review.request.publicRouteBinding.tlsSecretName} · UID {review.request.publicRouteBinding.tlsSecretUid}. Routing is not a DNS, TLS-handshake or health check.</p></details>}
      <p>Monetary cost is unknown. These workloads can trigger existing provider autoscaling; this connector does not obtain quotes, observe billing or enforce a spending cap.</p>
      <ul>{review.request.expected.map(row=><li key={`${row.kind}/${row.name}`}>{row.kind} {row.name} · {row.uid?`update observed UID ${row.uid}, version ${row.resourceVersion}`:'create if still absent'}</li>)}</ul>
      {(review.request.references?.length??0)>0&&<section aria-label="Reviewed runtime references"><h5>Existing runtime references</h5><ul>{review.request.references?.map(ref=><li key={`${ref.kind}/${ref.name}`}>{ref.name} · {ref.kind==='persistent-volume-claim'?`model volume, ${ref.phase}, ${ref.accessModes?.join(', ')}`:`${ref.kind}, selected keys: ${ref.keys?.join(', ')}`}<details><summary>Reference identity</summary><p>{ref.namespace} · UID {ref.uid} · version {ref.resourceVersion}. Metadata digest {ref.metadataDigest}.</p></details></li>)}</ul><p>Values and model contents were not copied or verified. A changed reference requires a new review before another write.</p></section>}
      <ul>{review.request.blueprint.services.filter(service=>service.health).map(service=><li key={service.id}>{service.id}: startup {service.health?.startup?`${service.health.startup.kind}${service.health.startup.path?` ${service.health.startup.path}`:''}`:'not configured'}; readiness {service.health?.readiness?`${service.health.readiness.kind}${service.health.readiness.path?` ${service.health.readiness.path}`:''}`:'not configured'} on port {service.port}.</li>)}</ul>
      <label><input type="checkbox" checked={ack} disabled={busy} onChange={event=>setAck(event.target.checked)}/>I authorize these workload changes and their resource costs within my ${review.monthlyBudgetUsd} monthly plan. This is not a billing cap or quote. I supplied the image digests; source-to-image provenance is unverified.</label>
      {review.request.blueprint.services.some(service=>service.collector)&&<label><input type="checkbox" checked={telemetryAck} disabled={busy} onChange={event=>setTelemetryAck(event.target.checked)}/>I reviewed the event schema and authorize this Collector to receive application signals and export them to {review.request.blueprint.services.find(service=>service.collector)?.collector?.destination.endpoint}. Application consent, instrumentation and destination retention remain my responsibility.</label>}
      <Button type="button" disabled={!allowed || busy || uncertain || !ack || !!review.request.blueprint.publicRoute&&!publicRouteAck || review.request.blueprint.services.some(service=>service.collector)&&!telemetryAck || Date.now()>=expiry} onClick={()=>void action(review,'apply')}>Apply reviewed workloads</Button>
    </section>}
    {loaded&&deployments.length===0&&<p>No retained cluster deployments are recorded for your account in this project.</p>}
    {deployments.length>0&&<section aria-label="Retained cluster deployments"><h4>Retained deployments</h4><p>Replica readiness is not application health. Unknown outcomes retain original identities and are never replayed. The most recent 50 completed observations and all unresolved operations remain listed.</p>
      <ul>{deployments.slice().reverse().map(row=><li key={row.review.id}><strong>{row.review.target.name} · {row.state==='ready'?'replicas ready; health unverified':row.state}</strong> · {new Date(row.review.createdAt).toLocaleString()}
        <Button type="button" variant="secondary" disabled={!allowed || busy || !row.review.applied} onClick={()=>void action(row.review,'observe')}>Observe original {row.review.id.slice(-8)}</Button>
        {row.review.request.blueprint.services.some(service=>service.collector)&&<Button type="button" variant="secondary" disabled={!canSetCredentials||busy||!row.review.applied} onClick={()=>void readTelemetry(row.review)}>Read recorded event counters</Button>}
        {row.review.request.blueprint.services.some(service=>service.collector?.ownedDestination)&&<Button type="button" variant="secondary" disabled={!canSetCredentials||busy||!row.review.applied} onClick={()=>void readTelemetry(row.review,true)}>Read stored product counters</Button>}
        {row.operation?.telemetry&&<TelemetryReadings row={row.operation.telemetry} currentReferences={row.operation.referenceState==='current'} disabled={!allowed||busy} onDiscuss={()=>{if(!discussionScope||!row.operation?.telemetry)return;window.dispatchEvent(new CustomEvent('luci:orchestrator-open',{detail:{scope:discussionScope,projectId,message:telemetryDiscussion(projectId,row.operation.telemetry)}}));setNotice('The attributed sample is a draft in project Luci. Review it and send explicitly; no model or project effect was started.');}} onExport={()=>exportTelemetry(row)}/>}
        {row.operation?.publicRoute&&<section aria-label="Observed HTTPS routing"><h5>HTTPS routing · {row.operation.publicRoute.state}</h5>{row.operation.publicRoute.state==='controller-reported'&&row.operation.publicRoute.url?<p><a href={row.operation.publicRoute.url} target="_blank" rel="noopener noreferrer">Open controller-reported HTTPS route</a>. Observed {new Date(row.operation.publicRoute.observedAt).toLocaleString()}. Controller addresses: {row.operation.publicRoute.addresses.join(', ')}. DNS, browser certificate trust, public reachability and application health remain unverified.</p>:<p>No public endpoint is confirmed by this original observation. Read its retained state before another reviewed action.</p>}</section>}
        {row.operation&&row.review.applied&&<OperationalView review={row.review} disabled={!allowed||busy}/>}
        {row.operation&&<ul>{row.operation.resources.map(resource=><li key={`${resource.kind}/${resource.name}`}>{resource.kind} {resource.name}: {resource.ready?'ready':resource.attempted?'attempted; not ready or uncertain':'not attempted'}</li>)}</ul>}
        {row.operation?.referenceState&&row.operation.referenceState!=='current'&&<p role="status">Runtime reference check: {row.operation.referenceState}. A fresh review is required before further workload changes.</p>}
        {row.operation?.workloads?.map(workload=><section key={workload.componentId} aria-label={`Observed workload ${workload.componentId}`}><h5>{workload.componentId} · {workload.state==='ready'?'configured readiness passed':workload.state==='unprobed'?'no readiness probe configured':workload.state==='pending'?'waiting for ready Pods':'readiness unknown'}</h5>
          <p>{workload.readyPods} ready / {workload.observedPods} observed owned Pods · {workload.restartCount} recorded restarts{workload.oomObserved?' · OOM observed':''}. Read {new Date(workload.observedAt).toLocaleString()}.</p>
          {workload.omitted&&<p>Objects were omitted or could not be confirmed. Readiness remains unknown.</p>}
          <ul>{workload.reasons.map(reason=><li key={reason}>{workloadReasonLabel(reason)}</li>)}</ul><p className="journey-muted">This reports the configured probe and owned Pod state, not model contents, response quality, application correctness or monetary cost.</p>
        </section>)}
      </li>)}</ul>
    </section>}
  </section>;
}

function workloadReasonLabel(code:string):string {
  const labels:Record<string,string>={'replica-set-not-observed':'Current ReplicaSet not observed','runtime-identity-unconfirmed':'Runtime identity could not be confirmed','scheduling-pending':'Waiting for Kubernetes scheduling','container-status-unavailable':'Container status unavailable','oom-observed':'Container memory limit was exceeded','image-pull-pending':'Image pull is pending or refused','restart-backoff':'Container is backing off after restarts','configuration-unavailable':'Referenced runtime configuration is unavailable','container-waiting':'Container is waiting'};
  return Object.prototype.hasOwnProperty.call(labels,code)?labels[code]:'Observation unavailable';
}
