import { cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, expect, it, vi } from 'vitest';
import { DeploymentWorkspace } from '../lens/DeployLens';
import { productApi } from '../../api/product';
import { resetCockpit } from '../../store/store';

vi.mock('../../hooks/useProjectDraft', () => ({ useProjectDraftScope: (id: string) => `owner:${id}` }));
vi.mock('../../hooks/useProjectAccess', () => ({ useProjectAccess: () => ({ canRead: true, canEdit: false, canExecute: false, loading: false, error: '', permissions: { read: true, edit: false, execute: false, deploy: false, secrets: false }, message: '' }) }));
vi.mock('../teams/TeamWorkspaceProvider', () => ({ useTeamWorkspace: () => null }));
vi.mock('./DeploymentOverview', () => ({ DeploymentOverview: () => null }));
vi.mock('./ProjectConfiguration', () => ({ ProjectConfiguration: () => null }));
vi.mock('../platform/PlatformWorkspace', () => ({ PlatformWorkspace: () => <p>Existing application platform</p> }));
afterEach(() => { cleanup(); vi.restoreAllMocks(); });

it('offers the project service editor in Deploy and retains its draft across workspace tabs without starting releases', async () => {
  resetCockpit(); const user = userEvent.setup();
  const release = vi.spyOn(productApi, 'release'), cloud = vi.spyOn(productApi, 'cloudRelease'), pages = vi.spyOn(productApi, 'pagesRelease');
  render(<DeploymentWorkspace projectId="p-one"/>);
  const blueprint = screen.getByRole('tab', { name: 'Service blueprint' });
  await user.click(blueprint);
  const panel = screen.getByRole('tabpanel', { name: 'Service blueprint' });
  expect(panel.id).toBe(blueprint.getAttribute('aria-controls'));
  await user.type(screen.getByRole('textbox', { name: 'Namespace' }), 'retained-draft');
  await user.click(screen.getByRole('tab', { name: 'Application platform' }));
  expect(screen.getByText('Existing application platform')).not.toBeNull();
  await user.click(blueprint);
  expect((screen.getByRole('textbox', { name: 'Namespace' }) as HTMLInputElement).value).toBe('retained-draft');
  expect(release).not.toHaveBeenCalled(); expect(cloud).not.toHaveBeenCalled(); expect(pages).not.toHaveBeenCalled();
});
