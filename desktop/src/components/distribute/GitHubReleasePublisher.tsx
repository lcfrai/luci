import { useEffect, useRef, useState } from 'react';
import { githubReleasesApi, type GitHubPublication } from '../../api/githubReleases';
import { distributeArtifactsApi, type ArtifactSummary } from '../../api/distributeArtifacts';
import { errorText } from '../../api/product';
import { teamScopeGeneration } from '../../api/teamScope';
import { useProjectDraftScope } from '../../hooks/useProjectDraft';
import { useProjectAccess } from '../../hooks/useProjectAccess';
import { useUnsavedProjectDraft } from '../../hooks/useUnsavedProjectDraft';
import { useTeamWorkspace } from '../teams/TeamWorkspaceProvider';
import { Button } from '../fluid/components/button';

export function GitHubReleasePublisher({projectId}:{projectId:string}){
 const scope=useProjectDraftScope(projectId),access=useProjectAccess(projectId),team=useTeamWorkspace();
 const read=!!scope&&access.canRead&&!access.loading&&!access.error&&!team?.accessBlocked;
 const publish=read&&access.canEdit&&!!access.permissions?.deploy&&!!access.permissions?.secrets;
 return <Publisher key={JSON.stringify([projectId,scope,read,publish,team?.teamId,team?.revision,teamScopeGeneration()])} projectId={projectId} read={read} publish={publish}/>;
}
function Publisher({projectId,read,publish}:{projectId:string;read:boolean;publish:boolean}){
 const [assets,setAssets]=useState<ArtifactSummary[]>([]),[rows,setRows]=useState<GitHubPublication[]>([]),[earlier,setEarlier]=useState(false),[loaded,setLoaded]=useState(false);
 const [artifactId,setArtifactId]=useState(''),[tag,setTag]=useState(''),[selected,setSelected]=useState<GitHubPublication|null>(null),[ack,setAck]=useState(false),[dirty,setDirty]=useState(false);
 const [busy,setBusy]=useState(false),[uncertain,setUncertain]=useState(false),[error,setError]=useState(''),[notice,setNotice]=useState(''),[olderId,setOlderId]=useState('');
 const controller=useRef<AbortController|null>(null),epoch=useRef(0),mounted=useRef(true),urls=useRef(new Map<string,ReturnType<typeof setTimeout>>());
 useEffect(()=>{mounted.current=true;return()=>{mounted.current=false;epoch.current++;controller.current?.abort();for(const [url,timer]of urls.current){clearTimeout(timer);URL.revokeObjectURL(url);}urls.current.clear();};},[]);
 function retire(){epoch.current++;controller.current?.abort();controller.current=null;setBusy(false);}
 function discard(){retire();setArtifactId('');setTag('');setDirty(false);setSelected(null);setAck(false);}
 useUnsavedProjectDraft(projectId,dirty,'GitHub release preparation','distribute',{retained:false,discard});
 async function work(task:(signal:AbortSignal,current:()=>boolean)=>Promise<void>){
  if(!read||busy)return;const request=new AbortController(),ticket=++epoch.current;controller.current?.abort();controller.current=request;setBusy(true);setError('');
  const current=()=>mounted.current&&!request.signal.aborted&&epoch.current===ticket;
  try{await task(request.signal,current);}catch(cause){if(current())setError(errorText(cause));}finally{if(current()){controller.current=null;setBusy(false);}}
 }
 function receive(value:GitHubPublication,prepared=false){setRows(previous=>[value,...previous.filter(row=>row.id!==value.id)]);setSelected(value);setAck(false);if(prepared)setDirty(false);setNotice(value.message);}
 async function load(){await work(async(signal,current)=>{
  const history=await githubReleasesApi.list(projectId,signal);if(!current())return;
  const list=await distributeArtifactsApi.list(projectId,signal);if(!current())return;
  if(list.projectId!==projectId||!Array.isArray(list.artifacts)||list.artifacts.length>100)throw new Error('The launch asset list could not be confirmed for this project.');
  setAssets(list.artifacts.filter(row=>row.state==='reviewed'&&['landing-copy','launch-brief','channel-copy'].includes(row.kind)));setRows(history.publications);setEarlier(history.earlierAvailable);setLoaded(true);setUncertain(false);setAck(false);
  setSelected(old=>old?history.publications.find(row=>row.id===old.id)??null:null);setNotice('Read saved launch assets and original publication records. No GitHub request or publication was performed.');
 });}
 async function prepare(){if(!publish||uncertain)return;const asset=assets.find(row=>row.id===artifactId);if(!asset)return;await work(async(signal,current)=>{
  const value=await githubReleasesApi.prepare(projectId,{artifactId:asset.id,artifactRevision:asset.revision,artifactDigest:asset.digest,tag},signal);if(current())receive(value,true);
 });}
 async function action(kind:'draft'|'review'|'publish'|'observe'){
  if(!publish||!selected||uncertain&&kind!=='observe'||(kind==='draft'||kind==='publish')&&!ack)return;
  const original=selected;
  await work(async(signal,current)=>{
   if(kind==='draft'||kind==='publish'){setUncertain(true);setAck(false);}
   const value=await githubReleasesApi.act(projectId,original,kind,kind==='draft'||kind==='publish'?ack:false,signal);
   if(current()){receive(value);setUncertain(false);}
  });
 }
 function exportRecord(row:GitHubPublication){
  if(!read)return;const url=URL.createObjectURL(new Blob([JSON.stringify(row,null,2)+'\n'],{type:'application/json'}));const timer=setTimeout(()=>{URL.revokeObjectURL(url);urls.current.delete(url);},1000);urls.current.set(url,timer);
  const link=document.createElement('a');link.href=url;link.download=`luci-${row.id}.json`;link.click();
 }
 const locked=!publish||busy,expired=!!selected&&Date.now()>=Date.parse(selected.expiresAt);
 const publishExpired=!selected?.publishExpiresAt||Date.now()>=Date.parse(selected.publishExpiresAt);
 return <section className="distribute-card" aria-label="GitHub release publishing"><h2>Publish a GitHub Release</h2>
  <p>Use one reviewed launch asset and an existing pushed tag for this project's attached GitHub repository. Create a draft first, then separately confirm publication. This does not deploy the app, upload files or send outreach.</p>
  {!read?<p>Current project read access is required.</p>:<>
   {!publish&&<p role="status">Current Edit, Deploy and Secrets access is required for GitHub reads and publishing. Instance policy must also permit publication.</p>}
   <Button type="button" variant="secondary" disabled={!read||busy} onClick={()=>void load()}>Read launch assets and publication history</Button>
   {loaded&&<fieldset disabled={locked||uncertain}><legend>Choose the saved public copy</legend>
    <label className="luci-field">Reviewed launch asset<select value={artifactId} onChange={e=>{setArtifactId(e.target.value);setSelected(null);setAck(false);setDirty(true);}}><option value="">Choose a reviewed asset</option>{assets.map(row=><option key={row.id} value={row.id}>{row.title} · revision {row.revision}</option>)}</select></label>
    {!assets.length&&<p>Save and explicitly review a landing, launch or channel asset above first.</p>}
    <label className="luci-field">Existing pushed release tag<input value={tag} maxLength={64} placeholder="v1.2.0" onChange={e=>{setTag(e.target.value);setSelected(null);setAck(false);setDirty(true);}}/><small>Up to 64 letters, digits, dots, underscores or hyphens; no slashes. It must resolve to this project's current committed source. No Git push is performed. If this tag disappears during draft creation, GitHub may recreate it at the exact reviewed commit.</small></label>
    <p>The complete title and body must fit the 32 KiB review limit. To change the copy, edit, save and review the launch asset above.</p>
    <Button type="button" disabled={locked||!artifactId||!/^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$/.test(tag)} onClick={()=>void prepare()}>Review release copy and tag</Button>
   </fieldset>}
   {rows.length>0&&<label className="luci-field">Original publication<select disabled={busy} value={selected?.id??''} onChange={e=>{setSelected(rows.find(row=>row.id===e.target.value)??null);setAck(false);}}><option value="">Choose a retained publication</option>{rows.map(row=><option key={row.id} value={row.id}>{row.intent.title} · {row.intent.tag} · {row.state.replace(/-/g,' ')} · {new Date(row.createdAt).toLocaleString()}</option>)}</select></label>}
   {selected&&<section aria-label="Exact GitHub release review"><h3>{selected.intent.title}</h3><p>{selected.intent.repository} · tag {selected.intent.tag} · commit <code>{selected.intent.commit}</code></p>
    <p>Workspace GitHub account: {selected.intent.account.login} (ID {selected.intent.account.id}). This is existing workspace CLI authentication, not a claim that this GitHub identity belongs to your Luci account.</p>
    <p>Artifact revision {selected.intent.artifactRevision} · <code>{selected.intent.artifactDigest}</code>. Release visibility follows repository settings. Publishing writes the exact copy below, without making it the latest release.</p>
    <p>Recorded {new Date(selected.updatedAt).toLocaleString()}. Observe the original for fresh GitHub metadata.</p><pre style={{whiteSpace:'pre-wrap',overflowWrap:'anywhere'}} tabIndex={0} aria-label="Complete release body">{selected.intent.body}</pre><p role="status">{selected.message}</p>
    {selected.remote&&<p>Original GitHub ID {selected.remote.id} · <a href={`https://github.com/${selected.intent.repository}/releases/${selected.remote.draft?'edit/'+selected.remote.id:'tag/'+encodeURIComponent(selected.intent.tag)}`} target="_blank" rel="noreferrer">Open original GitHub release</a></p>}
    {selected.unconfirmedRemote&&<p>GitHub returned a same-tag release (ID {selected.unconfirmedRemote.id}). It is not adopted as the original or eligible for publication from this record.</p>}
    {selected.state==='reviewed'&&!selected.draftAttempted&&<><label><input type="checkbox" checked={ack} disabled={locked||expired||uncertain} onChange={e=>setAck(e.target.checked)}/>I reviewed the complete title, body, repository, account and exact tag. Create one GitHub draft; do not publish it. I understand GitHub may recreate this verified tag at the shown commit if the tag disappears during creation.</label><Button type="button" disabled={locked||expired||uncertain||!ack} onClick={()=>void action('draft')}>Create reviewed GitHub draft</Button>{expired&&<p>This preparation expired. Review the current asset and tag again; no GitHub effect was started by expiration.</p>}</>}
    {selected.remote?.draft&&!selected.publishAttempted&&<Button type="button" variant="secondary" disabled={locked||uncertain} onClick={()=>void action('review')}>Review original draft for publication</Button>}
    {selected.state==='draft-reviewed'&&!selected.publishAttempted&&<><label><input type="checkbox" checked={ack} disabled={locked||publishExpired||uncertain} onChange={e=>setAck(e.target.checked)}/>Publish this exact reviewed copy from the original GitHub draft. I reviewed repository visibility and want this separate publication.</label><Button type="button" disabled={locked||publishExpired||uncertain||!ack} onClick={()=>void action('publish')}>Publish reviewed GitHub Release</Button></>}
    {selected.draftAttempted&&<Button type="button" variant="secondary" disabled={locked} onClick={()=>void action('observe')}>Observe original release</Button>}
    <Button type="button" variant="tertiary" disabled={busy} onClick={()=>exportRecord(selected)}>Export original record</Button>
   </section>}
   {earlier&&<details><summary>Open an earlier known record</summary><p>Older completed or expired preparations remain retained. Use the original Luci record ID from its export; unknown attempts are never evicted.</p><label className="luci-field">Original Luci record ID<input value={olderId} maxLength={80} onChange={e=>setOlderId(e.target.value)}/></label><Button type="button" disabled={busy||!/^gh-release-[A-Za-z0-9_-]+$/.test(olderId)} onClick={()=>void work(async(signal,current)=>{const value=await githubReleasesApi.get(projectId,olderId,signal);if(current())receive(value);})}>Read exact earlier record</Button></details>}
  </>}
  {busy&&<p role="status">Reading or settling the selected original publication…</p>}{error&&<p role="alert">{error}</p>}{notice&&<p role="status">{notice}</p>}
  {uncertain&&<p role="status">A response is unconfirmed. Read retained history, then Observe the original. Do not repeat draft creation or publication.</p>}
 </section>;
}
