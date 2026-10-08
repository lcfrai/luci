import {openTaskActions} from '../../test/swarmActions';
import { useState } from 'react';
import { act, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, it, vi } from 'vitest';
import { api } from '../../api';
import { ApiError } from '../../api/client';
import { projectRunsApi, type ProjectRun, type RunOperation } from '../../api/projectRuns';
import { projectSwarmApi, type SwarmReview } from '../../api/projectSwarm';
import { swarmHarnessFixture, swarmRunFixture } from '../../test/swarmFixtures';
import { aSession } from '../../test/fixtures';
import { resetCockpit, useCockpit } from '../../store/store';
import { ProjectSwarm } from './ProjectSwarm';
vi.mock('./SwarmRunQueue', () => ({ SwarmRunQueue: () => null }));

const base = swarmRunFixture;
const op = (run: ProjectRun, id: string, status = 'reserved'): RunOperation => ({id: `op-${id}`, kind: 'session', taskId: id, childId: `session-${id}`, worktreeId: `isolated-${id}`, attempt: 1, status, allocation: 'ready', baseCommit: 'a'.repeat(40), implementation: {...run.input.implementation, worktreeId: `isolated-${id}`, taskIds: [id], taskTitles: [`Build ${id}`]}, requestDigest: 'request', createdAt: run.createdAt, updatedAt: run.updatedAt});
const attach = (run: ProjectRun, operation: RunOperation): ProjectRun => ({...run, revision: run.revision + 1, operations: [...run.operations.filter(item => item.id !== operation.id), operation], swarm: {...run.swarm!, tasks: run.swarm!.tasks.map(item => item.id === operation.taskId ? {...item, operationId: operation.id} : item)}});
const child = (run: ProjectRun, operation: RunOperation) => aSession({id:operation.childId,worktreeId:operation.worktreeId!,projectId:run.projectId,projectRunId:run.id,projectRunOperationId:operation.id,status:'starting'});
function Harness({initial = base(), canEdit = true, canExecute = true, updated = vi.fn()} : {initial?: ProjectRun; canEdit?: boolean; canExecute?: boolean; updated?: (run: ProjectRun) => void}) {
  const [run, setRun] = useState(initial), [busy, setBusy] = useState('');
  return <ProjectSwarm run={run} canEdit={canEdit} canExecute={canExecute} busy={!!busy} onBusy={setBusy} onUpdate={value => {setRun(value); updated(value);}} onOpenSession={vi.fn()} />;
}
beforeEach(() => {vi.restoreAllMocks(); resetCockpit(); useCockpit.getState().setHarnesses([swarmHarnessFixture()]);});

it.each(['ask', 'plan'] as const)('keeps saved %s unchanged and requires a separately reviewed build before preparing autonomous tasks', async permissionMode => {
  const initial = base(); initial.swarm = undefined; initial.input.route.permissionMode = permissionMode;
  const prepare = vi.spyOn(projectSwarmApi, 'prepare'), reserve = vi.spyOn(projectSwarmApi, 'reserve'), create = vi.spyOn(api, 'createSession'), reviewPlan = vi.fn();
  render(<ProjectSwarm run={initial} canEdit canExecute busy={false} onBusy={vi.fn()} onUpdate={vi.fn()} onOpenSession={vi.fn()} onReviewPlan={reviewPlan} />);
  expect(screen.getByRole('button', {name: 'Prepare agent team'})).toHaveProperty('disabled', true);
  expect(screen.getByText(/requires an explicitly saved Bypass permission mode/)).not.toBeNull();
  expect(screen.getByText(/Keep Ask or Plan in an interactive agent/)).not.toBeNull();
  await userEvent.click(screen.getByRole('button', {name: 'Prepare agent team'}));
  await userEvent.click(screen.getByRole('button', {name: 'Review plan and create a new build'}));
  expect(reviewPlan).toHaveBeenCalledOnce(); expect(initial.input.route.permissionMode).toBe(permissionMode);
  expect(prepare).not.toHaveBeenCalled(); expect(reserve).not.toHaveBeenCalled(); expect(create).not.toHaveBeenCalled();
});

it('waits for advertised task capability and refreshes it without preparing or dispatching', async () => {
  const initial = base(); initial.swarm = undefined; useCockpit.getState().setHarnesses([]);
  const list = vi.spyOn(api, 'listHarnesses').mockResolvedValue([swarmHarnessFixture()]), prepare = vi.spyOn(projectSwarmApi, 'prepare'), create = vi.spyOn(api, 'createSession');
  render(<Harness initial={initial} />);
  expect(screen.getByRole('button', {name: 'Prepare agent team'})).toHaveProperty('disabled', true);
  await userEvent.click(screen.getByRole('button', {name: 'Refresh task capability'}));
  await waitFor(() => expect(screen.getByRole('button', {name: 'Prepare agent team'})).toHaveProperty('disabled', false));
  expect(list).toHaveBeenCalledOnce(); expect(prepare).not.toHaveBeenCalled(); expect(create).not.toHaveBeenCalled();
});

