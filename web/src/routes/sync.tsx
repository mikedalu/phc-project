import { createRoute, Link } from '@tanstack/react-router';
import { Route as rootRoute } from '@/routes/__root';
import { useSyncState } from '@/sync/status';
import { useOnline } from '@/sync/status';

/**
 * Sync status.
 *
 * Written for the person using it, not for the person who built it. Every line
 * answers "is my work safe?" and nothing on the page mentions sprints, buffers or
 * queues as if the reader knows what those mean.
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
              ['Status', friendlyStatus(status, online)],
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
        <h2 className="text-sm font-semibold text-text">What these numbers mean</h2>
        <ul className="mt-2 space-y-1.5 text-sm text-text-subtle">
          <li>
            <span className="font-medium text-text">Queued operations</span> — records saved on
            this computer that have not reached the server yet. They send themselves; nothing is
            lost if you close the browser.
          </li>
          <li>
            <span className="font-medium text-text">Unreviewed conflicts</span> — another
            workstation saved a different version of the same record at the same time. Both
            versions are kept. An administrator can review them under Activity.
          </li>
          <li>
            <span className="font-medium text-text">Offline</span> — work continues normally.
            Only when the offline window closes must you sign in again to carry on.
          </li>
        </ul>
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


/**
 * Turns the engine's internal state into a sentence a clinician can act on.
 *
 * "error" is not a status a nurse can do anything with; "Could not reach the
 * server — your work is saved" is.
 */
function friendlyStatus(status: string, online: boolean): string {
  switch (status) {
    case 'synced':
      return 'Everything is saved on the server';
    case 'syncing':
      return 'Sending your work…';
    case 'pending':
      return 'Saved here, waiting to send';
    case 'offline':
      return online ? 'Saved here, waiting to send' : 'No connection — work continues';
    case 'error':
      return 'Could not reach the server — your work is saved';
    case 'blocked':
      return 'Signed out of the server — sign in again to send';
    default:
      return status;
  }
}
