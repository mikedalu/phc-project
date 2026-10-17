import { createRoute, Link } from '@tanstack/react-router';
import { useLiveQuery } from 'dexie-react-hooks';
import { ArrowLeft, Crown } from 'lucide-react';
import { Route as rootRoute } from '@/routes/__root';
import { db } from '@/db';

/**
 * Household view.
 *
 * The billing party: every member, who the head is, and the balance. The balance
 * is DERIVED, never stored — it is 0 until Sprint 6 writes invoices, and the
 * screen says so rather than pretending the number is trustworthy.
 */
export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/households/$id',
  component: HouseholdPage,
});

function HouseholdPage() {
  const { id } = Route.useParams();

  const household = useLiveQuery(() => db.households.get(id), [id]);
  const members = useLiveQuery(
    () => db.patients.where('householdId').equals(id).toArray(),
    [id],
    [],
  );

  if (household === undefined) {
    return <p className="text-sm text-text-muted">Loading…</p>;
  }
  if (household === null) {
    return (
      <div className="space-y-3">
        <p className="text-sm text-text">No household with that id on this device.</p>
        <Link to="/patients" className="text-sm text-primary hover:underline">
          Back to patients
        </Link>
      </div>
    );
  }

  return (
    <div className="space-y-5">
      <Link to="/patients" className="inline-flex items-center gap-1.5 text-sm text-text-muted hover:text-text">
        <ArrowLeft aria-hidden="true" className="size-3.5" />
        Patients
      </Link>

      <header className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-text">
            {household.familyName} family
          </h1>
          <p className="mt-1 font-mono text-sm text-text-muted">{household.familyNumber}</p>
          {household.phone ? (
            <p className="mt-1 text-sm text-text-subtle">{household.phone}</p>
          ) : null}
          {household.address ? (
            <p className="text-sm text-text-subtle">{household.address}</p>
          ) : null}
        </div>
        <span
          className={
            household.syncState === 'synced'
              ? 'rounded-xs bg-success-soft px-2 py-0.5 text-xs font-medium text-success-fg'
              : 'rounded-xs bg-primary-soft px-2 py-0.5 text-xs font-medium text-primary-fg'
          }
        >
          {household.syncState === 'synced' ? 'Synced to server' : 'Saved on this device'}
        </span>
      </header>

      <section className="overflow-hidden rounded-md border border-border bg-surface">
        <table className="w-full border-collapse text-left">
          <caption className="sr-only">Members of this household</caption>
          <thead>
            <tr className="border-b border-border bg-surface-sunken text-xs uppercase tracking-wide text-text-muted">
              <th scope="col" className="px-4 py-2">Member</th>
              <th scope="col" className="px-4 py-2">Age</th>
              <th scope="col" className="px-4 py-2">Phone</th>
            </tr>
          </thead>
          <tbody>
            {members.length === 0 ? (
              <tr>
                <td colSpan={3} className="px-4 py-8 text-center text-sm text-text-muted">
                  No members on this device yet.
                </td>
              </tr>
            ) : null}
            {members.map((patient) => {
              const isHead = household.headPatientId === patient.id;
              return (
                <tr key={patient.id} className="border-b border-border last:border-b-0">
                  <td className="px-4 py-2.5">
                    <Link
                      to="/patients/$id"
                      params={{ id: patient.id }}
                      className="inline-flex items-center gap-1.5 text-sm font-medium text-primary hover:underline"
                    >
                      {isHead ? <Crown aria-hidden="true" className="size-3.5 text-warning" /> : null}
                      {patient.firstName} {patient.surname}
                    </Link>
                    <p className="font-mono text-xs text-text-muted">{patient.patientNumber}</p>
                  </td>
                  <td className="px-4 py-2.5 text-sm text-text-subtle">
                    {patient.ageYears ?? '—'} yr
                  </td>
                  <td className="px-4 py-2.5 font-mono text-xs text-text-subtle">
                    {patient.phone ?? '—'}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </section>

      <section className="rounded-md border border-border bg-surface p-4">
        <h2 className="text-sm font-medium text-text">Family balance</h2>
        <p className="mt-1 text-2xl font-semibold tnum text-text">₦0.00</p>
        <p className="mt-1 text-xs text-text-muted">
          Derived from invoices and payments, never stored. Billing arrives in Sprint 6, so this is
          zero for now rather than a figure to trust.
        </p>
      </section>
    </div>
  );
}
