import { useEffect, useRef, useState } from 'react';
import { distributeArtifactsApi, type ArtifactKind, type ArtifactList, type ArtifactSection, type ArtifactView, type DistributeArtifact } from '../../api/distributeArtifacts';
import { errorText } from '../../api/product';
import { projectRunsApi } from '../../api/projectRuns';
import { useUnsavedProjectDraft } from '../../hooks/useUnsavedProjectDraft';
import { useProjectDraftScope } from '../../hooks/useProjectDraft';
import { Button } from '../fluid/components/button';

const kinds: {kind:ArtifactKind;label:string}[]=[
  {kind:'go-to-market',label:'Go-to-market'}, {kind:'proposal',label:'Proposal'},
  {kind:'pricing',label:'Pricing'}, {kind:'risk',label:'Risk'}, {kind:'presentation',label:'Presentation'},
];
function lines(text:string):string[]{return text.split('\n').map(row=>row.trim()).filter(Boolean)}
function newKey():string{return `distribute-artifact-${crypto.randomUUID()}`}
type ArtifactSectionDraft=Omit<ArtifactSection,'bullets'>&{bulletText:string};

export function ArtifactWorkspace({projectId,canRead,canEdit}:{projectId:string;canRead:boolean;canEdit:boolean}){
  const scope=useProjectDraftScope(projectId);
  return <ScopedArtifactWorkspace key={JSON.stringify([scope,projectId])} projectId={projectId} scope={scope} canRead={canRead} canEdit={canEdit}/>;
}

