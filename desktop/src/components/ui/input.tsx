// Adapted from shadcn/ui (MIT). Exact source and grant: third-party/shadcn-ui.
import * as React from 'react';
import { cn } from './utils';

const Input = React.forwardRef<HTMLInputElement, React.ComponentPropsWithoutRef<'input'>>(
  ({ className, type, ...props }, ref) => <input type={type} ref={ref} data-slot="input"
    className={cn('luci-input flex w-full min-w-0 rounded-md border px-3 py-2 disabled:cursor-not-allowed disabled:opacity-50', className)} {...props} />,
);
Input.displayName = 'Input';
export { Input };
