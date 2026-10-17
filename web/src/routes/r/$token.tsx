import { createRoute } from '@tanstack/react-router';
import { Route as rootRoute } from '@/routes/__root';
import { useQuery } from '@tanstack/react-query';
import { readReferral } from '@/features/referrals/api';

/**
 * The PUBLIC referral view.
 *
 * TanStack Query is the right tool here and nowhere else in the clinical app:
 * this page is a stranger's browser, there is no local database, and the note is
 * a single read that must not be cached across patients.
 */
export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/r/$token',
  component: PublicReferralPage,
});

function PublicReferralPage() {
  const { token } = Route.useParams();
  const { data, isLoading, error } = useQuery({
    queryKey: ['referral', token],
    queryFn: () => readReferral(token),
    retry: false,
    staleTime: 0,
    gcTime: 0,
  });

  return (
    <main className="mx-auto min-h-screen max-w-2xl bg-surface-sunken px-4 py-8">
      <header className="mb-6 text-center">
        <p className="text-sm font-semibold tracking-tight text-text">Referral note</p>
        <p className="text-xs text-text-muted">Scanned referral · verify the code before acting</p>
      </header>

      {isLoading ? <p className="text-center text-sm text-text-muted">Checking the code…</p> : null}

      {error ? (
        <div role="alert" className="rounded-md border border-danger/30 bg-danger-soft p-4 text-sm text-danger-fg">
          {error instanceof Error ? error.message : 'This referral code could not be read.'}
        </div>
      ) : null}

      {data ? (
        <article className="space-y-4 rounded-md border border-border bg-surface p-6">
          <div className="flex items-start justify-between gap-3">
            <div>
              <h1 className="text-xl font-semibold text-text">{data.name}</h1>
              <p className="text-sm text-text-muted">
                {data.age ? `${data.age} years` : 'Age not given'}
                {data.sex ? ` · ${data.sex}` : ''}
              </p>
            </div>
            <span
              className={
                data.urgency === 'routine'
                  ? 'rounded-xs bg-surface-sunken px-2 py-0.5 text-xs text-text-muted'
                  : 'rounded-xs bg-danger px-2 py-0.5 text-xs font-medium text-text-inverse'
              }
            >
              {data.urgency}
            </span>
          </div>

          <dl className="grid gap-3 text-sm">
            <Row label="Referred from" value={data.from} />
            <Row label="Referred to" value={data.to} />
            <Row label="Reason" value={data.reason} />
            {data.dx ? <Row label="Diagnosis" value={data.dx} /> : null}
            {data.tx ? <Row label="Treatment given" value={data.tx} /> : null}
            <Row label="Referral number" value={data.ref} mono />
            <Row label="Valid until" value={new Date(data.exp).toLocaleString()} />
          </dl>

          <p className="border-t border-border pt-3 text-xs text-text-muted">
            This note contains no patient identifier beyond a name. Do not record it anywhere it
            could be joined to other data.
          </p>
        </article>
      ) : null}
    </main>
  );
}

function Row({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="grid grid-cols-[10rem_1fr] gap-2">
      <dt className="text-text-muted">{label}</dt>
      <dd className={mono ? 'font-mono text-xs text-text' : 'text-text'}>{value}</dd>
    </div>
  );
}
