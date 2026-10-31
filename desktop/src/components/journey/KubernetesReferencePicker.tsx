import { useEffect, useRef, useState } from 'react';
import { kubernetesDeploymentApi, type KubernetesReferenceDescriptor, type KubernetesReferenceKind } from '../../api/kubernetesDeployment';
import { errorText } from '../../api/product';
import { useProjectAccess } from '../../hooks/useProjectAccess';
import { useProjectDraftScope } from '../../hooks/useProjectDraft';
import { teamScopeGeneration } from '../../api/teamScope';
import { Button } from '../fluid/components/button';

export function KubernetesReferencePicker(props:{projectId:string;targetId:string;namespace:string;kind:KubernetesReferenceKind;active:boolean;onChoose(row:KubernetesReferenceDescriptor,key?:string):void}) {
  const scope=useProjectDraftScope(props.projectId),access=useProjectAccess(props.projectId);
  const allowed=props.active&&!!scope&&access.canRead&&access.canEdit&&!access.loading&&!access.error&&!!access.permissions?.deploy&&!!access.permissions?.secrets;
  const key=JSON.stringify([scope,props.projectId,props.targetId,props.namespace,props.kind,allowed,teamScopeGeneration()]);
  return <ReferencePicker key={key} {...props} allowed={allowed}/>;
}

function ReferencePicker({projectId,targetId,namespace,kind,active,allowed,onChoose}:{projectId:string;targetId:string;namespace:string;kind:KubernetesReferenceKind;active:boolean;allowed:boolean;onChoose(row:KubernetesReferenceDescriptor,key?:string):void}) {
  const [rows,setRows]=useState<KubernetesReferenceDescriptor[]>([]),[selected,setSelected]=useState(''),[detail,setDetail]=useState<KubernetesReferenceDescriptor|null>(null),[key,setKey]=useState('');
  const [busy,setBusy]=useState(false),[partial,setPartial]=useState(false),[error,setError]=useState(''),[read,setRead]=useState(false);
  const pending=useRef<AbortController|null>(null),ticket=useRef(0),alive=useRef(true);
  useEffect(()=>{alive.current=true;return()=>{alive.current=false;ticket.current++;pending.current?.abort();};},[]);
  const configured=/^cluster-[a-f0-9]{32}$/.test(targetId)&&!!namespace;
  async function load(name='') {
    if(!allowed||!active||!configured||busy)return;
    const controller=new AbortController(),request=++ticket.current;pending.current?.abort();pending.current=controller;setBusy(true);setError('');setDetail(null);setKey('');
    const current=()=>alive.current&&!controller.signal.aborted&&ticket.current===request;
    try {
      const value=await kubernetesDeploymentApi.references(projectId,targetId,kind,name,controller.signal);
      if(!current())return;if(value.namespace!==namespace)throw new Error('The target namespace changed. Select the current target in your blueprint.');
      if(name){setDetail(value.items[0]);}else{setRows(value.items);setSelected('');setRead(true);}setPartial(value.partial);
    } catch(cause){if(current())setError(errorText(cause));}
    finally{if(current()){pending.current=null;setBusy(false);}}
  }
  const model=kind==='persistent-volume-claim',label=model?'model volumes':kind==='secret'?'Secrets':'ConfigMaps';
  const shared=detail?.accessModes?.some(mode=>mode==='ReadOnlyMany'||mode==='ReadWriteMany');
  const compatible=model?detail?.phase==='Bound'&&detail.volumeMode==='Filesystem'&&!!detail.accessModes?.length:!!detail?.keysRead&&!!key&&!!detail.keys?.includes(key);
  return <details><summary>Choose from existing {label}</summary>
    <p className="journey-muted">Reads existing names and selected key names only. No volume or configuration is created. Values are never displayed or saved here.</p>
    {!configured?<p>Choose a saved cluster target in Deploy and use its namespace in this blueprint first.</p>:!allowed?<p>Current project Secrets and Deploy permission are required to inspect runtime references.</p>:null}
    <Button type="button" variant="secondary" disabled={!allowed||!configured||busy} onClick={()=>void load()}>Read existing {label}</Button>
    {busy&&<p role="status">Reading existing reference metadata…</p>}{error&&<p role="alert">{error}</p>}
    {partial&&<p role="status">Only a bounded set is shown. This is not a complete namespace inventory.</p>}
    {read&&rows.length===0&&<p>No existing {label} were returned in this namespace.</p>}
    {rows.length>0&&<label className="luci-field">Existing {model?'model volume':kind==='secret'?'Secret':'ConfigMap'}<select value={selected} disabled={!allowed||busy} onChange={event=>{const name=event.target.value;setSelected(name);setDetail(null);setKey('');if(name)void load(name);}}><option value="">Choose explicitly</option>{rows.map(row=><option key={row.uid} value={row.name}>{row.name}</option>)}</select></label>}
    {detail&&<div>
      {model?<><p>{detail.name}: {detail.phase??'State unavailable'} · {detail.volumeMode??'Volume mode unavailable'} · {detail.accessModes?.join(', ')||'Access mode unavailable'}.</p>
        {!shared&&<p>A nonshared model volume must be used by one service only, with one replica and Recreate. Separate services are not guaranteed to share a node. Using this selection will set those two fields explicitly; replacing the workload may interrupt service.</p>}
      </>:<label className="luci-field">Existing key in {detail.name}<select value={key} disabled={!allowed||busy} onChange={event=>setKey(event.target.value)}><option value="">Choose a key</option>{detail.keys?.map(name=><option key={name} value={name}>{name}</option>)}</select></label>}
      <Button type="button" variant="secondary" disabled={!allowed||busy||!compatible} onClick={()=>{if(detail&&compatible)onChoose(detail,model?undefined:key);}}>Use selected {model?'model volume':'resource key'}</Button>
    </div>}
  </details>;
}
