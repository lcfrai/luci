import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { projectSwarmApi } from '../../api/projectSwarm';
import { swarmReviewObligationsApi } from '../../api/swarmReviewObligations';
import type { RunOperation } from '../../api/projectRuns';
import { swarmRunFixture } from '../../test/swarmFixtures';
import { swarmSupervisionFixture } from '../../test/swarmSupervisionFixtures';
import { SwarmReviewPanel } from './SwarmReviewPanel';

beforeEach(() => localStorage.clear());
afterEach(() => vi.restoreAllMocks());

it('requires an approved reviewer role and an inspected exact candidate before recording an assignment', async () => {
  const run = swarmRunFixture();
  run.swarm!.tasks[0] = {...run.swarm!.tasks[0], id: 'task-a', operationId: 'op-a'};
  run.swarm!.tasks[1] = {...run.swarm!.tasks[1], id: 'task-b', role: 'reviewer', reviewOfTaskId: 'task-a', writePaths: [], operationId: 'op-b'};
  run.operations = [
    {id: 'op-a', taskId: 'task-a', status: 'complete', allocation: 'released'},
    {id: 'op-b', taskId: 'task-b', status: 'reserved', allocation: 'ready'},
  ] as RunOperation[];
  const view = swarmSupervisionFixture({context: {briefRevision: 1, planRevision: 1, contractDigest: 'a'.repeat(64), freshness: 'current'}});
  const inspect = vi.spyOn(projectSwarmApi, 'review').mockResolvedValue({taskId: 'task-a', operationId: 'op-a', sourceCommit: 'a'.repeat(40), targetCommit: 'b'.repeat(40), reviewDigest: 'c'.repeat(64), paths: ['src/a.ts'], patch: '+change', canIntegrate: false, reason: 'Independent review pending'});
  const create = vi.spyOn(swarmReviewObligationsApi, 'create').mockResolvedValue({runRevision: 2, obligation: {} as never});
  const onWrite = vi.fn(async (action: () => Promise<void>) => action());
  render(<SwarmReviewPanel run={run} view={view} messages={[]} canRead canEdit paused={false} recoveryScope="actor/workspace/run"
    onWrite={onWrite} onRefresh={vi.fn(async () => {})} onOpenTask={vi.fn()} />);
  expect(inspect).not.toHaveBeenCalled(); expect(create).not.toHaveBeenCalled();
  fireEvent.click(screen.getByText('Assign an independent reviewer'));
  fireEvent.change(screen.getByLabelText('Candidate coding task'), {target: {value: 'task-a'}});
  fireEvent.change(screen.getByLabelText('Reserved reviewer attempt'), {target: {value: 'op-b'}});
  expect(screen.getByRole('button', {name: 'Save exact review assignment'}).hasAttribute('disabled')).toBe(true);
  fireEvent.click(screen.getByRole('button', {name: 'Inspect exact candidate diff'}));
  await screen.findByText('+change');
  fireEvent.click(screen.getByRole('button', {name: 'Save exact review assignment'}));
  await waitFor(() => expect(create).toHaveBeenCalledOnce());
  expect(create.mock.calls[0][3]).toMatchObject({candidateOperationId: 'op-a', reviewerOperationId: 'op-b', reviewDigest: 'c'.repeat(64)});
  expect(onWrite).toHaveBeenCalledOnce();
});
