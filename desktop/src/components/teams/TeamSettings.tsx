import { useState } from 'react';
import { useTeamWorkspace } from './TeamWorkspaceProvider';
import { productApi, errorText, type SetupStatus } from '../../api/product';
import { WorkflowDialog } from '../common/WorkflowDialog';
import { Button } from '../fluid/components/button';
import { AgentConnections } from '../setup/AgentConnections';
import { AppearancePanel } from '../setup/AppearancePanel';
export function TeamSettings({status,onStatus,onClose,onSessionStarted}: {status:SetupStatus|null;onStatus(value:SetupStatus):void;onClose():void;onSessionStarted():void}) {
  const workspace=useTeamWorkspace();
  const permissions=workspace?.selected?.permissions;
  const [section,setSection]=useState<'workspace'|'agents'|'appearance'>('workspace');
  const [busy,setBusy]=useState(false),[error,setError]=useState('');
  const canCheck=permissions?.admin&&permissions.edit;
  const canManageAgents=permissions?.admin&&permissions.edit&&permissions.secrets&&permissions.execute;
  return <WorkflowDialog title={`${workspace?.selected?.team.name??'Team'} settings`} description="This workspace shares project context and provider connections with authorized team members." busy={busy} onClose={onClose}>
    <nav className="setup-tabs" aria-label="Team settings sections">{(['workspace','agents','appearance'] as const).map(value=><button key={value} aria-current={section===value?'page':undefined} onClick={()=>setSection(value)}>{value==='workspace'?'Workspace':value==='agents'?'Agent connections':'Appearance'}</button>)}</nav>
    {error&&<p role="alert" className="journey-error">{error}</p>}
    {section==='workspace'&&<section className="team-panel"><p>Projects belong to this team. Use Team access to invite people and share individual projects.</p><div className="setup-checks">{status?.checks.map(check=><div className="setup-check" key={check.id}><strong>{check.label}</strong><span className={`setup-status setup-status-${check.status}`}>{check.status}</span><p>{check.message}</p></div>)}</div><p className="journey-muted">The installation operator prepares Docker and the agent runtime on this host. Personal workspace connections and credentials are separate.</p>{canCheck&&<Button disabled={busy} onClick={()=>{setBusy(true);setError('');void productApi.check().then(onStatus).catch(cause=>setError(errorText(cause))).finally(()=>setBusy(false));}}>Check team workspace readiness</Button>}</section>}
    {section==='agents'&&(canManageAgents?<AgentConnections client={productApi} cloud={false} onSessionStarted={onSessionStarted} initialProvider="openai"/>:<p>Ask a team administrator with execution and credential permissions to connect providers. Using a connected model also requires the relevant project grants.</p>)}
    {section==='appearance'&&<AppearancePanel/>}
    <div className="workflow-actions"><Button variant="secondary" disabled={busy} onClick={onClose}>Back to projects</Button></div>
  </WorkflowDialog>;
}
