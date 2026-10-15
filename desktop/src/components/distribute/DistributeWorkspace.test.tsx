import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { DistributeWorkspace } from './DistributeWorkspace';
import { distributeControlsApi, type AssessmentView, type ControlLibraryView } from '../../api/distributeControls';
import { resetCockpit, useCockpit } from '../../store/store';
import { productApi } from '../../api/product';

const scope=vi.hoisted(()=>({teamId:'team-one',read:true,edit:true,admin:true}));
vi.mock('../teams/TeamWorkspaceProvider',()=>({useTeamWorkspace:()=>scope.teamId?{teamId:scope.teamId,selected:{permissions:{admin:scope.admin}}}:null,useTeamProjectPermissions:()=>({permissions:{read:scope.read,edit:scope.edit},loading:false,error:null})}));
const control={id:'SEC-1',requirement:'Review access controls',category:'Security',applicability:'Each release',owner:'Team',evidenceExpected:'Reviewed access test',sheet:'Controls',row:2,unmapped:{},unmappedHeaders:{}};
const library:ControlLibraryView={state:'configured',teamId:'team-one',revision:1,library:{revision:1,digest:'a'.repeat(64),sourceSHA256:'b'.repeat(64),sheet:'Controls',columns:{id:'A'},controls:[control],added:['SEC-1'],changed:[],removed:[],diffDigest:'c'.repeat(64),actorId:'admin',at:'now'}};
const assessment:AssessmentView={state:'unreviewed',projectId:'p-1',contextDigest:'d'.repeat(64),library:library.library,assessment:{version:1,teamId:'team-one',projectId:'p-1',revision:0,libraryRevision:0,libraryDigest:'',contextDigest:'',results:[],proposals:[],actorId:'',at:''}};
beforeEach(()=>{vi.restoreAllMocks();resetCockpit();scope.teamId='team-one';scope.read=true;scope.edit=true;scope.admin=true;useCockpit.setState(state=>({selection:{...state.selection,projectId:'p-1'}}));vi.spyOn(distributeControlsApi,'library').mockResolvedValue(library);vi.spyOn(distributeControlsApi,'assessment').mockResolvedValue(assessment)});
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