it('discards preparation consent when task capabilities or the saved model route change', async () => {
  const initial = base(); initial.swarm = undefined;
  const props = {canEdit: true, canExecute: true, busy: false, onBusy: vi.fn(), onUpdate: vi.fn(), onOpenSession: vi.fn()};
  const prepare = vi.spyOn(projectSwarmApi, 'prepare');
  const view = render(<ProjectSwarm {...props} run={initial} />);
  await userEvent.click(screen.getByRole('button', {name: 'Prepare agent team'}));
  expect(within(screen.getByRole('dialog')).getByText(/Task commands run without interactive approvals inside isolated Docker/)).not.toBeNull();
  act(() => useCockpit.getState().setHarnesses([{...swarmHarnessFixture(), taskPermissionModes: []}]));
  expect(screen.queryByRole('dialog')).toBeNull();
  act(() => useCockpit.getState().setHarnesses([swarmHarnessFixture()]));
  expect(screen.queryByRole('dialog')).toBeNull();
  await userEvent.click(screen.getByRole('button', {name: 'Prepare agent team'}));
  view.rerender(<ProjectSwarm {...props} run={{...initial, input: {...initial.input, route: {...initial.input.route, model: 'another-reviewed-model'}}}} />);
  expect(screen.queryByRole('dialog')).toBeNull(); expect(prepare).not.toHaveBeenCalled();
});

it('does not launch a delayed reservation after a task capability is revoked and restored', async () => {
  const initial = base(), operation = op(initial, 'a'), saved = attach(initial, operation);
  let finish!: (value: {run: ProjectRun; operation: RunOperation}) => void;
  vi.spyOn(projectSwarmApi, 'reserve').mockReturnValue(new Promise(resolve => { finish = resolve; }));
  const create = vi.spyOn(api, 'createSession'), updated = vi.fn();
  render(<Harness initial={initial} updated={updated} />);
  await userEvent.click(screen.getAllByRole('button', {name: 'Start task'})[0]);
  act(() => useCockpit.getState().setHarnesses([{...swarmHarnessFixture(), autonomousTask: false}]));
  act(() => useCockpit.getState().setHarnesses([swarmHarnessFixture()]));
  await act(async () => finish({run: saved, operation}));
  expect(create).not.toHaveBeenCalled(); expect(updated).not.toHaveBeenCalled();
});

it('reconciles each acknowledged dispatch with a fresh revision before starting the next independent task', async () => {
  let saved = base();
  const reserve = vi.spyOn(projectSwarmApi, 'reserve').mockImplementation(async (_project, _run, id, revision) => {
    expect(revision).toBe(saved.revision);
    const operation = op(saved, id); saved = attach(saved, operation); return {run: saved, operation};
  });
  const create = vi.spyOn(api, 'createSession').mockImplementation(async request => {
    const operation = saved.operations.find(item => item.id === request.projectRunOperationId)!;
    saved = attach(saved, {...operation, status: 'dispatching'});
    return child(saved,operation);
  });
  const get=vi.spyOn(projectRunsApi, 'get').mockImplementation(async () => saved);
  const reconcile=vi.spyOn(projectSwarmApi,'control').mockImplementation(async(project,run,action,revision)=>{
    expect([project,run,action,revision]).toEqual([saved.projectId,saved.id,'reconcile',saved.revision]);
    const dispatching=saved.operations.find(item=>item.status==='dispatching')!;
    saved=attach(saved,{...dispatching,status:'running'});return saved;
  });
  render(<Harness />);
  expect(screen.getByText('Waiting for reviewed integration: Build a; Build b.')).not.toBeNull();
  await userEvent.click(screen.getByRole('button', {name: 'Start ready tasks (2)'}));
  await waitFor(() => expect(screen.getAllByText('Recorded as running')).toHaveLength(2));
  expect(create).toHaveBeenCalledTimes(2);expect(get).toHaveBeenCalledTimes(2);expect(reconcile).toHaveBeenCalledTimes(2);
  expect(reserve.mock.calls.map(call=>call[3])).toEqual([1,4]);expect(reconcile.mock.calls.map(call=>call[3])).toEqual([3,6]);
  expect(reserve.mock.calls.map(call => call[2])).toEqual(['a', 'b']);
  expect(create.mock.calls.map(([request]) => [request.worktreeId, request.implementationContext?.taskIds, request.model, request.agentConnectionId])).toEqual([
    ['isolated-a', ['a'], 'recorded-model', 'saved'], ['isolated-b', ['b'], 'recorded-model', 'saved'],
  ]);
  expect(screen.getByText('New starts enabled · 2 of 2 agent slots occupied')).not.toBeNull();
  expect(screen.getByText('Waiting for reviewed integration: Build a; Build b.')).not.toBeNull();
  expect(screen.queryByText('Start outcome unconfirmed')).toBeNull();expect(screen.getAllByRole('button',{name:'Open task session'})).toHaveLength(2);
});

