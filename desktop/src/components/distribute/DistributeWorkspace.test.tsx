import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { DistributeWorkspace } from './DistributeWorkspace';
import { distributeControlsApi, type AssessmentView, type ControlLibraryView, type GapProposal } from '../../api/distributeControls';
import { definitionApi } from '../../api/definition';
import { distributeArtifactsApi } from '../../api/distributeArtifacts';
import { resetCockpit, useCockpit } from '../../store/store';
import { productApi } from '../../api/product';
import { discoveryDocumentsApi, type DiscoveryDocument } from '../../api/discoveryDocuments';
import { emptyGTM, gtmWorkspaceApi, type GTMView } from '../../api/gtmWorkspace';


const scope=vi.hoisted(()=>({teamId:'team-one',read:true,edit:true,admin:true,actor:'reviewer',workspace:'local'}));
vi.mock('../teams/TeamWorkspaceProvider',()=>({useTeamWorkspace:()=>scope.teamId?{teamId:scope.teamId,selected:{permissions:{admin:scope.admin}}}:null,useTeamProjectPermissions:()=>({permissions:{read:scope.read,edit:scope.edit},loading:false,error:null})}));
vi.mock('../auth/AuthProvider',()=>({useAuth:()=>({profile:{user:{id:scope.actor}}})}));
vi.mock('../setup/SetupProvider',()=>({useSetup:()=>({workspace:{mode:scope.workspace==='local'?'local':'cloud',connectionId:scope.workspace,host:scope.workspace},status:{config:{projectsDir:'/projects/reviewer'},access:{user:{username:scope.actor}}}})}));
const control={id:'SEC-1',requirement:'Review access controls',category:'Security',applicability:'Each release',owner:'Team',evidenceExpected:'Reviewed access test',sheet:'Controls',row:2,unmapped:{},unmappedHeaders:{}};
const library:ControlLibraryView={state:'configured',teamId:'team-one',revision:1,library:{revision:1,digest:'a'.repeat(64),sourceSHA256:'b'.repeat(64),sheet:'Controls',columns:{id:'A'},controls:[control],added:['SEC-1'],changed:[],removed:[],diffDigest:'c'.repeat(64),actorId:'admin',at:'now'}};
const assessment:AssessmentView={state:'unreviewed',projectId:'p-1',contextDigest:'d'.repeat(64),library:library.library,assessment:{version:1,teamId:'team-one',projectId:'p-1',revision:0,libraryRevision:0,libraryDigest:'',contextDigest:'',results:[],proposals:[],actorId:'',at:''}};
beforeEach(()=>{vi.restoreAllMocks();resetCockpit();scope.teamId='team-one';scope.read=true;scope.edit=true;scope.admin=true;scope.actor='reviewer';scope.workspace='local';useCockpit.setState(state=>({selection:{...state.selection,projectId:'p-1'}}));vi.spyOn(distributeControlsApi,'library').mockResolvedValue(library);vi.spyOn(distributeControlsApi,'assessment').mockResolvedValue(assessment);vi.spyOn(distributeArtifactsApi,'list').mockResolvedValue({projectId:'p-1',artifacts:[]})});
afterEach(cleanup);

beforeEach(() => {
  vi.spyOn(gtmWorkspaceApi, 'get').mockImplementation(async projectId => ({ projectId, source: { briefRevision: 0, briefDigest: 'a'.repeat(64), specificationRevision: 0, specificationDigest: 'b'.repeat(64), designRevision: 0, designDigest: 'c'.repeat(64), planRevision: 0, planDigest: 'd'.repeat(64), checks: 'not-selected', release: 'not-selected' }, sourceDigest: 'e'.repeat(64), freshness: 'none', workspace: null, history: [], requirements: [] } as GTMView));
});

