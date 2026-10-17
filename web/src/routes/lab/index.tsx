import { useState } from 'react';
import { createRoute, Link } from '@tanstack/react-router';
import { useLiveQuery } from 'dexie-react-hooks';
import { ArrowRight, FlaskConical, Loader2 } from 'lucide-react';
import { toast } from 'sonner';
import { Route as rootRoute } from '@/routes/__root';
import { db } from '@/db';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { collectOrder, recordResult } from '@/features/lab/write';
import type { LabOrderRow, LabResultRow } from '@/db/types';
import { cn } from '@/lib/cn';

/**
 * The lab worklist.
 *
 * Ordered in the order the bench works: specimens to collect first, then
 * collected specimens awaiting a result. An order is a single row, so a
 * specimen cannot be in two states at once.
 */
export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/lab',
  component: LabWorklistPage,
});

function LabWorklistPage() {
  const orders = useLiveQuery(() => db.lab_orders.toArray(), [], []);
  const patients = useLiveQuery(() => db.patients.toArray(), [], []);
  const results = useLiveQuery(() => db.lab_results.toArray(), [], []);

  const nameFor = (id: string) => {
    const p = patients.find((x) => x.id === id);
    return p ? `${p.firstName} ${p.surname}` : 'Unknown patient';
  };

  const toCollect = orders
    .filter((o) => o.status === 'ordered')
    .sort((a, b) => priorityRank(b.priority) - priorityRank(a.priority));
  const toResult = orders.filter((o) => o.status === 'collected');
  const done = orders.filter((o) => o.status === 'resulted');

  return (
    <div className="space-y-5">
      <header>
        <h1 className="text-2xl font-semibold tracking-tight text-text">Laboratory</h1>
        <p className="mt-1 text-sm text-text-muted">
          {toCollect.length} to collect · {toResult.length} awaiting results · {done.length} reported
        </p>
      </header>

      <div className="grid gap-4 lg:grid-cols-2">
        <section className="rounded-md border border-border bg-surface">
          <h2 className="flex items-center gap-2 border-b border-border bg-surface-sunken px-3 py-2 text-sm font-medium text-text">
            <FlaskConical aria-hidden="true" className="size-4" />
            Collect specimen ({toCollect.length})
          </h2>
          <div className="space-y-2 p-2">
            {toCollect.length === 0 ? (
              <p className="px-2 py-6 text-center text-xs text-text-muted">Nothing waiting.</p>
            ) : null}
            {toCollect.map((order) => (
              <OrderCard key={order.id} order={order} patientName={nameFor(order.patientId)} />
            ))}
          </div>
        </section>

        <section className="rounded-md border border-border bg-surface">
          <h2 className="flex items-center gap-2 border-b border-border bg-surface-sunken px-3 py-2 text-sm font-medium text-text">
            Awaiting result ({toResult.length})
          </h2>
          <div className="space-y-2 p-2">
            {toResult.length === 0 ? (
              <p className="px-2 py-6 text-center text-xs text-text-muted">Nothing on the bench.</p>
            ) : null}
            {toResult.map((order) => (
              <OrderCard key={order.id} order={order} patientName={nameFor(order.patientId)} />
            ))}
          </div>
        </section>
      </div>

      <section className="rounded-md border border-border bg-surface">
        <h2 className="border-b border-border bg-surface-sunken px-3 py-2 text-sm font-medium text-text">
          Reported ({done.length})
        </h2>
        <div className="divide-y divide-border">
          {done.length === 0 ? (
            <p className="px-3 py-6 text-center text-xs text-text-muted">No results yet.</p>
          ) : null}
          {done.map((order) => {
            const orderResults = results.filter((r) => r.labOrderId === order.id);
            return (
              <div key={order.id} className="flex flex-wrap items-baseline justify-between gap-2 px-3 py-2">
                <div>
                  <p className="text-sm text-text">{order.testName}</p>
                  <p className="font-mono text-xs text-text-muted">{order.orderNumber}</p>
                </div>
                <p className="text-xs text-text-subtle">
                  {orderResults.length} parameter{orderResults.length === 1 ? '' : 's'} ·{' '}
                  {orderResults.map((r) => resultSummary(r)).join(', ') || 'no values'}
                </p>
              </div>
            );
          })}
        </div>
      </section>
    </div>
  );
}

function priorityRank(p: string) {
  return p === 'stat' ? 3 : p === 'urgent' ? 2 : 1;
}

