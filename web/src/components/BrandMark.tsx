import { Cross } from 'lucide-react';
import { cn } from '@/lib/cn';

/**
 * The Spring Care glyph: a medical cross on the deep brand green.
 *
 * Drawn with a Lucide icon rather than an image so it inherits the current
 * colour, stays crisp on any display, and adds no request to the offline shell.
 */
export function BrandMark({ className }: { className?: string }) {
  return (
    <span
      aria-hidden="true"
      className={cn(
        'relative grid size-9 shrink-0 place-items-center overflow-hidden rounded-md',
        'bg-spring-900 text-white shadow-subtle',
        className,
      )}
    >
      <Cross className="size-5" strokeWidth={2.25} />
      <span className="absolute -right-1 -bottom-1 size-3 rounded-full bg-spring-400/80 blur-[2px]" />
    </span>
  );
}