it.each(['get-failure','reconcile-failure','reconcile-conflict'] as const)('retains the acknowledged child after %s without replaying or continuing the batch',async failure=>{
  const initial=base(),operation=op(initial,'a'),reserved=attach(initial,operation),dispatching=attach(reserved,{...operation,status:'dispatching'});
  const reserve=vi.spyOn(projectSwarmApi,'reserve').mockResolvedValue({run:reserved,operation});
  const session=child(reserved,operation),create=vi.spyOn(api,'createSession').mockResolvedValue(session);
  const get=vi.spyOn(projectRunsApi,'get');
  if(failure==='get-failure')get.mockRejectedValue(new Error('Status unavailable.'));else get.mockResolvedValue(dispatching);
  const reconcile=vi.spyOn(projectSwarmApi,'control').mockRejectedValue(failure==='reconcile-conflict'?new ApiError('conflict','Build changed.',409):new Error('Observation unavailable.'));
  render(<Harness initial={initial}/>);await userEvent.click(screen.getByRole('button',{name:'Start ready tasks (2)'}));
  await screen.findByText(/Task started; current status could not be checked/);
  expect(screen.getByText('Started · current status unconfirmed')).not.toBeNull();
  expect(screen.queryByText('Start outcome unconfirmed')).toBeNull();expect(screen.queryByText('Reserved · not started')).toBeNull();
  expect(screen.queryByText(/The start result is unconfirmed/)).toBeNull();
  expect(screen.queryByRole('button',{name:'Continue reserved task'})).toBeNull();
  expect(screen.getByRole('button',{name:'Open task session'})).toHaveProperty('disabled',false);
  expect(useCockpit.getState().sessions[session.id]).toEqual(session);
  expect(reserve).toHaveBeenCalledOnce();expect(create).toHaveBeenCalledOnce();expect(get).toHaveBeenCalledOnce();
  expect(reconcile).toHaveBeenCalledTimes(failure==='get-failure'?0:1);
});

it('preserves authoritative terminal status when the acknowledged worker has already exited',async()=>{
  const initial=base(),operation=op(initial,'a'),reserved=attach(initial,operation),dispatching=attach(reserved,{...operation,status:'dispatching'});
  vi.spyOn(projectSwarmApi,'reserve').mockResolvedValue({run:reserved,operation});vi.spyOn(api,'createSession').mockResolvedValue(child(reserved,operation));
  vi.spyOn(projectRunsApi,'get').mockResolvedValue(dispatching);vi.spyOn(projectSwarmApi,'control').mockResolvedValue(attach(dispatching,{...operation,status:'review-required'}));
  render(<Harness/>);await userEvent.click(screen.getAllByRole('button',{name:'Start task'})[0]);
  await screen.findByText('Awaiting review');expect(screen.queryByText('Recorded as running')).toBeNull();
  expect(screen.queryByText('Start outcome unconfirmed')).toBeNull();expect(screen.getByRole('button',{name:'Review task changes'})).not.toBeNull();
});

it.each(['get','reconcile'] as const)('refuses stale %s observations without replacing a newer run or replaying',async stage=>{
  const initial=base(),operation=op(initial,'a'),reserved=attach(initial,operation),dispatching=attach(reserved,{...operation,status:'dispatching'});
  vi.spyOn(projectSwarmApi,'reserve').mockResolvedValue({run:reserved,operation});const create=vi.spyOn(api,'createSession').mockResolvedValue(child(reserved,operation));
  vi.spyOn(projectRunsApi,'get').mockResolvedValue(stage==='get'?initial:dispatching);
  const reconcile=vi.spyOn(projectSwarmApi,'control').mockResolvedValue(reserved),updated=vi.fn();
  render(<Harness updated={updated}/>);await userEvent.click(screen.getAllByRole('button',{name:'Start task'})[0]);
  await screen.findByText(/Task started; current status could not be checked/);
  expect(updated.mock.calls.map(([run])=>run.revision)).toEqual(stage==='get'?[2]:[2,3]);
  expect(create).toHaveBeenCalledOnce();expect(reconcile).toHaveBeenCalledTimes(stage==='get'?0:1);
});

