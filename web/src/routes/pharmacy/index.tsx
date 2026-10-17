import { useState } from 'react';
import { createRoute, Link } from '@tanstack/react-router';
import { useLiveQuery } from 'dexie-react-hooks';
import { Loader2, Pill } from 'lucide-react';
import { toast } from 'sonner';
import { Route as rootRoute } from '@/routes/__root';
import { db } from '@/db';
import { Button } from '@/components/ui/button';
import { dispensePrescription } from '@/features/pharmacy/write';
import type { PrescriptionRow } from '@/db/types';
import { humanError } from '@/lib/errors';

/**
 * The pharmacy counter.
 *
 * Two columns, in the order the counter works: prescriptions waiting for money,
 * and prescriptions ready to be handed over. Dispensing is one button because
 * it is one action, repeated all day.
 */
export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/pharmacy',
  component: PharmacyPage,
});

function PharmacyPage() {
  const prescriptions = useLiveQuery(() => db.prescriptions.toArray(), [], []);
  const items = useLiveQuery(() => db.prescription_items.toArray(), [], []);
  const patients = useLiveQuery(() => db.patients.toArray(), [], []);

  const nameFor = (id: string) => {
    const p = patients.find((x) => x.id === id);
    return p ? `${p.firstName} ${p.surname}` : 'Unknown patient';
  };

  const pending = prescriptions.filter((r) => r.status === 'pending_payment');
  const ready = prescriptions.filter((r) => r.status === 'paid');
  const done = prescriptions
    .filter((r) => r.status === 'dispensed')
    .sort((a, b) => b.updatedAt.localeCompare(a.updatedAt))
    .slice(0, 8);

  return (
    <div className="space-y-5">
      <header>
        <h1 className="text-2xl font-semibold tracking-tight text-text">Pharmacy</h1>
        <p className="mt-1 text-sm text-text-muted">
          {pending.length} awaiting payment · {ready.length} ready to dispense · {done.length} dispensed
        </p>
      </header>

      <div className="grid gap-4 lg:grid-cols-2">
        <Column title={`Awaiting payment (${pending.length})`} empty="Nothing waiting for money.">
          {pending.map((rx) => (
            <RxCard key={rx.id} rx={rx} patientName={nameFor(rx.patientId)} lines={items.filter((i) => i.prescriptionId === rx.id)} />
          ))}
        </Column>

        <Column title={`Ready to dispense (${ready.length})`} empty="Nothing on the counter.">
          {ready.map((rx) => (
            <RxCard key={rx.id} rx={rx} patientName={nameFor(rx.patientId)} lines={items.filter((i) => i.prescriptionId === rx.id)} />
          ))}
        </Column>
      </div>

      <section className="rounded-md border border-border bg-surface">
        <h2 className="border-b border-border bg-surface-sunken px-3 py-2 text-sm font-medium text-text">
          Dispensed ({done.length})
        </h2>
        <div className="divide-y divide-border">
          {done.length === 0 ? (
            <p className="px-3 py-6 text-center text-xs text-text-muted">Nothing dispensed yet.</p>
          ) : null}
          {done.map((rx) => (
            <div key={rx.id} className="flex items-center justify-between px-3 py-2">
              <div>
                <p className="text-sm text-text">{nameFor(rx.patientId)}</p>
                <p className="font-mono text-xs text-text-muted">{rx.prescriptionNumber}</p>
              </div>
              <span className="rounded-xs bg-success-soft px-1.5 py-0.5 text-xs text-success-fg">dispensed</span>
            </div>
          ))}
        </div>
      </section>
    </div>
  );
}

function Column({ title, empty, children }: { title: string; empty: string; children: React.ReactNode }) {
  return (
    <section className="rounded-md border border-border bg-surface">
      <h2 className="flex items-center gap-2 border-b border-border bg-surface-sunken px-3 py-2 text-sm font-medium text-text">
        <Pill aria-hidden="true" className="size-4" />
        {title}
      </h2>
      <div className="space-y-2 p-2">
        {Array.isArray(children) && children.length === 0 ? (
          <p className="px-2 py-6 text-center text-xs text-text-muted">{empty}</p>
        ) : null}
        {children}
      </div>
    </section>
  );
}

function RxCard({
  rx,
  patientName,
  lines,
}: {
  rx: PrescriptionRow;
  patientName: string;
  lines: Array<{ id: string; drugName: string; dose: string; frequency: string; quantityPrescribed: number }>;
}) {
  const [busy, setBusy] = useState(false);

  const dispense = async () => {
    setBusy(true);
    try {
      await dispensePrescription(rx.id);
      toast.success(`${rx.prescriptionNumber} dispensed.`);
    } catch (error) {
      toast.error(humanError(error, 'Could not dispense the prescription.'));
    } finally {
      setBusy(false);
    }
  };

  return (
    <article className="rounded-sm border border-border p-2.5">
      <Link
        to="/patients/$id"
        params={{ id: rx.patientId }}
        className="block truncate text-sm font-medium text-primary hover:underline"
      >
        {patientName}
      </Link>
      <p className="font-mono text-xs text-text-muted">{rx.prescriptionNumber}</p>

      <ul className="mt-1 space-y-0.5">
        {lines.map((line) => (
          <li key={line.id} className="text-xs text-text-subtle">
            <span className="text-text">{line.drugName}</span> · {line.dose} · {line.frequency} ·{' '}
            {line.quantityPrescribed}
          </li>
        ))}
      </ul>

      {rx.status === 'paid' ? (
        <Button size="sm" className="mt-2" onClick={() => void dispense()} disabled={busy}>
          {busy ? <Loader2 aria-hidden="true" className="size-3.5 animate-spin" /> : null}
          Dispense
        </Button>
      ) : (
        <p className="mt-2 text-xs text-warning-fg">Waiting for the cashier.</p>
      )}
    </article>
  );
}
