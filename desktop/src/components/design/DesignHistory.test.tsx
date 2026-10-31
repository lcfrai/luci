import { act, cleanup, render, screen } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { designPlanApi } from '../../api/designPlan';
import { productApi } from '../../api/product';
import { designScenePlanFixture } from '../../test/designSceneFixtures';
import { DesignHistory } from './DesignHistory';

afterEach(() => { cleanup(); vi.restoreAllMocks(); });
const snapshot = (revision = 2, projectId = 'p-1') => ({ ...designScenePlanFixture(), projectId, revision, updatedAt: '2026-10-31T00:00:00Z' });

it('confirms project identity and snapshot shape before accepting saved history', async () => {
  const request = vi.spyOn(productApi, 'request');
  request.mockResolvedValueOnce({ revisions: [snapshot(2, 'other-project')] });
  await expect(designPlanApi.history('p-1')).rejects.toThrow(/could not be confirmed/);
  request.mockResolvedValueOnce({ revisions: [snapshot(), snapshot()] });
  await expect(designPlanApi.history('p-1')).rejects.toThrow(/could not be confirmed/);
  request.mockResolvedValueOnce({ revisions: [{ ...snapshot(), scene: { version: 2, frames: [] } }] });
  await expect(designPlanApi.history('p-1')).rejects.toThrow(/could not be confirmed/);
  request.mockResolvedValueOnce({ revisions: [snapshot()], archiveHead: 'a'.repeat(64) });
  const controller = new AbortController();
  expect((await designPlanApi.history('p-1', controller.signal)).revisions[0].revision).toBe(2);
  expect(request.mock.lastCall).toEqual(['/v1/product/projects/p-1/design-plan/history', undefined, 'GET', 30_000, controller.signal]);
});

it('shows historical status and the editor baseline without restoring, approving or saving a design', async () => {
  vi.spyOn(designPlanApi, 'history').mockResolvedValue({ revisions: [snapshot(1), { ...snapshot(2), status: 'approved' }], archiveHead: 'a'.repeat(64) });
  const save = vi.spyOn(designPlanApi, 'save'), approve = vi.spyOn(designPlanApi, 'approve');
  render(<DesignHistory projectId="p-1" scope="actor:workspace" canRead active currentRevision={1} />);
  await screen.findByRole('region', { name: 'Saved design revision 2' });
  expect(screen.getByText(/does not replace your draft or confirm its current approval/)).not.toBeNull();
  expect(screen.getByText(/Editor baseline/)).not.toBeNull();
  expect(screen.getByText(/Older revisions may be held in Context storage/)).not.toBeNull();
  expect(screen.queryByRole('button', { name: /Restore|Approve|Save design/ })).toBeNull();
  expect(save).not.toHaveBeenCalled(); expect(approve).not.toHaveBeenCalled();
});

it('aborts on a project change and cannot show a late snapshot from the previous scope', async () => {
  let finish!: (value: { revisions: ReturnType<typeof snapshot>[] }) => void;
  let firstSignal: AbortSignal | undefined;
  vi.spyOn(designPlanApi, 'history').mockImplementationOnce((_id, signal) => { firstSignal = signal; return new Promise(resolve => { finish = resolve; }); })
    .mockResolvedValue({ revisions: [snapshot(9, 'p-2')] });
  const view = render(<DesignHistory projectId="p-1" scope="actor:workspace" canRead active currentRevision={2} />);
  view.rerender(<DesignHistory projectId="p-2" scope="actor:workspace" canRead active currentRevision={9} />);
  await screen.findByRole('region', { name: 'Saved design revision 9' });
  expect(firstSignal?.aborted).toBe(true);
  await act(async () => finish({ revisions: [snapshot()] }));
  expect(screen.queryByRole('button', { name: 'Read design revision 2' })).toBeNull();
  view.rerender(<DesignHistory projectId="p-2" scope="actor:workspace" canRead={false} active currentRevision={9} />);
  expect(screen.queryByRole('region', { name: 'Saved design revision 9' })).toBeNull();
});
