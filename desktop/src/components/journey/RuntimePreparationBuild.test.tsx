import { cleanup, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, expect, it, vi } from 'vitest';
import { RuntimePreparationBuild, eligiblePreparationRuns, eligiblePreparationChecks } from './RuntimePreparationBuild';
import { runtimePrerequisitesApi, type RuntimePreparation } from '../../api/runtimePrerequisites';
import { projectRunsApi, type ProjectRun } from '../../api/projectRuns';
vi.mock('../../hooks/useUnsavedProjectDraft',()=>({useUnsavedProjectDraft:vi.fn()}));
const fixture=():RuntimePreparation=>({plan:{id:'prep-'+'c'.repeat(32),projectId:'p-one',actor:{userId:'alice',projectId:'p-one'},blueprintRevision:2,blueprintDigest:'d'.repeat(64),contextDigest:'e'.repeat(64),registry:{id:'registry-'+'a'.repeat(32),projectId:'p-one',name:'Team registry',repository:'registry.example/team/app',revision:1,fingerprint:'b'.repeat(64)},builds:[{selection:{serviceId:'app',dockerfile:'Dockerfile',memoryMiB:512,cpuMillis:1000},port:8080,artifacts:[]}],monthlyBudgetUsd:50,createdAt:'2026-10-31T01:00:00Z',expiresAt:'2026-11-01T01:00:00Z',digest:'f'.repeat(64)},revision:1,builds:[],suggestedTasks:[]});
const run=()=>({id:'run-one',projectId:'p-one',state:'awaiting-release',updatedAt:'2026-10-31T01:01:00Z',input:{implementation:{taskIds:['task-one']}},swarm:{tasks:[{id:'task-one',integration:{state:'integrated'}}]},checks:[{checkId:'check-one',originalAvailable:true,commit:'a'.repeat(40),createdAt:'2026-10-31T01:02:00Z',commands:[{status:'passed',exitCode:0}]}]} as unknown as ProjectRun);
afterEach(()=>{cleanup();vi.restoreAllMocks();});
it('starts the selected checked work before any cluster selection and forwards the exact preparation',async()=>{
 const row=fixture(),onChange=vi.fn();vi.spyOn(projectRunsApi,'list').mockResolvedValue([run()]);
 const start=vi.spyOn(runtimePrerequisitesApi,'start').mockResolvedValue({...row,revision:2});
 render(<RuntimePreparationBuild projectId="p-one" preparation={row} enabled onChange={onChange}/>);
 const user=userEvent.setup();expect(screen.getByRole('button',{name:'Build reviewed images'})).toBeDisabled();
 await user.click(screen.getByRole('button',{name:'Load completed Develop work'}));
 await user.selectOptions(await screen.findByLabelText('Integrated project run'),'run-one');
 await user.selectOptions(screen.getByLabelText('Passing project checks'),'check-one');
 await user.click(screen.getByRole('button',{name:'Build reviewed images'}));
 await waitFor(()=>expect(start).toHaveBeenCalledWith('p-one',row.plan.id,{expectedRevision:1,preparationDigest:row.plan.digest,runId:'run-one',checkId:'check-one'},expect.any(AbortSignal)));
 expect(onChange).toHaveBeenCalledTimes(1);
});
it('keeps an unknown start from being clicked again and offers an original read',async()=>{
 const row=fixture();vi.spyOn(projectRunsApi,'list').mockResolvedValue([run()]);
 const start=vi.spyOn(runtimePrerequisitesApi,'start').mockRejectedValue(new Error('response lost'));
 render(<RuntimePreparationBuild projectId="p-one" preparation={row} enabled onChange={vi.fn()}/>);
 const user=userEvent.setup();await user.click(screen.getByRole('button',{name:'Load completed Develop work'}));
 await user.selectOptions(await screen.findByLabelText('Integrated project run'),'run-one');await user.selectOptions(screen.getByLabelText('Passing project checks'),'check-one');
 await user.click(screen.getByRole('button',{name:'Build reviewed images'}));await screen.findByRole('alert');
 expect(screen.getByRole('button',{name:'Build reviewed images'})).toBeDisabled();expect(screen.getByText(/start response was not confirmed/)).toBeVisible();
 expect(screen.getByRole('button',{name:'Read preparation'})).toBeEnabled();expect(start).toHaveBeenCalledTimes(1);
});
it('excludes unintegrated tasks, wrong projects and missing or failed check receipts',()=>{
 const row=fixture(),good=run(),wrong={...run(),projectId:'other'};
 const unfinished=run();unfinished.swarm!.tasks[0].integration!.state='integrating';
 expect(eligiblePreparationRuns([good,wrong,unfinished],row).map(r=>r.id)).toEqual(['run-one']);
 const unavailable=run();unavailable.checks[0].originalAvailable=false;
 expect(eligiblePreparationChecks(unavailable)).toEqual([]);
 const failed=run();failed.checks[0].commands[0].exitCode=1;expect(eligiblePreparationChecks(failed)).toEqual([]);
});

it('includes completed review-only work without demanding a nonexistent integration receipt',()=>{
 const row=fixture(),value=run();
 value.swarm!.tasks.push({id:'review-one',title:'Independent review',dependsOn:['task-one'],writePaths:[],acceptance:'Review candidate',reviewOnly:true,operationId:'review-op'});
 value.operations=[{id:'review-op',status:'review-required',handoff:{digest:'a'.repeat(64)}} as unknown as ProjectRun['operations'][number]];
 expect(eligiblePreparationRuns([value],row)).toHaveLength(1);
 value.operations[0].handoff=undefined;expect(eligiblePreparationRuns([value],row)).toEqual([]);
});