it('mounts the versioned go-to-market editor in Distribute without publishing', async () => {
  render(<DistributeWorkspace />);
  expect(await screen.findByRole('heading', { name: 'Go to market' })).not.toBeNull();
  expect((await screen.findByLabelText('Positioning') as HTMLTextAreaElement).value).toBe(emptyGTM().positioning);
  expect(screen.queryByRole('button', { name: /Publish campaign/ })).toBeNull();
});

it('keeps the personal stage truthful without requesting team state',async()=>{
  scope.teamId='';render(<DistributeWorkspace/>);
  expect(screen.getByText(/personal workspace/)).toBeTruthy();
  expect(distributeControlsApi.library).not.toHaveBeenCalled();
});
it('shows current controls and guards unsaved assessment edits before navigation',async()=>{
  render(<DistributeWorkspace/>);
  await screen.findByText(/Review access controls/);
  await userEvent.selectOptions(screen.getByLabelText('Assessment'),'missing');
  expect(Object.values(useCockpit.getState().unsavedProjectDrafts).some(row=>row.lens==='distribute')).toBe(true);
  useCockpit.getState().setLens('design');
  expect(useCockpit.getState().pendingNavigation).toEqual({kind:'lens',lens:'design'});
  await userEvent.click(screen.getByRole('button',{name:'Discard assessment edits'}));
  await waitFor(()=>expect(Object.values(useCockpit.getState().unsavedProjectDrafts).some(row=>row.lens==='distribute')).toBe(false));
});
it('enforces admin import and project read grants in the mounted view',async()=>{
  scope.admin=false;const view=render(<DistributeWorkspace/>);
  await screen.findByText(/Review access controls/);
  expect(screen.queryByText('Import or update XLSX controls')).toBeNull();
  scope.read=false;view.rerender(<DistributeWorkspace/>);
  expect(screen.queryByText(/Review access controls/)).toBeNull();
});

it('offers the exact saved assessment report without starting another assessment',async()=>{
  vi.mocked(distributeControlsApi.assessment).mockResolvedValue({...assessment,state:'stale',assessment:{...assessment.assessment,revision:1}});
  const download=vi.spyOn(distributeControlsApi,'downloadReport').mockResolvedValue();
  const save=vi.spyOn(distributeControlsApi,'saveAssessment');
  render(<DistributeWorkspace/>);
  await userEvent.click(await screen.findByRole('button',{name:'Download assessment XLSX (marked stale)'}));
  expect(download).toHaveBeenCalledExactlyOnceWith('p-1',expect.any(Function));
  expect(save).not.toHaveBeenCalled();
});

