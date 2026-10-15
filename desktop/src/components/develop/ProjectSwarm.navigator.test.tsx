import { fireEvent, render, screen } from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { swarmHarnessFixture, swarmRunFixture } from '../../test/swarmFixtures';
import { resetCockpit, useCockpit } from '../../store/store';
import { ProjectSwarm } from '../journey/ProjectSwarm';
vi.mock('../journey/SwarmRunQueue', () => ({SwarmRunQueue: () => null}));
beforeEach(() => { resetCockpit(); useCockpit.getState().setHarnesses([swarmHarnessFixture()]); });
it('opens and focuses the exact existing task scope without opening a session or changing the run', () => {
  const update = vi.fn(), session = vi.fn();
  render(<ProjectSwarm run={swarmRunFixture()} canEdit canExecute busy={false} onBusy={vi.fn()} onUpdate={update} onOpenSession={session} />);
  fireEvent.click(screen.getByRole('button', {name: 'Select task Build c (c)'}));
  fireEvent.click(screen.getByRole('button', {name: 'Go to task controls'}));
  const heading = screen.getByRole('heading', {name: 'Build c', level: 4});
  expect(document.activeElement).toBe(heading);
  expect(heading.closest('li')?.querySelector('details')?.open).toBe(true);
  expect(update).not.toHaveBeenCalled(); expect(session).not.toHaveBeenCalled();
});
