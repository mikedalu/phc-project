import { createRoute } from '@tanstack/react-router';
import { useQuery } from '@tanstack/react-query';
import { Route as rootRoute } from '@/routes/__root';
import { apiGet } from '@/lib/api';

/**
 * The audit view.
 *
 * TanStack Query, not Dexie: this is an admin screen over POSTGRES, and the
 * change log is server truth rather than something the device replicates. A
 * cached local copy would be a second version of the record.
 */
export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/admin/activity',
  component: ActivityPage,
});

interface ActivityEntry {
  seq: number;
  table: string;
  rowId: string;
  operation: string;
  serverRev: number;
  actorName: string;
  actorStaff: string;
  changedAt: string;
}

interface ConflictEntry {
  id: string;
  table: string;
  rowId: string;
  resolution: string;
  createdAt: string;
}

function ActivityPage() {
  const { data, isLoading, error } = useQuery({
    queryKey: ['admin', 'activity'],
    queryFn: () => apiGet<{ activity: ActivityEntry[]; conflicts: ConflictEntry[] }>('/admin/activity?limit=50'),
    refetchInterval: 30_000,
  });

  const activity = data?.data.activity ?? [];
  const conflicts = data?.data.conflicts ?? [];

  return (
    <div className="space-y-5">
      <header>
        <h1 className="text-2xl font-semibold tracking-tight text-text">Activity</h1>
        <p className="mt-1 text-sm text-text-muted">
          Every write to every replicated table, newest first. Written by a
          database trigger, so nothing can be missed by forgetting to log it.
        </p>
      </header>

      {isLoading ? <p className="text-sm text-text-muted">Loading…</p> : null}
      {error ? (
        <p role="alert" className="text-sm text-danger-fg">
          {error instanceof Error ? error.message : 'Could not read the activity log.'}
        </p>
      ) : null}

      <section className="overflow-hidden rounded-md border border-border bg-surface">
        <h2 className="border-b border-border bg-surface-sunken px-3 py-2 text-sm font-medium text-text">
          Recent changes ({activity.length})
        </h2>
        <table className="w-full border-collapse text-left text-sm">
          <caption className="sr-only">Recent replicated changes</caption>
          <thead>
            <tr className="border-b border-border text-xs uppercase tracking-wide text-text-muted">
              <th scope="col" className="px-3 py-2">When</th>
              <th scope="col" className="px-3 py-2">Who</th>
              <th scope="col" className="px-3 py-2">Table</th>
              <th scope="col" className="px-3 py-2">Operation</th>
              <th scope="col" className="px-3 py-2">Rev</th>
            </tr>
          </thead>
          <tbody>
            {activity.length === 0 ? (
              <tr>
                <td colSpan={5} className="px-3 py-8 text-center text-text-muted">
                  Nothing recorded yet.
                </td>
              </tr>
            ) : null}
            {activity.map((entry) => (
              <tr key={entry.seq} className="border-b border-border last:border-b-0">
                <td className="px-3 py-2 text-xs text-text-muted">
                  {new Date(entry.changedAt).toLocaleString()}
                </td>
                <td className="px-3 py-2">
                  <span className="text-text">{entry.actorName}</span>
                  <span className="ml-1 font-mono text-xs text-text-muted">{entry.actorStaff}</span>
                </td>
                <td className="px-3 py-2 font-mono text-xs text-text-subtle">{entry.table}</td>
                <td className="px-3 py-2 text-xs text-text-subtle">{entry.operation}</td>
                <td className="px-3 py-2 tnum text-xs text-text-muted">{entry.serverRev}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>

      <section className="rounded-md border border-border bg-surface">
        <h2 className="border-b border-border bg-surface-sunken px-3 py-2 text-sm font-medium text-text">
          Conflicts recorded ({conflicts.length})
        </h2>
        <p className="px-3 pt-2 text-xs text-text-muted">
          Each row is a last-write-wins decision where two devices disagreed. Both
          payloads are stored; the losing copy is never silently discarded.
        </p>
        <div className="divide-y divide-border">
          {conflicts.length === 0 ? (
            <p className="px-3 py-6 text-center text-xs text-text-muted">No conflicts.</p>
          ) : null}
          {conflicts.map((c) => (
            <div key={c.id} className="flex items-center justify-between px-3 py-2 text-xs">
              <span className="font-mono text-text-subtle">
                {c.table} · {c.rowId.slice(0, 8)}
              </span>
              <span className="text-text-muted">
                {new Date(c.createdAt).toLocaleString()} · {c.resolution}
              </span>
            </div>
          ))}
        </div>
      </section>
    </div>
  );
}
