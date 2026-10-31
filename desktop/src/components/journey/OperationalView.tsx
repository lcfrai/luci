import { useEffect, useRef, useState } from 'react';
import { kubernetesOperationsApi, type LogResult, type OperationalEvidence, type OperationalPods, type OperationalSelection, type TempoSave, type TempoView, type TraceResult } from '../../api/kubernetesOperations';
import type { KubernetesReview } from '../../api/kubernetesDeployment';
import { errorText } from '../../api/product';
import { teamScopeGeneration } from '../../api/teamScope';
import { useProjectAccess } from '../../hooks/useProjectAccess';
import { useProjectDraftScope } from '../../hooks/useProjectDraft';
import { useUnsavedProjectDraft } from '../../hooks/useUnsavedProjectDraft';
import { useTeamWorkspace } from '../teams/TeamWorkspaceProvider';
import { Button } from '../fluid/components/button';

const emptyConnection=():TempoSave['connection']=>({endpoint:'',caPem:'',token:'',tenant:''});
export function operationalDiscussion(row:OperationalEvidence,note:string):string {
 return [`Please discuss this recorded operational observation for ${row.componentId}.`,
  `Source: ${row.kind}; observed ${row.observedAt}; state ${row.state}; bounded sample${row.partial?' (partial)':''}.`,
  `Project ${row.projectId}; deployment review ${row.reviewId}; plan ${row.planDigest}; source ${row.sourceCommit}.`,
  `Target ${row.targetId}; namespace UID ${row.namespaceUid}; Deployment UID ${row.deploymentUid}; Pod UID ${row.podUid}.`,
  row.kind==='logs'?`${row.lineCount} returned log lines; text SHA256 ${row.payloadDigest}. Raw log text is not included.`:`${row.traceCount} matching traces; ${row.spanCount} matching spans; ${row.errorSpans} error spans. This is not exhaustive trace coverage.`,
  `Retained evidence ${row.id}; observation digest ${row.observationDigest}.`,note.trim()?`My observation: ${note.trim()}`:''].filter(Boolean).join('\n');
}

