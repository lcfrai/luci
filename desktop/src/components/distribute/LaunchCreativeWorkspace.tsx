import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { designPlanApi, type DesignPlan } from '../../api/designPlan';
import { gtmWorkspaceApi } from '../../api/gtmWorkspace';
import { distributeArtifactsApi, type ArtifactList, type ArtifactView } from '../../api/distributeArtifacts';
import { creativeArtifactValid, creativeBriefValid, creativeEditorDraftValid, creativeSVGBlob, downloadLaunchCreative, launchCreativeApi, type CreativeArtifact, type CreativePrepared, type LaunchCreativeBrief, type CreativePending } from '../../api/launchCreative';
import { errorText, ProductApiError } from '../../api/product';
import { useProjectDraft, useProjectDraftScope } from '../../hooks/useProjectDraft';
import { useTeamWorkspace } from '../teams/TeamWorkspaceProvider';
import { useUnsavedProjectDraft } from '../../hooks/useUnsavedProjectDraft';
import { useCockpit } from '../../store/store';
import { Button } from '../fluid/components/button';
import './launch-creative.css';

const requestKey=()=>`creative-${crypto.randomUUID()}`;
const initialBrief=(product:string):LaunchCreativeBrief=>({product,headline:'',body:'',cta:'',alt:'',format:'wide',background:'#faf9f6',foreground:'#18181b',accent:'#244f44',frameId:'',assetId:''});
const short=(value:string,max:number)=>new TextEncoder().encode(value).length<=max?value:'';
type Saved=ArtifactView & {artifact:CreativeArtifact};

