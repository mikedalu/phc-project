import { createRoute, Link } from '@tanstack/react-router';
import { Route as rootRoute } from '@/routes/__root';
import { useSyncState } from '@/sync/status';
import { useOnline } from '@/sync/status';

/**
 * Sync status.
 *
 * Sprint 0 shows the truth about an empty outbox. The per-table breakdown,
 * the conflict list and the manual "Sync now" action arrive with the engine in
 * Sprint 2 — this page is the shell they will live in.
 */
function SyncPage() {
  const online = useOnline();
  const { status, pendingCount, conflictCount } = useSyncState();

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight text-text">
          Sync status
        </h1>
        <p className="mt-1 max-w-2xl text-sm text-text-subtle">
          Every clinical record is written to this device first and pushed to the
          server in the background. Nothing on this page blocks data entry.
        </p>
      </div>

      <section className="overflow-hidden rounded-md border border-border bg-surface shadow-sm">
        <dl className="divide-y divide-border">
          {(
            [
              ['Connection', online ? 'Online' : 'Offline'],
              ['Status', status],
              ['Queued operations', String(pendingCount)],
              ['Unreviewed conflicts', String(conflictCount)],
            ] as const
          ).map(([term, value]) => (
            <div key={term} className="flex items-center justify-between px-4 py-3">
              <dt className="text-sm text-text-subtle">{term}</dt>
              <dd className="tnum text-sm font-medium text-text">{value}</dd>
            </div>
          ))}
        </dl>
      </section>

      <section className="rounded-md border border-border bg-surface-sunken p-4">
        <h2 className="text-sm font-semibold text-text">Not built yet</h2>
        <ul className="mt-2 space-y-1 text-sm text-text-subtle">
          <li>· The outbox drain and the push/pull loop — Sprint 2</li>
          <li>· Offline grace countdown and re-authentication — Sprint 1</li>
          <li>· Conflict review for administrators — Sprint 2</li>
        </ul>
        <p className="mt-3 text-xs text-text-muted">
          The queued count above is read live from IndexedDB, so it will be
          accurate the moment the engine starts writing to it. Nothing here is
          mocked.
        </p>
      </section>

      <Link to="/" className="inline-block text-sm font-medium text-primary underline">
        ← Back to dashboard
      </Link>
    </div>
  );
}

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/sync',
  component: SyncPage,
});
