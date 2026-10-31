import { act, cleanup, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { ProjectOutcomes } from './ProjectOutcomes';
import { projectOverviewApi, type ProjectOverview, type ProjectOutcomes as Outcomes } from '../../api/projectOverview';

const environment = vi.hoisted(() => ({ scope: 'alice', canRead: true }));
vi.mock('../../hooks/useProjectDraft', () => ({ useProjectDraftScope: () => environment.scope }));
vi.mock('../../hooks/useProjectAccess', () => ({ useProjectAccess: () => ({ canRead: environment.canRead, loading: false, error: '' }) }));
vi.mock('../teams/TeamWorkspaceProvider', () => ({ useTeamWorkspace: () => null }));
const outcomes = (): Outcomes => ({ status: 'recorded', scope: 'retained-run-history', observedAt: '2026-10-31T00:00:00Z', retainedRuns: 4, recordedAcceptances: 1, unconfirmedAcceptedObservations: 1, reviewRequiredRuns: 2, unresolvedCheckRepairTasks: 3, omittedRuns: 0,
  latestAccepted: { runId: 'run-one', checkId: 'check-one', releaseId: 'release-one', acceptedAt: '2026-10-31T00:00:00Z', commit: 'a'.repeat(40), target: 'local' } });
// This component consumes only the optional projection, not other overview fields.
const response = (value: Outcomes | undefined) => ({ projectId: 'p-one', outcomes: value } as ProjectOverview);
beforeEach(() => { environment.scope = 'alice'; environment.canRead = true; vi.spyOn(projectOverviewApi, 'get').mockResolvedValue(response(outcomes())); });
afterEach(() => { cleanup(); vi.restoreAllMocks(); });

it('reads history only when opened and routes an exact accepted run through its parent', async () => {
  const user = userEvent.setup(), inspect = vi.fn();
  render(<ProjectOutcomes projectId="p-one" projectPath="/one" refresh={0} onInspect={inspect} />);
  expect(projectOverviewApi.get).not.toHaveBeenCalled();
  await user.click(screen.getByText('Recorded outcomes and review work'));
  expect(await screen.findByText(/4 retained runs inspected/)).not.toBeNull();
  expect(screen.getByText(/not confirm current production health/)).not.toBeNull();
  expect(screen.getByText(/accepted records have an unconfirmed/)).not.toBeNull();
  expect(projectOverviewApi.get).toHaveBeenCalledWith('p-one', expect.any(AbortSignal), { includeOutcomes: true });
  await user.click(screen.getByRole('button', { name: 'Inspect accepted run' }));
  expect(inspect).toHaveBeenCalledTimes(1);
  expect(inspect).toHaveBeenCalledWith('run-one');
});

it('keeps unavailable history unknown and reports an older service without inventing zero', async () => {
  const user = userEvent.setup();
  vi.mocked(projectOverviewApi.get).mockResolvedValueOnce(response({ ...outcomes(), status: 'unavailable', retainedRuns: null, recordedAcceptances: null, unconfirmedAcceptedObservations: null, reviewRequiredRuns: null, unresolvedCheckRepairTasks: null, omittedRuns: null, latestAccepted: null })).mockResolvedValueOnce(response(undefined));
  render(<ProjectOutcomes projectId="p-one" projectPath="/one" refresh={0} onInspect={vi.fn()} />);
  await user.click(screen.getByText('Recorded outcomes and review work'));
  expect(await screen.findByText(/counts are unknown/)).not.toBeNull();
  expect(screen.queryByText('Recorded acceptances')).toBeNull();
  await user.click(screen.getByRole('button', { name: 'Refresh outcomes' }));
  expect(await screen.findByText(/does not provide retained outcomes yet/)).not.toBeNull();
});

it('aborts an old scope and never renders its late history after access is removed', async () => {
  const user = userEvent.setup();
  let finish!: (value: ProjectOverview) => void;
  vi.mocked(projectOverviewApi.get).mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  const view = render(<ProjectOutcomes projectId="p-one" projectPath="/one" refresh={0} onInspect={vi.fn()} />);
  await user.click(screen.getByText('Recorded outcomes and review work'));
  await waitFor(() => expect(projectOverviewApi.get).toHaveBeenCalledTimes(1));
  const signal = vi.mocked(projectOverviewApi.get).mock.calls[0][1]!;
  environment.canRead = false;
  view.rerender(<ProjectOutcomes projectId="p-one" projectPath="/one" refresh={0} onInspect={vi.fn()} />);
  expect(signal.aborted).toBe(true);
  await act(async () => finish(response(outcomes())));
  expect(screen.queryByText(/4 retained runs inspected/)).toBeNull();
});
