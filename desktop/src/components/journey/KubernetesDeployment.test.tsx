import { act, cleanup, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { KubernetesDeployment } from './KubernetesDeployment';
import { kubernetesDeploymentApi, type KubernetesStatus } from '../../api/kubernetesDeployment';
import type { SavedDeploymentBlueprint } from '../../api/deploymentBlueprint';
import { projectRunsApi, type ProjectRun } from '../../api/projectRuns';
import { productApi } from '../../api/product';
import { clusterReviewFixture, clusterTargetFixture } from '../../test/kubernetesDeploymentFixtures';

const access=vi.hoisted(()=>({canRead:true,canEdit:true,loading:false,error:'',permissions:{deploy:true,secrets:true}}));
const owner=vi.hoisted(()=>({id:'alice'}));
const draft=vi.hoisted(()=>vi.fn());
vi.mock('../../hooks/useProjectAccess',()=>({useProjectAccess:()=>access}));
vi.mock('../../hooks/useProjectDraft',()=>({useProjectDraftScope:()=>`actor:${owner.id}:workspace`}));
vi.mock('../../hooks/useUnsavedProjectDraft',()=>({useUnsavedProjectDraft:(...args:unknown[])=>draft(...args)}));
vi.mock('../teams/TeamWorkspaceProvider',()=>({useTeamWorkspace:()=>null}));
const reviewed=()=>{const value=clusterReviewFixture(),now=Date.now();return {...value,createdAt:new Date(now).toISOString(),expiresAt:new Date(now+600_000).toISOString()};};
const saved=():SavedDeploymentBlueprint=>{const r=reviewed();return {projectId:'p-one',revision:r.blueprintRevision,digest:r.blueprintDigest,planDigest:r.request.identity.planDigest,blueprint:r.request.blueprint,actor:{id:'alice',displayName:'Alice'},at:r.createdAt};};
const recorded=():ProjectRun=>{
 const r=reviewed(),hash='a'.repeat(64);
 return {id:r.runId,projectId:'p-one',worktreeId:'worktree-one',revision:1,createdAt:r.createdAt,updatedAt:r.createdAt,state:'awaiting-release',reason:'',input:{implementation:{projectId:'p-one',worktreeId:'worktree-one',briefRevision:1,planRevision:1,specificationRevision:1,designPlanRevision:1,taskIds:['task-one'],taskTitles:['Working application'],requirementIds:[],briefDigest:hash,planDigest:hash,specificationDigest:hash,designPlanDigest:hash,sourceCommit:r.request.blueprint.source.commit,sourceDigest:hash,sourceDirty:false},approvals:{specificationApprovalId:'spec-one',specificationRevision:1,specificationDigest:hash,designApprovalId:'design-one',designRevision:1,designDigest:hash,briefDigest:hash,sketchDigest:hash},route:{harness:'codex',model:'test',permissionMode:'bypass',connection:{kind:'legacy',revision:0}},outcome:'publish',requirements:[]},operations:[],checks:[],releases:[],allowedActions:[]};
};
const status=():KubernetesStatus=>({review:{...reviewed(),applied:true},operation:null,state:'unknown',applicationHealthVerified:false,imageProvenance:'user-supplied immutable digest; checked-source-to-image provenance not verified'});
beforeEach(()=>{
 access.permissions.deploy=true;access.permissions.secrets=true;owner.id='alice';draft.mockReset();
 vi.spyOn(kubernetesDeploymentApi,'targets').mockResolvedValue([clusterTargetFixture()]);
 vi.spyOn(kubernetesDeploymentApi,'list').mockResolvedValue([]);
 vi.spyOn(kubernetesDeploymentApi,'review').mockImplementation(async()=>reviewed());
 vi.spyOn(kubernetesDeploymentApi,'action').mockResolvedValue(status());
 vi.spyOn(projectRunsApi,'list').mockResolvedValue([recorded()]);
 vi.spyOn(productApi,'checks').mockResolvedValue({runs:[{id:'check-one',projectId:'p-one',commit:'d'.repeat(40),dirty:false,status:'passed',commands:[],createdAt:'2026-10-31T00:00:00Z',updatedAt:'2026-10-31T00:00:00Z',message:''}],suggested:[]});
});
afterEach(()=>{cleanup();vi.restoreAllMocks();});

it('loads real targets explicitly and requires Deploy independently of editing',async()=>{
 const user=userEvent.setup(),onTarget=vi.fn();
 const view=render(<KubernetesDeployment projectId="p-one" active saved={saved()} draftMatchesSaved onTarget={onTarget}/>);
 expect(kubernetesDeploymentApi.targets).not.toHaveBeenCalled();
 await user.click(screen.getByRole('button',{name:'Read saved targets and deployments'}));
 await user.selectOptions(await screen.findByRole('combobox',{name:'Saved cluster target'}),clusterTargetFixture().id);
 await user.click(screen.getByRole('button',{name:'Use target in blueprint'}));expect(onTarget).toHaveBeenCalledWith(clusterTargetFixture());
 access.permissions.deploy=false;view.rerender(<KubernetesDeployment projectId="p-one" active saved={saved()} draftMatchesSaved onTarget={onTarget}/>);
 expect((screen.getByRole('button',{name:'Read saved targets and deployments'}) as HTMLButtonElement).disabled).toBe(true);
 expect(screen.queryByText(/No retained cluster deployments/)).toBeNull();
});

it('reviews an explicit saved blueprint and turns lost Apply into retained observation without replay',async()=>{
 const user=userEvent.setup();render(<KubernetesDeployment projectId="p-one" active saved={saved()} draftMatchesSaved onTarget={vi.fn()}/>);
 await user.click(screen.getByRole('button',{name:'Read saved targets and deployments'}));
 await user.selectOptions(screen.getByRole('combobox',{name:'Saved cluster target'}),clusterTargetFixture().id);
 await user.click(screen.getByRole('button',{name:'Load checked work'}));
 await user.selectOptions(screen.getByRole('combobox',{name:'Project run'}),recorded().id);
 await user.selectOptions(screen.getByRole('combobox',{name:'Passing check'}),'check-one');
 await user.type(screen.getByRole('spinbutton',{name:'Reviewed monthly budget / authority ceiling (USD)'}),'25');
 await user.click(screen.getByRole('button',{name:'Review cluster deployment'}));
 await screen.findByRole('region',{name:'Reviewed cluster deployment'});
 expect((screen.getByRole('button',{name:'Apply reviewed workloads'}) as HTMLButtonElement).disabled).toBe(true);
 expect(screen.getByText(/Monetary cost is unknown/)).not.toBeNull();
 await user.click(screen.getByRole('checkbox',{name:/I authorize these workload changes/}));
 vi.mocked(kubernetesDeploymentApi.action).mockRejectedValueOnce(new Error('Connection lost after request'));
 await user.click(screen.getByRole('button',{name:'Apply reviewed workloads'}));
 await screen.findByRole('alert');expect(screen.queryByRole('button',{name:'Apply reviewed workloads'})).toBeNull();
 expect(kubernetesDeploymentApi.action).toHaveBeenCalledTimes(1);
 vi.mocked(kubernetesDeploymentApi.list).mockResolvedValue([status()]);
 await user.click(screen.getByRole('button',{name:'Read saved targets and deployments'}));
 await user.click(await screen.findByRole('button',{name:/Observe original/}));
 await waitFor(()=>expect(kubernetesDeploymentApi.action).toHaveBeenCalledTimes(2));
 expect(vi.mocked(kubernetesDeploymentApi.action).mock.calls[1][2]).toBe('observe');
});

it('guards every unsaved connection field and withdraws credentials and late reads on scope or Secrets changes',async()=>{
 const user=userEvent.setup();let finish!:(value:ReturnType<typeof clusterTargetFixture>[])=>void;
 vi.mocked(kubernetesDeploymentApi.targets).mockImplementationOnce(()=>new Promise(resolve=>{finish=resolve;}));
 const view=render(<KubernetesDeployment projectId="p-one" active saved={saved()} draftMatchesSaved onTarget={vi.fn()}/>);
 await user.click(screen.getByText('Set up or rotate a cluster connection'));
 await user.type(screen.getByRole('textbox',{name:'Connection name'}),'Unsaved private setup');
 expect(draft.mock.lastCall?.[1]).toBe(true);
 await user.click(screen.getByRole('button',{name:'Read saved targets and deployments'}));
 access.permissions.secrets=false;view.rerender(<KubernetesDeployment projectId="p-one" active saved={saved()} draftMatchesSaved onTarget={vi.fn()}/>);
 await act(async()=>finish([clusterTargetFixture()]));
 await user.click(screen.getByText('Set up or rotate a cluster connection'));
 expect((screen.getByRole('textbox',{name:'Connection name'}) as HTMLInputElement).value).toBe('');
 expect(screen.queryByRole('option',{name:/Existing private cluster/})).toBeNull();
 expect(kubernetesDeploymentApi.list).not.toHaveBeenCalled();
});
