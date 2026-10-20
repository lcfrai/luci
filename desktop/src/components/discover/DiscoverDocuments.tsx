import { useEffect, useRef, useState } from 'react';
import { discoveryDocumentsApi, discoveryDocumentValid, discoveryDigest, discoverySourceLink, DISCOVERY_CORRECTION_REASON_BYTES, type DiscoveryDocument, type DiscoveryExtraction, type DiscoveryPage, type DiscoverySegment, type DiscoverySourceLink } from '../../api/discoveryDocuments';
import { clearDiscoveryRecovery, readDiscoveryRecovery, retainDiscoveryRecovery, type PendingDiscoveryRequest } from '../../api/discoveryDocumentRecovery';
import { definitionApi, type ProjectSpecification } from '../../api/definition';
import { errorText } from '../../api/product';
import { notifyProjectContextChanged } from '../../api/projectContextEvents';
import { useProjectAccess } from '../../hooks/useProjectAccess';
import { useProjectDraftScope } from '../../hooks/useProjectDraft';
import { useUnsavedProjectDraft } from '../../hooks/useUnsavedProjectDraft';
import { useCockpit } from '../../store/store';
import { Button } from '../fluid/components/button';
import { JourneyHeader } from '../journey/shared';
import './discover-documents.css';
import { SpecificationEvidenceLinks } from './SpecificationEvidenceLinks';

