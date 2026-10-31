import { runtimePrerequisitesApi, type RuntimePreparationSummary } from '../../api/runtimePrerequisites';
import { useCallback, useEffect, useRef, useState } from 'react';
import { coordinatorKubernetesApi, validCoordinatorKubernetesPlan, type CoordinatorKubernetesPlan } from '../../api/coordinatorKubernetes';
import { kubernetesDeploymentApi, type KubernetesTarget } from '../../api/kubernetesDeployment';
import { errorText } from '../../api/product';
import { useUnsavedProjectDraft } from '../../hooks/useUnsavedProjectDraft';
import { useProjectAccess } from '../../hooks/useProjectAccess';
import { Button } from '../fluid/components/button';

export interface CoordinatorKubernetesSelection {plan:CoordinatorKubernetesPlan|null;dirty:boolean;problem:string;draftDigest:string}
interface Props {projectId:string;scope:string;disabled:boolean;enabled:boolean;recordedDraft:string;onChange(value:CoordinatorKubernetesSelection):void;onNavigateDeploy():void}
export function CoordinatorKubernetesSetup(props:Props){const access=useProjectAccess(props.projectId);const disabled=props.disabled||access.loading||!!access.error||!access.canRead||!access.canEdit||!access.permissions.deploy||!access.permissions.secrets;return <Setup key={JSON.stringify([props.projectId,props.scope])} {...props} disabled={disabled}/>;}
function Setup({projectId,disabled,enabled,recordedDraft,onChange,onNavigateDeploy}:Props){
 const [preparations,setPreparations]=useState<RuntimePreparationSummary[]>([]),[preparationId,setPreparation]=useState('');
 const [publicRoute,setPublicRoute]=useState(false);
 const [opened,setOpened]=useState(false),[targets,setTargets]=useState<KubernetesTarget[]>([]),[targetId,setTarget]=useState(''),[budget,setBudget]=useState('');
 const [prepared,setPrepared]=useState<CoordinatorKubernetesPlan|null>(null),[cost,setCost]=useState(false),[images,setImages]=useState(false),[telemetry,setTelemetry]=useState(false);
 const [dirty,setDirty]=useState(false),[busy,setBusy]=useState(false),[error,setError]=useState(''),[loaded,setLoaded]=useState(false);
 const mounted=useRef(true),epoch=useRef(0),controller=useRef<AbortController|null>(null),latest=useRef({disabled,enabled,targetId,budget,preparationId});latest.current={disabled,enabled,targetId,budget,preparationId};
 const invalidate=useCallback(()=>{epoch.current++;controller.current?.abort();controller.current=null;setPrepared(null);setCost(false);setImages(false);setTelemetry(false);setPublicRoute(false);setBusy(false);},[]);
 const clear=useCallback(()=>{invalidate();setTarget('');setPreparation('');setBudget('');setDirty(false);setError('');},[invalidate]);
 const digest=JSON.stringify({targetId,budget,preparationId});
 useUnsavedProjectDraft(projectId,dirty,'Kubernetes continuation delegation','develop',{retained:false,discard:clear},'coordinator-kubernetes');
 useEffect(()=>{mounted.current=true;return()=>{mounted.current=false;epoch.current++;controller.current?.abort();};},[]);
 useEffect(()=>{invalidate();setTargets([]);setPreparations([]);setLoaded(false);},[disabled,enabled,invalidate]);
 useEffect(()=>{if(recordedDraft&&recordedDraft===digest)clear();},[recordedDraft,digest,clear]);
 async function read(kind:'targets'|'plan'){
  if(latest.current.disabled||!latest.current.enabled||busy)return;
  const selected=targetId,selectedPreparation=preparationId,amount=Number(budget);if(kind==='plan'&&(!targets.some(t=>t.id===selected)||!Number.isFinite(amount)||amount<=0))return;
  invalidate();const abort=new AbortController(),ticket=++epoch.current;controller.current=abort;setBusy(true);setError('');
  const current=()=>mounted.current&&!abort.signal.aborted&&ticket===epoch.current&&!latest.current.disabled&&latest.current.enabled&&(kind==='targets'||latest.current.targetId===selected&&latest.current.budget===budget&&latest.current.preparationId===selectedPreparation);
  try{
   if(kind==='targets'){const rows=await kubernetesDeploymentApi.targets(projectId,abort.signal);if(!current())return;const plans=await runtimePrerequisitesApi.list(projectId,abort.signal);if(current()){setTargets(rows);setPreparations(plans.filter(p=>!p.finalized));setLoaded(true);setTarget('');setPreparation('');if(selected||selectedPreparation)setDirty(true);}}
   else{const value=await coordinatorKubernetesApi.prepare(projectId,{targetId:selected,monthlyBudgetUsd:amount,...(selectedPreparation?{preparationId:selectedPreparation}:{})},abort.signal);if(current())setPrepared(value);}
  }catch(cause){if(current())setError(errorText(cause));}finally{if(current()){setBusy(false);controller.current=null;}}
 }
 function edit(key:'target'|'budget'|'preparation',value:string){invalidate();setDirty(true);setError('');if(key==='target')setTarget(value);else if(key==='preparation')setPreparation(value);else setBudget(value);}
 const candidate=prepared?{...prepared,acknowledgeCosts:cost,acknowledgeImages:images,acknowledgeTelemetry:telemetry,...(prepared.requiresPublicRoute?{acknowledgePublicRoute:publicRoute}:{})}:null;
 const problem=error||(dirty&&!prepared?'Read and review the current saved namespace, topology and immutable images.':dirty&&!validCoordinatorKubernetesPlan(candidate)?'Acknowledge the exact image provenance, ongoing costs and any telemetry collection.':'');
 const plan=!disabled&&enabled&&!busy&&!problem&&validCoordinatorKubernetesPlan(candidate)?candidate:null;
 const projection=JSON.stringify({plan,dirty,problem,draftDigest:digest});useEffect(()=>onChange(JSON.parse(projection) as CoordinatorKubernetesSelection),[projection,onChange]);
 return <details onToggle={e=>{if(e.currentTarget.open)setOpened(true);}}><summary>Continue to an existing Kubernetes namespace</summary>{opened&&<>
  <p>After the selected implementation tasks and independent checks, use the existing reviewed Kubernetes lifecycle. ClusterIP services do not require public DNS or a Coolify deployment. Missing prerequisites must be reviewed before this delegation can run.</p>
  {!enabled&&<p>Enable existing-target deployment permission first.</p>}
  <Button variant="secondary" disabled={disabled||!enabled||busy} onClick={()=>void read('targets')}>Read Kubernetes namespace targets</Button><Button variant="ghost" onClick={onNavigateDeploy}>Open Deploy</Button>
  {loaded&&!targets.length&&<p>No namespace target is configured. Connect or commission one in Deploy.</p>}
  <label className="luci-field">Kubernetes namespace target<select disabled={disabled||busy} value={targetId} onChange={e=>edit('target',e.target.value)}><option value="">Choose an existing target</option>{targets.map(target=><option key={target.id} value={target.id}>{target.name} · {target.namespace}</option>)}</select></label>
  <label className="luci-field">Image preparation<select disabled={disabled||busy} value={preparationId} onChange={e=>edit('preparation',e.target.value)}><option value="">Use current saved immutable images</option>{preparations.map(p=><option key={p.id} value={p.id}>{p.services.join(', ')} · blueprint {p.blueprintRevision} · {new Date(p.createdAt).toLocaleString()}</option>)}</select><small>Choose an explicit reviewed preparation to run its saved prerequisite tasks through existing checks, then build/publish its original images before workload review.</small></label>
  <label className="luci-field">Reviewed monthly workload budget · USD<input type="number" min="0.01" step="0.01" disabled={disabled||busy} value={budget} onChange={e=>edit('budget',e.target.value)}/><small>This is your authority ceiling, not a verified quote or provider-enforced spending cap.</small></label>
  <Button variant="secondary" disabled={disabled||!enabled||busy||!targetId||!Number.isFinite(Number(budget))||Number(budget)<=0} onClick={()=>void read('plan')}>Read current Kubernetes delegation</Button>
  {prepared&&!disabled&&<section aria-label="Review Kubernetes continuation"><p>Saved blueprint revision {prepared.blueprintRevision} · namespace identity <code>{prepared.namespaceUid}</code></p><p>Plan <code>{prepared.planDigest}</code></p><ul>{prepared.serviceImages.map(image=><li key={image.serviceId}>{image.serviceId}: <code style={{overflowWrap:'anywhere'}}>{image.image||'Build from the reviewed original preparation'}</code></li>)}</ul><p>Images are either saved immutable references or exact original preparation outputs. Their running behavior is not proved by a source commit alone. The separate final deployment review checks current namespace, source, runtime references and resource identities.</p></section>}
  <label className="setup-check"><input type="checkbox" disabled={disabled||busy||!prepared} checked={images} onChange={e=>{setImages(e.target.checked);setDirty(true);}}/>I reviewed these exact image references or the selected original build/publication preparation.</label>
  <label className="setup-check"><input type="checkbox" disabled={disabled||busy||!prepared} checked={cost} onChange={e=>{setCost(e.target.checked);setDirty(true);}}/>I authorize the reviewed existing-namespace workload effects and ongoing costs within this budget.</label>
  {prepared?.requiresPublicRoute&&<label className="setup-check"><input type="checkbox" disabled={disabled||busy} checked={publicRoute} onChange={e=>{setPublicRoute(e.target.checked);setDirty(true);}}/>I authorize the saved public HTTPS route through the existing Traefik controller and TLS Secret. Controller operation, DNS pointing and certificate trust remain explicit prerequisites; no DNS or certificate creation is delegated.</label>}
  {prepared?.requiresTelemetry&&<label className="setup-check"><input type="checkbox" disabled={disabled||busy} checked={telemetry} onChange={e=>{setTelemetry(e.target.checked);setDirty(true);}}/>I authorize the declared application telemetry collection and configured destination.</label>}
  <p>Workload readiness is not application behavior or model-quality acceptance. Unknown original mutations are retained and observed, never retried automatically.</p>
  {busy&&<p role="status">Reading current saved delegation…</p>}{problem&&<p role={error?'alert':'status'}>{problem}</p>}{dirty&&<Button variant="tertiary" disabled={disabled||busy} onClick={clear}>Discard Kubernetes delegation draft</Button>}
 </>}</details>;
}
