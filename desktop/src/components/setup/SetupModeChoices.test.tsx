import { cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, expect, it, vi } from 'vitest';
import { SetupModeChoices } from './SetupModeChoices';
afterEach(cleanup);

it('presents three explicit setup choices and keeps hosted availability truthful', async () => {
  const choose=vi.fn(), existing=vi.fn();
  render(<SetupModeChoices busy={false} manageBacking allowProvision onChoose={choose} onExisting={existing}/>);
  const group=screen.getByRole('group',{name:'Workspace setup options'});
  expect(group.querySelectorAll('button')).toHaveLength(3);
  expect(screen.getByRole('button',{name:'Local'})).toBeTruthy();
  expect(screen.getByRole('button',{name:'Bring your own cloud'})).toBeTruthy();
  const hosted=screen.getByRole('button',{name:'Luci Cloud'});
  expect(screen.getByText(/Not available yet/)).toBeTruthy();
  expect(screen.queryByRole('button',{name:/pay|subscribe|create hosted/i})).toBeNull();
  await userEvent.click(hosted);expect(choose).toHaveBeenCalledWith('luci-cloud');
  await userEvent.click(screen.getByRole('button',{name:'Connect to an existing workspace'}));expect(existing).toHaveBeenCalledOnce();
});

it('preserves existing connections when new cloud provisioning is forbidden', () => {
  render(<SetupModeChoices selected="local" busy={false} manageBacking allowProvision={false} onChoose={vi.fn()} onExisting={vi.fn()}/>);
  expect(screen.getByRole('button',{name:'Bring your own cloud'})).toHaveProperty('disabled',true);
  expect(screen.getByRole('button',{name:'Connect to an existing workspace'})).toHaveProperty('disabled',false);
  expect(screen.getByRole('button',{name:'Local'}).getAttribute('aria-pressed')).toBe('true');
});
