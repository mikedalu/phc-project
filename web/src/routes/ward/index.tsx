import { useState } from 'react';
import { createRoute, Link } from '@tanstack/react-router';
import { useLiveQuery } from 'dexie-react-hooks';
import { BedDouble, Loader2 } from 'lucide-react';
import { toast } from 'sonner';
import { Route as rootRoute } from '@/routes/__root';
import { db } from '@/db';
import { Button } from '@/components/ui/button';
import { chargeNight, dischargePatient, nightOf } from '@/features/ward/write';
import { formatMoney } from '@/features/billing/write';

/**
 * The ward board.
 *
 * Bed-days are charged one NIGHT at a time, because that is how a ward bills and
 * because it makes the double-charge impossible to hide: a night already on the
 * list cannot be added again.
 */
export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/ward',
  component: WardPage,
});

const WARD_DAY_MINOR = 300000; // ₦3,000.00, matching the seeded WARD_DAY tariff

function WardPage() {
  const admissions = useLiveQuery(() => db.admissions.toArray(), [], []);
  const charges = useLiveQuery(() => db.ward_charges.toArray(), [], []);
  const patients = useLiveQuery(() => db.patients.toArray(), [], []);

  const nameFor = (id: string) => {
    const p = patients.find((x) => x.id === id);
    return p ? `${p.firstName} ${p.surname}` : 'Unknown patient';
  };

  const current = admissions.filter((a) => a.status === 'admitted');
  const discharged = admissions
    .filter((a) => a.status === 'discharged')
    .sort((a, b) => b.updatedAt.localeCompare(a.updatedAt))
    .slice(0, 8);

  return (
    <div className="space-y-5">
      <header>
        <h1 className="text-2xl font-semibold tracking-tight text-text">Ward</h1>
        <p className="mt-1 text-sm text-text-muted">
          {current.length} inpatient{current.length === 1 ? '' : 's'} · billed nights:{' '}
          {charges.length}
        </p>
      </header>

      <section className="rounded-md border border-border bg-surface">
        <h2 className="flex items-center gap-2 border-b border-border bg-surface-sunken px-3 py-2 text-sm font-medium text-text">
          <BedDouble aria-hidden="true" className="size-4" />
          In the ward ({current.length})
        </h2>

        <div className="divide-y divide-border">
          {current.length === 0 ? (
            <p className="px-3 py-10 text-center text-sm text-text-muted">
              No inpatients. Admit a patient from their record.
            </p>
          ) : null}

          {current.map((admission) => {
            const nights = charges
              .filter((c) => c.admissionId === admission.id)
              .sort((a, b) => a.chargeDate.localeCompare(b.chargeDate));

            return (
              <div key={admission.id} className="px-3 py-3">
                <div className="flex flex-wrap items-start justify-between gap-2">
                  <div>
                    <Link
                      to="/patients/$id"
                      params={{ id: admission.patientId }}
                      className="text-sm font-medium text-primary hover:underline"
                    >
                      {nameFor(admission.patientId)}
                    </Link>
                    <p className="text-xs text-text-muted">
                      {admission.ward}
                      {admission.bedNumber ? ` · bed ${admission.bedNumber}` : ''} · admitted{' '}
                      {new Date(admission.admittedAt).toLocaleDateString()}
                    </p>
                  </div>
                  <p className="text-sm tnum text-text">
                    {formatMoney(nights.reduce((sum, n) => sum + n.amountMinor, 0))}
                  </p>
                </div>

                <WardActions admissionId={admission.id} nights={nights.map((n) => n.chargeDate)} />
              </div>
            );
          })}
        </div>
      </section>

      <section className="rounded-md border border-border bg-surface">
        <h2 className="border-b border-border bg-surface-sunken px-3 py-2 text-sm font-medium text-text">
          Discharged ({discharged.length})
        </h2>
        <div className="divide-y divide-border">
          {discharged.length === 0 ? (
            <p className="px-3 py-6 text-center text-xs text-text-muted">Nobody discharged yet.</p>
          ) : null}
          {discharged.map((admission) => (
            <div key={admission.id} className="flex items-center justify-between px-3 py-2">
              <div>
                <p className="text-sm text-text">{nameFor(admission.patientId)}</p>
                <p className="text-xs text-text-muted">{admission.ward}</p>
              </div>
              <span className="rounded-xs bg-success-soft px-1.5 py-0.5 text-xs text-success-fg">
                discharged
              </span>
            </div>
          ))}
        </div>
      </section>
    </div>
  );
}

function WardActions({ admissionId, nights }: { admissionId: string; nights: string[] }) {
  const [busy, setBusy] = useState(false);
  const tonight = nightOf(new Date());
  const already = nights.includes(tonight);

  const charge = async () => {
    setBusy(true);
    try {
      const billed = await chargeNight({
        admissionId,
        night: tonight,
        unitPriceMinor: WARD_DAY_MINOR,
      });
      toast[billed ? 'success' : 'message'](
        billed ? `Charged ${tonight}` : `${tonight} was already billed.`,
      );
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Could not charge the bed-day.');
    } finally {
      setBusy(false);
    }
  };

  const discharge = async () => {
    setBusy(true);
    try {
      await dischargePatient(admissionId, '');
      toast.success('Discharged.');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Could not discharge.');
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="mt-2 flex flex-wrap items-center gap-2">
      <Button size="sm" variant="secondary" onClick={() => void charge()} disabled={busy || already}>
        {busy ? <Loader2 aria-hidden="true" className="size-3.5 animate-spin" /> : null}
        {already ? 'Tonight billed' : `Charge ${tonight}`}
      </Button>
      <Button size="sm" variant="ghost" onClick={() => void discharge()} disabled={busy}>
        Discharge
      </Button>
      {nights.length > 0 ? (
        <span className="font-mono text-xs text-text-muted">{nights.join(' · ')}</span>
      ) : null}
    </div>
  );
}
