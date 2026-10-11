import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, it, vi } from 'vitest';
import { CoordinatorCheckSetup } from './CoordinatorCheckSetup';
import { resetCockpit, useCockpit } from '../../store/store';
const requirements = [{ id: 'REQ-1', title: 'Book', acceptance: 'A visitor can book' }];
const props = () => ({ projectId: 'p-1', scope: 'alice/local/p-1', base: 'approved-2', requirements, disabled: false, onChange: vi.fn() });
async function openSetup() { if (!screen.queryByLabelText("Executable")) { await userEvent.click(screen.getByText(/Independent checks for continued work/)); await screen.findByLabelText("Executable"); } }
async function addCommand() { await openSetup(); await userEvent.type(screen.getByLabelText('Executable'), 'npm'); fireEvent.change(screen.getByLabelText('Arguments · JSON array'), { target: { value: '["test"]' } }); await userEvent.click(screen.getByRole('button', { name: 'Add check command' })); }
async function consent() { await userEvent.click(screen.getByRole('checkbox', { name: /REQ-1/ })); await userEvent.click(screen.getByRole('checkbox', { name: /I delegate these exact/ })); }
beforeEach(() => { resetCockpit(); localStorage.clear(); });
it('requires exact command and requirement consent; editing invalidates consent', async () => {
  const input = props(); render(<CoordinatorCheckSetup {...input} />); await addCommand();
  expect(input.onChange.mock.lastCall?.[0].plan).toBeNull(); await consent();
  await waitFor(() => expect(input.onChange.mock.lastCall?.[0].plan).toEqual({ commands: [{ label: 'npm', executable: 'npm', args: ['test'] }], requirementIds: ['REQ-1'], requirementSnapshots: requirements, acknowledgeProjectCommands: true }));
  await userEvent.type(screen.getByLabelText('Executable'), 'go'); expect(input.onChange.mock.lastCall?.[0].plan).toBeNull();
});
it('retains scoped command text across reopen but never coverage or execution consent', async () => {
  const input = props(), ui = render(<CoordinatorCheckSetup {...input} />); await addCommand(); await consent();
  expect(Object.values(useCockpit.getState().unsavedProjectDrafts)[0]).toMatchObject({ projectId: 'p-1', purpose: 'coordinator-setup', retained: true });
  ui.unmount(); render(<CoordinatorCheckSetup {...input} />); await openSetup(); await screen.findByRole('region', { name: 'Continued-work checks draft recovery' });
  expect(input.onChange.mock.lastCall?.[0].plan).toBeNull(); await userEvent.click(screen.getByRole('button', { name: 'Restore draft' }));
  expect(screen.getByRole('button', { name: 'Remove npm' })).not.toBeNull();
  expect(screen.getByRole('checkbox', { name: /REQ-1/ })).toHaveProperty('checked', false);
  expect(screen.getByRole('checkbox', { name: /I delegate these exact/ })).toHaveProperty('checked', false);
});
it('invalidates coverage when acceptance text or authority changes, without losing commands', async () => {
  const input = props(), ui = render(<CoordinatorCheckSetup {...input} />); await addCommand(); await consent();
  ui.rerender(<CoordinatorCheckSetup {...input} base="approved-3" requirements={[{ ...requirements[0], acceptance: 'Visitor receives confirmation' }]} />);
  await waitFor(() => expect(input.onChange.mock.lastCall?.[0].plan).toBeNull()); expect(screen.getByRole('button', { name: 'Remove npm' })).not.toBeNull();
  await consent(); ui.rerender(<CoordinatorCheckSetup {...input} disabled />); expect(input.onChange.mock.lastCall?.[0].plan).toBeNull();
});
it('does not restore another project’s command draft and clears only the exact recorded draft', async () => {
  const input = props(), ui = render(<CoordinatorCheckSetup {...input} />); await addCommand(); const digest = input.onChange.mock.lastCall?.[0].draftDigest;
  ui.rerender(<CoordinatorCheckSetup {...input} recordedDraft="different" />); expect(screen.getByRole('button', { name: 'Remove npm' })).not.toBeNull();
  ui.rerender(<CoordinatorCheckSetup {...input} recordedDraft={digest} />); await waitFor(() => expect(screen.queryByRole('button', { name: 'Remove npm' })).toBeNull());
  await addCommand(); ui.unmount(); render(<CoordinatorCheckSetup {...props()} projectId="p-2" scope="alice/local/p-2" />);
  expect(screen.queryByRole('region', { name: /draft recovery/ })).toBeNull(); expect(screen.queryByRole('button', { name: 'Remove npm' })).toBeNull();
});
it('rejects shell paths and malformed argument lists without enabling a plan', async () => {
  const input = props(); render(<CoordinatorCheckSetup {...input} />); await openSetup();
  await userEvent.type(screen.getByLabelText('Executable'), '../run.sh'); await userEvent.click(screen.getByRole('button', { name: 'Add check command' }));
  expect((await screen.findByRole('alert')).textContent).toMatch(/simple executable/); expect(input.onChange.mock.lastCall?.[0].plan).toBeNull();
});

it('defers unopened editor controls but retains them and typed text after collapse', async () => {
  render(<CoordinatorCheckSetup {...props()} />); expect(screen.queryByLabelText('Executable')).toBeNull();
  await openSetup(); await userEvent.type(screen.getByLabelText('Executable'), 'npm');
  await userEvent.click(screen.getByText('Independent checks for continued work')); expect(screen.getByLabelText('Executable')).toHaveProperty('value', 'npm');
  await userEvent.click(screen.getByText('Independent checks for continued work')); expect(screen.getByLabelText('Executable')).toHaveProperty('value', 'npm');
});
