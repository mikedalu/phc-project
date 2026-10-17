import { createRoute, Link } from '@tanstack/react-router';
import { useLiveQuery } from 'dexie-react-hooks';
import { ArrowLeft } from 'lucide-react';
import { Route as rootRoute } from '@/routes/__root';
import { db } from '@/db';

/**
 * Patient profile.
 *
 * Local-first like the list: the record is on the device, so the screen renders
 * with no network. The history timeline arrives in Sprint 4 with encounters.
 */
export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/patients/$id',
  component: PatientProfilePage,
});

function PatientProfilePage() {
  const { id } = Route.useParams();

  const patient = useLiveQuery(() => db.patients.get(id), [id]);

  if (patient === undefined) {
    return <p className="text-sm text-text-muted">Loading…</p>;
  }
  if (patient === null) {
    return (
      <div className="space-y-3">
        <p className="text-sm text-text">No patient with that id on this device.</p>
        <Link to="/patients" className="text-sm text-primary hover:underline">
          Back to patients
        </Link>
      </div>
    );
  }

  const rows: Array<[string, string]> = [
    ['Patient number', patient.patientNumber],
    ['Sex', patient.gender],
    ['Date of birth', patient.dateOfBirth ?? '—'],
    ['Age', `${patient.ageYears ?? '—'} years / ${patient.ageMonths ?? '—'} months`],
    ['Phone', patient.phone ?? '—'],
    ['Alt. phone', patient.altPhone ?? '—'],
    ['Address', patient.address ?? '—'],
    ['Landmark', patient.addressLandmark ?? '—'],
    ['Tribe', patient.tribe ?? '—'],
    ['Occupation', patient.occupation ?? '—'],
    ['Marital status', patient.maritalStatus ?? '—'],
    ['Blood group', patient.bloodGroup ?? '—'],
    ['Genotype', patient.genotype ?? '—'],
    ['Allergies', patient.allergies ?? '—'],
    ['Registered', patient.registrationDate],
  ];

  return (
    <div className="space-y-5">
      <Link to="/patients" className="inline-flex items-center gap-1.5 text-sm text-text-muted hover:text-text">
        <ArrowLeft aria-hidden="true" className="size-3.5" />
        Patients
      </Link>

      <header className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-text">
            {patient.firstName} {patient.surname}
          </h1>
          <p className="mt-1 font-mono text-sm text-text-muted">{patient.patientNumber}</p>
        </div>
        <span
          className={
            patient.syncState === 'synced'
              ? 'rounded-xs bg-success-soft px-2 py-0.5 text-xs font-medium text-success-fg'
              : 'rounded-xs bg-primary-soft px-2 py-0.5 text-xs font-medium text-primary-fg'
          }
        >
          {patient.syncState === 'synced' ? 'Synced to server' : 'Saved on this device'}
        </span>
      </header>

      <section className="overflow-hidden rounded-md border border-border bg-surface">
        <dl className="divide-y divide-border">
          {rows.map(([label, value]) => (
            <div key={label} className="grid grid-cols-[11rem_1fr] gap-4 px-4 py-2.5">
              <dt className="text-sm text-text-muted">{label}</dt>
              <dd className="text-sm text-text">{value}</dd>
            </div>
          ))}
        </dl>
      </section>

      <p className="text-xs text-text-muted">
        Encounters, vitals and prescription history arrive with the queue in Sprint 4.
      </p>
    </div>
  );
}
