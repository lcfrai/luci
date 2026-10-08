import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { projectRunsApi } from '../../api/projectRuns';
import { projectSwarmApi } from '../../api/projectSwarm';
import { swarmQueueApi } from '../../api/swarmQueue';
import { swarmHarnessFixture, swarmQueueFixture, swarmQueueItem, swarmRunFixture } from '../../test/swarmFixtures';
import { resetCockpit, useCockpit } from '../../store/store';
import { ProjectSwarm } from './ProjectSwarm';

afterEach(() => vi.restoreAllMocks());
beforeEach(() => { resetCockpit(); useCockpit.getState().setHarnesses([swarmHarnessFixture()]); });
const run = swarmRunFixture();

it('disables a manual start already in the durable queue', async () => {
  vi.spyOn(swarmQueueApi, 'get').mockResolvedValue(swarmQueueFixture([swarmQueueItem()]));
  const reserve = vi.spyOn(projectSwarmApi, 'reserve');
  render(<ProjectSwarm run={run} canEdit canExecute busy={false} onBusy={vi.fn()} onUpdate={vi.fn()} onOpenSession={vi.fn()} />);
  await screen.findByRole('button', {name: 'Pause queued task'});
  expect(screen.getAllByRole('button', {name: 'Start task'})[0]).toHaveProperty('disabled', true);
  expect(screen.getByRole('button', {name: 'Start ready tasks (1)'})).toHaveProperty('disabled', false);
  expect(reserve).not.toHaveBeenCalled();
});

it('refreshes a background dispatch after an intervening UI operation and retries a failed read', async () => {
  vi.spyOn(swarmQueueApi, 'get').mockResolvedValue(swarmQueueFixture([swarmQueueItem({state: 'running', operationId: 'op-a', childId: 'child-a'})]));
  const updated = vi.fn(), get = vi.spyOn(projectRunsApi, 'get').mockRejectedValueOnce(new Error('temporary offline')).mockResolvedValue({...run,revision:2});
  const view = render(<ProjectSwarm run={run} canEdit canExecute busy onBusy={vi.fn()} onUpdate={updated} onOpenSession={vi.fn()} />);
  await screen.findByText('running');
  expect(get).not.toHaveBeenCalled();
  view.rerender(<ProjectSwarm run={run} canEdit canExecute busy={false} onBusy={vi.fn()} onUpdate={updated} onOpenSession={vi.fn()} />);
  await userEvent.click(screen.getByRole('button',{name:'Refresh queue'}));
  await waitFor(() => expect(get).toHaveBeenCalledTimes(1));
  expect(updated).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole('button',{name:'Refresh queue'}));
  await waitFor(() => expect(updated).toHaveBeenCalledWith({...run,revision:2}));
  expect(get).toHaveBeenCalledTimes(2);
});

it('offers retirement after a cancelled queue leaves an unstarted reserved workspace', async () => {
  const op = {id:'op-a',kind:'session',taskId:'a',childId:'child-a',attempt:1,status:'reserved',allocation:'ready' as const,requestDigest:'digest',createdAt:run.createdAt,updatedAt:run.updatedAt};
  const reserved = {...run, operations:[op], swarm: {...run.swarm!, tasks:run.swarm!.tasks.map(task => task.id === 'a' ? {...task,operationId:op.id} : task)}};
  vi.spyOn(swarmQueueApi, 'get').mockResolvedValue(swarmQueueFixture([swarmQueueItem({state:'cancelled',operationId:op.id,childId:op.childId})]));
  vi.spyOn(projectRunsApi, 'get').mockResolvedValue(reserved);
  const abandon = vi.spyOn(projectSwarmApi, 'abandon').mockResolvedValue({...reserved,revision:2,operations:[{...op,status:'cancelled',allocation:'abandoned'}]});
  render(<ProjectSwarm run={reserved} canEdit canExecute busy={false} onBusy={vi.fn()} onUpdate={vi.fn()} onOpenSession={vi.fn()} />);
  await screen.findByText(/This reservation belongs to the background queue/);
  expect(screen.getByRole('button',{name:'Continue reserved task'})).toHaveProperty('disabled',true);
  await userEvent.click(screen.getByRole('button',{name:'Retain changes and retry'}));
  await userEvent.click(screen.getByRole('button',{name:'Retain output and allow retry'}));
  expect(abandon).toHaveBeenCalledWith('p-1','run-1','a',1);
});
