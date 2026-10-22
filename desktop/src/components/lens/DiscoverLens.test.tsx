import { act, cleanup, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { definitionApi, type ProjectSpecification } from '../../api/definition';
import { productApi } from '../../api/product';
import { requestProjectContextEdit, takeProjectContextEdit } from '../../api/projectContextEvents';
import { resetCockpit, useCockpit } from '../../store/store';
import { DiscoverLens } from './DiscoverLens';

vi.mock('../auth/AuthProvider', () => ({ useAuth: () => ({ profile: { user: { id: 'discover-owner' } } }) }));
vi.mock('../setup/SetupProvider', () => ({ useSetup: () => ({ status: { access: { user: { username: 'discover-owner' } }, config: { projectsDir: '/projects' } } }) }));
vi.mock('../teams/TeamWorkspaceProvider', () => ({
  useTeamWorkspace: () => null,
  useTeamProjectPermissions: () => ({ loading: false, error: '', permissions: { read: true, edit: true, execute: false, secrets: false, deploy: false, admin: false } }),
}));
vi.mock('../discover/DiscoverDocuments', () => ({ DiscoverDocuments: () => <section aria-label="Document evidence"><button>Refresh documents</button></section> }));

const scope = () => JSON.stringify([productApi.baseUrl, 'discover-owner', 'local', '', '', 'discover-owner', '/projects', 'p-1']);
const fixture = (): ProjectSpecification => ({ projectId: 'p-1', version: 1, revision: 2, briefRevision: 3, startingPoint: 'new', purpose: 'Resume saved projects', audience: 'Dylan', constraints: 'Local first', outOfScope: [], requirements: [], journeys: [], roles: [], entities: [], integrations: [], glossary: [], decisions: [], questions: [], status: 'draft', digest: '', updatedAt: '' });
const disclosure = () => screen.getByText('Edit the structured project definition').closest('details') as HTMLDetailsElement;

beforeEach(() => {
  localStorage.clear(); resetCockpit();
  useCockpit.setState({ selection: { projectId: 'p-1', branchId: null, worktreeId: null, sessionId: null } });
  vi.spyOn(definitionApi, 'get').mockResolvedValue(fixture());
  vi.spyOn(definitionApi, 'listAssistance').mockResolvedValue({ jobs: [] });
  vi.spyOn(productApi, 'agentConnections').mockResolvedValue({ revision: 0, activeConnectionId: '', connections: [] });
  vi.spyOn(productApi, 'agents').mockResolvedValue({ providers: [] });
});
afterEach(() => { cleanup(); vi.restoreAllMocks(); });

it('keeps document evidence and the manually opened definition in the same keyboard-reachable workspace', async () => {
  const save = vi.spyOn(definitionApi, 'save'), assist = vi.spyOn(definitionApi, 'assist');
  render(<DiscoverLens />);
  const workspace = screen.getByRole('region', { name: 'Discover workspace' });
  await userEvent.tab(); expect(document.activeElement).toBe(workspace);
  await userEvent.tab(); expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Refresh documents' }));
  await userEvent.tab(); expect(document.activeElement).toBe(screen.getByText('Edit the structured project definition'));
  expect(disclosure().open).toBe(false);
  await userEvent.click(screen.getByText('Edit the structured project definition'));
  expect(disclosure().open).toBe(true);
  expect(workspace.contains(await screen.findByRole('textbox', { name: 'The idea' }))).toBe(true);
  expect(save).not.toHaveBeenCalled(); expect(assist).not.toHaveBeenCalled();
});

it('reveals the definition only for the matching scoped edit request without saving or invoking an agent', async () => {
  const save = vi.spyOn(definitionApi, 'save'), assist = vi.spyOn(definitionApi, 'assist');
  render(<DiscoverLens />);
  await waitFor(() => expect(definitionApi.get).toHaveBeenCalled());
  let cancel = () => {};
  act(() => { cancel = requestProjectContextEdit(scope() + '-other', 'p-1', 'definition'); });
  expect(disclosure().open).toBe(false); cancel();
  act(() => { cancel = requestProjectContextEdit(scope(), 'p-1', 'design'); });
  expect(disclosure().open).toBe(false); cancel();
  act(() => { cancel = requestProjectContextEdit(scope(), 'p-1', 'definition'); });
  await waitFor(() => expect(disclosure().open).toBe(true));
  expect(document.activeElement).toBe(screen.getByText('Edit the structured project definition'));
  expect(screen.getByRole('tab', { name: 'Specification' }).getAttribute('aria-selected')).toBe('true');
  expect(takeProjectContextEdit(scope(), 'p-1', 'definition')).toBe(false);
  await userEvent.click(screen.getByRole('textbox', { name: 'The idea' }));
  await userEvent.type(screen.getByRole('textbox', { name: 'The idea' }), ' and notes');
  expect(document.activeElement).toBe(screen.getByRole('textbox', { name: 'The idea' }));
  expect(save).not.toHaveBeenCalled(); expect(assist).not.toHaveBeenCalled(); cancel();
});

it('opens the outer disclosure before a delayed exact-question edit focuses its answer', async () => {
  let finish!: (value: ProjectSpecification) => void;
  vi.mocked(definitionApi.get).mockReturnValue(new Promise(resolve => { finish = resolve; }));
  const save = vi.spyOn(definitionApi, 'save'), assist = vi.spyOn(definitionApi, 'assist');
  const cancel = requestProjectContextEdit(scope(), 'p-1', 'definition', 'storage');
  render(<DiscoverLens />);
  expect(disclosure().open).toBe(false);
  await act(async () => finish({ ...fixture(), questions: [{ id: 'storage', question: 'Where are notes saved?', answer: '' }] }));
  const group = await screen.findByRole('group', { name: 'Open question 1' });
  const answer = within(group).getByRole('textbox', { name: 'Answer' });
  await waitFor(() => expect(document.activeElement).toBe(answer));
  expect(disclosure().open).toBe(true);
  expect(save).not.toHaveBeenCalled(); expect(assist).not.toHaveBeenCalled(); cancel();
});
