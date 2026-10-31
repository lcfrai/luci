import type { BlueprintPublicRoute, BlueprintService } from '../../api/deploymentBlueprint';

export function PublicRouteEditor({value,services,disabled,onChange}:{value?:BlueprintPublicRoute;services:BlueprintService[];disabled:boolean;onChange(value:BlueprintPublicRoute|undefined):void}){
 const apps=services.filter(service=>service.kind==='app'&&!service.collector);
 return <details className="definition-group"><summary>Public HTTPS route · existing Traefik</summary><fieldset disabled={disabled}>
  <p>Use an operator-owned Traefik Ingress controller with a public <code>websecure</code> entrypoint on 443 and Ingress status publication. Point your DNS hostname at that controller and provide an existing matching TLS Secret. Luci does not install a controller, create certificates, change DNS or obtain a public load balancer here.</p>
  <label><input type="checkbox" checked={!!value} onChange={event=>onChange(event.target.checked?{profile:'traefik-websecure',serviceId:apps[0]?.id??'',ingressClassName:'traefik',hostname:'',path:'/',pathType:'Prefix',tlsSecretName:''}:undefined)}/>Expose one application through reviewed HTTPS routing</label>
  {value&&<div className="blueprint-grid">
   <label className="luci-field">Application Service<select value={value.serviceId} onChange={event=>onChange({...value,serviceId:event.target.value})}><option value="">Choose an application</option>{apps.map(service=><option key={service.id} value={service.id}>{service.id} · internal Service port {service.port||'not selected'}</option>)}</select></label>
   <label className="luci-field">Existing Traefik IngressClass name<input maxLength={253} value={value.ingressClassName} onChange={event=>onChange({...value,ingressClassName:event.target.value})}/></label>
   <label className="luci-field">Dedicated DNS hostname<input maxLength={253} placeholder="app.example.com" value={value.hostname} onChange={event=>onChange({...value,hostname:event.target.value.toLowerCase()})}/></label>
   <label className="luci-field">Path<input maxLength={256} value={value.path} onChange={event=>onChange({...value,path:event.target.value})}/><small>Requests retain this path. No rewrite or arbitrary middleware is added.</small></label>
   <label className="luci-field">Path matching<select value={value.pathType} onChange={event=>onChange({...value,pathType:event.target.value as 'Exact'|'Prefix'})}><option value="Prefix">Path prefix</option><option value="Exact">Exact path</option></select></label>
   <label className="luci-field">Existing namespace TLS Secret name<input maxLength={253} value={value.tlsSecretName} onChange={event=>onChange({...value,tlsSecretName:event.target.value})}/><small>Only the name is saved. The private key and certificate are never put in the blueprint or model context.</small></label>
   <p>Deployment review binds the current controller class and TLS Secret identities. This first profile uses one dedicated hostname; conflicting retained routes or unavailable cluster-wide Ingress listing require operator review. Controller status does not verify DNS, browser certificate trust or application health.</p>
  </div>}
 </fieldset></details>;
}
