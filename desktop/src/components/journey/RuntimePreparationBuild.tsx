import { useEffect, useRef, useState } from 'react';
import { runtimePrerequisitesApi, type RuntimePreparation } from '../../api/runtimePrerequisites';
import { projectRunsApi, type ProjectRun } from '../../api/projectRuns';
import { errorText } from '../../api/product';
import { useUnsavedProjectDraft } from '../../hooks/useUnsavedProjectDraft';
import { Button } from '../fluid/components/button';

export function eligiblePreparationRuns(runs:ProjectRun[],preparation:RuntimePreparation):ProjectRun[]{
 // Reviewer-only work has an original report, not a coding integration. The server
 // remains authoritative for exact passed review obligations and contract bindings.
 const tasks=preparation.suggestedTasks.filter(t=>t.role!=='reviewer').map(t=>t.id);
 return runs.filter(run=>run.projectId===preparation.plan.projectId&&run.state!=='cancelled'&&!!run.swarm?.tasks.length&&run.swarm.tasks.every(t=>t.reviewOnly?run.operations.some(op=>op.id===t.operationId&&!!op.handoff&&['succeeded','review-required'].includes(op.status)):t.integration?.state==='integrated')&&tasks.every(id=>run.input.implementation.taskIds.includes(id)));
}
export function eligiblePreparationChecks(run?:ProjectRun){
 return run?.checks.filter(check=>check.originalAvailable&&check.commands.length>0&&check.commands.every(c=>c.status==='passed'&&c.exitCode===0))??[];
}
export function RuntimePreparationBuild({projectId,preparation,enabled,onChange}:{projectId:string;preparation:RuntimePreparation;enabled:boolean;onChange(value:RuntimePreparation):void}){
 const [runs,setRuns]=useState<ProjectRun[]>([]),[runId,setRun]=useState(''),[checkId,setCheck]=useState(''),[busy,setBusy]=useState(false),[unknown,setUnknown]=useState(false),[error,setError]=useState('');
 const [loaded,setLoaded]=useState(false);
 const generation=useRef(0),controller=useRef<AbortController|null>(null),mounted=useRef(true),inFlight=useRef(false),latest=useRef({enabled,preparation,onChange});
 latest.current={enabled,preparation,onChange};
 const retire=()=>{generation.current++;controller.current?.abort();controller.current=null;inFlight.current=false;};
 useEffect(()=>{mounted.current=true;return()=>{mounted.current=false;retire();};},[]);
 useEffect(()=>{if(!enabled){retire();setBusy(false);}},[enabled]);
 useUnsavedProjectDraft(projectId,!!runId&&!preparation.execution&&!unknown,'Image build selection','deploy',{retained:false,discard:()=>{retire();setRun('');setCheck('');setBusy(false);}});
 async function call(kind:'runs'|'start'|'observe'){
  if(!latest.current.enabled||inFlight.current)return;
  const original=latest.current.preparation;
  if(kind==='start'&&(!runId||!checkId||unknown||original.execution||original.builds.length>0))return;
  const epoch=++generation.current,abort=new AbortController();controller.current=abort;inFlight.current=true;setBusy(true);setError('');
  const current=()=>mounted.current&&epoch===generation.current&&latest.current.enabled&&latest.current.preparation.plan.id===original.plan.id;
  if(kind==='start')setUnknown(true);
  try{
   if(kind==='runs'){const values=await projectRunsApi.list(projectId,abort.signal);if(current()){setRuns(eligiblePreparationRuns(values,original));setLoaded(true);setRun('');setCheck('');}}
   else{
    const value=kind==='start'?await runtimePrerequisitesApi.start(projectId,original.plan.id,{expectedRevision:original.revision,preparationDigest:original.plan.digest,runId,checkId},abort.signal):await runtimePrerequisitesApi.observe(projectId,original.plan.id,abort.signal);
    if(current()){latest.current.onChange(value);setUnknown(false);}
   }
  }catch(cause){if(current())setError(errorText(cause));}
  finally{if(current()){inFlight.current=false;setBusy(false);controller.current=null;}}
 }
 async function readOriginal(){
  if(!latest.current.enabled||inFlight.current)return;
  const original=latest.current.preparation,epoch=++generation.current,abort=new AbortController();controller.current=abort;inFlight.current=true;setBusy(true);setError('');
  try{const row=await runtimePrerequisitesApi.get(projectId,original.plan.id,abort.signal);if(mounted.current&&epoch===generation.current&&latest.current.enabled){latest.current.onChange(row);setUnknown(false);}}
  catch(cause){if(mounted.current&&epoch===generation.current)setError(errorText(cause));}
  finally{if(mounted.current&&epoch===generation.current){inFlight.current=false;setBusy(false);controller.current=null;}}
 }
 const run=runs.find(r=>r.id===runId),checks=eligiblePreparationChecks(run),execution=preparation.execution,ready=enabled&&!busy;
 return <section aria-label="Build reviewed project images">
  <h4>{preparation.plan.builds.length?'Build before creating your cluster':'Prepare resources on your connected cluster'}</h4>
  <p>{preparation.plan.builds.length?'Choose the integrated Develop run and its passing checks. Luci builds its reviewed Dockerfiles, publishes immutable images, and saves them to your deployment blueprint. A cluster is not required to build images.':'Use the checked project source to prepare the reviewed storage, model and private connection resources. Existing application images are reused.'}</p>
  {!execution&&!preparation.builds.length&&!preparation.savedBlueprintRevision&&<>
   <Button variant="secondary" disabled={!ready||unknown} onClick={()=>void call('runs')}>Load completed Develop work</Button>
   {loaded&&!runs.length&&<p role="status">Finish and integrate the prerequisite tasks in Develop, then run the project checks. Completed work appears here.</p>}
   {!!runs.length&&<><label className="luci-field">Integrated project run<select value={runId} disabled={!ready||unknown} onChange={e=>{setRun(e.target.value);setCheck('');}}><option value="">Choose completed work</option>{runs.map(r=><option key={r.id} value={r.id}>{new Date(r.updatedAt).toLocaleString()} · {r.swarm?.tasks.length} tasks · {r.id}</option>)}</select></label>
   <label className="luci-field">Passing project checks<select value={checkId} disabled={!ready||unknown||!runId} onChange={e=>setCheck(e.target.value)}><option value="">Choose the checked source</option>{checks.map(c=><option key={c.checkId} value={c.checkId}>{c.commit.slice(0,12)} · {c.commands.length} checks · {new Date(c.createdAt).toLocaleString()}</option>)}</select></label>
   {run&&!checks.length&&<p role="status">This run has no available passing check receipt. Complete its combined checks in Develop.</p>}</>}
   <Button disabled={!ready||unknown||!runId||!checks.some(c=>c.checkId===checkId)} onClick={()=>void call('start')}>{preparation.plan.builds.length?'Build reviewed images':'Prepare reviewed resources'}</Button>
  </>}
  {execution&&<p role="status">{execution.state==='complete'?'Preparation complete. Blueprint revision '+preparation.savedBlueprintRevision+' contains the original image and resource references.':execution.state==='attention'?'The original preparation needs attention. Refresh its recorded outcome; an unknown operation is not restarted.':'The build batch was started. Refresh its original outcome to see progress.'} Source {execution.commit.slice(0,12)}.</p>}
  {unknown&&<p role="status">The start response was not confirmed. Read this original preparation before continuing; do not start a replacement.</p>}
  <div className="journey-actions"><Button variant="secondary" disabled={!ready} onClick={()=>void readOriginal()}>Read preparation</Button>{(execution||!!preparation.builds.length)&&<Button variant="ghost" disabled={!ready} onClick={()=>void call('observe')}>Refresh original outcome</Button>}</div>
  {busy&&<p role="status">Updating this preparation…</p>}{error&&<p role="alert">{error}</p>}
 </section>;
}
