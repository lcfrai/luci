// Actual TeamWorkspaceProvider + project permissions + ReviewedContextInspector
// + WorkflowDialog + Radix. HTTP replies controlled; no browser or worker runtime.
import {useState} from 'react';
import {act,cleanup,render,screen,waitFor,within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {afterEach,beforeEach,expect,it,vi} from 'vitest';
import {TeamWorkspaceProvider} from './TeamWorkspaceProvider';
import {teamsApi,personalPermissions,type TeamSummary} from '../../api/teams';
import {api} from '../../api';
import {ApiError,createClient,type ProblemCode,type SessionReviewedContext} from '../../api/client';
import {createProductClient,productApi} from '../../api/product';
import {teamWorkspaceStorageKey} from '../../api/teamWorkspacePreference';
import {selectTeamScope} from '../../api/teamScope';
import {resetCockpit,useCockpit} from '../../store/store';
import {aSession} from '../../test/fixtures';
import {ReviewedContextNote} from '../develop/ReviewedContextNote';
import {AgentConnections} from '../setup/AgentConnections';
import {useProjectAccess} from '../../hooks/useProjectAccess';
import {WorkflowDialog} from '../common/WorkflowDialog';
import {hasOpenOverlay,__resetOverlays} from '../../hooks/useDismissable';
const auth=vi.hoisted(()=>({user:'portal-alice'}));
vi.mock('../auth/AuthProvider',()=>({useAuth:()=>({profile:{user:{id:auth.user}},logout:vi.fn()})}));
vi.mock('../setup/SetupProvider',()=>({useSetup:()=>({status:{config:{projectsDir:'/controlled/projects'},access:{user:{username:'alice'}}},workspace:{mode:'local'}})}));
const teamId='team-abcdefghijklmnopqrstuv';
const summary:TeamSummary={team:{id:teamId,name:'Controlled private team',revision:1,createdAt:'',updatedAt:''},membership:{userId:'portal-alice',role:'editor',grants:personalPermissions,revision:1},permissions:personalPermissions};
const session=aSession({id:'s-portal',projectId:'p-portal',worktreeId:'w-portal',title:'Private retained session title',implementationContext:{projectId:'p-portal',worktreeId:'w-portal',briefRevision:1,planRevision:1,specificationRevision:1,designPlanRevision:1,briefDigest:'brief',planDigest:'plan',specificationDigest:'spec',designPlanDigest:'design',taskIds:['private-task'],taskTitles:['Private retained task name'],requirementIds:[],sourceCommit:'',sourceDigest:'source',sourceDirty:false},reviewedContextDelivery:{status:'captured',version:1,digest:'a'.repeat(64)}});
const captured:SessionReviewedContext={sessionId:session.id,projectId:session.projectId,worktreeId:session.worktreeId,version:1,digest:'a'.repeat(64),implementationContext:session.implementationContext!,document:{kind:'definition',sha256:'b'.repeat(64),bytes:100,value:{purpose:'Private original captured purpose',revision:1}}};
const client=createClient({fetch:async()=>new Response(JSON.stringify({error:'team_access_denied',message:'Workspace membership unavailable'}),{status:403,headers:{'Content-Type':'application/json'}})});
const pending=<T,>()=>{let resolve!:(value:T)=>void;let reject!:(error:Error)=>void;return{promise:new Promise<T>((yes,no)=>{resolve=yes;reject=no;}),get resolve(){return resolve;},get reject(){return reject;}};};
function NestedDraft(){const[value,setValue]=useState('Nested provider draft');return <label>Nested draft<input value={value} onChange={e=>setValue(e.target.value)}/></label>;}
const providerFetch=vi.fn<typeof fetch>(async(url)=>new Response(JSON.stringify(String(url).endsWith('/agent-connections')?{revision:1,connections:[],activeConnectionId:''}:String(url).includes('/codex/')?{configured:false}:{providers:[{id:'local',name:'OpenCode',configured:false,model:''}]})));
const providerClient=createProductClient('http://controlled.invalid',providerFetch);
function Surface({busy=false,initialOpen=false,opener=true,nestedProvider=false}:{busy?:boolean;initialOpen?:boolean;opener?:boolean;nestedProvider?:boolean}){
 const retained=useCockpit(state=>state.sessions[session.id]);
 const access=useProjectAccess(session.projectId),[open,setOpen]=useState(initialOpen),[answer,setAnswer]=useState('Keep unsaved private answer');
 return <><label>Unrelated retained draft<textarea defaultValue="Keep unsaved private idea"/></label>
 {retained&&<ReviewedContextNote session={retained} canInspect={access.canRead} readPending={access.loading}/>}
 {opener&&<button onClick={()=>setOpen(true)}>Open private action review</button>}
 {open&&<WorkflowDialog readAccess={access} title="Private action review" description="Review a private task without executing it." busy={busy} onClose={()=>{if(!busy)setOpen(false);}}>
 <NestedDraft/>{nestedProvider&&<AgentConnections client={providerClient} cloud={false} initialProvider="local"/>}<label>Retained answer<textarea value={answer} onChange={e=>setAnswer(e.target.value)}/></label><button disabled={busy} onClick={()=>setOpen(false)}>Close action review</button>
 </WorkflowDialog>}</>;
}
function PublicDialog(){const[open,setOpen]=useState(false);return <><button onClick={()=>setOpen(true)}>Open public recovery</button>{open&&<WorkflowDialog title="Public recovery" description="Outside the private workspace." onClose={()=>setOpen(false)}><button onClick={()=>setOpen(false)}>Close recovery</button></WorkflowDialog>}</>;}
async function open(){render(<TeamWorkspaceProvider><Surface/></TeamWorkspaceProvider>);await userEvent.click(await screen.findByRole('button',{name:'View reviewed context'}));const dialog=await screen.findByRole('dialog',{name:'Reviewed context'});await within(dialog).findByText('Private original captured purpose');expect(dialog.closest('.team-app-host')).toBeNull();expect(document.body.style.pointerEvents).toBe('none');expect(hasOpenOverlay()).toBe(true);return dialog;}
async function deny(){await act(async()=>{await client.listSessions().catch(()=>{});});}
beforeEach(()=>{providerFetch.mockClear();auth.user='portal-alice';localStorage.clear();resetCockpit();selectTeamScope('');__resetOverlays();localStorage.setItem(teamWorkspaceStorageKey(productApi.baseUrl,auth.user),teamId);localStorage.setItem(teamWorkspaceStorageKey(productApi.baseUrl,auth.user)+'.cockpit',teamId);useCockpit.getState().replaceSessions([session],[session.worktreeId]);vi.spyOn(teamsApi,'list').mockResolvedValue({teams:[summary]});vi.spyOn(teamsApi,'projectAccess').mockResolvedValue({team:summary.team,projectId:session.projectId,permissions:personalPermissions,grants:[]});vi.spyOn(api,'getSessionReviewedContext').mockResolvedValue(captured);vi.spyOn(api,'listProjects').mockResolvedValue([{id:session.projectId,name:'Private project',path:'/controlled/project'}]);vi.spyOn(api,'listSessions').mockResolvedValue([session]);});
afterEach(()=>{cleanup();resetCockpit();selectTeamScope('');__resetOverlays();vi.restoreAllMocks();});
it('CLOUD-060 actual Radix portal is removed on fresh authoritative denial before membership settles',async()=>{
 await open();const held=pending<{teams:TeamSummary[]}>();vi.mocked(teamsApi.list).mockReturnValueOnce(held.promise);await deny();
 expect(document.querySelector('.team-app-host')?.hasAttribute('hidden')).toBe(true);
 expect(screen.queryByRole('dialog',{name:'Reviewed context'})).toBeNull();
 expect((screen.getByText('Private original captured purpose').closest('[role=dialog]') as HTMLElement).style.display).toBe('none');
 await waitFor(()=>expect(document.body.style.pointerEvents).not.toBe('none'));
 expect(document.body.hasAttribute('data-scroll-locked')).toBe(false);expect(hasOpenOverlay()).toBe(false);
 const retry=screen.getByRole('button',{name:'Retry connection'});expect(retry.closest('[aria-hidden="true"]')).toBeNull();
 retry.focus();expect(document.activeElement).toBe(retry);await userEvent.tab();expect(document.activeElement?.closest('[hidden],[inert],[aria-hidden="true"]')).toBeNull();
 expect((document.querySelector('textarea') as HTMLTextAreaElement).value).toBe('Keep unsaved private idea');
 await act(async()=>held.reject(new Error('Controlled membership network interruption')));
 expect(screen.queryByRole('dialog')).toBeNull();
});
it('CLOUD-060 reopens retained review only after both fresh catalogues, without replaying its captured read',async()=>{
 await open();vi.mocked(teamsApi.list).mockRejectedValueOnce(new Error('Controlled network interruption'));await deny();
 const projects=pending<Awaited<ReturnType<typeof api.listProjects>>>(),sessions=pending<Awaited<ReturnType<typeof api.listSessions>>>();
 vi.mocked(api.listProjects).mockReturnValueOnce(projects.promise);vi.mocked(api.listSessions).mockReturnValueOnce(sessions.promise);
 await userEvent.click(screen.getByRole('button',{name:'Retry connection'}));
 await waitFor(()=>expect(api.listProjects).toHaveBeenCalledTimes(1));expect(screen.queryByRole('dialog')).toBeNull();
 await act(async()=>projects.resolve([{id:session.projectId,name:'Private project',path:'/controlled/project'}]));
 expect(screen.queryByRole('dialog')).toBeNull();expect(document.body.style.pointerEvents).not.toBe('none');
 await act(async()=>sessions.resolve([session]));
 await within(await screen.findByRole('dialog',{name:'Reviewed context'})).findByText('Private original captured purpose');
 expect(api.getSessionReviewedContext).toHaveBeenCalledTimes(1);expect(document.body.style.pointerEvents).toBe('none');
 await userEvent.click(screen.getByRole('button',{name:'Close reviewed context'}));await waitFor(()=>expect(screen.queryByRole('dialog')).toBeNull());
 await waitFor(()=>expect(document.activeElement).toBe(screen.getByRole('button',{name:'View reviewed context'})));
});
it('CLOUD-060 restored membership without the former project/session never reveals its cached portal',async()=>{
 await open();vi.mocked(teamsApi.list).mockRejectedValueOnce(new Error('Controlled network interruption'));await deny();
 vi.mocked(api.listProjects).mockResolvedValueOnce([]);vi.mocked(api.listSessions).mockResolvedValueOnce([]);
 await userEvent.click(screen.getByRole('button',{name:'Retry connection'}));await waitFor(()=>expect(document.querySelector('.team-app-host')?.hasAttribute('hidden')).toBe(false));
 expect(screen.queryByRole('dialog')).toBeNull();expect(screen.queryByText('Private original captured purpose')).toBeNull();
 expect(screen.queryByRole('button',{name:'View reviewed context'})).toBeNull();expect(api.getSessionReviewedContext).toHaveBeenCalledTimes(1);
 expect(document.body.style.pointerEvents).not.toBe('none');expect(hasOpenOverlay()).toBe(false);
});
it('CLOUD-060 busy private review releases modal traps without clearing its parent answer draft',async()=>{
 render(<TeamWorkspaceProvider><Surface busy/></TeamWorkspaceProvider>);await userEvent.click(await screen.findByRole('button',{name:'Open private action review'}));await userEvent.type(screen.getByRole('textbox',{name:'Retained answer'}),' amendment');await userEvent.type(screen.getByRole('textbox',{name:'Nested draft'}),' kept');
 vi.mocked(teamsApi.list).mockRejectedValueOnce(new Error('Controlled network interruption'));await deny();expect(screen.queryByRole('dialog')).toBeNull();await waitFor(()=>expect(document.body.style.pointerEvents).not.toBe('none'));
 await userEvent.click(screen.getByRole('button',{name:'Retry connection'}));await waitFor(()=>expect(document.querySelector('.team-app-host')?.hasAttribute('hidden')).toBe(false));
 await screen.findByRole('dialog',{name:'Private action review'});
 expect((screen.getByRole('textbox',{name:'Retained answer'}) as HTMLTextAreaElement).value).toBe('Keep unsaved private answer amendment');
 expect((screen.getByRole('textbox',{name:'Nested draft'}) as HTMLInputElement).value).toBe('Nested provider draft kept');
});
it('CLOUD-060 public recovery dialog outside admitted private scope remains interactive',async()=>{
 render(<><TeamWorkspaceProvider><Surface/></TeamWorkspaceProvider><PublicDialog/></>);await screen.findByRole('button',{name:'View reviewed context'});vi.mocked(teamsApi.list).mockRejectedValueOnce(new Error('Controlled network interruption'));await deny();
 await userEvent.click(screen.getByRole('button',{name:'Open public recovery'}));await screen.findByRole('dialog',{name:'Public recovery'});await userEvent.keyboard('{Escape}');await waitFor(()=>expect(screen.queryByRole('dialog')).toBeNull());
 expect(document.body.style.pointerEvents).not.toBe('none');
});

it('CLOUD-060 restored catalogues wait for the exact dialog project read grant, including a transient failure and retry',async()=>{
 await open();vi.mocked(teamsApi.list).mockRejectedValueOnce(new Error('Controlled membership interruption'));await deny();
 const grant=pending<Awaited<ReturnType<typeof teamsApi.projectAccess>>>();
 vi.mocked(teamsApi.projectAccess).mockReturnValue(grant.promise);
 await userEvent.click(screen.getByRole('button',{name:'Retry connection'}));
 await waitFor(()=>expect(document.querySelector('.team-app-host')?.hasAttribute('hidden')).toBe(false));
 expect(screen.queryByRole('dialog')).toBeNull();expect(screen.getByRole('status').textContent).toContain('Checking access for this review');
 expect(document.body.style.pointerEvents).not.toBe('none');expect(hasOpenOverlay()).toBe(false);
 const requested=vi.mocked(teamsApi.projectAccess).mock.calls;
 expect(requested.every(([team,project])=>team===teamId&&project===session.projectId)).toBe(true);
 await act(async()=>grant.reject(new Error('Controlled granular read interruption')));
 expect(screen.queryByRole('dialog')).toBeNull();expect(screen.getByRole('button',{name:'Close review'})).toBeTruthy();
 vi.mocked(teamsApi.projectAccess).mockResolvedValue({team:summary.team,projectId:session.projectId,permissions:personalPermissions,grants:[]});
 await userEvent.click(screen.getByRole('button',{name:'Retry review access'}));
 await within(await screen.findByRole('dialog',{name:'Reviewed context'})).findByText('Private original captured purpose');
 expect(api.getSessionReviewedContext).toHaveBeenCalledTimes(1);
});
it('CLOUD-060 exact granular read denial cannot reveal a restored project catalogue portal',async()=>{
 await open();vi.mocked(teamsApi.list).mockRejectedValueOnce(new Error('Controlled membership interruption'));await deny();
 const grant=pending<Awaited<ReturnType<typeof teamsApi.projectAccess>>>();vi.mocked(teamsApi.projectAccess).mockReturnValue(grant.promise);
 await userEvent.click(screen.getByRole('button',{name:'Retry connection'}));await waitFor(()=>expect(document.querySelector('.team-app-host')?.hasAttribute('hidden')).toBe(false));
 expect(screen.queryByRole('dialog')).toBeNull();
 await act(async()=>grant.reject(new ApiError('team_access_denied' as ProblemCode,'Project read revoked',403)));
 await waitFor(()=>expect(screen.queryByText('Private original captured purpose')).toBeNull());
 expect(screen.queryByRole('dialog')).toBeNull();expect(api.getSessionReviewedContext).toHaveBeenCalledTimes(1);
 expect(document.body.style.pointerEvents).not.toBe('none');expect(hasOpenOverlay()).toBe(false);
});
it('CLOUD-060 a repeated denial fences an older granular recovery and account replacement removes retained private state',async()=>{
 const tree=render(<TeamWorkspaceProvider><Surface/></TeamWorkspaceProvider>);
 await userEvent.click(await screen.findByRole('button',{name:'View reviewed context'}));await within(await screen.findByRole('dialog')).findByText('Private original captured purpose');
 vi.mocked(teamsApi.list).mockRejectedValueOnce(new Error('Controlled membership interruption'));await deny();
 const grant=pending<Awaited<ReturnType<typeof teamsApi.projectAccess>>>();vi.mocked(teamsApi.projectAccess).mockReturnValue(grant.promise);
 await userEvent.click(screen.getByRole('button',{name:'Retry connection'}));await waitFor(()=>expect(document.querySelector('.team-app-host')?.hasAttribute('hidden')).toBe(false));
 vi.mocked(teamsApi.list).mockRejectedValueOnce(new Error('Controlled second membership refusal'));await deny();
 await act(async()=>grant.resolve({team:summary.team,projectId:session.projectId,permissions:personalPermissions,grants:[]}));
 expect(document.querySelector('.team-app-host')?.hasAttribute('hidden')).toBe(true);expect(screen.queryByRole('dialog')).toBeNull();
 auth.user='portal-bob';tree.rerender(<TeamWorkspaceProvider><Surface/></TeamWorkspaceProvider>);
 await waitFor(()=>expect(screen.queryByRole('button',{name:'Retry connection'})).toBeNull());
 expect(screen.queryByText('Private original captured purpose')).toBeNull();expect(screen.queryByRole('dialog')).toBeNull();
 expect(document.body.style.pointerEvents).not.toBe('none');expect(hasOpenOverlay()).toBe(false);
});
it('CLOUD-060 pending granular authority still permits an ordinary close without retaining a ghost modal',async()=>{
 await open();vi.mocked(teamsApi.list).mockRejectedValueOnce(new Error('Controlled membership interruption'));await deny();
 const grant=pending<Awaited<ReturnType<typeof teamsApi.projectAccess>>>();vi.mocked(teamsApi.projectAccess).mockReturnValue(grant.promise);
 await userEvent.click(screen.getByRole('button',{name:'Retry connection'}));await screen.findByRole('button',{name:'Close review'});
 await userEvent.click(screen.getByRole('button',{name:'Close review'}));expect(screen.queryByRole('region',{name:'Review access'})).toBeNull();
 await act(async()=>grant.resolve({team:summary.team,projectId:session.projectId,permissions:personalPermissions,grants:[]}));
 expect(screen.queryByRole('dialog')).toBeNull();expect(screen.queryByText('Private original captured purpose')).toBeNull();
});

it('CLOUD-060 a programmatically opened dialog survives suspension without inventing an opening control',async()=>{
 render(<TeamWorkspaceProvider><Surface initialOpen/></TeamWorkspaceProvider>);await screen.findByRole('dialog',{name:'Private action review'});
 vi.mocked(teamsApi.list).mockRejectedValueOnce(new Error('Controlled membership interruption'));await deny();
 expect(screen.queryByRole('dialog')).toBeNull();await userEvent.click(screen.getByRole('button',{name:'Retry connection'}));
 await screen.findByRole('dialog',{name:'Private action review'});await userEvent.keyboard('{Escape}');await waitFor(()=>expect(screen.queryByRole('dialog')).toBeNull());
 expect(document.body.style.pointerEvents).not.toBe('none');expect(hasOpenOverlay()).toBe(false);
});
it('CLOUD-060 a removed opening control is never refocused after suspended review closes',async()=>{
 const tree=render(<TeamWorkspaceProvider><Surface/></TeamWorkspaceProvider>);const trigger=await screen.findByRole('button',{name:'Open private action review'});await userEvent.click(trigger);
 tree.rerender(<TeamWorkspaceProvider><Surface opener={false}/></TeamWorkspaceProvider>);expect(trigger.isConnected).toBe(false);
 vi.mocked(teamsApi.list).mockRejectedValueOnce(new Error('Controlled membership interruption'));await deny();
 await userEvent.click(screen.getByRole('button',{name:'Retry connection'}));await screen.findByRole('dialog',{name:'Private action review'});
 await userEvent.keyboard('{Escape}');await waitFor(()=>expect(screen.queryByRole('dialog')).toBeNull());expect(document.activeElement).not.toBe(trigger);
 expect(document.activeElement?.closest('[hidden],[inert],[aria-hidden="true"]')).toBeNull();
});
it('CLOUD-060 actual nested provider form fields survive suspension without saving or connecting',async()=>{
 render(<TeamWorkspaceProvider><Surface nestedProvider/></TeamWorkspaceProvider>);await userEvent.click(await screen.findByRole('button',{name:'Open private action review'}));
 const model=await screen.findByLabelText('Installed model ID');await userEvent.type(model,'fixture-local-coder');
 const endpoint=screen.getByLabelText('Model endpoint URL');await userEvent.clear(endpoint);await userEvent.type(endpoint,'http://fixture.invalid/v1');
 vi.mocked(teamsApi.list).mockRejectedValueOnce(new Error('Controlled membership interruption'));await deny();expect(screen.queryByRole('dialog')).toBeNull();
 await userEvent.click(screen.getByRole('button',{name:'Retry connection'}));await screen.findByRole('dialog',{name:'Private action review'});
 expect((screen.getByLabelText('Installed model ID') as HTMLInputElement).value).toBe('fixture-local-coder');
 expect((screen.getByLabelText('Model endpoint URL') as HTMLInputElement).value).toBe('http://fixture.invalid/v1');
 expect(providerFetch.mock.calls.every(([,init])=>!init?.method||init.method==='GET')).toBe(true);
});

it('CLOUD-060 an actual nested provider save settling while suspended reconnects from its owner without replay',async()=>{
 render(<TeamWorkspaceProvider><Surface nestedProvider/></TeamWorkspaceProvider>);await userEvent.click(await screen.findByRole('button',{name:'Open private action review'}));
 await userEvent.type(await screen.findByLabelText('Installed model ID'),'fixture-local-coder');
 const saved=pending<Response>();
 providerFetch.mockImplementationOnce(async()=>saved.promise);
 await userEvent.click(screen.getByRole('button',{name:'Save agent connection'}));
 const postsBefore=providerFetch.mock.calls.filter(([,init])=>init?.method==='POST').length;expect(postsBefore).toBe(1);
 vi.mocked(teamsApi.list).mockRejectedValueOnce(new Error('Controlled membership interruption'));await deny();
 expect(screen.queryByRole('dialog')).toBeNull();
 await act(async()=>saved.resolve(new Response(JSON.stringify({providers:[{id:'local',name:'OpenCode',configured:true,model:'fixture-local-coder'}]}))));
 await userEvent.click(screen.getByRole('button',{name:'Retry connection'}));await screen.findByRole('dialog',{name:'Private action review'});
 await waitFor(()=>expect((screen.getByLabelText('Coding agent') as HTMLSelectElement).disabled).toBe(false));
 expect(providerFetch.mock.calls.filter(([,init])=>init?.method==='POST')).toHaveLength(postsBefore);
 expect((screen.getByLabelText('Installed model ID') as HTMLInputElement).value).toBe('fixture-local-coder');
 expect(providerFetch.mock.calls.slice(-1)[0]?.[1]?.method??'GET').toBe('GET');
});
