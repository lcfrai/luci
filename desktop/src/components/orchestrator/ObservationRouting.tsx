import { useEffect, useRef, useState } from 'react';
import { observationKinds, researchApi, type Observation, type ObservationKind, type ObservationView } from '../../api/research';
import { useProjectDraftScope } from '../../hooks/useProjectDraft';
import { useProjectAccess } from '../../hooks/useProjectAccess';
import { errorText } from '../../api/product';
import { Button } from '../fluid/components/button';
import { useUnsavedProjectDraft } from '../../hooks/useUnsavedProjectDraft';

const labels:Record<ObservationKind,string>={research:'Saved research evidence',feedback:'Selected issue feedback',metrics:'Imported aggregate metrics and revenue',telemetry:'Selected retained owned product counters',operations:'Explicitly selected operational evidence metadata'};
export function ObservationRouting({projectId}:{projectId:string}) {
  const scope=useProjectDraftScope(projectId);
  const access=useProjectAccess(projectId);
  // Retire cached sensitive projections and in-flight reads on either Secrets
  // transition; a fresh public-source read still requires only normal access.
  return scope ? <ObservationRoutingBody key={`${scope}:${projectId}:${access.permissions.secrets?'secrets':'public'}`} projectId={projectId}/> : null;
}
function ObservationRoutingBody({projectId}:{projectId:string}) {
  const access=useProjectAccess(projectId), allowed=access.canRead&&access.canEdit&&access.canExecute;
  const [view,setView]=useState<ObservationView|null>(null),[kinds,setKinds]=useState<ObservationKind[]>([]),[ids,setIDs]=useState<string[]>([]),[turns,setTurns]=useState(4),[consent,setConsent]=useState(false);
  const [busy,setBusy]=useState(false),[error,setError]=useState(''),[notice,setNotice]=useState('');
  const epoch=useRef(0),controller=useRef<AbortController|null>(null),permission=useRef(allowed),dirty=useRef(false);permission.current=allowed;
  useUnsavedProjectDraft(projectId,dirty.current,'Observation source choices','develop',{retained:false,discard:()=>{dirty.current=false;setKinds(view?.route?.kinds??[]);setIDs(view?.route?.telemetryIds??[]);setTurns(Math.max(1,view?.route?.remaining??4));setConsent(false);}});
  useEffect(()=>()=>{epoch.current++;controller.current?.abort();},[]);
  useEffect(()=>{if(!allowed){epoch.current++;controller.current?.abort();setView(null);setBusy(false);setConsent(false);}},[allowed]);
  async function read(){if(!permission.current||busy)return;const ticket=++epoch.current;controller.current?.abort();const abort=new AbortController();controller.current=abort;setBusy(true);setError('');try{const next=await researchApi.observations(projectId,abort.signal);if(ticket!==epoch.current||!permission.current)return;setView(next);if(!dirty.current){setKinds(next.route?.kinds??[]);setIDs(next.route?.telemetryIds??[]);setTurns(Math.max(1,next.route?.remaining??4));}else setNotice('Saved observations refreshed. Your unsaved source choices remain; compare them before saving.');}catch(cause){if(ticket===epoch.current&&permission.current)setError(errorText(cause));}finally{if(ticket===epoch.current)setBusy(false);}}
  async function save(pause:boolean){if(!view||!permission.current||busy||!pause&&!consent)return;const ticket=++epoch.current;const abort=new AbortController();controller.current=abort;setBusy(true);setError('');try{const next=await researchApi.route(projectId,{expectedRouteRevision:view.routeRevision,kinds:pause?[]:kinds,telemetryIds:pause?[]:ids,maxTurns:pause?0:turns,acknowledgeModelCharges:!pause&&consent},abort.signal);if(ticket!==epoch.current||!permission.current)return;setView(next);setKinds(next.route?.kinds??[]);setIDs(next.route?.telemetryIds??[]);dirty.current=false;setConsent(false);setNotice(pause?'New observation proposals paused. Already retained or running requests were not cancelled.':'Saved a baseline. Only subsequent saved content changes may request proposals, up to the selected turn budget. No external sources will be polled.');}catch(cause){if(ticket===epoch.current&&permission.current)setError(errorText(cause));}finally{if(ticket===epoch.current)setBusy(false);}}
  async function queue(source:Observation){if(!view||!consent||!permission.current||busy)return;const ticket=++epoch.current;const abort=new AbortController();controller.current=abort;setBusy(true);setError('');try{await researchApi.queue(projectId,view.routeRevision,source,abort.signal);if(ticket===epoch.current&&permission.current)setNotice('Exact selected evidence retained for the ordinary project conversation. The same digest will not be resent; inspect its original finding if the outcome is uncertain.');}catch(cause){if(ticket===epoch.current&&permission.current)setError(errorText(cause));}finally{if(ticket===epoch.current)setBusy(false);}}
  if(!access.canRead)return null;
  return <details className="orchestrator-amendment"><summary>Turn saved findings into reviewable proposals</summary>
    <p>Use the saved provider/model route above. Luci can react to new saved evidence without approving its own changes. This delegates bounded model spending, not website polling, execution or publishing.</p>
    <Button variant="secondary" disabled={!allowed||busy} onClick={()=>void read()}>Read saved observation sources</Button>
    {error&&<p role="alert">{error}</p>}{notice&&<p role="status">{notice}</p>}{busy&&<p role="status">Reading or retaining the selected request…</p>}
    {view&&<>
      {!view.autoDiscuss&&<p>Save a project coding/model route and enable ordinary finding discussions above first. Current project auto-draft policy still governs model dispatch.</p>}
      {view.route&&<p>Saved budget remaining: {view.route.remaining} ordinary turns. Sources: {view.route.kinds.map(k=>labels[k]).join(', ')||'paused'}.</p>}
      {view.unavailable.length>0&&<p>Unavailable saved sources: {view.unavailable.map(k=>labels[k]).join(', ')}. Unknown sources are not treated as absent findings.</p>}
      <fieldset><legend>React to subsequent saved changes</legend>{observationKinds.map(k=><label key={k}><input type="checkbox" checked={kinds.includes(k)} disabled={!allowed||busy} onChange={e=>{dirty.current=true;setKinds(e.target.checked?[...kinds,k]:kinds.filter(v=>v!==k));}}/>{labels[k]}</label>)}</fieldset>
      {kinds.includes('telemetry')&&<fieldset><legend>Exact retained deployment observations</legend>{view.telemetryOptions.map(row=><label key={row.id}><input type="checkbox" checked={ids.includes(row.id)} disabled={!allowed||busy||ids.length>=8&&!ids.includes(row.id)} onChange={e=>{dirty.current=true;setIDs(e.target.checked?[...ids,row.id]:ids.filter(v=>v!==row.id));}}/>{row.label}</label>)}{!view.telemetryOptions.length&&<p>Read and retain owned product counters in Deploy first. No cluster read is performed here.</p>}</fieldset>}
      <label>Maximum new turns for this delegation <input type="number" min={1} max={16} value={turns} disabled={!allowed||busy} onChange={e=>{dirty.current=true;setTurns(Number(e.target.value));}}/></label>
      <label><input type="checkbox" checked={consent} onChange={e=>setConsent(e.target.checked)} disabled={!allowed||busy}/>Allow these selected saved observations to be shared in the project conversation using my saved model route. Provider charges may apply; retained source text is untrusted input.</label>
      <Button disabled={!allowed||busy||!view.autoDiscuss||!consent||!kinds.length||!Number.isInteger(turns)||turns<1||turns>16||kinds.includes('telemetry')&&!ids.length} onClick={()=>void save(false)}>Enable bounded observation proposals</Button>
      <Button variant="secondary" disabled={!allowed||busy||!view.route} onClick={()=>void save(true)}>Pause observation proposals</Button>
      <h4>Send a saved source once for review</h4>{view.sources.map(source=><section key={source.kind}><h5>{labels[source.kind]}</h5><p>{source.summary}</p><Button disabled={!allowed||busy||!view.autoDiscuss||!consent} onClick={()=>void queue(source)}>Ask Luci about this exact saved source</Button></section>)}
    </>}
  </details>;
}
