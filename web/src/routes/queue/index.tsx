import { useState } from 'react';
import { createRoute, Link } from '@tanstack/react-router';
import { useLiveQuery } from 'dexie-react-hooks';
import { toast } from 'sonner';
import { Route as rootRoute } from '@/routes/__root';
import { db } from '@/db';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { moveStage, recordVitals } from '@/features/queue/write';
import type { EncounterRow } from '@/db/types';
import { cn } from '@/lib/cn';

/**
 * The queue board, derived from `encounters` — an encounter IS the queue entry,
 * so a patient is in exactly one stage. Vitals are entered IN the card: the
 * nurse calls the next patient, types numbers, presses Enter, and the patient
 * moves to the doctor's column. A modal would add a click to the one action
 * performed forty times a day.
 */
export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/queue',
  component: QueuePage,
});

const COLUMNS: Array<{ stages: string[]; title: string }> = [
  { stages: ['registered'], title: 'Waiting for vitals' },
  { stages: ['vitals_done', 'awaiting_doctor'], title: 'Waiting for doctor' },
  { stages: ['in_consultation'], title: 'In consultation' },
  {
    stages: [
      'awaiting_lab', 'awaiting_lab_payment', 'lab_done',
      'awaiting_drug_payment', 'awaiting_pharmacy', 'pharmacy_done', 'admitted',
    ],
    title: 'Elsewhere',
  },
];

function QueuePage() {
  const encounters = useLiveQuery(
    () => db.encounters.filter((e) => e.status === 'active').toArray(),
    [],
    [],
  );
  const patients = useLiveQuery(() => db.patients.toArray(), [], []);

  const nameFor = (patientId: string) => {
    const p = patients.find((x) => x.id === patientId);
    return p ? `${p.firstName} ${p.surname}` : 'Unknown patient';
  };

  return (
    <div className="space-y-5">
      <header>
        <h1 className="text-2xl font-semibold tracking-tight text-text">Queue</h1>
        <p className="mt-1 text-sm text-text-muted">
          {encounters.length} active {encounters.length === 1 ? 'visit' : 'visits'} · live from this device
        </p>
      </header>

      <div className="grid gap-4 lg:grid-cols-4">
        {COLUMNS.map((column) => {
          const items = encounters
            .filter((e) => column.stages.includes(e.workflowStage))
            .sort((a, b) => b.priority - a.priority || a.queuedAt.localeCompare(b.queuedAt));

          return (
            <section key={column.title} className="rounded-md border border-border bg-surface">
              <h2 className="flex items-center justify-between border-b border-border bg-surface-sunken px-3 py-2 text-sm font-medium text-text">
                {column.title}
                <span className="rounded-xs bg-surface px-1.5 py-0.5 text-xs text-text-muted">{items.length}</span>
              </h2>
              <div className="space-y-2 p-2">
                {items.length === 0 ? (
                  <p className="px-2 py-6 text-center text-xs text-text-muted">Nobody here.</p>
                ) : null}
                {items.map((encounter) => (
                  <QueueCard key={encounter.id} encounter={encounter} patientName={nameFor(encounter.patientId)} />
                ))}
              </div>
            </section>
          );
        })}
      </div>

      <p className="text-xs text-text-muted">
        Start a visit from a patient&rsquo;s profile. The board updates on every device as visits sync.
      </p>
    </div>
  );
}

