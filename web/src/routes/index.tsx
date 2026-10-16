import { useEffect, useState } from 'react';
import { Link, createRoute, useRouter } from '@tanstack/react-router';
import { Check, Database, Loader2, Server, Smartphone, X } from 'lucide-react';
import { Route as rootRoute } from '@/routes/__root';
import { db, openDatabase } from '@/db';
import { fetchHealth, fetchMeta, fetchReady, type MetaResponse } from '@/lib/api';
import { cn } from '@/lib/cn';

/**
 * The walking skeleton.
 *
 * This page exists to answer ONE question: can a request travel
 * browser → Cloudflare Pages → Cloud Run → Neon → browser?
 *
 * It deliberately does more than ping /health. /health reads nothing, so a
 * green tick there proves only that the process is alive. The chain is only
 * proven by `/meta`, which runs a real SQL query (`count(*) FROM facilities`)
 * and therefore touches pgx, the pooler, and Neon.
 */

type CheckState =
  | { kind: 'loading' }
  | { kind: 'ok'; detail: string }
  | { kind: 'error'; detail: string };

function useHealthCheck<T>(
  label: string,
  run: () => Promise<T>,
  describe: (value: T) => string,
): [string, CheckState] {
  const [state, setState] = useState<CheckState>({ kind: 'loading' });

  useEffect(() => {
    let cancelled = false;

    run()
      .then((value) => {
        if (!cancelled) setState({ kind: 'ok', detail: describe(value) });
      })
      .catch((error: unknown) => {
        if (!cancelled) {
          setState({
            kind: 'error',
            detail: error instanceof Error ? error.message : 'Unknown error',
          });
        }
      });

    // Guards against a state update after unmount, which React 19 warns about
    // and which produces a confusing console error far from its cause.
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return [label, state];
}

function StatusRow({
  label,
  state,
  icon: Icon,
}: {
  label: string;
  state: CheckState;
  icon: typeof Server;
}) {
  return (
    <li className="flex items-start gap-3 border-b border-border px-4 py-3 last:border-b-0">
      <span
        className={cn(
          'mt-0.5 grid size-6 shrink-0 place-items-center rounded-sm',
          state.kind === 'ok' && 'bg-success-soft text-success-fg',
          state.kind === 'error' && 'bg-danger-soft text-danger-fg',
          state.kind === 'loading' && 'bg-surface-sunken text-text-muted',
        )}
      >
        {state.kind === 'loading' ? (
          <Loader2 aria-hidden="true" className="size-3.5 motion-safe:animate-spin" />
        ) : state.kind === 'ok' ? (
          <Check aria-hidden="true" className="size-3.5" />
        ) : (
          <X aria-hidden="true" className="size-3.5" />
        )}
      </span>

      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-1.5">
          <Icon aria-hidden="true" className="size-3.5 text-text-muted" />
          <p className="text-sm font-medium text-text">{label}</p>
        </div>
        <p
          className={cn(
            'mt-0.5 break-words text-xs',
            state.kind === 'error' ? 'text-danger' : 'text-text-muted',
          )}
        >
          {state.kind === 'loading' ? 'Checking…' : state.detail}
        </p>
      </div>
    </li>
  );
}

function DashboardPage() {
  const router = useRouter();

  const [metaLabel, metaState] = useHealthCheck(
    'API + database round trip (/meta)',
    () => fetchMeta(),
    (result) =>
      `schema v${result.data.schemaVersion} · ${result.data.facilityCount} facility row · api ${result.data.serverVersion}`,
  );

  const [readyLabel, readyState] = useHealthCheck(
    'Database readiness (/ready)',
    () => fetchReady(),
    (result) => `database ${result.data.database}`,
  );

  const [healthLabel, healthState] = useHealthCheck(
    'API liveness (/health)',
    () => fetchHealth(),
    (result) => `version ${result.data.version} · server time ${result.data.time}`,
  );

  const [localLabel, localState] = useHealthCheck(
    'Local database (Dexie / IndexedDB)',
    async () => {
      await openDatabase();
      const tables = db.tables.map((t) => t.name);
      const outbox = await db.outbox.count();
      return { tables, outbox };
    },
    ({ tables, outbox }) =>
      `${tables.length} tables ready · ${outbox} queued operation${outbox === 1 ? '' : 's'}`,
  );

  const [meta, setMeta] = useState<MetaResponse | null>(null);
  useEffect(() => {
    fetchMeta()
      .then((r) => setMeta(r.data))
      .catch(() => setMeta(null));
  }, []);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight text-text">
          Sprint 0 — walking skeleton
        </h1>
        <p className="mt-1 max-w-2xl text-sm text-text-subtle">
          This page proves the whole chain works before any clinical feature is
          written: browser → Cloudflare Pages → Cloud Run → Neon → browser. When
          all four checks below are green, the deployment risk is retired.
        </p>
      </div>

      <section className="overflow-hidden rounded-md border border-border bg-surface shadow-sm">
        <ul>
          <StatusRow label={metaLabel} state={metaState} icon={Database} />
          <StatusRow label={readyLabel} state={readyState} icon={Server} />
          <StatusRow label={healthLabel} state={healthState} icon={Server} />
          <StatusRow label={localLabel} state={localState} icon={Smartphone} />
        </ul>
      </section>

      {meta && (
        <section className="rounded-md border border-border bg-surface p-4 shadow-sm">
          <h2 className="text-sm font-semibold text-text">
            Server configuration (GET /meta)
          </h2>
          <dl className="mt-3 grid grid-cols-2 gap-x-6 gap-y-2 sm:grid-cols-3">
            {(
              [
                ['Schema version', String(meta.schemaVersion)],
                ['Server version', meta.serverVersion],
                ['Min client version', meta.minClientVersion],
                ['Sync batch size', String(meta.syncMaxBatchSize)],
                ['Referral TTL', `${meta.referralTtlHours}h`],
                ['Offline grace', `${meta.offlineGracePeriodHours}h`],
                ['Offline policy', meta.offlinePolicy],
                ['Server time', new Date(meta.serverTime).toLocaleTimeString()],
              ] as const
            ).map(([term, value]) => (
              <div key={term}>
                <dt className="text-xs text-text-muted">{term}</dt>
                <dd className="tnum text-sm font-medium text-text">{value}</dd>
              </div>
            ))}
          </dl>
        </section>
      )}

      <section className="rounded-md border border-border bg-surface p-4 shadow-sm">
        <h2 className="text-sm font-semibold text-text">Next</h2>
        <p className="mt-1 text-sm text-text-subtle">
          Sprint 1 adds authentication, roles and device identity. The{' '}
          <Link to="/sync" className="font-medium text-primary underline">
            sync status
          </Link>{' '}
          page currently reports an empty outbox, which is the truth — nothing
          can be queued until Sprint 2 builds the sync engine.
        </p>
        <button
          type="button"
          onClick={() => void router.invalidate()}
          className={cn(
            'mt-3 inline-flex h-(--spacing-control) items-center rounded-sm',
            'bg-primary px-3 text-sm font-medium text-text-inverse',
            'transition-colors duration-100 hover:bg-primary-hover',
          )}
        >
          Re-run checks
        </button>
      </section>
    </div>
  );
}

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  component: DashboardPage,
});
