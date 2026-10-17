import type { ButtonHTMLAttributes } from 'react';
import { cva, type VariantProps } from 'class-variance-authority';
import { cn } from '@/lib/cn';

/**
 * Button. Matches docs/03-UI-PATTERNS.md §3.1.
 *
 * `transition-colors duration-100` and nothing slower: a 200 ms hover on a
 * button a cashier presses fifty times an hour reads as lag, not polish.
 */
const button = cva(
  'inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-sm font-medium ' +
    'transition-colors duration-100 disabled:pointer-events-none disabled:opacity-55',
  {
    variants: {
      variant: {
        primary: 'bg-primary text-text-inverse hover:bg-primary-hover active:bg-primary-active',
        secondary: 'border border-border-strong bg-surface text-text hover:bg-surface-sunken',
        ghost: 'text-text-subtle hover:bg-surface-sunken hover:text-text',
      },
      size: {
        sm: 'h-8 px-3 text-sm',
        md: 'h-9 px-4 text-base',
        lg: 'h-10 px-5 text-md',
      },
    },
    defaultVariants: { variant: 'primary', size: 'md' },
  },
);

export interface ButtonProps
  extends ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof button> {}

export function Button({ className, variant, size, type = 'button', ...props }: ButtonProps) {
  return <button type={type} className={cn(button({ variant, size }), className)} {...props} />;
}