it('renders clean serialized preview and no-op comparison through the client',async()=>{
  const raw=new TextEncoder().encode('xlsx');
  const hash=Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256',raw))).map(byte=>byte.toString(16).padStart(2,'0')).join('');
  vi.spyOn(productApi,'uploadJSON').mockResolvedValue(JSON.parse(JSON.stringify({sourceSHA256:hash,workbook:{sheets:[{name:'Controls',headers:[{column:'A',name:'ID'}],rows:[]}],issues:null},suggestedColumns:{id:'A'}})));
  vi.spyOn(productApi,'request').mockResolvedValue(JSON.parse('{"expectedRevision":1,"sourceSHA256":"abc","sheet":"Controls","columns":{"id":"A"},"added":null,"changed":null,"removed":null,"controlCount":1,"diffDigest":"digest"}'));
  render(<DistributeWorkspace/>);
  await screen.findByText(/Review access controls/);
  await userEvent.click(screen.getByText('Import or update XLSX controls'));
  const file=new File(['xlsx'],'controls.xlsx',{type:'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet'});
  Object.defineProperty(file,'arrayBuffer',{value:async()=>raw.buffer});
  fireEvent.change(screen.getByLabelText(/Workbook \(.xlsx/),{target:{files:[file]}});
  await userEvent.click(screen.getByRole('button',{name:'Preview workbook'}));
  await screen.findByText(/Source SHA-256/);
  await userEvent.click(screen.getByRole('button',{name:'Compare full revision'}));
  await screen.findByText(/1 controls · 0 added · 0 changed · 0 removed/);
});

function evidenceDocument(letter:string,name:string):DiscoveryDocument{
  const actor={id:'reviewer',displayName:'Reviewer'};
  return {id:`doc-${letter.repeat(64)}`,projectId:'p-1',revision:1,original:{digest:letter.repeat(64),bytes:1,name,mimeType:'application/pdf'},createdAt:'now',createdBy:actor,attempts:[],extractions:[{revision:1,attemptId:'extract-one',originalDigest:letter.repeat(64),extractor:'retained',extractorVersion:'1',status:'ready',digest:letter.repeat(64),issues:[],at:'now',actor,pages:[{page:1,width:100,height:100,segments:[{id:'segment-one',kind:'text',text:`Evidence from ${name}`,textDigest:letter.repeat(64),confidence:100,uncertainty:'',location:{x:0,y:0,width:100,height:100,firstLine:1,lastLine:1}}]}]}]};
}
const evidenceA=evidenceDocument('a','First.pdf'),evidenceB=evidenceDocument('b','Current.pdf');
function deferred<T>(){let resolve!:(value:T)=>void;let reject!:(error:Error)=>void;const promise=new Promise<T>((yes,no)=>{resolve=yes;reject=no});return {promise,resolve,reject}}
async function openEvidencePicker(){
  vi.spyOn(discoveryDocumentsApi,'list').mockResolvedValue({documents:[evidenceA,evidenceB],nextCursor:'',omitted:false});
  const view=render(<DistributeWorkspace/>);
  await userEvent.click(await screen.findByRole('button',{name:'Link document evidence'}));
  await screen.findByRole('option',{name:'First.pdf'});
  return view;
}

it.each(['response','error'])('links only the current evidence when a previous document %s settles late',async(outcome)=>{
  const first=deferred<DiscoveryDocument>(),second=deferred<DiscoveryDocument>();
  const get=vi.spyOn(discoveryDocumentsApi,'get').mockImplementation((_project,id)=>id===evidenceA.id?first.promise:second.promise);
  const save=vi.spyOn(distributeControlsApi,'saveAssessment').mockResolvedValue({...assessment.assessment,revision:1});
  await openEvidencePicker();
  await userEvent.selectOptions(screen.getByLabelText('Private document'),evidenceA.id);
  await userEvent.selectOptions(screen.getByLabelText('Private document'),evidenceB.id);
  expect((get.mock.calls[0][2] as AbortSignal).aborted).toBe(true);
  await act(async()=>{second.resolve(evidenceB)});
  await act(async()=>{if(outcome==='response')first.resolve(evidenceA);else first.reject(new Error('Earlier private document failed'))});
  expect(screen.getByLabelText('Private document')).toHaveProperty('value',evidenceB.id);
  expect(screen.queryByRole('option',{name:/Evidence from First.pdf/})).toBeNull();
  expect(screen.queryByText('Earlier private document failed')).toBeNull();
  await userEvent.selectOptions(screen.getByLabelText('Extracted source segment'),'1:1:segment-one');
  await userEvent.click(screen.getByRole('button',{name:'Link exact source'}));
  await userEvent.click(screen.getByRole('button',{name:'Save reviewed assessment'}));
  expect(save).toHaveBeenCalledWith('p-1',expect.objectContaining({results:[expect.objectContaining({controlId:'SEC-1',evidenceRefs:[expect.objectContaining({documentId:evidenceB.id,originalDigest:evidenceB.original.digest,extractionRevision:1,page:1,segmentId:'segment-one'})]})]}));
});

it('keeps a cleared evidence selection empty after its previous read returns',async()=>{
  const pending=deferred<DiscoveryDocument>();
  vi.spyOn(discoveryDocumentsApi,'get').mockReturnValue(pending.promise);
  await openEvidencePicker();
  await userEvent.selectOptions(screen.getByLabelText('Private document'),evidenceA.id);
  await userEvent.selectOptions(screen.getByLabelText('Private document'),'');
  await act(async()=>{pending.resolve(evidenceA)});
  expect(screen.getByLabelText('Private document')).toHaveProperty('value','');
  expect(screen.queryByRole('option',{name:/Evidence from First.pdf/})).toBeNull();
  expect(screen.getByRole('button',{name:'Link exact source'}).hasAttribute('disabled')).toBe(true);
});

it('does not restore old evidence after read access is revoked and regranted',async()=>{
  const pending=deferred<DiscoveryDocument>();
  const get=vi.spyOn(discoveryDocumentsApi,'get').mockReturnValue(pending.promise);
  const view=await openEvidencePicker();
  await userEvent.selectOptions(screen.getByLabelText('Private document'),evidenceA.id);
  scope.read=false;view.rerender(<DistributeWorkspace/>);
  expect((get.mock.calls[0][2] as AbortSignal).aborted).toBe(true);
  scope.read=true;view.rerender(<DistributeWorkspace/>);
  await screen.findByLabelText('Private document');
  await act(async()=>{pending.resolve(evidenceA)});
  expect(screen.getByLabelText('Private document')).toHaveProperty('value','');
  expect(screen.getByRole('button',{name:'Link exact source'}).hasAttribute('disabled')).toBe(true);
});

it.each(['document','project'])('refuses evidence readback with a different %s identity',async(identity)=>{
  vi.spyOn(discoveryDocumentsApi,'get').mockResolvedValue(identity==='document'?evidenceB:{...evidenceA,projectId:'p-other'});
  await openEvidencePicker();
  await userEvent.selectOptions(screen.getByLabelText('Private document'),evidenceA.id);
  expect(await screen.findByText('The selected document could not be verified. Choose it again.')).toBeTruthy();
  expect(screen.getByRole('button',{name:'Link exact source'}).hasAttribute('disabled')).toBe(true);
});

it('shows the current evidence read failure and permits selecting another document',async()=>{
  vi.spyOn(discoveryDocumentsApi,'get').mockRejectedValueOnce(new Error('Current document is unavailable')).mockResolvedValueOnce(evidenceB);
  await openEvidencePicker();
  await userEvent.selectOptions(screen.getByLabelText('Private document'),evidenceA.id);
  await screen.findByText('Current document is unavailable');
  await userEvent.selectOptions(screen.getByLabelText('Private document'),evidenceB.id);
  await screen.findByRole('option',{name:/Evidence from Current.pdf/});
  expect(screen.queryByText('Current document is unavailable')).toBeNull();
});

it('keeps assessment edits entered while a refresh is pending and saves only on request against the prior revision',async()=>{
  const pending=deferred<AssessmentView>();
  const save=vi.spyOn(distributeControlsApi,'saveAssessment').mockResolvedValue({...assessment.assessment,revision:1});
  render(<DistributeWorkspace/>);
  await screen.findByLabelText('Reviewer note');
  vi.mocked(distributeControlsApi.assessment).mockReturnValueOnce(pending.promise);
  await userEvent.click(screen.getByRole('button',{name:'Refresh evidence'}));
  expect(screen.getByRole('button',{name:'Refresh evidence'})).toHaveProperty('disabled',true);
  await userEvent.selectOptions(screen.getByLabelText('Assessment'),'missing');
  await userEvent.type(screen.getByLabelText('Reviewer note'),'Keep this unsaved review');
  await act(async()=>pending.resolve({...assessment,state:'current',assessment:{...assessment.assessment,revision:4,results:[{controlId:'SEC-1',status:'supported',note:'Other saved review',evidenceRefs:[]}]}}));
  expect(screen.getByLabelText('Assessment')).toHaveProperty('value','missing');
  expect(screen.getByLabelText('Reviewer note')).toHaveProperty('value','Keep this unsaved review');
  expect(screen.getByText(/Your assessment edits were kept/)).toBeTruthy();
  expect(Object.values(useCockpit.getState().unsavedProjectDrafts).some(row=>row.lens==='distribute')).toBe(true);
  expect(save).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole('button',{name:'Save reviewed assessment'}));
  expect(save).toHaveBeenCalledExactlyOnceWith('p-1',expect.objectContaining({expectedRevision:0,results:[expect.objectContaining({status:'missing',note:'Keep this unsaved review'})]}));
});

