import { createContext, Fragment, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from 'react';
import { teamsApi, noPermissions, personalPermissions, type TeamSummary, type TeamPermissions } from '../../api/teams';
import { productApi, errorText } from '../../api/product';
import { ApiError } from '../../api/client';
import { selectTeamScope, selectedTeamId } from '../../api/teamScope';
import { teamWorkspaceStorageKey } from '../../api/teamWorkspacePreference';
import { detachAllTerminals } from '../../api/attachRegistry';
import { resetCockpit, useCockpit } from '../../store/store';
import { useAuth } from '../auth/AuthProvider';
import { JoinTeamInvitation } from './TeamInvitations';
import { WorkflowDialog } from '../common/WorkflowDialog';
import { Button } from '../fluid/components/button';
import './teams.css';
interface TeamWorkspaceValue {
  teams: TeamSummary[]; selected: TeamSummary | null; teamId: string; managedTeamId: string; revision: number;
  refresh(): Promise<void>; select(id: string): void;
}
const TeamWorkspaceContext = createContext<TeamWorkspaceValue | null>(null);
export const useTeamWorkspace = () => useContext(TeamWorkspaceContext);
export function TeamWorkspaceProvider({children}: {children:ReactNode}) {
  const auth = useAuth();
  const userId = auth?.profile.user.id ?? '';
  const storageKey = teamWorkspaceStorageKey(productApi.baseUrl,userId);
  // Account and installation changes discard private invitation input as well as
  // old membership requests before either can affect the next workspace.
  return <ScopedTeamWorkspaceProvider key={storageKey} auth={auth} storageKey={storageKey}>{children}</ScopedTeamWorkspaceProvider>;
}
function ScopedTeamWorkspaceProvider({children,auth,storageKey}: {children:ReactNode;auth:ReturnType<typeof useAuth>;storageKey:string}) {
  const userId = auth?.profile.user.id ?? '';
  const [teams,setTeams] = useState<TeamSummary[]>([]);
  const [selected,setSelected] = useState('');
  const [managedTeamId,setManagedTeamId] = useState('');
  const [ready,setReady] = useState(false);
  const [error,setError] = useState('');
  const [blocked,setBlocked] = useState(false);
  const [retry,setRetry] = useState(0);
  const [epoch,setEpoch] = useState(0);
  const [revision,setRevision] = useState(0);
  const [pending,setPending] = useState<string|null>(null);
  const [signingOut,setSigningOut] = useState(false);
  const [signOutError,setSignOutError] = useState('');
  const request = useRef(0);
  const active = useRef('');
  const drafts = useCockpit(s=>s.unsavedProjectDrafts);
  const apply = useCallback((id:string, initial=false) => {
    detachAllTerminals();
    let sameStoredScope=false;
    try { sameStoredScope=localStorage.getItem(storageKey+'.cockpit')===id; } catch { /* Re-fetch without restoring navigation. */ }
    if(!initial||!sameStoredScope) resetCockpit();
    selectTeamScope(id); active.current=id;
    setSelected(id); setEpoch(v=>v+1); setError(''); setBlocked(false); setPending(null);
    try { localStorage.setItem(storageKey,id); localStorage.setItem(storageKey+'.cockpit',id); } catch { /* Selection can remain session-only. */ }
  },[storageKey]);
  const refresh = useCallback(async()=>{
    const attempt=++request.current;
    try {
      const result=await teamsApi.list();
      if(attempt!==request.current)return;
      setManagedTeamId(result.managedTeamId??'');
      setTeams(result.teams); setRevision(v=>v+1);
      if(result.managedTeamId && result.managedTeamId!==active.current) {
        detachAllTerminals(); setBlocked(true); setError('This service is bound to a different team. Reopen Luci to load its workspace.');
      } else if(active.current && !result.teams.some(row=>row.team.id===active.current)) {
        detachAllTerminals(); setBlocked(true); setError(result.managedTeamId?'You no longer have access to this team. Your saved work remains with the team. Ask a team owner to restore your access.':'You no longer have access to this team. Your saved work remains with the team. Choose another workspace to continue.');
      } else { setError('');setBlocked(false); }
    } catch(cause) { if(attempt===request.current)setError(errorText(cause)); throw cause; }
  },[]);
  useEffect(()=>{
    let live=true;
    setReady(false); setError('');
    let saved=''; try { saved=localStorage.getItem(storageKey)??''; } catch { /* Personal is the safe initial choice. */ }
    if(saved && !/^team-[A-Za-z0-9_-]{22}$/.test(saved)) saved='';
    void teamsApi.list().then(result=>{
      if(!live)return;
      if(result.managedTeamId && !/^team-[A-Za-z0-9_-]{22}$/.test(result.managedTeamId))throw new Error('The service returned an invalid team workspace binding.');
      setTeams(result.teams);
      setManagedTeamId(result.managedTeamId??'');
      saved=result.managedTeamId||saved;
      // Never render personal projects while a remembered team is still resolving.
      apply(saved,true);
      if(saved && !result.teams.some(row=>row.team.id===saved)) {setBlocked(true);setError(result.managedTeamId?'This service is reserved for its managed team. Ask a team owner for access.':'Your saved team is no longer available. Choose another workspace to continue.');}
      setReady(true);
    }).catch(cause=>{if(live)setError(errorText(cause));});
    return()=>{live=false;request.current++;};
  },[storageKey,apply,retry]);
  useEffect(()=>{
    const onFocus=()=>{if(ready)void refresh().catch(()=>{});};
    window.addEventListener('focus',onFocus);
    const timer=window.setInterval(onFocus,60000);
    return()=>{window.removeEventListener('focus',onFocus);window.clearInterval(timer);};
  },[ready,refresh]);
  const select=(id:string)=>{
    if(managedTeamId && id!==managedTeamId)return;
    if(id===selectedTeamId())return;
    if(Object.keys(useCockpit.getState().unsavedProjectDrafts).length) setPending(id); else apply(id);
  };
  const value={teams,selected:teams.find(row=>row.team.id===selected)??null,teamId:selected,managedTeamId,revision,refresh,select};
  const retained=Object.values(drafts).every(draft=>draft.retained);
  const localSetup=Object.values(drafts).some(draft=>draft.purpose==='verification-setup');
  return <TeamWorkspaceContext.Provider value={value}>
    {ready&&<div className="team-app-host" hidden={blocked}><Fragment key={`${userId}:${epoch}`}>{children}</Fragment></div>}
    {ready&&!blocked&&error&&<div className="team-connection-status" role="status">Workspace access could not be refreshed: {error} <button onClick={()=>void refresh().catch(()=>{})}>Retry</button></div>}
    {(!ready||blocked)&&<main className="team-entry">
      <h1>{error?'Workspace unavailable':'Opening your workspace…'}</h1>
      {error&&<><p role="alert">{error}</p><Button onClick={()=>{if(!ready)setRetry(v=>v+1);else void refresh().catch(()=>{});}}>Retry connection</Button>{ready&&!managedTeamId&&<label>Workspace<select value={selected} onChange={event=>select(event.target.value)}><option value="">Personal workspace</option>{!teams.some(row=>row.team.id===selected)&&selected&&<option value={selected} disabled>Unavailable team</option>}{teams.map(row=><option key={row.team.id} value={row.team.id}>{row.team.name}</option>)}</select></label>}</>}
      {ready&&blocked&&managedTeamId===selected&&managedTeamId&&!signingOut&&<JoinTeamInvitation key={JSON.stringify([storageKey,managedTeamId])} expectedTeamId={managedTeamId} onJoined={async()=>{await refresh();}} />}
      {auth&&<><p>Signed in as {auth.profile.user.displayName||auth.profile.user.username||userId}.</p><Button variant="secondary" disabled={signingOut} onClick={()=>{
        setSigningOut(true);setSignOutError('');request.current++;
        void auth.logout().catch(cause=>setSignOutError(errorText(cause))).finally(()=>setSigningOut(false));
      }}>Sign out to use another account</Button>{signOutError&&<p role="alert">{signOutError}</p>}</>}
    </main>}
    {pending!==null&&<WorkflowDialog title="Switch workspace?" description={localSetup?'Your project has unsaved edits. Continue editing here, keep any retained browser drafts, or discard the edits before switching.':'Your project has unsaved edits. Save them before leaving, keep the retained browser drafts, or discard them.'} onClose={()=>setPending(null)}>
      <div className="workflow-actions"><Button variant="secondary" onClick={()=>{Object.values(drafts).forEach(draft=>draft.discard?.());apply(pending);}}>Discard edits and switch</Button>{retained&&<Button variant="secondary" onClick={()=>apply(pending)}>Keep drafts and switch</Button>}<Button onClick={()=>setPending(null)}>Stay here</Button></div>
    </WorkflowDialog>}
  </TeamWorkspaceContext.Provider>;
}
export function useTeamProjectPermissions(projectId:string): {permissions:TeamPermissions;loading:boolean;error:string} {
  const workspace=useTeamWorkspace();
  const teamId=workspace?.teamId??'';
  const revision=workspace?.revision??0;
  const scope=JSON.stringify([teamId,projectId]);
  const key=JSON.stringify([teamId,projectId,revision]);
  const [result,setResult]=useState<{scope:string;key:string;permissions:TeamPermissions;error:string}|null>(null);
  useEffect(()=>{
    if(!teamId||!projectId)return;
    let live=true;
    void teamsApi.projectAccess(teamId,projectId).then(value=>{
      if(live)setResult({scope,key,permissions:value.permissions,error:''});
    }).catch(cause=>{
      if(!live)return;
      const denied=cause instanceof ApiError && [401,403,404].includes(cause.status);
      setResult(previous=>({scope,key,permissions:{...noPermissions,read:!denied&&previous?.scope===scope&&previous.permissions.read},error:errorText(cause)}));
    });
    return()=>{live=false;};
  },[teamId,projectId,scope,key]);
  if(!teamId)return {permissions:personalPermissions,loading:false,error:''};
  if(!projectId)return {permissions:workspace?.selected?.permissions??noPermissions,loading:false,error:''};
  if(result?.key===key)return {permissions:result.permissions,loading:false,error:result.error};
  // Revalidating the same project must not unmount its editors and discard drafts.
  // Keep only its previous read projection until the server confirms current access;
  // writes and execution remain disabled, including for callers that miss loading.
  const permissions=result?.scope===scope?{...noPermissions,read:result.permissions.read}:noPermissions;
  return {permissions,loading:true,error:''};
}
