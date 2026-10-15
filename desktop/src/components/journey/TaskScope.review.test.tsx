import { fireEvent, render, screen } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import type { ImplementationTask } from '../../api/product';
import { TaskScope } from './TaskScope';

it('requires an explicit saved reviewer role and candidate instead of treating empty write paths as review', async () => {
  const coding: ImplementationTask = {id: 'task-code', title: 'Build files', status: 'todo', writePaths: [], acceptance: 'Files implement the saved task'};
  const reviewer: ImplementationTask = {id: 'task-review', title: 'Inspect source', status: 'todo', writePaths: [], acceptance: 'Report checks of the candidate'};
  const onChange = vi.fn();
  render(<TaskScope task={reviewer} index={1} tasks={[coding, reviewer]} disabled={false} onChange={onChange} />);
  fireEvent.click(screen.getByText(/Scope and dependencies for task 2/));
  expect((await screen.findByLabelText('Task role') as HTMLSelectElement).value).toBe('coding');
  fireEvent.change(screen.getByLabelText('Task role'), {target: {value: 'reviewer'}});
  expect(onChange).toHaveBeenCalledWith(expect.objectContaining({role: 'reviewer', reviewOfTaskId: 'task-code', writePaths: [], dependsOn: []}));
});
