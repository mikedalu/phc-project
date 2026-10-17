import { useState } from 'react';
import { createRoute, Link } from '@tanstack/react-router';
import { useLiveQuery } from 'dexie-react-hooks';
import { Search } from 'lucide-react';
import { Route as rootRoute } from '@/routes/__root';
import { db } from '@/db';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/cn';

/**
 * Household list.
 *
 * Local-first like every clinical list: the families are already on the device.
 * Member counts come from one pass over the patients table rather than a query
 * per row — a per-row count on a list of a few hundred is the sort of thing that
 * is invisible until a clinic has been running for a year.
 */
export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/households',
  component: HouseholdListPage,
});

function HouseholdListPage() {
  const [term, setTerm] = useState('');

  const households = useLiveQuery(() => db.households.orderBy('updatedAt').reverse().toArray(), [], []);
  const patients = useLiveQuery(() => db.patients.toArray(), [], []);

  const counts = new Map<string, number>();
  for (const patient of patients) {
    if (!patient.householdId) continue;
    counts.set(patient.householdId, (counts.get(patient.householdId) ?? 0) + 1);
  }

  const needle = term.trim().toLowerCase();
  const visible = households.filter(
    (h) =>
      !needle ||
      h.familyName.toLowerCase().includes(needle) ||
      h.familyNumber.toLowerCase().includes(needle) ||
      (h.phone ?? '').includes(needle),
  );

  return (
    <div className="space-y-5">
      <header>
        <h1 className="text-2xl font-semibold tracking-tight text-text">Households</h1>
        <p className="mt-1 text-sm text-text-muted">
          {households.length} famil{households.length === 1 ? 'y' : 'ies'} on this device
        </p>
      </header>

      <div className="relative max-w-md">
        <Search aria-hidden="true" className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-text-muted" />
        <Input
          value={term}
          onChange={(event) => setTerm(event.target.value)}
          placeholder="Search family name or number…"
          aria-label="Search households"
          className="pl-9"
        />
      </div>

      <div className="overflow-hidden rounded-md border border-border bg-surface">
        <table className="w-full border-collapse text-left">
          <caption className="sr-only">Households registered on this device</caption>
          <thead>
            <tr className="border-b border-border bg-surface-sunken text-xs uppercase tracking-wide text-text-muted">
              <th scope="col" className="px-4 py-2">Family</th>
              <th scope="col" className="px-4 py-2">Members</th>
              <th scope="col" className="px-4 py-2">Phone</th>
              <th scope="col" className="px-4 py-2">State</th>
            </tr>
          </thead>
          <tbody>
            {visible.length === 0 ? (
              <tr>
                <td colSpan={4} className="px-4 py-10 text-center text-sm text-text-muted">
                  {households.length === 0
                    ? 'No families registered yet. Register one from the Patients screen.'
                    : 'No family matches that search.'}
                </td>
              </tr>
            ) : null}

            {visible.map((household) => (
              <tr key={household.id} className="border-b border-border last:border-b-0 hover:bg-surface-sunken">
                <td className="px-4 py-2.5">
                  <Link
                    to="/households/$id"
                    params={{ id: household.id }}
                    className="text-sm font-medium text-primary hover:underline"
                  >
                    {household.familyName}
                  </Link>
                  <p className="font-mono text-xs text-text-muted">{household.familyNumber}</p>
                </td>
                <td className="px-4 py-2.5 text-sm text-text-subtle">
                  {counts.get(household.id) ?? 0}
                </td>
                <td className="px-4 py-2.5 font-mono text-xs text-text-subtle">
                  {household.phone ?? '—'}
                </td>
                <td className="px-4 py-2.5">
                  <span
                    className={cn(
                      'rounded-xs px-1.5 py-0.5 text-xs font-medium',
                      household.syncState === 'synced' && 'bg-success-soft text-success-fg',
                      household.syncState === 'pending' && 'bg-primary-soft text-primary-fg',
                      household.syncState === 'conflict' && 'bg-danger-soft text-danger-fg',
                    )}
                  >
                    {household.syncState}
                  </span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
