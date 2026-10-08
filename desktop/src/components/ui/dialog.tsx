// Adapted from shadcn/ui (MIT). Exact source and grant: third-party/shadcn-ui.
import * as React from 'react';
import * as DialogPrimitive from '@radix-ui/react-dialog';
import { NavigationIcon } from '../common/NavigationIcon';
import { cn } from './utils';
import { Button } from './button';
import { useSizeVariant } from '../fluid/lib/size-context';

const Dialog = DialogPrimitive.Root;
const DialogPortal = DialogPrimitive.Portal;
interface DialogSlotProps extends React.ComponentPropsWithoutRef<typeof DialogPrimitive.Trigger> { render?: React.ReactElement }
const DialogTrigger = React.forwardRef<HTMLButtonElement, DialogSlotProps>(({ render, children, asChild, ...props }, ref) =>
  <DialogPrimitive.Trigger ref={ref} data-slot="dialog-trigger" asChild={!!render || asChild} {...props}>{render ?? children}</DialogPrimitive.Trigger>);
const DialogClose = React.forwardRef<HTMLButtonElement, DialogSlotProps>(({ render, children, asChild, ...props }, ref) =>
  <DialogPrimitive.Close ref={ref} data-slot="dialog-close" asChild={!!render || asChild} {...props}>{render ?? children}</DialogPrimitive.Close>);
const DialogOverlay = React.forwardRef<HTMLDivElement, React.ComponentPropsWithoutRef<typeof DialogPrimitive.Overlay>>(({ className, ...props }, ref) =>
  <DialogPrimitive.Overlay ref={ref} data-slot="dialog-overlay" className={cn('luci-dialog-overlay fluid-dialog-overlay', className)} {...props} />);

interface DialogContentProps extends React.ComponentPropsWithoutRef<typeof DialogPrimitive.Content> {
  size?: 'sm' | 'lg' | 'xl';
  container?: HTMLElement | null;
  showCloseButton?: boolean;
  position?: 'center' | 'top';
}
const DialogContent = React.forwardRef<HTMLDivElement, DialogContentProps>(({ className, children, size = 'sm', container, showCloseButton = true, position = 'center', ...props }, ref) => {
  const compact = useSizeVariant() === 'compact';
  return <DialogPortal container={container ?? undefined}>
    <DialogOverlay data-contained={!!container} />
    <DialogPrimitive.Content ref={ref} data-slot="dialog-content" data-position={position} data-contained={!!container} data-size={size} data-density={compact ? 'compact' : 'default'}
      className={cn('luci-dialog fluid-dialog', className)} {...props}>
      {children}
      {showCloseButton && <DialogPrimitive.Close asChild><Button variant="ghost" size="icon" className="luci-dialog-close" aria-label="Close"><NavigationIcon name="close" /></Button></DialogPrimitive.Close>}
    </DialogPrimitive.Content>
  </DialogPortal>;
});
function DialogHeader({ className, ...props }: React.ComponentProps<'div'>) { return <div data-slot="dialog-header" className={cn('luci-dialog-header', className)} {...props} />; }
function DialogFooter({ className, ...props }: React.ComponentProps<'div'>) { return <div data-slot="dialog-footer" className={cn('luci-dialog-footer', className)} {...props} />; }
const DialogTitle = React.forwardRef<HTMLHeadingElement, React.ComponentPropsWithoutRef<typeof DialogPrimitive.Title>>(({ className, ...props }, ref) =>
  <DialogPrimitive.Title ref={ref} data-slot="dialog-title" className={cn('luci-dialog-title', className)} {...props} />);
const DialogDescription = React.forwardRef<HTMLParagraphElement, React.ComponentPropsWithoutRef<typeof DialogPrimitive.Description>>(({ className, ...props }, ref) =>
  <DialogPrimitive.Description ref={ref} data-slot="dialog-description" className={cn('luci-dialog-description', className)} {...props} />);
DialogTrigger.displayName = 'DialogTrigger'; DialogClose.displayName = 'DialogClose'; DialogOverlay.displayName = 'DialogOverlay'; DialogContent.displayName = 'DialogContent'; DialogTitle.displayName = 'DialogTitle'; DialogDescription.displayName = 'DialogDescription';
export { Dialog, DialogTrigger, DialogPortal, DialogClose, DialogOverlay, DialogContent, DialogHeader, DialogFooter, DialogTitle, DialogDescription };
export type { DialogSlotProps as DialogTriggerProps, DialogSlotProps as DialogCloseProps };
