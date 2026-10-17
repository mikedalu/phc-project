import { useState } from 'react';
import { createRoute, Link } from '@tanstack/react-router';
import { useLiveQuery } from 'dexie-react-hooks';
import { ArrowLeft, FlaskConical, Plus, Stethoscope } from 'lucide-react';
import { toast } from 'sonner';
import { Route as rootRoute } from '@/routes/__root';
import { db } from '@/db';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { FormField } from '@/components/ui/form-field';
import { addDiagnosis, addLabOrder } from '@/features/consultation/write';
import { searchICD10, type ICD10Entry } from '@/features/consultation/icd10';
import type { DiagnosisRow } from '@/db/types';

/**
 * The consultation screen: what the doctor concluded and what they ordered.
 *
 * Two panels on one visit, because they are one decision. Splitting them across
 * screens would mean a doctor writing a diagnosis, navigating away, and losing
 * the thread — and the diagnosis is what justifies the order.
 */
export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/consultations/$encounterId',
  component: ConsultationPage,
});

function ConsultationPage() {
  const { encounterId } = Route.useParams();

  const encounter = useLiveQuery(() => db.encounters.get(encounterId), [encounterId]);
  const patient = useLiveQuery(
    () => (encounter ? db.patients.get(encounter.patientId) : undefined),
    [encounter?.patientId],
  );
  const diagnoses = useLiveQuery(
    () => db.diagnoses.where('encounterId').equals(encounterId).toArray(),
    [encounterId],
    [],
  );
  const orders = useLiveQuery(
    () => db.lab_orders.where('encounterId').equals(encounterId).toArray(),
    [encounterId],
    [],
  );

  if (encounter === undefined) return <p className="text-sm text-text-muted">Loading…</p>;
  if (encounter === null) {
    return <p className="text-sm text-text">No visit with that id on this device.</p>;
  }

  return (
    <div className="space-y-5">
      <Link to="/queue" className="inline-flex items-center gap-1.5 text-sm text-text-muted hover:text-text">
        <ArrowLeft aria-hidden="true" className="size-3.5" />
        Queue
      </Link>

      <header className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-text">
            {patient ? `${patient.firstName} ${patient.surname}` : 'Consultation'}
          </h1>
          <p className="mt-1 font-mono text-sm text-text-muted">
            {encounter.visitNumber} · {encounter.workflowStage.replaceAll('_', ' ')}
          </p>
          {patient ? (
            <p className="text-sm text-text-subtle">
              {patient.ageYears ?? '—'} years · {patient.gender}
              {patient.patientNumber ? ` · ${patient.patientNumber}` : ''}
            </p>
          ) : null}
        </div>
        {patient ? (
          <Link
            to="/patients/$id"
            params={{ id: patient.id }}
            className="text-sm text-primary hover:underline"
          >
            Open full record
          </Link>
        ) : null}
      </header>

      <div className="grid gap-4 lg:grid-cols-2">
        <DiagnosisPanel
          patientId={encounter.patientId}
          encounterId={encounter.id}
          diagnoses={diagnoses}
        />
        <LabOrderPanel
          patientId={encounter.patientId}
          encounterId={encounter.id}
          orders={orders}
        />
      </div>
    </div>
  );
}

function DiagnosisPanel({
  patientId,
  encounterId,
  diagnoses,
}: {
  patientId: string;
  encounterId: string;
  diagnoses: DiagnosisRow[];
}) {
  const [description, setDescription] = useState('');
  const [code, setCode] = useState('');
  const [notes, setNotes] = useState('');
  const [suggestions, setSuggestions] = useState<ICD10Entry[]>([]);
  const [saving, setSaving] = useState(false);

  const pick = (entry: ICD10Entry) => {
    setCode(entry.code);
    setDescription(entry.label);
    setSuggestions([]);
  };

  const save = async () => {
    setSaving(true);
    try {
      await addDiagnosis({
        patientId,
        encounterId,
        diagnosisType: diagnoses.length === 0 ? 'primary' : 'secondary',
        icd10Code: code || null,
        description,
        clinicalNotes: notes || null,
      });
      setDescription('');
      setCode('');
      setNotes('');
      toast.success('Diagnosis recorded.');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Could not record the diagnosis.');
    } finally {
      setSaving(false);
    }
  };

  return (
    <section className="rounded-md border border-border bg-surface">
      <h2 className="flex items-center gap-2 border-b border-border bg-surface-sunken px-4 py-2 text-sm font-medium text-text">
        <Stethoscope aria-hidden="true" className="size-4" />
        Diagnosis ({diagnoses.length})
      </h2>

      <div className="space-y-3 p-4">
        <FormField label="Diagnosis" required>
          {(field) => (
            <div className="relative">
              <Input
                id={field.id}
                value={description}
                onChange={(event) => {
                  setDescription(event.target.value);
                  setSuggestions(searchICD10(event.target.value));
                }}
                onKeyDown={(event) => {
                  const first = suggestions[0];
                  if (event.key === 'Enter' && first) {
                    event.preventDefault();
                    pick(first);
                  }
                }}
                placeholder="Type a condition, or an ICD-10 code…"
                autoComplete="off"
              />
              {suggestions.length > 0 ? (
                <ul className="absolute z-10 mt-1 w-full overflow-hidden rounded-sm border border-border bg-surface shadow-md">
                  {suggestions.map((entry) => (
                    <li key={entry.code}>
                      <button
                        type="button"
                        onClick={() => pick(entry)}
                        className="flex w-full items-baseline gap-2 px-3 py-1.5 text-left text-sm hover:bg-surface-sunken"
                      >
                        <span className="font-mono text-xs text-text-muted">{entry.code}</span>
                        <span className="truncate">{entry.label}</span>
                      </button>
                    </li>
                  ))}
                </ul>
              ) : null}
            </div>
          )}
        </FormField>

        <div className="grid grid-cols-2 gap-3">
          <FormField label="ICD-10" hint="Optional, free text">
            {(field) => (
              <Input id={field.id} value={code} onChange={(event) => setCode(event.target.value)} />
            )}
          </FormField>
          <FormField label="Notes" hint="Optional">
            {(field) => (
              <Input id={field.id} value={notes} onChange={(event) => setNotes(event.target.value)} />
            )}
          </FormField>
        </div>

        <Button onClick={() => void save()} disabled={saving || !description.trim()}>
          <Plus aria-hidden="true" className="size-4" />
          Record diagnosis
        </Button>

        <ul className="divide-y divide-border">
          {diagnoses.map((d) => (
            <li key={d.id} className="py-2">
              <div className="flex items-baseline justify-between gap-2">
                <p className="text-sm text-text">{d.description}</p>
                <span className="shrink-0 rounded-xs bg-surface-sunken px-1.5 py-0.5 text-xs text-text-muted">
                  {d.diagnosisType}
                </span>
              </div>
              <p className="text-xs text-text-muted">
                {d.icd10Code ? `${d.icd10Code} · ` : ''}
                {new Date(d.diagnosedAt).toLocaleString()} · {d.syncState}
              </p>
            </li>
          ))}
        </ul>
      </div>
    </section>
  );
}

