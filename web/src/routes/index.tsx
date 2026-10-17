import { createRoute, Link } from '@tanstack/react-router';
import { useLiveQuery } from 'dexie-react-hooks';
import {
  Banknote,
  BedDouble,
  FlaskConical,
  Pill,
  Stethoscope,
  UserPlus,
  Users,
} from 'lucide-react';
import { Route as rootRoute } from '@/routes/__root';
import { db } from '@/db';
import { useSession } from '@/features/auth/session';

/**
 * The dashboard.
 *
 * Everything on this page answers a question a member of staff actually asks at
 * the start of a shift: who is waiting, what needs doing, how much has been
 * taken. It reads Dexie, so it is correct with no network, and it shows nothing
 * about how the software is built.
 */
export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  component: DashboardPage,
});

function DashboardPage() {
  const user = useSession((s) => s.user);

  const queue = useLiveQuery(() => db.encounters.filter((e) => e.status === 'active').toArray(), [], []);
  const patients = useLiveQuery(() => db.patients.count(), [], 0);
  const invoices = useLiveQuery(() => db.invoices.toArray(), [], []);
  const orders = useLiveQuery(() => db.lab_orders.toArray(), [], []);
  const prescriptions = useLiveQuery(() => db.prescriptions.toArray(), [], []);
  const admissions = useLiveQuery(() => db.admissions.toArray(), [], []);
  const charges = useLiveQuery(() => db.charges.toArray(), [], []);
  const allocations = useLiveQuery(() => db.payment_allocations.toArray(), [], []);

  const waitingVitals = queue.filter((e) => e.workflowStage === 'registered').length;
  const waitingDoctor = queue.filter((e) => ['vitals_done', 'awaiting_doctor'].includes(e.workflowStage)).length;
  const inConsultation = queue.filter((e) => e.workflowStage === 'in_consultation').length;

  const toCollect = orders.filter((o) => o.status === 'ordered').length;
  const awaitingResult = orders.filter((o) => o.status === 'collected').length;

  const toDispense = prescriptions.filter((p) => p.status === 'paid').length;
  const awaitingPayment = prescriptions.filter((p) => p.status === 'pending_payment').length;

  const inpatients = admissions.filter((a) => a.status === 'admitted').length;
  const unpaid = invoices.filter((i) => i.status === 'unpaid' || i.status === 'partially_paid').length;

  const settledChargeIDs = new Set(allocations.map((a) => a.chargeId));
  const outstandingMinor = charges
    .filter((c) => c.status === 'active' && !settledChargeIDs.has(c.id))
    .reduce((sum, c) => sum + c.amountMinor, 0);

  const roles = user?.roles ?? [];
  const is = (role: string) => roles.includes(role);

  return (
    <div className="space-y-6">
      <header>
        <h1 className="text-2xl font-semibold tracking-tight text-text">
          {greeting()}, {user?.displayName ?? 'there'}
        </h1>
        <p className="mt-1 text-sm text-text-muted">
          {patients} patient{patients === 1 ? '' : 's'} on this device · {queue.length} open visit
          {queue.length === 1 ? '' : 's'}
        </p>
      </header>

      <section className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <Tile label="Waiting for vitals" value={waitingVitals} to="/queue" icon={Stethoscope} />
        <Tile label="Waiting for doctor" value={waitingDoctor} to="/queue" icon={Stethoscope} />
        <Tile label="In consultation" value={inConsultation} to="/queue" icon={Stethoscope} />
        <Tile label="Patients" value={patients} to="/patients" icon={Users} />
      </section>

      <section className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <Tile label="Specimens to collect" value={toCollect} to="/lab" icon={FlaskConical} />
        <Tile label="Results to enter" value={awaitingResult} to="/lab" icon={FlaskConical} />
        <Tile label="Prescriptions ready" value={toDispense} to="/pharmacy" icon={Pill} />
        <Tile label="Prescriptions unpaid" value={awaitingPayment} to="/pharmacy" icon={Pill} />
      </section>

      <section className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <Tile label="Inpatients" value={inpatients} to="/ward" icon={BedDouble} />
        <Tile label="Unpaid invoices" value={unpaid} to="/billing" icon={Banknote} />
        <Tile
          label="Outstanding"
          value={formatMoney(outstandingMinor)}
          to="/billing"
          icon={Banknote}
          isMoney
        />
        {is('admin') || is('front_desk') ? (
          <Link
            to="/patients/new"
            className="flex items-center gap-3 rounded-md border border-border bg-surface p-4 hover:border-border-strong"
          >
            <span className="grid size-9 place-items-center rounded-sm bg-primary-soft text-primary-fg">
              <UserPlus aria-hidden="true" className="size-4" />
            </span>
            <span>
              <span className="block text-sm font-medium text-text">Register a patient</span>
              <span className="block text-xs text-text-muted">Works without a connection</span>
            </span>
          </Link>
        ) : null}
      </section>

      <section className="rounded-md border border-border bg-surface p-4">
        <h2 className="text-sm font-medium text-text">Your shortcuts</h2>
        <div className="mt-2 flex flex-wrap gap-2">
          <Shortcut to="/queue" label="Open the queue" />
          <Shortcut to="/patients" label="Find a patient" />
          {is('pharmacist') || is('admin') ? <Shortcut to="/pharmacy" label="Pharmacy" /> : null}
          {is('lab_scientist') || is('admin') ? <Shortcut to="/laboratory" label="Laboratory" /> : null}
          {is('cashier') || is('front_desk') || is('accountant') || is('admin') ? (
            <Shortcut to="/billing" label="Billing" />
          ) : null}
          {is('nurse') || is('doctor') || is('admin') ? <Shortcut to="/ward" label="Ward" /> : null}
          {is('admin') ? <Shortcut to="/admin/users" label="Staff accounts" /> : null}
        </div>
      </section>
    </div>
  );
}

function Tile({
  label,
  value,
  to,
  icon: Icon,
  isMoney,
}: {
  label: string;
  value: number | string;
  to: string;
  icon: typeof Users;
  isMoney?: boolean;
}) {
  return (
    <Link
      to={to}
      className="rounded-md border border-border bg-surface p-4 transition-colors hover:border-border-strong"
    >
      <span className="flex items-center gap-2 text-xs font-medium uppercase tracking-wide text-text-muted">
        <Icon aria-hidden="true" className="size-3.5" />
        {label}
      </span>
      <span className={isMoney ? 'mt-1 block text-xl font-semibold tnum text-text' : 'mt-1 block text-3xl font-semibold tnum text-text'}>
        {value}
      </span>
    </Link>
  );
}

function Shortcut({ to, label }: { to: string; label: string }) {
  return (
    <Link
      to={to}
      className="rounded-sm border border-border px-3 py-1.5 text-sm text-primary hover:border-border-strong"
    >
      {label}
    </Link>
  );
}

function greeting(): string {
  const hour = new Date().getHours();
  if (hour < 12) return 'Good morning';
  if (hour < 17) return 'Good afternoon';
  return 'Good evening';
}

function formatMoney(minor: number): string {
  return `₦${Math.floor(minor / 100).toLocaleString()}.${String(minor % 100).padStart(2, '0')}`;
}
