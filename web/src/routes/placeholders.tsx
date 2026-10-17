import { createRoute } from '@tanstack/react-router';
import { Route as rootRoute } from '@/routes/__root';
import { Construction } from 'lucide-react';

/**
 * Placeholder screens.
 *
 * These exist so the shell's navigation does not 404 while Sprint 0 is in
 * progress. Each one states plainly what it will become and which sprint
 * delivers it — a stub that pretends to be a feature is worse than an honest
 * placeholder, and this file is deleted as the real screens land.
 */

interface StubProps {
  title: string;
  sprint: string;
  description: string;
}

function Stub({ title, sprint, description }: StubProps) {
  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-semibold tracking-tight text-text">{title}</h1>

      <section className="flex items-start gap-3 rounded-md border border-border bg-surface p-4 shadow-sm">
        <span className="mt-0.5 grid size-8 shrink-0 place-items-center rounded-sm bg-warning-soft text-warning-fg">
          <Construction aria-hidden="true" className="size-4" />
        </span>
        <div>
          <p className="text-sm font-medium text-text">
            Not built yet — planned for {sprint}
          </p>
          <p className="mt-1 max-w-2xl text-sm text-text-subtle">{description}</p>
        </div>
      </section>
    </div>
  );
}

export const QueueRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/queue',
  component: () => (
    <Stub
      title="Queue"
      sprint="Sprint 4"
      description="The triage board: encounters by workflow stage, vitals capture, and the front-desk → nurse → doctor handoffs. The board is derived from encounters rather than a separate queue table, so there is exactly one row to move a patient forward."
    />
  ),
});