it.each(['get','reconcile'] as const)('fences a late %s response after execution authority is revoked and restored',async stage=>{
  const initial=base(),operation=op(initial,'a'),reserved=attach(initial,operation),dispatching=attach(reserved,{...operation,status:'dispatching'});
  let finish!:(value:ProjectRun)=>void;const delayed=new Promise<ProjectRun>(resolve=>{finish=resolve;});
  vi.spyOn(projectSwarmApi,'reserve').mockResolvedValue({run:reserved,operation});const create=vi.spyOn(api,'createSession').mockResolvedValue(child(reserved,operation));
  const get=vi.spyOn(projectRunsApi,'get').mockReturnValue(stage==='get'?delayed:Promise.resolve(dispatching));
  const reconcile=vi.spyOn(projectSwarmApi,'control').mockReturnValue(delayed),updated=vi.fn();
  const view=render(<Harness updated={updated}/>);await userEvent.click(screen.getAllByRole('button',{name:'Start task'})[0]);
  await waitFor(()=>expect(stage==='get'?get:reconcile).toHaveBeenCalledOnce());
  const updates=updated.mock.calls.length;view.rerender(<Harness updated={updated} canExecute={false}/>);view.rerender(<Harness updated={updated}/>);
  await act(async()=>finish(attach(dispatching,{...operation,status:'running'})));
  expect(updated).toHaveBeenCalledTimes(updates);expect(reconcile).toHaveBeenCalledTimes(stage==='get'?0:1);expect(create).toHaveBeenCalledOnce();
  expect(screen.queryByText('Recorded as running')).toBeNull();
});

it('does not inspect or upsert a late successful create after switching project and run',async()=>{
  const initial=base(),operation=op(initial,'a'),reserved=attach(initial,operation);
  let finish!:(value:ReturnType<typeof child>)=>void;const create=vi.spyOn(api,'createSession').mockReturnValue(new Promise(resolve=>{finish=resolve;}));
  vi.spyOn(projectSwarmApi,'reserve').mockResolvedValue({run:reserved,operation});const get=vi.spyOn(projectRunsApi,'get'),reconcile=vi.spyOn(projectSwarmApi,'control');
  const props={canEdit:true,canExecute:true,busy:false,onBusy:vi.fn(),onUpdate:vi.fn(),onOpenSession:vi.fn()};
  const view=render(<ProjectSwarm {...props} run={initial}/>);await userEvent.click(screen.getAllByRole('button',{name:'Start task'})[0]);
  await waitFor(()=>expect(create).toHaveBeenCalledOnce());view.rerender(<ProjectSwarm {...props} run={{...initial,id:'other-run',projectId:'other-project'}}/>);
  await act(async()=>finish(child(reserved,operation)));
  expect(get).not.toHaveBeenCalled();expect(reconcile).not.toHaveBeenCalled();expect(useCockpit.getState().sessions[operation.childId]).toBeUndefined();
});

it('does not silently retry a lost start response and keeps the reserved child visible', async () => {
  const initial = base(), operation = op(initial, 'a'), saved = attach(initial, operation);
  const reserve = vi.spyOn(projectSwarmApi, 'reserve').mockResolvedValue({run: saved, operation});
  const create = vi.spyOn(api, 'createSession').mockRejectedValue(new Error('Connection interrupted'));
  const get=vi.spyOn(projectRunsApi,'get'),reconcile=vi.spyOn(projectSwarmApi,'control');
  render(<Harness initial={initial} />);
  await userEvent.click(screen.getAllByRole('button', {name: 'Start task'})[0]);
  await screen.findByRole('alert');
  expect(screen.getByText(/start result is unconfirmed/)).not.toBeNull();
  expect(screen.getByRole('button', {name: 'Continue reserved task'})).toHaveProperty('disabled', true);
  expect(reserve).toHaveBeenCalledTimes(1); expect(create).toHaveBeenCalledTimes(1);
  expect(get).not.toHaveBeenCalled();expect(reconcile).not.toHaveBeenCalled();
});

it('does not acknowledge or reconcile a successful response for a different child',async()=>{
  const initial=base(),operation=op(initial,'a'),saved=attach(initial,operation);
  vi.spyOn(projectSwarmApi,'reserve').mockResolvedValue({run:saved,operation});
  const create=vi.spyOn(api,'createSession').mockResolvedValue({...child(saved,operation),id:'foreign-child'});
  const get=vi.spyOn(projectRunsApi,'get'),reconcile=vi.spyOn(projectSwarmApi,'control');
  render(<Harness/>);await userEvent.click(screen.getAllByRole('button',{name:'Start task'})[0]);
  await screen.findByText(/session did not match its task reservation/);
  expect(screen.getByText(/The start result is unconfirmed/)).not.toBeNull();
  expect(screen.getByRole('button',{name:'Continue reserved task'})).toHaveProperty('disabled',true);
  expect(useCockpit.getState().sessions['foreign-child']).toBeUndefined();
  expect(create).toHaveBeenCalledOnce();expect(get).not.toHaveBeenCalled();expect(reconcile).not.toHaveBeenCalled();
});

