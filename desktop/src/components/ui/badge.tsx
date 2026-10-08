// Adapted from shadcn/ui (MIT). Exact source and grant: third-party/shadcn-ui.
import * as React from 'react';
import { Slot } from '@radix-ui/react-slot';
import { cva, type VariantProps } from 'class-variance-authority';
import { cn } from './utils';

const badgeVariants = cva('luci-badge inline-flex w-fit shrink-0 items-center justify-center gap-1 overflow-hidden rounded-md border px-2 py-0.5 text-xs font-medium', {
  variants: { variant: { default: 'luci-badge-default', secondary: 'luci-badge-secondary', destructive: 'luci-badge-destructive', outline: 'luci-badge-outline' } },
  defaultVariants: { variant: 'default' },
});
function Badge({ className, variant = 'default', asChild = false, ...props }: React.ComponentProps<'span'> & VariantProps<typeof badgeVariants> & { asChild?: boolean }) {
  const Comp = asChild ? Slot : 'span';
  return <Comp data-slot="badge" data-variant={variant} className={cn(badgeVariants({ variant, className }))} {...props} />;
}
export { Badge, badgeVariants };
