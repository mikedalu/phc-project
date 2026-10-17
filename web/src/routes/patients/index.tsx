import { useEffect, useMemo, useRef, useState } from 'react';
import { createRoute, Link, useNavigate } from '@tanstack/react-router';
import { useLiveQuery } from 'dexie-react-hooks';
import { Plus, Search } from 'lucide-react';
import { Route as rootRoute } from '@/routes/__root';
import { db } from '@/db';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/cn';

/**
 * The patient list.
 *
 * Reads DEXIE, not the server. Every registered patient is already on the
 * device — that is the architecture — so a list that waits for a network round
 * trip would be slower and would simply fail offline.
 *
 * Ctrl+K is a global habit from the rest of the app: it focuses the search box
 * from anywhere on the screen, so a clerk never reaches for the mouse.
 */
export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/patients',
  component: PatientListPage,
});

function PatientListPage() {
  const navigate = useNavigate();
  const [term, setTerm] = useState('');
  const searchRef = useRef<HTMLInputElement>(null);

  const patients = useLiveQuery(
    () => db.patients.orderBy('updatedAt').reverse().toArray(),
    [],
    [],
  );

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault();
        searchRef.current?.focus();
        searchRef.current?.select();
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, []);

  const visible = useMemo(() => {
    const needle = term.trim().toLowerCase();
    if (!needle) return patients.slice(0, 200);

    return patients
      .filter((p) => {
        const haystack = [
          p.firstName,
          p.middleName ?? '',
          p.surname,
          p.patientNumber,
          p.phone ?? '',
        ]
          .join(' ')
          .toLowerCase();
        return haystack.includes(needle);
      })
      .slice(0, 200);
  }, [patients, term]);

  return (
    <div className="space-y-5">
      <header className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-text">Patients</h1>
          <p className="mt-1 text-sm text-text-muted">
            {patients.length} registered on this device
            {term ? ` · ${visible.length} matching` : ''}
          </p>
        </div>
        <Button onClick={() => void navigate({ to: '/patients/new' })}>
          <Plus aria-hidden="true" className="size-4" />
          Register patient
        </Button>
      </header>

      <div className="relative">
        <Search
          aria-hidden="true"
          className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-text-muted"
        />
        <Input
          ref={searchRef}
          value={term}
          onChange={(event) => setTerm(event.target.value)}
          placeholder="Search name, number or phone… (Ctrl+K)"
          aria-label="Search patients"
          className="pl-9"
        />
      </div>

      <div className="overflow-hidden rounded-md border border-border bg-surface">
        <table className="w-full border-collapse text-left">
          <caption className="sr-only">Patients registered on this device</caption>
          <thead>
            <tr className="border-b border-border bg-surface-sunken text-xs uppercase tracking-wide text-text-muted">
              <th scope="col" className="px-4 py-2">Patient</th>
              <th scope="col" className="px-4 py-2">Age / sex</th>
              <th scope="col" className="px-4 py-2">Phone</th>
              <th scope="col" className="px-4 py-2">State</th>
            </tr>
          </thead>
          <tbody>
            {visible.length === 0 ? (
              <tr>
                <td colSpan={4} className="px-4 py-10 text-center text-sm text-text-muted">
                  {patients.length === 0
                    ? 'No patients registered yet. Use “Register patient” to add the first one.'
                    : 'No patient matches that search.'}
                </td>
              </tr>
            ) : null}

            {visible.map((patient) => (
              <tr key={patient.id} className="border-b border-border last:border-b-0 hover:bg-surface-sunken">
                <td className="px-4 py-2.5">
                  <Link
                    to="/patients/$id"
                    params={{ id: patient.id }}
                    className="text-sm font-medium text-primary hover:underline"
                  >
                    {patient.firstName} {patient.surname}
                  </Link>
                  <p className="font-mono text-xs text-text-muted">{patient.patientNumber}</p>
                </td>
                <td className="px-4 py-2.5 text-sm text-text-subtle">
                  {formatAge(patient.ageYears, patient.ageMonths)} · {patient.gender}
                </td>
                <td className="px-4 py-2.5 font-mono text-xs text-text-subtle">
                  {patient.phone ?? '—'}
                </td>
                <td className="px-4 py-2.5">
                  <span
                    className={cn(
                      'rounded-xs px-1.5 py-0.5 text-xs font-medium',
                      patient.syncState === 'synced' && 'bg-success-soft text-success-fg',
                      patient.syncState === 'pending' && 'bg-primary-soft text-primary-fg',
                      patient.syncState === 'conflict' && 'bg-danger-soft text-danger-fg',
                    )}
                  >
                    {patient.syncState}
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

function formatAge(years: number | null, months: number | null): string {
  if (months !== null && months < 24) return `${months} mo`;
  if (years !== null) return `${years} yr`;
  return '—';
}
