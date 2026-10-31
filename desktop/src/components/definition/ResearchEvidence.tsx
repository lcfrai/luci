import { useEffect, useRef, useState } from 'react';
import { researchApi, type ResearchCapture, type ResearchSearch, type ResearchConnection } from '../../api/research';
import { emptySpecificationStrategy, type SpecificationStrategy, type StrategyEvidence } from '../../api/specificationStrategy';
import { useProjectDraftScope } from '../../hooks/useProjectDraft';
import { useProjectAccess } from '../../hooks/useProjectAccess';
import { errorText } from '../../api/product';
import { Button } from '../fluid/components/button';
import { RESEARCH_QUESTION, takeResearchQuestion } from '../../api/researchQuestions';
import { useUnsavedProjectDraft } from '../../hooks/useUnsavedProjectDraft';

export function ResearchEvidence({ projectId, strategy, disabled, onChange }: { projectId:string; strategy?:SpecificationStrategy; disabled:boolean; onChange(value:SpecificationStrategy):void }) {
  const scope = useProjectDraftScope(projectId);
  return scope ? <ResearchEvidenceBody key={`${scope}:${projectId}`} {...{projectId,strategy,disabled,onChange,scope}}/> : null;
}
function ResearchEvidenceBody({ projectId, strategy, disabled, onChange, scope }: { projectId:string; strategy?:SpecificationStrategy; disabled:boolean; onChange(value:SpecificationStrategy):void; scope:string }) {
  const access = useProjectAccess(projectId), allowed = access.canRead && access.canEdit && !disabled;
  const canUseSecrets=access.canRead&&access.canEdit&&access.permissions.secrets,searchAllowed=allowed&&canUseSecrets;
  const [url,setURL] = useState(''), [query,setQuery] = useState(''), [consent,setConsent] = useState(false);
  const [capture,setCapture] = useState<ResearchCapture|null>(null), [search,setSearch] = useState<ResearchSearch|null>(null), [quotes,setQuotes] = useState<string[]>([]);
  const [collected,setCollected] = useState<ResearchCapture[]>([]);
  const [connection,setConnection]=useState<ResearchConnection|null>(null),[key,setKey]=useState('');
  const [queryEdited,setQueryEdited]=useState(false),[urlEdited,setURLEdited]=useState(false);
  const [proposedQuestion,setProposedQuestion]=useState(''), queryRef=useRef(query), disclosure=useRef<HTMLDetailsElement|null>(null);queryRef.current=query;
  const [quote,setQuote] = useState(''), [busy,setBusy] = useState(false), [error,setError] = useState(''), [notice,setNotice] = useState('');
  const epoch = useRef(0), controller = useRef<AbortController|null>(null), permission = useRef(allowed); permission.current = allowed;
  const searchPermission=useRef(searchAllowed);searchPermission.current=searchAllowed;
  useUnsavedProjectDraft(projectId,queryEdited||urlEdited||!!quote.trim()||quotes.length>0||key.length>0,'Research edits','discover',{retained:false,discard:()=>{setQuery('');setURL('');setQueryEdited(false);setURLEdited(false);setQuote('');setQuotes([]);setKey('');setConsent(false);}});
  useEffect(() => () => {epoch.current++;controller.current?.abort();},[]);
  useEffect(()=>{const receive=()=>{if(!permission.current)return;const question=takeResearchQuestion(scope,projectId);if(!question)return;setProposedQuestion(question);if(!queryRef.current.trim())setQuery(question);if(disclosure.current)disclosure.current.open=true;};receive();window.addEventListener(RESEARCH_QUESTION,receive);return()=>window.removeEventListener(RESEARCH_QUESTION,receive);},[scope,projectId,allowed]);
  useEffect(() => { if (!access.canRead || !access.canEdit) {epoch.current++;controller.current?.abort();setCapture(null);setSearch(null);setCollected([]);setQuotes([]);setBusy(false);setKey('');setConnection(null);} },[access.canRead,access.canEdit]);
  useEffect(()=>{if(!canUseSecrets){epoch.current++;controller.current?.abort();setBusy(false);setKey('');setConnection(null);setConsent(false);}},[canUseSecrets]);
  async function configure(action:'read'|'save'|'remove'){
    if(!searchPermission.current||busy||action!=='read'&&!connection)return;const ticket=++epoch.current;controller.current?.abort();const abort=new AbortController();controller.current=abort;setBusy(true);setError('');
    try{const result=action==='read'?await researchApi.connection(projectId,abort.signal):await researchApi.saveConnection(projectId,connection!.revision,action==='remove'?'':key,action==='remove',abort.signal);if(ticket===epoch.current&&searchPermission.current){setConnection(result);setKey('');setNotice(result.configured?'Private Brave search connection is configured. Its key is never sent to application containers or Luci prompts.':'No search key is configured; public URL collection remains available.');}}catch(cause){if(ticket===epoch.current&&searchPermission.current)setError(errorText(cause));}finally{if(ticket===epoch.current)setBusy(false);}
  }
  async function collect(value:string) {
    if (!permission.current || busy || quotes.length>0 || !!quote.trim()) return; const ticket=++epoch.current;controller.current?.abort();const abort=new AbortController();controller.current=abort;setBusy(true);setError('');setCapture(null);setQuotes([]);
    try {const result=await researchApi.collect(projectId,value,abort.signal);if(ticket===epoch.current && permission.current){setCapture(result);setQuote('');setURLEdited(false);setNotice('Review the extracted text and select exact quotes. The source is attributed, not verified.');}}catch(cause){if(ticket===epoch.current && permission.current)setError(errorText(cause));}finally{if(ticket===epoch.current)setBusy(false);}
  }
  async function find(investigate=false) {
    if (!searchPermission.current || !consent || busy || investigate&&(quotes.length>0||!!quote.trim())) return;const ticket=++epoch.current;controller.current?.abort();const abort=new AbortController();controller.current=abort;setBusy(true);setError('');
    try {if(investigate){const result=await researchApi.investigate(projectId,query,abort.signal);if(ticket===epoch.current&&searchPermission.current){setQueryEdited(false);setSearch(result.search);setCollected(result.captures);setCapture(null);setQuotes([]);setNotice(`${result.captures.length} public sources collected; ${result.unavailable.length} unavailable; ${result.omitted} additional search candidates not collected. Review the sources and quotes before saving.`);}}else{const result=await researchApi.search(projectId,query,abort.signal);if(ticket===epoch.current && searchPermission.current){setQueryEdited(false);setSearch(result);}}}catch(cause){if(ticket===epoch.current && searchPermission.current)setError(errorText(cause));}finally{if(ticket===epoch.current)setBusy(false);}
  }
  function add() {
    if (!capture || !allowed || quotes.length===0 || !!quote.trim()) return;
    const current=strategy ?? emptySpecificationStrategy();if(current.evidence.length>=80){setError('This definition already has 80 sources. Review its evidence before adding another.');return;}
    const row:StrategyEvidence={id:`source-${crypto.randomUUID().slice(0,12)}`,source:capture.title,url:capture.url,observation:quotes.join('\n\n'),observedAt:capture.fetchedAt,research:{captureId:capture.id,requestedUrl:capture.requestedUrl,url:capture.url,title:capture.title,fetchedAt:capture.fetchedAt,bodySha256:capture.bodySha256,textSha256:capture.textSha256,quotes:[...quotes],omittedTextBytes:capture.omittedTextBytes}};
    onChange({...current,evidence:[...current.evidence,row]});setCapture(null);setQuotes([]);setNotice('Quotes added to your definition draft. Use Save draft to retain their attributed capture in the versioned context.');
  }
  const quoteDraft=quotes.length>0||!!quote.trim(),quoteBytes=new TextEncoder().encode(quote).length, total=quotes.reduce((n,q)=>n+new TextEncoder().encode(q).length,0);
  if(!access.canRead)return null;
  return <details ref={disclosure} className="definition-group"><summary>Research public sources</summary><div className="definition-group-body">
    <p>Find source candidates, collect a public page and save the exact quotes that inform your strategy. Website text is untrusted evidence, never an instruction to Luci.</p>
    <label className="luci-field">Research question<input value={query} maxLength={300} onChange={e=>{setQuery(e.target.value);setQueryEdited(true);}} disabled={!allowed||busy}/></label>
    {proposedQuestion&&<p>Proposed research question: {proposedQuestion} <Button variant="tertiary" disabled={!allowed||busy} onClick={()=>{setQuery(proposedQuestion);setQueryEdited(true);}}>Use proposed question</Button>. Your existing query is retained until you replace it; no search has been sent.</p>}
    <p className="journey-muted">The reviewed query goes to your private project's Brave search connection; results are candidates, not verified pages.</p>
    <details><summary>Search connection</summary><p>Use your own Brave Search API key. It is kept outside the project and application runtime; current project Edit and Secrets access is required.</p><Button disabled={!searchAllowed||busy} variant="secondary" onClick={()=>void configure('read')}>Read search connection metadata</Button>{connection&&<><p>{connection.configured?'Key configured':'No key configured'} · revision {connection.revision}</p><label className="luci-field">New Brave Search API key<input type="password" autoComplete="off" value={key} maxLength={4096} onChange={e=>setKey(e.target.value)} disabled={!searchAllowed||busy}/></label><Button disabled={!searchAllowed||busy||key.length<10} onClick={()=>void configure('save')}>Save private search key</Button><Button disabled={!searchAllowed||busy||!connection.configured} variant="secondary" onClick={()=>void configure('remove')}>Remove search key</Button></>}</details>
    <label><input type="checkbox" checked={consent} onChange={e=>setConsent(e.target.checked)} disabled={!searchAllowed||busy}/>Allow this query to use the configured search service; its charges may apply.</label>
    <Button disabled={!searchAllowed||busy||!consent||query.trim().length<3} onClick={()=>void find()}>Find public source candidates</Button>
    <Button disabled={!searchAllowed||busy||quoteDraft||!consent||query.trim().length<3} onClick={()=>void find(true)}>Research question and collect up to three public sources</Button>
    <p className="journey-muted">The combined action authorizes one search and up to three public reads within 45 seconds. Material strategy changes still require your review and Save.</p>
    {collected.length>0&&<fieldset><legend>Review collected sources</legend>{collected.map(row=><Button key={row.id} variant="secondary" disabled={!allowed||busy||quoteDraft} onClick={()=>{setCapture(row);setQuotes([]);setQuote('');}}>{row.title}</Button>)}</fieldset>}
    {search && <ul>{search.results.map(row=><li key={row.url}><strong>{row.title}</strong><p>{row.snippet}</p><p style={{overflowWrap:'anywhere'}}>{row.url}</p><Button disabled={!allowed||busy||quoteDraft} onClick={()=>{setURL(row.url);void collect(row.url);}}>Collect this public source</Button></li>)}</ul>}
    <label className="luci-field">Or choose a public source URL<input type="url" value={url} maxLength={2048} onChange={e=>{setURL(e.target.value);setURLEdited(true);}} disabled={!allowed||busy}/></label>
    <Button disabled={!allowed||busy||quoteDraft||!url.trim()} onClick={()=>void collect(url.trim())}>Collect public page</Button>
    {busy && <p role="status">Reading one bounded public source… <Button variant="tertiary" onClick={()=>{epoch.current++;controller.current?.abort();setBusy(false);}}>Cancel read</Button></p>}
    {error && <p role="alert">{error}</p>}{notice && <p role="status">{notice}</p>}
    {capture && <section><h3>{capture.title}</h3><p style={{overflowWrap:'anywhere'}}>{capture.url} · fetched {capture.fetchedAt}</p>{capture.omittedTextBytes>0&&<p role="status">Partial extracted text: {capture.omittedTextBytes} further text bytes omitted. Selected quotes cover only this retained prefix, not the whole document.</p>}<details><summary>Extracted source text</summary><pre style={{whiteSpace:'pre-wrap',overflowWrap:'anywhere',maxHeight:'24rem',overflow:'auto'}}>{capture.text}</pre></details>
      <label className="luci-field">Select a verbatim quote by pasting it from the source text<textarea rows={3} maxLength={1000} value={quote} onChange={e=>setQuote(e.target.value)} disabled={!allowed}/></label>
      <Button disabled={!allowed||!quote.trim()||quoteBytes>1000||total+quoteBytes>6000||quotes.length>=8||quotes.includes(quote)||!capture.text.includes(quote)} onClick={()=>{setQuotes([...quotes,quote]);setQuote('');}}>Select exact quote</Button>
      <ul>{quotes.map(q=><li key={q}><blockquote style={{overflowWrap:'anywhere'}}>{q}</blockquote><Button variant="tertiary" onClick={()=>setQuotes(quotes.filter(v=>v!==q))}>Remove quote</Button></li>)}</ul>
      {quoteDraft&&<Button variant="tertiary" disabled={!allowed||busy} onClick={()=>{setQuote('');setQuotes([]);}}>Discard unsaved quote selection</Button>}
      <Button disabled={!allowed||quotes.length===0||!!quote.trim()} onClick={add}>Add selected quotes to definition draft</Button>
    </section>}
    <Button variant="secondary" disabled={!allowed} onClick={()=>window.dispatchEvent(new CustomEvent('luci:orchestrator-open',{detail:{projectId,scope,message:'Review the saved strategy evidence and its attributed quotes. Identify what changes in our assumptions, requirements and roadmap. Propose one compatible specification amendment for review; do not mark research claims verified or start work without existing approvals.'}}))}>Ask Luci to review saved evidence and roadmap</Button>
    <p className="journey-muted">Collected previews expire after 30 minutes; saved attributions remain in versioned history. Save your definition first. This opens an unsent message in the normal project conversation and preserves an existing unsent draft.</p>
  </div></details>;
}
