// Luci compatibility adapter. The rendered control is the pinned shadcn/ui Button.
// Fluid's retained icon/density contracts remain attributed in its manifest.
import { cloneElement, forwardRef, isValidElement, type ButtonHTMLAttributes, type ReactElement, type ReactNode } from 'react';
import { Button as ShadcnButton, buttonVariants as canonicalVariants } from '../../ui/button';
import type { IconComponent } from '../lib/icon-context';
import { useSizeVariant } from '../lib/size-context';

type ButtonSize = 'default' | 'compact' | 'icon' | 'icon-compact' | 'sm' | 'md' | 'lg' | 'icon-sm' | 'icon-lg';
type ButtonVariant = 'primary' | 'secondary' | 'tertiary' | 'ghost';
interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant | null;
  size?: ButtonSize;
  asChild?: boolean;
  render?: ReactElement;
  nativeButton?: boolean;
  loading?: boolean;
  leadingIcon?: IconComponent;
  trailingIcon?: IconComponent;
  active?: boolean;
}
const variants = { primary: 'default', secondary: 'secondary', tertiary: 'outline', ghost: 'ghost' } as const;
const sizes = { default: 'default', compact: 'sm', icon: 'icon', 'icon-compact': 'icon-sm', sm: 'sm', md: 'default', lg: 'default', 'icon-sm': 'icon-sm', 'icon-lg': 'icon' } as const;
const buttonVariants = ({ variant = 'primary', size = 'default', className }: { variant?: ButtonVariant; size?: ButtonSize; className?: string } = {}) => canonicalVariants({ variant: variants[variant], size: sizes[size], className });
const Button = forwardRef<HTMLButtonElement, ButtonProps>(({ variant = 'primary', size, asChild, render, nativeButton: _nativeButton, loading, leadingIcon: LeadingIcon, trailingIcon: TrailingIcon, active, disabled, children, onClick, ...props }, ref) => {
  const density = useSizeVariant();
  const resolved = sizes[size ?? (density === 'compact' ? 'compact' : 'default')];
  const child = render && isValidElement(render)
    ? (children === undefined ? render : cloneElement(render as ReactElement<{ children?: ReactNode }>, undefined, children))
    : asChild && isValidElement(children) ? children : null;
  const label = child ? (child as ReactElement<{ children?: ReactNode }>).props.children : children;
  const pending = disabled || loading;
  const content = <>{loading && <svg aria-hidden="true" className="luci-ui-button-spinner" viewBox="0 0 24 24" fill="none"><circle cx="12" cy="12" r="9" stroke="currentColor" strokeWidth="2" strokeDasharray="36 20" /></svg>}{!loading && LeadingIcon && <LeadingIcon size={resolved.includes('sm') ? 14 : 16} />}<span>{label}</span>{TrailingIcon && <TrailingIcon size={resolved.includes('sm') ? 14 : 16} />}</>;
  return <ShadcnButton {...props} ref={ref} variant={variants[variant ?? 'primary']} size={resolved}
    asChild={!!child} disabled={child ? undefined : pending} aria-disabled={child && pending ? true : undefined}
    aria-busy={loading || undefined} data-active={active || undefined} tabIndex={child && pending ? -1 : props.tabIndex}
    onClick={event => { if (pending) { event.preventDefault(); event.stopPropagation(); return; } onClick?.(event); }}>
    {child ? cloneElement(child as ReactElement<{ children?: ReactNode }>, undefined, content) : content}
  </ShadcnButton>;
});
Button.displayName = 'Button';
export { Button, buttonVariants };
export type { ButtonProps, ButtonSize };
