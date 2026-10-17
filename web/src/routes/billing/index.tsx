import { useState } from 'react';
import { createRoute, Link } from '@tanstack/react-router';
import { useLiveQuery } from 'dexie-react-hooks';
import { Banknote, Loader2, Receipt } from 'lucide-react';
import { toast } from 'sonner';
import { Route as rootRoute } from '@/routes/__root';
import { db } from '@/db';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { takePayment, formatMoney } from '@/features/billing/write';
import { cn } from '@/lib/cn';

/**
 * The cashier's desk.
 *
 * Outstanding invoices, newest first, with the balance DERIVED from charges minus
 * allocations on every render. Nothing on this screen trusts a stored total to
 * tell it what is owed.
 */
export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/billing',
  component: BillingPage,
});

function BillingPage() {
  const invoices = useLiveQuery(() => db.invoices.toArray(), [], []);
  const charges = useLiveQuery(() => db.charges.toArray(), [], []);
  const allocations = useLiveQuery(() => db.payment_allocations.toArray(), [], []);
  const patients = useLiveQuery(() => db.patients.toArray(), [], []);
  const payments = useLiveQuery(() => db.payments.orderBy('paidAt').reverse().limit(8).toArray(), [], []);

  const nameFor = (id: string) => {
    const p = patients.find((x) => x.id === id);
    return p ? `${p.firstName} ${p.surname}` : 'Unknown patient';
  };

  // DERIVED, never read from a column.
  const paidFor = (invoiceId: string) => {
    const chargeIds = new Set(charges.filter((c) => c.invoiceId === invoiceId && c.status === 'active').map((c) => c.id));
    return allocations
      .filter((a) => chargeIds.has(a.chargeId))
      .reduce((sum, a) => sum + a.amountMinor, 0);
  };
  const totalFor = (invoiceId: string) =>
    charges.filter((c) => c.invoiceId === invoiceId && c.status === 'active').reduce((s, c) => s + c.amountMinor, 0);

  const outstanding = invoices
    .filter((i) => i.status !== 'paid' && i.status !== 'void')
    .sort((a, b) => a.issuedAt.localeCompare(b.issuedAt));

  const collected = payments.reduce((sum, p) => sum + p.amountMinor, 0);

  return (
    <div className="space-y-5">
      <header className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-text">Billing</h1>
          <p className="mt-1 text-sm text-text-muted">
            {outstanding.length} unpaid · collected today {formatMoney(collected)}
          </p>
        </div>
      </header>

      <section className="rounded-md border border-border bg-surface">
        <h2 className="flex items-center gap-2 border-b border-border bg-surface-sunken px-3 py-2 text-sm font-medium text-text">
          <Banknote aria-hidden="true" className="size-4" />
          Outstanding invoices
        </h2>

        <div className="divide-y divide-border">
          {outstanding.length === 0 ? (
            <p className="px-3 py-10 text-center text-sm text-text-muted">
              Nothing outstanding. Order a test or raise a bill from a consultation.
            </p>
          ) : null}

          {outstanding.map((invoice) => {
            const total = totalFor(invoice.id);
            const paid = paidFor(invoice.id);
            const due = total - paid;
            return (
              <div key={invoice.id} className="flex flex-wrap items-center justify-between gap-3 px-3 py-3">
                <div className="min-w-0">
                  <Link
                    to="/patients/$id"
                    params={{ id: invoice.patientId }}
                    className="text-sm font-medium text-primary hover:underline"
                  >
                    {nameFor(invoice.patientId)}
                  </Link>
                  <p className="font-mono text-xs text-text-muted">{invoice.invoiceNumber}</p>
                </div>

                <div className="text-right">
                  <p className="text-sm font-semibold tnum text-text">{formatMoney(due)}</p>
                  <p className="text-xs text-text-muted">
                    of {formatMoney(total)}
                    {invoice.status === 'partially_paid' ? ' · part paid' : ''}
                  </p>
                </div>

                <TakePayment invoiceId={invoice.id} due={due} />
              </div>
            );
          })}
        </div>
      </section>

      <section className="rounded-md border border-border bg-surface">
        <h2 className="flex items-center gap-2 border-b border-border bg-surface-sunken px-3 py-2 text-sm font-medium text-text">
          <Receipt aria-hidden="true" className="size-4" />
          Recent receipts
        </h2>
        <div className="divide-y divide-border">
          {payments.length === 0 ? (
            <p className="px-3 py-8 text-center text-sm text-text-muted">No payments yet.</p>
          ) : null}
          {payments.map((payment) => (
            <div key={payment.id} className="flex items-center justify-between px-3 py-2">
              <div>
                <p className="font-mono text-xs text-text-muted">{payment.receiptNumber}</p>
                <p className="text-xs text-text-subtle">
                  {new Date(payment.paidAt).toLocaleString()} · {payment.paymentMethod}
                </p>
              </div>
              <div className="text-right">
                <p className="text-sm tnum text-text">{formatMoney(payment.amountMinor)}</p>
                <p className="text-xs text-text-muted">{payment.syncState}</p>
              </div>
            </div>
          ))}
        </div>
      </section>
    </div>
  );
}

function TakePayment({ invoiceId, due }: { invoiceId: string; due: number }) {
  const [open, setOpen] = useState(false);
  const [amount, setAmount] = useState('');
  const [method, setMethod] = useState<'cash' | 'transfer' | 'pos_card'>('cash');
  const [busy, setBusy] = useState(false);

  const minor = Math.round(Number(amount) * 100);

  const pay = async () => {
    setBusy(true);
    try {
      const payment = await takePayment({
        invoiceId,
        amountMinor: minor,
        method,
      });
      toast.success(`Paid ${formatMoney(payment.amountMinor)} · ${payment.receiptNumber}`);
      setOpen(false);
      setAmount('');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Could not take the payment.');
    } finally {
      setBusy(false);
    }
  };

  if (!open) {
    return (
      <Button size="sm" onClick={() => setOpen(true)}>
        Take payment
      </Button>
    );
  }

  return (
    <div className="flex flex-wrap items-end gap-2">
      <label className="flex flex-col gap-1">
        <span className="text-xs text-text-muted">Amount (₦)</span>
        <Input
          size="sm"
          autoFocus
          inputMode="decimal"
          value={amount}
          placeholder={(due / 100).toFixed(2)}
          onChange={(e) => setAmount(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && minor > 0) void pay();
          }}
          className="w-28"
        />
      </label>
      <select
        value={method}
        onChange={(e) => setMethod(e.target.value as typeof method)}
        className="h-8 rounded-sm border border-border-strong bg-surface px-2 text-sm text-text"
        aria-label="Payment method"
      >
        <option value="cash">Cash</option>
        <option value="transfer">Transfer</option>
        <option value="pos_card">POS</option>
      </select>
      <Button size="sm" onClick={() => void pay()} disabled={busy || minor <= 0 || minor > due}>
        {busy ? <Loader2 aria-hidden="true" className="size-3.5 animate-spin" /> : null}
        Pay
      </Button>
      <Button size="sm" variant="ghost" onClick={() => setOpen(false)}>
        Cancel
      </Button>
      {minor > due ? (
        <span className={cn('text-xs text-danger')}>More than the balance</span>
      ) : null}
    </div>
  );
}