it('fences a delayed reservation after execution authority changes, even if later restored', async () => {
  const initial = base(), operation = op(initial, 'a'), saved = attach(initial, operation);
  let finish!: (value: {run: ProjectRun; operation: RunOperation}) => void;
  vi.spyOn(projectSwarmApi, 'reserve').mockReturnValue(new Promise(resolve => {finish = resolve;}));
  const create = vi.spyOn(api, 'createSession'), updated = vi.fn();
  const view = render(<Harness initial={initial} updated={updated} />);
  await userEvent.click(screen.getAllByRole('button', {name: 'Start task'})[0]);
  view.rerender(<Harness initial={initial} updated={updated} canExecute={false} />);
  view.rerender(<Harness initial={initial} updated={updated} canExecute />);
  await act(async () => finish({run: saved, operation}));
  expect(create).not.toHaveBeenCalled(); expect(updated).not.toHaveBeenCalled();
});

it('fences a delayed reservation after leaving the team view', async () => {
  const initial = base(), operation = op(initial, 'a');
  let finish!: (value: {run: ProjectRun; operation: RunOperation}) => void;
  vi.spyOn(projectSwarmApi, 'reserve').mockReturnValue(new Promise(resolve => {finish = resolve;}));
  const create = vi.spyOn(api, 'createSession');
  const view = render(<Harness initial={initial} />);
  await userEvent.click(screen.getAllByRole('button', {name: 'Start task'})[0]); view.unmount();
  await act(async () => finish({run: attach(initial, operation), operation}));
  expect(create).not.toHaveBeenCalled();
});

it('pause stops future starts without claiming existing agents stopped, and resume never launches them', async () => {
  const initial = base();
  const control = vi.spyOn(projectSwarmApi, 'control').mockImplementation(async (_p, _r, action) => ({...initial, revision: action === 'pause' ? 2 : 3, swarm: {...initial.swarm!, state: action === 'pause' ? 'paused' : 'ready'}}));
  const create = vi.spyOn(api, 'createSession');
  render(<Harness initial={initial} />);
  await userEvent.click(screen.getByRole('button', {name: 'Pause new starts'}));
  expect(await screen.findByText('Future task starts stopped. Existing sessions keep their own stop controls.')).not.toBeNull();
  expect(screen.getByRole('button', {name: 'Start ready tasks'})).toHaveProperty('disabled', true);
  await userEvent.click(screen.getByRole('button', {name: 'Resume new starts'}));
  expect(await screen.findByText('New starts enabled. Previously authorized queued tasks can continue when their dependencies and capacity allow.')).not.toBeNull();
  expect(control.mock.calls.map(call => call[2])).toEqual(['pause', 'resume']); expect(create).not.toHaveBeenCalled();
});

it.each(['review-required', 'succeeded'])('requires exact-change review for %s before integrating, and integration alone does not claim verification', async status => {
  const initial = base(), operation = op(initial, 'a', status), saved = attach(initial, operation);
  if (status === 'review-required') operation.message = 'The agent process ended. Its exit code and task correctness are not established by this observation. Review retained changes and verification evidence before integration.';
  const review: SwarmReview = {taskId: 'a', operationId: operation.id, sourceCommit: 'b'.repeat(40), targetCommit: 'a'.repeat(40), reviewDigest: 'exact-digest', paths: ['src/a.ts'], patch: '+export const a = 1;', canIntegrate: true, reason: 'Ready for integration'};
  vi.spyOn(projectSwarmApi, 'review').mockResolvedValue(review);
  const integrate = vi.spyOn(projectSwarmApi, 'integrate').mockResolvedValue({...saved, revision: 3, swarm: {...saved.swarm!, tasks: saved.swarm!.tasks.map(item => item.id === 'a' ? {...item, integration: {state: 'integrated' as const, reviewDigest: review.reviewDigest, commit: 'c'.repeat(40), sourceCommit: review.sourceCommit, targetCommit: review.targetCommit, integratedAt: saved.updatedAt}} : item)}});
  render(<Harness initial={saved} />);
  expect(screen.getByText('Awaiting review')).not.toBeNull();
  if (operation.message) {
    expect(screen.getByText(operation.message)).not.toBeNull();
    expect(screen.queryByText('Review the retained changes before integration, then verify the combined build.')).toBeNull();
  } else expect(screen.getByText('Review the retained changes before integration, then verify the combined build.')).not.toBeNull();
  await userEvent.click(screen.getByRole('button', {name: 'Review task changes'}));
  const dialog = await screen.findByRole('dialog', {name: 'Review task changes'});
  expect(within(dialog).getByRole('button', {name: 'Integrate reviewed changes'})).toHaveProperty('disabled', true);
  expect(within(dialog).getByLabelText('Reviewed task diff').textContent).toContain('export const a = 1');
  await userEvent.click(within(dialog).getByRole('checkbox'));
  await userEvent.click(within(dialog).getByRole('button', {name: 'Integrate reviewed changes'}));
  await waitFor(() => expect(integrate).toHaveBeenCalledWith('p-1', 'run-1', 'a', saved.revision, 'exact-digest'));
  expect(await screen.findByText('Reviewed changes integrated. Dependent tasks can now start; the combined build still needs verification.')).not.toBeNull();
  expect(screen.getByText('Waiting for reviewed integration: Build b.')).not.toBeNull();
});