function resultSummary(result: LabResultRow) {
  const value = result.valueText ?? result.valueNumeric ?? '—';
  return `${result.parameterName}: ${value}${result.unit ? ` ${result.unit}` : ''}`;
}

function OrderCard({ order, patientName }: { order: LabOrderRow; patientName: string }) {
  const [busy, setBusy] = useState(false);
  const [showResult, setShowResult] = useState(false);

  const collect = async () => {
    setBusy(true);
    try {
      await collectOrder(order.id);
      toast.success('Specimen collected.');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Could not collect the specimen.');
    } finally {
      setBusy(false);
    }
  };

  return (
    <article
      className={cn(
        'rounded-sm border p-2.5',
        order.priority !== 'routine' ? 'border-danger/40 bg-danger-soft/40' : 'border-border',
      )}
    >
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <Link
            to="/patients/$id"
            params={{ id: order.patientId }}
            className="block truncate text-sm font-medium text-primary hover:underline"
          >
            {patientName}
          </Link>
          <p className="text-sm text-text">{order.testName}</p>
          <p className="font-mono text-xs text-text-muted">{order.orderNumber}</p>
        </div>
        {order.priority !== 'routine' ? (
          <span className="rounded-xs bg-danger px-1.5 py-0.5 text-xs font-medium text-text-inverse">
            {order.priority}
          </span>
        ) : null}
      </div>

      {showResult ? (
        <ResultForm order={order} onDone={() => setShowResult(false)} />
      ) : (
        <div className="mt-2 flex gap-1.5">
          {order.status === 'ordered' ? (
            <Button size="sm" onClick={() => void collect()} disabled={busy}>
              {busy ? <Loader2 aria-hidden="true" className="size-3.5 animate-spin" /> : null}
              Collect specimen
            </Button>
          ) : (
            <Button size="sm" onClick={() => setShowResult(true)}>
              Enter result
              <ArrowRight aria-hidden="true" className="size-3.5" />
            </Button>
          )}
        </div>
      )}
    </article>
  );
}

function ResultForm({ order, onDone }: { order: LabOrderRow; onDone: () => void }) {
  const [parameter, setParameter] = useState(defaultParameter(order.testCode));
  const [value, setValue] = useState('');
  const [unit, setUnit] = useState('');
  const [saving, setSaving] = useState(false);

  const asNumber = Number(value);
  const numeric = value.trim() !== '' && !Number.isNaN(asNumber) && /^[0-9.]+$/.test(value.trim());

  const save = async () => {
    setSaving(true);
    try {
      await recordResult({
        orderId: order.id,
        parameterName: parameter,
        valueText: numeric ? null : value.trim() || null,
        valueNumeric: numeric ? asNumber : null,
        unit: unit || null,
        referenceRange: null,
        abnormalFlag: null,
      });
      toast.success('Result recorded.');
      onDone();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Could not record the result.');
      setSaving(false);
    }
  };

  return (
    <div className="mt-2 space-y-2 rounded-sm bg-surface-sunken p-2">
      <div className="grid grid-cols-3 gap-2">
        <label className="flex flex-col gap-1">
          <span className="text-xs text-text-muted">Parameter</span>
          <Input size="sm" value={parameter} onChange={(e) => setParameter(e.target.value)} />
        </label>
        <label className="flex flex-col gap-1">
          <span className="text-xs text-text-muted">Value</span>
          <Input
            size="sm"
            autoFocus
            value={value}
            onChange={(e) => setValue(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') void save();
            }}
          />
        </label>
        <label className="flex flex-col gap-1">
          <span className="text-xs text-text-muted">Unit</span>
          <Input size="sm" value={unit} onChange={(e) => setUnit(e.target.value)} />
        </label>
      </div>
      <div className="flex gap-2">
        <Button size="sm" onClick={() => void save()} disabled={saving || !value.trim()}>
          Save result
        </Button>
        <Button size="sm" variant="ghost" onClick={onDone}>
          Cancel
        </Button>
      </div>
    </div>
  );
}

/** Sensible default parameter per test, so the common case is one keystroke. */
function defaultParameter(testCode: string): string {
  switch (testCode) {
    case 'MP':
      return 'Malaria Parasite';
    case 'FBC':
      return 'Haemoglobin';
    case 'URIN':
      return 'Protein';
    case 'RBS':
      return 'Glucose';
    case 'WIDAL':
      return 'O antigen';
    case 'GENEXP':
      return 'MTB detected';
    default:
      return 'Result';
  }
}
