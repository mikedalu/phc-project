import { useState } from 'react';
import { createRoute, Link, useNavigate } from '@tanstack/react-router';
import { useLiveQuery } from 'dexie-react-hooks';
import { ArrowLeft } from 'lucide-react';
import { Route as rootRoute } from '@/routes/__root';
import { db } from '@/db';
import { Button } from '@/components/ui/button';
import { startVisit } from '@/features/queue/write';
import { toast } from 'sonner';
import { useLiveQuery as useVitalsQuery } from 'dexie-react-hooks';
import { humanError } from '@/lib/errors';

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
  const navigate = useNavigate();
  const [starting, setStarting] = useState(false);

  const onStartVisit = async () => {
    if (!patient) return;
    setStarting(true);
    try {
      await startVisit({
        patientId: patient.id,
        householdId: patient.householdId,
        priority: 0,
      });
      await navigate({ to: '/queue' });
    } catch (error) {
      toast.error(humanError(error, 'Could not start the visit.'));
      setStarting(false);
    }
  };

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
        <div className="flex items-center gap-2">
        <Button onClick={() => void onStartVisit()} disabled={starting}>
          Start visit
        </Button>
        <span
          className={
            patient.syncState === 'synced'
              ? 'rounded-xs bg-success-soft px-2 py-0.5 text-xs font-medium text-success-fg'
              : 'rounded-xs bg-primary-soft px-2 py-0.5 text-xs font-medium text-primary-fg'
          }
        >
          {patient.syncState === 'synced' ? 'Synced to server' : 'Saved on this device'}
        </span>
        </div>
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

      <VitalsTrend patientId={patient.id} />
    </div>
  );
}


/**
 * The vitals trend.
 *
 * A single reading is a number; five readings are a chart, and the chart is what
 * a doctor actually reads. Rows are newest first, and every row is a separate
 * observation — vitals are append-only, so nothing here was ever overwritten.
 */
function VitalsTrend({ patientId }: { patientId: string }) {
  const readings = useVitalsQuery(
    () => db.vitals.where('patientId').equals(patientId).toArray(),
    [patientId],
    [],
  );

  const sorted = [...readings].sort((a, b) => b.recordedAt.localeCompare(a.recordedAt)).slice(0, 10);

  return (
    <section className="rounded-md border border-border bg-surface">
      <div className="flex items-center justify-between border-b border-border bg-surface-sunken px-4 py-2">
        <h2 className="text-sm font-medium text-text">Vitals</h2>
        <span className="text-xs text-text-muted">
          {readings.length} reading{readings.length === 1 ? '' : 's'}
        </span>
      </div>

      {sorted.length === 0 ? (
        <p className="px-4 py-8 text-center text-sm text-text-muted">
          No readings yet. Record them from the queue.
        </p>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full border-collapse text-left text-sm">
            <caption className="sr-only">Recent vitals for this patient</caption>
            <thead>
              <tr className="border-b border-border text-xs uppercase tracking-wide text-text-muted">
                <th scope="col" className="px-3 py-2">When</th>
                <th scope="col" className="px-3 py-2">BP</th>
                <th scope="col" className="px-3 py-2">Temp</th>
                <th scope="col" className="px-3 py-2">Pulse</th>
                <th scope="col" className="px-3 py-2">Resp</th>
                <th scope="col" className="px-3 py-2">SpO₂</th>
                <th scope="col" className="px-3 py-2">Weight</th>
                <th scope="col" className="px-3 py-2">BMI</th>
                <th scope="col" className="px-3 py-2">State</th>
              </tr>
            </thead>
            <tbody>
              {sorted.map((v) => (
                <tr key={v.id} className="border-b border-border last:border-b-0">
                  <td className="px-3 py-2 text-xs text-text-muted">
                    {new Date(v.recordedAt).toLocaleString()}
                  </td>
                  <td className="px-3 py-2 tnum">
                    {v.bpSystolic ?? '—'}/{v.bpDiastolic ?? '—'}
                  </td>
                  <td className="px-3 py-2 tnum">{v.temperatureC ?? '—'}</td>
                  <td className="px-3 py-2 tnum">{v.pulseRate ?? '—'}</td>
                  <td className="px-3 py-2 tnum">{v.respiratoryRate ?? '—'}</td>
                  <td className="px-3 py-2 tnum">{v.spo2 ?? '—'}</td>
                  <td className="px-3 py-2 tnum">{v.weightKg ?? '—'}</td>
                  <td className="px-3 py-2 tnum">{v.bmi ?? '—'}</td>
                  <td className="px-3 py-2">
                    <span
                      className={
                        v.syncState === 'synced'
                          ? 'rounded-xs bg-success-soft px-1.5 py-0.5 text-xs text-success-fg'
                          : 'rounded-xs bg-primary-soft px-1.5 py-0.5 text-xs text-primary-fg'
                      }
                    >
                      {v.syncState}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}