it('serializes unspecified or overlapping file scopes and blocks readers from execution', async () => {
  const initial = base(), operation = op(initial, 'a', 'running');
  let saved = attach(initial, operation);
  saved = {...saved, swarm: {...saved.swarm!, tasks: saved.swarm!.tasks.map(item => item.id === 'b' ? {...item, writePaths: ['SRC/A.ts']} : item)}};
  render(<Harness initial={saved} canEdit={false} canExecute={false} />);
  expect(screen.getByText('Wait for Build a: intended files overlap or have not been specified.')).not.toBeNull();
  expect(screen.getByRole('button', {name: 'Start ready tasks'})).toHaveProperty('disabled', true);
  screen.getAllByRole('button', {name: 'Start task'}).forEach(button => expect(button).toHaveProperty('disabled', true));
  expect(screen.getByRole('button', {name: 'Pause new starts'})).toHaveProperty('disabled', true);
  expect(screen.getByRole('button', {name: 'Open task session'})).toHaveProperty('disabled', false);
});

it.each(['integrating', 'unknown'] as const)('retains %s integration uncertainty instead of releasing dependencies or claiming success', async state => {
  const initial = base();
  initial.swarm!.tasks[0].integration = {state, reviewDigest: 'retained', sourceCommit: 'b'.repeat(40), targetCommit: 'a'.repeat(40)};
  render(<Harness initial={initial} />);
  expect(screen.getByText('0 of 3 integrated')).not.toBeNull();
  expect(screen.getByText('Integration outcome unconfirmed')).not.toBeNull();
  expect(screen.getByText('Waiting for reviewed integration: Build a; Build b.')).not.toBeNull();
  expect(screen.queryByText('Integrated', {exact: true})).toBeNull();
});

it('offers the saved subscription route only one slot and prepares without starting an agent', async () => {
  const initial = base(); delete initial.swarm;
  initial.input.route.connection = {...initial.input.route.connection, kind: 'codex-subscription', authMode: 'chatgpt'};
  const prepare = vi.spyOn(projectSwarmApi, 'prepare').mockResolvedValue({...initial, swarm: {...base().swarm!, maxParallel: 1}, revision: 2});
  const create = vi.spyOn(api, 'createSession');
  render(<Harness initial={initial} />);
  await userEvent.click(screen.getByRole('button', {name: 'Prepare agent team'}));
  const dialog = await screen.findByRole('dialog', {name: 'Prepare agent team'});
  expect(within(dialog).getByRole('combobox')).toHaveProperty('value', '1');
  expect(within(dialog).getByRole('combobox')).toHaveProperty('disabled', true);
  expect(within(dialog).getByText(/supports one active agent/)).not.toBeNull();
  await userEvent.click(within(dialog).getByRole('button', {name: 'Prepare team'}));
  await waitFor(() => expect(prepare).toHaveBeenCalledWith('p-1', 'run-1', 1, 1));
  expect(create).not.toHaveBeenCalled();
});

it('requires an explicit recovery choice, retains output and does not launch the retry', async () => {
  const initial = base(), operation = {...op(initial, 'a', 'failed'), allocation: 'unknown' as const};
  const saved = attach(initial, operation);
  const abandon = vi.spyOn(projectSwarmApi, 'abandon').mockResolvedValue(attach(saved, {...operation, status: 'cancelled', allocation: 'abandoned'}));
  const create = vi.spyOn(api, 'createSession');
  render(<Harness initial={saved} />);
  await openTaskActions();
  expect(screen.queryByRole('button', {name: 'Open task session'})).toBeNull();
  expect(screen.getByRole('button', {name: 'Inspect workspace output'})).not.toBeNull();
  await userEvent.click(screen.getByRole('button', {name: 'Retain changes and retry'}));
  expect(abandon).not.toHaveBeenCalled();
  const dialog = await screen.findByRole('dialog', {name: 'Retain changes and retry'});
  await userEvent.click(within(dialog).getByRole('button', {name: 'Retain output and allow retry'}));
  await waitFor(() => expect(abandon).toHaveBeenCalledWith('p-1', 'run-1', 'a', saved.revision));
  expect(await screen.findByText('Attempt retired. Output is retained. No new agent was started.')).not.toBeNull();
  expect(create).not.toHaveBeenCalled();
});

