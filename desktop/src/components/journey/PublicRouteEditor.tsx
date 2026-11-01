import type { BlueprintPublicRoute, BlueprintService } from '../../api/deploymentBlueprint';
import { useEffect, useLayoutEffect, useRef, useState } from 'react';
import { publicHTTPSApi, type HTTPSStatus } from '../../api/publicHTTPS';
import { errorText } from '../../api/product';
import { useProjectAccess } from '../../hooks/useProjectAccess';
import { useProjectDraftScope } from '../../hooks/useProjectDraft';
import { useTeamWorkspace } from '../teams/TeamWorkspaceProvider';
import { Button } from '../fluid/components/button';

export function PublicRouteEditor({projectId,targetId,namespace,value,services,disabled,onChange}:{projectId?:string;targetId?:string;namespace?:string;value?:BlueprintPublicRoute;services:BlueprintService[];disabled:boolean;onChange(value:BlueprintPublicRoute|undefined):void}){
 const apps=services.filter(service=>service.kind==='app'&&!service.collector);
 return <details className="definition-group"><summary>Public HTTPS route</summary><fieldset disabled={disabled}>
  <p>Choose a commissioned endpoint below, or use your existing Traefik controller and certificate. Commission a new domain in Public HTTPS after its application Service has been deployed.</p>
  <label><input type="checkbox" checked={!!value} onChange={event=>onChange(event.target.checked?{profile:'traefik-websecure',serviceId:apps[0]?.id??'',ingressClassName:'traefik',hostname:'',path:'/',pathType:'Prefix',tlsSecretName:''}:undefined)}/>Expose one application through reviewed HTTPS routing</label>
  {value&&<><label className="luci-field">Endpoint source<select value={value.profile} onChange={event=>onChange(event.target.value==='luci-managed-https'?{profile:'luci-managed-https',managedHTTPSId:'',serviceId:'',ingressClassName:'',hostname:'',path:'/',pathType:'Prefix',tlsSecretName:''}:{profile:'traefik-websecure',serviceId:apps[0]?.id??'',ingressClassName:'traefik',hostname:'',path:'/',pathType:'Prefix',tlsSecretName:''})}><option value="traefik-websecure">Existing controller and TLS Secret</option><option value="luci-managed-https">Commissioned Luci HTTPS endpoint</option></select></label>
  {value.profile==='luci-managed-https'?<>
   <ManagedHTTPSPicker projectId={projectId??''} targetId={targetId??''} namespace={namespace??''} services={apps} disabled={disabled} selectedId={value.managedHTTPSId??''} onSelect={row=>onChange({profile:'luci-managed-https',managedHTTPSId:row.id,serviceId:row.plan.destination.serviceId,hostname:row.plan.hostname,path:row.plan.path,pathType:row.plan.pathType,ingressClassName:'',tlsSecretName:''})}/>
   {value.managedHTTPSId&&<p>{value.hostname}{value.path} → {value.serviceId}. Save the blueprint, then review this release. Its original commissioned proxy is rechecked; no duplicate Ingress or certificate Secret is created.</p>}
  </>:<div className="blueprint-grid">
   <p>Use an operator-owned Traefik Ingress controller with a public <code>websecure</code> entrypoint on 443 and Ingress status publication. Point DNS at that controller and provide an existing matching TLS Secret. This manual profile does not create a controller, certificate, DNS record or load balancer.</p>
   <label className="luci-field">Application Service<select value={value.serviceId} onChange={event=>onChange({...value,serviceId:event.target.value})}><option value="">Choose an application</option>{apps.map(service=><option key={service.id} value={service.id}>{service.id} · internal Service port {service.port||'not selected'}</option>)}</select></label>
   <label className="luci-field">Existing Traefik IngressClass name<input maxLength={253} value={value.ingressClassName} onChange={event=>onChange({...value,ingressClassName:event.target.value})}/></label>
   <label className="luci-field">Dedicated DNS hostname<input maxLength={253} placeholder="app.example.com" value={value.hostname} onChange={event=>onChange({...value,hostname:event.target.value.toLowerCase()})}/></label>
   <label className="luci-field">Path<input maxLength={256} value={value.path} onChange={event=>onChange({...value,path:event.target.value})}/><small>Requests retain this path. No rewrite or arbitrary middleware is added.</small></label>
   <label className="luci-field">Path matching<select value={value.pathType} onChange={event=>onChange({...value,pathType:event.target.value as 'Exact'|'Prefix'})}><option value="Prefix">Path prefix</option><option value="Exact">Exact path</option></select></label>
   <label className="luci-field">Existing namespace TLS Secret name<input maxLength={253} value={value.tlsSecretName} onChange={event=>onChange({...value,tlsSecretName:event.target.value})}/><small>Only the name is saved. The private key and certificate are never put in the blueprint or model context.</small></label>
   <p>Deployment review binds the current controller class and TLS Secret identities. This first profile uses one dedicated hostname; conflicting retained routes or unavailable cluster-wide Ingress listing require operator review. Controller status does not verify DNS, browser certificate trust or application health.</p>
  </div>}</>}
 </fieldset></details>;
}