const requestKey=()=>`document-${crypto.randomUUID()}`;
const utf8Bytes=(text:string)=>new TextEncoder().encode(text).length;
export function DiscoverDocuments({projectId}:{projectId:string}) {
  const scope=useProjectDraftScope(projectId),access=useProjectAccess(projectId);
  return <ProjectDiscoverDocuments key={`${scope}:${projectId}`} projectId={projectId} scope={scope} canRead={access.canRead} canEdit={access.canEdit}/>;
}
export function ProjectDiscoverDocuments({projectId,scope,canRead,canEdit}:{projectId:string;scope:string|null;canRead:boolean;canEdit:boolean}) {
  const [documents,setDocuments]=useState<DiscoveryDocument[]>([]),[selected,setSelected]=useState<DiscoveryDocument|null>(null),[nextCursor,setNextCursor]=useState('');
  const [cursor,setCursor]=useState(''),[previous,setPrevious]=useState<string[]>([]),[omitted,setOmitted]=useState(false),[error,setError]=useState(''),[notice,setNotice]=useState(''),[busy,setBusy]=useState(false);
  const [pending,setPending]=useState<PendingDiscoveryRequest|null>(null),[recoveryError,setRecoveryError]=useState(''),[rejected,setRejected]=useState(false);
  const [links,setLinks]=useState<DiscoverySourceLink[]>([]),[question,setQuestion]=useState('');
  const alive=useRef(true),working=useRef(false),generation=useRef(0),requests=useRef(new Set<AbortController>()),download=useRef('');
  const current=useRef({canRead,canEdit});current.current={canRead,canEdit};
  const pendingRef=useRef(pending);pendingRef.current=pending;
 const unsaved=useCockpit(state=>state.unsavedProjectDrafts),correctionDirty=Object.values(unsaved).some(item=>item.projectId===projectId && item.label==="Document correction");
  useEffect(()=>{try{if(scope)setPending(readDiscoveryRecovery(scope));}catch(cause){setRecoveryError(errorText(cause));}},[scope]);
  useEffect(()=>{alive.current=true;return()=>{alive.current=false;generation.current++;for(const request of requests.current)request.abort();if(download.current)URL.revokeObjectURL(download.current);};},[]);
  useEffect(()=>{if(!canRead){generation.current++;for(const request of requests.current)request.abort();setDocuments([]);setSelected(null);setLinks([]);if(download.current){URL.revokeObjectURL(download.current);download.current='';}}},[canRead]);
  async function readPage(after=cursor) {
    if(!current.current.canRead)return;const request=new AbortController();requests.current.add(request);const turn=++generation.current;setError('');
    try{const page=await discoveryDocumentsApi.list(projectId,after,request.signal);if(!alive.current || request.signal.aborted || turn!==generation.current || !current.current.canRead)return;
      if(!page || !Array.isArray(page.documents) || page.documents.length>25 || !page.documents.every(item=>discoveryDocumentValid(item,projectId)) || typeof page.nextCursor!=='string' || typeof page.omitted!=='boolean')throw new Error('The private document page could not be verified.');
      setDocuments(page.documents);setNextCursor(page.nextCursor);setOmitted(page.omitted===true);
    }catch(cause){if(alive.current && !request.signal.aborted)setError(errorText(cause));}finally{requests.current.delete(request);}
  }
  async function refreshDocuments() {
    if(!current.current.canRead || working.current || correctionDirty)return;
    working.current=true;setBusy(true);setError('');setNotice('');
    const request=new AbortController();requests.current.add(request);const turn=++generation.current;
    const prior=selected;
    const currentRead=()=>alive.current && !request.signal.aborted && turn===generation.current && current.current.canRead;
    try{
      const page=await discoveryDocumentsApi.list(projectId,cursor,request.signal);
      if(!currentRead())return;
      if(!page || !Array.isArray(page.documents) || page.documents.length>25 || !page.documents.every(item=>discoveryDocumentValid(item,projectId)) || typeof page.nextCursor!=='string' || typeof page.omitted!=='boolean')throw new Error('The private document page could not be verified.');
      const document=prior?await discoveryDocumentsApi.get(projectId,prior.id,request.signal):null;
      if(!currentRead())return;
      if(prior && (!document || !discoveryDocumentValid(document,projectId) || document.id!==prior.id || document.original.digest!==prior.original.digest || document.revision<prior.revision))throw new Error('The selected document readback could not be verified. Its previous evidence remains open.');
      setDocuments(page.documents);setNextCursor(page.nextCursor);setOmitted(page.omitted);
      if(document){setSelected(document);if(document.revision!==prior?.revision)setLinks([]);}
      setNotice('Saved documents and the open source are refreshed. No extraction or other document action was repeated.');
    }catch(cause){if(currentRead())setError(errorText(cause));}
    finally{requests.current.delete(request);working.current=false;if(alive.current)setBusy(false);}
  }
  useEffect(()=>{if(canRead)void readPage(cursor);},[projectId,canRead,cursor]);
  async function inspect(documentId:string) {
    if(!current.current.canRead || working.current || correctionDirty)return;const request=new AbortController();requests.current.add(request);const turn=++generation.current;setError('');
    try{const document=await discoveryDocumentsApi.get(projectId,documentId,request.signal);if(!alive.current || request.signal.aborted || turn!==generation.current || !current.current.canRead)return;if(!discoveryDocumentValid(document,projectId))throw new Error('The document scope could not be verified.');setSelected(document);setLinks([]);}
    catch(cause){if(alive.current && !request.signal.aborted)setError(errorText(cause));}finally{requests.current.delete(request);}
  }
  function remember(request:PendingDiscoveryRequest) {if(!scope)throw new Error('A signed-in workspace is required to retain this exact document request.');retainDiscoveryRecovery(scope,request);pendingRef.current=request;setPending(request);setRejected(false);}
  function confirmed(request:PendingDiscoveryRequest,document:DiscoveryDocument) {
    if(!scope || !discoveryDocumentValid(document,projectId))throw new Error('The exact document response could not be verified. Keep its retained request.');clearDiscoveryRecovery(scope,request);pendingRef.current=null;setPending(null);setRejected(false);setSelected(document);setLinks([]);void readPage(cursor);
  }
  async function mutate(request:PendingDiscoveryRequest,file?:File) {
    if(!current.current.canRead || !current.current.canEdit || working.current || recoveryError)return;working.current=true;setBusy(true);setError('');setNotice('');const controller=new AbortController();requests.current.add(controller);
    try{remember(request);let document:DiscoveryDocument;
      if(request.action==='upload'){if(!file)throw new Error('Reselect the exact original to retry this retained upload.');if(file.name!==request.original.name || file.size!==request.original.bytes || await discoveryDigest(new Uint8Array(await file.arrayBuffer()))!==request.original.digest)throw new Error('That file does not match the retained upload. Its exact request remains saved.');document=await discoveryDocumentsApi.upload(projectId,file,request.idempotencyKey,controller.signal);}
      else if(request.action==='extract')document=(await discoveryDocumentsApi.start(projectId,request.documentId,request.expectedRevision,request.idempotencyKey,controller.signal)).document;
      else if(request.action==='cancel')document=await discoveryDocumentsApi.cancel(projectId,request.documentId,request.attemptId,request.expectedRevision,request.idempotencyKey,controller.signal);
      else document=await discoveryDocumentsApi.correct(projectId,request.documentId,request.expectedRevision,request.extractionRevision,request.idempotencyKey,request.changes,controller.signal);
      if(!alive.current || controller.signal.aborted || !current.current.canRead)return;confirmed(request,document);setNotice('The exact document receipt is saved. Extracted or corrected text remains evidence for review.');
    }catch(cause){if(alive.current && !controller.signal.aborted)setError(`${errorText(cause)} The exact request is retained; inspect its saved disposition before choosing another action.`);}
    finally{requests.current.delete(controller);working.current=false;if(alive.current)setBusy(false);}
  }
  async function chooseFile(file:File) {
    if(working.current || !current.current.canEdit)return;
    try{if(file.size<1 || file.size>10*1024*1024)throw new Error('Choose an original between 1 byte and 10 MiB.');
      const existing=pendingRef.current;if(existing){if(existing.action!=='upload')throw new Error('Inspect the retained document request first.');await mutate(existing,file);return;}
      const digest=await discoveryDigest(new Uint8Array(await file.arrayBuffer()));if(!alive.current || !current.current.canEdit)return;
      await mutate({action:'upload',idempotencyKey:requestKey(),documentId:'',expectedRevision:0,original:{name:file.name,bytes:file.size,digest,mimeType:file.type || 'application/octet-stream'}},file);
    }catch(cause){if(alive.current)setError(errorText(cause));}
  }
  async function inspectPending() {
    const retained=pendingRef.current;if(!retained || working.current || !canRead)return;setBusy(true);working.current=true;setError('');const request=new AbortController();requests.current.add(request);
    try{const result=await discoveryDocumentsApi.disposition(projectId,retained,request.signal);if(!alive.current || request.signal.aborted || !current.current.canRead)return;
      if(result.state==='recorded' && result.document){confirmed(retained,result.document);setNotice('The exact retained request was recorded. No extraction was replayed.');}
      else if(result.state==='rejected'){setRejected(true);setNotice('The saved revision advanced without this request. Its old CAS cannot apply; your authoring request is retained for comparison.');}
      else if(result.state==='unconfirmed'){setNotice(retained.action==='upload'?'The upload is still unconfirmed. Reselect the exact original to retry the same retained key.':'This request is still unconfirmed. Inspect again or explicitly retry this exact retained request. Its key and revision stay unchanged.');}
      else throw new Error('The exact request disposition is unknown.');
    }catch(cause){if(alive.current && !request.signal.aborted)setError(errorText(cause));}finally{requests.current.delete(request);working.current=false;if(alive.current)setBusy(false);}
  }
  async function inspectOriginal() {
    if(!selected || busy || !canRead)return;setBusy(true);const request=new AbortController();requests.current.add(request);
    try{const blob=await discoveryDocumentsApi.original(projectId,selected,request.signal);if(!alive.current || request.signal.aborted || !current.current.canRead)return;if(download.current)URL.revokeObjectURL(download.current);const url=URL.createObjectURL(blob);download.current=url;const link=document.createElement('a');link.href=url;link.download=selected.original.name;link.click();setTimeout(()=>{URL.revokeObjectURL(url);if(download.current===url)download.current='';},1000);}
    catch(cause){if(alive.current && !request.signal.aborted)setError(errorText(cause));}finally{requests.current.delete(request);if(alive.current)setBusy(false);}
  }
  async function discuss() {
    if(!links.length || busy || !canRead)return;setBusy(true);const request=new AbortController();requests.current.add(request);
    try{const result=await discoveryDocumentsApi.discussion(projectId,links,question,request.signal);if(!alive.current || request.signal.aborted || !current.current.canRead)return;if(result.projectId!==projectId || typeof result.message!=='string' || utf8Bytes(result.message)>12*1024)throw new Error('The scoped discussion projection could not be verified.');window.dispatchEvent(new CustomEvent('luci:orchestrator-open',{detail:{projectId,scope,message:result.message}}));setNotice('Selected evidence is in Luci’s unsent composer. Review the chosen connection and Send when ready.');}
    catch(cause){if(alive.current && !request.signal.aborted)setError(errorText(cause));}finally{requests.current.delete(request);if(alive.current)setBusy(false);}
  }
  const blocked=busy || !!pending || !!recoveryError || !scope;
  if(!canRead)return <p>Current project read access is required to inspect Discover evidence.</p>;
  return <section className="discover-documents" aria-label="Discover document evidence">
    <JourneyHeader title="Understand the work" subtitle="Bring source evidence into your conversation with Luci. Inspect uncertainties and contradictions before accepting project context."/>
    <div className="discover-document-toolbar"><label className="luci-field">Attach a private document<input aria-label="Attach a private document" type="file" disabled={!canEdit || !scope || correctionDirty || busy || !!recoveryError || !!pending && pending.action!=='upload'} onChange={event=>{const file=event.target.files?.[0];if(file)void chooseFile(file);event.target.value='';}}/></label>
      <Button variant="secondary" disabled={busy || correctionDirty} onClick={()=>void refreshDocuments()}>Refresh documents</Button><Button variant="secondary" disabled={busy} onClick={()=>window.dispatchEvent(new CustomEvent('luci:orchestrator-open',{detail:{projectId,scope,message:'Help me discover the project. Investigate the saved context and selected evidence, identify contradictions and useful dependent questions, explain recommended decisions, and propose a source-backed specification through the normal review flow.'}}))}>Talk with Luci</Button>
    </div>
    <p className="journey-muted">Originals stay private to this workspace and project. UTF-8 TXT and Markdown extract locally. PDF and raster originals can be retained, but their required contained extraction/OCR runtime is not available yet. Attaching a file does not send it to a model.</p>
    {correctionDirty && <p role="status">Save or discard your document correction before refreshing saved evidence.</p>}
    {error && <p role="alert">{error}</p>}{notice && <p role="status">{notice}</p>}{recoveryError && <p role="alert">{recoveryError} Preserve the retained request before replacing its storage.</p>}
    {pending && <section className="discover-pending" aria-label="Retained document request"><p>Retained {pending.action} request · {pending.idempotencyKey}. Its response may be uncertain.</p><Button variant="secondary" disabled={busy} onClick={()=>void inspectPending()}>Inspect exact saved request</Button>
      {pending.action!=='upload' && !rejected && <Button variant="secondary" disabled={busy || !canEdit || !!recoveryError} onClick={()=>void mutate(pending)}>Retry exact retained request</Button>}
      {rejected && <Button variant="tertiary" disabled={busy} onClick={()=>{try{if(scope){clearDiscoveryRecovery(scope,pending);pendingRef.current=null;setPending(null);setRejected(false);setNotice('The rejected request was retired locally. Compare your draft with the current saved document before applying a new request.');}}catch(cause){setError(errorText(cause));}}}>Discard rejected request draft</Button>}
    </section>}
    <div className="discover-document-layout"><aside aria-label="Private source documents"><h2>Sources</h2>{documents.length===0 && <p className="journey-muted">Attach evidence, or begin your conversation from the existing project context.</p>}
      <ul>{documents.map(item=><li key={item.id}><button type="button" disabled={busy || correctionDirty} aria-pressed={selected?.id===item.id} onClick={()=>void inspect(item.id)}><span>{item.original.name}</span><small>{item.original.bytes.toLocaleString()} bytes · {item.attempts.length?item.attempts[item.attempts.length-1].status:'retained original'}</small></button></li>)}</ul>
      <div className="discover-document-pagination"><Button variant="tertiary" disabled={!previous.length || busy} onClick={()=>{const values=[...previous];setCursor(values.pop() ?? '');setPrevious(values);}}>Previous sources</Button><Button variant="tertiary" disabled={!nextCursor || busy} onClick={()=>{setPrevious([...previous,cursor]);setCursor(nextCursor);}}>Next sources</Button></div>
      {(omitted || !!nextCursor) && <p role="status" className="journey-muted">More source documents are available on the next page.</p>}
    </aside><div className="discover-document-inspection">
      {selected?<><h2>{selected.original.name}</h2><p className="discover-source-identity">Original SHA-256 <code>{selected.original.digest}</code><br/>Retained by {selected.createdBy.displayName} · document revision {selected.revision}</p>
        <div className="discover-document-toolbar"><Button variant="secondary" disabled={busy} onClick={()=>void inspectOriginal()}>Inspect original file</Button><Button variant="secondary" disabled={blocked || !canEdit || selected.original.mimeType!=='text/plain' && selected.original.mimeType!=='text/markdown'} onClick={()=>void mutate({action:'extract',idempotencyKey:requestKey(),documentId:selected.id,expectedRevision:selected.revision})}>Extract text / start fresh attempt</Button></div>
        {selected.attempts.some(item=>item.status==='extracting') && <p role="status">A saved extraction is claimed. This view does not observe an active worker. Inspect its exact receipt or cancel it before a fresh attempt.</p>}
        {selected.attempts.filter(item=>item.status==='extracting').map(attempt=><Button key={attempt.id} variant="tertiary" disabled={blocked || !canEdit} onClick={()=>void mutate({action:'cancel',idempotencyKey:requestKey(),documentId:selected.id,expectedRevision:selected.revision,attemptId:attempt.id})}>Cancel exact attempt {attempt.id}</Button>)}
        <DocumentEvidence key={`${selected.id}:${selected.revision}`} document={selected} projectId={projectId} scope={scope} canEdit={canEdit && !blocked} busy={busy} links={links} onLinks={setLinks} onCorrect={(extractionRevision,changes)=>void mutate({action:'correct',idempotencyKey:requestKey(),documentId:selected.id,expectedRevision:selected.revision,extractionRevision,changes})}/>
      </>:<p className="journey-muted">Choose a source to inspect its original identity, extraction locations and corrections.</p>}
    </div></div>
    <div className="discover-discussion"><label className="luci-field">Ask about selected evidence<textarea value={question} maxLength={2000} onChange={event=>setQuestion(event.target.value)} placeholder="Which facts conflict, and what decision should we resolve first?"/></label><p>{links.length} exact source segments selected</p><Button variant="primary" disabled={!links.length || busy} onClick={()=>void discuss()}>Discuss selected evidence with Luci</Button>
      <EvidenceContextLinker projectId={projectId} scope={scope} links={links} disabled={!canEdit || blocked || correctionDirty}/>
    </div>
  </section>;
}