it.each(['unknown', 'dispatching', 'cancelled'])('uses exact missing-child recovery for %s without execution permission or launching work', async status => {
  const initial = base(), operation = op(initial, 'a', status);
  if (status === 'cancelled') operation.absenceRetirement = {childId: operation.childId, leaseId: 'lease-a', backend: 'docker', observedAt: initial.updatedAt, inventoryObservedAt: initial.updatedAt};
  const saved = attach(initial, operation);
  const abandon = vi.spyOn(projectSwarmApi, 'abandon').mockResolvedValue(attach(saved, {...operation, status: 'cancelled', ...(operation.absenceRetirement ? {absenceRetirement: {...operation.absenceRetirement, finalizedAt: initial.updatedAt}} : {})}));
  const create = vi.spyOn(api, 'createSession');
  render(<Harness initial={saved} canExecute={false} />);
  await openTaskActions();
  if (status === 'cancelled') {
    expect(screen.getByText('Recovery pending · finish recovery to release capacity')).not.toBeNull();
    expect(screen.queryByRole('button', {name: 'Retry in a new workspace'})).toBeNull();
    expect(screen.getByText('New starts enabled · 1 of 2 agent slots occupied')).not.toBeNull();
  }
  const action = status === 'cancelled' ? 'Finish recovery' : 'Retain changes and retry';
  await userEvent.click(screen.getByRole('button', {name: action}));
  const dialog = screen.getByRole('dialog', {name: action});
  expect(within(dialog).getByText(/prove the child is stopped or was never allocated/)).not.toBeNull();
  await userEvent.click(within(dialog).getByRole('button', {name: status === 'cancelled' ? 'Finish recovery' : 'Retain output and allow retry'}));
  await waitFor(() => expect(abandon).toHaveBeenCalledWith('p-1', 'run-1', 'a', saved.revision, {operationId: operation.id, childId: operation.childId, recoverMissingChild: true}));
  expect(create).not.toHaveBeenCalled();
  expect(screen.getByRole('button', {name: 'Retry in a new workspace'})).toHaveProperty('disabled', true);
});

it('keeps an uncertain attempt available for inspection when absence cannot be proved', async () => {
  const initial = base(), operation = op(initial, 'a', 'unknown'), saved = attach(initial, operation);
  vi.spyOn(projectSwarmApi, 'abandon').mockRejectedValue(new Error('The backend inventory is stale. Capacity remains reserved.'));
  vi.spyOn(projectRunsApi, 'get').mockResolvedValue(saved);
  const create = vi.spyOn(api, 'createSession');
  render(<Harness initial={saved} canExecute={false} />);
  await openTaskActions();
  await userEvent.click(screen.getByRole('button', {name: 'Retain changes and retry'}));
  await userEvent.click(screen.getByRole('button', {name: 'Retain output and allow retry'}));
  await screen.findAllByText('The backend inventory is stale. Capacity remains reserved.');
  expect(screen.queryByRole('button', {name: 'Retry in a new workspace'})).toBeNull();
  await userEvent.click(screen.getByRole('button', {name: 'Keep inspecting'}));
  expect(screen.getByRole('button', {name: 'Inspect workspace output'})).not.toBeNull();
  expect(create).not.toHaveBeenCalled();
});

it('discards recovery consent when a newer run replaces the inspected attempt', async () => {
  const initial = base(), saved = attach(initial, op(initial, 'a', 'unknown'));
  const props = {canEdit: true, canExecute: true, busy: false, onBusy: vi.fn(), onUpdate: vi.fn(), onOpenSession: vi.fn()};
  const abandon = vi.spyOn(projectSwarmApi, 'abandon');
  const view = render(<ProjectSwarm {...props} run={saved} />);
  await openTaskActions();
  await userEvent.click(screen.getByRole('button', {name: 'Retain changes and retry'}));
  expect(screen.getByRole('dialog', {name: 'Retain changes and retry'})).not.toBeNull();
  view.rerender(<ProjectSwarm {...props} run={{...saved, revision: saved.revision + 1}} />);
  await waitFor(() => expect(screen.queryByRole('dialog', {name: 'Retain changes and retry'})).toBeNull());
  expect(abandon).not.toHaveBeenCalled();
});

