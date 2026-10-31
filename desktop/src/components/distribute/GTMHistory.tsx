import { useEffect, useRef, useState } from 'react';
import { gtmWorkspaceApi, type GTMArchiveChoice, type GTMHistoryView, type GTMView } from '../../api/gtmWorkspace';
import { errorText } from '../../api/product';
import { Button } from '../fluid/components/button';

// This viewer cannot restore a revision into the editor or issue a save. The
// parent retains its ordinary unsaved draft while history is read/exported.
export function GTMHistory({projectId,scope,view,canRead}:{projectId:string;scope:string|null;view:GTMView;canRead:boolean}){
  const [history,setHistory]=useState<{owner:string;view:GTMHistoryView}|null>(null),[busy,setBusy]=useState(false),[error,setError]=useState('');
  const request=useRef<AbortController|null>(null),generation=useRef(0),mounted=useRef(true),downloads=useRef(new Map<string,ReturnType<typeof setTimeout>>());
  const key=JSON.stringify([scope,projectId,canRead]);const owner=useRef(key);if(owner.current!==key){owner.current=key;generation.current++;request.current?.abort();}
  useEffect(()=>{mounted.current=true;return()=>{mounted.current=false;generation.current++;request.current?.abort();for(const [url,timer]of downloads.current){clearTimeout(timer);URL.revokeObjectURL(url);}downloads.current.clear();};},[]);
  useEffect(()=>{setHistory(null);setBusy(false);setError('');},[key]);
  async function read(archive?:GTMArchiveChoice,revision?:GTMView['history'][number]){
    if(!canRead||!scope||busy)return;const controller=new AbortController(),ticket=++generation.current;request.current?.abort();request.current=controller;setBusy(true);setError('');setHistory(null);
    const current=()=>mounted.current&&!controller.signal.aborted&&generation.current===ticket&&owner.current===key;
    try{const value=archive?await gtmWorkspaceApi.archive(projectId,archive,controller.signal):await gtmWorkspaceApi.revision(projectId,revision!.revision,revision!.digest,controller.signal);if(current())setHistory({owner:key,view:value});}catch(cause){if(current())setError(errorText(cause));}finally{if(current()){request.current=null;setBusy(false);}}
  }
  const visible=history&&history.owner===key?history.view:null;
  function exportHistory(){
    if(!visible||!canRead||!scope||owner.current!==key||busy)return;
    const url=URL.createObjectURL(new Blob([JSON.stringify(visible,null,2)+'\n'],{type:'application/json'}));const timer=setTimeout(()=>{URL.revokeObjectURL(url);downloads.current.delete(url);},1000);downloads.current.set(url,timer);
    const link=document.createElement('a');link.href=url;link.download=visible.archiveId?`luci-gtm-revisions-${visible.revisions[0].revision}-${visible.revisions[visible.revisions.length-1].revision}.json`:`luci-gtm-revision-${visible.revisions[0].revision}.json`;link.click();
  }
  if(!canRead||!scope)return <p>Current project read access is required to view saved GTM history.</p>;
  return <details><summary>Saved GTM history ({(view.archivedRevisions??0)+view.history.length} revisions)</summary><p>Older revisions remain unchanged in private archives. Viewing or exporting history keeps your current edits. Historical source links and results are recorded evidence, not current project approval.</p>
    <p>Storage: up to 50 active revisions, 128 archived chunks and 64 MiB of archives. Saving continues automatically in a new chunk; no old revision is deleted. Interrupted archive attempts remain preserved and their reserved space counts toward this capacity.</p>
    <ul>{view.history.map(row=><li key={row.revision}>Revision {row.revision} · {new Date(row.at).toLocaleString()} <Button type="button" variant="tertiary" disabled={!canRead||!scope||busy} onClick={()=>void read(undefined,row)}>View revision {row.revision}</Button></li>)}</ul>
    {!!view.archives?.length&&<ul aria-label="Older GTM history">{view.archives.map(archive=><li key={archive.id}>Revisions {archive.firstRevision}–{archive.lastRevision} · {archive.revisions} saved revisions <Button type="button" variant="secondary" disabled={!canRead||!scope||busy} onClick={()=>void read(archive)}>Open revisions {archive.firstRevision}–{archive.lastRevision}</Button></li>)}</ul>}
    {busy&&<p role="status">Reading the exact saved history…</p>}{error&&<p role="alert">{error}</p>}
    {canRead&&scope&&visible&&<section aria-label="Historical GTM revisions"><h3>Historical revisions</h3><Button type="button" variant="secondary" disabled={busy} onClick={exportHistory}>Export these historical revisions</Button>{visible.revisions.map(row=><details key={row.revision}><summary>Revision {row.revision} · {new Date(row.at).toLocaleString()}</summary><p>{row.actor.displayName} · source {row.sourceDigest} · revision {row.digest}</p><pre tabIndex={0}>{JSON.stringify(row.content,null,2)}</pre></details>)}</section>}
  </details>;
}
