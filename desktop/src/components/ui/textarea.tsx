// Adapted from shadcn/ui (MIT). Exact source and grant: third-party/shadcn-ui.
import * as React from 'react';
import { cn } from './utils';

const Textarea = React.forwardRef<HTMLTextAreaElement, React.ComponentPropsWithoutRef<'textarea'>>(
  ({ className, ...props }, ref) => <textarea ref={ref} data-slot="textarea"
    className={cn('luci-input flex min-h-16 w-full rounded-md border px-3 py-2 disabled:cursor-not-allowed disabled:opacity-50', className)} {...props} />,
);
Textarea.displayName = 'Textarea';
export { Textarea };
