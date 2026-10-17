import { useId, type ReactNode } from 'react';
import { cn } from '@/lib/cn';

/**
 * Label + hint + error, with the ARIA wiring done once.
 *
 * ═══ WHY A RENDER PROP ═══
 *
 * The alternative is cloning the child element to inject `id`,
 * `aria-describedby` and `aria-invalid` — a trick that quietly breaks as soon as
 * the child is a wrapper or a fragment. Handing the ids to a render prop makes
 * the contract explicit: "here is what your input must carry", and the compiler
 * makes you take them.
 *
 * The ids are generated from `useId`, so two fields on one screen can never
 * collide, and the order is fixed: hint first, error second. A screen reader
 * announcing the error before the hint gives the user a problem with no context.
 */
export interface FieldWiring {
  id: string;
  /** Pass to `aria-describedby` on the input. Undefined when there is no hint or error. */
  describedBy: string | undefined;
  /** Pass to `aria-invalid` on the input. */
  invalid: true | undefined;
}

export interface FormFieldProps {
  label: string;
  hint?: string;
  error?: string | undefined;
  required?: boolean;
  className?: string;
  children: (wiring: FieldWiring) => ReactNode;
}

export function FormField({
  label,
  hint,
  error,
  required = false,
  className,
  children,
}: FormFieldProps) {
  const baseId = useId();
  const fieldId = `${baseId}-input`;
  const hintId = `${baseId}-hint`;
  const errorId = `${baseId}-error`;

  // Only describe what actually exists: pointing aria-describedby at a missing
  // element silently describes nothing, and a screen reader says nothing.
  const describedBy = [hint ? hintId : null, error ? errorId : null]
    .filter((v): v is string => v !== null)
    .join(' ');

  return (
    <div className={cn('flex flex-col gap-2.5', className)}>
      <label
        htmlFor={fieldId}
        className={cn(
          'text-base font-medium text-text',
          required && 'after:ml-0.5 after:text-danger after:content-["*"]',
        )}
      >
        {label}
      </label>

      {children({
        id: fieldId,
        describedBy: describedBy === '' ? undefined : describedBy,
        invalid: error ? true : undefined,
      })}

      {hint ? (
        <p id={hintId} className="text-sm text-text-muted">
          {hint}
        </p>
      ) : null}

      {error ? (
        <p id={errorId} role="alert" className="flex items-start gap-1.5 text-sm font-medium text-danger">
          {error}
        </p>
      ) : null}
    </div>
  );
}
