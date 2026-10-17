import { useState } from 'react';
import { createRoute, Link } from '@tanstack/react-router';
import { useLiveQuery } from 'dexie-react-hooks';
import { ArrowLeft, FlaskConical, Loader2, Pill, Plus, Stethoscope } from 'lucide-react';
import { toast } from 'sonner';
import { Route as rootRoute } from '@/routes/__root';
import { db } from '@/db';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { FormField } from '@/components/ui/form-field';
import { addDiagnosis, addLabOrder, addPrescription } from '@/features/consultation/write';
import { formatMoney } from '@/features/billing/write';
import { searchICD10, type ICD10Entry } from '@/features/consultation/icd10';
import type { DiagnosisRow } from '@/db/types';
import { humanError } from '@/lib/errors';

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
        <div className="flex items-center gap-3">
          <Link
            to="/referrals/new"
            search={{ encounterId: encounter.id }}
            className="text-sm text-primary hover:underline"
          >
            Refer patient
          </Link>
          {patient ? (
            <Link
              to="/patients/$id"
              params={{ id: patient.id }}
              className="text-sm text-primary hover:underline"
            >
              Open full record
            </Link>
          ) : null}
        </div>
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

      <PrescriptionPanel patientId={encounter.patientId} encounterId={encounter.id} />
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
      toast.error(humanError(error, 'Could not record the diagnosis.'));
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
      toast.error(humanError(error, 'Could not raise the test order.'));
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


/**
 * Prescribing.
 *
 * Writes the prescription, its bill and its lines in ONE local transaction, so
 * the doctor can prescribe offline and the cashier still sees the bill.
 */
function PrescriptionPanel({ patientId, encounterId }: { patientId: string; encounterId: string }) {
  const prescriptions = useLiveQuery(
    () => db.prescriptions.where('encounterId').equals(encounterId).toArray(),
    [encounterId],
    [],
  );
  const items = useLiveQuery(
    () => db.prescription_items.toArray(),
    [],
    [],
  );

  const [drug, setDrug] = useState('');
  const [dose, setDose] = useState('1 tablet');
  const [frequency, setFrequency] = useState('BD');
  const [days, setDays] = useState('3');
  const [quantity, setQuantity] = useState('6');
  const [price, setPrice] = useState('500');
  const [lines, setLines] = useState<Array<{ drug: string; dose: string; frequency: string; days: number; quantity: number; price: number }>>([]);
  const [saving, setSaving] = useState(false);

  const addLine = () => {
    if (!drug.trim()) return;
    setLines((current) => [
      ...current,
      {
        drug: drug.trim(),
        dose: dose.trim(),
        frequency: frequency.trim(),
        days: Number(days) || 0,
        quantity: Number(quantity) || 0,
        price: Math.round((Number(price) || 0) * 100),
      },
    ]);
    setDrug('');
  };

  const save = async () => {
    setSaving(true);
    try {
      const result = await addPrescription({
        patientId,
        encounterId,
        lines: lines.map((l) => ({
          drugName: l.drug,
          dose: l.dose,
          frequency: l.frequency,
          durationDays: l.days || null,
          quantity: l.quantity,
          unitPriceMinor: l.price,
        })),
      });
      toast.success(`Prescribed · ${formatMoney(result.totalMinor)} billed.`);
      setLines([]);
    } catch (error) {
      toast.error(humanError(error, 'Could not save the prescription.'));
    } finally {
      setSaving(false);
    }
  };

  return (
    <section className="rounded-md border border-border bg-surface">
      <h2 className="flex items-center gap-2 border-b border-border bg-surface-sunken px-4 py-2 text-sm font-medium text-text">
        <Pill aria-hidden="true" className="size-4" />
        Prescription ({prescriptions.length})
      </h2>

      <div className="space-y-3 p-4">
        <div className="grid gap-2 sm:grid-cols-6">
          <FormField label="Drug">
            {(field) => (
              <Input
                id={field.id}
                value={drug}
                onChange={(e) => setDrug(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') addLine();
                }}
                placeholder="Artemether/Lumefantrine"
              />
            )}
          </FormField>
          <FormField label="Dose">
            {(field) => <Input id={field.id} value={dose} onChange={(e) => setDose(e.target.value)} />}
          </FormField>
          <FormField label="Frequency">
            {(field) => (
              <Input id={field.id} value={frequency} onChange={(e) => setFrequency(e.target.value)} />
            )}
          </FormField>
          <FormField label="Days">
            {(field) => (
              <Input id={field.id} inputMode="numeric" value={days} onChange={(e) => setDays(e.target.value)} />
            )}
          </FormField>
          <FormField label="Quantity">
            {(field) => (
              <Input id={field.id} inputMode="numeric" value={quantity} onChange={(e) => setQuantity(e.target.value)} />
            )}
          </FormField>
          <FormField label="Unit price (₦)">
            {(field) => (
              <Input id={field.id} inputMode="decimal" value={price} onChange={(e) => setPrice(e.target.value)} />
            )}
          </FormField>
        </div>

        <Button size="sm" variant="secondary" onClick={addLine} disabled={!drug.trim()}>
          <Plus aria-hidden="true" className="size-3.5" />
          Add drug
        </Button>

        {lines.length > 0 ? (
          <ul className="divide-y divide-border rounded-sm border border-border">
            {lines.map((line, index) => (
              <li key={`${line.drug}-${index}`} className="flex items-center justify-between px-3 py-1.5 text-sm">
                <span>
                  <span className="text-text">{line.drug}</span>{' '}
                  <span className="text-text-muted">
                    · {line.dose} · {line.frequency} · {line.days}d · ×{line.quantity}
                  </span>
                </span>
                <span className="tnum text-text">{formatMoney(line.price * line.quantity)}</span>
              </li>
            ))}
          </ul>
        ) : null}

        <Button onClick={() => void save()} disabled={saving || lines.length === 0}>
          {saving ? <Loader2 aria-hidden="true" className="size-4 animate-spin" /> : null}
          Prescribe {lines.length > 0 ? `(${formatMoney(lines.reduce((s, l) => s + l.price * l.quantity, 0))})` : ''}
        </Button>

        <ul className="divide-y divide-border">
          {prescriptions.map((rx) => {
            const rxItems = items.filter((i) => i.prescriptionId === rx.id);
            return (
              <li key={rx.id} className="py-2">
                <div className="flex items-baseline justify-between gap-2">
                  <p className="font-mono text-xs text-text-muted">{rx.prescriptionNumber}</p>
                  <span className="rounded-xs bg-surface-sunken px-1.5 py-0.5 text-xs text-text-muted">
                    {rx.status.replaceAll('_', ' ')}
                  </span>
                </div>
                <p className="text-xs text-text-subtle">
                  {rxItems.map((i) => `${i.drugName} ${i.dose} ${i.frequency}`).join(' · ')}
                </p>
              </li>
            );
          })}
        </ul>
      </div>
    </section>
  );
}