export function LaunchCreativeWorkspace({projectId,canRead,canEdit}:{projectId:string;canRead:boolean;canEdit:boolean}){
  const accountScope=useProjectDraftScope(projectId),team=useTeamWorkspace();
  const scope=accountScope?JSON.stringify([accountScope,team?.teamId??'personal']):null;
  return <CreativeBody key={JSON.stringify([scope,projectId,canRead,canEdit])} projectId={projectId} scope={scope} canRead={canRead} canEdit={canEdit}/>;
}
function CreativeBody({projectId,scope,canRead,canEdit}:{projectId:string;scope:string|null;canRead:boolean;canEdit:boolean}){
  const name=useCockpit(state=>state.projects.find(p=>p.id===projectId)?.name??'');
  const [brief,setBrief]=useState(()=>initialBrief(short(name,120))),[design,setDesign]=useState<DesignPlan|null>(null),[list,setList]=useState<ArtifactList|null>(null);
  const [pendingSave,setPendingSave]=useState<CreativePending|null>(null);
  const [prepared,setPrepared]=useState<CreativePrepared|null>(null),[saved,setSaved]=useState<Saved|null>(null),[svg,setSVG]=useState('');
  const [dirty,setDirty]=useState(false),[busy,setBusy]=useState(false),[uncertain,setUncertain]=useState(false),[error,setError]=useState(''),[notice,setNotice]=useState('');
  const key=useRef(''),reviewKey=useRef(''),lifecycle=useRef({mounted:true,epoch:0,controller:null as AbortController|null});
  useEffect(()=>{lifecycle.current.mounted=true;return()=>{lifecycle.current.mounted=false;lifecycle.current.epoch++;lifecycle.current.controller?.abort()}},[]);
  const validDraft=useCallback((v:unknown):v is import('../../api/launchCreative').CreativeEditorDraft=>creativeEditorDraftValid(v,projectId),[projectId]);
  const draftValue=useMemo(()=>({projectId,brief,saved:saved?.artifact??null,pending:pendingSave}),[projectId,brief,saved,pendingSave]);
  const draft=useProjectDraft({scope,kind:'launch-creative',base:saved?.artifact.digest??'new',value:draftValue,dirty:dirty||uncertain||!!pendingSave,valid:validDraft});
  useUnsavedProjectDraft(projectId,dirty||uncertain||!!draft.pending,'Launch creative','distribute',{retained:draft.retained||!!draft.pending,discard:()=>{if(pendingSave||draft.pending?.value.pending){draft.offerRecovery();setBrief(initialBrief(short(name,120)));setSaved(null);setPrepared(null);setSVG('');setDirty(false);setPendingSave(null);setUncertain(false);return;}draft.clear();restore();}});
  function restore(){setBrief(saved?{...saved.artifact.creative.brief}:initialBrief(short(name,120)));setDirty(false);setPrepared(null);draft.clear();key.current='';if(!saved)setSVG('');}
  function change(update:Partial<LaunchCreativeBrief>){if(!canEdit||busy||uncertain||draft.pending)return;setBrief(v=>({...v,...update}));setDirty(true);setPrepared(null);key.current='';if(!saved)setSVG('');}
  async function run(action:(signal:AbortSignal,current:()=>boolean)=>Promise<void>){
    if(!scope||!canRead||busy)return;
    lifecycle.current.controller?.abort();const controller=new AbortController(),epoch=++lifecycle.current.epoch;lifecycle.current.controller=controller;
    const current=()=>lifecycle.current.mounted&&lifecycle.current.epoch===epoch&&!controller.signal.aborted;
    setBusy(true);setError('');setNotice('');
    try{await action(controller.signal,current)}catch(cause){if(current())setError(errorText(cause))}finally{if(current())setBusy(false)}
  }
  useEffect(()=>{
    if(!scope||!canRead)return;
    const controller=new AbortController(),epoch=lifecycle.current.epoch;lifecycle.current.controller=controller;
    distributeArtifactsApi.list(projectId,controller.signal).then(v=>{if(lifecycle.current.mounted&&lifecycle.current.epoch===epoch&&!controller.signal.aborted&&v.projectId===projectId)setList(v)}).catch(c=>{if(lifecycle.current.mounted&&lifecycle.current.epoch===epoch&&!controller.signal.aborted)setError(errorText(c))});
    return()=>controller.abort();
  },[projectId,scope,canRead]);
  async function loadContext(signal:AbortSignal,current:()=>boolean){
    const [plan,gtm]=await Promise.all([designPlanApi.get(projectId),gtmWorkspaceApi.get(projectId,signal)]);
    if(!current())return;
    if(plan.projectId!==projectId||gtm.projectId!==projectId||gtm.freshness!=='current'||!gtm.workspace)throw new Error('Save a current GTM strategy before composing launch artwork. Existing creative records remain in the gallery.');
    const colors=plan.tokens.map(t=>t.value).filter(v=>/^#[a-fA-F0-9]{6}$/.test(v));
    setDesign(plan);setSaved(null);setPrepared(null);setSVG('');
    setBrief({...initialBrief(short(name,120)),headline:short(gtm.workspace.content.positioning,240),body:short(gtm.workspace.content.offer,800),background:colors[0]??'#faf9f6',foreground:colors[1]??'#18181b',accent:colors[2]??'#244f44'});
    setDirty(true);key.current='';
    setNotice('Saved GTM copy and Design colours loaded as editable suggestions. Oversized source text is left empty; enter concise launch copy and an image description. Review colours and claims before generating.');
  }
  async function open(id:string,signal:AbortSignal,current:()=>boolean){
    if(dirty||uncertain)throw new Error('Save or discard your current brief before opening another creative. An uncertain save must be resolved using its original request.');
    const value=await launchCreativeApi.get(projectId,id,signal);if(!current())return;
    setSaved(value);setBrief({...value.artifact.creative.brief});setPrepared(null);setDirty(false);setSVG('');key.current='';reviewKey.current='';
    const output=await launchCreativeApi.svg(projectId,value.artifact,signal);if(current())setSVG(output);
  }
  async function refreshSaved(signal:AbortSignal,current:()=>boolean){
    if(!saved)return;
    const value=await launchCreativeApi.get(projectId,saved.artifact.id,signal);if(!current())return;
    if(dirty&&value.artifact.digest!==saved.artifact.digest)throw new Error('The saved baseline changed. Your edits were kept; copy the recovered brief before discarding and opening the latest creative.');
    setSaved(value);if(!dirty)setBrief({...value.artifact.creative.brief});setSVG('');
    const output=await launchCreativeApi.svg(projectId,value.artifact,signal);if(current())setSVG(output);
  }
  async function save(signal:AbortSignal,current:()=>boolean){
    if(!canEdit||!scope||draft.pending)return;
    const request=pendingSave??(saved?{mode:'revise' as const,key:key.current||requestKey(),brief:{...brief},row:saved.artifact}:prepared?{mode:'create' as const,key:key.current||requestKey(),brief:{...prepared.draft.creative.brief},prepareDigest:prepared.prepareDigest}:null);
    if(!request)throw new Error('Generate a complete preview before saving a new creative.');
    // Retain the exact request synchronously before any write can leave this window.
    if(!draft.retain({projectId,brief,saved:saved?.artifact??null,pending:request},saved?.artifact.digest??'new'))throw new Error('Browser recovery could not be confirmed. No save was sent. Enable storage or keep this brief open.');
    key.current=request.key;setPendingSave(request);
    let acknowledged=false;
    try{
      const value=await launchCreativeApi.sendPending(projectId,request,signal);
      if(!current())return;
      acknowledged=true;draft.clear();setPendingSave(null);setSaved(value);setBrief({...value.artifact.creative.brief});setPrepared(null);setSVG('');setDirty(false);setUncertain(false);key.current='';reviewKey.current='';
      setNotice('Creative saved privately. Review the artwork and its declared source licences before launch.');
      const rows=await distributeArtifactsApi.list(projectId,signal);if(!current())return;setList(rows);
      const output=await launchCreativeApi.svg(projectId,value.artifact,signal);if(current())setSVG(output);
    }catch(cause){
      if(current()&&!acknowledged){
        const refused=cause instanceof ProductApiError&&['invalid_request','artifact_request_invalid','artifact_kind_invalid','artifact_source_changed','artifact_changed','artifact_content_invalid','artifact_sources_changed','artifact_history_full','creative_unavailable','gtm_launch_source_unavailable','forbidden','authentication_required','distribute_capacity_pending'].includes(cause.code);
        if(refused&&draft.retain({projectId,brief,saved:saved?.artifact??null,pending:null},saved?.artifact.digest??'new')){setPendingSave(null);setUncertain(false);setPrepared(null);setDirty(true);key.current='';if(!saved)setSVG('');}
        else setUncertain(true);
      }
      throw cause;
    }
  }
  const gallery=list?.artifacts.filter(v=>v.kind==='launch-creative')??[];
  const visual=saved?{frame:saved.artifact.creative.frame,assets:saved.artifact.creative.assets}:null;
  const selectedVisual=brief.frameId?`frame:${brief.frameId}`:brief.assetId?`asset:${brief.assetId}`:'';
  const editable=canEdit&&!busy&&!uncertain&&!draft.pending&&(!saved||saved.freshness==='current');
  function launchDraft(){
    if(!scope||!canEdit||!saved||saved.freshness!=='current'||saved.artifact.state!=='reviewed'||dirty||uncertain)return;
    const row=saved.artifact;
    window.dispatchEvent(new CustomEvent('luci:orchestrator-open',{detail:{scope,projectId,message:`Prepare a launch handoff using the explicitly reviewed creative ${row.id}, revision ${row.revision}, digest ${row.digest}; source digest ${row.sourceDigest}. Headline: ${row.creative.brief.headline}. CTA: ${row.creative.brief.cta}. Use the downloaded SVG/PNG as composed artwork, not proof of a running application. Review asset licences and marketing claims. Suggest where this creative fits the saved launch plan; preserve unrelated context and leave publication or changes for my explicit review.`}}));
    setNotice('The exact creative reference is in an unsent Luci draft. No model or publishing action was started.');
  }
  return <section className="distribute-card launch-creative" aria-label="Launch creatives"><h2>Launch creatives</h2><p>Compose a launch visual from your product design and message. Edit the brief, generate a preview, then keep and download the artwork.</p>
    {!scope&&<p role="status">Account and workspace context must finish loading before private artwork is read.</p>}
    {draft.error&&<p role="alert">{draft.error}</p>}
    {draft.pending&&<section aria-label="Creative draft recovery"><p>A browser draft or unresolved save is available for this account, team and project.</p><Button variant="secondary" disabled={busy||!canEdit} onClick={()=>{const record=draft.take();if(!record)return;const v=record.value;setBrief(v.brief);setSaved(v.saved?{artifact:v.saved,freshness:'unavailable'}:null);setPendingSave(v.pending);key.current=v.pending?.key??'';setUncertain(!!v.pending);setDirty(true);setPrepared(null);setSVG('');setNotice(v.pending?'Original save restored. Retry it to read back its outcome; no replacement request will be created.':'Browser brief restored. Load its saved artwork or generate a new composition after reviewing the current product context.');}}>Restore browser draft</Button></section>}
    {error&&<p role="alert">{error}</p>}{notice&&<p role="status">{notice}</p>}
    {canRead&&scope&&<>
      <div className="journey-actions"><Button variant="secondary" disabled={!canEdit||busy||dirty||uncertain||!!draft.pending} onClick={()=>void run(loadContext)}>New from saved product context</Button><Button variant="tertiary" disabled={busy||dirty||uncertain||!!draft.pending} onClick={()=>void run(async(signal,current)=>{const rows=await distributeArtifactsApi.list(projectId,signal);if(current())setList(rows)})}>Refresh gallery</Button></div>
      <div className="launch-creative-layout"><div>
        <fieldset disabled={!editable}><legend>Creative brief</legend>
          {([['product','Product name',120],['headline','Headline',240],['body','Supporting copy',800],['cta','Call to action',80],['alt','Image description',1000]] as const).map(([field,label,max])=><label key={field}>{label}<textarea rows={field==='body'||field==='alt'?3:2} value={brief[field]} maxLength={max} onChange={e=>change({[field]:e.target.value})}/></label>)}
          <label>Launch format<select value={brief.format} onChange={e=>change({format:e.target.value as LaunchCreativeBrief['format']})}><option value="wide">Wide · 1200 × 675</option><option value="square">Square · 1080 × 1080</option></select></label>
          <label>Product visual<select value={selectedVisual} disabled={!!saved} onChange={e=>{const [kind,...parts]=e.target.value.split(':');const id=parts.join(':');change({frameId:kind==='frame'?id:'',assetId:kind==='asset'?id:''})}}><option value="">Typography only</option>{!saved&&selectedVisual&&!design&&<option value={selectedVisual}>Retained visual · load saved visual choices</option>}
            {(saved&&visual?.frame?[visual.frame]:design?.scene?.frames??[]).map(f=><option key={`frame:${f.id}`} value={`frame:${f.id}`}>Design frame · {f.name} · {f.width} × {f.height}</option>)}
            {(saved?visual?.assets??[]:design?.assets??[]).filter(a=>a.content).map(a=><option key={`asset:${a.id}`} value={`asset:${a.id}`}>Pinned image · {a.path}</option>)}
          </select></label>
          <Button variant="tertiary" disabled={!editable||!!saved} onClick={()=>void run(async(_signal,current)=>{const plan=await designPlanApi.get(projectId);if(current()&&plan.projectId===projectId)setDesign(plan);})}>Load saved visual choices</Button>
          <div className="launch-creative-palette">{([['background','Background'],['foreground','Text'],['accent','CTA fill']] as const).map(([field,label])=><label key={field}>{label}<input type="color" value={brief[field]} onChange={e=>change({[field]:e.target.value})}/></label>)}</div>
        </fieldset>
        <details><summary>Composition limits and source meaning</summary><p className="journey-muted">The whole selected frame fits inside the artwork; authored hidden layers stay hidden. Design frames are illustrations, not implemented-product screenshots. Text uses local system fonts with word-aware wrapping; token and line overflow is refused. For exact typography across devices, use a pinned raster. Shorten overflowing copy rather than losing it.</p></details>
        <div className="journey-actions">{!saved&&<Button variant="primary" disabled={!editable||!creativeBriefValid(brief)} onClick={()=>void run(async(signal,current)=>{const p=await launchCreativeApi.prepare(projectId,brief,signal);if(current()){setPrepared(p);setSVG(p.svg);key.current=requestKey();setNotice('Complete composition generated by the deterministic renderer. Save it to retain the exact artwork.')}})}>Generate composition</Button>}
          <Button variant="primary" disabled={!canEdit||busy||!!draft.pending||!dirty&&!uncertain||!saved&&!prepared&&!pendingSave||!!saved&&saved.freshness!=='current'&&!uncertain} onClick={()=>void run(save)}>{uncertain?'Retry original save':saved?'Save copy revision':'Save to gallery'}</Button>
          <Button variant="tertiary" disabled={busy||!dirty||uncertain} onClick={restore}>Discard brief edits</Button>
        </div>
        {uncertain&&<p role="status">The save outcome is unconfirmed. Your exact brief and original request key were saved in this browser before sending. Retry the original save; do not generate or send a replacement request.</p>}
        {saved&&dirty&&!uncertain&&saved.freshness!=='current'&&<><Button variant="secondary" disabled={busy} onClick={()=>void run(async(signal,current)=>{const value=await launchCreativeApi.get(projectId,saved.artifact.id,signal);if(!current())return;if(value.artifact.digest!==saved.artifact.digest)throw new Error('The saved baseline changed. Copy the recovered brief below before discarding edits and opening the latest creative.');setSaved(value);setNotice('Recovered brief baseline checked. Edits remain yours; saving still uses revision CAS.');})}>Check recovered baseline</Button><details><summary>Recovered brief text</summary><pre>{JSON.stringify(brief,null,2)}</pre></details></>}
        {saved&&<p>Saved revision {saved.artifact.revision} · {saved.artifact.state} · {saved.freshness}. {dirty?'Preview still shows the last saved artwork. Save copy changes to update it.':''} {saved.freshness==='readback'?'Refresh the saved record to verify its current source.':saved.freshness==='outdated'?'The source changed. Keep this artwork or start from current product context.':saved.freshness==='unavailable'?'Source readback is unavailable. Your saved artwork and edits are kept. Refresh the saved record before reviewing.':''}</p>}
      </div><div className="launch-creative-result"><CreativePreview svg={svg} alt={saved?.artifact.creative.brief.alt??brief.alt}/>
        {saved&&<><details><summary>Artwork sources and licence notes</summary><p>Design source {saved.artifact.source.designRevision} · GTM {saved.artifact.source.gtmRevision}. {saved.artifact.creative.frameDigest&&<>Frame digest <code>{saved.artifact.creative.frameDigest}</code>.</>}</p>
          {saved.artifact.creative.assets.map(a=><details key={a.id}><summary>Source and declared licence: {a.path}</summary><p>{a.licence||'No licence statement recorded; review the source before external use.'}</p><p>Commit <code>{a.content?.sourceCommit}</code> · SHA-256 <code>{a.content?.digest}</code></p></details>)}</details>
          <div className="journey-actions"><Button variant="secondary" disabled={busy||uncertain||!!draft.pending} onClick={()=>void run(refreshSaved)}>Refresh saved record</Button>
            {(['svg','png'] as const).map(format=><Button key={format} variant="secondary" disabled={busy||dirty||uncertain||!svg} onClick={()=>void run(async(_signal,current)=>{const output=svg;await downloadLaunchCreative(output,format,`launch-${saved.artifact.id}-r${saved.artifact.revision}`,current)})}>Download {format.toUpperCase()}</Button>)}
            <Button variant="secondary" disabled={!canEdit||busy||dirty||uncertain||saved.freshness!=='current'||saved.artifact.state==='reviewed'||!svg} onClick={()=>void run(async(_signal,current)=>{const result=await distributeArtifactsApi.review(projectId,saved.artifact,reviewKey.current||=(requestKey()));if(!current())return;if(!creativeArtifactValid(result.artifact,projectId)||result.artifact.id!==saved.artifact.id)throw new Error('The reviewed creative response could not be confirmed. Read the saved record before continuing.');setSaved(result as Saved);reviewKey.current='';setNotice('Creative marked reviewed. This does not publish it or verify product claims.');})}>Mark reviewed</Button>
            <Button variant="secondary" disabled={!canEdit||busy||dirty||uncertain||saved.freshness!=='current'||saved.artifact.state!=='reviewed'} onClick={launchDraft}>Prepare launch handoff in Luci</Button>
          </div></>}
      </div></div>
      <section aria-label="Saved creative gallery"><h3>Saved creative gallery</h3>{!gallery.length&&<p>No launch creatives saved yet.</p>}<ul className="launch-creative-gallery">{gallery.map(row=><li key={row.id}><Button variant={saved?.artifact.id===row.id?'secondary':'tertiary'} disabled={busy||dirty||uncertain||!!draft.pending} onClick={()=>void run((signal,current)=>open(row.id,signal,current))}>{row.title} · revision {row.revision} · {row.state}</Button></li>)}</ul></section>
    </>}
  </section>;
}

function CreativePreview({svg,alt}:{svg:string;alt:string}){
  const [url,setURL]=useState(''),[error,setError]=useState('');
  useEffect(()=>{setURL('');setError('');if(!svg)return;let url='';try{url=URL.createObjectURL(creativeSVGBlob(svg));setURL(url)}catch(cause){setError(errorText(cause))}return()=>{if(url)URL.revokeObjectURL(url)}},[svg]);
  return <figure className="launch-creative-preview">{url?<img src={url} alt={alt} onError={()=>setError('The artwork preview could not be displayed. Read the saved artwork again; the private record is unchanged.')}/>:<p>Generate a composition or open a saved creative to preview the actual artwork.</p>}{error&&<p role="alert">{error}</p>}<figcaption>Composed launch artwork · not a live product demonstration</figcaption></figure>;
}