function ManagedHTTPSPicker(props:{projectId:string;targetId:string;namespace:string;services:BlueprintService[];disabled:boolean;selectedId:string;onSelect(row:HTTPSStatus):void}) {
 const scope=useProjectDraftScope(props.projectId),team=useTeamWorkspace();
 const key=JSON.stringify([scope,team?.teamId??'',props.projectId,props.targetId,props.namespace,props.services.map(s=>[s.id,s.port])]);
 return <ManagedHTTPSPickerBody key={key} {...props}/>;
}
function ManagedHTTPSPickerBody({projectId,targetId,namespace,services,disabled,selectedId,onSelect}:{projectId:string;targetId:string;namespace:string;services:BlueprintService[];disabled:boolean;selectedId:string;onSelect(row:HTTPSStatus):void}) {
 const access=useProjectAccess(projectId),team=useTeamWorkspace(),[rows,setRows]=useState<HTTPSStatus[]>([]),[busy,setBusy]=useState(false),[error,setError]=useState('');
 const allowed=!disabled&&!!projectId&&!!targetId&&!!namespace&&access.canRead&&access.canEdit&&!!access.permissions?.deploy&&!access.loading&&!access.error&&!team?.accessBlocked;
 const epoch=useRef(0),controller=useRef<AbortController|null>(null),live=useRef(true),working=useRef(false);
 useLayoutEffect(()=>{epoch.current++;controller.current?.abort();working.current=false;setBusy(false);if(!allowed)setRows([]);},[allowed]);
 useEffect(()=>{live.current=true;return()=>{live.current=false;epoch.current++;controller.current?.abort();};},[]);
 const eligible=(row:HTTPSStatus)=>row.state==='ready'&&row.httpsVerified&&Date.parse(row.certificateNotAfter??'')>Date.now()&&row.plan.destination.targetId===targetId&&row.plan.destination.namespace===namespace&&services.some(s=>s.id===row.plan.destination.serviceId&&s.port===row.plan.destination.servicePort);
 async function load(){if(!allowed||working.current)return;working.current=true;const ticket=++epoch.current,request=new AbortController();controller.current?.abort();controller.current=request;setBusy(true);setError('');const current=()=>live.current&&epoch.current===ticket&&!request.signal.aborted;
  try{const values=await publicHTTPSApi.list(projectId,request.signal);if(!current())return;if(selectedId&&!values.some(r=>r.id===selectedId)){const original=await publicHTTPSApi.get(projectId,selectedId,request.signal);if(!current())return;values.push(original);}setRows(values.filter(eligible));}catch(cause){if(current())setError(errorText(cause));}finally{if(current()){working.current=false;setBusy(false);}}
 }
 return <div><Button variant="secondary" disabled={!allowed||busy} onClick={()=>void load()}>{busy?'Reading endpoints…':'Load verified endpoints for this target'}</Button>{error&&<p role="alert">{error}</p>}<label className="luci-field">Commissioned endpoint<select disabled={!allowed||busy} value={rows.some(r=>r.id===selectedId&&eligible(r))?selectedId:''} onChange={event=>{const row=rows.find(r=>r.id===event.target.value);if(row&&allowed&&eligible(row))onSelect(row);}}><option value="">Choose a verified endpoint</option>{rows.filter(eligible).map(row=><option key={row.id} value={row.id}>{row.plan.hostname}{row.plan.path} · {row.plan.destination.serviceId}</option>)}</select></label><small>Only verified original endpoints with an unexpired recorded certificate on this target / namespace and the same application Service port are offered. Use Check endpoint in Public HTTPS to refresh a stale record. Historical saved selections remain readable.</small></div>;
}