it('does not replace an active assessment draft with a late refresh error',async()=>{
  const pending=deferred<AssessmentView>();
  render(<DistributeWorkspace/>);
  await screen.findByLabelText('Reviewer note');
  vi.mocked(distributeControlsApi.assessment).mockReturnValueOnce(pending.promise);
  await userEvent.click(screen.getByRole('button',{name:'Refresh evidence'}));
  await userEvent.type(screen.getByLabelText('Reviewer note'),'Still editing this review');
  await act(async()=>pending.reject(new Error('Earlier refresh failed')));
  expect(screen.queryByText('Earlier refresh failed')).toBeNull();
  expect(screen.getByLabelText('Reviewer note')).toHaveProperty('value','Still editing this review');
  expect(screen.getByRole('button',{name:'Discard assessment edits'})).toBeTruthy();
});

it('drops a refresh from before read permission was revoked and regranted',async()=>{
  const pending=deferred<AssessmentView>();
  const view=render(<DistributeWorkspace/>);
  await screen.findByLabelText('Reviewer note');
  vi.mocked(distributeControlsApi.assessment).mockReturnValueOnce(pending.promise);
  await userEvent.click(screen.getByRole('button',{name:'Refresh evidence'}));
  scope.read=false;view.rerender(<DistributeWorkspace/>);
  expect(screen.queryByLabelText('Reviewer note')).toBeNull();
  vi.mocked(distributeControlsApi.assessment).mockResolvedValue({...assessment,state:'current',assessment:{...assessment.assessment,revision:2,results:[{controlId:'SEC-1',status:'partial',note:'Current permission read',evidenceRefs:[]}]}});
  scope.read=true;view.rerender(<DistributeWorkspace/>);
  await waitFor(()=>expect(screen.getByLabelText('Reviewer note')).toHaveProperty('value','Current permission read'));
  await act(async()=>pending.resolve({...assessment,assessment:{...assessment.assessment,revision:1,results:[{controlId:'SEC-1',status:'missing',note:'Obsolete permission read',evidenceRefs:[]}]}}));
  expect(screen.getByLabelText('Reviewer note')).toHaveProperty('value','Current permission read');
  expect(screen.getByLabelText('Assessment')).toHaveProperty('value','partial');
});

