import {RuntimeBlobJobPicker} from './RuntimeBlobJobPicker';
import { useEffect, useState } from 'react';
import type { SavedDeploymentBlueprint } from '../../api/deploymentBlueprint';
import type { RuntimeResourceSelection } from '../../api/runtimeResources';

type VolumeDraft={selected:boolean;storageClass:string;size:string;url:string;sha:string;bytes:string};
type Props={saved:SavedDeploymentBlueprint|null;disabled:boolean;onChange(value:RuntimeResourceSelection|null,problem:string,edited?:boolean):void};
export function RuntimeResourceFields({saved,disabled,onChange}:Props){
 const [blobJob,setBlobJob]=useState('');
 const [volumes,setVolumes]=useState<Record<string,VolumeDraft>>({}),[tls,setTLS]=useState(false),[pull,setPull]=useState(false),[consumers,setConsumers]=useState<string[]>([]),[builder,setBuilder]=useState('');
 const services=saved?.blueprint.services??[];
 function emit(nextVolumes:Record<string,VolumeDraft>,nextTLS:boolean,nextPull:boolean,nextConsumers:string[],nextBuilder:string,edited=true,nextBlobJob=blobJob){
  const selected=services.filter(s=>nextVolumes[s.id]?.selected);
  if(!selected.length&&!nextTLS&&!nextPull&&!nextBlobJob){onChange(null,'',edited);return;}
  const wantsModel=selected.some(s=>s.kind==='inference');
  const value:RuntimeResourceSelection={...(nextBlobJob?{telemetryBlobJobId:nextBlobJob}:{}),volumes:selected.map(s=>{const d=nextVolumes[s.id],kind:string=s.kind;return {serviceId:s.id,storageClass:d.storageClass.trim(),sizeGiB:Number(d.size),...(kind==='metrics-store'?{purpose:'telemetry-tsdb' as const}:kind==='trace-store'?{purpose:'telemetry-wal' as const}:{})};}),models:selected.filter(s=>s.kind==='inference').map(s=>{const d=nextVolumes[s.id];return {serviceId:s.id,url:d.url.trim(),sha256:d.sha.trim(),bytes:Number(d.bytes)};}),generateTelemetryTls:nextTLS,generateImagePullSecret:nextPull,...(nextPull?{imagePullServiceIds:[...nextConsumers]}:{}),...(wantsModel?{modelSinkBuilderImage:nextBuilder.trim()}:{}),acknowledgeNewResources:true};
  let problem='';
  if(value.volumes?.some(v=>!v.storageClass||!Number.isSafeInteger(v.sizeGiB)||v.sizeGiB<1||v.sizeGiB>1024))problem='Choose an existing dynamic StorageClass and whole GiB size for each new volume.';
  if(value.models?.some(m=>!/^https:\/\//.test(m.url)||!/^([a-f0-9]{64})$/.test(m.sha256)||!Number.isSafeInteger(m.bytes)||m.bytes<1||m.bytes>8*2**30))problem='Each model needs its exact public HTTPS file URL, SHA-256 and byte count.';
  if(wantsModel&&!/^[^\s@]+@sha256:[a-f0-9]{64}$/.test(nextBuilder.trim()))problem='Choose an existing immutable Go 1.25+ builder image for the fixed model-transfer helper.';
  if(!saved?.blueprint.infrastructure.reference||!saved.blueprint.infrastructure.namespace)problem='Build images first if needed, then save an existing target and namespace before reviewing resources.';
  onChange(value,problem,edited);
 }
 function volume(id:string,patch:Partial<VolumeDraft>){const next={...volumes,[id]:{selected:false,storageClass:'',size:'10',url:'',sha:'',bytes:'',...volumes[id],...patch}};setVolumes(next);emit(next,tls,pull,consumers,builder);}
 // A new saved topology is an explicit new baseline. Retain the typed choices
 // in place, but the parent's saved-draft/review fences prevent silent Apply.
 useEffect(()=>{emit(volumes,tls,pull,consumers,builder,false);},[saved?.revision,saved?.digest]);
 const eligible=services.filter(s=>['inference','metrics-store','trace-store'].includes(s.kind));
 return <details><summary>Prepare model storage and private runtime connections</summary><fieldset disabled={disabled}><legend>New owned resources in the selected namespace</legend>
  <p>Existing volumes and credentials are never overwritten. Review includes new storage, one-time model transfer, internal certificates and explicitly selected registry consumers. Storage can keep incurring cost after a helper exits; the budget is an authority limit, not a provider quote.</p>
  {eligible.map(service=>{const d=volumes[service.id]??{selected:false,storageClass:'',size:'10',url:'',sha:'',bytes:''};return <fieldset key={service.id}><legend><label><input type="checkbox" checked={d.selected} onChange={e=>volume(service.id,{selected:e.target.checked})}/>New volume for {service.id} · {service.kind}</label></legend>{d.selected&&<>
   <label className="luci-field">{service.id} existing StorageClass<input value={d.storageClass} onChange={e=>volume(service.id,{storageClass:e.target.value})}/></label>
   <label className="luci-field">{service.id} new storage · GiB<input inputMode="numeric" value={d.size} onChange={e=>volume(service.id,{size:e.target.value})}/></label>
   {service.kind==='inference'?<><p>A public direct file, copied without archive unpacking. The exact file must fit the reviewed volume. The saved model relative path names one file; set the container mount path to the file path its actual server expects, and keep explicit image-specific arguments. Its hash confirms bytes, not model quality or licensing. New model files use group1000 with0750 directories and0640 files; the derived consumer joins that group, without world access.</p><label className="luci-field">{service.id} model file HTTPS URL<input type="url" value={d.url} onChange={e=>volume(service.id,{url:e.target.value})}/></label><label className="luci-field">{service.id} model SHA-256<input value={d.sha} onChange={e=>volume(service.id,{sha:e.target.value})}/></label><label className="luci-field">{service.id} exact model bytes<input inputMode="numeric" value={d.bytes} onChange={e=>volume(service.id,{bytes:e.target.value})}/></label></>:<p>New telemetry storage requires Immediate binding so its original PVC is positively Bound before deployment review. An existing Bound volume remains usable.</p>}
  </>}</fieldset>;})}
  {eligible.some(s=>s.kind==='inference'&&volumes[s.id]?.selected)&&<label className="luci-field">Existing Go 1.25+ builder image · repository@sha256<input value={builder} onChange={e=>{setBuilder(e.target.value);emit(volumes,tls,pull,consumers,e.target.value);}}/><small>Builds the fixed rootless Linux/amd64 transfer helper through the same reviewed engine and registry. No image is acquired by opening this form.</small></label>}
  {saved&&services.some(s=>s.telemetryStorage?.azure)&&<RuntimeBlobJobPicker key={saved.blueprint.projectId} projectId={saved.blueprint.projectId} accountName={services.find(s=>s.telemetryStorage?.azure)?.telemetryStorage?.azure?.accountName??''} value={blobJob} disabled={disabled} onChange={id=>{setBlobJob(id);emit(volumes,tls,pull,consumers,builder,true,id);}}/>}
  <label className="setup-check"><input type="checkbox" checked={tls} onChange={e=>{setTLS(e.target.checked);emit(volumes,e.target.checked,pull,consumers,builder);}}/>Generate the exact internal telemetry mTLS Secret references already in this saved topology</label>
  <label className="setup-check"><input type="checkbox" checked={pull} onChange={e=>{setPull(e.target.checked);emit(volumes,tls,e.target.checked,consumers,builder);}}/>Copy this reviewed registry connection into a new private image-pull Secret</label>
  {pull&&<><p>Only selected consumers receive this registry credential. Include every newly built private image; the fixed model helper also uses this connection.</p>{services.map(s=><label className="setup-check" key={s.id}><input type="checkbox" checked={consumers.includes(s.id)} onChange={e=>{const next=e.target.checked?[...consumers,s.id]:consumers.filter(id=>id!==s.id);setConsumers(next);emit(volumes,tls,pull,next,builder);}}/>Allow {s.id} to pull from this repository</label>)}</>}
 </fieldset></details>;
}
