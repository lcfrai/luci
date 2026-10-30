import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import type { ReactNode } from 'react';
import { emptyGTM } from '../../api/gtmWorkspace';
import { AmendmentReview } from './AmendmentReview';
import { context, proposalTurn, view } from './fixtures';

vi.mock('../common/WorkflowDialog', () => ({ WorkflowDialog: ({ title, description, children }: { title: string; description: string; children: ReactNode }) => <section aria-label={title}><p>{description}</p>{children}</section> }));
function proposed() {
  const before = { ...emptyGTM(), offer: 'Old pilot' };
  const saved = { ...context(), gtm: { revision: 1, digest: 'b'.repeat(64), sourceDigest: 'c'.repeat(64), content: before, artifactLinks: [], artifactsOmitted: 0 } };
  const base = proposalTurn();
  return { ...base, context: saved, answer: { ...base.answer!, proposal: { target: 'gtm' as const, summary: 'Narrow the pilot', specification: null, designPlan: null, plan: null, gtm: { ...before, offer: 'A scoped paid pilot' } } } };
}
describe('GTM amendment review', () => {
  it('shows saved and proposed strategy and requires explicit acknowledgement', async () => {
    const turn = proposed(), apply = vi.fn();
    render(<AmendmentReview turn={turn} view={view({ context: turn.context })} readAccess={{ canRead: true, loading: false, error: '' }} busy={false} blocked="" error="" onClose={vi.fn()} onApply={apply} />);
    expect(screen.getByText('Old pilot')).not.toBeNull(); expect(screen.getByText('A scoped paid pilot')).not.toBeNull();
    expect(screen.getByText(/does not run experiments, publish campaigns or spend money/)).not.toBeNull();
    const button = screen.getByRole('button', { name: 'Apply reviewed amendment' });
    expect(button).toHaveProperty('disabled', true); await userEvent.click(button); expect(apply).not.toHaveBeenCalled();
    await userEvent.click(screen.getByRole('checkbox')); await userEvent.click(button); expect(apply).toHaveBeenCalledTimes(1);
  });
  it('keeps a stale proposal or unsaved-editor fence disabled after acknowledgement', async () => {
    const turn = proposed(), apply = vi.fn();
    const mounted = render(<AmendmentReview turn={turn} view={view({ context: turn.context })} readAccess={{ canRead: true, loading: false, error: '' }} busy={false} blocked="Go-to-market workspace has unsaved edits." error="" onClose={vi.fn()} onApply={apply} />);
    await userEvent.click(screen.getByRole('checkbox'));
    expect(screen.getByRole('button', { name: 'Apply reviewed amendment' })).toHaveProperty('disabled', true);
    mounted.rerender(<AmendmentReview turn={turn} view={view({ context: { ...turn.context, digest: 'd'.repeat(64) } })} readAccess={{ canRead: true, loading: false, error: '' }} busy={false} blocked="" error="" onClose={vi.fn()} onApply={apply} />);
    await userEvent.click(screen.getByRole('button', { name: 'Apply reviewed amendment' })); expect(apply).not.toHaveBeenCalled();
    expect(screen.getByRole('alert').textContent).toContain('saved context changed');
  });
});
