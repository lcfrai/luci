import { act, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { ownDesignWorkStatus } from '../../store/designWorkStatus';
import { DesignWorkIndicator } from './DesignWorkIndicator';
afterEach(() => vi.useRealTimers());
it('animates only a fresh observed running job, expires without polling and clears on owner cleanup', () => {
  vi.useFakeTimers(); vi.setSystemTime(1000); const owner = ownDesignWorkStatus('scope');
  owner.publish({jobId: 'real-job', phase: 'working', updatedAt: 'now', observedAt: Date.now()});
  render(<DesignWorkIndicator scope="scope" />); expect(screen.getByRole('status').getAttribute('data-active')).toBe('true');
  act(() => vi.advanceTimersByTime(10001)); expect(screen.getByRole('status').hasAttribute('data-active')).toBe(false);
  expect(screen.getByText(/current work is unconfirmed/)).toBeTruthy();
  act(() => owner.close()); expect(screen.getByText(/No current Luci job observation/)).toBeTruthy();
});
it('stays static while hidden/inactive and requires a new response after becoming visible', () => {
  vi.useFakeTimers(); vi.setSystemTime(1000); const owner = ownDesignWorkStatus('scope-hidden');
  owner.publish({jobId: 'real-job', phase: 'working', updatedAt: 'now', observedAt: Date.now()});
  const view = render(<DesignWorkIndicator scope="scope-hidden" />);
  const original = Object.getOwnPropertyDescriptor(document, 'visibilityState');
  Object.defineProperty(document, 'visibilityState', {configurable: true, value: 'hidden'}); fireEvent(document, new Event('visibilitychange'));
  Object.defineProperty(document, 'visibilityState', {configurable: true, value: 'visible'}); fireEvent(document, new Event('visibilitychange'));
  expect(screen.getByRole('status').hasAttribute('data-active')).toBe(false);
  act(() => { vi.advanceTimersByTime(1); owner.publish({jobId: 'real-job', phase: 'working', updatedAt: 'now', observedAt: Date.now()}); });
  expect(screen.getByRole('status').getAttribute('data-active')).toBe('true');
  view.rerender(<DesignWorkIndicator scope="scope-hidden" active={false} />); expect(screen.getByRole('status').hasAttribute('data-active')).toBe(false);
  owner.close(); if (original) Object.defineProperty(document, 'visibilityState', original); else Reflect.deleteProperty(document, 'visibilityState');
});