const COMMON_TESTS = [
  { code: 'MP', name: 'Malaria Parasite', specimen: 'blood' },
  { code: 'FBC', name: 'Full Blood Count', specimen: 'blood' },
  { code: 'URIN', name: 'Urinalysis', specimen: 'urine' },
  { code: 'RBS', name: 'Random Blood Sugar', specimen: 'blood' },
  { code: 'WIDAL', name: 'Widal Test', specimen: 'blood' },
  { code: 'GENEXP', name: 'GeneXpert (TB)', specimen: 'sputum' },
] as const;

function LabOrderPanel({
  patientId,
  encounterId,
  orders,
}: {
  patientId: string;
  encounterId: string;
  orders: Array<{ id: string; testName: string; status: string; priority: string; orderNumber: string; syncState: string }>;
}) {
  const [testCode, setTestCode] = useState<string>(COMMON_TESTS[0].code);
  const [priority, setPriority] = useState<'routine' | 'urgent' | 'stat'>('routine');
  const [saving, setSaving] = useState(false);

  const selected = COMMON_TESTS.find((t) => t.code === testCode) ?? COMMON_TESTS[0];

  const save = async () => {
    setSaving(true);
    try {
      await addLabOrder({
        patientId,
        encounterId,
        testCode: selected.code,
        testName: selected.name,
        specimenType: selected.specimen,
        clinicalIndication: null,
        priority,
      });
      toast.success(`${selected.name} ordered.`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Could not raise the order.');
    } finally {
      setSaving(false);
    }
  };

  return (
    <section className="rounded-md border border-border bg-surface">
      <h2 className="flex items-center gap-2 border-b border-border bg-surface-sunken px-4 py-2 text-sm font-medium text-text">
        <FlaskConical aria-hidden="true" className="size-4" />
        Lab orders ({orders.length})
      </h2>

      <div className="space-y-3 p-4">
        <div className="grid grid-cols-2 gap-3">
          <FormField label="Test">
            {(field) => (
              <select
                id={field.id}
                value={testCode}
                onChange={(event) => setTestCode(event.target.value)}
                className="h-9 w-full rounded-sm border border-border-strong bg-surface px-3 text-md text-text"
              >
                {COMMON_TESTS.map((test) => (
                  <option key={test.code} value={test.code}>
                    {test.name}
                  </option>
                ))}
              </select>
            )}
          </FormField>
          <FormField label="Priority">
            {(field) => (
              <select
                id={field.id}
                value={priority}
                onChange={(event) => setPriority(event.target.value as typeof priority)}
                className="h-9 w-full rounded-sm border border-border-strong bg-surface px-3 text-md text-text"
              >
                <option value="routine">Routine</option>
                <option value="urgent">Urgent</option>
                <option value="stat">STAT</option>
              </select>
            )}
          </FormField>
        </div>

        <Button onClick={() => void save()} disabled={saving}>
          <Plus aria-hidden="true" className="size-4" />
          Order test
        </Button>

        <ul className="divide-y divide-border">
          {orders.map((order) => (
            <li key={order.id} className="flex items-baseline justify-between gap-2 py-2">
              <div>
                <p className="text-sm text-text">{order.testName}</p>
                <p className="font-mono text-xs text-text-muted">{order.orderNumber}</p>
              </div>
              <div className="text-right">
                <span className="rounded-xs bg-surface-sunken px-1.5 py-0.5 text-xs text-text-muted">
                  {order.status}
                </span>
                {order.priority !== 'routine' ? (
                  <p className="mt-0.5 text-xs font-medium text-danger">{order.priority}</p>
                ) : null}
              </div>
            </li>
          ))}
        </ul>
      </div>
    </section>
  );
}