function QueueCard({ encounter, patientName }: { encounter: EncounterRow; patientName: string }) {
  const [showVitals, setShowVitals] = useState(false);
  const [busy, setBusy] = useState(false);

  const advance = async (to: string) => {
    setBusy(true);
    try {
      await moveStage(encounter.id, to);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Could not move the patient.');
    } finally {
      setBusy(false);
    }
  };

  return (
    <article
      className={cn(
        'rounded-sm border p-2.5',
        encounter.priority > 0 ? 'border-danger/40 bg-danger-soft/40' : 'border-border',
      )}
    >
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <Link
            to="/patients/$id"
            params={{ id: encounter.patientId }}
            className="block truncate text-sm font-medium text-primary hover:underline"
          >
            {patientName}
          </Link>
          <p className="font-mono text-xs text-text-muted">{encounter.visitNumber}</p>
        </div>
        {encounter.priority > 0 ? (
          <span className="rounded-xs bg-danger px-1.5 py-0.5 text-xs font-medium text-text-inverse">
            {encounter.priority === 2 ? 'Emergency' : 'Urgent'}
          </span>
        ) : null}
      </div>

      {showVitals ? (
        <VitalsForm
          encounter={encounter}
          onDone={() => {
            setShowVitals(false);
            toast.success('Vitals recorded.');
          }}
        />
      ) : (
        <div className="mt-2 flex flex-wrap gap-1.5">
          {encounter.workflowStage === 'registered' ? (
            <Button size="sm" onClick={() => setShowVitals(true)}>Record vitals</Button>
          ) : null}
          {encounter.workflowStage === 'vitals_done' ? (
            <Button size="sm" onClick={() => void advance('awaiting_doctor')} disabled={busy}>Send to doctor</Button>
          ) : null}
          {encounter.workflowStage === 'awaiting_doctor' ? (
            <Button size="sm" onClick={() => void advance('in_consultation')} disabled={busy}>Open consultation</Button>
          ) : null}
          {encounter.workflowStage === 'in_consultation' ? (
            <Button size="sm" variant="secondary" onClick={() => void advance('closed')} disabled={busy}>Close visit</Button>
          ) : null}
        </div>
      )}
    </article>
  );
}

function VitalsForm({ encounter, onDone }: { encounter: EncounterRow; onDone: () => void }) {
  const [values, setValues] = useState({
    bpSystolic: '', bpDiastolic: '', temperatureC: '', pulseRate: '',
    respiratoryRate: '', heightCm: '', weightKg: '', spo2: '',
  });
  const [saving, setSaving] = useState(false);

  const num = (v: string) => (v.trim() === '' ? null : Number(v));

  const save = async () => {
    setSaving(true);
    try {
      await recordVitals({
        patientId: encounter.patientId,
        encounterId: encounter.id,
        bpSystolic: num(values.bpSystolic),
        bpDiastolic: num(values.bpDiastolic),
        temperatureC: num(values.temperatureC),
        pulseRate: num(values.pulseRate),
        respiratoryRate: num(values.respiratoryRate),
        heightCm: num(values.heightCm),
        weightKg: num(values.weightKg),
        spo2: num(values.spo2),
      });
      onDone();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Could not save vitals.');
      setSaving(false);
    }
  };

  const field = (key: keyof typeof values, label: string, suffix?: string, autoFocus = false) => (
    <label className="flex flex-col gap-1">
      <span className="text-xs text-text-muted">{label}{suffix ? ` (${suffix})` : ''}</span>
      <Input
        size="sm"
        inputMode="decimal"
        autoFocus={autoFocus}
        value={values[key]}
        onChange={(event) => setValues((v) => ({ ...v, [key]: event.target.value }))}
        onKeyDown={(event) => {
          if (event.key === 'Enter') void save();
        }}
      />
    </label>
  );

  return (
    <div className="mt-2 space-y-2 rounded-sm bg-surface-sunken p-2">
      <div className="grid grid-cols-2 gap-2">
        {field('bpSystolic', 'Systolic', 'mmHg', true)}
        {field('bpDiastolic', 'Diastolic', 'mmHg')}
        {field('temperatureC', 'Temp', '°C')}
        {field('pulseRate', 'Pulse', 'bpm')}
        {field('respiratoryRate', 'Resp', '/min')}
        {field('spo2', 'SpO₂', '%')}
        {field('heightCm', 'Height', 'cm')}
        {field('weightKg', 'Weight', 'kg')}
      </div>
      <div className="flex gap-2">
        <Button size="sm" onClick={() => void save()} disabled={saving}>Save vitals</Button>
        <Button size="sm" variant="ghost" onClick={onDone}>Cancel</Button>
      </div>
    </div>
  );
}
