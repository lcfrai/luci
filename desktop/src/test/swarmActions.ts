import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

/** Follow the visible secondary-action disclosure before operating a task. */
export async function openTaskActions() {
  for (const summary of screen.getAllByText('More task actions', {exact: true})) {
    const details = summary.closest('details');
    if (!details) throw new Error('Task actions disclosure is missing.');
    if (!details.open) await userEvent.click(summary);
  }
}
