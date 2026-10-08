import { createRef, useState } from 'react';
import { render, screen, waitFor, cleanup } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, expect, it, vi } from 'vitest';
import { Button } from './button';
import { Button as CompatibleButton } from '../fluid/components/button';
import { Dialog, DialogTrigger, DialogContent, DialogTitle, DialogDescription, DialogClose } from './dialog';
afterEach(cleanup);

it('canonical button composes an anchor without nested buttons and forwards native refs', async () => {
  const ref = createRef<HTMLButtonElement>();
  const submitted = vi.fn();
  render(<form onSubmit={submitted}><Button ref={ref}>Review</Button><Button asChild><a href="#result">Result</a></Button></form>);
  await userEvent.click(screen.getByRole('button', { name: 'Review' }));
  expect(submitted).not.toHaveBeenCalled();
  expect(ref.current).toBe(screen.getByRole('button'));
  expect(screen.getByRole('link', { name: 'Result' }).getAttribute('data-slot')).toBe('button');
  expect(screen.getAllByRole('button')).toHaveLength(1);
});
it('compatibility pending controls preserve their label and prevent repeat actions', async () => {
  const click = vi.fn();
  render(<CompatibleButton loading onClick={click}>Save plan</CompatibleButton>);
  const button = screen.getByRole('button', { name: 'Save plan' });
  expect(button.getAttribute('aria-busy')).toBe('true');
  expect((button as HTMLButtonElement).disabled).toBe(true);
  await userEvent.click(button); expect(click).not.toHaveBeenCalled();
});
it('Radix modal traps focus, closes by Escape and restores its trigger', async () => {
  function Example() {
    const [open, setOpen] = useState(false);
    return <><button>Outside</button><Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild><CompatibleButton>Review changes</CompatibleButton></DialogTrigger>
      <DialogContent showCloseButton={false}><DialogTitle>Review changes</DialogTitle><DialogDescription>Inspect the retained output.</DialogDescription>
        <input aria-label="Comment" /><DialogClose asChild><Button>Keep editing</Button></DialogClose>
      </DialogContent></Dialog></>;
  }
  const user = userEvent.setup(); render(<Example />);
  const trigger = screen.getByRole('button', { name: 'Review changes' });
  await user.click(trigger); const dialog = screen.getByRole('dialog', { name: 'Review changes' });
  expect(dialog.contains(document.activeElement)).toBe(true);
  await user.tab({ shift: true }); expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Keep editing' }));
  await user.tab(); expect(document.activeElement).toBe(screen.getByRole('textbox', { name: 'Comment' }));
  await user.keyboard('{Escape}');
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
  await waitFor(() => expect(document.activeElement).toBe(trigger));
});
