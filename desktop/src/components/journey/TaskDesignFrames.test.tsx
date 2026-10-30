import { cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, expect, it, vi } from 'vitest';
import type { DesignPlan } from '../../api/designPlan';
import { TaskDesignFrames } from './TaskDesignFrames';

const design: DesignPlan = { projectId: 'p-one', revision: 2, briefRevision: 1, specificationRevision: 1, sketchRevision: 1, status: 'approved', digest: 'a'.repeat(64), updatedAt: '', screens: [], tokens: [], assets: [], implementationNotes: '', prototypeReleaseId: '', prototypeCommit: '', scene: { version: 1, frames: [{ id: 'phone', screenId: 'screen-home', name: 'Home phone', state: 'Error', width: 390, height: 844, background: '#ffffff', nodes: [] }] } };
afterEach(cleanup);

it('selects the exact approved frame and retains the complete task metadata', async () => {
  const onChange = vi.fn(), task = { id: 'ui', title: 'Build Home', status: 'todo' as const, writePaths: ['src/Home.tsx'] };
  render(<TaskDesignFrames task={task} index={0} design={design} disabled={false} loading={false} error="" onChange={onChange}/>);
  await userEvent.click(screen.getByText('Design frames for task 1 · 0 linked'));
  await userEvent.click(screen.getByLabelText('Link frame phone to task 1'));
  expect(onChange).toHaveBeenCalledWith({ ...task, designFrameIds: ['phone'] });
  expect(screen.getByText(/Home phone · 390 × 844/)).not.toBeNull();
});

it('allows explicit repair of removed links while the design is unavailable', async () => {
  const onChange = vi.fn(), task = { id: 'ui', title: 'Build Home', status: 'todo' as const, designFrameIds: ['removed'] };
  render(<TaskDesignFrames task={task} index={0} design={null} disabled={false} loading={false} error="Design unavailable" onChange={onChange}/>);
  await userEvent.click(screen.getByText('Design frames for task 1 · 1 linked'));
  await userEvent.click(screen.getByLabelText('Remove unavailable frame removed from task 1'));
  expect(onChange).toHaveBeenCalledWith({ ...task, designFrameIds: [] });
});

it('does not allow new draft frame links or any changes without edit access', async () => {
  const onChange = vi.fn(), task = { id: 'cli', title: 'Build CLI', status: 'todo' as const };
  const view = render(<TaskDesignFrames task={task} index={0} design={{ ...design, status: 'draft' }} disabled={false} loading={false} error="" onChange={onChange}/>);
  expect((screen.getByLabelText('Link frame phone to task 1') as HTMLInputElement).disabled).toBe(true);
  view.rerender(<TaskDesignFrames task={{ ...task, designFrameIds: ['phone'] }} index={0} design={design} disabled loading={false} error="" onChange={onChange}/>);
  expect((screen.getByLabelText('Link frame phone to task 1') as HTMLInputElement).disabled).toBe(true);
  expect(onChange).not.toHaveBeenCalled();
});

it('keeps nonvisual tasks valid and rejects malformed recovered frame lists', async () => {
  const { planDraftValid } = await import('../../hooks/projectDraftValidation');
  const task = { id: 'cli', title: 'Build CLI', status: 'todo' };
  expect(planDraftValid({ tasks: [task], title: '' })).toBe(true);
  expect(planDraftValid({ tasks: [{ ...task, designFrameIds: ['phone'] }], title: '' })).toBe(true);
  expect(planDraftValid({ tasks: [{ ...task, designFrameIds: [42] }], title: '' })).toBe(false);
});
