import { useState } from 'react';
import { createRoute } from '@tanstack/react-router';
import { useLiveQuery } from 'dexie-react-hooks';
import { RefreshCw, Trash2 } from 'lucide-react';
import { Route as rootRoute } from '@/routes/__root';
import { db } from '@/db';
import { writeProbe, localDelete } from '@/db/write';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { runSync } from '@/sync/engine';
import { useSyncState } from '@/sync/status';
import { cn } from '@/lib/cn';

/**
 * The sync probe.
 *
 * A development screen with one job: make replication VISIBLE. Two browsers
 * write rows here and watch each other's arrive. Without a screen like this, the
 * only way to see whether the engine works is to read IndexedDB, which is not a
 * demo and not a debugging tool.
 *
 * It writes through `localWrite`, exactly like a clinical screen will — so what
 * is proven here is proven for `patients`.
 */
export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/dev/sync',
  component: SyncProbePage,
});

function SyncProbePage() {
  const [title, setTitle] = useState('');
  const [busy, setBusy] = useState(false);
  const sync = useSyncState();

  const rows = useLiveQuery(
    () => db.sync_probe.orderBy('updatedAt').reverse().toArray(),
    [],
    [],
  );

  const conflicts = useLiveQuery(() => db.conflicts.toArray(), [], []);

  const add = async () => {
    if (!title.trim()) return;
    setBusy(true);
    try {
      await writeProbe({ title: title.trim(), body: null });
      setTitle('');
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="space-y-5">
      <header>
        <h1 className="text-2xl font-semibold tracking-tight text-text">Sync probe</h1>
        <p className="mt-1 text-sm text-text-muted">
          Development screen — not clinical data. Every row here goes through the same{' '}
          <code className="font-mono text-xs">localWrite</code> path a patient record will.
        </p>
      </header>

      <section className="rounded-md border border-border bg-surface p-4">
        <div className="flex flex-wrap items-center gap-3">
          <div className="flex flex-1 items-center gap-2">
            <Input
              value={title}
              onChange={(event) => setTitle(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === 'Enter') void add();
              }}
              placeholder="Say something, then watch it on the other device"
              aria-label="Probe title"
            />
            <Button onClick={() => void add()} disabled={busy || !title.trim()}>
              Add row
            </Button>
          </div>

          <Button variant="secondary" onClick={() => void runSync()}>
            <RefreshCw aria-hidden="true" className={cn('size-4', sync.status === 'syncing' && 'animate-spin')} />
            Sync now
          </Button>
        </div>

        <p className="mt-3 text-xs text-text-muted">
          Status: <span className="font-medium text-text-subtle">{sync.status}</span>
          {' · '}
          {sync.pendingCount} waiting
          {sync.conflictCount > 0 ? ` · ${sync.conflictCount} conflict(s)` : ''}
        </p>
      </section>

      <section className="overflow-hidden rounded-md border border-border bg-surface">
        <table className="w-full border-collapse text-left">
          <caption className="sr-only">Probe rows on this device</caption>
          <thead>
            <tr className="border-b border-border bg-surface-sunken text-xs uppercase tracking-wide text-text-muted">
              <th scope="col" className="px-4 py-2">Title</th>
              <th scope="col" className="px-4 py-2">State</th>
              <th scope="col" className="px-4 py-2">Rev</th>
              <th scope="col" className="px-4 py-2 text-right">Actions</th>
            </tr>
          </thead>
          <tbody>
            {rows.length === 0 ? (
              <tr>
                <td colSpan={4} className="px-4 py-8 text-center text-sm text-text-muted">
                  Nothing yet. Add a row above.
                </td>
              </tr>
            ) : null}
            {rows.map((row) => (
              <tr key={row.id} className="border-b border-border last:border-b-0">
                <td className="px-4 py-2 text-sm text-text">
                  {row.title}
                  {row.deletedAt ? (
                    <span className="ml-2 text-xs text-text-muted">(deleted)</span>
                  ) : null}
                </td>
                <td className="px-4 py-2">
                  <span
                    className={cn(
                      'rounded-xs px-1.5 py-0.5 text-xs font-medium',
                      row.syncState === 'synced' && 'bg-success-soft text-success-fg',
                      row.syncState === 'pending' && 'bg-primary-soft text-primary-fg',
                      row.syncState === 'conflict' && 'bg-danger-soft text-danger-fg',
                    )}
                  >
                    {row.syncState}
                  </span>
                </td>
                <td className="px-4 py-2 font-mono text-xs text-text-muted">{row.serverRev}</td>
                <td className="px-4 py-2 text-right">
                  <Button
                    variant="ghost"
                    size="sm"
                    aria-label={`Delete ${row.title}`}
                    onClick={() => void localDelete('sync_probe', row.id)}
                  >
                    <Trash2 aria-hidden="true" className="size-3.5" />
                  </Button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>

      {conflicts.length > 0 ? (
        <section className="rounded-md border border-warning/30 bg-warning-soft p-4">
          <h2 className="text-sm font-medium text-warning-fg">
            {conflicts.length} conflict{conflicts.length === 1 ? '' : 's'} recorded
          </h2>
          <ul className="mt-2 space-y-1">
            {conflicts.map((c) => (
              <li key={c.id} className="text-xs text-text-subtle">
                <span className="font-medium">{c.conflictType}</span> on {c.entityTable} · {c.reason}
              </li>
            ))}
          </ul>
        </section>
      ) : null}
    </div>
  );
}