function ScopedArtifactWorkspace({projectId,scope,canRead,canEdit}:{projectId:string;scope:string|null;canRead:boolean;canEdit:boolean}){
  const [list,setList]=useState<ArtifactList|null>(null),[selected,setSelected]=useState<ArtifactView|null>(null);
  const [prepared,setPrepared]=useState<{draft:DistributeArtifact;prepareDigest:string}|null>(null);
  const [kind,setKind]=useState<ArtifactKind>('proposal'),[title,setTitle]=useState(''),[sections,setSections]=useState<ArtifactSectionDraft[]>([]);
  const [acceptedRuns,setAcceptedRuns]=useState<{id:string;label:string}[]>([]),[selectedRun,setSelectedRun]=useState('');
  const [gaps,setGaps]=useState(''),[assumptions,setAssumptions]=useState('');
  const [dirty,setDirty]=useState(false),[busy,setBusy]=useState(false),[error,setError]=useState(''),[notice,setNotice]=useState('');
  const saveKey=useRef(''),reviewKeys=useRef(new Map<string,string>()),reviseKey=useRef('');
  const exportIdentity=JSON.stringify([scope,projectId,canRead,selected?.artifact.projectId,selected?.artifact.id,selected?.artifact.revision,selected?.artifact.digest]);
  const exportState=useRef({identity:exportIdentity,generation:0,mounted:true});
  if(exportState.current.identity!==exportIdentity){exportState.current.identity=exportIdentity;exportState.current.generation++}
  useEffect(()=>{exportState.current.mounted=true;return()=>{exportState.current.mounted=false;exportState.current.generation++}},[]);
  useUnsavedProjectDraft(projectId,dirty,'Distribute artifact','distribute',{retained:true,discard:()=>{setDirty(false);setPrepared(null);if(selected)loadEditor(selected.artifact)}});
  function loadEditor(row:DistributeArtifact){setTitle(row.title);setSections(row.sections.map(({bullets,...part})=>({...part,bulletText:bullets.join('\n'),sourceRefs:[...part.sourceRefs]})));setGaps(row.gaps.join('\n'));setAssumptions(row.assumptions.join('\n'));setDirty(false);reviseKey.current=''}
  function reviewKeyFor(row:DistributeArtifact){
    const identity=JSON.stringify([projectId,row.id,row.revision,row.digest]);
    let key=reviewKeys.current.get(identity);
    if(!key){key=newKey();reviewKeys.current.set(identity,key)}
    return key;
  }
  useEffect(()=>{if(!canRead){setList(null);setSelected(null);return}const controller=new AbortController();distributeArtifactsApi.list(projectId,controller.signal).then(setList).catch(cause=>{if(!controller.signal.aborted)setError(errorText(cause))});return()=>controller.abort()},[canRead,projectId]);
  async function step(action:()=>Promise<void>){setBusy(true);setError('');setNotice('');try{await action()}catch(cause){setError(errorText(cause))}finally{setBusy(false)}}
  async function refresh(){if(dirty){setError('Save or discard artifact edits before refreshing.');return}const next=await distributeArtifactsApi.list(projectId);setList(next);if(selected){const view=await distributeArtifactsApi.get(projectId,selected.artifact.id);setSelected(view);loadEditor(view.artifact)}}
  async function open(id:string){if(dirty){setError('Save or discard artifact edits before opening another artifact.');return}const view=await distributeArtifactsApi.get(projectId,id);setPrepared(null);setSelected(view);loadEditor(view.artifact)}
  function download(format:'docx'|'pptx'|'xlsx'){
    if(!scope||!canRead||!selected||selected.artifact.projectId!==projectId||busy||dirty)return Promise.resolve();
    const {identity,generation}=exportState.current;
    return distributeArtifactsApi.download(projectId,selected.artifact,format,()=>exportState.current.mounted&&exportState.current.identity===identity&&exportState.current.generation===generation);
  }
  const editable=selected?.freshness==='current';
  const proposal=prepared?.draft;
  return <section className="distribute-card" aria-label="Project artifacts"><h2>Project artifacts</h2><p>Generate source-linked drafts for review. Unknown checks, release status and commercial facts stay visible as gaps. Reviewing a draft does not publish it.</p>
    {canRead&&!scope&&<p role="status">Account and workspace context must finish loading before downloading private artifacts.</p>}
    {error&&<p role="alert" className="journey-error">{error}</p>}{notice&&<p role="status">{notice}</p>}
    {canRead&&<><div className="journey-actions"><label>Artifact kind<select aria-label="Artifact kind" value={kind} disabled={!canEdit||busy||dirty} onChange={event=>{setKind(event.target.value as ArtifactKind);setPrepared(null)}}>{kinds.map(row=><option key={row.kind} value={row.kind}>{row.label}</option>)}</select></label><Button type="button" variant="secondary" disabled={!canEdit||busy||dirty} onClick={()=>step(async()=>{const next=await distributeArtifactsApi.prepare(projectId,kind,selectedRun);setPrepared(next);setSelected(null);saveKey.current=newKey()})}>Prepare source draft</Button><Button type="button" variant="tertiary" disabled={busy||dirty} onClick={()=>step(refresh)}>Refresh artifacts</Button></div>
      <div className="journey-actions"><Button type="button" variant="tertiary" disabled={busy||dirty} onClick={()=>step(async()=>{const runs=await projectRunsApi.list(projectId);setAcceptedRuns(runs.filter(run=>run.state==='complete'&&!!run.acceptance).map(run=>({id:run.id,label:`${run.id} · accepted ${run.acceptance!.acceptedAt}`})))})}>Load accepted run evidence</Button><label>Exact check and release<select aria-label="Exact check and release" disabled={busy||dirty} value={selectedRun} onChange={event=>{setSelectedRun(event.target.value);setPrepared(null)}}><option value="">None selected; keep evidence gaps</option>{acceptedRuns.map(row=><option value={row.id} key={row.id}>{row.label}</option>)}</select></label></div>
      {proposal&&<div className="distribute-evidence"><h3>Prepared {kinds.find(row=>row.kind===proposal.kind)?.label} draft</h3><p>Source: brief {proposal.source.briefRevision}, requirements {proposal.source.specificationRevision}, Design {proposal.source.designRevision}, plan {proposal.source.planRevision}{proposal.source.libraryRevision?`, team controls ${proposal.source.libraryRevision}`:''}{proposal.source.selectedEvidence?`, accepted run ${proposal.source.selectedEvidence.runId}`:''}.</p>{proposal.gaps.map((gap,index)=><p key={index}>Gap: {gap}</p>)}<Button type="button" variant="primary" disabled={!canEdit||busy} onClick={()=>step(async()=>{const view=await distributeArtifactsApi.save(projectId,kind,selectedRun,prepared.prepareDigest,saveKey.current);setSelected(view);setPrepared(null);loadEditor(view.artifact);setList(await distributeArtifactsApi.list(projectId));setNotice('Private draft saved. Review its claims and gaps before marking it reviewed.')})}>Save private draft</Button><Button type="button" variant="secondary" disabled={busy} onClick={()=>setPrepared(null)}>Discard prepared draft</Button></div>}
      {list?.artifacts.length===0&&<p>No artifacts saved for this project yet.</p>}
      {list?.artifacts.map(row=><Button key={row.id} type="button" variant={selected?.artifact.id===row.id?'primary':'tertiary'} disabled={busy} onClick={()=>step(()=>open(row.id))}>{row.title} · revision {row.revision} · {row.state}{row.freshness==='outdated'?' · source changed':row.freshness==='requires-readback'?' · open to verify receipt':''}</Button>)}
      {selected&&<div className="distribute-evidence"><h3>Review {selected.artifact.kind}</h3><p>Revision {selected.artifact.revision} · {selected.artifact.state} · {selected.freshness==='outdated'?'Source changed; prepare a fresh draft before review.':selected.freshness==='unavailable'?'The selected evidence is unavailable; the retained draft remains readable but cannot be reviewed.':selected.freshness==='readback'?'Saved response read back. Refresh to verify the source before reviewing.':'Exact saved source is current at readback.'}</p>
        <label>Title<input value={title} maxLength={200} disabled={!canEdit||!editable||busy} onChange={event=>{setTitle(event.target.value);setDirty(true);reviseKey.current=''}} /></label>
        {sections.map((section,index)=><div key={section.id} className="distribute-control"><h4>{section.heading}</h4><p>Sources: {section.sourceRefs.join(', ')||'No selected source'}</p><label>Claim or explanation<textarea value={section.body} maxLength={8192} disabled={!canEdit||!editable||busy} onChange={event=>{setSections(rows=>rows.map((row,at)=>at===index?{...row,body:event.target.value}:row));setDirty(true);reviseKey.current=''}} /></label><label>Key points, one per line<textarea value={section.bulletText} maxLength={8192} disabled={!canEdit||!editable||busy} onChange={event=>{setSections(rows=>rows.map((row,at)=>at===index?{...row,bulletText:event.target.value}:row));setDirty(true);reviseKey.current=''}} /></label></div>)}
        <label>Evidence gaps, one per line<textarea value={gaps} disabled={!canEdit||!editable||busy} onChange={event=>{setGaps(event.target.value);setDirty(true);reviseKey.current=''}} /></label><label>Assumptions, one per line<textarea value={assumptions} disabled={!canEdit||!editable||busy} onChange={event=>{setAssumptions(event.target.value);setDirty(true);reviseKey.current=''}} /></label>
        <div className="journey-actions"><Button type="button" variant="primary" disabled={!canEdit||!editable||!dirty||busy} onClick={()=>step(async()=>{reviseKey.current ||= newKey();const view=await distributeArtifactsApi.revise(projectId,selected.artifact,title,sections.map(({bulletText,...section})=>({...section,bullets:lines(bulletText)})),lines(gaps),lines(assumptions),reviseKey.current);setSelected(view);loadEditor(view.artifact);setList(await distributeArtifactsApi.list(projectId));setNotice('New private draft revision saved; review is still required.')})}>Save artifact edits</Button><Button type="button" variant="secondary" disabled={!dirty||busy} onClick={()=>loadEditor(selected.artifact)}>Discard edits</Button><Button type="button" variant="secondary" disabled={!canEdit||!editable||selected.artifact.state!=='draft'||dirty||busy} onClick={()=>step(async()=>{const view=await distributeArtifactsApi.review(projectId,selected.artifact,reviewKeyFor(selected.artifact));setSelected(view);loadEditor(view.artifact);setList(await distributeArtifactsApi.list(projectId));setNotice('Review recorded. This does not publish or certify the artifact.')})}>Mark reviewed</Button><Button type="button" variant="secondary" disabled={!scope||busy||dirty} onClick={()=>step(()=>download('docx'))}>Download DOCX</Button><Button type="button" variant="secondary" disabled={!scope||busy||dirty} onClick={()=>step(()=>download('pptx'))}>Download PPTX</Button><Button type="button" variant="secondary" disabled={!scope||busy||dirty} onClick={()=>step(()=>download('xlsx'))}>Download XLSX</Button></div>
        {selected.history&&<details><summary>Saved revision history</summary>{selected.history.map(row=><p key={row.revision}>Revision {row.revision} · {row.state} · {row.at}</p>)}</details>}
      </div>}
    </>}
  </section>;
}
