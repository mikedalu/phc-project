import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

/**
 * Joins class names and resolves Tailwind conflicts.
 *
 * `twMerge` means the LAST conflicting utility wins:
 *   cn('p-4', 'p-6')            → 'p-6'
 *   cn('text-danger', props.className)
 *
 * Without it, conditional classes depend on stylesheet order rather than
 * intention, which produces bugs that only appear in some components.
 */
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs));
}
