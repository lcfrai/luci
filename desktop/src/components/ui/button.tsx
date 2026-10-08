// Adapted from shadcn/ui (MIT). Exact source and grant: third-party/shadcn-ui.
import * as React from 'react';
import { Slot } from '@radix-ui/react-slot';
import { cva, type VariantProps } from 'class-variance-authority';
import { cn } from './utils';

const buttonVariants = cva(
  'luci-ui-button inline-flex shrink-0 items-center justify-center gap-2 whitespace-nowrap rounded-md text-sm font-medium disabled:pointer-events-none disabled:opacity-50 [&_svg]:pointer-events-none [&_svg]:shrink-0',
  {
    variants: {
      variant: {
        default: 'luci-ui-button-primary',
        destructive: 'luci-ui-button-destructive',
        outline: 'luci-ui-button-outline',
        secondary: 'luci-ui-button-secondary',
        ghost: 'luci-ui-button-ghost',
        link: 'luci-ui-button-link',
      },
      size: {
        default: 'h-9 px-4 py-2',
        sm: 'h-7 gap-1.5 px-3 text-xs',
        lg: 'h-10 px-6',
        icon: 'h-9 w-9 p-0',
        'icon-sm': 'h-7 w-7 p-0',
        'icon-lg': 'h-10 w-10 p-0',
      },
    },
    defaultVariants: { variant: 'default', size: 'default' },
  },
);

type ButtonProps = React.ComponentPropsWithoutRef<'button'> &
  VariantProps<typeof buttonVariants> & { asChild?: boolean };

const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant = 'default', size = 'default', asChild = false, type = 'button', ...props }, ref) => {
    const Comp = asChild ? Slot : 'button';
    return <Comp ref={ref} data-slot="button" data-variant={variant} data-size={size}
      {...(!asChild ? { type } : {})} className={cn(buttonVariants({ variant, size, className }))} {...props} />;
  },
);
Button.displayName = 'Button';
export { Button, buttonVariants };
export type { ButtonProps };