it('reads back partial retirement once and requires a new explicit finalization action', async () => {
  const initial = base(), operation = op(initial, 'a', 'unknown'), saved = attach(initial, operation);
  const pending = attach(saved, {...operation,status:'cancelled',absenceRetirement:{childId:operation.childId,leaseId:'lease-a',backend:'docker',observedAt:saved.updatedAt,inventoryObservedAt:saved.updatedAt}});
  const abandon = vi.spyOn(projectSwarmApi, 'abandon').mockRejectedValue(new Error('Finalization interrupted'));
  const get = vi.spyOn(projectRunsApi, 'get').mockResolvedValue(pending);
  const create = vi.spyOn(api, 'createSession');
  render(<Harness initial={saved} />);
  await openTaskActions();
  await userEvent.click(screen.getByRole('button', {name:'Retain changes and retry'}));
  await userEvent.click(screen.getByRole('button', {name:'Retain output and allow retry'}));
  await screen.findByText('Finalization interrupted');
  expect(get).toHaveBeenCalledTimes(1);
  expect(abandon).toHaveBeenCalledTimes(1);
  expect(screen.getByRole('button', {name:'Finish recovery'})).toHaveProperty('disabled',false);
  expect(screen.queryByRole('button', {name:'Open task session'})).toBeNull();
  expect(screen.queryByRole('button', {name:'Retry in a new workspace'})).toBeNull();
  expect(screen.getByRole('button', {name:'Inspect workspace output'})).not.toBeNull();
  expect(create).not.toHaveBeenCalled();
});

it('explains unconfirmed recovery when readback also fails and never replays it', async () => {
  const initial = base(), saved = attach(initial, op(initial, 'a', 'unknown'));
  const abandon = vi.spyOn(projectSwarmApi, 'abandon').mockRejectedValue(new Error('Connection interrupted'));
  const get = vi.spyOn(projectRunsApi, 'get').mockRejectedValue(new Error('Offline'));
  render(<Harness initial={saved} />);
  await openTaskActions();
  await userEvent.click(screen.getByRole('button', {name:'Retain changes and retry'}));
  await userEvent.click(screen.getByRole('button', {name:'Retain output and allow retry'}));
  await screen.findAllByText(/recovery result could not be refreshed. Check team status/);
  expect(abandon).toHaveBeenCalledTimes(1);
  expect(get).toHaveBeenCalledTimes(1);
});

it('discards checked integration consent across execution revocation and restoration', async () => {
  const initial = base(), operation = op(initial, 'a', 'succeeded'), saved = attach(initial, operation);
  vi.spyOn(projectSwarmApi, 'review').mockResolvedValue({taskId:'a',operationId:operation.id,sourceCommit:'b'.repeat(40),targetCommit:'a'.repeat(40),reviewDigest:'exact',paths:['src/a.ts'],patch:'+a',canIntegrate:true,reason:'Ready'});
  const integrate = vi.spyOn(projectSwarmApi, 'integrate');
  const props = {run:saved,canEdit:true,busy:false,onBusy:vi.fn(),onUpdate:vi.fn(),onOpenSession:vi.fn()};
  const view = render(<ProjectSwarm {...props} canExecute />);
  await userEvent.click(screen.getByRole('button', {name:'Review task changes'}));
  await userEvent.click(within(screen.getByRole('dialog')).getByRole('checkbox'));
  view.rerender(<ProjectSwarm {...props} canExecute={false} />);
  view.rerender(<ProjectSwarm {...props} canExecute />);
  expect(screen.queryByRole('dialog')).toBeNull();
  await userEvent.click(screen.getByRole('button', {name:'Review task changes'}));
  expect(within(screen.getByRole('dialog')).getByRole('checkbox')).toHaveProperty('checked',false);
  expect(integrate).not.toHaveBeenCalled();
});

it('restores the exact review opener after asynchronous loading temporarily loses native focus', async () => {
  const initial=base(), operation=op(initial,'a','review-required'), saved=attach(initial,operation);
  let finish!: (value:SwarmReview)=>void;
  vi.spyOn(projectSwarmApi,'review').mockReturnValue(new Promise(resolve=>{finish=resolve;}));
  const integrate=vi.spyOn(projectSwarmApi,'integrate');
  render(<Harness initial={saved}/>);
  const opener=screen.getByRole('button',{name:'Review task changes'});
  await userEvent.click(opener);
  expect(opener).toHaveProperty('disabled',true);
  // Chromium drops focus when the asynchronous read disables its trigger.
  document.body.tabIndex=-1; document.body.focus();
  expect(document.activeElement).toBe(document.body);
  document.body.removeAttribute('tabindex');
  await act(async()=>finish({taskId:'a',operationId:operation.id,sourceCommit:'b'.repeat(40),targetCommit:'a'.repeat(40),reviewDigest:'d'.repeat(64),paths:[],patch:'',canIntegrate:true,reason:'Review retained changes'}));
  const dialog=screen.getByRole('dialog',{name:'Review task changes'});
  await userEvent.click(within(dialog).getByRole('button',{name:'Keep reviewing'}));
  await waitFor(()=>expect(document.activeElement).toBe(opener));
  expect(integrate).not.toHaveBeenCalled();
});