it('keeps a confirmed save busy until its exact assessment readback settles',async()=>{
  const pending=deferred<AssessmentView>();
  const saved={...assessment.assessment,revision:1,results:[{controlId:'SEC-1',status:'partial' as const,note:'Saved review',evidenceRefs:[]}]};
  vi.spyOn(distributeControlsApi,'saveAssessment').mockResolvedValue(saved);
  render(<DistributeWorkspace/>);
  await screen.findByLabelText('Reviewer note');
  await userEvent.selectOptions(screen.getByLabelText('Assessment'),'partial');
  await userEvent.type(screen.getByLabelText('Reviewer note'),'Saved review');
  vi.mocked(distributeControlsApi.assessment).mockReturnValueOnce(pending.promise);
  await userEvent.click(screen.getByRole('button',{name:'Save reviewed assessment'}));
  await waitFor(()=>expect(distributeControlsApi.saveAssessment).toHaveBeenCalledOnce());
  expect(screen.getByLabelText('Reviewer note')).toHaveProperty('disabled',true);
  await act(async()=>pending.resolve({...assessment,state:'current',assessment:saved}));
  await waitFor(()=>expect(screen.getByLabelText('Reviewer note')).toHaveProperty('disabled',false));
  expect(screen.getByLabelText('Reviewer note')).toHaveProperty('value','Saved review');
  expect(distributeControlsApi.saveAssessment).toHaveBeenCalledOnce();
});