export function OperationalView({review,disabled=false}:{review:KubernetesReview;disabled?:boolean}) {
 const projectId=review.request.identity.projectId,scope=useProjectDraftScope(projectId),access=useProjectAccess(projectId),team=useTeamWorkspace();
 const allowed=!!scope&&access.canRead&&access.canEdit&&!!access.permissions?.deploy&&!!access.permissions?.secrets&&!access.loading&&!access.error&&!team?.accessBlocked;
 const key=JSON.stringify([scope,projectId,review.id,review.request.identity,review.contextDigest,review.blueprintDigest,team?.teamId,team?.revision,teamScopeGeneration(),allowed]);
 return <OperationalEditor key={key} review={review} allowed={allowed} disabled={disabled}/>;
}
function OperationalEditor({review,allowed,disabled}:{review:KubernetesReview;allowed:boolean;disabled:boolean}) {
 const projectId=review.request.identity.projectId,scope=useProjectDraftScope(projectId);
 const [open,setOpen]=useState(false),[component,setComponent]=useState(''),[pods,setPods]=useState<OperationalPods|null>(null),[pod,setPod]=useState('');
 const [logs,setLogs]=useState<LogResult|null>(null),[traces,setTraces]=useState<TraceResult|null>(null),[evidence,setEvidence]=useState<OperationalEvidence|null>(null);
 const [tail,setTail]=useState(100),[seconds,setSeconds]=useState(900),[previous,setPrevious]=useState(false),[logAck,setLogAck]=useState(false);
 const [tempo,setTempo]=useState<TempoView|null>(null),[connection,setConnection]=useState(emptyConnection),[name,setName]=useState(''),[configDirty,setConfigDirty]=useState(false),[needsBaselineAck,setNeedsBaselineAck]=useState(false),[baselineAck,setBaselineAck]=useState(false),[saveUnknown,setSaveUnknown]=useState(false);
 const [selection,setSelection]=useState<OperationalSelection|null>(null),[selectionUnknown,setSelectionUnknown]=useState(false);
 const [note,setNote]=useState(''),[busy,setBusy]=useState(false),[error,setError]=useState(''),[notice,setNotice]=useState('');
 const controller=useRef<AbortController|null>(null),epoch=useRef(0),mounted=useRef(true),inFlight=useRef(false);
 useEffect(()=>{mounted.current=true;return()=>{mounted.current=false;epoch.current++;controller.current?.abort();};},[]);
 function retire(){epoch.current++;controller.current?.abort();controller.current=null;inFlight.current=false;setBusy(false);}
 function discard(){retire();setConnection(emptyConnection());setName('');setConfigDirty(false);setNote('');setLogs(null);setTraces(null);setEvidence(null);setSaveUnknown(false);}
 useUnsavedProjectDraft(projectId,configDirty||!!note,'Operational inspection draft','deploy',{retained:false,discard});
 async function work(task:(signal:AbortSignal,current:()=>boolean)=>Promise<void>){
  if(!allowed||disabled||inFlight.current)return;const request=new AbortController(),ticket=++epoch.current;controller.current?.abort();controller.current=request;inFlight.current=true;setBusy(true);setError('');
  const current=()=>mounted.current&&!request.signal.aborted&&epoch.current===ticket;
  try{await task(request.signal,current);}catch(cause){if(current())setError(errorText(cause));}finally{if(current()){controller.current=null;inFlight.current=false;setBusy(false);}}
 }
 function chooseService(value:string){if(note)return;retire();setComponent(value);setPods(null);setPod('');setLogs(null);setTraces(null);setEvidence(null);setLogAck(false);}
 function choosePod(value:string){if(note)return;retire();setPod(value);setLogs(null);setTraces(null);setEvidence(null);setLogAck(false);}
 function invalidateQuery(){retire();setLogs(null);setTraces(null);setEvidence(null);setLogAck(false);}
 async function readPods(){if(note)return;await work(async(signal,current)=>{const result=await kubernetesOperationsApi.pods(review,component,signal);if(current()){setPods(result);setPod('');setLogs(null);setTraces(null);setEvidence(null);setNotice('Owned Pods read. Choose the original Pod to inspect.');}});}
 async function readLogs(){if(!pod||!logAck||note)return;await work(async(signal,current)=>{const result=await kubernetesOperationsApi.logs(review,{componentId:component,podUid:pod,tailLines:tail,sinceSeconds:seconds,previous},true,signal);if(current()){setLogs(result);setTraces(null);setEvidence(result.evidence);setNotice('Log text is shown in this view only. Saved evidence contains identity, counts and hashes.');}});}
 async function readTempo(){await work(async(signal,current)=>{const result=await kubernetesOperationsApi.tempo(projectId,review.target.id,signal);if(current()){if(configDirty&&tempo&&tempo.revision!==result.revision){setNeedsBaselineAck(true);setBaselineAck(false);}setTempo(result);setSaveUnknown(false);}});}
 async function saveTempo(){if(!tempo||saveUnknown||needsBaselineAck&&!baselineAck)return;await work(async(signal,current)=>{setSaveUnknown(true);const result=await kubernetesOperationsApi.saveTempo(projectId,review.target.id,{expectedRevision:tempo.revision,name,connection},signal);if(current()){setTempo(result);setConnection(emptyConnection());setName('');setConfigDirty(false);setNeedsBaselineAck(false);setBaselineAck(false);setSaveUnknown(false);setNotice('Tempo connection saved. Query traces explicitly to check for data.');}});}
 async function readTraces(owned=false){if(!pod||!owned&&(!tempo?.configured||tempo.targetFingerprint!==review.target.fingerprint)||note)return;await work(async(signal,current)=>{const result=await kubernetesOperationsApi.traces(review,{componentId:component,podUid:pod,lookbackSeconds:seconds,...(owned?{ownedBackend:true}:{})},signal);if(current()){setTraces(result);setLogs(null);setEvidence(result.evidence);setNotice('Bounded matching-span sample read from the configured Tempo backend.');}});}
 async function readSelection(){await work(async(signal,current)=>{const result=await kubernetesOperationsApi.selection(projectId,signal);if(current()){setSelection(result);setSelectionUnknown(false);}});}
 async function clearSelection(){if(!selection||selectionUnknown)return;const old=selection;await work(async(signal,current)=>{setSelectionUnknown(true);const result=await kubernetesOperationsApi.clearSelection(projectId,old,signal);if(current()){setSelection(result);setSelectionUnknown(false);setEvidence(previous=>{if(!previous)return previous;const original=old.rows.find(row=>row.id===previous.id);return original?{...previous,revision:original.revision+1,selected:false,selectedAt:''}:previous;});setNotice('These saved structured selections were cleared. Historical observations remain retained.');}});}
 async function selectEvidence(){if(!evidence)return;const original=evidence;await work(async(signal,current)=>{const result=await kubernetesOperationsApi.select(review,original,!original.selected,signal);if(current()){setEvidence(result);setNotice(result.selected?'This structured observation is selected for ordinary Luci findings. Raw logs and span attributes are excluded.':'This observation was removed from Luci findings.');}});}
 function discuss(){if(!allowed||!scope||!evidence||new TextEncoder().encode(note).length>2000)return;window.dispatchEvent(new CustomEvent('luci:orchestrator-open',{detail:{scope,projectId,message:operationalDiscussion(evidence,note)}}));setNotice('The observation is an unsent draft in project Luci. Review and send it when ready.');}
 const locked=!allowed||disabled||busy,queryLocked=locked||!!note,selectedPod=pods?.pods.find(row=>row.uid===pod&&row.componentId===component);
 return <details className="journey-details" open={open} onToggle={event=>setOpen(event.currentTarget.open)}><summary>Inspect this deployment</summary>{open&&<>
  {!allowed?<p>Current Edit, Deploy and Secrets access is required to inspect private operations.</p>:<>
   <label className="luci-field">Service<select value={component} disabled={queryLocked} onChange={event=>chooseService(event.target.value)}><option value="">Choose a service</option>{review.request.blueprint.services.map(service=><option key={service.id} value={service.id}>{service.id} · {service.kind}</option>)}</select></label>
   <Button type="button" variant="secondary" disabled={queryLocked||!component} onClick={()=>void readPods()}>Read owned Pods</Button>
   {pods&&<><p>Read {new Date(pods.observedAt).toLocaleString()}{pods.omitted?' · object limit reached; the list is partial':''}.</p>{!pods.pods.length?<p>No current owned Pods were confirmed for this service. That does not establish application availability.</p>:<label className="luci-field">Original Pod<select value={pod} disabled={queryLocked} onChange={event=>choosePod(event.target.value)}><option value="">Choose an observed Pod</option>{pods.pods.map(row=><option key={row.uid} value={row.uid}>{row.name} · {row.phase}{row.ready?' · ready':''} · {row.uid.slice(0,8)}</option>)}</select></label>}</>}
   {selectedPod&&<><p>Original Pod <code>{selectedPod.name}</code> · {selectedPod.phase}. Each query checks the current workload identity again.</p>
    <label className="luci-field">Time window<select value={seconds} disabled={queryLocked} onChange={event=>{invalidateQuery();setSeconds(Number(event.target.value));}}><option value={300}>Last 5 minutes</option><option value={900}>Last 15 minutes</option><option value={3600}>Last hour</option></select></label>
    <details><summary>Application logs</summary><label className="luci-field">Maximum lines<select value={tail} disabled={queryLocked} onChange={event=>{invalidateQuery();setTail(Number(event.target.value));}}><option value={50}>50</option><option value={100}>100</option><option value={200}>200</option></select></label>
     <label><input type="checkbox" checked={previous} disabled={queryLocked} onChange={event=>{invalidateQuery();setPrevious(event.target.checked);}}/>Read the previous container's logs, if retained</label>
     <label><input type="checkbox" checked={logAck} disabled={locked} onChange={event=>setLogAck(event.target.checked)}/>I want to read this Pod's private log output. It may contain sensitive application data.</label>
     <Button type="button" variant="secondary" disabled={queryLocked||!logAck} onClick={()=>void readLogs()}>Read bounded logs</Button>
     {logs&&<section aria-label="Observed application logs"><p>{logs.observation.lineCount} lines · read {new Date(logs.observation.observedAt).toLocaleString()}{logs.observation.truncated?' · truncated at 64 KiB':''}. Text is not copied into Luci.</p>{logs.observation.text?<pre tabIndex={0} style={{maxHeight:320,overflow:'auto',whiteSpace:'pre-wrap',overflowWrap:'anywhere'}}>{logs.observation.text}</pre>:<p>No log text was returned in this window.</p>}</section>}
    </details>
    <details><summary>Traces from Tempo</summary><p>Query an existing Tempo backend for this exact service and Pod instance. Sampling, retention and missing instrumentation can leave gaps.</p>
     <Button type="button" variant="secondary" disabled={locked} onClick={()=>void readTempo()}>Read saved Tempo connection</Button>
     {tempo&&<p>{tempo.configured?`${tempo.name} · ${tempo.endpoint}`:'No Tempo query connection is saved.'}{tempo.targetFingerprint!==review.target.fingerprint?' These settings belong to an earlier target connection. Replace them before querying.':''}</p>}
     <Button type="button" variant="secondary" disabled={queryLocked||!tempo?.configured||tempo.targetFingerprint!==review.target.fingerprint} onClick={()=>void readTraces()}>Read bounded trace sample</Button>
     {review.request.blueprint.services.some(s=>s.collector?.ownedDestination)&&<><p>The planned owned Tempo backend is queried through its exact original Pod with private mTLS; cluster credentials need pods/portforward. No external connection or desktop cluster DNS is used.</p><Button type="button" variant="secondary" disabled={queryLocked} onClick={()=>void readTraces(true)}>Read owned Tempo trace sample</Button></>}
     <details><summary>Set up or replace the query connection</summary><p>This connects to an existing HTTPS Tempo query frontend. A Collector alone does not store or query traces.</p>
      <fieldset disabled={locked}><label className="luci-field">Connection name<input value={name} maxLength={120} onChange={event=>{setName(event.target.value);setConfigDirty(true);}}/></label>
       <label className="luci-field">Tempo HTTPS endpoint<input value={connection.endpoint} placeholder="https://tempo.example.com" maxLength={2048} onChange={event=>{setConnection({...connection,endpoint:event.target.value});setConfigDirty(true);}}/></label>
       <label className="luci-field">Private CA PEM<textarea value={connection.caPem} rows={4} spellCheck={false} onChange={event=>{setConnection({...connection,caPem:event.target.value});setConfigDirty(true);}}/></label>
       <label className="luci-field">Bearer token, if required<input type="password" autoComplete="off" value={connection.token} maxLength={16384} onChange={event=>{setConnection({...connection,token:event.target.value});setConfigDirty(true);}}/></label>
       <label className="luci-field">Tempo tenant, if required<input value={connection.tenant} maxLength={128} onChange={event=>{setConnection({...connection,tenant:event.target.value});setConfigDirty(true);}}/></label>
       {needsBaselineAck&&<label><input type="checkbox" checked={baselineAck} onChange={event=>setBaselineAck(event.target.checked)}/>I reviewed saved revision {tempo?.revision} and want these complete settings to replace it.</label>}
       <Button type="button" disabled={!tempo||!name.trim()||!connection.endpoint||!connection.caPem||saveUnknown||needsBaselineAck&&!baselineAck} onClick={()=>void saveTempo()}>Save private query connection</Button>
      </fieldset>{saveUnknown&&<p>The save response is unconfirmed. Read the saved connection before another save; your draft remains here.</p>}
     </details>
     {traces&&<section aria-label="Observed trace sample"><p>Bounded trace sample · {traces.observation.traces.length} matching traces · {new Date(traces.observation.startAt).toLocaleTimeString()}–{new Date(traces.observation.endAt).toLocaleTimeString()}.</p>{!traces.observation.traces.length?<p>No matching spans were returned. Availability and request success remain unknown.</p>:<ul>{traces.observation.traces.map(trace=><li key={trace.traceId}><code>{trace.traceId}</code> · {trace.spanCount} matching spans · {trace.errorSpans} error spans · {new Date(trace.firstSpanAt).toLocaleTimeString()}–{new Date(trace.lastSpanAt).toLocaleTimeString()}</li>)}</ul>}<p>Only the original Pod's matching spans are shown. This is not complete trace coverage or a production-health verdict.</p></section>}
    </details>
   </>}
   <details><summary>Saved Luci findings selection</summary><p>Choose up to 16 structured observations. Stale selections are withheld from findings; you can still clear them here.</p><Button type="button" variant="secondary" disabled={locked} onClick={()=>void readSelection()}>Read saved selection</Button>{selection&&<><ul>{selection.rows.map(row=><li key={row.id}>{row.componentId} · {row.kind} · recorded {new Date(row.observedAt).toLocaleString()}</li>)}</ul>{!selection.rows.length&&<p>No structured observations are selected.</p>}<Button type="button" variant="secondary" disabled={locked||!selection.rows.length||selectionUnknown} onClick={()=>void clearSelection()}>Clear these saved selections</Button></>}{selectionUnknown&&<p>The clear response is unconfirmed. Read the saved selection before another change.</p>}</details>
   {evidence&&<section aria-label="Operational observation handoff"><h5>Use this observation</h5><p>Retained {evidence.kind} metadata · {evidence.state} · {new Date(evidence.observedAt).toLocaleString()}. Raw log text, span names and attributes are excluded.</p><Button type="button" variant="secondary" disabled={locked} onClick={()=>void selectEvidence()}>{evidence.selected?'Remove from Luci findings':'Select for Luci findings'}</Button><label className="luci-field">Your note<textarea rows={3} maxLength={2000} disabled={locked} value={note} onChange={event=>setNote(event.target.value)} placeholder="Describe what you want Luci to investigate; omit credentials."/></label>{note&&<p>Clear this note before choosing another observation. <Button type="button" variant="ghost" disabled={locked} onClick={()=>setNote('')}>Clear note</Button></p>}<Button type="button" variant="secondary" disabled={locked||new TextEncoder().encode(note).length>2000} onClick={discuss}>Open unsent discussion in Luci</Button></section>}
  </>}
  {busy&&<p role="status">Reading evidence or saving the selected settings. A bounded query can take up to 90 seconds. <Button type="button" variant="ghost" onClick={retire}>Cancel request</Button></p>}{error&&<p role="alert">{error}</p>}{notice&&<p role="status">{notice}</p>}
 </>}</details>;
}
