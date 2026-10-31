import {useEffect,useRef,useState} from 'react';
import {productApi,errorText,type CloudJob} from '../../api/product';
import {useProjectAccess} from '../../hooks/useProjectAccess';
import {Button} from '../fluid/components/button';
export function RuntimeBlobJobPicker({projectId,accountName,value,disabled,onChange}:{projectId:string;accountName:string;value:string;disabled:boolean;onChange(id:string):void}){
 const access=useProjectAccess(projectId),allowed=!disabled&&access.canEdit&&access.permissions.deploy&&access.permissions.secrets;
 const [jobs,setJobs]=useState<CloudJob[]|null>(null),[busy,setBusy]=useState(false),[error,setError]=useState(''),epoch=useRef(0),pending=useRef<AbortController|null>(null),mounted=useRef(true),owner=useRef({allowed,projectId,accountName});
 if(owner.current.allowed!==allowed||owner.current.projectId!==projectId||owner.current.accountName!==accountName){owner.current={allowed,projectId,accountName};epoch.current++}
 useEffect(()=>{mounted.current=true;return()=>{mounted.current=false;epoch.current++;pending.current?.abort()}},[]);
 useEffect(()=>{pending.current?.abort();setJobs(null);setBusy(false)},[allowed,projectId,accountName]);
 async function read(){if(!allowed||busy)return;const ticket=++epoch.current,controller=new AbortController();pending.current?.abort();pending.current=controller;setBusy(true);setError('');setJobs(null);
  const current=()=>mounted.current&&owner.current.allowed&&epoch.current===ticket&&!controller.signal.aborted;
  try{const result=await productApi.cloudJobs(controller.signal);if(current()){if(!Array.isArray(result.jobs)||result.jobs.length>512)throw new Error('Owned storage history could not be confirmed.');setJobs(result.jobs.filter(job=>job.kind==='telemetry-storage'&&job.status==='resources_ready'&&typeof job.id==='string'&&/^cj-[A-Za-z0-9_-]+$/.test(job.id)&&typeof job.name==='string'&&job.name.length<=40&&Number.isFinite(Date.parse(job.createdAt))&&job.telemetryStorage?.state==='resources_ready'&&typeof job.telemetryStorage.accountResourceId==='string'&&job.telemetryStorage.accountResourceId.toLowerCase().endsWith('/storageaccounts/'+accountName.toLowerCase())));}}catch(cause){if(current())setError(errorText(cause))}finally{if(current()){pending.current=null;setBusy(false)}}
 }
 return <fieldset disabled={!allowed||busy}><legend>Import the owned trace storage credential privately</legend><p>Optional: choose the original observed Blob job to create the account-key Secret referenced by this topology. The server reads its dedicated account key under current Secrets/Deploy access and keeps it only in the original preparation’s private Secret bytes. Existing Secrets remain untouched. No credential value appears here.</p><Button type="button" variant="secondary" onClick={()=>void read()} disabled={!allowed||busy||!accountName}>{busy?'Reading storage jobs…':'Read matching owned Blob jobs'}</Button>
  <label className="luci-field">Original dedicated storage job<select value={value} onChange={e=>onChange(e.target.value)}><option value="">Use the existing Secret; do not import a key</option>{value&&!jobs?.some(job=>job.id===value)&&<option value={value}>Retained original choice · read its current status</option>}{allowed&&jobs?.map(job=><option key={job.id} value={job.id}>{job.name} · {new Date(job.createdAt).toLocaleDateString()} · {job.id.slice(-8)}</option>)}</select></label>
  {allowed&&jobs?.length===0&&<p>No matching original ready storage job is available. Review/create it first or use an existing private Secret.</p>}{error&&<p role="alert">{error}</p>}
 </fieldset>;
}
