import type { InputHTMLAttributes, Ref } from 'react';
import { cva, type VariantProps } from 'class-variance-authority';
import { cn } from '@/lib/cn';

/**
 * Text input. Matches docs/03-UI-PATTERNS.md §3.1.
 *
 * The control boundary is `border-border-strong` (3.46:1), not the decorative
 * `border-border` (1.23:1). A field whose outline you cannot see is a field an
 * older user cannot find.
 *
 * Text is 16px (`text-md`) so iOS does not zoom the viewport on focus — a zoom
 * that shifts the whole form and makes the next field jump under the thumb.
 */
const input = cva(
  'w-full rounded-sm border border-border-strong bg-surface text-text ' +
    'placeholder:text-text-muted transition-colors duration-100 ' +
    'disabled:bg-surface-sunken disabled:text-text-muted ' +
    'aria-[invalid=true]:border-danger',
  {
    variants: {
      size: {
        sm: 'h-8 px-2 text-base',
        md: 'h-9 px-3 text-md',
      },
    },
    defaultVariants: { size: 'md' },
  },
);

export interface InputProps
  extends Omit<InputHTMLAttributes<HTMLInputElement>, 'size'>,
    VariantProps<typeof input> {
  /** React 19 passes ref as a normal prop; the interface has to admit it. */
  ref?: Ref<HTMLInputElement>;
}

export function Input({ className, size, ...props }: InputProps) {
  return <input className={cn(input({ size }), className)} {...props} />;
}
