import { beforeEach, expect, it, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MachineChip } from './MachineChip';
import { aMachine } from '../../test/fixtures';
import { resetCockpit } from '../../store/store';
import axe from 'axe-core';

const teamWorkspace = vi.hoisted(() => ({ teamId: '' }));
vi.mock('../teams/TeamWorkspaceProvider', () => ({ useTeamWorkspace: () => teamWorkspace }));
beforeEach(() => { resetCockpit(); teamWorkspace.teamId = ''; });

it('identifies the default team execution host without describing it as the viewer’s PC', async () => {
  teamWorkspace.teamId = 'team-aaaaaaaaaaaaaaaaaaaaaa';
  const machine = aMachine({ id: 'local', name: 'This PC', kind: 'local', state: 'error', message: 'Docker is not responding on the execution host.', sessionMemoryMiB: 4096, sessionCPUMillis: 2000 });
  render(<MachineChip machine={machine} />);
  expect(screen.getByRole('img', { name: 'error · Team execution host' })).toBeTruthy();
  await userEvent.click(screen.getByRole('button', { name: /Team execution host/ }));
  const details = await screen.findByRole('dialog');
  expect(within(details).getByText('Team execution host · error · docker')).toBeTruthy();
  expect(within(details).getByText(machine.message!)).toBeTruthy();
  expect(within(details).getByText('4096 MiB · 2 CPU per agent session')).toBeTruthy();
  for (const limitation of machine.limitations) expect(within(details).getByText(limitation)).toBeTruthy();
  expect(screen.queryByText('This PC')).toBeNull();
  expect(machine.id).toBe('local'); expect(machine.name).toBe('This PC');
  await userEvent.keyboard('{Escape}');
  expect(screen.queryByRole('dialog')).toBeNull();
  expect(document.activeElement).toBe(screen.getByRole('button', { name: /Team execution host/ }));
  const result = await axe.run(document, { runOnly: { type: 'rule', values: ['button-name', 'aria-valid-attr-value'] } });
  expect(result.violations.map(rule => rule.id)).toEqual([]);
});

it('updates the display label across personal and team scopes without changing the machine record', async () => {
  const machine = Object.freeze(aMachine({ id: 'local', name: 'This PC', kind: 'local' }));
  const view = render(<MachineChip machine={machine} />);
  await userEvent.click(screen.getByRole('button', { name: /This PC/ }));
  await screen.findByRole('dialog');
  expect(screen.getByText('This PC · running · docker')).toBeTruthy();
  teamWorkspace.teamId = 'team-aaaaaaaaaaaaaaaaaaaaaa';
  view.rerender(<MachineChip machine={machine} />);
  expect(screen.getByRole('img', { name: 'running · Team execution host' })).toBeTruthy();
  expect(screen.getByText('Team execution host · running · docker')).toBeTruthy();
  teamWorkspace.teamId = '';
  view.rerender(<MachineChip machine={machine} />);
  expect(screen.getByRole('img', { name: 'running · This PC' })).toBeTruthy();
  expect(screen.getByText('This PC · running · docker')).toBeTruthy();
  expect(machine.id).toBe('local'); expect(machine.name).toBe('This PC');
});

it.each([
  { id: 'local', kind: 'local' as const, name: 'Build workstation', default: true },
  { id: 'remote', kind: 'cloud' as const, name: 'Azure builder', default: true },
  { id: 'remote', kind: 'local' as const, name: 'This PC', default: true },
  { id: 'local', kind: 'cloud' as const, name: 'This PC', default: true },
  { id: 'local', kind: 'local' as const, name: 'This PC', default: false },
])('preserves the server name for $id/$kind/$name/default=$default', input => {
  teamWorkspace.teamId = 'team-aaaaaaaaaaaaaaaaaaaaaa';
  render(<MachineChip machine={aMachine(input)} />);
  expect(screen.getByRole('img', { name: `running · ${input.name}` })).toBeTruthy();
  expect(screen.queryByText('Team execution host')).toBeNull();
});

it('does not invent a team machine when none is reported', () => {
  teamWorkspace.teamId = 'team-aaaaaaaaaaaaaaaaaaaaaa';
  render(<MachineChip machine={undefined} />);
  expect(screen.getByText('No machine yet. The control plane reports none.')).toBeTruthy();
  expect(screen.queryByRole('button')).toBeNull();
});