function DocumentEvidence({document,projectId,scope,canEdit,busy,links,onLinks,onCorrect}:{document:DiscoveryDocument;projectId:string;scope:string|null;canEdit:boolean;busy:boolean;links:DiscoverySourceLink[];onLinks(links:DiscoverySourceLink[]):void;onCorrect(revision:number,changes:{page:number;segmentId:string;expectedTextDigest:string;text:string;reason:string}[]):void}) {
  const [revision,setRevision]=useState(document.extractions[document.extractions.length-1]?.revision ?? 0),[pageNumber,setPageNumber]=useState(1),[offset,setOffset]=useState(0),[editing,setEditing]=useState<string|null>(null);
  const extraction=document.extractions.find(item=>item.revision===revision),page=extraction?.pages.find(item=>item.page===pageNumber) ?? extraction?.pages[0];
  return <div className="discover-extraction"><label>Extraction revision<select value={revision} disabled={!document.extractions.length || busy || editing!==null} onChange={event=>{setRevision(Number(event.target.value));setPageNumber(1);setOffset(0);setEditing(null);onLinks([]);}}>{!document.extractions.length && <option value={0}>No extraction yet</option>}{document.extractions.map(item=><option key={item.revision} value={item.revision}>{item.revision} · {item.status} · {item.actor.displayName}{item.correctsRevision?` · corrects ${item.correctsRevision}`:''}</option>)}</select></label>
    {extraction && <><p>Extractor {extraction.extractor} · {extraction.extractorVersion}. Text and OCR remain evidence, not accepted facts.</p>{extraction.issues.map((issue,index)=><p className="journey-warning" key={index}>{issue}</p>)}
      {revision!==document.extractions[document.extractions.length-1]?.revision && <p role="status">A newer extraction exists. This view preserves the exact older source; accepted context does not change automatically.</p>}
      <label>Source page<select value={page?.page ?? 0} disabled={!extraction.pages.length || busy || editing!==null} onChange={event=>{setPageNumber(Number(event.target.value));setOffset(0);setEditing(null);}}>{extraction.pages.map(item=><option key={item.page} value={item.page}>Page {item.page}</option>)}</select></label>
      {page?.segments.slice(offset,offset+50).map(segment=>{const link=discoverySourceLink(document,extraction,page,segment),key=JSON.stringify(link),selected=links.some(item=>JSON.stringify(item)===key);return <article className="discover-evidence-segment" key={segment.id}>
        <label><input type="checkbox" checked={selected} disabled={busy || !selected && links.length>=40} onChange={event=>onLinks(event.target.checked?[...links,link]:links.filter(item=>JSON.stringify(item)!==key))}/>Page {page.page} · {segment.location.firstLine?`line ${segment.location.firstLine}`:`segment ${segment.id}`} · {segment.kind}{segment.confidence!==null?` · OCR confidence ${segment.confidence}%`:''}</label><p className="discover-source-text">{segment.text}</p>{segment.uncertainty && <p className="journey-muted">{segment.uncertainty}</p>}
        {editing===segment.id?<ExtractionCorrection key={segment.id} projectId={projectId} scope={scope} extraction={extraction} page={page} segment={segment} disabled={!canEdit} onSave={change=>onCorrect(extraction.revision,[change])} onClose={()=>setEditing(null)}/>:<Button variant="tertiary" disabled={!canEdit || editing!==null || revision!==document.extractions[document.extractions.length-1]?.revision} onClick={()=>setEditing(segment.id)}>Correct this extraction</Button>}
      </article>;})}
      {page && <div className="discover-document-pagination"><Button variant="tertiary" disabled={offset===0 || editing!==null} onClick={()=>setOffset(Math.max(0,offset-50))}>Previous text segments</Button><span>{page.segments.length?offset+1:0}–{Math.min(offset+50,page.segments.length)} of {page.segments.length}</span><Button variant="tertiary" disabled={offset+50>=page.segments.length || editing!==null} onClick={()=>setOffset(offset+50)}>Next text segments</Button></div>}
    </>}
  </div>;
}
function ExtractionCorrection({projectId,scope,page,segment,disabled,onSave,onClose}:{projectId:string;scope:string|null;extraction:DiscoveryExtraction;page:DiscoveryPage;segment:DiscoverySegment;disabled:boolean;onSave(change:{page:number;segmentId:string;expectedTextDigest:string;text:string;reason:string}):void;onClose():void}) {
  const [text,setText]=useState(segment.text),[reason,setReason]=useState('');const dirty=text!==segment.text || reason!=='';
  useUnsavedProjectDraft(projectId,dirty,'Document correction','discover',scope?{retained:false,discard:()=>{setText(segment.text);setReason('');}}:undefined);
  const valid=!!reason.trim() && utf8Bytes(reason)<=DISCOVERY_CORRECTION_REASON_BYTES && utf8Bytes(text)<=64*1024;
  return <fieldset disabled={disabled} className="discover-correction"><legend>Correct source text</legend><label>Corrected text<textarea rows={5} value={text} onChange={event=>setText(event.target.value)}/></label><label>Reason for correction<textarea rows={2} value={reason} onChange={event=>setReason(event.target.value)}/></label><p className="journey-muted">Reason: {utf8Bytes(reason)} / {DISCOVERY_CORRECTION_REASON_BYTES} UTF-8 bytes, including space reserved for the retained attribution prefix. The original and earlier extraction remain unchanged.</p><Button variant="secondary" disabled={!valid || disabled} onClick={()=>onSave({page:page.page,segmentId:segment.id,expectedTextDigest:segment.textDigest,text,reason})}>Save a new extraction revision</Button><Button variant="tertiary" onClick={()=>{if(!dirty)onClose();else{setText(segment.text);setReason('');}}}>{dirty?'Discard correction draft':'Close correction'}</Button></fieldset>;
}
function EvidenceContextLinker({projectId,scope,links,disabled}:{projectId:string;scope:string|null;links:DiscoverySourceLink[];disabled:boolean}) {
  const [spec,setSpec]=useState<ProjectSpecification|null>(null),[target,setTarget]=useState('purpose:'),[busy,setBusy]=useState(false),[error,setError]=useState(''),[notice,setNotice]=useState(''),[evidenceDirty,setEvidenceDirty]=useState(false);
  const drafts=useCockpit(state=>state.unsavedProjectDrafts),definitionDirty=Object.values(drafts).some(item=>item.projectId===projectId && item.label==='Definition');
  const active=useRef(true);useEffect(()=>{active.current=true;return()=>{active.current=false;};},[]);
  useUnsavedProjectDraft(projectId,evidenceDirty,'Source reference draft','discover',scope?{retained:false,discard:()=>{setSpec(null);setEvidenceDirty(false);}}:undefined);
  async function load(){setBusy(true);setError('');try{const value=await definitionApi.get(projectId);if(active.current){setSpec(value);setEvidenceDirty(false);}}catch(cause){if(active.current)setError(errorText(cause));}finally{if(active.current)setBusy(false);}}
  return <div className="discover-context-links"><Button variant="secondary" disabled={disabled || busy || evidenceDirty} onClick={()=>void load()}>Review evidence links for specification</Button>{evidenceDirty && <Button variant="tertiary" disabled={busy} onClick={()=>{setSpec(null);setEvidenceDirty(false);}}>Discard source reference draft</Button>}{spec && <>
    <label>Context supported by this evidence<select value={target} disabled={disabled || busy} onChange={event=>setTarget(event.target.value)}><option value="purpose:">Purpose</option><option value="audience:">Audience</option><option value="constraint:">Constraints</option>{spec.requirements.map(item=><option key={item.id} value={`requirement:${item.id}`}>{item.id} · {item.title}</option>)}{spec.glossary.map(item=><option key={item.id} value={`glossary:${item.id}`}>Term · {item.term}</option>)}{spec.decisions.map(item=><option key={item.id} value={`decision:${item.id}`}>Decision · {item.title}</option>)}</select></label>
    <SpecificationEvidenceLinks specification={spec} disabled={disabled || busy} onChange={evidence=>{setSpec({...spec,evidence});setEvidenceDirty(true);}}/>
    <p className="journey-muted">Save exact source references in a new draft specification revision. This does not approve the context or start development.</p><Button variant="secondary" disabled={disabled || busy || definitionDirty || !links.length && !evidenceDirty} onClick={async()=>{setBusy(true);setError('');try{const [kind,id]=target.split(':');const evidence=[...(spec.evidence ?? [])];for(const link of links){const next={...link,targetKind:kind as DiscoverySourceLink['targetKind'],targetId:id};if(!evidence.some(item=>JSON.stringify(item)===JSON.stringify(next)))evidence.push(next);}const value=await definitionApi.save(projectId,{...spec,evidence},spec.revision,spec.briefRevision);if(!active.current)return;setSpec(value);setEvidenceDirty(false);notifyProjectContextChanged(scope,projectId);setNotice('Exact source links are saved in the draft specification. Review the normal context approval before using them for implementation.');}catch(cause){if(active.current)setError(errorText(cause));}finally{if(active.current)setBusy(false);}}}>Save source links in draft specification</Button>{definitionDirty && <p role="status">Resolve the existing Discover definition draft before saving these links.</p>}
  </>}{error && <p role="alert">{error}</p>}{notice && <p role="status">{notice}</p>}</div>;
}
