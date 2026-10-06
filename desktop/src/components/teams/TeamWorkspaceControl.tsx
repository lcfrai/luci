import { useCallback, useEffect, useId, useRef, useState, type ComponentProps } from 'react';
import { teamsApi, emptyGrants, type TeamDetail, type TeamGrants, type TeamMember, type ProjectGrant, type ProjectAccess } from '../../api/teams';
import { errorText } from '../../api/product';
import { useCockpit } from '../../store/store';
import { useTeamWorkspace } from './TeamWorkspaceProvider';
import { WorkflowDialog } from '../common/WorkflowDialog';
import { TabsSubtle, TabsSubtleItem, TabsSubtlePanel } from '../fluid/components/tabs-subtle';
import { Button } from '../fluid/components/button';
import './teams.css';
import { TeamServiceStatus } from '../auth/TeamServiceStatus';
import { TeamDataLifecycle } from './TeamDataLifecycle';
import { TeamAudit } from './TeamAudit';
import { TeamInvitations, JoinTeamInvitation } from './TeamInvitations';
import { BillingReturnNavigation } from './BillingReturnNavigation';
import type { BillingReturnKind } from '../../api/billingReturn';
const grantLabels: [keyof TeamGrants,string][] = [['execute','Run agents and checks'],['secrets','Use and manage credentials'],['deploy','Deploy applications'],['admin','Manage team access']];
function Grants({value,onChange,disabled=false,project=false}: {value:TeamGrants;onChange(value:TeamGrants):void;disabled?:boolean;project?:boolean}) {
  return <div className="team-grants">{grantLabels.filter(([key])=>!project||key!=='admin').map(([key,label])=><label key={key}><input type="checkbox" checked={value[key]} disabled={disabled} onChange={event=>onChange({...value,[key]:event.target.checked})}/>{label}</label>)}</div>;
}
function AccessEditor({label,initial,ownerAllowed,project=false,busy,onSave,onRemove}: {
  label:string;initial:Pick<TeamMember,'role'|'grants'>;ownerAllowed:boolean;project?:boolean;busy:boolean;
  onSave(role:TeamMember['role'],grants:TeamGrants):Promise<void>;onRemove():Promise<void>;
}) {
  const [role,setRole]=useState(initial.role);
  const [grants,setGrants]=useState(initial.grants);
  const [confirm,setConfirm]=useState(false);
  return <fieldset className="team-member"><legend>{label}</legend>
    {initial.role==='owner'&&!ownerAllowed&&<p className="team-muted">Only a team owner can change another owner’s access.</p>}
    <label>Role<select aria-label="Role" value={role} disabled={busy||initial.role==='owner'&&!ownerAllowed} onChange={event=>setRole(event.target.value as TeamMember['role'])}><option value="viewer">Viewer</option><option value="editor">Editor</option>{(!project&&(ownerAllowed||initial.role==='owner'))&&<option value="owner">Owner</option>}</select></label>
    <Grants value={grants} onChange={setGrants} disabled={busy||initial.role==='owner'&&!ownerAllowed} project={project}/>
    <div className="workflow-actions"><Button disabled={busy||initial.role==='owner'&&!ownerAllowed} onClick={()=>void onSave(role,role==='owner'?{...grants,admin:true}:grants)}>Save access</Button><Button variant="secondary" disabled={busy||initial.role==='owner'&&!ownerAllowed} onClick={()=>setConfirm(true)}>Remove access</Button></div>
    {confirm&&<div><p>Remove {label}’s {project?'project':'team'} access? Their existing shared work and running resources will remain. Authorized members can stop those resources.</p><div className="workflow-actions"><Button disabled={busy} onClick={()=>void onRemove()}>Confirm removal</Button><Button variant="secondary" onClick={()=>setConfirm(false)}>Keep access</Button></div></div>}
  </fieldset>;
}
function AccessRow(props:ComponentProps<typeof AccessEditor>) {
  return <details className="team-access-row"><summary aria-label={`${props.project?'Project':'Team'} access for ${props.label}`}><span><strong>{props.label}</strong><span className="team-muted">{props.initial.role}</span></span><span>Edit {props.project?'project':'team'} access</span></summary><AccessEditor {...props}/></details>;
}
export function TeamWorkspaceControl() {
  const workspace=useTeamWorkspace();
  const [open,setOpen]=useState(false);
  const [billingReturn,setBillingReturn]=useState<BillingReturnKind|null>(null);
  const openBilling=useCallback((kind:BillingReturnKind)=>{setBillingReturn(kind);setOpen(true);},[]);
  if(!workspace)return null;
  return <div className="team-workspace-control"><label htmlFor="team-workspace">Workspace</label><select id="team-workspace" value={workspace.selected?.team.id??''} onChange={event=>workspace.select(event.target.value)}>{!workspace.managedTeamId&&<option value="">Personal workspace</option>}{workspace.teams.map(row=><option key={row.team.id} value={row.team.id}>{row.team.name}</option>)}</select><button className="rail-settings" onClick={()=>{setBillingReturn(null);setOpen(true);}}>{workspace.selected?'Team access':'Manage teams'}</button><BillingReturnNavigation onOpenBilling={openBilling}/>{open&&<TeamPanel billingReturn={billingReturn} onClose={()=>setOpen(false)}/>}</div>;
}
function TeamPanel({onClose,billingReturn=null}:{onClose():void;billingReturn?:BillingReturnKind|null}) {
  const workspace=useTeamWorkspace()!;
  const teamId=workspace.selected?.team.id??'';
  const projectId=useCockpit(state=>state.selection.projectId)??'';
  const projectName=useCockpit(state=>state.projects.find(project=>project.id===projectId)?.name)??'Selected project';
  const tabsId=useId();
  const [tab,setTab]=useState(billingReturn?2:0);
  const working=useRef(false);
  const [detail,setDetail]=useState<TeamDetail|null>(null);
  const [projectAccess,setProjectAccess]=useState<ProjectAccess|null>(null);
  const [error,setError]=useState('');
  const [busy,setBusy]=useState(false);
  const [loading,setLoading]=useState(!!teamId);
  const [name,setName]=useState('');
  const [username,setUsername]=useState('');
  const [role,setRole]=useState<TeamMember['role']>('viewer');
  const [grants,setGrants]=useState<TeamGrants>({...emptyGrants});
  const [sharing,setSharing]=useState('');
  const [shareRole,setShareRole]=useState<ProjectGrant['role']>('viewer');
  const [shareGrants,setShareGrants]=useState<TeamGrants>({...emptyGrants});
  const refresh=async()=>{
    await workspace.refresh();
    if(!teamId)return;
    const value=await teamsApi.detail(teamId);setDetail(value);
    if(projectId&&value.permissions.admin)setProjectAccess(await teamsApi.projectAccess(teamId,projectId));
  };
  useEffect(()=>{
    let live=true;
    setDetail(null);setProjectAccess(null);setError('');setLoading(!!teamId);
    if(teamId)void teamsApi.detail(teamId).then(async value=>{
      if(!live)return;setDetail(value);
      if(projectId&&value.permissions.admin){const access=await teamsApi.projectAccess(teamId,projectId);if(live)setProjectAccess(access);}
    }).catch(cause=>{if(live)setError(errorText(cause));}).finally(()=>{if(live)setLoading(false);});
    return()=>{live=false;};
  },[teamId,projectId]);
  const perform=async(action:()=>Promise<unknown>)=>{
    if(working.current)return;working.current=true;setBusy(true);setError('');
    try{await action();await refresh();}catch(cause){setError(errorText(cause));try{await refresh();}catch{/* Keep the actionable original error. */}}finally{working.current=false;setBusy(false);}
  };
  const admin=!!detail?.permissions.admin;
  const owner=detail?.membership.role==='owner';
  const addMember=()=>perform(async()=>{if(!detail)return;await teamsApi.addMember(teamId,detail.team.revision,username,role,role==='owner'?{...grants,admin:true}:grants);setUsername('');setGrants({...emptyGrants});});
  const saveGrant=(grant:Pick<ProjectGrant,'userId'|'role'|'grants'>)=>perform(()=>teamsApi.saveProjectGrant(teamId,projectId,detail!.team.revision,grant));
  return <WorkflowDialog title={teamId?'Team access':'Your teams'} description={teamId?'Share a workspace and choose what each person can do.':'Keep personal projects separate or collaborate in a shared team workspace.'} busy={busy} onClose={onClose}>
    <div className="team-panel">
      {error&&<div role="alert"><p>{error}</p><Button variant="secondary" disabled={busy} onClick={()=>void perform(async()=>{})}>Reload current access</Button></div>}{loading&&<p role="status">Loading team access…</p>}
      {!teamId&&!workspace.managedTeamId&&<form onSubmit={event=>{event.preventDefault();void perform(async()=>{const value=await teamsApi.create(name.trim());await workspace.refresh();workspace.select(value.team.id);});}}><label>Team name<input autoComplete="organization" value={name} onChange={event=>setName(event.target.value)} maxLength={100} required disabled={busy}/></label><p className="team-muted">A team has its own projects and provider connections. Creating one does not provision a cloud host or charge a payment method.</p><Button type="submit" disabled={busy||!name.trim()}>Create team</Button></form>}
      {!teamId&&!workspace.managedTeamId&&<JoinTeamInvitation onJoined={async team=>{await workspace.refresh();workspace.select(team.id);}}/>}
      {detail&&<>
        <p><strong>{detail.team.name}</strong> · Your role: {detail.membership.role}</p>
        <details className="team-help"><summary>How permissions work</summary><p className="team-muted">Viewers can read shared context. Editors can save and review it. Running agents, using credentials, deploying, and managing access each require a separate grant. Team administrators can access every project in this team.</p></details>
        <TabsSubtle aria-label="Team settings" idPrefix={tabsId} selectedIndex={tab} onSelect={value=>{if(!busy)setTab(value);}}>{["Members", "Project sharing", "Service and data"].map((label,index)=><TabsSubtleItem key={label} index={index} label={label}/>)}</TabsSubtle>
        <TabsSubtlePanel idPrefix={tabsId} selectedIndex={tab} index={0}>
        {admin&&<details className="team-add"><summary>Add an existing member</summary><form onSubmit={event=>{event.preventDefault();void addMember();}}><fieldset disabled={busy}><legend>Add an existing Luci account</legend><label>Username<input value={username} onChange={event=>setUsername(event.target.value)} autoComplete="off" required maxLength={64}/></label><label>Role<select aria-label="Role" value={role} onChange={event=>setRole(event.target.value as TeamMember['role'])}><option value="viewer">Viewer</option><option value="editor">Editor</option>{owner&&<option value="owner">Owner</option>}</select></label><Grants value={grants} onChange={setGrants}/><Button type="submit" disabled={!username.trim()}>Add member</Button></fieldset></form></details>}
        <TeamInvitations team={detail.team} membership={detail.membership} permissions={detail.permissions} onChange={refresh}/>
        <TeamAudit teamId={detail.team.id} actorId={detail.membership.userId} allowed={detail.permissions.admin}/>
        <section aria-label="Team members"><h3>Members <span className="team-muted">{detail.members.length}</span></h3>{detail.members.map(member=>admin?<AccessRow key={`${member.userId}:${member.revision}`} label={member.user.displayName||member.user.username} initial={member} ownerAllowed={owner} busy={busy} onSave={(role,value)=>perform(()=>teamsApi.saveMember(teamId,detail.team.revision,{userId:member.userId,role,grants:value}))} onRemove={()=>perform(()=>teamsApi.removeMember(teamId,detail.team.revision,member.userId))}/>:<p key={member.userId}>{member.user.displayName||member.user.username} · {member.role}</p>)}</section>
        </TabsSubtlePanel>
        <TabsSubtlePanel idPrefix={tabsId} selectedIndex={tab} index={1}>
        {!admin&&<p className="team-muted">A team administrator manages project sharing. Your available projects appear in the workspace.</p>}
        {admin&&projectId&&projectAccess&&<section aria-label="Project sharing"><h3>Share {projectName}</h3><p className="team-muted">Project grants narrow each person’s team permissions. Adding a member does not automatically share projects with them.</p>
          <form onSubmit={event=>{event.preventDefault();void saveGrant({userId:sharing,role:shareRole,grants:shareGrants});}}><fieldset disabled={busy}><legend>Grant project access</legend><label>Team member<select aria-label="Team member" value={sharing} onChange={event=>setSharing(event.target.value)} required><option value="">Choose a member</option>{detail.members.filter(member=>!member.grants.admin).map(member=><option key={member.userId} value={member.userId}>{member.user.displayName||member.user.username}</option>)}</select></label><label>Project role<select aria-label="Project role" value={shareRole} onChange={event=>setShareRole(event.target.value as ProjectGrant['role'])}><option value="viewer">Viewer</option><option value="editor">Editor</option></select></label><Grants value={shareGrants} onChange={setShareGrants} project/><Button type="submit" disabled={!sharing}>Share project</Button></fieldset></form>
          {projectAccess.grants.map(grant=><AccessRow key={`${grant.userId}:${grant.revision}`} label={detail.members.find(member=>member.userId===grant.userId)?.user.displayName||'Team member'} initial={grant} ownerAllowed={false} project busy={busy} onSave={(role,value)=>saveGrant({userId:grant.userId,role:role==='viewer'?'viewer':'editor',grants:value})} onRemove={()=>perform(()=>teamsApi.removeProjectGrant(teamId,projectId,detail.team.revision,grant.userId))}/>)}
        </section>}
        {admin&&!projectId&&<p>Select a project to manage its sharing.</p>}
        </TabsSubtlePanel>
        <TabsSubtlePanel idPrefix={tabsId} selectedIndex={tab} index={2}>
          {billingReturn&&<section aria-label="Billing return guidance"><h3>{billingReturn==='cancel'?'Checkout closed':billingReturn==='portal'?'Back from subscription management':'Back from checkout'}</h3><p>{billingReturn==='cancel'?'You can resume an existing reserved checkout below when you are ready. Closing checkout does not cancel an existing subscription.':billingReturn==='portal'?'Review the last saved status below after changing subscription settings in Stripe.':'Review the last saved subscription status below after checkout.'} This return link does not confirm payment or activate access. A team owner can choose Verify subscription with Stripe to check the current outcome.</p></section>}
          <TeamServiceStatus teamId={detail.team.id}/><TeamDataLifecycle team={detail.team} membership={detail.membership} onChange={refresh}/>
        </TabsSubtlePanel>
      </>}
      <div className="workflow-actions"><Button variant="secondary" disabled={busy} onClick={onClose}>Close</Button></div>
    </div>
  </WorkflowDialog>;
}