it.each(['account','workspace'])('never applies a previous %s assessment refresh to the current selection',async(kind)=>{
  const pending=deferred<AssessmentView>();
  const view=render(<DistributeWorkspace/>);
  await screen.findByLabelText('Reviewer note');
  vi.mocked(distributeControlsApi.assessment).mockReturnValueOnce(pending.promise);
  await userEvent.click(screen.getByRole('button',{name:'Refresh evidence'}));
  vi.mocked(distributeControlsApi.assessment).mockResolvedValue({...assessment,state:'current',assessment:{...assessment.assessment,revision:2,results:[{controlId:'SEC-1',status:'partial',note:'Current scoped assessment',evidenceRefs:[]}]}});
  if(kind==='account')scope.actor='new-reviewer';else scope.workspace='another-connection';
  view.rerender(<DistributeWorkspace/>);
  await waitFor(()=>expect(screen.getByLabelText('Reviewer note')).toHaveProperty('value','Current scoped assessment'));
  await act(async()=>pending.resolve({...assessment,assessment:{...assessment.assessment,revision:1,results:[{controlId:'SEC-1',status:'supported',note:'Previous private assessment',evidenceRefs:[]}]}}));
  expect(screen.getByLabelText('Reviewer note')).toHaveProperty('value','Current scoped assessment');
  expect(screen.queryByText('Previous private assessment')).toBeNull();
});

it('does not redirect navigation after leaving an applied gap pending readback',async()=>{
  const proposal:GapProposal={id:'gap-one',controlId:'SEC-1',assessmentRevision:1,target:'discover',summary:'Review the access evidence gap.',state:'pending-review',actorId:'reviewer',at:'now'};
  const current:AssessmentView={...assessment,state:'current',assessment:{...assessment.assessment,revision:1,proposals:[proposal]}};
  vi.mocked(distributeControlsApi.assessment).mockResolvedValue(current);
  vi.spyOn(definitionApi,'get').mockResolvedValue({projectId:'p-1',version:1,revision:2,briefRevision:1,startingPoint:'new',purpose:'Current idea',audience:'People',constraints:'',outOfScope:[],requirements:[],journeys:[],roles:[],entities:[],integrations:[],glossary:[],decisions:[],questions:[],status:'draft',digest:'',updatedAt:''});
  const apply=vi.spyOn(distributeControlsApi,'applyGap').mockResolvedValue({...proposal,state:'applied-draft',appliedRevision:3});
  const pending=deferred<AssessmentView>();
  const view=render(<DistributeWorkspace/>);
  await userEvent.click(await screen.findByText('Recorded gap proposals',{selector:'summary'}));
  vi.mocked(distributeControlsApi.assessment).mockReturnValueOnce(pending.promise);
  await userEvent.click(screen.getByRole('button',{name:'Apply to discover draft'}));
  await waitFor(()=>expect(distributeControlsApi.assessment).toHaveBeenCalledTimes(2));
  expect(apply).toHaveBeenCalledExactlyOnceWith('p-1',proposal,{expectedAssessmentRevision:1,expectedContextDigest:assessment.contextDigest,expectedTargetRevision:2});
  act(()=>useCockpit.getState().setLens('design'));
  view.unmount();
  await act(async()=>pending.resolve({...current,assessment:{...current.assessment,proposals:[{...proposal,state:'applied-draft',appliedRevision:3}]}}));
  expect(useCockpit.getState().lens).toBe('design');
  expect(useCockpit.getState().pendingNavigation).toBeNull();
  expect(apply).toHaveBeenCalledOnce();
});
