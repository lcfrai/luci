import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { DistributeWorkspace } from './DistributeWorkspace';
import { distributeControlsApi, type AssessmentView, type ControlLibraryView } from '../../api/distributeControls';
import { distributeArtifactsApi } from '../../api/distributeArtifacts';
import { resetCockpit, useCockpit } from '../../store/store';
import { productApi } from '../../api/product';
import { discoveryDocumentsApi, type DiscoveryDocument } from '../../api/discoveryDocuments';

const scope=vi.hoisted(()=>({teamId:'team-one',read:true,edit:true,admin:true}));
vi.mock('../teams/TeamWorkspaceProvider',()=>({useTeamWorkspace:()=>scope.teamId?{teamId:scope.teamId,selected:{permissions:{admin:scope.admin}}}:null,useTeamProjectPermissions:()=>({permissions:{read:scope.read,edit:scope.edit},loading:false,error:null})}));
const control={id:'SEC-1',requirement:'Review access controls',category:'Security',applicability:'Each release',owner:'Team',evidenceExpected:'Reviewed access test',sheet:'Controls',row:2,unmapped:{},unmappedHeaders:{}};
const library:ControlLibraryView={state:'configured',teamId:'team-one',revision:1,library:{revision:1,digest:'a'.repeat(64),sourceSHA256:'b'.repeat(64),sheet:'Controls',columns:{id:'A'},controls:[control],added:['SEC-1'],changed:[],removed:[],diffDigest:'c'.repeat(64),actorId:'admin',at:'now'}};
const assessment:AssessmentView={state:'unreviewed',projectId:'p-1',contextDigest:'d'.repeat(64),library:library.library,assessment:{version:1,teamId:'team-one',projectId:'p-1',revision:0,libraryRevision:0,libraryDigest:'',contextDigest:'',results:[],proposals:[],actorId:'',at:''}};
beforeEach(()=>{vi.restoreAllMocks();resetCockpit();scope.teamId='team-one';scope.read=true;scope.edit=true;scope.admin=true;useCockpit.setState(state=>({selection:{...state.selection,projectId:'p-1'}}));vi.spyOn(distributeControlsApi,'library').mockResolvedValue(library);vi.spyOn(distributeControlsApi,'assessment').mockResolvedValue(assessment);vi.spyOn(distributeArtifactsApi,'list').mockResolvedValue({projectId:'p-1',artifacts:[]})});
afterEach(cleanup);

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
